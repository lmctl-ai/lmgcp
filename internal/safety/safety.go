// SPDX-License-Identifier: MIT

package safety

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	PrefixUnderscore = "lmgcp_"
	PrefixHyphen     = "lmgcp-"
	TestLabelKey     = "lmgcp_test"
	CreatedAtKey     = "lmgcp_created_at"
)

var (
	reOAuthToken  = regexp.MustCompile(`ya29\.[A-Za-z0-9._\-]+`)
	reAPIKey      = regexp.MustCompile(`AIza[0-9A-Za-z._\-]{10,}`)
	rePEMKey      = regexp.MustCompile(`(?s)-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----.*?-----END [A-Z0-9 ]*PRIVATE KEY-----`)
	reJSONPrivKey = regexp.MustCompile(`"private_key"\s*:\s*"[^"]*"`)
	reConnString  = regexp.MustCompile(`(postgres|postgresql|mysql|mariadb|mongodb|redis|sqlserver)://[^:\s/]+:[^@\s]+@`)
	reKVSecret    = regexp.MustCompile(`(?i)(password|passwd|secret_value|client_secret|access_token|refresh_token|private_key_id|session_token)\s*([=:])\s*([^\s,;"]+)`)
)

// RedactSecrets scrubs tokens, keys, passwords, connection credentials, and secret payloads.
func RedactSecrets(s string) string {
	if s == "" {
		return ""
	}
	out := rePEMKey.ReplaceAllString(s, "[REDACTED_PRIVATE_KEY]")
	out = reJSONPrivKey.ReplaceAllString(out, `"private_key":"[REDACTED]"`)
	out = reOAuthToken.ReplaceAllString(out, "ya29.[REDACTED]")
	out = reAPIKey.ReplaceAllString(out, "AIza[REDACTED]")
	out = reConnString.ReplaceAllString(out, "$1://[REDACTED]@")
	out = reKVSecret.ReplaceAllString(out, "$1$2[REDACTED]")
	return out
}

// MaskProjectID masks a GCP project ID or number so raw identifiers never leak into images, logs, or docs.
func MaskProjectID(projectID string) string {
	trimmed := strings.TrimSpace(projectID)
	if trimmed == "" {
		return "gcp-proj-****0000"
	}
	if strings.HasPrefix(trimmed, "gcp-proj-****") {
		return trimmed
	}
	suffix := trimmed
	if len(suffix) > 4 {
		suffix = suffix[len(suffix)-4:]
	}
	return "gcp-proj-****" + suffix
}

// HasSafePrefix reports whether name begins with lmgcp_ or lmgcp-.
func HasSafePrefix(name string) bool {
	trimmed := strings.TrimSpace(name)
	if strings.HasPrefix(trimmed, PrefixUnderscore) && len(trimmed) > len(PrefixUnderscore) {
		return true
	}
	if strings.HasPrefix(trimmed, PrefixHyphen) && len(trimmed) > len(PrefixHyphen) {
		return true
	}
	return false
}

// ValidateResourcePrefix enforces Safety Rule 3: every created resource must start with lmgcp_ or lmgcp-.
func ValidateResourcePrefix(kind, name string) error {
	if !HasSafePrefix(name) {
		return fmt.Errorf("safety policy violation: %s resource name %q must start with %q or %q", kind, name, PrefixUnderscore, PrefixHyphen)
	}
	return nil
}

var allowedMachineTypes = map[string]bool{
	"e2-micro":      true,
	"e2-small":      true,
	"e2-medium":     true,
	"f1-micro":      true,
	"g1-small":      true,
	"n1-standard-1": true,
}

// ValidateMachineType enforces Safety Rule 5: only cost-free / near-free micro/small shapes are allowed.
func ValidateMachineType(machineType string) error {
	mt := strings.TrimSpace(machineType)
	if idx := strings.LastIndex(mt, "/"); idx >= 0 {
		mt = mt[idx+1:]
	}
	if !allowedMachineTypes[mt] {
		return fmt.Errorf("safety policy violation: machine type %q is not allowed (only e2-micro, e2-small, e2-medium, f1-micro, g1-small, n1-standard-1 permitted)", machineType)
	}
	return nil
}

var allowedSqlTiers = map[string]bool{
	"db-f1-micro":      true,
	"db-g1-small":      true,
	"db-custom-1-3840": true,
}

