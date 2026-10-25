// SPDX-License-Identifier: MIT

package host

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"

	"cloud.google.com/go/auth/credentials"
	"cloud.google.com/go/compute/metadata"
	"github.com/lmctl-ai/lmgcp/internal/fakegcp"
	"github.com/lmctl-ai/lmgcp/internal/membudget"
	"github.com/lmctl-ai/lmgcp/internal/safety"
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
	"google.golang.org/api/option"
	"google.golang.org/api/pubsub/v1"
	"google.golang.org/api/run/v2"
	"google.golang.org/api/secretmanager/v1"
	"google.golang.org/api/sqladmin/v1"
	"google.golang.org/api/storage/v1"
)

// Host manages Google Cloud Go SDK clients, safety guardrails, run ledger, and fake/live transport.
type Host struct {
	mu          sync.Mutex
	projectID   string
	region      string
	zone        string
	liveMode    bool
	autoCleanup bool
	lastError   string
	ledger      *safety.RunLedger
	fakeSrv     *fakegcp.Server

	computeSvc    *compute.Service
	storageSvc    *storage.Service
	pubsubSvc     *pubsub.Service
	runSvc        *run.Service
	sqlSvc        *sqladmin.Service
	bqSvc         *bigquery.Service
	iamSvc        *iam.Service
	crmSvc        *cloudresourcemanager.Service
	secretSvc     *secretmanager.Service
	kmsSvc        *cloudkms.Service
	gkeSvc        *container.Service
	arSvc         *artifactregistry.Service
	dnsSvc        *dns.Service
	loggingSvc    *logging.Service
	monitoringSvc *monitoring.Service
}

// NewHost initializes the lmgcp Host.
// Unless LMGCP_LIVE=1 is set in the environment, it starts an in-process fake GCP server
// and binds all Google Cloud Go SDK clients to loopback HTTP with option.WithoutAuthentication().
func NewHost() (*Host, error) {
	proj := strings.TrimSpace(os.Getenv("LMGCP_PROJECT"))
	if proj == "" {
		proj = "lmgcp-sandbox-8f2a"
	}
	reg := strings.TrimSpace(os.Getenv("LMGCP_REGION"))
	if reg == "" {
		reg = "us-central1"
	}
	zn := strings.TrimSpace(os.Getenv("LMGCP_ZONE"))
	if zn == "" {
		zn = "us-central1-a"
	}
	live := os.Getenv("LMGCP_LIVE") == "1"

	h := &Host{
		projectID:   proj,
		region:      reg,
		zone:        zn,
		liveMode:    live,
		autoCleanup: live,
		ledger:      safety.NewRunLedger("run-20261024"),
	}

	ctx := context.Background()
	if !live {
		srv, err := fakegcp.NewServer()
		if err != nil {
			return nil, fmt.Errorf("failed to start in-process fake GCP server: %w", err)
		}
		h.fakeSrv = srv
		if err := h.initClients(ctx, func(svc string) []option.ClientOption {
			return []option.ClientOption{
				option.WithEndpoint(srv.ServiceEndpoint(svc)),
				option.WithoutAuthentication(),
			}
		}); err != nil {
			srv.Close()
			return nil, err
		}
	} else {
		_ = metadata.OnGCE
		creds, err := credentials.DetectDefault(&credentials.DetectOptions{
			Scopes: []string{"https://www.googleapis.com/auth/cloud-platform"},
		})
		if err != nil {
			return nil, fmt.Errorf("ADC detection failed (LMGCP_LIVE=1): %s", safety.RedactSecrets(err.Error()))
		}
		if err := h.initClients(ctx, func(_ string) []option.ClientOption {
			return []option.ClientOption{
				option.WithAuthCredentials(creds),
			}
		}); err != nil {
			return nil, err
		}
	}
	return h, nil
}

func (h *Host) initClients(ctx context.Context, optsFor func(svc string) []option.ClientOption) error {
	var err error
	if h.computeSvc, err = compute.NewService(ctx, optsFor("compute")...); err != nil {
		return err
	}
	if h.storageSvc, err = storage.NewService(ctx, optsFor("storage")...); err != nil {
		return err
	}
	if h.pubsubSvc, err = pubsub.NewService(ctx, optsFor("pubsub")...); err != nil {
		return err
	}
	if h.runSvc, err = run.NewService(ctx, optsFor("run")...); err != nil {
		return err
	}
	if h.sqlSvc, err = sqladmin.NewService(ctx, optsFor("sql")...); err != nil {
		return err
	}
	if h.bqSvc, err = bigquery.NewService(ctx, optsFor("bigquery")...); err != nil {
		return err
	}
	if h.iamSvc, err = iam.NewService(ctx, optsFor("iam")...); err != nil {
		return err
	}
	if h.crmSvc, err = cloudresourcemanager.NewService(ctx, optsFor("crm")...); err != nil {
		return err
	}
	if h.secretSvc, err = secretmanager.NewService(ctx, optsFor("secrets")...); err != nil {
		return err
	}
	if h.kmsSvc, err = cloudkms.NewService(ctx, optsFor("kms")...); err != nil {
		return err
	}
	if h.gkeSvc, err = container.NewService(ctx, optsFor("gke")...); err != nil {
		return err
	}
	if h.arSvc, err = artifactregistry.NewService(ctx, optsFor("ar")...); err != nil {
		return err
	}
	if h.dnsSvc, err = dns.NewService(ctx, optsFor("dns")...); err != nil {
		return err
	}
	if h.loggingSvc, err = logging.NewService(ctx, optsFor("logging")...); err != nil {
		return err
	}
	if h.monitoringSvc, err = monitoring.NewService(ctx, optsFor("monitoring")...); err != nil {
		return err
	}
	return nil
}

func (h *Host) Close() {
	if h.autoCleanup {
		_ = h.CleanupLedger()
	}
	if h.fakeSrv != nil {
		h.fakeSrv.Close()
	}
}

func (h *Host) fail(err error) {
	if err == nil {
		h.lastError = ""
		return
	}
	h.lastError = safety.RedactSecrets(err.Error())
}

func (h *Host) clearErr() {
	h.lastError = ""
}

// --- Core & Safety ---

func (h *Host) Project() string {
	return safety.MaskProjectID(h.projectID)
}

func (h *Host) MaskedProject() string {
	return safety.MaskProjectID(h.projectID)
}

func (h *Host) SetProject(p string) {
	if strings.TrimSpace(p) != "" {
		h.projectID = strings.TrimSpace(p)
	}
}

func (h *Host) Region() string { return h.region }

func (h *Host) SetRegion(r string) {
	if strings.TrimSpace(r) != "" {
		h.region = strings.TrimSpace(r)
	}
}

func (h *Host) Zone() string { return h.zone }

func (h *Host) SetZone(z string) {
	if strings.TrimSpace(z) != "" {
		h.zone = strings.TrimSpace(z)
	}
}

func (h *Host) IsLive() bool { return h.liveMode }

func (h *Host) AuthMode() string {
	if h.liveMode {
		return "adc-live"
	}
	return "fake-emulator"
}

func (h *Host) Endpoint() string {
	if h.fakeSrv != nil {
		return h.fakeSrv.URL()
	}
	return "https://googleapis.com"
}

func (h *Host) RunID() string { return h.ledger.RunID() }

func (h *Host) SetRunID(id string) { h.ledger.SetRunID(id) }

func (h *Host) LastError() string { return h.lastError }

func (h *Host) SetAutoCleanup(enabled bool) { h.autoCleanup = enabled }

func (h *Host) LedgerCount() int64 { return int64(h.ledger.Count()) }

func (h *Host) LedgerEntries() []string {
	snap := h.ledger.Snapshot()
	out := make([]string, 0, len(snap))
	for _, e := range snap {
		out = append(out, e.Key())
	}
	return out
}

func (h *Host) LedgerOwns(kind, name string) bool {
	return h.ledger.Owns(kind, name)
}

// CleanupLedger deletes all resources recorded in the current run's ledger in reverse order.
func (h *Host) CleanupLedger() int64 {
	snap := h.ledger.Snapshot()
	var deleted int64
	for i := len(snap) - 1; i >= 0; i-- {
		e := snap[i]
		var ok bool
		switch e.Kind {
		case "compute_instance":
			ok = h.ComputeDeleteInstance(e.Location, e.Name)
		case "firewall":
			ok = h.NetworkDeleteFirewall(e.Name)
		case "subnet":
			ok = h.NetworkDeleteSubnet(e.Location, e.Name)
		case "vpc":
			ok = h.NetworkDeleteVPC(e.Name)
		case "bucket":
			ok = h.StorageDeleteBucket(e.Name)
		case "pubsub_sub":
			ok = h.PubsubDeleteSubscription(e.Name)
		case "pubsub_topic":
			ok = h.PubsubDeleteTopic(e.Name)
		case "cloud_run":
			ok = h.RunDeleteService(e.Location, e.Name)
		case "sql_instance":
			ok = h.SQLDeleteInstance(e.Name)
		case "bq_table":
			parts := strings.SplitN(e.Name, "/", 2)
			if len(parts) == 2 {
				ok = h.BQDeleteTable(parts[0], parts[1])
			}
		case "bq_dataset":
			ok = h.BQDeleteDataset(e.Name)
		case "secret":
			ok = h.SecretsDelete(e.Name)
		case "artifact_repo":
			ok = h.ARDeleteRepository(e.Location, e.Name)
		case "dns_zone":
			ok = h.DNSDeleteZone(e.Name)
		case "log_sink":
			ok = h.LoggingDeleteSink(e.Name)
		case "alert_policy":
			ok = h.MonitoringDeleteAlertPolicy(e.Name)
		}
		if ok {
			deleted++
		}
	}
	return deleted
}

func (h *Host) SeedDemoAge(kind, name string, ageHours int64) bool {
	if err := safety.ValidateResourcePrefix(kind, name); err != nil {
		h.fail(err)
		return false
	}
	if h.fakeSrv != nil {
		h.fakeSrv.SetResourceAgeHours(kind, name, ageHours)
	}
	h.clearErr()
	return true
}

// SweepDryRun lists lmgcp_ / lmgcp- resources in the ledger whose age >= maxAgeHours.
func (h *Host) SweepDryRun(maxAgeHours int64) []string {
	snap := h.ledger.Snapshot()
	var candidates []string
	for _, e := range snap {
		if !safety.HasSafePrefix(e.Name) {
			continue
		}
		var age int64
		if h.fakeSrv != nil {
			age = h.fakeSrv.GetResourceAgeHours(e.Kind, e.Name)
		}
		if age >= maxAgeHours {
			candidates = append(candidates, fmt.Sprintf("%s:%s:%s (age=%dh)", e.Kind, e.Location, e.Name, age))
		}
	}
	return candidates
}

// SweepDelete deletes lmgcp_ / lmgcp- resources in the ledger whose age >= maxAgeHours.
func (h *Host) SweepDelete(maxAgeHours int64) int64 {
	snap := h.ledger.Snapshot()
	var deleted int64
	for i := len(snap) - 1; i >= 0; i-- {
		e := snap[i]
		if !safety.HasSafePrefix(e.Name) {
			continue
		}
		var age int64
		if h.fakeSrv != nil {
			age = h.fakeSrv.GetResourceAgeHours(e.Kind, e.Name)
		}
		if age < maxAgeHours {
			continue
		}
		var ok bool
		switch e.Kind {
		case "compute_instance":
			ok = h.ComputeDeleteInstance(e.Location, e.Name)
		case "firewall":
			ok = h.NetworkDeleteFirewall(e.Name)
		case "subnet":
			ok = h.NetworkDeleteSubnet(e.Location, e.Name)
		case "vpc":
			ok = h.NetworkDeleteVPC(e.Name)
		case "bucket":
			ok = h.StorageDeleteBucket(e.Name)
		case "pubsub_sub":
			ok = h.PubsubDeleteSubscription(e.Name)
		case "pubsub_topic":
			ok = h.PubsubDeleteTopic(e.Name)
		case "cloud_run":
			ok = h.RunDeleteService(e.Location, e.Name)
		case "sql_instance":
			ok = h.SQLDeleteInstance(e.Name)
		case "bq_dataset":
			ok = h.BQDeleteDataset(e.Name)
		case "secret":
			ok = h.SecretsDelete(e.Name)
		case "artifact_repo":
			ok = h.ARDeleteRepository(e.Location, e.Name)
		case "dns_zone":
			ok = h.DNSDeleteZone(e.Name)
		case "log_sink":
			ok = h.LoggingDeleteSink(e.Name)
		case "alert_policy":
			ok = h.MonitoringDeleteAlertPolicy(e.Name)
		}
		if ok {
			deleted++
		}
	}
	return deleted
}

