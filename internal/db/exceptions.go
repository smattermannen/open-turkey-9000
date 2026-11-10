// exceptions.go — the exceptions table from before website lists, read once
// by migrateWebLists.
package db

const exceptionsSchema = `CREATE TABLE IF NOT EXISTS exceptions (
    id       INTEGER PRIMARY KEY AUTOINCREMENT,
    block_id INTEGER NOT NULL,
    pattern  TEXT NOT NULL,
    FOREIGN KEY (block_id) REFERENCES blocks(id) ON DELETE CASCADE
);`
