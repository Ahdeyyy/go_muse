// Package store persists analyzed tracks to a pure-Go SQLite database and
// exports CSV. The (path, size, mod_time) triple acts as an incremental cache
// so re-runs skip files that have not changed.
package store

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/Ahdeyyy/go_muse/internal/model"
	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

// Open opens (creating if needed) the SQLite database at path.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	// Single writer; pragmas for speed/safety on flash storage.
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA journal_mode=WAL; PRAGMA synchronous=NORMAL;`); err != nil {
		db.Close()
		return nil, err
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS tracks (
    path           TEXT PRIMARY KEY,
    name           TEXT,
    ext            TEXT,
    size           INTEGER,
    mod_time_unix  INTEGER,
    title          TEXT,
    artist         TEXT,
    album          TEXT,
    album_artist   TEXT,
    genre          TEXT,
    year           INTEGER,
    track_num      INTEGER,
    duration_sec       REAL,
    sample_rate        INTEGER,
    rms_db             REAL,
    peak_db            REAL,
    dynamic_range_db   REAL,
    zcr                REAL,
    spectral_centroid  REAL,
    spectral_rolloff   REAL,
    spectral_bandwidth REAL,
    spectral_flatness  REAL,
    onset_rate         REAL,
    tempo_bpm          REAL,
    beat_strength      REAL,
    music_key          INTEGER,
    music_mode         INTEGER,
    key_confidence     REAL,
    harmonic_ratio     REAL,
    mfcc_json          TEXT,
    danceability       REAL,
    energy             REAL,
    valence            REAL,
    acousticness       REAL,
    instrumentalness   REAL,
    liveness           REAL,
    speechiness        REAL,
    analyzed_at_unix   INTEGER,
    error              TEXT
);`)
	return err
}

// IsFresh reports whether the file at fi is already analyzed with matching size
// and mod time (and no stored error), so it can be skipped.
func (s *Store) IsFresh(fi model.FileInfo) (bool, error) {
	var size, mod int64
	var errStr sql.NullString
	row := s.db.QueryRow(
		`SELECT size, mod_time_unix, error FROM tracks WHERE path = ?`, fi.Path)
	switch err := row.Scan(&size, &mod, &errStr); err {
	case sql.ErrNoRows:
		return false, nil
	case nil:
		fresh := size == fi.Size &&
			mod == fi.ModTime.UnixNano() &&
			(!errStr.Valid || errStr.String == "")
		return fresh, nil
	default:
		return false, err
	}
}

