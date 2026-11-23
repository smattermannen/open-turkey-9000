// partner.go — the accountability partner (SPEC §4.15, ROADMAP 2.13).
//
//	kc9k partner [--json]                  # partners, invitations, whom you partner
//	kc9k partner invite EMAIL              # ask someone to be your partner
//	kc9k partner accept EMAIL              # be EMAIL's partner (as the partner)
//	kc9k partner leave EMAIL               # stop being EMAIL's partner
//	kc9k partner remove EMAIL              # weakening: refused while anything is locked
//	kc9k partner reports [daily|weekly|off]  # less often weakens
//	kc9k partner send                      # send what is queued now (the daemon does every minute)
//	kc9k partner inbox [--json]            # as a partner: reports, alerts, passwords you hold
//	kc9k partner request BLOCK|settings    # ask your partners to unlock
//	kc9k partner requests [--json]         # yours and those you may decide
//	kc9k partner approve|deny ID           # as a partner
//	kc9k partner unlock BLOCK|settings     # use an approved request
//
// A partner gets reports (daily or weekly: time per site and app, times
// blocked, top blocked) and alerts as they happen (bypass prevention, a
// block switched off, unlocked or deleted, tamper events, KC9K uninstalled;
// a device not heard from is the cloud's to notice). A partner can hold
// password locks and the settings password (--partner-holds on start and
// 'password set': KC9K makes the password up and sends it only to the
// partners) and approves unlock requests, which hand the password over
// for one use. Everything goes through KC9K's cloud (internal/cloud), the
// emails too (an owner step).
package cli

import (
	"bufio"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/smattermannen/kc9k/internal/cloud"
	"github.com/smattermannen/kc9k/internal/daemon"
	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
	"github.com/spf13/cobra"
)

const (
	settingPartnerReports    = "partner.reports"     // daily, weekly (default) or off
	settingPartnerLastReport = "partner.report.last" // unix seconds of the last report sent
	settingPartnerHeartbeat  = "partner.heartbeat"   // unix seconds of the last heartbeat
	settingPartnerChecked    = "partner.checked"     // unix seconds partners were last read
)

// Report frequencies, strongest first.
var partnerReportKinds = []string{"daily", "weekly", "off"}

func partnerReports(database *db.DB) string {
	if v, _ := database.GetSetting(settingPartnerReports); slices.Contains(partnerReportKinds, v) {
		return v
	}
	return "weekly"
}

// partnerView is `kc9k partner --json`, for the GUIs.
type partnerView struct {
	SignedIn  bool            `json:"signed_in"`
	Email     string          `json:"email,omitempty"`
	Reports   string          `json:"reports"`
	Partners  []cloud.Partner `json:"partners"`  // yours
	Partnered []cloud.Partner `json:"partnered"` // you are theirs (or invited)
	Queued    int             `json:"queued"`    // reports and alerts not sent yet
	Error     string          `json:"error,omitempty"`
}

// readPartners reads the partnerships and remembers whether any of the
// user's is active (alerts are queued only then).
func readPartners(database *db.DB, c *cloud.Client, s *cloud.Session) (mine, theirs []cloud.Partner, err error) {
	all, err := c.Partners(s)
	if err != nil {
		return nil, nil, err
	}
	active := false
	for _, p := range all {
		if p.OwnerID == s.UserID {
			mine = append(mine, p)
			active = active || p.Status == cloud.PartnerActive
		} else {
			theirs = append(theirs, p)
		}
	}
	_ = database.SetSetting(db.SettingPartnerActive, onOff(active))
	_ = database.SetSetting(settingPartnerChecked, fmt.Sprint(time.Now().Unix()))
	return mine, theirs, nil
}

func findPartner(list []cloud.Partner, email string, owner bool) (cloud.Partner, bool) {
	for _, p := range list {
		if owner && strings.EqualFold(p.PartnerEmail, email) || !owner && strings.EqualFold(p.OwnerEmail, email) {
			return p, true
		}
	}
	return cloud.Partner{}, false
}

