//go:build linux

package blocker

import (
	"reflect"
	"testing"
)

func TestParsePasswdUsers(t *testing.T) {
	passwd := `root:x:0:0:root:/root:/bin/bash
daemon:x:1:1:daemon:/usr/sbin:/usr/sbin/nologin
anna:x:1000:1000:Anna:/home/anna:/bin/bash
ben:x:1001:1001::/home/ben:/usr/bin/zsh
svc:x:1002:1002::/var/svc:/usr/sbin/nologin
nobody:x:65534:65534:nobody:/nonexistent:/bin/sh
broken line`
	if got := parsePasswdUsers(passwd); !reflect.DeepEqual(got, []string{"anna", "ben"}) {
		t.Errorf("users = %v", got)
	}
}
