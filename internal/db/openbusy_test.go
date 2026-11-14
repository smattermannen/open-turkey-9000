package db

import (
	"fmt"
	"path/filepath"
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
