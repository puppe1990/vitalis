package photos

import (
	"bytes"
	"errors"
	"testing"
)

func TestSniff_acceptsJPEG(t *testing.T) {
	got, err := Sniff([]byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10})
	if err != nil {
		t.Fatal(err)
	}
	if got.ContentType != "image/jpeg" || got.Ext != "jpg" {
		t.Errorf("got %+v", got)
	}
}

func TestSniff_acceptsPNG(t *testing.T) {
	got, err := Sniff([]byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00})
	if err != nil {
		t.Fatal(err)
	}
	if got.ContentType != "image/png" || got.Ext != "png" {
		t.Errorf("got %+v", got)
	}
}

func TestSniff_acceptsWebP(t *testing.T) {
	body := []byte("RIFF....WEBPVP8 ")
	got, err := Sniff(body)
	if err != nil {
		t.Fatal(err)
	}
	if got.ContentType != "image/webp" || got.Ext != "webp" {
		t.Errorf("got %+v", got)
	}
}

func TestSniff_acceptsHEIC(t *testing.T) {
	// ISO BMFF: size(4) + "ftyp" + brand "heic"
	body := []byte{0x00, 0x00, 0x00, 0x18, 'f', 't', 'y', 'p', 'h', 'e', 'i', 'c'}
	got, err := Sniff(body)
	if err != nil {
		t.Fatal(err)
	}
	if got.ContentType != "image/heic" || got.Ext != "heic" {
		t.Errorf("got %+v", got)
	}
}

func TestSniff_rejectsPDF(t *testing.T) {
	_, err := Sniff([]byte("%PDF-1.4"))
	if !errors.Is(err, ErrUnsupportedType) {
		t.Errorf("err = %v, want ErrUnsupportedType", err)
	}
}

func TestSniff_rejectsEmpty(t *testing.T) {
	_, err := Sniff(nil)
	if !errors.Is(err, ErrUnsupportedType) {
		t.Errorf("err = %v, want ErrUnsupportedType", err)
	}
}

func TestSniff_rejectsOversized(t *testing.T) {
	body := bytes.Repeat([]byte{0xff, 0xd8, 0xff}, MaxBytes/3+1)
	_, err := Sniff(body)
	if !errors.Is(err, ErrTooLarge) {
		t.Errorf("err = %v, want ErrTooLarge", err)
	}
}
