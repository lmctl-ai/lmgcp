// SPDX-License-Identifier: MIT

package gcp

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/lmctl-ai/lmcloud/hostkit"
)

const (
	firestoreCatalogCollection = "lmcloudmeta"

	tablePKAttr      = "pk"
	tableSKAttr      = "sk"
	tableVersionAttr = "__lmcloud_v"

	maxUpdateCASAttempts = 8
)

type tableLayoutCache struct {
	mu      sync.RWMutex
	layouts map[string]bool
}

func newTableLayoutCache() *tableLayoutCache {
	return &tableLayoutCache{layouts: make(map[string]bool)}
}

func (c *tableLayoutCache) key(target, project, table string) string {
	return target + "|" + project + "|" + table
}

func (c *tableLayoutCache) get(target, project, table string) (bool, bool) {
	if c == nil {
		return false, false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	sorted, ok := c.layouts[c.key(target, project, table)]
	return sorted, ok
}

func (c *tableLayoutCache) set(target, project, table string, sorted bool) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.layouts[c.key(target, project, table)] = sorted
}

func (c *tableLayoutCache) del(target, project, table string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.layouts, c.key(target, project, table))
}

func newRandomHexToken() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", hostkit.ProviderError(hostkit.Internal, 0, "EntropyError", err.Error())
	}
	return hex.EncodeToString(b[:]), nil
}

type catalogEntry struct {
	sorted      bool
	scope       string
	createToken string
	incarnation string
	deleting    bool
}

func (h *Host) parseCatalogDoc(doc fsDocument) (catalogEntry, error) {
	var entry catalogEntry
	if st, ok := doc.Fields["state"]; ok {
		if st.Kind != fsKindString {
			return catalogEntry{}, hostkit.ProviderError(hostkit.Conflict, 409, "IncompatibleTableSchema", "Firestore catalog document has non-string state field")
		}
		switch st.StrVal {
		case "active":
		case "deleting":
			entry.deleting = true
		default:
			return catalogEntry{}, hostkit.ProviderError(hostkit.Conflict, 409, "IncompatibleTableSchema", fmt.Sprintf("Firestore catalog document has unsupported state %q", st.StrVal))
		}
	}

	sc, hasScope := doc.Fields["scope"]
	if !hasScope || sc.Kind != fsKindString || sc.StrVal == "" {
		return catalogEntry{}, hostkit.ProviderError(hostkit.Conflict, 409, "IncompatibleTableSchema", "Firestore catalog document is missing scope field")
	}
	if sc.StrVal != h.project {
		return catalogEntry{}, hostkit.ProviderError(hostkit.Conflict, 409, "TableScopeMismatch", fmt.Sprintf("Firestore table belongs to scope %q, expected %q", sc.StrVal, h.project))
	}
	entry.scope = sc.StrVal

	lay, hasLayout := doc.Fields["layout"]
	if !hasLayout || lay.Kind != fsKindString {
		return catalogEntry{}, hostkit.ProviderError(hostkit.Conflict, 409, "IncompatibleTableSchema", "Firestore catalog document is missing layout field")
	}
	switch lay.StrVal {
	case "key":
		entry.sorted = false
	case "sorted":
		entry.sorted = true
	default:
		return catalogEntry{}, hostkit.ProviderError(hostkit.Conflict, 409, "IncompatibleTableSchema", fmt.Sprintf("Firestore catalog document has unsupported layout %q", lay.StrVal))
	}

	if tok, ok := doc.Fields["token"]; ok && tok.Kind == fsKindString {
		entry.createToken = tok.StrVal
	}
	if inc, ok := doc.Fields["incarnation"]; ok && inc.Kind == fsKindString && inc.StrVal != "" {
		entry.incarnation = inc.StrVal
	} else if !doc.CreateTime.IsZero() {
		entry.incarnation = fmt.Sprintf("ts:%d.%09d", doc.CreateTime.Seconds, doc.CreateTime.Nanos)
	}
	return entry, nil
}

func (h *Host) describeTableCatalog(ctx context.Context, name string) (catalogEntry, error) {
	req := &fsGetDocumentRequest{
		Name: h.firestoreDocPath(firestoreCatalogCollection, name),
	}
	var doc fsDocument
	if err := h.firestoreUnary(ctx, rpcGetDocument, true, req, &doc); err != nil {
		var ce *hostkit.CloudError
		if errors.As(err, &ce) && ce.Code == hostkit.NotFound {
			return catalogEntry{}, ce.WithAbsent(hostkit.AbsentTable)
		}
		return catalogEntry{}, err
	}
	return h.parseCatalogDoc(doc)
}

func (h *Host) cacheTableLayout(name string, sorted bool) {
	if target, err := h.firestoreTarget(); err == nil {
		h.tableLayouts.set(target, h.project, name, sorted)
	}
}