// --- 1. Compute Engine ---

func machineSpecs(mt string) (int64, int64) {
	if idx := strings.LastIndex(mt, "/"); idx >= 0 {
		mt = mt[idx+1:]
	}
	switch mt {
	case "f1-micro":
		return 1, 614
	case "g1-small":
		return 1, 1740
	case "e2-micro":
		return 2, 1024
	case "e2-small":
		return 2, 2048
	case "e2-medium":
		return 2, 4096
	case "e2-standard-2":
		return 2, 8192
	case "e2-standard-4":
		return 4, 16384
	case "n2-standard-8":
		return 8, 32768
	case "n2-highmem-4":
		return 4, 32768
	default:
		return 2, 4096
	}
}

func toComputeInstance(inst *compute.Instance) *ComputeInstance {
	if inst == nil {
		return &ComputeInstance{LabelsVal: map[string]string{}}
	}
	var netName, intIP, extIP, sa string
	if len(inst.NetworkInterfaces) > 0 {
		netName = inst.NetworkInterfaces[0].Network
		intIP = inst.NetworkInterfaces[0].NetworkIP
		if len(inst.NetworkInterfaces[0].AccessConfigs) > 0 {
			extIP = inst.NetworkInterfaces[0].AccessConfigs[0].NatIP
		}
	}
	var diskGB int64
	for _, d := range inst.Disks {
		diskGB += d.DiskSizeGb
	}
	if len(inst.ServiceAccounts) > 0 {
		sa = inst.ServiceAccounts[0].Email
	}
	var vtpm bool
	if inst.ShieldedInstanceConfig != nil {
		vtpm = inst.ShieldedInstanceConfig.EnableVtpm
	}
	vcpus, memMB := machineSpecs(inst.MachineType)
	labels := make(map[string]string, len(inst.Labels))
	for k, v := range inst.Labels {
		labels[k] = safety.RedactSecrets(v)
	}
	return &ComputeInstance{
		NameVal:               inst.Name,
		ZoneVal:               inst.Zone,
		MachineTypeVal:        inst.MachineType,
		StatusVal:             inst.Status,
		NetworkVal:            netName,
		InternalIPVal:         intIP,
		ExternalIPVal:         extIP,
		CPUPlatformVal:        inst.CpuPlatform,
		DiskGBVal:             diskGB,
		VCPUsVal:              vcpus,
		MemoryMBVal:           memMB,
		LabelsVal:             labels,
		ServiceAccountVal:     sa,
		ShieldedVTPMVal:       vtpm,
		DeletionProtectionVal: inst.DeletionProtection,
	}
}

func (h *Host) ComputeListInstances(zone string) []string {
	if zone == "" {
		zone = "all"
	}
	res, err := h.computeSvc.Instances.List(h.projectID, zone).Do()
	if err != nil {
		h.fail(err)
		return nil
	}
	h.clearErr()
	var names []string
	for _, item := range res.Items {
		names = append(names, item.Name)
	}
	sort.Strings(names)
	return names
}

func (h *Host) ComputeGetInstance(zone, name string) *ComputeInstance {
	if zone == "" {
		zone = h.zone
	}
	inst, err := h.computeSvc.Instances.Get(h.projectID, zone, name).Do()
	if err != nil {
		h.fail(err)
		return &ComputeInstance{NameVal: name, ZoneVal: zone, StatusVal: "NOT_FOUND", LabelsVal: map[string]string{}}
	}
	h.clearErr()
	return toComputeInstance(inst)
}

func (h *Host) ComputeListDisks(zone string) []string {
	if zone == "" {
		zone = "all"
	}
	res, err := h.computeSvc.Disks.List(h.projectID, zone).Do()
	if err != nil {
		h.fail(err)
		return nil
	}
	h.clearErr()
	var names []string
	for _, d := range res.Items {
		names = append(names, d.Name)
	}
	sort.Strings(names)
	return names
}

func (h *Host) ComputeCreateInstance(zone, name, machineType, imageFamily string) *ComputeInstance {
	if zone == "" {
		zone = h.zone
	}
	if err := safety.ValidateResourcePrefix("compute_instance", name); err != nil {
		h.fail(err)
		return &ComputeInstance{NameVal: name, ZoneVal: zone, StatusVal: "ERROR", LabelsVal: map[string]string{}}
	}
	if err := safety.ValidateMachineType(machineType); err != nil {
		h.fail(err)
		return &ComputeInstance{NameVal: name, ZoneVal: zone, StatusVal: "ERROR", LabelsVal: map[string]string{}}
	}
	labels := h.ledger.EnrichLabels(map[string]string{"image_family": imageFamily}, fakegcp.DefaultEpoch)
	req := &compute.Instance{
		Name:        name,
		Zone:        zone,
		MachineType: machineType,
		Status:      "RUNNING",
		Labels:      labels,
		Disks: []*compute.AttachedDisk{
			{Boot: true, DiskSizeGb: 20, Source: name + "-boot"},
		},
		NetworkInterfaces: []*compute.NetworkInterface{
			{Network: "prod-vpc", NetworkIP: "10.10.9.50"},
		},
		ShieldedInstanceConfig: &compute.ShieldedInstanceConfig{EnableVtpm: true},
	}
	if _, err := h.computeSvc.Instances.Insert(h.projectID, zone, req).Do(); err != nil {
		h.fail(err)
		return &ComputeInstance{NameVal: name, ZoneVal: zone, StatusVal: "ERROR", LabelsVal: map[string]string{}}
	}
	h.ledger.Record("compute_instance", zone, name, fakegcp.DefaultEpoch)
	h.clearErr()
	return h.ComputeGetInstance(zone, name)
}

func (h *Host) ComputeSetInstanceLabels(zone, name string, labels map[string]string) bool {
	if zone == "" {
		zone = h.zone
	}
	if err := h.ledger.ValidateModifyOrDelete("compute_instance", name); err != nil {
		h.fail(err)
		return false
	}
	enriched := h.ledger.EnrichLabels(labels, fakegcp.DefaultEpoch)
	_, err := h.computeSvc.Instances.SetLabels(h.projectID, zone, name, &compute.InstancesSetLabelsRequest{
		Labels: enriched,
	}).Do()
	if err != nil {
		h.fail(err)
		return false
	}
	h.clearErr()
	return true
}

func (h *Host) ComputeStopInstance(zone, name string) bool {
	if zone == "" {
		zone = h.zone
	}
	if err := h.ledger.ValidateModifyOrDelete("compute_instance", name); err != nil {
		h.fail(err)
		return false
	}
	if _, err := h.computeSvc.Instances.Stop(h.projectID, zone, name).Do(); err != nil {
		h.fail(err)
		return false
	}
	h.clearErr()
	return true
}

func (h *Host) ComputeStartInstance(zone, name string) bool {
	if zone == "" {
		zone = h.zone
	}
	if err := h.ledger.ValidateModifyOrDelete("compute_instance", name); err != nil {
		h.fail(err)
		return false
	}
	if _, err := h.computeSvc.Instances.Start(h.projectID, zone, name).Do(); err != nil {
		h.fail(err)
		return false
	}
	h.clearErr()
	return true
}

func (h *Host) ComputeDeleteInstance(zone, name string) bool {
	if zone == "" {
		zone = h.zone
	}
	if err := h.ledger.ValidateModifyOrDelete("compute_instance", name); err != nil {
		h.fail(err)
		return false
	}
	if _, err := h.computeSvc.Instances.Delete(h.projectID, zone, name).Do(); err != nil {
		h.fail(err)
		return false
	}
	h.ledger.Remove("compute_instance", zone, name)
	h.clearErr()
	return true
}

// --- 2. VPC Networks, Subnets & Firewalls ---

func (h *Host) NetworkListVPCs() []string {
	res, err := h.computeSvc.Networks.List(h.projectID).Do()
	if err != nil {
		h.fail(err)
		return nil
	}
	h.clearErr()
	var names []string
	for _, n := range res.Items {
		names = append(names, n.Name)
	}
	sort.Strings(names)
	return names
}

func (h *Host) NetworkGetVPC(name string) *VpcNetwork {
	n, err := h.computeSvc.Networks.Get(h.projectID, name).Do()
	if err != nil {
		h.fail(err)
		return &VpcNetwork{NameVal: name}
	}
	var subCount int64
	if subs, err2 := h.computeSvc.Subnetworks.List(h.projectID, "all").Do(); err2 == nil {
		for _, sn := range subs.Items {
			if sn.Network == name {
				subCount++
			}
		}
	}
	mode := "REGIONAL"
	if n.RoutingConfig != nil && n.RoutingConfig.RoutingMode != "" {
		mode = n.RoutingConfig.RoutingMode
	}
	h.clearErr()
	return &VpcNetwork{
		NameVal:              n.Name,
		AutoCreateSubnetsVal: n.AutoCreateSubnetworks,
		RoutingModeVal:       mode,
		MTUVal:               n.Mtu,
		SubnetCountVal:       subCount,
	}
}

func (h *Host) NetworkCreateVPC(name string, autoSubnets bool) *VpcNetwork {
	if err := safety.ValidateResourcePrefix("vpc", name); err != nil {
		h.fail(err)
		return &VpcNetwork{NameVal: name}
	}
	req := &compute.Network{
		Name:                  name,
		AutoCreateSubnetworks: autoSubnets,
		Mtu:                   1460,
		RoutingConfig:         &compute.NetworkRoutingConfig{RoutingMode: "GLOBAL"},
	}
	if _, err := h.computeSvc.Networks.Insert(h.projectID, req).Do(); err != nil {
		h.fail(err)
		return &VpcNetwork{NameVal: name}
	}
	h.ledger.Record("vpc", "global", name, fakegcp.DefaultEpoch)
	h.clearErr()
	return h.NetworkGetVPC(name)
}

func (h *Host) NetworkDeleteVPC(name string) bool {
	if err := h.ledger.ValidateModifyOrDelete("vpc", name); err != nil {
		h.fail(err)
		return false
	}
	if _, err := h.computeSvc.Networks.Delete(h.projectID, name).Do(); err != nil {
		h.fail(err)
		return false
	}
	h.ledger.Remove("vpc", "global", name)
	h.clearErr()
	return true
}

func (h *Host) NetworkListSubnets(region string) []string {
	if region == "" {
		region = "all"
	}
	res, err := h.computeSvc.Subnetworks.List(h.projectID, region).Do()
	if err != nil {
		h.fail(err)
		return nil
	}
	h.clearErr()
	var names []string
	for _, sn := range res.Items {
		names = append(names, sn.Name)
	}
	sort.Strings(names)
	return names
}

func (h *Host) NetworkGetSubnet(region, name string) *Subnet {
	if region == "" {
		region = h.region
	}
	sn, err := h.computeSvc.Subnetworks.Get(h.projectID, region, name).Do()
	if err != nil {
		h.fail(err)
		return &Subnet{NameVal: name, RegionVal: region}
	}
	h.clearErr()
	return &Subnet{
		NameVal:                  sn.Name,
		RegionVal:                sn.Region,
		NetworkVal:               sn.Network,
		CIDRVal:                  sn.IpCidrRange,
		PrivateIPGoogleAccessVal: sn.PrivateIpGoogleAccess,
		FlowLogsVal:              sn.EnableFlowLogs,
	}
}

