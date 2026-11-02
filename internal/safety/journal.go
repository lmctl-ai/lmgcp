// SPDX-License-Identifier: MIT

package safety

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	JournalVersion = 1
	EventMeta      = "meta"
	EventIntent    = "intent"
	EventConfirmed = "confirmed"
	EventRetained  = "retained"
	EventDeleted   = "deleted"
	EventPolicy    = "policy"
)

// JournalMeta binds a recovery journal to one provider/account/tenant/scope.
type JournalMeta struct {
	RunID    string `json:"run_id"`
	Provider string `json:"provider"`
	Account  string `json:"account"`
	Tenant   string `json:"tenant,omitempty"`
	Scope    string `json:"scope,omitempty"`
	Endpoint string `json:"endpoint,omitempty"`
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
	mu   sync.Mutex
	path string
	file *os.File
	meta JournalMeta
	seq  int64
	err  error
}

// DefaultStateDir returns LMGCP_STATE_DIR or the per-user state directory.
func DefaultStateDir() string {
	if dir := strings.TrimSpace(os.Getenv("LMGCP_STATE_DIR")); dir != "" {
		return dir
	}
	if dir, err := os.UserConfigDir(); err == nil && dir != "" {
		return filepath.Join(dir, "lmgcp")
	}
	return filepath.Join(os.TempDir(), "lmgcp-state")
}

// JournalPath returns the canonical journal file for a run ID.
func JournalPath(stateDir, runID string) string {
	return filepath.Join(stateDir, "journal-"+strings.TrimSpace(runID)+".jsonl")
}

// OpenJournal creates or reopens one journal and validates its bound metadata.
func OpenJournal(path string, meta JournalMeta) (*Journal, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("journal path must not be empty")
	}
	if strings.TrimSpace(meta.RunID) == "" {
		return nil, errors.New("journal run ID must not be empty")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create journal directory: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return nil, fmt.Errorf("secure journal directory: %w", err)
	}
	existing, err := LoadJournal(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if len(existing.Records) > 0 {
		if existing.Meta.RunID != meta.RunID || existing.Meta.Provider != meta.Provider ||
			existing.Meta.Account != meta.Account || existing.Meta.Tenant != meta.Tenant ||
			existing.Meta.Scope != meta.Scope || existing.Meta.Endpoint != meta.Endpoint {
			return nil, errors.New("recovery journal metadata does not match this host")
		}
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open recovery journal: %w", err)
	}
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		return nil, fmt.Errorf("secure recovery journal: %w", err)
	}
	j := &Journal{path: path, file: file, meta: meta, seq: existing.MaxSeq}
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
	return j, nil
}

func (j *Journal) append(record JournalRecord) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.err != nil {
		return j.err
	}
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
		j.err = err
		return fmt.Errorf("write recovery journal: %w", err)
	}
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
	err := j.file.Close()
	j.file = nil
	return err
}

// JournalLoad is the result of replaying one journal.
type JournalLoad struct {
	Meta    JournalMeta
	Records []JournalRecord
	MaxSeq  int64
}

// LoadJournal replays a journal file. Missing files return os.ErrNotExist.
func syncDir(dir string) error {
	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}

func LoadJournal(path string) (JournalLoad, error) {
	var out JournalLoad
	file, err := os.Open(path)
	if err != nil {
		return out, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var record JournalRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			return out, fmt.Errorf("invalid recovery journal: %w", err)
		}
		if record.Version != JournalVersion {
			return out, fmt.Errorf("unsupported recovery journal version %d", record.Version)
		}
		if record.Event == EventMeta && out.Meta.RunID == "" {
			out.Meta = JournalMeta{RunID: record.RunID, Provider: record.Provider, Account: record.Account, Tenant: record.Tenant, Scope: record.Scope, Endpoint: record.Endpoint}
		}
		if record.Seq > out.MaxSeq {
			out.MaxSeq = record.Seq
		}
		out.Records = append(out.Records, record)
	}
	if err := scanner.Err(); err != nil {
		return out, fmt.Errorf("read recovery journal: %w", err)
	}
	if len(out.Records) > 0 && out.Meta.RunID == "" {
		return out, errors.New("recovery journal has no metadata record")
	}
	return out, nil
}
