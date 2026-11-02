package gcp

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	lmcode "github.com/lmctl-ai/lmcode-go"
)

type gcpError struct {
	Error struct {
		Code    int                                `json:"code"`
		Message string                             `json:"message"`
		Errors  []struct{ Reason, Message string } `json:"errors"`
	} `json:"error"`
}

func (h *Host) setErr(op, target string, e *CloudError) error {
	h.mu.Lock()
	h.lastError = e
	abort := h.abortOnError
	h.mu.Unlock()
	if abort {
		return e
	}
	return nil
}

func (h *Host) classify(status int, body []byte) *CloudError {
	var ge gcpError
	_ = json.Unmarshal(body, &ge)
	code := CodeInternal
	switch status {
	case 400:
		code = CodeInvalidArgument
	case 401:
		code = CodeUnauthenticated
	case 403:
		code = CodePermissionDenied
	case 404:
		code = CodeNotFound
	case 409:
		code = CodeConflict
	case 412:
		code = CodePreconditionFailed
	}
	if status >= 200 && status < 300 {
		return okError()
	}
	reason := ""
	if len(ge.Error.Errors) > 0 {
		reason = ge.Error.Errors[0].Reason
	}
	msg := ge.Error.Message
	if msg == "" {
		msg = strings.TrimSpace(string(body))
	}
	return newCloudError(code, status, reason, msg)
}

func (h *Host) request(ctx context.Context, method, urlStr string, body []byte, contentType string) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, urlStr, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("Accept", "application/json")
	client := &http.Client{Timeout: 25 * time.Second, Transport: &http.Transport{Proxy: nil}}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	return resp.StatusCode, data, err
}

func (h *Host) storageBase() string {
	ep, err := h.Endpoint("storage")
	if err != nil || ep == "" {
		return ""
	}
	ep = strings.TrimRight(ep, "/")
	if strings.HasSuffix(ep, "/storage/v1") {
		return ep
	}
	return ep + "/storage/v1"
}
func (h *Host) queueBase() string {
	ep, err := h.Endpoint("queue")
	if err != nil || ep == "" {
		return ""
	}
	ep = strings.TrimRight(ep, "/")
	if strings.HasSuffix(ep, "/v1") {
		return ep
	}
	return ep + "/v1"
}
func (h *Host) topicName(name string) string {
	return "projects/" + h.project + "/topics/" + url.PathEscape(name)
}
func (h *Host) subName(name string) string {
	return "projects/" + h.project + "/subscriptions/" + url.PathEscape(name)
}
func (h *Host) topicPath(name string) string { return h.queueBase() + "/" + h.topicName(name) }
func (h *Host) subPath(name string) string   { return h.queueBase() + "/" + h.subName(name) }

func (h *Host) BucketCreate(name string) (bool, error) {
	if e := ValidateBucketName(name); e != nil {
		return false, h.setErr("bucket_create", name, e)
	}
	body, _ := json.Marshal(map[string]string{"name": name})
	status, data, err := h.request(context.Background(), http.MethodPost, h.storageBase()+"/b?project="+url.QueryEscape(h.project), body, "application/json")
	if err != nil {
		return false, h.setErr("bucket_create", name, newCloudError(CodeInternal, 0, "transport", err.Error()))
	}
	switch status {
	case 200, 201:
		h.mu.Lock()
		h.lastError = okError()
		h.mu.Unlock()
		return true, nil
	case 409:
		h.mu.Lock()
		h.lastError = okError()
		h.mu.Unlock()
		return false, nil
	}
	return false, h.setErr("bucket_create", name, h.classify(status, data))
}
func (h *Host) BucketDelete(name string) (bool, error) {
	if e := ValidateBucketName(name); e != nil {
		return false, h.setErr("bucket_delete", name, e)
	}
	status, data, err := h.request(context.Background(), http.MethodDelete, h.storageBase()+"/b/"+url.PathEscape(name), nil, "")
	if err != nil {
		return false, h.setErr("bucket_delete", name, newCloudError(CodeInternal, 0, "transport", err.Error()))
	}
	if status == 404 {
		return false, nil
	}
	if status == 409 {
		return false, h.setErr("bucket_delete", name, h.classify(status, data))
	}
	if status >= 200 && status < 300 {
		return true, nil
	}
	return false, h.setErr("bucket_delete", name, h.classify(status, data))
}
func (h *Host) BucketDeleteErr(name string) error { _, err := h.BucketDelete(name); return err }

