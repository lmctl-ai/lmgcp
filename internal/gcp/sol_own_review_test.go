// SPDX-License-Identifier: MIT

package gcp

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/lmctl-ai/lmcloud/hostkit"
	"github.com/lmctl-ai/lmcloud/hostkit/cloudhost"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func solOwnItem(h *Host, table, pk string) fsDocument {
	fields, _ := encodeFirestoreFields(hostkit.Item{PK: pk, Attrs: map[string]hostkit.Attr{"v": hostkit.IntAttr(1)}}, "old-version")
	return fsDocument{Name: h.firestoreDocPath(table, pk), Fields: fields, UpdateTime: fsTimestamp{Seconds: 7}}
}

func TestSolOwnTombstoneResume(t *testing.T) {
	m, ep := newMockFirestoreServer(t)
	a := NewAdapter()
	ctx := hostkit.WithEndpoint(context.Background(), ep)
	if _, e := a.CreateTable(ctx, "orders", false); e != nil {
		t.Fatal(e)
	}
	if _, e := a.PutItem(ctx, "orders", hostkit.Item{PK: "p", Attrs: map[string]hostkit.Attr{"v": hostkit.IntAttr(1)}}, hostkit.PutIfAbsent, ""); e != nil {
		t.Fatal(e)
	}
	var n atomic.Int32
	limited := hostkit.WithRetryHooks(ctx, hostkit.RetryHooks{BeforeAttempt: func(context.Context, int) error {
		if n.Add(1) == 3 {
			return hostkit.ProviderError(hostkit.CostCapExceeded, 0, "STOP", "stop purge")
		}
		return nil
	}})
	e1 := a.DeleteTable(limited, "orders")
	e2 := a.DeleteTable(ctx, "orders")
	h := NewHost()
	cat := h.firestoreDocPath(firestoreCatalogCollection, "orders")
	child := h.firestoreDocPath("orders", "p")
	m.mu.Lock()
	_, catExists := m.docs[cat]
	_, childExists := m.docs[child]
	m.mu.Unlock()
	t.Logf("first=%v retry=%v catalog=%v child=%v", e1, e2, catExists, childExists)
	if catExists || childExists {
		t.Error("retry failed to resume tombstoned deletion")
	}
}

func TestSolOwnCachedWriteDuringDeletion(t *testing.T) {
	m, ep := newMockFirestoreServer(t)
	a := NewAdapter()
	ctx := hostkit.WithEndpoint(context.Background(), ep)
	if _, e := a.CreateTable(ctx, "orders", false); e != nil {
		t.Fatal(e)
	}
	h := NewHost()
	cat := h.firestoreDocPath(firestoreCatalogCollection, "orders")
	m.mu.Lock()
	d := m.docs[cat]
	d.Fields["state"] = fsString("deleting")
	m.docs[cat] = d
	m.mu.Unlock()
	_, e := a.PutItem(ctx, "orders", hostkit.Item{PK: "new", Attrs: map[string]hostkit.Attr{}}, hostkit.PutUnconditional, "")
	t.Logf("write with cached layout + deleting catalog: %v", e)
	if e == nil {
		t.Error("write ignored deletion fence")
	}
}

func TestSolOwnDeletingTombstoneFencesReadsAndWrites(t *testing.T) {
	m, ep := newMockFirestoreServer(t)
	a := NewAdapter()
	ctx := hostkit.WithEndpoint(context.Background(), ep)
	h := NewHost()
	if _, e := a.CreateTable(ctx, "orders", true); e != nil {
		t.Fatal(e)
	}
	if _, e := a.PutItem(ctx, "orders", hostkit.Item{PK: "p", SK: "s", Attrs: map[string]hostkit.Attr{"x": hostkit.StringAttr("v")}}, hostkit.PutUnconditional, ""); e != nil {
		t.Fatal(e)
	}
	cat := h.firestoreDocPath(firestoreCatalogCollection, "orders")
	m.mu.Lock()
	d := m.docs[cat]
	d.Fields["state"] = fsString("deleting")
	m.docs[cat] = d
	m.mu.Unlock()
	for name, fn := range map[string]func() error{
		"get":    func() error { _, e := a.GetItem(ctx, "orders", "p", "s"); return e },
		"put":    func() error { _, e := a.PutItem(ctx, "orders", hostkit.Item{PK: "p", SK: "s2"}, hostkit.PutUnconditional, ""); return e },
		"update": func() error { _, e := a.UpdateItem(ctx, "orders", "p", "s", hostkit.Item{Attrs: map[string]hostkit.Attr{"x": hostkit.StringAttr("v2")}}); return e },
		"delete": func() error { return a.DeleteItem(ctx, "orders", "p", "s") },
		"query":  func() error { _, e := a.Query(ctx, "orders", "p", "", "", 10, ""); return e },
		"scan":   func() error { _, e := a.Scan(ctx, "orders", 10, ""); return e },
	} {
		if err := fn(); err == nil {
			t.Fatalf("%s succeeded while catalog state=deleting", name)
		}
	}
}

