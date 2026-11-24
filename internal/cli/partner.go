package cli

// partner.go — the accountability partner (SPEC §4.15; ROADMAP 2.13),
// through the KC9K account (cli/sync.go, internal/cloud). The user invites
// partners by email; each gets reports (daily or weekly) and alerts as they
// happen, can hold a password lock or the settings password (generated
// here, sent only to them, never shown to the user) and approves or denies
// unlock requests. On the partner's own KC9K: `kc9k partner inbox`,
// `accept`, `approve`, `deny`. Removing a partner, fewer reports and alerts
// off weaken: refused while anything is locked (R-WEAK-1, logged by
// refused()). Without an account or partners nothing leaves the device.

import (
	"bufio"
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/smattermannen/kc9k/internal/cloud"
	"github.com/smattermannen/kc9k/internal/daemon"
	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
	"github.com/spf13/cobra"
)

const (
	settingPartnerHolds     = "partner.holds"     // JSON: what -> the partner's email
	settingPartnerReports   = "partner.reports"   // JSON: period -> the last day reported
	settingPartnerHeartbeat = "partner.heartbeat" // unix seconds of the last heartbeat
	// What a partner can hold: the settings password, or a block's password
	// lock ("lock:" + block).
	holdSettingsPassword = "settings-password"
	holdLockPrefix       = "lock:"
)

// partnerHolds is what this device's partners hold, by what.
func partnerHolds(database *db.DB) map[string]string {
	m := map[string]string{}
	if v, _ := database.GetSetting(settingPartnerHolds); v != "" {
		_ = json.Unmarshal([]byte(v), &m)
	}
	return m
}

func savePartnerHolds(database *db.DB, m map[string]string) error {
	data, _ := json.Marshal(m)
	return database.SetSetting(settingPartnerHolds, string(data))
}

// newPartnerSecret is a password nobody but the partner will see: 120
// random bits.
func newPartnerSecret() string {
	b := make([]byte, 15)
	_, _ = rand.Read(b)
	return strings.ToLower(base32.StdEncoding.EncodeToString(b))
}

func findPartner(c *cloud.Client, s *cloud.Session, email string) (cloud.Partner, error) {
	ps, err := c.Partners(s)
	if err != nil {
		return cloud.Partner{}, err
	}
	for _, p := range ps {
		if strings.EqualFold(p.Email, strings.TrimSpace(email)) {
			return p, nil
		}
	}
	return cloud.Partner{}, fmt.Errorf("%s isn't one of your partners ('kc9k partner list')", email)
}

// refusePartnerWeakening refuses a change that weakens what the partners
// see or hold while any block or setting is locked (§4.15).
func refusePartnerWeakening(database *db.DB, what string) error {
	if err := requireSettingsPassword(database); err != nil {
		return err
	}
	now := time.Now()
	trusted, err := database.TrustedNow(now)
	if err != nil {
		return err
	}
	locked, err := enforce.AnyLock(database, now, trusted)
	if err != nil {
		return err
	}
	if locked {
		return refused(database, "", fmt.Errorf("%s weakens your accountability: refused while a block or setting is locked", what))
	}
	return nil
}

// reportsRank orders report periods from most to least often.
var reportsRank = map[string]int{"daily": 0, "weekly": 1, "off": 2}

var partnerCmd = &cobra.Command{
	Use:   "partner",
	Short: "Your accountability partners: reports, alerts, held passwords, unlock requests",
	Long: `An accountability partner (needs a KC9K account, 'kc9k account') gets
reports of your time on sites and apps and how often they were blocked
(daily or weekly), alerts as things happen (a block switched off, unlocked
or deleted, bypass prevention, tamper events, KC9K stopped or not heard
from), and can hold a password lock or the settings password: KC9K makes
the password and sends it only to them. Ask them with 'kc9k partner
request'; once they approve, 'kc9k partner requests' unlocks.

Your partner signs in to KC9K with the invited address and uses 'kc9k
partner inbox', 'accept', 'approve' and 'deny'. No screenshots, ever.

Removing a partner, fewer reports and alerts off are refused while any
block or setting is locked.`,
}

