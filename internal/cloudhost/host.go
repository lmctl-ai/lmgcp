package cloudhost

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
)

type Host struct {
	mu           sync.Mutex
	abortOnError bool
	lastError    *CloudError
	endpoints    map[string]string
	defaultEP    string
	project      string
}

func New() *Host {
	def := strings.TrimSpace(os.Getenv("LMCLOUD_ENDPOINT"))
	if def == "" {
		def = strings.TrimSpace(os.Getenv("LMGCP_ENDPOINT"))
	}
	if def == "" {
		def = strings.TrimSpace(os.Getenv("STORAGE_EMULATOR_HOST"))
	}
	if def == "" {
		def = strings.TrimSpace(os.Getenv("PUBSUB_EMULATOR_HOST"))
	}
	project := strings.TrimSpace(os.Getenv("LMGCP_SIM_PROJECT"))
	if project == "" {
		project = strings.TrimSpace(os.Getenv("GOOGLE_CLOUD_PROJECT"))
	}
	if project == "" {
		project = "gcpsim-local"
	}
	return &Host{abortOnError: true, lastError: okError(), endpoints: map[string]string{}, defaultEP: strings.TrimRight(def, "/"), project: project}
}

func (h *Host) Provider() string   { return "gcp" }
func (h *Host) AbortOnError() bool { h.mu.Lock(); defer h.mu.Unlock(); return h.abortOnError }
func (h *Host) SetAbortOnError(v bool) *Host {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.abortOnError = v
	h.lastError = okError()
	return h
}
func (h *Host) LastError() *CloudError {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.lastError == nil {
		return okError()
	}
	return h.lastError
}

func validService(s string) bool {
	switch s {
	case "", "storage", "object-storage", "object_storage", "gcs", "objectstorage", "queue", "pubsub", "sqs", "all":
		return true
	}
	return false
}

func validateEndpoint(raw string) (string, *CloudError) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", &CloudError{Code: CodeInvalidArgument, Message: "invalid endpoint URL"}
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", &CloudError{Code: CodeInvalidArgument, Message: "endpoint must be http(s)"}
	}
	host := u.Hostname()
	loop := host == "127.0.0.1" || host == "localhost" || host == "::1"
	if u.Scheme == "http" && !loop {
		return "", &CloudError{Code: CodeInvalidArgument, Message: "plain http endpoint must be loopback"}
	}
	u.User = nil
	u.RawQuery = ""
	u.Fragment = ""
	return strings.TrimRight(u.String(), "/"), nil
}

func (h *Host) Endpoint(service string) (string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !validService(service) {
		e := &CloudError{Code: CodeInvalidArgument, Message: "unknown service"}
		h.lastError = e
		if h.abortOnError {
			return "", h.failLocked("endpoint", service, e)
		}
		return "", nil
	}
	return h.resolveLocked(service), nil
}

func (h *Host) SetEndpoint(service, raw string) (*Host, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !validService(service) {
		e := &CloudError{Code: CodeInvalidArgument, Message: "unknown service"}
		h.lastError = e
		if h.abortOnError {
			return h, h.failLocked("set_endpoint", service, e)
		}
		return h, nil
	}
	ep, cerr := validateEndpoint(raw)
	if cerr != nil {
		h.lastError = cerr
		if h.abortOnError {
			return h, h.failLocked("set_endpoint", raw, cerr)
		}
		return h, nil
	}
	h.endpoints[service] = ep
	if service == "" || service == "all" {
		h.defaultEP = ep
	}
	h.lastError = okError()
	return h, nil
}

func (h *Host) resolveLocked(service string) string {
	if ep := h.endpoints[service]; ep != "" {
		return ep
	}
	if ep := h.endpoints["all"]; ep != "" {
		return ep
	}
	switch service {
	case "storage", "object-storage", "object_storage", "gcs", "objectstorage":
		if ep := h.endpoints["storage"]; ep != "" {
			return ep
		}
	case "queue", "pubsub", "sqs":
		if ep := h.endpoints["queue"]; ep != "" {
			return ep
		}
	}
	if ep := h.endpoints[""]; ep != "" {
		return ep
	}
	return h.defaultEP
}

func (h *Host) failLocked(op, target string, e *CloudError) error {
	if e.Code == "" || e.Code == CodeOK {
		e.Code = CodeInternal
	}
	h.lastError = e
	return fmt.Errorf("[%s] %s (%s): %s (http=%d)", e.Code, op, target, e.Message, e.HTTPStatus)
}

func (h *Host) finishOK() *CloudError {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lastError = okError()
	return h.lastError
}
