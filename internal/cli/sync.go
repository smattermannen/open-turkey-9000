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
	"github.com/smattermannen/kc9k/internal/circadian"
	"github.com/smattermannen/kc9k/internal/cloud"
	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
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

// settingSyncedLocks remembers which locks came here by Download all, and
// from which account (syncedLocks).
const settingSyncedLocks = "sync.locks"

type syncedLocks struct {
	User   string   `json:"user"`
	Prints []string `json:"prints"`
}

// lockPrints describes each lock running here (the backup's running locks:
// blocks and settings, not the device's own), one string per lock, equal
// while the lock is unchanged.
func lockPrints(database *db.DB) (map[string]string, error) {
	b, err := exportBackupExtras(database)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, l := range b.BlockLocks {
		data, _ := json.Marshal(l)
		out["block:"+string(data)] = "block '" + l.Block + "'"
	}
	for _, l := range b.SettingLocks {
		if slices.Contains(deviceSettingLocks, l.Name) {
			continue
		}
		data, _ := json.Marshal(l)
		out["setting:"+string(data)] = "the setting " + l.Name
	}
	return out, nil
}

// localLocks names the locks running here that didn't come by Download all
// from this account, unchanged since (DECIDED 2026-11-23, ROADMAP §5): those
// were set on this device, and Download all never lifts them; otherwise an
// empty setup uploaded from a second account would lift every lock.
func localLocks(database *db.DB, user string) ([]string, error) {
	prints, err := lockPrints(database)
	if err != nil {
		return nil, err
	}
	var known syncedLocks
	if v, _ := database.GetSetting(settingSyncedLocks); v != "" {
		_ = json.Unmarshal([]byte(v), &known)
	}
	var out []string
	for p, name := range prints {
		if known.User != user || !slices.Contains(known.Prints, p) {
			out = append(out, name)
		}
	}
	slices.Sort(out)
	return out, nil
}

func rememberSyncedLocks(database *db.DB, user string) error {
	prints, err := lockPrints(database)
	if err != nil {
		return err
	}
	known := syncedLocks{User: user, Prints: []string{}}
	for p := range prints {
		known.Prints = append(known.Prints, p)
	}
	slices.Sort(known.Prints)
	data, _ := json.Marshal(known)
	return database.SetSetting(settingSyncedLocks, string(data))
}

// deviceItems are this device's own items that Download all keeps, by name
// (§4.14): the blocks' place rules, affected users, apps' paths and file
// fingerprints; the modes' places and wallpapers; the circadian
// wallpapers. Places, statistics and the device's settings are never
// cleared.
type deviceItems struct {
	places        map[string][]int // block -> place IDs
	users         map[string][]string
	apps          map[string][]string
	modePlaces    map[string][]int
	modeWalls     map[string][]circadian.Wallpaper
	circadianWall []circadian.Wallpaper
}

func saveDeviceItems(database *db.DB) (deviceItems, error) {
	k := deviceItems{places: map[string][]int{}, users: map[string][]string{}, apps: map[string][]string{},
		modePlaces: map[string][]int{}, modeWalls: map[string][]circadian.Wallpaper{}}
	places, err := database.Places()
	if err != nil {
		return k, err
	}
	for _, p := range places {
		for _, b := range p.Blocks {
			k.places[b] = append(k.places[b], p.ID)
		}
	}
	blocks, err := database.ListBlocks()
	if err != nil {
		return k, err
	}
	for _, b := range blocks {
		if users, err := database.BlockUsers(b.Name); err == nil && len(users) > 0 {
			k.users[b.Name] = users
		}
		if detail, err := database.GetBlock(b.Name); err == nil {
			for _, a := range detail.Apps {
				if deviceApp(a) {
					k.apps[b.Name] = append(k.apps[b.Name], a)
				}
			}
		}
	}
	modes, err := database.Modes()
	if err != nil {
		return k, err
	}
	for _, m := range modes {
		k.modePlaces[strings.ToLower(m.Name)] = m.Spec.Places
		k.modeWalls[strings.ToLower(m.Name)] = enforce.ModeWallpapers(database, m.ID)
	}
	if enforce.CircadianConfigured(database) {
		k.circadianWall = enforce.CircadianConfig(database).Wallpapers
	}
	return k, nil
}

