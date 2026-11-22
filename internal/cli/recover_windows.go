package cli

import "github.com/smattermannen/kc9k/internal/blocker"

const (
	recoverIntro       = "Run it from an administrator's console after starting Windows in Safe Mode (Settings › System › Recovery › Advanced startup, or F8 while the computer starts)."
	recoverNotRoot     = "run it from an administrator's console, in Safe Mode"
	recoverNotRecovery = "Windows isn't in Safe Mode: start it in Safe Mode first (Settings › System › Recovery › Advanced startup, or F8 while the computer starts)"
)

// recoveryBoot: Safe Mode is Windows' recovery boot (W13, SPEC §6
// 2026-11-22 By Opus).
func recoveryBoot() bool { return blocker.SafeModeBoot() }