func TestSolOwnUncataloguedItem(t *testing.T) {
	m, ep := newMockFirestoreServer(t)
	a := NewAdapter()
	ctx := hostkit.WithEndpoint(context.Background(), ep)
	h := NewHost()
	d := solOwnItem(h, "orders", "p")
	m.mu.Lock()
	m.docs[d.Name] = d
	m.mu.Unlock()
	got, e := a.GetItem(ctx, "orders", "p", "")
	_, ue := a.UpdateItem(ctx, "orders", "p", "", hostkit.Item{Attrs: map[string]hostkit.Attr{"v": hostkit.IntAttr(2)}})
	_, pe := a.PutItem(ctx, "orders", hostkit.Item{PK: "p2", Attrs: map[string]hostkit.Attr{"v": hostkit.IntAttr(3)}}, hostkit.PutUnconditional, "")
	_, se := a.Scan(ctx, "orders", 10, "")
	_, ce := a.CreateTable(ctx, "orders", false)
	t.Logf("no catalog: get exists=%v err=%v update=%v put=%v scan=%v create=%v", got.Exists, e, ue, pe, se, ce)
	if e == nil || ue == nil || pe == nil || se == nil || ce == nil {
		t.Error("orphan/foreign collection bypassed authoritative catalog")
	}
}

func TestSolOwnMissingUpdateTime(t *testing.T) {
	m, ep := newMockFirestoreServer(t)
	a := NewAdapter()
	ctx := hostkit.WithEndpoint(context.Background(), ep)
	h := NewHost()
	if _, e := a.CreateTable(ctx, "orders", false); e != nil {
		t.Fatal(e)
	}
	d := solOwnItem(h, "orders", "p")
	d.UpdateTime = fsTimestamp{}
	m.mu.Lock()
	m.docs[d.Name] = d
	m.mu.Unlock()
	m.updateHook = func(req *fsUpdateDocumentRequest) error {
		m.mu.Lock()
		newer := m.docs[d.Name]
		for i := 0; i < 239; i++ {
			newer.Fields[fmt.Sprintf("a%03d", i)] = fsInt(1)
		}
		newer.Fields[tableVersionAttr] = fsString("concurrent-version")
		m.docs[d.Name] = newer
		m.mu.Unlock()
		return nil
	}
	_, e := a.UpdateItem(ctx, "orders", "p", "", hostkit.Item{Attrs: map[string]hostkit.Attr{"extra": hostkit.IntAttr(1)}})
	m.mu.Lock()
	persisted := m.docs[d.Name]
	m.mu.Unlock()
	t.Logf("missing updateTime: err=%v attrs incl internal=%d", e, len(persisted.Fields))
	if e == nil || len(persisted.Fields) > 242 {
		t.Error("exists-only fallback admitted illegal concurrent merge")
	}
}

func TestSolOwnReceiveVisibility(t *testing.T) {
	var mods atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/projects/gcpsim-local/subscriptions/jobs:modifyAckDeadline" {
			mods.Add(1)
		}
		fmt.Fprint(w, `{"receivedMessages":[{"ackId":"r","message":{"data":"eA==","messageId":"m"}}]}`)
	}))
	defer s.Close()
	a := NewAdapter()
	ctx := hostkit.WithEndpoint(context.Background(), s.URL)
	out, e := a.Receive(ctx, "jobs", 1, 0, 7)
	t.Logf("receive vis=7: msgs=%d err=%v modifyAckDeadline calls=%d", len(out), e, mods.Load())
	if e != nil {
		t.Fatal(e)
	}
	if mods.Load() == 0 {
		t.Error("receive visibility parameter ignored")
	}
}