func (h *Host) BucketExists(name string) (bool, error) {
	if e := ValidateBucketName(name); e != nil {
		return false, h.setErr("bucket_exists", name, e)
	}
	status, data, err := h.request(context.Background(), http.MethodGet, h.storageBase()+"/b/"+url.PathEscape(name), nil, "")
	if err != nil {
		return false, h.setErr("bucket_exists", name, newCloudError(CodeInternal, 0, "transport", err.Error()))
	}
	if status == 404 {
		return false, nil
	}
	if status >= 200 && status < 300 {
		return true, nil
	}
	return false, h.setErr("bucket_exists", name, h.classify(status, data))
}
func (h *Host) BucketList() ([]string, error) {
	var names []string
	token := ""
	for {
		u := h.storageBase() + "/b?project=" + url.QueryEscape(h.project)
		if token != "" {
			u += "&pageToken=" + url.QueryEscape(token)
		}
		status, data, err := h.request(context.Background(), http.MethodGet, u, nil, "")
		if err != nil {
			return nil, h.setErr("bucket_list", "", newCloudError(CodeInternal, 0, "transport", err.Error()))
		}
		if status < 200 || status >= 300 {
			return nil, h.setErr("bucket_list", "", h.classify(status, data))
		}
		var page struct {
			Items []struct {
				Name string `json:"name"`
			} `json:"items"`
			Next string `json:"nextPageToken"`
		}
		if err := json.Unmarshal(data, &page); err != nil {
			return nil, h.setErr("bucket_list", "", newCloudError(CodeInternal, status, "decode", err.Error()))
		}
		for _, it := range page.Items {
			names = append(names, it.Name)
		}
		if page.Next == "" {
			break
		}
		token = page.Next
	}
	sort.Strings(names)
	return names, nil
}

