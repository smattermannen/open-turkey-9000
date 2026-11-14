package blocker

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// R-MONK-2: every wired interface is found, and nothing else.
func TestR_MONK_2_EthernetLinks(t *testing.T) {
	root := t.TempDir()
	old := sysNet
	sysNet = root
	defer func() { sysNet = old }()
	add := func(name, kind, flags string, extra ...string) {
		dir := filepath.Join(root, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		os.WriteFile(filepath.Join(dir, "type"), []byte(kind+"\n"), 0o644)
		os.WriteFile(filepath.Join(dir, "flags"), []byte(flags+"\n"), 0o644)
		for _, e := range extra {
			os.Mkdir(filepath.Join(dir, e), 0o755)
		}
	}
	add("lo", "772", "0x9")
	add("enp3s0", "1", "0x1003", "device")       // up
	add("enx00e04c680001", "1", "0x1002", "device") // USB adapter, down
	add("wlp2s0", "1", "0x1003", "device", "wireless", "phy80211")
	add("docker0", "1", "0x1003")
	add("veth1a2b", "1", "0x1003")
	add("tun0", "65534", "0x1091")
	want := map[string]bool{"enp3s0": true, "enx00e04c680001": false}
	if got := EthernetLinks(); !reflect.DeepEqual(got, want) {
		t.Errorf("EthernetLinks = %v, want %v", got, want)
	}
}