var partnerCmd = &cobra.Command{
	Use:   "partner",
	Short: "Accountability partner: someone who gets your reports and alerts and can hold your passwords",
	Long: `An accountability partner is someone you share your Killer Chicken with
(they need a KC9K account too). They get reports (daily or weekly: time per
site and app, times blocked, top blocked) and alerts as they happen (bypass
prevention, a block switched off, unlocked or deleted, tamper events, KC9K
uninstalled), can hold password locks and the settings password
('kc9k start B --password-stdin --partner-holds', 'kc9k password set
--partner-holds': you never see the password) and approve your unlock
requests. Removing a partner is weakening: refused while anything is
locked. Without arguments, lists your partners.`,
	Args: cobra.NoArgs,
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		asJSON, _ := cmd.Flags().GetBool("json")
		view := partnerView{Reports: partnerReports(database), Partners: []cloud.Partner{}, Partnered: []cloud.Partner{}}
		if out, err := database.PartnerOutbox(); err == nil {
			view.Queued = len(out)
		}
		c, s, err := signedIn(database)
		if err == nil {
			view.SignedIn, view.Email = true, s.Email
			var mine, theirs []cloud.Partner
			if mine, theirs, err = readPartners(database, c, s); err == nil {
				view.Partners, view.Partnered = append(view.Partners, mine...), append(view.Partnered, theirs...)
			}
		}
		if err != nil {
			view.Error = err.Error()
		}
		if asJSON {
			data, _ := json.Marshal(view)
			fmt.Println(string(data))
			return nil
		}
		if !view.SignedIn {
			return err
		}
		if len(view.Partners) == 0 {
			fmt.Println("No accountability partner yet ('kc9k partner invite EMAIL').")
		}
		for _, p := range view.Partners {
			fmt.Printf("Your partner: %s (%s)\n", p.PartnerEmail, map[string]string{cloud.PartnerActive: "active",
				cloud.PartnerInvited: "invited, not accepted yet"}[p.Status])
		}
		for _, p := range view.Partnered {
			if p.Status == cloud.PartnerActive {
				fmt.Printf("You are the partner of %s.\n", p.OwnerEmail)
			} else {
				fmt.Printf("%s invites you to be their partner ('kc9k partner accept %s').\n", p.OwnerEmail, p.OwnerEmail)
			}
		}
		fmt.Printf("Reports: %s. Waiting to be sent: %d.\n", view.Reports, view.Queued)
		return err
	}),
}

var partnerInviteCmd = &cobra.Command{
	Use:   "invite <email>",
	Short: "Invite someone to be your accountability partner (they accept with their own KC9K account)",
	Args:  cobra.ExactArgs(1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		c, s, err := signedIn(database)
		if err != nil {
			return err
		}
		p, err := c.InvitePartner(s, args[0])
		if err != nil {
			return err
		}
		fmt.Printf("Invited %s. Once they accept ('kc9k partner accept %s' on their KC9K), they get your reports and alerts.\n",
			p.PartnerEmail, s.Email)
		return nil
	}),
}

var partnerAcceptCmd = &cobra.Command{
	Use:   "accept <email>",
	Short: "Accept an invitation: become the accountability partner of the user with this email",
	Args:  cobra.ExactArgs(1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		c, s, err := signedIn(database)
		if err != nil {
			return err
		}
		_, theirs, err := readPartners(database, c, s)
		if err != nil {
			return err
		}
		p, ok := findPartner(theirs, args[0], false)
		if !ok {
			return fmt.Errorf("%s hasn't invited %s", args[0], s.Email)
		}
		if p.Status == cloud.PartnerActive {
			fmt.Printf("You are already the partner of %s.\n", p.OwnerEmail)
			return nil
		}
		if err := c.AcceptPartner(s, p.ID); err != nil {
			return err
		}
		fmt.Printf("You are now the accountability partner of %s.\n", p.OwnerEmail)
		return nil
	}),
}