// restore puts the device's items back on the blocks and modes that the
// download brought under the same names.
func (k deviceItems) restore(database *db.DB) error {
	for name, ids := range k.places {
		if _, err := database.GetBlock(name); err != nil {
			continue
		}
		for _, id := range ids {
			if err := database.SetPlaceRule(name, id, true); err != nil {
				return err
			}
		}
	}
	for name, users := range k.users {
		if _, err := database.GetBlock(name); err == nil {
			if err := database.SetBlockUsers(name, users); err != nil {
				return err
			}
		}
	}
	for name, apps := range k.apps {
		if _, err := database.GetBlock(name); err == nil {
			if err := database.AddApps(name, apps); err != nil {
				return err
			}
		}
	}
	modes, err := database.Modes()
	if err != nil {
		return err
	}
	for _, m := range modes {
		key := strings.ToLower(m.Name)
		if ids := k.modePlaces[key]; len(ids) > 0 {
			spec := m.Spec.Clone()
			for _, id := range ids {
				if !slices.Contains(spec.Places, id) {
					spec.Places = append(spec.Places, id)
				}
			}
			if err := database.SetModeAutomation(m.ID, spec); err != nil {
				return err
			}
		}
		if ws := k.modeWalls[key]; len(ws) > 0 {
			if err := enforce.SetModeWallpapers(database, m.ID, ws); err != nil {
				return err
			}
		}
	}
	if len(k.circadianWall) > 0 {
		c := enforce.CircadianConfig(database)
		c.Wallpapers = k.circadianWall
		if err := enforce.SetCircadianConfig(database, c); err != nil {
			return err
		}
	}
	return nil
}

// quietly runs f with its output thrown away (the download's check pass).
func quietly(f func() error) error {
	r, w, err := os.Pipe()
	if err != nil {
		return f()
	}
	drained := make(chan struct{})
	go func() {
		_, _ = io.Copy(io.Discard, r)
		close(drained)
	}()
	old := os.Stdout
	os.Stdout = w
	defer func() {
		os.Stdout = old
		w.Close()
		<-drained
		r.Close()
	}()
	return f()
}

// confirmReplace asks before Download all, as confirmLocks asks before a
// lock: typed yes on a terminal, --yes elsewhere (the GUIs ask first).
func confirmReplace(cmd *cobra.Command, what string) error {
	if f := cmd.Flags().Lookup("yes"); f != nil && f.Value.String() == "true" {
		return nil
	}
	if !isTerminalInput() {
		return fmt.Errorf("%s; add --yes to confirm", what)
	}
	fmt.Printf("%s. Type yes to confirm: ", what)
	answer, _ := stdinLines.ReadString('\n')
	if strings.TrimSpace(strings.ToLower(answer)) != "yes" {
		return errors.New("nothing was changed")
	}
	return nil
}

