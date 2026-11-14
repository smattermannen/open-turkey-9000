// clusters.go — clusters: website lists kept on their own, outside any
// block. A cluster has a name, the places it looks (url / title / text) and
// entries, like a block's list, but nothing enforces it: it is a stock of
// entries (the academia sites, say) to copy into a block's lists, as a new
// list or into one the block has. Whether those entries are blocked or
// spared is decided where they are copied to, so a cluster has no kind and
// no actions. Editing a cluster never changes a block. A phrase cluster
// (Phrases) holds phrases, in their typed form, instead of entries, and
// looks at any of url, title and text, like a phrase set (R-PHRASE-10).
package db

import (
	"database/sql"
	"fmt"
	"strings"
)

const clustersSchema = `CREATE TABLE IF NOT EXISTS clusters (
    id       INTEGER PRIMARY KEY AUTOINCREMENT,
    name     TEXT NOT NULL UNIQUE,
    in_url   BOOLEAN NOT NULL DEFAULT 1,
    in_title BOOLEAN NOT NULL DEFAULT 0,
    in_text  BOOLEAN NOT NULL DEFAULT 0,
    phrases  BOOLEAN NOT NULL DEFAULT 0,
    type     TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS cluster_entries (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    cluster_id INTEGER NOT NULL,
    entry      TEXT NOT NULL,
    FOREIGN KEY (cluster_id) REFERENCES clusters(id) ON DELETE CASCADE
);`

// Cluster is a website list that belongs to no block.
type Cluster struct {
	ID      int
	Name    string
	Where   Where
	Entries []string
	Phrases bool   // a phrase cluster: Entries are phrases
	Type    string // ListAddress or ListTerm (not for a phrase cluster)
}

