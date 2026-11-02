package cloudhost

import (
	"fmt"
	"os"
	"sort"

	lmcode "github.com/lmctl-ai/lmcode-go"
)

func strList(reg *lmcode.Registry, values []string) (lmcode.Value, error) {
	ctor := reg.FunctionBySignature("fn str_list() -> StrList")
	add := reg.MethodBySignature("method StrList.add(string) -> StrList")
	if ctor == nil || add == nil {
		return lmcode.None(), fmt.Errorf("std StrList missing")
	}
	v, err := ctor.Fn(nil)
	if err != nil {
		return lmcode.None(), err
	}
	for _, s := range values {
		if v, err = add.Fn(v.Obj, []lmcode.Value{lmcode.String(s)}); err != nil {
			return lmcode.None(), err
		}
	}
	return v, nil
}

func strMap(reg *lmcode.Registry, m map[string]string) (lmcode.Value, error) {
	ctor := reg.FunctionBySignature("fn str_map() -> StrMap")
	set := reg.MethodBySignature("method StrMap.set(string, string) -> StrMap")
	if ctor == nil || set == nil {
		return lmcode.None(), fmt.Errorf("std StrMap missing")
	}
	v, err := ctor.Fn(nil)
	if err != nil {
		return lmcode.None(), err
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if v, err = set.Fn(v.Obj, []lmcode.Value{lmcode.String(k), lmcode.String(m[k])}); err != nil {
			return lmcode.None(), err
		}
	}
	return v, nil
}

func stub(op string) (lmcode.Value, error) {
	return lmcode.None(), fmt.Errorf("lmcloud phase A: %s not implemented", op)
}

func dryMethod(fn lmcode.Method) lmcode.DrySlot[lmcode.Method] { return lmcode.Dry(fn) }