func (h *Host) evictTableLayout(name string) {
	if target, err := h.firestoreTarget(); err == nil {
		h.tableLayouts.del(target, h.project, name)
	}
}

func (h *Host) lookupCachedLayout(name string) (bool, bool) {
	target, err := h.firestoreTarget()
	if err != nil {
		return false, false
	}
	return h.tableLayouts.get(target, h.project, name)
}

func (h *Host) ensureTableLayout(ctx context.Context, name string) (bool, error) {
	if sorted, ok := h.lookupCachedLayout(name); ok {
		return sorted, nil
	}
	cat, err := h.describeTableCatalog(ctx, name)
	if err != nil {
		return false, err
	}
	if cat.deleting {
		h.evictTableLayout(name)
		return false, hostkit.ProviderError(hostkit.NotFound, 404, "TableNotFound", "table is being deleted").WithAbsent(hostkit.AbsentTable)
	}
	h.cacheTableLayout(name, cat.sorted)
	return cat.sorted, nil
}

func validateKeyLayout(table string, sorted bool, sk string, preflight bool) error {
	if sorted && sk == "" {
		if preflight {
			return hostkit.Preflightf("sorted table %q requires a non-empty sort key", table)
		}
		return hostkit.ProviderError(hostkit.InvalidArgument, 0, "InvalidKeyLayout", fmt.Sprintf("sorted table %q requires a non-empty sort key", table))
	}
	if !sorted && sk != "" {
		if preflight {
			return hostkit.Preflightf("key-only table %q does not accept a sort key", table)
		}
		return hostkit.ProviderError(hostkit.InvalidArgument, 0, "InvalidKeyLayout", fmt.Sprintf("key-only table %q does not accept a sort key", table))
	}
	return nil
}

func (h *Host) CreateTable(ctx context.Context, name string, sorted bool) (bool, error) {
	incarnation, err := newRandomHexToken()
	if err != nil {
		return false, err
	}
	layout := "key"
	if sorted {
		layout = "sorted"
	}
	createToken := hostkit.CreateToken(ctx)
	if createToken == "" {
		createToken = h.createToken
	}
	fields := map[string]fsValue{
		"layout":      fsString(layout),
		"scope":       fsString(h.project),
		"incarnation": fsString(incarnation),
		"state":       fsString("active"),
		"created":     fsInt(time.Now().Unix()),
	}
	if createToken != "" {
		fields["token"] = fsString(createToken)
	}
	req := &fsCreateDocumentRequest{
		Parent:       h.firestoreDocumentsRoot(),
		CollectionId: firestoreCatalogCollection,
		DocumentId:   name,
		Document: fsDocument{
			Fields: fields,
		},
	}
	var createdDoc fsDocument
	err = h.firestoreUnary(ctx, rpcCreateDocument, createToken != "", req, &createdDoc)
	if err == nil {
		h.cacheTableLayout(name, sorted)
		return true, nil
	}
	var ce *hostkit.CloudError
	if !errors.As(err, &ce) || ce.Code != hostkit.Conflict {
		return false, err
	}

	// Table catalog doc already exists: inspect it via an admitted GetDocument
	// so layout mismatches, deleting state, or foreign scopes fail with CONFLICT,
	// while matching layout returns (false, nil) for cloudhost token comparison.
	cat, derr := h.describeTableCatalog(ctx, name)
	if derr != nil {
		var dce *hostkit.CloudError
		if errors.As(derr, &dce) {
			switch dce.Code {
			case hostkit.PermissionDenied, hostkit.Unauthenticated, hostkit.Conflict, hostkit.CostCapExceeded:
				return false, derr
			case hostkit.NotFound:
				return false, hostkit.ProviderError(hostkit.Conflict, 409, "TableInTransition", "table already exists or is in transition")
			}
		}
		return false, derr
	}
	if cat.deleting {
		return false, hostkit.ProviderError(hostkit.Conflict, 409, "TableDeleting", "table is currently being deleted")
	}
	if cat.sorted != sorted {
		return false, hostkit.ProviderError(hostkit.Conflict, 409, "LayoutMismatch", "table already exists with a different key layout")
	}
	h.cacheTableLayout(name, cat.sorted)
	return false, nil
}

func (h *Host) TableCreateToken(ctx context.Context, name string) (string, error) {
	cat, err := h.describeTableCatalog(ctx, name)
	if err != nil {
		return "", err
	}
	if cat.deleting {
		h.evictTableLayout(name)
		return "", hostkit.ProviderError(hostkit.NotFound, 404, "TableNotFound", "table not found").WithAbsent(hostkit.AbsentTable)
	}
	h.cacheTableLayout(name, cat.sorted)
	return cat.createToken, nil
}

