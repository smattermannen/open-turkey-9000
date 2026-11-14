package circadian

import "math"

// Preset is a ready-made set of clips to start from.
type Preset struct {
	Name   string
	Title  string
	Detail string
	Lanes  map[string][]Point
}

// Presets lists them in display order. Times are minutes of the day.
var Presets = []Preset{
	{"natural", "Natural day",
		"Bright and cool from mid-morning, warming from early evening, dim and warm at night.",
		map[string][]Point{
			LaneBacklight: {{390, 50, 0.5}, {510, 100, 0}, {1110, 100, -0.3}, {1290, 60, 0}, {1410, 35, 0}},
			LaneSoftware:  {{360, 85, 0.4}, {450, 100, 0}, {1290, 100, 0}, {1410, 85, 0}},
			LaneKelvin:    {{270, 2300, 0}, {360, 3400, 0.5}, {480, 6500, 0}, {1080, 6500, -0.4}, {1260, 3400, 0}, {1380, 2300, 0}},
		}},
	{"early-bird", "Early bird",
		"Up at dawn: a cool, bright morning that wakes you, and an early warm evening.",
		map[string][]Point{
			LaneBacklight: {{300, 60, 0.5}, {390, 100, 0}, {1050, 100, -0.3}, {1200, 55, 0}, {1290, 30, 0}},
			LaneSoftware:  {{300, 90, 0.4}, {360, 100, 0}, {1200, 100, 0}, {1290, 80, 0}},
			LaneKelvin:    {{240, 2300, 0}, {300, 4500, 0.5}, {390, 7000, 0}, {600, 6500, 0}, {1020, 6500, -0.4}, {1170, 3200, 0}, {1260, 2200, 0}},
		}},
	{"night-owl", "Night owl",
		"A late start and a long evening, still winding down well before sleep.",
		map[string][]Point{
			LaneBacklight: {{510, 50, 0.5}, {630, 100, 0}, {1200, 100, -0.3}, {1380, 65, 0}, {90, 40, 0}},
			LaneSoftware:  {{480, 85, 0.4}, {570, 100, 0}, {1380, 100, 0}, {90, 85, 0}},
			LaneKelvin:    {{390, 2300, 0}, {480, 3400, 0.5}, {600, 6500, 0}, {1200, 6500, -0.4}, {1380, 3400, 0}, {60, 2300, 0}},
		}},
	{"gentle", "Gentle evenings",
		"Leaves the day alone and only warms the screen a little in the evening.",
		map[string][]Point{
			LaneBacklight: {{0, 100, 0}},
			LaneSoftware:  {{0, 100, 0}},
			LaneKelvin:    {{420, 4500, 0.4}, {510, 6500, 0}, {1140, 6500, -0.3}, {1320, 4500, 0}},
		}},
	{"candlelight", "Candlelight",
		"Strong: deep amber and dim from late evening, as if the room were lit by candles.",
		map[string][]Point{
			LaneBacklight: {{390, 40, 0.5}, {510, 100, 0}, {1080, 100, -0.4}, {1260, 45, 0}, {1380, 20, 0}},
			LaneSoftware:  {{390, 75, 0.4}, {480, 100, 0}, {1260, 100, 0}, {1380, 70, 0}},
			LaneKelvin:    {{300, 1900, 0}, {390, 3000, 0.5}, {510, 6500, 0}, {1050, 6500, -0.5}, {1230, 2700, 0}, {1350, 1900, 0}},
		}},
}

// FindPreset returns the preset with that name.
func FindPreset(name string) (Preset, bool) {
	for _, p := range Presets {
		if p.Name == name {
			return p, true
		}
	}
	return Preset{}, false
}

// WhitePoint is the red, green and blue multipliers (0–1) that give a white
// of the colour temperature kelvin, with 6500 K as plain white. It uses
// Tanner Helland's fit of the black-body colours; the interface draws its
// swatches with the same formula.
func WhitePoint(kelvin float64) (r, g, b float64) {
	r0, g0, b0 := blackBody(kelvin)
	nr, ng, nb := blackBody(6500)
	r, g, b = r0/nr, g0/ng, b0/nb
	top := math.Max(r, math.Max(g, b))
	return r / top, g / top, b / top
}

// blackBody is Tanner Helland's approximation of a black body's colour, in
// 0–255 per channel.
func blackBody(kelvin float64) (r, g, b float64) {
	t := clamp(kelvin, 1000, 40000) / 100
	if t <= 66 {
		r = 255
		g = 99.4708025861*math.Log(t) - 161.1195681661
	} else {
		r = 329.698727446 * math.Pow(t-60, -0.1332047592)
		g = 288.1221695283 * math.Pow(t-60, -0.0755148492)
	}
	switch {
	case t >= 66:
		b = 255
	case t <= 19:
		b = 0
	default:
		b = 138.5177312231*math.Log(t-10) - 305.0447927307
	}
	return clamp(r, 0, 255), clamp(g, 1, 255), clamp(b, 1, 255)
}

// Ramp is a gamma ramp of size steps per channel that draws colour
// temperature kelvin at brightness (0–1). Values are 16-bit, as X11 wants.
func Ramp(size int, kelvin, brightness float64) (red, green, blue []uint16) {
	wr, wg, wb := WhitePoint(kelvin)
	brightness = clamp(brightness, 0, 1)
	red, green, blue = make([]uint16, size), make([]uint16, size), make([]uint16, size)
	for i := 0; i < size; i++ {
		v := 65535.0 * brightness
		if size > 1 {
			v *= float64(i) / float64(size-1)
		}
		red[i] = uint16(math.Round(v * wr))
		green[i] = uint16(math.Round(v * wg))
		blue[i] = uint16(math.Round(v * wb))
	}
	return
}
