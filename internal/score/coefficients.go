// Package score converts objective DSP features into Spotify-style perceptual
// scores via tunable heuristic formulas. There is no training step; instead the
// formula coefficients live in a JSON file that is loaded each run (and written
// with defaults if absent), so the model stays inspectable and adjustable.
package score

import (
	"encoding/json"
	"os"
)

// Range is an inclusive [Lo,Hi] used to linearly normalize a measurement to
// 0..1 (values outside are clamped).
type Range struct {
	Lo float64 `json:"lo"`
	Hi float64 `json:"hi"`
}

// Coefficients holds every tunable constant. Weights within a feature are
// applied to already-normalized 0..1 sub-scores; they need not sum to 1 (the
// result is clamped). Tweak this file and re-run to recalibrate.
type Coefficients struct {
	// Shared normalization ranges (measurement units noted).
	LoudnessDb     Range `json:"loudness_db"`     // RMS dB -> 0..1
	CentroidHz     Range `json:"centroid_hz"`     // brightness
	OnsetRate      Range `json:"onset_rate"`      // onsets/sec
	DynamicRangeDb Range `json:"dynamic_range_db"` // crest factor
	ZCR            Range `json:"zcr"`              // zero-crossing rate

	TempoCenter float64 `json:"tempo_center"` // most "danceable" BPM
	TempoWidth  float64 `json:"tempo_width"`  // gaussian width (BPM)

	Energy       EnergyW       `json:"energy"`
	Danceability DanceW        `json:"danceability"`
	Valence      ValenceW      `json:"valence"`
	Acousticness AcousticW     `json:"acousticness"`
	Speechiness  SpeechW       `json:"speechiness"`
	Instrumental InstrumentalW `json:"instrumentalness"`
	Liveness     LivenessW     `json:"liveness"`
}

type EnergyW struct {
	Loudness float64 `json:"loudness"`
	Centroid float64 `json:"centroid"`
	Onset    float64 `json:"onset"`
	Flatness float64 `json:"flatness"`
}
type DanceW struct {
	Beat  float64 `json:"beat"`
	Tempo float64 `json:"tempo"`
	Onset float64 `json:"onset"`
}
type ValenceW struct {
	Mode       float64 `json:"mode"`
	Brightness float64 `json:"brightness"`
	Tempo      float64 `json:"tempo"`
	Energy     float64 `json:"energy"`
}
type AcousticW struct {
	Harmonic     float64 `json:"harmonic"`
	LowBright    float64 `json:"low_brightness"`
	LowEnergy    float64 `json:"low_energy"`
	DynamicRange float64 `json:"dynamic_range"`
}
type SpeechW struct {
	ZCR      float64 `json:"zcr"`
	Flatness float64 `json:"flatness"`
	Onset    float64 `json:"onset"`
}
type InstrumentalW struct {
	NotSpeech float64 `json:"not_speech"`
	Harmonic  float64 `json:"harmonic"`
}
type LivenessW struct {
	Flatness  float64 `json:"flatness"`
	LowDynamic float64 `json:"low_dynamic"`
	Scale     float64 `json:"scale"`
}

// Default returns hand-tuned starting coefficients.
func Default() Coefficients {
	return Coefficients{
		LoudnessDb:     Range{Lo: -30, Hi: -6},
		CentroidHz:     Range{Lo: 500, Hi: 4000},
		OnsetRate:      Range{Lo: 0.5, Hi: 6},
		DynamicRangeDb: Range{Lo: 3, Hi: 18},
		ZCR:            Range{Lo: 0.03, Hi: 0.20},
		TempoCenter:    118,
		TempoWidth:     45,
		Energy:         EnergyW{Loudness: 0.5, Centroid: 0.25, Onset: 0.2, Flatness: 0.05},
		Danceability:   DanceW{Beat: 0.5, Tempo: 0.3, Onset: 0.2},
		Valence:        ValenceW{Mode: 0.35, Brightness: 0.25, Tempo: 0.2, Energy: 0.2},
		Acousticness:   AcousticW{Harmonic: 0.4, LowBright: 0.3, LowEnergy: 0.2, DynamicRange: 0.1},
		Speechiness:    SpeechW{ZCR: 0.5, Flatness: 0.2, Onset: 0.3},
		Instrumental:   InstrumentalW{NotSpeech: 0.6, Harmonic: 0.4},
		Liveness:       LivenessW{Flatness: 0.5, LowDynamic: 0.5, Scale: 0.6},
	}
}

// Load reads coefficients from path. If the file does not exist it writes the
// defaults to that path and returns them.
func Load(path string) (Coefficients, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		c := Default()
		if werr := Save(path, c); werr != nil {
			return c, werr
		}
		return c, nil
	}
	if err != nil {
		return Coefficients{}, err
	}
	var c Coefficients
	if err := json.Unmarshal(b, &c); err != nil {
		return Coefficients{}, err
	}
	return c, nil
}

// Save writes coefficients as indented JSON.
func Save(path string, c Coefficients) error {
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}