func (d *DB) clusterEntries(id int) ([]string, error) {
	rows, err := d.conn.Query("SELECT entry FROM cluster_entries WHERE cluster_id = ? ORDER BY id", id)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch cluster entries: %w", err)
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var e string
		if err := rows.Scan(&e); err != nil {
			return nil, fmt.Errorf("failed to read a cluster entry: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// GetCluster returns the named cluster with its entries.
func (d *DB) GetCluster(name string) (Cluster, error) {
	c := Cluster{Name: name}
	err := d.conn.QueryRow("SELECT id, in_url, in_title, in_text, phrases, type FROM clusters WHERE name = ?", name).
		Scan(&c.ID, &c.Where.URL, &c.Where.Title, &c.Where.Text, &c.Phrases, &c.Type)
	if err == sql.ErrNoRows {
		return c, fmt.Errorf("there is no cluster named '%s'", name)
	}
	if err != nil {
		return c, fmt.Errorf("failed to read cluster '%s': %w", name, err)
	}
	c.Entries, err = d.clusterEntries(c.ID)
	return c, err
}

// ClusterExists reports whether a cluster has this name.
func (d *DB) ClusterExists(name string) (bool, error) {
	var n int
	if err := d.conn.QueryRow("SELECT COUNT(*) FROM clusters WHERE name = ?", name).Scan(&n); err != nil {
		return false, fmt.Errorf("failed to look up cluster '%s': %w", name, err)
	}
	return n > 0, nil
}

// ListClusters returns every cluster with its entries, by name.
func (d *DB) ListClusters() ([]Cluster, error) {
	rows, err := d.conn.Query("SELECT id, name, in_url, in_title, in_text, phrases, type FROM clusters ORDER BY name COLLATE NOCASE")
	if err != nil {
		return nil, fmt.Errorf("failed to list clusters: %w", err)
	}
	var out []Cluster
	for rows.Next() {
		var c Cluster
		if err := rows.Scan(&c.ID, &c.Name, &c.Where.URL, &c.Where.Title, &c.Where.Text, &c.Phrases, &c.Type); err != nil {
			rows.Close()
			return nil, fmt.Errorf("failed to read a cluster: %w", err)
		}
		out = append(out, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		if out[i].Entries, err = d.clusterEntries(out[i].ID); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// CreateCluster adds an empty cluster, of the type its places suggest (see
// DefaultType). Names follow the rules for list names.
func (d *DB) CreateCluster(name string, where Where) error {
	return d.CreateTypedCluster(name, DefaultType(where), where)
}

// CreateTypedCluster adds an empty address or term cluster.
func (d *DB) CreateTypedCluster(name, typ string, where Where) error {
	return d.createCluster(name, typ, where, false)
}

// CreatePhraseCluster adds an empty phrase cluster.
func (d *DB) CreatePhraseCluster(name string, where Where) error {
	return d.createCluster(name, ListTerm, where, true)
}

func (d *DB) createCluster(name, typ string, where Where, phrases bool) error {
	if err := CheckListName(name); err != nil {
		return err
	}
	if err := checkClusterWhere(typ, where, phrases); err != nil {
		return err
	}
	_, err := d.conn.Exec("INSERT INTO clusters (name, in_url, in_title, in_text, phrases, type) VALUES (?, ?, ?, ?, ?, ?)",
		name, where.URL, where.Title, where.Text, phrases, typ)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return fmt.Errorf("there already is a cluster named '%s'", name)
		}
		return fmt.Errorf("failed to create cluster '%s': %w", name, err)
	}
	return nil
}

// RenameCluster renames a cluster.
func (d *DB) RenameCluster(id int, name string) error {
	if err := CheckListName(name); err != nil {
		return err
	}
	if _, err := d.conn.Exec("UPDATE clusters SET name = ? WHERE id = ?", name, id); err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return fmt.Errorf("there already is a cluster named '%s'", name)
		}
		return fmt.Errorf("failed to rename the cluster: %w", err)
	}
	return nil
}

// DeleteCluster removes a cluster with its entries.
func (d *DB) DeleteCluster(id int) error {
	if _, err := d.conn.Exec("DELETE FROM clusters WHERE id = ?", id); err != nil {
		return fmt.Errorf("failed to remove the cluster: %w", err)
	}
	return nil
}

// checkClusterWhere: a list cluster is an address or a term cluster; a
// phrase cluster looks anywhere, like a phrase set.
func checkClusterWhere(typ string, where Where, phrases bool) error {
	if phrases {
		if !where.Any() {
			return fmt.Errorf("a phrase cluster looks at the address, the title or the text")
		}
		return nil
	}
	return CheckScope(typ, where)
}

// SetClusterWhere sets where a cluster looks.
func (d *DB) SetClusterWhere(id int, where Where) error {
	var phrases bool
	var typ string
	if err := d.conn.QueryRow("SELECT phrases, type FROM clusters WHERE id = ?", id).Scan(&phrases, &typ); err != nil {
		return fmt.Errorf("failed to read the cluster: %w", err)
	}
	if err := checkClusterWhere(typ, where, phrases); err != nil {
		return err
	}
	_, err := d.conn.Exec("UPDATE clusters SET in_url = ?, in_title = ?, in_text = ? WHERE id = ?",
		where.URL, where.Title, where.Text, id)
	if err != nil {
		return fmt.Errorf("failed to set where the cluster looks: %w", err)
	}
	return nil
}

// AddClusterEntries adds entries to a cluster, skipping ones it has
// (ignoring case), and returns how many were new.
func (d *DB) AddClusterEntries(id int, entries []string) (int, error) {
	have, err := d.clusterEntries(id)
	if err != nil {
		return 0, err
	}
	seen := map[string]bool{}
	for _, e := range have {
		seen[strings.ToLower(e)] = true
	}
	tx, err := d.conn.Begin()
	if err != nil {
		return 0, fmt.Errorf("failed to add to the cluster: %w", err)
	}
	defer tx.Rollback()
	added := 0
	for _, e := range entries {
		e = strings.TrimSpace(e)
		if e == "" || seen[strings.ToLower(e)] {
			continue
		}
		seen[strings.ToLower(e)] = true
		if _, err := tx.Exec("INSERT INTO cluster_entries (cluster_id, entry) VALUES (?, ?)", id, e); err != nil {
			return 0, fmt.Errorf("failed to add %q: %w", e, err)
		}
		added++
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to add to the cluster: %w", err)
	}
	return added, nil
}

// RemoveClusterEntries removes entries from a cluster.
func (d *DB) RemoveClusterEntries(id int, entries []string) error {
	for _, e := range entries {
		if _, err := d.conn.Exec("DELETE FROM cluster_entries WHERE cluster_id = ? AND entry = ?", id, e); err != nil {
			return fmt.Errorf("failed to remove %q: %w", e, err)
		}
	}
	return nil
}
