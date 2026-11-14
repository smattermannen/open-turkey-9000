// gamma.go — software brightness and colour temperature for the circadian
// rhythm, through the X server's gamma ramps (the RandR extension), the way
// Redshift and the desktops' night lights do it. It speaks the few RandR
// requests it needs over the same cgo-free client as window titles
// (x11.go), with the same caution about what a user can plant.
package blocker

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
)

// randrOpcode asks the server for the RandR extension and its version. It
// returns the extension's major opcode.
func (c *x11Conn) randrOpcode() (byte, error) {
	const name = "RANDR"
	var req bytes.Buffer
	req.WriteByte(98) // QueryExtension
	req.WriteByte(0)
	binary.Write(&req, binary.LittleEndian, uint16(2+(len(name)+pad4(len(name)))/4))
	binary.Write(&req, binary.LittleEndian, uint16(len(name)))
	req.Write([]byte{0, 0})
	req.WriteString(name)
	req.Write(make([]byte, pad4(len(name))))
	head, _, err := c.request(req.Bytes(), true)
	if err != nil {
		return 0, err
	}
	if head[8] == 0 {
		return 0, fmt.Errorf("the X server has no RandR")
	}
	op := head[9]
	// QueryVersion 1.5: the server answers with what it has; 1.3 is needed
	// for GetScreenResourcesCurrent.
	version := make([]byte, 12)
	version[0], version[1] = op, 0
	binary.LittleEndian.PutUint16(version[2:], 3)
	binary.LittleEndian.PutUint32(version[4:], 1)
	binary.LittleEndian.PutUint32(version[8:], 5)
	head, _, err = c.request(version, true)
	if err != nil {
		return 0, err
	}
	major, minor := binary.LittleEndian.Uint32(head[8:12]), binary.LittleEndian.Uint32(head[12:16])
	if major < 1 || (major == 1 && minor < 3) {
		return 0, fmt.Errorf("the X server has RandR %d.%d; 1.3 is needed", major, minor)
	}
	return op, nil
}

// crtcs lists the screen's CRTCs (the outputs' scan-out engines; each has
// its own gamma ramp).
func (c *x11Conn) crtcs(op byte) ([]uint32, error) {
	req := make([]byte, 8)
	req[0], req[1] = op, 25 // GetScreenResourcesCurrent
	binary.LittleEndian.PutUint16(req[2:], 2)
	binary.LittleEndian.PutUint32(req[4:], c.root)
	head, extra, err := c.request(req, true)
	if err != nil {
		return nil, err
	}
	n := int(binary.LittleEndian.Uint16(head[16:18]))
	if len(extra) < 4*n {
		return nil, fmt.Errorf("short RandR reply")
	}
	return uint32s(extra[:4*n]), nil
}

func (c *x11Conn) gammaSize(op byte, crtc uint32) (int, error) {
	req := make([]byte, 8)
	req[0], req[1] = op, 22 // GetCrtcGammaSize
	binary.LittleEndian.PutUint16(req[2:], 2)
	binary.LittleEndian.PutUint32(req[4:], crtc)
	head, _, err := c.request(req, true)
	if err != nil {
		return 0, err
	}
	return int(binary.LittleEndian.Uint16(head[8:10])), nil
}

func (c *x11Conn) gamma(op byte, crtc uint32) (red, green, blue []uint16, err error) {
	req := make([]byte, 8)
	req[0], req[1] = op, 23 // GetCrtcGamma
	binary.LittleEndian.PutUint16(req[2:], 2)
	binary.LittleEndian.PutUint32(req[4:], crtc)
	head, extra, err := c.request(req, true)
	if err != nil {
		return nil, nil, nil, err
	}
	n := int(binary.LittleEndian.Uint16(head[8:10]))
	if len(extra) < 6*n {
		return nil, nil, nil, fmt.Errorf("short gamma reply")
	}
	read := func(at int) []uint16 {
		out := make([]uint16, n)
		for i := range out {
			out[i] = binary.LittleEndian.Uint16(extra[at+2*i:])
		}
		return out
	}
	return read(0), read(2 * n), read(4 * n), nil
}

func (c *x11Conn) setGamma(op byte, crtc uint32, red, green, blue []uint16) error {
	n := len(red)
	body := 6 * n
	var req bytes.Buffer
	req.WriteByte(op)
	req.WriteByte(24) // SetCrtcGamma
	binary.Write(&req, binary.LittleEndian, uint16((12+body+pad4(body))/4))
	binary.Write(&req, binary.LittleEndian, crtc)
	binary.Write(&req, binary.LittleEndian, uint16(n))
	req.Write([]byte{0, 0})
	for _, channel := range [][]uint16{red, green, blue} {
		binary.Write(&req, binary.LittleEndian, channel)
	}
	req.Write(make([]byte, pad4(body)))
	_, _, err := c.request(req.Bytes(), false)
	return err
}

