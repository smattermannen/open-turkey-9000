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
