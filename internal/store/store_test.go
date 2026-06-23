package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Ahdeyyy/go_muse/internal/model"
)

func sampleTrack(path, genre string) model.Track {
	return model.Track{
		File: model.FileInfo{Path: path, Name: filepath.Base(path), Ext: ".mp3",
			Size: 123, ModTime: time.Unix(1000, 0)},
		Meta: model.Metadata{Artist: "A", Album: "B", Title: "T", Genre: genre, Year: 2020},
		Low: model.LowLevel{DurationSec: 200, SampleRate: 22050, RMSDb: -12,
			TempoBPM: 120, Key: 0, Mode: 1, MFCC: []float64{1, 2, 3}},
		Spot: model.Spotify{Danceability: 0.5, Energy: 0.6, Valence: 0.4,
			Tempo: 120, Loudness: -12},
		AnalyzedAt: time.Now(),
	}
}

func TestStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	if err := st.Upsert(sampleTrack("/m/1.mp3", "Afrobeats")); err != nil {
		t.Fatal(err)
	}
	if err := st.Upsert(sampleTrack("/m/2.mp3", "Afrobeats")); err != nil {
		t.Fatal(err)
	}
	if err := st.Upsert(sampleTrack("/m/3.mp3", "Hip-Hop")); err != nil {
		t.Fatal(err)
	}

	n, err := st.Count()
	if err != nil || n != 3 {
		t.Fatalf("count = %d, err = %v; want 3", n, err)
	}

	// Freshness cache.
	fi := model.FileInfo{Path: "/m/1.mp3", Size: 123, ModTime: time.Unix(1000, 0)}
	fresh, err := st.IsFresh(fi)
	if err != nil || !fresh {
		t.Fatalf("IsFresh = %v, err = %v; want true", fresh, err)
	}
	stale := model.FileInfo{Path: "/m/1.mp3", Size: 999, ModTime: time.Unix(1000, 0)}
	if fresh, _ := st.IsFresh(stale); fresh {
		t.Fatal("changed size should not be fresh")
	}

	genres, err := st.GenreHistogram()
	if err != nil {
		t.Fatal(err)
	}
	if len(genres) == 0 || genres[0].Genre != "Afrobeats" || genres[0].N != 2 {
		t.Fatalf("histogram top = %+v, want Afrobeats=2", genres)
	}

	avg, err := st.Averages()
	if err != nil {
		t.Fatal(err)
	}
	if avg.Tempo != 120 {
		t.Errorf("avg tempo = %.1f, want 120", avg.Tempo)
	}

	csv := filepath.Join(dir, "out.csv")
	if err := st.ExportCSV(csv); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(csv)
	if err != nil || len(b) == 0 {
		t.Fatalf("csv empty, err = %v", err)
	}
}
