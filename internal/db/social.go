// social.go — social media settings: per block, presets that keep a site
// usable but take a part of it away ("no Shorts", "messages only"). The
// extension carries them out (webext/files/social.js); here are their names.
//
// A YouTube playlist a block lets through is its address in an exception
// list (§3.2 Exception list; owner 2026-11-17). For the extension it is
// handed over as the setting "youtube:playlist=<id>", never stored: with one
// or more of those, YouTube's video pages are only for videos of those
// playlists (withPlaylists).
package db

import (
	"database/sql"
	"fmt"
	"regexp"
	"slices"
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
	{"instagram:following-only", "Instagram", "Following only (no suggested posts)"},
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

// PlaylistPrefix starts the setting that lets one YouTube playlist through,
// as the extension gets it (withPlaylists); never stored since 2026-11-17.
const PlaylistPrefix = "youtube:playlist="

// VideoPages is the exception a playlist let through needs besides its own
// page: YouTube's video pages. Which videos, the extension decides (the
// playlists' only). Added for enforcement, never stored (withPlaylists).
const VideoPages = "youtube.com/watch"

// PlaylistException is a playlist's address as an exception list keeps it.
func PlaylistException(id string) string { return "youtube.com/playlist?list=" + id }

var playlistAddress = regexp.MustCompile(`(?i)^(?:https?://)?(?:(?:www|m|music)\.)?youtube\.com/playlist\?(?:[^#]*&)?list=([A-Za-z0-9_-]{10,64})(?:[&#].*)?$`)

// PlaylistInEntry returns the playlist an entry is the address of
// (youtube.com/playlist?list=<id>, with or without https://www.).
func PlaylistInEntry(entry string) (string, bool) {
	m := playlistAddress.FindStringSubmatch(strings.TrimSpace(entry))
	if m == nil {
		return "", false
	}
	return m[1], true
}

// Playlists returns the YouTube playlists the exception lists let through:
// their addresses in an exception list that looks at the address.
func (l Lists) Playlists() []string {
	var out []string
	for _, list := range l {
		if list.Kind != ListExceptions || !list.Where.URL {
			continue
		}
		for _, e := range list.Entries {
			if id, ok := PlaylistInEntry(e); ok && !slices.Contains(out, id) {
				out = append(out, id)
			}
		}
	}
	return out
}

// withPlaylists gives an active block what its playlists need to be
// enforced: the restriction to their videos for the extension, and the
// video pages among the exceptions of the list that holds them. Neither is
// stored, so a playlist is one exception entry to add or remove.
func withPlaylists(a *ActiveBlockDetail) {
	ids := a.Lists.Playlists()
	if len(ids) == 0 {
		return
	}
	for _, id := range ids {
		if p := PlaylistPrefix + id; !slices.Contains(a.Social, p) {
			a.Social = append(a.Social, p)
		}
	}
	for i, list := range a.Lists {
		if list.Kind != ListExceptions || !list.Where.URL || len(Lists{list}.Playlists()) == 0 {
			continue
		}
		if !slices.ContainsFunc(list.Entries, func(e string) bool { return strings.EqualFold(e, VideoPages) }) {
			a.Lists[i].Entries = append(slices.Clone(list.Entries), VideoPages)
		}
		return
	}
}

// migratePlaylistExceptions (owner 2026-11-17): a playlist let through was
// kept as a setting "youtube:playlist=<id>" beside two exceptions, its page
// and the video pages. Now its page in an exception list is all there is:
// the settings go, the page stays (added where missing), and the video
// pages exception goes from blocks that had a playlist (enforcement adds it
// while one is there), so what is enforced stays the same.
func migratePlaylistExceptions(conn *sql.DB) error {
	rows, err := conn.Query("SELECT block_id, preset FROM block_social WHERE preset LIKE ?", PlaylistPrefix+"%")
	if err != nil {
		return err
	}
	byBlock := map[int][]string{}
	var order []int
	for rows.Next() {
		var block int
		var preset string
		if err := rows.Scan(&block, &preset); err != nil {
			rows.Close()
			return err
		}
		if _, ok := byBlock[block]; !ok {
			order = append(order, block)
		}
		byBlock[block] = append(byBlock[block], strings.TrimPrefix(preset, PlaylistPrefix))
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, block := range order {
		if err := adoptPlaylists(conn, block, byBlock[block]); err != nil {
			return err
		}
	}
	return nil
}

// AdoptOldPlaylists brings playlists of an older project file (its block's
// social media settings "youtube:playlist=<id>") into the block as
// migratePlaylistExceptions does.
func (d *DB) AdoptOldPlaylists(blockName string, ids []string) error {
	blockID, err := d.getBlockID(blockName)
	if err != nil {
		return err
	}
	return adoptPlaylists(d.conn, blockID, ids)
}

// adoptPlaylists: each playlist's page in an exception list, no video pages
// exception of its own, no old setting rows.
func adoptPlaylists(conn querier, block int, ids []string) error {
	lists, err := getBlockLists(conn, block)
	if err != nil {
		return err
	}
	var target int
	for _, l := range lists {
		if l.Kind == ListExceptions && l.Where.URL && (target == 0 || l.Name == DefaultExceptionsList) {
			target = l.ID
		}
	}
	if target == 0 {
		res, err := conn.Exec("INSERT INTO web_lists (block_id, kind, name, in_url) VALUES (?, ?, ?, 1)",
			block, ListExceptions, DefaultExceptionsList)
		if err != nil {
			return err
		}
		id, _ := res.LastInsertId()
		target = int(id)
	}
	have := Lists(lists).Playlists()
	for _, id := range ids {
		if playlistID.MatchString(id) && !slices.Contains(have, id) {
			if _, err := conn.Exec("INSERT INTO web_list_entries (list_id, entry) VALUES (?, ?)", target, PlaylistException(id)); err != nil {
				return err
			}
		}
	}
	for _, l := range lists {
		if l.Kind != ListExceptions {
			continue
		}
		for _, e := range l.Entries {
			if strings.EqualFold(e, VideoPages) {
				if _, err := conn.Exec("DELETE FROM web_list_entries WHERE list_id = ? AND entry = ?", l.ID, e); err != nil {
					return err
				}
			}
		}
	}
	if _, err := conn.Exec("DELETE FROM block_social WHERE block_id = ? AND preset LIKE ?", block, PlaylistPrefix+"%"); err != nil {
		return err
	}
	return nil
}

// OldPlaylists splits an older project file's social media settings into
// the settings and the playlists it let through ("youtube:playlist=<id>").
func OldPlaylists(social []string) (settings, playlists []string) {
	for _, s := range social {
		if id, ok := strings.CutPrefix(s, PlaylistPrefix); ok {
			if playlistID.MatchString(id) {
				playlists = append(playlists, id)
			}
			continue
		}
		settings = append(settings, s)
	}
	return settings, playlists
}

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

// IsSocialPreset reports whether name is a social media setting.
func IsSocialPreset(name string) bool {
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
