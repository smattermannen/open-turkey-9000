// words.go — word recording (§3.12). While a word recording is on, the
// extension reports the rendered visible words of each page opened, with
// how often each shows; they are kept in recorded_words. The baseline
// (word_baseline) counts, per word, the pages of normal browsing it showed
// on: only word counts, no addresses, no private windows. Specificity
// compares the two and ranks the recorded words.
package db

import (
	"fmt"
	"math"
	"sort"
	"strconv"
)

const wordsSchema = `CREATE TABLE IF NOT EXISTS recorded_words (
    word  TEXT PRIMARY KEY,
    count INTEGER NOT NULL DEFAULT 0,
    pages INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS word_baseline (
    word  TEXT PRIMARY KEY,
    pages INTEGER NOT NULL DEFAULT 0
);`

const (
	// SettingRecordedPages counts the pages of the word recording.
	SettingRecordedPages = "recording.pages"
	// SettingWordBaseline is "on" while the baseline is gathered.
	SettingWordBaseline = "words.baseline"
	// SettingBaselinePages counts the baseline's pages.
	SettingBaselinePages = "words.baseline.pages"
	// MaxRecordedWords and MaxBaselineWords bound the two tables.
	MaxRecordedWords = 20000
	MaxBaselineWords = 50000
	// MaxPageWords bounds the words one page report adds.
	MaxPageWords = 400
)

// RecordedWord is one word of a word recording, ranked by Specificity.
type RecordedWord struct {
	Word        string
	Count       int64 // times shown, over all recorded pages
	Pages       int64 // recorded pages it showed on
	Specificity float64
}

func (d *DB) bump(key string) error {
	_, err := d.conn.Exec(`INSERT INTO settings (key, value) VALUES (?, '1')
		ON CONFLICT(key) DO UPDATE SET value = CAST(CAST(value AS INTEGER) + 1 AS TEXT)`, key)
	return err
}

func (d *DB) counter(key string) int64 {
	v, _ := d.GetSetting(key)
	n, _ := strconv.ParseInt(v, 10, 64)
	return n
}

// AddRecordedPage adds one page's words (word → times shown).
func (d *DB) AddRecordedPage(words map[string]int) error {
	if err := d.bump(SettingRecordedPages); err != nil {
		return err
	}
	return d.addWords(`UPDATE recorded_words SET count = count + ?2, pages = pages + 1 WHERE word = ?1`,
		`INSERT INTO recorded_words (word, count, pages) SELECT ?1, ?2, 1
			WHERE (SELECT COUNT(*) FROM recorded_words) < ?3`, MaxRecordedWords, words)
}

// addWords runs update for each word, and insert (bounded by limit) for
// the words not there yet, in one transaction.
func (d *DB) addWords(update, insert string, limit int, words map[string]int) error {
	tx, err := d.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for w, n := range words {
		res, err := tx.Exec(update, w, n)
		if err != nil {
			return fmt.Errorf("failed to add words: %w", err)
		}
		if changed, _ := res.RowsAffected(); changed == 0 {
			if _, err := tx.Exec(insert, w, n, limit); err != nil {
				return fmt.Errorf("failed to add words: %w", err)
			}
		}
	}
	return tx.Commit()
}

// AddBaselinePage adds one page of normal browsing to the baseline.
func (d *DB) AddBaselinePage(words map[string]int) error {
	if err := d.bump(SettingBaselinePages); err != nil {
		return err
	}
	return d.addWords(`UPDATE word_baseline SET pages = pages + 1 WHERE word = ?1 AND ?2 IS NOT NULL`,
		`INSERT INTO word_baseline (word, pages) SELECT ?1, 1
			WHERE ?2 IS NOT NULL AND (SELECT COUNT(*) FROM word_baseline) < ?3`, MaxBaselineWords, words)
}

// WordBaselineOn reports whether the baseline is gathered.
func (d *DB) WordBaselineOn() bool {
	v, _ := d.GetSetting(SettingWordBaseline)
	return v == "on"
}

// SetWordBaseline turns gathering the baseline on or off; off forgets it.
func (d *DB) SetWordBaseline(on bool) error {
	if on {
		return d.SetSetting(SettingWordBaseline, "on")
	}
	if _, err := d.conn.Exec("DELETE FROM word_baseline"); err != nil {
		return err
	}
	if err := d.SetSetting(SettingBaselinePages, "0"); err != nil {
		return err
	}
	return d.SetSetting(SettingWordBaseline, "")
}

// BaselinePages returns how many pages the baseline has.
func (d *DB) BaselinePages() int64 { return d.counter(SettingBaselinePages) }

// RecordedPages returns how many pages the word recording has.
func (d *DB) RecordedPages() int64 { return d.counter(SettingRecordedPages) }

// Specificity is how much more common a word is on recorded pages than in
// normal browsing: the share of recorded pages it showed on over its share
// of baseline pages, both smoothed (add one), so words never seen in normal
// browsing rank high and words on every page rank near 1 or below.
func Specificity(pages, recorded, basePages, baseline int64) float64 {
	return (float64(pages) + 1) / (float64(recorded) + 2) / ((float64(basePages) + 1) / (float64(baseline) + 2))
}

// RecordedWords returns the recorded words, most specific first (ties:
// shown more often first, then alphabetically).
func (d *DB) RecordedWords() ([]RecordedWord, error) {
	rows, err := d.conn.Query(`SELECT r.word, r.count, r.pages, COALESCE(b.pages, 0)
		FROM recorded_words r LEFT JOIN word_baseline b ON b.word = r.word`)
	if err != nil {
		return nil, fmt.Errorf("failed to read the recorded words: %w", err)
	}
	defer rows.Close()
	recorded, baseline := d.RecordedPages(), d.BaselinePages()
	var out []RecordedWord
	for rows.Next() {
		var w RecordedWord
		var base int64
		if err := rows.Scan(&w.Word, &w.Count, &w.Pages, &base); err != nil {
			return nil, err
		}
		w.Specificity = Specificity(w.Pages, recorded, base, baseline)
		out = append(out, w)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if math.Abs(a.Specificity-b.Specificity) > 1e-9 {
			return a.Specificity > b.Specificity
		}
		if a.Count != b.Count {
			return a.Count > b.Count
		}
		return a.Word < b.Word
	})
	return out, rows.Err()
}

// CutWords drops the lowest percent (0–99) of ranked words (§3.12 cut).
func CutWords(words []RecordedWord, percent int) []RecordedWord {
	if percent <= 0 {
		return words
	}
	keep := len(words) - len(words)*percent/100
	return words[:keep]
}
