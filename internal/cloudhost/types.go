package cloudhost

import "strings"

const (
	CodeOK              = "OK"
	CodeInvalid         = "INVALID_ARGUMENT"
	CodeNotFound        = "NOT_FOUND"
	CodeConflict        = "CONFLICT"
	CodePrecondition    = "PRECONDITION_FAILED"
	CodeUnauthenticated = "UNAUTHENTICATED"
	CodePermission      = "PERMISSION_DENIED"
	CodeInternal        = "INTERNAL"
)

type CloudError struct {
	OK           bool
	Code         string
	HTTPStatus   int
	ProviderCode string
	Message      string
}

func okError() *CloudError { return &CloudError{OK: true, Code: CodeOK} }

type ObjectMeta struct {
	Exists      bool
	Bucket      string
	Key         string
	Size        int64
	ETag        string
	ContentType string
	Metadata    map[string]string
}

type ObjectSummary struct {
	Key  string
	Size int64
	ETag string
}

type ObjectListPage struct {
	Items     []*ObjectSummary
	HasMore   bool
	NextToken string
}

type QueueMessage struct {
	ID      string
	Body    string
	Receipt string
}

type QueueMessageList struct{ Items []*QueueMessage }

func NormalizeETag(raw string) string {
	s := strings.TrimSpace(raw)
	if strings.HasPrefix(s, "W/") || strings.HasPrefix(s, "w/") {
		s = strings.TrimSpace(s[2:])
	}
	if len(s) >= 2 && strings.HasPrefix(s, `"`) && strings.HasSuffix(s, `"`) {
		s = s[1 : len(s)-1]
	}
	return s
}