// GammaReading is what the screens' gamma ramps show.
type GammaReading int

const (
	GammaUnknown  GammaReading = iota // no X session to read
	GammaOurs                         // the ramps KC9K would set now
	GammaIdentity                     // plain ramps: nothing changes the colours
	GammaOther                        // ramps someone else set (a night light, Redshift)
)

func (g GammaReading) String() string {
	return [...]string{"unknown", "ours", "identity", "other"}[g]
}

// RampFunc makes a ramp of a size: the circadian package's Ramp with the
// colour temperature and brightness wanted.
type RampFunc func(size int) (red, green, blue []uint16)

// rampTolerance is how far a read ramp may be from the one expected: the
// driver may round what it was given to the hardware's precision.
const rampTolerance = 1024

func rampsMatch(want RampFunc, red, green, blue []uint16) bool {
	wr, wg, wb := want(len(red))
	for i := range red {
		for _, pair := range [][2]uint16{{wr[i], red[i]}, {wg[i], green[i]}, {wb[i], blue[i]}} {
			if math.Abs(float64(pair[0])-float64(pair[1])) > rampTolerance {
				return false
			}
		}
	}
	return true
}

func allZero(channels ...[]uint16) bool {
	for _, channel := range channels {
		for _, v := range channel {
			if v != 0 {
				return false
			}
		}
	}
	return true
}

func identityRamp(size int) (red, green, blue []uint16) {
	red = make([]uint16, size)
	for i := range red {
		if size > 1 {
			red[i] = uint16(math.Round(65535 * float64(i) / float64(size-1)))
		}
	}
	return red, append([]uint16{}, red...), append([]uint16{}, red...)
}

// gammaSessions is every desktop session's X display (replaceable in tests).
var gammaSessions = func() []titleSession {
	var out []titleSession
	for _, u := range desktopUsers() {
		out = append(out, sessionFor(u))
	}
	return out
}

// eachCRTC runs fn on every lit CRTC of every desktop session.
func eachCRTC(fn func(c *x11Conn, op byte, crtc uint32, size int) error) (seen int, err error) {
	for _, s := range gammaSessions() {
		c, openErr := openUserDisplay(s.display, s.xauth, s.uid)
		if openErr != nil {
			continue // no X session there (a text console, or Wayland)
		}
		op, opErr := c.randrOpcode()
		if opErr != nil {
			c.Close()
			if err == nil {
				err = opErr
			}
			continue
		}
		crtcs, listErr := c.crtcs(op)
		if listErr != nil && err == nil {
			err = listErr
		}
		for _, crtc := range crtcs {
			size, sizeErr := c.gammaSize(op, crtc)
			if sizeErr != nil || size < 2 || size > 1<<14 {
				continue
			}
			seen++
			if fnErr := fn(c, op, crtc, size); fnErr != nil && err == nil {
				err = fnErr
			}
		}
		if syncErr := c.sync(); syncErr != nil && err == nil {
			err = syncErr
		}
		c.Close()
	}
	return seen, err
}

// ReadGamma tells whether the screens show the ramps want would make, plain
// ramps, or someone else's. One screen not showing ours is enough to say so.
func ReadGamma(want RampFunc) GammaReading {
	result := GammaUnknown
	_, _ = eachCRTC(func(c *x11Conn, op byte, crtc uint32, size int) error {
		red, green, blue, err := c.gamma(op, crtc)
		if err != nil || len(red) != size {
			return err
		}
		var this GammaReading
		switch {
		case want != nil && rampsMatch(want, red, green, blue):
			this = GammaOurs
		case rampsMatch(identityRamp, red, green, blue) || allZero(red, green, blue):
			// An all-black ramp would leave nothing to see: it is a
			// server that keeps no ramps (Xvfb), not anyone's setting.
			this = GammaIdentity
		default:
			this = GammaOther
		}
		// Other outranks identity, which outranks ours.
		if result == GammaUnknown || this > result {
			result = this
		}
		return nil
	})
	return result
}

// SetGamma gives every screen the ramps ramp makes; nil restores plain
// ramps. It reports how many screens it reached.
func SetGamma(ramp RampFunc) (int, error) {
	if ramp == nil {
		ramp = identityRamp
	}
	return eachCRTC(func(c *x11Conn, op byte, crtc uint32, size int) error {
		red, green, blue := ramp(size)
		return c.setGamma(op, crtc, red, green, blue)
	})
}