func (h *Host) HeadTable(ctx context.Context, name string) error {
	cat, err := h.describeTableCatalog(ctx, name)
	if err != nil {
		return err
	}
	if cat.deleting {
		h.evictTableLayout(name)
		return hostkit.ProviderError(hostkit.NotFound, 404, "TableNotFound", "table not found").WithAbsent(hostkit.AbsentTable)
	}
	h.cacheTableLayout(name, cat.sorted)
	return nil
}

func (h *Host) DescribeTable(ctx context.Context, name string) (hostkit.TableInfo, error) {
	cat, err := h.describeTableCatalog(ctx, name)
	if err != nil {
		return hostkit.TableInfo{}, err
	}
	if cat.deleting {
		h.evictTableLayout(name)
		return hostkit.TableInfo{}, hostkit.ProviderError(hostkit.NotFound, 404, "TableNotFound", "table not found").WithAbsent(hostkit.AbsentTable)
	}
	if strings.TrimSpace(cat.incarnation) == "" {
		return hostkit.TableInfo{}, hostkit.ProviderError(hostkit.Internal, 0, "MissingTableIncarnationToken", "Firestore catalog document is missing incarnation token")
	}
	h.cacheTableLayout(name, cat.sorted)
	return hostkit.TableInfo{
		Sorted:      cat.sorted,
		CreateToken: cat.incarnation,
	}, nil
}

func (h *Host) DeleteTable(ctx context.Context, name string) error {
	h.evictTableLayout(name)
	catDocPath := h.firestoreDocPath(firestoreCatalogCollection, name)

	var catDoc fsDocument
	if err := h.firestoreUnary(ctx, rpcGetDocument, true, &fsGetDocumentRequest{Name: catDocPath}, &catDoc); err != nil {
		var ce *hostkit.CloudError
		if errors.As(err, &ce) && ce.Code == hostkit.NotFound {
			return ce.WithAbsent(hostkit.AbsentTable)
		}
		return err
	}
	cat, err := h.parseCatalogDoc(catDoc)
	if err != nil {
		return err
	}
	if cat.deleting {
		return hostkit.ProviderError(hostkit.NotFound, 404, "TableNotFound", "table not found").WithAbsent(hostkit.AbsentTable)
	}

	// Fence concurrent callers by marking the catalog entry as deleting before
	// purging child documents in collection {name}.
	catDoc.Fields["state"] = fsString("deleting")
	var updatedCat fsDocument
	if err := h.firestoreUnary(ctx, rpcUpdateDocument, true, &fsUpdateDocumentRequest{
		Document: catDoc,
		UpdateMask: &fsDocumentMask{
			FieldPaths: []string{"state"},
		},
		CurrentDocument: &fsPrecondition{HasExists: true, Exists: true},
	}, &updatedCat); err != nil {
		var ce *hostkit.CloudError
		if errors.As(err, &ce) && ce.Code == hostkit.NotFound {
			return ce.WithAbsent(hostkit.AbsentTable)
		}
		return err
	}

	// Purge all child documents in collection {name} via bounded RunQuery batches.
	const purgeBatchSize = 100
	for {
		docs, err := h.firestoreRunQuery(ctx, &fsRunQueryRequest{
			Parent: h.firestoreDocumentsRoot(),
			StructuredQuery: fsStructuredQuery{
				CollectionId: name,
				OrderBy: []fsOrder{
					{FieldPath: "__name__", Direction: fsDirectionAscending},
				},
				Limit: purgeBatchSize,
			},
		}, purgeBatchSize)
		if err != nil {
			return err
		}
		if len(docs) == 0 {
			break
		}
		for _, d := range docs {
			var empty fsEmpty
			if err := h.firestoreUnary(ctx, rpcDeleteDocument, true, &fsDeleteDocumentRequest{Name: d.Name}, &empty); err != nil {
				var ce *hostkit.CloudError
				if errors.As(err, &ce) && ce.Code == hostkit.NotFound {
					continue
				}
				return err
			}
		}
		if len(docs) < purgeBatchSize {
			break
		}
	}

	var empty fsEmpty
	if err := h.firestoreUnary(ctx, rpcDeleteDocument, true, &fsDeleteDocumentRequest{Name: catDocPath}, &empty); err != nil {
		var ce *hostkit.CloudError
		if errors.As(err, &ce) && ce.Code == hostkit.NotFound {
			return nil
		}
		return err
	}
	return nil
}

func docBaseName(docPath string) string {
	idx := strings.LastIndexByte(docPath, '/')
	if idx < 0 {
		return docPath
	}
	return docPath[idx+1:]
}

