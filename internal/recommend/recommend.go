// Package recommend turns a user's playlist request (mood, activity, era,
// energy, discovery, filters, size bounds) into a ranked list of songs.
//
// The model is target-vector similarity. Mood and activity presets, plus the
// explicit energy/discovery inputs, are blended into a single weighted target
// over perceptual dimensions. Every candidate that survives the hard filters
// (genre / artist / favorites) is scored by its weighted closeness to that
// target, combined with a familiarity term (driven by discovery) and a soft
// era preference. The top scorers are returned, with a light per-artist cap so
// one artist can't dominate.
package recommend

import (
	"math"
	"math/rand"
	"sort"
	"strconv"
	"strings"

	"github.com/Ahdeyyy/go_muse/internal/library"
)

// Request is a playlist generation request from the UI.
type Request struct {
	Mood          string   `json:"mood"`
	Activity      string   `json:"activity"`
	Era           string   `json:"era"`
	Energy        *float64 `json:"energy"`    // optional 0..1 override
	Discovery     float64  `json:"discovery"` // 0..1; higher => more unfamiliar
	MinSongs      int      `json:"minSongs"`
	MaxSongs      int      `json:"maxSongs"`
	Genres        []string `json:"genres"`        // include filter (any match)
	Artists       []string `json:"artists"`       // include filter (any match)
	FavoritesOnly bool     `json:"favoritesOnly"` // require favorited
	StrictEra     bool     `json:"strictEra"`     // era as hard filter
	Seed          int64    `json:"seed"`          // tie-break shuffle seed
}

// Result is the generated playlist plus diagnostics.
type Result struct {
	Songs     []library.Song     `json:"songs"`
	Target    map[string]float64 `json:"target"`
	Candidates int               `json:"candidates"` // passed hard filters
	Requested  int               `json:"requested"`  // MaxSongs
	Notes      []string          `json:"notes"`
}

// blend weights for the three score components.
const (
	wSimilarity = 1.0
	wFamiliarity = 0.6
	wEra         = 0.4
)

// Generate produces a playlist for req from the library's songs.
func Generate(songs []library.Song, req Request) Result {
	req = normalize(req)
	target, weights := buildTarget(req)

	var notes []string

	// 1) Hard filters.
	genreF := toLowerSet(req.Genres)
	artistF := toLowerSet(req.Artists)
	minYear, maxYear, eraOK := eraRange(req.Era)

	var cands []library.Song
	for _, s := range songs {
		if req.FavoritesOnly && !s.Favorite {
			continue
		}
		if len(genreF) > 0 && !matchAny(s.Genre, genreF) {
			continue
		}
		if len(artistF) > 0 && !matchAny(s.Artist, artistF) {
			continue
		}
		if req.StrictEra && eraOK && (s.Year < minYear || s.Year > maxYear) {
			continue
		}
		cands = append(cands, s)
	}

	// 2) Score every candidate. The discovery and era terms only carry weight
	// when the user actually expressed a preference: a discovery slider left at
	// the neutral midpoint (0.5) contributes nothing, and era weighs in only
	// when an era was chosen. This keeps an unset control from quietly
	// dominating the mood/activity match.
	targetFam := 1 - req.Discovery
	effWFam := wFamiliarity * math.Abs(req.Discovery-0.5) * 2 // 0 at neutral, wFam at extremes
	effWEra := 0.0
	if eraOK && !req.StrictEra {
		effWEra = wEra
	}
	denom := wSimilarity + effWFam + effWEra

	type scored struct {
		s     library.Song
		score float64
	}
	scoredList := make([]scored, 0, len(cands))
	for _, s := range cands {
		sim := similarity(s, target, weights)

		fam := s.Familiarity // 0 when unmatched (treated as a discovery)
		famScore := 1 - math.Abs(fam-targetFam)

		eraScore := 0.0
		if effWEra > 0 {
			eraScore = eraCloseness(s.Year, minYear, maxYear)
		}

		total := (wSimilarity*sim + effWFam*famScore + effWEra*eraScore) / denom
		scoredList = append(scoredList, scored{s, total})
	}

	// 3) Sort by score; jitter ties so regeneration can vary.
	rng := rand.New(rand.NewSource(req.Seed))
	sort.SliceStable(scoredList, func(a, b int) bool {
		da, db := scoredList[a].score, scoredList[b].score
		if math.Abs(da-db) < 1e-9 {
			return rng.Float64() < 0.5
		}
		return da > db
	})

	// 4) Select up to MaxSongs with a soft per-artist cap for variety.
	perArtist := map[string]int{}
	artistCap := perArtistCap(req.MaxSongs)
	var picked []library.Song
	for _, sc := range scoredList {
		if len(picked) >= req.MaxSongs {
			break
		}
		ak := strings.ToLower(strings.TrimSpace(sc.s.Artist))
		if ak != "" && perArtist[ak] >= artistCap {
			continue
		}
		perArtist[ak]++
		picked = append(picked, sc.s)
	}
	// If the cap left us short of the minimum, backfill ignoring the cap.
	if len(picked) < req.MinSongs && len(picked) < len(scoredList) {
		have := make(map[string]bool, len(picked))
		for _, p := range picked {
			have[p.Path] = true
		}
		for _, sc := range scoredList {
			if len(picked) >= req.MaxSongs {
				break
			}
			if !have[sc.s.Path] {
				picked = append(picked, sc.s)
				have[sc.s.Path] = true
			}
		}
	}

	if len(picked) < req.MinSongs {
		notes = append(notes, "Only "+strconv.Itoa(len(picked))+" song(s) matched your filters (asked for at least "+strconv.Itoa(req.MinSongs)+"). Try loosening filters or era.")
	}

	return Result{
		Songs:      picked,
		Target:     target,
		Candidates: len(cands),
		Requested:  req.MaxSongs,
		Notes:      notes,
	}
}