func (h *Host) NetworkCreateSubnet(region, name, vpcName, cidr string, privGoogle bool) *Subnet {
	if region == "" {
		region = h.region
	}
	if err := safety.ValidateResourcePrefix("subnet", name); err != nil {
		h.fail(err)
		return &Subnet{NameVal: name, RegionVal: region}
	}
	req := &compute.Subnetwork{
		Name:                  name,
		Region:                region,
		Network:               vpcName,
		IpCidrRange:           cidr,
		PrivateIpGoogleAccess: privGoogle,
		EnableFlowLogs:        true,
	}
	if _, err := h.computeSvc.Subnetworks.Insert(h.projectID, region, req).Do(); err != nil {
		h.fail(err)
		return &Subnet{NameVal: name, RegionVal: region}
	}
	h.ledger.Record("subnet", region, name, fakegcp.DefaultEpoch)
	h.clearErr()
	return h.NetworkGetSubnet(region, name)
}

func (h *Host) NetworkDeleteSubnet(region, name string) bool {
	if region == "" {
		region = h.region
	}
	if err := h.ledger.ValidateModifyOrDelete("subnet", name); err != nil {
		h.fail(err)
		return false
	}
	if _, err := h.computeSvc.Subnetworks.Delete(h.projectID, region, name).Do(); err != nil {
		h.fail(err)
		return false
	}
	h.ledger.Remove("subnet", region, name)
	h.clearErr()
	return true
}

func (h *Host) NetworkListFirewalls() []string {
	res, err := h.computeSvc.Firewalls.List(h.projectID).Do()
	if err != nil {
		h.fail(err)
		return nil
	}
	h.clearErr()
	var names []string
	for _, fw := range res.Items {
		names = append(names, fw.Name)
	}
	sort.Strings(names)
	return names
}

func toFirewallRule(fw *compute.Firewall) *FirewallRule {
	if fw == nil {
		return &FirewallRule{}
	}
	action := "ALLOW"
	proto := "tcp"
	var ports []string
	if len(fw.Allowed) > 0 {
		proto = fw.Allowed[0].IPProtocol
		ports = fw.Allowed[0].Ports
	} else if len(fw.Denied) > 0 {
		action = "DENY"
		proto = fw.Denied[0].IPProtocol
		ports = fw.Denied[0].Ports
	}
	var logEn bool
	if fw.LogConfig != nil {
		logEn = fw.LogConfig.Enable
	}
	return &FirewallRule{
		NameVal:       fw.Name,
		NetworkVal:    fw.Network,
		DirectionVal:  fw.Direction,
		PriorityVal:   fw.Priority,
		ActionVal:     action,
		ProtocolVal:   proto,
		PortsVal:      strings.Join(ports, ","),
		SourceCIDRVal: strings.Join(fw.SourceRanges, ","),
		TargetTagsVal: strings.Join(fw.TargetTags, ","),
		DisabledVal:   fw.Disabled,
		LogEnabledVal: logEn,
	}
}

func (h *Host) NetworkGetFirewall(name string) *FirewallRule {
	fw, err := h.computeSvc.Firewalls.Get(h.projectID, name).Do()
	if err != nil {
		h.fail(err)
		return &FirewallRule{NameVal: name}
	}
	h.clearErr()
	return toFirewallRule(fw)
}

func (h *Host) NetworkCreateFirewall(name, vpcName, direction string, priority int64, proto, ports, srcCIDR string) *FirewallRule {
	if err := safety.ValidateResourcePrefix("firewall", name); err != nil {
		h.fail(err)
		return &FirewallRule{NameVal: name}
	}
	req := &compute.Firewall{
		Name:         name,
		Network:      vpcName,
		Direction:    strings.ToUpper(direction),
		Priority:     priority,
		SourceRanges: []string{srcCIDR},
		Allowed: []*compute.FirewallAllowed{
			{IPProtocol: proto, Ports: strings.Split(ports, ",")},
		},
		LogConfig: &compute.FirewallLogConfig{Enable: true},
	}
	if _, err := h.computeSvc.Firewalls.Insert(h.projectID, req).Do(); err != nil {
		h.fail(err)
		return &FirewallRule{NameVal: name}
	}
	h.ledger.Record("firewall", "global", name, fakegcp.DefaultEpoch)
	h.clearErr()
	return h.NetworkGetFirewall(name)
}

func (h *Host) NetworkDeleteFirewall(name string) bool {
	if err := h.ledger.ValidateModifyOrDelete("firewall", name); err != nil {
		h.fail(err)
		return false
	}
	if _, err := h.computeSvc.Firewalls.Delete(h.projectID, name).Do(); err != nil {
		h.fail(err)
		return false
	}
	h.ledger.Remove("firewall", "global", name)
	h.clearErr()
	return true
}

// --- 3. Cloud Storage (GCS) ---

func (h *Host) toStorageBucket(b *storage.Bucket) *StorageBucket {
	if b == nil {
		return &StorageBucket{LabelsVal: map[string]string{}}
	}
	var ver, ubla bool
	pap := "inherited"
	if b.Versioning != nil {
		ver = b.Versioning.Enabled
	}
	if b.IamConfiguration != nil {
		if b.IamConfiguration.UniformBucketLevelAccess != nil {
			ubla = b.IamConfiguration.UniformBucketLevelAccess.Enabled
		}
		if b.IamConfiguration.PublicAccessPrevention != "" {
			pap = b.IamConfiguration.PublicAccessPrevention
		}
	}
	var delDays, nearDays int64
	if b.Lifecycle != nil {
		for _, r := range b.Lifecycle.Rule {
			if r.Action != nil && r.Condition != nil && r.Condition.Age != nil {
				if r.Action.Type == "Delete" {
					delDays = *r.Condition.Age
				} else if r.Action.Type == "SetStorageClass" {
					nearDays = *r.Condition.Age
				}
			}
		}
	}
	enc := "GOOGLE_MANAGED"
	if b.Encryption != nil && b.Encryption.DefaultKmsKeyName != "" {
		enc = "CMEK"
	}
	var objCount, totalBytes int64
	if h.fakeSrv != nil {
		objCount, totalBytes = h.fakeSrv.BucketStats(b.Name)
	}
	labels := make(map[string]string, len(b.Labels))
	for k, v := range b.Labels {
		labels[k] = safety.RedactSecrets(v)
	}
	return &StorageBucket{
		NameVal:                   b.Name,
		LocationVal:               b.Location,
		StorageClassVal:           b.StorageClass,
		VersioningVal:             ver,
		UniformAccessVal:          ubla,
		PublicAccessPreventionVal: pap,
		LifecycleDeleteDaysVal:    delDays,
		LifecycleNearlineDaysVal:  nearDays,
		EncryptionTypeVal:         enc,
		ObjectCountVal:            objCount,
		TotalBytesVal:             totalBytes,
		LabelsVal:                 labels,
	}
}

func (h *Host) StorageListBuckets() []string {
	res, err := h.storageSvc.Buckets.List(h.projectID).Do()
	if err != nil {
		h.fail(err)
		return nil
	}
	h.clearErr()
	var names []string
	for _, b := range res.Items {
		names = append(names, b.Name)
	}
	sort.Strings(names)
	return names
}

func (h *Host) StorageGetBucket(name string) *StorageBucket {
	b, err := h.storageSvc.Buckets.Get(name).Do()
	if err != nil {
		h.fail(err)
		return &StorageBucket{NameVal: name, LabelsVal: map[string]string{}}
	}
	h.clearErr()
	return h.toStorageBucket(b)
}

func (h *Host) StorageCreateBucket(name, location, storageClass string) *StorageBucket {
	if err := safety.ValidateResourcePrefix("bucket", name); err != nil {
		h.fail(err)
		return &StorageBucket{NameVal: name, LabelsVal: map[string]string{}}
	}
	labels := h.ledger.EnrichLabels(nil, fakegcp.DefaultEpoch)
	req := &storage.Bucket{
		Name:         name,
		Location:     strings.ToUpper(location),
		StorageClass: strings.ToUpper(storageClass),
		Labels:       labels,
		Versioning:   &storage.BucketVersioning{Enabled: true},
		IamConfiguration: &storage.BucketIamConfiguration{
			UniformBucketLevelAccess: &storage.BucketIamConfigurationUniformBucketLevelAccess{Enabled: true},
			PublicAccessPrevention:   "enforced",
		},
	}
	b, err := h.storageSvc.Buckets.Insert(h.projectID, req).Do()
	if err != nil {
		h.fail(err)
		return &StorageBucket{NameVal: name, LabelsVal: map[string]string{}}
	}
	h.ledger.Record("bucket", location, name, fakegcp.DefaultEpoch)
	h.clearErr()
	return h.toStorageBucket(b)
}

func (h *Host) StorageSetBucketLifecycle(name string, deleteAgeDays, nearlineAgeDays int64) bool {
	if err := h.ledger.ValidateModifyOrDelete("bucket", name); err != nil {
		h.fail(err)
		return false
	}
	var rules []*storage.BucketLifecycleRule
	if deleteAgeDays > 0 {
		d := deleteAgeDays
		rules = append(rules, &storage.BucketLifecycleRule{
			Action:    &storage.BucketLifecycleRuleAction{Type: "Delete"},
			Condition: &storage.BucketLifecycleRuleCondition{Age: &d},
		})
	}
	if nearlineAgeDays > 0 {
		n := nearlineAgeDays
		rules = append(rules, &storage.BucketLifecycleRule{
			Action:    &storage.BucketLifecycleRuleAction{Type: "SetStorageClass", StorageClass: "NEARLINE"},
			Condition: &storage.BucketLifecycleRuleCondition{Age: &n},
		})
	}
	_, err := h.storageSvc.Buckets.Patch(name, &storage.Bucket{
		Lifecycle: &storage.BucketLifecycle{Rule: rules},
	}).Do()
	if err != nil {
		h.fail(err)
		return false
	}
	h.clearErr()
	return true
}

func (h *Host) StorageSetBucketVersioning(name string, enabled bool) bool {
	if err := h.ledger.ValidateModifyOrDelete("bucket", name); err != nil {
		h.fail(err)
		return false
	}
	_, err := h.storageSvc.Buckets.Patch(name, &storage.Bucket{
		Versioning: &storage.BucketVersioning{Enabled: enabled},
	}).Do()
	if err != nil {
		h.fail(err)
		return false
	}
	h.clearErr()
	return true
}

func (h *Host) StorageSetUniformAccess(name string, enabled bool) bool {
	if err := h.ledger.ValidateModifyOrDelete("bucket", name); err != nil {
		h.fail(err)
		return false
	}
	_, err := h.storageSvc.Buckets.Patch(name, &storage.Bucket{
		IamConfiguration: &storage.BucketIamConfiguration{
			UniformBucketLevelAccess: &storage.BucketIamConfigurationUniformBucketLevelAccess{Enabled: enabled},
			PublicAccessPrevention:   "enforced",
		},
	}).Do()
	if err != nil {
		h.fail(err)
		return false
	}
	h.clearErr()
	return true
}

func (h *Host) StorageSetBucketLabels(name string, labels map[string]string) bool {
	if err := h.ledger.ValidateModifyOrDelete("bucket", name); err != nil {
		h.fail(err)
		return false
	}
	enriched := h.ledger.EnrichLabels(labels, fakegcp.DefaultEpoch)
	_, err := h.storageSvc.Buckets.Patch(name, &storage.Bucket{Labels: enriched}).Do()
	if err != nil {
		h.fail(err)
		return false
	}
	h.clearErr()
	return true
}

func (h *Host) StoragePutObject(bucket, objName, content, contentType string) bool {
	if err := h.ledger.ValidateModifyOrDelete("bucket", bucket); err != nil {
		h.fail(err)
		return false
	}
	if err := membudget.Charge(int64(len(content))); err != nil {
		h.fail(err)
		return false
	}
	if h.fakeSrv != nil {
		if err := h.fakeSrv.PutObject(bucket, objName, safety.RedactSecrets(content), contentType); err != nil {
			h.fail(err)
			return false
		}
	}
	h.clearErr()
	return true
}

func (h *Host) StorageListObjects(bucket, prefix string) []string {
	call := h.storageSvc.Objects.List(bucket)
	if prefix != "" {
		call = call.Prefix(prefix)
	}
	res, err := call.Do()
	if err != nil {
		h.fail(err)
		return nil
	}
	h.clearErr()
	var names []string
	for _, o := range res.Items {
		names = append(names, o.Name)
	}
	sort.Strings(names)
	return names
}

