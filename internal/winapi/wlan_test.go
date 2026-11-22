package winapi

import (
	"encoding/binary"
	"testing"
)

// W10: the Native Wifi API's layouts, built by hand as Windows lays them out.
func TestParseBSSList(t *testing.T) {
	buf := make([]byte, 8+2*wlanBSSEntrySize)
	binary.LittleEndian.PutUint32(buf[4:], 2)
	put := func(i int, ssid string, mac []byte) {
		e := buf[8+i*wlanBSSEntrySize:]
		binary.LittleEndian.PutUint32(e, uint32(len(ssid)))
		copy(e[4:], ssid)
		copy(e[40:], mac)
	}
	put(0, "Home", []byte{0xaa, 0xbb, 0xcc, 0x01, 0x02, 0x03})
	put(1, "Café:5G", []byte{0, 1, 2, 3, 4, 0xff})
	got, err := ParseBSSList(buf)
	if err != nil {
		t.Fatal(err)
	}
	want := []BSS{{"Home", "AA:BB:CC:01:02:03"}, {"Café:5G", "00:01:02:03:04:FF"}}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("got %v, want %v", got, want)
	}
	binary.LittleEndian.PutUint32(buf[4:], 3)
	if _, err := ParseBSSList(buf); err == nil {
		t.Error("an entry past the end was read")
	}
	if _, err := ParseBSSList(buf[:5]); err == nil {
		t.Error("a short list was read")
	}
}

func TestParseInterfaceListAndRadio(t *testing.T) {
	buf := make([]byte, 8+2*wlanInterfaceInfoSize)
	binary.LittleEndian.PutUint32(buf, 2)
	buf[8] = 7
	buf[8+wlanInterfaceInfoSize] = 9
	ifs, err := ParseInterfaceList(buf)
	if err != nil || len(ifs) != 2 || ifs[0][0] != 7 || ifs[1][0] != 9 {
		t.Fatalf("interfaces %v, %v", ifs, err)
	}
	rs := make([]byte, 4+2*12)
	binary.LittleEndian.PutUint32(rs, 2)
	binary.LittleEndian.PutUint32(rs[4:], 0)
	binary.LittleEndian.PutUint32(rs[8:], 2)
	binary.LittleEndian.PutUint32(rs[12:], 1)
	binary.LittleEndian.PutUint32(rs[16:], 1)
	binary.LittleEndian.PutUint32(rs[20:], 1)
	binary.LittleEndian.PutUint32(rs[24:], 1)
	got, err := ParseRadioState(rs)
	if err != nil || len(got) != 2 || got[0] != (RadioState{0, 2, 1}) || got[1] != (RadioState{1, 1, 1}) {
		t.Fatalf("radio %v, %v", got, err)
	}
}
