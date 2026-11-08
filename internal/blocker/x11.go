// x11.go — a minimal X11 client: list the windows on a display with their
// titles and processes, and ask a window to close.
//
// Window-title blocking needs this every few seconds. Running xprop per
// window would start dozens of processes each cycle, and KC9K is built
// without cgo, so this speaks the few X11 requests it needs directly:
// InternAtom, GetProperty, QueryTree and SendEvent. It authenticates with
// the display's MIT-MAGIC-COOKIE-1 from the user's Xauthority file.
package blocker

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

// X11Window is a top-level window: its ID, title and owning process (0 when
// the window doesn't say).
type X11Window struct {
	ID    uint32
	Title string
	PID   int
}

type x11Conn struct {
	conn  net.Conn
	r     *bufio.Reader
	root  uint32
	atoms map[string]uint32
}

const x11Timeout = 3 * time.Second

// xauthCookie finds the MIT-MAGIC-COOKIE-1 for display number display in
// Xauthority file content. Entries are: family, address, display number,
// auth name and auth data, each but the first a 16-bit big-endian length
// followed by bytes.
func xauthCookie(data []byte, display string) []byte {
	r := bytes.NewReader(data)
	read := func() ([]byte, bool) {
		var n uint16
		if binary.Read(r, binary.BigEndian, &n) != nil {
			return nil, false
		}
		b := make([]byte, n)
		if _, err := io.ReadFull(r, b); err != nil {
			return nil, false
		}
		return b, true
	}
	var fallback []byte
	for {
		var family uint16
		if binary.Read(r, binary.BigEndian, &family) != nil {
			return fallback
		}
		_, ok1 := read()
		number, ok2 := read()
		name, ok3 := read()
		cookie, ok4 := read()
		if !ok1 || !ok2 || !ok3 || !ok4 {
			return fallback
		}
		if string(name) != "MIT-MAGIC-COOKIE-1" {
			continue
		}
		if string(number) == display {
			return cookie
		}
		if fallback == nil {
			fallback = cookie
		}
	}
}

func pad4(n int) int { return (4 - n%4) % 4 }

// dialX11 connects to display number display (":0" is "0") as the owner of
// xauthority. An empty cookie tries without authentication.
func dialX11(display string, cookie []byte) (*x11Conn, error) {
	conn, err := net.DialTimeout("unix", "/tmp/.X11-unix/X"+display, x11Timeout)
	if err != nil {
		return nil, err
	}
	_ = conn.SetDeadline(time.Now().Add(x11Timeout))
	c := &x11Conn{conn: conn, r: bufio.NewReader(conn), atoms: map[string]uint32{}}

	authName := ""
	if len(cookie) > 0 {
		authName = "MIT-MAGIC-COOKIE-1"
	}
	var req bytes.Buffer
	req.WriteByte('l') // little-endian
	req.WriteByte(0)
	binary.Write(&req, binary.LittleEndian, uint16(11))
	binary.Write(&req, binary.LittleEndian, uint16(0))
	binary.Write(&req, binary.LittleEndian, uint16(len(authName)))
	binary.Write(&req, binary.LittleEndian, uint16(len(cookie)))
	req.Write([]byte{0, 0})
	req.WriteString(authName)
	req.Write(make([]byte, pad4(len(authName))))
	req.Write(cookie)
	req.Write(make([]byte, pad4(len(cookie))))
	if _, err := conn.Write(req.Bytes()); err != nil {
		conn.Close()
		return nil, err
	}

	head := make([]byte, 8)
	if _, err := io.ReadFull(c.r, head); err != nil {
		conn.Close()
		return nil, err
	}
	rest := make([]byte, int(binary.LittleEndian.Uint16(head[6:8]))*4)
	if _, err := io.ReadFull(c.r, rest); err != nil {
		conn.Close()
		return nil, err
	}
	if head[0] != 1 {
		conn.Close()
		return nil, fmt.Errorf("the X server refused the connection")
	}
	// rest starts at byte 8 of the setup reply: vendor length at 16,
	// number of formats at 21, the vendor string at 32, then the formats
	// (8 bytes each), then the first screen, whose first field is its root.
	if len(rest) < 32 {
		conn.Close()
		return nil, fmt.Errorf("short X setup reply")
	}
	vendorLen := int(binary.LittleEndian.Uint16(rest[16:18]))
	formats := int(rest[21])
	at := 32 + vendorLen + pad4(vendorLen) + 8*formats
	if len(rest) < at+4 {
		conn.Close()
		return nil, fmt.Errorf("short X setup reply")
	}
	c.root = binary.LittleEndian.Uint32(rest[at : at+4])
	return c, nil
}

