// Package m3u writes extended M3U playlists (#EXTM3U / #EXTINF) from songs.
//
// The extended format carries a per-track duration and "Artist - Title"
// display label, which most players (including the source PixelPlayer app on
// Android) read back. Track location is the song's on-disk path from the
// gomuse analysis, so the playlist works on the device the music lives on.
package m3u

import (
	"strconv"
	"strings"

	"github.com/Ahdeyyy/go_muse/internal/library"
)

// Write renders songs as an extended .m3u document. name is an optional
// playlist title recorded as a leading comment.
func Write(name string, songs []library.Song) string {
	var b strings.Builder
	b.WriteString("#EXTM3U\n")
	if name = strings.TrimSpace(name); name != "" {
		b.WriteString("#PLAYLIST:")
		b.WriteString(sanitize(name))
		b.WriteByte('\n')
	}
	for _, s := range songs {
		secs := int(s.DurationSec + 0.5)
		label := displayLabel(s)
		b.WriteString("#EXTINF:")
		b.WriteString(strconv.Itoa(secs))
		b.WriteByte(',')
		b.WriteString(label)
		b.WriteByte('\n')
		b.WriteString(s.Path)
		b.WriteByte('\n')
	}
	return b.String()
}

func displayLabel(s library.Song) string {
	title := strings.TrimSpace(s.Title)
	artist := strings.TrimSpace(s.Artist)
	switch {
	case artist != "" && title != "":
		return sanitize(artist + " - " + title)
	case title != "":
		return sanitize(title)
	case artist != "":
		return sanitize(artist)
	default:
		return sanitize(baseName(s.Path))
	}
}

// sanitize strips newlines so a value can't break the line-oriented format.
func sanitize(s string) string {
	return strings.NewReplacer("\r", " ", "\n", " ").Replace(s)
}

func baseName(p string) string {
	p = strings.ReplaceAll(p, "\\", "/")
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		return p[i+1:]
	}
	return p
}
