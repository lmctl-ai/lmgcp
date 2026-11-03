// SPDX-License-Identifier: MIT

package gcp

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/lmctl-ai/lmcloud/hostkit"
	"github.com/lmctl-ai/lmcloud/hostkit/fakeadapter"
)

type Adapter struct {
	fake         *fakeadapter.Fake
	client       *http.Client
	grpcPool     *firestorePool
	tableLayouts *tableLayoutCache
	knownQueues  *tableLayoutCache
	ctx          context.Context
}

func NewAdapter() *Adapter {
	return &Adapter{
		fake:         fakeadapter.New(fakeadapter.Options{}),
		client:       hostkit.NewHTTPClient(60 * time.Second),
		grpcPool:     defaultFirestorePool,
		tableLayouts: newTableLayoutCache(),
		knownQueues:  newTableLayoutCache(),
	}
}

func Endpoints() *hostkit.EndpointResolver {
	return &hostkit.EndpointResolver{
		Prefix: "LMGCP",
		Native: map[string][]hostkit.NativeEnv{
			"storage": {{Name: "STORAGE_EMULATOR_HOST", BareHostPort: true}},
			"queue":   {{Name: "PUBSUB_EMULATOR_HOST", BareHostPort: true}},
			"tables":  {{Name: "FIRESTORE_EMULATOR_HOST", BareHostPort: true}},
		},
	}
}

func isLiveFlag(v string) bool {
	v = strings.TrimSpace(v)
	return v == "1" || strings.EqualFold(v, "true")
}

func (a *Adapter) route(ctx context.Context) (*Host, bool, error) {
	if isLiveFlag(os.Getenv("LMGCP_LIVE")) || isLiveFlag(os.Getenv("LMCLOUD_LIVE")) {
		return nil, false, hostkit.ProviderError(hostkit.Unauthenticated, 0, "LiveCredentialsNotImplemented", "live GCP credentials are not implemented; use a loopback emulator")
	}
	base := hostkit.EndpointFrom(ctx)
	if base == "" {
		return nil, true, nil
	}
	u, err := url.Parse(base)
	if err != nil || !hostkit.IsLoopbackHost(u.Hostname()) {
		return nil, false, hostkit.ProviderError(hostkit.Unauthenticated, 0, "LiveCredentialsNotImplemented", "live GCP credentials are not implemented; use a loopback emulator")
	}
	h := NewHost()
	h.ctx = ctx
	h.client = a.client
	if a.grpcPool != nil {
		h.grpcPool = a.grpcPool
	}
	if a.tableLayouts != nil {
		h.tableLayouts = a.tableLayouts
	}
	h.createToken = hostkit.CreateToken(ctx)
	h.authToken = strings.TrimSpace(os.Getenv("LMGCP_SIM_TOKEN"))
	if _, err := h.SetEndpoint("storage", base); err != nil {
		return nil, false, err
	}
	if _, err := h.SetEndpoint("queue", base); err != nil {
		return nil, false, err
	}
	if _, err := h.SetEndpoint("tables", base); err != nil {
		return nil, false, err
	}
	return h, false, nil
}
func cerr(err error) error {
	if err == nil {
		return nil
	}
	if e, ok := err.(*CloudError); ok {
		return hostkit.ProviderError(hostkit.Code(e.Code), e.HTTPStatus, e.ProviderCode, e.Message)
	}
	return err
}

func meta(m *ObjectMeta) hostkit.ObjectMeta {
	if m == nil {
		return hostkit.ObjectMeta{}
	}
	return hostkit.ObjectMeta{Exists: m.Exists, Bucket: m.Bucket, Key: m.Key, Size: m.Size, ETag: NormalizeETag(m.ETag), ContentType: m.ContentType, Metadata: m.Metadata}
}

