// SPDX-License-Identifier: MIT

package safety

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	PrefixUnderscore         = "lmgcp_"
	PrefixHyphen             = "lmgcp-"
	TestLabelKey             = "lmgcp_test"
	CreatedAtKey             = "lmgcp_created_at"
	DefaultExampleCostCapUSD = 0.05
	HardRunCostCeilingUSD    = 0.25
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

// PriceQuote describes the unit pricing and default test lifetime for a resource/operation kind.
type PriceQuote struct {
	UnitPriceUSD   float64
	DefaultMinutes int64
	EstimatedUSD   float64
}

// LookupPrice returns the price table entry for a given resource kind and subtype (us-central1 public list pricing).
func LookupPrice(kind, subtype string) PriceQuote {
	st := strings.TrimSpace(subtype)
	if idx := strings.LastIndex(st, "/"); idx >= 0 {
		st = st[idx+1:]
	}
	switch kind {
	case "compute_instance":
		switch st {
		case "f1-micro":
			return PriceQuote{UnitPriceUSD: 0.0076, DefaultMinutes: 5, EstimatedUSD: 0.0008}
		case "e2-micro":
			return PriceQuote{UnitPriceUSD: 0.0084, DefaultMinutes: 5, EstimatedUSD: 0.0009}
		case "e2-small":
			return PriceQuote{UnitPriceUSD: 0.0168, DefaultMinutes: 5, EstimatedUSD: 0.0016}
		case "g1-small":
			return PriceQuote{UnitPriceUSD: 0.0257, DefaultMinutes: 5, EstimatedUSD: 0.0023}
		case "e2-medium":
			return PriceQuote{UnitPriceUSD: 0.0335, DefaultMinutes: 5, EstimatedUSD: 0.0030}
		case "n1-standard-1":
			return PriceQuote{UnitPriceUSD: 0.0475, DefaultMinutes: 5, EstimatedUSD: 0.0042}
		default:
			return PriceQuote{UnitPriceUSD: 0.0084, DefaultMinutes: 5, EstimatedUSD: 0.0009}
		}
	case "sql_instance":
		switch st {
		case "db-f1-micro":
			return PriceQuote{UnitPriceUSD: 0.0150, DefaultMinutes: 10, EstimatedUSD: 0.0027}
		case "db-g1-small":
			return PriceQuote{UnitPriceUSD: 0.0500, DefaultMinutes: 10, EstimatedUSD: 0.0085}
		case "db-custom-1-3840":
			return PriceQuote{UnitPriceUSD: 0.0620, DefaultMinutes: 10, EstimatedUSD: 0.0105}
		default:
			return PriceQuote{UnitPriceUSD: 0.0150, DefaultMinutes: 10, EstimatedUSD: 0.0027}
		}
	case "cloud_run":
		return PriceQuote{UnitPriceUSD: 0.0002, DefaultMinutes: 2, EstimatedUSD: 0.0004}
	case "bucket":
		return PriceQuote{UnitPriceUSD: 0.0001, DefaultMinutes: 2, EstimatedUSD: 0.0001}
	case "storage_object":
		return PriceQuote{UnitPriceUSD: 0.0001, DefaultMinutes: 1, EstimatedUSD: 0.0001}
	case "pubsub_topic":
		return PriceQuote{UnitPriceUSD: 0.0001, DefaultMinutes: 2, EstimatedUSD: 0.0001}
	case "pubsub_sub":
		return PriceQuote{UnitPriceUSD: 0.0001, DefaultMinutes: 2, EstimatedUSD: 0.0001}
	case "pubsub_msg":
		return PriceQuote{UnitPriceUSD: 0.0001, DefaultMinutes: 1, EstimatedUSD: 0.0001}
	case "bq_dataset":
		return PriceQuote{UnitPriceUSD: 0.0001, DefaultMinutes: 2, EstimatedUSD: 0.0001}
	case "bq_table":
		return PriceQuote{UnitPriceUSD: 0.0002, DefaultMinutes: 2, EstimatedUSD: 0.0002}
	case "firestore_doc":
		return PriceQuote{UnitPriceUSD: 0.0001, DefaultMinutes: 1, EstimatedUSD: 0.0001}
	case "secret":
		return PriceQuote{UnitPriceUSD: 0.0002, DefaultMinutes: 2, EstimatedUSD: 0.0002}
	case "kms_key":
		return PriceQuote{UnitPriceUSD: 0.0010, DefaultMinutes: 5, EstimatedUSD: 0.0010}
	case "artifact_repo":
		return PriceQuote{UnitPriceUSD: 0.0002, DefaultMinutes: 2, EstimatedUSD: 0.0002}
	case "dns_zone":
		return PriceQuote{UnitPriceUSD: 0.0007, DefaultMinutes: 5, EstimatedUSD: 0.0007}
	case "dns_record":
		return PriceQuote{UnitPriceUSD: 0.0001, DefaultMinutes: 1, EstimatedUSD: 0.0001}
	case "log_sink":
		return PriceQuote{UnitPriceUSD: 0.0001, DefaultMinutes: 2, EstimatedUSD: 0.0001}
	case "log_entry":
		return PriceQuote{UnitPriceUSD: 0.0001, DefaultMinutes: 1, EstimatedUSD: 0.0001}
	case "alert_policy":
		return PriceQuote{UnitPriceUSD: 0.0002, DefaultMinutes: 2, EstimatedUSD: 0.0002}
	case "vpc":
		return PriceQuote{UnitPriceUSD: 0.0001, DefaultMinutes: 2, EstimatedUSD: 0.0001}
	case "subnet":
		return PriceQuote{UnitPriceUSD: 0.0002, DefaultMinutes: 2, EstimatedUSD: 0.0002}
	case "firewall":
		return PriceQuote{UnitPriceUSD: 0.0001, DefaultMinutes: 2, EstimatedUSD: 0.0001}
	default:
		return PriceQuote{UnitPriceUSD: 0.0001, DefaultMinutes: 1, EstimatedUSD: 0.0001}
	}
}

