// Package winsvc is KC9K on Windows: the service, the named pipe the CLI
// forwards its commands over, the session agent and install/uninstall
// (ROADMAP §4 W1; notes/windows.md). This file, the pipe's framing, is
// shared so it is tested on Linux too.
//
// A forwarded command is one connection: the client writes a Request
// (one JSON frame), then input frames; the service answers with output
// frames and ends with an exit frame. Every frame is a type byte, a
// 4-byte big-endian length and the payload.
package winsvc

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// PipeName is the pipe the CLI forwards commands over.
const PipeName = `\\.\pipe\kc9k`

// Frame types.
const (
	FrameRequest   byte = 'Q' // client → service: the Request, first frame
	FrameStdin     byte = 'I' // client → service: typed or piped input
	FrameStdinEOF  byte = 'E' // client → service: input ended
	FrameInterrupt byte = 'C' // client → service: Ctrl-C in a cooked console
	FrameStdout    byte = 'O' // service → client
	FrameStderr    byte = 'R' // service → client
	FrameExit      byte = 'X' // service → client: 4-byte exit code, last frame
)

// maxFrame bounds a frame: a pipe peer can't make the other side allocate
// without limit.
const maxFrame = 1 << 20

// Request is what the client asks the service to run.
type Request struct {
	Args []string `json:"args"`
	// Dir is the client's working directory, for relative paths.
	Dir string `json:"dir"`
	// Terminal is set when the client's input and output are a console:
	// the command may then read typed keys (the text lock's challenge).
	Terminal *Terminal `json:"terminal,omitempty"`
}

// Terminal describes the client's console.
type Terminal struct {
	Cols int `json:"cols"`
	Rows int `json:"rows"`
	// Nonce marks the control sequences the command writes to switch the
	// client's console between cooked and raw input, so text a command
	// prints (a block's name) can't switch it.
	Nonce string `json:"nonce"`
}

// WriteFrame writes one frame.
func WriteFrame(w io.Writer, kind byte, payload []byte) error {
	if len(payload) > maxFrame {
		return fmt.Errorf("frame of %d bytes is too large", len(payload))
	}
	head := make([]byte, 5, 5+len(payload))
	head[0] = kind
	binary.BigEndian.PutUint32(head[1:], uint32(len(payload)))
	_, err := w.Write(append(head, payload...))
	return err
}

// ReadFrame reads one frame.
func ReadFrame(r io.Reader) (kind byte, payload []byte, err error) {
	var head [5]byte
	if _, err := io.ReadFull(r, head[:]); err != nil {
		return 0, nil, err
	}
	n := binary.BigEndian.Uint32(head[1:])
	if n > maxFrame {
		return 0, nil, fmt.Errorf("frame of %d bytes is too large", n)
	}
	payload = make([]byte, n)
	if _, err := io.ReadFull(r, payload); err != nil {
		return 0, nil, err
	}
	return head[0], payload, nil
}

// WriteRequest writes the request frame.
func WriteRequest(w io.Writer, req Request) error {
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	return WriteFrame(w, FrameRequest, data)
}

// ReadRequest reads the request frame, which must come first.
func ReadRequest(r io.Reader) (Request, error) {
	kind, payload, err := ReadFrame(r)
	if err != nil {
		return Request{}, err
	}
	if kind != FrameRequest {
		return Request{}, errors.New("the first frame is not a request")
	}
	var req Request
	if err := json.Unmarshal(payload, &req); err != nil {
		return Request{}, fmt.Errorf("bad request: %w", err)
	}
	if len(req.Args) > 4096 {
		return Request{}, errors.New("too many arguments")
	}
	return req, nil
}

// ExitPayload encodes an exit code.
func ExitPayload(code int) []byte {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, uint32(int32(code)))
	return b
}

// ExitCode decodes an exit frame's payload.
func ExitCode(payload []byte) int {
	if len(payload) != 4 {
		return 1
	}
	return int(int32(binary.BigEndian.Uint32(payload)))
}

// FrameWriter is an io.Writer that sends each write as one frame of kind.
type FrameWriter struct {
	W    io.Writer
	Kind byte
	Lock interface {
		Lock()
		Unlock()
	}
}

func (f FrameWriter) Write(p []byte) (int, error) {
	if f.Lock != nil {
		f.Lock.Lock()
		defer f.Lock.Unlock()
	}
	written := 0
	for len(p) > 0 {
		n := min(len(p), maxFrame)
		if err := WriteFrame(f.W, f.Kind, p[:n]); err != nil {
			return written, err
		}
		written += n
		p = p[n:]
	}
	return written, nil
}

// Console control sequences: the command (in the service) asks the
// client's console for raw input while the challenge is typed, and for
// cooked input again after. The client strips them from the output.
const (
	ctlPrefix  = "\x1b]7799;kc9k;"
	ctlEnd     = "\x07"
	CtlRaw     = "raw"
	CtlCooked  = "cooked"
	CtlEchoOff = "echo-off"
	CtlEchoOn  = "echo-on"
)

// Control is the sequence that asks the client for mode, marked with nonce.
func Control(nonce, mode string) string {
	return ctlPrefix + nonce + ";" + mode + ctlEnd
}

// ControlFilter strips the client's control sequences from output and
// reports the modes they ask for. Sequences may be split across writes.
type ControlFilter struct {
	Nonce string
	held  []byte
}

// Filter returns the output to show and the modes asked for, in order.
func (f *ControlFilter) Filter(p []byte) (out []byte, modes []string) {
	data := append(f.held, p...)
	f.held = nil
	marker := []byte(ctlPrefix + f.Nonce + ";")
	for {
		i := indexBytes(data, marker)
		if i < 0 {
			// Hold back a tail that could start a marker.
			keep := partialSuffix(data, marker)
			out = append(out, data[:len(data)-keep]...)
			f.held = append([]byte(nil), data[len(data)-keep:]...)
			return out, modes
		}
		out = append(out, data[:i]...)
		rest := data[i+len(marker):]
		end := indexBytes(rest, []byte(ctlEnd))
		if end < 0 {
			if len(rest) > 32 { // not ours after all
				out = append(out, data[i:]...)
				return out, modes
			}
			f.held = append([]byte(nil), data[i:]...)
			return out, modes
		}
		modes = append(modes, string(rest[:end]))
		data = rest[end+len(ctlEnd):]
	}
}

// Flush returns what is held back.
func (f *ControlFilter) Flush() []byte {
	h := f.held
	f.held = nil
	return h
}

func indexBytes(s, sep []byte) int {
	for i := 0; i+len(sep) <= len(s); i++ {
		if string(s[i:i+len(sep)]) == string(sep) {
			return i
		}
	}
	return -1
}

// partialSuffix is the length of the longest suffix of s that is a proper
// prefix of marker.
func partialSuffix(s, marker []byte) int {
	for n := min(len(s), len(marker)-1); n > 0; n-- {
		if string(s[len(s)-n:]) == string(marker[:n]) {
			return n
		}
	}
	return 0
}
