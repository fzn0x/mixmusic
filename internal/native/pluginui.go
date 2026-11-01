package native

import (
	"sync"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/plugins"
)

// hosts are the plugin hosts of the apps, one each: plugins run as long as
// MixMusic does, whatever windows come and go.
var hosts struct {
	sync.Mutex
	m map[*app.App]*plugins.Host
}

// pluginHost is the host of a's plugins, which play through p.
func pluginHost(a *app.App, p *player.Player) *plugins.Host {
	hosts.Lock()
	defer hosts.Unlock()
	if hosts.m == nil {
		hosts.m = map[*app.App]*plugins.Host{}
	}
	h := hosts.m[a]
	if h == nil {
		h = plugins.New(a, p)
		hosts.m[a] = h
	}
	return h
}

// pluginUI is the main window, as the plugins show through it.
type pluginUI struct{ s *shell }

func (u pluginUI) Toast(message, tone string) {
	u.s.update(func() {
		n := notice(message)
		n.Error = tone == "error"
		u.s.notes = append(u.s.notes, n)
	})
}

func (u pluginUI) Navigate(path string) { u.s.update(func() { u.s.open(path) }) }

func (u pluginUI) Changed() {
	if u.s.win != nil {
		u.s.win.Invalidate()
	}
}

// usePlugins has the main window show the plugins of its app, until it
// closes.
func (s *shell) usePlugins() {
	s.plugins = pluginHost(s.a, s.p)
	s.plugins.SetUI(pluginUI{s})
	s.restyle()
	s.syncPlugins()
	s.stop = append(s.stop, func() {
		s.plugins.SetUI(nil)
		for v := range s.mountedViews {
			v.Unmount()
		}
		clear(s.mountedViews)
	})
}

// syncPlugins runs the plugins the settings enable.
func (s *shell) syncPlugins() { s.plugins.Sync(s.set.EnabledPlugins, s.inv.Plugins) }

// svgOf is the icon of an SVG a plugin gave, or a puzzle piece.
func (s *shell) svgOf(svg string) *ui.SVG {
	if svg == "" {
		return icon("puzzle")
	}
	if cached, ok := s.svgs[svg]; ok {
		return cached
	}
	parsed, err := ui.ParseSVG([]byte(svg))
	if err != nil {
		parsed = icon("puzzle")
	}
	s.svgs[svg] = parsed
	return parsed
}

// pluginTheme is the look, as plugins draw in it.
func pluginTheme(k *look) plugins.Theme {
	t := plugins.Theme{Accent: hexColor(k.accent), AccentText: hexColor(k.accentText), Text: hexColor(k.text),
		Muted: hexColor(k.muted), Background: hexColor(k.bg), Surface: hexColor(k.surface.Over(k.bg)), TileText: hexColor(k.tileText), Dark: k.dark}
	for i := range t.Tiles {
		t.Tiles[i] = hexColor(k.tile(i))
	}
	return t
}

// showView mounts a view of a plugin the window shows, which the end of
// the frame unmounts unless it shows still.
func (s *shell) showView(v *plugins.View) {
	if !s.mountedViews[v] {
		v.Mount()
		s.mountedViews[v] = true
	}
	s.seenViews[v] = true
}

// sweepViews unmounts the views of plugins the frame no longer showed.
func (s *shell) sweepViews() {
	for v := range s.mountedViews {
		if !s.seenViews[v] {
			v.Unmount()
			delete(s.mountedViews, v)
		}
	}
	clear(s.seenViews)
}

// pluginPage is a page a plugin adds, at path.
func (s *shell) pluginPage(c *ui.Context, path string) {
	page := s.plugins.Page(path)
	if page == nil {
		s.lost(c, "This page may belong to a plugin that is not enabled.")
		return
	}
	s.pageScroll(c, func() {
		s.pageHeader(c, page.Title, nil, nil)
		s.pluginView(c, "plugin-page", page.View)
	})
}