func (h *Host) StorageReadObject(bucket, objName string) string {
	if h.fakeSrv != nil {
		data, _, err := h.fakeSrv.ReadObject(bucket, objName)
		if err != nil {
			h.fail(err)
			return ""
		}
		h.clearErr()
		return safety.RedactSecrets(data)
	}
	return ""
}

func (h *Host) StorageObjectSize(bucket, objName string) int64 {
	if h.fakeSrv != nil {
		_, sz, err := h.fakeSrv.ReadObject(bucket, objName)
		if err != nil {
			h.fail(err)
			return 0
		}
		h.clearErr()
		return sz
	}
	return 0
}

func (h *Host) StorageDeleteObject(bucket, objName string) bool {
	if err := h.ledger.ValidateModifyOrDelete("bucket", bucket); err != nil {
		h.fail(err)
		return false
	}
	if err := h.storageSvc.Objects.Delete(bucket, objName).Do(); err != nil {
		h.fail(err)
		return false
	}
	h.clearErr()
	return true
}

func (h *Host) StorageDeleteBucket(name string) bool {
	if err := h.ledger.ValidateModifyOrDelete("bucket", name); err != nil {
		h.fail(err)
		return false
	}
	if err := h.storageSvc.Buckets.Delete(name).Do(); err != nil {
		h.fail(err)
		return false
	}
	h.ledger.Remove("bucket", "", name)
	h.clearErr()
	return true
}

// --- 4. Pub/Sub ---

func shortResourceName(full string) string {
	if idx := strings.LastIndex(full, "/"); idx >= 0 {
		return full[idx+1:]
	}
	return full
}

func parseSecDuration(s string) int64 {
	s = strings.TrimSuffix(strings.TrimSpace(s), "s")
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}

func (h *Host) toPubsubTopic(t *pubsub.Topic) *PubsubTopic {
	if t == nil {
		return &PubsubTopic{LabelsVal: map[string]string{}}
	}
	short := shortResourceName(t.Name)
	var pubCount int64
	if h.fakeSrv != nil {
		pubCount = h.fakeSrv.TopicPublishedCount(short)
	}
	labels := make(map[string]string, len(t.Labels))
	for k, v := range t.Labels {
		labels[k] = safety.RedactSecrets(v)
	}
	return &PubsubTopic{
		NameVal:           short,
		RetentionSecVal:   parseSecDuration(t.MessageRetentionDuration),
		PublishedCountVal: pubCount,
		LabelsVal:         labels,
	}
}

func (h *Host) PubsubListTopics() []string {
	res, err := h.pubsubSvc.Projects.Topics.List("projects/" + h.projectID).Do()
	if err != nil {
		h.fail(err)
		return nil
	}
	h.clearErr()
	var names []string
	for _, t := range res.Topics {
		names = append(names, shortResourceName(t.Name))
	}
	sort.Strings(names)
	return names
}

func (h *Host) PubsubGetTopic(name string) *PubsubTopic {
	t, err := h.pubsubSvc.Projects.Topics.Get(fmt.Sprintf("projects/%s/topics/%s", h.projectID, name)).Do()
	if err != nil {
		h.fail(err)
		return &PubsubTopic{NameVal: name, LabelsVal: map[string]string{}}
	}
	h.clearErr()
	return h.toPubsubTopic(t)
}

func (h *Host) PubsubCreateTopic(name string) *PubsubTopic {
	if err := safety.ValidateResourcePrefix("pubsub_topic", name); err != nil {
		h.fail(err)
		return &PubsubTopic{NameVal: name, LabelsVal: map[string]string{}}
	}
	labels := h.ledger.EnrichLabels(nil, fakegcp.DefaultEpoch)
	t, err := h.pubsubSvc.Projects.Topics.Create(fmt.Sprintf("projects/%s/topics/%s", h.projectID, name), &pubsub.Topic{
		Labels:                   labels,
		MessageRetentionDuration: "604800s",
	}).Do()
	if err != nil {
		h.fail(err)
		return &PubsubTopic{NameVal: name, LabelsVal: map[string]string{}}
	}
	h.ledger.Record("pubsub_topic", "global", name, fakegcp.DefaultEpoch)
	h.clearErr()
	return h.toPubsubTopic(t)
}

func (h *Host) PubsubSetTopicLabels(name string, labels map[string]string) bool {
	if err := h.ledger.ValidateModifyOrDelete("pubsub_topic", name); err != nil {
		h.fail(err)
		return false
	}
	enriched := h.ledger.EnrichLabels(labels, fakegcp.DefaultEpoch)
	_, err := h.pubsubSvc.Projects.Topics.Patch(fmt.Sprintf("projects/%s/topics/%s", h.projectID, name), &pubsub.UpdateTopicRequest{
		Topic:      &pubsub.Topic{Labels: enriched},
		UpdateMask: "labels",
	}).Do()
	if err != nil {
		h.fail(err)
		return false
	}
	h.clearErr()
	return true
}

func (h *Host) PubsubDeleteTopic(name string) bool {
	if err := h.ledger.ValidateModifyOrDelete("pubsub_topic", name); err != nil {
		h.fail(err)
		return false
	}
	if _, err := h.pubsubSvc.Projects.Topics.Delete(fmt.Sprintf("projects/%s/topics/%s", h.projectID, name)).Do(); err != nil {
		h.fail(err)
		return false
	}
	h.ledger.Remove("pubsub_topic", "global", name)
	h.clearErr()
	return true
}

func (h *Host) toPubsubSub(sb *pubsub.Subscription) *PubsubSubscription {
	if sb == nil {
		return &PubsubSubscription{}
	}
	short := shortResourceName(sb.Name)
	var dlq string
	var maxAtt int64
	if sb.DeadLetterPolicy != nil {
		dlq = shortResourceName(sb.DeadLetterPolicy.DeadLetterTopic)
		maxAtt = sb.DeadLetterPolicy.MaxDeliveryAttempts
	}
	var backlog int64
	if h.fakeSrv != nil {
		backlog = h.fakeSrv.SubscriptionBacklog(short)
	}
	return &PubsubSubscription{
		NameVal:                short,
		TopicVal:               shortResourceName(sb.Topic),
		AckDeadlineSecVal:      sb.AckDeadlineSeconds,
		DeadLetterTopicVal:     dlq,
		MaxDeliveryAttemptsVal: maxAtt,
		BacklogCountVal:        backlog,
		RetainAckedVal:         sb.RetainAckedMessages,
	}
}

func (h *Host) PubsubListSubscriptions() []string {
	res, err := h.pubsubSvc.Projects.Subscriptions.List("projects/" + h.projectID).Do()
	if err != nil {
		h.fail(err)
		return nil
	}
	h.clearErr()
	var names []string
	for _, sb := range res.Subscriptions {
		names = append(names, shortResourceName(sb.Name))
	}
	sort.Strings(names)
	return names
}

func (h *Host) PubsubGetSubscription(name string) *PubsubSubscription {
	sb, err := h.pubsubSvc.Projects.Subscriptions.Get(fmt.Sprintf("projects/%s/subscriptions/%s", h.projectID, name)).Do()
	if err != nil {
		h.fail(err)
		return &PubsubSubscription{NameVal: name}
	}
	h.clearErr()
	return h.toPubsubSub(sb)
}

func (h *Host) PubsubCreateSubscription(name, topicName string, ackSec int64, dlqTopic string) *PubsubSubscription {
	if err := safety.ValidateResourcePrefix("pubsub_sub", name); err != nil {
		h.fail(err)
		return &PubsubSubscription{NameVal: name}
	}
	req := &pubsub.Subscription{
		Topic:              fmt.Sprintf("projects/%s/topics/%s", h.projectID, topicName),
		AckDeadlineSeconds: ackSec,
		Labels:             h.ledger.EnrichLabels(nil, fakegcp.DefaultEpoch),
	}
	if dlqTopic != "" {
		req.DeadLetterPolicy = &pubsub.DeadLetterPolicy{
			DeadLetterTopic:     fmt.Sprintf("projects/%s/topics/%s", h.projectID, dlqTopic),
			MaxDeliveryAttempts: 5,
		}
	}
	sb, err := h.pubsubSvc.Projects.Subscriptions.Create(fmt.Sprintf("projects/%s/subscriptions/%s", h.projectID, name), req).Do()
	if err != nil {
		h.fail(err)
		return &PubsubSubscription{NameVal: name}
	}
	h.ledger.Record("pubsub_sub", "global", name, fakegcp.DefaultEpoch)
	h.clearErr()
	return h.toPubsubSub(sb)
}

func (h *Host) PubsubPublish(topicName, data string, attrs map[string]string) string {
	if err := h.ledger.ValidateModifyOrDelete("pubsub_topic", topicName); err != nil {
		h.fail(err)
		return ""
	}
	cleanData := safety.RedactSecrets(data)
	res, err := h.pubsubSvc.Projects.Topics.Publish(fmt.Sprintf("projects/%s/topics/%s", h.projectID, topicName), &pubsub.PublishRequest{
		Messages: []*pubsub.PubsubMessage{
			{
				Data:       base64.StdEncoding.EncodeToString([]byte(cleanData)),
				Attributes: attrs,
			},
		},
	}).Do()
	if err != nil || len(res.MessageIds) == 0 {
		h.fail(err)
		return ""
	}
	h.clearErr()
	return res.MessageIds[0]
}

func (h *Host) PubsubPull(subName string, maxMessages int64) []string {
	if err := h.ledger.ValidateModifyOrDelete("pubsub_sub", subName); err != nil {
		h.fail(err)
		return nil
	}
	res, err := h.pubsubSvc.Projects.Subscriptions.Pull(fmt.Sprintf("projects/%s/subscriptions/%s", h.projectID, subName), &pubsub.PullRequest{
		MaxMessages: maxMessages,
	}).Do()
	if err != nil {
		h.fail(err)
		return nil
	}
	var out []string
	for _, rm := range res.ReceivedMessages {
		if rm.Message != nil {
			dec, err := base64.StdEncoding.DecodeString(rm.Message.Data)
			if err == nil {
				out = append(out, safety.RedactSecrets(string(dec)))
			} else {
				out = append(out, safety.RedactSecrets(rm.Message.Data))
			}
		}
	}
	h.clearErr()
	return out
}

func (h *Host) PubsubDeleteSubscription(name string) bool {
	if err := h.ledger.ValidateModifyOrDelete("pubsub_sub", name); err != nil {
		h.fail(err)
		return false
	}
	if _, err := h.pubsubSvc.Projects.Subscriptions.Delete(fmt.Sprintf("projects/%s/subscriptions/%s", h.projectID, name)).Do(); err != nil {
		h.fail(err)
		return false
	}
	h.ledger.Remove("pubsub_sub", "global", name)
	h.clearErr()
	return true
}

// --- 5. Cloud Run v2 ---

func (h *Host) toCloudRunService(region string, svc *run.GoogleCloudRunV2Service) *CloudRunService {
	if svc == nil {
		return &CloudRunService{RegionVal: region, LabelsVal: map[string]string{}}
	}
	short := shortResourceName(svc.Name)
	var image string
	var minInst, maxInst, conc int64 = 0, 10, 80
	if svc.Template != nil {
		if svc.Template.MaxInstanceRequestConcurrency > 0 {
			conc = svc.Template.MaxInstanceRequestConcurrency
		}
		if svc.Template.Scaling != nil {
			minInst = svc.Template.Scaling.MinInstanceCount
			maxInst = svc.Template.Scaling.MaxInstanceCount
		}
		if len(svc.Template.Containers) > 0 {
			image = svc.Template.Containers[0].Image
		}
	}
	var cpuMilli, memMB, revCount, stablePct, canaryPct int64 = 1000, 512, 1, 100, 0
	if h.fakeSrv != nil {
		c, m, r, st, ca := h.fakeSrv.GetRunMeta(region, short)
		if c > 0 {
			cpuMilli, memMB, revCount, stablePct, canaryPct = c, m, r, st, ca
		}
	}
	labels := make(map[string]string, len(svc.Labels))
	for k, v := range svc.Labels {
		labels[k] = safety.RedactSecrets(v)
	}
	return &CloudRunService{
		NameVal:             short,
		RegionVal:           region,
		ImageVal:            image,
		URIVal:              svc.Uri,
		CPUMilliVal:         cpuMilli,
		MemoryMBVal:         memMB,
		MinInstancesVal:     minInst,
		MaxInstancesVal:     maxInst,
		ConcurrencyVal:      conc,
		IngressVal:          svc.Ingress,
		LatestRevisionVal:   svc.LatestReadyRevision,
		RevisionCountVal:    revCount,
		StableTrafficPctVal: stablePct,
		CanaryTrafficPctVal: canaryPct,
		LabelsVal:           labels,
	}
}