var partnerInviteCmd = &cobra.Command{
	Use:   "invite <email>",
	Short: "Invite a partner (they get an email and accept on their own KC9K)",
	Args:  cobra.ExactArgs(1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		reports, _ := cmd.Flags().GetString("reports")
		alerts, _ := cmd.Flags().GetBool("alerts")
		if _, ok := reportsRank[reports]; !ok {
			return fmt.Errorf("--reports is daily, weekly or off")
		}
		c, s, err := signedIn(database)
		if err != nil {
			return err
		}
		p, err := c.InvitePartner(s, args[0], reports, alerts)
		if err != nil {
			return err
		}
		fmt.Printf("Invited %s (reports %s, alerts %s). They accept on their own KC9K, signed in with that address.\n",
			p.Email, p.Reports, onOff(p.Alerts))
		return nil
	}),
}

// partnerView is a partner for `partner list --json` and the GUIs.
type partnerView struct {
	cloud.Partner
	Requests []cloud.Request `json:"requests,omitempty"`
}

var partnerListCmd = &cobra.Command{
	Use:   "list",
	Short: "Your partners: accepted or not, what they get, what they hold, your requests",
	Args:  cobra.NoArgs,
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		asJSON, _ := cmd.Flags().GetBool("json")
		c, s, err := signedIn(database)
		if err != nil {
			return err
		}
		ps, err := c.Partners(s)
		if err != nil {
			return err
		}
		reqs, err := c.Requests(s)
		if err != nil {
			return err
		}
		views := []partnerView{}
		for _, p := range ps {
			v := partnerView{Partner: p}
			for _, r := range reqs {
				if strings.EqualFold(r.Partner, p.Email) && (r.Status == "pending" || r.Status == "approved") {
					r.Secret = "" // never shown
					v.Requests = append(v.Requests, r)
				}
			}
			views = append(views, v)
		}
		if asJSON {
			data, _ := json.Marshal(views)
			fmt.Println(string(data))
			return nil
		}
		if len(views) == 0 {
			fmt.Println("No partners ('kc9k partner invite <email>').")
		}
		for _, v := range views {
			state := "accepted"
			if !v.Accepted {
				state = "invited, not accepted yet"
			}
			fmt.Printf("%s: %s; reports %s, alerts %s", v.Email, state, v.Reports, onOff(v.Alerts))
			if len(v.Holds) > 0 {
				fmt.Printf("; holds %s", strings.Join(v.Holds, ", "))
			}
			fmt.Println()
			for _, r := range v.Requests {
				fmt.Printf("  request %d (%s): %s\n", r.ID, r.What, r.Status)
			}
		}
		return nil
	}),
}

var partnerSetCmd = &cobra.Command{
	Use:   "set <email>",
	Short: "Change what a partner gets (fewer reports or alerts off weaken)",
	Args:  cobra.ExactArgs(1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		c, s, err := signedIn(database)
		if err != nil {
			return err
		}
		p, err := findPartner(c, s, args[0])
		if err != nil {
			return err
		}
		reports, alerts := p.Reports, p.Alerts
		if cmd.Flags().Changed("reports") {
			reports, _ = cmd.Flags().GetString("reports")
			if _, ok := reportsRank[reports]; !ok {
				return fmt.Errorf("--reports is daily, weekly or off")
			}
		}
		if cmd.Flags().Changed("alerts") {
			alerts, _ = cmd.Flags().GetBool("alerts")
		}
		if reportsRank[reports] > reportsRank[p.Reports] || p.Alerts && !alerts {
			if err := refusePartnerWeakening(database, "Fewer reports or alerts off for "+p.Email); err != nil {
				return err
			}
		}
		if err := c.SetPartner(s, p.ID, reports, alerts); err != nil {
			return err
		}
		fmt.Printf("%s: reports %s, alerts %s.\n", p.Email, reports, onOff(alerts))
		return nil
	}),
}

