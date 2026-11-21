//go:build linux

package blocker

// hostsPath is the path of the Linux hosts file.
// This file exists on every Unix/Linux system and is read automatically by
// the operating system before any DNS query.
// hostsPath is /etc/hosts; a variable for tests.
var hostsPath = "/etc/hosts"

// hostsWritten: glibc reads /etc/hosts at each lookup and systemd-resolved
// notices the change itself.
func hostsWritten() {}
