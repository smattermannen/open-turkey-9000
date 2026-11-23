// sync.go — the KC9K account and cross-device sync (SPEC §4.14; ROADMAP
// 2.10). Upload all sends this device's whole setup, less its own items,
// to KC9K's cloud (internal/cloud); Download all (next step) replaces the
// setup here with the last upload. The account's session is kept in the
// database, which only root (LocalSystem on Windows) can read.
package cli

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/smattermannen/kc9k/internal/blocker"
	"github.com/smattermannen/kc9k/internal/cloud"
	"github.com/smattermannen/kc9k/internal/db"
	"github.com/spf13/cobra"
)

// KC9K's cloud: set at build time (-ldflags "-X …cli.cloudURL=…") once the
// project exists; 'kc9k account server' overrides them (tests, self-hosting).
var (
	cloudURL     = ""
	cloudAnonKey = ""
)

const (
	settingCloudURL     = "cloud.url"
	settingCloudKey     = "cloud.key"
	settingCloudSession = "cloud.session"
	maxSetupSize        = 50 << 20 // the bucket's limit (cloud/supabase/schema.sql)
)

// cloudClient is replaced in tests.
var cloudClient = func(database *db.DB) (*cloud.Client, error) {
	u, key := cloudURL, cloudAnonKey
	if v, _ := database.GetSetting(settingCloudURL); v != "" {
		u, _ = database.GetSetting(settingCloudURL)
		key, _ = database.GetSetting(settingCloudKey)
	}
	if u == "" || key == "" {
		return nil, errors.New("KC9K's cloud isn't set up in this build yet")
	}
	return cloud.New(u, key), nil
}

func loadSession(database *db.DB) *cloud.Session {
	v, _ := database.GetSetting(settingCloudSession)
	if v == "" {
		return nil
	}
	var s cloud.Session
	if json.Unmarshal([]byte(v), &s) != nil {
		return nil
	}
	return &s
}

func saveSession(database *db.DB, s *cloud.Session) error {
	if s == nil {
		return database.SetSetting(settingCloudSession, "")
	}
	data, _ := json.Marshal(s)
	return database.SetSetting(settingCloudSession, string(data))
}

// signedIn returns the client and a fresh session, stored if refreshed.
func signedIn(database *db.DB) (*cloud.Client, *cloud.Session, error) {
	c, err := cloudClient(database)
	if err != nil {
		return nil, nil, err
	}
	s, changed, err := c.Fresh(loadSession(database))
	if errors.Is(err, cloud.ErrSignedOut) {
		_ = saveSession(database, nil)
	}
	if err != nil {
		return nil, nil, err
	}
	if changed {
		if err := saveSession(database, s); err != nil {
			return nil, nil, err
		}
	}
	return c, s, nil
}

// syncSetup leaves out of a backup what belongs to this device (§4.14's
// device-specific items): places and place rules, affected users, strict
// mode, no-escape and firmware lock, wallpapers, apps' paths and file
// fingerprints, statistics. Circadian rhythm's screens and hardware and
// the device's system passwords are never in a project file.
func syncSetup(p *projectExport) {
	p.Kind = "sync"
	p.Places = nil
	p.Wallpaper = nil
	if p.Circadian != nil {
		p.Circadian.Wallpapers = nil
		p.Circadian.Config.Wallpapers = nil
	}
	for i := range p.Modes {
		p.Modes[i].Places = nil
		p.Modes[i].Wallpapers = nil
	}
	for i := range p.Blocks {
		b := &p.Blocks[i]
		b.Places, b.Users = nil, nil
		b.Apps = slices.DeleteFunc(slices.Clone(b.Apps), deviceApp)
	}
	if p.Backup != nil {
		p.Backup.Statistics = nil
		p.Backup.SettingLocks = slices.DeleteFunc(p.Backup.SettingLocks, func(l db.SettingLock) bool {
			return slices.Contains(deviceSettingLocks, l.Name)
		})
	}
}

// deviceSettingLocks are the setting locks of device-specific items.
var deviceSettingLocks = []string{"strict", "no-escape", "firmware-lock"}

// deviceApp: an app entry naming a file on this device (a full path or the
// program's contents) rather than a program by name.
func deviceApp(entry string) bool {
	return strings.HasPrefix(entry, blocker.HashPrefix) || filepath.IsAbs(entry) || strings.HasPrefix(entry, "/") ||
		len(entry) > 2 && entry[1] == ':' && (entry[2] == '\\' || entry[2] == '/')
}

func gzipped(data []byte) []byte {
	var buf bytes.Buffer
	w, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	_, _ = w.Write(data)
	_ = w.Close()
	return buf.Bytes()
}

func gunzipped(data []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("the uploaded setup is damaged: %w", err)
	}
	defer r.Close()
	out, err := io.ReadAll(io.LimitReader(r, 8*maxSetupSize))
	if err != nil {
		return nil, fmt.Errorf("the uploaded setup is damaged: %w", err)
	}
	return out, nil
}

// deviceName names this device in the upload's record.
var deviceName = func() string {
	name, _ := os.Hostname()
	return name
}

func when(t time.Time) string { return t.Local().Format("2006-01-02 15:04") }

var accountCmd = &cobra.Command{
	Use:   "account",
	Short: "Your KC9K account, for cross-device sync",
	Long: `A KC9K account is needed only for cross-device sync (and later the
accountability partner): nothing else ever leaves this device. Passwords are
read from standard input; this device keeps only the session.`,
}