var partnerRemoveCmd = &cobra.Command{
	Use:   "remove <email>",
	Short: "Remove a partner (weakens: refused while anything is locked)",
	Args:  cobra.ExactArgs(1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		c, s, err := signedIn(database)
		if err != nil {
			return err
		}
		p, err := findPartner(c, s, args[0])
		if err != nil {
			return err
		}
		if err := refusePartnerWeakening(database, "Removing "+p.Email); err != nil {
			return err
		}
		holds := partnerHolds(database)
		if strings.EqualFold(holds[holdSettingsPassword], p.Email) {
			if set, _ := settingsPasswordSet(database); set {
				return refused(database, "", fmt.Errorf("%s holds your settings password: ask for it first ('kc9k partner request settings-password'), or you would be locked out of your settings", p.Email))
			}
		}
		if err := c.RemovePartner(s, p.ID); err != nil {
			return err
		}
		for what, who := range holds {
			if strings.EqualFold(who, p.Email) {
				delete(holds, what)
			}
		}
		if err := savePartnerHolds(database, holds); err != nil {
			return err
		}
		fmt.Printf("%s is no longer your partner (they are told).\n", p.Email)
		return nil
	}),
}

var partnerHoldCmd = &cobra.Command{
	Use:   "hold settings-password <email>",
	Short: "Give the settings password to a partner to hold (KC9K makes it; you never see it)",
	Args:  cobra.ExactArgs(2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		if args[0] != holdSettingsPassword {
			return fmt.Errorf("a partner holds the settings-password here; for a block's lock use 'kc9k partner lock <block> <email>'")
		}
		if err := requireSettingsPassword(database); err != nil {
			return err
		}
		c, s, err := signedIn(database)
		if err != nil {
			return err
		}
		p, err := findPartner(c, s, args[1])
		if err != nil {
			return err
		}
		if !p.Accepted {
			return fmt.Errorf("%s hasn't accepted yet", p.Email)
		}
		if err := confirmLocks(cmd, lockToSet{Target: "the settings", Kind: "settings password only " + p.Email + " knows"}); err != nil {
			return err
		}
		secret := newPartnerSecret()
		hash, err := db.HashPassword(secret)
		if err != nil {
			return err
		}
		if err := c.Hold(s, p.ID, holdSettingsPassword, secret); err != nil {
			return err
		}
		if err := database.SetSetting(SettingSettingsPassword, hash); err != nil {
			return err
		}
		holds := partnerHolds(database)
		holds[holdSettingsPassword] = p.Email
		if err := savePartnerHolds(database, holds); err != nil {
			return err
		}
		fmt.Printf("%s now holds your settings password. Ask them with 'kc9k partner request settings-password'.\n", p.Email)
		return nil
	}),
}

var partnerLockCmd = &cobra.Command{
	Use:   "lock <block> <email>",
	Short: "Add a password lock to a block that only your partner knows (starts the block)",
	Args:  cobra.ExactArgs(2),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		block := args[0]
		if _, err := database.GetBlock(block); err != nil {
			return err
		}
		what := holdLockPrefix + block
		holds := partnerHolds(database)
		if who := holds[what]; who != "" {
			if hold, err := blockHold(database, block, time.Now()); err == nil && hold.held() {
				return fmt.Errorf("%s already holds a lock of block '%s'", who, block)
			}
		}
		c, s, err := signedIn(database)
		if err != nil {
			return err
		}
		p, err := findPartner(c, s, args[1])
		if err != nil {
			return err
		}
		if !p.Accepted {
			return fmt.Errorf("%s hasn't accepted yet", p.Email)
		}
		if err := confirmLocks(cmd, lockToSet{Target: "block '" + block + "'", Kind: "password lock only " + p.Email + " knows"}); err != nil {
			return err
		}
		secret := newPartnerSecret()
		if err := c.Hold(s, p.ID, what, secret); err != nil {
			return err
		}
		if err := withStdin(secret+"\n", func() error {
			return runQueued([]string{"start", block, "--password-stdin", "--yes"})
		}); err != nil {
			_ = c.Release(s, what)
			return err
		}
		holds[what] = p.Email
		if err := savePartnerHolds(database, holds); err != nil {
			return err
		}
		fmt.Printf("Block '%s' has a password lock only %s knows. Ask them with 'kc9k partner request %s'.\n", block, p.Email, block)
		return nil
	}),
}

