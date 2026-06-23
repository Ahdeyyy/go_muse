// Package pxpl parses PixelPlayer ".pxpl" backup files.
//
// A .pxpl file is a standard ZIP archive prefixed with the 4-byte magic
// "PXPL". Inside are several JSON modules; this package decodes the ones
// relevant to playlist generation:
//
//   - engagement_stats.json  how much each song was listened to (play count,
//     total play time, last played) — the core "familiarity" signal.
//   - playback_history.json   individual play events with timestamps, used for
//     listening-over-time charts and time-of-day signals.
//   - favorites.json          which songs are favorited.
//   - lyrics.json             per-song lyrics whose header carries the only
//     human-readable [ti:title]/[ar:artist] tags in the whole backup, so it is
//     our bridge from the opaque songId to a title/artist we can join against
//     the gomuse analysis database.
//   - playlists.json          existing user/AI playlists (songId lists).
//
// Song identifiers appear as both JSON strings and numbers across modules, so
// every id is normalized to a string.
package pxpl

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
)

// Magic is the 4-byte prefix that precedes the embedded ZIP archive.
var Magic = []byte("PXPL")

// flexID decodes a song id that may be a JSON string or number into a string.
type flexID string

func (f *flexID) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || string(b) == "null" {
		*f = ""
		return nil
	}
	if b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*f = flexID(s)
		return nil
	}
	// Numeric id; keep the integer form without a decimal point.
	var n json.Number
	if err := json.Unmarshal(b, &n); err != nil {
		return err
	}
	*f = flexID(n.String())
	return nil
}

// EngagementStat is one song's lifetime listening summary.
type EngagementStat struct {
	SongID              flexID `json:"songId"`
	PlayCount           int    `json:"playCount"`
	TotalPlayDurationMs int64  `json:"totalPlayDurationMs"`
	LastPlayedTimestamp int64  `json:"lastPlayedTimestamp"`
}

// PlayEvent is a single playback occurrence.
type PlayEvent struct {
	SongID         flexID `json:"songId"`
	DurationMs     int64  `json:"durationMs"`
	StartTimestamp int64  `json:"startTimestamp"`
	EndTimestamp   int64  `json:"endTimestamp"`
	Timestamp      int64  `json:"timestamp"`
}

// Favorite marks a song as favorited (or not).
type Favorite struct {
	SongID     flexID `json:"songId"`
	IsFavorite bool   `json:"isFavorite"`
	Timestamp  int64  `json:"timestamp"`
}

// lyricEntry is the raw lyrics record; only the header tags are of interest.
type lyricEntry struct {
	SongID  flexID `json:"songId"`
	Content string `json:"content"`
	Source  string `json:"source"`
}

// Playlist is an existing playlist from the backup.
type Playlist struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	SongIDs       []flexID `json:"songIds"`
	IsAiGenerated bool     `json:"isAiGenerated"`
	CreatedAt     int64    `json:"createdAt"`
}

type playlistsFile struct {
	Playlists []Playlist `json:"playlists"`
}

// SongMeta is the title/artist recovered from a song's lyrics header.
type SongMeta struct {
	Title  string
	Artist string
}

// Backup is the decoded, in-memory view of a .pxpl file.
type Backup struct {
	AppVersion  string
	SchemaVer   int
	Engagement  []EngagementStat
	Plays       []PlayEvent
	Favorites   []Favorite
	Playlists   []Playlist
	Meta        map[string]SongMeta // songId -> title/artist (from lyrics)
	ArtistImage map[string]string   // artist name -> image url
}

// EngagementBySong indexes engagement stats by song id.
func (b *Backup) EngagementBySong() map[string]EngagementStat {
	m := make(map[string]EngagementStat, len(b.Engagement))
	for _, e := range b.Engagement {
		m[string(e.SongID)] = e
	}
	return m
}

// FavoriteSet returns the set of song ids currently favorited.
func (b *Backup) FavoriteSet() map[string]bool {
	m := make(map[string]bool)
	for _, f := range b.Favorites {
		if f.IsFavorite {
			m[string(f.SongID)] = true
		}
	}
	return m
}