func Register(reg *lmcode.Registry, h *Host) error {
	if h == nil {
		h = New()
	}
	if err := lmcode.RegisterTesting(reg, os.Stdout); err != nil {
		return err
	}
	if err := reg.Context("cloud", "Cloud"); err != nil {
		return err
	}
	must := func(err error) error {
		if err != nil {
			return err
		}
		return nil
	}
	_ = must

	printBool := lmcode.Function(func(a []lmcode.Value) (lmcode.Value, error) {
		fmt.Fprintln(os.Stdout, a[0].Bool)
		return lmcode.None(), nil
	})
	printFloat := lmcode.Function(func(a []lmcode.Value) (lmcode.Value, error) {
		fmt.Fprintln(os.Stdout, lmcode.FormatG15(a[0].Float))
		return lmcode.None(), nil
	})
	printInt := lmcode.Function(func(a []lmcode.Value) (lmcode.Value, error) {
		fmt.Fprintln(os.Stdout, a[0].Int)
		return lmcode.None(), nil
	})
	for _, f := range []struct {
		params []string
		fn     lmcode.Function
		name   string
	}{
		{[]string{"bool"}, printBool, "bool"}, {[]string{"float"}, printFloat, "float"}, {[]string{"int"}, printInt, "int"},
	} {
		if err := reg.Function("print", f.params, "void", f.fn, lmcode.Dry(f.fn)); err != nil {
			return err
		}
	}

	m := func(name string, params []string, ret string, fn lmcode.Method) error {
		return reg.Method("Cloud", name, params, ret, fn, lmcode.Dry(fn))
	}
	get := func(typ, name, ret string, field func(any) lmcode.Value) error {
		return reg.Method(typ, name, nil, ret, func(self any, _ []lmcode.Value) (lmcode.Value, error) { return field(self), nil }, dryMethod(func(self any, _ []lmcode.Value) (lmcode.Value, error) { return field(self), nil }))
	}
	if err := m("provider", nil, "string", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.String(self.(*Host).Provider()), nil
	}); err != nil {
		return err
	}
	if err := m("abort_on_error", nil, "bool", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(self.(*Host).AbortOnError()), nil
	}); err != nil {
		return err
	}
	if err := m("set_abort_on_error", []string{"bool"}, "Cloud", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Object("Cloud", self.(*Host).SetAbortOnError(a[0].Bool)), nil
	}); err != nil {
		return err
	}
	if err := m("endpoint", []string{"string"}, "string", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		ep, err := self.(*Host).Endpoint(a[0].Str)
		if err != nil {
			return lmcode.None(), err
		}
		return lmcode.String(ep), nil
	}); err != nil {
		return err
	}
	if err := m("set_endpoint", []string{"string", "string"}, "Cloud", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		c, err := self.(*Host).SetEndpoint(a[0].Str, a[1].Str)
		if err != nil {
			return lmcode.None(), err
		}
		return lmcode.Object("Cloud", c), nil
	}); err != nil {
		return err
	}
	if err := m("last_error", nil, "CloudError", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Object("CloudError", self.(*Host).LastError()), nil
	}); err != nil {
		return err
	}
	stubs := map[string][2]string{
		"bucket_create": {"string", "bool"}, "bucket_delete": {"string", "bool"}, "bucket_exists": {"string", "bool"},
		"object_get": {"string,string", "string"}, "object_delete": {"string,string", "void"},
		"queue_create": {"string", "bool"}, "queue_delete": {"string", "bool"}, "queue_exists": {"string", "bool"},
		"queue_send": {"string,string", "string"}, "queue_receive": {"string", "QueueMessageList"}, "queue_ack": {"string,string", "void"}, "queue_nack": {"string,string,int", "string"},
	}
	for name, sig := range stubs {
		var params []string
		if sig[0] != "" {
			params = splitParams(sig[0])
		}
		op := name
		if err := m(name, params, sig[1], func(_ any, a []lmcode.Value) (lmcode.Value, error) { return h.Op(op, a) }); err != nil {
			return err
		}
	}
	// overloads and typed stubs
	overloads := []struct {
		name   string
		params []string
		ret    string
	}{
		{"object_put", []string{"string", "string", "string"}, "ObjectMeta"},
		{"object_put", []string{"string", "string", "string", "string", "StrMap"}, "ObjectMeta"},
		{"object_head", []string{"string", "string"}, "ObjectMeta"},
		{"object_list", []string{"string", "string"}, "ObjectListPage"},
		{"object_list", []string{"string", "string", "int", "string"}, "ObjectListPage"},
		{"queue_receive", []string{"string", "int", "int", "int"}, "QueueMessageList"},
	}
	for _, o := range overloads {
		op := o.name
		if err := m(o.name, o.params, o.ret, func(_ any, a []lmcode.Value) (lmcode.Value, error) { return h.Op(op, a) }); err != nil {
			return err
		}
	}

	if err := m("bucket_list", nil, "StrList", func(_ any, _ []lmcode.Value) (lmcode.Value, error) {
		v, err := h.BucketList()
		if err != nil {
			return lmcode.None(), err
		}
		return strList(reg, v)
	}); err != nil {
		return err
	}
	if err := m("object_list_all", []string{"string", "string"}, "StrList", func(_ any, a []lmcode.Value) (lmcode.Value, error) {
		v, err := h.ObjectListAll(a[0].Str, a[1].Str)
		if err != nil {
			return lmcode.None(), err
		}
		return strList(reg, v)
	}); err != nil {
		return err
	}
	if err := m("queue_list", nil, "StrList", func(_ any, _ []lmcode.Value) (lmcode.Value, error) {
		v, err := h.QueueList()
		if err != nil {
			return lmcode.None(), err
		}
		return strList(reg, v)
	}); err != nil {
		return err
	}

	// Getter registrations.
	type getter struct {
		typ, name, ret string
		fn             func(any) lmcode.Value
	}
	gs := []getter{
		{"CloudError", "ok", "bool", func(a any) lmcode.Value { return lmcode.Bool(a.(*CloudError).OK) }},
		{"CloudError", "code", "ErrorCode", func(a any) lmcode.Value { return lmcode.String(a.(*CloudError).Code) }},
		{"CloudError", "http_status", "int", func(a any) lmcode.Value { return lmcode.Int(int64(a.(*CloudError).HTTPStatus)) }},
		{"CloudError", "provider_code", "string", func(a any) lmcode.Value { return lmcode.String(a.(*CloudError).ProviderCode) }},
		{"CloudError", "message", "string", func(a any) lmcode.Value { return lmcode.String(a.(*CloudError).Message) }},
		{"ObjectMeta", "exists", "bool", func(a any) lmcode.Value { return lmcode.Bool(a.(*ObjectMeta).Exists) }},
		{"ObjectMeta", "bucket", "string", func(a any) lmcode.Value { return lmcode.String(a.(*ObjectMeta).Bucket) }},
		{"ObjectMeta", "key", "string", func(a any) lmcode.Value { return lmcode.String(a.(*ObjectMeta).Key) }},
		{"ObjectMeta", "size", "int", func(a any) lmcode.Value { return lmcode.Int(a.(*ObjectMeta).Size) }},
		{"ObjectMeta", "etag", "string", func(a any) lmcode.Value { return lmcode.String(a.(*ObjectMeta).ETag) }},
		{"ObjectMeta", "content_type", "string", func(a any) lmcode.Value { return lmcode.String(a.(*ObjectMeta).ContentType) }},
		{"ObjectSummary", "key", "string", func(a any) lmcode.Value { return lmcode.String(a.(*ObjectSummary).Key) }},
		{"ObjectSummary", "size", "int", func(a any) lmcode.Value { return lmcode.Int(a.(*ObjectSummary).Size) }},
		{"ObjectSummary", "etag", "string", func(a any) lmcode.Value { return lmcode.String(a.(*ObjectSummary).ETag) }},
		{"ObjectListPage", "len", "int", func(a any) lmcode.Value { return lmcode.Int(int64(len(a.(*ObjectListPage).Items))) }},
		{"ObjectListPage", "has_more", "bool", func(a any) lmcode.Value { return lmcode.Bool(a.(*ObjectListPage).HasMore) }},
		{"ObjectListPage", "next_token", "string", func(a any) lmcode.Value { return lmcode.String(a.(*ObjectListPage).NextToken) }},
		{"QueueMessage", "id", "string", func(a any) lmcode.Value { return lmcode.String(a.(*QueueMessage).ID) }},
		{"QueueMessage", "body", "string", func(a any) lmcode.Value { return lmcode.String(a.(*QueueMessage).Body) }},
		{"QueueMessage", "receipt", "string", func(a any) lmcode.Value { return lmcode.String(a.(*QueueMessage).Receipt) }},
		{"QueueMessageList", "len", "int", func(a any) lmcode.Value { return lmcode.Int(int64(len(a.(*QueueMessageList).Items))) }},
	}
	for _, g := range gs {
		if err := get(g.typ, g.name, g.ret, g.fn); err != nil {
			return err
		}
	}
	// ObjectMeta.metadata returns StrMap.
	if err := reg.Method("ObjectMeta", "metadata", nil, "StrMap", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return strMap(reg, self.(*ObjectMeta).Metadata)
	}, dryMethod(func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return strMap(reg, self.(*ObjectMeta).Metadata)
	})); err != nil {
		return err
	}
	if err := reg.Method("ObjectListPage", "keys", nil, "StrList", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		p := self.(*ObjectListPage)
		out := make([]string, 0, len(p.Items))
		for _, it := range p.Items {
			out = append(out, it.Key)
		}
		return strList(reg, out)
	}, dryMethod(func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		p := self.(*ObjectListPage)
		out := make([]string, 0, len(p.Items))
		for _, it := range p.Items {
			out = append(out, it.Key)
		}
		return strList(reg, out)
	})); err != nil {
		return err
	}
	// at() methods
	if err := reg.Method("ObjectListPage", "at", []string{"int"}, "ObjectSummary", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		p := self.(*ObjectListPage)
		i := a[0].Int
		if i < 0 || i >= int64(len(p.Items)) {
			return lmcode.None(), fmt.Errorf("index out of range")
		}
		return lmcode.Object("ObjectSummary", p.Items[i]), nil
	}); err != nil {
		return err
	}
	if err := reg.Method("QueueMessageList", "at", []string{"int"}, "QueueMessage", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		p := self.(*QueueMessageList)
		i := a[0].Int
		if i < 0 || i >= int64(len(p.Items)) {
			return lmcode.None(), fmt.Errorf("index out of range")
		}
		return lmcode.Object("QueueMessage", p.Items[i]), nil
	}); err != nil {
		return err
	}
	_ = h
	return nil
}

func splitParams(s string) []string {
	if s == "" {
		return nil
	}
	out := []string{}
	for _, p := range splitComma(s) {
		out = append(out, p)
	}
	return out
}
func splitComma(s string) []string {
	var out []string
	cur := ""
	for _, r := range s {
		if r == ',' {
			if cur != "" {
				out = append(out, cur)
				cur = ""
			}
		} else if r != ' ' {
			cur += string(r)
		}
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}