func TestSolOwnDeleteRaceCatalogReplacement(t *testing.T) {
	m, ep := newMockFirestoreServer(t)
	a := NewAdapter()
	ctx := hostkit.WithEndpoint(context.Background(), ep)
	if _, e := a.CreateTable(ctx, "orders", false); e != nil {
		t.Fatal(e)
	}
	h := NewHost()
	catpath := h.firestoreDocPath(firestoreCatalogCollection, "orders")
	m.updateHook = func(req *fsUpdateDocumentRequest) error {
		m.mu.Lock()
		d := m.docs[catpath]
		d.Fields["incarnation"] = fsString("replacement")
		d.Fields["token"] = fsString("foreign")
		d.UpdateTime = fsTimestamp{Seconds: 9999}
		m.docs[catpath] = d
		m.mu.Unlock()
		return nil
	}
	e := a.DeleteTable(ctx, "orders")
	m.mu.Lock()
	_, exists := m.docs[catpath]
	m.mu.Unlock()
	t.Logf("replace catalog before tombstone: err=%v replacement survives=%v", e, exists)
	if !exists {
		t.Error("deleted replacement catalog without incarnation precondition")
	}
}

func TestSolOwnNotSentFromServerMessage(t *testing.T) {
	e := status.Error(codes.Unavailable, "connection refused after I committed the mutation")
	if isGRPCNotSent(e) {
		t.Error("provider status text treated as proof request was not sent")
	}
}

func TestSolOwnPurgeCost(t *testing.T) {
	_, ep := newMockFirestoreServer(t)
	a := NewAdapter()
	t.Setenv("LMGCP_ENDPOINT", ep)
	t.Setenv("LMCLOUD_CAMPAIGN_STATE", filepath.Join(t.TempDir(), "campaign-state.json"))
	h := cloudhost.New(cloudhost.Config{Provider: "gcp", Store: a, Queue: a, Tables: a, Endpoints: Endpoints()})
	h.SetAbortOnError(false)
	if _, e := h.TableCreate(false, "orders", false); e != nil {
		t.Fatal(e)
	}
	item, _ := h.ItemNew("p", "", false)
	item.SetInt("n", 1)
	if _, e := h.ItemPut(false, "orders", item, hostkit.PutIfAbsent, ""); e != nil {
		t.Fatal(e)
	}
	if _, e := h.TableDelete(false, "orders"); e != nil {
		t.Fatal(e)
	}
	line := h.CostReport().Item("table_delete")
	t.Logf("purge cost report: %+v", line)
	if line.EstimatedUSD == 0 {
		t.Error("all catalog/query/child-delete dispatches charged at zero-cost parent tariff")
	}
}

func solOwnSpy(t *testing.T) (*mockFirestoreServer, string, *atomic.Int32, *atomic.Int32) {
	t.Helper()
	lis, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	m := &mockFirestoreServer{docs: map[string]fsDocument{}, nextTick: 1000}
	var reads, writes atomic.Int32
	srv := grpc.NewServer(grpc.ForceServerCodec(rawProtoCodec{}), grpc.UnknownServiceHandler(func(v any, s grpc.ServerStream) error {
		method, _ := grpc.MethodFromServerStream(s)
		if method == rpcCreateDocument || method == rpcUpdateDocument || method == rpcDeleteDocument || method == rpcCommit {
			writes.Add(1)
		} else {
			reads.Add(1)
		}
		return m.handleStream(v, s)
	}))
	go srv.Serve(lis)
	t.Cleanup(func() { srv.Stop(); lis.Close() })
	return m, "http://" + lis.Addr().String(), &reads, &writes
}

