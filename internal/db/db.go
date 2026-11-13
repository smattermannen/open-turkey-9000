// Package db provides the SQLite database access layer for Killer Chicken 9000.
//
// We use the "modernc.org/sqlite" driver because it's a pure Go
// implementation, meaning it needs neither CGo nor C libraries installed on
// the system. That makes cross-compiling and distributing the final binary
// easier.
package db

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	// Import the pure-Go SQLite driver. The underscore (_) in front means we
	// won't call functions from this package directly — it registers itself
	// as a SQL driver when imported. It's a common pattern in Go.
	_ "modernc.org/sqlite"
)

// DefaultDBPath is the default path where the database is stored.
// We use /var/lib/ because it's the standard Linux directory for persistent
// application data (following the Filesystem Hierarchy Standard — FHS).
const DefaultDBPath = "/var/lib/kc9k/kc9k.db"

// dbPathEnv, when set by a non-root process, overrides the database path. Root
// always uses DefaultDBPath, so the override can never redirect enforcement. This exists mainly so the
// CLI and tests can run against a scratch database without root. The installed
// systemd unit never sets it, so the daemon always uses DefaultDBPath in
// production — a user can't point the daemon at an empty database without
// already having root (in which case the whole tool is bypassable anyway).
const dbPathEnv = "KC9K_DB"

// ResolveDBPath returns the database path to use: the KC9K_DB override
// if set, otherwise DefaultDBPath.
func ResolveDBPath() string {
	if p := os.Getenv(dbPathEnv); p != "" && os.Geteuid() != 0 {
		return p
	}
	return DefaultDBPath
}

// --------------------------------------------------------------------------
// Types (structs) — represent the data flowing between the database and the app
// --------------------------------------------------------------------------

// DB wraps the database connection.
// We wrap it instead of exposing *sql.DB directly to control which operations
// are allowed and keep the public interface clean.
type DB struct {
	conn *sql.DB
}

// Block modes from before website lists, read once by migrateWebLists. An
// allowlist block blocked everything except its sites; it became a "*.*"
// blacklist and an exception list.
const (
	ModeBlocklist = "blocklist"
	ModeAllowlist = "allowlist"
)

// Block represents a block in the general listing.
// EntryCount (blacklist entries) and AppCount avoid loading every list just
// to count them.
type Block struct {
	ID         int
	Name       string
	EntryCount int
	AppCount   int
	CreatedAt  string
}

// BlockDetail carries all the information about a specific block,
// including the full lists of blocked sites and apps.
type BlockDetail struct {
	ID          int
	Name        string
	Lists       Lists // website blacklists and exception lists
	Apps        []string
	Titles      []string // window-title keywords
	Social      []string // social media settings (social.go)
	Active      bool
	Locked      bool
	LockChars   int
	LockedUntil int64 // unix seconds; 0 means no time lock
	Scheduled   bool  // activated automatically by a schedule
	CreatedAt   string
}

// ActiveBlockDetail represents a block that is currently active.
// The daemon uses it to know what to block.
type ActiveBlockDetail struct {
	BlockName   string
	Lists       Lists
	Apps        []string
	Titles      []string
	Social      []string
	Locked      bool
	LockChars   int
	LockedUntil int64
	Scheduled   bool
}

// --------------------------------------------------------------------------
// Migrations (database schema)
// --------------------------------------------------------------------------

