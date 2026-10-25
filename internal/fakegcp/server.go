// SPDX-License-Identifier: MIT

package fakegcp

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"google.golang.org/api/artifactregistry/v1"
	"google.golang.org/api/bigquery/v2"
	"google.golang.org/api/cloudkms/v1"
	"google.golang.org/api/cloudresourcemanager/v1"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/container/v1"
	"google.golang.org/api/dns/v1"
	"google.golang.org/api/iam/v1"
	"google.golang.org/api/logging/v2"
	"google.golang.org/api/monitoring/v3"
	"google.golang.org/api/pubsub/v1"
	"google.golang.org/api/run/v2"
	"google.golang.org/api/secretmanager/v1"
	"google.golang.org/api/sqladmin/v1"
	"google.golang.org/api/storage/v1"
)

type bucketStat struct {
	ObjectCount int64
	TotalBytes  int64
}

type storedObject struct {
	Name        string
	ContentType string
	Content     string
}

type runMetaInfo struct {
	CPUMilli      int64
	MemoryMB      int64
	RevisionCount int64
	StablePct     int64
	CanaryPct     int64
}

type saMetaInfo struct {
	ShortName       string
	UserManagedKeys int64
	KeyMaxAgeDays   int64
	IsDefault       bool
	Roles           []string
}

type secretMetaInfo struct {
	ReplicationPolicy string
	RotationDays      int64
	VersionCount      int64
	EnabledVersions   int64
	CMEKEnabled       bool
	HasExpiration     bool
}

type gkeMetaInfo struct {
	Autopilot        bool
	PrivateNodes     bool
	WorkloadIdentity bool
	NetworkPolicy    bool
	BinaryAuth       bool
	NodePoolCount    int64
	TotalNodes       int64
}

type arMetaInfo struct {
	ImmutableTags         bool
	CleanupPolicies       int64
	VulnerabilityScanning bool
	SizeMB                int64
	PackageCount          int64
}

type alertMetaInfo struct {
	MetricType  string
	Threshold   float64
	DurationSec int64
}

type pubsubMsg struct {
	ID         string
	Data       string
	Attributes map[string]string
}

// Server is an in-process fake Google Cloud REST server consumed by google.golang.org/api clients.
type Server struct {
	mu       sync.RWMutex
	listener net.Listener
	httpSrv  *http.Server
	baseURL  string
	msgSeq   int64

	vpcs          map[string]*compute.Network
	subnets       map[string]*compute.Subnetwork
	firewalls     map[string]*compute.Firewall
	instances     map[string]*compute.Instance
	disks         map[string]*compute.Disk
	buckets       map[string]*storage.Bucket
	bucketStats   map[string]bucketStat
	objects       map[string]map[string]storedObject
	topics        map[string]*pubsub.Topic
	topicStats    map[string]int64
	subscriptions map[string]*pubsub.Subscription
	subBacklog    map[string]int64
	subMessages   map[string][]pubsubMsg
	runServices   map[string]*run.GoogleCloudRunV2Service
	runMeta       map[string]runMetaInfo
	sqlInstances  map[string]*sqladmin.DatabaseInstance
	firestoreDocs map[string]map[string]map[string]string
	bqDatasets    map[string]*bigquery.Dataset
	bqTables      map[string]*bigquery.Table
	serviceAccounts map[string]*iam.ServiceAccount
	saMeta        map[string]saMetaInfo
	iamPolicy     *cloudresourcemanager.Policy
	secrets       map[string]*secretmanager.Secret
	secretMeta    map[string]secretMetaInfo
	kmsKeyRings   map[string]*cloudkms.KeyRing
	kmsKeys       map[string]*cloudkms.CryptoKey
	kmsRotDays    map[string]int64
	gkeClusters   map[string]*container.Cluster
	gkeMeta       map[string]gkeMetaInfo
	arRepos       map[string]*artifactregistry.Repository
	arMeta        map[string]arMetaInfo
	dnsZones      map[string]*dns.ManagedZone
	dnsRecords    map[string][]*dns.ResourceRecordSet
	logSinks      map[string]*logging.LogSink
	logEntries    []string
	logSeverityCounts map[string]int64
	alertPolicies map[string]*monitoring.AlertPolicy
	alertMeta     map[string]alertMetaInfo
	metricSeries  map[string][]float64
	createdAgeHrs map[string]int64 // kind:name -> simulated age in hours
}

// NewServer starts a new loopback fake GCP REST server pre-seeded with a baseline estate.
func NewServer() (*Server, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	s := &Server{
		listener:          ln,
		baseURL:           "http://" + ln.Addr().String(),
		vpcs:              make(map[string]*compute.Network),
		subnets:           make(map[string]*compute.Subnetwork),
		firewalls:         make(map[string]*compute.Firewall),
		instances:         make(map[string]*compute.Instance),
		disks:             make(map[string]*compute.Disk),
		buckets:           make(map[string]*storage.Bucket),
		bucketStats:       make(map[string]bucketStat),
		objects:           make(map[string]map[string]storedObject),
		topics:            make(map[string]*pubsub.Topic),
		topicStats:        make(map[string]int64),
		subscriptions:     make(map[string]*pubsub.Subscription),
		subBacklog:        make(map[string]int64),
		subMessages:       make(map[string][]pubsubMsg),
		runServices:       make(map[string]*run.GoogleCloudRunV2Service),
		runMeta:           make(map[string]runMetaInfo),
		sqlInstances:      make(map[string]*sqladmin.DatabaseInstance),
		firestoreDocs:     make(map[string]map[string]map[string]string),
		bqDatasets:        make(map[string]*bigquery.Dataset),
		bqTables:          make(map[string]*bigquery.Table),
		serviceAccounts:   make(map[string]*iam.ServiceAccount),
		saMeta:            make(map[string]saMetaInfo),
		secrets:           make(map[string]*secretmanager.Secret),
		secretMeta:        make(map[string]secretMetaInfo),
		kmsKeyRings:       make(map[string]*cloudkms.KeyRing),
		kmsKeys:           make(map[string]*cloudkms.CryptoKey),
		kmsRotDays:        make(map[string]int64),
		gkeClusters:       make(map[string]*container.Cluster),
		gkeMeta:           make(map[string]gkeMetaInfo),
		arRepos:           make(map[string]*artifactregistry.Repository),
		arMeta:            make(map[string]arMetaInfo),
		dnsZones:          make(map[string]*dns.ManagedZone),
		dnsRecords:        make(map[string][]*dns.ResourceRecordSet),
		logSinks:          make(map[string]*logging.LogSink),
		logSeverityCounts: make(map[string]int64),
		alertPolicies:     make(map[string]*monitoring.AlertPolicy),
		alertMeta:         make(map[string]alertMetaInfo),
		metricSeries:      make(map[string][]float64),
		createdAgeHrs:     make(map[string]int64),
	}
	s.seedBaseline()

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.serveHTTP)
	s.httpSrv = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		_ = s.httpSrv.Serve(ln)
	}()
	return s, nil
}

func (s *Server) URL() string {
	return s.baseURL
}

func (s *Server) ServiceEndpoint(svc string) string {
	return s.baseURL + "/_svc/" + svc + "/"
}

func (s *Server) Close() {
	if s.httpSrv != nil {
		_ = s.httpSrv.Close()
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]any{
			"code":    status,
			"message": msg,
		},
	})
}

func lastSegment(p string) string {
	p = strings.TrimSuffix(p, "/")
	if idx := strings.LastIndex(p, "/"); idx >= 0 {
		return p[idx+1:]
	}
	return p
}

