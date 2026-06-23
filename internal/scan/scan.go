// Package scan walks a directory tree and collects supported audio files.
package scan

import (
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/Ahdeyyy/go_muse/internal/model"
)

// Supported lists the file extensions we attempt to analyze. ffmpeg decodes
// all of these; the metadata reader covers the common containers.
var Supported = map[string]bool{
	".mp3":  true,
	".m4a":  true,
	".aac":  true,
	".flac": true,
	".ogg":  true,
	".oga":  true,
	".opus": true,
	".wav":  true,
	".wma":  true,
}

// Walk returns every supported audio file under root. Unreadable entries are
// skipped rather than aborting the whole scan.
func Walk(root string) ([]model.FileInfo, error) {
	var out []model.FileInfo
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable dirs/files, keep going
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if !Supported[ext] {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		out = append(out, model.FileInfo{
			Path:    path,
			Name:    d.Name(),
			Ext:     ext,
			Size:    info.Size(),
			ModTime: info.ModTime(),
		})
		return nil
	})
	return out, err
}
