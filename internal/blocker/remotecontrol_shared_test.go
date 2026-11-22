package blocker

import "testing"

// W11: the switches and variables that open a browser to remote control,
// as Windows' command lines and environments give them too.
func TestRemoteControlArgs(t *testing.T) {
	cases := []struct {
		name      string
		args, env []string
		want      bool
	}{
		{"plain Chrome", []string{`C:\Program Files\Google\Chrome\Application\chrome.exe`, "https://example.com"}, []string{"PATH=C:\\Windows"}, false},
		{"debugging port", []string{"chrome.exe", "--remote-debugging-port=9222"}, nil, true},
		{"debugging pipe", []string{"msedge.exe", "--remote-debugging-pipe"}, nil, true},
		{"extensions off", []string{"chrome.exe", "--disable-extensions"}, nil, true},
		{"Firefox, Windows' slash", []string{"firefox.exe", "/marionette"}, nil, true},
		{"Firefox, one dash, any case", []string{"librewolf.exe", "-Marionette"}, nil, true},
		{"a path is no switch", []string{"chrome.exe", "/marionette/page.html"}, nil, false},
		{"Marionette by environment", []string{"firefox.exe"}, []string{"Moz_Marionette=1"}, true},
		{"empty variable", []string{"firefox.exe"}, []string{"MOZ_MARIONETTE="}, false},
	}
	for _, c := range cases {
		if got := remoteControlArgs(c.args, c.env); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}
