// Command gomuse scans a music library, analyzes each track's metadata and
// waveform, derives Spotify-style perceptual scores, and stores results in
// SQLite + CSV. Designed to run under Termux on Android (needs ffmpeg).
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/Ahdeyyy/go_muse/internal/decode"
	"github.com/Ahdeyyy/go_muse/internal/model"
	"github.com/Ahdeyyy/go_muse/internal/pipeline"
	"github.com/Ahdeyyy/go_muse/internal/scan"
	"github.com/Ahdeyyy/go_muse/internal/score"
	"github.com/Ahdeyyy/go_muse/internal/store"
)

func main() {
	var (
		dir        = flag.String("dir", ".", "music directory to scan (e.g. /storage/emulated/0/Music)")
		dbPath     = flag.String("db", "gomuse.db", "SQLite database path")
		csvPath    = flag.String("csv", "gomuse.csv", "CSV export path (empty to skip)")
		coeffPath  = flag.String("coeffs", "coefficients.json", "scoring coefficients file (created if missing)")
		workers    = flag.Int("workers", runtime.NumCPU(), "concurrent analysis workers")
		skipSec    = flag.Float64("skip", 30, "seconds to skip from each track's start")
		lenSec     = flag.Float64("len", 90, "seconds to analyze per track (0 = whole track)")
		sampleRate = flag.Int("sr", 22050, "analysis sample rate (Hz)")
		ffmpeg     = flag.String("ffmpeg", "ffmpeg", "path to ffmpeg binary")
		timeout    = flag.Int("timeout", 60, "per-file decode timeout (seconds)")
		force      = flag.Bool("force", false, "re-analyze even unchanged files")
		reportOnly = flag.Bool("report", false, "skip analysis; just print report from existing DB")
		topGenres  = flag.Int("top", 20, "number of top genres to show")
	)
	flag.Parse()

	if err := run(*dir, *dbPath, *csvPath, *coeffPath, *workers, *skipSec, *lenSec,
		*sampleRate, *ffmpeg, *timeout, *force, *reportOnly, *topGenres); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(dir, dbPath, csvPath, coeffPath string, workers int, skipSec, lenSec float64,
	sampleRate int, ffmpeg string, timeout int, force, reportOnly bool, topGenres int) error {

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	coeffs, err := score.Load(coeffPath)
	if err != nil {
		return fmt.Errorf("coefficients: %w", err)
	}

	st, err := store.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer st.Close()

	if !reportOnly {
		fmt.Printf("Scanning %s ...\n", dir)
		files, err := scan.Walk(dir)
		if err != nil {
			return fmt.Errorf("scan: %w", err)
		}
		fmt.Printf("Found %d audio files.\n", len(files))
		if len(files) == 0 {
			fmt.Println("Nothing to analyze. Check -dir (Termux: run termux-setup-storage first).")
		} else {
			if workers < 1 {
				workers = 1
			}
			cfg := pipeline.Config{
				Workers: workers,
				Force:   force,
				Coeffs:  coeffs,
				Decode: decode.Options{
					SampleRate: sampleRate,
					SkipSec:    skipSec,
					LenSec:     lenSec,
					Timeout:    time.Duration(timeout) * time.Second,
					FFmpegPath: ffmpeg,
				},
				OnProgress: progressFn(),
			}
			stats, err := pipeline.Run(ctx, st, files, cfg)
			if err != nil {
				return err
			}
			fmt.Printf("\nDone in %s — analyzed %d, skipped %d (cached), failed %d.\n",
				stats.Elapsed.Round(time.Second), stats.Analyzed, stats.Skipped, stats.Failed)
		}
	}

	if csvPath != "" {
		if err := st.ExportCSV(csvPath); err != nil {
			return fmt.Errorf("csv export: %w", err)
		}
		fmt.Printf("Exported CSV -> %s\n", csvPath)
	}

	return printReport(st, topGenres)
}

// progressFn returns a throttled progress printer.
func progressFn() func(done, total int, t model.Track) {
	last := time.Now()
	return func(done, total int, t model.Track) {
		if done == total || time.Since(last) > 500*time.Millisecond {
			fmt.Printf("\r  analyzing %d/%d ...", done, total)
			last = time.Now()
		}
	}
}

func printReport(st *store.Store, top int) error {
	n, err := st.Count()
	if err != nil {
		return err
	}
	fmt.Printf("\n===== Library Report (%d analyzed tracks) =====\n\n", n)
	if n == 0 {
		return nil
	}

	genres, err := st.GenreHistogram()
	if err != nil {
		return err
	}
	fmt.Printf("Top genres:\n")
	for i, g := range genres {
		if i >= top {
			break
		}
		pct := 100 * float64(g.N) / float64(n)
		fmt.Printf("  %-28s %5d  (%4.1f%%)\n", truncate(g.Genre, 28), g.N, pct)
	}

	a, err := st.Averages()
	if err != nil {
		return err
	}
	fmt.Printf("\nAverage perceptual scores (0..1 unless noted):\n")
	fmt.Printf("  danceability     %.3f   [%s]\n", a.Danceability, score.Reliability["danceability"])
	fmt.Printf("  energy           %.3f   [%s]\n", a.Energy, score.Reliability["energy"])
	fmt.Printf("  valence          %.3f   [%s]\n", a.Valence, score.Reliability["valence"])
	fmt.Printf("  acousticness     %.3f   [%s]\n", a.Acousticness, score.Reliability["acousticness"])
	fmt.Printf("  instrumentalness %.3f   [%s]\n", a.Instrumentalness, score.Reliability["instrumentalness"])
	fmt.Printf("  liveness         %.3f   [%s]\n", a.Liveness, score.Reliability["liveness"])
	fmt.Printf("  speechiness      %.3f   [%s]\n", a.Speechiness, score.Reliability["speechiness"])
	fmt.Printf("  tempo            %.1f BPM [%s]\n", a.Tempo, score.Reliability["tempo"])
	fmt.Printf("  loudness         %.1f dB  [%s]\n", a.Loudness, score.Reliability["loudness"])
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
