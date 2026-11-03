// SPDX-License-Identifier: MIT

package gcp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lmctl-ai/lmcloud/hostkit"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protowire"
)

func consumeTagHelper(b []byte) (protowire.Number, protowire.Type, int) {
	return protowire.ConsumeTag(b)
}
func consumeBytesHelper(b []byte) ([]byte, int) {
	return protowire.ConsumeBytes(b)
}
func consumeStringHelper(b []byte) (string, int) {
	return protowire.ConsumeString(b)
}
func consumeVarintHelper(b []byte) (uint64, int) {
	return protowire.ConsumeVarint(b)
}
func skipFieldHelper(num protowire.Number, wtype protowire.Type, b []byte) int {
	return protowire.ConsumeFieldValue(num, wtype, b)
}

type mockFirestoreServer struct {
	mu          sync.Mutex
	docs        map[string]fsDocument
	nextTick    int64
	updateHook  func(req *fsUpdateDocumentRequest) error
	getHook     func(req *fsGetDocumentRequest) (*fsDocument, error)
	runQueryRaw [][]byte
}

func newMockFirestoreServer(t *testing.T) (*mockFirestoreServer, string) {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	m := &mockFirestoreServer{
		docs:     make(map[string]fsDocument),
		nextTick: 1000,
	}
	srv := grpc.NewServer(
		grpc.ForceServerCodec(rawProtoCodec{}),
		grpc.MaxRecvMsgSize(32*1024*1024),
		grpc.MaxSendMsgSize(32*1024*1024),
		grpc.UnknownServiceHandler(m.handleStream),
	)
	go func() {
		_ = srv.Serve(lis)
	}()
	t.Cleanup(func() {
		srv.Stop()
		_ = lis.Close()
	})
	return m, "http://" + lis.Addr().String()
}

func (m *mockFirestoreServer) nextTimestampLocked() fsTimestamp {
	m.nextTick++
	return fsTimestamp{Seconds: m.nextTick, Nanos: 0}
}

func cloneDoc(d fsDocument) fsDocument {
	out := fsDocument{
		Name:       d.Name,
		CreateTime: d.CreateTime,
		UpdateTime: d.UpdateTime,
		Fields:     make(map[string]fsValue, len(d.Fields)),
	}
	for k, v := range d.Fields {
		out.Fields[k] = v
	}
	return out
}

