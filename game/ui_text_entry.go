package game

import (
	"github.com/MobRulesGames/glop/gin"
	"github.com/MobRulesGames/glop/gui"
	"github.com/MobRulesGames/haunts/base"
	"github.com/MobRulesGames/opengl/gl"
)

var valid_keys map[byte]bool
var shift_keys map[byte]byte

func init() {
	valid_keys = make(map[byte]bool)
	shift_keys = make(map[byte]byte)
	keys := []byte(" abcdefghijklmnopqrstuvwxyz`1234567890-=,./;[]\\'")
	shif := []byte(" ABCDEFGHIJKLMNOPQRSTUVWXYZ~!@#$%^&*()_+<>?:{}|\"")
	for i := range keys {
		valid_keys[keys[i]] = true
		shift_keys[keys[i]] = shif[i]
	}
}

type TextEntry struct {
	Button Button

	Entry struct {
		Default string

		// The X offset at which the text entry start, leave as 0 if it should
		// start immediately.
		X int

		// Width of the text entry.
		Dx int

		bounds struct {
			x, y, dx, dy int
		}

		// true iff the user is currently entering text.
		entering bool

		cursor, ghost struct {
			// x offset to draw cursor at, negative if it shouldn't be drawn
			offset int

			// if the cursor is active then this is the index into text at which new
			// text should be inserted
			index int
		}
		text string
		prev string
	}
}

func (te *TextEntry) Text() string {
	return te.Entry.text
}

func (te *TextEntry) SetText(text string) {
	te.Entry.text = text
	te.Entry.prev = text
	te.Entry.cursor.index = 0
	te.Entry.cursor.offset = -1
	te.Entry.ghost.index = 0
	te.Entry.ghost.offset = -1
}

func (te *TextEntry) handleClick(x, y int, data interface{}) bool {
	if te.Button.handleClick(x, y, data) {
		return true
	}
	old := te.Entry.entering
	te.Entry.entering = te.setCursor(x, y)
	if te.Entry.entering {
		te.Entry.cursor = te.Entry.ghost
		if !old {
			te.Entry.prev = te.Entry.text
		}
	} else {
		te.Entry.prev = ""
	}
	return te.Entry.entering
}

func (te *TextEntry) HasFocus() bool {
	return te.Entry.entering
}

func (te *TextEntry) DropFocus() {
	te.Entry.entering = false
	te.Entry.ghost.offset = -1
}

func (te *TextEntry) Over(mx, my int) bool {
	return pointInsideRect(
		mx,
		my,
		te.Button.bounds.x,
		te.Button.bounds.y,
		te.Button.bounds.dx+te.Entry.Dx,
		te.Button.bounds.dy)
}

// Returns true iff the position specified is a valid position to click in the
// text area.  Also sets everything up so that if te.Entry.entering can be set
// to true to begin editing at that position.
func (te *TextEntry) setCursor(mx, my int) bool {
	if !pointInsideRect(mx, my, te.Entry.bounds.x, te.Entry.bounds.y, te.Entry.bounds.dx, te.Entry.bounds.dy) {
		te.Entry.ghost.offset = -1
		return false
	}

	d := base.GetDictionary(te.Button.Text.Size)
	last_dx := 0
	te.Entry.ghost.index = -1
	for i := range te.Entry.text {
		w := int(d.StringWidth(te.Entry.text[0 : i+1]))
		avg := (last_dx + w) / 2
		if pointInsideRect(mx, my, te.Entry.bounds.x, te.Entry.bounds.y, avg, te.Entry.bounds.dy) {
			te.Entry.ghost.offset = last_dx
			te.Entry.ghost.index = i
			break
		}
		last_dx = w
	}
	if te.Entry.ghost.index < 0 {
		te.Entry.ghost.offset = int(d.StringWidth(te.Entry.text))
		te.Entry.ghost.index = len(te.Entry.text)
	}
	return true
}

