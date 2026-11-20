package db

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
)

// Commands that start together must not collide: opening a database that is
// up to date writes nothing, so it can't fail with "database is locked"
// while another command writes.
func TestOpenWhileAnotherCommandWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.db")
	first, err := OpenDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	var wg sync.WaitGroup
	errs := make(chan error, 64)
	for worker := 0; worker < 4; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 8; i++ {
				d, err := OpenDB(path)
				if err != nil {
					errs <- err
					return
				}
				if err := d.SetSetting(fmt.Sprintf("busy.%d.%d", worker, i), "x"); err != nil {
					errs <- err
				}
				d.Close()
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

// A fresh install starts the daemon and runs `kc9k-bin browser
// install-librewolf` at once (install.sh): separate processes create the same
// new database. None may fail with "database is locked" (ROADMAP A13). Each
// opener is this test binary run again (KC9K_TEST_OPEN names the database).
func TestFreshDatabaseOpenedByManyAtOnce(t *testing.T) {
	if path := os.Getenv("KC9K_TEST_OPEN"); path != "" {
		d, err := OpenDB(path)
		if err == nil {
			err = d.SetSetting("fresh."+strconv.Itoa(os.Getpid()), "x")
			d.Close()
		}
		if err != nil {
			fmt.Println("OPEN FAILED:", err)
			os.Exit(1)
		}
		return
	}
	for round := 0; round < 4; round++ {
		path := filepath.Join(t.TempDir(), "fresh.db")
		var cmds []*exec.Cmd
		var outs []*bytes.Buffer
		for worker := 0; worker < 4; worker++ {
			cmd := exec.Command(os.Args[0], "-test.run=^TestFreshDatabaseOpenedByManyAtOnce$")
			cmd.Env = append(os.Environ(), "KC9K_TEST_OPEN="+path)
			out := &bytes.Buffer{}
			cmd.Stdout, cmd.Stderr = out, out
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			cmds, outs = append(cmds, cmd), append(outs, out)
		}
		for i, cmd := range cmds {
			if err := cmd.Wait(); err != nil {
				t.Errorf("round %d, opener %d: %v\n%s", round, i, err, outs[i])
			}
		}
	}
}
