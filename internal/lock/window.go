package lock

// window.go — the text lock's challenge without a terminal (SPEC §4.6,
// R-LOCK-5; §6 2026-11-14, 2026-11-20). The command opens a window itself,
// as the signed-in user on their desktop, and reads the keys typed there one
// by one; it never reads an answer piped to it. The window
// (challenge_window.py, embedded, so nothing on disk can stand in for it)
// only shows the pages and reports keys; scoring, paste and burst detection
// happen here.

import (
	"bufio"
	_ "embed"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// windowPageLines is how many challenge lines one page of the window holds.
const windowPageLines = 10

// burstMillis: more than pasteBurst printable keys within this many
// milliseconds can't be typed by hand (the window's backstop like the
// terminal's, for input that bypasses its paste checks).
const burstMillis = 5

// StartWindow opens the challenge window for pages and returns the key
// events it writes and a function that waits for it to close. Tests replace
// it.
var StartWindow = startWindow

// lastLine is the last non-empty line of text (a Python traceback's error).
func lastLine(text string) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

// runChallengeWindow has the challenge typed in the window.
func runChallengeWindow(challenge string) (bool, error) {
	pageList := pages(challenge, maxLineWidth, windowPageLines)
	fmt.Println("No terminal: the challenge opens in its own window on your desktop. Type it there.")
	events, wait, err := StartWindow(pageList)
	if err != nil {
		return false, fmt.Errorf("no terminal, and the challenge window didn't open: %w (an answer piped in is never read)", err)
	}
	typed, outcome, readErr := readWindowKeys(events, pageList)
	waitErr := wait()
	if readErr != nil {
		return false, readErr
	}
	switch outcome {
	case outcomePaste:
		fmt.Println("Paste detected. Unlocking requires typing the text by hand.")
		return false, nil
	case outcomeSynthetic:
		fmt.Println("A key that wasn't typed on the keyboard. Unlocking requires typing the text by hand.")
		return false, nil
	case outcomeCancel:
		if waitErr != nil {
			return false, fmt.Errorf("the challenge window closed: %w", waitErr)
		}
		fmt.Println("Cancelled.")
		return false, nil
	}
	return reportAccuracy(score(typed, challenge)), nil
}

// readWindowKeys reads the window's key lines (challenge_window.py) and
// returns what was typed over every page, or how it ended otherwise.
func readWindowKeys(events io.Reader, pageList []string) (string, inputOutcome, error) {
	var total, page strings.Builder
	var times []int64 // the last printable keys' times, for burst detection
	n := 0
	scanner := bufio.NewScanner(events)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 0 {
			continue
		}
		switch fields[0] {
		case "p":
			return "", outcomePaste, nil
		case "s":
			return "", outcomeSynthetic, nil
		case "c":
			return "", outcomeCancel, nil
		case "b":
			if s := page.String(); s != "" {
				page.Reset()
				page.WriteString(s[:len(s)-1])
			}
		case "n":
			total.WriteString(page.String())
			page.Reset()
			if n++; n >= len(pageList) {
				return total.String(), outcomeDone, nil
			}
		case "k":
			if len(fields) < 3 {
				continue
			}
			code, err1 := strconv.Atoi(fields[1])
			at, err2 := strconv.ParseInt(fields[2], 10, 64)
			if err1 != nil || err2 != nil || code < 0x20 || code >= 0x7f {
				continue
			}
			times = append(times, at)
			if len(times) > pasteBurst+1 {
				times = times[1:]
			}
			if len(times) == pasteBurst+1 && times[len(times)-1]-times[0] <= burstMillis {
				return "", outcomePaste, nil
			}
			if page.Len() < len(pageList[n])+inputSlack {
				page.WriteByte(byte(code))
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return "", outcomeDone, fmt.Errorf("failed to read the challenge window: %w", err)
	}
	return "", outcomeCancel, nil // closed before the last page was done
}
