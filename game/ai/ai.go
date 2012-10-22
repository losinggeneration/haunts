package ai

import (
	"encoding/gob"
	"errors"
	"fmt"
	"github.com/howeyc/fsnotify"
	"github.com/runningwild/haunts/base"
	"github.com/runningwild/haunts/game"
	lua "github.com/xenith-studios/golua"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
)

// The Ai struct contains a glop.AiGraph object as well as a few channels for
// communicating with the game itself.
type Ai struct {
	L *lua.State

	game *game.Game

	ent *game.Entity

	// Path to the script to run
	path string

	// The actual lua program to run when executing this ai
	Prog string

	// Need to store this so that we know what files to reload if anything
	// changes on disk
	kind game.AiKind

	// Watch all of the files that this Ai depends on so that we can reload it
	// if any of them change
	watcher *fsnotify.Watcher

	// Set to true at the beginning of the turn, turned off as soon as the ai is
	// done for the turn.
	active       bool
	evaluating   bool
	active_set   chan bool
	active_query chan bool
	exec_query   chan struct{}
	terminate    chan struct{}

	// Once we send an Action for execution we have to wait until it is done
	// before we make the next one.  This channel is used to handle that.
	pause chan struct{}
	execs chan game.ActionExec

	// This exists so that we can gob this without error.  Gob doesn't like
	// gobbing things that don't have any exported fields, and since we might
	// want exported fields later we'll just have this here for now so we can
	// leave the Ai as an exported field in the entities.
	Dummy int
}

func init() {
	gob.Register(&Ai{})
	game.SetAiMaker(makeAi)
}

func makeAi(path string, g *game.Game, ent *game.Entity, dst_iface *game.Ai, kind game.AiKind) {
	ai_struct := new(Ai)
	ai_struct.path = path
	var err error
	ai_struct.watcher, err = fsnotify.NewWatcher()
	if err != nil {
		base.Warn().Printf("Unable to create a filewatcher - '%s' will not reload ai files dynamically: %v", path, err)
		ai_struct.watcher = nil
	}
	ai_struct.ent = ent
	ai_struct.game = g

	ai_struct.active_set = make(chan bool)
	ai_struct.active_query = make(chan bool)
	ai_struct.exec_query = make(chan struct{})
	ai_struct.pause = make(chan struct{})
	ai_struct.terminate = make(chan struct{})
	ai_struct.execs = make(chan game.ActionExec)
	ai_struct.kind = kind

	err = ai_struct.setupLuaState()
	if err != nil {
		base.Error().Printf("Unable to make ai: %v", err)
		if ai_struct.watcher != nil {
			ai_struct.watcher.Close()
		}
		dst_iface = nil
		return
	}
	go ai_struct.masterRoutine()

	*dst_iface = ai_struct
}

func (a *Ai) setupLuaState() error {
	base.CheckPathCasing(a.path)
	prog, err := ioutil.ReadFile(a.path)
	if err != nil {
		return errors.New(fmt.Sprintf("Unable to load ai file %s: %v", a.path, err))
	}
	a.Prog = string(prog)
	a.watcher.Watch(a.path)
	a.L = lua.NewState()
	a.L.OpenLibs()
	switch a.kind {
	case game.EntityAi:
		a.addEntityContext()
		a.loadUtils("entity")
		if a.ent.Side() == game.SideHaunt {
			a.loadUtils("denizen_entity")
		}
		if a.ent.Side() == game.SideExplorers {
			a.loadUtils("intruder_entity")
		}

	case game.MinionsAi:
		a.addMinionsContext()
		a.loadUtils("minions")

	case game.DenizensAi:
		a.addDenizensContext()
		a.loadUtils("denizens")

	case game.IntrudersAi:
		a.addIntrudersContext()
		a.loadUtils("intruders")

	default:
		panic("Unknown ai kind")
	}
	// Add this to all contexts
	a.L.Register("print", func(L *lua.State) int {
		var res string
		n := L.GetTop()
		for i := -n; i < 0; i++ {
			res += game.LuaStringifyParam(L, i) + " "
		}
		base.Log().Printf("Ai(%p): %s", a, res)
		return 0
	})
	a.L.Register("randN", func(L *lua.State) int {
		n := L.GetTop()
		if n == 0 || !L.IsNumber(-1) {
			L.PushInteger(0)
			return 1
		}
		val := L.ToInteger(-1)
		if val <= 0 {
			base.Error().Printf("Can't call randN with a value <= 0.")
			return 0
		}
		L.PushInteger(rand.Intn(val) + 1)
		return 1
	})
	a.L.DoString(a.Prog)
	return nil
}

