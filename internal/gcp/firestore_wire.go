// SPDX-License-Identifier: MIT

package gcp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/lmctl-ai/lmcloud/hostkit"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protowire"
)

const (
	// maxFirestoreRecvBytes bounds any single Firestore gRPC message to 8 MiB.
	maxFirestoreRecvBytes = 8 * 1024 * 1024

	firestoreServicePrefix = "/google.firestore.v1.Firestore/"
	rpcGetDocument         = firestoreServicePrefix + "GetDocument"
	rpcCreateDocument      = firestoreServicePrefix + "CreateDocument"
	rpcUpdateDocument      = firestoreServicePrefix + "UpdateDocument"
	rpcDeleteDocument      = firestoreServicePrefix + "DeleteDocument"
	rpcListDocuments       = firestoreServicePrefix + "ListDocuments"
	rpcRunQuery            = firestoreServicePrefix + "RunQuery"
	rpcCommit              = firestoreServicePrefix + "Commit"
)

// protoMsg is implemented by minimal hand-rolled Firestore wire structs.
type protoMsg interface {
	marshalProto() ([]byte, error)
	unmarshalProto([]byte) error
}

// rawProtoCodec implements encoding.Codec for protoMsg values without requiring
// generated descriptor registries.
type rawProtoCodec struct{}

func (rawProtoCodec) Name() string { return "proto" }

func (rawProtoCodec) Marshal(v any) ([]byte, error) {
	if m, ok := v.(protoMsg); ok {
		return m.marshalProto()
	}
	if b, ok := v.(*[]byte); ok && b != nil {
		return *b, nil
	}
	return nil, fmt.Errorf("firestore wire codec: unsupported marshal type %T", v)
}

func (rawProtoCodec) Unmarshal(data []byte, v any) error {
	if m, ok := v.(protoMsg); ok {
		return m.unmarshalProto(data)
	}
	if b, ok := v.(*[]byte); ok && b != nil {
		*b = append((*b)[:0], data...)
		return nil
	}
	return fmt.Errorf("firestore wire codec: unsupported unmarshal type %T", v)
}

const maxFirestorePoolConns = 16

type firestorePool struct {
	mu    sync.Mutex
	conns map[string]*grpc.ClientConn
}

func newFirestorePool() *firestorePool {
	return &firestorePool{conns: make(map[string]*grpc.ClientConn)}
}

func (p *firestorePool) conn(target string) (*grpc.ClientConn, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.conns == nil {
		p.conns = make(map[string]*grpc.ClientConn)
	}
	if c, ok := p.conns[target]; ok {
		return c, nil
	}
	if len(p.conns) >= maxFirestorePoolConns {
		for k, old := range p.conns {
			_ = old.Close()
			delete(p.conns, k)
			break
		}
	}
	c, err := grpc.NewClient(
		"passthrough:///"+target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDisableRetry(),
		grpc.WithDefaultCallOptions(
			grpc.ForceCodec(rawProtoCodec{}),
			grpc.MaxCallRecvMsgSize(maxFirestoreRecvBytes),
			grpc.MaxCallSendMsgSize(maxFirestoreRecvBytes),
		),
	)
	if err != nil {
		return nil, err
	}
	p.conns[target] = c
	return c, nil
}

func (p *firestorePool) Close() error {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	var firstErr error
	for k, c := range p.conns {
		if err := c.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		delete(p.conns, k)
	}
	return firstErr
}

func (h *Host) firestoreTarget() (string, error) {
	ep, err := h.Endpoint("tables")
	if err != nil || ep == "" {
		return "", hostkit.ProviderError(hostkit.InvalidArgument, 0, "InvalidEndpoint", "missing tables endpoint")
	}
	u, err := url.Parse(ep)
	if err != nil || u.Host == "" {
		return "", hostkit.ProviderError(hostkit.InvalidArgument, 0, "InvalidEndpoint", "invalid tables endpoint")
	}
	if u.Scheme != "" && !strings.EqualFold(u.Scheme, "http") {
		return "", hostkit.ProviderError(hostkit.InvalidArgument, 0, "InvalidEndpoint", "loopback Firestore emulator endpoint must use http scheme")
	}
	if !hostkit.IsLoopbackHost(u.Hostname()) {
		return "", hostkit.ProviderError(hostkit.Unauthenticated, 0, "LiveCredentialsNotImplemented", "live GCP credentials are not implemented; use a loopback emulator")
	}
	return u.Host, nil
}

func (h *Host) firestoreDatabaseRoot() string {
	project := h.project
	if project == "" {
		project = "gcpsim-local"
	}
	return "projects/" + project + "/databases/(default)"
}

func (h *Host) firestoreDocumentsRoot() string {
	return h.firestoreDatabaseRoot() + "/documents"
}

func (h *Host) firestoreCollectionPath(collection string) string {
	return h.firestoreDocumentsRoot() + "/" + collection
}

func (h *Host) firestoreDocPath(collection, docID string) string {
	return h.firestoreDocumentsRoot() + "/" + collection + "/" + docID
}

func httpStatusForGRPCCode(c codes.Code) int {
	switch c {
	case codes.OK:
		return 200
	case codes.InvalidArgument, codes.OutOfRange:
		return 400
	case codes.Unauthenticated:
		return 401
	case codes.PermissionDenied:
		return 403
	case codes.NotFound:
		return 404
	case codes.AlreadyExists, codes.Aborted:
		return 409
	case codes.FailedPrecondition:
		return 412
	case codes.ResourceExhausted:
		return 429
	case codes.Unavailable:
		return 503
	case codes.DeadlineExceeded:
		return 504
	default:
		return 500
	}
}

func isGRPCNotSent(err error) bool {
	return err != nil && hostkit.RequestNotSent(err)
}

func httpStatusFromGRPCMessage(msg string) int {
	switch {
	case strings.Contains(msg, "status code received from server: 401"),
		strings.Contains(msg, "HTTP status code 401"),
		strings.Contains(msg, `"code":401`),
		strings.Contains(msg, `"status":"UNAUTHENTICATED"`):
		return 401
	case strings.Contains(msg, "status code received from server: 403"),
		strings.Contains(msg, "HTTP status code 403"),
		strings.Contains(msg, `"code":403`),
		strings.Contains(msg, `"status":"PERMISSION_DENIED"`):
		return 403
	case strings.Contains(msg, "status code received from server: 400"),
		strings.Contains(msg, "HTTP status code 400"):
		return 400
	case strings.Contains(msg, "status code received from server: 404"),
		strings.Contains(msg, "HTTP status code 404"):
		return 404
	default:
		return 0
	}
}