func (h *Host) RunListServices(region string) []string {
	if region == "" {
		region = "-"
	}
	parent := fmt.Sprintf("projects/%s/locations/%s", h.projectID, region)
	res, err := h.runSvc.Projects.Locations.Services.List(parent).Do()
	if err != nil {
		h.fail(err)
		return nil
	}
	h.clearErr()
	var names []string
	for _, s := range res.Services {
		names = append(names, shortResourceName(s.Name))
	}
	sort.Strings(names)
	return names
}

func (h *Host) RunGetService(region, name string) *CloudRunService {
	if region == "" {
		region = h.region
	}
	full := fmt.Sprintf("projects/%s/locations/%s/services/%s", h.projectID, region, name)
	svc, err := h.runSvc.Projects.Locations.Services.Get(full).Do()
	if err != nil {
		h.fail(err)
		return &CloudRunService{NameVal: name, RegionVal: region, LabelsVal: map[string]string{}}
	}
	h.clearErr()
	return h.toCloudRunService(region, svc)
}

func (h *Host) RunDeployService(region, name, image string, cpuMilli, memMB, maxInstances int64) *CloudRunService {
	if region == "" {
		region = h.region
	}
	if err := safety.ValidateResourcePrefix("cloud_run", name); err != nil {
		h.fail(err)
		return &CloudRunService{NameVal: name, RegionVal: region, LabelsVal: map[string]string{}}
	}
	if cpuMilli > 4000 || memMB > 4096 || maxInstances > 25 {
		h.fail(fmt.Errorf("safety policy violation: Cloud Run limits exceed sandbox cap (cpu<=4000m, mem<=4096Mi, max_instances<=25)"))
		return &CloudRunService{NameVal: name, RegionVal: region, LabelsVal: map[string]string{}}
	}
	parent := fmt.Sprintf("projects/%s/locations/%s", h.projectID, region)
	labels := h.ledger.EnrichLabels(nil, fakegcp.DefaultEpoch)
	req := &run.GoogleCloudRunV2Service{
		Name:    parent + "/services/" + name,
		Ingress: "INGRESS_TRAFFIC_ALL",
		Labels:  labels,
		Template: &run.GoogleCloudRunV2RevisionTemplate{
			MaxInstanceRequestConcurrency: 80,
			Scaling:                       &run.GoogleCloudRunV2RevisionScaling{MinInstanceCount: 0, MaxInstanceCount: maxInstances},
			Containers:                    []*run.GoogleCloudRunV2Container{{Image: image}},
		},
	}
	if _, err := h.runSvc.Projects.Locations.Services.Create(parent, req).ServiceId(name).Do(); err != nil {
		h.fail(err)
		return &CloudRunService{NameVal: name, RegionVal: region, LabelsVal: map[string]string{}}
	}
	if h.fakeSrv != nil {
		h.fakeSrv.SetRunMeta(region, name, cpuMilli, memMB, 1, 100, 0)
	}
	h.ledger.Record("cloud_run", region, name, fakegcp.DefaultEpoch)
	h.clearErr()
	return h.RunGetService(region, name)
}

func (h *Host) RunSetTrafficSplit(region, name string, stablePct, canaryPct int64) bool {
	if region == "" {
		region = h.region
	}
	if err := h.ledger.ValidateModifyOrDelete("cloud_run", name); err != nil {
		h.fail(err)
		return false
	}
	if stablePct+canaryPct != 100 || stablePct < 0 || canaryPct < 0 {
		h.fail(fmt.Errorf("traffic split percentages must sum to 100 (got %d + %d)", stablePct, canaryPct))
		return false
	}
	full := fmt.Sprintf("projects/%s/locations/%s/services/%s", h.projectID, region, name)
	_, err := h.runSvc.Projects.Locations.Services.Patch(full, &run.GoogleCloudRunV2Service{
		Traffic: []*run.GoogleCloudRunV2TrafficTarget{
			{Type: "TRAFFIC_TARGET_ALLOCATION_TYPE_REVISION", Percent: stablePct},
			{Type: "TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST", Percent: canaryPct},
		},
	}).Do()
	if err != nil {
		h.fail(err)
		return false
	}
	if h.fakeSrv != nil {
		c, m, r, _, _ := h.fakeSrv.GetRunMeta(region, name)
		h.fakeSrv.SetRunMeta(region, name, c, m, r+1, stablePct, canaryPct)
	}
	h.clearErr()
	return true
}

func (h *Host) RunSetLabels(region, name string, labels map[string]string) bool {
	if region == "" {
		region = h.region
	}
	if err := h.ledger.ValidateModifyOrDelete("cloud_run", name); err != nil {
		h.fail(err)
		return false
	}
	full := fmt.Sprintf("projects/%s/locations/%s/services/%s", h.projectID, region, name)
	_, err := h.runSvc.Projects.Locations.Services.Patch(full, &run.GoogleCloudRunV2Service{
		Labels: h.ledger.EnrichLabels(labels, fakegcp.DefaultEpoch),
	}).Do()
	if err != nil {
		h.fail(err)
		return false
	}
	h.clearErr()
	return true
}

func (h *Host) RunDeleteService(region, name string) bool {
	if region == "" {
		region = h.region
	}
	if err := h.ledger.ValidateModifyOrDelete("cloud_run", name); err != nil {
		h.fail(err)
		return false
	}
	full := fmt.Sprintf("projects/%s/locations/%s/services/%s", h.projectID, region, name)
	if _, err := h.runSvc.Projects.Locations.Services.Delete(full).Do(); err != nil {
		h.fail(err)
		return false
	}
	h.ledger.Remove("cloud_run", region, name)
	h.clearErr()
	return true
}

// --- 6. Databases & Analytics (Cloud SQL, Firestore, BigQuery) ---

func toSQLInstance(inst *sqladmin.DatabaseInstance) *SqlInstance {
	if inst == nil {
		return &SqlInstance{LabelsVal: map[string]string{}}
	}
	var tier, avail string
	var backup, pitr, ssl, pubIP bool
	var sizeGB int64
	labels := map[string]string{}
	if inst.Settings != nil {
		tier = inst.Settings.Tier
		avail = inst.Settings.AvailabilityType
		sizeGB = inst.Settings.DataDiskSizeGb
		if inst.Settings.BackupConfiguration != nil {
			backup = inst.Settings.BackupConfiguration.Enabled
			pitr = inst.Settings.BackupConfiguration.PointInTimeRecoveryEnabled
		}
		if inst.Settings.IpConfiguration != nil {
			ssl = inst.Settings.IpConfiguration.RequireSsl
			pubIP = inst.Settings.IpConfiguration.Ipv4Enabled
		}
		for k, v := range inst.Settings.UserLabels {
			labels[k] = safety.RedactSecrets(v)
		}
	}
	return &SqlInstance{
		NameVal:            inst.Name,
		DBVersionVal:       inst.DatabaseVersion,
		TierVal:            tier,
		RegionVal:          inst.Region,
		StateVal:           inst.State,
		HAEnabledVal:       avail == "REGIONAL",
		BackupEnabledVal:   backup,
		PITREnabledVal:     pitr,
		RequireSSLVal:      ssl,
		PublicIPEnabledVal: pubIP,
		StorageGBVal:       sizeGB,
		LabelsVal:          labels,
	}
}

func (h *Host) SQLListInstances() []string {
	res, err := h.sqlSvc.Instances.List(h.projectID).Do()
	if err != nil {
		h.fail(err)
		return nil
	}
	h.clearErr()
	var names []string
	for _, i := range res.Items {
		names = append(names, i.Name)
	}
	sort.Strings(names)
	return names
}

func (h *Host) SQLGetInstance(name string) *SqlInstance {
	inst, err := h.sqlSvc.Instances.Get(h.projectID, name).Do()
	if err != nil {
		h.fail(err)
		return &SqlInstance{NameVal: name, LabelsVal: map[string]string{}}
	}
	h.clearErr()
	return toSQLInstance(inst)
}

func (h *Host) SQLCreateInstance(name, dbVer, tier, region string, ha bool) *SqlInstance {
	if region == "" {
		region = h.region
	}
	if err := safety.ValidateResourcePrefix("sql_instance", name); err != nil {
		h.fail(err)
		return &SqlInstance{NameVal: name, LabelsVal: map[string]string{}}
	}
	if err := safety.ValidateSqlTier(tier); err != nil {
		h.fail(err)
		return &SqlInstance{NameVal: name, LabelsVal: map[string]string{}}
	}
	avail := "ZONAL"
	if ha {
		avail = "REGIONAL"
	}
	req := &sqladmin.DatabaseInstance{
		Name:            name,
		DatabaseVersion: dbVer,
		Region:          region,
		State:           "RUNNABLE",
		Settings: &sqladmin.Settings{
			Tier:             tier,
			AvailabilityType: avail,
			DataDiskSizeGb:   20,
			UserLabels:       h.ledger.EnrichLabels(nil, fakegcp.DefaultEpoch),
			BackupConfiguration: &sqladmin.BackupConfiguration{
				Enabled:                    true,
				PointInTimeRecoveryEnabled: ha,
			},
			IpConfiguration: &sqladmin.IpConfiguration{
				RequireSsl:  true,
				Ipv4Enabled: false,
			},
		},
	}
	if _, err := h.sqlSvc.Instances.Insert(h.projectID, req).Do(); err != nil {
		h.fail(err)
		return &SqlInstance{NameVal: name, LabelsVal: map[string]string{}}
	}
	h.ledger.Record("sql_instance", region, name, fakegcp.DefaultEpoch)
	h.clearErr()
	return h.SQLGetInstance(name)
}

func (h *Host) SQLDeleteInstance(name string) bool {
	if err := h.ledger.ValidateModifyOrDelete("sql_instance", name); err != nil {
		h.fail(err)
		return false
	}
	if _, err := h.sqlSvc.Instances.Delete(h.projectID, name).Do(); err != nil {
		h.fail(err)
		return false
	}
	h.ledger.Remove("sql_instance", "", name)
	h.clearErr()
	return true
}

func (h *Host) FirestoreListCollections() []string {
	if h.fakeSrv != nil {
		h.clearErr()
		return h.fakeSrv.FirestoreListCollections()
	}
	return nil
}

func (h *Host) FirestorePutDoc(col, docID string, fields map[string]string) bool {
	if err := safety.ValidateResourcePrefix("firestore_collection", col); err != nil {
		h.fail(err)
		return false
	}
	if h.fakeSrv != nil {
		h.fakeSrv.FirestorePutDoc(col, docID, fields)
	}
	h.ledger.Record("firestore_doc", col, docID, fakegcp.DefaultEpoch)
	h.clearErr()
	return true
}

func (h *Host) FirestoreGetDoc(col, docID string) map[string]string {
	if h.fakeSrv != nil {
		if m, ok := h.fakeSrv.FirestoreGetDoc(col, docID); ok {
			h.clearErr()
			return m
		}
	}
	h.fail(fmt.Errorf("firestore doc %s/%s not found", col, docID))
	return map[string]string{}
}

func (h *Host) FirestoreListDocs(col string) []string {
	if h.fakeSrv != nil {
		h.clearErr()
		return h.fakeSrv.FirestoreListDocs(col)
	}
	return nil
}

func (h *Host) FirestoreDeleteDoc(col, docID string) bool {
	if err := safety.ValidateResourcePrefix("firestore_collection", col); err != nil {
		h.fail(err)
		return false
	}
	if h.fakeSrv != nil {
		h.fakeSrv.FirestoreDeleteDoc(col, docID)
	}
	h.ledger.Remove("firestore_doc", col, docID)
	h.clearErr()
	return true
}