var accountSignupCmd = &cobra.Command{
	Use:   "signup <email>",
	Short: "Make a KC9K account (password on standard input)",
	Args:  cobra.ExactArgs(1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		c, err := cloudClient(database)
		if err != nil {
			return err
		}
		password, err := readPasswordStdin("New KC9K account password")
		if err != nil {
			return err
		}
		s, err := c.SignUp(args[0], password)
		if err != nil {
			return err
		}
		if s == nil {
			fmt.Printf("Account made. Open the link sent to %s, then 'kc9k account login %s'.\n", args[0], args[0])
			return nil
		}
		if err := saveSession(database, s); err != nil {
			return err
		}
		fmt.Printf("Account made; signed in as %s.\n", s.Email)
		return nil
	}),
}

var accountLoginCmd = &cobra.Command{
	Use:   "login <email>",
	Short: "Sign in to your KC9K account (password on standard input)",
	Args:  cobra.ExactArgs(1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		c, err := cloudClient(database)
		if err != nil {
			return err
		}
		password, err := readPasswordStdin("KC9K account password")
		if err != nil {
			return err
		}
		s, err := c.Login(args[0], password)
		if err != nil {
			return err
		}
		if err := saveSession(database, s); err != nil {
			return err
		}
		fmt.Printf("Signed in as %s.\n", s.Email)
		return nil
	}),
}

var accountLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Sign out of your KC9K account on this device",
	Args:  cobra.NoArgs,
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		s := loadSession(database)
		if s == nil {
			fmt.Println("Not signed in.")
			return nil
		}
		if c, err := cloudClient(database); err == nil {
			if fresh, _, err := c.Fresh(s); err == nil {
				_ = c.Logout(fresh) // the device forgets it either way
			}
		}
		if err := saveSession(database, nil); err != nil {
			return err
		}
		fmt.Println("Signed out.")
		return nil
	}),
}

// accountState is `kc9k account status --json`, for the GUIs.
type accountState struct {
	Available bool          `json:"available"` // this build knows KC9K's cloud
	Email     string        `json:"email,omitempty"`
	Upload    *cloud.Upload `json:"last_upload,omitempty"`
	Error     string        `json:"error,omitempty"`
}

var accountStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Say which KC9K account this device is signed in to, and the last upload",
	Args:  cobra.NoArgs,
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		asJSON, _ := cmd.Flags().GetBool("json")
		var st accountState
		if _, err := cloudClient(database); err == nil {
			st.Available = true
			if s := loadSession(database); s != nil {
				st.Email = s.Email
				if c, fresh, err := signedIn(database); err != nil {
					st.Error = err.Error()
				} else if u, err := c.Latest(fresh); err == nil {
					st.Upload = &u
				} else if !errors.Is(err, cloud.ErrNoUpload) {
					st.Error = err.Error()
				}
			}
		}
		if asJSON {
			data, _ := json.Marshal(st)
			fmt.Println(string(data))
			return nil
		}
		switch {
		case !st.Available:
			fmt.Println("KC9K's cloud isn't set up in this build yet.")
		case st.Email == "":
			fmt.Println("Not signed in ('kc9k account login <email>', or 'kc9k account signup <email>').")
		default:
			fmt.Printf("Signed in as %s.\n", st.Email)
			if st.Upload != nil {
				fmt.Printf("Last upload: %s from %s.\n", when(st.Upload.UploadedAt), st.Upload.Device)
			} else if st.Error == "" {
				fmt.Println("Nothing uploaded yet.")
			}
		}
		if st.Error != "" {
			return errors.New(st.Error)
		}
		return nil
	}),
}

var accountServerCmd = &cobra.Command{
	Use:    "server <url> <anon key>",
	Short:  "Use another KC9K cloud (a self-hosted Supabase project); '' '' for the built-in one",
	Hidden: true,
	Args:   cobra.ExactArgs(2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		if err := database.SetSetting(settingCloudURL, strings.TrimRight(args[0], "/")); err != nil {
			return err
		}
		if err := database.SetSetting(settingCloudKey, args[1]); err != nil {
			return err
		}
		return saveSession(database, nil) // a session belongs to its server
	}),
}

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Copy the whole setup between your devices through your KC9K account",
	Long: `Upload all sends this device's whole setup to your KC9K account; Download all
replaces this device's setup with the last upload. Places, affected users,
strict mode, no-escape, firmware lock, wallpapers, apps' paths and file
fingerprints, and statistics belong to each device and are never copied.`,
}

var syncUploadCmd = &cobra.Command{
	Use:   "upload",
	Short: "Upload all: send this device's whole setup to your KC9K account",
	Args:  cobra.NoArgs,
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		c, s, err := signedIn(database)
		if err != nil {
			return err
		}
		p, err := buildProject(cmd, database, false)
		if err != nil {
			return err
		}
		syncSetup(&p)
		data, err := json.Marshal(p)
		if err != nil {
			return err
		}
		packed := gzipped(data)
		if len(packed) > maxSetupSize {
			return fmt.Errorf("the setup is %d MB packed, over the %d MB an upload may be", len(packed)>>20, maxSetupSize>>20)
		}
		u, err := c.Put(s, packed, deviceName(), projectVersion)
		if err != nil {
			return err
		}
		fmt.Printf("Uploaded: %d blocks, %d KB, %s from %s.\n", len(p.Blocks), (len(packed)+1023)/1024, when(u.UploadedAt), u.Device)
		return nil
	}),
}

func init() {
	accountStatusCmd.Flags().Bool("json", false, "Print as JSON")
	accountCmd.AddCommand(accountSignupCmd, accountLoginCmd, accountLogoutCmd, accountStatusCmd, accountServerCmd)
	syncCmd.AddCommand(syncUploadCmd)
	rootCmd.AddCommand(accountCmd, syncCmd)
}
