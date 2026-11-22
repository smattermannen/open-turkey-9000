package winagent

import (
	"encoding/json"
	"testing"
)

// W51: 5000 gamma reads, gamma sets and monitor lists in one process (Go
// stops at 2000 callbacks: one per call ended the agent). Needs a
// desktop with a screen.
func TestWinDisplaysManyCalls(t *testing.T) {
	if names, _ := displays(); len(names) == 0 {
		t.Skip("no desktop with a screen")
	}
	var d dispatcher
	get := func() []Ramp {
		rep := d.answer(Request{ID: 1, Op: OpGetGamma})
		var ramps []Ramp
		if rep.Error != "" || json.Unmarshal(rep.Result, &ramps) != nil {
			t.Fatalf("get gamma: %+v", rep)
		}
		return ramps
	}
	before := get()
	plain, _ := json.Marshal(identityRamp())
	defer d.answer(Request{ID: 9, Op: OpSetGamma, Args: plain})
	for i := 0; i < 5000; i++ {
		get()
		if rep := d.answer(Request{ID: 2, Op: OpSetGamma, Args: plain}); rep.Error != "" && i == 0 {
			t.Logf("set gamma: %s", rep.Error) // a runner's screen may refuse ramps
		}
		if rep := d.answer(Request{ID: 3, Op: OpMonitors}); rep.Error != "" && i == 0 {
			t.Logf("monitors: %s", rep.Error)
		}
	}
	if len(get()) != len(before) {
		t.Errorf("screens changed while testing")
	}
}
