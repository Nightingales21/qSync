// Package manifest scans a directory tree and produces a content-addressed
// listing of its files, shared by both the qSync client and server so that
// diffing is apples-to-apples.
package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// FileEntry describes a single file within a synced directory tree.
type FileEntry struct {
	// Path is the file's location relative to the root of the synced
	// directory, using forward slashes regardless of OS.
	Path string `json:"path"`
	Size int64  `json:"size"`
	// ModTime is Unix nanoseconds; informational only, not used for diffing.
	ModTime int64 `json:"mod_time"`
	// Hash is the lowercase hex-encoded SHA-256 digest of the file contents.
	Hash string `json:"hash"`
}

// Manifest is an ordered (by Path) collection of FileEntry, indexable by path.
type Manifest struct {
	Entries []FileEntry `json:"entries"`
}

// ByPath returns the manifest entries keyed by their relative path.
func (m Manifest) ByPath() map[string]FileEntry {
	out := make(map[string]FileEntry, len(m.Entries))
	for _, e := range m.Entries {
		out[e.Path] = e
	}
	return out
}

// Scan walks root and builds a Manifest of every regular file found beneath
// it. Paths in the resulting manifest are relative to root and use forward
// slashes.
func Scan(root string) (Manifest, error) {
	var entries []FileEntry

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		hash, err := hashFile(path)
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		entries = append(entries, FileEntry{
			Path:    filepath.ToSlash(rel),
			Size:    info.Size(),
			ModTime: info.ModTime().UnixNano(),
			Hash:    hash,
		})
		return nil
	})
	if err != nil {
		return Manifest{}, err
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })

	return Manifest{Entries: entries}, nil
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
