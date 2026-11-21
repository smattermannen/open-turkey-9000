package blocker

import (
	"strings"
	"time"
)

// TitleCloseGrace is how long a window asked to close may stay before its
// program is killed.
const TitleCloseGrace = 4 * time.Second

// matchingKeyword returns the keyword title contains (case-insensitive), or "".
func matchingKeyword(title string, keywords []string) string {
	lower := strings.ToLower(title)
	for _, k := range keywords {
		if k != "" && strings.Contains(lower, strings.ToLower(k)) {
			return k
		}
	}
	return ""
}