// pluginWidgets are the widgets plugins add to the now playing page.
func (s *shell) pluginWidgets(c *ui.Context) {
	for _, w := range s.plugins.Widgets() {
		ui.Column(c.Key("widget-" + w.ID)).Children(func() {
			s.sectionTitle(c, w.Title)
			s.pluginView(c, "widget-"+w.ID, w.View)
		})
	}
}

// pluginView shows a view of a plugin: the elements it set last, in the
// interface's own styles, which follow the theme.
func (s *shell) pluginView(c *ui.Context, key string, v *plugins.View) {
	s.showView(v)
	nodes := v.Nodes()
	ui.Column(c).Gap(16).AlignItems(ui.Start).Children(func() {
		for i, n := range nodes {
			s.pluginNode(c, key+"-"+count(i), v, n)
		}
	})
}

func (s *shell) pluginNode(c *ui.Context, key string, v *plugins.View, n plugins.Node) {
	k := &s.lk
	children := func(nodes []plugins.Node) {
		for i, child := range nodes {
			s.pluginNode(c, key+"-"+count(i), v, child)
		}
	}
	gap := func(fallback float32) float32 {
		if n.Gap > 0 {
			return n.Gap
		}
		return fallback
	}
	switch n.Kind {
	case "text":
		t := ui.Text(c, n.Text).FontSize(k.bodySize()).MaxWidth(720)
		switch n.Size {
		case "small":
			t.FontSize(k.smallSize())
		case "subtitle":
			t.FontSize(k.subtitleSize()).FontWeight(300)
		case "title":
			t.FontSize(k.titleSize()).FontWeight(300).Font(k.display)
		}
		switch n.Tone {
		case "muted":
			t.TextColor(k.muted)
		case "accent":
			t.TextColor(k.accent)
		case "danger":
			t.TextColor(k.danger)
		}
	case "heading":
		ui.Column(c).AlignSelf(ui.Stretch).Children(func() { s.sectionTitle(c, n.Text) })
	case "button":
		if s.button(c, key, n.Text, n.Primary).Clicked() && n.Handler != 0 {
			v.Click(n.Handler)
		}
	case "link":
		if s.link(c, key, n.Text) {
			if n.Path != "" {
				s.open(n.Path)
			} else if n.Handler != 0 {
				v.Click(n.Handler)
			}
		}
	case "tiles":
		ui.Row(c).Wrap().Gap(10).Children(func() { children(n.Children) })
	case "tile":
		s.pluginTile(c, key, v, n)
	case "row":
		ui.Row(c).Wrap().Gap(gap(8)).AlignItems(ui.Center).Children(func() { children(n.Children) })
	case "column":
		ui.Column(c).Gap(gap(12)).Children(func() { children(n.Children) })
	case "canvas":
		s.pluginCanvas(c, key, v, n)
	case "image":
		size := n.Width
		if size == 0 {
			size = 160
		}
		frame := ui.Box(c.Key(key)).Size(size, size).Shrink(0).Radius(k.radiusSm).Clip().Background(k.surface3)
		if n.Round {
			frame.Radius(size / 2)
		}
		if b := s.art.get(c, n.URL, artPixels(size)); b != nil {
			frame.Children(func() { ui.Image(c, b).Size(size, size).Fit(ui.Cover) })
		}
	case "spacer":
		ui.Box(c).Height(n.Height).Width(1)
	case "toggle":
		// The plugin holds what it shows: a toggle shows the state the
		// plugin set last, which its onChange sets again.
		on := n.On
		ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
			if s.switchOf(c.Key(key), &on).Label(n.Text).Changed() && n.Handler != 0 {
				v.Toggle(n.Handler, on)
			}
			ui.Text(c, n.Text).FontSize(k.bodySize())
		})
	case "input":
		draft, ok := s.pluginInputs[key]
		if !ok {
			draft = new(string)
			*draft = n.Value
			s.pluginInputs[key] = draft
		}
		in := ui.TextInput(c.Key(key), draft).Placeholder(n.Text).Width(320).Label(n.Text)
		if in.Submitted() && n.Handler != 0 {
			v.Submit(n.Handler, *draft)
		}
	}
}

