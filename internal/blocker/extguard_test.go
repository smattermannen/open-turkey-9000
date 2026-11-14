package blocker

import (
	"net"
	"os"
	"testing"
	"time"
)

func TestParseAge(t *testing.T) {
	// pid (comm with ) and spaces) state ppid ... field 22 = 500 ticks = 5 s.
	stat := "123 (a) b c) S 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 500 19 20"
	age, ok := parseAge(stat, 65*time.Second)
	if !ok || age != 60*time.Second {
		t.Errorf("got %v %v, want 60s", age, ok)
	}
	if _, ok := parseAge("garbage", time.Second); ok {
		t.Error("parsed garbage")
	}
}

// The guard must find the socket a process opened to the daemon's port.
func TestSocketInodeFindsOwnConnection(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip("no loopback:", err)
	}
	defer listener.Close()
	go func() {
		if c, err := listener.Accept(); err == nil {
			time.Sleep(time.Second)
			c.Close()
		}
	}()
	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	client := conn.LocalAddr().(*net.TCPAddr).Port
	server := listener.Addr().(*net.TCPAddr).Port

	inode := socketInode(client, server)
	if inode == "" {
		t.Fatal("socket not found in /proc/net/tcp")
	}
	fds, _ := os.ReadDir("/proc/self/fd")
	for _, fd := range fds {
		if link, _ := os.Readlink("/proc/self/fd/" + fd.Name()); link == "socket:["+inode+"]" {
			return
		}
	}
	t.Errorf("inode %s is not one of this process's sockets", inode)
}

// closeReason decides which browsers the guard closes.
func TestCloseReason(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	old := ExtensionGrace + time.Minute
	cases := []struct {
		name        string
		p           guardedProcess
		changed     time.Time
		unsupported bool
		closed      bool
	}{
		{"new browser, grace", guardedProcess{supported: true, age: 10 * time.Second, ageKnown: true}, time.Time{}, false, false},
		{"old, never checked in", guardedProcess{supported: true, age: old, ageKnown: true}, time.Time{}, false, true},
		{"old, checked in", guardedProcess{supported: true, age: old, ageKnown: true, seen: true, lastSeen: now.Add(-5 * time.Second)}, time.Time{}, false, false},
		{"old, check-in stale", guardedProcess{supported: true, age: old, ageKnown: true, seen: true, lastSeen: now.Add(-2 * ExtensionGrace)}, time.Time{}, false, true},
		{"remote control", guardedProcess{supported: true, remote: true, age: time.Second, ageKnown: true}, time.Time{}, false, true},
		{"started before a protecting change", guardedProcess{supported: true, age: 10 * time.Minute, ageKnown: true, seen: true, lastSeen: now},
			now.Add(-2 * time.Minute), false, true},
		{"started after a protecting change", guardedProcess{supported: true, age: time.Minute, ageKnown: true, seen: true, lastSeen: now},
			now.Add(-2 * time.Minute), false, false},
		// Block unsupported browsers: others close at once, even fresh or
		// (impossibly) checked in; supported ones keep their grace.
		{"unsupported, fresh, setting off", guardedProcess{age: time.Second, ageKnown: true}, time.Time{}, false, false},
		{"unsupported, fresh, setting on", guardedProcess{age: time.Second, ageKnown: true}, time.Time{}, true, true},
		{"unsupported, checked in, setting on", guardedProcess{age: old, ageKnown: true, seen: true, lastSeen: now}, time.Time{}, true, true},
		{"unsupported, age unknown, setting on", guardedProcess{}, time.Time{}, true, true},
		{"supported, fresh, setting on", guardedProcess{supported: true, age: time.Second, ageKnown: true}, time.Time{}, true, false},
		{"supported, checked in, setting on", guardedProcess{supported: true, age: old, ageKnown: true, seen: true, lastSeen: now}, time.Time{}, true, false},
		{"supported, no extension, setting on", guardedProcess{supported: true, age: old, ageKnown: true}, time.Time{}, true, true},
	}
	for _, c := range cases {
		reason := closeReason(c.p, now, c.changed, c.unsupported)
		if (reason != "") != c.closed {
			t.Errorf("%s: reason %q; want closed = %v", c.name, reason, c.closed)
		}
	}
}