func (h *Host) ListTables(ctx context.Context, token string) ([]string, string, error) {
	req := &fsListDocumentsRequest{
		Parent:       h.firestoreDocumentsRoot(),
		CollectionId: firestoreCatalogCollection,
		PageSize:     100,
		PageToken:    token,
	}
	var resp fsListDocumentsResponse
	if err := h.firestoreUnary(ctx, rpcListDocuments, true, req, &resp); err != nil {
		return nil, "", err
	}
	names := make([]string, 0, len(resp.Documents))
	for _, doc := range resp.Documents {
		cat, err := h.parseCatalogDoc(doc)
		if err != nil || cat.deleting {
			continue
		}
		name := docBaseName(doc.Name)
		if name != "" {
			h.cacheTableLayout(name, cat.sorted)
			names = append(names, name)
		}
	}
	if token == "" && resp.NextPageToken == "" {
		sort.Strings(names)
	}
	return names, resp.NextPageToken, nil
}

func firestoreDocID(sorted bool, pk, sk string) string {
	if sorted {
		return pk + "#" + sk
	}
	return pk
}

func encodeFirestoreFields(item hostkit.Item, version string) (map[string]fsValue, error) {
	fields := make(map[string]fsValue, len(item.Attrs)+3)
	fields[tablePKAttr] = fsString(item.PK)
	if item.SK != "" {
		fields[tableSKAttr] = fsString(item.SK)
	}
	fields[tableVersionAttr] = fsString(version)
	for k, v := range item.Attrs {
		switch {
		case v.S != nil:
			fields[k] = fsString(*v.S)
		case v.I != nil:
			fields[k] = fsInt(*v.I)
		case v.N != nil:
			fields[k] = fsDouble(*v.N)
		case v.B != nil:
			fields[k] = fsBool(*v.B)
		default:
			return nil, hostkit.ProviderError(hostkit.InvalidArgument, 0, "InvalidAttributeType", fmt.Sprintf("unsupported attribute kind for %q", k))
		}
	}
	return fields, nil
}

func decodeFirestoreDoc(doc fsDocument) (hostkit.Item, bool, error) {
	pkVal, hasPK := doc.Fields[tablePKAttr]
	if !hasPK || pkVal.Kind != fsKindString || pkVal.StrVal == "" {
		return hostkit.Item{}, false, hostkit.ProviderError(hostkit.Internal, 0, "CorruptItem", "Firestore document is missing string pk field")
	}
	var sk string
	var hasSK bool
	if skVal, ok := doc.Fields[tableSKAttr]; ok {
		if skVal.Kind != fsKindString || skVal.StrVal == "" {
			return hostkit.Item{}, false, hostkit.ProviderError(hostkit.Internal, 0, "CorruptItem", "Firestore document has invalid sk field")
		}
		sk = skVal.StrVal
		hasSK = true
	}
	vVal, hasVer := doc.Fields[tableVersionAttr]
	if !hasVer || vVal.Kind != fsKindString || vVal.StrVal == "" {
		return hostkit.Item{}, false, hostkit.ProviderError(hostkit.Internal, 0, "CorruptItem", "Firestore document is missing __lmcloud_v version field")
	}
	attrs := make(map[string]hostkit.Attr)
	for k, fv := range doc.Fields {
		if k == tablePKAttr || k == tableSKAttr || k == tableVersionAttr || strings.HasPrefix(k, "__lmcloud_") {
			continue
		}
		switch fv.Kind {
		case fsKindString:
			attrs[k] = hostkit.StringAttr(fv.StrVal)
		case fsKindInt:
			attrs[k] = hostkit.IntAttr(fv.IntVal)
		case fsKindDouble:
			attrs[k] = hostkit.NumberAttr(fv.DoubleVal)
		case fsKindBool:
			attrs[k] = hostkit.BoolAttr(fv.BoolVal)
		default:
			return hostkit.Item{}, false, hostkit.ProviderError(hostkit.Internal, 0, "CorruptItem", fmt.Sprintf("Firestore document attribute %q has unsupported wire type", k))
		}
	}
	return hostkit.Item{
		PK:      pkVal.StrVal,
		SK:      sk,
		Version: vVal.StrVal,
		Exists:  true,
		Attrs:   attrs,
	}, hasSK, nil
}

func docPrecondition(doc fsDocument) *fsPrecondition {
	if !doc.UpdateTime.IsZero() {
		return &fsPrecondition{
			HasUpdateTime: true,
			UpdateTime:    doc.UpdateTime,
		}
	}
	return &fsPrecondition{
		HasExists: true,
		Exists:    true,
	}
}