// migrations contains the SQL that creates the database tables.
// We put everything in a single string because it's simple and direct.
//
// Schema design decisions:
//   - blocks: the central table. The "name" field is UNIQUE to avoid duplicate blocks.
//   - sites: domains associated with a block. ON DELETE CASCADE ensures that
//     when a block is deleted, its sites are removed automatically.
//   - apps: processes associated with a block. Same CASCADE logic.
//   - active_blocks: records which blocks are currently active.
//     The "locked" field indicates whether the user locked the block (can't
//     be easily deactivated).
//     The "lock_chars" field stores how many random characters the user must
//     type to unlock (a "friction" mechanism to prevent impulsive
//     deactivation). block_id is UNIQUE because a block can only be active
//     once.
const migrations = `
CREATE TABLE IF NOT EXISTS blocks (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT UNIQUE NOT NULL,
    mode       TEXT NOT NULL DEFAULT 'blocklist',
    act_block     BOOLEAN NOT NULL DEFAULT 1,
    act_greyscale BOOLEAN NOT NULL DEFAULT 0,
    act_no_images BOOLEAN NOT NULL DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS sites (
    id       INTEGER PRIMARY KEY AUTOINCREMENT,
    block_id INTEGER NOT NULL,
    domain   TEXT NOT NULL,
    FOREIGN KEY (block_id) REFERENCES blocks(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS apps (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    block_id     INTEGER NOT NULL,
    process_name TEXT NOT NULL,
    FOREIGN KEY (block_id) REFERENCES blocks(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS patterns (
    id       INTEGER PRIMARY KEY AUTOINCREMENT,
    block_id INTEGER NOT NULL,
    pattern  TEXT NOT NULL,
    FOREIGN KEY (block_id) REFERENCES blocks(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS schedules (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    block_id  INTEGER NOT NULL,
    days_mask INTEGER NOT NULL,
    start_min INTEGER NOT NULL,
    end_min   INTEGER NOT NULL,
    allow     BOOLEAN NOT NULL DEFAULT 0,
    FOREIGN KEY (block_id) REFERENCES blocks(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS pending_edits (
    id       INTEGER PRIMARY KEY AUTOINCREMENT,
    block_id INTEGER NOT NULL,
    action   TEXT NOT NULL,
    payload  TEXT NOT NULL,
    apply_at INTEGER NOT NULL,
    FOREIGN KEY (block_id) REFERENCES blocks(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS active_blocks (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    block_id     INTEGER UNIQUE NOT NULL,
    locked       BOOLEAN NOT NULL DEFAULT 0,
    lock_chars   INTEGER NOT NULL DEFAULT 0,
    locked_until INTEGER NOT NULL DEFAULT 0,
    scheduled    BOOLEAN NOT NULL DEFAULT 0,
    activated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (block_id) REFERENCES blocks(id) ON DELETE CASCADE
);
`

// --------------------------------------------------------------------------
// Opening and closing the database
// --------------------------------------------------------------------------

// OpenDB opens (or creates) the SQLite database at the given path.
//
// Important steps that happen here:
//  1. Open the connection using the "sqlite" driver (registered by modernc.org/sqlite).
//  2. Enable foreign keys — by default SQLite does NOT enforce foreign keys!
//     Without this PRAGMA, ON DELETE CASCADE wouldn't work.
//  3. Enable WAL mode (Write-Ahead Logging) — this allows concurrent reads and
//     writes without locking. It's much better than the default (journal) mode
//     for applications that read and write at the same time (e.g. daemon + CLI).
//  4. Run the migrations to ensure the tables exist.
func OpenDB(dbPath string) (*DB, error) {
	// "sqlite" is the name of the driver registered by modernc.org/sqlite.
	// dbPath is the path of the .db file on disk.
	//
	// Every connection in the pool gets these settings (the driver applies
	// _pragma on each new one; a plain PRAGMA would reach only one):
	//   - busy_timeout: wait up to 5 s for another writer (the daemon, or a
	//     second kc9k command) instead of failing with "database is locked".
	//   - foreign_keys: SQLite ignores FOREIGN KEY, and so CASCADE, without it.
	conn, err := sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("failed to open database at '%s': %w", dbPath, err)
	}

	// Check that the connection actually works.
	// sql.Open() doesn't really connect — it only validates the parameters.
	// Ping() forces a real connection.
	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// WAL (Write-Ahead Logging) lets reads happen while a write is in
	// progress. In the default (journal) mode, the database locks completely
	// during writes.
	if _, err := conn.Exec("PRAGMA journal_mode = WAL"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	// Run the migrations to create the tables if they don't exist.
	// "IF NOT EXISTS" makes running this several times safe (idempotent).
	if _, err := conn.Exec(migrations); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to create database tables: %w", err)
	}

	// Bring older databases (created before newer columns existed) up to date.
	if _, err := conn.Exec(webListsSchema + clustersSchema + recordingSchema + eventsSchema + nepsisSchema + clockSchema + freezeSchema + quotaSchema + exceptionsSchema + settingsSchema + settingLocksSchema + freezeScheduleSchema + titlesSchema + pageKeywordsSchema + hotkeysSchema + pomodoroSchema + usageSchema + pausesSchema + breaksSchema + socialSchema + placesSchema + phrasesSchema); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to create clock table: %w", err)
	}
	if err := upgradeSchema(conn); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to upgrade database schema: %w", err)
	}

	return &DB{conn: conn}, nil
}

// Close closes the database connection.
// Always call Close() when you're done using the database (defer db.Close()).
func (d *DB) Close() error {
	if err := d.conn.Close(); err != nil {
		return fmt.Errorf("failed to close database: %w", err)
	}
	return nil
}

