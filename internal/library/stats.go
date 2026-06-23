package library

import (
	"sort"
	"strings"
	"time"

	"github.com/Ahdeyyy/go_muse/internal/pxpl"
)

// Stats is the dashboard payload: headline counters plus the series that drive
// the FlareCharts visualizations on the frontend.
type Stats struct {
	TotalTracks   int `json:"totalTracks"`
	MatchedTracks int `json:"matchedTracks"`
	TotalPlays    int `json:"totalPlays"`
	ListeningHours float64 `json:"listeningHours"`
	Favorites     int `json:"favorites"`
	DistinctArtists int `json:"distinctArtists"`
	DistinctGenres  int `json:"distinctGenres"`

	Genres        []Labeled        `json:"genres"`         // top genres by track count
	TopArtists    []Labeled        `json:"topArtists"`     // by play count
	TopSongs      []SongPlays      `json:"topSongs"`       // by play count
	FeatureAvg    map[string]float64 `json:"featureAvg"`   // mean perceptual scores
	PlaysByMonth  []TimeCount      `json:"playsByMonth"`   // listening over time
	PlaysByHour   []Labeled        `json:"playsByHour"`    // hour-of-day histogram
	PlayCountHist []Labeled        `json:"playCountHist"`  // discovery/familiarity buckets
	EnergyDist    []Labeled        `json:"energyDist"`     // energy histogram (10 bins)
	ValenceDist   []Labeled        `json:"valenceDist"`    // valence histogram (10 bins)
}

// Labeled is a generic label/value pair for bar & donut charts.
type Labeled struct {
	Label string  `json:"label"`
	Value float64 `json:"value"`
}

// TimeCount is a point on a time series (ISO month + count).
type TimeCount struct {
	Date  string  `json:"date"`
	Value float64 `json:"value"`
}

// SongPlays is a most-played row.
type SongPlays struct {
	Title  string `json:"title"`
	Artist string `json:"artist"`
	Plays  int    `json:"plays"`
}

// ComputeStats aggregates the library (and backup, if present) for the dashboard.
func (lib *Library) ComputeStats(topN int) Stats {
	if topN <= 0 {
		topN = 12
	}
	st := Stats{
		TotalTracks: len(lib.Songs),
		FeatureAvg:  map[string]float64{},
	}

	genreCount := map[string]int{}
	artistSet := map[string]bool{}
	artistPlays := map[string]int{}
	var energySum, valSum, danceSum, acoustSum, instrSum float64
	var nFeat int
	energyBins := make([]int, 10)
	valenceBins := make([]int, 10)
	var topByPlays []SongPlays

	for _, s := range lib.Songs {
		g := strings.TrimSpace(s.Genre)
		if g == "" {
			g = "(unknown)"
		}
		genreCount[g]++
		if a := strings.TrimSpace(s.Artist); a != "" {
			artistSet[strings.ToLower(a)] = true
		}
		energySum += s.Energy
		valSum += s.Valence
		danceSum += s.Danceability
		acoustSum += s.Acousticness
		instrSum += s.Instrumentalness
		nFeat++
		energyBins[binOf(s.Energy)]++
		valenceBins[binOf(s.Valence)]++

		if s.Matched {
			st.MatchedTracks++
			st.TotalPlays += s.PlayCount
			if s.Favorite {
				st.Favorites++
			}
			if a := strings.TrimSpace(s.Artist); a != "" && s.PlayCount > 0 {
				artistPlays[a] += s.PlayCount
			}
			if s.PlayCount > 0 {
				topByPlays = append(topByPlays, SongPlays{s.Title, s.Artist, s.PlayCount})
			}
		}
	}

	st.DistinctArtists = len(artistSet)
	st.DistinctGenres = len(genreCount)
	if nFeat > 0 {
		st.FeatureAvg["energy"] = energySum / float64(nFeat)
		st.FeatureAvg["valence"] = valSum / float64(nFeat)
		st.FeatureAvg["danceability"] = danceSum / float64(nFeat)
		st.FeatureAvg["acousticness"] = acoustSum / float64(nFeat)
		st.FeatureAvg["instrumentalness"] = instrSum / float64(nFeat)
	}
	st.Genres = topLabeled(genreCount, topN)
	st.TopArtists = topLabeled(artistPlays, topN)
	st.EnergyDist = binsToLabeled(energyBins)
	st.ValenceDist = binsToLabeled(valenceBins)

	sort.Slice(topByPlays, func(a, b int) bool { return topByPlays[a].Plays > topByPlays[b].Plays })
	if len(topByPlays) > topN {
		topByPlays = topByPlays[:topN]
	}
	st.TopSongs = topByPlays

	// Backup-derived series (independent of the DB join).
	if lib.Backup != nil {
		st.ListeningHours, st.PlaysByMonth, st.PlaysByHour = playSeries(lib.Backup.Plays)
		st.PlayCountHist = playCountHistogram(lib.Backup.Engagement)
	}
	return st
}

