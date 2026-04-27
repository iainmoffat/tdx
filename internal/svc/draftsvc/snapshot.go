package draftsvc

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/iainmoffat/tdx/internal/config"
	"github.com/iainmoffat/tdx/internal/domain"
)

// OpTag identifies the operation that triggered a snapshot.
type OpTag string

const (
	// OpPrePull tags a snapshot taken before a remote pull.
	OpPrePull OpTag = "pre-pull"
	// OpPrePush tags a snapshot taken before a remote push.
	OpPrePush OpTag = "pre-push"
	// OpPreRefresh tags a snapshot taken before refreshing remote data.
	OpPreRefresh OpTag = "pre-refresh"
	// OpPreRestore tags a snapshot taken before restoring from a prior snapshot.
	OpPreRestore OpTag = "pre-restore"
	// OpPreDelete tags a snapshot taken before deleting a draft.
	OpPreDelete OpTag = "pre-delete"
	// OpManual tags a snapshot taken by explicit user request.
	OpManual OpTag = "manual"
)

// SnapshotInfo describes one persisted snapshot.
type SnapshotInfo struct {
	// Sequence is the monotonically-increasing snapshot number within the draft.
	Sequence int
	// Op is the operation that triggered the snapshot.
	Op OpTag
	// Taken is the UTC time at which the snapshot was written.
	Taken time.Time
	// Pinned reports whether the snapshot is exempt from retention pruning.
	Pinned bool
	// Note is the optional human-readable annotation embedded in the filename.
	Note string
	// Path is the absolute path to the snapshot YAML file.
	Path string
}

// SnapshotStore manages per-draft snapshot files in <draftName>.snapshots/.
// Snapshot filenames follow the pattern NNNN-<op>-<ts>[-<note>].yaml.
type SnapshotStore struct {
	paths     config.Paths
	retention int
}

// NewSnapshotStore constructs a SnapshotStore. retention <= 0 defaults to 10.
func NewSnapshotStore(paths config.Paths, retention int) *SnapshotStore {
	if retention <= 0 {
		retention = 10
	}
	return &SnapshotStore{paths: paths, retention: retention}
}

func (ss *SnapshotStore) dir(profile string, weekStart time.Time, name string) string {
	dateDir := weekStart.In(domain.EasternTZ).Format("2006-01-02")
	return filepath.Join(ss.paths.ProfileWeeksDir(profile), dateDir, name+".snapshots")
}

// snapNameRE matches NNNN-<op>-<ts>[-<note>].yaml.
// The op-tag character class [\w-]+ is greedy but backtracks against the rigid
// timestamp pattern \d{8}T\d{6}Z, ensuring even hyphenated tags like "pre-pull"
// parse correctly.
var snapNameRE = regexp.MustCompile(`^(\d{4})-([\w-]+)-(\d{8}T\d{6}Z)(?:-([^.]+))?\.yaml$`)

// Take writes a snapshot of d and returns its info.
// The snapshot directory is created on first use.
// After writing, auto-prune is applied per the configured retention limit.
func (ss *SnapshotStore) Take(d domain.WeekDraft, op OpTag, note string) (SnapshotInfo, error) {
	dir := ss.dir(d.Profile, d.WeekStart, d.Name)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return SnapshotInfo{}, err
	}

	seq, err := ss.nextSequence(dir)
	if err != nil {
		return SnapshotInfo{}, err
	}

	ts := time.Now().UTC().Format("20060102T150405Z")
	suffix := ""
	if note != "" {
		// Sanitize note for filename use: lower-case alphanumerics and hyphens only.
		safe := strings.Map(func(r rune) rune {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
				return r
			}
			if r >= 'A' && r <= 'Z' {
				return r + 32
			}
			return '-'
		}, note)
		suffix = "-" + safe
	}
	filename := fmt.Sprintf("%04d-%s-%s%s.yaml", seq, op, ts, suffix)
	p := filepath.Join(dir, filename)

	data, err := yaml.Marshal(d)
	if err != nil {
		return SnapshotInfo{}, err
	}
	if err := os.WriteFile(p, data, 0o600); err != nil {
		return SnapshotInfo{}, err
	}

	if err := ss.prune(d.Profile, d.WeekStart, d.Name); err != nil {
		return SnapshotInfo{}, err
	}
	return SnapshotInfo{Sequence: seq, Op: op, Taken: time.Now().UTC(), Note: note, Path: p}, nil
}

