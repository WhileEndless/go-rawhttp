package main

import "testing"

func TestBuildBodyDataHex(t *testing.T) {
	// A body containing a NUL byte — the case that truncates on the shell.
	cfg := &Config{DataHex: []string{"00 01 ff 0a"}}
	body, _, err := buildBody(cfg)
	if err != nil {
		t.Fatalf("buildBody: %v", err)
	}
	want := []byte{0x00, 0x01, 0xff, 0x0a}
	if string(body) != string(want) {
		t.Fatalf("got % x, want % x", body, want)
	}
}

func TestBuildBodyDataBase64(t *testing.T) {
	cfg := &Config{DataBase64: []string{"AAH/Cg=="}} // same 00 01 ff 0a
	body, _, err := buildBody(cfg)
	if err != nil {
		t.Fatalf("buildBody: %v", err)
	}
	want := []byte{0x00, 0x01, 0xff, 0x0a}
	if string(body) != string(want) {
		t.Fatalf("got % x, want % x", body, want)
	}
}

func TestDecodeHexTolerant(t *testing.T) {
	for _, in := range []string{"0xDEADBEEF", "DE AD BE EF", "deadbeef", "de\nad\nbe\nef"} {
		b, err := decodeHex(in)
		if err != nil {
			t.Fatalf("decodeHex(%q): %v", in, err)
		}
		if string(b) != string([]byte{0xde, 0xad, 0xbe, 0xef}) {
			t.Fatalf("decodeHex(%q) = % x", in, b)
		}
	}
	if _, err := decodeHex("zz"); err == nil {
		t.Fatal("expected error for invalid hex")
	}
}

func TestDecodeBase64Variants(t *testing.T) {
	// std padded, std unpadded, url alphabet — all decode to the same 3 bytes.
	for _, in := range []string{"+/8=", "+/8", "-_8=", "-_8"} {
		b, err := decodeBase64(in)
		if err != nil {
			t.Fatalf("decodeBase64(%q): %v", in, err)
		}
		if string(b) != string([]byte{0xfb, 0xff}) {
			t.Fatalf("decodeBase64(%q) = % x", in, b)
		}
	}
}