var partnerLeaveCmd = &cobra.Command{
	Use:   "leave <email>",
	Short: "Stop being the accountability partner of the user with this email (or decline their invitation)",
	Args:  cobra.ExactArgs(1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		c, s, err := signedIn(database)
		if err != nil {
			return err
		}
		_, theirs, err := readPartners(database, c, s)
		if err != nil {
			return err
		}
		p, ok := findPartner(theirs, args[0], false)
		if !ok {
			return fmt.Errorf("you aren't the partner of %s", args[0])
		}
		if err := c.RemovePartner(s, p.ID); err != nil {
			return err
		}
		fmt.Printf("You are no longer the partner of %s.\n", p.OwnerEmail)
		return nil
	}),
}

// gatePartnerWeakening refuses what weakens the partnership (removing an
// active partner, fewer reports) as tamper protection's off: the settings
// password, and never while anything is locked (§4.15: removing one is
// weakening; R-WEAK-1/2).
func gatePartnerWeakening(database *db.DB, what string) error {
	if err := requireSettingsPassword(database); err != nil {
		return err
	}
	now := time.Now()
	trusted, err := database.TrustedNow(now)
	if err != nil {
		return err
	}
	if locked, err := enforce.AnyLock(database, now, trusted); err != nil {
		return err
	} else if locked {
		return refused(database, "", fmt.Errorf("%s weakens your accountability: refused while a block or setting is locked", what))
	}
	return nil
}

var partnerRemoveCmd = &cobra.Command{
	Use:   "remove <email>",
	Short: "Remove an accountability partner (weakening: refused while anything is locked; they are told)",
	Args:  cobra.ExactArgs(1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		c, s, err := signedIn(database)
		if err != nil {
			return err
		}
		mine, _, err := readPartners(database, c, s)
		if err != nil {
			return err
		}
		p, ok := findPartner(mine, args[0], true)
		if !ok {
			return fmt.Errorf("%s isn't your partner", args[0])
		}
		if p.Status == cloud.PartnerActive {
			if err := gatePartnerWeakening(database, "removing your partner "+p.PartnerEmail); err != nil {
				return err
			}
			// They hear it first, with whatever is still queued.
			database.PartnerAlert(s.Email+" removed "+p.PartnerEmail+" as accountability partner",
				p.PartnerEmail+" no longer gets reports and alerts from "+s.Email+".")
			if _, err := partnerFlush(database, c, s, time.Now(), false); err != nil {
				return fmt.Errorf("your partners can't be told, so %s stays: %w", p.PartnerEmail, err)
			}
		}
		if err := c.RemovePartner(s, p.ID); err != nil {
			return err
		}
		_, _, _ = readPartners(database, c, s)
		fmt.Printf("%s is no longer your partner.\n", p.PartnerEmail)
		return nil
	}),
}

var partnerReportsCmd = &cobra.Command{
	Use:   "reports [daily|weekly|off]",
	Short: "How often your partners get a report (less often weakens)",
	Args:  cobra.MaximumNArgs(1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		cur := partnerReports(database)
		if len(args) == 0 {
			fmt.Printf("Reports: %s.\n", cur)
			return nil
		}
		want := args[0]
		if !slices.Contains(partnerReportKinds, want) {
			return fmt.Errorf("use daily, weekly or off")
		}
		if slices.Index(partnerReportKinds, want) > slices.Index(partnerReportKinds, cur) {
			if err := gatePartnerWeakening(database, "fewer reports"); err != nil {
				return err
			}
		}
		if err := database.SetSetting(settingPartnerReports, want); err != nil {
			return err
		}
		fmt.Printf("Reports: %s.\n", want)
		return nil
	}),
}

var partnerSendCmd = &cobra.Command{
	Use:   "send",
	Short: "Send the queued alerts, a report that is due and this device's sign of life now",
	Args:  cobra.NoArgs,
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		c, s, err := signedIn(database)
		if err != nil {
			return err
		}
		n, err := partnerFlush(database, c, s, time.Now(), true)
		if err != nil {
			return err
		}
		fmt.Printf("Sent %d to your partners.\n", n)
		return nil
	}),
}