// withStdin runs f with standard input's lines read from text.
func withStdin(text string, f func() error) error {
	old := stdinLines
	stdinLines = bufio.NewReader(strings.NewReader(text))
	defer func() { stdinLines = old }()
	return f()
}

var partnerRequestCmd = &cobra.Command{
	Use:   "request <block>|settings-password",
	Short: "Ask the partner holding its password to approve unlocking",
	Args:  cobra.ExactArgs(1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		reason, _ := cmd.Flags().GetString("reason")
		what := args[0]
		if what != holdSettingsPassword {
			what = holdLockPrefix + what
		}
		c, s, err := signedIn(database)
		if err != nil {
			return err
		}
		reqs, err := c.RequestUnlock(s, what, reason)
		if errors.Is(err, cloud.ErrNotFound) {
			return fmt.Errorf("no partner holds the password of %s", args[0])
		}
		if err != nil {
			return err
		}
		for _, r := range reqs {
			fmt.Printf("Asked %s (request %d). 'kc9k partner requests' unlocks once they approve.\n", r.Partner, r.ID)
		}
		return nil
	}),
}

var partnerRequestsCmd = &cobra.Command{
	Use:   "requests",
	Short: "Your unlock requests; an approved one is carried out now",
	Args:  cobra.NoArgs,
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		c, s, err := signedIn(database)
		if err != nil {
			return err
		}
		reqs, err := c.Requests(s)
		if err != nil {
			return err
		}
		shown := 0
		for _, r := range reqs {
			switch r.Status {
			case "pending":
				fmt.Printf("Request %d to %s (%s): waiting for their answer.\n", r.ID, r.Partner, r.What)
			case "denied":
				if time.Since(r.At) < 7*24*time.Hour {
					fmt.Printf("Request %d to %s (%s): denied.\n", r.ID, r.Partner, r.What)
				}
			case "approved":
				if err := useApproval(database, c, s, r); err != nil {
					fmt.Printf("Request %d to %s (%s): approved, but %v; try again later.\n", r.ID, r.Partner, r.What, err)
				}
			default:
				continue
			}
			shown++
		}
		if shown == 0 {
			fmt.Println("No open requests.")
		}
		return nil
	}),
}

// useApproval carries out an approved request: a block's partner lock is
// unlocked with the secret (never shown); the settings password is shown
// once, as the partner telling it, and no longer theirs to hold.
func useApproval(database *db.DB, c *cloud.Client, s *cloud.Session, r cloud.Request) error {
	holds := partnerHolds(database)
	if r.What == holdSettingsPassword {
		fmt.Printf("Request %d: %s approved. Your settings password is: %s\n", r.ID, r.Partner, r.Secret)
		fmt.Println("Change it, or give it to a partner again ('kc9k partner hold settings-password <email>').")
	} else {
		block := strings.TrimPrefix(r.What, holdLockPrefix)
		if err := withStdin(r.Secret+"\n", func() error { return runQueued([]string{"unlock", block}) }); err != nil {
			return err
		}
		fmt.Printf("Request %d: %s approved; block '%s' unlocked.\n", r.ID, r.Partner, block)
	}
	if err := c.UsedRequest(s, r.ID); err != nil {
		return err
	}
	_ = c.Release(s, r.What)
	delete(holds, r.What)
	return savePartnerHolds(database, holds)
}

var partnerReportCmd = &cobra.Command{
	Use:   "report",
	Short: "Send (or with --print only show) the report your partners get",
	Args:  cobra.NoArgs,
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		period, _ := cmd.Flags().GetString("period")
		if period != "daily" && period != "weekly" {
			return fmt.Errorf("--period is daily or weekly")
		}
		subject, body, err := partnerReport(database, period, time.Now())
		if err != nil {
			return err
		}
		if p, _ := cmd.Flags().GetBool("print"); p {
			fmt.Println(subject)
			fmt.Println()
			fmt.Println(body)
			return nil
		}
		c, s, err := signedIn(database)
		if err != nil {
			return err
		}
		n, err := c.SendToPartners(s, cloud.KindReport, period, subject, body)
		if err != nil {
			return err
		}
		fmt.Printf("Sent to %d partner(s) with %s reports.\n", n, period)
		return nil
	}),
}

