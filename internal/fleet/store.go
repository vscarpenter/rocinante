package fleet

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
)

// Snapshot is the whole fleet at one instant: the agents that read cleanly, and
// the files that did not. Errors are keyed by filename so the bridge can show a
// visible error line instead of crashing on a bad file.
type Snapshot struct {
	Agents []Status
	Errors map[string]error
}

// LoadSnapshot reads every status file in dir and returns a Snapshot. A missing
// directory yields an empty snapshot, not an error, since the fleet may simply
// have no agents yet. Files that do not end in .json, including the temp files
// from an in-flight write, are ignored.
func LoadSnapshot(dir string) Snapshot {
	snap := Snapshot{Errors: map[string]error{}}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return snap // missing or unreadable dir means no agents yet
	}

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".json") {
			continue
		}
		s, err := Read(filepath.Join(dir, name))
		if err != nil {
			snap.Errors[name] = err
			continue
		}
		snap.Agents = append(snap.Agents, s)
	}

	sort.Slice(snap.Agents, func(i, j int) bool {
		return snap.Agents[i].ID < snap.Agents[j].ID
	})
	return snap
}

// Store watches a fleet directory and publishes a fresh Snapshot whenever the
// directory changes. It bridges fsnotify, which runs in its own goroutine, to
// the Bubble Tea render loop through the Updates channel.
type Store struct {
	dir     string
	watcher *fsnotify.Watcher
	updates chan Snapshot
	done    chan struct{}

	mu      sync.RWMutex
	current Snapshot
}

// NewStore creates the fleet directory if needed, loads the initial snapshot,
// and starts watching for changes.
func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	if err := watcher.Add(dir); err != nil {
		_ = watcher.Close()
		return nil, err
	}

	s := &Store{
		dir:     dir,
		watcher: watcher,
		updates: make(chan Snapshot),
		done:    make(chan struct{}),
		current: LoadSnapshot(dir),
	}
	go s.loop()
	return s, nil
}

// Snapshot returns the most recent fleet snapshot. It is safe to call from the
// render goroutine while the watcher is running.
func (s *Store) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.current
}

// Updates is the channel the bridge listens on. Each value is a complete, fresh
// snapshot, so a missed intermediate event never loses final state.
func (s *Store) Updates() <-chan Snapshot {
	return s.updates
}

// Close stops watching and releases the watcher. The loop goroutine exits even
// if it is mid-publish.
func (s *Store) Close() error {
	close(s.done)
	return s.watcher.Close()
}

// loop folds fsnotify events into reloads. On any change it reloads the whole
// directory, which is cheap for a small fleet and avoids tracking per-file
// state. It then publishes the new snapshot, blocking until the bridge reads it
// or the store closes.
func (s *Store) loop() {
	for {
		select {
		case <-s.done:
			return
		case _, ok := <-s.watcher.Events:
			if !ok {
				return
			}
			snap := LoadSnapshot(s.dir)
			s.mu.Lock()
			s.current = snap
			s.mu.Unlock()
			select {
			case s.updates <- snap:
			case <-s.done:
				return
			}
		case _, ok := <-s.watcher.Errors:
			if !ok {
				return
			}
			// A watcher error should not take down the bridge. Reload so the
			// view stays as current as the filesystem allows.
			snap := LoadSnapshot(s.dir)
			s.mu.Lock()
			s.current = snap
			s.mu.Unlock()
			select {
			case s.updates <- snap:
			case <-s.done:
				return
			}
		}
	}
}