func (h *Host) PutItem(ctx context.Context, table string, item hostkit.Item, mode hostkit.PutMode, expectVersion string) (hostkit.Item, error) {
	sorted, cached := h.lookupCachedLayout(table)
	if cached {
		if err := validateKeyLayout(table, sorted, item.SK, true); err != nil {
			return hostkit.Item{}, err
		}
	} else if mode != hostkit.PutIfVersion {
		var err error
		sorted, err = h.ensureTableLayout(ctx, table)
		if err != nil {
			return hostkit.Item{}, err
		}
		if err := validateKeyLayout(table, sorted, item.SK, false); err != nil {
			return hostkit.Item{}, err
		}
		cached = true
	} else {
		sorted = item.SK != ""
	}

	newVer, err := newRandomHexToken()
	if err != nil {
		return hostkit.Item{}, err
	}
	fields, err := encodeFirestoreFields(item, newVer)
	if err != nil {
		return hostkit.Item{}, err
	}
	if _, cerr := hostkit.CanonicalItemBytes(item.Attrs); cerr != nil {
		return hostkit.Item{}, cerr
	}
	stored := item.Clone()
	stored.Version = newVer
	stored.Exists = true
	newBill := hostkit.NativeItemBillableBytes(stored)
	docID := firestoreDocID(sorted, item.PK, item.SK)
	docPath := h.firestoreDocPath(table, docID)

	switch mode {
	case hostkit.PutIfAbsent:
		req := &fsCreateDocumentRequest{
			Parent:       h.firestoreDocumentsRoot(),
			CollectionId: table,
			DocumentId:   docID,
			Document: fsDocument{
				Fields: fields,
			},
		}
		var outDoc fsDocument
		if err := h.firestoreUnary(ctx, rpcCreateDocument, false, req, &outDoc); err != nil {
			return hostkit.Item{}, err
		}
		hostkit.ReportTableOpMetrics(ctx, hostkit.TableOpMetrics{
			PriorExists:   false,
			PriorBytes:    0,
			NewBytes:      newBill,
			Authoritative: true,
		})
		return stored, nil

	case hostkit.PutUnconditional:
		req := &fsUpdateDocumentRequest{
			Document: fsDocument{
				Name:   docPath,
				Fields: fields,
			},
		}
		var outDoc fsDocument
		if err := h.firestoreUnary(ctx, rpcUpdateDocument, false, req, &outDoc); err != nil {
			return hostkit.Item{}, err
		}
		hostkit.ReportTableOpMetrics(ctx, hostkit.TableOpMetrics{
			PriorExists:   false,
			PriorBytes:    0,
			NewBytes:      newBill,
			Authoritative: false,
		})
		return stored, nil

	case hostkit.PutIfVersion:
		var curDoc fsDocument
		if err := h.firestoreUnary(ctx, rpcGetDocument, true, &fsGetDocumentRequest{Name: docPath}, &curDoc); err != nil {
			var ce *hostkit.CloudError
			if errors.As(err, &ce) && ce.Code == hostkit.NotFound {
				if !cached {
					actualSorted, terr := h.ensureTableLayout(ctx, table)
					if terr != nil {
						return hostkit.Item{}, terr
					}
					if kerr := validateKeyLayout(table, actualSorted, item.SK, false); kerr != nil {
						return hostkit.Item{}, kerr
					}
				}
				return hostkit.Item{}, ce.WithAbsent(hostkit.AbsentItem)
			}
			return hostkit.Item{}, err
		}
		priorItem, hasSK, err := decodeFirestoreDoc(curDoc)
		if err != nil {
			return hostkit.Item{}, err
		}
		h.cacheTableLayout(table, hasSK)
		if err := validateKeyLayout(table, hasSK, item.SK, false); err != nil {
			return hostkit.Item{}, err
		}
		if priorItem.Version != expectVersion {
			return hostkit.Item{}, hostkit.ProviderError(hostkit.PreconditionFailed, 412, "VersionMismatch", "expected version does not match current item version")
		}
		if _, cerr := hostkit.CanonicalItemBytes(priorItem.Attrs); cerr != nil {
			return hostkit.Item{}, cerr
		}
		priorBill := hostkit.NativeItemBillableBytes(priorItem)

		req := &fsUpdateDocumentRequest{
			Document: fsDocument{
				Name:   docPath,
				Fields: fields,
			},
			CurrentDocument: docPrecondition(curDoc),
		}
		var outDoc fsDocument
		if err := h.firestoreUnary(ctx, rpcUpdateDocument, false, req, &outDoc); err != nil {
			var ce *hostkit.CloudError
			if errors.As(err, &ce) && ce.Code == hostkit.NotFound {
				return hostkit.Item{}, ce.WithAbsent(hostkit.AbsentItem)
			}
			return hostkit.Item{}, err
		}
		hostkit.ReportTableOpMetrics(ctx, hostkit.TableOpMetrics{
			PriorExists:   true,
			PriorBytes:    priorBill,
			NewBytes:      newBill,
			Authoritative: false,
		})
		return stored, nil

	default:
		return hostkit.Item{}, hostkit.ProviderError(hostkit.InvalidArgument, 0, "InvalidPutMode", "unsupported put mode")
	}
}

