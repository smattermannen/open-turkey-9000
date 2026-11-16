// breaks.go — breaks a block allows, set before it is locked.
//
// A reward earns break time while the block is on: Allowance seconds for
// every Per seconds of blocking (Balance, in milliseconds so small steps add
// up), kept up to one Allowance, until a period ends, or each part for a
// length after it was earned (Keep). A break is taken when the user chooses
// and lifts the block as a pause (pauses.go).
//
// Relock makes unlocking a break too: passing the block's challenge or
// giving its password lifts it for Relock seconds, after which it is back
// with its locks. Unlocking again during that break ends the block.
package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strconv"
)

const breaksSchema = `CREATE TABLE IF NOT EXISTS block_breaks (
    block_id   INTEGER PRIMARY KEY,
    allowance  INTEGER NOT NULL DEFAULT 0,
    per        INTEGER NOT NULL DEFAULT 0,
    balance_ms INTEGER NOT NULL DEFAULT 0,
    relock     INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (block_id) REFERENCES blocks(id) ON DELETE CASCADE
);`

// Limits on breaks.
const (
	MaxBreakSeconds = 4 * 3600
	MaxBreakPer     = 24 * 3600
)

// Breaks is a block's break settings.
type Breaks struct {
	Allowance int64 // seconds earned per Per seconds of blocking; 0: none
	Per       int64
	BalanceMs int64 // earned and not yet taken
	Relock    int64 // seconds an unlock lifts the block for; 0: unlocking ends it
	// Delay break (§3.6): asking for it starts a countdown of DelayWait,
	// then the block is lifted for DelayLength; it stays locked. DelayAt is
	// when an asked-for break begins (trusted unix), 0 when none is asked.
	DelayLength int64
	DelayWait   int64
	DelayAt     int64
	// Keep (§3.6 reward): "" saves up to one allowance; "window", "day",
	// "week" or "month" lets reward accumulate until that period ends; a
	// number of seconds (KeepSeconds) keeps each part of it that long after
	// it was earned, with no upper limit. Period names the period the
	// balance was earned in.
	Keep   string
	Period string
	// Chunks: for a reward kept for a length, what was earned in each hour,
	// as JSON [[hour, ms], …], oldest first (hour: trusted unix / 3600);
	// together they are BalanceMs. OnBreak: the parts the running break
	// took, so that ending it early gives back what is left at its own age.
	Chunks  string
	OnBreak string
}

// RewardKeeps are the periods a reward can accumulate over, shortest first.
var RewardKeeps = []string{"window", "day", "week", "month"}

