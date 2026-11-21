package lock

// window_windows.go — the challenge window on Windows: the command (run by
// the service, with no console) asks the session agent of the person who
// called to open it (winagent/challenge_windows.go) and reads back the
// keys typed there, in the same events as the Linux window's.

import (
	"errors"
	"io"
	"strings"

	"github.com/smattermannen/kc9k/internal/platform"
	"github.com/smattermannen/kc9k/internal/winagent"
)

func startWindow(pages []string) (io.Reader, func() error, error) {
	session := platform.CallerSession()
	if session <= 0 {
		return nil, nil, errors.New("no session of yours to open it in: run the command in a console")
	}
	var events string
	err := winagent.Call(uint32(session), winagent.OpChallenge,
		winagent.ChallengeArgs{Pages: pages, Accuracy: MinAccuracy * 100, Width: maxLineWidth}, &events)
	if err != nil {
		return nil, nil, err
	}
	return strings.NewReader(events), func() error { return nil }, nil
}
