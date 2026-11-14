package circadian

import (
	"math"
	"testing"
	"time"
)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

func TestValueAtStraightAndWrapping(t *testing.T) {
	l := Lane{On: true, Points: []Point{{360, 100, 0}, {1080, 50, 0}}}
	cases := map[float64]float64{
		360:  100,
		720:  75, // half way from 06:00 to 18:00
		1080: 50,
		0:    75, // half way from 18:00 to 06:00 the next day
		1440: 75, // 24:00 is 00:00
	}
	for minute, want := range cases {
		got, ok := l.ValueAt(minute)
		if !ok || !near(got, want) {
			t.Errorf("ValueAt(%v) = %v, want %v", minute, got, want)
		}
	}
	if _, ok := (Lane{}).ValueAt(0); ok {
		t.Error("a clip without points has no value")
	}
	if v, _ := (Lane{Points: []Point{{600, 42, 0.7}}}).ValueAt(3); v != 42 {
		t.Errorf("one point is a flat clip, got %v", v)
	}
}

func TestTensionBulgesUp(t *testing.T) {
	for _, rising := range []bool{true, false} {
		from, to := 20.0, 80.0
		if !rising {
			from, to = 80, 20
		}
		straight := Lane{Points: []Point{{0, from, 0}, {600, to, 0}}}
		up := Lane{Points: []Point{{0, from, 0.8}, {600, to, 0}}}
		down := Lane{Points: []Point{{0, from, -0.8}, {600, to, 0}}}
		s, _ := straight.ValueAt(300)
		u, _ := up.ValueAt(300)
		d, _ := down.ValueAt(300)
		if !(u > s && s > d) {
			t.Errorf("rising=%v: up %v, straight %v, down %v", rising, u, s, d)
		}
		// The curve's middle is where the handle sits: m of the way.
		m := 0.5 + 0.49*0.8
		want := from + (to-from)*m
		if !rising {
			want = from + (to-from)*(1-m)
		}
		if !near(u, want) {
			t.Errorf("rising=%v: middle %v, want %v", rising, u, want)
		}
		// Ends stay put whatever the tension.
		if a, _ := up.ValueAt(0); !near(a, from) {
			t.Errorf("start moved to %v", a)
		}
		if b, _ := up.ValueAt(599.9999); math.Abs(b-to) > 0.01 {
			t.Errorf("end moved to %v", b)
		}
	}
}

func TestNextPointAfter(t *testing.T) {
	loc := time.FixedZone("x", 3600)
	l := Lane{Points: []Point{{360, 1, 0}, {1080, 2, 0}}}
	now := time.Date(2026, 11, 14, 12, 0, 0, 0, loc)
	if got, _ := l.NextPointAfter(now); !got.Equal(time.Date(2026, 11, 14, 18, 0, 0, 0, loc)) {
		t.Errorf("next after noon: %v", got)
	}
	late := time.Date(2026, 11, 14, 20, 0, 0, 0, loc)
	if got, _ := l.NextPointAfter(late); !got.Equal(time.Date(2026, 11, 15, 6, 0, 0, 0, loc)) {
		t.Errorf("next after 20:00: %v", got)
	}
	at := time.Date(2026, 11, 14, 18, 0, 0, 0, loc)
	if got, _ := l.NextPointAfter(at); !got.Equal(time.Date(2026, 11, 15, 6, 0, 0, 0, loc)) {
		t.Errorf("next after 18:00 sharp: %v", got)
	}
}

func TestParseAndFormatPoints(t *testing.T) {
	info, _ := FindLane(LaneKelvin)
	points, err := ParsePoints(info, "21:00=3400~-0.5, 06:30=6500K 23:00=2300")
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 3 || points[0].Minute != 390 || points[1].Tension != -0.5 {
		t.Fatalf("parsed %+v", points)
	}
	if got := FormatPoints(points); got != "06:30=6500 21:00=3400~-0.5 23:00=2300" {
		t.Errorf("formatted %q", got)
	}
	for _, bad := range []string{"25:00=3000", "10:00=500", "10:00", "10:00=3000~2", "10:00=abc"} {
		if _, err := ParsePoints(info, bad); err == nil {
			t.Errorf("%q was accepted", bad)
		}
	}
}

