// SPDX-License-Identifier: MIT

package safety

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	JournalVersion = 1
	EventMeta      = "meta"
	EventIntent    = "intent"
	EventConfirmed = "confirmed"
	EventFailed    = "failed"
	EventRetained  = "retained"
	EventDeleted   = "deleted"
	EventPolicy    = "policy"
)

var runIDPattern = regexp.MustCompile(`^run-[0-9]+-[0-9a-f]{12}$`)

// JournalMeta binds a recovery journal to one provider/account/tenant/scope.
type JournalMeta struct {
	RunID    string `json:"run_id"`
	Provider string `json:"provider"`
	Account  string `json:"account"`
	Tenant   string `json:"tenant,omitempty"`
	Scope    string `json:"scope,omitempty"`
	Endpoint string `json:"endpoint,omitempty"`
}

func (m JournalMeta) validate() error {
	if !runIDPattern.MatchString(m.RunID) {
		return fmt.Errorf("invalid run ID %q", m.RunID)
	}
	if strings.TrimSpace(m.Provider) == "" || strings.TrimSpace(m.Account) == "" {
		return errors.New("journal provider and account must not be empty")
	}
	return nil
}

// JournalRecord is one append-only recovery event.
type JournalRecord struct {
	Version       int    `json:"version"`
	Seq           int64  `json:"seq"`
	Time          string `json:"time"`
	Event         string `json:"event"`
	RunID         string `json:"run_id"`
	Provider      string `json:"provider"`
	Account       string `json:"account"`
	Tenant        string `json:"tenant,omitempty"`
	Scope         string `json:"scope,omitempty"`
	Endpoint      string `json:"endpoint,omitempty"`
	Kind          string `json:"kind,omitempty"`
	Location      string `json:"location,omitempty"`
	Name          string `json:"name,omitempty"`
	CreatedAtUnix int64  `json:"created_at_unix,omitempty"`
	ProviderID    string `json:"provider_id,omitempty"`
	OperationID   string `json:"operation_id,omitempty"`
	Reason        string `json:"reason,omitempty"`
	Retain        bool   `json:"retain,omitempty"`
}

// Journal is an append-only, fsynced recovery log. A write failure poisons the
// journal so the host refuses further mutations until the operator intervenes.
type Journal struct {
	mu     sync.Mutex
	path   string
	file   *os.File
	meta   JournalMeta
	seq    int64
	offset int64
	locked bool
	err    error
}

// DefaultStateDir returns LMGCP_STATE_DIR or a per-user state directory.
func DefaultStateDir() string {
	if dir := strings.TrimSpace(os.Getenv("LMGCP_STATE_DIR")); dir != "" {
		return dir
	}
	if dir, err := os.UserConfigDir(); err == nil && dir != "" {
		return filepath.Join(dir, "lmgcp")
	}
	return filepath.Join(os.TempDir(), fmt.Sprintf("lmgcp-%d", os.Getuid()))
}

// JournalPath returns the canonical journal file for a valid run ID, or "".
func JournalPath(stateDir, runID string) string {
	runID = strings.TrimSpace(runID)
	if !runIDPattern.MatchString(runID) {
		return ""
	}
	return filepath.Join(stateDir, "journal-"+runID+".jsonl")
}

// EnsureStateDir creates only the leaf journal directory with 0700, or verifies
// an existing leaf directory is owned by the current user and not shared.
func EnsureStateDir(dir string) error {
	info, err := os.Lstat(dir)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.Mkdir(dir, 0o700); err != nil {
			return fmt.Errorf("create state directory: %w", err)
		}
		info, err = os.Lstat(dir)
	}
	if err != nil {
		return fmt.Errorf("inspect state directory: %w", err)
	}
	return checkPrivateDir(dir, info)
}

func checkPrivateDir(dir string, info os.FileInfo) error {
	if !info.IsDir() {
		return fmt.Errorf("state path %q is not a directory", dir)
	}
	return platformDirCheck(dir, info)
}

