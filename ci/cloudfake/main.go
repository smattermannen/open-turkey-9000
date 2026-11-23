// Command cloudfake serves internal/cloud/cloudtest's fake Supabase (Auth,
// Storage, rows with row-level security) for tests outside Go, such as
// ci/gui_account_smoke.py: it prints "<url> <anon key>" on one line, then
// serves until killed.
//
//	go run ./ci/cloudfake
package main

import (
	"fmt"

	"github.com/smattermannen/kc9k/internal/cloud/cloudtest"
)

func main() {
	f := cloudtest.New()
	fmt.Println(f.URL, cloudtest.AnonKey)
	select {}
}
