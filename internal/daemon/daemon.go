// Package daemon implements the main loop of Killer Chicken 9000's background service.
//
// The daemon runs as a systemd service and has one crucial job: making sure
// every blocking layer always matches what the active blocks require, and that
// scheduled blocks turn on and off at the right times.
//
// Every 5 seconds it runs one enforcement tick (see the enforce package): it
// reconciles schedules, then re-applies only the layers that have drifted
// (someone edited /etc/hosts, flushed iptables, deleted a policy file, etc.).
// Because it only acts on drift, an idle system causes no disk writes.
//
// The daemon handles SIGTERM and SIGINT to shut down cleanly.
package daemon

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/smattermannen/kc9k/internal/blocker"
	"github.com/smattermannen/kc9k/internal/db"
	"github.com/smattermannen/kc9k/internal/enforce"
)

// Run starts the enforcement loop and blocks until a shutdown signal arrives.
func Run() error {
	database, err := db.OpenDB(db.ResolveDBPath())
	if err != nil {
		return err
	}
	defer database.Close()

	go func() {
		if err := enforce.ServeRules(database); err != nil {
			log.Printf("daemon: rules server: %v", err)
		}
	}()

	// During a lock freeze, catch an unlocked screen within half a second
	// instead of waiting for the next enforcement cycle.
	go func() {
		relock := time.NewTicker(500 * time.Millisecond)
		defer relock.Stop()
		for range relock.C {
			if enforce.LockFreezeEngaged(database, time.Now()) {
				blocker.RelockUnlocked()
			}
		}
	}()

	// Statistics: count the app in the foreground every 5 seconds, and drop
	// old days once a day.
	go func() {
		sample := time.NewTicker(5 * time.Second)
		defer sample.Stop()
		last, pruned := time.Now(), ""
		for now := range sample.C {
			enforce.RecordApps(database, now, now.Sub(last))
			last = now
			if day := enforce.Day(now); day != pruned {
				enforce.PruneStats(database, now)
				pruned = day
			}
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	log.Println("daemon: starting Killer Chicken 9000 enforcement loop")

	// Enforce immediately so blocks are in place the moment the daemon starts.
	if err := enforce.Tick(database, time.Now()); err != nil {
		log.Printf("daemon: error in initial enforcement cycle: %v", err)
	}

	for {
		select {
		case <-ticker.C:
			if err := enforce.Tick(database, time.Now()); err != nil {
				log.Printf("daemon: error in enforcement cycle: %v", err)
			}
		case sig := <-sigChan:
			log.Printf("daemon: signal received (%v), shutting down gracefully...", sig)
			return nil
		}
	}
}