func (c *x11Conn) Close() error { return c.conn.Close() }

// request sends one request and, if it has a reply, returns the reply's 32
// fixed bytes and its extra data. An X error comes back as an error.
func (c *x11Conn) request(req []byte, hasReply bool) ([]byte, []byte, error) {
	_ = c.conn.SetDeadline(time.Now().Add(x11Timeout))
	if _, err := c.conn.Write(req); err != nil {
		return nil, nil, err
	}
	if !hasReply {
		return nil, nil, nil
	}
	for {
		head := make([]byte, 32)
		if _, err := io.ReadFull(c.r, head); err != nil {
			return nil, nil, err
		}
		switch head[0] {
		case 0:
			return nil, nil, xError(head[1])
		case 1:
			extra := make([]byte, int(binary.LittleEndian.Uint32(head[4:8]))*4)
			if _, err := io.ReadFull(c.r, extra); err != nil {
				return nil, nil, err
			}
			return head, extra, nil
		default:
			continue // an event: not asked for, skip it
		}
	}
}

type xError byte

func (e xError) Error() string { return "X error " + strconv.Itoa(int(e)) }

func (c *x11Conn) atom(name string) (uint32, error) {
	if a, ok := c.atoms[name]; ok {
		return a, nil
	}
	var req bytes.Buffer
	n := len(name)
	req.WriteByte(16) // InternAtom
	req.WriteByte(0)  // create it if missing
	binary.Write(&req, binary.LittleEndian, uint16(2+(n+pad4(n))/4))
	binary.Write(&req, binary.LittleEndian, uint16(n))
	req.Write([]byte{0, 0})
	req.WriteString(name)
	req.Write(make([]byte, pad4(n)))
	head, _, err := c.request(req.Bytes(), true)
	if err != nil {
		return 0, err
	}
	a := binary.LittleEndian.Uint32(head[8:12])
	c.atoms[name] = a
	return a, nil
}

// property returns a window property's value and its format (8, 16 or 32).
func (c *x11Conn) property(window uint32, name string) ([]byte, int, error) {
	prop, err := c.atom(name)
	if err != nil {
		return nil, 0, err
	}
	var req bytes.Buffer
	req.WriteByte(20) // GetProperty
	req.WriteByte(0)  // don't delete
	binary.Write(&req, binary.LittleEndian, uint16(6))
	binary.Write(&req, binary.LittleEndian, window)
	binary.Write(&req, binary.LittleEndian, prop)
	binary.Write(&req, binary.LittleEndian, uint32(0))     // any type
	binary.Write(&req, binary.LittleEndian, uint32(0))     // from the start
	binary.Write(&req, binary.LittleEndian, uint32(16384)) // up to 64 KiB
	head, extra, err := c.request(req.Bytes(), true)
	if err != nil {
		return nil, 0, err
	}
	format := int(head[1])
	length := int(binary.LittleEndian.Uint32(head[16:20]))
	size := length * format / 8
	if size > len(extra) {
		size = len(extra)
	}
	return extra[:size], format, nil
}

// children lists a window's children (QueryTree).
func (c *x11Conn) children(window uint32) ([]uint32, error) {
	var req bytes.Buffer
	req.WriteByte(15) // QueryTree
	req.WriteByte(0)
	binary.Write(&req, binary.LittleEndian, uint16(2))
	binary.Write(&req, binary.LittleEndian, window)
	head, extra, err := c.request(req.Bytes(), true)
	if err != nil {
		return nil, err
	}
	n := int(binary.LittleEndian.Uint16(head[16:18]))
	out := make([]uint32, 0, n)
	for i := 0; i < n && 4*i+4 <= len(extra); i++ {
		out = append(out, binary.LittleEndian.Uint32(extra[4*i:]))
	}
	return out, nil
}

