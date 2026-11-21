#!/bin/sh
# Builds a .deb of Killer Chicken 9000 (§4.11): kc9k_<version>_<arch>.deb in
# the current folder. Same files and paths as 'make install'; the package
# scripts do what install.sh does after copying (sudoers, database folder,
# service), and removal runs 'kc9k-bin prepare-uninstall', which refuses
# while anything is locked (R-LOCK-9). TAGS=devmaster for a testing build.
#
#   sh packaging/build-deb.sh [version]
set -eu
root=$(cd "$(dirname "$0")/.." && pwd)
version=${1:-$(cd "$root" && git describe --tags --always 2>/dev/null | sed 's/^v//; s/^\([^0-9]\)/0~\1/')}
arch=$(dpkg --print-architecture)
stage=$(mktemp -d)
trap 'rm -rf "$stage"' EXIT

PATH="/usr/local/go/bin:$PATH"
(cd "$root" && CGO_ENABLED=0 go build -tags "${TAGS:-}" -o "$stage/kc9k-bin" ./cmd/kc9k)

pkg="$stage/pkg"
install -d "$pkg/usr/local/bin" "$pkg/usr/share/applications" "$pkg/etc/systemd/system" "$pkg/DEBIAN"
install -m 755 "$stage/kc9k-bin" "$pkg/usr/local/bin/kc9k-bin"
install -m 755 "$root/gui/kc9k-gui.py" "$pkg/usr/local/bin/kc9k-gui"
install -m 644 "$root/gui/kc9k.desktop" "$pkg/usr/share/applications/kc9k.desktop"
install -m 644 "$root/systemd/kc9k.service" "$pkg/etc/systemd/system/kc9k.service"
printf '#!/bin/bash\n# Killer Chicken 9000: runs the real binary through sudo (see /etc/sudoers.d/kc9k).\nexec sudo /usr/local/bin/kc9k-bin "$@"\n' > "$pkg/usr/local/bin/kc9k"
chmod 755 "$pkg/usr/local/bin/kc9k"

cat > "$pkg/DEBIAN/control" <<CONTROL
Package: kc9k
Version: $version
Architecture: $arch
Maintainer: Killer Chicken 9000 <noreply@localhost>
Depends: sudo, nftables | iptables, python3, python3-gi, gir1.2-gtk-3.0
Recommends: zenity, libnotify-bin, extrepo
Section: utils
Priority: optional
Description: Killer Chicken 9000, a blocker for websites and apps
 Blocks, locks and bypass prevention for Linux: hosts, firewall, browser
 policies, a browser extension and app closing, run by a systemd daemon.
CONTROL

cat > "$pkg/DEBIAN/postinst" <<'POSTINST'
#!/bin/sh
set -e
if [ "$1" = configure ]; then
	tmp=$(mktemp)
	bin=/usr/local/bin/kc9k-bin
	printf '%%sudo ALL=(root) NOPASSWD: %s\nCmnd_Alias KC9K_POLL = %s gui-state, %s stats --json *\nDefaults!KC9K_POLL !log_allowed, !pam_session\n' "$bin" "$bin" "$bin" > "$tmp"
	if visudo -cf "$tmp" >/dev/null; then
		install -m 440 -o root -g root "$tmp" /etc/sudoers.d/kc9k
	else
		echo "visudo rejected the sudoers rule; /etc/sudoers.d/kc9k unchanged" >&2
	fi
	rm -f "$tmp"
	mkdir -p /var/lib/kc9k
	systemctl daemon-reload || true
	systemctl enable kc9k.service || true
	systemctl restart kc9k.service || true
fi
POSTINST

cat > "$pkg/DEBIAN/prerm" <<'PRERM'
#!/bin/sh
set -e
# Removal is refused while anything is locked (§4.11, R-LOCK-9); the
# blocking layers are cleaned up first. An upgrade keeps everything; a
# reinstall or an older version may be refused while anything is locked
# (tamper protection "block the installer", prepare-install).
guard() {
	if ! out=$(/usr/local/bin/kc9k-bin prepare-install 2>&1); then
		case "$out" in *"unknown command"*) ;; *) echo "$out" >&2; exit 1;; esac
	fi
}
# upgrade: the installed package asked; $2 is the version coming in.
if [ "$1" = upgrade ] && ! dpkg --compare-versions "$2" gt "@VERSION@"; then
	guard
fi
# failed-upgrade: the installed package's prerm refused, and dpkg asks the
# incoming package's instead (this one); going on here would install it
# anyway. $2 is the installed version.
if [ "$1" = failed-upgrade ] && ! dpkg --compare-versions "@VERSION@" gt "$2"; then
	guard
fi
if [ "$1" = remove ] || [ "$1" = deconfigure ]; then
	/usr/local/bin/kc9k-bin prepare-uninstall
	systemctl stop kc9k.service || true
	systemctl disable kc9k.service || true
fi
PRERM

cat > "$pkg/DEBIAN/postrm" <<'POSTRM'
#!/bin/sh
set -e
if [ "$1" = remove ] || [ "$1" = purge ]; then
	rm -f /etc/sudoers.d/kc9k
	systemctl daemon-reload || true
fi
if [ "$1" = purge ]; then
	rm -rf /var/lib/kc9k
fi
POSTRM
sed -i "s/@VERSION@/$version/" "$pkg/DEBIAN/prerm"
chmod 755 "$pkg/DEBIAN/postinst" "$pkg/DEBIAN/prerm" "$pkg/DEBIAN/postrm"

out="kc9k_${version}_${arch}.deb"
dpkg-deb --root-owner-group --build "$pkg" "$out" >/dev/null
echo "$out"