var syncDownloadCmd = &cobra.Command{
	Use:   "download",
	Short: "Download all: replace this device's setup with the last upload",
	Long: `Replaces this device's whole setup with the last upload from your KC9K
account: blocks, lists, phrase sets, locks and their state, settings and
presets. What belongs to this device stays: places and the blocks' and modes'
place rules, affected users, apps' paths and file fingerprints, wallpapers,
statistics, strict mode, no-escape and firmware lock.

Locks don't stop it, but a lock set on this device does: Download all lifts
only locks that came here by an earlier Download all from the same account
(unchanged since). With --dry-run it says what would happen.`,
	Args: cobra.NoArgs,
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		dry, _ := cmd.Flags().GetBool("dry-run")
		c, s, err := signedIn(database)
		if err != nil {
			return err
		}
		u, err := c.Latest(s)
		if err != nil {
			return err
		}
		packed, err := c.Get(s)
		if err != nil {
			return err
		}
		raw, err := gunzipped(packed)
		if err != nil {
			return err
		}
		var in projectExport
		if err := json.Unmarshal(projectText(raw), &in); err != nil || in.KC9KProject == 0 {
			return errors.New("the uploaded setup is damaged")
		}
		if in.KC9KProject > projectVersion {
			return errors.New("the uploaded setup was made by a newer Killer Chicken 9000: update it here first")
		}
		syncSetup(&in) // never a device's items from a file, whoever made it
		fmt.Printf("Last upload: %s from %s (%d blocks).\n", when(u.UploadedAt), u.Device, len(in.Blocks))

		local, err := localLocks(database, s.UserID)
		if err != nil {
			return err
		}
		if len(local) > 0 {
			return refused(database, "", fmt.Errorf("Download all would lift locks set on this device (%s); they stay until they end or are unlocked here",
				strings.Join(local, ", ")))
		}
		// Check the whole file as if onto an empty setup before touching
		// anything, and gather the locks it sets.
		var locks []lockToSet
		if err := quietly(func() error {
			return importProject(cmd, in, importOptions{dry: true, replacing: true, oneToOne: true, locks: &locks})
		}); err != nil {
			return fmt.Errorf("the uploaded setup can't be used here: %w", err)
		}
		blocks, err := database.ListBlocks()
		if err != nil {
			return err
		}
		here := 0
		for _, b := range blocks {
			if draftOf(database, b.Name) == "" {
				here++
			}
		}
		if dry {
			fmt.Printf("Would: remove this device's setup (%d blocks), keeping its own items\n", here)
			for _, b := range in.Blocks {
				fmt.Printf("Would: add block '%s'\n", b.Name)
			}
			for _, l := range locks {
				fmt.Printf("Would: set a lock: %s\n", l)
			}
			return nil
		}
		if err := confirmReplace(cmd, fmt.Sprintf("Download all replaces this device's setup (%d blocks) with the upload of %s from %s",
			here, when(u.UploadedAt), u.Device)); err != nil {
			return err
		}
		if what := projectMonk(in); what != "" {
			if err := requireMonkAck(cmd, what); err != nil {
				return err
			}
		}
		if customs := projectCustoms(in); len(customs) > 0 {
			if err := confirmCustoms(cmd, customs); err != nil {
				return err
			}
		}
		if err := confirmLocks(cmd, locks...); err != nil {
			return err
		}

		keep, err := saveDeviceItems(database)
		if err != nil {
			return err
		}
		if err := database.ClearForSync(deviceSettingLocks); err != nil {
			return err
		}
		if err := importProject(cmd, in, importOptions{confirmed: true, oneToOne: true}); err != nil {
			return fmt.Errorf("the download stopped halfway (upload this device's setup again from a device that has it, or download again): %w", err)
		}
		if err := keep.restore(database); err != nil {
			return err
		}
		if err := rememberSyncedLocks(database, s.UserID); err != nil {
			return err
		}
		if err := enforce.ApplyFromDB(database); err != nil {
			return err
		}
		fmt.Printf("Downloaded: this device has the setup uploaded %s from %s.\n", when(u.UploadedAt), u.Device)
		return nil
	}),
}

func init() {
	accountStatusCmd.Flags().Bool("json", false, "Print as JSON")
	accountCmd.AddCommand(accountSignupCmd, accountLoginCmd, accountLogoutCmd, accountStatusCmd, accountServerCmd)
	syncDownloadCmd.Flags().Bool("dry-run", false, "Say what would happen; change nothing")
	addYesFlag(syncDownloadCmd)
	addMonkAckFlag(syncDownloadCmd)
	syncCmd.AddCommand(syncUploadCmd, syncDownloadCmd)
	rootCmd.AddCommand(accountCmd, syncCmd)
}
