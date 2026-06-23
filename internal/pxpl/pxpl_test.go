package pxpl

import (
	"encoding/json"
	"testing"
)

func TestMetaFromLyrics(t *testing.T) {
	content := "[ti:Earthquake]\n[ar:Labrinth]\n[offset:+0]\n[00:01.22] Labrinth come in\n"
	m, ok := metaFromLyrics(content)
	if !ok {
		t.Fatal("expected meta")
	}
	if m.Title != "Earthquake" || m.Artist != "Labrinth" {
		t.Fatalf("got %+v", m)
	}
}

func TestMetaFromLyricsNone(t *testing.T) {
	if _, ok := metaFromLyrics("[00:01.22] just lyrics, no header\n"); ok {
		t.Fatal("expected no meta")
	}
}

func TestFlexID(t *testing.T) {
	var s struct {
		A flexID `json:"a"`
		B flexID `json:"b"`
	}
	if err := json.Unmarshal([]byte(`{"a":"123","b":456}`), &s); err != nil {
		t.Fatal(err)
	}
	if s.A != "123" || s.B != "456" {
		t.Fatalf("got %q %q", s.A, s.B)
	}
}