// reportSpan is the last whole day or week (Monday to Sunday) before now.
func reportSpan(period string, now time.Time) (from, to time.Time) {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if period == "daily" {
		return today.AddDate(0, 0, -1), today
	}
	monday := today.AddDate(0, 0, -((int(today.Weekday()) + 6) % 7))
	return monday.AddDate(0, 0, -7), monday
}

// partnerReport is the report of the last day or week (§4.15: time per
// site and app, times blocked, top blocked).
func partnerReport(database *db.DB, period string, now time.Time) (subject, body string, err error) {
	from, to := reportSpan(period, now)
	rows, err := database.UsageSince(from.Format("2006-01-02"))
	if err != nil {
		return "", "", err
	}
	totals := map[string]map[string]int64{}
	for _, r := range rows {
		if r.Day >= to.Format("2006-01-02") {
			continue
		}
		if totals[r.Kind] == nil {
			totals[r.Kind] = map[string]int64{}
		}
		totals[r.Kind][r.Name] += r.Seconds
	}
	top := func(m map[string]int64, n int) []string {
		names := make([]string, 0, len(m))
		for k := range m {
			names = append(names, k)
		}
		sort.Slice(names, func(i, j int) bool {
			if m[names[i]] != m[names[j]] {
				return m[names[i]] > m[names[j]]
			}
			return names[i] < names[j]
		})
		return names[:min(n, len(names))]
	}
	var b strings.Builder
	span := from.Format("Mon 2 Jan")
	if period == "weekly" {
		span += " to " + to.AddDate(0, 0, -1).Format("Mon 2 Jan")
	}
	for _, part := range []struct{ kind, title string }{{db.UsageSite, "Time on sites"}, {db.UsageApp, "Time in apps"}} {
		fmt.Fprintf(&b, "%s:\n", part.title)
		names := top(totals[part.kind], 10)
		if len(names) == 0 {
			b.WriteString("  none recorded\n")
		}
		for _, n := range names {
			fmt.Fprintf(&b, "  %-30s %s\n", n, humanDuration(time.Duration(totals[part.kind][n])*time.Second))
		}
	}
	var blocked int64
	for _, v := range totals[db.UsageBlocked] {
		blocked += v
	}
	fmt.Fprintf(&b, "Times blocked: %d\n", blocked)
	if names := top(totals[db.UsageBlocked], 5); len(names) > 0 {
		b.WriteString("Most blocked:\n")
		for _, n := range names {
			fmt.Fprintf(&b, "  %-30s %d\n", n, totals[db.UsageBlocked][n])
		}
	}
	if names := top(totals[db.UsageBlockedBlock], 10); len(names) > 0 {
		b.WriteString("Blocked by block:\n")
		for _, n := range names {
			fmt.Fprintf(&b, "  %-30s %d\n", n, totals[db.UsageBlockedBlock][n])
		}
	}
	return fmt.Sprintf("Killer Chicken 9000 %s report: %s", period, span), b.String(), nil
}

// Partner's side ---------------------------------------------------------

var partnerInboxCmd = &cobra.Command{
	Use:   "inbox",
	Short: "As a partner: invitations, reports and alerts, passwords you hold, requests to answer",
	Args:  cobra.NoArgs,
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		asJSON, _ := cmd.Flags().GetBool("json")
		c, s, err := signedIn(database)
		if err != nil {
			return err
		}
		in, err := c.PartnerInbox(s)
		if err != nil {
			return err
		}
		if asJSON {
			data, _ := json.Marshal(in)
			fmt.Println(string(data))
			return nil
		}
		for _, i := range in.Invitations {
			fmt.Printf("Invitation from %s: 'kc9k partner accept %s'\n", i.From, i.From)
		}
		for _, w := range in.Watching {
			seen := "never heard from"
			if !w.LastSeen.IsZero() {
				seen = "last heard from " + when(w.LastSeen) + " (" + w.Device + ")"
			}
			fmt.Printf("You are %s's partner; their KC9K was %s.\n", w.Email, seen)
		}
		for _, h := range in.Held {
			fmt.Printf("You hold %s's %s: %s\n", h.From, h.What, h.Secret)
		}
		for _, r := range in.Requests {
			fmt.Printf("Request %d from %s to unlock %s: %q ('kc9k partner approve %d' or 'deny %d')\n", r.ID, r.From, r.What, r.Reason, r.ID, r.ID)
		}
		for _, m := range in.Messages {
			fmt.Printf("%s  %s: %s\n", when(m.At), m.From, m.Subject)
		}
		return nil
	}),
}

