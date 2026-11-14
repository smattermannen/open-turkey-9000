package db

import "testing"

func TestParseActions(t *testing.T) {
	if a, err := ParseActions(nil); err != nil || a != DefaultActions {
		t.Errorf("empty = %+v, %v; want the default", a, err)
	}
	if a, err := ParseActions([]string{"Greyscale", " no-images"}); err != nil || a != (Actions{Greyscale: true, NoImages: true}) {
		t.Errorf("parsed = %+v, %v", a, err)
	}
	if _, err := ParseActions([]string{"blur"}); err == nil {
		t.Error("an unknown action should be refused")
	}
}
