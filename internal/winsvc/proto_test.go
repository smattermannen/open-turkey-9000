package winsvc

import (
	"bytes"
	"strings"
	"testing"
)

func TestFrames(t *testing.T) {
	var buf bytes.Buffer
	req := Request{Args: []string{"block", "list"}, Dir: `C:\Users\me`, Terminal: &Terminal{Cols: 80, Rows: 25, Nonce: "n"}}
	if err := WriteRequest(&buf, req); err != nil {
		t.Fatal(err)
	}
	w := FrameWriter{W: &buf, Kind: FrameStdout}
	if _, err := w.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	if err := WriteFrame(&buf, FrameExit, ExitPayload(-3)); err != nil {
		t.Fatal(err)
	}
	got, err := ReadRequest(&buf)
	if err != nil || strings.Join(got.Args, " ") != "block list" || got.Terminal.Cols != 80 || got.Dir != req.Dir {
		t.Fatalf("request = %+v, %v", got, err)
	}
	kind, p, err := ReadFrame(&buf)
	if err != nil || kind != FrameStdout || string(p) != "hello" {
		t.Fatalf("stdout frame = %c %q %v", kind, p, err)
	}
	kind, p, err = ReadFrame(&buf)
	if err != nil || kind != FrameExit || ExitCode(p) != -3 {
		t.Fatalf("exit frame = %c %v %v", kind, p, err)
	}
}

func TestRequestMustComeFirst(t *testing.T) {
	var buf bytes.Buffer
	_ = WriteFrame(&buf, FrameStdin, []byte("x"))
	if _, err := ReadRequest(&buf); err == nil {
		t.Fatal("input before the request was accepted")
	}
}

func TestFrameTooLarge(t *testing.T) {
	var buf bytes.Buffer
	buf.Write([]byte{FrameStdin, 0xff, 0xff, 0xff, 0xff})
	if _, _, err := ReadFrame(&buf); err == nil {
		t.Fatal("a 4 GB frame was accepted")
	}
}

func TestControlFilter(t *testing.T) {
	f := &ControlFilter{Nonce: "abc"}
	seq := Control("abc", CtlRaw)
	forged := Control("xyz", CtlRaw)
	input := "before" + seq + "middle" + forged + "after" + Control("abc", CtlCooked)
	var out []byte
	var modes []string
	// Byte by byte: sequences split across writes are still found.
	for i := 0; i < len(input); i++ {
		o, m := f.Filter([]byte{input[i]})
		out = append(out, o...)
		modes = append(modes, m...)
	}
	out = append(out, f.Flush()...)
	if string(out) != "beforemiddle"+forged+"after" {
		t.Errorf("output = %q", out)
	}
	if strings.Join(modes, ",") != "raw,cooked" {
		t.Errorf("modes = %v", modes)
	}
}