func (m *mockFirestoreServer) handleStream(_ any, stream grpc.ServerStream) error {
	method, ok := grpc.MethodFromServerStream(stream)
	if !ok {
		return status.Error(codes.Unimplemented, "missing method")
	}
	var rawReq []byte
	if err := stream.RecvMsg(&rawReq); err != nil {
		return err
	}

	switch method {
	case rpcGetDocument:
		var req fsGetDocumentRequest
		if err := req.unmarshalProto(rawReq); err != nil {
			return status.Error(codes.InvalidArgument, err.Error())
		}
		if m.getHook != nil {
			doc, herr := m.getHook(&req)
			if herr != nil {
				return herr
			}
			if doc != nil {
				return stream.SendMsg(doc)
			}
		}
		m.mu.Lock()
		doc, exists := m.docs[req.Name]
		if exists {
			doc = cloneDoc(doc)
		}
		m.mu.Unlock()
		if !exists {
			return status.Error(codes.NotFound, "document not found: "+req.Name)
		}
		return stream.SendMsg(&doc)

	case rpcCreateDocument:
		var req fsCreateDocumentRequest
		if err := req.unmarshalProto(rawReq); err != nil {
			return status.Error(codes.InvalidArgument, err.Error())
		}
		name := req.Parent + "/" + req.CollectionId + "/" + req.DocumentId
		m.mu.Lock()
		if _, exists := m.docs[name]; exists {
			m.mu.Unlock()
			return status.Error(codes.AlreadyExists, "document already exists: "+name)
		}
		ts := m.nextTimestampLocked()
		stored := cloneDoc(req.Document)
		stored.Name = name
		stored.CreateTime = ts
		stored.UpdateTime = ts
		m.docs[name] = stored
		out := cloneDoc(stored)
		m.mu.Unlock()
		return stream.SendMsg(&out)

	case rpcUpdateDocument:
		var req fsUpdateDocumentRequest
		if err := req.unmarshalProto(rawReq); err != nil {
			return status.Error(codes.InvalidArgument, err.Error())
		}
		if m.updateHook != nil {
			if herr := m.updateHook(&req); herr != nil {
				return herr
			}
		}
		// Re-parse update_mask and current_document from rawReq for full fidelity.
		maskPaths, pre := parseUpdateExtras(rawReq)
		name := req.Document.Name
		m.mu.Lock()
		existing, exists := m.docs[name]
		if pre != nil {
			if pre.HasExists {
				if pre.Exists && !exists {
					m.mu.Unlock()
					return status.Error(codes.NotFound, "no document to update: "+name)
				}
				if !pre.Exists && exists {
					m.mu.Unlock()
					return status.Error(codes.AlreadyExists, "document already exists: "+name)
				}
			} else if pre.HasUpdateTime {
				if !exists || existing.UpdateTime != pre.UpdateTime {
					m.mu.Unlock()
					return status.Error(codes.FailedPrecondition, "update_time precondition failed")
				}
			}
		}
		ts := m.nextTimestampLocked()
		var nextDoc fsDocument
		if len(maskPaths) > 0 && exists {
			nextDoc = cloneDoc(existing)
			for _, p := range maskPaths {
				if v, ok := req.Document.Fields[p]; ok {
					nextDoc.Fields[p] = v
				} else {
					delete(nextDoc.Fields, p)
				}
			}
			nextDoc.UpdateTime = ts
		} else {
			nextDoc = cloneDoc(req.Document)
			if exists {
				nextDoc.CreateTime = existing.CreateTime
			} else {
				nextDoc.CreateTime = ts
			}
			nextDoc.UpdateTime = ts
		}
		m.docs[name] = nextDoc
		out := cloneDoc(nextDoc)
		m.mu.Unlock()
		return stream.SendMsg(&out)

	case rpcDeleteDocument:
		var req fsDeleteDocumentRequest
		if err := req.unmarshalProto(rawReq); err != nil {
			return status.Error(codes.InvalidArgument, err.Error())
		}
		pre := parseDeletePrecondition(rawReq)
		m.mu.Lock()
		_, exists := m.docs[req.Name]
		if pre != nil && pre.HasExists && pre.Exists && !exists {
			m.mu.Unlock()
			return status.Error(codes.NotFound, "no document to delete: "+req.Name)
		}
		delete(m.docs, req.Name)
		m.mu.Unlock()
		return stream.SendMsg(&fsEmpty{})

	case rpcListDocuments:
		var req fsListDocumentsRequest
		if err := req.unmarshalProto(rawReq); err != nil {
			return status.Error(codes.InvalidArgument, err.Error())
		}
		prefix := req.Parent + "/" + req.CollectionId + "/"
		m.mu.Lock()
		var resp fsListDocumentsResponse
		for name, doc := range m.docs {
			if strings.HasPrefix(name, prefix) && !strings.Contains(strings.TrimPrefix(name, prefix), "/") {
				resp.Documents = append(resp.Documents, cloneDoc(doc))
			}
		}
		m.mu.Unlock()
		return stream.SendMsg(&resp)

	case rpcRunQuery:
		m.mu.Lock()
		rawChunks := m.runQueryRaw
		m.mu.Unlock()
		if len(rawChunks) > 0 {
			for _, ch := range rawChunks {
				c := ch
				if err := stream.SendMsg(&c); err != nil {
					return err
				}
			}
			return nil
		}
		parent, col := parseRunQueryCollection(rawReq)
		prefix := parent + "/" + col + "/"
		m.mu.Lock()
		var docs []fsDocument
		for name, doc := range m.docs {
			if strings.HasPrefix(name, prefix) && !strings.Contains(strings.TrimPrefix(name, prefix), "/") {
				docs = append(docs, cloneDoc(doc))
			}
		}
		m.mu.Unlock()
		for i := range docs {
			d := docs[i]
			resp := fsRunQueryResponse{Document: &d}
			if err := stream.SendMsg(&resp); err != nil {
				return err
			}
		}
		return nil

	default:
		return status.Error(codes.Unimplemented, "unknown method: "+method)
	}
}

