// social.go — social media settings: per block, presets that keep a site
// usable but take a part of it away ("no Shorts", "messages only"). The
// extension carries them out (webext/files/social.js); here are their names.
//
// A YouTube playlist a block lets through is kept the same way, as
// "youtube:playlist=<id>": with one or more of those, YouTube's video pages
// are only for videos of those playlists.
package db

import (
	"fmt"
	"regexp"
	"strings"
)

const socialSchema = `CREATE TABLE IF NOT EXISTS block_social (
    block_id INTEGER NOT NULL,
    preset   TEXT NOT NULL,
    PRIMARY KEY (block_id, preset),
    FOREIGN KEY (block_id) REFERENCES blocks(id) ON DELETE CASCADE
);`

// SocialPreset is one thing that can be taken away from a site.
type SocialPreset struct {
	Name  string `json:"name"` // "<site>:<what>"
	Site  string `json:"site"`
	Title string `json:"title"`
}

// SocialPresets lists the presets in display order.
var SocialPresets = []SocialPreset{
	{"youtube:no-shorts", "YouTube", "No Shorts"},
	{"youtube:no-home", "YouTube", "No home feed (opens Subscriptions)"},
	{"youtube:no-recommendations", "YouTube", "No recommendations beside and after videos"},
	{"youtube:no-comments", "YouTube", "No comments"},
	{"instagram:messages-only", "Instagram", "Messages only"},
	{"instagram:no-reels", "Instagram", "No Reels"},
	{"instagram:no-explore", "Instagram", "No Explore page"},
	{"facebook:no-feed", "Facebook", "No news feed"},
	{"facebook:no-reels", "Facebook", "No Reels or Watch"},
	{"facebook:messages-only", "Facebook", "Messages only"},
	{"x:no-explore", "X", "No Explore page or trends"},
	{"x:messages-only", "X", "Messages only"},
	{"reddit:no-popular", "Reddit", "No Popular or All"},
	{"reddit:no-home", "Reddit", "No home feed"},
	{"tiktok:following-only", "TikTok", "Following only (no For You)"},
	{"tiktok:no-explore", "TikTok", "No Explore page"},
	{"linkedin:no-feed", "LinkedIn", "No feed"},
	{"linkedin:no-news", "LinkedIn", "No news and suggestions column"},
}

// PlaylistPrefix starts the preset that lets one YouTube playlist through.
const PlaylistPrefix = "youtube:playlist="

var playlistID = regexp.MustCompile(`^[A-Za-z0-9_-]{10,64}$`)

// PlaylistID returns the playlist ID in an ID or a YouTube address.
func PlaylistID(text string) (string, bool) {
	text = strings.TrimSpace(text)
	if i := strings.Index(text, "list="); i >= 0 {
		text = text[i+len("list="):]
		if j := strings.IndexAny(text, "&#"); j >= 0 {
			text = text[:j]
		}
	}
	return text, playlistID.MatchString(text)
}

// IsSocialPreset reports whether name is a preset or a playlist entry.
func IsSocialPreset(name string) bool {
	if id, ok := strings.CutPrefix(name, PlaylistPrefix); ok {
		return playlistID.MatchString(id)
	}
	for _, p := range SocialPresets {
		if p.Name == name {
			return true
		}
	}
	return false
}

func (d *DB) getBlockSocial(blockID int) ([]string, error) {
	rows, err := d.conn.Query("SELECT preset FROM block_social WHERE block_id = ? ORDER BY preset", blockID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch social media settings: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, fmt.Errorf("failed to read a social media setting: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// SetSocial turns a preset on or off for a block.
func (d *DB) SetSocial(blockName, preset string, on bool) error {
	if !IsSocialPreset(preset) {
		return fmt.Errorf("unknown social media setting %q", preset)
	}
	blockID, err := d.getBlockID(blockName)
	if err != nil {
		return err
	}
	if on {
		_, err = d.conn.Exec("INSERT OR IGNORE INTO block_social (block_id, preset) VALUES (?, ?)", blockID, preset)
	} else {
		_, err = d.conn.Exec("DELETE FROM block_social WHERE block_id = ? AND preset = ?", blockID, preset)
	}
	if err != nil {
		return fmt.Errorf("failed to save a social media setting of '%s': %w", blockName, err)
	}
	return nil
}
