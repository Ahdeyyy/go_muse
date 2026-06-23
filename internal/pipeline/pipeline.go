// Package pipeline orchestrates the scan -> decode -> analyze -> score -> store
// flow across a worker pool, skipping files already analyzed and unchanged.
package pipeline

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Ahdeyyy/go_muse/internal/audio"
	"github.com/Ahdeyyy/go_muse/internal/decode"
	"github.com/Ahdeyyy/go_muse/internal/meta"
	"github.com/Ahdeyyy/go_muse/internal/model"
	"github.com/Ahdeyyy/go_muse/internal/score"
	"github.com/Ahdeyyy/go_muse/internal/store"
)

// Config controls a run.
type Config struct {
	Workers   int
	Decode    decode.Options
	Coeffs    score.Coefficients
	Force     bool // ignore cache, re-analyze everything
	OnProgress func(done, total int, t model.Track)
}

// Stats summarizes a run.
type Stats struct {
	Total     int
	Analyzed  int
	Skipped   int
	Failed    int
	Elapsed   time.Duration
}

// Run analyzes the given files, persisting results to st.
func Run(ctx context.Context, st *store.Store, files []model.FileInfo, cfg Config) (Stats, error) {
	start := time.Now()
	stats := Stats{Total: len(files)}

	// 1) Filter out unchanged files unless forced.
	todo := files[:0:0]
	for _, fi := range files {
		if !cfg.Force {
			fresh, err := st.IsFresh(fi)
			if err != nil {
				return stats, err
			}
			if fresh {
				stats.Skipped++
				continue
			}
		}
		todo = append(todo, fi)
	}

	if len(todo) == 0 {
		stats.Elapsed = time.Since(start)
		return stats, nil
	}

	jobs := make(chan model.FileInfo)
	results := make(chan model.Track)

	// 2) Worker pool: decode + analyze + score (CPU-bound, parallel).
	var wg sync.WaitGroup
	for i := 0; i < cfg.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for fi := range jobs {
				results <- analyzeOne(ctx, fi, cfg)
			}
		}()
	}

	// 3) Feed jobs.
	go func() {
		for _, fi := range todo {
			select {
			case <-ctx.Done():
				close(jobs)
				return
			case jobs <- fi:
			}
		}
		close(jobs)
	}()

	// 4) Close results once all workers finish.
	go func() {
		wg.Wait()
		close(results)
	}()

	// 5) Single writer drains results -> DB (SQLite is single-writer).
	var done int32
	for t := range results {
		if t.Error != "" {
			stats.Failed++
		} else {
			stats.Analyzed++
		}
		if err := st.Upsert(t); err != nil {
			return stats, fmt.Errorf("upsert %s: %w", t.File.Path, err)
		}
		d := int(atomic.AddInt32(&done, 1))
		if cfg.OnProgress != nil {
			cfg.OnProgress(d, len(todo), t)
		}
	}

	stats.Elapsed = time.Since(start)
	return stats, nil
}

// analyzeOne runs the full per-file analysis, capturing any error in the Track.
func analyzeOne(ctx context.Context, fi model.FileInfo, cfg Config) model.Track {
	t := model.Track{File: fi, AnalyzedAt: time.Now()}

	// Metadata (non-fatal).
	if m, err := meta.Read(fi.Path); err == nil {
		t.Meta = m
	}

	sig, err := decode.Decode(ctx, fi.Path, cfg.Decode)
	if err != nil {
		t.Error = "decode: " + err.Error()
		return t
	}
	t.Low = audio.Analyze(sig)
	t.Spot = score.Compute(t.Low, cfg.Coeffs)
	return t
}
