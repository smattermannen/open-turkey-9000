// freeze.go — Frozen Turkey and the JSON state used by the graphical interface.
package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/brunodcdo/open-turkey/internal/blocker"
	"github.com/brunodcdo/open-turkey/internal/freeze"
	"github.com/spf13/cobra"
)

var freezeCmd = &cobra.Command{
	Use:   "freeze",
	Short: "Frozen Turkey: keep the screen locked for a set time",
	Long: `Lock every session and keep it locked until the time has passed.

There is no way to cancel from the desktop. A running freeze can only be
extended. The time keeps counting while the computer is off.

Emergency exit: in GRUB, press e on the boot entry and add
` + freeze.BypassParam + ` to the line that starts with "linux".`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		forDur, _ := cmd.Flags().GetString("for")
		yes, _ := cmd.Flags().GetBool("yes")

		d, err := time.ParseDuration(forDur)
		if err != nil {
			return fmt.Errorf("invalid --for duration %q: %w", forDur, err)
		}
		if err := freeze.Validate(d); err != nil {
			return err
		}
		if !yes {
			return fmt.Errorf("this locks the computer for %s and cannot be cancelled; add --yes to confirm", d)
		}

		database, err := openDB()
		if err != nil {
			return err
		}
		defer database.Close()

		until := time.Now().Add(d).Unix()
		current, err := database.FreezeUntil()
		if err != nil {
			return err
		}
		if until <= current {
			return fmt.Errorf("already frozen until %s; a freeze can only be extended", time.Unix(current, 0).Format("2006-01-02 15:04"))
		}
		if err := database.SetFreezeUntil(until); err != nil {
			return err
		}
		fmt.Printf("Frozen until %s\n", time.Unix(until, 0).Format("2006-01-02 15:04"))
		return blocker.LockSessions()
	},
}

type guiBlock struct {
	Name        string   `json:"name"`
	Mode        string   `json:"mode"`
	Sites       []string `json:"sites"`
	Apps        []string `json:"apps"`
	Schedules   []string `json:"schedules"`
	Active      bool     `json:"active"`
	Held        bool     `json:"held"`
	CharLocked  bool     `json:"char_locked"`
	Scheduled   bool     `json:"scheduled"`
	LockSeconds int64    `json:"lock_seconds"`
}

type guiState struct {
	FreezeSeconds int64      `json:"freeze_seconds"`
	Blocks        []guiBlock `json:"blocks"`
}

var guiStateCmd = &cobra.Command{
	Use:    "gui-state",
	Short:  "Internal: print the state as JSON for the graphical interface",
	Hidden: true,
	Args:   cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := openDB()
		if err != nil {
			return err
		}
		defer database.Close()

		now := time.Now()
		trusted, err := database.TrustedNow(now)
		if err != nil {
			return err
		}
		state := guiState{Blocks: []guiBlock{}}
		until, err := database.FreezeUntil()
		if err != nil {
			return err
		}
		if until > now.Unix() {
			state.FreezeSeconds = until - now.Unix()
		}

		blocks, err := database.ListBlocks()
		if err != nil {
			return err
		}
		for _, b := range blocks {
			detail, err := database.GetBlock(b.Name)
			if err != nil {
				return err
			}
			hold, err := blockHold(database, b.Name, now)
			if err != nil {
				return err
			}
			schedules, err := database.ListSchedules(b.Name)
			if err != nil {
				return err
			}
			g := guiBlock{
				Name: detail.Name, Mode: detail.Mode,
				Sites: append([]string{}, detail.Sites...), Apps: append([]string{}, detail.Apps...),
				Schedules: []string{},
				Active:    detail.Active, Held: hold.held(), CharLocked: detail.Active && detail.Locked,
				Scheduled: detail.Scheduled,
			}
			if detail.Active && detail.LockedUntil > trusted {
				g.LockSeconds = detail.LockedUntil - trusted
			}
			for _, s := range schedules {
				g.Schedules = append(g.Schedules, s.Window().String())
			}
			state.Blocks = append(state.Blocks, g)
		}

		out, err := json.Marshal(state)
		if err != nil {
			return err
		}
		fmt.Println(string(out))
		return nil
	},
}

func init() {
	freezeCmd.Flags().String("for", "", "How long to stay frozen (e.g. 30m, 2h; at most 24h)")
	freezeCmd.Flags().Bool("yes", false, "Confirm the freeze")
	_ = freezeCmd.MarkFlagRequired("for")
	rootCmd.AddCommand(freezeCmd, guiStateCmd)
}