// partnerFlush sends this device's heartbeat (at most every 10 minutes, or
// always when forced), the report if one is due, and the outbox. It
// returns how many messages went.
func partnerFlush(database *db.DB, c *cloud.Client, s *cloud.Session, now time.Time, force bool) (int, error) {
	unix := func(key string) int64 {
		var n int64
		v, _ := database.GetSetting(key)
		fmt.Sscan(v, &n)
		return n
	}
	if force || now.Unix()-unix(settingPartnerHeartbeat) >= 600 {
		if err := c.Heartbeat(s, deviceName()); err != nil {
			return 0, err
		}
		_ = database.SetSetting(settingPartnerHeartbeat, fmt.Sprint(now.Unix()))
	}
	if force || now.Unix()-unix(settingPartnerChecked) >= 1800 {
		if _, _, err := readPartners(database, c, s); err != nil {
			return 0, err
		}
	}
	if v, _ := database.GetSetting(db.SettingPartnerActive); v != "on" {
		return 0, nil
	}
	sent := 0
	if every := map[string]int64{"daily": 86400, "weekly": 7 * 86400}[partnerReports(database)]; every > 0 {
		last := unix(settingPartnerLastReport)
		if last == 0 { // the first report covers one period, a period from now
			last = now.Unix()
			_ = database.SetSetting(settingPartnerLastReport, fmt.Sprint(last))
		}
		if now.Unix()-last >= every {
			subject, body := partnerReport(database, s.Email, time.Unix(last, 0), now)
			if err := c.Send(s, []cloud.Message{{Kind: cloud.MessageReport, Subject: subject, Body: body}}); err != nil {
				return 0, err
			}
			_ = database.SetSetting(settingPartnerLastReport, fmt.Sprint(now.Unix()))
			sent++
		}
	}
	out, err := database.PartnerOutbox()
	if err != nil || len(out) == 0 {
		return sent, err
	}
	msgs := make([]cloud.Message, len(out))
	for i, m := range out {
		msgs[i] = cloud.Message{Kind: m.Kind, Subject: m.Subject, Body: m.Body, CreatedAt: time.Unix(m.At, 0).UTC()}
	}
	if err := c.Send(s, msgs); err != nil {
		return sent, err
	}
	return sent + len(out), database.DropPartnerMessages(out[len(out)-1].ID)
}