// CostItem records a billable resource or write operation in the run's cost ledger.
type CostItem struct {
	Kind         string  `json:"kind"`
	Subtype      string  `json:"subtype,omitempty"`
	Name         string  `json:"name"`
	UnitPriceUSD float64 `json:"unit_price_usd"`
	MinutesAlive int64   `json:"minutes_alive"`
	Requests     int64   `json:"requests"`
	CostUSD      float64 `json:"cost_usd"`
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

// RunLedger tracks all resources created by the current run so only run-owned resources can be modified/deleted,
// and enforces per-example ($0.05 default) and per-run ($0.25 hard ceiling) cost caps.
type RunLedger struct {
	mu            sync.Mutex
	runID         string
	entries       []LedgerEntry
	byKey         map[string]LedgerEntry
	costCapUSD    float64
	totalCostUSD  float64
	totalRequests int64
	costItems     []CostItem
}

// NewRunLedger creates a new RunLedger for a run ID with the default $0.05 example cap.
func NewRunLedger(runID string) *RunLedger {
	if runID == "" {
		runID = fmt.Sprintf("run-%d", time.Now().UnixNano()%1000000)
	}
	return &RunLedger{
		runID:      runID,
		byKey:      make(map[string]LedgerEntry),
		costCapUSD: DefaultExampleCostCapUSD,
	}
}

func round6(v float64) float64 {
	return math.Round(v*1e6) / 1e6
}

// CostCapUSD returns the current run cost cap in USD.
func (l *RunLedger) CostCapUSD() float64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.costCapUSD
}

// SetCostCapUSD sets the example cost cap in USD, enforcing the $0.25 hard ceiling.
func (l *RunLedger) SetCostCapUSD(capUSD float64) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if capUSD <= 0 || capUSD > HardRunCostCeilingUSD {
		return fmt.Errorf("safety policy violation: cost cap $%.4f must be in (0.00, $%.2f] (hard ceiling $%.2f)", capUSD, HardRunCostCeilingUSD, HardRunCostCeilingUSD)
	}
	l.costCapUSD = round6(capUSD)
	return nil
}

