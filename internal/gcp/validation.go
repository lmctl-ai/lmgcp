package gcp

import "github.com/lmctl-ai/lmcloud/hostkit"

// Shared validation lives in hostkit. These names remain because the GCS wire
// helpers in ops.go call them; they only add provider-specific narrowing here.
func ValidateBucketName(string) *CloudError                      { return nil }
func ValidateObjectKey(string) *CloudError                       { return nil }
func ValidateContentType(string) *CloudError                     { return nil }
func ValidateMetadata(map[string]string) *CloudError             { return nil }
func ValidateListPrefix(string) *CloudError                      { return nil }
func ValidateQueueName(string) *CloudError                       { return nil }
func ValidateQueueBody(string) *CloudError                       { return nil }
func ValidateQueueReceiveParams(int64, int64, int64) *CloudError { return nil }
func ValidateQueueReceipt(string) *CloudError                    { return nil }
func ValidateQueueNackTimeout(int64) *CloudError                 { return nil }
func EncodeObjectKeyPath(key string) string                      { return hostkit.EscapeKeyWhole(key) }
func ValidateMaxKeys(int64) *CloudError                          { return nil }
func EncodeQueueBody(body string) string                         { return hostkit.EncodeQueueBody(body, false) }
func DecodeQueueBody(wire string) string {
	body, e := hostkit.DecodeQueueBody(wire)
	if e != nil {
		return wire
	}
	return body
}