// --------------------------------------------------------------------------
// Block CRUD
// --------------------------------------------------------------------------

// CreateBlock creates a new block with an empty blacklist ("Blocked") and
// exception list ("Exceptions"), both looking at addresses.
// The name must be unique — if it already exists, the database returns an error.
func (d *DB) CreateBlock(name string) error {
	tx, err := d.conn.Begin()
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()
	res, err := tx.Exec("INSERT INTO blocks (name, lists_ready) VALUES (?, 1)", name)
	if err != nil {
		// If it's a UNIQUE constraint error, give a clearer message.
		if strings.Contains(err.Error(), "UNIQUE") {
			return fmt.Errorf("a block named '%s' already exists", name)
		}
		return fmt.Errorf("failed to create block '%s': %w", name, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	if err := createDefaultLists(tx, int(id)); err != nil {
		return err
	}
	return tx.Commit()
}

// AddApps adds a list of process names to an existing block.
// The logic is identical to AddSites — we use a transaction for the same reason.
func (d *DB) AddApps(blockName string, processNames []string) error {
	blockID, err := d.getBlockID(blockName)
	if err != nil {
		return err
	}

	tx, err := d.conn.Begin()
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare("INSERT INTO apps (block_id, process_name) VALUES (?, ?)")
	if err != nil {
		return fmt.Errorf("failed to prepare app insert: %w", err)
	}
	defer stmt.Close()

	for _, name := range processNames {
		if _, err := stmt.Exec(blockID, name); err != nil {
			return fmt.Errorf("failed to add process '%s' to block '%s': %w", name, blockName, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to save apps for block '%s': %w", blockName, err)
	}

	return nil
}

// RemoveApps removes a list of process names from an existing block.
//
// If any process doesn't exist in the block, it returns an error and the
// whole removal is rolled back.
func (d *DB) RemoveApps(blockName string, processNames []string) error {
	blockID, err := d.getBlockID(blockName)
	if err != nil {
		return err
	}

	tx, err := d.conn.Begin()
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare("DELETE FROM apps WHERE block_id = ? AND process_name = ?")
	if err != nil {
		return fmt.Errorf("failed to prepare app removal: %w", err)
	}
	defer stmt.Close()

	for _, name := range processNames {
		result, err := stmt.Exec(blockID, name)
		if err != nil {
			return fmt.Errorf("failed to remove process '%s' from block '%s': %w", name, blockName, err)
		}
		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("failed to verify removal of process '%s': %w", name, err)
		}
		if rowsAffected == 0 {
			return fmt.Errorf("process '%s' not found in block '%s'", name, blockName)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to save app removal for block '%s': %w", blockName, err)
	}

	return nil
}

// ListBlocks returns every registered block with its site and app counts.
//
// We use LEFT JOIN + COUNT(DISTINCT ...) to fetch everything in a single query.
// LEFT JOIN ensures blocks with no sites or apps also appear (with count 0).
// DISTINCT avoids double counting when there are multiple joins.
func (d *DB) ListBlocks() ([]Block, error) {
	query := `
		SELECT
			b.id,
			b.name,
			(SELECT COUNT(*) FROM web_list_entries e JOIN web_lists l ON l.id = e.list_id
				WHERE l.block_id = b.id AND l.kind = 'blacklist') AS entry_count,
			COUNT(DISTINCT a.id) AS app_count,
			b.created_at
		FROM blocks b
		LEFT JOIN apps  a ON a.block_id = b.id
		GROUP BY b.id
		ORDER BY b.name
	`

	rows, err := d.conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list blocks: %w", err)
	}
	// defer rows.Close() is required to release the cursor's resources.
	// Without it, the connection can get "stuck" and cause deadlocks.
	defer rows.Close()

	var blocks []Block
	for rows.Next() {
		var b Block
		if err := rows.Scan(&b.ID, &b.Name, &b.EntryCount, &b.AppCount, &b.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to read block data: %w", err)
		}
		blocks = append(blocks, b)
	}

	// rows.Err() checks whether any error occurred during iteration.
	// It's different from the error returned by rows.Next() — it covers
	// network errors, etc.
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate blocks: %w", err)
	}

	return blocks, nil
}

// GetBlock returns the full details of a block: sites, apps, activation state.
func (d *DB) GetBlock(name string) (*BlockDetail, error) {
	// Fetch the block's basic data + activation info (if any).
	// LEFT JOIN with active_blocks because the block may not be active.
	var detail BlockDetail
	err := d.conn.QueryRow(`
		SELECT
			b.id,
			b.name,
			b.created_at,
			CASE WHEN ab.id IS NOT NULL THEN 1 ELSE 0 END AS active,
			COALESCE(ab.locked, 0) AS locked,
			COALESCE(ab.lock_chars, 0) AS lock_chars,
			COALESCE(ab.locked_until, 0) AS locked_until,
			COALESCE(ab.scheduled, 0) AS scheduled
		FROM blocks b
		LEFT JOIN active_blocks ab ON ab.block_id = b.id
		WHERE b.name = ?
	`, name).Scan(
		&detail.ID,
		&detail.Name,
		&detail.CreatedAt,
		&detail.Active,
		&detail.Locked,
		&detail.LockChars,
		&detail.LockedUntil,
		&detail.Scheduled,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("block '%s' not found", name)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to fetch block '%s': %w", name, err)
	}

	if detail.Lists, err = getBlockLists(d.conn, detail.ID); err != nil {
		return nil, err
	}

	// Fetch the block's apps.
	detail.Apps, err = d.getBlockApps(detail.ID)
	if err != nil {
		return nil, err
	}

	detail.Titles, err = d.getBlockTitles(detail.ID)
	if err != nil {
		return nil, err
	}
	if detail.Social, err = d.getBlockSocial(detail.ID); err != nil {
		return nil, err
	}

	return &detail, nil
}

// RemoveBlock deletes a block by name.
// If the block is active, removal is prevented — the user must deactivate it
// before removing it. This avoids accidental removal while the block is in
// effect.
func (d *DB) RemoveBlock(name string) error {
	// Check whether the block is active before trying to remove it.
	active, err := d.IsBlockActive(name)
	if err != nil {
		return err
	}
	if active {
		return fmt.Errorf("cannot remove block '%s' because it is active. Deactivate it first", name)
	}

	result, err := d.conn.Exec("DELETE FROM blocks WHERE name = ?", name)
	if err != nil {
		return fmt.Errorf("failed to remove block '%s': %w", name, err)
	}

	// Check whether any row was actually deleted.
	// If RowsAffected() == 0, the block didn't exist.
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to verify removal of block '%s': %w", name, err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("block '%s' not found", name)
	}

	return nil
}

// --------------------------------------------------------------------------
// Block activation and deactivation
// --------------------------------------------------------------------------

// ActivateBlock marks a block as active. This makes the sites and apps in
// that block start being blocked by the daemon.
//
// Parameters:
//   - locked: if true, the block can't be deactivated via "stop" (needs the
//     typing challenge via "unlock").
//   - lockChars: how many random characters the challenge requires.
//   - lockedUntil: unix seconds before which the block cannot be removed at all
//     (a time lock); 0 means none.
//   - scheduled: true when the daemon activated this block from a schedule, so
//     the daemon may deactivate it again when the window ends.
func (d *DB) ActivateBlock(name string, locked bool, lockChars int, lockedUntil int64, scheduled bool) error {
	blockID, err := d.getBlockID(name)
	if err != nil {
		return err
	}

	_, err = d.conn.Exec(
		"INSERT INTO active_blocks (block_id, locked, lock_chars, locked_until, scheduled) VALUES (?, ?, ?, ?, ?)",
		blockID, locked, lockChars, lockedUntil, scheduled,
	)
	if err != nil {
		// If the block is already active, the UNIQUE block_id will cause an error.
		if strings.Contains(err.Error(), "UNIQUE") {
			return fmt.Errorf("block '%s' is already active", name)
		}
		return fmt.Errorf("failed to activate block '%s': %w", name, err)
	}

	return nil
}

// StrengthenLocks updates an active block's locks in place (the "scheduled"
// flag is kept). The caller (start on an active block) only ever passes values
// at least as strong as the current ones: a char lock can be added or
// lengthened, and a time lock added or extended, but never the reverse.
func (d *DB) StrengthenLocks(name string, locked bool, lockChars int, lockedUntil int64) error {
	blockID, err := d.getBlockID(name)
	if err != nil {
		return err
	}
	result, err := d.conn.Exec(
		"UPDATE active_blocks SET locked = ?, lock_chars = ?, locked_until = ? WHERE block_id = ?",
		locked, lockChars, lockedUntil, blockID,
	)
	if err != nil {
		return fmt.Errorf("failed to update locks of block '%s': %w", name, err)
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return fmt.Errorf("block '%s' is not active", name)
	}
	return nil
}

// DeactivateBlock removes the block from the active blocks table.
// If the block isn't active, it returns an error.
func (d *DB) DeactivateBlock(name string) error {
	blockID, err := d.getBlockID(name)
	if err != nil {
		return err
	}

	result, err := d.conn.Exec("DELETE FROM active_blocks WHERE block_id = ?", blockID)
	if err != nil {
		return fmt.Errorf("failed to deactivate block '%s': %w", name, err)
	}
	// A pause belongs to this run of the block.
	if _, err := d.conn.Exec("DELETE FROM block_pauses WHERE block_id = ?", blockID); err != nil {
		return fmt.Errorf("failed to deactivate block '%s': %w", name, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to verify deactivation of block '%s': %w", name, err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("block '%s' is not active", name)
	}

	return nil
}

// IsBlockActive checks whether a block is currently active.
func (d *DB) IsBlockActive(name string) (bool, error) {
	var count int
	err := d.conn.QueryRow(`
		SELECT COUNT(*) FROM active_blocks ab
		JOIN blocks b ON b.id = ab.block_id
		WHERE b.name = ?
	`, name).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to check whether block '%s' is active: %w", name, err)
	}
	return count > 0, nil
}

// --------------------------------------------------------------------------
// Queries for the daemon (active blocks)
// --------------------------------------------------------------------------

// GetActiveBlocks returns every active block with its sites and apps.
// The daemon uses this to know what to block.
func (d *DB) GetActiveBlocks() ([]ActiveBlockDetail, error) {
	rows, err := d.conn.Query(`
		SELECT b.id, b.name,
			ab.locked, ab.lock_chars, ab.locked_until, ab.scheduled
		FROM active_blocks ab
		JOIN blocks b ON b.id = ab.block_id
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch active blocks: %w", err)
	}
	defer rows.Close()

	var activeBlocks []ActiveBlockDetail
	var ids []int
	for rows.Next() {
		var blockID int
		var abd ActiveBlockDetail

		if err := rows.Scan(&blockID, &abd.BlockName, &abd.Locked, &abd.LockChars, &abd.LockedUntil, &abd.Scheduled); err != nil {
			return nil, fmt.Errorf("failed to read active block: %w", err)
		}
		activeBlocks = append(activeBlocks, abd)
		ids = append(ids, blockID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate active blocks: %w", err)
	}
	// The child lookups below open their own queries, so we must finish reading
	// (and let the deferred Close run) before issuing them; gather rows first.
	for i := range activeBlocks {
		if activeBlocks[i].Lists, err = getBlockLists(d.conn, ids[i]); err != nil {
			return nil, err
		}
		if activeBlocks[i].Apps, err = d.getBlockApps(ids[i]); err != nil {
			return nil, err
		}
		if activeBlocks[i].Titles, err = d.getBlockTitles(ids[i]); err != nil {
			return nil, err
		}
		if activeBlocks[i].Social, err = d.getBlockSocial(ids[i]); err != nil {
			return nil, err
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate active blocks: %w", err)
	}

	return activeBlocks, nil
}

// --------------------------------------------------------------------------
// Helper functions (private)
// --------------------------------------------------------------------------

// getBlockID looks up a block's ID by name.
// It's a helper used by several other functions that need to convert the
// name (which the user provides) into the ID (which the database uses
// internally).
func (d *DB) getBlockID(name string) (int, error) {
	var id int
	err := d.conn.QueryRow("SELECT id FROM blocks WHERE name = ?", name).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, fmt.Errorf("block '%s' not found", name)
	}
	if err != nil {
		return 0, fmt.Errorf("failed to fetch block '%s': %w", name, err)
	}
	return id, nil
}

// getBlockApps returns every process name of a block by ID.
func (d *DB) getBlockApps(blockID int) ([]string, error) {
	rows, err := d.conn.Query("SELECT process_name FROM apps WHERE block_id = ? ORDER BY process_name", blockID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch block apps: %w", err)
	}
	defer rows.Close()

	var apps []string
	for rows.Next() {
		var app string
		if err := rows.Scan(&app); err != nil {
			return nil, fmt.Errorf("failed to read app: %w", err)
		}
		apps = append(apps, app)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate apps: %w", err)
	}

	return apps, nil
}

// Empty reports whether a block has nothing to enforce: no apps, window
// titles, or blacklist that does something (see WebList.Enforces).
func (b *BlockDetail) Empty() bool {
	return len(b.Apps) == 0 && len(b.Titles) == 0 && len(b.Social) == 0 && !b.Lists.Enforces()
}