func (h *Host) BQListDatasets() []string {
	res, err := h.bqSvc.Datasets.List(h.projectID).Do()
	if err != nil {
		h.fail(err)
		return nil
	}
	h.clearErr()
	var ids []string
	for _, ds := range res.Datasets {
		if ds.DatasetReference != nil {
			ids = append(ids, ds.DatasetReference.DatasetId)
		}
	}
	sort.Strings(ids)
	return ids
}

func (h *Host) BQGetDataset(dsID string) *BqDataset {
	ds, err := h.bqSvc.Datasets.Get(h.projectID, dsID).Do()
	if err != nil {
		h.fail(err)
		return &BqDataset{DatasetIDVal: dsID, LabelsVal: map[string]string{}}
	}
	labels := make(map[string]string, len(ds.Labels))
	for k, v := range ds.Labels {
		labels[k] = safety.RedactSecrets(v)
	}
	h.clearErr()
	return &BqDataset{
		DatasetIDVal:             dsID,
		LocationVal:              ds.Location,
		DefaultExpirationDaysVal: ds.DefaultTableExpirationMs / (86400 * 1000),
		LabelsVal:                labels,
	}
}

func (h *Host) BQCreateDataset(dsID, location string) *BqDataset {
	if err := safety.ValidateResourcePrefix("bq_dataset", dsID); err != nil {
		h.fail(err)
		return &BqDataset{DatasetIDVal: dsID, LabelsVal: map[string]string{}}
	}
	req := &bigquery.Dataset{
		DatasetReference:         &bigquery.DatasetReference{DatasetId: dsID, ProjectId: h.projectID},
		Location:                 location,
		DefaultTableExpirationMs: 30 * 86400 * 1000,
		Labels:                   h.ledger.EnrichLabels(nil, fakegcp.DefaultEpoch),
	}
	if _, err := h.bqSvc.Datasets.Insert(h.projectID, req).Do(); err != nil {
		h.fail(err)
		return &BqDataset{DatasetIDVal: dsID, LabelsVal: map[string]string{}}
	}
	h.ledger.Record("bq_dataset", location, dsID, fakegcp.DefaultEpoch)
	h.clearErr()
	return h.BQGetDataset(dsID)
}

func (h *Host) BQCreateTable(dsID, tblID, schemaSpec, partField string) *BqTable {
	if err := safety.ValidateResourcePrefix("bq_dataset", dsID); err != nil {
		h.fail(err)
		return &BqTable{DatasetIDVal: dsID, TableIDVal: tblID}
	}
	req := &bigquery.Table{
		TableReference: &bigquery.TableReference{DatasetId: dsID, TableId: tblID, ProjectId: h.projectID},
		Description:    schemaSpec,
	}
	if partField != "" {
		req.TimePartitioning = &bigquery.TimePartitioning{Field: partField, Type: "DAY"}
	}
	if _, err := h.bqSvc.Tables.Insert(h.projectID, dsID, req).Do(); err != nil {
		h.fail(err)
		return &BqTable{DatasetIDVal: dsID, TableIDVal: tblID}
	}
	h.ledger.Record("bq_table", "US", dsID+"/"+tblID, fakegcp.DefaultEpoch)
	h.clearErr()
	return h.BQGetTable(dsID, tblID)
}

func (h *Host) BQGetTable(dsID, tblID string) *BqTable {
	tbl, err := h.bqSvc.Tables.Get(h.projectID, dsID, tblID).Do()
	if err != nil {
		h.fail(err)
		return &BqTable{DatasetIDVal: dsID, TableIDVal: tblID}
	}
	var pf string
	if tbl.TimePartitioning != nil {
		pf = tbl.TimePartitioning.Field
	}
	h.clearErr()
	return &BqTable{
		DatasetIDVal:      dsID,
		TableIDVal:        tblID,
		SchemaSpecVal:     tbl.Description,
		PartitionFieldVal: pf,
		RowCountVal:       int64(tbl.NumRows),
		BytesVal:          tbl.NumBytes,
	}
}

func (h *Host) BQDeleteTable(dsID, tblID string) bool {
	if err := h.ledger.ValidateModifyOrDelete("bq_table", dsID+"/"+tblID); err != nil {
		h.fail(err)
		return false
	}
	if err := h.bqSvc.Tables.Delete(h.projectID, dsID, tblID).Do(); err != nil {
		h.fail(err)
		return false
	}
	h.ledger.Remove("bq_table", "", dsID+"/"+tblID)
	h.clearErr()
	return true
}

func (h *Host) BQDeleteDataset(dsID string) bool {
	if err := h.ledger.ValidateModifyOrDelete("bq_dataset", dsID); err != nil {
		h.fail(err)
		return false
	}
	if err := h.bqSvc.Datasets.Delete(h.projectID, dsID).Do(); err != nil {
		h.fail(err)
		return false
	}
	h.ledger.Remove("bq_dataset", "", dsID)
	h.clearErr()
	return true
}

// --- 7. Identity & Access Management (IAM) — Read-Only Hygiene & Audit ---

func (h *Host) IAMListServiceAccounts() []string {
	res, err := h.iamSvc.Projects.ServiceAccounts.List("projects/" + h.projectID).Do()
	if err != nil {
		h.fail(err)
		return nil
	}
	h.clearErr()
	var emails []string
	for _, sa := range res.Accounts {
		emails = append(emails, sa.Email)
	}
	sort.Strings(emails)
	return emails
}

func (h *Host) IAMGetServiceAccount(id string) *ServiceAccount {
	sa, err := h.iamSvc.Projects.ServiceAccounts.Get(fmt.Sprintf("projects/%s/serviceAccounts/%s", h.projectID, id)).Do()
	if err != nil {
		h.fail(err)
		return &ServiceAccount{NameVal: id, EmailVal: id}
	}
	short := id
	var keys, keyAge int64
	var isDef bool
	var roles []string
	if h.fakeSrv != nil {
		short, keys, keyAge, isDef, roles = h.fakeSrv.GetSAMeta(sa.Email)
		if short == "" {
			short = sa.Email
		}
	}
	h.clearErr()
	return &ServiceAccount{
		NameVal:            short,
		EmailVal:           sa.Email,
		DisplayNameVal:     sa.DisplayName,
		DisabledVal:        sa.Disabled,
		UserManagedKeysVal: keys,
		KeyMaxAgeDaysVal:   keyAge,
		IsDefaultVal:       isDef,
		RolesVal:           roles,
	}
}

func (h *Host) IAMListBindings() []string {
	pol, err := h.crmSvc.Projects.GetIamPolicy(h.projectID, &cloudresourcemanager.GetIamPolicyRequest{}).Do()
	if err != nil {
		h.fail(err)
		return nil
	}
	h.clearErr()
	var out []string
	for _, b := range pol.Bindings {
		for _, m := range b.Members {
			out = append(out, b.Role+"|"+m)
		}
	}
	sort.Strings(out)
	return out
}

// --- 8. Secret Manager Metadata & Cloud KMS Metadata ---

func (h *Host) SecretsList() []string {
	res, err := h.secretSvc.Projects.Secrets.List("projects/" + h.projectID).Do()
	if err != nil {
		h.fail(err)
		return nil
	}
	h.clearErr()
	var names []string
	for _, s := range res.Secrets {
		names = append(names, shortResourceName(s.Name))
	}
	sort.Strings(names)
	return names
}

func (h *Host) SecretsGetMetadata(name string) *SecretMetadata {
	sec, err := h.secretSvc.Projects.Secrets.Get(fmt.Sprintf("projects/%s/secrets/%s", h.projectID, name)).Do()
	if err != nil {
		h.fail(err)
		return &SecretMetadata{NameVal: name, LabelsVal: map[string]string{}}
	}
	repl := "automatic"
	var rotDays, verCount, enabledVers int64 = 0, 1, 1
	var cmek, hasExp bool
	if h.fakeSrv != nil {
		r, rd, vc, ev, cm, ex := h.fakeSrv.GetSecretMeta(name)
		if r != "" {
			repl, rotDays, verCount, enabledVers, cmek, hasExp = r, rd, vc, ev, cm, ex
		}
	}
	labels := make(map[string]string, len(sec.Labels))
	for k, v := range sec.Labels {
		labels[k] = safety.RedactSecrets(v)
	}
	h.clearErr()
	return &SecretMetadata{
		NameVal:              name,
		ReplicationPolicyVal: repl,
		VersionCountVal:      verCount,
		EnabledVersionsVal:   enabledVers,
		RotationDaysVal:      rotDays,
		CMEKEnabledVal:       cmek,
		HasExpirationVal:     hasExp,
		LabelsVal:            labels,
	}
}

func (h *Host) SecretsCreateMetadata(name, replPolicy string, rotDays int64, labels map[string]string) *SecretMetadata {
	if err := safety.ValidateResourcePrefix("secret", name); err != nil {
		h.fail(err)
		return &SecretMetadata{NameVal: name, LabelsVal: map[string]string{}}
	}
	enriched := h.ledger.EnrichLabels(labels, fakegcp.DefaultEpoch)
	req := &secretmanager.Secret{
		Labels:      enriched,
		Replication: &secretmanager.Replication{Automatic: &secretmanager.Automatic{}},
	}
	if _, err := h.secretSvc.Projects.Secrets.Create("projects/"+h.projectID, req).SecretId(name).Do(); err != nil {
		h.fail(err)
		return &SecretMetadata{NameVal: name, LabelsVal: map[string]string{}}
	}
	if h.fakeSrv != nil {
		h.fakeSrv.SetSecretMeta(name, replPolicy, rotDays, 1, 1, true, false)
	}
	h.ledger.Record("secret", "global", name, fakegcp.DefaultEpoch)
	h.clearErr()
	return h.SecretsGetMetadata(name)
}

func (h *Host) SecretsDelete(name string) bool {
	if err := h.ledger.ValidateModifyOrDelete("secret", name); err != nil {
		h.fail(err)
		return false
	}
	if _, err := h.secretSvc.Projects.Secrets.Delete(fmt.Sprintf("projects/%s/secrets/%s", h.projectID, name)).Do(); err != nil {
		h.fail(err)
		return false
	}
	h.ledger.Remove("secret", "global", name)
	h.clearErr()
	return true
}

func (h *Host) KMSListKeyRings(location string) []string {
	if location == "" {
		location = h.region
	}
	parent := fmt.Sprintf("projects/%s/locations/%s", h.projectID, location)
	res, err := h.kmsSvc.Projects.Locations.KeyRings.List(parent).Do()
	if err != nil {
		h.fail(err)
		return nil
	}
	h.clearErr()
	var rings []string
	for _, kr := range res.KeyRings {
		rings = append(rings, shortResourceName(kr.Name))
	}
	sort.Strings(rings)
	return rings
}

func (h *Host) KMSListKeys(location, keyring string) []string {
	if location == "" {
		location = h.region
	}
	parent := fmt.Sprintf("projects/%s/locations/%s/keyRings/%s", h.projectID, location, keyring)
	res, err := h.kmsSvc.Projects.Locations.KeyRings.CryptoKeys.List(parent).Do()
	if err != nil {
		h.fail(err)
		return nil
	}
	h.clearErr()
	var keys []string
	for _, ck := range res.CryptoKeys {
		keys = append(keys, shortResourceName(ck.Name))
	}
	sort.Strings(keys)
	return keys
}

func (h *Host) KMSGetKey(location, keyring, keyName string) *KmsKey {
	if location == "" {
		location = h.region
	}
	full := fmt.Sprintf("projects/%s/locations/%s/keyRings/%s/cryptoKeys/%s", h.projectID, location, keyring, keyName)
	ck, err := h.kmsSvc.Projects.Locations.KeyRings.CryptoKeys.Get(full).Do()
	if err != nil {
		h.fail(err)
		return &KmsKey{NameVal: keyName, KeyRingVal: keyring, LocationVal: location}
	}
	state := "ENABLED"
	prot := "HSM"
	if ck.Primary != nil {
		state = ck.Primary.State
		prot = ck.Primary.ProtectionLevel
	}
	var rotDays int64
	if h.fakeSrv != nil {
		rotDays = h.fakeSrv.GetKMSRotDays(location, keyring, keyName)
	}
	h.clearErr()
	return &KmsKey{
		NameVal:            keyName,
		KeyRingVal:         keyring,
		LocationVal:        location,
		PurposeVal:         ck.Purpose,
		RotationDaysVal:    rotDays,
		PrimaryStateVal:    state,
		ProtectionLevelVal: prot,
	}
}