func parseUpdateExtras(b []byte) ([]string, *fsPrecondition) {
	var mask []string
	var pre *fsPrecondition
	for len(b) > 0 {
		num, wtype, n := consumeTagHelper(b)
		if n < 0 {
			break
		}
		b = b[n:]
		switch num {
		case 2:
			mb, m := consumeBytesHelper(b)
			if m < 0 {
				return mask, pre
			}
			b = b[m:]
			for len(mb) > 0 {
				fnum, _, fn := consumeTagHelper(mb)
				if fn < 0 {
					break
				}
				mb = mb[fn:]
				if fnum == 1 {
					s, fm := consumeStringHelper(mb)
					if fm < 0 {
						break
					}
					mask = append(mask, s)
					mb = mb[fm:]
				}
			}
		case 4:
			pb, m := consumeBytesHelper(b)
			if m < 0 {
				return mask, pre
			}
			b = b[m:]
			pre = parsePreconditionBytes(pb)
		default:
			m := skipFieldHelper(num, wtype, b)
			if m < 0 {
				return mask, pre
			}
			b = b[m:]
		}
	}
	return mask, pre
}

func parseDeletePrecondition(b []byte) *fsPrecondition {
	var pre *fsPrecondition
	for len(b) > 0 {
		num, wtype, n := consumeTagHelper(b)
		if n < 0 {
			break
		}
		b = b[n:]
		if num == 2 {
			pb, m := consumeBytesHelper(b)
			if m < 0 {
				return pre
			}
			b = b[m:]
			pre = parsePreconditionBytes(pb)
		} else {
			m := skipFieldHelper(num, wtype, b)
			if m < 0 {
				return pre
			}
			b = b[m:]
		}
	}
	return pre
}

func parsePreconditionBytes(pb []byte) *fsPrecondition {
	pre := &fsPrecondition{}
	for len(pb) > 0 {
		num, wtype, n := consumeTagHelper(pb)
		if n < 0 {
			break
		}
		pb = pb[n:]
		switch num {
		case 1:
			v, m := consumeVarintHelper(pb)
			if m < 0 {
				return pre
			}
			pre.HasExists = true
			pre.Exists = v != 0
			pb = pb[m:]
		case 2:
			tb, m := consumeBytesHelper(pb)
			if m < 0 {
				return pre
			}
			pre.HasUpdateTime = true
			_ = pre.UpdateTime.unmarshalProto(tb)
			pb = pb[m:]
		default:
			m := skipFieldHelper(num, wtype, pb)
			if m < 0 {
				return pre
			}
			pb = pb[m:]
		}
	}
	return pre
}

func parseRunQueryCollection(b []byte) (string, string) {
	var parent, col string
	for len(b) > 0 {
		num, wtype, n := consumeTagHelper(b)
		if n < 0 {
			break
		}
		b = b[n:]
		switch num {
		case 1:
			s, m := consumeStringHelper(b)
			if m < 0 {
				return parent, col
			}
			parent = s
			b = b[m:]
		case 2:
			qb, m := consumeBytesHelper(b)
			if m < 0 {
				return parent, col
			}
			b = b[m:]
			for len(qb) > 0 {
				qnum, qwtype, qn := consumeTagHelper(qb)
				if qn < 0 {
					break
				}
				qb = qb[qn:]
				if qnum == 2 {
					fb, fm := consumeBytesHelper(qb)
					if fm < 0 {
						break
					}
					qb = qb[fm:]
					for len(fb) > 0 {
						fnum, fwtype, fn := consumeTagHelper(fb)
						if fn < 0 {
							break
						}
						fb = fb[fn:]
						if fnum == 2 {
							cs, cm := consumeStringHelper(fb)
							if cm < 0 {
								break
							}
							col = cs
							fb = fb[cm:]
						} else {
							cm := skipFieldHelper(fnum, fwtype, fb)
							if cm < 0 {
								break
							}
							fb = fb[cm:]
						}
					}
				} else {
					qm := skipFieldHelper(qnum, qwtype, qb)
					if qm < 0 {
						break
					}
					qb = qb[qm:]
				}
			}
		default:
			m := skipFieldHelper(num, wtype, b)
			if m < 0 {
				return parent, col
			}
			b = b[m:]
		}
	}
	return parent, col
}

