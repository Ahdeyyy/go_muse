package store

import (
	"encoding/csv"
	"os"
	"strconv"
)

// csvColumns is the export column order.
var csvColumns = []string{
	"path", "name", "ext", "artist", "album", "title", "genre", "year",
	"duration_sec", "tempo_bpm", "music_key", "music_mode", "key_confidence",
	"rms_db", "peak_db", "dynamic_range_db", "zcr",
	"spectral_centroid", "spectral_rolloff", "spectral_bandwidth", "spectral_flatness",
	"onset_rate", "beat_strength", "harmonic_ratio",
	"danceability", "energy", "valence", "acousticness",
	"instrumentalness", "liveness", "speechiness",
}

// ExportCSV writes one row per analyzed track to a CSV file.
func (s *Store) ExportCSV(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()

	if err := w.Write(csvColumns); err != nil {
		return err
	}

	rows, err := s.db.Query(`
SELECT path,name,ext,artist,album,title,genre,year,
       duration_sec,tempo_bpm,music_key,music_mode,key_confidence,
       rms_db,peak_db,dynamic_range_db,zcr,
       spectral_centroid,spectral_rolloff,spectral_bandwidth,spectral_flatness,
       onset_rate,beat_strength,harmonic_ratio,
       danceability,energy,valence,acousticness,instrumentalness,liveness,speechiness
FROM tracks WHERE error='' OR error IS NULL`)
	if err != nil {
		return err
	}
	defer rows.Close()

	cols, _ := rows.Columns()
	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	rec := make([]string, len(cols))
	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			return err
		}
		for i, v := range vals {
			rec[i] = cell(v)
		}
		if err := w.Write(rec); err != nil {
			return err
		}
	}
	return rows.Err()
}

// cell renders a scanned SQL value as a CSV string.
func cell(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case []byte:
		return string(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case float64:
		return strconv.FormatFloat(t, 'g', 6, 64)
	case bool:
		if t {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}
