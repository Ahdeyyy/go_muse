package score

import (
	"math"

	"github.com/Ahdeyyy/go_muse/internal/model"
)

// Reliability documents how trustworthy each heuristic is, surfaced in reports
// so the numbers aren't over-interpreted.
var Reliability = map[string]string{
	"tempo":            "high (measured)",
	"loudness":         "high (measured)",
	"key":              "medium (measured)",
	"energy":           "medium",
	"danceability":     "medium",
	"acousticness":     "medium",
	"valence":          "low-medium",
	"speechiness":      "low (no vocal model)",
	"instrumentalness": "low (no vocal model)",
	"liveness":         "very low (proxy)",
}

// Compute maps low-level DSP features to Spotify-style perceptual scores.
func Compute(low model.LowLevel, c Coefficients) model.Spotify {
	loud := norm(low.RMSDb, c.LoudnessDb)
	bright := norm(low.SpectralCentroid, c.CentroidHz)
	onset := norm(low.OnsetRate, c.OnsetRate)
	dyn := norm(low.DynamicRangeDb, c.DynamicRangeDb)
	zcr := norm(low.ZCR, c.ZCR)
	flat := clamp01(low.SpectralFlatness)
	harm := clamp01(low.HarmonicRatio)
	beat := clamp01(low.BeatStrength)
	tempoScore := gaussian(low.TempoBPM, c.TempoCenter, c.TempoWidth)

	// Mode score: major leans positive, minor negative, scaled by confidence.
	modeScore := 0.5
	switch low.Mode {
	case 1:
		modeScore = 0.5 + 0.5*low.KeyConfidence
	case 0:
		modeScore = 0.5 - 0.5*low.KeyConfidence
	}

	energy := clamp01(
		c.Energy.Loudness*loud +
			c.Energy.Centroid*bright +
			c.Energy.Onset*onset +
			c.Energy.Flatness*flat)

	dance := clamp01(
		c.Danceability.Beat*beat +
			c.Danceability.Tempo*tempoScore +
			c.Danceability.Onset*onset)

	valence := clamp01(
		c.Valence.Mode*modeScore +
			c.Valence.Brightness*bright +
			c.Valence.Tempo*tempoScore +
			c.Valence.Energy*energy)

	acoustic := clamp01(
		c.Acousticness.Harmonic*harm +
			c.Acousticness.LowBright*(1-bright) +
			c.Acousticness.LowEnergy*(1-energy) +
			c.Acousticness.DynamicRange*dyn)

	speech := clamp01(
		c.Speechiness.ZCR*zcr +
			c.Speechiness.Flatness*flat +
			c.Speechiness.Onset*onset)

	instrumental := clamp01(
		c.Instrumental.NotSpeech*(1-speech) +
			c.Instrumental.Harmonic*harm)

	liveness := clamp01(
		c.Liveness.Scale * (c.Liveness.Flatness*flat + c.Liveness.LowDynamic*(1-dyn)))

	return model.Spotify{
		Danceability:     round3(dance),
		Energy:           round3(energy),
		Valence:          round3(valence),
		Acousticness:     round3(acoustic),
		Instrumentalness: round3(instrumental),
		Liveness:         round3(liveness),
		Speechiness:      round3(speech),
		Loudness:         low.RMSDb,
		Tempo:            low.TempoBPM,
	}
}

// norm linearly maps v from [r.Lo,r.Hi] to [0,1], clamped.
func norm(v float64, r Range) float64 {
	if r.Hi == r.Lo {
		return 0
	}
	return clamp01((v - r.Lo) / (r.Hi - r.Lo))
}

func gaussian(v, center, width float64) float64 {
	if width == 0 {
		return 0
	}
	d := (v - center) / width
	return math.Exp(-0.5 * d * d)
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func round3(v float64) float64 { return math.Round(v*1000) / 1000 }
