package blocker

import (
	"bufio"
	"encoding/binary"
	"io"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestReadXauthorityRefusesLinksPipesAndOthersFiles(t *testing.T) {
	dir := t.TempDir()
	uid := uint64(os.Getuid())
	file := filepath.Join(dir, "Xauthority")
	if err := os.WriteFile(file, []byte("cookie"), 0600); err != nil {
		t.Fatal(err)
	}
	if data, err := readXauthority(file, uid); err != nil || string(data) != "cookie" {
		t.Fatalf("own file: %q %v", data, err)
	}
	if _, err := readXauthority(file, uid+1); err == nil {
		t.Error("read a file of another user's")
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(file, link); err != nil {
		t.Fatal(err)
	}
	if _, err := readXauthority(link, uid); err == nil {
		t.Error("followed a symlink")
	}
	fifo := filepath.Join(dir, "fifo")
	if err := syscall.Mkfifo(fifo, 0600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := readXauthority(fifo, uid); done <- err }()
	select {
	case err := <-done:
		if err == nil {
			t.Error("read a pipe")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("a pipe with no writer hung the read")
	}
	big := filepath.Join(dir, "big")
	if err := os.WriteFile(big, make([]byte, 2*maxXauthority), 0600); err != nil {
		t.Fatal(err)
	}
	if data, _ := readXauthority(big, uid); len(data) != maxXauthority {
		t.Errorf("read %d bytes of a big file, want %d", len(data), maxXauthority)
	}
}

// fakeX11 returns a client connection whose server side runs serve, with
// the whole connection due within timeout, as dialX11 sets it.
func fakeX11(t *testing.T, timeout time.Duration, serve func(server net.Conn)) *x11Conn {
	t.Helper()
	client, server := net.Pipe()
	t.Cleanup(func() { client.Close(); server.Close() })
	go serve(server)
	_ = client.SetDeadline(time.Now().Add(timeout))
	return &x11Conn{conn: client, r: bufio.NewReader(client), atoms: map[string]uint32{}}
}

func TestX11RefusesHugeReplies(t *testing.T) {
	c := fakeX11(t, 2*time.Second, func(server net.Conn) {
		io.ReadFull(server, make([]byte, 8)) // the QueryTree request
		reply := make([]byte, 32)
		reply[0] = 1
		binary.LittleEndian.PutUint32(reply[4:], 1<<30) // 4 GiB to follow
		server.Write(reply)
	})
	if _, err := c.children(1); err == nil {
		t.Fatal("accepted a 4 GiB reply")
	}
}

func TestX11ConnectionHasOneDeadline(t *testing.T) {
	c := fakeX11(t, 300*time.Millisecond, func(server net.Conn) {
		io.Copy(io.Discard, server) // takes requests, never answers
	})
	start := time.Now()
	if _, err := c.children(1); err == nil {
		t.Fatal("a server that never answers answered")
	}
	if waited := time.Since(start); waited > 2*time.Second {
		t.Fatalf("waited %v, past the connection's deadline", waited)
	}
	// Later requests don't get a new deadline.
	start = time.Now()
	if _, err := c.children(1); err == nil {
		t.Fatal("answered after the deadline")
	}
	if waited := time.Since(start); waited > 100*time.Millisecond {
		t.Fatalf("a request after the deadline waited %v", waited)
	}
}

// Anyone can make a socket in /tmp/.X11-unix while no server holds the
// name: a user's cookie goes only to root's socket or that user's own, in a
// directory no one else can swap it in.
func TestTrustedX11Socket(t *testing.T) {
	dir := t.TempDir()
	old := x11SocketDir
	x11SocketDir = dir
	t.Cleanup(func() { x11SocketDir = old })
	l, err := net.Listen("unix", filepath.Join(dir, "X5"))
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	uid := uint64(os.Getuid())
	if uid != 0 {
		if trustedX11Socket("5", uid) {
			t.Error("trusted a socket in a directory a user owns")
		}
		return
	}
	if !trustedX11Socket("5", 1500) {
		t.Error("refused root's socket in root's directory")
	}
	if trustedX11Socket("6", 1500) {
		t.Error("trusted a missing socket")
	}
	if err := os.Chown(filepath.Join(dir, "X5"), 1501, 1501); err != nil {
		t.Fatal(err)
	}
	if trustedX11Socket("5", 1500) {
		t.Error("would send user 1500's cookie to user 1501's socket")
	}
	if !trustedX11Socket("5", 1501) {
		t.Error("refused a user's own socket")
	}
	if err := os.Chmod(dir, 0777); err != nil {
		t.Fatal(err)
	}
	if trustedX11Socket("5", 1501) {
		t.Error("trusted a directory anyone can swap sockets in")
	}
	if err := os.Chmod(dir, 0777|os.ModeSticky); err != nil {
		t.Fatal(err)
	}
	if !trustedX11Socket("5", 1501) {
		t.Error("refused a sticky directory")
	}
}