// KeepSeconds is how long a reward kept for a length keeps each part of
// it; 0 for "" and the periods.
func KeepSeconds(keep string) int64 {
	n, err := strconv.ParseInt(keep, 10, 64)
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

// ValidKeep reports whether keep is a way to keep reward.
func ValidKeep(keep string) bool {
	return keep == "" || KeepSeconds(keep) >= 3600 || slices.Contains(RewardKeeps, keep)
}

// keepRank orders the periods: longer is looser.
func keepRank(keep string) int {
	for i, k := range RewardKeeps {
		if k == keep {
			return i + 1
		}
	}
	return 0
}

// KeepLooser reports whether keeping reward as keep may keep more than
// old: up to one allowance is the strictest; a longer period or a longer
// length is looser; a period and a length can't be compared (a part earned
// late in a week is kept longer by "3d" than by "week"), so a change
// between them counts as looser.
func KeepLooser(keep, old string) bool {
	switch {
	case keep == old || keep == "":
		return false
	case old == "":
		return true
	case KeepSeconds(keep) > 0 && KeepSeconds(old) > 0:
		return KeepSeconds(keep) > KeepSeconds(old)
	case KeepSeconds(keep) == 0 && KeepSeconds(old) == 0:
		return keepRank(keep) > keepRank(old)
	}
	return true
}

// rewardChunk is what one hour earned: [hour, ms].
type rewardChunk [2]int64

func decodeChunks(s string) []rewardChunk {
	var chunks []rewardChunk
	if s != "" {
		_ = json.Unmarshal([]byte(s), &chunks)
	}
	return chunks
}

func encodeChunks(chunks []rewardChunk) string {
	var kept []rewardChunk
	for _, c := range chunks {
		if c[1] > 0 {
			kept = append(kept, c)
		}
	}
	if len(kept) == 0 {
		return ""
	}
	out, _ := json.Marshal(kept)
	return string(out)
}

func chunksMs(chunks []rewardChunk) int64 {
	var ms int64
	for _, c := range chunks {
		ms += c[1]
	}
	return ms
}

// Expire drops what the reward no longer keeps at now (trusted unix): for
// a period, the whole balance once period (the current period's name)
// differs from the one it was earned in; for a length, each hour's part
// that length after the end of its hour.
func (b Breaks) Expire(period string, now int64) Breaks {
	if keep := KeepSeconds(b.Keep); keep > 0 {
		var kept []rewardChunk
		for _, c := range decodeChunks(b.Chunks) {
			if (c[0]+1)*3600+keep > now {
				kept = append(kept, c)
			}
		}
		if chunks := encodeChunks(kept); chunks != b.Chunks {
			b.Chunks, b.BalanceMs = chunks, chunksMs(kept)
		}
		return b
	}
	if b.Keep == "" || b.Period == period {
		return b
	}
	b.BalanceMs, b.Period = 0, period
	return b
}

// Balance returns the break time available, in whole seconds.
func (b Breaks) Balance() int64 { return b.BalanceMs / 1000 }

// Earn adds what seconds of blocking earn at now (trusted unix): up to one
// allowance, or without limit for a reward that accumulates.
func (b Breaks) Earn(seconds, now int64) Breaks {
	if b.Allowance <= 0 || b.Per <= 0 || seconds <= 0 {
		return b
	}
	add := seconds * 1000 * b.Allowance / b.Per
	if b.Keep == "" {
		b.BalanceMs = min(b.BalanceMs+add, b.Allowance*1000)
		return b
	}
	b.BalanceMs += add
	if KeepSeconds(b.Keep) > 0 {
		chunks := decodeChunks(b.Chunks)
		if n, hour := len(chunks), now/3600; n > 0 && chunks[n-1][0] == hour {
			chunks[n-1][1] += add
		} else {
			chunks = append(chunks, rewardChunk{hour, add})
		}
		b.Chunks = encodeChunks(chunks)
	}
	return b
}

// Take removes ms of balance for a break, the oldest reward first; for a
// reward kept for a length, the parts taken are noted in OnBreak.
func (b Breaks) Take(ms int64) Breaks {
	ms = min(ms, b.BalanceMs)
	b.BalanceMs -= ms
	b.OnBreak = ""
	if KeepSeconds(b.Keep) == 0 {
		return b
	}
	chunks := decodeChunks(b.Chunks)
	var taken []rewardChunk
	for ms > 0 && len(chunks) > 0 {
		n := min(ms, chunks[0][1])
		taken = append(taken, rewardChunk{chunks[0][0], n})
		chunks[0][1] -= n
		ms -= n
		if chunks[0][1] == 0 {
			chunks = chunks[1:]
		}
	}
	b.Chunks, b.OnBreak = encodeChunks(chunks), encodeChunks(taken)
	return b
}

// GiveBack returns ms of a break ended early: up to one allowance for
// reward saved up to one; for a reward kept for a length, the newest of
// the parts the break took, each at its own age (what the break used came
// from the oldest).
func (b Breaks) GiveBack(ms int64) Breaks {
	switch {
	case b.Allowance <= 0 || ms <= 0:
	case b.Keep == "":
		b.BalanceMs = min(b.BalanceMs+ms, b.Allowance*1000)
	case KeepSeconds(b.Keep) == 0:
		b.BalanceMs += ms
	default:
		taken := decodeChunks(b.OnBreak)
		chunks := decodeChunks(b.Chunks)
		for i := len(taken) - 1; i >= 0 && ms > 0; i-- {
			n := min(ms, taken[i][1])
			chunks = append(chunks, rewardChunk{taken[i][0], n})
			ms -= n
		}
		sort.SliceStable(chunks, func(i, j int) bool { return chunks[i][0] < chunks[j][0] })
		var merged []rewardChunk
		for _, c := range chunks {
			if n := len(merged); n > 0 && merged[n-1][0] == c[0] {
				merged[n-1][1] += c[1]
			} else {
				merged = append(merged, c)
			}
		}
		b.Chunks, b.BalanceMs = encodeChunks(merged), chunksMs(merged)
	}
	b.OnBreak = ""
	return b
}

// WithKeep changes how reward is kept, keeping what is saved: capped at
// one allowance for "", as one part earned at now (trusted unix) for a
// length, in period (the current one) for a period.
func (b Breaks) WithKeep(keep, period string, now int64) Breaks {
	if keep == b.Keep {
		return b
	}
	b.Keep, b.Chunks, b.OnBreak, b.Period = keep, "", "", ""
	switch {
	case keep == "":
		b.BalanceMs = min(b.BalanceMs, b.Allowance*1000)
	case KeepSeconds(keep) > 0:
		if b.BalanceMs > 0 {
			b.Chunks = encodeChunks([]rewardChunk{{now / 3600, b.BalanceMs}})
		}
	default:
		b.Period = period
	}
	return b
}

// LooserThan reports whether b allows more breaks than old.
func (b Breaks) LooserThan(old Breaks) bool {
	if b.Relock > old.Relock {
		return true
	}
	if b.DelayLength > old.DelayLength || (b.DelayLength > 0 && (old.DelayLength == 0 || b.DelayWait < old.DelayWait)) {
		return true
	}
	if b.Allowance > 0 && KeepLooser(b.Keep, old.Keep) {
		return true
	}
	if b.Allowance == 0 {
		return false
	}
	// More time per hour of blocking, or more at once.
	return old.Allowance == 0 || b.Allowance > old.Allowance || b.Allowance*old.Per > old.Allowance*b.Per
}

// GetBreaks returns a block's break settings (zero when it has none).
func (d *DB) GetBreaks(name string) (Breaks, error) {
	var b Breaks
	err := d.conn.QueryRow(`SELECT k.allowance, k.per, k.balance_ms, k.relock, k.delay_length, k.delay_wait, k.delay_at, k.keep, k.period,
		k.chunks, k.on_break FROM block_breaks k
		JOIN blocks b ON b.id = k.block_id WHERE b.name = ?`, name).Scan(&b.Allowance, &b.Per, &b.BalanceMs, &b.Relock,
		&b.DelayLength, &b.DelayWait, &b.DelayAt, &b.Keep, &b.Period, &b.Chunks, &b.OnBreak)
	if err == sql.ErrNoRows {
		return Breaks{}, nil
	}
	if err != nil {
		return Breaks{}, fmt.Errorf("failed to read the breaks of '%s': %w", name, err)
	}
	return b, nil
}

// SetBreaks saves a block's break settings.
func (d *DB) SetBreaks(name string, b Breaks) error {
	blockID, err := d.getBlockID(name)
	if err != nil {
		return err
	}
	if b.Allowance == 0 {
		b.Per, b.BalanceMs, b.Chunks, b.OnBreak = 0, 0, "", ""
	}
	if b.DelayLength == 0 {
		b.DelayWait, b.DelayAt = 0, 0
	}
	if b.Allowance == 0 && b.Relock == 0 && b.DelayLength == 0 {
		_, err = d.conn.Exec("DELETE FROM block_breaks WHERE block_id = ?", blockID)
	} else {
		_, err = d.conn.Exec(`INSERT INTO block_breaks (block_id, allowance, per, balance_ms, relock, delay_length, delay_wait, delay_at, keep, period, chunks, on_break)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(block_id) DO UPDATE SET allowance = excluded.allowance, per = excluded.per,
			balance_ms = excluded.balance_ms, relock = excluded.relock, delay_length = excluded.delay_length,
			delay_wait = excluded.delay_wait, delay_at = excluded.delay_at, keep = excluded.keep, period = excluded.period,
			chunks = excluded.chunks, on_break = excluded.on_break`,
			blockID, b.Allowance, b.Per, b.BalanceMs, b.Relock, b.DelayLength, b.DelayWait, b.DelayAt, b.Keep, b.Period, b.Chunks, b.OnBreak)
	}
	if err != nil {
		return fmt.Errorf("failed to save the breaks of '%s': %w", name, err)
	}
	return nil
}