func TestWallpaperAt(t *testing.T) {
	c := Config{Wallpapers: []Wallpaper{
		{Image: "day", From: 360, To: 1200},
		{Image: "night", From: 1200, To: 360},
		{Image: "lunch", From: 720, To: 780},
	}}
	for minute, want := range map[float64]string{100: "night", 400: "day", 730: "lunch", 800: "day", 1300: "night"} {
		if i := c.WallpaperAt(minute); i < 0 || c.Wallpapers[i].Image != want {
			t.Errorf("at %v: %d", minute, i)
		}
	}
	if (Config{}).WallpaperAt(5) != -1 {
		t.Error("no wallpapers, but one was chosen")
	}
}

func TestDarkAt(t *testing.T) {
	c := Config{Dark: Dark{Mode: DarkScheduled, LightFrom: 420, LightTo: 1200}}
	if dark, ok := c.DarkAt(600); dark || !ok {
		t.Error("10:00 should be light")
	}
	if dark, _ := c.DarkAt(1300); !dark {
		t.Error("21:40 should be dark")
	}
	c.Dark.Mode = DarkSystem
	if _, ok := c.DarkAt(600); ok {
		t.Error("the system mode decides nothing")
	}
}

func TestWhitePointAndRamp(t *testing.T) {
	r, g, b := WhitePoint(6500)
	if !near(r, 1) || !near(g, 1) || !near(b, 1) {
		t.Errorf("6500 K is %v %v %v", r, g, b)
	}
	r, g, b = WhitePoint(2300)
	if !(r == 1 && g < 0.8 && b < g) {
		t.Errorf("2300 K is %v %v %v", r, g, b)
	}
	r, g, b = WhitePoint(9000)
	if !(b == 1 && r < 1) {
		t.Errorf("9000 K is %v %v %v", r, g, b)
	}
	red, green, blue := Ramp(256, 6500, 1)
	if red[255] != 65535 || red[0] != 0 || green[128] != blue[128] {
		t.Errorf("neutral ramp: %d %d %d", red[255], green[128], blue[128])
	}
	red, _, _ = Ramp(256, 6500, 0.5)
	if red[255] < 32700 || red[255] > 32800 {
		t.Errorf("half brightness tops out at %d", red[255])
	}
}

func TestPresetsAndDefault(t *testing.T) {
	for _, p := range Presets {
		for _, info := range Lanes {
			points := p.Lanes[info.Name]
			if len(points) == 0 {
				t.Errorf("%s has no %s points", p.Name, info.Name)
			}
			for _, pt := range points {
				if pt.Value < info.Min || pt.Value > info.Max || pt.Minute < 0 || pt.Minute >= DayMinutes {
					t.Errorf("%s %s: %+v out of range", p.Name, info.Name, pt)
				}
			}
		}
	}
	c := Default()
	if c.On || c.Dark.Mode != DarkSystem || len(c.Lanes) != len(Lanes) {
		t.Errorf("default: %+v", c)
	}
	for name, l := range c.Lanes {
		for i := 1; i < len(l.Points); i++ {
			if l.Points[i].Minute <= l.Points[i-1].Minute {
				t.Errorf("%s points not sorted", name)
			}
		}
	}
}

func TestThemes(t *testing.T) {
	installed := map[string]bool{"Mint-Y-Aqua": true, "Mint-Y-Dark-Aqua": true, "Adwaita": true, "Adwaita-dark": true, "Mint-Y": true, "Mint-Y-Dark": true}
	exists := func(n string) bool { return installed[n] }
	for light, dark := range map[string]string{"Mint-Y-Aqua": "Mint-Y-Dark-Aqua", "Adwaita": "Adwaita-dark", "Mint-Y": "Mint-Y-Dark", "Plain": ""} {
		if got := DarkTheme(light, exists); got != dark {
			t.Errorf("DarkTheme(%s) = %q, want %q", light, got, dark)
		}
		if dark != "" {
			if got := LightTheme(dark, exists); got != light {
				t.Errorf("LightTheme(%s) = %q, want %q", dark, got, light)
			}
		}
	}
	if DarkTheme("Mint-Y-Dark", exists) != "Mint-Y-Dark" || LightTheme("Mint-Y", exists) != "Mint-Y" {
		t.Error("a theme already of the wanted kind should stay")
	}
}

func TestClock(t *testing.T) {
	if m, err := ParseClock("24:00"); err != nil || m != 1440 {
		t.Errorf("24:00 -> %d %v", m, err)
	}
	if _, err := ParseClock("24:30"); err == nil {
		t.Error("24:30 accepted")
	}
	if Clock(1440) != "00:00" || Clock(75) != "01:15" {
		t.Error("Clock")
	}
}
