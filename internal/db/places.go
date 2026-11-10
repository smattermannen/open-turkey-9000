// places.go — places, known by the Wi-Fi networks in range there, and the
// blocks that are on while the computer is at one (enforce/places.go).
package db

import (
	"database/sql"
	"fmt"
	"strings"
)

const placesSchema = `CREATE TABLE IF NOT EXISTS places (
    id   INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE
);
CREATE TABLE IF NOT EXISTS place_networks (
    place_id INTEGER NOT NULL,
    bssid    TEXT NOT NULL,
    ssid     TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (place_id, bssid),
    FOREIGN KEY (place_id) REFERENCES places(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS place_rules (
    block_id INTEGER NOT NULL,
    place_id INTEGER NOT NULL,
    PRIMARY KEY (block_id, place_id),
    FOREIGN KEY (block_id) REFERENCES blocks(id) ON DELETE CASCADE,
    FOREIGN KEY (place_id) REFERENCES places(id) ON DELETE CASCADE
);`

// Network is a Wi-Fi access point: its hardware address and its name.
type Network struct {
	BSSID string
	SSID  string
}

// Place is a named set of networks, with the blocks that are on there.
type Place struct {
	ID       int
	Name     string
	Networks []Network
	Blocks   []string
}

// MaxPlaceNetworks is how many networks a place remembers.
const MaxPlaceNetworks = 24

// Places returns every place.
func (d *DB) Places() ([]Place, error) {
	rows, err := d.conn.Query("SELECT id, name FROM places ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("failed to read places: %w", err)
	}
	var out []Place
	for rows.Next() {
		var p Place
		if err := rows.Scan(&p.ID, &p.Name); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		nets, err := d.conn.Query("SELECT bssid, ssid FROM place_networks WHERE place_id = ? ORDER BY bssid", out[i].ID)
		if err != nil {
			return nil, err
		}
		for nets.Next() {
			var n Network
			if err := nets.Scan(&n.BSSID, &n.SSID); err != nil {
				nets.Close()
				return nil, err
			}
			out[i].Networks = append(out[i].Networks, n)
		}
		nets.Close()
		blocks, err := d.conn.Query(`SELECT b.name FROM place_rules r JOIN blocks b ON b.id = r.block_id
			WHERE r.place_id = ? ORDER BY b.name`, out[i].ID)
		if err != nil {
			return nil, err
		}
		for blocks.Next() {
			var name string
			if err := blocks.Scan(&name); err != nil {
				blocks.Close()
				return nil, err
			}
			out[i].Blocks = append(out[i].Blocks, name)
		}
		blocks.Close()
	}
	return out, nil
}

// GetPlace returns one place.
func (d *DB) GetPlace(name string) (Place, error) {
	places, err := d.Places()
	if err != nil {
		return Place{}, err
	}
	for _, p := range places {
		if strings.EqualFold(p.Name, name) {
			return p, nil
		}
	}
	return Place{}, fmt.Errorf("no place named '%s'", name)
}

// SavePlace creates a place or adds networks to the one with that name, and
// returns how many networks it knows afterwards.
func (d *DB) SavePlace(name string, networks []Network) (int, error) {
	if err := CheckListName(name); err != nil {
		return 0, err
	}
	tx, err := d.conn.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var id int
	err = tx.QueryRow("SELECT id FROM places WHERE name = ? COLLATE NOCASE", name).Scan(&id)
	if err == sql.ErrNoRows {
		res, err := tx.Exec("INSERT INTO places (name) VALUES (?)", name)
		if err != nil {
			return 0, fmt.Errorf("failed to save the place: %w", err)
		}
		last, _ := res.LastInsertId()
		id = int(last)
	} else if err != nil {
		return 0, err
	}
	for _, n := range networks {
		var count int
		if err := tx.QueryRow("SELECT COUNT(*) FROM place_networks WHERE place_id = ?", id).Scan(&count); err != nil {
			return 0, err
		}
		if count >= MaxPlaceNetworks {
			break
		}
		if _, err := tx.Exec("INSERT OR IGNORE INTO place_networks (place_id, bssid, ssid) VALUES (?, ?, ?)",
			id, strings.ToUpper(n.BSSID), n.SSID); err != nil {
			return 0, err
		}
	}
	var total int
	if err := tx.QueryRow("SELECT COUNT(*) FROM place_networks WHERE place_id = ?", id).Scan(&total); err != nil {
		return 0, err
	}
	return total, tx.Commit()
}

// DeletePlace removes a place and its rules.
func (d *DB) DeletePlace(id int) error {
	if _, err := d.conn.Exec("DELETE FROM places WHERE id = ?", id); err != nil {
		return fmt.Errorf("failed to remove the place: %w", err)
	}
	return nil
}

// SetPlaceRule makes a block be on at a place, or stops that.
func (d *DB) SetPlaceRule(blockName string, placeID int, on bool) error {
	blockID, err := d.getBlockID(blockName)
	if err != nil {
		return err
	}
	if on {
		_, err = d.conn.Exec("INSERT OR IGNORE INTO place_rules (block_id, place_id) VALUES (?, ?)", blockID, placeID)
	} else {
		_, err = d.conn.Exec("DELETE FROM place_rules WHERE block_id = ? AND place_id = ?", blockID, placeID)
	}
	if err != nil {
		return fmt.Errorf("failed to save the place rule: %w", err)
	}
	return nil
}
