package schedule

import "testing"

// §3.6 Automation (owner, 2026-11-16): a block enabled or disabled by
// default, switched by its windows and places, needing both or either.
func TestAutomationEnabled(t *testing.T) {
	w := []Window{{Days: 0x7f, Start: 9 * 60, End: 17 * 60}}
	p := []int{1}
	cases := []struct {
		name              string
		a                 Automation
		inWindow, atPlace bool
		want              bool
	}{
		{"continuous", Automation{}, false, false, true},
		{"continuous, enabled by default", Automation{DefaultOn: true}, true, true, true},
		{"window turns it on", Automation{Windows: w}, true, false, true},
		{"outside the window", Automation{Windows: w}, false, true, false},
		{"window turns it off", Automation{Windows: w, DefaultOn: true}, true, false, false},
		{"outside: on", Automation{Windows: w, DefaultOn: true}, false, false, true},
		{"place turns it on", Automation{Places: p}, false, true, true},
		{"away: off", Automation{Places: p}, true, false, false},
		{"arriving turns it off", Automation{Places: p, DefaultOn: true}, false, true, false},
		{"either: the window is enough", Automation{Windows: w, Places: p}, true, false, true},
		{"either: the place is enough", Automation{Windows: w, Places: p}, false, true, true},
		{"either: neither", Automation{Windows: w, Places: p}, false, false, false},
		{"both: the window alone isn't", Automation{Windows: w, Places: p, NeedBoth: true}, true, false, false},
		{"both: both", Automation{Windows: w, Places: p, NeedBoth: true}, true, true, true},
		{"on by default, both: only both turn it off", Automation{Windows: w, Places: p, DefaultOn: true, NeedBoth: true}, true, false, true},
		{"on by default, both", Automation{Windows: w, Places: p, DefaultOn: true, NeedBoth: true}, true, true, false},
		{"on by default, either: the place turns it off", Automation{Windows: w, Places: p, DefaultOn: true}, false, true, false},
	}
	for _, c := range cases {
		if got := c.a.Enabled(c.inWindow, c.atPlace); got != c.want {
			t.Errorf("%s: Enabled(%v, %v) = %v", c.name, c.inWindow, c.atPlace, got)
		}
	}
	// With no network seen the place counts as what keeps the block on.
	for _, a := range []Automation{{Places: p}, {Places: p, DefaultOn: true}, {Windows: w, Places: p, NeedBoth: true}} {
		if !a.Enabled(false, a.UnknownPlace()) && !a.NeedBoth {
			t.Errorf("%+v: no network seen lifted the block", a)
		}
	}
}

// Changes towards more enabled time are free; the rest weaken the block.
func TestAutomationWeakens(t *testing.T) {
	work := Window{Days: 0x3e, Start: 9 * 60, End: 17 * 60}
	night := Window{Days: 0x7f, Start: 22 * 60, End: 24 * 60}
	cases := []struct {
		name string
		a, b Automation
		want bool
	}{
		{"a first place on a continuous block (a lock bypass)", Automation{}, Automation{Places: []int{1}}, true},
		{"a first window on a continuous block", Automation{}, Automation{Windows: []Window{work}}, true},
		{"another window turning it on", Automation{Windows: []Window{work}}, Automation{Windows: []Window{work, night}}, false},
		{"removing a window turning it on", Automation{Windows: []Window{work, night}}, Automation{Windows: []Window{work}}, true},
		{"another window turning it off", Automation{Windows: []Window{work}, DefaultOn: true},
			Automation{Windows: []Window{work, night}, DefaultOn: true}, true},
		{"removing a window turning it off", Automation{Windows: []Window{work, night}, DefaultOn: true},
			Automation{Windows: []Window{work}, DefaultOn: true}, false},
		{"removing the last window turning it off", Automation{Windows: []Window{work}, DefaultOn: true}, Automation{}, false},
		{"another place turning it on", Automation{Places: []int{1}}, Automation{Places: []int{1, 2}}, false},
		{"another place turning it off", Automation{Places: []int{1}, DefaultOn: true}, Automation{Places: []int{1, 2}, DefaultOn: true}, true},
		{"removing a place turning it off", Automation{Places: []int{1, 2}, DefaultOn: true}, Automation{Places: []int{1}, DefaultOn: true}, false},
		{"either to both (turning on)", Automation{Windows: []Window{work}, Places: []int{1}},
			Automation{Windows: []Window{work}, Places: []int{1}, NeedBoth: true}, true},
		{"both to either (turning on)", Automation{Windows: []Window{work}, Places: []int{1}, NeedBoth: true},
			Automation{Windows: []Window{work}, Places: []int{1}}, false},
		{"either to both (turning off)", Automation{Windows: []Window{work}, Places: []int{1}, DefaultOn: true},
			Automation{Windows: []Window{work}, Places: []int{1}, DefaultOn: true, NeedBoth: true}, false},
		{"both to either (turning off)", Automation{Windows: []Window{work}, Places: []int{1}, DefaultOn: true, NeedBoth: true},
			Automation{Windows: []Window{work}, Places: []int{1}, DefaultOn: true}, true},
		{"disabled by default to enabled by default", Automation{Windows: []Window{work}},
			Automation{Windows: []Window{work}, DefaultOn: true}, true},
		{"the default of a continuous block", Automation{}, Automation{DefaultOn: true}, false},
		{"nothing changed", Automation{Windows: []Window{work}, Places: []int{1}, DefaultOn: true},
			Automation{Windows: []Window{work}, Places: []int{1}, DefaultOn: true}, false},
	}
	for _, c := range cases {
		if got := AutomationWeakens(c.a, c.b); got != c.want {
			t.Errorf("%s: weakens = %v", c.name, got)
		}
	}
}