func TestSolAdmissionDenialPropagated(t *testing.T) {
	_, ep := newMockFirestoreServer(t)
	a := NewAdapter()
	baseCtx := hostkit.WithEndpoint(context.Background(), ep)

	denyErr := hostkit.ProviderError(hostkit.CostCapExceeded, 0, "CostCapExceeded", "simulated budget denial")
	deniedCtx := hostkit.WithRetryHooks(baseCtx, hostkit.RetryHooks{
		BeforeAttempt: func(context.Context, int) error {
			return denyErr
		},
	})

	cases := []struct {
		name string
		fn   func() error
	}{
		{"CreateTable", func() error { _, err := a.CreateTable(deniedCtx, "t1", false); return err }},
		{"HeadTable", func() error { return a.HeadTable(deniedCtx, "t1") }},
		{"DescribeTable", func() error { _, err := a.DescribeTable(deniedCtx, "t1"); return err }},
		{"TableCreateToken", func() error { _, err := a.TableCreateToken(deniedCtx, "t1"); return err }},
		{"ListTables", func() error { _, _, err := a.ListTables(deniedCtx, ""); return err }},
		{"DeleteTable", func() error { return a.DeleteTable(deniedCtx, "t1") }},
		{"PutItem", func() error {
			_, err := a.PutItem(deniedCtx, "t1", hostkit.Item{PK: "p1", Attrs: map[string]hostkit.Attr{"a": hostkit.StringAttr("v")}}, hostkit.PutUnconditional, "")
			return err
		}},
		{"GetItem", func() error { _, err := a.GetItem(deniedCtx, "t1", "p1", ""); return err }},
		{"DeleteItem", func() error { return a.DeleteItem(deniedCtx, "t1", "p1", "") }},
		{"UpdateItem", func() error {
			_, err := a.UpdateItem(deniedCtx, "t1", "p1", "", hostkit.Item{PK: "p1", Attrs: map[string]hostkit.Attr{"a": hostkit.StringAttr("v2")}})
			return err
		}},
		{"Query", func() error { _, err := a.Query(deniedCtx, "t1", "p1", "", "", 10, ""); return err }},
		{"Scan", func() error { _, err := a.Scan(deniedCtx, "t1", 10, ""); return err }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.fn()
			if err == nil {
				t.Fatalf("expected COST_CAP_EXCEEDED error, got nil")
			}
			var ce *hostkit.CloudError
			if !errors.As(err, &ce) || ce.Code != hostkit.CostCapExceeded {
				t.Fatalf("expected COST_CAP_EXCEEDED, got %v", err)
			}
		})
	}
}