func TestSolOwnNormalDryRun(t *testing.T) {
	if _, err := os.Stat("/home/ubuntu/lmctl-mono/lmscript/bin/lmscript"); err != nil {
		t.Skip(err)
	}
	_, ep, reads, writes := solOwnSpy(t)
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "lmgcp")
	if out, err := exec.Command("go", "build", "-o", bin, "../../cmd/lmgcp").CombinedOutput(); err != nil {
		t.Fatalf("build lmgcp: %v\n%s", err, out)
	}
	schemaPath, err := filepath.Abs("../../schema/lmcloud.dsl.json")
	if err != nil {
		t.Fatal(err)
	}
	script := fmt.Sprintf(`s = load_schema(schema=%q);
p = _lmcode_ s;
 cloud.set_endpoint("tables", %q);
 cloud.table_create("dryorders");
 cloud.item_put("dryorders", cloud.item_new("p").set_int("n", 1));
 cloud.item_update("dryorders", "p", cloud.item_new("ignored").set_int("n", 2));
 cloud.item_delete("dryorders", "p");
 cloud.table_delete("dryorders");
 lmcode_return_put("ok", cloud.last_error().code() == ErrorCode.OK);
end _lmcode_
r = run_lmcode(p, {command:[%q, "cloud", "--lmcode", "$DSLFILE"]});
assert(r.ok && r.result.ok, "dry table operations failed");
`, schemaPath, ep, bin)
	path := filepath.Join(tmp, "normal-dry-probe.lms")
	if e := os.WriteFile(path, []byte(script), 0600); e != nil {
		t.Fatal(e)
	}
	out, e := exec.Command("/home/ubuntu/lmctl-mono/lmscript/bin/lmscript", "--dry-run", path).CombinedOutput()
	t.Logf("dry rc=%v reads=%d writes=%d output=%s", e, reads.Load(), writes.Load(), out)
	if e != nil || reads.Load() == 0 || writes.Load() != 0 || strings.Contains(string(out), `"ok":false`) {
		t.Fatal("dry run failed or mutated")
	}
}

func TestSolOwnAllLiveRefused(t *testing.T) {
	_, ep, reads, writes := solOwnSpy(t)
	a := NewAdapter()
	ctx := hostkit.WithEndpoint(context.Background(), ep)
	t.Setenv("LMGCP_LIVE", "true")
	item := hostkit.Item{PK: "p", Attrs: map[string]hostkit.Attr{}}
	calls := []func() error{
		func() error { _, e := a.CreateTable(ctx, "orders", false); return e },
		func() error { return a.DeleteTable(ctx, "orders") },
		func() error { return a.HeadTable(ctx, "orders") },
		func() error { _, e := a.DescribeTable(ctx, "orders"); return e },
		func() error { _, e := a.TableCreateToken(ctx, "orders"); return e },
		func() error { _, _, e := a.ListTables(ctx, ""); return e },
		func() error { _, e := a.PutItem(ctx, "orders", item, hostkit.PutUnconditional, ""); return e },
		func() error { _, e := a.GetItem(ctx, "orders", "p", ""); return e },
		func() error { return a.DeleteItem(ctx, "orders", "p", "") },
		func() error { _, e := a.UpdateItem(ctx, "orders", "p", "", item); return e },
		func() error { _, e := a.Query(ctx, "orders", "p", "", "", 1, ""); return e },
		func() error { _, e := a.Scan(ctx, "orders", 1, ""); return e },
	}
	for i, f := range calls {
		e := f()
		if e == nil || hostkit.AsCloudError(e).Code != hostkit.Unauthenticated {
			t.Errorf("op %d err %v", i, e)
		}
	}
	if reads.Load() != 0 || writes.Load() != 0 {
		t.Fatal("live calls reached network")
	}
}

func TestSolR2ReadThenDeleteThenWrite(t *testing.T) {
	m, ep := newMockFirestoreServer(t)
	a := NewAdapter()
	defer a.Close()
	ctx := hostkit.WithEndpoint(context.Background(), ep)
	if _, e := a.CreateTable(ctx, "orders", false); e != nil {
		t.Fatal(e)
	}
	h := NewHost()
	catPath := h.firestoreDocPath(firestoreCatalogCollection, "orders")
	var injected atomic.Bool
	m.getHook = func(req *fsGetDocumentRequest) (*fsDocument, error) {
		if req.Name == catPath && injected.CompareAndSwap(false, true) {
			m.mu.Lock()
			old := cloneDoc(m.docs[catPath])
			m.mu.Unlock()
			if e := a.DeleteTable(ctx, "orders"); e != nil {
				t.Errorf("delete during stale catalog read: %v", e)
			}
			return &old, nil
		}
		return nil, nil
	}
	_, e := a.PutItem(ctx, "orders", hostkit.Item{PK: "late", Attrs: map[string]hostkit.Attr{}}, hostkit.PutIfAbsent, "")
	m.mu.Lock()
	_, catExists := m.docs[catPath]
	_, childExists := m.docs[h.firestoreDocPath("orders", "late")]
	m.mu.Unlock()
	t.Logf("write resumed after table deletion: err=%v catalog=%v child=%v", e, catExists, childExists)
	if childExists {
		t.Error("catalog pre-read failed to fence concurrent deletion")
	}
}

