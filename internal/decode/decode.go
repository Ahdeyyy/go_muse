// Package decode turns any audio file into a mono float32 PCM signal by
// shelling out to ffmpeg. This gives uniform handling of mp3/m4a/aac/flac/
// ogg/opus/wav without per-format Go decoders.
package decode

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"os/exec"
	"time"
)

// Options controls decoding and excerpting.
type Options struct {
	SampleRate int           // target sample rate, e.g. 22050
	SkipSec    float64       // seconds to skip from the start (-ss)
	LenSec     float64       // seconds to decode; 0 = whole file (-t)
	Timeout    time.Duration // hard cap per file
	FFmpegPath string        // "ffmpeg" by default
}

// DefaultOptions returns analysis-friendly defaults: a 90s excerpt taken 30s
// in (skips intros, keeps runtime bounded for large libraries).
func DefaultOptions() Options {
	return Options{
		SampleRate: 22050,
		SkipSec:    30,
		LenSec:     90,
		Timeout:    60 * time.Second,
		FFmpegPath: "ffmpeg",
	}
}

// Signal is a decoded mono waveform.
type Signal struct {
	Samples    []float32
	SampleRate int
}

// Duration returns the decoded (excerpt) length in seconds.
func (s Signal) Duration() float64 {
	if s.SampleRate == 0 {
		return 0
	}
	return float64(len(s.Samples)) / float64(s.SampleRate)
}

// Decode runs ffmpeg and reads f32le mono PCM from its stdout.
func Decode(ctx context.Context, path string, o Options) (Signal, error) {
	if o.SampleRate == 0 {
		o.SampleRate = 22050
	}
	if o.FFmpegPath == "" {
		o.FFmpegPath = "ffmpeg"
	}
	if o.Timeout == 0 {
		o.Timeout = 60 * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, o.Timeout)
	defer cancel()

	args := []string{"-v", "error", "-nostdin"}
	if o.SkipSec > 0 {
		args = append(args, "-ss", trim(o.SkipSec)) // before -i: fast seek
	}
	args = append(args, "-i", path)
	if o.LenSec > 0 {
		args = append(args, "-t", trim(o.LenSec))
	}
	args = append(args,
		"-vn",                                  // ignore cover art / video
		"-ac", "1",                             // mono
		"-ar", fmt.Sprintf("%d", o.SampleRate), // resample
		"-f", "f32le", // raw little-endian float32
		"-",
	)

	cmd := exec.CommandContext(ctx, o.FFmpegPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return Signal{}, fmt.Errorf("ffmpeg: %w: %s", err, trunc(stderr.String()))
	}

	raw := stdout.Bytes()
	n := len(raw) / 4
	if n == 0 {
		return Signal{}, fmt.Errorf("ffmpeg produced no audio (excerpt past end of file?)")
	}
	samples := make([]float32, n)
	for i := 0; i < n; i++ {
		bits := binary.LittleEndian.Uint32(raw[i*4:])
		v := math.Float32frombits(bits)
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			v = 0
		}
		samples[i] = v
	}
	return Signal{Samples: samples, SampleRate: o.SampleRate}, nil
}

func trim(sec float64) string { return fmt.Sprintf("%.3f", sec) }

func trunc(s string) string {
	const max = 200
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}
