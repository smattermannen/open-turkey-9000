// entryfile.go — reading files to import (§4.3): one entry per line, #
// comments. KC9K tells the layouts apart itself (owner, 2026-11-16: one
// Import button): phrases ("[eat, nice] [hello, grandpa]"), rendered terms,
// and sites, also from the classic block-list layouts: hosts files
// ("0.0.0.0 example.com") and adblock rules ("||example.com^").
package cli

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"

	"github.com/smattermannen/kc9k/internal/blocker"
	"github.com/smattermannen/kc9k/internal/db"
	"github.com/spf13/cobra"
)

// entryFile is what a file holds, each line read once.
type entryFile struct {
	Phrases []string `json:"phrases"`
	Entries []string `json:"entries"` // sites, URL entries, patterns and rendered terms, in file order
	Sites   []string `json:"sites"`   // the sites, URL entries and patterns among Entries
	Terms   []string `json:"terms"`   // the rendered terms among Entries
	Skipped []string `json:"skipped"` // lines that are none of these (adblock page rules and exceptions)
}

// readEntries reads a file as the user who runs kc9k.
func readEntries(path string) (entryFile, error) {
	data, err := readAsCaller(path)
	if err != nil {
		return entryFile{}, err
	}
	f := classifyEntryLines(strings.Split(string(data), "\n"))
	if len(f.Phrases)+len(f.Entries)+len(f.Skipped) == 0 {
		return f, fmt.Errorf("%s has no entries", path)
	}
	return f, nil
}

// hostsNames are the names a hosts file gives the computer itself.
var hostsNames = map[string]bool{"localhost": true, "localhost.localdomain": true, "local": true,
	"broadcasthost": true, "ip6-localhost": true, "ip6-loopback": true, "ip6-localnet": true,
	"ip6-mcastprefix": true, "ip6-allnodes": true, "ip6-allrouters": true, "ip6-allhosts": true, "0.0.0.0": true}

// hostsLine reads a hosts file line ("0.0.0.0 a.com b.com # note"): its
// names, or false when the line isn't one.
func hostsLine(line string) ([]string, bool) {
	fields := strings.Fields(strings.SplitN(line, "#", 2)[0])
	if len(fields) < 2 || net.ParseIP(fields[0]) == nil {
		return nil, false
	}
	var names []string
	for _, name := range fields[1:] {
		name = strings.ToLower(strings.TrimSuffix(name, "."))
		if !hostsNames[name] && blocker.IsValidDomain(name) {
			names = append(names, name)
		}
	}
	return names, true
}

// adblockLine reads an adblock rule: a site ("||example.com^"), a URL entry
// ("||example.com/ads", "|https://example.com/x"), "" for a rule KC9K has
// no entry for (exceptions, page rules), or false when the line isn't one.
func adblockLine(line string) (string, bool) {
	switch {
	case strings.HasPrefix(line, "@@"), strings.Contains(line, "##"), strings.Contains(line, "#@#"),
		strings.Contains(line, "#?#"), strings.Contains(line, "#$#"):
		return "", true
	case strings.HasPrefix(line, "||"):
		rule := strings.TrimPrefix(line, "||")
		rule, _, _ = strings.Cut(rule, "$") // options: third-party, script…
		rule = strings.TrimSuffix(strings.TrimSuffix(rule, "|"), "^")
		host, path, _ := strings.Cut(rule, "/")
		host = strings.ToLower(strings.TrimSuffix(host, "^"))
		if !blocker.IsValidDomain(host) || strings.ContainsAny(path, "^|") {
			return "", true
		}
		if path != "" {
			return host + "/" + path, true
		}
		return host, true
	case strings.HasPrefix(line, "|http://"), strings.HasPrefix(line, "|https://"):
		rule, _, _ := strings.Cut(strings.TrimPrefix(line, "|"), "$")
		rule = strings.TrimSuffix(strings.TrimSuffix(rule, "|"), "^")
		if !urlEntry(rule) {
			return "", true
		}
		return normalizeURLEntry(rule), true
	}
	return "", false
}

// classifyEntryLines sorts a file's lines: phrases, sites and rendered
// terms, each once (case ignored), and what can't be used.
func classifyEntryLines(lines []string) entryFile {
	f := entryFile{Phrases: []string{}, Entries: []string{}, Sites: []string{}, Terms: []string{}, Skipped: []string{}}
	seen := map[string]bool{}
	entry := func(e string) {
		key := strings.ToLower(e)
		if seen[key] {
			return
		}
		seen[key] = true
		f.Entries = append(f.Entries, e)
		if urlEntry(e) {
			f.Sites = append(f.Sites, e)
		} else {
			f.Terms = append(f.Terms, e)
		}
	}
	for _, raw := range lines {
		line := strings.TrimSpace(strings.TrimPrefix(raw, "\ufeff"))
		lower := strings.ToLower(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") ||
			(strings.HasPrefix(lower, "[adblock") && strings.HasSuffix(line, "]")) {
			continue
		}
		if names, ok := hostsLine(line); ok {
			for _, name := range names {
				entry(name)
			}
			continue
		}
		if site, ok := adblockLine(line); ok {
			if site == "" {
				f.Skipped = append(f.Skipped, line)
			} else {
				entry(site)
			}
			continue
		}
		if strings.HasPrefix(line, "[") {
			// A phrase: [a, b] [c]. Read loosely, so one today's rules
			// refuse is still taken for a phrase and named by the import.
			if p, err := db.ParsePhraseLoose(line); err == nil && len(p.Sets) >= 2 {
				if key := "phrase " + p.Key(); !seen[key] {
					seen[key] = true
					f.Phrases = append(f.Phrases, line)
				}
			} else {
				f.Skipped = append(f.Skipped, line)
			}
			continue
		}
		if urlEntry(line) {
			line = normalizeURLEntry(line)
		}
		entry(line)
	}
	return f
}

// describeEntryFile says what a file holds, e.g. "3 phrases, 12 sites".
func describeEntryFile(f entryFile) string {
	var parts []string
	for _, c := range []struct {
		n          int
		one, other string
	}{{len(f.Phrases), "phrase", "phrases"}, {len(f.Sites), "site", "sites"}, {len(f.Terms), "rendered term", "rendered terms"},
		{len(f.Skipped), "line KC9K can't use", "lines KC9K can't use"}} {
		if c.n == 1 {
			parts = append(parts, "1 "+c.one)
		} else if c.n > 1 {
			parts = append(parts, fmt.Sprintf("%d %s", c.n, c.other))
		}
	}
	if len(parts) == 0 {
		return "nothing"
	}
	return strings.Join(parts, ", ")
}

// listReadCmd tells the interface what a file holds before it imports it,
// so one Import button takes any of the layouts (owner, 2026-11-16).
var listReadCmd = &cobra.Command{
	Use:    "read [file]",
	Short:  "Show what a file to import holds: phrases, sites, rendered terms (JSON)",
	Hidden: true,
	Args:   cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f, err := readEntries(args[0])
		if err != nil {
			return err
		}
		data, err := json.Marshal(f)
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	},
}

func init() {
	listCmd.AddCommand(listReadCmd)
}
