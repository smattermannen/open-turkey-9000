package blocker

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// HashPrefix starts an app entry that names a program by its contents:
// "sha256:<hex>:<size>:<path it was taken from>". 'block add-app' adds one
// next to a full path, so a renamed or moved copy of the program is still
// blocked.
const HashPrefix = "sha256:"

// HashEntry returns the contents entry for the program at path.
func HashEntry(path string) (string, bool) {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || !filepath.IsAbs(path) {
		return "", false
	}
	sum, err := fileSHA256(path)
	if err != nil {
		return "", false
	}
	return fmt.Sprintf("%s%s:%d:%s", HashPrefix, sum, info.Size(), path), true
}

// HashEntryFor reports whether entry is the contents entry made for path.
func HashEntryFor(entry, path string) bool {
	return strings.HasPrefix(entry, HashPrefix) && strings.HasSuffix(entry, ":"+path)
}

func parseHashEntry(entry string) (sum string, size int64, ok bool) {
	if !strings.HasPrefix(entry, HashPrefix) {
		return "", 0, false
	}
	parts := strings.SplitN(entry[len(HashPrefix):], ":", 3)
	if len(parts) < 2 || len(parts[0]) != 64 {
		return "", 0, false
	}
	size, err := strconv.ParseInt(parts[1], 10, 64)
	return parts[0], size, err == nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