func (h *Host) ObjectPut(bucket, key, data, contentType string, metadata map[string]string) (*ObjectMeta, error) {
	if e := ValidateBucketName(bucket); e != nil {
		return nil, h.setErr("object_put", bucket, e)
	}
	if e := ValidateObjectKey(key); e != nil {
		return nil, h.setErr("object_put", key, e)
	}
	if e := ValidateContentType(contentType); e != nil {
		return nil, h.setErr("object_put", key, e)
	}
	if e := ValidateMetadata(metadata); e != nil {
		return nil, h.setErr("object_put", key, e)
	}
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	hdr := textproto.MIMEHeader{}
	hdr.Set("Content-Type", "application/json; charset=UTF-8")
	part, _ := mw.CreatePart(hdr)
	meta, _ := json.Marshal(map[string]any{"name": key, "contentType": contentType, "metadata": metadata})
	part.Write(meta)
	mh := textproto.MIMEHeader{}
	mh.Set("Content-Type", contentType)
	media, _ := mw.CreatePart(mh)
	media.Write([]byte(data))
	mw.Close()
	u := h.storageBase() + "/b/" + url.PathEscape(bucket) + "/o?uploadType=multipart&name=" + url.QueryEscape(key)
	status, resp, err := h.request(context.Background(), http.MethodPost, u, buf.Bytes(), "multipart/related; boundary="+mw.Boundary())
	if err != nil {
		return nil, h.setErr("object_put", key, newCloudError(CodeInternal, 0, "transport", err.Error()))
	}
	if status < 200 || status >= 300 {
		return nil, h.setErr("object_put", key, h.classify(status, resp))
	}
	return h.objectMetaFromJSON(bucket, key, resp)
}
func (h *Host) objectMetaFromJSON(bucket, key string, data []byte) (*ObjectMeta, error) {
	var o struct {
		Name, ETag, ContentType string
		Size                    string            `json:"size"`
		Metadata                map[string]string `json:"metadata"`
	}
	if err := json.Unmarshal(data, &o); err != nil {
		return nil, h.setErr("object", key, newCloudError(CodeInternal, 0, "decode", err.Error()))
	}
	if o.Name != "" {
		key = o.Name
	}
	if o.Metadata == nil {
		o.Metadata = map[string]string{}
	}
	size, _ := strconv.ParseInt(o.Size, 10, 64)
	return &ObjectMeta{Exists: true, Bucket: bucket, Key: key, Size: size, ETag: NormalizeETag(o.ETag), ContentType: o.ContentType, Metadata: o.Metadata}, nil
}
func (h *Host) ObjectGet(bucket, key string) (string, error) {
	status, data, err := h.request(context.Background(), http.MethodGet, h.storageBase()+"/b/"+url.PathEscape(bucket)+"/o/"+EncodeObjectKeyPath(key)+"?alt=media", nil, "")
	if err != nil {
		return "", h.setErr("object_get", key, newCloudError(CodeInternal, 0, "transport", err.Error()))
	}
	if status < 200 || status >= 300 {
		return "", h.setErr("object_get", key, h.classify(status, data))
	}
	return string(data), nil
}
func (h *Host) ObjectHead(bucket, key string) (*ObjectMeta, error) {
	status, data, err := h.request(context.Background(), http.MethodGet, h.storageBase()+"/b/"+url.PathEscape(bucket)+"/o/"+EncodeObjectKeyPath(key), nil, "")
	if err != nil {
		return nil, h.setErr("object_head", key, newCloudError(CodeInternal, 0, "transport", err.Error()))
	}
	if status == 404 {
		exists, berr := h.BucketExists(bucket)
		if berr != nil {
			return nil, berr
		}
		if !exists {
			return nil, h.setErr("object_head", bucket, newCloudError(CodeNotFound, 404, "bucketNotFound", "bucket not found"))
		}
		return &ObjectMeta{Exists: false, Bucket: bucket, Key: key, Metadata: map[string]string{}}, nil
	}
	if status < 200 || status >= 300 {
		return nil, h.setErr("object_head", key, h.classify(status, data))
	}
	return h.objectMetaFromJSON(bucket, key, data)
}
func (h *Host) ObjectDelete(bucket, key string) error {
	status, data, err := h.request(context.Background(), http.MethodDelete, h.storageBase()+"/b/"+url.PathEscape(bucket)+"/o/"+EncodeObjectKeyPath(key), nil, "")
	if err != nil {
		return h.setErr("object_delete", key, newCloudError(CodeInternal, 0, "transport", err.Error()))
	}
	if status >= 200 && status < 300 {
		return nil
	}
	if status == 404 {
		exists, berr := h.BucketExists(bucket)
		if berr != nil {
			return berr
		}
		if !exists {
			return h.setErr("object_delete", bucket, newCloudError(CodeNotFound, 404, "notFound", "bucket not found"))
		}
		return nil
	}
	return h.setErr("object_delete", key, h.classify(status, data))
}
func (h *Host) ObjectList(bucket, prefix string, maxKeys int64, token string) (*ObjectListPage, error) {
	if e := ValidateBucketName(bucket); e != nil {
		return nil, h.setErr("object_list", bucket, e)
	}
	if e := ValidateListPrefix(prefix); e != nil {
		return nil, h.setErr("object_list", prefix, e)
	}
	if e := ValidateMaxKeys(maxKeys); e != nil {
		return nil, h.setErr("object_list", prefix, e)
	}
	if maxKeys == 0 {
		maxKeys = 1000
	}
	u := h.storageBase() + "/b/" + url.PathEscape(bucket) + "/o?prefix=" + url.QueryEscape(prefix) + "&maxResults=" + fmt.Sprint(maxKeys)
	if token != "" {
		u += "&pageToken=" + url.QueryEscape(token)
	}
	status, data, err := h.request(context.Background(), http.MethodGet, u, nil, "")
	if err != nil {
		return nil, h.setErr("object_list", bucket, newCloudError(CodeInternal, 0, "transport", err.Error()))
	}
	if status < 200 || status >= 300 {
		return nil, h.setErr("object_list", bucket, h.classify(status, data))
	}
	var page struct {
		Items []struct {
			Name, ETag string
			Size       string `json:"size"`
		} `json:"items"`
		Next string `json:"nextPageToken"`
	}
	if err := json.Unmarshal(data, &page); err != nil {
		return nil, h.setErr("object_list", bucket, newCloudError(CodeInternal, status, "decode", err.Error()))
	}
	out := &ObjectListPage{Items: []*ObjectSummary{}, HasMore: page.Next != "", NextToken: page.Next}
	for _, it := range page.Items {
		size, _ := strconv.ParseInt(it.Size, 10, 64)
		out.Items = append(out.Items, &ObjectSummary{Key: it.Name, Size: size, ETag: NormalizeETag(it.ETag)})
	}
	sort.Slice(out.Items, func(i, j int) bool { return out.Items[i].Key < out.Items[j].Key })
	return out, nil
}
func (h *Host) ObjectListAll(bucket, prefix string) ([]string, error) {
	var all []string
	token := ""
	for {
		page, err := h.ObjectList(bucket, prefix, 1000, token)
		if err != nil {
			return nil, err
		}
		if page == nil {
			return all, nil
		}
		for _, it := range page.Items {
			all = append(all, it.Key)
		}
		if !page.HasMore {
			break
		}
		token = page.NextToken
	}
	return all, nil
}

