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

## Web dashboard & playlist generator (`cmd/web`)

A single self-contained binary that serves a **Svelte 5** dashboard (embedded
via `go:embed`) backed by a small JSON API. It visualizes your library with
[FlareCharts](https://flarecharts.gitbook.io/docs) and generates `.m3u`
playlists from a recommendation engine.

```
analysis (gomuse.db)  ─┐
                       ├─► join on title+artist ─► charts + recommender ─► .m3u
PixelPlayer .pxpl  ────┘   (engagement: plays, favorites, recency)
```

- **Two data sources, joined.** `gomuse.db` supplies audio attributes (energy,
  valence, danceability, genre, year) and the on-disk path each `.m3u` entry
  needs. The uploaded **`.pxpl`** backup supplies *listening* data — how often
  each song was played, favorites, and per-play timestamps. They are matched on
  a normalized title+artist key (the backup's `songId` is opaque, so titles are
  recovered from the lyrics module's `[ti:]/[ar:]` tags).
- **Charts (datapoints chosen for you):** top genres (donut), listening over
  time (area), audio-fingerprint averages, familiarity spread by play count,
  plays by hour of day, energy distribution, top artists & most-played tracks.
- **Playlist inputs:** mood, activity, era, energy, **discovery** (familiar ↔
  new, driven by play count), min/max songs, and filters (genres, artists,
  favorites). The recommender blends mood/activity presets + the energy/era
  inputs into a target vector, scores every track by weighted similarity plus a
  discovery (familiarity) term, and exports the result as extended `.m3u`.

### Build & run

```bash
# 1) build the SPA into internal/webapi/dist (one-time / after UI changes)
npm --prefix web install
npm --prefix web run build

# 2) build & run the embedded binary
go build -o gomuse-web ./cmd/web
./gomuse-web -db gomuse.db          # opens http://127.0.0.1:8765
```

| Flag | Default | Meaning |
|------|---------|---------|
| `-db` | `gomuse.db` | gomuse analysis database (audio attributes + paths) |
| `-addr` | `127.0.0.1:8765` | listen address |
| `-open` | `true` | open the dashboard in a browser on start |

Then click **Upload .pxpl** to layer in your listening history. For local
development, run the Go server and `npm --prefix web run dev` (Vite proxies
`/api` to `:8765`).

> `tools/seedtest` builds a synthetic `gomuse.db` from a `.pxpl` (real
> titles/artists, fabricated features) so you can try the dashboard without
> running the full audio analysis: `go run ./tools/seedtest -in backup.pxpl`.

## Project layout

```
cmd/gomuse        CLI entrypoint (analyzer)
cmd/web           embedded web dashboard + JSON API
web/              Svelte 5 + Vite SPA (FlareCharts)
internal/model    shared structs
internal/scan     directory walk / format filter
internal/meta     tag metadata (dhowden/tag)
internal/decode   ffmpeg → mono float32 PCM
internal/audio    DSP feature extraction (gonum FFT)
internal/score    heuristic scoring + coefficients.json
internal/store    SQLite (modernc) + CSV + reports
internal/pipeline worker-pool orchestration + caching
internal/pxpl     PixelPlayer .pxpl backup parser
internal/library  gomuse.db ↔ .pxpl join + dashboard stats
internal/recommend playlist recommendation engine
internal/m3u      extended .m3u writer
internal/webapi   HTTP handlers + embedded SPA
```
