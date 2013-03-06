// Stubbed version of the sound manager - lets us test things without having
// to link in fmod.

// +build nosound

package sound

func Init() {}
func PlayMusic(name string) {}
func PlaySound(name string, volume float64) {}
func SetMusicParam(name string, val float64) {}
func StopMusic() {}