func (a *Adapter) CreateBucket(ctx context.Context, b string) (bool, error) {
	h, fake, err := a.route(ctx)
	if err != nil {
		return false, err
	}
	if fake {
		return a.fake.CreateBucket(ctx, b)
	}
	v, e := h.BucketCreate(b)
	if e == nil && !v {
		return false, hostkit.ProviderError(hostkit.Conflict, 409, "conflict", "bucket already exists")
	}
	return v, cerr(e)
}
func (a *Adapter) DeleteBucket(ctx context.Context, b string) error {
	h, fake, err := a.route(ctx)
	if err != nil {
		return err
	}
	if fake {
		return a.fake.DeleteBucket(ctx, b)
	}
	v, e := h.BucketDelete(b)
	if e == nil && !v {
		return hostkit.ProviderError(hostkit.NotFound, 404, "notFound", "bucket not found").WithAbsent(hostkit.AbsentBucket)
	}
	return cerr(e)
}
func (a *Adapter) HeadBucket(ctx context.Context, b string) error {
	h, fake, err := a.route(ctx)
	if err != nil {
		return err
	}
	if fake {
		return a.fake.HeadBucket(ctx, b)
	}
	v, e := h.BucketExists(b)
	if e == nil && !v {
		return hostkit.ProviderError(hostkit.NotFound, 404, "notFound", "bucket not found").WithAbsent(hostkit.AbsentBucket)
	}
	return cerr(e)
}
func (a *Adapter) ListBuckets(ctx context.Context, token string) ([]string, string, error) {
	h, fake, err := a.route(ctx)
	if err != nil {
		return nil, "", err
	}
	if fake {
		return a.fake.ListBuckets(ctx, token)
	}
	v, next, e := h.BucketListPage(token)
	return v, next, cerr(e)
}
func (a *Adapter) PutObject(ctx context.Context, b, k string, body []byte, ct string, md map[string]string) (hostkit.ObjectMeta, error) {
	h, fake, err := a.route(ctx)
	if err != nil {
		return hostkit.ObjectMeta{}, err
	}
	if fake {
		return a.fake.PutObject(ctx, b, k, body, ct, md)
	}
	v, e := h.ObjectPut(b, k, string(body), ct, md)
	return meta(v), cerr(e)
}
func (a *Adapter) GetObject(ctx context.Context, b, k string, limit int64) ([]byte, error) {
	h, fake, err := a.route(ctx)
	if err != nil {
		return nil, err
	}
	if fake {
		return a.fake.GetObject(ctx, b, k, limit)
	}
	v, e := h.ObjectGet(b, k)
	if e != nil {
		return nil, cerr(e)
	}
	if int64(len(v)) > limit {
		return nil, hostkit.ProviderError(hostkit.Internal, 0, "", "object exceeds read limit")
	}
	return []byte(v), nil
}
func (a *Adapter) HeadObject(ctx context.Context, b, k string) (hostkit.ObjectMeta, error) {
	h, fake, err := a.route(ctx)
	if err != nil {
		return hostkit.ObjectMeta{}, err
	}
	if fake {
		return a.fake.HeadObject(ctx, b, k)
	}
	v, e := h.ObjectHead(b, k)
	if e != nil {
		if ce, ok := e.(*CloudError); ok && ce.Code == CodeNotFound {
			absent := hostkit.AbsentObject
			if ce.ProviderCode == "bucketNotFound" {
				absent = hostkit.AbsentBucket
			}
			return hostkit.ObjectMeta{}, hostkit.ProviderError(hostkit.NotFound, ce.HTTPStatus, ce.ProviderCode, ce.Message).WithAbsent(absent)
		}
		return hostkit.ObjectMeta{}, cerr(e)
	}
	if !v.Exists {
		return hostkit.ObjectMeta{}, hostkit.ProviderError(hostkit.NotFound, 404, "notFound", "object not found").WithAbsent(hostkit.AbsentObject)
	}
	return meta(v), nil
}
func (a *Adapter) DeleteObject(ctx context.Context, b, k string) error {
	h, fake, err := a.route(ctx)
	if err != nil {
		return err
	}
	if fake {
		return a.fake.DeleteObject(ctx, b, k)
	}
	e := h.ObjectDelete(b, k)
	if ce, ok := e.(*CloudError); ok && ce.Code == CodeNotFound {
		return hostkit.ProviderError(hostkit.NotFound, ce.HTTPStatus, ce.ProviderCode, ce.Message).WithAbsent(hostkit.AbsentBucket)
	}
	return cerr(e)
}

func (a *Adapter) queueExists(ctx context.Context, h *Host, q string) error {
	v, e := h.QueueExists(q)
	if e != nil {
		return cerr(e)
	}
	if !v {
		return hostkit.ProviderError(hostkit.NotFound, 404, "notFound", "queue not found").WithAbsent(hostkit.AbsentQueue)
	}
	return nil
}
func (a *Adapter) ListObjects(ctx context.Context, b, prefix string, max int, token string) (hostkit.ObjectPage, error) {
	h, fake, err := a.route(ctx)
	if err != nil {
		return hostkit.ObjectPage{}, err
	}
	if fake {
		return a.fake.ListObjects(ctx, b, prefix, max, token)
	}
	v, e := h.ObjectList(b, prefix, int64(max), token)
	if e != nil {
		return hostkit.ObjectPage{}, cerr(e)
	}
	p := hostkit.ObjectPage{NextToken: v.NextToken}
	for _, it := range v.Items {
		p.Items = append(p.Items, hostkit.ObjectSummary{Key: it.Key, Size: it.Size, ETag: NormalizeETag(it.ETag)})
	}
	return p, nil
}

