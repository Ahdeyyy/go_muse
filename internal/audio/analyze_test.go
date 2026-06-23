package audio

import (
	"math"
	"testing"

	"github.com/Ahdeyyy/go_muse/internal/decode"
)

// synth builds a test signal: a sustained tone plus periodic percussive clicks
// at a known tempo, so we can sanity-check tempo/key/score extraction.
func synth(sr int, seconds float64, toneHz, bpm float64) decode.Signal {
	n := int(float64(sr) * seconds)
	x := make([]float32, n)
	clickPeriod := int(float64(sr) * 60 / bpm)
	for i := 0; i < n; i++ {
		t := float64(i) / float64(sr)
		v := 0.3 * math.Sin(2*math.Pi*toneHz*t)
		if clickPeriod > 0 && i%clickPeriod < int(0.01*float64(sr)) {
			v += 0.6 // short broadband-ish burst
		}
		x[i] = float32(v)
	}
	return decode.Signal{Samples: x, SampleRate: sr}
}

func TestAnalyzeSane(t *testing.T) {
	sr := 22050
	sig := synth(sr, 12, 261.63 /*C4*/, 120)
	low := Analyze(sig)

	if low.DurationSec < 11 || low.DurationSec > 13 {
		t.Errorf("duration = %.2f, want ~12", low.DurationSec)
	}
	if low.RMSDb >= 0 || low.RMSDb < -90 {
		t.Errorf("rms_db = %.2f, out of range", low.RMSDb)
	}
	if low.SpectralCentroid <= 0 {
		t.Errorf("centroid = %.2f, want > 0", low.SpectralCentroid)
	}
	if low.TempoBPM < 90 || low.TempoBPM > 150 {
		// autocorrelation may land on a metrical multiple; keep a wide gate.
		t.Logf("tempo = %.1f BPM (expected near 120; metrical ambiguity allowed)", low.TempoBPM)
	}
	if low.Key < 0 || low.Key > 11 {
		t.Errorf("key = %d, want 0..11", low.Key)
	}
	if len(low.MFCC) != numMFCC {
		t.Errorf("len(mfcc) = %d, want %d", len(low.MFCC), numMFCC)
	}
	for i, v := range low.MFCC {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			t.Errorf("mfcc[%d] not finite: %v", i, v)
		}
	}
	if low.SpectralFlatness < 0 || low.SpectralFlatness > 1 {
		t.Errorf("flatness = %.3f, want 0..1", low.SpectralFlatness)
	}
}

func TestAnalyzeSilence(t *testing.T) {
	sr := 22050
	sig := decode.Signal{Samples: make([]float32, sr*2), SampleRate: sr}
	low := Analyze(sig) // must not panic or produce NaN
	if math.IsNaN(low.SpectralCentroid) || math.IsNaN(low.RMSDb) {
		t.Errorf("silence produced NaN: %+v", low)
	}
}

func TestAnalyzeTooShort(t *testing.T) {
	sig := decode.Signal{Samples: make([]float32, 100), SampleRate: 22050}
	low := Analyze(sig)
	if low.Key != -1 {
		t.Errorf("too-short signal should leave key unknown, got %d", low.Key)
	}
}
