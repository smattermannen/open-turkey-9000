// projectcircadian.go — the circadian rhythm in project files (§3.13,
// ROADMAP 2.1): its settings and wallpaper pictures. Import sets them only
// where this computer has no circadian settings yet (import only adds).
package cli

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/smattermannen/kc9k/internal/circadian"
	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
)

// circadianExport is the circadian rhythm in a project file.
type circadianExport struct {
	Config     circadian.Config  `json:"config"`
	Wallpapers map[string]string `json:"wallpapers,omitempty"` // stored name -> base64
}

// exportCircadian returns the circadian rhythm, or nil if never set.
func exportCircadian(database *db.DB) *circadianExport {
	if !enforce.CircadianConfigured(database) {
		return nil
	}
	out := &circadianExport{Config: enforce.CircadianConfig(database), Wallpapers: map[string]string{}}
	for _, w := range out.Config.Wallpapers {
		if data, err := os.ReadFile(filepath.Join(enforce.WallpapersDir(), filepath.Base(w.Image))); err == nil {
			out.Wallpapers[w.Image] = base64.StdEncoding.EncodeToString(data)
		}
	}
	return out
}

// checkedWallpaper decodes a wallpaper from a file: it must be a PNG, JPEG
// or WebP picture named by its SHA-256 and type, as storeWallpaper names it.
func checkedWallpaper(name, encoded string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(raw) > maxWallpaperSize {
		return nil, fmt.Errorf("the wallpaper %s in the file is damaged", name)
	}
	ext := map[string]string{"image/png": "png", "image/jpeg": "jpg", "image/webp": "webp"}[http.DetectContentType(raw)]
	sum := sha256.Sum256(raw)
	if ext == "" || name != hex.EncodeToString(sum[:])+"."+ext {
		return nil, fmt.Errorf("the wallpaper %s in the file is damaged", name)
	}
	return raw, nil
}

// importCircadian sets the circadian rhythm from a file where none is set
// here; wallpapers whose picture isn't in the file are left out.
func importCircadian(database *db.DB, in *circadianExport, dry bool, say func(string, ...any)) error {
	if in == nil {
		return nil
	}
	if enforce.CircadianConfigured(database) {
		fmt.Println("The circadian rhythm is set up here already: left as it is.")
		return nil
	}
	c := in.Config
	var keep []circadian.Wallpaper
	pictures := map[string][]byte{}
	for _, w := range c.Wallpapers {
		raw, err := checkedWallpaper(w.Image, in.Wallpapers[w.Image])
		if err != nil {
			fmt.Printf("Wallpaper %s left out: %v\n", w.Name, err)
			continue
		}
		pictures[w.Image] = raw
		keep = append(keep, w)
	}
	c.Wallpapers = keep
	say("set the circadian rhythm (%d wallpapers)", len(keep))
	if dry {
		return nil
	}
	dir := enforce.WallpapersDir()
	if len(pictures) > 0 {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		_ = os.Chmod(dir, 0o755)
	}
	for name, raw := range pictures {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, raw, 0o644); err != nil {
			return fmt.Errorf("failed to store a wallpaper: %w", err)
		}
		_ = os.Chmod(path, 0o644)
	}
	return enforce.SetCircadianConfig(database, c)
}