func (a *Ai) loadUtils(dir string) {
	root := filepath.Join(filepath.Join(filepath.Dir(a.path), "utils", dir))
	a.watcher.Watch(root)
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.HasSuffix(info.Name(), ".lua") {
			f, err := os.Open(path)
			if err != nil {
				return nil
			}
			data, err := ioutil.ReadAll(f)
			f.Close()
			if err != nil {
				return nil
			}
			base.Log().Printf("Loaded lua utils file '%s'", path)
			a.L.DoString(string(data))
		}
		return nil
	})
}

// Need a goroutine for each ai - all things will go through is so that things
// stay synchronized
func (a *Ai) masterRoutine() {
	for {
		select {
		case <-a.terminate:
			if a.watcher != nil {
				a.watcher.Close()
			}
			close(a.active_query)
			return

		case a.active = <-a.active_set:
			if a.active == false {
				if a.ent == nil {
					base.Log().Printf("Evaluating = false")
				} else {
					base.Log().Printf("Ent %p inactivated", a.ent)
				}
				a.evaluating = false
			} else {
				if a.ent != nil {
					base.Log().Printf("Ent %s activated", a.ent.Name)
				}
			}

		case a.active_query <- a.active:

		case <-a.exec_query:
			if a.active {
				select {
				case a.pause <- struct{}{}:
				default:
				}
			}
			if a.active && !a.evaluating {
				a.evaluating = true
				go func() {
					if a.ent == nil {
						base.Log().Printf("Eval master")
					} else {
						base.Log().Printf("Eval ent: %p", a.ent)
					}
					base.Log().Printf("Evaluating lua script: %s", a.Prog)
					// Reset the execution limit in case it was set to 0 due to a
					// previous error
					a.L.SetExecutionLimit(2500000)

					// DoString will panic, and we can catch that, calling it manually
					// will exit() if it fails, which we cannot catch
					a.L.DoString("Think()")
					if a.ent == nil {
						base.Log().Printf("Completed master")
					} else {
						base.Log().Printf("Completed ent: %p", a.ent)
					}
					a.active_set <- false
					a.execs <- nil
					base.Log().Printf("Sent nil value")
				}()
			}
		}
	}
}

func (a *Ai) Terminate() {
	a.terminate <- struct{}{}
}

func (a *Ai) Activate() {
	if a.ent != nil {
		base.Log().Printf("Activated entity: %v", a.ent.Name)
	}
	reload := false
	for {
		select {
		case <-a.watcher.Event:
			reload = true
		default:
			goto no_more_events
		}
	}
no_more_events:
	if reload {
		a.setupLuaState()
		base.Log().Printf("Reloaded lua state for '%p'", a)
	}
	a.active_set <- true
}

func (a *Ai) Active() bool {
	return <-a.active_query
}

func (a *Ai) ActionExecs() <-chan game.ActionExec {
	select {
	case a.pause <- struct{}{}:
	default:
	}
	a.exec_query <- struct{}{}
	return a.execs
}

func (a *Ai) GobDecode([]byte) error {
	return nil
}
func (a *Ai) GobEncode() ([]byte, error) {
	return nil, nil
}

func rangedDistBetween(e1, e2 *game.Entity) int {
	e1x, e1y := e1.Pos()
	e2x, e2y := e2.Pos()
	dx := e1x - e2x
	dy := e1y - e2y
	if dx < 0 {
		dx = -dx
	}
	if dy < 0 {
		dy = -dy
	}
	if dx > dy {
		return dx
	}
	return dy
}