func (h *Host) classifyFirestoreError(err error) *hostkit.CloudError {
	if err == nil {
		return nil
	}
	var ce *hostkit.CloudError
	if errors.As(err, &ce) {
		return ce
	}
	st, ok := status.FromError(err)
	if !ok {
		return hostkit.ProviderError(hostkit.Internal, 0, "transport", h.redactSecrets(err.Error()))
	}
	code := st.Code()
	msg := h.redactSecrets(st.Message())
	switch httpStatusFromGRPCMessage(msg) {
	case 401:
		return hostkit.ProviderError(hostkit.Unauthenticated, 401, "UNAUTHENTICATED", msg)
	case 403:
		return hostkit.ProviderError(hostkit.PermissionDenied, 403, "PERMISSION_DENIED", msg)
	case 400:
		return hostkit.ProviderError(hostkit.InvalidArgument, 400, "INVALID_ARGUMENT", msg)
	case 404:
		return hostkit.ProviderError(hostkit.NotFound, 404, "NOT_FOUND", msg)
	}
	switch code {
	case codes.InvalidArgument, codes.OutOfRange:
		return hostkit.ProviderError(hostkit.InvalidArgument, 400, code.String(), msg)
	case codes.Unauthenticated:
		return hostkit.ProviderError(hostkit.Unauthenticated, 401, code.String(), msg)
	case codes.PermissionDenied:
		return hostkit.ProviderError(hostkit.PermissionDenied, 403, code.String(), msg)
	case codes.NotFound:
		return hostkit.ProviderError(hostkit.NotFound, 404, code.String(), msg)
	case codes.AlreadyExists, codes.Aborted:
		return hostkit.ProviderError(hostkit.Conflict, 409, code.String(), msg)
	case codes.FailedPrecondition:
		return hostkit.ProviderError(hostkit.PreconditionFailed, 412, code.String(), msg)
	case codes.ResourceExhausted:
		lower := strings.ToLower(msg)
		if strings.Contains(lower, "received message larger than max") || strings.Contains(lower, "sent message larger than max") || strings.Contains(lower, "message larger than max") {
			return hostkit.ProviderError(hostkit.Internal, 0, "ResponseTooLarge", msg)
		}
		return hostkit.ProviderError(hostkit.Internal, 429, code.String(), msg)
	case codes.Unavailable:
		return hostkit.ProviderError(hostkit.Internal, 503, code.String(), msg)
	case codes.DeadlineExceeded, codes.Canceled:
		return hostkit.ProviderError(hostkit.Internal, 0, code.String(), msg)
	default:
		return hostkit.ProviderError(hostkit.Internal, 500, code.String(), msg)
	}
}

func (h *Host) doGCP(ctx context.Context, idempotent bool, fn func(callCtx context.Context, conn *grpc.ClientConn) error) error {
	if ctx == nil {
		ctx = h.ctx
	}
	if ctx == nil {
		ctx = context.Background()
	}
	pool := h.grpcPool
	if pool == nil {
		pool = newFirestorePool()
		h.grpcPool = pool
	}

	var lastCallErr error
	last, _ := h.retry.Do(ctx, idempotent, func(attemptCtx context.Context, _ int) hostkit.Attempt {
		target, terr := h.firestoreTarget()
		if terr != nil {
			lastCallErr = terr
			return hostkit.Attempt{Err: terr, NotSent: true}
		}
		conn, cerr := pool.conn(target)
		if cerr != nil {
			werr := hostkit.ProviderError(hostkit.Internal, 0, "transport", h.redactSecrets(cerr.Error()))
			lastCallErr = werr
			return hostkit.Attempt{Err: werr, NotSent: true}
		}
		callCtx := attemptCtx
		if _, hasDeadline := callCtx.Deadline(); !hasDeadline {
			var cancel context.CancelFunc
			callCtx, cancel = context.WithTimeout(callCtx, 30*time.Second)
			defer cancel()
		}
		if h.authToken != "" {
			callCtx = metadata.AppendToOutgoingContext(callCtx, "authorization", "Bearer "+h.authToken)
		}
		err := fn(callCtx, conn)
		lastCallErr = err
		if err == nil {
			return hostkit.Attempt{Status: 200}
		}
		var ce *hostkit.CloudError
		if errors.As(err, &ce) {
			st := ce.HTTPStatus
			if st == 0 {
				st = 400
			}
			return hostkit.Attempt{Status: st}
		}
		if st, ok := status.FromError(err); ok {
			if hst := httpStatusFromGRPCMessage(st.Message()); hst != 0 {
				return hostkit.Attempt{Status: hst}
			}
			c := st.Code()
			if c == codes.ResourceExhausted {
				lower := strings.ToLower(st.Message())
				if strings.Contains(lower, "message larger than max") {
					return hostkit.Attempt{Status: 400}
				}
				return hostkit.Attempt{Status: 429}
			}
			if c == codes.Unavailable && isGRPCNotSent(err) {
				return hostkit.Attempt{Err: err, NotSent: true}
			}
			return hostkit.Attempt{Status: httpStatusForGRPCCode(c)}
		}
		return hostkit.Attempt{Err: err, NotSent: hostkit.RequestNotSent(err)}
	})

	if last.Err != nil {
		var ce *hostkit.CloudError
		if errors.As(last.Err, &ce) {
			return ce
		}
		return h.classifyFirestoreError(last.Err)
	}
	if lastCallErr != nil {
		return h.classifyFirestoreError(lastCallErr)
	}
	return nil
}

func (h *Host) firestoreUnary(ctx context.Context, method string, idempotent bool, req, resp protoMsg) error {
	return h.doGCP(ctx, idempotent, func(callCtx context.Context, conn *grpc.ClientConn) error {
		return conn.Invoke(callCtx, method, req, resp)
	})
}

