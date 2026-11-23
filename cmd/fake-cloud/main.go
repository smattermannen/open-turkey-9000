// Command fake-cloud serves internal/cloud/cloudtest's fake of KC9K's cloud
// (Supabase Auth, Storage, the setups table) for tests outside Go, such as
// ci/gui_account_smoke.py. It prints the address and the anon key on one
// line, then serves until killed. Never part of a release.
package main

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/smattermannen/kc9k/internal/cloud/cloudtest"
)

func main() {
	s := cloudtest.New()
	defer s.Close()
	fmt.Println(s.URL, cloudtest.AnonKey)
	os.Stdout.Sync()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop
}
