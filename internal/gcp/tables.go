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

func runDeleteSubOp(ctx context.Context, subOp string, payloadBytes int64, fn func(context.Context) error) error {
	return hostkit.RunSubOperation(ctx, hostkit.SubOperationSpec{
		Operation:    subOp,
		PayloadBytes: payloadBytes,
		IsCleanup:    true,
	}, fn)
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
	deleteClaim string
	deleting    bool
}

func (h *Host) parseCatalogDoc(doc fsDocument) (catalogEntry, error) {
	var entry catalogEntry
	st, hasState := doc.Fields["state"]
	if !hasState || st.Kind != fsKindString {
		return catalogEntry{}, hostkit.ProviderError(hostkit.Conflict, 409, "IncompatibleTableSchema", "Firestore catalog document is missing string state field")
	}
	switch st.StrVal {
	case "active":
	case "deleting":
		entry.deleting = true
	default:
		return catalogEntry{}, hostkit.ProviderError(hostkit.Conflict, 409, "IncompatibleTableSchema", fmt.Sprintf("Firestore catalog document has unsupported state %q", st.StrVal))
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
	if dc, ok := doc.Fields["delete_claim"]; ok && dc.Kind == fsKindString {
		entry.deleteClaim = dc.StrVal
	}
	if inc, ok := doc.Fields["incarnation"]; ok && inc.Kind == fsKindString && inc.StrVal != "" {
		entry.incarnation = inc.StrVal
	} else if !doc.CreateTime.IsZero() {
		entry.incarnation = fmt.Sprintf("ts:%d.%09d", doc.CreateTime.Seconds, doc.CreateTime.Nanos)
	}
	return entry, nil
}

func (h *Host) describeTableCatalogDoc(ctx context.Context, name string) (catalogEntry, fsDocument, error) {
	req := &fsGetDocumentRequest{
		Name: h.firestoreDocPath(firestoreCatalogCollection, name),
	}
	var doc fsDocument
	if err := h.firestoreUnary(ctx, rpcGetDocument, true, req, &doc); err != nil {
		var ce *hostkit.CloudError
		if errors.As(err, &ce) && ce.Code == hostkit.NotFound {
			return catalogEntry{}, fsDocument{}, ce.WithAbsent(hostkit.AbsentTable)
		}
		return catalogEntry{}, fsDocument{}, err
	}
	cat, err := h.parseCatalogDoc(doc)
	if err != nil {
		return catalogEntry{}, fsDocument{}, err
	}
	return cat, doc, nil
}

func (h *Host) describeTableCatalog(ctx context.Context, name string) (catalogEntry, error) {
	cat, _, err := h.describeTableCatalogDoc(ctx, name)
	return cat, err
}

func (h *Host) ensureActiveCatalog(ctx context.Context, name string) (catalogEntry, fsDocument, error) {
	cat, doc, err := h.describeTableCatalogDoc(ctx, name)
	if err != nil {
		return catalogEntry{}, fsDocument{}, err
	}
	if cat.deleting {
		return catalogEntry{}, fsDocument{}, hostkit.ProviderError(hostkit.NotFound, 404, "TableNotFound", "table is being deleted").WithAbsent(hostkit.AbsentTable)
	}
	if strings.TrimSpace(cat.incarnation) == "" {
		return catalogEntry{}, fsDocument{}, hostkit.ProviderError(hostkit.Internal, 0, "MissingTableIncarnationToken", "Firestore catalog document is missing incarnation token")
	}
	return cat, doc, nil
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
	// Refuse adopting an existing uncatalogued user collection if no catalog
	// entry exists yet. First check if the catalog document already exists.
	existingCat, _, gerr := h.describeTableCatalogDoc(ctx, name)
	if gerr == nil {
		if existingCat.deleting {
			return false, hostkit.ProviderError(hostkit.Conflict, 409, "TableDeleting", "table is currently being deleted")
		}
		if existingCat.sorted != sorted {
			return false, hostkit.ProviderError(hostkit.Conflict, 409, "LayoutMismatch", "table already exists with a different key layout")
		}
		return false, nil
	}
	var gce *hostkit.CloudError
	if !errors.As(gerr, &gce) || gce.Code != hostkit.NotFound {
		return false, gerr
	}

	// Catalog doc does not exist: verify user collection {name} has no orphan documents.
	orphanDocs, qerr := h.firestoreRunQuery(ctx, &fsRunQueryRequest{
		Parent: h.firestoreDocumentsRoot(),
		StructuredQuery: fsStructuredQuery{
			CollectionId: name,
			Limit:        1,
		},
	}, 1)
	if qerr != nil {
		return false, qerr
	}
	if len(orphanDocs) > 0 {
		return false, hostkit.ProviderError(hostkit.Conflict, 409, "UncataloguedCollectionExists", fmt.Sprintf("Firestore collection %q already contains uncatalogued documents", name))
	}

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
		return true, nil
	}
	var ce *hostkit.CloudError
	if !errors.As(err, &ce) || ce.Code != hostkit.Conflict {
		return false, err
	}

	// Concurrent creation landed between our check and CreateDocument: inspect it.
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
	return false, nil
}