func TestSolR2PurgeSpendLowerBound(t *testing.T) {
	_, ep := newMockFirestoreServer(t)
	a := NewAdapter()
	defer a.Close()
	t.Setenv("LMGCP_ENDPOINT", ep)
	t.Setenv("LMCLOUD_CAMPAIGN_STATE", filepath.Join(t.TempDir(), "campaign-state.json"))
	h := cloudhost.New(cloudhost.Config{Provider: "gcp", Store: a, Queue: a, Tables: a, Endpoints: Endpoints()})
	h.SetAbortOnError(false)
	if _, e := h.TableCreate(false, "orders", false); e != nil {
		t.Fatal(e)
	}
	item, _ := h.ItemNew("p", "", false)
	item.SetString("n", "v")
	if _, e := h.ItemPut(false, "orders", item, hostkit.PutIfAbsent, ""); e != nil {
		t.Fatal(e)
	}
	if _, e := h.TableDelete(false, "orders"); e != nil {
		t.Fatal(e)
	}
	// Per-operation synthetic tariffs: catalog get 250, tombstone write 1250,
	// one query 250, child delete 1250, catalog delete 1250 => minimum 4250 nanoUSD.
	line := h.CostReport().Item("table_delete")
	t.Logf("purge line=%+v", line)
	if line.EstimatedUSD < 0.00000425 {
		t.Error("purge settled less than admitted physical operations")
	}
}

func TestSolR2StalePurgeDeletesReplacementChild(t *testing.T) {
	lis, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	m := &mockFirestoreServer{docs: map[string]fsDocument{}, nextTick: 1000}
	var enabled, injected atomic.Bool
	h := NewHost()
	catPath := h.firestoreDocPath(firestoreCatalogCollection, "orders")
	childPath := h.firestoreDocPath("orders", "p")
	srv := grpc.NewServer(grpc.ForceServerCodec(rawProtoCodec{}), grpc.UnknownServiceHandler(func(v any, s grpc.ServerStream) error {
		method, _ := grpc.MethodFromServerStream(s)
		err := m.handleStream(v, s)
		if method == rpcRunQuery && enabled.Load() && injected.CompareAndSwap(false, true) {
			m.mu.Lock()
			cat := cloneDoc(m.docs[catPath])
			cat.Fields["state"] = fsString("active")
			cat.Fields["incarnation"] = fsString("replacement-incarnation")
			cat.UpdateTime = fsTimestamp{Seconds: 9999}
			m.docs[catPath] = cat
			child := cloneDoc(m.docs[childPath])
			child.Fields[tableVersionAttr] = fsString("replacement-version")
			child.UpdateTime = fsTimestamp{Seconds: 9999}
			m.docs[childPath] = child
			m.mu.Unlock()
		}
		return err
	}))
	go srv.Serve(lis)
	defer srv.Stop()
	defer lis.Close()
	a := NewAdapter()
	defer a.Close()
	ctx := hostkit.WithEndpoint(context.Background(), "http://"+lis.Addr().String())
	if _, e := a.CreateTable(ctx, "orders", false); e != nil {
		t.Fatal(e)
	}
	if _, e := a.PutItem(ctx, "orders", hostkit.Item{PK: "p", Attrs: map[string]hostkit.Attr{}}, hostkit.PutIfAbsent, ""); e != nil {
		t.Fatal(e)
	}
	enabled.Store(true)
	e = a.DeleteTable(ctx, "orders")
	m.mu.Lock()
	_, survives := m.docs[childPath]
	m.mu.Unlock()
	t.Logf("replacement child after stale purge: survives=%v err=%v", survives, e)
	if !survives {
		t.Error("unconditional child delete removed replacement incarnation's document")
	}
}
