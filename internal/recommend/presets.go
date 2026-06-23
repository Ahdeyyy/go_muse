package recommend

// A preset is a partial target: it pins only the perceptual dimensions it
// cares about (each 0..1; tempo is normalized to 0..1 via tempoNorm). Mood and
// activity presets are blended together with the explicit Energy/Discovery
// inputs to form the final target vector the recommender scores against.
type preset map[string]float64

// Dimension keys used throughout scoring.
const (
	dEnergy   = "energy"
	dValence  = "valence"
	dDance    = "danceability"
	dAcoustic = "acousticness"
	dInstr    = "instrumentalness"
	dTempo    = "tempoNorm"
	dSpeech   = "speechiness"
)

// Moods map an emotional target onto valence/energy (+ supporting dims).
var Moods = map[string]preset{
	"happy":      {dValence: 0.85, dEnergy: 0.70, dDance: 0.65},
	"sad":        {dValence: 0.15, dEnergy: 0.30, dAcoustic: 0.55},
	"chill":      {dValence: 0.55, dEnergy: 0.30, dAcoustic: 0.55},
	"energetic":  {dValence: 0.75, dEnergy: 0.90, dDance: 0.75},
	"angry":      {dValence: 0.20, dEnergy: 0.90, dDance: 0.45},
	"romantic":   {dValence: 0.65, dEnergy: 0.40, dAcoustic: 0.45},
	"focus":      {dValence: 0.50, dEnergy: 0.45, dInstr: 0.70, dSpeech: 0.15},
	"melancholy": {dValence: 0.25, dEnergy: 0.35, dAcoustic: 0.50},
	"uplifting":  {dValence: 0.80, dEnergy: 0.65, dDance: 0.60},
	"dark":       {dValence: 0.20, dEnergy: 0.55, dAcoustic: 0.30},
}

// Activities map a context onto energy/danceability/tempo (+ supporting dims).
var Activities = map[string]preset{
	"workout":  {dEnergy: 0.90, dDance: 0.75, dTempo: 0.80, dValence: 0.65},
	"running":  {dEnergy: 0.85, dTempo: 0.90, dDance: 0.70},
	"study":    {dEnergy: 0.40, dInstr: 0.75, dSpeech: 0.12, dAcoustic: 0.55},
	"focus":    {dEnergy: 0.45, dInstr: 0.70, dSpeech: 0.15},
	"party":    {dEnergy: 0.85, dDance: 0.85, dValence: 0.80, dTempo: 0.70},
	"sleep":    {dEnergy: 0.10, dAcoustic: 0.80, dInstr: 0.55, dTempo: 0.25},
	"relax":    {dEnergy: 0.30, dAcoustic: 0.65, dValence: 0.55},
	"commute":  {dEnergy: 0.60, dValence: 0.60, dDance: 0.55},
	"driving":  {dEnergy: 0.65, dValence: 0.60, dTempo: 0.60},
	"gaming":   {dEnergy: 0.75, dInstr: 0.45, dTempo: 0.65},
	"cooking":  {dEnergy: 0.60, dValence: 0.70, dDance: 0.60},
	"reading":  {dEnergy: 0.30, dInstr: 0.65, dAcoustic: 0.60, dSpeech: 0.15},
	"cleaning": {dEnergy: 0.80, dDance: 0.80, dValence: 0.75},
}

// eraRange returns the inclusive [min,max] year window for an era key, plus a
// boolean for whether the key was recognized. Keys: "2020s","2010s","2000s",
// "1990s"/"90s", "1980s"/"80s", "1970s"/"70s", "classic" (<1970), "any"/"".
func eraRange(era string) (int, int, bool) {
	switch era {
	case "", "any":
		return 0, 0, false
	case "2020s":
		return 2020, 2100, true
	case "2010s":
		return 2010, 2019, true
	case "2000s":
		return 2000, 2009, true
	case "1990s", "90s":
		return 1990, 1999, true
	case "1980s", "80s":
		return 1980, 1989, true
	case "1970s", "70s":
		return 1970, 1979, true
	case "classic", "oldies":
		return 1900, 1969, true
	}
	return 0, 0, false
}

// MoodKeys / ActivityKeys / EraKeys expose the available options to the API so
// the frontend can render them without hardcoding.
func MoodKeys() []string    { return keys(Moods) }
func ActivityKeys() []string { return keys(Activities) }

func EraKeys() []string {
	return []string{"any", "2020s", "2010s", "2000s", "1990s", "1980s", "1970s", "classic"}
}

func keys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
