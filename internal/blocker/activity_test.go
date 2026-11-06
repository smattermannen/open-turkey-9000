package blocker

import "testing"

func TestLastField(t *testing.T) {
	cases := map[string]string{
		"_NET_ACTIVE_WINDOW(WINDOW): window id # 0x3e00049\n": "0x3e00049",
		"_NET_WM_PID(CARDINAL) = 7276\n":                      "7276",
		"(uint32 42,)\n":                                      "42",
		"":                                                    "",
	}
	for in, want := range cases {
		if got := lastField(in); got != want {
			t.Errorf("lastField(%q) = %q, want %q", in, got, want)
		}
	}
}
