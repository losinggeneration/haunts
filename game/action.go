package game

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"github.com/MobRulesGames/glop/gui"
	"github.com/MobRulesGames/haunts/base"
	"github.com/MobRulesGames/haunts/house"
	"github.com/MobRulesGames/haunts/texture"
	lua "github.com/MobRulesGames/golua"
)

var action_map map[string]func() Action

func MakeAction(name string) Action {
	f, ok := action_map[name]
	if !ok {
		base.Error().Printf("Unable to find an Action named '%s'", name)
	}
	return f()
}

var action_makers []func() map[string]func() Action

// Ahahahahahaha
func RegisterActionMakers(f func() map[string]func() Action) {
	action_makers = append(action_makers, f)
}

func RegisterActions() {
	action_map = make(map[string]func() Action)
	for _, maker := range action_makers {
		m := maker()
		for name, f := range m {
			if _, ok := action_map[name]; ok {
				panic(fmt.Sprintf("Tried to register more than one action by the same name: '%s'", name))
			}
			action_map[name] = f
		}
	}
}

// Acceptable values to be returned from Action.Maintain()
type MaintenanceStatus int

const (
	// The Action is in progress and should not be interrupted.
	InProgress MaintenanceStatus = iota

	// The Action is interrupted and can be interrupted immediately.
	CheckForInterrupts

	// The Action has been completed.
	Complete
)

// All implementations of ActionExec will probably use exactly this setup,
// so we just provide it here so we don't duplicate a ton of code everywhere.
type BasicActionExec struct {
	Ent   EntityId
	Index int
}

func (bae BasicActionExec) EntityId() EntityId {
	return bae.Ent
}
func (bae BasicActionExec) ActionIndex() int {
	return bae.Index
}
func (bae BasicActionExec) Push(L *lua.State, g *Game) {
	ent := g.EntityById(bae.Ent)
	if bae.Index < 0 || bae.Index >= len(ent.Actions) {
		base.Error().Printf("Tried to push an exec for an invalid action index: '%s' %d.", ent.Name)
		L.PushNil()
		return
	}
	L.NewTable()
	L.PushString("Action")
	ent.Actions[bae.Index].Push(L)
	L.SetTable(-3)
	L.PushString("Ent")
	LuaPushEntity(L, ent)
	L.SetTable(-3)
}
func (bae BasicActionExec) GetPath() []int {
	return nil
}
func (bae BasicActionExec) TruncatePath(int) {}
func (bae *BasicActionExec) SetBasicData(ent *Entity, action Action) {
	bae.Ent = ent.Id
	bae.Index = -1
	for i := range ent.Actions {
		if ent.Actions[i] == action {
			bae.Index = i
		}
	}
	if bae.Index == -1 {
		base.Error().Printf("Action '%v' was unable to find itself in Entity %v's Actions: %v", action, ent, ent.Actions)
	}
}

// When an entity commits to an action it will create an ActionExec.  This
// will be passed to the Action 
type ActionExec interface {
	// Entity whose action created this ActionExec
	EntityId() EntityId

	// Index into Entity.Actions
	ActionIndex() int

	Push(L *lua.State, g *Game)

	GetPath() []int
	TruncatePath(length int)
}

func encodeActionExec(ae ActionExec) []byte {
	b := bytes.NewBuffer(nil)
	enc := gob.NewEncoder(b)
	err := enc.Encode(ae)
	if err != nil {
		base.Error().Printf("Failed to gob an ActionExec: %v", err)
		return nil
	}
	return b.Bytes()
}

func decodeActionExec(b []byte) ActionExec {
	var ae ActionExec
	dec := gob.NewDecoder(bytes.NewReader(b))
	err := dec.Decode(&ae)
	if err != nil {
		base.Error().Printf("Failed to ungob an ActionExec: %v", err)
		return nil
	}
	return ae
}

type Action interface {
	// The amount of Ap that this action costs if performed in its current
	// state.  This method should only be called on Actions that have already
	// been prepped.
	AP() int

	// The name that will be displayed to the user to represent this Action.
	String() string

	// Returns a texture that can be used to identify this Action.
	Icon() *texture.Object

	// Returns true iff this action can be used as an interrupt.
	Readyable() bool

	// If Preppable returns true then Prep will return true.  Unlike Prep,
	// however, it does not actually change the state of the Action.
	Preppable(e *Entity, g *Game) bool

	// Called when the user attempts to select the action.  Returns true if the
	// actions can be performed at least minimally, false if the action cannot
	// be performed at all.
	Prep(e *Entity, g *Game) bool

	// The boolean return value indicates whether or not the input was consumed
	// If this function returns false then the ActionExec returned will be nil,
	// otherwise, if it is not nil, it indicates that the entity has committed
	// to this action.  That ActionExec should be passed to the Action on the
	// next call to Maintain().
	HandleInput(gui.EventGroup, *Game) (bool, ActionExec)

	// Got to have some way for the user to see what is going on
	house.FloorDrawer

	// Called if the user cancels the action - done this way so that all actions
	// can be cancelled in the same way instead of each action deciding how to
	// cancel itself.  This method is only called before Maintain() is called
	// the first time, or after Maintain() returns CheckForInterrupts.
	Cancel()

	// Actually executes the action.  Returns a value after every call
	// indicating whether the action is done, still in progress, or can be
	// interrupted.
	Maintain(dt int64, g *Game, exec ActionExec) MaintenanceStatus

	// This will be called if the action has been readied at this is a logical
	// point for an interrupt to happen.  Should return true if the action
	// should take place.
	Interrupt() bool

	// Pushes a table containing information about the action onto the stack.
	Push(L *lua.State)

	// Returns a mapping from trigger to the sound that should play for that
	// trigger.
	SoundMap() map[string]string
}