func (a *Adapter) QueueCaps() hostkit.QueueCaps {
	return hostkit.QueueCaps{LongPoll: false, TextOnlyWire: false}
}
func (a *Adapter) CreateQueue(ctx context.Context, q string) (bool, error) {
	h, fake, err := a.route(ctx)
	if err != nil {
		return false, err
	}
	if fake {
		return a.fake.CreateQueue(ctx, q)
	}
	v, e := h.QueueCreate(q)
	if e == nil {
		a.knownQueues.set(h.queueBase(), h.project, q, true)
		if !v {
			return false, hostkit.ProviderError(hostkit.Conflict, 409, "conflict", "queue already exists")
		}
	}
	return v, cerr(e)
}
func (a *Adapter) DeleteQueue(ctx context.Context, q string) error {
	h, fake, err := a.route(ctx)
	if err != nil {
		return err
	}
	if fake {
		return a.fake.DeleteQueue(ctx, q)
	}
	a.knownQueues.del(h.queueBase(), h.project, q)
	v, e := h.QueueDelete(q)
	if e == nil && !v {
		return hostkit.ProviderError(hostkit.NotFound, 404, "notFound", "queue not found").WithAbsent(hostkit.AbsentQueue)
	}
	return cerr(e)
}
func (a *Adapter) LookupQueue(ctx context.Context, q string) error {
	h, fake, err := a.route(ctx)
	if err != nil {
		return err
	}
	if fake {
		return a.fake.LookupQueue(ctx, q)
	}
	v, e := h.QueueExists(q)
	if e == nil {
		if !v {
			a.knownQueues.del(h.queueBase(), h.project, q)
			return hostkit.ProviderError(hostkit.NotFound, 404, "notFound", "queue not found").WithAbsent(hostkit.AbsentQueue)
		}
		a.knownQueues.set(h.queueBase(), h.project, q, true)
	}
	return cerr(e)
}
func (a *Adapter) ListQueues(ctx context.Context, token string) ([]string, string, error) {
	h, fake, err := a.route(ctx)
	if err != nil {
		return nil, "", err
	}
	if fake {
		return a.fake.ListQueues(ctx, token)
	}
	v, next, e := h.QueueListPage(token)
	if e == nil {
		for _, q := range v {
			a.knownQueues.set(h.queueBase(), h.project, q, true)
		}
	}
	return v, next, cerr(e)
}
func (a *Adapter) BucketCreateToken(ctx context.Context, b string) (string, error) {
	h, fake, err := a.route(ctx)
	if err != nil {
		return "", err
	}
	if fake {
		return a.fake.BucketCreateToken(ctx, b)
	}
	v, e := h.BucketCreateToken(b)
	return v, cerr(e)
}
func (a *Adapter) QueueCreateToken(ctx context.Context, q string) (string, error) {
	h, fake, err := a.route(ctx)
	if err != nil {
		return "", err
	}
	if fake {
		return a.fake.QueueCreateToken(ctx, q)
	}
	v, e := h.QueueCreateToken(q)
	return v, cerr(e)
}

func (a *Adapter) Send(ctx context.Context, q, wire string) (string, error) {
	h, fake, err := a.route(ctx)
	if err != nil {
		return "", err
	}
	if fake {
		return a.fake.Send(ctx, q, wire)
	}
	v, e := h.QueueSendWire(q, wire)
	if e == nil {
		a.knownQueues.set(h.queueBase(), h.project, q, true)
	}
	return v, cerr(e)
}
func (a *Adapter) Receive(ctx context.Context, q string, max, wait, vis int) ([]hostkit.RawMessage, error) {
	h, fake, err := a.route(ctx)
	if err != nil {
		return nil, err
	}
	if fake {
		return a.fake.Receive(ctx, q, max, wait, vis)
	}
	raw, e := h.QueueReceiveWire(q, int64(max), int64(wait), int64(vis))
	if e != nil {
		return nil, cerr(e)
	}
	a.knownQueues.set(h.queueBase(), h.project, q, true)
	out := make([]hostkit.RawMessage, 0, len(raw))
	for _, m := range raw {
		out = append(out, hostkit.RawMessage{ID: m.ID, Wire: m.Wire, Receipt: m.Receipt})
	}
	return out, nil
}
func (a *Adapter) Ack(ctx context.Context, q, receipt string) error {
	h, fake, err := a.route(ctx)
	if err != nil {
		return err
	}
	if fake {
		return a.fake.Ack(ctx, q, receipt)
	}
	if _, ok := a.knownQueues.get(h.queueBase(), h.project, q); !ok {
		if e := a.queueExists(ctx, h, q); e != nil {
			return e
		}
		a.knownQueues.set(h.queueBase(), h.project, q, true)
	}
	return cerr(h.QueueAck(q, receipt))
}
func (a *Adapter) Nack(ctx context.Context, q, receipt string, vis int) (string, error) {
	h, fake, err := a.route(ctx)
	if err != nil {
		return "", err
	}
	if fake {
		return a.fake.Nack(ctx, q, receipt, vis)
	}
	v, e := h.QueueNack(q, receipt, int64(vis))
	if ce, ok := e.(*CloudError); ok && ce.Code == CodeNotFound {
		a.knownQueues.del(h.queueBase(), h.project, q)
		return "", hostkit.ProviderError(hostkit.NotFound, ce.HTTPStatus, ce.ProviderCode, ce.Message).WithAbsent(hostkit.AbsentQueue)
	}
	if e == nil {
		a.knownQueues.set(h.queueBase(), h.project, q, true)
	}
	return v, cerr(e)
}

