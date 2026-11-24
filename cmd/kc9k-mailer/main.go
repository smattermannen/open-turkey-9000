// Command kc9k-mailer sends the accountability partners' email (SPEC
// §4.15): the messages devices queue in KC9K's cloud (invitations, reports,
// alerts, unlock requests), through any SMTP service. It runs where the
// owner chooses (ROADMAP 2.13: a small server, or a scheduled job), never
// on a user's device: it needs the project's service role key.
//
//	KC9K_CLOUD_URL=https://<project>.supabase.co KC9K_SERVICE_KEY=… \
//	SMTP_HOST=smtp.example.org SMTP_PORT=587 SMTP_USER=… SMTP_PASSWORD=… \
//	SMTP_FROM='Killer Chicken 9000 <noreply@example.org>' kc9k-mailer [-once]
package main

import (
	"flag"
	"log"
	"os"
	"time"

	"github.com/smattermannen/kc9k/internal/cloud"
)

func main() {
	once := flag.Bool("once", false, "Send what is queued, then stop")
	every := flag.Duration("every", time.Minute, "How often to look at the queue")
	flag.Parse()
	need := func(name string) string {
		v := os.Getenv(name)
		if v == "" {
			log.Fatalf("kc9k-mailer: %s is not set", name)
		}
		return v
	}
	c := cloud.New(need("KC9K_CLOUD_URL"), need("KC9K_SERVICE_KEY"))
	m := cloud.SMTPMailer{Host: need("SMTP_HOST"), Port: need("SMTP_PORT"), User: need("SMTP_USER"),
		Password: need("SMTP_PASSWORD"), From: need("SMTP_FROM")}
	for {
		for {
			n, err := cloud.DeliverOutbox(c, m, 50)
			if err != nil {
				log.Printf("kc9k-mailer: %v", err)
			}
			if n < 50 || err != nil {
				break
			}
		}
		if *once {
			return
		}
		time.Sleep(*every)
	}
}