// Upsert inserts or replaces a track record.
func (s *Store) Upsert(t model.Track) error {
	mfccJSON, _ := json.Marshal(t.Low.MFCC)
	_, err := s.db.Exec(`
INSERT INTO tracks (
    path,name,ext,size,mod_time_unix,
    title,artist,album,album_artist,genre,year,track_num,
    duration_sec,sample_rate,rms_db,peak_db,dynamic_range_db,zcr,
    spectral_centroid,spectral_rolloff,spectral_bandwidth,spectral_flatness,
    onset_rate,tempo_bpm,beat_strength,music_key,music_mode,key_confidence,
    harmonic_ratio,mfcc_json,
    danceability,energy,valence,acousticness,instrumentalness,liveness,speechiness,
    analyzed_at_unix,error
) VALUES (?,?,?,?,?, ?,?,?,?,?,?,?, ?,?,?,?,?,?, ?,?,?,?, ?,?,?,?,?,?, ?,?, ?,?,?,?,?,?,?, ?,?)
ON CONFLICT(path) DO UPDATE SET
    name=excluded.name,ext=excluded.ext,size=excluded.size,mod_time_unix=excluded.mod_time_unix,
    title=excluded.title,artist=excluded.artist,album=excluded.album,album_artist=excluded.album_artist,
    genre=excluded.genre,year=excluded.year,track_num=excluded.track_num,
    duration_sec=excluded.duration_sec,sample_rate=excluded.sample_rate,rms_db=excluded.rms_db,
    peak_db=excluded.peak_db,dynamic_range_db=excluded.dynamic_range_db,zcr=excluded.zcr,
    spectral_centroid=excluded.spectral_centroid,spectral_rolloff=excluded.spectral_rolloff,
    spectral_bandwidth=excluded.spectral_bandwidth,spectral_flatness=excluded.spectral_flatness,
    onset_rate=excluded.onset_rate,tempo_bpm=excluded.tempo_bpm,beat_strength=excluded.beat_strength,
    music_key=excluded.music_key,music_mode=excluded.music_mode,key_confidence=excluded.key_confidence,
    harmonic_ratio=excluded.harmonic_ratio,mfcc_json=excluded.mfcc_json,
    danceability=excluded.danceability,energy=excluded.energy,valence=excluded.valence,
    acousticness=excluded.acousticness,instrumentalness=excluded.instrumentalness,
    liveness=excluded.liveness,speechiness=excluded.speechiness,
    analyzed_at_unix=excluded.analyzed_at_unix,error=excluded.error;`,
		t.File.Path, t.File.Name, t.File.Ext, t.File.Size, t.File.ModTime.UnixNano(),
		t.Meta.Title, t.Meta.Artist, t.Meta.Album, t.Meta.AlbumArtist, t.Meta.Genre, t.Meta.Year, t.Meta.Track,
		t.Low.DurationSec, t.Low.SampleRate, t.Low.RMSDb, t.Low.PeakDb, t.Low.DynamicRangeDb, t.Low.ZCR,
		t.Low.SpectralCentroid, t.Low.SpectralRolloff, t.Low.SpectralBandwidth, t.Low.SpectralFlatness,
		t.Low.OnsetRate, t.Low.TempoBPM, t.Low.BeatStrength, t.Low.Key, t.Low.Mode, t.Low.KeyConfidence,
		t.Low.HarmonicRatio, string(mfccJSON),
		t.Spot.Danceability, t.Spot.Energy, t.Spot.Valence, t.Spot.Acousticness,
		t.Spot.Instrumentalness, t.Spot.Liveness, t.Spot.Speechiness,
		t.AnalyzedAt.Unix(), t.Error,
	)
	return err
}

// Count returns the number of stored tracks (without errors).
func (s *Store) Count() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM tracks WHERE error = '' OR error IS NULL`).Scan(&n)
	return n, err
}

// GenreCount is one row of the genre histogram.
type GenreCount struct {
	Genre string
	N     int
}

// GenreHistogram returns genre counts, most common first ("" => Unknown).
func (s *Store) GenreHistogram() ([]GenreCount, error) {
	rows, err := s.db.Query(`
SELECT CASE WHEN genre IS NULL OR genre='' THEN '(unknown)' ELSE genre END AS g,
       COUNT(*) FROM tracks GROUP BY g ORDER BY COUNT(*) DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GenreCount
	for rows.Next() {
		var gc GenreCount
		if err := rows.Scan(&gc.Genre, &gc.N); err != nil {
			return nil, err
		}
		out = append(out, gc)
	}
	return out, rows.Err()
}

// FeatureAverages holds library-wide means of the perceptual scores.
type FeatureAverages struct {
	Danceability, Energy, Valence, Acousticness float64
	Instrumentalness, Liveness, Speechiness     float64
	Tempo, Loudness                             float64
}

// Averages computes mean perceptual scores across analyzed tracks.
func (s *Store) Averages() (FeatureAverages, error) {
	var a FeatureAverages
	err := s.db.QueryRow(`
SELECT AVG(danceability),AVG(energy),AVG(valence),AVG(acousticness),
       AVG(instrumentalness),AVG(liveness),AVG(speechiness),AVG(tempo_bpm),AVG(rms_db)
FROM tracks WHERE error='' OR error IS NULL`).Scan(
		nz(&a.Danceability), nz(&a.Energy), nz(&a.Valence), nz(&a.Acousticness),
		nz(&a.Instrumentalness), nz(&a.Liveness), nz(&a.Speechiness), nz(&a.Tempo), nz(&a.Loudness))
	return a, err
}

// nz scans a possibly-NULL average into a float64 (NULL -> 0).
func nz(p *float64) any { return (*nullFloat)(p) }

type nullFloat float64

func (n *nullFloat) Scan(v any) error {
	if v == nil {
		*n = 0
		return nil
	}
	switch t := v.(type) {
	case float64:
		*n = nullFloat(t)
	case int64:
		*n = nullFloat(t)
	default:
		return fmt.Errorf("unexpected avg type %T", v)
	}
	return nil
}