func (h *Host) GetItem(ctx context.Context, table, pk, sk string) (hostkit.Item, error) {
	sorted, cached := h.lookupCachedLayout(table)
	if cached {
		if err := validateKeyLayout(table, sorted, sk, true); err != nil {
			return hostkit.Item{}, err
		}
	} else {
		sorted = sk != ""
	}

	docID := firestoreDocID(sorted, pk, sk)
	var doc fsDocument
	if err := h.firestoreUnary(ctx, rpcGetDocument, true, &fsGetDocumentRequest{
		Name: h.firestoreDocPath(table, docID),
	}, &doc); err != nil {
		var ce *hostkit.CloudError
		if errors.As(err, &ce) && ce.Code == hostkit.NotFound {
			if !cached {
				actualSorted, terr := h.ensureTableLayout(ctx, table)
				if terr != nil {
					return hostkit.Item{}, terr
				}
				if kerr := validateKeyLayout(table, actualSorted, sk, false); kerr != nil {
					return hostkit.Item{}, kerr
				}
			}
			return hostkit.Item{}, ce.WithAbsent(hostkit.AbsentItem)
		}
		return hostkit.Item{}, err
	}

	item, hasSK, err := decodeFirestoreDoc(doc)
	if err != nil {
		return hostkit.Item{}, err
	}
	h.cacheTableLayout(table, hasSK)
	if err := validateKeyLayout(table, hasSK, sk, false); err != nil {
		return hostkit.Item{}, err
	}
	return item, nil
}

func (h *Host) DeleteItem(ctx context.Context, table, pk, sk string) error {
	sorted, cached := h.lookupCachedLayout(table)
	if cached {
		if err := validateKeyLayout(table, sorted, sk, true); err != nil {
			return err
		}
	} else {
		var err error
		sorted, err = h.ensureTableLayout(ctx, table)
		if err != nil {
			return err
		}
		if err := validateKeyLayout(table, sorted, sk, false); err != nil {
			return err
		}
		cached = true
	}

	docID := firestoreDocID(sorted, pk, sk)
	var empty fsEmpty
	if err := h.firestoreUnary(ctx, rpcDeleteDocument, true, &fsDeleteDocumentRequest{
		Name:            h.firestoreDocPath(table, docID),
		CurrentDocument: &fsPrecondition{HasExists: true, Exists: true},
	}, &empty); err != nil {
		var ce *hostkit.CloudError
		if errors.As(err, &ce) && ce.Code == hostkit.NotFound {
			return ce.WithAbsent(hostkit.AbsentItem)
		}
		return err
	}
	return nil
}

