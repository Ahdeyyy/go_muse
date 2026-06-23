# go_muse

Analyze your local music library: read tag **metadata**, extract **waveform/DSP
features** from each track, and derive **Spotify-style perceptual scores**
(danceability, energy, valence, …) using tunable heuristic formulas. Results are
stored in **SQLite** and exported to **CSV**, with a console report of top genres
and library-wide averages. Built to run under **Termux on Android**.

> The perceptual scores are *approximations*, not Spotify's proprietary model.
> Each is tagged with a reliability level in the report. `tempo`, `loudness` and
> `key` are real measurements; `liveness` / `instrumentalness` are rough proxies
> (no vocal/crowd model).

## How it works

```
scan → (ffmpeg decode → mono PCM) → DSP analysis → heuristic scoring → SQLite + CSV
```

- **Decoding:** shells out to `ffmpeg`, so every format (mp3/m4a/aac/flac/ogg/
  opus/wav) goes through one code path. By default only a **90s excerpt from 30s
  in** is decoded per track (fast for large libraries; `-len 0` = whole track).
- **Caching:** each track is keyed by `(path, size, mod_time)`. Re-runs skip
  unchanged files, so analysis is incremental.
- **Scoring "weights":** there is no training step. The heuristic coefficients
  live in `coefficients.json`, loaded each run (written with defaults if absent).
  Edit that file and re-run to recalibrate — this is the persistent, reloadable
  "model" for this approach.

## Setup (Termux)

```bash
pkg update && pkg install golang ffmpeg git
termux-setup-storage            # grant access to /storage/emulated/0
git clone <this repo> && cd go_muse
CGO_ENABLED=0 go build ./cmd/gomuse     # pure-Go deps, no C toolchain needed
```

## Usage

```bash
# Analyze the phone's Music folder (path may vary by device/ROM)
./gomuse -dir /storage/emulated/0/Music

# Whole tracks instead of a 90s excerpt, more genres in the report
./gomuse -dir ~/storage/music -len 0 -top 30

# Re-run later: only new/changed files are analyzed (cache hit on the rest)
./gomuse -dir /storage/emulated/0/Music

# Just print the report / re-export CSV from the existing DB
./gomuse -report
```

### Flags

| Flag | Default | Meaning |
|------|---------|---------|
| `-dir` | `.` | music directory to scan |
| `-db` | `gomuse.db` | SQLite database path |
| `-csv` | `gomuse.csv` | CSV export path (`""` to skip) |
| `-coeffs` | `coefficients.json` | scoring coefficients (created if missing) |
| `-workers` | NumCPU | concurrent analysis workers |
| `-skip` | `30` | seconds skipped from each track start |
| `-len` | `90` | seconds analyzed per track (`0` = whole track) |
| `-sr` | `22050` | analysis sample rate (Hz) |
| `-ffmpeg` | `ffmpeg` | path to ffmpeg binary |
| `-timeout` | `60` | per-file decode timeout (s) |
| `-force` | `false` | re-analyze even unchanged files |
| `-report` | `false` | skip analysis, just print report |
| `-top` | `20` | number of top genres to show |

## Output

- **`gomuse.db`** — full per-track table (metadata + DSP + scores). Query it:
  ```bash
  sqlite3 gomuse.db "SELECT artist,title,tempo_bpm,energy,danceability \
    FROM tracks ORDER BY danceability DESC LIMIT 20;"
  ```
- **`gomuse.csv`** — one row per track, for spreadsheets / pandas.
- **Console report** — top genres + average perceptual scores with reliability.

## What's measured

**Metadata:** title, artist, album, album-artist, genre, year, track number.

**DSP (objective):** duration, RMS loudness (dB), peak, dynamic range, ZCR,
spectral centroid/rolloff/bandwidth/flatness, onset rate, tempo (BPM), beat
strength, musical key + mode + confidence, harmonic ratio, 13 MFCC means.

**Perceptual (heuristic):** danceability, energy, valence, acousticness,
instrumentalness, liveness, speechiness (+ tempo, loudness passthrough).

## Project layout

```
cmd/gomuse        CLI entrypoint
internal/model    shared structs
internal/scan     directory walk / format filter
internal/meta     tag metadata (dhowden/tag)
internal/decode   ffmpeg → mono float32 PCM
internal/audio    DSP feature extraction (gonum FFT)
internal/score    heuristic scoring + coefficients.json
internal/store    SQLite (modernc) + CSV + reports
internal/pipeline worker-pool orchestration + caching
```