// partnerReport is the report since from (§4.15): time per site and app,
// times blocked, top blocked.
func partnerReport(database *db.DB, email string, from, now time.Time) (subject, body string) {
	rows, _ := database.UsageSince(enforce.Day(from))
	totals := map[string]map[string]int64{}
	for _, r := range rows {
		if totals[r.Kind] == nil {
			totals[r.Kind] = map[string]int64{}
		}
		totals[r.Kind][r.Name] += r.Seconds
	}
	top := func(kind string, n int) ([]string, int64) {
		var names []string
		var sum int64
		for name, v := range totals[kind] {
			names = append(names, name)
			sum += v
		}
		sort.Slice(names, func(i, j int) bool {
			a, b := totals[kind][names[i]], totals[kind][names[j]]
			return a > b || a == b && names[i] < names[j]
		})
		if len(names) > n {
			names = names[:n]
		}
		return names, sum
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Killer Chicken 9000 report for %s, %s to %s.\n", email, from.Format("2006-01-02 15:04"), now.Format("2006-01-02 15:04"))
	for _, k := range []struct{ kind, title string }{{db.UsageSite, "Time on websites"}, {db.UsageApp, "Time in apps"}} {
		names, sum := top(k.kind, 10)
		fmt.Fprintf(&b, "\n%s: %s\n", k.title, formatSpent(sum))
		for _, name := range names {
			fmt.Fprintf(&b, "  %-40s %s\n", name, formatSpent(totals[k.kind][name]))
		}
	}
	names, sum := top(db.UsageBlocked, 10)
	fmt.Fprintf(&b, "\nTimes blocked: %d\n", sum)
	for _, name := range names {
		fmt.Fprintf(&b, "  %-40s %d\n", name, totals[db.UsageBlocked][name])
	}
	if events, err := database.RecentEvents(from.Unix(), 1000); err == nil {
		count := map[string]int{}
		for _, e := range events {
			count[e.Kind]++
		}
		if count[db.EventRefused] > 0 || count[db.EventTamper] > 0 {
			fmt.Fprintf(&b, "\nRefused weakening attempts: %d. Tamper events: %d.\n", count[db.EventRefused], count[db.EventTamper])
		}
	}
	return "Killer Chicken 9000 report for " + email, b.String()
}

var partnerInboxCmd = &cobra.Command{
	Use:   "inbox",
	Short: "As a partner: the reports, alerts and passwords of the users you partner",
	Args:  cobra.NoArgs,
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		c, s, err := signedIn(database)
		if err != nil {
			return err
		}
		msgs, err := c.Inbox(s)
		if err != nil {
			return err
		}
		var theirs []cloud.Message
		for _, m := range msgs {
			if m.OwnerID != s.UserID {
				theirs = append(theirs, m)
			}
		}
		if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
			data, _ := json.Marshal(append([]cloud.Message{}, theirs...))
			fmt.Println(string(data))
			return nil
		}
		if len(theirs) == 0 {
			fmt.Println("Nothing from the users you partner.")
		}
		for _, m := range theirs {
			fmt.Printf("%s  %s  %s\n", when(m.CreatedAt), m.OwnerEmail, m.Subject)
			if m.Kind == cloud.MessagePassword {
				fmt.Printf("  You hold the password of %s: %s\n", targetWords(m.Target), m.Secret)
			} else if m.Kind == cloud.MessageAlert {
				fmt.Printf("  %s\n", strings.ReplaceAll(strings.TrimSpace(m.Body), "\n", "\n  "))
			}
		}
		return nil
	}),
}

// partnerTarget turns "settings", a block's name or a lockable setting's
// (browser or tamper protection) into a request target.
func partnerTarget(database *db.DB, arg string) (string, error) {
	if arg == "settings" {
		return "settings", nil
	}
	if _, err := database.GetBlock(arg); err == nil {
		return "block:" + arg, nil
	}
	if name := strings.TrimPrefix(arg, "setting:"); enforce.IsLockableSetting(name) {
		return "setting:" + name, nil
	}
	return "", fmt.Errorf("no block or lockable setting %q (or 'settings' for the settings password)", arg)
}

func targetWords(target string) string {
	if name, ok := strings.CutPrefix(target, "block:"); ok {
		return "block '" + name + "' (password lock)"
	}
	if name, ok := strings.CutPrefix(target, "setting:"); ok {
		return "setting " + name + " (password lock)"
	}
	if target == "settings" {
		return "the settings password"
	}
	return target
}

// partnerHeldPassword makes up a password that only the user's active
// partners get (--partner-holds): sent before anything is locked with it,
// never shown or kept here.
func partnerHeldPassword(database *db.DB, targets ...string) (string, error) {
	c, s, err := signedIn(database)
	if err != nil {
		return "", fmt.Errorf("--partner-holds needs your KC9K account: %w", err)
	}
	mine, _, err := readPartners(database, c, s)
	if err != nil {
		return "", err
	}
	var to []string
	for _, p := range mine {
		if p.Status == cloud.PartnerActive {
			to = append(to, p.PartnerEmail)
		}
	}
	if len(to) == 0 {
		return "", errors.New("--partner-holds needs an active accountability partner ('kc9k partner invite EMAIL')")
	}
	const letters = "abcdefghjkmnpqrstuvwxyz23456789"
	pw := make([]byte, 16)
	for i := range pw {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		if err != nil {
			return "", err
		}
		pw[i] = letters[n.Int64()]
	}
	password := string(pw)
	var msgs []cloud.Message // one per lock it opens (a mode's blocks share one)
	for _, target := range targets {
		msgs = append(msgs, cloud.Message{Kind: cloud.MessagePassword, Target: target,
			Subject: "You hold a password for " + s.Email,
			Body: s.Email + " locked " + targetWords(target) + " with a password only you have. They can ask you to unlock " +
				"('kc9k partner requests'); approving hands it over for one use.",
			Secret: password})
	}
	if err := c.Send(s, msgs); err != nil {
		return "", fmt.Errorf("the password couldn't reach your partners, so nothing was locked: %w", err)
	}
	fmt.Printf("The password went to %s; you won't see it.\n", strings.Join(to, ", "))
	return password, nil
}