var (
	reTitle  = regexp.MustCompile(`(?mi)^\[ti:(.*)\]\s*$`)
	reArtist = regexp.MustCompile(`(?mi)^\[ar:(.*)\]\s*$`)
)

// metaFromLyrics extracts the [ti:]/[ar:] header tags from a lyrics blob.
// Returns ok=false when neither tag is present.
func metaFromLyrics(content string) (SongMeta, bool) {
	var m SongMeta
	if g := reTitle.FindStringSubmatch(content); g != nil {
		m.Title = strings.TrimSpace(g[1])
	}
	if g := reArtist.FindStringSubmatch(content); g != nil {
		m.Artist = strings.TrimSpace(g[1])
	}
	if m.Title == "" && m.Artist == "" {
		return m, false
	}
	return m, true
}

// Parse decodes a .pxpl backup from raw bytes.
func Parse(raw []byte) (*Backup, error) {
	if !bytes.HasPrefix(raw, Magic) {
		return nil, fmt.Errorf("pxpl: missing %q magic header", Magic)
	}
	inner := raw[len(Magic):]
	zr, err := zip.NewReader(bytes.NewReader(inner), int64(len(inner)))
	if err != nil {
		return nil, fmt.Errorf("pxpl: open zip: %w", err)
	}

	b := &Backup{
		Meta:        make(map[string]SongMeta),
		ArtistImage: make(map[string]string),
	}

	for _, f := range zr.File {
		name := f.Name
		switch name {
		case "manifest.json", "engagement_stats.json", "playback_history.json",
			"favorites.json", "lyrics.json", "playlists.json", "artist_images.json":
		default:
			continue // ignore modules we don't use
		}
		data, err := readZip(f)
		if err != nil {
			return nil, fmt.Errorf("pxpl: read %s: %w", name, err)
		}
		switch name {
		case "manifest.json":
			var man struct {
				AppVersion    string `json:"appVersion"`
				SchemaVersion int    `json:"schemaVersion"`
			}
			_ = json.Unmarshal(data, &man)
			b.AppVersion = man.AppVersion
			b.SchemaVer = man.SchemaVersion
		case "engagement_stats.json":
			if err := json.Unmarshal(data, &b.Engagement); err != nil {
				return nil, fmt.Errorf("pxpl: engagement: %w", err)
			}
		case "playback_history.json":
			if err := json.Unmarshal(data, &b.Plays); err != nil {
				return nil, fmt.Errorf("pxpl: playback: %w", err)
			}
		case "favorites.json":
			if err := json.Unmarshal(data, &b.Favorites); err != nil {
				return nil, fmt.Errorf("pxpl: favorites: %w", err)
			}
		case "playlists.json":
			var pf playlistsFile
			if err := json.Unmarshal(data, &pf); err != nil {
				return nil, fmt.Errorf("pxpl: playlists: %w", err)
			}
			b.Playlists = pf.Playlists
		case "artist_images.json":
			var imgs []struct {
				ArtistName string `json:"artistName"`
				ImageURL   string `json:"imageUrl"`
			}
			_ = json.Unmarshal(data, &imgs)
			for _, im := range imgs {
				if im.ArtistName != "" && im.ImageURL != "" {
					b.ArtistImage[normArtistKey(im.ArtistName)] = im.ImageURL
				}
			}
		case "lyrics.json":
			var ents []lyricEntry
			if err := json.Unmarshal(data, &ents); err != nil {
				return nil, fmt.Errorf("pxpl: lyrics: %w", err)
			}
			for _, e := range ents {
				if m, ok := metaFromLyrics(e.Content); ok {
					b.Meta[string(e.SongID)] = m
				}
			}
		}
	}
	return b, nil
}

func readZip(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

func normArtistKey(s string) string {
	return strings.ToLower(strings.Trim(strings.TrimSpace(s), `"'`))
}