func (h *Host) TableCreateToken(ctx context.Context, name string) (string, error) {
	cat, _, err := h.ensureActiveCatalog(ctx, name)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(cat.createToken) == "" {
		return cat.incarnation, nil
	}
	return cat.createToken, nil
}

func (h *Host) HeadTable(ctx context.Context, name string) error {
	_, _, err := h.ensureActiveCatalog(ctx, name)
	return err
}

func (h *Host) DescribeTable(ctx context.Context, name string) (hostkit.TableInfo, error) {
	cat, _, err := h.ensureActiveCatalog(ctx, name)
	if err != nil {
		return hostkit.TableInfo{}, err
	}
	return hostkit.TableInfo{
		Sorted:      cat.sorted,
		CreateToken: cat.incarnation,
	}, nil
}

func (h *Host) DeleteTable(ctx context.Context, name string) error {
	catDocPath := h.firestoreDocPath(firestoreCatalogCollection, name)

	var catDoc fsDocument
	if err := runDeleteSubOp(ctx, "item_get", 4096, func(sctx context.Context) error {
		return h.firestoreUnary(sctx, rpcGetDocument, true, &fsGetDocumentRequest{Name: catDocPath}, &catDoc)
	}); err != nil {
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
	if strings.TrimSpace(cat.incarnation) == "" {
		return hostkit.ProviderError(hostkit.Internal, 0, "MissingTableIncarnationToken", "Firestore catalog document is missing incarnation token")
	}
	catPre, err := docPrecondition(catDoc)
	if err != nil {
		return err
	}
	claimID, err := newRandomHexToken()
	if err != nil {
		return err
	}

	// Atomically transition (or re-claim an interrupted tombstone) using the exact
	// observed catalog UpdateTime so a concurrent replacement incarnation is never
	// overwritten or deleted.
	catDoc.Fields["state"] = fsString("deleting")
	catDoc.Fields["delete_claim"] = fsString(claimID)
	var updatedCat fsDocument
	if err := runDeleteSubOp(ctx, "item_put", 1024, func(sctx context.Context) error {
		return h.firestoreUnary(sctx, rpcUpdateDocument, true, &fsUpdateDocumentRequest{
			Document: catDoc,
			UpdateMask: &fsDocumentMask{
				FieldPaths: []string{"state", "delete_claim"},
			},
			CurrentDocument: catPre,
		}, &updatedCat)
	}); err != nil {
		var ce *hostkit.CloudError
		if errors.As(err, &ce) {
			if ce.Code == hostkit.NotFound {
				return ce.WithAbsent(hostkit.AbsentTable)
			}
			if ce.Code == hostkit.PreconditionFailed || ce.Code == hostkit.Conflict {
				return hostkit.ProviderError(hostkit.Conflict, 409, "DeleteTableConflict", "table catalog was modified concurrently during delete")
			}
		}
		return err
	}

	// Purge all child documents in collection {name} via bounded RunQuery batches.
	// Every child delete is executed atomically with a catalog deletion-claim fence
	// (state == "deleting", incarnation == cat.incarnation, delete_claim == claimID,
	// CurrentDocument.UpdateTime == updatedCat.UpdateTime) AND the child document's
	// observed UpdateTime from the query batch, so a stale purge cannot delete a
	// replacement child created under a new table incarnation.
	const purgeBatchSize = 100
	for {
		var docs []fsDocument
		if err := runDeleteSubOp(ctx, "item_query", int64(purgeBatchSize)*4096, func(sctx context.Context) error {
			var qerr error
			docs, qerr = h.firestoreRunQuery(sctx, &fsRunQueryRequest{
				Parent: h.firestoreDocumentsRoot(),
				StructuredQuery: fsStructuredQuery{
					CollectionId: name,
					OrderBy: []fsOrder{
						{FieldPath: "__name__", Direction: fsDirectionAscending},
					},
					Limit: purgeBatchSize,
				},
			}, purgeBatchSize)
			return qerr
		}); err != nil {
			return err
		}
		if len(docs) == 0 {
			break
		}
		retryBatch := false
		for _, d := range docs {
			childPre, err := docPrecondition(d)
			if err != nil {
				return err
			}
			claimPre, err := docPrecondition(updatedCat)
			if err != nil {
				return err
			}
			docName := d.Name
			var commitResp fsCommitResponse
			err = runDeleteSubOp(ctx, "item_update", 1024, func(fenceCtx context.Context) error {
				dispatched := false
				derr := runDeleteSubOp(fenceCtx, "item_delete", 1024, func(delCtx context.Context) error {
					orig := hostkit.RetryHooksFrom(delCtx)
					trackCtx := hostkit.WithRetryHooks(delCtx, hostkit.RetryHooks{
						BeforeAttempt: orig.BeforeAttempt,
						AfterAttempt: func(c context.Context, attempt int, a hostkit.Attempt) {
							if !a.NotSent {
								dispatched = true
							}
							if orig.AfterAttempt != nil {
								orig.AfterAttempt(c, attempt, a)
							}
						},
					})
					return h.firestoreUnary(trackCtx, rpcCommit, false, &fsCommitRequest{
						Database: h.firestoreDatabaseRoot(),
						Writes: []fsWrite{
							{
								Update: &fsDocument{
									Name: catDocPath,
									Fields: map[string]fsValue{
										"state":        fsString("deleting"),
										"incarnation":  fsString(cat.incarnation),
										"delete_claim": fsString(claimID),
									},
								},
								UpdateMask: &fsDocumentMask{
									FieldPaths: []string{"state", "incarnation", "delete_claim"},
								},
								CurrentDocument: claimPre,
							},
							{
								Delete:          docName,
								CurrentDocument: childPre,
							},
						},
					}, &commitResp)
				})
				if !dispatched {
					if fh := hostkit.RetryHooksFrom(fenceCtx); fh.AfterAttempt != nil {
						fh.AfterAttempt(fenceCtx, 0, hostkit.Attempt{Err: derr, NotSent: true})
					}
				}
				return derr
			})
			if err == nil {
				if len(commitResp.WriteResults) > 0 && !commitResp.WriteResults[0].UpdateTime.IsZero() {
					updatedCat.UpdateTime = commitResp.WriteResults[0].UpdateTime
				} else if !commitResp.CommitTime.IsZero() {
					updatedCat.UpdateTime = commitResp.CommitTime
				}
				continue
			}
			var ce *hostkit.CloudError
			if !errors.As(err, &ce) || (ce.Code != hostkit.PreconditionFailed && ce.Code != hostkit.Conflict && ce.Code != hostkit.NotFound) {
				return err
			}
			// Re-verify catalog incarnation and claim before continuing.
			var checkDoc fsDocument
			if gerr := runDeleteSubOp(ctx, "item_get", 4096, func(sctx context.Context) error {
				return h.firestoreUnary(sctx, rpcGetDocument, true, &fsGetDocumentRequest{Name: catDocPath}, &checkDoc)
			}); gerr != nil {
				var gce *hostkit.CloudError
				if errors.As(gerr, &gce) && gce.Code == hostkit.NotFound {
					return hostkit.ProviderError(hostkit.Conflict, 409, "DeleteTableConflict", "table catalog was removed concurrently during purge")
				}
				return gerr
			}
			checkCat, perr := h.parseCatalogDoc(checkDoc)
			if perr != nil {
				return perr
			}
			if !checkCat.deleting || checkCat.incarnation != cat.incarnation || checkCat.deleteClaim != claimID {
				return hostkit.ProviderError(hostkit.Conflict, 409, "DeleteTableConflict", "table catalog incarnation changed during purge")
			}
			updatedCat = checkDoc
			retryBatch = true
		}
		if len(docs) < purgeBatchSize && !retryBatch {
			break
		}
	}

	finalPre, err := docPrecondition(updatedCat)
	if err != nil {
		return err
	}
	if err := runDeleteSubOp(ctx, "item_delete", 1024, func(sctx context.Context) error {
		var empty fsEmpty
		return h.firestoreUnary(sctx, rpcDeleteDocument, true, &fsDeleteDocumentRequest{
			Name:            catDocPath,
			CurrentDocument: finalPre,
		}, &empty)
	}); err != nil {
		var ce *hostkit.CloudError
		if errors.As(err, &ce) {
			if ce.Code == hostkit.NotFound {
				return nil
			}
			if ce.Code == hostkit.PreconditionFailed || ce.Code == hostkit.Conflict {
				return hostkit.ProviderError(hostkit.Conflict, 409, "DeleteTableConflict", "table catalog changed before final delete")
			}
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

func docPrecondition(doc fsDocument) (*fsPrecondition, error) {
	if doc.UpdateTime.IsZero() {
		return nil, hostkit.ProviderError(hostkit.Internal, 0, "MissingDocumentUpdateTime", "Firestore document is missing update_time required for version precondition")
	}
	return &fsPrecondition{
		HasUpdateTime: true,
		UpdateTime:    doc.UpdateTime,
	}, nil
}

func (h *Host) commitCatalogFencedWrite(ctx context.Context, catDoc fsDocument, cat catalogEntry, childWrite fsWrite) (fsCommitResponse, error) {
	catPre, err := docPrecondition(catDoc)
	if err != nil {
		return fsCommitResponse{}, err
	}
	req := &fsCommitRequest{
		Database: h.firestoreDatabaseRoot(),
		Writes: []fsWrite{
			{
				Update: &fsDocument{
					Name: catDoc.Name,
					Fields: map[string]fsValue{
						"state":       fsString("active"),
						"incarnation": fsString(cat.incarnation),
					},
				},
				UpdateMask: &fsDocumentMask{
					FieldPaths: []string{"state", "incarnation"},
				},
				CurrentDocument: catPre,
			},
			childWrite,
		},
	}
	var resp fsCommitResponse
	err = hostkit.RunSubOperation(ctx, hostkit.SubOperationSpec{
		Operation:    "item_update",
		PayloadBytes: 1024,
	}, func(fenceCtx context.Context) error {
		dispatched := false
		orig := hostkit.RetryHooksFrom(ctx)
		trackCtx := hostkit.WithRetryHooks(ctx, hostkit.RetryHooks{
			BeforeAttempt: orig.BeforeAttempt,
			AfterAttempt: func(c context.Context, attempt int, a hostkit.Attempt) {
				if !a.NotSent {
					dispatched = true
				}
				if orig.AfterAttempt != nil {
					orig.AfterAttempt(c, attempt, a)
				}
			},
		})
		callErr := h.firestoreUnary(trackCtx, rpcCommit, false, req, &resp)
		if !dispatched {
			if fh := hostkit.RetryHooksFrom(fenceCtx); fh.AfterAttempt != nil {
				fh.AfterAttempt(fenceCtx, 0, hostkit.Attempt{Err: callErr, NotSent: true})
			}
		}
		return callErr
	})
	if err != nil {
		return fsCommitResponse{}, err
	}
	return resp, nil
}

func (h *Host) PutItem(ctx context.Context, table string, item hostkit.Item, mode hostkit.PutMode, expectVersion string) (hostkit.Item, error) {
	cat, catDoc, err := h.ensureActiveCatalog(ctx, table)
	if err != nil {
		return hostkit.Item{}, err
	}
	sorted := cat.sorted
	if err := validateKeyLayout(table, sorted, item.SK, false); err != nil {
		return hostkit.Item{}, err
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
		for attempt := 0; attempt < maxUpdateCASAttempts; attempt++ {
			if attempt > 0 {
				latestCat, latestDoc, cerr := h.ensureActiveCatalog(ctx, table)
				if cerr != nil {
					return hostkit.Item{}, cerr
				}
				if latestCat.incarnation != cat.incarnation {
					return hostkit.Item{}, hostkit.ProviderError(hostkit.Conflict, 409, "TableIncarnationChanged", "table incarnation changed concurrently during item_put")
				}
				cat, catDoc = latestCat, latestDoc
			}
			_, err := h.commitCatalogFencedWrite(ctx, catDoc, cat, fsWrite{
				Update: &fsDocument{
					Name:   docPath,
					Fields: fields,
				},
				CurrentDocument: &fsPrecondition{HasExists: true, Exists: false},
			})
			if err != nil {
				var ce *hostkit.CloudError
				if errors.As(err, &ce) && ce.Code == hostkit.PreconditionFailed {
					continue
				}
				return hostkit.Item{}, err
			}
			hostkit.ReportTableOpMetrics(ctx, hostkit.TableOpMetrics{
				PriorExists:   false,
				PriorBytes:    0,
				NewBytes:      newBill,
				Authoritative: true,
			})
			return stored, nil
		}
		return hostkit.Item{}, hostkit.ProviderError(hostkit.Conflict, 409, "ConcurrentPutConflict", "item_put exceeded maximum concurrent catalog retries")

	case hostkit.PutUnconditional:
		for attempt := 0; attempt < maxUpdateCASAttempts; attempt++ {
			if attempt > 0 {
				latestCat, latestDoc, cerr := h.ensureActiveCatalog(ctx, table)
				if cerr != nil {
					return hostkit.Item{}, cerr
				}
				if latestCat.incarnation != cat.incarnation {
					return hostkit.Item{}, hostkit.ProviderError(hostkit.Conflict, 409, "TableIncarnationChanged", "table incarnation changed concurrently during item_put")
				}
				cat, catDoc = latestCat, latestDoc
			}
			_, err := h.commitCatalogFencedWrite(ctx, catDoc, cat, fsWrite{
				Update: &fsDocument{
					Name:   docPath,
					Fields: fields,
				},
			})
			if err != nil {
				var ce *hostkit.CloudError
				if errors.As(err, &ce) && ce.Code == hostkit.PreconditionFailed {
					continue
				}
				return hostkit.Item{}, err
			}
			hostkit.ReportTableOpMetrics(ctx, hostkit.TableOpMetrics{
				PriorExists:   false,
				PriorBytes:    0,
				NewBytes:      newBill,
				Authoritative: false,
			})
			return stored, nil
		}
		return hostkit.Item{}, hostkit.ProviderError(hostkit.Conflict, 409, "ConcurrentPutConflict", "item_put exceeded maximum concurrent catalog retries")

	case hostkit.PutIfVersion:
		for attempt := 0; attempt < maxUpdateCASAttempts; attempt++ {
			var curDoc fsDocument
			if err := h.firestoreUnary(ctx, rpcGetDocument, true, &fsGetDocumentRequest{Name: docPath}, &curDoc); err != nil {
				var ce *hostkit.CloudError
				if errors.As(err, &ce) && ce.Code == hostkit.NotFound {
					return hostkit.Item{}, ce.WithAbsent(hostkit.AbsentItem)
				}
				return hostkit.Item{}, err
			}
			priorItem, hasSK, err := decodeFirestoreDoc(curDoc)
			if err != nil {
				return hostkit.Item{}, err
			}
			if hasSK != sorted {
				return hostkit.Item{}, hostkit.ProviderError(hostkit.Internal, 0, "CorruptItem", "Firestore document key layout does not match table catalog")
			}
			if priorItem.Version != expectVersion {
				return hostkit.Item{}, hostkit.ProviderError(hostkit.PreconditionFailed, 412, "VersionMismatch", "expected version does not match current item version")
			}
			if _, cerr := hostkit.CanonicalItemBytes(priorItem.Attrs); cerr != nil {
				return hostkit.Item{}, cerr
			}
			priorBill := hostkit.NativeItemBillableBytes(priorItem)
			pre, err := docPrecondition(curDoc)
			if err != nil {
				return hostkit.Item{}, err
			}

			_, err = h.commitCatalogFencedWrite(ctx, catDoc, cat, fsWrite{
				Update: &fsDocument{
					Name:   docPath,
					Fields: fields,
				},
				CurrentDocument: pre,
			})
			if err != nil {
				var ce *hostkit.CloudError
				if errors.As(err, &ce) && (ce.Code == hostkit.PreconditionFailed || ce.Code == hostkit.NotFound) {
					latestCat, latestDoc, cerr := h.ensureActiveCatalog(ctx, table)
					if cerr != nil {
						return hostkit.Item{}, cerr
					}
					if latestCat.incarnation != cat.incarnation {
						return hostkit.Item{}, hostkit.ProviderError(hostkit.Conflict, 409, "TableIncarnationChanged", "table incarnation changed concurrently during item_put")
					}
					if latestDoc.UpdateTime != catDoc.UpdateTime {
						cat, catDoc = latestCat, latestDoc
						continue
					}
					if ce.Code == hostkit.NotFound {
						return hostkit.Item{}, ce.WithAbsent(hostkit.AbsentItem)
					}
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
		}
		return hostkit.Item{}, hostkit.ProviderError(hostkit.Conflict, 409, "ConcurrentPutConflict", "item_put exceeded maximum concurrent catalog retries")

	default:
		return hostkit.Item{}, hostkit.ProviderError(hostkit.InvalidArgument, 0, "InvalidPutMode", "unsupported put mode")
	}
}

func (h *Host) GetItem(ctx context.Context, table, pk, sk string) (hostkit.Item, error) {
	cat, _, err := h.ensureActiveCatalog(ctx, table)
	if err != nil {
		return hostkit.Item{}, err
	}
	sorted := cat.sorted
	if err := validateKeyLayout(table, sorted, sk, false); err != nil {
		return hostkit.Item{}, err
	}

	docID := firestoreDocID(sorted, pk, sk)
	var doc fsDocument
	if err := h.firestoreUnary(ctx, rpcGetDocument, true, &fsGetDocumentRequest{
		Name: h.firestoreDocPath(table, docID),
	}, &doc); err != nil {
		var ce *hostkit.CloudError
		if errors.As(err, &ce) && ce.Code == hostkit.NotFound {
			return hostkit.Item{}, ce.WithAbsent(hostkit.AbsentItem)
		}
		return hostkit.Item{}, err
	}

	item, hasSK, err := decodeFirestoreDoc(doc)
	if err != nil {
		return hostkit.Item{}, err
	}
	if hasSK != sorted {
		return hostkit.Item{}, hostkit.ProviderError(hostkit.Internal, 0, "CorruptItem", "Firestore document key layout does not match table catalog")
	}
	return item, nil
}

func (h *Host) DeleteItem(ctx context.Context, table, pk, sk string) error {
	cat, catDoc, err := h.ensureActiveCatalog(ctx, table)
	if err != nil {
		return err
	}
	sorted := cat.sorted
	if err := validateKeyLayout(table, sorted, sk, false); err != nil {
		return err
	}

	docID := firestoreDocID(sorted, pk, sk)
	docPath := h.firestoreDocPath(table, docID)
	for attempt := 0; attempt < maxUpdateCASAttempts; attempt++ {
		if attempt > 0 {
			latestCat, latestDoc, cerr := h.ensureActiveCatalog(ctx, table)
			if cerr != nil {
				return cerr
			}
			if latestCat.incarnation != cat.incarnation {
				return hostkit.ProviderError(hostkit.Conflict, 409, "TableIncarnationChanged", "table incarnation changed concurrently during item_delete")
			}
			cat, catDoc = latestCat, latestDoc
		}
		_, err := h.commitCatalogFencedWrite(ctx, catDoc, cat, fsWrite{
			Delete:          docPath,
			CurrentDocument: &fsPrecondition{HasExists: true, Exists: true},
		})
		if err != nil {
			var ce *hostkit.CloudError
			if errors.As(err, &ce) {
				if ce.Code == hostkit.NotFound {
					return ce.WithAbsent(hostkit.AbsentItem)
				}
				if ce.Code == hostkit.PreconditionFailed {
					continue
				}
			}
			return err
		}
		return nil
	}
	return hostkit.ProviderError(hostkit.Conflict, 409, "ConcurrentDeleteConflict", "item_delete exceeded maximum concurrent catalog retries")
}

func (h *Host) UpdateItem(ctx context.Context, table, pk, sk string, changes hostkit.Item) (hostkit.Item, error) {
	cat, catDoc, err := h.ensureActiveCatalog(ctx, table)
	if err != nil {
		return hostkit.Item{}, err
	}
	sorted := cat.sorted
	if err := validateKeyLayout(table, sorted, sk, false); err != nil {
		return hostkit.Item{}, err
	}

	docID := firestoreDocID(sorted, pk, sk)
	docPath := h.firestoreDocPath(table, docID)

	for attempt := 0; attempt < maxUpdateCASAttempts; attempt++ {
		if attempt > 0 {
			latestCat, latestDoc, cerr := h.ensureActiveCatalog(ctx, table)
			if cerr != nil {
				return hostkit.Item{}, cerr
			}
			if latestCat.incarnation != cat.incarnation {
				return hostkit.Item{}, hostkit.ProviderError(hostkit.Conflict, 409, "TableIncarnationChanged", "table incarnation changed concurrently during item_update")
			}
			cat, catDoc = latestCat, latestDoc
		}

		var curDoc fsDocument
		if err := h.firestoreUnary(ctx, rpcGetDocument, true, &fsGetDocumentRequest{Name: docPath}, &curDoc); err != nil {
			var ce *hostkit.CloudError
			if errors.As(err, &ce) && ce.Code == hostkit.NotFound {
				return hostkit.Item{}, ce.WithAbsent(hostkit.AbsentItem)
			}
			return hostkit.Item{}, err
		}

		priorItem, hasSK, err := decodeFirestoreDoc(curDoc)
		if err != nil {
			return hostkit.Item{}, err
		}
		if hasSK != sorted {
			return hostkit.Item{}, hostkit.ProviderError(hostkit.Internal, 0, "CorruptItem", "Firestore document key layout does not match table catalog")
		}
		if _, cerr := hostkit.CanonicalItemBytes(priorItem.Attrs); cerr != nil {
			return hostkit.Item{}, cerr
		}
		priorBill := hostkit.NativeItemBillableBytes(priorItem)

		pre, err := docPrecondition(curDoc)
		if err != nil {
			return hostkit.Item{}, err
		}

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

		_, err = h.commitCatalogFencedWrite(ctx, catDoc, cat, fsWrite{
			Update: &fsDocument{
				Name:   docPath,
				Fields: patchFields,
			},
			UpdateMask: &fsDocumentMask{
				FieldPaths: maskPaths,
			},
			CurrentDocument: pre,
		})
		if err != nil {
			var ce *hostkit.CloudError
			if errors.As(err, &ce) && (ce.Code == hostkit.PreconditionFailed || ce.Code == hostkit.Conflict || ce.Code == hostkit.NotFound) {
				continue
			}
			return hostkit.Item{}, err
		}

		hostkit.ReportTableOpMetrics(ctx, hostkit.TableOpMetrics{
			PriorExists:   true,
			PriorBytes:    priorBill,
			NewBytes:      newBill,
			Authoritative: false,
		})
		return mergedItem, nil
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
	cat, _, err := h.ensureActiveCatalog(ctx, table)
	if err != nil {
		return hostkit.TablePage{}, err
	}
	sorted := cat.sorted
	if !sorted && (skFrom != "" || skTo != "") {
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

	// Index-free range query over document __name__ (12-tables-design.md §3.1 / §6 D6):
	// For sorted tables, document IDs are "{pk}#{sk}". Since '#' is ASCII 0x23 and '$'
	// is ASCII 0x24 (which precedes every valid key character), all documents in
	// partition {pk} lie strictly inside ["{pk}#", "{pk}$").
	sq := fsStructuredQuery{
		CollectionId: table,
		OrderBy: []fsOrder{
			{FieldPath: "__name__", Direction: fsDirectionAscending},
		},
		Limit: int32(max),
	}
	if sorted {
		if token != "" {
			sq.StartAt = &fsCursor{
				Values: []fsValue{fsRef(h.firestoreDocPath(table, pk+"#"+cur.SK))},
				Before: false, // exclusive (> pk#cur.SK)
			}
		} else if skFrom != "" {
			sq.StartAt = &fsCursor{
				Values: []fsValue{fsRef(h.firestoreDocPath(table, pk+"#"+skFrom))},
				Before: true, // inclusive (>= pk#skFrom)
			}
		} else {
			sq.StartAt = &fsCursor{
				Values: []fsValue{fsRef(h.firestoreDocPath(table, pk+"#"))},
				Before: true, // inclusive (>= pk#)
			}
		}
		if skTo != "" {
			sq.EndAt = &fsCursor{
				Values: []fsValue{fsRef(h.firestoreDocPath(table, pk+"#"+skTo))},
				Before: true, // exclusive (< pk#skTo)
			}
		} else {
			sq.EndAt = &fsCursor{
				Values: []fsValue{fsRef(h.firestoreDocPath(table, pk+"$"))},
				Before: true, // exclusive (< pk$)
			}
		}
	} else {
		docRef := fsRef(h.firestoreDocPath(table, pk))
		sq.StartAt = &fsCursor{
			Values: []fsValue{docRef},
			Before: token == "",
		}
		sq.EndAt = &fsCursor{
			Values: []fsValue{docRef},
			Before: false,
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
			return hostkit.TablePage{}, hostkit.ProviderError(hostkit.Internal, 0, "CorruptItem", "Firestore query returned document violating table key schema")
		}
		if it.PK != pk {
			continue
		}
		if sorted {
			if token != "" && it.SK <= cur.SK {
				continue
			}
			if skFrom != "" && it.SK < skFrom {
				continue
			}
			if skTo != "" && it.SK >= skTo {
				continue
			}
		} else if token != "" {
			continue
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
	cat, _, err := h.ensureActiveCatalog(ctx, table)
	if err != nil {
		return hostkit.TablePage{}, err
	}
	sorted := cat.sorted

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