func (h *Host) KMSCreateKey(location, keyring, keyName, purpose string, rotDays int64) *KmsKey {
	if location == "" {
		location = h.region
	}
	if err := safety.ValidateResourcePrefix("kms_key", keyName); err != nil {
		h.fail(err)
		return &KmsKey{NameVal: keyName, KeyRingVal: keyring, LocationVal: location}
	}
	parent := fmt.Sprintf("projects/%s/locations/%s/keyRings/%s", h.projectID, location, keyring)
	req := &cloudkms.CryptoKey{
		Purpose:        purpose,
		RotationPeriod: fmt.Sprintf("%ds", rotDays*86400),
		Primary:        &cloudkms.CryptoKeyVersion{State: "ENABLED", ProtectionLevel: "HSM"},
	}
	if _, err := h.kmsSvc.Projects.Locations.KeyRings.CryptoKeys.Create(parent, req).CryptoKeyId(keyName).Do(); err != nil {
		h.fail(err)
		return &KmsKey{NameVal: keyName, KeyRingVal: keyring, LocationVal: location}
	}
	if h.fakeSrv != nil {
		h.fakeSrv.SetKMSRotDays(location, keyring, keyName, rotDays)
	}
	h.ledger.Record("kms_key", location+"/"+keyring, keyName, fakegcp.DefaultEpoch)
	h.clearErr()
	return h.KMSGetKey(location, keyring, keyName)
}

// --- 9. Container Services (GKE & Artifact Registry) ---

func (h *Host) GKEListClusters(location string) []string {
	if location == "" {
		location = "-"
	}
	parent := fmt.Sprintf("projects/%s/locations/%s", h.projectID, location)
	res, err := h.gkeSvc.Projects.Locations.Clusters.List(parent).Do()
	if err != nil {
		h.fail(err)
		return nil
	}
	h.clearErr()
	var names []string
	for _, c := range res.Clusters {
		names = append(names, c.Name)
	}
	sort.Strings(names)
	return names
}

func (h *Host) GKEGetCluster(location, name string) *GkeCluster {
	if location == "" {
		location = h.region
	}
	full := fmt.Sprintf("projects/%s/locations/%s/clusters/%s", h.projectID, location, name)
	c, err := h.gkeSvc.Projects.Locations.Clusters.Get(full).Do()
	if err != nil {
		h.fail(err)
		return &GkeCluster{NameVal: name, LocationVal: location, LabelsVal: map[string]string{}}
	}
	channel := "REGULAR"
	if c.ReleaseChannel != nil && c.ReleaseChannel.Channel != "" {
		channel = c.ReleaseChannel.Channel
	}
	var ap, priv, wi, np, ba bool
	var pools, nodes int64
	if h.fakeSrv != nil {
		ap, priv, wi, np, ba, pools, nodes = h.fakeSrv.GetGKEMeta(location, name)
	}
	labels := make(map[string]string, len(c.ResourceLabels))
	for k, v := range c.ResourceLabels {
		labels[k] = safety.RedactSecrets(v)
	}
	h.clearErr()
	return &GkeCluster{
		NameVal:             c.Name,
		LocationVal:         c.Location,
		StatusVal:           c.Status,
		K8sVersionVal:       c.CurrentMasterVersion,
		ReleaseChannelVal:   channel,
		AutopilotVal:        ap,
		PrivateNodesVal:     priv,
		WorkloadIdentityVal: wi,
		NetworkPolicyVal:    np,
		BinaryAuthVal:       ba,
		NodePoolCountVal:    pools,
		TotalNodesVal:       nodes,
		LabelsVal:           labels,
	}
}

func (h *Host) ARListRepositories(location string) []string {
	if location == "" {
		location = h.region
	}
	parent := fmt.Sprintf("projects/%s/locations/%s", h.projectID, location)
	res, err := h.arSvc.Projects.Locations.Repositories.List(parent).Do()
	if err != nil {
		h.fail(err)
		return nil
	}
	h.clearErr()
	var names []string
	for _, r := range res.Repositories {
		names = append(names, shortResourceName(r.Name))
	}
	sort.Strings(names)
	return names
}

func (h *Host) ARGetRepository(location, name string) *ArtifactRepo {
	if location == "" {
		location = h.region
	}
	full := fmt.Sprintf("projects/%s/locations/%s/repositories/%s", h.projectID, location, name)
	r, err := h.arSvc.Projects.Locations.Repositories.Get(full).Do()
	if err != nil {
		h.fail(err)
		return &ArtifactRepo{NameVal: name, LocationVal: location}
	}
	var immut, vuln bool
	var cleanup, sizeMB, pkgCount int64
	if h.fakeSrv != nil {
		immut, vuln, cleanup, sizeMB, pkgCount = h.fakeSrv.GetARMeta(location, name)
	}
	h.clearErr()
	return &ArtifactRepo{
		NameVal:                  name,
		LocationVal:              location,
		FormatVal:                r.Format,
		ImmutableTagsVal:         immut,
		CleanupPolicyCountVal:    cleanup,
		VulnerabilityScanningVal: vuln,
		SizeMBVal:                sizeMB,
		PackageCountVal:          pkgCount,
	}
}

func (h *Host) ARCreateRepository(location, name, format string, immutableTags bool) *ArtifactRepo {
	if location == "" {
		location = h.region
	}
	if err := safety.ValidateResourcePrefix("artifact_repo", name); err != nil {
		h.fail(err)
		return &ArtifactRepo{NameVal: name, LocationVal: location}
	}
	parent := fmt.Sprintf("projects/%s/locations/%s", h.projectID, location)
	req := &artifactregistry.Repository{
		Format: strings.ToUpper(format),
		Labels: h.ledger.EnrichLabels(nil, fakegcp.DefaultEpoch),
	}
	if _, err := h.arSvc.Projects.Locations.Repositories.Create(parent, req).RepositoryId(name).Do(); err != nil {
		h.fail(err)
		return &ArtifactRepo{NameVal: name, LocationVal: location}
	}
	if h.fakeSrv != nil {
		h.fakeSrv.SetARMeta(location, name, immutableTags, true, 1, 10, 1)
	}
	h.ledger.Record("artifact_repo", location, name, fakegcp.DefaultEpoch)
	h.clearErr()
	return h.ARGetRepository(location, name)
}

func (h *Host) ARDeleteRepository(location, name string) bool {
	if location == "" {
		location = h.region
	}
	if err := h.ledger.ValidateModifyOrDelete("artifact_repo", name); err != nil {
		h.fail(err)
		return false
	}
	full := fmt.Sprintf("projects/%s/locations/%s/repositories/%s", h.projectID, location, name)
	if _, err := h.arSvc.Projects.Locations.Repositories.Delete(full).Do(); err != nil {
		h.fail(err)
		return false
	}
	h.ledger.Remove("artifact_repo", location, name)
	h.clearErr()
	return true
}

// --- 10. Cloud DNS ---

func (h *Host) DNSListZones() []string {
	res, err := h.dnsSvc.ManagedZones.List(h.projectID).Do()
	if err != nil {
		h.fail(err)
		return nil
	}
	h.clearErr()
	var names []string
	for _, z := range res.ManagedZones {
		names = append(names, z.Name)
	}
	sort.Strings(names)
	return names
}

func (h *Host) DNSGetZone(name string) *DnsZone {
	z, err := h.dnsSvc.ManagedZones.Get(h.projectID, name).Do()
	if err != nil {
		h.fail(err)
		return &DnsZone{NameVal: name}
	}
	var dnssec bool
	if z.DnssecConfig != nil && z.DnssecConfig.State == "on" {
		dnssec = true
	}
	recs, _ := h.dnsSvc.ResourceRecordSets.List(h.projectID, name).Do()
	var rc int64
	if recs != nil {
		rc = int64(len(recs.Rrsets))
	}
	h.clearErr()
	return &DnsZone{
		NameVal:        z.Name,
		DNSNameVal:     z.DnsName,
		VisibilityVal:  z.Visibility,
		DNSSECVal:      dnssec,
		RecordCountVal: rc,
	}
}

func (h *Host) DNSCreateZone(name, dnsName, visibility string, dnssec bool) *DnsZone {
	if err := safety.ValidateResourcePrefix("dns_zone", name); err != nil {
		h.fail(err)
		return &DnsZone{NameVal: name}
	}
	state := "off"
	if dnssec {
		state = "on"
	}
	req := &dns.ManagedZone{
		Name:         name,
		DnsName:      dnsName,
		Visibility:   visibility,
		Labels:       h.ledger.EnrichLabels(nil, fakegcp.DefaultEpoch),
		DnssecConfig: &dns.ManagedZoneDnsSecConfig{State: state},
	}
	if _, err := h.dnsSvc.ManagedZones.Create(h.projectID, req).Do(); err != nil {
		h.fail(err)
		return &DnsZone{NameVal: name}
	}
	h.ledger.Record("dns_zone", "global", name, fakegcp.DefaultEpoch)
	h.clearErr()
	return h.DNSGetZone(name)
}

func (h *Host) DNSListRecords(zoneName string) []string {
	res, err := h.dnsSvc.ResourceRecordSets.List(h.projectID, zoneName).Do()
	if err != nil {
		h.fail(err)
		return nil
	}
	h.clearErr()
	var out []string
	for _, r := range res.Rrsets {
		out = append(out, fmt.Sprintf("%s|%s|%d|%s", r.Name, r.Type, r.Ttl, strings.Join(r.Rrdatas, ",")))
	}
	sort.Strings(out)
	return out
}

func (h *Host) DNSAddRecord(zoneName, fqdn, recType string, ttl int64, rrdata string) bool {
	if err := h.ledger.ValidateModifyOrDelete("dns_zone", zoneName); err != nil {
		h.fail(err)
		return false
	}
	_, err := h.dnsSvc.ResourceRecordSets.Create(h.projectID, zoneName, &dns.ResourceRecordSet{
		Name:    fqdn,
		Type:    recType,
		Ttl:     ttl,
		Rrdatas: []string{rrdata},
	}).Do()
	if err != nil {
		h.fail(err)
		return false
	}
	h.clearErr()
	return true
}

func (h *Host) DNSDeleteRecord(zoneName, fqdn, recType string) bool {
	if err := h.ledger.ValidateModifyOrDelete("dns_zone", zoneName); err != nil {
		h.fail(err)
		return false
	}
	if _, err := h.dnsSvc.ResourceRecordSets.Delete(h.projectID, zoneName, fqdn, recType).Do(); err != nil {
		h.fail(err)
		return false
	}
	h.clearErr()
	return true
}

func (h *Host) DNSDeleteZone(name string) bool {
	if err := h.ledger.ValidateModifyOrDelete("dns_zone", name); err != nil {
		h.fail(err)
		return false
	}
	if err := h.dnsSvc.ManagedZones.Delete(h.projectID, name).Do(); err != nil {
		h.fail(err)
		return false
	}
	h.ledger.Remove("dns_zone", "global", name)
	h.clearErr()
	return true
}

// --- 11. Cloud Logging & Cloud Monitoring ---

func (h *Host) LoggingListSinks() []string {
	res, err := h.loggingSvc.Projects.Sinks.List("projects/" + h.projectID).Do()
	if err != nil {
		h.fail(err)
		return nil
	}
	h.clearErr()
	var names []string
	for _, s := range res.Sinks {
		names = append(names, s.Name)
	}
	sort.Strings(names)
	return names
}

func (h *Host) LoggingGetSink(name string) *LogSink {
	sk, err := h.loggingSvc.Projects.Sinks.Get(fmt.Sprintf("projects/%s/sinks/%s", h.projectID, name)).Do()
	if err != nil {
		h.fail(err)
		return &LogSink{NameVal: name}
	}
	h.clearErr()
	return &LogSink{
		NameVal:        sk.Name,
		DestinationVal: sk.Destination,
		FilterVal:      sk.Filter,
		DisabledVal:    sk.Disabled,
	}
}