// ValidateSqlTier enforces Safety Rule 5 for Cloud SQL instances.
func ValidateSqlTier(tier string) error {
	t := strings.TrimSpace(tier)
	if !allowedSqlTiers[t] {
		return fmt.Errorf("safety policy violation: Cloud SQL tier %q is not allowed (only db-f1-micro, db-g1-small, db-custom-1-3840 permitted)", tier)
	}
	return nil
}

// LedgerEntry records a resource created during the current host run.
type LedgerEntry struct {
	Kind          string
	Location      string
	Name          string
	RunID         string
	CreatedAtUnix int64
}

func (e LedgerEntry) Key() string {
	return fmt.Sprintf("%s:%s:%s", e.Kind, e.Location, e.Name)
}

// RunLedger tracks all resources created by the current run so only run-owned resources can be modified/deleted.
type RunLedger struct {
	mu      sync.Mutex
	runID   string
	entries []LedgerEntry
	byKey   map[string]LedgerEntry
}

// NewRunLedger creates a new RunLedger for a run ID.
func NewRunLedger(runID string) *RunLedger {
	if runID == "" {
		runID = fmt.Sprintf("run-%d", time.Now().UnixNano()%1000000)
	}
	return &RunLedger{
		runID: runID,
		byKey: make(map[string]LedgerEntry),
	}
}

func (l *RunLedger) RunID() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.runID
}

func (l *RunLedger) SetRunID(id string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if strings.TrimSpace(id) != "" {
		l.runID = strings.TrimSpace(id)
	}
}

// Record adds a newly created resource to the run's ledger.
func (l *RunLedger) Record(kind, location, name string, createdAtUnix int64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if createdAtUnix <= 0 {
		createdAtUnix = time.Now().Unix()
	}
	entry := LedgerEntry{
		Kind:          kind,
		Location:      location,
		Name:          name,
		RunID:         l.runID,
		CreatedAtUnix: createdAtUnix,
	}
	k := entry.Key()
	if _, exists := l.byKey[k]; !exists {
		l.entries = append(l.entries, entry)
	}
	l.byKey[k] = entry
}

// Remove removes a deleted resource from the active ledger.
func (l *RunLedger) Remove(kind, location, name string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for k, e := range l.byKey {
		if e.Kind == kind && e.Name == name && (location == "" || e.Location == location) {
			delete(l.byKey, k)
		}
	}
	kept := l.entries[:0]
	for _, e := range l.entries {
		if e.Kind == kind && e.Name == name && (location == "" || e.Location == location) {
			continue
		}
		kept = append(kept, e)
	}
	l.entries = kept
}

// Owns reports whether the current run created the given resource.
func (l *RunLedger) Owns(kind, name string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, e := range l.byKey {
		if e.Kind == kind && e.Name == name {
			return true
		}
	}
	return false
}

// ValidateModifyOrDelete enforces Safety Rules 2, 3, and 4:
// - The resource name must carry the lmgcp_ / lmgcp- prefix
// - The resource must have been created in this run's ledger
func (l *RunLedger) ValidateModifyOrDelete(kind, name string) error {
	if err := ValidateResourcePrefix(kind, name); err != nil {
		return err
	}
	if !l.Owns(kind, name) {
		return fmt.Errorf("safety policy violation: %s %q is not in this run's ledger (refusing to modify or delete pre-existing resource)", kind, name)
	}
	return nil
}

// Count returns the number of active resources in the ledger.
func (l *RunLedger) Count() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.entries)
}

// Snapshot returns a copy of all active ledger entries in creation order.
func (l *RunLedger) Snapshot() []LedgerEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]LedgerEntry, len(l.entries))
	copy(out, l.entries)
	return out
}

// EnrichLabels copies user labels and injects mandatory lmgcp_test and lmgcp_created_at labels.
func (l *RunLedger) EnrichLabels(userLabels map[string]string, nowUnix int64) map[string]string {
	out := make(map[string]string, len(userLabels)+2)
	for k, v := range userLabels {
		out[k] = RedactSecrets(v)
	}
	out[TestLabelKey] = l.RunID()
	if nowUnix <= 0 {
		nowUnix = time.Now().Unix()
	}
	out[CreatedAtKey] = fmt.Sprintf("%d", nowUnix)
	return out
}
