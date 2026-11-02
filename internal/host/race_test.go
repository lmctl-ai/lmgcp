// SPDX-License-Identifier: MIT

package host

import (
	"sync"
	"testing"
)

// TestCleanupLedgerConcurrentLastError reproduces the SolReviewer race:
// CleanupLedger (via StorageDeleteBucket -> fail/clearErr) overlapping a
// registered last_error read. Run with -race.
func TestCleanupLedgerConcurrentLastError(t *testing.T) {
	h, err := NewHost(false)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	if got := h.StorageCreateBucket("lmgcp-race-bucket", "US", "STANDARD"); got == nil || h.LastError() != "" {
		t.Fatalf("seed owned bucket: %v", h.LastError())
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			_ = h.LastError()
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			_ = h.CleanupLedger()
		}
	}()
	wg.Wait()
}
