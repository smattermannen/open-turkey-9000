// Command chrome-store-zip writes the extension as the Chrome Web Store takes
// it (ROADMAP 2.17): the Manifest V3 files of webext.PackChrome, as a plain
// zip (webext.PackChromeStoreZip) with no key or update_url (the store signs it, gives it its ID and
// serves its updates). The listing's texts and pictures are in
// internal/webext/store/.
//
//	make chrome-store-zip            # kc9k-chrome-store-<version>.zip
//	go run ./cmd/chrome-store-zip -o <file>
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/smattermannen/kc9k/internal/webext"
)

func main() {
	out := flag.String("o", "", "where to write the zip (default kc9k-chrome-store-<version>.zip)")
	flag.Parse()
	path := *out
	if path == "" {
		path = "kc9k-chrome-store-" + webext.Version() + ".zip"
	}
	data, err := webext.PackChromeStoreZip()
	if err == nil {
		err = os.WriteFile(path, data, 0644)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "chrome-store-zip:", err)
		os.Exit(1)
	}
	fmt.Println(path)
}