func (h *Host) firestoreRunQuery(ctx context.Context, req *fsRunQueryRequest, maxDocs int) ([]fsDocument, error) {
	var out []fsDocument
	err := h.doGCP(ctx, true, func(callCtx context.Context, conn *grpc.ClientConn) error {
		out = nil
		desc := &grpc.StreamDesc{
			StreamName:    "RunQuery",
			ServerStreams: true,
		}
		stream, err := conn.NewStream(callCtx, desc, rpcRunQuery)
		if err != nil {
			return err
		}
		if err := stream.SendMsg(req); err != nil {
			return err
		}
		if err := stream.CloseSend(); err != nil {
			return err
		}
		var totalBytes int64
		for {
			var chunk []byte
			if err := stream.RecvMsg(&chunk); err != nil {
				if errors.Is(err, io.EOF) {
					return nil
				}
				return err
			}
			totalBytes += int64(len(chunk))
			if totalBytes > maxFirestoreRecvBytes {
				return hostkit.ProviderError(hostkit.Internal, 0, "ResponseTooLarge", "Firestore RunQuery stream exceeded response byte limit")
			}
			var msg fsRunQueryResponse
			if err := msg.unmarshalProto(chunk); err != nil {
				return hostkit.ProviderError(hostkit.Internal, 0, "DecodeError", err.Error())
			}
			if msg.Document != nil && msg.Document.Name != "" {
				out = append(out, *msg.Document)
				if maxDocs > 0 && len(out) > maxDocs {
					return hostkit.ProviderError(hostkit.Internal, 0, "ResponseTooLarge", "Firestore RunQuery returned more documents than requested limit")
				}
			}
		}
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// --- Minimal protobuf wire messages for google.firestore.v1 ---

type fsTimestamp struct {
	Seconds int64
	Nanos   int32
}

func (t fsTimestamp) IsZero() bool {
	return t.Seconds == 0 && t.Nanos == 0
}

func (t fsTimestamp) marshalProto() ([]byte, error) {
	var b []byte
	if t.Seconds != 0 {
		b = protowire.AppendTag(b, 1, protowire.VarintType)
		b = protowire.AppendVarint(b, uint64(t.Seconds))
	}
	if t.Nanos != 0 {
		b = protowire.AppendTag(b, 2, protowire.VarintType)
		b = protowire.AppendVarint(b, uint64(uint32(t.Nanos)))
	}
	return b, nil
}

func (t *fsTimestamp) unmarshalProto(b []byte) error {
	*t = fsTimestamp{}
	for len(b) > 0 {
		num, wtype, n := protowire.ConsumeTag(b)
		if n < 0 {
			return protowire.ParseError(n)
		}
		b = b[n:]
		switch num {
		case 1:
			if wtype != protowire.VarintType {
				return fmt.Errorf("timestamp.seconds unexpected wire type %d", wtype)
			}
			v, m := protowire.ConsumeVarint(b)
			if m < 0 {
				return protowire.ParseError(m)
			}
			t.Seconds = int64(v)
			b = b[m:]
		case 2:
			if wtype != protowire.VarintType {
				return fmt.Errorf("timestamp.nanos unexpected wire type %d", wtype)
			}
			v, m := protowire.ConsumeVarint(b)
			if m < 0 {
				return protowire.ParseError(m)
			}
			t.Nanos = int32(v)
			b = b[m:]
		default:
			m := protowire.ConsumeFieldValue(num, wtype, b)
			if m < 0 {
				return protowire.ParseError(m)
			}
			b = b[m:]
		}
	}
	return nil
}

type fsValueKind int

const (
	fsKindUnset fsValueKind = iota
	fsKindNull
	fsKindBool
	fsKindInt
	fsKindDouble
	fsKindString
	fsKindReference
)

type fsValue struct {
	Kind      fsValueKind
	BoolVal   bool
	IntVal    int64
	DoubleVal float64
	StrVal    string
}

func fsBool(v bool) fsValue        { return fsValue{Kind: fsKindBool, BoolVal: v} }
func fsInt(v int64) fsValue        { return fsValue{Kind: fsKindInt, IntVal: v} }
func fsDouble(v float64) fsValue   { return fsValue{Kind: fsKindDouble, DoubleVal: v} }
func fsString(v string) fsValue    { return fsValue{Kind: fsKindString, StrVal: v} }
func fsRef(docPath string) fsValue { return fsValue{Kind: fsKindReference, StrVal: docPath} }

func (v fsValue) marshalProto() ([]byte, error) {
	var b []byte
	switch v.Kind {
	case fsKindNull:
		b = protowire.AppendTag(b, 11, protowire.VarintType)
		b = protowire.AppendVarint(b, 0)
	case fsKindBool:
		b = protowire.AppendTag(b, 1, protowire.VarintType)
		var u uint64
		if v.BoolVal {
			u = 1
		}
		b = protowire.AppendVarint(b, u)
	case fsKindInt:
		b = protowire.AppendTag(b, 2, protowire.VarintType)
		b = protowire.AppendVarint(b, uint64(v.IntVal))
	case fsKindDouble:
		b = protowire.AppendTag(b, 3, protowire.Fixed64Type)
		b = protowire.AppendFixed64(b, math.Float64bits(v.DoubleVal))
	case fsKindString:
		b = protowire.AppendTag(b, 17, protowire.BytesType)
		b = protowire.AppendString(b, v.StrVal)
	case fsKindReference:
		b = protowire.AppendTag(b, 5, protowire.BytesType)
		b = protowire.AppendString(b, v.StrVal)
	}
	return b, nil
}

func (v *fsValue) unmarshalProto(b []byte) error {
	*v = fsValue{}
	for len(b) > 0 {
		num, wtype, n := protowire.ConsumeTag(b)
		if n < 0 {
			return protowire.ParseError(n)
		}
		b = b[n:]
		switch num {
		case 11: // null_value
			if wtype != protowire.VarintType {
				return fmt.Errorf("value.null_value unexpected wire type %d", wtype)
			}
			_, m := protowire.ConsumeVarint(b)
			if m < 0 {
				return protowire.ParseError(m)
			}
			v.Kind = fsKindNull
			b = b[m:]
		case 1: // boolean_value
			if wtype != protowire.VarintType {
				return fmt.Errorf("value.boolean_value unexpected wire type %d", wtype)
			}
			u, m := protowire.ConsumeVarint(b)
			if m < 0 {
				return protowire.ParseError(m)
			}
			v.Kind = fsKindBool
			v.BoolVal = u != 0
			b = b[m:]
		case 2: // integer_value
			if wtype != protowire.VarintType {
				return fmt.Errorf("value.integer_value unexpected wire type %d", wtype)
			}
			u, m := protowire.ConsumeVarint(b)
			if m < 0 {
				return protowire.ParseError(m)
			}
			v.Kind = fsKindInt
			v.IntVal = int64(u)
			b = b[m:]
		case 3: // double_value
			if wtype != protowire.Fixed64Type {
				return fmt.Errorf("value.double_value unexpected wire type %d", wtype)
			}
			u, m := protowire.ConsumeFixed64(b)
			if m < 0 {
				return protowire.ParseError(m)
			}
			v.Kind = fsKindDouble
			v.DoubleVal = math.Float64frombits(u)
			b = b[m:]
		case 17: // string_value
			if wtype != protowire.BytesType {
				return fmt.Errorf("value.string_value unexpected wire type %d", wtype)
			}
			s, m := protowire.ConsumeString(b)
			if m < 0 {
				return protowire.ParseError(m)
			}
			v.Kind = fsKindString
			v.StrVal = s
			b = b[m:]
		case 5: // reference_value
			if wtype != protowire.BytesType {
				return fmt.Errorf("value.reference_value unexpected wire type %d", wtype)
			}
			s, m := protowire.ConsumeString(b)
			if m < 0 {
				return protowire.ParseError(m)
			}
			v.Kind = fsKindReference
			v.StrVal = s
			b = b[m:]
		default:
			m := protowire.ConsumeFieldValue(num, wtype, b)
			if m < 0 {
				return protowire.ParseError(m)
			}
			b = b[m:]
		}
	}
	return nil
}

type fsDocument struct {
	Name       string
	Fields     map[string]fsValue
	CreateTime fsTimestamp
	UpdateTime fsTimestamp
}

func (d fsDocument) marshalProto() ([]byte, error) {
	var b []byte
	if d.Name != "" {
		b = protowire.AppendTag(b, 1, protowire.BytesType)
		b = protowire.AppendString(b, d.Name)
	}
	if len(d.Fields) > 0 {
		keys := make([]string, 0, len(d.Fields))
		for k := range d.Fields {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			vb, err := d.Fields[k].marshalProto()
			if err != nil {
				return nil, err
			}
			var entry []byte
			entry = protowire.AppendTag(entry, 1, protowire.BytesType)
			entry = protowire.AppendString(entry, k)
			entry = protowire.AppendTag(entry, 2, protowire.BytesType)
			entry = protowire.AppendBytes(entry, vb)

			b = protowire.AppendTag(b, 2, protowire.BytesType)
			b = protowire.AppendBytes(b, entry)
		}
	}
	if !d.CreateTime.IsZero() {
		tb, _ := d.CreateTime.marshalProto()
		b = protowire.AppendTag(b, 3, protowire.BytesType)
		b = protowire.AppendBytes(b, tb)
	}
	if !d.UpdateTime.IsZero() {
		tb, _ := d.UpdateTime.marshalProto()
		b = protowire.AppendTag(b, 4, protowire.BytesType)
		b = protowire.AppendBytes(b, tb)
	}
	return b, nil
}

func (d *fsDocument) unmarshalProto(b []byte) error {
	*d = fsDocument{Fields: make(map[string]fsValue)}
	for len(b) > 0 {
		num, wtype, n := protowire.ConsumeTag(b)
		if n < 0 {
			return protowire.ParseError(n)
		}
		b = b[n:]
		switch num {
		case 1: // name
			if wtype != protowire.BytesType {
				return fmt.Errorf("document.name unexpected wire type %d", wtype)
			}
			s, m := protowire.ConsumeString(b)
			if m < 0 {
				return protowire.ParseError(m)
			}
			d.Name = s
			b = b[m:]
		case 2: // fields entry
			if wtype != protowire.BytesType {
				return fmt.Errorf("document.fields unexpected wire type %d", wtype)
			}
			eb, m := protowire.ConsumeBytes(b)
			if m < 0 {
				return protowire.ParseError(m)
			}
			b = b[m:]
			var k string
			var val fsValue
			for len(eb) > 0 {
				fnum, fwtype, fn := protowire.ConsumeTag(eb)
				if fn < 0 {
					return protowire.ParseError(fn)
				}
				eb = eb[fn:]
				switch fnum {
				case 1:
					if fwtype != protowire.BytesType {
						return fmt.Errorf("fieldsEntry.key unexpected wire type %d", fwtype)
					}
					s, fm := protowire.ConsumeString(eb)
					if fm < 0 {
						return protowire.ParseError(fm)
					}
					k = s
					eb = eb[fm:]
				case 2:
					if fwtype != protowire.BytesType {
						return fmt.Errorf("fieldsEntry.value unexpected wire type %d", fwtype)
					}
					vb, fm := protowire.ConsumeBytes(eb)
					if fm < 0 {
						return protowire.ParseError(fm)
					}
					if err := val.unmarshalProto(vb); err != nil {
						return err
					}
					eb = eb[fm:]
				default:
					fm := protowire.ConsumeFieldValue(fnum, fwtype, eb)
					if fm < 0 {
						return protowire.ParseError(fm)
					}
					eb = eb[fm:]
				}
			}
			d.Fields[k] = val
		case 3: // create_time
			if wtype != protowire.BytesType {
				return fmt.Errorf("document.create_time unexpected wire type %d", wtype)
			}
			tb, m := protowire.ConsumeBytes(b)
			if m < 0 {
				return protowire.ParseError(m)
			}
			if err := d.CreateTime.unmarshalProto(tb); err != nil {
				return err
			}
			b = b[m:]
		case 4: // update_time
			if wtype != protowire.BytesType {
				return fmt.Errorf("document.update_time unexpected wire type %d", wtype)
			}
			tb, m := protowire.ConsumeBytes(b)
			if m < 0 {
				return protowire.ParseError(m)
			}
			if err := d.UpdateTime.unmarshalProto(tb); err != nil {
				return err
			}
			b = b[m:]
		default:
			m := protowire.ConsumeFieldValue(num, wtype, b)
			if m < 0 {
				return protowire.ParseError(m)
			}
			b = b[m:]
		}
	}
	return nil
}

type fsPrecondition struct {
	HasExists     bool
	Exists        bool
	HasUpdateTime bool
	UpdateTime    fsTimestamp
}

func (p fsPrecondition) marshalProto() ([]byte, error) {
	var b []byte
	if p.HasExists {
		b = protowire.AppendTag(b, 1, protowire.VarintType)
		var u uint64
		if p.Exists {
			u = 1
		}
		b = protowire.AppendVarint(b, u)
	} else if p.HasUpdateTime {
		tb, err := p.UpdateTime.marshalProto()
		if err != nil {
			return nil, err
		}
		b = protowire.AppendTag(b, 2, protowire.BytesType)
		b = protowire.AppendBytes(b, tb)
	}
	return b, nil
}

func (p *fsPrecondition) unmarshalProto(b []byte) error {
	*p = fsPrecondition{}
	for len(b) > 0 {
		num, wtype, n := protowire.ConsumeTag(b)
		if n < 0 {
			return protowire.ParseError(n)
		}
		b = b[n:]
		switch num {
		case 1:
			if wtype != protowire.VarintType {
				return fmt.Errorf("precondition.exists unexpected wire type %d", wtype)
			}
			u, m := protowire.ConsumeVarint(b)
			if m < 0 {
				return protowire.ParseError(m)
			}
			p.HasExists = true
			p.Exists = u != 0
			b = b[m:]
		case 2:
			if wtype != protowire.BytesType {
				return fmt.Errorf("precondition.update_time unexpected wire type %d", wtype)
			}
			tb, m := protowire.ConsumeBytes(b)
			if m < 0 {
				return protowire.ParseError(m)
			}
			if err := p.UpdateTime.unmarshalProto(tb); err != nil {
				return err
			}
			p.HasUpdateTime = true
			b = b[m:]
		default:
			m := protowire.ConsumeFieldValue(num, wtype, b)
			if m < 0 {
				return protowire.ParseError(m)
			}
			b = b[m:]
		}
	}
	return nil
}

type fsDocumentMask struct {
	FieldPaths []string
}

func (m fsDocumentMask) marshalProto() ([]byte, error) {
	var b []byte
	for _, p := range m.FieldPaths {
		b = protowire.AppendTag(b, 1, protowire.BytesType)
		b = protowire.AppendString(b, p)
	}
	return b, nil
}

func (m *fsDocumentMask) unmarshalProto(b []byte) error {
	*m = fsDocumentMask{}
	for len(b) > 0 {
		num, wtype, n := protowire.ConsumeTag(b)
		if n < 0 {
			return protowire.ParseError(n)
		}
		b = b[n:]
		if num == 1 && wtype == protowire.BytesType {
			s, k := protowire.ConsumeString(b)
			if k < 0 {
				return protowire.ParseError(k)
			}
			m.FieldPaths = append(m.FieldPaths, s)
			b = b[k:]
		} else {
			k := protowire.ConsumeFieldValue(num, wtype, b)
			if k < 0 {
				return protowire.ParseError(k)
			}
			b = b[k:]
		}
	}
	return nil
}

type fsWrite struct {
	Update          *fsDocument
	Delete          string
	UpdateMask      *fsDocumentMask
	CurrentDocument *fsPrecondition
}

func (w fsWrite) marshalProto() ([]byte, error) {
	var b []byte
	if w.Update != nil {
		db, err := w.Update.marshalProto()
		if err != nil {
			return nil, err
		}
		b = protowire.AppendTag(b, 1, protowire.BytesType)
		b = protowire.AppendBytes(b, db)
	} else if w.Delete != "" {
		b = protowire.AppendTag(b, 2, protowire.BytesType)
		b = protowire.AppendString(b, w.Delete)
	}
	if w.UpdateMask != nil {
		mb, err := w.UpdateMask.marshalProto()
		if err != nil {
			return nil, err
		}
		b = protowire.AppendTag(b, 3, protowire.BytesType)
		b = protowire.AppendBytes(b, mb)
	}
	if w.CurrentDocument != nil {
		pb, err := w.CurrentDocument.marshalProto()
		if err != nil {
			return nil, err
		}
		b = protowire.AppendTag(b, 4, protowire.BytesType)
		b = protowire.AppendBytes(b, pb)
	}
	return b, nil
}

func (w *fsWrite) unmarshalProto(b []byte) error {
	*w = fsWrite{}
	for len(b) > 0 {
		num, wtype, n := protowire.ConsumeTag(b)
		if n < 0 {
			return protowire.ParseError(n)
		}
		b = b[n:]
		switch num {
		case 1:
			db, m := protowire.ConsumeBytes(b)
			if m < 0 {
				return protowire.ParseError(m)
			}
			var d fsDocument
			if err := d.unmarshalProto(db); err != nil {
				return err
			}
			w.Update = &d
			b = b[m:]
		case 2:
			s, m := protowire.ConsumeString(b)
			if m < 0 {
				return protowire.ParseError(m)
			}
			w.Delete = s
			b = b[m:]
		case 3:
			mb, m := protowire.ConsumeBytes(b)
			if m < 0 {
				return protowire.ParseError(m)
			}
			var mask fsDocumentMask
			if err := mask.unmarshalProto(mb); err != nil {
				return err
			}
			w.UpdateMask = &mask
			b = b[m:]
		case 4:
			pb, m := protowire.ConsumeBytes(b)
			if m < 0 {
				return protowire.ParseError(m)
			}
			var pre fsPrecondition
			if err := pre.unmarshalProto(pb); err != nil {
				return err
			}
			w.CurrentDocument = &pre
			b = b[m:]
		default:
			m := protowire.ConsumeFieldValue(num, wtype, b)
			if m < 0 {
				return protowire.ParseError(m)
			}
			b = b[m:]
		}
	}
	return nil
}

type fsCommitRequest struct {
	Database    string
	Writes      []fsWrite
	Transaction []byte
}

func (r *fsCommitRequest) marshalProto() ([]byte, error) {
	var b []byte
	if r.Database != "" {
		b = protowire.AppendTag(b, 1, protowire.BytesType)
		b = protowire.AppendString(b, r.Database)
	}
	for _, w := range r.Writes {
		wb, err := w.marshalProto()
		if err != nil {
			return nil, err
		}
		b = protowire.AppendTag(b, 2, protowire.BytesType)
		b = protowire.AppendBytes(b, wb)
	}
	if len(r.Transaction) > 0 {
		b = protowire.AppendTag(b, 3, protowire.BytesType)
		b = protowire.AppendBytes(b, r.Transaction)
	}
	return b, nil
}

func (r *fsCommitRequest) unmarshalProto(b []byte) error {
	*r = fsCommitRequest{}
	for len(b) > 0 {
		num, wtype, n := protowire.ConsumeTag(b)
		if n < 0 {
			return protowire.ParseError(n)
		}
		b = b[n:]
		switch num {
		case 1:
			s, m := protowire.ConsumeString(b)
			if m < 0 {
				return protowire.ParseError(m)
			}
			r.Database = s
			b = b[m:]
		case 2:
			wb, m := protowire.ConsumeBytes(b)
			if m < 0 {
				return protowire.ParseError(m)
			}
			var w fsWrite
			if err := w.unmarshalProto(wb); err != nil {
				return err
			}
			r.Writes = append(r.Writes, w)
			b = b[m:]
		case 3:
			tb, m := protowire.ConsumeBytes(b)
			if m < 0 {
				return protowire.ParseError(m)
			}
			r.Transaction = append([]byte(nil), tb...)
			b = b[m:]
		default:
			m := protowire.ConsumeFieldValue(num, wtype, b)
			if m < 0 {
				return protowire.ParseError(m)
			}
			b = b[m:]
		}
	}
	return nil
}

type fsWriteResult struct {
	UpdateTime fsTimestamp
}

func (r fsWriteResult) marshalProto() ([]byte, error) {
	var b []byte
	if !r.UpdateTime.IsZero() {
		tb, err := r.UpdateTime.marshalProto()
		if err != nil {
			return nil, err
		}
		b = protowire.AppendTag(b, 1, protowire.BytesType)
		b = protowire.AppendBytes(b, tb)
	}
	return b, nil
}

func (r *fsWriteResult) unmarshalProto(b []byte) error {
	*r = fsWriteResult{}
	for len(b) > 0 {
		num, wtype, n := protowire.ConsumeTag(b)
		if n < 0 {
			return protowire.ParseError(n)
		}
		b = b[n:]
		if num == 1 && wtype == protowire.BytesType {
			tb, m := protowire.ConsumeBytes(b)
			if m < 0 {
				return protowire.ParseError(m)
			}
			if err := r.UpdateTime.unmarshalProto(tb); err != nil {
				return err
			}
			b = b[m:]
		} else {
			m := protowire.ConsumeFieldValue(num, wtype, b)
			if m < 0 {
				return protowire.ParseError(m)
			}
			b = b[m:]
		}
	}
	return nil
}

type fsCommitResponse struct {
	WriteResults []fsWriteResult
	CommitTime   fsTimestamp
}

func (r *fsCommitResponse) marshalProto() ([]byte, error) {
	var b []byte
	for _, wr := range r.WriteResults {
		wb, err := wr.marshalProto()
		if err != nil {
			return nil, err
		}
		b = protowire.AppendTag(b, 1, protowire.BytesType)
		b = protowire.AppendBytes(b, wb)
	}
	if !r.CommitTime.IsZero() {
		tb, err := r.CommitTime.marshalProto()
		if err != nil {
			return nil, err
		}
		b = protowire.AppendTag(b, 2, protowire.BytesType)
		b = protowire.AppendBytes(b, tb)
	}
	return b, nil
}

func (r *fsCommitResponse) unmarshalProto(b []byte) error {
	*r = fsCommitResponse{}
	for len(b) > 0 {
		num, wtype, n := protowire.ConsumeTag(b)
		if n < 0 {
			return protowire.ParseError(n)
		}
		b = b[n:]
		switch num {
		case 1:
			wb, m := protowire.ConsumeBytes(b)
			if m < 0 {
				return protowire.ParseError(m)
			}
			var wr fsWriteResult
			if err := wr.unmarshalProto(wb); err != nil {
				return err
			}
			r.WriteResults = append(r.WriteResults, wr)
			b = b[m:]
		case 2:
			tb, m := protowire.ConsumeBytes(b)
			if m < 0 {
				return protowire.ParseError(m)
			}
			if err := r.CommitTime.unmarshalProto(tb); err != nil {
				return err
			}
			b = b[m:]
		default:
			m := protowire.ConsumeFieldValue(num, wtype, b)
			if m < 0 {
				return protowire.ParseError(m)
			}
			b = b[m:]
		}
	}
	return nil
}

type fsGetDocumentRequest struct {
	Name string
}

func (r *fsGetDocumentRequest) marshalProto() ([]byte, error) {
	var b []byte
	if r.Name != "" {
		b = protowire.AppendTag(b, 1, protowire.BytesType)
		b = protowire.AppendString(b, r.Name)
	}
	return b, nil
}

func (r *fsGetDocumentRequest) unmarshalProto(b []byte) error {
	*r = fsGetDocumentRequest{}
	for len(b) > 0 {
		num, wtype, n := protowire.ConsumeTag(b)
		if n < 0 {
			return protowire.ParseError(n)
		}
		b = b[n:]
		if num == 1 && wtype == protowire.BytesType {
			s, m := protowire.ConsumeString(b)
			if m < 0 {
				return protowire.ParseError(m)
			}
			r.Name = s
			b = b[m:]
		} else {
			m := protowire.ConsumeFieldValue(num, wtype, b)
			if m < 0 {
				return protowire.ParseError(m)
			}
			b = b[m:]
		}
	}
	return nil
}

type fsCreateDocumentRequest struct {
	Parent       string
	CollectionId string
	DocumentId   string
	Document     fsDocument
}

func (r *fsCreateDocumentRequest) marshalProto() ([]byte, error) {
	var b []byte
	if r.Parent != "" {
		b = protowire.AppendTag(b, 1, protowire.BytesType)
		b = protowire.AppendString(b, r.Parent)
	}
	if r.CollectionId != "" {
		b = protowire.AppendTag(b, 2, protowire.BytesType)
		b = protowire.AppendString(b, r.CollectionId)
	}
	if r.DocumentId != "" {
		b = protowire.AppendTag(b, 3, protowire.BytesType)
		b = protowire.AppendString(b, r.DocumentId)
	}
	db, err := r.Document.marshalProto()
	if err != nil {
		return nil, err
	}
	b = protowire.AppendTag(b, 4, protowire.BytesType)
	b = protowire.AppendBytes(b, db)
	return b, nil
}

func (r *fsCreateDocumentRequest) unmarshalProto(b []byte) error {
	*r = fsCreateDocumentRequest{}
	for len(b) > 0 {
		num, wtype, n := protowire.ConsumeTag(b)
		if n < 0 {
			return protowire.ParseError(n)
		}
		b = b[n:]
		switch num {
		case 1:
			s, m := protowire.ConsumeString(b)
			if m < 0 {
				return protowire.ParseError(m)
			}
			r.Parent = s
			b = b[m:]
		case 2:
			s, m := protowire.ConsumeString(b)
			if m < 0 {
				return protowire.ParseError(m)
			}
			r.CollectionId = s
			b = b[m:]
		case 3:
			s, m := protowire.ConsumeString(b)
			if m < 0 {
				return protowire.ParseError(m)
			}
			r.DocumentId = s
			b = b[m:]
		case 4:
			db, m := protowire.ConsumeBytes(b)
			if m < 0 {
				return protowire.ParseError(m)
			}
			if err := r.Document.unmarshalProto(db); err != nil {
				return err
			}
			b = b[m:]
		default:
			m := protowire.ConsumeFieldValue(num, wtype, b)
			if m < 0 {
				return protowire.ParseError(m)
			}
			b = b[m:]
		}
	}
	return nil
}

type fsUpdateDocumentRequest struct {
	Document        fsDocument
	UpdateMask      *fsDocumentMask
	CurrentDocument *fsPrecondition
}

func (r *fsUpdateDocumentRequest) marshalProto() ([]byte, error) {
	var b []byte
	db, err := r.Document.marshalProto()
	if err != nil {
		return nil, err
	}
	b = protowire.AppendTag(b, 1, protowire.BytesType)
	b = protowire.AppendBytes(b, db)
	if r.UpdateMask != nil {
		mb, err := r.UpdateMask.marshalProto()
		if err != nil {
			return nil, err
		}
		b = protowire.AppendTag(b, 2, protowire.BytesType)
		b = protowire.AppendBytes(b, mb)
	}
	if r.CurrentDocument != nil {
		pb, err := r.CurrentDocument.marshalProto()
		if err != nil {
			return nil, err
		}
		b = protowire.AppendTag(b, 4, protowire.BytesType)
		b = protowire.AppendBytes(b, pb)
	}
	return b, nil
}

func (r *fsUpdateDocumentRequest) unmarshalProto(b []byte) error {
	*r = fsUpdateDocumentRequest{}
	for len(b) > 0 {
		num, wtype, n := protowire.ConsumeTag(b)
		if n < 0 {
			return protowire.ParseError(n)
		}
		b = b[n:]
		switch num {
		case 1:
			db, m := protowire.ConsumeBytes(b)
			if m < 0 {
				return protowire.ParseError(m)
			}
			if err := r.Document.unmarshalProto(db); err != nil {
				return err
			}
			b = b[m:]
		default:
			m := protowire.ConsumeFieldValue(num, wtype, b)
			if m < 0 {
				return protowire.ParseError(m)
			}
			b = b[m:]
		}
	}
	return nil
}

type fsDeleteDocumentRequest struct {
	Name            string
	CurrentDocument *fsPrecondition
}

func (r *fsDeleteDocumentRequest) marshalProto() ([]byte, error) {
	var b []byte
	if r.Name != "" {
		b = protowire.AppendTag(b, 1, protowire.BytesType)
		b = protowire.AppendString(b, r.Name)
	}
	if r.CurrentDocument != nil {
		pb, err := r.CurrentDocument.marshalProto()
		if err != nil {
			return nil, err
		}
		b = protowire.AppendTag(b, 2, protowire.BytesType)
		b = protowire.AppendBytes(b, pb)
	}
	return b, nil
}

func (r *fsDeleteDocumentRequest) unmarshalProto(b []byte) error {
	*r = fsDeleteDocumentRequest{}
	for len(b) > 0 {
		num, wtype, n := protowire.ConsumeTag(b)
		if n < 0 {
			return protowire.ParseError(n)
		}
		b = b[n:]
		if num == 1 && wtype == protowire.BytesType {
			s, m := protowire.ConsumeString(b)
			if m < 0 {
				return protowire.ParseError(m)
			}
			r.Name = s
			b = b[m:]
		} else {
			m := protowire.ConsumeFieldValue(num, wtype, b)
			if m < 0 {
				return protowire.ParseError(m)
			}
			b = b[m:]
		}
	}
	return nil
}

type fsEmpty struct{}

func (*fsEmpty) marshalProto() ([]byte, error) { return nil, nil }
func (*fsEmpty) unmarshalProto([]byte) error   { return nil }

type fsListDocumentsRequest struct {
	Parent       string
	CollectionId string
	PageSize     int32
	PageToken    string
	OrderBy      string
}

func (r *fsListDocumentsRequest) marshalProto() ([]byte, error) {
	var b []byte
	if r.Parent != "" {
		b = protowire.AppendTag(b, 1, protowire.BytesType)
		b = protowire.AppendString(b, r.Parent)
	}
	if r.CollectionId != "" {
		b = protowire.AppendTag(b, 2, protowire.BytesType)
		b = protowire.AppendString(b, r.CollectionId)
	}
	if r.PageSize != 0 {
		b = protowire.AppendTag(b, 3, protowire.VarintType)
		b = protowire.AppendVarint(b, uint64(uint32(r.PageSize)))
	}
	if r.PageToken != "" {
		b = protowire.AppendTag(b, 4, protowire.BytesType)
		b = protowire.AppendString(b, r.PageToken)
	}
	if r.OrderBy != "" {
		b = protowire.AppendTag(b, 6, protowire.BytesType)
		b = protowire.AppendString(b, r.OrderBy)
	}
	return b, nil
}

func (r *fsListDocumentsRequest) unmarshalProto(b []byte) error {
	*r = fsListDocumentsRequest{}
	for len(b) > 0 {
		num, wtype, n := protowire.ConsumeTag(b)
		if n < 0 {
			return protowire.ParseError(n)
		}
		b = b[n:]
		switch num {
		case 1:
			s, m := protowire.ConsumeString(b)
			if m < 0 {
				return protowire.ParseError(m)
			}
			r.Parent = s
			b = b[m:]
		case 2:
			s, m := protowire.ConsumeString(b)
			if m < 0 {
				return protowire.ParseError(m)
			}
			r.CollectionId = s
			b = b[m:]
		case 3:
			u, m := protowire.ConsumeVarint(b)
			if m < 0 {
				return protowire.ParseError(m)
			}
			r.PageSize = int32(u)
			b = b[m:]
		case 4:
			s, m := protowire.ConsumeString(b)
			if m < 0 {
				return protowire.ParseError(m)
			}
			r.PageToken = s
			b = b[m:]
		default:
			m := protowire.ConsumeFieldValue(num, wtype, b)
			if m < 0 {
				return protowire.ParseError(m)
			}
			b = b[m:]
		}
	}
	return nil
}

type fsListDocumentsResponse struct {
	Documents     []fsDocument
	NextPageToken string
}

func (r *fsListDocumentsResponse) marshalProto() ([]byte, error) {
	var b []byte
	for _, d := range r.Documents {
		db, err := d.marshalProto()
		if err != nil {
			return nil, err
		}
		b = protowire.AppendTag(b, 1, protowire.BytesType)
		b = protowire.AppendBytes(b, db)
	}
	if r.NextPageToken != "" {
		b = protowire.AppendTag(b, 2, protowire.BytesType)
		b = protowire.AppendString(b, r.NextPageToken)
	}
	return b, nil
}

func (r *fsListDocumentsResponse) unmarshalProto(b []byte) error {
	*r = fsListDocumentsResponse{}
	for len(b) > 0 {
		num, wtype, n := protowire.ConsumeTag(b)
		if n < 0 {
			return protowire.ParseError(n)
		}
		b = b[n:]
		switch num {
		case 1:
			if wtype != protowire.BytesType {
				return fmt.Errorf("listDocumentsResponse.documents unexpected wire type %d", wtype)
			}
			db, m := protowire.ConsumeBytes(b)
			if m < 0 {
				return protowire.ParseError(m)
			}
			var d fsDocument
			if err := d.unmarshalProto(db); err != nil {
				return err
			}
			r.Documents = append(r.Documents, d)
			b = b[m:]
		case 2:
			if wtype != protowire.BytesType {
				return fmt.Errorf("listDocumentsResponse.next_page_token unexpected wire type %d", wtype)
			}
			s, m := protowire.ConsumeString(b)
			if m < 0 {
				return protowire.ParseError(m)
			}
			r.NextPageToken = s
			b = b[m:]
		default:
			m := protowire.ConsumeFieldValue(num, wtype, b)
			if m < 0 {
				return protowire.ParseError(m)
			}
			b = b[m:]
		}
	}
	return nil
}

// StructuredQuery wire types for RunQuery.

const (
	fsFilterOpLessThan           int32 = 1
	fsFilterOpLessThanOrEqual    int32 = 2
	fsFilterOpGreaterThan        int32 = 3
	fsFilterOpGreaterThanOrEqual int32 = 4
	fsFilterOpEqual              int32 = 5

	fsCompositeOpAnd int32 = 1

	fsDirectionAscending int32 = 1
)

type fsFieldFilter struct {
	FieldPath string
	Op        int32
	Value     fsValue
}

func (f fsFieldFilter) marshalProto() ([]byte, error) {
	var fb []byte
	fb = protowire.AppendTag(fb, 2, protowire.BytesType)
	fb = protowire.AppendString(fb, f.FieldPath)

	vb, err := f.Value.marshalProto()
	if err != nil {
		return nil, err
	}

	var b []byte
	b = protowire.AppendTag(b, 1, protowire.BytesType)
	b = protowire.AppendBytes(b, fb)
	if f.Op != 0 {
		b = protowire.AppendTag(b, 2, protowire.VarintType)
		b = protowire.AppendVarint(b, uint64(uint32(f.Op)))
	}
	b = protowire.AppendTag(b, 3, protowire.BytesType)
	b = protowire.AppendBytes(b, vb)
	return b, nil
}

type fsFilter struct {
	CompositeAnd []fsFilter
	Field        *fsFieldFilter
}

func (f fsFilter) marshalProto() ([]byte, error) {
	var b []byte
	if len(f.CompositeAnd) > 0 {
		var cb []byte
		cb = protowire.AppendTag(cb, 1, protowire.VarintType)
		cb = protowire.AppendVarint(cb, uint64(uint32(fsCompositeOpAnd)))
		for _, sub := range f.CompositeAnd {
			sb, err := sub.marshalProto()
			if err != nil {
				return nil, err
			}
			cb = protowire.AppendTag(cb, 2, protowire.BytesType)
			cb = protowire.AppendBytes(cb, sb)
		}
		b = protowire.AppendTag(b, 1, protowire.BytesType)
		b = protowire.AppendBytes(b, cb)
	} else if f.Field != nil {
		fb, err := f.Field.marshalProto()
		if err != nil {
			return nil, err
		}
		b = protowire.AppendTag(b, 2, protowire.BytesType)
		b = protowire.AppendBytes(b, fb)
	}
	return b, nil
}

type fsOrder struct {
	FieldPath string
	Direction int32
}

func (o fsOrder) marshalProto() ([]byte, error) {
	var fb []byte
	fb = protowire.AppendTag(fb, 2, protowire.BytesType)
	fb = protowire.AppendString(fb, o.FieldPath)

	var b []byte
	b = protowire.AppendTag(b, 1, protowire.BytesType)
	b = protowire.AppendBytes(b, fb)
	if o.Direction != 0 {
		b = protowire.AppendTag(b, 2, protowire.VarintType)
		b = protowire.AppendVarint(b, uint64(uint32(o.Direction)))
	}
	return b, nil
}

type fsCursor struct {
	Values []fsValue
	Before bool
}

func (c fsCursor) marshalProto() ([]byte, error) {
	var b []byte
	for _, v := range c.Values {
		vb, err := v.marshalProto()
		if err != nil {
			return nil, err
		}
		b = protowire.AppendTag(b, 1, protowire.BytesType)
		b = protowire.AppendBytes(b, vb)
	}
	if c.Before {
		b = protowire.AppendTag(b, 2, protowire.VarintType)
		b = protowire.AppendVarint(b, 1)
	}
	return b, nil
}

type fsStructuredQuery struct {
	CollectionId string
	Where        *fsFilter
	OrderBy      []fsOrder
	StartAt      *fsCursor
	EndAt        *fsCursor
	Limit        int32
}

func (q fsStructuredQuery) marshalProto() ([]byte, error) {
	var b []byte
	if q.CollectionId != "" {
		var from []byte
		from = protowire.AppendTag(from, 2, protowire.BytesType)
		from = protowire.AppendString(from, q.CollectionId)
		b = protowire.AppendTag(b, 2, protowire.BytesType)
		b = protowire.AppendBytes(b, from)
	}
	if q.Where != nil {
		wb, err := q.Where.marshalProto()
		if err != nil {
			return nil, err
		}
		b = protowire.AppendTag(b, 3, protowire.BytesType)
		b = protowire.AppendBytes(b, wb)
	}
	for _, o := range q.OrderBy {
		ob, err := o.marshalProto()
		if err != nil {
			return nil, err
		}
		b = protowire.AppendTag(b, 4, protowire.BytesType)
		b = protowire.AppendBytes(b, ob)
	}
	if q.StartAt != nil {
		sb, err := q.StartAt.marshalProto()
		if err != nil {
			return nil, err
		}
		b = protowire.AppendTag(b, 7, protowire.BytesType)
		b = protowire.AppendBytes(b, sb)
	}
	if q.EndAt != nil {
		eb, err := q.EndAt.marshalProto()
		if err != nil {
			return nil, err
		}
		b = protowire.AppendTag(b, 8, protowire.BytesType)
		b = protowire.AppendBytes(b, eb)
	}
	if q.Limit > 0 {
		var lb []byte
		lb = protowire.AppendTag(lb, 1, protowire.VarintType)
		lb = protowire.AppendVarint(lb, uint64(uint32(q.Limit)))
		b = protowire.AppendTag(b, 5, protowire.BytesType)
		b = protowire.AppendBytes(b, lb)
	}
	return b, nil
}

type fsRunQueryRequest struct {
	Parent          string
	StructuredQuery fsStructuredQuery
}

func (r *fsRunQueryRequest) marshalProto() ([]byte, error) {
	var b []byte
	if r.Parent != "" {
		b = protowire.AppendTag(b, 1, protowire.BytesType)
		b = protowire.AppendString(b, r.Parent)
	}
	qb, err := r.StructuredQuery.marshalProto()
	if err != nil {
		return nil, err
	}
	b = protowire.AppendTag(b, 2, protowire.BytesType)
	b = protowire.AppendBytes(b, qb)
	return b, nil
}

func (r *fsRunQueryRequest) unmarshalProto(b []byte) error {
	*r = fsRunQueryRequest{}
	for len(b) > 0 {
		num, wtype, n := protowire.ConsumeTag(b)
		if n < 0 {
			return protowire.ParseError(n)
		}
		b = b[n:]
		if num == 1 && wtype == protowire.BytesType {
			s, m := protowire.ConsumeString(b)
			if m < 0 {
				return protowire.ParseError(m)
			}
			r.Parent = s
			b = b[m:]
		} else {
			m := protowire.ConsumeFieldValue(num, wtype, b)
			if m < 0 {
				return protowire.ParseError(m)
			}
			b = b[m:]
		}
	}
	return nil
}

type fsRunQueryResponse struct {
	Document *fsDocument
}

func (r *fsRunQueryResponse) marshalProto() ([]byte, error) {
	var b []byte
	if r.Document != nil {
		db, err := r.Document.marshalProto()
		if err != nil {
			return nil, err
		}
		b = protowire.AppendTag(b, 1, protowire.BytesType)
		b = protowire.AppendBytes(b, db)
	}
	return b, nil
}

func (r *fsRunQueryResponse) unmarshalProto(b []byte) error {
	*r = fsRunQueryResponse{}
	for len(b) > 0 {
		num, wtype, n := protowire.ConsumeTag(b)
		if n < 0 {
			return protowire.ParseError(n)
		}
		b = b[n:]
		switch num {
		case 1:
			if wtype != protowire.BytesType {
				return fmt.Errorf("runQueryResponse.document unexpected wire type %d", wtype)
			}
			db, m := protowire.ConsumeBytes(b)
			if m < 0 {
				return protowire.ParseError(m)
			}
			var d fsDocument
			if err := d.unmarshalProto(db); err != nil {
				return err
			}
			r.Document = &d
			b = b[m:]
		default:
			m := protowire.ConsumeFieldValue(num, wtype, b)
			if m < 0 {
				return protowire.ParseError(m)
			}
			b = b[m:]
		}
	}
	return nil
}