var partnerRequestCmd = &cobra.Command{
	Use:   "request <block|setting|settings>",
	Short: "Ask your partners to unlock a block's or setting's password lock, or lift the settings password, they hold",
	Args:  cobra.ExactArgs(1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		target, err := partnerTarget(database, args[0])
		if err != nil {
			return err
		}
		c, s, err := signedIn(database)
		if err != nil {
			return err
		}
		reason, _ := cmd.Flags().GetString("reason")
		q, err := c.RequestUnlock(s, target, reason)
		if err != nil {
			return err
		}
		fmt.Printf("Asked your partners to unlock %s (request %s). Once one approves: 'kc9k partner unlock %s'.\n",
			targetWords(target), q.ID, args[0])
		return nil
	}),
}

var partnerRequestsCmd = &cobra.Command{
	Use:   "requests",
	Short: "Unlock requests: yours, and those of the users you partner",
	Args:  cobra.NoArgs,
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		c, s, err := signedIn(database)
		if err != nil {
			return err
		}
		list, err := c.UnlockRequests(s)
		if err != nil {
			return err
		}
		for i := range list {
			list[i].Secret = "" // never printed: 'partner unlock' uses it
		}
		if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
			data, _ := json.Marshal(append([]cloud.UnlockRequest{}, list...))
			fmt.Println(string(data))
			return nil
		}
		if len(list) == 0 {
			fmt.Println("No unlock requests.")
		}
		for _, q := range list {
			who := "You"
			if q.OwnerID != s.UserID {
				who = q.OwnerEmail
			}
			fmt.Printf("%s  %s  %s ask to unlock %s: %s", q.ID, when(q.CreatedAt), who, targetWords(q.Target), q.Status)
			if q.DecidedBy != "" {
				fmt.Printf(" by %s", q.DecidedBy)
			}
			if q.Reason != "" {
				fmt.Printf(" (%s)", q.Reason)
			}
			fmt.Println()
		}
		return nil
	}),
}

func decideCmd(approve bool) *cobra.Command {
	verb := map[bool]string{true: "approve", false: "deny"}[approve]
	return &cobra.Command{
		Use: verb + " <request id>",
		Short: map[bool]string{true: "As a partner: approve an unlock request (hands the password you hold over, for one use)",
			false: "As a partner: deny an unlock request"}[approve],
		Args: cobra.ExactArgs(1),
		RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
			c, s, err := signedIn(database)
			if err != nil {
				return err
			}
			list, err := c.UnlockRequests(s)
			if err != nil {
				return err
			}
			i := slices.IndexFunc(list, func(q cloud.UnlockRequest) bool { return q.ID == args[0] && q.OwnerID != s.UserID })
			if i < 0 {
				return fmt.Errorf("no request %s from a user you partner", args[0])
			}
			q := list[i]
			if q.Status != cloud.RequestPending {
				return fmt.Errorf("request %s is %s already", q.ID, q.Status)
			}
			secret := ""
			if approve {
				msgs, err := c.Inbox(s)
				if err != nil {
					return err
				}
				// The newest password that user sent for that lock.
				for _, m := range msgs {
					if m.OwnerID == q.OwnerID && m.Kind == cloud.MessagePassword && m.Target == q.Target {
						secret = m.Secret
						break
					}
				}
				if secret == "" {
					return fmt.Errorf("you hold no password for %s of %s", targetWords(q.Target), q.OwnerEmail)
				}
			}
			if err := c.DecideUnlock(s, q.ID, approve, secret); err != nil {
				return err
			}
			fmt.Printf("%s: %s's request to unlock %s.\n", map[bool]string{true: "Approved", false: "Denied"}[approve],
				q.OwnerEmail, targetWords(q.Target))
			return nil
		}),
	}
}