// buildTarget blends mood + activity presets and the explicit energy input into
// a target vector and per-dimension weight map.
func buildTarget(req Request) (target, weights map[string]float64) {
	sum := map[string]float64{}
	wsum := map[string]float64{}

	add := func(p preset, w float64) {
		for dim, v := range p {
			sum[dim] += v * w
			wsum[dim] += w
		}
	}
	if p, ok := Moods[req.Mood]; ok {
		add(p, 1.0)
	}
	if p, ok := Activities[req.Activity]; ok {
		add(p, 1.0)
	}
	// Explicit energy slider dominates the energy dimension.
	if req.Energy != nil {
		sum[dEnergy] += clamp01(*req.Energy) * 2.0
		wsum[dEnergy] += 2.0
	}

	target = map[string]float64{}
	weights = map[string]float64{}
	for dim, s := range sum {
		w := wsum[dim]
		if w <= 0 {
			continue
		}
		target[dim] = s / w
		weights[dim] = w
	}
	return target, weights
}

// similarity is the weighted closeness of a song to the target over the active
// dimensions, in 0..1 (1 = identical). Returns 0.5 (neutral) if no dimensions.
func similarity(s library.Song, target, weights map[string]float64) float64 {
	if len(target) == 0 {
		return 0.5
	}
	var acc, wtot float64
	for dim, t := range target {
		w := weights[dim]
		acc += w * (1 - math.Abs(songDim(s, dim)-t))
		wtot += w
	}
	if wtot == 0 {
		return 0.5
	}
	return acc / wtot
}

// songDim returns a song's value for a target dimension, normalized to 0..1.
func songDim(s library.Song, dim string) float64 {
	switch dim {
	case dEnergy:
		return clamp01(s.Energy)
	case dValence:
		return clamp01(s.Valence)
	case dDance:
		return clamp01(s.Danceability)
	case dAcoustic:
		return clamp01(s.Acousticness)
	case dInstr:
		return clamp01(s.Instrumentalness)
	case dSpeech:
		return clamp01(s.Speechiness)
	case dTempo:
		return tempoNorm(s.Tempo)
	}
	return 0.5
}

// tempoNorm maps BPM onto 0..1 across a 60..180 BPM window.
func tempoNorm(bpm float64) float64 {
	return clamp01((bpm - 60) / (180 - 60))
}

// eraCloseness is 1 inside the era window and decays one unit per decade out.
func eraCloseness(year, lo, hi int) float64 {
	if year == 0 {
		return 0.3 // unknown year: mild penalty
	}
	if year >= lo && year <= hi {
		return 1
	}
	var dist int
	if year < lo {
		dist = lo - year
	} else {
		dist = year - hi
	}
	return clamp01(1 - float64(dist)/10.0)
}

func perArtistCap(maxSongs int) int {
	c := maxSongs/4 + 1
	if c < 2 {
		c = 2
	}
	return c
}

func normalize(req Request) Request {
	req.Mood = strings.ToLower(strings.TrimSpace(req.Mood))
	req.Activity = strings.ToLower(strings.TrimSpace(req.Activity))
	req.Era = strings.ToLower(strings.TrimSpace(req.Era))
	req.Discovery = clamp01(req.Discovery)
	if req.MaxSongs <= 0 {
		req.MaxSongs = 25
	}
	if req.MaxSongs > 500 {
		req.MaxSongs = 500
	}
	if req.MinSongs < 0 {
		req.MinSongs = 0
	}
	if req.MinSongs > req.MaxSongs {
		req.MinSongs = req.MaxSongs
	}
	return req
}

func toLowerSet(xs []string) map[string]bool {
	if len(xs) == 0 {
		return nil
	}
	m := make(map[string]bool, len(xs))
	for _, x := range xs {
		x = strings.ToLower(strings.TrimSpace(x))
		if x != "" {
			m[x] = true
		}
	}
	return m
}

// matchAny reports whether field matches any filter value (case-insensitive
// substring either direction, so "hip hop" matches "hip-hop/rap" loosely).
func matchAny(field string, set map[string]bool) bool {
	f := strings.ToLower(strings.TrimSpace(field))
	if f == "" {
		return false
	}
	for v := range set {
		if f == v || strings.Contains(f, v) || strings.Contains(v, f) {
			return true
		}
	}
	return false
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