func (s *Server) serveHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if !strings.HasPrefix(path, "/_svc/") {
		writeErr(w, http.StatusNotFound, "unknown endpoint prefix: "+path)
		return
	}
	rest := strings.TrimPrefix(path, "/_svc/")
	slash := strings.IndexByte(rest, '/')
	if slash < 0 {
		writeErr(w, http.StatusNotFound, "missing service route: "+path)
		return
	}
	svc := rest[:slash]
	sub := rest[slash:] // starts with '/'

	s.mu.Lock()
	defer s.mu.Unlock()

	switch svc {
	case "compute":
		s.handleCompute(w, r, sub)
	case "storage":
		s.handleStorage(w, r, sub)
	case "pubsub":
		s.handlePubsub(w, r, sub)
	case "run":
		s.handleCloudRun(w, r, sub)
	case "sql":
		s.handleSQL(w, r, sub)
	case "bigquery":
		s.handleBigQuery(w, r, sub)
	case "iam":
		s.handleIAM(w, r, sub)
	case "crm":
		s.handleCRM(w, r, sub)
	case "secrets":
		s.handleSecrets(w, r, sub)
	case "kms":
		s.handleKMS(w, r, sub)
	case "gke":
		s.handleGKE(w, r, sub)
	case "ar":
		s.handleAR(w, r, sub)
	case "dns":
		s.handleDNS(w, r, sub)
	case "logging":
		s.handleLogging(w, r, sub)
	case "monitoring":
		s.handleMonitoring(w, r, sub)
	default:
		writeErr(w, http.StatusNotFound, "unsupported fake service: "+svc)
	}
}

// --- Compute Engine & VPC Network handlers ---
func (s *Server) handleCompute(w http.ResponseWriter, r *http.Request, sub string) {
	parts := strings.Split(strings.Trim(sub, "/"), "/")
	// Expected shapes under /projects/{project}/...
	if len(parts) < 3 || parts[0] != "projects" {
		writeErr(w, http.StatusNotFound, "bad compute path: "+sub)
		return
	}
	// Global networks: projects/{p}/global/networks[/{name}]
	if len(parts) >= 4 && parts[2] == "global" && parts[3] == "networks" {
		if len(parts) == 4 {
			if r.Method == http.MethodGet {
				var names []string
				for k := range s.vpcs {
					names = append(names, k)
				}
				sort.Strings(names)
				var items []*compute.Network
				for _, k := range names {
					items = append(items, s.vpcs[k])
				}
				writeJSON(w, http.StatusOK, &compute.NetworkList{Items: items})
				return
			}
			if r.Method == http.MethodPost {
				var netObj compute.Network
				if err := json.NewDecoder(r.Body).Decode(&netObj); err != nil {
					writeErr(w, http.StatusBadRequest, err.Error())
					return
				}
				if netObj.Mtu == 0 {
					netObj.Mtu = 1460
				}
				if netObj.RoutingConfig == nil {
					netObj.RoutingConfig = &compute.NetworkRoutingConfig{RoutingMode: "REGIONAL"}
				}
				s.vpcs[netObj.Name] = &netObj
				writeJSON(w, http.StatusOK, &compute.Operation{Name: "op-net-insert", Status: "DONE"})
				return
			}
		} else if len(parts) == 5 {
			name := parts[4]
			if r.Method == http.MethodGet {
				n, ok := s.vpcs[name]
				if !ok {
					writeErr(w, http.StatusNotFound, "network not found: "+name)
					return
				}
				writeJSON(w, http.StatusOK, n)
				return
			}
			if r.Method == http.MethodDelete {
				delete(s.vpcs, name)
				writeJSON(w, http.StatusOK, &compute.Operation{Name: "op-net-del", Status: "DONE"})
				return
			}
		}
	}

	// Global firewalls: projects/{p}/global/firewalls[/{name}]
	if len(parts) >= 4 && parts[2] == "global" && parts[3] == "firewalls" {
		if len(parts) == 4 {
			if r.Method == http.MethodGet {
				var names []string
				for k := range s.firewalls {
					names = append(names, k)
				}
				sort.Strings(names)
				var items []*compute.Firewall
				for _, k := range names {
					items = append(items, s.firewalls[k])
				}
				writeJSON(w, http.StatusOK, &compute.FirewallList{Items: items})
				return
			}
			if r.Method == http.MethodPost {
				var fw compute.Firewall
				if err := json.NewDecoder(r.Body).Decode(&fw); err != nil {
					writeErr(w, http.StatusBadRequest, err.Error())
					return
				}
				s.firewalls[fw.Name] = &fw
				writeJSON(w, http.StatusOK, &compute.Operation{Name: "op-fw-insert", Status: "DONE"})
				return
			}
		} else if len(parts) == 5 {
			name := parts[4]
			if r.Method == http.MethodGet {
				fw, ok := s.firewalls[name]
				if !ok {
					writeErr(w, http.StatusNotFound, "firewall not found: "+name)
					return
				}
				writeJSON(w, http.StatusOK, fw)
				return
			}
			if r.Method == http.MethodDelete {
				delete(s.firewalls, name)
				writeJSON(w, http.StatusOK, &compute.Operation{Name: "op-fw-del", Status: "DONE"})
				return
			}
		}
	}

	// Regional subnetworks: projects/{p}/regions/{region}/subnetworks[/{name}]
	if len(parts) >= 5 && parts[2] == "regions" && parts[4] == "subnetworks" {
		region := parts[3]
		if len(parts) == 5 {
			if r.Method == http.MethodGet {
				var keys []string
				for k, sn := range s.subnets {
					if region == "all" || sn.Region == region {
						keys = append(keys, k)
					}
				}
				sort.Strings(keys)
				var items []*compute.Subnetwork
				for _, k := range keys {
					items = append(items, s.subnets[k])
				}
				writeJSON(w, http.StatusOK, &compute.SubnetworkList{Items: items})
				return
			}
			if r.Method == http.MethodPost {
				var sn compute.Subnetwork
				if err := json.NewDecoder(r.Body).Decode(&sn); err != nil {
					writeErr(w, http.StatusBadRequest, err.Error())
					return
				}
				sn.Region = region
				s.subnets[region+"/"+sn.Name] = &sn
				writeJSON(w, http.StatusOK, &compute.Operation{Name: "op-sn-insert", Status: "DONE"})
				return
			}
		} else if len(parts) == 6 {
			name := parts[5]
			key := region + "/" + name
			if r.Method == http.MethodGet {
				sn, ok := s.subnets[key]
				if !ok {
					writeErr(w, http.StatusNotFound, "subnet not found: "+key)
					return
				}
				writeJSON(w, http.StatusOK, sn)
				return
			}
			if r.Method == http.MethodDelete {
				delete(s.subnets, key)
				writeJSON(w, http.StatusOK, &compute.Operation{Name: "op-sn-del", Status: "DONE"})
				return
			}
		}
	}

	// Zonal instances & disks: projects/{p}/zones/{zone}/instances... or disks...
	if len(parts) >= 5 && parts[2] == "zones" {
		zone := parts[3]
		kind := parts[4]
		if kind == "disks" && len(parts) == 5 && r.Method == http.MethodGet {
			var keys []string
			for k, d := range s.disks {
				if zone == "all" || d.Zone == zone {
					keys = append(keys, k)
				}
			}
			sort.Strings(keys)
			var items []*compute.Disk
			for _, k := range keys {
				items = append(items, s.disks[k])
			}
			writeJSON(w, http.StatusOK, &compute.DiskList{Items: items})
			return
		}
		if kind == "instances" {
			if len(parts) == 5 {
				if r.Method == http.MethodGet {
					var keys []string
					for k, inst := range s.instances {
						if zone == "all" || inst.Zone == zone {
							keys = append(keys, k)
						}
					}
					sort.Strings(keys)
					var items []*compute.Instance
					for _, k := range keys {
						items = append(items, s.instances[k])
					}
					writeJSON(w, http.StatusOK, &compute.InstanceList{Items: items})
					return
				}
				if r.Method == http.MethodPost {
					var inst compute.Instance
					if err := json.NewDecoder(r.Body).Decode(&inst); err != nil {
						writeErr(w, http.StatusBadRequest, err.Error())
						return
					}
					inst.Zone = zone
					if inst.Status == "" {
						inst.Status = "RUNNING"
					}
					if inst.CpuPlatform == "" {
						inst.CpuPlatform = "Intel Ice Lake"
					}
					if len(inst.NetworkInterfaces) == 0 {
						inst.NetworkInterfaces = []*compute.NetworkInterface{
							{Network: "prod-vpc", NetworkIP: "10.10.9.50"},
						}
					}
					if len(inst.Disks) == 0 {
						inst.Disks = []*compute.AttachedDisk{{Boot: true, DiskSizeGb: 20, Source: inst.Name + "-boot"}}
					}
					if inst.ShieldedInstanceConfig == nil {
						inst.ShieldedInstanceConfig = &compute.ShieldedInstanceConfig{EnableVtpm: true}
					}
					s.instances[zone+"/"+inst.Name] = &inst
					s.disks[zone+"/"+inst.Name+"-boot"] = &compute.Disk{
						Name:   inst.Name + "-boot",
						Zone:   zone,
						SizeGb: 20,
						Status: "READY",
						Type:   "pd-balanced",
					}
					writeJSON(w, http.StatusOK, &compute.Operation{Name: "op-inst-insert", Status: "DONE"})
					return
				}
			} else if len(parts) == 6 {
				name := parts[5]
				key := zone + "/" + name
				if r.Method == http.MethodGet {
					inst, ok := s.instances[key]
					if !ok {
						writeErr(w, http.StatusNotFound, "instance not found: "+key)
						return
					}
					writeJSON(w, http.StatusOK, inst)
					return
				}
				if r.Method == http.MethodDelete {
					delete(s.instances, key)
					delete(s.disks, key+"-boot")
					writeJSON(w, http.StatusOK, &compute.Operation{Name: "op-inst-del", Status: "DONE"})
					return
				}
			} else if len(parts) == 7 {
				name := parts[5]
				action := parts[6]
				key := zone + "/" + name
				inst, ok := s.instances[key]
				if !ok {
					writeErr(w, http.StatusNotFound, "instance not found: "+key)
					return
				}
				switch action {
				case "stop":
					inst.Status = "TERMINATED"
				case "start":
					inst.Status = "RUNNING"
				case "setLabels":
					var req compute.InstancesSetLabelsRequest
					_ = json.NewDecoder(r.Body).Decode(&req)
					inst.Labels = req.Labels
				}
				writeJSON(w, http.StatusOK, &compute.Operation{Name: "op-inst-" + action, Status: "DONE"})
				return
			}
		}
	}
	writeErr(w, http.StatusNotFound, "unhandled compute route: "+sub)
}