func (a *Adapter) CreateTable(ctx context.Context, name string, sorted bool) (bool, error) {
	h, fake, err := a.route(ctx)
	if err != nil {
		return false, err
	}
	if fake {
		return a.fake.CreateTable(ctx, name, sorted)
	}
	return h.CreateTable(ctx, name, sorted)
}

func (a *Adapter) DeleteTable(ctx context.Context, name string) error {
	h, fake, err := a.route(ctx)
	if err != nil {
		return err
	}
	if fake {
		return a.fake.DeleteTable(ctx, name)
	}
	return h.DeleteTable(ctx, name)
}

func (a *Adapter) HeadTable(ctx context.Context, name string) error {
	h, fake, err := a.route(ctx)
	if err != nil {
		return err
	}
	if fake {
		return a.fake.HeadTable(ctx, name)
	}
	return h.HeadTable(ctx, name)
}

func (a *Adapter) DescribeTable(ctx context.Context, name string) (hostkit.TableInfo, error) {
	h, fake, err := a.route(ctx)
	if err != nil {
		return hostkit.TableInfo{}, err
	}
	if fake {
		return a.fake.DescribeTable(ctx, name)
	}
	return h.DescribeTable(ctx, name)
}

func (a *Adapter) TableCreateToken(ctx context.Context, name string) (string, error) {
	h, fake, err := a.route(ctx)
	if err != nil {
		return "", err
	}
	if fake {
		return a.fake.TableCreateToken(ctx, name)
	}
	return h.TableCreateToken(ctx, name)
}

func (a *Adapter) ListTables(ctx context.Context, token string) ([]string, string, error) {
	h, fake, err := a.route(ctx)
	if err != nil {
		return nil, "", err
	}
	if fake {
		return a.fake.ListTables(ctx, token)
	}
	return h.ListTables(ctx, token)
}

func (a *Adapter) PutItem(ctx context.Context, table string, item hostkit.Item, mode hostkit.PutMode, expectVersion string) (hostkit.Item, error) {
	h, fake, err := a.route(ctx)
	if err != nil {
		return hostkit.Item{}, err
	}
	if fake {
		return a.fake.PutItem(ctx, table, item, mode, expectVersion)
	}
	return h.PutItem(ctx, table, item, mode, expectVersion)
}

func (a *Adapter) GetItem(ctx context.Context, table, pk, sk string) (hostkit.Item, error) {
	h, fake, err := a.route(ctx)
	if err != nil {
		return hostkit.Item{}, err
	}
	if fake {
		return a.fake.GetItem(ctx, table, pk, sk)
	}
	return h.GetItem(ctx, table, pk, sk)
}

func (a *Adapter) DeleteItem(ctx context.Context, table, pk, sk string) error {
	h, fake, err := a.route(ctx)
	if err != nil {
		return err
	}
	if fake {
		return a.fake.DeleteItem(ctx, table, pk, sk)
	}
	return h.DeleteItem(ctx, table, pk, sk)
}

func (a *Adapter) UpdateItem(ctx context.Context, table, pk, sk string, changes hostkit.Item) (hostkit.Item, error) {
	h, fake, err := a.route(ctx)
	if err != nil {
		return hostkit.Item{}, err
	}
	if fake {
		return a.fake.UpdateItem(ctx, table, pk, sk, changes)
	}
	return h.UpdateItem(ctx, table, pk, sk, changes)
}

func (a *Adapter) Query(ctx context.Context, table, pk, skFrom, skTo string, max int, token string) (hostkit.TablePage, error) {
	h, fake, err := a.route(ctx)
	if err != nil {
		return hostkit.TablePage{}, err
	}
	if fake {
		return a.fake.Query(ctx, table, pk, skFrom, skTo, max, token)
	}
	return h.Query(ctx, table, pk, skFrom, skTo, max, token)
}

func (a *Adapter) Scan(ctx context.Context, table string, max int, token string) (hostkit.TablePage, error) {
	h, fake, err := a.route(ctx)
	if err != nil {
		return hostkit.TablePage{}, err
	}
	if fake {
		return a.fake.Scan(ctx, table, max, token)
	}
	return h.Scan(ctx, table, max, token)
}
