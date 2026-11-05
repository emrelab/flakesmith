package capsule

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/emrelab/flakesmith/internal/domain"
)

// Store is a directory of capsule JSON files (repo/.flakesmith by default).
type Store struct{ Dir string }

// NewStore binds a store to a directory; it does not create anything.
func NewStore(dir string) Store { return Store{Dir: dir} }

// Path is where a capsule with this id lives.
func (s Store) Path(id string) string { return filepath.Join(s.Dir, id+".json") }

// Write persists the capsule, creating the directory lazily.
//
// The write goes through a temp file + rename so a killed process can never
// leave a half-written JSON that later fails to parse.
func (s Store) Write(c domain.Capsule) (string, error) {
	if c.ID == "" {
		return "", errors.New("capsule: empty id")
	}
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return "", fmt.Errorf("%w: %v", domain.ErrInternal, err)
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return "", fmt.Errorf("%w: %v", domain.ErrInternal, err)
	}
	data = append(data, '\n')

	final := s.Path(c.ID)
	tmp, err := os.CreateTemp(s.Dir, ".tmp-*.json")
	if err != nil {
		return "", fmt.Errorf("%w: %v", domain.ErrInternal, err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return "", fmt.Errorf("%w: %v", domain.ErrInternal, err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return "", fmt.Errorf("%w: %v", domain.ErrInternal, err)
	}
	if err := os.Rename(tmpName, final); err != nil {
		_ = os.Remove(tmpName)
		return "", fmt.Errorf("%w: %v", domain.ErrInternal, err)
	}
	return final, nil
}

// Read loads one capsule by id.
func (s Store) Read(id string) (domain.Capsule, error) {
	var c domain.Capsule
	data, err := os.ReadFile(s.Path(id))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return c, fmt.Errorf("capsule not found: %s", id)
		}
		return c, err
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return c, fmt.Errorf("%w: capsule is corrupt (%s): %v", domain.ErrInternal, id, err)
	}
	if c.Schema != domain.SchemaVersion {
		return c, fmt.Errorf("capsule schema version not supported: %s (expected %s)", c.Schema, domain.SchemaVersion)
	}
	return c, nil
}

// List returns the ids in the store, newest file first.
func (s Store) List() ([]string, error) {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	type item struct {
		id  string
		mod int64
	}
	var items []item
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") || strings.HasPrefix(e.Name(), ".tmp-") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		items = append(items, item{
			id:  strings.TrimSuffix(e.Name(), ".json"),
			mod: info.ModTime().UnixNano(),
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].mod > items[j].mod })
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.id
	}
	return out, nil
}