// --- Cloud Storage (GCS) handlers ---
func (s *Server) handleStorage(w http.ResponseWriter, r *http.Request, sub string) {
	parts := strings.Split(strings.Trim(sub, "/"), "/")
	// /b or /b/{bucket} or /b/{bucket}/o or /b/{bucket}/o/{object}
	if len(parts) == 1 && parts[0] == "b" {
		if r.Method == http.MethodGet {
			var names []string
			for k := range s.buckets {
				names = append(names, k)
			}
			sort.Strings(names)
			var items []*storage.Bucket
			for _, k := range names {
				items = append(items, s.buckets[k])
			}
			writeJSON(w, http.StatusOK, &storage.Buckets{Items: items})
			return
		}
		if r.Method == http.MethodPost {
			var b storage.Bucket
			if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
				writeErr(w, http.StatusBadRequest, err.Error())
				return
			}
			if b.Location == "" {
				b.Location = "US-CENTRAL1"
			}
			if b.StorageClass == "" {
				b.StorageClass = "STANDARD"
			}
			if b.Versioning == nil {
				b.Versioning = &storage.BucketVersioning{Enabled: false}
			}
			if b.IamConfiguration == nil {
				b.IamConfiguration = &storage.BucketIamConfiguration{
					UniformBucketLevelAccess: &storage.BucketIamConfigurationUniformBucketLevelAccess{Enabled: true},
					PublicAccessPrevention:   "enforced",
				}
			}
			s.buckets[b.Name] = &b
			if _, ok := s.objects[b.Name]; !ok {
				s.objects[b.Name] = make(map[string]storedObject)
			}
			writeJSON(w, http.StatusOK, &b)
			return
		}
	}
	if len(parts) == 2 && parts[0] == "b" {
		bktName := parts[1]
		bkt, ok := s.buckets[bktName]
		if !ok {
			writeErr(w, http.StatusNotFound, "bucket not found: "+bktName)
			return
		}
		if r.Method == http.MethodGet {
			writeJSON(w, http.StatusOK, bkt)
			return
		}
		if r.Method == http.MethodPatch || r.Method == http.MethodPut {
			var patch storage.Bucket
			if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
				writeErr(w, http.StatusBadRequest, err.Error())
				return
			}
			if patch.Versioning != nil {
				bkt.Versioning = patch.Versioning
			}
			if patch.Lifecycle != nil {
				bkt.Lifecycle = patch.Lifecycle
			}
			if patch.IamConfiguration != nil {
				bkt.IamConfiguration = patch.IamConfiguration
			}
			if patch.Labels != nil {
				bkt.Labels = patch.Labels
			}
			writeJSON(w, http.StatusOK, bkt)
			return
		}
		if r.Method == http.MethodDelete {
			delete(s.buckets, bktName)
			delete(s.bucketStats, bktName)
			delete(s.objects, bktName)
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}
	if len(parts) >= 3 && parts[0] == "b" && parts[2] == "o" {
		bktName := parts[1]
		objs, ok := s.objects[bktName]
		if !ok {
			writeErr(w, http.StatusNotFound, "bucket not found: "+bktName)
			return
		}
		if len(parts) == 3 && r.Method == http.MethodGet {
			prefix := r.URL.Query().Get("prefix")
			var names []string
			for k := range objs {
				if prefix == "" || strings.HasPrefix(k, prefix) {
					names = append(names, k)
				}
			}
			sort.Strings(names)
			var items []*storage.Object
			for _, k := range names {
				o := objs[k]
				items = append(items, &storage.Object{
					Bucket:      bktName,
					Name:        o.Name,
					ContentType: o.ContentType,
					Size:        uint64(len(o.Content)),
				})
			}
			writeJSON(w, http.StatusOK, &storage.Objects{Items: items})
			return
		}
		if len(parts) >= 4 {
			objName := strings.Join(parts[3:], "/")
			if r.Method == http.MethodGet {
				o, exists := objs[objName]
				if !exists {
					writeErr(w, http.StatusNotFound, "object not found: "+objName)
					return
				}
				writeJSON(w, http.StatusOK, &storage.Object{
					Bucket:      bktName,
					Name:        o.Name,
					ContentType: o.ContentType,
					Size:        uint64(len(o.Content)),
				})
				return
			}
			if r.Method == http.MethodDelete {
				if o, exists := objs[objName]; exists {
					st := s.bucketStats[bktName]
					if st.ObjectCount > 0 {
						st.ObjectCount--
					}
					st.TotalBytes -= int64(len(o.Content))
					if st.TotalBytes < 0 {
						st.TotalBytes = 0
					}
					s.bucketStats[bktName] = st
					delete(objs, objName)
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
	}
	writeErr(w, http.StatusNotFound, "unhandled storage route: "+sub)
}

// --- Pub/Sub handlers ---
func (s *Server) handlePubsub(w http.ResponseWriter, r *http.Request, sub string) {
	// Routes start with /v1/projects/{p}/topics or /v1/projects/{p}/subscriptions
	trimmed := strings.TrimPrefix(sub, "/v1/")
	parts := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(parts) < 3 || parts[0] != "projects" {
		writeErr(w, http.StatusNotFound, "bad pubsub route: "+sub)
		return
	}
	kind := parts[2]
	if kind == "topics" {
		if len(parts) == 3 && r.Method == http.MethodGet {
			var names []string
			for k := range s.topics {
				names = append(names, k)
			}
			sort.Strings(names)
			var list []*pubsub.Topic
			for _, k := range names {
				list = append(list, s.topics[k])
			}
			writeJSON(w, http.StatusOK, &pubsub.ListTopicsResponse{Topics: list})
			return
		}
		if len(parts) == 4 {
			raw := parts[3]
			if strings.HasSuffix(raw, ":publish") && r.Method == http.MethodPost {
				topicName := strings.TrimSuffix(raw, ":publish")
				if _, ok := s.topics[topicName]; !ok {
					writeErr(w, http.StatusNotFound, "topic not found: "+topicName)
					return
				}
				var req pubsub.PublishRequest
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					writeErr(w, http.StatusBadRequest, err.Error())
					return
				}
				var ids []string
				for _, m := range req.Messages {
					s.msgSeq++
					id := fmt.Sprintf("msg-%d", s.msgSeq)
					ids = append(ids, id)
					decoded, err := base64.StdEncoding.DecodeString(m.Data)
					payload := m.Data
					if err == nil {
						payload = string(decoded)
					}
					s.topicStats[topicName]++
					for subName, subObj := range s.subscriptions {
						if lastSegment(subObj.Topic) == topicName {
							s.subMessages[subName] = append(s.subMessages[subName], pubsubMsg{
								ID:         id,
								Data:       payload,
								Attributes: m.Attributes,
							})
							s.subBacklog[subName]++
						}
					}
				}
				writeJSON(w, http.StatusOK, &pubsub.PublishResponse{MessageIds: ids})
				return
			}
			topicName := raw
			switch r.Method {
			case http.MethodPut:
				var t pubsub.Topic
				_ = json.NewDecoder(r.Body).Decode(&t)
				t.Name = fmt.Sprintf("projects/%s/topics/%s", parts[1], topicName)
				if t.MessageRetentionDuration == "" {
					t.MessageRetentionDuration = "604800s"
				}
				s.topics[topicName] = &t
				writeJSON(w, http.StatusOK, &t)
				return
			case http.MethodPatch:
				var req pubsub.UpdateTopicRequest
				_ = json.NewDecoder(r.Body).Decode(&req)
				t, ok := s.topics[topicName]
				if !ok {
					writeErr(w, http.StatusNotFound, "topic not found")
					return
				}
				if req.Topic != nil && req.Topic.Labels != nil {
					t.Labels = req.Topic.Labels
				}
				writeJSON(w, http.StatusOK, t)
				return
			case http.MethodGet:
				t, ok := s.topics[topicName]
				if !ok {
					writeErr(w, http.StatusNotFound, "topic not found")
					return
				}
				writeJSON(w, http.StatusOK, t)
				return
			case http.MethodDelete:
				delete(s.topics, topicName)
				delete(s.topicStats, topicName)
				writeJSON(w, http.StatusOK, map[string]any{})
				return
			}
		}
	}
	if kind == "subscriptions" {
		if len(parts) == 3 && r.Method == http.MethodGet {
			var names []string
			for k := range s.subscriptions {
				names = append(names, k)
			}
			sort.Strings(names)
			var list []*pubsub.Subscription
			for _, k := range names {
				list = append(list, s.subscriptions[k])
			}
			writeJSON(w, http.StatusOK, &pubsub.ListSubscriptionsResponse{Subscriptions: list})
			return
		}
		if len(parts) == 4 {
			raw := parts[3]
			if strings.HasSuffix(raw, ":pull") && r.Method == http.MethodPost {
				subName := strings.TrimSuffix(raw, ":pull")
				if _, ok := s.subscriptions[subName]; !ok {
					writeErr(w, http.StatusNotFound, "subscription not found: "+subName)
					return
				}
				var req pubsub.PullRequest
				_ = json.NewDecoder(r.Body).Decode(&req)
				maxN := int(req.MaxMessages)
				if maxN <= 0 {
					maxN = 10
				}
				queue := s.subMessages[subName]
				if len(queue) > maxN {
					queue = queue[:maxN]
				}
				var received []*pubsub.ReceivedMessage
				for _, m := range queue {
					received = append(received, &pubsub.ReceivedMessage{
						AckId: "ack-" + m.ID,
						Message: &pubsub.PubsubMessage{
							MessageId:  m.ID,
							Data:       base64.StdEncoding.EncodeToString([]byte(m.Data)),
							Attributes: m.Attributes,
						},
					})
				}
				s.subMessages[subName] = s.subMessages[subName][len(queue):]
				s.subBacklog[subName] -= int64(len(queue))
				if s.subBacklog[subName] < 0 {
					s.subBacklog[subName] = 0
				}
				writeJSON(w, http.StatusOK, &pubsub.PullResponse{ReceivedMessages: received})
				return
			}
			subName := raw
			switch r.Method {
			case http.MethodPut:
				var sb pubsub.Subscription
				_ = json.NewDecoder(r.Body).Decode(&sb)
				sb.Name = fmt.Sprintf("projects/%s/subscriptions/%s", parts[1], subName)
				if sb.AckDeadlineSeconds == 0 {
					sb.AckDeadlineSeconds = 20
				}
				s.subscriptions[subName] = &sb
				writeJSON(w, http.StatusOK, &sb)
				return
			case http.MethodGet:
				sb, ok := s.subscriptions[subName]
				if !ok {
					writeErr(w, http.StatusNotFound, "subscription not found")
					return
				}
				writeJSON(w, http.StatusOK, sb)
				return
			case http.MethodDelete:
				delete(s.subscriptions, subName)
				delete(s.subBacklog, subName)
				delete(s.subMessages, subName)
				writeJSON(w, http.StatusOK, map[string]any{})
				return
			}
		}
	}
	writeErr(w, http.StatusNotFound, "unhandled pubsub route: "+sub)
}

// --- Cloud Run v2 handlers ---
func (s *Server) handleCloudRun(w http.ResponseWriter, r *http.Request, sub string) {
	// /v2/projects/{p}/locations/{loc}/services[/{name}]
	trimmed := strings.TrimPrefix(sub, "/v2/")
	parts := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(parts) < 5 || parts[0] != "projects" || parts[2] != "locations" || parts[4] != "services" {
		writeErr(w, http.StatusNotFound, "bad run route: "+sub)
		return
	}
	loc := parts[3]
	if len(parts) == 5 {
		if r.Method == http.MethodGet {
			var keys []string
			for k := range s.runServices {
				if loc == "-" || loc == "all" || strings.HasPrefix(k, loc+"/") {
					keys = append(keys, k)
				}
			}
			sort.Strings(keys)
			var list []*run.GoogleCloudRunV2Service
			for _, k := range keys {
				list = append(list, s.runServices[k])
			}
			writeJSON(w, http.StatusOK, &run.GoogleCloudRunV2ListServicesResponse{Services: list})
			return
		}
		if r.Method == http.MethodPost {
			svcID := r.URL.Query().Get("serviceId")
			var svc run.GoogleCloudRunV2Service
			if err := json.NewDecoder(r.Body).Decode(&svc); err != nil {
				writeErr(w, http.StatusBadRequest, err.Error())
				return
			}
			if svcID == "" {
				svcID = lastSegment(svc.Name)
			}
			svc.Name = fmt.Sprintf("projects/%s/locations/%s/services/%s", parts[1], loc, svcID)
			svc.Uri = fmt.Sprintf("https://%s-masked-%s.a.run.app", svcID, loc)
			if svc.Ingress == "" {
				svc.Ingress = "INGRESS_TRAFFIC_ALL"
			}
			svc.LatestReadyRevision = svcID + "-rev-001"
			key := loc + "/" + svcID
			s.runServices[key] = &svc
			rawResp, _ := json.Marshal(&svc)
			writeJSON(w, http.StatusOK, &run.GoogleLongrunningOperation{
				Name:     "operations/run-" + svcID,
				Done:     true,
				Response: rawResp,
			})
			return
		}
	} else if len(parts) == 6 {
		svcID := parts[5]
		key := loc + "/" + svcID
		svc, ok := s.runServices[key]
		if !ok {
			writeErr(w, http.StatusNotFound, "cloud run service not found: "+key)
			return
		}
		if r.Method == http.MethodGet {
			writeJSON(w, http.StatusOK, svc)
			return
		}
		if r.Method == http.MethodPatch {
			var patch run.GoogleCloudRunV2Service
			_ = json.NewDecoder(r.Body).Decode(&patch)
			if patch.Labels != nil {
				svc.Labels = patch.Labels
			}
			if len(patch.Traffic) > 0 {
				svc.Traffic = patch.Traffic
			}
			rawResp, _ := json.Marshal(svc)
			writeJSON(w, http.StatusOK, &run.GoogleLongrunningOperation{
				Name:     "operations/run-patch-" + svcID,
				Done:     true,
				Response: rawResp,
			})
			return
		}
		if r.Method == http.MethodDelete {
			delete(s.runServices, key)
			delete(s.runMeta, key)
			writeJSON(w, http.StatusOK, &run.GoogleLongrunningOperation{
				Name: "operations/run-del-" + svcID,
				Done: true,
			})
			return
		}
	}
	writeErr(w, http.StatusNotFound, "unhandled cloud run route: "+sub)
}

// --- Cloud SQL Admin handlers ---
func (s *Server) handleSQL(w http.ResponseWriter, r *http.Request, sub string) {
	// /v1/projects/{p}/instances[/{name}] or /sql/v1beta4/projects/{p}/instances
	trimmed := strings.TrimPrefix(strings.TrimPrefix(sub, "/sql/v1beta4/"), "/v1/")
	parts := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(parts) < 3 || parts[0] != "projects" || parts[2] != "instances" {
		writeErr(w, http.StatusNotFound, "bad sql route: "+sub)
		return
	}
	if len(parts) == 3 {
		if r.Method == http.MethodGet {
			var names []string
			for k := range s.sqlInstances {
				names = append(names, k)
			}
			sort.Strings(names)
			var items []*sqladmin.DatabaseInstance
			for _, k := range names {
				items = append(items, s.sqlInstances[k])
			}
			writeJSON(w, http.StatusOK, &sqladmin.InstancesListResponse{Items: items})
			return
		}
		if r.Method == http.MethodPost {
			var inst sqladmin.DatabaseInstance
			if err := json.NewDecoder(r.Body).Decode(&inst); err != nil {
				writeErr(w, http.StatusBadRequest, err.Error())
				return
			}
			inst.State = "RUNNABLE"
			s.sqlInstances[inst.Name] = &inst
			writeJSON(w, http.StatusOK, &sqladmin.Operation{Name: "op-sql-insert", Status: "DONE"})
			return
		}
	} else if len(parts) == 4 {
		name := parts[3]
		if r.Method == http.MethodGet {
			inst, ok := s.sqlInstances[name]
			if !ok {
				writeErr(w, http.StatusNotFound, "sql instance not found: "+name)
				return
			}
			writeJSON(w, http.StatusOK, inst)
			return
		}
		if r.Method == http.MethodDelete {
			delete(s.sqlInstances, name)
			writeJSON(w, http.StatusOK, &sqladmin.Operation{Name: "op-sql-del", Status: "DONE"})
			return
		}
	}
	writeErr(w, http.StatusNotFound, "unhandled sql route: "+sub)
}

// --- BigQuery handlers ---
func (s *Server) handleBigQuery(w http.ResponseWriter, r *http.Request, sub string) {
	parts := strings.Split(strings.Trim(sub, "/"), "/")
	if len(parts) < 3 || parts[0] != "projects" || parts[2] != "datasets" {
		writeErr(w, http.StatusNotFound, "bad bigquery route: "+sub)
		return
	}
	if len(parts) == 3 {
		if r.Method == http.MethodGet {
			var names []string
			for k := range s.bqDatasets {
				names = append(names, k)
			}
			sort.Strings(names)
			var list []*bigquery.DatasetListDatasets
			for _, k := range names {
				ds := s.bqDatasets[k]
				list = append(list, &bigquery.DatasetListDatasets{
					DatasetReference: ds.DatasetReference,
					Location:         ds.Location,
					Labels:           ds.Labels,
				})
			}
			writeJSON(w, http.StatusOK, &bigquery.DatasetList{Datasets: list})
			return
		}
		if r.Method == http.MethodPost {
			var ds bigquery.Dataset
			_ = json.NewDecoder(r.Body).Decode(&ds)
			id := ds.DatasetReference.DatasetId
			s.bqDatasets[id] = &ds
			writeJSON(w, http.StatusOK, &ds)
			return
		}
	} else if len(parts) == 4 {
		dsID := parts[3]
		if r.Method == http.MethodGet {
			ds, ok := s.bqDatasets[dsID]
			if !ok {
				writeErr(w, http.StatusNotFound, "dataset not found: "+dsID)
				return
			}
			writeJSON(w, http.StatusOK, ds)
			return
		}
		if r.Method == http.MethodDelete {
			delete(s.bqDatasets, dsID)
			for k := range s.bqTables {
				if strings.HasPrefix(k, dsID+"/") {
					delete(s.bqTables, k)
				}
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
	} else if len(parts) >= 5 && parts[4] == "tables" {
		dsID := parts[3]
		if len(parts) == 5 && r.Method == http.MethodPost {
			var tbl bigquery.Table
			_ = json.NewDecoder(r.Body).Decode(&tbl)
			key := dsID + "/" + tbl.TableReference.TableId
			s.bqTables[key] = &tbl
			writeJSON(w, http.StatusOK, &tbl)
			return
		}
		if len(parts) == 6 {
			tblID := parts[5]
			key := dsID + "/" + tblID
			if r.Method == http.MethodGet {
				tbl, ok := s.bqTables[key]
				if !ok {
					writeErr(w, http.StatusNotFound, "table not found: "+key)
					return
				}
				writeJSON(w, http.StatusOK, tbl)
				return
			}
			if r.Method == http.MethodDelete {
				delete(s.bqTables, key)
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
	}
	writeErr(w, http.StatusNotFound, "unhandled bigquery route: "+sub)
}

// --- IAM & Cloud Resource Manager handlers ---
func (s *Server) handleIAM(w http.ResponseWriter, r *http.Request, sub string) {
	trimmed := strings.TrimPrefix(sub, "/v1/")
	parts := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(parts) >= 3 && parts[0] == "projects" && parts[2] == "serviceAccounts" {
		if len(parts) == 3 && r.Method == http.MethodGet {
			var keys []string
			for k := range s.serviceAccounts {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			var list []*iam.ServiceAccount
			for _, k := range keys {
				list = append(list, s.serviceAccounts[k])
			}
			writeJSON(w, http.StatusOK, &iam.ListServiceAccountsResponse{Accounts: list})
			return
		}
		if len(parts) == 4 && r.Method == http.MethodGet {
			id := parts[3]
			if sa, ok := s.serviceAccounts[id]; ok {
				writeJSON(w, http.StatusOK, sa)
				return
			}
			for email, info := range s.saMeta {
				if info.ShortName == id {
					writeJSON(w, http.StatusOK, s.serviceAccounts[email])
					return
				}
			}
			writeErr(w, http.StatusNotFound, "service account not found: "+id)
			return
		}
	}
	writeErr(w, http.StatusNotFound, "unhandled iam route: "+sub)
}

func (s *Server) handleCRM(w http.ResponseWriter, r *http.Request, sub string) {
	if strings.HasSuffix(sub, ":getIamPolicy") && r.Method == http.MethodPost {
		writeJSON(w, http.StatusOK, s.iamPolicy)
		return
	}
	writeErr(w, http.StatusNotFound, "unhandled crm route: "+sub)
}

// --- Secret Manager (Metadata ONLY) handlers ---
func (s *Server) handleSecrets(w http.ResponseWriter, r *http.Request, sub string) {
	trimmed := strings.TrimPrefix(sub, "/v1/")
	parts := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(parts) >= 3 && parts[0] == "projects" && parts[2] == "secrets" {
		if len(parts) == 3 {
			if r.Method == http.MethodGet {
				var names []string
				for k := range s.secrets {
					names = append(names, k)
				}
				sort.Strings(names)
				var list []*secretmanager.Secret
				for _, k := range names {
					list = append(list, s.secrets[k])
				}
				writeJSON(w, http.StatusOK, &secretmanager.ListSecretsResponse{Secrets: list})
				return
			}
			if r.Method == http.MethodPost {
				secID := r.URL.Query().Get("secretId")
				var sec secretmanager.Secret
				_ = json.NewDecoder(r.Body).Decode(&sec)
				sec.Name = fmt.Sprintf("projects/%s/secrets/%s", parts[1], secID)
				s.secrets[secID] = &sec
				writeJSON(w, http.StatusOK, &sec)
				return
			}
		} else if len(parts) == 4 {
			secID := parts[3]
			if r.Method == http.MethodGet {
				sec, ok := s.secrets[secID]
				if !ok {
					writeErr(w, http.StatusNotFound, "secret metadata not found: "+secID)
					return
				}
				writeJSON(w, http.StatusOK, sec)
				return
			}
			if r.Method == http.MethodDelete {
				delete(s.secrets, secID)
				delete(s.secretMeta, secID)
				writeJSON(w, http.StatusOK, map[string]any{})
				return
			}
		}
	}
	writeErr(w, http.StatusNotFound, "unhandled secrets route: "+sub)
}

// --- Cloud KMS handlers ---
func (s *Server) handleKMS(w http.ResponseWriter, r *http.Request, sub string) {
	trimmed := strings.TrimPrefix(sub, "/v1/")
	parts := strings.Split(strings.Trim(trimmed, "/"), "/")
	// projects/{p}/locations/{loc}/keyRings[/{ring}/cryptoKeys[/{key}]]
	if len(parts) >= 5 && parts[0] == "projects" && parts[2] == "locations" && parts[4] == "keyRings" {
		loc := parts[3]
		if len(parts) == 5 && r.Method == http.MethodGet {
			var keys []string
			for k := range s.kmsKeyRings {
				if strings.HasPrefix(k, loc+"/") {
					keys = append(keys, k)
				}
			}
			sort.Strings(keys)
			var list []*cloudkms.KeyRing
			for _, k := range keys {
				list = append(list, s.kmsKeyRings[k])
			}
			writeJSON(w, http.StatusOK, &cloudkms.ListKeyRingsResponse{KeyRings: list})
			return
		}
		if len(parts) >= 7 && parts[6] == "cryptoKeys" {
			ring := parts[5]
			prefix := loc + "/" + ring + "/"
			if len(parts) == 7 {
				if r.Method == http.MethodGet {
					var keys []string
					for k := range s.kmsKeys {
						if strings.HasPrefix(k, prefix) {
							keys = append(keys, k)
						}
					}
					sort.Strings(keys)
					var list []*cloudkms.CryptoKey
					for _, k := range keys {
						list = append(list, s.kmsKeys[k])
					}
					writeJSON(w, http.StatusOK, &cloudkms.ListCryptoKeysResponse{CryptoKeys: list})
					return
				}
				if r.Method == http.MethodPost {
					keyID := r.URL.Query().Get("cryptoKeyId")
					var ck cloudkms.CryptoKey
					_ = json.NewDecoder(r.Body).Decode(&ck)
					ck.Name = fmt.Sprintf("projects/%s/locations/%s/keyRings/%s/cryptoKeys/%s", parts[1], loc, ring, keyID)
					if ck.Primary == nil {
						ck.Primary = &cloudkms.CryptoKeyVersion{State: "ENABLED", ProtectionLevel: "HSM"}
					}
					s.kmsKeyRings[loc+"/"+ring] = &cloudkms.KeyRing{
						Name: fmt.Sprintf("projects/%s/locations/%s/keyRings/%s", parts[1], loc, ring),
					}
					s.kmsKeys[prefix+keyID] = &ck
					writeJSON(w, http.StatusOK, &ck)
					return
				}
			} else if len(parts) == 8 && r.Method == http.MethodGet {
				keyID := parts[7]
				ck, ok := s.kmsKeys[prefix+keyID]
				if !ok {
					writeErr(w, http.StatusNotFound, "kms key not found: "+keyID)
					return
				}
				writeJSON(w, http.StatusOK, ck)
				return
			}
		}
	}
	writeErr(w, http.StatusNotFound, "unhandled kms route: "+sub)
}

// --- GKE & Artifact Registry handlers ---
func (s *Server) handleGKE(w http.ResponseWriter, r *http.Request, sub string) {
	trimmed := strings.TrimPrefix(sub, "/v1/")
	parts := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(parts) >= 5 && parts[0] == "projects" && parts[2] == "locations" && parts[4] == "clusters" {
		loc := parts[3]
		if len(parts) == 5 && r.Method == http.MethodGet {
			var keys []string
			for k := range s.gkeClusters {
				if loc == "-" || loc == "all" || strings.HasPrefix(k, loc+"/") {
					keys = append(keys, k)
				}
			}
			sort.Strings(keys)
			var list []*container.Cluster
			for _, k := range keys {
				list = append(list, s.gkeClusters[k])
			}
			writeJSON(w, http.StatusOK, &container.ListClustersResponse{Clusters: list})
			return
		}
		if len(parts) == 6 && r.Method == http.MethodGet {
			key := loc + "/" + parts[5]
			c, ok := s.gkeClusters[key]
			if !ok {
				writeErr(w, http.StatusNotFound, "gke cluster not found: "+key)
				return
			}
			writeJSON(w, http.StatusOK, c)
			return
		}
	}
	writeErr(w, http.StatusNotFound, "unhandled gke route: "+sub)
}

func (s *Server) handleAR(w http.ResponseWriter, r *http.Request, sub string) {
	trimmed := strings.TrimPrefix(sub, "/v1/")
	parts := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(parts) >= 5 && parts[0] == "projects" && parts[2] == "locations" && parts[4] == "repositories" {
		loc := parts[3]
		if len(parts) == 5 {
			if r.Method == http.MethodGet {
				var keys []string
				for k := range s.arRepos {
					if loc == "-" || loc == "all" || strings.HasPrefix(k, loc+"/") {
						keys = append(keys, k)
					}
				}
				sort.Strings(keys)
				var list []*artifactregistry.Repository
				for _, k := range keys {
					list = append(list, s.arRepos[k])
				}
				writeJSON(w, http.StatusOK, &artifactregistry.ListRepositoriesResponse{Repositories: list})
				return
			}
			if r.Method == http.MethodPost {
				repoID := r.URL.Query().Get("repositoryId")
				var repo artifactregistry.Repository
				_ = json.NewDecoder(r.Body).Decode(&repo)
				repo.Name = fmt.Sprintf("projects/%s/locations/%s/repositories/%s", parts[1], loc, repoID)
				s.arRepos[loc+"/"+repoID] = &repo
				rawResp, _ := json.Marshal(&repo)
				writeJSON(w, http.StatusOK, &artifactregistry.Operation{
					Name:     "operations/ar-" + repoID,
					Done:     true,
					Response: rawResp,
				})
				return
			}
		} else if len(parts) == 6 {
			repoID := parts[5]
			key := loc + "/" + repoID
			if r.Method == http.MethodGet {
				repo, ok := s.arRepos[key]
				if !ok {
					writeErr(w, http.StatusNotFound, "repository not found: "+key)
					return
				}
				writeJSON(w, http.StatusOK, repo)
				return
			}
			if r.Method == http.MethodDelete {
				delete(s.arRepos, key)
				delete(s.arMeta, key)
				writeJSON(w, http.StatusOK, &artifactregistry.Operation{Name: "operations/ar-del-" + repoID, Done: true})
				return
			}
		}
	}
	writeErr(w, http.StatusNotFound, "unhandled artifact registry route: "+sub)
}

// --- Cloud DNS handlers ---
func (s *Server) handleDNS(w http.ResponseWriter, r *http.Request, sub string) {
	trimmed := strings.TrimPrefix(strings.TrimPrefix(sub, "/dns/v1/"), "/v1/")
	parts := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(parts) >= 3 && parts[0] == "projects" && parts[2] == "managedZones" {
		if len(parts) == 3 {
			if r.Method == http.MethodGet {
				var names []string
				for k := range s.dnsZones {
					names = append(names, k)
				}
				sort.Strings(names)
				var list []*dns.ManagedZone
				for _, k := range names {
					list = append(list, s.dnsZones[k])
				}
				writeJSON(w, http.StatusOK, &dns.ManagedZonesListResponse{ManagedZones: list})
				return
			}
			if r.Method == http.MethodPost {
				var z dns.ManagedZone
				_ = json.NewDecoder(r.Body).Decode(&z)
				s.dnsZones[z.Name] = &z
				writeJSON(w, http.StatusOK, &z)
				return
			}
		} else if len(parts) == 4 {
			zName := parts[3]
			if r.Method == http.MethodGet {
				z, ok := s.dnsZones[zName]
				if !ok {
					writeErr(w, http.StatusNotFound, "dns zone not found: "+zName)
					return
				}
				writeJSON(w, http.StatusOK, z)
				return
			}
			if r.Method == http.MethodDelete {
				delete(s.dnsZones, zName)
				delete(s.dnsRecords, zName)
				w.WriteHeader(http.StatusNoContent)
				return
			}
		} else if len(parts) >= 5 && parts[4] == "rrsets" {
			zName := parts[3]
			if len(parts) == 5 {
				if r.Method == http.MethodGet {
					writeJSON(w, http.StatusOK, &dns.ResourceRecordSetsListResponse{Rrsets: s.dnsRecords[zName]})
					return
				}
				if r.Method == http.MethodPost {
					var rec dns.ResourceRecordSet
					_ = json.NewDecoder(r.Body).Decode(&rec)
					s.dnsRecords[zName] = append(s.dnsRecords[zName], &rec)
					writeJSON(w, http.StatusOK, &rec)
					return
				}
			} else if len(parts) == 7 && r.Method == http.MethodDelete {
				fqdn := parts[5]
				rtype := parts[6]
				recs := s.dnsRecords[zName]
				kept := recs[:0]
				for _, rec := range recs {
					if rec.Name == fqdn && rec.Type == rtype {
						continue
					}
					kept = append(kept, rec)
				}
				s.dnsRecords[zName] = kept
				writeJSON(w, http.StatusOK, map[string]any{})
				return
			}
		}
	}
	writeErr(w, http.StatusNotFound, "unhandled dns route: "+sub)
}

// --- Cloud Logging & Cloud Monitoring handlers ---
func (s *Server) handleLogging(w http.ResponseWriter, r *http.Request, sub string) {
	trimmed := strings.TrimPrefix(sub, "/v2/")
	if trimmed == "entries:write" && r.Method == http.MethodPost {
		body, _ := io.ReadAll(r.Body)
		s.logEntries = append(s.logEntries, string(body))
		s.logSeverityCounts["INFO"]++
		writeJSON(w, http.StatusOK, &logging.WriteLogEntriesResponse{})
		return
	}
	parts := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(parts) >= 3 && parts[0] == "projects" && parts[2] == "sinks" {
		if len(parts) == 3 {
			if r.Method == http.MethodGet {
				var names []string
				for k := range s.logSinks {
					names = append(names, k)
				}
				sort.Strings(names)
				var list []*logging.LogSink
				for _, k := range names {
					list = append(list, s.logSinks[k])
				}
				writeJSON(w, http.StatusOK, &logging.ListSinksResponse{Sinks: list})
				return
			}
			if r.Method == http.MethodPost {
				var sk logging.LogSink
				_ = json.NewDecoder(r.Body).Decode(&sk)
				s.logSinks[sk.Name] = &sk
				writeJSON(w, http.StatusOK, &sk)
				return
			}
		} else if len(parts) == 4 {
			name := parts[3]
			if r.Method == http.MethodGet {
				sk, ok := s.logSinks[name]
				if !ok {
					writeErr(w, http.StatusNotFound, "sink not found: "+name)
					return
				}
				writeJSON(w, http.StatusOK, sk)
				return
			}
			if r.Method == http.MethodDelete {
				delete(s.logSinks, name)
				writeJSON(w, http.StatusOK, map[string]any{})
				return
			}
		}
	}
	writeErr(w, http.StatusNotFound, "unhandled logging route: "+sub)
}

func (s *Server) handleMonitoring(w http.ResponseWriter, r *http.Request, sub string) {
	trimmed := strings.TrimPrefix(sub, "/v3/")
	parts := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(parts) >= 3 && parts[0] == "projects" && parts[2] == "alertPolicies" {
		if len(parts) == 3 {
			if r.Method == http.MethodGet {
				var names []string
				for k := range s.alertPolicies {
					names = append(names, k)
				}
				sort.Strings(names)
				var list []*monitoring.AlertPolicy
				for _, k := range names {
					list = append(list, s.alertPolicies[k])
				}
				writeJSON(w, http.StatusOK, &monitoring.ListAlertPoliciesResponse{AlertPolicies: list})
				return
			}
			if r.Method == http.MethodPost {
				var ap monitoring.AlertPolicy
				_ = json.NewDecoder(r.Body).Decode(&ap)
				id := ap.DisplayName
				ap.Name = fmt.Sprintf("projects/%s/alertPolicies/%s", parts[1], id)
				s.alertPolicies[id] = &ap
				writeJSON(w, http.StatusOK, &ap)
				return
			}
		} else if len(parts) == 4 {
			id := parts[3]
			if r.Method == http.MethodGet {
				ap, ok := s.alertPolicies[id]
				if !ok {
					writeErr(w, http.StatusNotFound, "alert policy not found: "+id)
					return
				}
				writeJSON(w, http.StatusOK, ap)
				return
			}
			if r.Method == http.MethodDelete {
				delete(s.alertPolicies, id)
				delete(s.alertMeta, id)
				writeJSON(w, http.StatusOK, map[string]any{})
				return
			}
		}
	}
	writeErr(w, http.StatusNotFound, "unhandled monitoring route: "+sub)
}

// --- Direct state helpers for supplemental metadata and fast in-process inspection ---

func (s *Server) PutObject(bucket, name, content, contentType string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.buckets[bucket]; !ok {
		return fmt.Errorf("bucket %q not found", bucket)
	}
	if s.objects[bucket] == nil {
		s.objects[bucket] = make(map[string]storedObject)
	}
	prev, existed := s.objects[bucket][name]
	s.objects[bucket][name] = storedObject{
		Name:        name,
		ContentType: contentType,
		Content:     content,
	}
	st := s.bucketStats[bucket]
	if !existed {
		st.ObjectCount++
	} else {
		st.TotalBytes -= int64(len(prev.Content))
	}
	st.TotalBytes += int64(len(content))
	s.bucketStats[bucket] = st
	return nil
}

func (s *Server) ReadObject(bucket, name string) (string, int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	objs, ok := s.objects[bucket]
	if !ok {
		return "", 0, fmt.Errorf("bucket %q not found", bucket)
	}
	o, exists := objs[name]
	if !exists {
		return "", 0, fmt.Errorf("object %q not found in bucket %q", name, bucket)
	}
	return o.Content, int64(len(o.Content)), nil
}

func (s *Server) BucketStats(bucket string) (int64, int64) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	st := s.bucketStats[bucket]
	return st.ObjectCount, st.TotalBytes
}

func (s *Server) TopicPublishedCount(topic string) int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.topicStats[topic]
}

func (s *Server) SubscriptionBacklog(sub string) int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.subBacklog[sub]
}

func (s *Server) SetRunMeta(region, name string, cpuMilli, memMB, revCount, stablePct, canaryPct int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runMeta[region+"/"+name] = runMetaInfo{
		CPUMilli:      cpuMilli,
		MemoryMB:      memMB,
		RevisionCount: revCount,
		StablePct:     stablePct,
		CanaryPct:     canaryPct,
	}
}

func (s *Server) GetRunMeta(region, name string) (int64, int64, int64, int64, int64) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m := s.runMeta[region+"/"+name]
	return m.CPUMilli, m.MemoryMB, m.RevisionCount, m.StablePct, m.CanaryPct
}

func (s *Server) GetSAMeta(email string) (string, int64, int64, bool, []string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m := s.saMeta[email]
	return m.ShortName, m.UserManagedKeys, m.KeyMaxAgeDays, m.IsDefault, append([]string(nil), m.Roles...)
}

func (s *Server) SetSecretMeta(name, repl string, rotDays, verCount, enabledVers int64, cmek, hasExp bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.secretMeta[name] = secretMetaInfo{
		ReplicationPolicy: repl,
		RotationDays:      rotDays,
		VersionCount:      verCount,
		EnabledVersions:   enabledVers,
		CMEKEnabled:       cmek,
		HasExpiration:     hasExp,
	}
}

func (s *Server) GetSecretMeta(name string) (string, int64, int64, int64, bool, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m := s.secretMeta[name]
	return m.ReplicationPolicy, m.RotationDays, m.VersionCount, m.EnabledVersions, m.CMEKEnabled, m.HasExpiration
}

func (s *Server) SetKMSRotDays(loc, ring, key string, days int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.kmsRotDays[loc+"/"+ring+"/"+key] = days
}

func (s *Server) GetKMSRotDays(loc, ring, key string) int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.kmsRotDays[loc+"/"+ring+"/"+key]
}

func (s *Server) GetGKEMeta(loc, name string) (bool, bool, bool, bool, bool, int64, int64) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m := s.gkeMeta[loc+"/"+name]
	return m.Autopilot, m.PrivateNodes, m.WorkloadIdentity, m.NetworkPolicy, m.BinaryAuth, m.NodePoolCount, m.TotalNodes
}

