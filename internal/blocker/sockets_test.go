package blocker

import (
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestPrivilegedSocketsLockAndRestore(t *testing.T) {
	dir := t.TempDir()
	sock := filepath.Join(dir, "docker.sock")
	listener, err := net.Listen("unix", sock)
	if err != nil {
		t.Skipf("unix sockets unavailable: %v", err)
	}
	defer listener.Close()
	if err := os.Chmod(sock, 0o660); err != nil {
		t.Fatal(err)
	}
	plain := filepath.Join(dir, "not-a-socket")
	if err := os.WriteFile(plain, nil, 0o666); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(plain, 0o666); err != nil { // past the umask
		t.Fatal(err)
	}

	oldSockets, oldState := privilegedSockets, socketStatePath
	privilegedSockets = []string{sock, plain, filepath.Join(dir, "missing.sock")}
	socketStatePath = filepath.Join(dir, "state.json")
	defer func() { privilegedSockets, socketStatePath = oldSockets, oldState }()

	mode := func(p string) fs.FileMode {
		info, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		return info.Mode().Perm()
	}

	if err := LockPrivilegedSockets(); err != nil {
		t.Fatal(err)
	}
	if got := mode(sock); got != 0o600 {
		t.Fatalf("locked socket mode = %o, want 600", got)
	}
	if got := mode(plain); got != 0o666 {
		t.Fatalf("a plain file was touched: %o", got)
	}

	// Docker restarts and recreates its socket group-writable: the next
	// cycle closes it again, still remembering the original mode.
	if err := os.Chmod(sock, 0o660); err != nil {
		t.Fatal(err)
	}
	if err := LockPrivilegedSockets(); err != nil {
		t.Fatal(err)
	}
	if got := mode(sock); got != 0o600 {
		t.Fatalf("relocked socket mode = %o, want 600", got)
	}

	if err := UnlockPrivilegedSockets(); err != nil {
		t.Fatal(err)
	}
	if got := mode(sock); got != 0o660 {
		t.Fatalf("restored socket mode = %o, want 660", got)
	}
	if _, err := os.Stat(socketStatePath); !os.IsNotExist(err) {
		t.Error("state file left behind after unlocking")
	}
	// Unlocking with nothing locked is harmless.
	if err := UnlockPrivilegedSockets(); err != nil {
		t.Fatal(err)
	}
}
