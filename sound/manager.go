//

// +build !nosound
package sound

import (
	fmod "github.com/runningwild/fmod/event"
	"github.com/runningwild/haunts/base"
	"math"
	"path/filepath"
	"time"
)

var (
	system        *fmod.System
	music_volume  chan float64
	music         *fmod.Event
	music_start   chan string
	music_name    string
	music_stop    chan bool
	param_control chan paramRequest

	// used to define how fast things fade in and out
	freq     time.Duration
	approach float64
)

func Init() {
	var err error
	system, err = fmod.EventSystemCreate()
	if err != nil {
		base.Error().Printf("Unable to create sound system: %v", err)
		return
	}

	err = system.Init(32, fmod.INIT_NORMAL, nil, fmod.EVENT_INIT_NORMAL)
	if err != nil {
		base.Error().Printf("Unable to initialize sound system: %v", err)
		return
	}
	version, _ := system.GetVersion()
	base.Log().Printf("Fmod version %x", version)

	err = system.SetMediaPath(filepath.Join(base.GetDataDir(), "sound") + "/")
	if err != nil {
		base.Error().Printf("Unable to set media path: %v\n", err)
		return
	}

	err = system.LoadPath("Haunts.fev", nil)
	if err != nil {
		base.Error().Printf("Unable to load fev: %v\n", err)
		return
	}

	freq = time.Millisecond * 3
	approach = 0.01
	music_volume = make(chan float64, 1)
	music_start = make(chan string, 1)
	param_control = make(chan paramRequest, 1)
	music_stop = make(chan bool, 1)
	go musicControl()
}

type musicState struct {
	name  string
	event *fmod.Event
	stop  bool
	vol   struct {
		cur, target float64
	}
	params map[string]paramState
}

type paramState struct {
	param       *fmod.Param
	cur, target float64
}

type paramRequest struct {
	name string
	val  float64
}

// Manages anything that might happen with one music event.
func musicControl() {
	musics := make(map[string]*musicState)
	var current *musicState
	tick := time.NewTicker(freq)
	for {
		select {
		case <-tick.C:
			for _, music := range musics {
				music.vol.cur = music.vol.target*approach + music.vol.cur*(1-approach)
				if math.Abs(music.vol.cur-music.vol.target) > 1e-5 {
					music.event.SetVolume(music.vol.cur)
				}
				for name, param := range music.params {
					param.cur = param.target*approach + param.cur*(1-approach)
					music.params[name] = param
					if math.Abs(param.cur-param.target) > 1e-5 {
						param.param.SetValue(param.cur)
					}
				}
			}

		case name := <-music_start:
			if current != nil && name == current.name {
				current.vol.target = 1.0
			} else {
				if current != nil {
					current.vol.target = 0.0
					current.stop = true
				}
				event, err := system.GetEvent(name, fmod.MODE_DEFAULT)
				if err != nil {
					base.Error().Printf("Unable to play music '%s': %v", name, err)
					continue
				}
				err = event.Start()
				var next musicState
				next.name = name
				next.event = event
				next.vol.cur, err = event.GetVolume()
				next.vol.target = 1.0
				next.params = make(map[string]paramState)
				musics[name] = &next
				current = &next
			}

		case target := <-music_volume:
			if current != nil {
				current.vol.target = target
			}

		case <-music_stop:
			if current != nil {
				current.vol.target = 0.0
				current.stop = true
			}

		case req := <-param_control:
			if current != nil {
				if _, ok := current.params[req.name]; !ok {
					param, err := current.event.GetParameter(req.name)
					if err != nil {
						base.Error().Printf("Can't get parameter '%s': %v", req.name, err)
						break
					}
					current.params[req.name] = paramState{param: param}
				}
				state := current.params[req.name]
				state.cur, _ = state.param.GetValue()
				state.target = req.val
				current.params[req.name] = state
			}
		}
	}
}

func PlayMusic(name string) {
	if system == nil {
		return
	}
	music_start <- name
}

func StopMusic() {
	music_stop <- true
}

func SetMusicParam(name string, val float64) {
	if system == nil {
		return
	}
	param_control <- paramRequest{name: name, val: val}
}

func PlaySound(name string, volume float64) {
	if system == nil {
		return
	}
	sound, err := system.GetEvent(name, fmod.MODE_DEFAULT)
	if err != nil {
		base.Error().Printf("Unable to get event '%s': %v", name, err)
		return
	}
	sound.SetVolume(volume)
	sound.Start()
}