func uint32s(b []byte) []uint32 {
	out := make([]uint32, 0, len(b)/4)
	for i := 0; i+4 <= len(b); i += 4 {
		out = append(out, binary.LittleEndian.Uint32(b[i:]))
	}
	return out
}

// windowInfo reads a window's title (_NET_WM_NAME, else WM_NAME) and process.
func (c *x11Conn) windowInfo(id uint32) (X11Window, bool) {
	w := X11Window{ID: id}
	if name, _, err := c.property(id, "_NET_WM_NAME"); err == nil && len(name) > 0 {
		w.Title = string(name)
	} else if name, _, err := c.property(id, "WM_NAME"); err == nil {
		w.Title = string(name)
	}
	if pid, format, err := c.property(id, "_NET_WM_PID"); err == nil && format == 32 && len(pid) >= 4 {
		w.PID = int(binary.LittleEndian.Uint32(pid))
	}
	return w, w.Title != ""
}

// windows lists the top-level windows: the window manager's client list
// when there is one, otherwise the root's children and their children
// (windows without a window manager, or inside frames).
func (c *x11Conn) windows() ([]X11Window, error) {
	var ids []uint32
	if list, format, err := c.property(c.root, "_NET_CLIENT_LIST"); err == nil && format == 32 && len(list) > 0 {
		ids = uint32s(list)
	} else {
		top, err := c.children(c.root)
		if err != nil {
			return nil, err
		}
		for _, t := range top {
			ids = append(ids, t)
			if inner, err := c.children(t); err == nil {
				ids = append(ids, inner...)
			}
		}
	}
	var out []X11Window
	for _, id := range ids {
		if w, ok := c.windowInfo(id); ok {
			out = append(out, w)
		}
	}
	return out, nil
}

// closeWindow asks the window manager to close a window politely
// (_NET_CLOSE_WINDOW), as the window's own close button would, so an app
// can save or ask first.
func (c *x11Conn) closeWindow(id uint32) error {
	closeAtom, err := c.atom("_NET_CLOSE_WINDOW")
	if err != nil {
		return err
	}
	var req bytes.Buffer
	req.WriteByte(25) // SendEvent
	req.WriteByte(0)  // don't propagate
	binary.Write(&req, binary.LittleEndian, uint16(11))
	binary.Write(&req, binary.LittleEndian, c.root)
	// SubstructureRedirect | SubstructureNotify
	binary.Write(&req, binary.LittleEndian, uint32(1<<20|1<<19))
	event := make([]byte, 32)
	event[0] = 33 // ClientMessage
	event[1] = 32 // format
	binary.LittleEndian.PutUint32(event[4:], id)
	binary.LittleEndian.PutUint32(event[8:], closeAtom)
	binary.LittleEndian.PutUint32(event[12:], uint32(time.Now().Unix()))
	binary.LittleEndian.PutUint32(event[16:], 2) // source: a pager or tool
	req.Write(event)
	_, _, err = c.request(req.Bytes(), false)
	return err
}

// displayNumber turns ":0" or ":0.0" into "0".
func displayNumber(display string) string {
	d := strings.TrimPrefix(display, ":")
	if i := strings.IndexByte(d, '.'); i >= 0 {
		d = d[:i]
	}
	return d
}

// openUserDisplay connects to display as the user whose Xauthority file is
// xauthPath (it may be missing when the server needs no cookie).
func openUserDisplay(display, xauthPath string) (*x11Conn, error) {
	number := displayNumber(display)
	var cookie []byte
	if data, err := os.ReadFile(xauthPath); err == nil {
		cookie = xauthCookie(data, number)
	}
	c, err := dialX11(number, cookie)
	if err != nil && len(cookie) > 0 {
		return nil, err
	}
	if err != nil {
		return nil, errors.Join(err, fmt.Errorf("no X cookie in %s", xauthPath))
	}
	return c, nil
}
