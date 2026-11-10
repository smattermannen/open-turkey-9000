// pagekeywords.go — the page keyword tables from before website lists, read
// once by migrateWebLists, and the thresholds they used.
package db

const pageKeywordsSchema = `CREATE TABLE IF NOT EXISTS page_keywords (
    id       INTEGER PRIMARY KEY AUTOINCREMENT,
    block_id INTEGER NOT NULL,
    keyword  TEXT NOT NULL,
    FOREIGN KEY (block_id) REFERENCES blocks(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS keyword_thresholds (
    block_id INTEGER NOT NULL,
    action   TEXT NOT NULL,
    count    INTEGER NOT NULL,
    PRIMARY KEY (block_id, action),
    FOREIGN KEY (block_id) REFERENCES blocks(id) ON DELETE CASCADE
);`

// MaxThreshold caps a keyword threshold; a block can't have more keywords
// than this be useful anyway.
const MaxThreshold = 1000

// Thresholds holds, per website action, how many of a block's distinct page
// keywords a page must contain before the action applies to it.
type Thresholds struct {
	Block     int `json:"block"`
	Greyscale int `json:"greyscale"`
	NoImages  int `json:"no-images"`
}

// DefaultThresholds act on the first keyword found.
var DefaultThresholds = Thresholds{Block: 1, Greyscale: 1, NoImages: 1}

// Get returns the threshold for the named action.
func (t Thresholds) Get(action string) int {
	switch action {
	case ActionBlock:
		return t.Block
	case ActionGreyscale:
		return t.Greyscale
	case ActionNoImages:
		return t.NoImages
	}
	return 0
}

// With returns a copy with the named action's threshold set to n.
func (t Thresholds) With(action string, n int) Thresholds {
	switch action {
	case ActionBlock:
		t.Block = n
	case ActionGreyscale:
		t.Greyscale = n
	case ActionNoImages:
		t.NoImages = n
	}
	return t
}
