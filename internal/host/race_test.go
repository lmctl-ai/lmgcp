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
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			_ = h.LastError()
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			_ = h.CleanupLedger()
		}
	}()
	wg.Wait()
}