func (h *Host) QueueCreate(name string) (bool, error) {
	if e := ValidateQueueName(name); e != nil {
		return false, h.setErr("queue_create", name, e)
	}
	topic := h.topicPath(name)
	status, data, err := h.request(context.Background(), http.MethodPut, topic, []byte("{}"), "application/json")
	if err != nil {
		return false, h.setErr("queue_create", name, newCloudError(CodeInternal, 0, "transport", err.Error()))
	}
	topicCreated := status >= 200 && status < 300
	if status != 409 && !topicCreated {
		return false, h.setErr("queue_create", fmt.Sprintf("%s topic-status=%d", name, status), h.classify(status, data))
	}
	sub := h.subPath(name)
	body, _ := json.Marshal(map[string]any{"topic": h.topicName(name), "ackDeadlineSeconds": 30, "expirationPolicy": map[string]any{}})
	status, data, err = h.request(context.Background(), http.MethodPut, sub, body, "application/json")
	if err != nil {
		return false, h.setErr("queue_create", name, newCloudError(CodeInternal, 0, "transport", err.Error()))
	}
	subCreated := status >= 200 && status < 300
	if status != 409 && !subCreated {
		return false, h.setErr("queue_create", fmt.Sprintf("%s sub-status=%d", name, status), h.classify(status, data))
	}
	return topicCreated || subCreated, nil
}
func (h *Host) QueueDelete(name string) (bool, error) {
	if e := ValidateQueueName(name); e != nil {
		return false, h.setErr("queue_delete", name, e)
	}
	deleted := false
	for _, u := range []string{h.subPath(name), h.topicPath(name)} {
		status, data, err := h.request(context.Background(), http.MethodDelete, u, nil, "")
		if err != nil {
			return false, h.setErr("queue_delete", name, newCloudError(CodeInternal, 0, "transport", err.Error()))
		}
		if status >= 200 && status < 300 {
			deleted = true
			continue
		}
		if status == 404 {
			continue
		}
		return false, h.setErr("queue_delete", name, h.classify(status, data))
	}
	return deleted, nil
}
func (h *Host) queueGet(path string) (map[string]any, *CloudError, bool) {
	status, data, err := h.request(context.Background(), http.MethodGet, path, nil, "")
	if err != nil {
		return nil, newCloudError(CodeInternal, 0, "transport", err.Error()), false
	}
	if status == 404 {
		return nil, nil, false
	}
	if status < 200 || status >= 300 {
		return nil, h.classify(status, data), false
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, newCloudError(CodeInternal, status, "decode", err.Error()), false
	}
	return m, nil, true
}
func (h *Host) QueueExists(name string) (bool, error) {
	if e := ValidateQueueName(name); e != nil {
		return false, h.setErr("queue_exists", name, e)
	}
	_, cerr, ok := h.queueGet(h.topicPath(name))
	if cerr != nil {
		return false, h.setErr("queue_exists", name, cerr)
	}
	if !ok {
		return false, nil
	}
	sub, cerr, ok := h.queueGet(h.subPath(name))
	if cerr != nil {
		return false, h.setErr("queue_exists", name, cerr)
	}
	if !ok {
		return false, nil
	}
	want := h.topicName(name)
	got, _ := sub["topic"].(string)
	return got == want, nil
}
func (h *Host) QueueList() ([]string, error) {
	topics := map[string]bool{}
	subs := map[string]bool{}
	collect := func(base, kind string, into map[string]bool) error {
		token := ""
		for {
			u := base
			if token != "" {
				u += "&pageToken=" + url.QueryEscape(token)
			}
			status, data, err := h.request(context.Background(), http.MethodGet, u, nil, "")
			if err != nil {
				return h.setErr("queue_list", kind, newCloudError(CodeInternal, 0, "transport", err.Error()))
			}
			if status < 200 || status >= 300 {
				return h.setErr("queue_list", kind, h.classify(status, data))
			}
			var page struct {
				Topics []struct {
					Name string `json:"name"`
				} `json:"topics"`
				Subscriptions []struct {
					Name  string `json:"name"`
					Topic string `json:"topic"`
				} `json:"subscriptions"`
				Next string `json:"nextPageToken"`
			}
			if err := json.Unmarshal(data, &page); err != nil {
				return h.setErr("queue_list", kind, newCloudError(CodeInternal, status, "decode", err.Error()))
			}
			for _, t := range page.Topics {
				into[shortName(t.Name)] = true
			}
			for _, s := range page.Subscriptions {
				into[shortName(s.Name)] = true
			}
			if page.Next == "" {
				break
			}
			token = page.Next
		}
		return nil
	}
	if err := collect(h.queueBase()+"/projects/"+url.PathEscape(h.project)+"/topics", "topics", topics); err != nil {
		return nil, err
	}
	if err := collect(h.queueBase()+"/projects/"+url.PathEscape(h.project)+"/subscriptions", "subscriptions", subs); err != nil {
		return nil, err
	}
	var out []string
	for n := range topics {
		if subs[n] {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out, nil
}
func shortName(full string) string {
	if i := strings.LastIndex(full, "/"); i >= 0 {
		return full[i+1:]
	}
	return full
}
func (h *Host) QueueSend(queue, body string) (string, error) {
	if e := ValidateQueueName(queue); e != nil {
		return "", h.setErr("queue_send", queue, e)
	}
	if e := ValidateQueueBody(body); e != nil {
		return "", h.setErr("queue_send", queue, e)
	}
	wire := EncodeQueueBody(body)
	req, _ := json.Marshal(map[string]any{"messages": []map[string]string{{"data": base64.StdEncoding.EncodeToString([]byte(wire))}}})
	status, data, err := h.request(context.Background(), http.MethodPost, h.topicPath(queue)+":publish", req, "application/json")
	if err != nil {
		return "", h.setErr("queue_send", queue, newCloudError(CodeInternal, 0, "transport", err.Error()))
	}
	if status < 200 || status >= 300 {
		return "", h.setErr("queue_send", queue, h.classify(status, data))
	}
	var res struct {
		MessageIds []string `json:"messageIds"`
	}
	if err := json.Unmarshal(data, &res); err != nil || len(res.MessageIds) == 0 {
		return "", h.setErr("queue_send", queue, newCloudError(CodeInternal, status, "decode", "missing messageIds"))
	}
	return res.MessageIds[0], nil
}
func (h *Host) QueueReceive(queue string, maxMessages, waitSeconds, visibility int64) (*QueueMessageList, error) {
	if e := ValidateQueueName(queue); e != nil {
		return nil, h.setErr("queue_receive", queue, e)
	}
	if e := ValidateQueueReceiveParams(maxMessages, waitSeconds, visibility); e != nil {
		return nil, h.setErr("queue_receive", queue, e)
	}
	if maxMessages == 0 {
		maxMessages = 1
	}
	if waitSeconds == 0 {
		waitSeconds = 1
	}
	deadline := time.Now().Add(time.Duration(waitSeconds) * time.Second)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return &QueueMessageList{}, nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), remaining)
		req, _ := json.Marshal(map[string]any{"maxMessages": maxMessages, "returnImmediately": false})
		status, data, err := h.request(ctx, http.MethodPost, h.subPath(queue)+":pull", req, "application/json")
		cancel()
		if err != nil {
			if time.Now().Before(deadline) {
				continue
			}
			return &QueueMessageList{}, nil
		}
		if status < 200 || status >= 300 {
			return nil, h.setErr("queue_receive", h.subPath(queue)+":pull", h.classify(status, data))
		}
		var pull struct {
			ReceivedMessages []struct {
				AckId   string `json:"ackId"`
				Message struct {
					Data string `json:"data"`
				} `json:"message"`
			} `json:"receivedMessages"`
		}
		if err := json.Unmarshal(data, &pull); err != nil {
			return nil, h.setErr("queue_receive", queue, newCloudError(CodeInternal, status, "decode", err.Error()))
		}
		if len(pull.ReceivedMessages) == 0 {
			if time.Now().Before(deadline) {
				time.Sleep(50 * time.Millisecond)
				continue
			}
			return &QueueMessageList{}, nil
		}
		ids := make([]string, 0, len(pull.ReceivedMessages))
		out := &QueueMessageList{}
		for _, rm := range pull.ReceivedMessages {
			ids = append(ids, rm.AckId)
			raw, _ := base64.StdEncoding.DecodeString(rm.Message.Data)
			out.Items = append(out.Items, &QueueMessage{ID: rm.AckId, Body: DecodeQueueBody(string(raw)), Receipt: rm.AckId})
		}
		mod, _ := json.Marshal(map[string]any{"ackIds": ids, "ackDeadlineSeconds": visibility})
		status, data, err = h.request(context.Background(), http.MethodPost, h.subPath(queue)+":modifyAckDeadline", mod, "application/json")
		if err != nil {
			return nil, h.setErr("queue_receive", queue, newCloudError(CodeInternal, 0, "transport", err.Error()))
		}
		if status < 200 || status >= 300 {
			return nil, h.setErr("queue_receive", h.subPath(queue)+":modifyAckDeadline", h.classify(status, data))
		}
		return out, nil
	}
}
func (h *Host) QueueAck(queue, receipt string) error {
	if e := ValidateQueueName(queue); e != nil {
		return h.setErr("queue_ack", queue, e)
	}
	if e := ValidateQueueReceipt(receipt); e != nil {
		return h.setErr("queue_ack", queue, e)
	}
	body, _ := json.Marshal(map[string]any{"ackIds": []string{receipt}})
	status, data, err := h.request(context.Background(), http.MethodPost, h.subPath(queue)+":acknowledge", body, "application/json")
	if err != nil {
		return h.setErr("queue_ack", queue, newCloudError(CodeInternal, 0, "transport", err.Error()))
	}
	if status < 200 || status >= 300 {
		return h.setErr("queue_ack", queue, h.classify(status, data))
	}
	return nil
}
func (h *Host) QueueNack(queue, receipt string, visibility int64) (string, error) {
	if e := ValidateQueueName(queue); e != nil {
		return "", h.setErr("queue_nack", queue, e)
	}
	if e := ValidateQueueReceipt(receipt); e != nil {
		return "", h.setErr("queue_nack", queue, e)
	}
	if e := ValidateQueueNackTimeout(visibility); e != nil {
		return "", h.setErr("queue_nack", queue, e)
	}
	body, _ := json.Marshal(map[string]any{"ackIds": []string{receipt}, "ackDeadlineSeconds": visibility})
	status, data, err := h.request(context.Background(), http.MethodPost, h.subPath(queue)+":modifyAckDeadline", body, "application/json")
	if err != nil {
		return "", h.setErr("queue_nack", queue, newCloudError(CodeInternal, 0, "transport", err.Error()))
	}
	if status < 200 || status >= 300 {
		return "", h.setErr("queue_nack", queue, h.classify(status, data))
	}
	return receipt, nil
}

