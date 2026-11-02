// SPDX-License-Identifier: MIT

package host

import (
	"reflect"
	"strings"
	"testing"

	"github.com/lmctl-ai/lmcode-go"
)

// TestSnapshotGettersDryParity covers every registered object snapshot getter:
// each must have a Dry callback, and dry/real outputs must be identical for a
// populated snapshot. This is the SolReviewer dry-registration audit regression.
func TestSnapshotGettersDryParity(t *testing.T) {
	samples := map[string]any{
		"ComputeInstance":    &ComputeInstance{},
		"VpcNetwork":         &VpcNetwork{},
		"Subnet":             &Subnet{},
		"FirewallRule":       &FirewallRule{},
		"StorageBucket":      &StorageBucket{},
		"PubsubTopic":        &PubsubTopic{},
		"PubsubSubscription": &PubsubSubscription{},
		"CloudRunService":    &CloudRunService{},
		"SqlInstance":        &SqlInstance{},
		"BqDataset":          &BqDataset{},
		"BqTable":            &BqTable{},
		"ServiceAccount":     &ServiceAccount{},
		"SecretMetadata":     &SecretMetadata{},
		"KmsKey":             &KmsKey{},
		"GkeCluster":         &GkeCluster{},
		"ArtifactRepo":       &ArtifactRepo{},
		"DnsZone":            &DnsZone{},
		"LogSink":            &LogSink{},
		"AlertPolicy":        &AlertPolicy{},
	}
	for _, sample := range samples {
		populateSnapshot(sample)
	}

	reg := lmcode.NewRegistry()
	h, err := NewHost(false)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	if err := Register(reg, h); err != nil {
		t.Fatal(err)
	}

	checked := 0
	for _, signature := range reg.Signatures() {
		if !strings.HasPrefix(signature, "method ") {
			continue
		}
		body := strings.TrimPrefix(signature, "method ")
		dot := strings.IndexByte(body, '.')
		if dot < 0 {
			continue
		}
		typeName := body[:dot]
		self, ok := samples[typeName]
		if !ok {
			continue
		}
		entry := reg.MethodBySignature(signature)
		if entry == nil {
			t.Fatalf("missing method entry %s", signature)
		}
		if entry.DryFn == nil {
			t.Fatalf("snapshot getter lacks Dry callback: %s", signature)
		}
		args := []lmcode.Value{}
		if len(entry.Sig.Params) == 1 && entry.Sig.Params[0] == "string" {
			args = []lmcode.Value{lmcode.Str("k")}
		}
		realValue, realErr := entry.Fn(self, args)
		dryValue, dryErr := entry.DryFn(self, args)
		if (realErr == nil) != (dryErr == nil) {
			t.Fatalf("%s error parity: real=%v dry=%v", signature, realErr, dryErr)
		}
		if realErr != nil {
			continue
		}
		if !sameValue(realValue, dryValue) {
			t.Fatalf("%s dry/real mismatch: real=%#v dry=%#v", signature, realValue, dryValue)
		}
		checked++
	}
	if checked < 161 {
		t.Fatalf("checked %d snapshot getters, want at least 161", checked)
	}
}

func populateSnapshot(value any) {
	rv := reflect.ValueOf(value).Elem()
	for i := 0; i < rv.NumField(); i++ {
		field := rv.Field(i)
		if !field.CanSet() {
			continue
		}
		switch field.Kind() {
		case reflect.String:
			field.SetString("v")
		case reflect.Int64:
			field.SetInt(7)
		case reflect.Bool:
			field.SetBool(false)
		case reflect.Map:
			m := reflect.MakeMap(field.Type())
			m.SetMapIndex(reflect.ValueOf("k").Convert(field.Type().Key()), reflect.ValueOf("v").Convert(field.Type().Elem()))
			field.Set(m)
		case reflect.Slice:
			slice := reflect.MakeSlice(field.Type(), 1, 1)
			slice.Index(0).Set(reflect.ValueOf("v").Convert(field.Type().Elem()))
			field.Set(slice)
		}
	}
}

func sameValue(a, b lmcode.Value) bool {
	if a.Kind != b.Kind || a.Type != b.Type {
		return false
	}
	switch a.Kind {
	case lmcode.KindVoid:
		return true
	case lmcode.KindInt:
		return a.Int == b.Int
	case lmcode.KindFloat:
		return a.Float == b.Float
	case lmcode.KindStr:
		return a.Str == b.Str
	case lmcode.KindBool:
		return a.Bool == b.Bool
	default:
		return reflect.DeepEqual(a.Obj, b.Obj)
	}
}
