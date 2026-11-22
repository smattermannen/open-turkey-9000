package blocker

import "testing"

func TestLinuxOnly(t *testing.T) {
	for entry, linux := range map[string]bool{
		"gnome-terminal-server": true, "/usr/bin/steam": true, "~/games/": true, "/opt/games/": true,
		"org.gnome.Nautilus": true, "com.valvesoftware.Steam": true, "xfce4-terminal": true,
		"steam": false, "steam.exe": false, "bot.py": false, `C:\Games\`: false, `\\server\share\x.exe`: false,
		"//server/share/": false, "my.game.app.exe": false, "sha256:00": false, "discord": false,
	} {
		if got := LinuxOnlyApp(entry) != ""; got != linux {
			t.Errorf("LinuxOnlyApp(%q) = %v, want %v", entry, got, linux)
		}
	}
	for cmd, linux := range map[string]bool{
		"systemctl suspend": true, "sudo loginctl lock-session": true, "/usr/bin/notify-send hi": true,
		"notepad.exe": false, "python bot.py": false, "": false,
	} {
		if got := LinuxOnlyCommand(cmd) != ""; got != linux {
			t.Errorf("LinuxOnlyCommand(%q) = %v, want %v", cmd, got, linux)
		}
	}
}
