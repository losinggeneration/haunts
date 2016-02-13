package game

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/MobRulesGames/glop/gin"
	"github.com/MobRulesGames/glop/gui"
	"github.com/MobRulesGames/haunts/base"
	"github.com/MobRulesGames/haunts/texture"
	"github.com/MobRulesGames/opengl/gl"
)

type Paragraph struct {
	X, Y, Dx, Size int
	Halign         string
	Valign         string
}

type dialogSection struct {
	// Center of the image
	X, Y      int
	Paragraph Paragraph

	// The clickable region
	Region struct {
		X, Y, Dx, Dy int
	}
}

type dialogLayoutSpec struct {
	Sections []dialogSection
}

type dialogLayout struct {
	Background texture.Object
	Next, Prev Button

	Formats map[string]dialogLayoutSpec
}

type dialogData struct {
	Size  string
	Pages map[string]struct {
		Format   string
		Next     string // Just to error check - this should always be empty
		Sections []struct {
			Id      string
			Next    string
			Image   texture.Object
			Text    string
			shading float64
		}
	}

	prev     []string
	cur_page string
}

func (d *dialogData) process(m map[string]string) {
	for key, value := range m {
		for name := range d.Pages {
			page := d.Pages[name]
			for i := range page.Sections {
				section := &page.Sections[i]
				old := fmt.Sprintf("{%s}", key)
				section.Text = strings.Replace(section.Text, old, value, -1)
			}
			d.Pages[name] = page
		}
	}
}

type MediumDialogBox struct {
	layout dialogLayout
	format dialogLayoutSpec
	// state  mediumDialogState
	data dialogData

	region gui.Region

	buttons []*Button

	// Position of the mouse
	mx, my int

	last_t int64

	done   bool
	result chan string
}

func MakeDialogBox(source string, args map[string]string) (*MediumDialogBox, <-chan string, error) {
	var mdb MediumDialogBox
	datadir := base.GetDataDir()
	err := base.LoadAndProcessObject(filepath.Join(datadir, source), "json", &mdb.data)
	if err != nil {
		return nil, nil, err
	}
	err = base.LoadAndProcessObject(filepath.Join(datadir, "ui", "dialog", fmt.Sprintf("%s.json", mdb.data.Size)), "json", &mdb.layout)
	if err != nil {
		return nil, nil, err
	}

	if _, ok := mdb.data.Pages["Start"]; !ok {
		return nil, nil, errors.New("Section 'Start' was not specified.")
	}

	// Make sure that all of the pages specified by "Next"s are available
	// in the map
	for _, page := range mdb.data.Pages {
		if page.Next != "" {
			return nil, nil, errors.New(fmt.Sprintf("Specified 'Next': '%s' on a page, but must be in a section on the page.", page.Next))
		}
		for _, section := range page.Sections {
			if section.Next == "" {
				continue
			}
			if _, ok := mdb.data.Pages[section.Next]; !ok {
				return nil, nil, errors.New(fmt.Sprintf("Section '%s' specified but doesn't exist.", section.Next))
			}
		}
		if _, ok := mdb.layout.Formats[page.Format]; !ok {
			return nil, nil, errors.New(fmt.Sprintf("Unknown dialog box format '%s'.", page.Format))
		}
		if len(page.Sections) != len(mdb.layout.Formats[page.Format].Sections) {
			return nil, nil, errors.New(fmt.Sprintf("Format '%s' requires exactly %d sections.", page.Format, len(mdb.layout.Formats[page.Format].Sections)))
		}
	}

	mdb.data.cur_page = "Start"
	mdb.format = mdb.layout.Formats[mdb.data.Pages[mdb.data.cur_page].Format]

	// return nil, nil, errors.New(fmt.Sprintf("Unknown format string: '%s'.", format))

	mdb.buttons = []*Button{
		&mdb.layout.Next,
		&mdb.layout.Prev,
	}

	mdb.result = make(chan string, 1)

	mdb.layout.Next.valid_func = func() bool {
		sections := mdb.data.Pages[mdb.data.cur_page].Sections
		return len(sections) == 1
	}

	mdb.layout.Next.f = func(_data interface{}) {
		sections := mdb.data.Pages[mdb.data.cur_page].Sections
		if len(sections) == 1 {
			if sections[0].Next == "" {
				if !mdb.done {
					close(mdb.result)
					mdb.done = true
				}
			} else {
				mdb.data.prev = append(mdb.data.prev, mdb.data.cur_page)
				mdb.data.cur_page = sections[0].Next
				mdb.format = mdb.layout.Formats[mdb.data.Pages[mdb.data.cur_page].Format]
			}
		}
	}

	mdb.layout.Prev.valid_func = func() bool {
		return len(mdb.data.prev) > 0
	}

	mdb.layout.Prev.f = func(_data interface{}) {
		if len(mdb.data.prev) > 0 {
			mdb.data.cur_page = mdb.data.prev[len(mdb.data.prev)-1]
			mdb.data.prev = mdb.data.prev[0 : len(mdb.data.prev)-1]
			mdb.format = mdb.layout.Formats[mdb.data.Pages[mdb.data.cur_page].Format]
		}
	}

	mdb.data.process(args)

	return &mdb, mdb.result, nil
}

