// Package library loads analyzed tracks from a gomuse SQLite database and
// joins them against listening data from a PixelPlayer .pxpl backup.
//
// The gomuse database supplies the objective + perceptual attributes (genre,
// year, energy, valence, danceability, …) and, crucially, the on-disk file
// path needed to write an .m3u playlist. The backup supplies engagement
// (play counts, favorites, recency). The two are joined on a normalized
// title+artist key, since the backup's songId is opaque and the lyrics module
// is the only place a human-readable title/artist exists.
package library

import (
	"database/sql"
	"regexp"
	"sort"
	"strings"

	"github.com/Ahdeyyy/go_muse/internal/pxpl"
	_ "modernc.org/sqlite"
)

// Song is a unified view of one track: analysis attributes plus (when matched)
// listening engagement.
type Song struct {
	Path        string  `json:"path"`
	Title       string  `json:"title"`
	Artist      string  `json:"artist"`
	Album       string  `json:"album"`
	Genre       string  `json:"genre"`
	Year        int     `json:"year"`
	DurationSec float64 `json:"durationSec"`

	// Perceptual attributes, all 0..1 except Tempo (BPM) and Loudness (dB).
	Energy           float64 `json:"energy"`
	Valence          float64 `json:"valence"`
	Danceability     float64 `json:"danceability"`
	Acousticness     float64 `json:"acousticness"`
	Instrumentalness float64 `json:"instrumentalness"`
	Liveness         float64 `json:"liveness"`
	Speechiness      float64 `json:"speechiness"`
	Tempo            float64 `json:"tempo"`
	Loudness         float64 `json:"loudness"`

	// Engagement (zero when the track is not present in the backup).
	SongID      string `json:"songId,omitempty"`
	PlayCount   int    `json:"playCount"`
	TotalPlayMs int64  `json:"totalPlayMs"`
	LastPlayed  int64  `json:"lastPlayed"`
	Favorite    bool   `json:"favorite"`
	Matched     bool   `json:"matched"`

	// Familiarity is the play-count percentile among matched songs, 0..1.
	// 0 = never/rarely played (a "discovery"), 1 = one of the most played.
	Familiarity float64 `json:"familiarity"`
}

// Library is the joined dataset the rest of the app operates on.
type Library struct {
	Songs   []Song
	Backup  *pxpl.Backup
	HasDB   bool
	HasPxpl bool
}

// LoadDB reads all analyzed tracks (without errors) from a gomuse database.
func LoadDB(path string) ([]Song, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`
SELECT path,title,artist,album,genre,year,duration_sec,
       energy,valence,danceability,acousticness,instrumentalness,liveness,speechiness,
       tempo_bpm,rms_db
FROM tracks
WHERE (error IS NULL OR error='')`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Song
	for rows.Next() {
		var s Song
		var title, artist, album, genre sql.NullString
		if err := rows.Scan(
			&s.Path, &title, &artist, &album, &genre, &s.Year, &s.DurationSec,
			&s.Energy, &s.Valence, &s.Danceability, &s.Acousticness,
			&s.Instrumentalness, &s.Liveness, &s.Speechiness,
			&s.Tempo, &s.Loudness,
		); err != nil {
			return nil, err
		}
		s.Title = title.String
		s.Artist = artist.String
		s.Album = album.String
		s.Genre = genre.String
		out = append(out, s)
	}
	return out, rows.Err()
}

// Build joins analysis songs with a backup (which may be nil) and computes
// familiarity percentiles over the matched set.
func Build(songs []Song, backup *pxpl.Backup) *Library {
	lib := &Library{Songs: songs, Backup: backup, HasDB: len(songs) > 0}
	if backup == nil {
		return lib
	}
	lib.HasPxpl = true

	// Index the backup: title+artist key -> songId (from lyrics metadata).
	keyToID := make(map[string]string, len(backup.Meta))
	for id, m := range backup.Meta {
		keyToID[matchKey(m.Title, m.Artist)] = id
		// Also index by title alone as a weaker fallback.
		if k := matchKey(m.Title, ""); k != "" {
			if _, exists := keyToID[k]; !exists {
				keyToID[k] = id
			}
		}
	}
	eng := backup.EngagementBySong()
	favs := backup.FavoriteSet()

	for i := range lib.Songs {
		s := &lib.Songs[i]
		id := keyToID[matchKey(s.Title, s.Artist)]
		if id == "" {
			id = keyToID[matchKey(s.Title, "")] // fallback: title only
		}
		if id == "" {
			continue
		}
		s.SongID = id
		s.Matched = true
		if e, ok := eng[id]; ok {
			s.PlayCount = e.PlayCount
			s.TotalPlayMs = e.TotalPlayDurationMs
			s.LastPlayed = e.LastPlayedTimestamp
		}
		s.Favorite = favs[id]
	}

	computeFamiliarity(lib.Songs)
	return lib
}

// computeFamiliarity assigns each matched song a 0..1 percentile rank by play
// count. Ties share the average rank; unmatched songs stay at 0.
func computeFamiliarity(songs []Song) {
	type ref struct {
		idx   int
		plays int
	}
	var matched []ref
	for i := range songs {
		if songs[i].Matched {
			matched = append(matched, ref{i, songs[i].PlayCount})
		}
	}
	if len(matched) <= 1 {
		for _, r := range matched {
			songs[r.idx].Familiarity = 0
		}
		return
	}
	sort.Slice(matched, func(a, b int) bool { return matched[a].plays < matched[b].plays })
	n := len(matched)
	for rank, r := range matched {
		songs[r.idx].Familiarity = float64(rank) / float64(n-1)
	}
}

// Stats summarizes the library for the dashboard.
func (lib *Library) MatchedCount() int {
	n := 0
	for _, s := range lib.Songs {
		if s.Matched {
			n++
		}
	}
	return n
}

var (
	reFeat   = regexp.MustCompile(`(?i)\s*[\(\[]\s*(feat|ft|featuring|with|prod)\.?.*?[\)\]]`)
	reParen  = regexp.MustCompile(`\s*[\(\[][^\)\]]*[\)\]]`)
	reNonAN  = regexp.MustCompile(`[^a-z0-9]+`)
	reSpaces = regexp.MustCompile(`\s+`)
)

// matchKey normalizes a title/artist pair into a stable join key. It strips
// "feat."/parenthetical noise, lowercases, removes punctuation, and uses only
// the primary artist (text before a comma / "&" / "feat").
func matchKey(title, artist string) string {
	t := normPart(title)
	if t == "" {
		return ""
	}
	if artist == "" {
		return t
	}
	a := primaryArtist(artist)
	a = normPart(a)
	if a == "" {
		return t
	}
	return t + "|" + a
}

func primaryArtist(artist string) string {
	a := strings.TrimSpace(artist)
	for _, sep := range []string{",", "&", " feat", " ft", " featuring", " with ", " x ", "/"} {
		if i := strings.Index(strings.ToLower(a), strings.TrimSpace(sep)); i > 0 {
			a = a[:i]
		}
	}
	return a
}

func normPart(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = reFeat.ReplaceAllString(s, "")
	s = reParen.ReplaceAllString(s, "")
	s = reNonAN.ReplaceAllString(s, " ")
	s = reSpaces.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}