func (h *Host) UpdateItem(ctx context.Context, table, pk, sk string, changes hostkit.Item) (hostkit.Item, error) {
	sorted, cached := h.lookupCachedLayout(table)
	if cached {
		if err := validateKeyLayout(table, sorted, sk, true); err != nil {
			return hostkit.Item{}, err
		}
	} else {
		sorted = sk != ""
	}

	docID := firestoreDocID(sorted, pk, sk)
	docPath := h.firestoreDocPath(table, docID)

	for attempt := 0; attempt < maxUpdateCASAttempts; attempt++ {
		var curDoc fsDocument
		if err := h.firestoreUnary(ctx, rpcGetDocument, true, &fsGetDocumentRequest{Name: docPath}, &curDoc); err != nil {
			var ce *hostkit.CloudError
			if errors.As(err, &ce) && ce.Code == hostkit.NotFound {
				if !cached {
					actualSorted, terr := h.ensureTableLayout(ctx, table)
					if terr != nil {
						return hostkit.Item{}, terr
					}
					if kerr := validateKeyLayout(table, actualSorted, sk, false); kerr != nil {
						return hostkit.Item{}, kerr
					}
				}
				return hostkit.Item{}, ce.WithAbsent(hostkit.AbsentItem)
			}
			return hostkit.Item{}, err
		}

		priorItem, hasSK, err := decodeFirestoreDoc(curDoc)
		if err != nil {
			return hostkit.Item{}, err
		}
		h.cacheTableLayout(table, hasSK)
		cached = true
		if err := validateKeyLayout(table, hasSK, sk, false); err != nil {
			return hostkit.Item{}, err
		}
		if _, cerr := hostkit.CanonicalItemBytes(priorItem.Attrs); cerr != nil {
			return hostkit.Item{}, cerr
		}
		priorBill := hostkit.NativeItemBillableBytes(priorItem)

		mergedAttrs := make(map[string]hostkit.Attr, len(priorItem.Attrs)+len(changes.Attrs))
		for k, v := range priorItem.Attrs {
			mergedAttrs[k] = v.Clone()
		}
		for k, v := range changes.Attrs {
			mergedAttrs[k] = v.Clone()
		}

		// Validate the merged item's attribute count (<= 240) and canonical size
		// (<= 262,144 bytes) BEFORE writing so an over-limit update never mutates
		// the stored document or its version token.
		if _, cerr := hostkit.CanonicalItemBytes(mergedAttrs); cerr != nil {
			return hostkit.Item{}, cerr
		}

		newVer, err := newRandomHexToken()
		if err != nil {
			return hostkit.Item{}, err
		}
		mergedItem := hostkit.Item{
			PK:      pk,
			SK:      sk,
			Version: newVer,
			Exists:  true,
			Attrs:   mergedAttrs,
		}
		newBill := hostkit.NativeItemBillableBytes(mergedItem)
		patchFields, err := encodeFirestoreFields(hostkit.Item{
			PK:    pk,
			SK:    sk,
			Attrs: changes.Attrs,
		}, newVer)
		if err != nil {
			return hostkit.Item{}, err
		}
		maskPaths := make([]string, 0, len(changes.Attrs)+1)
		for k := range changes.Attrs {
			maskPaths = append(maskPaths, k)
		}
		sort.Strings(maskPaths)
		maskPaths = append(maskPaths, tableVersionAttr)

		var outDoc fsDocument
		err = h.firestoreUnary(ctx, rpcUpdateDocument, false, &fsUpdateDocumentRequest{
			Document: fsDocument{
				Name:   docPath,
				Fields: patchFields,
			},
			UpdateMask: &fsDocumentMask{
				FieldPaths: maskPaths,
			},
			CurrentDocument: docPrecondition(curDoc),
		}, &outDoc)
		if err != nil {
			var ce *hostkit.CloudError
			if errors.As(err, &ce) && (ce.Code == hostkit.PreconditionFailed || ce.Code == hostkit.Conflict || ce.Code == hostkit.NotFound) {
				continue
			}
			return hostkit.Item{}, err
		}

		decoded, _, err := decodeFirestoreDoc(outDoc)
		if err != nil {
			decoded = mergedItem
		}
		hostkit.ReportTableOpMetrics(ctx, hostkit.TableOpMetrics{
			PriorExists:   true,
			PriorBytes:    priorBill,
			NewBytes:      newBill,
			Authoritative: false,
		})
		return decoded, nil
	}

	return hostkit.Item{}, hostkit.ProviderError(hostkit.Conflict, 409, "ConcurrentUpdateConflict", "item_update exceeded maximum concurrent modification retries")
}

type queryCursor struct {
	PK     string `json:"pk"`
	SK     string `json:"sk"`
	SKFrom string `json:"sk_from,omitempty"`
	SKTo   string `json:"sk_to,omitempty"`
}

type scanCursor struct {
	DocID string `json:"doc_id"`
}

func encodeCursorJSON(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", hostkit.ProviderError(hostkit.Internal, 0, "CursorEncodeError", err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func decodeCursorJSON(raw string, out any) error {
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return hostkit.ProviderError(hostkit.InvalidArgument, 0, "InvalidPageToken", "malformed pagination token")
	}
	if err := json.Unmarshal(b, out); err != nil {
		return hostkit.ProviderError(hostkit.InvalidArgument, 0, "InvalidPageToken", "malformed pagination token")
	}
	return nil
}