var partnerUnlockCmd = &cobra.Command{
	Use:   "unlock <block|setting|settings>",
	Short: "Use an approved unlock request: the block's or setting's password lock is unlocked, or the settings password removed",
	Args:  cobra.ExactArgs(1),
	RunE: withDB(func(cmd *cobra.Command, database *db.DB, args []string) error {
		target, err := partnerTarget(database, args[0])
		if err != nil {
			return err
		}
		c, s, err := signedIn(database)
		if err != nil {
			return err
		}
		list, err := c.UnlockRequests(s)
		if err != nil {
			return err
		}
		i := slices.IndexFunc(list, func(q cloud.UnlockRequest) bool {
			return q.OwnerID == s.UserID && q.Target == target && q.Status == cloud.RequestApproved
		})
		if i < 0 {
			return fmt.Errorf("no approved request to unlock %s ('kc9k partner request %s')", targetWords(target), args[0])
		}
		q := list[i]
		if target == "settings" {
			hash, _ := database.GetSetting(SettingSettingsPassword)
			if hash == "" || !db.CheckPassword(hash, q.Secret) {
				return fmt.Errorf("the approved password isn't the settings password (changed since?)")
			}
			if err := database.SetSetting(SettingSettingsPassword, ""); err != nil {
				return err
			}
			fmt.Printf("The settings password is removed (approved by %s).\n", q.DecidedBy)
		} else {
			// The unlock command, with the partner's password on its input
			// (the settings password, if one is set, asked here first: the
			// unlock then doesn't ask again).
			if err := requireSettingsPassword(database); err != nil {
				return err
			}
			old := stdinLines
			stdinLines = newLineReader(q.Secret + "\n")
			if name, ok := strings.CutPrefix(target, "setting:"); ok {
				err = unlockBrowserSetting(database, name)
			} else {
				err = runQueued([]string{"unlock", strings.TrimPrefix(target, "block:")})
			}
			stdinLines = old
			if err != nil {
				return err
			}
		}
		if err := c.UseUnlock(s, q.ID); err != nil {
			return err
		}
		database.PartnerAlert(s.Email+" used an approved unlock: "+targetWords(target), "Approved by "+q.DecidedBy+".")
		return nil
	}),
}

// partnerTick is the daemon's minute: alerts, a due report and the
// heartbeat reach the cloud while signed in (quietly otherwise).
func partnerTick(database *db.DB) {
	if loadSession(database) == nil {
		return
	}
	c, s, err := signedIn(database)
	if err != nil {
		return
	}
	_, _ = partnerFlush(database, c, s, time.Now(), false)
}

func init() {
	partnerCmd.Flags().Bool("json", false, "Print as JSON")
	partnerInboxCmd.Flags().Bool("json", false, "Print as JSON")
	partnerRequestsCmd.Flags().Bool("json", false, "Print as JSON")
	partnerRequestCmd.Flags().String("reason", "", "Why you ask (your partners see it)")
	partnerCmd.AddCommand(partnerInviteCmd, partnerAcceptCmd, partnerLeaveCmd, partnerRemoveCmd, partnerReportsCmd,
		partnerSendCmd, partnerInboxCmd, partnerRequestCmd, partnerRequestsCmd, decideCmd(true), decideCmd(false),
		partnerUnlockCmd)
	rootCmd.AddCommand(partnerCmd)
	daemon.EveryMinute = append(daemon.EveryMinute, partnerTick)
}

func newLineReader(lines string) *bufio.Reader { return bufio.NewReader(strings.NewReader(lines)) }
