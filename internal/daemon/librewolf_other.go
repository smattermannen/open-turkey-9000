//go:build !linux

package daemon

// installLibreWolf: the Windows MSI installs browsers its own way (W18).
func installLibreWolf(stop <-chan struct{}) {}