// OpenJournal creates or reopens one journal, validates its bound metadata,
// and optionally takes the exclusive run lock.
func OpenJournal(path string, meta JournalMeta, lock bool) (*Journal, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("journal path must not be empty")
	}
	if err := meta.validate(); err != nil {
		return nil, err
	}
	dir := filepath.Dir(path)
	info, err := os.Lstat(dir)
	if err != nil {
		return nil, fmt.Errorf("journal directory must exist: %w", err)
	}
	if err := checkPrivateDir(dir, info); err != nil {
		return nil, err
	}
	existing, err := LoadJournal(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if len(existing.Records) > 0 {
		if existing.Meta != meta {
			return nil, errors.New("recovery journal metadata does not match this host")
		}
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_RDWR|oNoFollow, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open recovery journal: %w", err)
	}
	opened, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("inspect recovery journal: %w", err)
	}
	if !opened.Mode().IsRegular() {
		file.Close()
		return nil, errors.New("recovery journal must be a regular file")
	}
	if uid, ok := fileOwner(opened); ok && uid != os.Getuid() {
		file.Close()
		return nil, fmt.Errorf("recovery journal is owned by uid %d, not %d", uid, os.Getuid())
	}
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		return nil, fmt.Errorf("secure recovery journal: %w", err)
	}
	j := &Journal{path: path, file: file, meta: meta, seq: existing.MaxSeq}
	if lock {
		if err := lockFile(file); err != nil {
			file.Close()
			return nil, fmt.Errorf("recovery journal is locked by another run: %w", err)
		}
		j.locked = true
	}
	if data, err := os.ReadFile(path); err == nil && len(data) > 0 && data[len(data)-1] != '\n' {
		cut := bytes.LastIndexByte(data, '\n')
		size := int64(0)
		if cut >= 0 {
			size = int64(cut + 1)
		}
		if err := file.Truncate(size); err != nil {
			file.Close()
			return nil, fmt.Errorf("truncate torn journal tail: %w", err)
		}
		if err := file.Sync(); err != nil {
			file.Close()
			return nil, err
		}
	}
	if len(existing.Records) == 0 {
		if err := j.append(JournalRecord{Event: EventMeta}); err != nil {
			file.Close()
			return nil, err
		}
		if err := syncDir(dir); err != nil {
			file.Close()
			return nil, err
		}
	}
	offset, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		file.Close()
		return nil, err
	}
	j.offset = offset
	return j, nil
}

func (j *Journal) append(record JournalRecord) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.err != nil {
		return j.err
	}
	offset := j.offset
	j.seq++
	record.Version = JournalVersion
	record.Seq = j.seq
	record.Time = time.Now().UTC().Format(time.RFC3339Nano)
	record.RunID = j.meta.RunID
	record.Provider = j.meta.Provider
	record.Account = j.meta.Account
	record.Tenant = j.meta.Tenant
	record.Scope = j.meta.Scope
	record.Endpoint = j.meta.Endpoint
	payload, err := json.Marshal(record)
	if err != nil {
		j.err = err
		return err
	}
	payload = append(payload, '\n')
	if _, err = j.file.Write(payload); err == nil {
		err = j.file.Sync()
	}
	if err != nil {
		_ = j.file.Truncate(offset)
		_ = j.file.Sync()
		j.err = fmt.Errorf("write recovery journal: %w", err)
		return j.err
	}
	j.offset = offset + int64(len(payload))
	return nil
}

// Intent durably records a create before the provider request is issued.
func (j *Journal) Intent(kind, location, name string, createdAtUnix int64) error {
	if createdAtUnix <= 0 {
		createdAtUnix = time.Now().Unix()
	}
	token := j.meta.RunID + ":" + kind + ":" + name
	return j.append(JournalRecord{Event: EventIntent, Kind: kind, Location: location, Name: name, CreatedAtUnix: createdAtUnix, OperationID: token})
}