var partnerAcceptCmd = &cobra.Command{
	Use:   "accept <email>",
	Short: "As a partner: accept the invitation of the person with this address",
	Args:  cobra.ExactArgs(1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		c, s, err := signedIn(database)
		if err != nil {
			return err
		}
		in, err := c.PartnerInbox(s)
		if err != nil {
			return err
		}
		for _, i := range in.Invitations {
			if strings.EqualFold(i.From, args[0]) || i.ID == args[0] {
				if err := c.AcceptInvitation(s, i.ID); err != nil {
					return err
				}
				fmt.Printf("You are now %s's accountability partner.\n", i.From)
				return nil
			}
		}
		return fmt.Errorf("no invitation from %s", args[0])
	}),
}

func answerCmd(approve bool) *cobra.Command {
	verb := map[bool]string{true: "approve", false: "deny"}[approve]
	return &cobra.Command{
		Use:   verb + " <request>",
		Short: "As a partner: " + verb + " an unlock request",
		Args:  cobra.ExactArgs(1),
		RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("give the request's number ('kc9k partner inbox')")
			}
			c, s, err := signedIn(database)
			if err != nil {
				return err
			}
			if err := c.AnswerRequest(s, id, approve); err != nil {
				if errors.Is(err, cloud.ErrNotFound) {
					return fmt.Errorf("no open request %d for you", id)
				}
				return err
			}
			fmt.Printf("Request %d %s.\n", id, map[bool]string{true: "approved", false: "denied"}[approve])
			return nil
		}),
	}
}

// Alerts and the daemon -------------------------------------------------

// partnerAlertCommands are the commands whose success is an alert, with
// the alert's words (§4.15: a block switched off, unlocked or deleted, a
// lock ended early, KC9K uninstalled).
var partnerAlertCommands = map[string]string{
	"stop":              "Block '%s' was switched off",
	"unlock":            "A lock of '%s' was unlocked",
	"block remove":      "Block '%s' was deleted",
	"master":            "Every lock ended early (the master password)",
	"recover":           "Every lock ended early (recovery from the boot menu)",
	"prepare-uninstall": "Killer Chicken 9000 is being uninstalled",
}

// noteForPartners queues an alert after a command that succeeded (called
// by Execute). It never fails the command.
func noteForPartners(c *cobra.Command, args []string) {
	path := strings.TrimPrefix(c.CommandPath(), c.Root().Name()+" ")
	text, ok := partnerAlertCommands[path]
	if path == "toggle" && len(args) > 0 {
		text, ok = "Block '%s' was switched off", true
	}
	if !ok {
		return
	}
	database, err := db.OpenDB(db.ResolveDBPath())
	if err != nil {
		return
	}
	defer database.Close()
	if path == "toggle" {
		if b, err := database.GetBlock(args[0]); err != nil || b.Active {
			return // switched on
		}
	}
	if strings.Contains(text, "%s") {
		name := ""
		if len(args) > 0 {
			name = args[0]
		}
		text = fmt.Sprintf(text, name)
	}
	_ = database.QueuePartnerAlert(text, "")
	if path == "prepare-uninstall" {
		partnerTick(database, time.Now()) // the daemon is about to go
	}
}