func (h *Host) Query(ctx context.Context, table, pk, skFrom, skTo string, max int, token string) (hostkit.TablePage, error) {
	sorted, cached := h.lookupCachedLayout(table)
	if !cached {
		var err error
		sorted, err = h.ensureTableLayout(ctx, table)
		if err != nil {
			return hostkit.TablePage{}, err
		}
	}
	if !sorted && (skFrom != "" || skTo != "") {
		if cached {
			return hostkit.TablePage{}, hostkit.Preflightf("sort key range [sk_from, sk_to) is not valid on partition-key-only table %q", table)
		}
		return hostkit.TablePage{}, hostkit.ProviderError(hostkit.InvalidArgument, 0, "InvalidKeyLayout", fmt.Sprintf("sort key range [sk_from, sk_to) is not valid on partition-key-only table %q", table))
	}

	var cur queryCursor
	if token != "" {
		if err := decodeCursorJSON(token, &cur); err != nil {
			return hostkit.TablePage{}, err
		}
		if cur.PK != pk || cur.SKFrom != skFrom || cur.SKTo != skTo || (sorted && cur.SK == "") {
			return hostkit.TablePage{}, hostkit.ProviderError(hostkit.InvalidArgument, 0, "InvalidPageToken", "query page token does not match query parameters")
		}
	}

	filters := []fsFilter{
		{
			Field: &fsFieldFilter{
				FieldPath: tablePKAttr,
				Op:        fsFilterOpEqual,
				Value:     fsString(pk),
			},
		},
	}
	if sorted && skFrom != "" {
		filters = append(filters, fsFilter{
			Field: &fsFieldFilter{
				FieldPath: tableSKAttr,
				Op:        fsFilterOpGreaterThanOrEqual,
				Value:     fsString(skFrom),
			},
		})
	}
	if sorted && skTo != "" {
		filters = append(filters, fsFilter{
			Field: &fsFieldFilter{
				FieldPath: tableSKAttr,
				Op:        fsFilterOpLessThan,
				Value:     fsString(skTo),
			},
		})
	}

	var where fsFilter
	if len(filters) == 1 {
		where = filters[0]
	} else {
		where = fsFilter{CompositeAnd: filters}
	}

	orderField := "__name__"
	if sorted {
		orderField = tableSKAttr
	}
	sq := fsStructuredQuery{
		CollectionId: table,
		Where:        &where,
		OrderBy: []fsOrder{
			{FieldPath: orderField, Direction: fsDirectionAscending},
		},
		Limit: int32(max),
	}
	if token != "" {
		if sorted {
			sq.StartAt = &fsCursor{
				Values: []fsValue{fsString(cur.SK)},
				Before: false, // exclusive lower bound (> cur.SK)
			}
		} else {
			sq.StartAt = &fsCursor{
				Values: []fsValue{fsRef(h.firestoreDocPath(table, pk))},
				Before: false,
			}
		}
	}

	docs, err := h.firestoreRunQuery(ctx, &fsRunQueryRequest{
		Parent:          h.firestoreDocumentsRoot(),
		StructuredQuery: sq,
	}, max)
	if err != nil {
		return hostkit.TablePage{}, err
	}

	items := make([]hostkit.Item, 0, len(docs))
	for _, doc := range docs {
		it, hasSK, err := decodeFirestoreDoc(doc)
		if err != nil {
			return hostkit.TablePage{}, err
		}
		if hasSK != sorted || it.PK != pk {
			return hostkit.TablePage{}, hostkit.ProviderError(hostkit.Internal, 0, "CorruptItem", "Firestore query returned document violating table key schema")
		}
		if sorted {
			if skFrom != "" && it.SK < skFrom {
				continue
			}
			if skTo != "" && it.SK >= skTo {
				continue
			}
		}
		items = append(items, it)
	}

	var nextToken string
	if len(docs) == max && len(items) > 0 {
		last := items[len(items)-1]
		nextToken, err = encodeCursorJSON(queryCursor{
			PK:     last.PK,
			SK:     last.SK,
			SKFrom: skFrom,
			SKTo:   skTo,
		})
		if err != nil {
			return hostkit.TablePage{}, err
		}
	}
	return hostkit.TablePage{
		Items:     items,
		NextToken: nextToken,
	}, nil
}

func (h *Host) Scan(ctx context.Context, table string, max int, token string) (hostkit.TablePage, error) {
	sorted, err := h.ensureTableLayout(ctx, table)
	if err != nil {
		return hostkit.TablePage{}, err
	}

	var cur scanCursor
	if token != "" {
		if err := decodeCursorJSON(token, &cur); err != nil {
			return hostkit.TablePage{}, err
		}
		if cur.DocID == "" || strings.Contains(cur.DocID, "/") {
			return hostkit.TablePage{}, hostkit.ProviderError(hostkit.InvalidArgument, 0, "InvalidPageToken", "malformed scan page token")
		}
	}

	sq := fsStructuredQuery{
		CollectionId: table,
		OrderBy: []fsOrder{
			{FieldPath: "__name__", Direction: fsDirectionAscending},
		},
		Limit: int32(max),
	}
	if cur.DocID != "" {
		sq.StartAt = &fsCursor{
			Values: []fsValue{fsRef(h.firestoreDocPath(table, cur.DocID))},
			Before: false, // exclusive lower bound (> cur.DocID)
		}
	}

	docs, err := h.firestoreRunQuery(ctx, &fsRunQueryRequest{
		Parent:          h.firestoreDocumentsRoot(),
		StructuredQuery: sq,
	}, max)
	if err != nil {
		return hostkit.TablePage{}, err
	}

	items := make([]hostkit.Item, 0, len(docs))
	for _, doc := range docs {
		it, hasSK, err := decodeFirestoreDoc(doc)
		if err != nil {
			return hostkit.TablePage{}, err
		}
		if hasSK != sorted {
			return hostkit.TablePage{}, hostkit.ProviderError(hostkit.Internal, 0, "CorruptItem", "Firestore scan returned document violating table key layout")
		}
		items = append(items, it)
	}

	var nextToken string
	if len(docs) == max && len(docs) > 0 {
		lastDocID := docBaseName(docs[len(docs)-1].Name)
		nextToken, err = encodeCursorJSON(scanCursor{DocID: lastDocID})
		if err != nil {
			return hostkit.TablePage{}, err
		}
	}
	return hostkit.TablePage{
		Items:     items,
		NextToken: nextToken,
	}, nil
}
