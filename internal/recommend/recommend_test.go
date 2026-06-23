package recommend

import (
	"testing"

	"github.com/Ahdeyyy/go_muse/internal/library"
)

func sample() []library.Song {
	return []library.Song{
		{Path: "/a.mp3", Title: "Calm", Artist: "X", Genre: "Ambient", Year: 2021,
			Energy: 0.1, Valence: 0.5, Acousticness: 0.8, Instrumentalness: 0.7, Tempo: 70,
			Matched: true, Familiarity: 0.9, DurationSec: 200},
		{Path: "/b.mp3", Title: "Banger", Artist: "Y", Genre: "EDM", Year: 2022,
			Energy: 0.95, Valence: 0.8, Danceability: 0.9, Tempo: 128,
			Matched: true, Familiarity: 0.1, DurationSec: 180},
		{Path: "/c.mp3", Title: "Sad Song", Artist: "Z", Genre: "Indie", Year: 2010,
			Energy: 0.3, Valence: 0.15, Acousticness: 0.5, Tempo: 90,
			Matched: true, Familiarity: 0.5, DurationSec: 240},
	}
}

func TestWorkoutPicksHighEnergy(t *testing.T) {
	res := Generate(sample(), Request{Activity: "workout", Discovery: 0.5, MinSongs: 1, MaxSongs: 1})
	if len(res.Songs) != 1 {
		t.Fatalf("want 1 song, got %d", len(res.Songs))
	}
	if res.Songs[0].Path != "/b.mp3" {
		t.Fatalf("workout should pick the banger, got %s", res.Songs[0].Title)
	}
}

func TestEnergyOverride(t *testing.T) {
	lo := 0.1
	res := Generate(sample(), Request{Energy: &lo, Discovery: 0.5, MinSongs: 1, MaxSongs: 1})
	if res.Songs[0].Path != "/a.mp3" {
		t.Fatalf("low energy should pick Calm, got %s", res.Songs[0].Title)
	}
}

func TestDiscoveryPrefersUnfamiliar(t *testing.T) {
	// High discovery => prefer low familiarity (Banger, fam 0.1).
	res := Generate(sample(), Request{Discovery: 1.0, MinSongs: 1, MaxSongs: 1})
	if res.Songs[0].Path != "/b.mp3" {
		t.Fatalf("discovery should surface unfamiliar Banger, got %s", res.Songs[0].Title)
	}
}

func TestGenreFilter(t *testing.T) {
	res := Generate(sample(), Request{Genres: []string{"indie"}, MaxSongs: 10})
	if len(res.Songs) != 1 || res.Songs[0].Path != "/c.mp3" {
		t.Fatalf("genre filter failed: %+v", res.Songs)
	}
}

func TestFavoritesOnlyEmpty(t *testing.T) {
	res := Generate(sample(), Request{FavoritesOnly: true, MinSongs: 1, MaxSongs: 5})
	if len(res.Songs) != 0 {
		t.Fatalf("no favorites in sample, want 0 got %d", len(res.Songs))
	}
	if len(res.Notes) == 0 {
		t.Fatal("expected a shortfall note")
	}
}

func TestMaxSongsBound(t *testing.T) {
	res := Generate(sample(), Request{MaxSongs: 2})
	if len(res.Songs) != 2 {
		t.Fatalf("want 2, got %d", len(res.Songs))
	}
}