func (s *Server) SetARMeta(loc, name string, immut, vulnScan bool, cleanup, sizeMB, pkgCount int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.arMeta[loc+"/"+name] = arMetaInfo{
		ImmutableTags:         immut,
		CleanupPolicies:       cleanup,
		VulnerabilityScanning: vulnScan,
		SizeMB:                sizeMB,
		PackageCount:          pkgCount,
	}
}

func (s *Server) GetARMeta(loc, name string) (bool, bool, int64, int64, int64) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m := s.arMeta[loc+"/"+name]
	return m.ImmutableTags, m.VulnerabilityScanning, m.CleanupPolicies, m.SizeMB, m.PackageCount
}

func (s *Server) FirestoreListCollections() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var cols []string
	for k := range s.firestoreDocs {
		cols = append(cols, k)
	}
	sort.Strings(cols)
	return cols
}

func (s *Server) FirestorePutDoc(col, docID string, fields map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.firestoreDocs[col] == nil {
		s.firestoreDocs[col] = make(map[string]map[string]string)
	}
	cp := make(map[string]string, len(fields))
	for k, v := range fields {
		cp[k] = v
	}
	s.firestoreDocs[col][docID] = cp
}

func (s *Server) FirestoreGetDoc(col, docID string) (map[string]string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	docs, ok := s.firestoreDocs[col]
	if !ok {
		return nil, false
	}
	d, exists := docs[docID]
	if !exists {
		return nil, false
	}
	cp := make(map[string]string, len(d))
	for k, v := range d {
		cp[k] = v
	}
	return cp, true
}

