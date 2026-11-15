// blockpassword.go — the block password (§3.11): hides a block's contents
// (lists, phrase sets, apps, window titles) until it is typed. Viewing only:
// locks and what is blocked don't change.
//
//	kc9k block password B set      # new password on stdin (the old one first, if any)
//	kc9k block password B remove   # the password on stdin
package cli

import (
	"errors"
	"fmt"
	"time"

	"github.com/smattermannen/kc9k/internal/db"
	"github.com/spf13/cobra"
)

var blockPasswordCmd = &cobra.Command{
	Use:   "password [block] set|remove",
	Short: "Hide a block's contents behind a password (viewing only)",
	Args:  cobra.ExactArgs(2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		name := args[0]
		current, err := database.BlockViewPassword(name)
		if err != nil {
			return err
		}
		if current != "" {
			if err := checkBlockPassword(database, name, current); err != nil {
				return err
			}
		}
		switch args[1] {
		case "set":
			password, err := readPasswordStdin("New block password")
			if err != nil {
				return err
			}
			if len(password) < 4 {
				return fmt.Errorf("a block password has at least 4 characters")
			}
			hash, err := db.HashPassword(password)
			if err != nil {
				return err
			}
			if err := database.SetBlockViewPassword(name, hash); err != nil {
				return err
			}
			fmt.Printf("Block '%s' shows its contents only with its password.\n", name)
			return nil
		case "remove":
			if current == "" {
				return fmt.Errorf("block '%s' has no block password", name)
			}
			if err := database.SetBlockViewPassword(name, ""); err != nil {
				return err
			}
			fmt.Printf("Block '%s' has no block password now.\n", name)
			return nil
		}
		return fmt.Errorf("use set or remove")
	}),
}

// checkBlockPassword reads a block's password from stdin and checks it; a
// wrong one waits a second, like access passwords.
func checkBlockPassword(database *db.DB, name, hash string) error {
	password, err := readPasswordStdin("Block password of '" + name + "'")
	if err != nil {
		return err
	}
	if !db.CheckPassword(hash, password) {
		time.Sleep(time.Second)
		return errors.New("wrong block password")
	}
	return nil
}

// revealBlock asks for a block's password before its contents are shown,
// when it has one.
func revealBlock(database *db.DB, name string) error {
	hash, err := database.BlockViewPassword(name)
	if err != nil || hash == "" {
		return err
	}
	return checkBlockPassword(database, name, hash)
}

func init() {
	blockCmd.AddCommand(blockPasswordCmd)
}
