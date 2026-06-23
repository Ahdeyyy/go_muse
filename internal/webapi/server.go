// Package webapi serves the embedded Svelte dashboard and the JSON API that
// backs it: library stats for the charts, .pxpl backup upload, and playlist
// generation + .m3u export.
package webapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Ahdeyyy/go_muse/internal/library"
	"github.com/Ahdeyyy/go_muse/internal/m3u"
	"github.com/Ahdeyyy/go_muse/internal/pxpl"
	"github.com/Ahdeyyy/go_muse/internal/recommend"
)

const maxUpload = 96 << 20 // 96 MiB; lyrics module can be large

// Server holds the loaded library and serves HTTP. The analyzed songs from the
// gomuse DB are immutable; the backup and the joined view are swapped under a
// mutex whenever a new .pxpl is uploaded.
type Server struct {
	dbPath  string
	dbSongs []library.Song // analyzed tracks (may be empty if no DB)

	mu  sync.RWMutex
	lib *library.Library
}

// New builds a server. dbPath may point to a gomuse.db; if it is missing the
// server still starts in backup-only mode (charts light up after upload).
func New(dbPath string) (*Server, error) {
	s := &Server{dbPath: dbPath}
	songs, err := library.LoadDB(dbPath)
	if err != nil {
		// Not fatal: allow running without a DB. Report via /api/state.
		fmt.Printf("warning: could not load %s: %v\n", dbPath, err)
	}
	s.dbSongs = songs
	s.lib = library.Build(cloneSongs(songs), nil)
	return s, nil
}

// Handler returns the root HTTP handler (API + embedded SPA).
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/state", s.handleState)
	mux.HandleFunc("GET /api/stats", s.handleStats)
	mux.HandleFunc("POST /api/backup", s.handleBackup)
	mux.HandleFunc("POST /api/playlist", s.handlePlaylist)
	mux.HandleFunc("POST /api/playlist.m3u", s.handleM3U)

	spa := spaHandler(staticFS())
	mux.Handle("/", spa)
	return logRequests(mux)
}

// ---- state ----

type stateResp struct {
	HasDB        bool     `json:"hasDb"`
	HasBackup    bool     `json:"hasBackup"`
	DBPath       string   `json:"dbPath"`
	TotalTracks  int      `json:"totalTracks"`
	Matched      int      `json:"matched"`
	BackupVer    string   `json:"backupVersion,omitempty"`
	Moods        []string `json:"moods"`
	Activities   []string `json:"activities"`
	Eras         []string `json:"eras"`
	Genres       []string `json:"genres"`
	Artists      []string `json:"artists"`
}

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	lib := s.lib
	s.mu.RUnlock()

	resp := stateResp{
		HasDB:       lib.HasDB,
		HasBackup:   lib.HasPxpl,
		DBPath:      s.dbPath,
		TotalTracks: len(lib.Songs),
		Matched:     lib.MatchedCount(),
		Moods:       recommend.MoodKeys(),
		Activities:  recommend.ActivityKeys(),
		Eras:        recommend.EraKeys(),
		Genres:      distinctGenres(lib.Songs),
		Artists:     distinctArtists(lib.Songs),
	}
	if lib.Backup != nil {
		resp.BackupVer = lib.Backup.AppVersion
	}
	writeJSON(w, resp)
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	lib := s.lib
	s.mu.RUnlock()
	writeJSON(w, lib.ComputeStats(12))
}

// ---- backup upload ----

func (s *Server) handleBackup(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUpload)
	if err := r.ParseMultipartForm(maxUpload); err != nil {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("parse upload: %w", err))
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("missing 'file' field: %w", err))
		return
	}
	defer file.Close()

	raw, err := io.ReadAll(file)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	backup, err := pxpl.Parse(raw)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}

	lib := library.Build(cloneSongs(s.dbSongs), backup)
	s.mu.Lock()
	s.lib = lib
	s.mu.Unlock()

	writeJSON(w, map[string]any{
		"ok":            true,
		"backupVersion": backup.AppVersion,
		"songsWithMeta": len(backup.Meta),
		"engagement":    len(backup.Engagement),
		"plays":         len(backup.Plays),
		"matched":       lib.MatchedCount(),
		"totalTracks":   len(lib.Songs),
	})
}

// ---- playlist generation ----

func (s *Server) handlePlaylist(w http.ResponseWriter, r *http.Request) {
	req, err := decodeRequest(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	s.mu.RLock()
	songs := s.lib.Songs
	s.mu.RUnlock()
	writeJSON(w, recommend.Generate(songs, req))
}

func (s *Server) handleM3U(w http.ResponseWriter, r *http.Request) {
	var body struct {
		recommend.Request
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	s.mu.RLock()
	songs := s.lib.Songs
	s.mu.RUnlock()

	res := recommend.Generate(songs, body.Request)
	name := strings.TrimSpace(body.Name)
	if name == "" {
		name = "gomuse playlist"
	}
	out := m3u.Write(name, res.Songs)

	w.Header().Set("Content-Type", "audio/x-mpegurl; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", safeFilename(name)+".m3u"))
	_, _ = io.WriteString(w, out)
}

func decodeRequest(r *http.Request) (recommend.Request, error) {
	var req recommend.Request
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		return req, fmt.Errorf("decode request: %w", err)
	}
	return req, nil
}

// ---- SPA static serving ----

// spaHandler serves embedded static assets and falls back to index.html for
// unknown paths (client-side routing).
func spaHandler(fsys fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(fsys))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if p == "" {
			p = "index.html"
		}
		if _, err := fs.Stat(fsys, p); err != nil {
			serveIndex(w, fsys) // unknown path: serve the SPA shell
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}

func serveIndex(w http.ResponseWriter, fsys fs.FS) {
	data, err := fs.ReadFile(fsys, "index.html")
	if err != nil {
		http.Error(w, "frontend not built", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

// ---- helpers ----

func cloneSongs(in []library.Song) []library.Song {
	out := make([]library.Song, len(in))
	copy(out, in)
	return out
}

func distinctGenres(songs []library.Song) []string {
	return distinctField(songs, func(s library.Song) string { return s.Genre })
}

func distinctArtists(songs []library.Song) []string {
	return distinctField(songs, func(s library.Song) string { return s.Artist })
}

func distinctField(songs []library.Song, f func(library.Song) string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range songs {
		v := strings.TrimSpace(f(s))
		if v == "" {
			continue
		}
		k := strings.ToLower(v)
		if !seen[k] {
			seen[k] = true
			out = append(out, v)
		}
	}
	sort.Slice(out, func(a, b int) bool {
		return strings.ToLower(out[a]) < strings.ToLower(out[b])
	})
	return out
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(v); err != nil && !errors.Is(err, http.ErrHandlerTimeout) {
		// Response likely already partially written; nothing actionable.
		return
	}
}

func writeErr(w http.ResponseWriter, code int, err error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

func safeFilename(name string) string {
	repl := strings.NewReplacer("/", "-", "\\", "-", ":", "-", "*", "-",
		"?", "", "\"", "", "<", "", ">", "", "|", "-")
	return strings.TrimSpace(repl.Replace(name))
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		if strings.HasPrefix(r.URL.Path, "/api/") {
			fmt.Printf("%s %s %s\n", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
		}
	})
}