type RawMessage struct{ ID, Wire, Receipt string }

func (h *Host) QueueSendWire(queue, wire string) (string, error) {
	req, _ := json.Marshal(map[string]any{"messages": []map[string]string{{"data": base64.StdEncoding.EncodeToString([]byte(wire))}}})
	status, data, err := h.request(context.Background(), http.MethodPost, h.topicPath(queue)+":publish", req, "application/json")
	if err != nil {
		return "", h.setErr("queue_send", queue, newCloudError(CodeInternal, 0, "transport", err.Error()))
	}
	if status < 200 || status >= 300 {
		return "", h.setErr("queue_send", queue, h.classify(status, data))
	}
	var res struct {
		MessageIds []string `json:"messageIds"`
	}
	if err := json.Unmarshal(data, &res); err != nil || len(res.MessageIds) == 0 {
		return "", h.setErr("queue_send", queue, newCloudError(CodeInternal, status, "decode", "missing messageIds"))
	}
	return res.MessageIds[0], nil
}

func (h *Host) QueueReceiveWire(queue string, maxMessages, waitSeconds, visibility int64) ([]RawMessage, error) {
	if maxMessages == 0 {
		maxMessages = 1
	}
	if waitSeconds == 0 {
		waitSeconds = 1
	}
	deadline := time.Now().Add(time.Duration(waitSeconds) * time.Second)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return []RawMessage{}, nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), remaining)
		req, _ := json.Marshal(map[string]any{"maxMessages": maxMessages, "returnImmediately": false})
		status, data, err := h.request(ctx, http.MethodPost, h.subPath(queue)+":pull", req, "application/json")
		cancel()
		if err != nil {
			if time.Now().Before(deadline) {
				continue
			}
			return []RawMessage{}, nil
		}
		if status < 200 || status >= 300 {
			return nil, h.setErr("queue_receive", queue, h.classify(status, data))
		}
		var pull struct {
			ReceivedMessages []struct {
				AckId   string `json:"ackId"`
				Message struct {
					Data string `json:"data"`
				} `json:"message"`
			} `json:"receivedMessages"`
		}
		if err := json.Unmarshal(data, &pull); err != nil {
			return nil, h.setErr("queue_receive", queue, newCloudError(CodeInternal, status, "decode", err.Error()))
		}
		if len(pull.ReceivedMessages) == 0 {
			if time.Now().Before(deadline) {
				time.Sleep(50 * time.Millisecond)
				continue
			}
			return []RawMessage{}, nil
		}
		ids := make([]string, 0, len(pull.ReceivedMessages))
		out := make([]RawMessage, 0, len(pull.ReceivedMessages))
		for _, rm := range pull.ReceivedMessages {
			ids = append(ids, rm.AckId)
			raw, _ := base64.StdEncoding.DecodeString(rm.Message.Data)
			out = append(out, RawMessage{ID: rm.AckId, Wire: string(raw), Receipt: rm.AckId})
		}
		mod, _ := json.Marshal(map[string]any{"ackIds": ids, "ackDeadlineSeconds": visibility})
		status, data, err = h.request(context.Background(), http.MethodPost, h.subPath(queue)+":modifyAckDeadline", mod, "application/json")
		if err != nil {
			return nil, h.setErr("queue_receive", queue, newCloudError(CodeInternal, 0, "transport", err.Error()))
		}
		if status < 200 || status >= 300 {
			return nil, h.setErr("queue_receive", queue, h.classify(status, data))
		}
		return out, nil
	}
}