// Confirm records the concrete provider identity after a successful create.
func (j *Journal) Confirm(kind, location, name, providerID string, createdAtUnix int64) error {
	if createdAtUnix <= 0 {
		createdAtUnix = time.Now().Unix()
	}
	return j.append(JournalRecord{Event: EventConfirmed, Kind: kind, Location: location, Name: name, ProviderID: providerID, CreatedAtUnix: createdAtUnix})
}

// Failed records a definite create failure so no pending intent survives.
func (j *Journal) Failed(kind, location, name, reason string) error {
	return j.append(JournalRecord{Event: EventFailed, Kind: kind, Location: location, Name: name, Reason: reason})
}

// Retain keeps an entry with a reason instead of tombstoning it.
func (j *Journal) Retain(kind, location, name, reason string) error {
	return j.append(JournalRecord{Event: EventRetained, Kind: kind, Location: location, Name: name, Reason: reason})
}

// Deleted tombstones only after a confirmed cleanup or terminal absence.
func (j *Journal) Deleted(kind, location, name string) error {
	return j.append(JournalRecord{Event: EventDeleted, Kind: kind, Location: location, Name: name})
}

// SetRetention persists the deliberate retention policy.
func (j *Journal) SetRetention(retain bool) error {
	return j.append(JournalRecord{Event: EventPolicy, Retain: retain})
}

func (j *Journal) Path() string { return j.path }

func (j *Journal) Err() error {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.err
}

func (j *Journal) Close() error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.file == nil {
		return nil
	}
	if j.locked {
		_ = unlockFile(j.file)
		j.locked = false
	}
	err := j.file.Close()
	j.file = nil
	return err
}

func syncDir(dir string) error {
	// Go cannot flush directory handles on Windows; the journal file itself is
	// still synced before mutation, so keep the portable open path usable there.
	if runtime.GOOS == "windows" {
		return nil
	}
	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}

// JournalLoad is the result of replaying one journal.
type JournalLoad struct {
	Meta     JournalMeta
	Records  []JournalRecord
	MaxSeq   int64
	Warnings []string
}

// LoadJournal replays a journal file. A torn final line without a newline is
// ignored with a warning; any other invalid line is fatal.
func LoadJournal(path string) (JournalLoad, error) {
	var out JournalLoad
	data, err := os.ReadFile(path)
	if err != nil {
		return out, err
	}
	if len(data) == 0 {
		return out, nil
	}
	hasFinalNewline := data[len(data)-1] == '\n'
	lines := bytes.Split(data, []byte{'\n'})
	for index, raw := range lines {
		line := strings.TrimSpace(string(raw))
		if line == "" {
			continue
		}
		var record JournalRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			if index == len(lines)-1 && !hasFinalNewline {
				out.Warnings = append(out.Warnings, "ignored torn final journal line")
				break
			}
			return out, fmt.Errorf("invalid recovery journal: %w", err)
		}
		if record.Version != JournalVersion {
			return out, fmt.Errorf("unsupported recovery journal version %d", record.Version)
		}
		if record.Event == EventMeta {
			meta := JournalMeta{RunID: record.RunID, Provider: record.Provider, Account: record.Account, Tenant: record.Tenant, Scope: record.Scope, Endpoint: record.Endpoint}
			if err := meta.validate(); err != nil {
				return out, err
			}
			if out.Meta.RunID == "" {
				out.Meta = meta
			}
		}
		if out.Meta.RunID == "" {
			return out, errors.New("recovery journal has no metadata record")
		}
		if record.RunID != out.Meta.RunID || record.Provider != out.Meta.Provider || record.Account != out.Meta.Account ||
			record.Tenant != out.Meta.Tenant || record.Scope != out.Meta.Scope || record.Endpoint != out.Meta.Endpoint {
			return out, fmt.Errorf("recovery journal record seq %d does not match metadata binding", record.Seq)
		}
		if record.Seq > out.MaxSeq {
			out.MaxSeq = record.Seq
		}
		out.Records = append(out.Records, record)
	}
	return out, nil
}
