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

// reportCrash records a panic as the daemon's own crash, so the restart
// systemd makes isn't counted as tampering (§3.10), then crashes on.
func reportCrash(database *db.DB) {
	if r := recover(); r != nil {
		enforce.NoteDaemonCrash(database, r)
		panic(r)
	}
}

// Run starts the enforcement loop and blocks until a shutdown signal arrives.
func Run() error {
	stop := make(chan struct{})
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		sig := <-sigChan
		log.Printf("daemon: signal received (%v), shutting down gracefully...", sig)
		close(stop)
	}()
	return RunUntil(stop)
}

// RunUntil starts the enforcement loop and blocks until stop is closed (the
// Windows service's stop request; a signal on Linux).
func RunUntil(stop <-chan struct{}) error {
	database, err := db.OpenDB(db.ResolveDBPath())
	if err != nil {
		return err
	}
	defer database.Close()

	enforce.NoteDaemonStart(database)
	defer reportCrash(database)

	go func() {
		defer reportCrash(database)
		if err := enforce.ServeRules(database); err != nil {
			log.Printf("daemon: rules server: %v", err)
		}
	}()

	// During a lock freeze, catch an unlocked screen within half a second
	// instead of waiting for the next enforcement cycle.
	go func() {
		defer reportCrash(database)
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
		defer reportCrash(database)
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

	// The clock check (§4.10): once per boot while a timer lock runs, the
	// internet's time gives powered-off time back; tried every 5 minutes
	// until it answers.
	go func() {
		defer reportCrash(database)
		time.Sleep(30 * time.Second) // let the network come up after boot
		enforce.CheckClock(database, time.Now())
		check := time.NewTicker(5 * time.Minute)
		defer check.Stop()
		for now := range check.C {
			enforce.CheckClock(database, now)
		}
	}()

	// LibreWolf, once, if the installation left it out (§4.11).
	installLibreWolf(stop)

	// The circadian rhythm (§4.10) keeps its own pace beside the cycle.
	go func() {
		defer reportCrash(database)
		enforce.RunCircadian(database)
	}()

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
		case <-stop:
			return nil
		}
	}
}