// partnerTick is the daemon's work for the partners, every few minutes:
// queued alerts sent, a heartbeat, reports when due, held passwords of
// locks that ended released. Without an account or accepted partners the
// alerts are dropped unsent (principle 6).
func partnerTick(database *db.DB, now time.Time) {
	alerts, _ := database.PartnerAlerts()
	c, s, err := signedIn(database)
	if err != nil {
		if len(alerts) > 0 {
			_ = database.DropPartnerAlerts(alerts[len(alerts)-1].ID)
		}
		return
	}
	ps, err := c.Partners(s)
	if err != nil {
		return // offline: try again next time
	}
	if !slices.ContainsFunc(ps, func(p cloud.Partner) bool { return p.Accepted }) {
		if len(alerts) > 0 {
			_ = database.DropPartnerAlerts(alerts[len(alerts)-1].ID)
		}
		return
	}
	last, _ := database.GetSetting(settingPartnerHeartbeat)
	if t, _ := strconv.ParseInt(last, 10, 64); now.Unix()-t >= 15*60 {
		if c.Heartbeat(s, deviceName()) == nil {
			_ = database.SetSetting(settingPartnerHeartbeat, strconv.FormatInt(now.Unix(), 10))
		}
	}
	for i, a := range alerts {
		subject, body := "Killer Chicken 9000: "+a.Subject, a.At.Local().Format("2006-01-02 15:04")+" on "+deviceName()+"\n"+a.Body
		if len(alerts)-i > 20 { // a long queue: the rest in one message
			var lines []string
			for _, r := range alerts[i:] {
				lines = append(lines, r.At.Local().Format("2006-01-02 15:04")+"  "+r.Subject+" "+r.Body)
			}
			subject, body = fmt.Sprintf("Killer Chicken 9000: %d alerts", len(alerts)-i), strings.Join(lines, "\n")
		}
		if _, err := c.SendToPartners(s, cloud.KindAlert, "", subject, body); err != nil {
			return
		}
		_ = database.DropPartnerAlerts(a.ID)
		if len(alerts)-i > 20 {
			_ = database.DropPartnerAlerts(alerts[len(alerts)-1].ID)
			break
		}
	}
	sent := map[string]string{}
	if v, _ := database.GetSetting(settingPartnerReports); v != "" {
		_ = json.Unmarshal([]byte(v), &sent)
	}
	for _, period := range []string{"daily", "weekly"} {
		from, _ := reportSpan(period, now)
		day := from.Format("2006-01-02")
		if sent[period] >= day {
			continue
		}
		subject, body, err := partnerReport(database, period, now)
		if err != nil {
			continue
		}
		if _, err := c.SendToPartners(s, cloud.KindReport, period, subject, body); err == nil {
			sent[period] = day
		}
	}
	if data, err := json.Marshal(sent); err == nil {
		_ = database.SetSetting(settingPartnerReports, string(data))
	}
	holds := partnerHolds(database)
	changed := false
	for what := range holds {
		block, isLock := strings.CutPrefix(what, holdLockPrefix)
		if !isLock {
			continue
		}
		if hold, err := blockHold(database, block, now); err == nil && hold.held() {
			continue
		}
		if c.Release(s, what) == nil {
			delete(holds, what)
			changed = true
		}
	}
	if changed {
		_ = savePartnerHolds(database, holds)
	}
}

func init() {
	partnerInviteCmd.Flags().String("reports", "weekly", "Reports: daily, weekly or off")
	partnerInviteCmd.Flags().Bool("alerts", true, "Alerts as things happen")
	partnerSetCmd.Flags().String("reports", "", "Reports: daily, weekly or off")
	partnerSetCmd.Flags().Bool("alerts", true, "Alerts as things happen")
	partnerListCmd.Flags().Bool("json", false, "Print as JSON")
	partnerInboxCmd.Flags().Bool("json", false, "Print as JSON")
	partnerRequestCmd.Flags().String("reason", "", "Why, for your partner")
	partnerReportCmd.Flags().String("period", "weekly", "daily or weekly")
	partnerReportCmd.Flags().Bool("print", false, "Only show it")
	addYesFlag(partnerHoldCmd, partnerLockCmd)
	partnerCmd.AddCommand(partnerInviteCmd, partnerListCmd, partnerSetCmd, partnerRemoveCmd, partnerHoldCmd,
		partnerLockCmd, partnerRequestCmd, partnerRequestsCmd, partnerReportCmd, partnerInboxCmd, partnerAcceptCmd,
		answerCmd(true), answerCmd(false))
	rootCmd.AddCommand(partnerCmd)
	daemon.PartnerTick = partnerTick
}
