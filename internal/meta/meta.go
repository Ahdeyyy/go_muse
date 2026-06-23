// Package meta reads container tag metadata using github.com/dhowden/tag.
package meta

import (
	"os"
	"strings"

	"github.com/Ahdeyyy/go_muse/internal/model"
	"github.com/dhowden/tag"
)

// Read extracts tag metadata from an audio file. A missing or unreadable tag
// is not fatal: it returns a zero-value Metadata and a nil error so the track
// can still be analyzed acoustically.
func Read(path string) (model.Metadata, error) {
	f, err := os.Open(path)
	if err != nil {
		return model.Metadata{}, err
	}
	defer f.Close()

	m, err := tag.ReadFrom(f)
	if err != nil {
		// No/garbled tags — return empty metadata, not an error.
		return model.Metadata{}, nil
	}

	trackNum, _ := m.Track() // (number, total)
	return model.Metadata{
		Title:       strings.TrimSpace(m.Title()),
		Artist:      strings.TrimSpace(m.Artist()),
		Album:       strings.TrimSpace(m.Album()),
		AlbumArtist: strings.TrimSpace(m.AlbumArtist()),
		Genre:       strings.TrimSpace(m.Genre()),
		Year:        m.Year(),
		Track:       trackNum,
	}, nil
}