func (h *Host) LoggingCreateSink(name, destination, filter string) *LogSink {
	if err := safety.ValidateResourcePrefix("log_sink", name); err != nil {
		h.fail(err)
		return &LogSink{NameVal: name}
	}
	req := &logging.LogSink{
		Name:        name,
		Destination: destination,
		Filter:      filter,
	}
	if _, err := h.loggingSvc.Projects.Sinks.Create("projects/"+h.projectID, req).Do(); err != nil {
		h.fail(err)
		return &LogSink{NameVal: name}
	}
	h.ledger.Record("log_sink", "global", name, fakegcp.DefaultEpoch)
	h.clearErr()
	return h.LoggingGetSink(name)
}

func (h *Host) LoggingDeleteSink(name string) bool {
	if err := h.ledger.ValidateModifyOrDelete("log_sink", name); err != nil {
		h.fail(err)
		return false
	}
	if _, err := h.loggingSvc.Projects.Sinks.Delete(fmt.Sprintf("projects/%s/sinks/%s", h.projectID, name)).Do(); err != nil {
		h.fail(err)
		return false
	}
	h.ledger.Remove("log_sink", "global", name)
	h.clearErr()
	return true
}

func (h *Host) LoggingWriteEntry(logName, severity, message string, labels map[string]string) bool {
	if err := safety.ValidateResourcePrefix("log_stream", logName); err != nil {
		h.fail(err)
		return false
	}
	cleanMsg := safety.RedactSecrets(message)
	_, err := h.loggingSvc.Entries.Write(&logging.WriteLogEntriesRequest{
		LogName: fmt.Sprintf("projects/%s/logs/%s", h.projectID, logName),
		Labels:  h.ledger.EnrichLabels(labels, fakegcp.DefaultEpoch),
		Entries: []*logging.LogEntry{
			{Severity: strings.ToUpper(severity), TextPayload: cleanMsg},
		},
	}).Do()
	if err != nil {
		h.fail(err)
		return false
	}
	if h.fakeSrv != nil && strings.ToUpper(severity) != "INFO" {
		h.fakeSrv.BumpLogSeverity(severity)
	}
	h.clearErr()
	return true
}

func (h *Host) LoggingSeverityCounts() map[string]string {
	out := make(map[string]string)
	if h.fakeSrv != nil {
		for k, v := range h.fakeSrv.LogSeverityCounts() {
			out[k] = strconv.FormatInt(v, 10)
		}
	}
	h.clearErr()
	return out
}

func (h *Host) MonitoringListMetrics() []string {
	if h.fakeSrv != nil {
		h.clearErr()
		return h.fakeSrv.ListMetrics()
	}
	return nil
}

func (h *Host) MonitoringQuerySeries(metricType string, points int64) []float64 {
	if h.fakeSrv != nil {
		h.clearErr()
		return h.fakeSrv.QueryMetricSeries(metricType, int(points))
	}
	return nil
}

func (h *Host) MonitoringListAlertPolicies() []string {
	res, err := h.monitoringSvc.Projects.AlertPolicies.List("projects/" + h.projectID).Do()
	if err != nil {
		h.fail(err)
		return nil
	}
	h.clearErr()
	var names []string
	for _, ap := range res.AlertPolicies {
		names = append(names, shortResourceName(ap.Name))
	}
	sort.Strings(names)
	return names
}

func (h *Host) MonitoringGetAlertPolicy(name string) *AlertPolicy {
	ap, err := h.monitoringSvc.Projects.AlertPolicies.Get(fmt.Sprintf("projects/%s/alertPolicies/%s", h.projectID, name)).Do()
	if err != nil {
		h.fail(err)
		return &AlertPolicy{NameVal: name}
	}
	var mtype string
	var thresh float64
	var dur int64
	if h.fakeSrv != nil {
		mtype, thresh, dur = h.fakeSrv.GetAlertMeta(name)
	}
	h.clearErr()
	return &AlertPolicy{
		NameVal:                 shortResourceName(ap.Name),
		MetricTypeVal:           mtype,
		ThresholdVal:            thresh,
		DurationSecVal:          dur,
		EnabledVal:              ap.Enabled,
		NotificationChannelsVal: int64(len(ap.NotificationChannels)),
	}
}

func (h *Host) MonitoringCreateAlertPolicy(name, metricType string, threshold float64, durSec int64) *AlertPolicy {
	if err := safety.ValidateResourcePrefix("alert_policy", name); err != nil {
		h.fail(err)
		return &AlertPolicy{NameVal: name}
	}
	req := &monitoring.AlertPolicy{
		DisplayName:          name,
		Enabled:              true,
		UserLabels:           h.ledger.EnrichLabels(nil, fakegcp.DefaultEpoch),
		NotificationChannels: []string{"projects/" + h.projectID + "/notificationChannels/slack-sre"},
	}
	if _, err := h.monitoringSvc.Projects.AlertPolicies.Create("projects/"+h.projectID, req).Do(); err != nil {
		h.fail(err)
		return &AlertPolicy{NameVal: name}
	}
	if h.fakeSrv != nil {
		h.fakeSrv.SetAlertMeta(name, metricType, threshold, durSec)
	}
	h.ledger.Record("alert_policy", "global", name, fakegcp.DefaultEpoch)
	h.clearErr()
	return h.MonitoringGetAlertPolicy(name)
}

func (h *Host) MonitoringDeleteAlertPolicy(name string) bool {
	if err := h.ledger.ValidateModifyOrDelete("alert_policy", name); err != nil {
		h.fail(err)
		return false
	}
	if _, err := h.monitoringSvc.Projects.AlertPolicies.Delete(fmt.Sprintf("projects/%s/alertPolicies/%s", h.projectID, name)).Do(); err != nil {
		h.fail(err)
		return false
	}
	h.ledger.Remove("alert_policy", "global", name)
	h.clearErr()
	return true
}

// --- 12. Cross-Service Inventory, Compliance & Topology Graph ---

func (h *Host) InventorySummary() map[string]string {
	m := map[string]string{
		"compute_instances": strconv.Itoa(len(h.ComputeListInstances("all"))),
		"vpc_networks":      strconv.Itoa(len(h.NetworkListVPCs())),
		"subnets":           strconv.Itoa(len(h.NetworkListSubnets("all"))),
		"firewall_rules":    strconv.Itoa(len(h.NetworkListFirewalls())),
		"storage_buckets":   strconv.Itoa(len(h.StorageListBuckets())),
		"pubsub_topics":     strconv.Itoa(len(h.PubsubListTopics())),
		"pubsub_subs":       strconv.Itoa(len(h.PubsubListSubscriptions())),
		"cloud_run":         strconv.Itoa(len(h.RunListServices("-"))),
		"sql_instances":     strconv.Itoa(len(h.SQLListInstances())),
		"bq_datasets":       strconv.Itoa(len(h.BQListDatasets())),
		"gke_clusters":      strconv.Itoa(len(h.GKEListClusters("-"))),
		"artifact_repos":    strconv.Itoa(len(h.ARListRepositories("all"))),
		"dns_zones":         strconv.Itoa(len(h.DNSListZones())),
		"secrets_metadata":  strconv.Itoa(len(h.SecretsList())),
		"log_sinks":         strconv.Itoa(len(h.LoggingListSinks())),
		"alert_policies":    strconv.Itoa(len(h.MonitoringListAlertPolicies())),
	}
	h.clearErr()
	return m
}

func hasAllKeys(labels map[string]string, req []string) bool {
	for _, k := range req {
		if strings.TrimSpace(labels[k]) == "" {
			return false
		}
	}
	return true
}

func pctString(okCount, total int) string {
	if total == 0 {
		return "100"
	}
	return strconv.Itoa((okCount * 100) / total)
}

func (h *Host) AuditLabelCompliance(reqKeys []string) map[string]string {
	out := make(map[string]string)

	// Compute
	var compOK, compTotal int
	if res, err := h.computeSvc.Instances.List(h.projectID, "all").Do(); err == nil {
		for _, i := range res.Items {
			compTotal++
			if hasAllKeys(i.Labels, reqKeys) {
				compOK++
			}
		}
	}
	out["compute_pct"] = pctString(compOK, compTotal)
	out["compute_compliant"] = strconv.Itoa(compOK)
	out["compute_total"] = strconv.Itoa(compTotal)

	// Storage
	var bktOK, bktTotal int
	for _, name := range h.StorageListBuckets() {
		bktTotal++
		b := h.StorageGetBucket(name)
		if hasAllKeys(b.LabelsVal, reqKeys) {
			bktOK++
		}
	}
	out["storage_pct"] = pctString(bktOK, bktTotal)
	out["storage_compliant"] = strconv.Itoa(bktOK)
	out["storage_total"] = strconv.Itoa(bktTotal)

	// Cloud Run
	var runOK, runTotal int
	if res, err := h.runSvc.Projects.Locations.Services.List(fmt.Sprintf("projects/%s/locations/-", h.projectID)).Do(); err == nil {
		for _, s := range res.Services {
			runTotal++
			if hasAllKeys(s.Labels, reqKeys) {
				runOK++
			}
		}
	}
	out["cloud_run_pct"] = pctString(runOK, runTotal)

	// SQL
	var sqlOK, sqlTotal int
	for _, name := range h.SQLListInstances() {
		sqlTotal++
		s := h.SQLGetInstance(name)
		if hasAllKeys(s.LabelsVal, reqKeys) {
			sqlOK++
		}
	}
	out["sql_pct"] = pctString(sqlOK, sqlTotal)

	// GKE
	var gkeOK, gkeTotal int
	if res, err := h.gkeSvc.Projects.Locations.Clusters.List(fmt.Sprintf("projects/%s/locations/-", h.projectID)).Do(); err == nil {
		for _, c := range res.Clusters {
			gkeTotal++
			if hasAllKeys(c.ResourceLabels, reqKeys) {
				gkeOK++
			}
		}
	}
	out["gke_pct"] = pctString(gkeOK, gkeTotal)

	// Secrets
	var secOK, secTotal int
	for _, name := range h.SecretsList() {
		secTotal++
		s := h.SecretsGetMetadata(name)
		if hasAllKeys(s.LabelsVal, reqKeys) {
			secOK++
		}
	}
	out["secrets_pct"] = pctString(secOK, secTotal)

	totalOK := compOK + bktOK + runOK + sqlOK + gkeOK + secOK
	totalAll := compTotal + bktTotal + runTotal + sqlTotal + gkeTotal + secTotal
	out["overall_pct"] = pctString(totalOK, totalAll)
	out["overall_compliant"] = strconv.Itoa(totalOK)
	out["overall_total"] = strconv.Itoa(totalAll)
	h.clearErr()
	return out
}

func (h *Host) ResourceGraphEdges() []string {
	var edges []string
	if subs, err := h.computeSvc.Subnetworks.List(h.projectID, "all").Do(); err == nil {
		for _, sn := range subs.Items {
			edges = append(edges, fmt.Sprintf("vpc:%s->subnet:%s", sn.Network, sn.Name))
		}
	}
	if fws, err := h.computeSvc.Firewalls.List(h.projectID).Do(); err == nil {
		for _, fw := range fws.Items {
			edges = append(edges, fmt.Sprintf("vpc:%s->firewall:%s", fw.Network, fw.Name))
		}
	}
	if insts, err := h.computeSvc.Instances.List(h.projectID, "all").Do(); err == nil {
		for _, inst := range insts.Items {
			if len(inst.NetworkInterfaces) > 0 {
				edges = append(edges, fmt.Sprintf("vpc:%s->instance:%s", inst.NetworkInterfaces[0].Network, inst.Name))
			}
		}
	}
	for _, subName := range h.PubsubListSubscriptions() {
		sb := h.PubsubGetSubscription(subName)
		edges = append(edges, fmt.Sprintf("topic:%s->subscription:%s", sb.TopicVal, sb.NameVal))
		if sb.DeadLetterTopicVal != "" {
			edges = append(edges, fmt.Sprintf("subscription:%s->dlq:%s", sb.NameVal, sb.DeadLetterTopicVal))
		}
	}
	for _, sinkName := range h.LoggingListSinks() {
		sk := h.LoggingGetSink(sinkName)
		edges = append(edges, fmt.Sprintf("log_sink:%s->dest:%s", sk.NameVal, shortResourceName(sk.DestinationVal)))
	}
	sort.Strings(edges)
	h.clearErr()
	return edges
}