// List returns all snapshots for the given draft, ordered by sequence ascending.
func (ss *SnapshotStore) List(profile string, weekStart time.Time, name string) ([]SnapshotInfo, error) {
	dir := ss.dir(profile, weekStart, name)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []SnapshotInfo
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if e.Name() == ".pinned" {
			continue
		}
		m := snapNameRE.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		seq, _ := strconv.Atoi(m[1])
		ts, _ := time.Parse("20060102T150405Z", m[3])
		info := SnapshotInfo{
			Sequence: seq,
			Op:       OpTag(m[2]),
			Taken:    ts,
			Note:     m[4],
			Path:     filepath.Join(dir, e.Name()),
		}
		out = append(out, info)
	}
	pinned, _ := ss.loadPinned(dir)
	for i := range out {
		if pinned[out[i].Sequence] {
			out[i].Pinned = true
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Sequence < out[j].Sequence })
	return out, nil
}

// Pin marks a snapshot as exempt from retention pruning.
// The pin is recorded in a .pinned sidecar file in the snapshots directory.
func (ss *SnapshotStore) Pin(profile string, weekStart time.Time, name string, seq int, note string) error {
	dir := ss.dir(profile, weekStart, name)
	pinned, _ := ss.loadPinned(dir)
	pinned[seq] = true
	return ss.savePinned(dir, pinned)
}

// Load reads a specific snapshot by sequence number and returns the draft it
// contains. Returns an error if seq is not found.
func (ss *SnapshotStore) Load(profile string, weekStart time.Time, name string, seq int) (domain.WeekDraft, error) {
	list, err := ss.List(profile, weekStart, name)
	if err != nil {
		return domain.WeekDraft{}, err
	}
	for _, s := range list {
		if s.Sequence == seq {
			data, err := os.ReadFile(s.Path)
			if err != nil {
				return domain.WeekDraft{}, err
			}
			var d domain.WeekDraft
			if err := yaml.Unmarshal(data, &d); err != nil {
				return domain.WeekDraft{}, err
			}
			return d, nil
		}
	}
	return domain.WeekDraft{}, fmt.Errorf("snapshot %d not found", seq)
}

// nextSequence returns the next available sequence number by scanning existing
// snapshot filenames in dir.
func (ss *SnapshotStore) nextSequence(dir string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 1, nil
		}
		return 0, err
	}
	max := 0
	for _, e := range entries {
		m := snapNameRE.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		if seq, _ := strconv.Atoi(m[1]); seq > max {
			max = seq
		}
	}
	return max + 1, nil
}

// prune deletes the oldest unpinned snapshots until at most ss.retention unpinned
// snapshots remain. Pinned snapshots are never removed.
func (ss *SnapshotStore) prune(profile string, weekStart time.Time, name string) error {
	list, err := ss.List(profile, weekStart, name)
	if err != nil {
		return err
	}
	var unpinned []SnapshotInfo
	for _, s := range list {
		if !s.Pinned {
			unpinned = append(unpinned, s)
		}
	}
	if len(unpinned) <= ss.retention {
		return nil
	}
	sort.SliceStable(unpinned, func(i, j int) bool { return unpinned[i].Sequence < unpinned[j].Sequence })
	excess := len(unpinned) - ss.retention
	for i := 0; i < excess; i++ {
		if err := os.Remove(unpinned[i].Path); err != nil {
			return err
		}
	}
	return nil
}

// loadPinned reads the .pinned sidecar file and returns a set of pinned sequence
// numbers. Missing .pinned is treated as an empty set.
func (ss *SnapshotStore) loadPinned(dir string) (map[int]bool, error) {
	p := filepath.Join(dir, ".pinned")
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return map[int]bool{}, nil
		}
		return nil, err
	}
	out := map[int]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		if seq, err := strconv.Atoi(strings.TrimSpace(line)); err == nil {
			out[seq] = true
		}
	}
	return out, nil
}

// savePinned writes the set of pinned sequence numbers to the .pinned sidecar.
func (ss *SnapshotStore) savePinned(dir string, pinned map[int]bool) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	var lines []string
	for seq := range pinned {
		lines = append(lines, strconv.Itoa(seq))
	}
	sort.Strings(lines)
	return os.WriteFile(filepath.Join(dir, ".pinned"), []byte(strings.Join(lines, "\n")+"\n"), 0o600)
}