// EstimatedCostUSD returns the cumulative estimated cost in USD for the current run.
func (l *RunLedger) EstimatedCostUSD() float64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return round6(l.totalCostUSD)
}

// ChargeResource verifies that creating/mutating the resource will not exceed the run's cost cap
// (default $0.05, hard ceiling $0.25) and records the cost item in the ledger.
func (l *RunLedger) ChargeResource(kind, subtype, name string, requests int64) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	q := LookupPrice(kind, subtype)
	if requests <= 0 {
		requests = 1
	}
	nextTotal := round6(l.totalCostUSD + q.EstimatedUSD)
	effectiveCap := l.costCapUSD
	if effectiveCap > HardRunCostCeilingUSD {
		effectiveCap = HardRunCostCeilingUSD
	}
	if nextTotal > effectiveCap+1e-9 {
		return fmt.Errorf("safety policy violation: cost cap exceeded creating %s %q ($%.4f would bring run estimate to $%.4f, exceeding cap $%.4f; hard ceiling $%.2f)",
			kind, name, q.EstimatedUSD, nextTotal, effectiveCap, HardRunCostCeilingUSD)
	}
	l.totalCostUSD = nextTotal
	l.totalRequests += requests
	l.costItems = append(l.costItems, CostItem{
		Kind:         kind,
		Subtype:      subtype,
		Name:         name,
		UnitPriceUSD: q.UnitPriceUSD,
		MinutesAlive: q.DefaultMinutes,
		Requests:     requests,
		CostUSD:      q.EstimatedUSD,
	})
	return nil
}

// FormatCostReport returns the canonical cost estimate line required at the end of every example.
func (l *RunLedger) FormatCostReport() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.costItems) == 0 || l.totalCostUSD == 0 {
		return "estimated cost: $0.00 (read-only; resources=0, minutes_alive=0, requests=0)"
	}
	var maxMin int64
	for _, item := range l.costItems {
		if item.MinutesAlive > maxMin {
			maxMin = item.MinutesAlive
		}
	}
	capStr := fmt.Sprintf("$%.2f", l.costCapUSD)
	if l.costCapUSD < 0.01 {
		capStr = fmt.Sprintf("$%.4f", l.costCapUSD)
	}
	return fmt.Sprintf("estimated cost: $%.4f (resources=%d, minutes_alive=%d, requests=%d, cap=%s)",
		l.totalCostUSD, len(l.costItems), maxMin, l.totalRequests, capStr)
}

// CostEvidenceJSON serializes the run's cost ledger and teardown state as JSON evidence.
func (l *RunLedger) CostEvidenceJSON(exampleName string, liveMode bool) string {
	l.mu.Lock()
	defer l.mu.Unlock()
	mode := "fake-emulator (estimated from price table, not yet measured live)"
	if liveMode {
		mode = "adc-live"
	}
	formatted := "$0.00"
	if l.totalCostUSD > 0 {
		formatted = fmt.Sprintf("$%.4f", l.totalCostUSD)
	}
	items := l.costItems
	if items == nil {
		items = []CostItem{}
	}
	doc := map[string]any{
		"example":                  exampleName,
		"run_id":                   l.runID,
		"mode":                     mode,
		"region":                   "us-central1",
		"estimated_cost_usd":       round6(l.totalCostUSD),
		"estimated_cost_formatted": formatted,
		"cost_cap_usd":             l.costCapUSD,
		"hard_ceiling_usd":         HardRunCostCeilingUSD,
		"leftover_count":           len(l.entries),
		"total_requests":           l.totalRequests,
		"items":                    items,
	}
	b, _ := json.MarshalIndent(doc, "", "  ")
	return string(b) + "\n"
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
