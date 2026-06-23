// Package model holds the shared data structures passed between the scan,
// decode, audio-analysis, scoring and storage layers.
package model

import "time"

// FileInfo identifies an audio file on disk. The triple (Path, Size, ModTime)
// is the cache key: if none changed since the last run, analysis is skipped.
type FileInfo struct {
	Path    string    `json:"path"`
	Name    string    `json:"name"`
	Ext     string    `json:"ext"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
}

// Metadata is the tag information read from the container (ID3, MP4, etc).
type Metadata struct {
	Title       string `json:"title"`
	Artist      string `json:"artist"`
	Album       string `json:"album"`
	AlbumArtist string `json:"album_artist"`
	Genre       string `json:"genre"`
	Year        int    `json:"year"`
	Track       int    `json:"track"`
}

// LowLevel holds the objective DSP measurements extracted from the waveform.
// These are real measurements (as opposed to the heuristic Spotify scores).
type LowLevel struct {
	DurationSec       float64   `json:"duration_sec"`
	SampleRate        int       `json:"sample_rate"`
	RMSDb             float64   `json:"rms_db"`             // average loudness
	PeakDb            float64   `json:"peak_db"`            // peak level
	DynamicRangeDb    float64   `json:"dynamic_range_db"`   // crest-ish factor
	ZCR               float64   `json:"zcr"`                // zero-crossing rate
	SpectralCentroid  float64   `json:"spectral_centroid"`  // brightness, Hz
	SpectralRolloff   float64   `json:"spectral_rolloff"`   // 85% energy, Hz
	SpectralBandwidth float64   `json:"spectral_bandwidth"` // spread, Hz
	SpectralFlatness  float64   `json:"spectral_flatness"`  // tonal vs noise, 0..1
	OnsetRate         float64   `json:"onset_rate"`         // onsets per second
	TempoBPM          float64   `json:"tempo_bpm"`
	BeatStrength      float64   `json:"beat_strength"` // 0..1 rhythmic salience
	Key               int       `json:"key"`           // 0=C .. 11=B, -1 unknown
	Mode              int       `json:"mode"`          // 1=major, 0=minor, -1 unknown
	KeyConfidence     float64   `json:"key_confidence"`
	HarmonicRatio     float64   `json:"harmonic_ratio"` // tonal energy fraction
	MFCC              []float64 `json:"mfcc"`           // mean of first N coeffs
}

// Spotify holds the heuristic, Spotify-style perceptual scores. All 0..1
// except Tempo (BPM) and Loudness (dB). These are approximations, not the
// output of Spotify's proprietary models.
type Spotify struct {
	Danceability     float64 `json:"danceability"`
	Energy           float64 `json:"energy"`
	Valence          float64 `json:"valence"`
	Acousticness     float64 `json:"acousticness"`
	Instrumentalness float64 `json:"instrumentalness"`
	Liveness         float64 `json:"liveness"`
	Speechiness      float64 `json:"speechiness"`
	Loudness         float64 `json:"loudness"` // dB (= LowLevel.RMSDb)
	Tempo            float64 `json:"tempo"`    // BPM (= LowLevel.TempoBPM)
}

// Track is the full per-file analysis record persisted to SQLite/CSV.
type Track struct {
	File       FileInfo  `json:"file"`
	Meta       Metadata  `json:"meta"`
	Low        LowLevel  `json:"low"`
	Spot       Spotify   `json:"spotify"`
	AnalyzedAt time.Time `json:"analyzed_at"`
	Error      string    `json:"error,omitempty"`
}