func (te *TextEntry) Respond(group gui.EventGroup, data interface{}) bool {
	if te.Button.Respond(group, data) {
		return true
	}
	if !te.Entry.entering {
		return false
	}
	for _, event := range group.Events {
		if event.Type == gin.Press {
			id := event.Key.Id()
			if id <= 255 && valid_keys[byte(id)] {
				b := byte(id)
				if gin.In().GetKey(gin.EitherShift).CurPressAmt() > 0 {
					b = shift_keys[b]
				}
				t := te.Entry.text
				index := te.Entry.cursor.index
				t = t[0:index] + string([]byte{b}) + t[index:]
				te.Entry.text = t
				te.Entry.cursor.index++
			} else if event.Key.Id() == gin.DeleteOrBackspace {
				if te.Entry.cursor.index > 0 {
					index := te.Entry.cursor.index
					t := te.Entry.text
					te.Entry.text = t[0:index-1] + t[index:]
					te.Entry.cursor.index--
				}
			} else if event.Key.Id() == gin.Left {
				if te.Entry.cursor.index > 0 {
					te.Entry.cursor.index--
				}
			} else if event.Key.Id() == gin.Right {
				if te.Entry.cursor.index < len(te.Entry.text) {
					te.Entry.cursor.index++
				}
			} else if event.Key.Id() == gin.Return {
				te.Entry.entering = false
				if te.Button.f != nil {
					te.Button.f(nil)
				}
			} else if event.Key.Id() == gin.Escape {
				te.Entry.entering = false
				te.Entry.text = te.Entry.prev
				te.Entry.prev = ""
				te.Entry.cursor.index = 0
			}
			d := base.GetDictionary(te.Button.Text.Size)
			te.Entry.cursor.offset = int(d.StringWidth(te.Entry.text[0:te.Entry.cursor.index]))
		}
	}
	return false
}

func (te *TextEntry) Think(x, y, mx, my int, dt int64) {
	if te.Entry.Default != "" {
		te.Entry.text = te.Entry.Default
		te.Entry.Default = ""
	}
	te.Button.Think(x, y, mx, my, dt)
	te.setCursor(gin.In().GetCursor("Mouse").Point())
}

func (te *TextEntry) RenderAt(x, y int) {
	te.Button.RenderAt(x, y)
	d := base.GetDictionary(te.Button.Text.Size)
	x += te.Entry.X
	y += te.Button.Y
	x2 := x + te.Entry.Dx
	y2 := y + int(d.MaxHeight())
	te.Entry.bounds.x = x
	te.Entry.bounds.y = y
	te.Entry.bounds.dx = x2 - x
	te.Entry.bounds.dy = y2 - y
	gl.Disable(gl.TEXTURE_2D)
	if te.Entry.entering {
		gl.Color4ub(255, 255, 255, 255)
	} else {
		gl.Color4ub(255, 255, 255, 128)
	}
	gl.Begin(gl.QUADS)
	gl.Vertex2i(x-3, y-3)
	gl.Vertex2i(x-3, y2+3)
	gl.Vertex2i(x2+3, y2+3)
	gl.Vertex2i(x2+3, y-3)
	gl.End()
	gl.Color4ub(0, 0, 0, 255)
	gl.Begin(gl.QUADS)
	gl.Vertex2i(x, y)
	gl.Vertex2i(x, y2)
	gl.Vertex2i(x2, y2)
	gl.Vertex2i(x2, y)
	gl.End()

	gl.Color4ub(255, 255, 255, 255)
	d.RenderString(te.Entry.text, float64(x), float64(y), 0, d.MaxHeight(), gui.Left)

	if te.Entry.ghost.offset >= 0 {
		gl.Disable(gl.TEXTURE_2D)
		gl.Color4ub(255, 100, 100, 127)
		gl.Begin(gl.LINES)
		gl.Vertex2i(te.Entry.bounds.x+te.Entry.ghost.offset, te.Entry.bounds.y)
		gl.Vertex2i(te.Entry.bounds.x+te.Entry.ghost.offset, te.Entry.bounds.y+te.Entry.bounds.dy)
		gl.End()
	}
	if te.Entry.entering {
		gl.Disable(gl.TEXTURE_2D)
		gl.Color4ub(255, 100, 100, 255)
		gl.Begin(gl.LINES)
		gl.Vertex2i(te.Entry.bounds.x+te.Entry.cursor.offset, te.Entry.bounds.y)
		gl.Vertex2i(te.Entry.bounds.x+te.Entry.cursor.offset, te.Entry.bounds.y+te.Entry.bounds.dy)
		gl.End()
	}
}
