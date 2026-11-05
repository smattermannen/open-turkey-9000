package blocker

import "testing"

func TestMountOptions(t *testing.T) {
	info := "22 1 8:1 / / rw,relatime shared:1 - ext4 /dev/sda1 rw\n" +
		"30 22 8:1 /home /home rw,nosuid,nodev,noexec,relatime shared:1 - ext4 /dev/sda1 rw\n"
	mounts := mountOptions(info)
	if hasOption(mounts["/"], "noexec") {
		t.Error("/ is not noexec")
	}
	if !hasOption(mounts["/home"], "noexec") {
		t.Error("/home is noexec")
	}
	if hasOption("rw,noexecish", "noexec") {
		t.Error("only exact options match")
	}
}
