// SPDX-License-Identifier: MIT

package gcp

import (
	"context"
	"net/url"

	"github.com/lmctl-ai/lmcloud/hostkit"
	"github.com/lmctl-ai/lmcloud/hostkit/fakeadapter"
)

type Adapter struct {
	fake *fakeadapter.Fake
}

func NewAdapter() *Adapter { return &Adapter{fake: fakeadapter.New(fakeadapter.Options{})} }

func Endpoints() *hostkit.EndpointResolver {
	return &hostkit.EndpointResolver{
		Prefix: "LMGCP",
		Native: map[string][]hostkit.NativeEnv{
			"storage": {{Name: "STORAGE_EMULATOR_HOST", BareHostPort: true}},
			"queue":   {{Name: "PUBSUB_EMULATOR_HOST", BareHostPort: true}},
		},
	}
}

func (a *Adapter) route(ctx context.Context) (*Host, bool, error) {
	base := hostkit.EndpointFrom(ctx)
	if base == "" {
		return nil, true, nil
	}
	u, err := url.Parse(base)
	if err != nil || !hostkit.IsLoopbackHost(u.Hostname()) {
		return nil, false, hostkit.ProviderError(hostkit.Internal, 0, "", "live GCP is not supported by this host; use a loopback emulator")
	}
	h := NewHost()
	if _, err := h.SetEndpoint("storage", base); err != nil {
		return nil, false, err
	}
	if _, err := h.SetEndpoint("queue", base); err != nil {
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
	v, e := h.BucketList()
	return v, "", cerr(e)
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
	return hostkit.QueueCaps{LongPoll: true, TextOnlyWire: false}
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
	if e == nil && !v {
		return hostkit.ProviderError(hostkit.NotFound, 404, "notFound", "queue not found").WithAbsent(hostkit.AbsentQueue)
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
	v, e := h.QueueList()
	return v, "", cerr(e)
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
	if e := a.queueExists(ctx, h, q); e != nil {
		return e
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
	if e := a.queueExists(ctx, h, q); e != nil {
		return "", e
	}
	v, e := h.QueueNack(q, receipt, int64(vis))
	return v, cerr(e)
}
