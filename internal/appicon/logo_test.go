package appicon

import (
	"bytes"
	"os"
	"testing"
)

func TestLogoIsTheOriginalProjectImage(t *testing.T) {
	original, err := os.ReadFile("../../logo.png")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(LogoPNG(), original) {
		t.Fatal("embedded logo differs from the original project image")
	}
	copy := LogoPNG()
	copy[0] ^= 0xff
	if !bytes.Equal(LogoPNG(), original) {
		t.Fatal("caller changed the embedded logo")
	}
}
