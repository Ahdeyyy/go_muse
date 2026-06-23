// Command seedtest builds a synthetic gomuse.db from a .pxpl backup so the web
// dashboard and recommender can be exercised end-to-end without running the
// full audio analysis. It reuses the real titles/artists recovered from the
// backup's lyrics so the title+artist join produces genuine matches, and
// fabricates plausible (deterministic) genre / year / perceptual features.
//
// This is a development utility, not part of the shipped product.
package main

import (
	"flag"
	"fmt"
	"hash/fnv"
	"math"
	"os"
	"strings"
	"time"

	"github.com/Ahdeyyy/go_muse/internal/model"
	"github.com/Ahdeyyy/go_muse/internal/pxpl"
	"github.com/Ahdeyyy/go_muse/internal/store"
)

var genres = []string{
	"Hip-Hop", "R&B", "Pop", "Afrobeats", "Rock", "Electronic",
	"Indie", "Soul", "Jazz", "Alternative", "Dancehall", "Trap",
}

func main() {
	in := flag.String("in", "", "path to .pxpl backup")
	out := flag.String("out", "gomuse.db", "output gomuse.db path")
	flag.Parse()
	if *in == "" {
		fmt.Fprintln(os.Stderr, "usage: seedtest -in backup.pxpl -out gomuse.db")
		os.Exit(2)
	}

	raw, err := os.ReadFile(*in)
	must(err)
	backup, err := pxpl.Parse(raw)
	must(err)

	_ = os.Remove(*out)
	st, err := store.Open(*out)
	must(err)
	defer st.Close()

	eng := backup.EngagementBySong()
	n := 0
	for id, m := range backup.Meta {
		if strings.TrimSpace(m.Title) == "" {
			continue
		}
		h := hash64(id)
		r := newRand(h)

		// Deterministic, somewhat-correlated perceptual features.
		energy := r()
		valence := clamp(0.2 + 0.6*r())
		dance := clamp(0.3 + 0.5*r())
		acoustic := clamp(1 - 0.8*energy + 0.1*r())
		instr := clamp(0.05 + 0.3*r())
		tempo := 70 + 120*energy

		dur := 150.0 + 130*r()
		if e, ok := eng[id]; ok && e.PlayCount > 0 && e.TotalPlayDurationMs > 0 {
			avg := float64(e.TotalPlayDurationMs) / float64(e.PlayCount) / 1000.0
			if avg > 60 && avg < 600 {
				dur = avg
			}
		}

		track := model.Track{
			File: model.FileInfo{
				Path:    "/storage/emulated/0/Music/" + safe(m.Artist) + " - " + safe(m.Title) + ".mp3",
				Name:    safe(m.Title) + ".mp3",
				Ext:     ".mp3",
				Size:    int64(1_000_000 + int(h%9_000_000)),
				ModTime: time.Unix(1_600_000_000, 0),
			},
			Meta: model.Metadata{
				Title:  m.Title,
				Artist: m.Artist,
				Genre:  genres[h%uint64(len(genres))],
				Year:   1990 + int(h%35),
			},
			Low: model.LowLevel{DurationSec: dur, TempoBPM: tempo, RMSDb: -14 + 6*energy},
			Spot: model.Spotify{
				Danceability: dance, Energy: energy, Valence: valence,
				Acousticness: acoustic, Instrumentalness: instr,
				Liveness: 0.1 + 0.2*r(), Speechiness: 0.05 + 0.2*r(),
				Tempo: tempo, Loudness: -14 + 6*energy,
			},
			AnalyzedAt: time.Now(),
		}
		must(st.Upsert(track))
		n++
	}
	fmt.Printf("seeded %d tracks into %s\n", n, *out)
}

func safe(s string) string {
	return strings.NewReplacer("/", "-", "\\", "-", ":", "-").Replace(strings.TrimSpace(s))
}

func hash64(s string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return h.Sum64()
}

// newRand returns a deterministic 0..1 generator seeded by h.
func newRand(h uint64) func() float64 {
	x := h | 1
	return func() float64 {
		x ^= x << 13
		x ^= x >> 7
		x ^= x << 17
		return float64(x%1_000_000) / 1_000_000.0
	}
}

func clamp(v float64) float64 { return math.Max(0, math.Min(1, v)) }

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
