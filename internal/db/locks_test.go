package db

import "testing"

func TestPassword(t *testing.T) {
	hash, err := HashPassword("hemligt")
	if err != nil {
		t.Fatal(err)
	}
	if !CheckPassword(hash, "hemligt") {
		t.Error("the right password should match")
	}
	if CheckPassword(hash, "fel") {
		t.Error("a wrong password should not match")
	}
	if CheckPassword("skräp", "hemligt") {
		t.Error("a broken hash should not match")
	}
}