func binOf(v float64) int {
	if v < 0 {
		v = 0
	}
	i := int(v * 10)
	if i > 9 {
		i = 9
	}
	return i
}

func binsToLabeled(bins []int) []Labeled {
	out := make([]Labeled, len(bins))
	for i, n := range bins {
		lo := i * 10
		out[i] = Labeled{Label: itoa(lo) + "-" + itoa(lo+10) + "%", Value: float64(n)}
	}
	return out
}

func topLabeled(m map[string]int, n int) []Labeled {
	out := make([]Labeled, 0, len(m))
	for k, v := range m {
		out = append(out, Labeled{Label: k, Value: float64(v)})
	}
	sort.Slice(out, func(a, b int) bool {
		if out[a].Value != out[b].Value {
			return out[a].Value > out[b].Value
		}
		return out[a].Label < out[b].Label
	})
	if len(out) > n {
		out = out[:n]
	}
	return out
}

// playSeries computes total listening hours, a per-month play count series, and
// a 24-bucket hour-of-day histogram from the playback history.
func playSeries(plays []pxpl.PlayEvent) (hours float64, byMonth []TimeCount, byHour []Labeled) {
	monthCount := map[string]int{}
	hourBuckets := make([]int, 24)
	var totalMs int64
	for _, p := range plays {
		totalMs += p.DurationMs
		ts := p.StartTimestamp
		if ts <= 0 {
			ts = p.Timestamp
		}
		if ts <= 0 {
			continue
		}
		t := time.UnixMilli(ts).UTC()
		monthCount[t.Format("2006-01")]++
		hourBuckets[t.Hour()]++
	}
	hours = float64(totalMs) / 3_600_000.0

	months := make([]string, 0, len(monthCount))
	for k := range monthCount {
		months = append(months, k)
	}
	sort.Strings(months)
	for _, m := range months {
		byMonth = append(byMonth, TimeCount{Date: m + "-01", Value: float64(monthCount[m])})
	}
	for h := 0; h < 24; h++ {
		byHour = append(byHour, Labeled{Label: itoa(h), Value: float64(hourBuckets[h])})
	}
	return hours, byMonth, byHour
}

// playCountHistogram buckets songs by lifetime play count — the familiarity /
// discovery distribution.
func playCountHistogram(eng []pxpl.EngagementStat) []Labeled {
	type bucket struct {
		label string
		lo    int
		hi    int // inclusive; -1 = open
	}
	buckets := []bucket{
		{"0", 0, 0}, {"1-2", 1, 2}, {"3-5", 3, 5}, {"6-10", 6, 10},
		{"11-20", 11, 20}, {"21-50", 21, 50}, {"50+", 51, -1},
	}
	counts := make([]int, len(buckets))
	for _, e := range eng {
		pc := e.PlayCount
		for i, b := range buckets {
			if pc >= b.lo && (b.hi == -1 || pc <= b.hi) {
				counts[i]++
				break
			}
		}
	}
	out := make([]Labeled, len(buckets))
	for i, b := range buckets {
		out[i] = Labeled{Label: b.label, Value: float64(counts[i])}
	}
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