// pluginTile is a tile of a plugin: a block of one of the theme's four
// colours, with a title, which does what the plugin says when clicked.
func (s *shell) pluginTile(c *ui.Context, key string, v *plugins.View, n plugins.Node) {
	k := &s.lk
	tone := 0
	if len(n.Tone) == 1 && n.Tone[0] >= '1' && n.Tone[0] <= '4' {
		tone = int(n.Tone[0] - '1')
	}
	t := s.buttonBase(c.Key(key)).Size(172, 120).Padding(16).Column().AlignItems(ui.Start).Justify(ui.SpaceBetween).
		Background(k.tile(tone)).Radius(k.radius).Label(n.Text).Cursor(ui.CursorPointer).Clip()
	if t.Pressed() {
		t.Scale(0.97, 0.97)
	}
	if t.Clicked() && n.Handler != 0 {
		v.Click(n.Handler)
	}
	t.Children(func() {
		if texture, ink := k.fx.texture, k.tileText; texture != "" {
			ui.Box(c).Absolute().Top(0).Right(0).Bottom(0).Left(0).PassThrough().
				Draw(func(p *ui.Painter, r ui.Rect) { paintTexture(p, r, texture, ink) })
		}
		ui.Text(c, n.Text).FontSize(k.size(1.5)).FontWeight(300).Font(k.display).TextColor(k.tileText).LineHeight(0.95)
		if n.Caption != "" {
			ui.Text(c, n.Caption).FontSize(k.smallSize()).TextColor(k.tileText).MaxLines(2)
		}
	})
}

// pluginCanvas is a canvas a plugin draws on: it paints the frame the
// plugin drew last, and asks for another at every frame while it
// animates, unless the computer asks for less motion, or else when its
// size changes.
func (s *shell) pluginCanvas(c *ui.Context, key string, v *plugins.View, n plugins.Node) {
	cv := v.Canvas(n.Canvas)
	box := ui.Box(c.Key(key)).Height(n.Height).FillWidth().Label("Drawing")
	if n.Width > 0 {
		box.MaxWidth(n.Width)
	}
	animate := n.Animate && !c.Preferences().ReduceMotion
	box.Draw(func(p *ui.Painter, r ui.Rect) {
		if cv == nil {
			return
		}
		size := [2]float32{r.W, r.H}
		if animate || s.canvasSizes[cv] != size {
			s.canvasSizes[cv] = size
			cv.Request(r.W, r.H)
		}
		if animate {
			p.AnimationFrame()
		}
		p.Clip(r, 0, func() {
			for _, op := range cv.Frame() {
				s.paintOp(p, r, op)
			}
		})
	})
}

// paintOp paints what a plugin's canvas drew, at r.
func (s *shell) paintOp(p *ui.Painter, r ui.Rect, op plugins.Op) {
	ink, ok := s.cssColors[op.Color]
	if !ok {
		parsed, valid := parseColor(op.Color)
		if !valid {
			parsed, _ = parseColor("#000")
		}
		ink = col(parsed)
		s.cssColors[op.Color] = ink
	}
	if op.Alpha < 1 {
		ink = ink.Alpha(op.Alpha)
	}
	var path ui.Path
	for _, sp := range op.Path {
		pts := sp.Points
		if len(pts) < 2 {
			continue
		}
		path.MoveTo(r.X+pts[0], r.Y+pts[1])
		for i := 2; i+1 < len(pts); i += 2 {
			path.LineTo(r.X+pts[i], r.Y+pts[i+1])
		}
		if sp.Closed {
			path.Close()
		}
	}
	if op.Fill {
		p.FillPath(&path, ink)
	} else {
		p.StrokePath(&path, op.Width, ink)
	}
}