func TestSolMultiStepPhysicalAdmission(t *testing.T) {
	_, ep := newMockFirestoreServer(t)
	a := NewAdapter()
	baseCtx := hostkit.WithEndpoint(context.Background(), ep)

	if _, err := a.CreateTable(baseCtx, "t_multi", false); err != nil {
		t.Fatalf("CreateTable: %v", err)
	}
	it, err := a.PutItem(baseCtx, "t_multi", hostkit.Item{
		PK:    "k1",
		Attrs: map[string]hostkit.Attr{"x": hostkit.StringAttr("1")},
	}, hostkit.PutIfAbsent, "")
	if err != nil {
		t.Fatalf("PutItem: %v", err)
	}

	// PutIfVersion performs 2 physical calls (GetDocument + UpdateDocument).
	// Denying on call #2 must stop before UpdateDocument and propagate COST_CAP_EXCEEDED.
	var calls int32
	denyOnSecondCtx := hostkit.WithRetryHooks(baseCtx, hostkit.RetryHooks{
		BeforeAttempt: func(context.Context, int) error {
			if atomic.AddInt32(&calls, 1) >= 2 {
				return hostkit.ProviderError(hostkit.CostCapExceeded, 0, "CostCapExceeded", "second physical call denied")
			}
			return nil
		},
	})
	_, err = a.PutItem(denyOnSecondCtx, "t_multi", hostkit.Item{
		PK:    "k1",
		Attrs: map[string]hostkit.Attr{"x": hostkit.StringAttr("2")},
	}, hostkit.PutIfVersion, it.Version)
	var ce *hostkit.CloudError
	if !errors.As(err, &ce) || ce.Code != hostkit.CostCapExceeded {
		t.Fatalf("expected COST_CAP_EXCEEDED on 2nd physical call, got %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("expected 2 BeforeAttempt calls, got %d", got)
	}

	// Verify the stored item was NOT mutated when the 2nd physical call was denied.
	cur, err := a.GetItem(baseCtx, "t_multi", "k1", "")
	if err != nil {
		t.Fatalf("GetItem: %v", err)
	}
	if cur.Version != it.Version || *cur.Attrs["x"].S != "1" {
		t.Fatalf("stored item mutated despite denied write: %+v", cur)
	}
}

func TestSolIncompatibleTableSchemaRejected(t *testing.T) {
	m, ep := newMockFirestoreServer(t)
	a := NewAdapter()
	ctx := hostkit.WithEndpoint(context.Background(), ep)

	// Inject a corrupt catalog document missing "layout".
	m.mu.Lock()
	m.docs["projects/gcpsim-local/databases/(default)/documents/lmcloudmeta/bad_layout"] = fsDocument{
		Name: "projects/gcpsim-local/databases/(default)/documents/lmcloudmeta/bad_layout",
		Fields: map[string]fsValue{
			"scope":       fsString("gcpsim-local"),
			"incarnation": fsString("inc1"),
			"state":       fsString("active"),
		},
	}
	// Inject a foreign-scope catalog document.
	m.docs["projects/gcpsim-local/databases/(default)/documents/lmcloudmeta/foreign_scope"] = fsDocument{
		Name: "projects/gcpsim-local/databases/(default)/documents/lmcloudmeta/foreign_scope",
		Fields: map[string]fsValue{
			"layout":      fsString("key"),
			"scope":       fsString("other-project"),
			"incarnation": fsString("inc2"),
			"state":       fsString("active"),
		},
	}
	m.mu.Unlock()

	for _, tbl := range []string{"bad_layout", "foreign_scope"} {
		if err := a.HeadTable(ctx, tbl); err == nil {
			t.Fatalf("HeadTable(%s): expected CONFLICT, got nil", tbl)
		} else if ce := hostkit.AsCloudError(err); ce.Code != hostkit.Conflict {
			t.Fatalf("HeadTable(%s): expected CONFLICT, got %v", tbl, err)
		}
		if _, err := a.DescribeTable(ctx, tbl); err == nil {
			t.Fatalf("DescribeTable(%s): expected CONFLICT, got nil", tbl)
		} else if ce := hostkit.AsCloudError(err); ce.Code != hostkit.Conflict {
			t.Fatalf("DescribeTable(%s): expected CONFLICT, got %v", tbl, err)
		}
	}
}

func TestSolDescribeTableMissingIncarnationTokenFailsClosed(t *testing.T) {
	m, ep := newMockFirestoreServer(t)
	a := NewAdapter()
	ctx := hostkit.WithEndpoint(context.Background(), ep)

	// Catalog doc with valid layout and scope, but no incarnation field and zero CreateTime.
	m.mu.Lock()
	m.docs["projects/gcpsim-local/databases/(default)/documents/lmcloudmeta/no_inc"] = fsDocument{
		Name: "projects/gcpsim-local/databases/(default)/documents/lmcloudmeta/no_inc",
		Fields: map[string]fsValue{
			"layout": fsString("key"),
			"scope":  fsString("gcpsim-local"),
			"state":  fsString("active"),
		},
	}
	m.mu.Unlock()

	_, err := a.DescribeTable(ctx, "no_inc")
	if err == nil {
		t.Fatalf("expected INTERNAL error when incarnation token is missing, got nil")
	}
	if ce := hostkit.AsCloudError(err); ce.Code != hostkit.Internal {
		t.Fatalf("expected INTERNAL error, got %v", err)
	}
}

func TestSolOversizedFirestoreResponseRejected(t *testing.T) {
	m, ep := newMockFirestoreServer(t)
	a := NewAdapter()
	ctx := hostkit.WithEndpoint(context.Background(), ep)

	if _, err := a.CreateTable(ctx, "t_big", false); err != nil {
		t.Fatalf("CreateTable: %v", err)
	}
	bigStr := strings.Repeat("x", maxFirestoreRecvBytes+1024)
	m.getHook = func(req *fsGetDocumentRequest) (*fsDocument, error) {
		return &fsDocument{
			Name: req.Name,
			Fields: map[string]fsValue{
				"pk":          fsString("k1"),
				"__lmcloud_v": fsString("v1"),
				"huge":        fsString(bigStr),
			},
		}, nil
	}

	_, err := a.GetItem(ctx, "t_big", "k1", "")
	if err == nil {
		t.Fatalf("expected oversized response error, got nil")
	}
	if ce := hostkit.AsCloudError(err); ce.Code != hostkit.Internal {
		t.Fatalf("expected INTERNAL error on oversized gRPC response, got %v", err)
	}
}

func TestSolUpdateItemMergedLimitsAndCAS(t *testing.T) {
	m, ep := newMockFirestoreServer(t)
	a := NewAdapter()
	ctx := hostkit.WithEndpoint(context.Background(), ep)

	if _, err := a.CreateTable(ctx, "t_upd", false); err != nil {
		t.Fatalf("CreateTable: %v", err)
	}

	// 1. Merged > 240 attributes rejected before writing.
	initialAttrs := make(map[string]hostkit.Attr, 240)
	for i := 0; i < 240; i++ {
		initialAttrs[fmt.Sprintf("a_%03d", i)] = hostkit.IntAttr(int64(i))
	}
	putRes, err := a.PutItem(ctx, "t_upd", hostkit.Item{
		PK:    "p_240",
		Attrs: initialAttrs,
	}, hostkit.PutIfAbsent, "")
	if err != nil {
		t.Fatalf("PutItem 240 attrs: %v", err)
	}

	_, err = a.UpdateItem(ctx, "t_upd", "p_240", "", hostkit.Item{
		PK:    "p_240",
		Attrs: map[string]hostkit.Attr{"extra_241": hostkit.IntAttr(1)},
	})
	if err == nil {
		t.Fatalf("expected UpdateItem > 240 merged attrs to fail")
	}
	if ce := hostkit.AsCloudError(err); ce.Code != hostkit.InvalidArgument {
		t.Fatalf("expected INVALID_ARGUMENT, got %v", err)
	}

	// Verify item and version were NOT mutated.
	after240, err := a.GetItem(ctx, "t_upd", "p_240", "")
	if err != nil {
		t.Fatalf("GetItem p_240: %v", err)
	}
	if after240.Version != putRes.Version || len(after240.Attrs) != 240 {
		t.Fatalf("p_240 was mutated after rejected update: version=%s len=%d", after240.Version, len(after240.Attrs))
	}

	// 2. Merged > 262,144 canonical bytes rejected before writing.
	// 7 strings of 32,768 bytes each = 229,376 value bytes (< 262,144).
	chunk := strings.Repeat("a", 32768)
	byteAttrs := make(map[string]hostkit.Attr, 7)
	for i := 0; i < 7; i++ {
		byteAttrs[fmt.Sprintf("s_%d", i)] = hostkit.StringAttr(chunk)
	}
	putBig, err := a.PutItem(ctx, "t_upd", hostkit.Item{
		PK:    "p_bytes",
		Attrs: byteAttrs,
	}, hostkit.PutIfAbsent, "")
	if err != nil {
		t.Fatalf("PutItem 7x32KiB: %v", err)
	}

	// Adding an 8th 32,768-byte string pushes canonical bytes > 262,144.
	_, err = a.UpdateItem(ctx, "t_upd", "p_bytes", "", hostkit.Item{
		PK:    "p_bytes",
		Attrs: map[string]hostkit.Attr{"s_7": hostkit.StringAttr(chunk)},
	})
	if err == nil {
		t.Fatalf("expected UpdateItem > 262,144 canonical bytes to fail")
	}
	if ce := hostkit.AsCloudError(err); ce.Code != hostkit.InvalidArgument {
		t.Fatalf("expected INVALID_ARGUMENT, got %v", err)
	}
	afterBig, err := a.GetItem(ctx, "t_upd", "p_bytes", "")
	if err != nil {
		t.Fatalf("GetItem p_bytes: %v", err)
	}
	if afterBig.Version != putBig.Version || len(afterBig.Attrs) != 7 {
		t.Fatalf("p_bytes was mutated after rejected update: version=%s len=%d", afterBig.Version, len(afterBig.Attrs))
	}

	// 3. CAS conflict on first UpdateDocument attempt retries and succeeds.
	var casInjected int32
	m.updateHook = func(req *fsUpdateDocumentRequest) error {
		if atomic.CompareAndSwapInt32(&casInjected, 0, 1) {
			return status.Error(codes.FailedPrecondition, "simulated concurrent write")
		}
		return nil
	}
	defer func() { m.updateHook = nil }()

	updated, err := a.UpdateItem(ctx, "t_upd", "p_240", "", hostkit.Item{
		PK:    "p_240",
		Attrs: map[string]hostkit.Attr{"a_000": hostkit.IntAttr(999)},
	})
	if err != nil {
		t.Fatalf("UpdateItem with CAS retry failed: %v", err)
	}
	if updated.Version == putRes.Version || *updated.Attrs["a_000"].I != 999 {
		t.Fatalf("unexpected updated item after CAS retry: %+v", updated)
	}
}

func TestSolUpdateItemShrinkRetainsMaxReservation(t *testing.T) {
	_, ep := newMockFirestoreServer(t)
	a := NewAdapter()
	ctx := hostkit.WithEndpoint(context.Background(), ep)

	if _, err := a.CreateTable(ctx, "t_shrink", false); err != nil {
		t.Fatalf("CreateTable: %v", err)
	}
	if _, err := a.PutItem(ctx, "t_shrink", hostkit.Item{
		PK:    "k1",
		Attrs: map[string]hostkit.Attr{"payload": hostkit.StringAttr(strings.Repeat("z", 16384))},
	}, hostkit.PutIfAbsent, ""); err != nil {
		t.Fatalf("PutItem: %v", err)
	}

	var sink hostkit.TableOpMetrics
	mctx := hostkit.WithTableOpMetrics(ctx, &sink)
	if _, err := a.UpdateItem(mctx, "t_shrink", "k1", "", hostkit.Item{
		PK:    "k1",
		Attrs: map[string]hostkit.Attr{"payload": hostkit.StringAttr("tiny")},
	}); err != nil {
		t.Fatalf("UpdateItem: %v", err)
	}
	if sink.Authoritative {
		t.Fatalf("UpdateItem must report Authoritative=false so cloudhost retains its conservative max reservation")
	}
}

func TestSolLiveTablesFailClosed(t *testing.T) {
	var called int32
	m, ep := newMockFirestoreServer(t)
	m.getHook = func(*fsGetDocumentRequest) (*fsDocument, error) {
		atomic.AddInt32(&called, 1)
		return nil, nil
	}

	for _, envName := range []string{"LMGCP_LIVE", "LMCLOUD_LIVE"} {
		t.Run(envName, func(t *testing.T) {
			t.Setenv(envName, "1")
			a := NewAdapter()
			ctx := hostkit.WithEndpoint(context.Background(), ep)
			if err := a.HeadTable(ctx, "any_table"); err == nil {
				t.Fatalf("expected live mode to fail closed, got nil")
			} else if ce := hostkit.AsCloudError(err); ce.Code != hostkit.Unauthenticated && ce.Code != hostkit.PermissionDenied {
				t.Fatalf("expected UNAUTHENTICATED or PERMISSION_DENIED, got %v", err)
			}
			if atomic.LoadInt32(&called) != 0 {
				t.Fatalf("expected 0 network calls in live mode, got %d", atomic.LoadInt32(&called))
			}
		})
	}
	_ = time.Now()
}