func (s *Server) FirestoreListDocs(col string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	docs := s.firestoreDocs[col]
	var ids []string
	for id := range docs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func (s *Server) FirestoreDeleteDoc(col, docID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	docs, ok := s.firestoreDocs[col]
	if !ok {
		return false
	}
	delete(docs, docID)
	if len(docs) == 0 {
		delete(s.firestoreDocs, col)
	}
	return true
}

func (s *Server) LogSeverityCounts() map[string]int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cp := make(map[string]int64, len(s.logSeverityCounts))
	for k, v := range s.logSeverityCounts {
		cp[k] = v
	}
	return cp
}

func (s *Server) BumpLogSeverity(sev string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logSeverityCounts[strings.ToUpper(sev)]++
}

func (s *Server) SetAlertMeta(name, metric string, threshold float64, durSec int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.alertMeta[name] = alertMetaInfo{
		MetricType:  metric,
		Threshold:   threshold,
		DurationSec: durSec,
	}
}

func (s *Server) GetAlertMeta(name string) (string, float64, int64) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m := s.alertMeta[name]
	return m.MetricType, m.Threshold, m.DurationSec
}

func (s *Server) ListMetrics() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var m []string
	for k := range s.metricSeries {
		m = append(m, k)
	}
	sort.Strings(m)
	return m
}

func (s *Server) QueryMetricSeries(metric string, points int) []float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	vals, ok := s.metricSeries[metric]
	if !ok {
		vals = []float64{0.25, 0.30, 0.35, 0.40}
	}
	if points <= 0 || points >= len(vals) {
		return append([]float64(nil), vals...)
	}
	return append([]float64(nil), vals[:points]...)
}

func (s *Server) SetResourceAgeHours(kind, name string, ageHours int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.createdAgeHrs[kind+":"+name] = ageHours
}

func (s *Server) GetResourceAgeHours(kind, name string) int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.createdAgeHrs[kind+":"+name]
}