func (mdb *MediumDialogBox) Requested() gui.Dims {
	return gui.Dims{
		Dx: mdb.layout.Background.Data().Dx(),
		Dy: mdb.layout.Background.Data().Dy(),
	}
}

func (mdb *MediumDialogBox) Expandable() (bool, bool) {
	return false, false
}

func (mdb *MediumDialogBox) Rendered() gui.Region {
	return mdb.region
}

func (mdb *MediumDialogBox) Think(g *gui.Gui, t int64) {
	if mdb.done {
		return
	}
	if mdb.last_t == 0 {
		mdb.last_t = t
		return
	}
	dt := t - mdb.last_t
	mdb.last_t = t
	for _, button := range mdb.buttons {
		button.Think(mdb.region.X, mdb.region.Y, mdb.mx, mdb.my, dt)
	}
	for i := range mdb.format.Sections {
		section := mdb.format.Sections[i]
		data := &mdb.data.Pages[mdb.data.cur_page].Sections[i]
		if section.Region.Dx*section.Region.Dy <= 0 {
			data.shading = 1.0
		}
		in := pointInsideRect(mdb.mx, mdb.my, mdb.region.X+section.Region.X, mdb.region.Y+section.Region.Y, section.Region.Dx, section.Region.Dy)
		data.shading = doShading(data.shading, in, dt)
	}
	if len(mdb.format.Sections) == 1 {
		mdb.data.Pages[mdb.data.cur_page].Sections[0].shading = 1.0
	}
}

func (mdb *MediumDialogBox) Respond(g *gui.Gui, group gui.EventGroup) bool {
	cursor := group.Events[0].Key.Cursor()
	if cursor != nil {
		mdb.mx, mdb.my = cursor.Point()
		if !pointInsideRect(mdb.mx, mdb.my, mdb.region.X, mdb.region.Y, mdb.layout.Background.Data().Dx(), mdb.layout.Background.Data().Dy()) {
			return false
		}
	}

	for _, button := range mdb.buttons {
		if button.Respond(group, mdb) {
			return true
		}
	}

	if found, event := group.FindEvent(gin.MouseLButton); found && event.Type == gin.Press {
		for _, button := range mdb.buttons {
			if button.handleClick(mdb.mx, mdb.my, mdb) {
				return true
			}
		}
		for i, section := range mdb.format.Sections {
			if pointInsideRect(
				mdb.mx,
				mdb.my,
				mdb.region.X+section.Region.X,
				mdb.region.Y+section.Region.Y,
				section.Region.Dx,
				section.Region.Dy) {
				if !mdb.done {
					mdb.data.prev = mdb.data.prev[0:0]
					mdb.result <- mdb.data.Pages[mdb.data.cur_page].Sections[i].Id
					mdb.data.cur_page = mdb.data.Pages[mdb.data.cur_page].Sections[i].Next
					if mdb.data.cur_page == "" {
						close(mdb.result)
						mdb.done = true
					} else {
						mdb.format = mdb.layout.Formats[mdb.data.Pages[mdb.data.cur_page].Format]
					}
					break
				}
			}
		}
	}

	return cursor != nil
}

func (mdb *MediumDialogBox) Draw(region gui.Region) {
	mdb.region = region
	if mdb.done {
		return
	}
	gl.Enable(gl.TEXTURE_2D)
	gl.Color4ub(255, 255, 255, 255)
	mdb.layout.Background.Data().RenderNatural(region.X, region.Y)
	for _, button := range mdb.buttons {
		button.RenderAt(region.X, region.Y)
	}

	for i := range mdb.format.Sections {
		section := mdb.format.Sections[i]
		data := mdb.data.Pages[mdb.data.cur_page].Sections[i]
		p := section.Paragraph
		d := base.GetDictionary(p.Size)
		var just gui.Justification
		switch p.Halign {
		case "left":
			just = gui.Left
		case "right":
			just = gui.Right
		case "center":
			just = gui.Center
		default:
			base.Error().Printf("Unknown justification '%s'", p.Halign)
			p.Halign = "left"
		}
		var valign gui.Justification
		switch p.Valign {
		case "top":
			valign = gui.Top
		case "bottom":
			valign = gui.Bottom
		case "center":
			valign = gui.Center
		default:
			base.Error().Printf("Unknown justification '%s'", p.Valign)
			p.Valign = "top"
		}
		gl.Color4ub(255, 255, 255, 255)

		d.RenderParagraph(data.Text, float64(p.X+region.X), float64(p.Y+region.Y)-d.MaxHeight()/2, 0, float64(p.Dx), d.MaxHeight(), just, valign)

		gl.Color4ub(255, 255, 255, byte(data.shading*255))
		tex := data.Image.Data()
		tex.RenderNatural(region.X+section.X-tex.Dx()/2, region.Y+section.Y-tex.Dy()/2)
	}
}

func (mdb *MediumDialogBox) DrawFocused(region gui.Region) {}

func (mdb *MediumDialogBox) String() string {
	return "medium dialog box"
}