// Op dispatches a registered Cloud operation.
func (h *Host) Op(op string, a []lmcode.Value) (lmcode.Value, error) {
	switch op {
	case "bucket_create":
		v, err := h.BucketCreate(a[0].Str)
		return lmcode.Bool(v), err
	case "bucket_delete":
		v, err := h.BucketDelete(a[0].Str)
		return lmcode.Bool(v), err
	case "bucket_exists":
		v, err := h.BucketExists(a[0].Str)
		return lmcode.Bool(v), err
	case "object_put":
		ct := "application/octet-stream"
		meta := map[string]string{}
		if len(a) == 5 {
			ct = a[3].Str
			meta = extractMeta(a[4])
		}
		m, err := h.ObjectPut(a[0].Str, a[1].Str, a[2].Str, ct, meta)
		if err != nil {
			return lmcode.None(), err
		}
		return lmcode.Object("ObjectMeta", m), nil
	case "object_get":
		v, err := h.ObjectGet(a[0].Str, a[1].Str)
		return lmcode.String(v), err
	case "object_head":
		m, err := h.ObjectHead(a[0].Str, a[1].Str)
		if err != nil {
			return lmcode.None(), err
		}
		return lmcode.Object("ObjectMeta", m), nil
	case "object_delete":
		return lmcode.None(), h.ObjectDelete(a[0].Str, a[1].Str)
	case "object_list":
		mk := int64(0)
		tok := ""
		if len(a) == 4 {
			mk = a[2].Int
			tok = a[3].Str
		}
		p, err := h.ObjectList(a[0].Str, a[1].Str, mk, tok)
		if err != nil {
			return lmcode.None(), err
		}
		return lmcode.Object("ObjectListPage", p), nil
	case "queue_create":
		v, err := h.QueueCreate(a[0].Str)
		return lmcode.Bool(v), err
	case "queue_delete":
		v, err := h.QueueDelete(a[0].Str)
		return lmcode.Bool(v), err
	case "queue_exists":
		v, err := h.QueueExists(a[0].Str)
		return lmcode.Bool(v), err
	case "queue_send":
		v, err := h.QueueSend(a[0].Str, a[1].Str)
		return lmcode.String(v), err
	case "queue_receive":
		mx, wait, vis := int64(1), int64(0), int64(30)
		if len(a) == 4 {
			mx = a[1].Int
			wait = a[2].Int
			vis = a[3].Int
		}
		v, err := h.QueueReceive(a[0].Str, mx, wait, vis)
		if err != nil {
			return lmcode.None(), err
		}
		return lmcode.Object("QueueMessageList", v), nil
	case "queue_ack":
		return lmcode.None(), h.QueueAck(a[0].Str, a[1].Str)
	case "queue_nack":
		v, err := h.QueueNack(a[0].Str, a[1].Str, a[2].Int)
		return lmcode.String(v), err
	}
	return lmcode.None(), fmt.Errorf("unimplemented op %s", op)
}
func extractMeta(v lmcode.Value) map[string]string {
	out := map[string]string{}
	if v.Obj == nil {
		return out
	}
	if m, ok := v.Obj.(*lmcode.StrMap); ok {
		for _, e := range m.Entries() {
			out[e.Key] = e.Value
		}
	}
	return out
}
