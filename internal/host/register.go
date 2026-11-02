// SPDX-License-Identifier: MIT

package host

import (
	"github.com/lmctl-ai/lmcode-go"
	"github.com/lmctl-ai/lmgcp/internal/membudget"
	"github.com/lmctl-ai/lmgcp/internal/safety"
)

// Register installs the lmgcp context, testing helpers, and all GCP service methods on reg.
func Register(reg *lmcode.Registry, h *Host) error {
	if err := lmcode.RegisterTesting(reg, nil); err != nil {
		return err
	}
	if err := reg.Context("gcp", "Gcp"); err != nil {
		return err
	}

	m := func(typeName, name string, params []string, ret string, fn lmcode.Method) error {
		return reg.Method(typeName, name, params, ret, fn)
	}
	mSnapshot := func(typeName, name string, params []string, ret string, fn lmcode.Method) error {
		// Explicit purity declaration: snapshot accessors use the same callback in dry.
		return reg.Method(typeName, name, params, ret, fn, lmcode.Dry(fn))
	}
	mLocal := func(typeName, name string, params []string, ret string, fn lmcode.Method) error {
		// Local-only configuration updates are safe to execute in dry mode.
		return reg.Method(typeName, name, params, ret, fn, lmcode.Dry(fn))
	}
	mSelf := func(typeName, name string, params []string, ret string, fn lmcode.Method) error {
		preview := lmcode.Method(func(self any, _ []lmcode.Value) (lmcode.Value, error) {
			return lmcode.Object(typeName, self), nil
		})
		return reg.Method(typeName, name, params, ret, fn, lmcode.Dry(preview))
	}
	mRead := func(typeName, name string, params []string, ret string, fn lmcode.Method) error {
		dryFn := fn
		if zeroHostObject(ret) != nil {
			dryFn = func(self any, args []lmcode.Value) (lmcode.Value, error) {
				value, err := fn(self, args)
				if err != nil {
					return value, err
				}
				if value.Kind == lmcode.KindObj && value.Obj == nil {
					return lmcode.Object(ret, zeroHostObject(ret)), nil
				}
				return value, nil
			}
		}
		return reg.Method(typeName, name, params, ret, fn, lmcode.Dry(dryFn))
	}
	mPreview := func(typeName, name string, params []string, ret string, fn lmcode.Method) error {
		preview := lmcode.Method(func(any, []lmcode.Value) (lmcode.Value, error) {
			return lmcode.Object(ret, zeroHostObject(ret)), nil
		})
		return reg.Method(typeName, name, params, ret, fn, lmcode.Dry(preview))
	}

	// --- Core & Safety on Gcp ---
	if err := mRead("Gcp", "project", nil, "string", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(self.(*Host).Project()), nil
	}); err != nil {
		return err
	}
	if err := mRead("Gcp", "masked_project", nil, "string", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(self.(*Host).MaskedProject()), nil
	}); err != nil {
		return err
	}
	if err := mLocal("Gcp", "set_project", []string{"string"}, "Gcp", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		hs := self.(*Host)
		hs.SetProject(a[0].Str)
		return lmcode.Obj("Gcp", hs), nil
	}); err != nil {
		return err
	}
	if err := mRead("Gcp", "region", nil, "string", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(self.(*Host).Region()), nil
	}); err != nil {
		return err
	}
	if err := mLocal("Gcp", "set_region", []string{"string"}, "Gcp", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		hs := self.(*Host)
		hs.SetRegion(a[0].Str)
		return lmcode.Obj("Gcp", hs), nil
	}); err != nil {
		return err
	}
	if err := mRead("Gcp", "zone", nil, "string", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(self.(*Host).Zone()), nil
	}); err != nil {
		return err
	}
	if err := mLocal("Gcp", "set_zone", []string{"string"}, "Gcp", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		hs := self.(*Host)
		hs.SetZone(a[0].Str)
		return lmcode.Obj("Gcp", hs), nil
	}); err != nil {
		return err
	}
	if err := mRead("Gcp", "is_live", nil, "bool", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(self.(*Host).IsLive()), nil
	}); err != nil {
		return err
	}
	if err := mRead("Gcp", "auth_mode", nil, "string", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(self.(*Host).AuthMode()), nil
	}); err != nil {
		return err
	}
	if err := mRead("Gcp", "endpoint", nil, "string", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(self.(*Host).Endpoint()), nil
	}); err != nil {
		return err
	}
	if err := mRead("Gcp", "run_id", nil, "string", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(self.(*Host).RunID()), nil
	}); err != nil {
		return err
	}
	if err := mLocal("Gcp", "set_run_id", []string{"string"}, "Gcp", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		hs := self.(*Host)
		hs.SetRunID(a[0].Str)
		return lmcode.Obj("Gcp", hs), nil
	}); err != nil {
		return err
	}
	if err := mRead("Gcp", "last_error", nil, "string", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(self.(*Host).LastError()), nil
	}); err != nil {
		return err
	}
	if err := mRead("Gcp", "redact", []string{"string"}, "string", func(_ any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(safety.RedactSecrets(a[0].Str)), nil
	}); err != nil {
		return err
	}
	if err := mSelf("Gcp", "set_auto_cleanup", []string{"bool"}, "Gcp", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		hs := self.(*Host)
		hs.SetAutoCleanup(a[0].Bool)
		return lmcode.Obj("Gcp", hs), nil
	}); err != nil {
		return err
	}
	if err := mRead("Gcp", "ledger_count", nil, "int", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(self.(*Host).LedgerCount()), nil
	}); err != nil {
		return err
	}
	if err := mRead("Gcp", "ledger_entries", nil, "StrList", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return makeStrList(reg, self.(*Host).LedgerEntries())
	}); err != nil {
		return err
	}
	if err := mRead("Gcp", "ledger_owns", []string{"string", "string"}, "bool", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(self.(*Host).LedgerOwns(a[0].Str, a[1].Str)), nil
	}); err != nil {
		return err
	}
	if err := mRead("Gcp", "cost_cap_usd", nil, "float", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Float(self.(*Host).CostCapUSD()), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "set_cost_cap_usd", []string{"float"}, "bool", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(self.(*Host).SetCostCapUSD(a[0].Float)), nil
	}); err != nil {
		return err
	}
	if err := mRead("Gcp", "estimated_cost_usd", nil, "float", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Float(self.(*Host).EstimatedCostUSD()), nil
	}); err != nil {
		return err
	}
	if err := mRead("Gcp", "cost_report", nil, "string", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(self.(*Host).CostReport()), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "write_cost_evidence", []string{"string", "string"}, "bool", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(self.(*Host).WriteCostEvidence(a[0].Str, a[1].Str)), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "cleanup_ledger", nil, "int", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(self.(*Host).CleanupLedger()), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "seed_demo_age", []string{"string", "string", "int"}, "bool", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(self.(*Host).SeedDemoAge(a[0].Str, a[1].Str, a[2].Int)), nil
	}); err != nil {
		return err
	}
	if err := mRead("Gcp", "sweep_dry_run", []string{"int"}, "StrList", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return makeStrList(reg, self.(*Host).SweepDryRun(a[0].Int))
	}); err != nil {
		return err
	}
	if err := m("Gcp", "sweep_delete", []string{"int"}, "int", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(self.(*Host).SweepDelete(a[0].Int)), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "alloc_scratch_mb", []string{"int"}, "bool", func(_ any, a []lmcode.Value) (lmcode.Value, error) {
		if a[0].Int <= 0 {
			return lmcode.Bool(true), nil
		}
		if err := membudget.Charge(a[0].Int * 1024 * 1024); err != nil {
			return lmcode.None(), err
		}
		return lmcode.Bool(true), nil
	}); err != nil {
		return err
	}

	// --- 1. Compute Engine on Gcp ---
	if err := mRead("Gcp", "compute_list_instances", []string{"string"}, "StrList", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return makeStrList(reg, self.(*Host).ComputeListInstances(a[0].Str))
	}); err != nil {
		return err
	}
	if err := mRead("Gcp", "compute_get_instance", []string{"string", "string"}, "ComputeInstance", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Obj("ComputeInstance", self.(*Host).ComputeGetInstance(a[0].Str, a[1].Str)), nil
	}); err != nil {
		return err
	}
	if err := mRead("Gcp", "compute_instance_count", []string{"string"}, "int", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(int64(len(self.(*Host).ComputeListInstances(a[0].Str)))), nil
	}); err != nil {
		return err
	}
	if err := mRead("Gcp", "compute_list_disks", []string{"string"}, "StrList", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return makeStrList(reg, self.(*Host).ComputeListDisks(a[0].Str))
	}); err != nil {
		return err
	}
	if err := mPreview("Gcp", "compute_create_instance", []string{"string", "string", "string", "string"}, "ComputeInstance", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Obj("ComputeInstance", self.(*Host).ComputeCreateInstance(a[0].Str, a[1].Str, a[2].Str, a[3].Str)), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "compute_set_instance_labels", []string{"string", "string", "StrMap"}, "bool", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(self.(*Host).ComputeSetInstanceLabels(a[0].Str, a[1].Str, extractStrMap(a[2]))), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "compute_stop_instance", []string{"string", "string"}, "bool", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(self.(*Host).ComputeStopInstance(a[0].Str, a[1].Str)), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "compute_start_instance", []string{"string", "string"}, "bool", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(self.(*Host).ComputeStartInstance(a[0].Str, a[1].Str)), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "compute_delete_instance", []string{"string", "string"}, "bool", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(self.(*Host).ComputeDeleteInstance(a[0].Str, a[1].Str)), nil
	}); err != nil {
		return err
	}

	// ComputeInstance methods
	if err := mSnapshot("ComputeInstance", "name", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*ComputeInstance).NameVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("ComputeInstance", "zone", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*ComputeInstance).ZoneVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("ComputeInstance", "machine_type", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*ComputeInstance).MachineTypeVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("ComputeInstance", "status", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*ComputeInstance).StatusVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("ComputeInstance", "network", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*ComputeInstance).NetworkVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("ComputeInstance", "internal_ip", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*ComputeInstance).InternalIPVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("ComputeInstance", "external_ip", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*ComputeInstance).ExternalIPVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("ComputeInstance", "cpu_platform", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*ComputeInstance).CPUPlatformVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("ComputeInstance", "disk_gb", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*ComputeInstance).DiskGBVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("ComputeInstance", "vcpus", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*ComputeInstance).VCPUsVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("ComputeInstance", "memory_mb", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*ComputeInstance).MemoryMBVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("ComputeInstance", "labels", nil, "StrMap", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return makeStrMap(reg, s.(*ComputeInstance).LabelsVal)
	}); err != nil {
		return err
	}
	if err := mSnapshot("ComputeInstance", "has_label", []string{"string"}, "bool", func(s any, a []lmcode.Value) (lmcode.Value, error) {
		_, ok := s.(*ComputeInstance).LabelsVal[a[0].Str]
		return lmcode.Bool(ok), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("ComputeInstance", "label", []string{"string"}, "string", func(s any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*ComputeInstance).LabelsVal[a[0].Str]), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("ComputeInstance", "service_account", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*ComputeInstance).ServiceAccountVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("ComputeInstance", "shielded_vtpm", nil, "bool", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(s.(*ComputeInstance).ShieldedVTPMVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("ComputeInstance", "deletion_protection", nil, "bool", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(s.(*ComputeInstance).DeletionProtectionVal), nil
	}); err != nil {
		return err
	}

	// --- 2. Network, Subnets & Firewalls ---
	if err := mRead("Gcp", "network_list_vpcs", nil, "StrList", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return makeStrList(reg, self.(*Host).NetworkListVPCs())
	}); err != nil {
		return err
	}
	if err := mRead("Gcp", "network_get_vpc", []string{"string"}, "VpcNetwork", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Obj("VpcNetwork", self.(*Host).NetworkGetVPC(a[0].Str)), nil
	}); err != nil {
		return err
	}
	if err := mPreview("Gcp", "network_create_vpc", []string{"string", "bool"}, "VpcNetwork", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Obj("VpcNetwork", self.(*Host).NetworkCreateVPC(a[0].Str, a[1].Bool)), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "network_delete_vpc", []string{"string"}, "bool", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(self.(*Host).NetworkDeleteVPC(a[0].Str)), nil
	}); err != nil {
		return err
	}
	if err := mRead("Gcp", "network_list_subnets", []string{"string"}, "StrList", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return makeStrList(reg, self.(*Host).NetworkListSubnets(a[0].Str))
	}); err != nil {
		return err
	}
	if err := mRead("Gcp", "network_get_subnet", []string{"string", "string"}, "Subnet", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Obj("Subnet", self.(*Host).NetworkGetSubnet(a[0].Str, a[1].Str)), nil
	}); err != nil {
		return err
	}
	if err := mPreview("Gcp", "network_create_subnet", []string{"string", "string", "string", "string", "bool"}, "Subnet", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Obj("Subnet", self.(*Host).NetworkCreateSubnet(a[0].Str, a[1].Str, a[2].Str, a[3].Str, a[4].Bool)), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "network_delete_subnet", []string{"string", "string"}, "bool", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(self.(*Host).NetworkDeleteSubnet(a[0].Str, a[1].Str)), nil
	}); err != nil {
		return err
	}
	if err := mRead("Gcp", "network_list_firewalls", nil, "StrList", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return makeStrList(reg, self.(*Host).NetworkListFirewalls())
	}); err != nil {
		return err
	}
	if err := mRead("Gcp", "network_get_firewall", []string{"string"}, "FirewallRule", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Obj("FirewallRule", self.(*Host).NetworkGetFirewall(a[0].Str)), nil
	}); err != nil {
		return err
	}
	if err := mPreview("Gcp", "network_create_firewall", []string{"string", "string", "string", "int", "string", "string", "string"}, "FirewallRule", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Obj("FirewallRule", self.(*Host).NetworkCreateFirewall(a[0].Str, a[1].Str, a[2].Str, a[3].Int, a[4].Str, a[5].Str, a[6].Str)), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "network_delete_firewall", []string{"string"}, "bool", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(self.(*Host).NetworkDeleteFirewall(a[0].Str)), nil
	}); err != nil {
		return err
	}

	// VpcNetwork methods
	if err := mSnapshot("VpcNetwork", "name", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*VpcNetwork).NameVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("VpcNetwork", "auto_create_subnets", nil, "bool", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(s.(*VpcNetwork).AutoCreateSubnetsVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("VpcNetwork", "routing_mode", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*VpcNetwork).RoutingModeVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("VpcNetwork", "mtu", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*VpcNetwork).MTUVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("VpcNetwork", "subnet_count", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*VpcNetwork).SubnetCountVal), nil
	}); err != nil {
		return err
	}

	// Subnet methods
	if err := mSnapshot("Subnet", "name", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*Subnet).NameVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("Subnet", "region", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*Subnet).RegionVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("Subnet", "network", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*Subnet).NetworkVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("Subnet", "cidr", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*Subnet).CIDRVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("Subnet", "private_ip_google_access", nil, "bool", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(s.(*Subnet).PrivateIPGoogleAccessVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("Subnet", "flow_logs", nil, "bool", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(s.(*Subnet).FlowLogsVal), nil
	}); err != nil {
		return err
	}

	// FirewallRule methods
	if err := mSnapshot("FirewallRule", "name", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*FirewallRule).NameVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("FirewallRule", "network", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*FirewallRule).NetworkVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("FirewallRule", "direction", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*FirewallRule).DirectionVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("FirewallRule", "priority", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*FirewallRule).PriorityVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("FirewallRule", "action", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*FirewallRule).ActionVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("FirewallRule", "protocol", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*FirewallRule).ProtocolVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("FirewallRule", "ports", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*FirewallRule).PortsVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("FirewallRule", "source_cidr", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*FirewallRule).SourceCIDRVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("FirewallRule", "target_tags", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*FirewallRule).TargetTagsVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("FirewallRule", "disabled", nil, "bool", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(s.(*FirewallRule).DisabledVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("FirewallRule", "log_enabled", nil, "bool", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(s.(*FirewallRule).LogEnabledVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("FirewallRule", "is_open_to_world", nil, "bool", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(s.(*FirewallRule).IsOpenToWorld()), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("FirewallRule", "risk_score", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*FirewallRule).RiskScore()), nil
	}); err != nil {
		return err
	}

	// --- 3. Cloud Storage (GCS) ---
	if err := mRead("Gcp", "storage_list_buckets", nil, "StrList", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return makeStrList(reg, self.(*Host).StorageListBuckets())
	}); err != nil {
		return err
	}
	if err := mRead("Gcp", "storage_get_bucket", []string{"string"}, "StorageBucket", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Obj("StorageBucket", self.(*Host).StorageGetBucket(a[0].Str)), nil
	}); err != nil {
		return err
	}
	if err := mPreview("Gcp", "storage_create_bucket", []string{"string", "string", "string"}, "StorageBucket", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Obj("StorageBucket", self.(*Host).StorageCreateBucket(a[0].Str, a[1].Str, a[2].Str)), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "storage_set_bucket_lifecycle", []string{"string", "int", "int"}, "bool", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(self.(*Host).StorageSetBucketLifecycle(a[0].Str, a[1].Int, a[2].Int)), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "storage_set_bucket_versioning", []string{"string", "bool"}, "bool", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(self.(*Host).StorageSetBucketVersioning(a[0].Str, a[1].Bool)), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "storage_set_uniform_access", []string{"string", "bool"}, "bool", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(self.(*Host).StorageSetUniformAccess(a[0].Str, a[1].Bool)), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "storage_set_bucket_labels", []string{"string", "StrMap"}, "bool", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(self.(*Host).StorageSetBucketLabels(a[0].Str, extractStrMap(a[1]))), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "storage_put_object", []string{"string", "string", "string", "string"}, "bool", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(self.(*Host).StoragePutObject(a[0].Str, a[1].Str, a[2].Str, a[3].Str)), nil
	}); err != nil {
		return err
	}
	if err := mRead("Gcp", "storage_list_objects", []string{"string", "string"}, "StrList", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return makeStrList(reg, self.(*Host).StorageListObjects(a[0].Str, a[1].Str))
	}); err != nil {
		return err
	}
	if err := mRead("Gcp", "storage_read_object", []string{"string", "string"}, "string", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(self.(*Host).StorageReadObject(a[0].Str, a[1].Str)), nil
	}); err != nil {
		return err
	}
	if err := mRead("Gcp", "storage_object_size", []string{"string", "string"}, "int", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(self.(*Host).StorageObjectSize(a[0].Str, a[1].Str)), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "storage_delete_object", []string{"string", "string"}, "bool", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(self.(*Host).StorageDeleteObject(a[0].Str, a[1].Str)), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "storage_delete_bucket", []string{"string"}, "bool", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(self.(*Host).StorageDeleteBucket(a[0].Str)), nil
	}); err != nil {
		return err
	}

	// StorageBucket methods
	if err := mSnapshot("StorageBucket", "name", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*StorageBucket).NameVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("StorageBucket", "location", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*StorageBucket).LocationVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("StorageBucket", "storage_class", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*StorageBucket).StorageClassVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("StorageBucket", "versioning", nil, "bool", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(s.(*StorageBucket).VersioningVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("StorageBucket", "uniform_access", nil, "bool", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(s.(*StorageBucket).UniformAccessVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("StorageBucket", "public_access_prevention", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*StorageBucket).PublicAccessPreventionVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("StorageBucket", "lifecycle_delete_days", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*StorageBucket).LifecycleDeleteDaysVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("StorageBucket", "lifecycle_nearline_days", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*StorageBucket).LifecycleNearlineDaysVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("StorageBucket", "encryption_type", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*StorageBucket).EncryptionTypeVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("StorageBucket", "object_count", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*StorageBucket).ObjectCountVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("StorageBucket", "total_bytes", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*StorageBucket).TotalBytesVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("StorageBucket", "labels", nil, "StrMap", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return makeStrMap(reg, s.(*StorageBucket).LabelsVal)
	}); err != nil {
		return err
	}
	if err := mSnapshot("StorageBucket", "label", []string{"string"}, "string", func(s any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*StorageBucket).LabelsVal[a[0].Str]), nil
	}); err != nil {
		return err
	}

	// --- 4. Pub/Sub ---
	if err := mRead("Gcp", "pubsub_list_topics", nil, "StrList", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return makeStrList(reg, self.(*Host).PubsubListTopics())
	}); err != nil {
		return err
	}
	if err := mRead("Gcp", "pubsub_get_topic", []string{"string"}, "PubsubTopic", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Obj("PubsubTopic", self.(*Host).PubsubGetTopic(a[0].Str)), nil
	}); err != nil {
		return err
	}
	if err := mPreview("Gcp", "pubsub_create_topic", []string{"string"}, "PubsubTopic", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Obj("PubsubTopic", self.(*Host).PubsubCreateTopic(a[0].Str)), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "pubsub_set_topic_labels", []string{"string", "StrMap"}, "bool", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(self.(*Host).PubsubSetTopicLabels(a[0].Str, extractStrMap(a[1]))), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "pubsub_delete_topic", []string{"string"}, "bool", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(self.(*Host).PubsubDeleteTopic(a[0].Str)), nil
	}); err != nil {
		return err
	}
	if err := mRead("Gcp", "pubsub_list_subscriptions", nil, "StrList", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return makeStrList(reg, self.(*Host).PubsubListSubscriptions())
	}); err != nil {
		return err
	}
	if err := mRead("Gcp", "pubsub_get_subscription", []string{"string"}, "PubsubSubscription", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Obj("PubsubSubscription", self.(*Host).PubsubGetSubscription(a[0].Str)), nil
	}); err != nil {
		return err
	}
	if err := mPreview("Gcp", "pubsub_create_subscription", []string{"string", "string", "int", "string"}, "PubsubSubscription", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Obj("PubsubSubscription", self.(*Host).PubsubCreateSubscription(a[0].Str, a[1].Str, a[2].Int, a[3].Str)), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "pubsub_publish", []string{"string", "string", "StrMap"}, "string", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(self.(*Host).PubsubPublish(a[0].Str, a[1].Str, extractStrMap(a[2]))), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "pubsub_pull", []string{"string", "int"}, "StrList", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return makeStrList(reg, self.(*Host).PubsubPull(a[0].Str, a[1].Int))
	}); err != nil {
		return err
	}
	if err := m("Gcp", "pubsub_delete_subscription", []string{"string"}, "bool", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(self.(*Host).PubsubDeleteSubscription(a[0].Str)), nil
	}); err != nil {
		return err
	}

	// PubsubTopic & PubsubSubscription methods
	if err := mSnapshot("PubsubTopic", "name", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*PubsubTopic).NameVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("PubsubTopic", "retention_sec", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*PubsubTopic).RetentionSecVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("PubsubTopic", "published_count", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*PubsubTopic).PublishedCountVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("PubsubTopic", "labels", nil, "StrMap", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return makeStrMap(reg, s.(*PubsubTopic).LabelsVal)
	}); err != nil {
		return err
	}
	if err := mSnapshot("PubsubSubscription", "name", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*PubsubSubscription).NameVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("PubsubSubscription", "topic", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*PubsubSubscription).TopicVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("PubsubSubscription", "ack_deadline_sec", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*PubsubSubscription).AckDeadlineSecVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("PubsubSubscription", "dead_letter_topic", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*PubsubSubscription).DeadLetterTopicVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("PubsubSubscription", "max_delivery_attempts", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*PubsubSubscription).MaxDeliveryAttemptsVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("PubsubSubscription", "backlog_count", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*PubsubSubscription).BacklogCountVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("PubsubSubscription", "retain_acked", nil, "bool", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(s.(*PubsubSubscription).RetainAckedVal), nil
	}); err != nil {
		return err
	}

	// --- 5. Cloud Run v2 ---
	if err := mRead("Gcp", "run_list_services", []string{"string"}, "StrList", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return makeStrList(reg, self.(*Host).RunListServices(a[0].Str))
	}); err != nil {
		return err
	}
	if err := mRead("Gcp", "run_get_service", []string{"string", "string"}, "CloudRunService", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Obj("CloudRunService", self.(*Host).RunGetService(a[0].Str, a[1].Str)), nil
	}); err != nil {
		return err
	}
	if err := mPreview("Gcp", "run_deploy_service", []string{"string", "string", "string", "int", "int", "int"}, "CloudRunService", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Obj("CloudRunService", self.(*Host).RunDeployService(a[0].Str, a[1].Str, a[2].Str, a[3].Int, a[4].Int, a[5].Int)), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "run_set_traffic_split", []string{"string", "string", "int", "int"}, "bool", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(self.(*Host).RunSetTrafficSplit(a[0].Str, a[1].Str, a[2].Int, a[3].Int)), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "run_set_labels", []string{"string", "string", "StrMap"}, "bool", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(self.(*Host).RunSetLabels(a[0].Str, a[1].Str, extractStrMap(a[2]))), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "run_delete_service", []string{"string", "string"}, "bool", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(self.(*Host).RunDeleteService(a[0].Str, a[1].Str)), nil
	}); err != nil {
		return err
	}

	// CloudRunService methods
	if err := mSnapshot("CloudRunService", "name", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*CloudRunService).NameVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("CloudRunService", "region", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*CloudRunService).RegionVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("CloudRunService", "image", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*CloudRunService).ImageVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("CloudRunService", "uri", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*CloudRunService).URIVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("CloudRunService", "cpu_milli", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*CloudRunService).CPUMilliVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("CloudRunService", "memory_mb", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*CloudRunService).MemoryMBVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("CloudRunService", "min_instances", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*CloudRunService).MinInstancesVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("CloudRunService", "max_instances", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*CloudRunService).MaxInstancesVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("CloudRunService", "concurrency", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*CloudRunService).ConcurrencyVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("CloudRunService", "ingress", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*CloudRunService).IngressVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("CloudRunService", "latest_revision", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*CloudRunService).LatestRevisionVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("CloudRunService", "revision_count", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*CloudRunService).RevisionCountVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("CloudRunService", "stable_traffic_pct", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*CloudRunService).StableTrafficPctVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("CloudRunService", "canary_traffic_pct", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*CloudRunService).CanaryTrafficPctVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("CloudRunService", "labels", nil, "StrMap", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return makeStrMap(reg, s.(*CloudRunService).LabelsVal)
	}); err != nil {
		return err
	}

	// --- 6. Databases & Analytics (SQL, Firestore, BigQuery) ---
	if err := mRead("Gcp", "sql_list_instances", nil, "StrList", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return makeStrList(reg, self.(*Host).SQLListInstances())
	}); err != nil {
		return err
	}
	if err := mRead("Gcp", "sql_get_instance", []string{"string"}, "SqlInstance", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Obj("SqlInstance", self.(*Host).SQLGetInstance(a[0].Str)), nil
	}); err != nil {
		return err
	}
	if err := mPreview("Gcp", "sql_create_instance", []string{"string", "string", "string", "string", "bool"}, "SqlInstance", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Obj("SqlInstance", self.(*Host).SQLCreateInstance(a[0].Str, a[1].Str, a[2].Str, a[3].Str, a[4].Bool)), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "sql_delete_instance", []string{"string"}, "bool", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(self.(*Host).SQLDeleteInstance(a[0].Str)), nil
	}); err != nil {
		return err
	}

	// SqlInstance methods
	if err := mSnapshot("SqlInstance", "name", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*SqlInstance).NameVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("SqlInstance", "db_version", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*SqlInstance).DBVersionVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("SqlInstance", "tier", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*SqlInstance).TierVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("SqlInstance", "region", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*SqlInstance).RegionVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("SqlInstance", "state", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*SqlInstance).StateVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("SqlInstance", "ha_enabled", nil, "bool", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(s.(*SqlInstance).HAEnabledVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("SqlInstance", "backup_enabled", nil, "bool", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(s.(*SqlInstance).BackupEnabledVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("SqlInstance", "pitr_enabled", nil, "bool", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(s.(*SqlInstance).PITREnabledVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("SqlInstance", "require_ssl", nil, "bool", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(s.(*SqlInstance).RequireSSLVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("SqlInstance", "public_ip_enabled", nil, "bool", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(s.(*SqlInstance).PublicIPEnabledVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("SqlInstance", "storage_gb", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*SqlInstance).StorageGBVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("SqlInstance", "labels", nil, "StrMap", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return makeStrMap(reg, s.(*SqlInstance).LabelsVal)
	}); err != nil {
		return err
	}

	// Firestore
	if err := mRead("Gcp", "firestore_list_collections", nil, "StrList", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return makeStrList(reg, self.(*Host).FirestoreListCollections())
	}); err != nil {
		return err
	}
	if err := m("Gcp", "firestore_put_doc", []string{"string", "string", "StrMap"}, "bool", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(self.(*Host).FirestorePutDoc(a[0].Str, a[1].Str, extractStrMap(a[2]))), nil
	}); err != nil {
		return err
	}
	if err := mRead("Gcp", "firestore_get_doc", []string{"string", "string"}, "StrMap", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return makeStrMap(reg, self.(*Host).FirestoreGetDoc(a[0].Str, a[1].Str))
	}); err != nil {
		return err
	}
	if err := mRead("Gcp", "firestore_list_docs", []string{"string"}, "StrList", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return makeStrList(reg, self.(*Host).FirestoreListDocs(a[0].Str))
	}); err != nil {
		return err
	}
	if err := m("Gcp", "firestore_delete_doc", []string{"string", "string"}, "bool", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(self.(*Host).FirestoreDeleteDoc(a[0].Str, a[1].Str)), nil
	}); err != nil {
		return err
	}

	// BigQuery
	if err := mRead("Gcp", "bq_list_datasets", nil, "StrList", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return makeStrList(reg, self.(*Host).BQListDatasets())
	}); err != nil {
		return err
	}
	if err := mRead("Gcp", "bq_get_dataset", []string{"string"}, "BqDataset", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Obj("BqDataset", self.(*Host).BQGetDataset(a[0].Str)), nil
	}); err != nil {
		return err
	}
	if err := mPreview("Gcp", "bq_create_dataset", []string{"string", "string"}, "BqDataset", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Obj("BqDataset", self.(*Host).BQCreateDataset(a[0].Str, a[1].Str)), nil
	}); err != nil {
		return err
	}
	if err := mPreview("Gcp", "bq_create_table", []string{"string", "string", "string", "string"}, "BqTable", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Obj("BqTable", self.(*Host).BQCreateTable(a[0].Str, a[1].Str, a[2].Str, a[3].Str)), nil
	}); err != nil {
		return err
	}
	if err := mRead("Gcp", "bq_get_table", []string{"string", "string"}, "BqTable", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Obj("BqTable", self.(*Host).BQGetTable(a[0].Str, a[1].Str)), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "bq_delete_table", []string{"string", "string"}, "bool", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(self.(*Host).BQDeleteTable(a[0].Str, a[1].Str)), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "bq_delete_dataset", []string{"string"}, "bool", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(self.(*Host).BQDeleteDataset(a[0].Str)), nil
	}); err != nil {
		return err
	}

	// BqDataset & BqTable methods
	if err := mSnapshot("BqDataset", "dataset_id", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*BqDataset).DatasetIDVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("BqDataset", "location", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*BqDataset).LocationVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("BqDataset", "default_expiration_days", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*BqDataset).DefaultExpirationDaysVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("BqDataset", "labels", nil, "StrMap", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return makeStrMap(reg, s.(*BqDataset).LabelsVal)
	}); err != nil {
		return err
	}
	if err := mSnapshot("BqTable", "dataset_id", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*BqTable).DatasetIDVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("BqTable", "table_id", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*BqTable).TableIDVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("BqTable", "schema_spec", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*BqTable).SchemaSpecVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("BqTable", "partition_field", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*BqTable).PartitionFieldVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("BqTable", "row_count", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*BqTable).RowCountVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("BqTable", "bytes", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*BqTable).BytesVal), nil
	}); err != nil {
		return err
	}

	// --- 7. IAM ---
	if err := mRead("Gcp", "iam_list_service_accounts", nil, "StrList", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return makeStrList(reg, self.(*Host).IAMListServiceAccounts())
	}); err != nil {
		return err
	}
	if err := mRead("Gcp", "iam_get_service_account", []string{"string"}, "ServiceAccount", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Obj("ServiceAccount", self.(*Host).IAMGetServiceAccount(a[0].Str)), nil
	}); err != nil {
		return err
	}
	if err := mRead("Gcp", "iam_list_bindings", nil, "StrList", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return makeStrList(reg, self.(*Host).IAMListBindings())
	}); err != nil {
		return err
	}

	// ServiceAccount methods
	if err := mSnapshot("ServiceAccount", "name", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*ServiceAccount).NameVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("ServiceAccount", "email", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*ServiceAccount).EmailVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("ServiceAccount", "display_name", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*ServiceAccount).DisplayNameVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("ServiceAccount", "disabled", nil, "bool", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(s.(*ServiceAccount).DisabledVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("ServiceAccount", "user_managed_keys", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*ServiceAccount).UserManagedKeysVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("ServiceAccount", "key_max_age_days", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*ServiceAccount).KeyMaxAgeDaysVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("ServiceAccount", "is_default_compute", nil, "bool", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(s.(*ServiceAccount).IsDefaultVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("ServiceAccount", "roles", nil, "StrList", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return makeStrList(reg, s.(*ServiceAccount).RolesVal)
	}); err != nil {
		return err
	}

	// --- 8. Secret Manager Metadata & KMS ---
	if err := mRead("Gcp", "secrets_list", nil, "StrList", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return makeStrList(reg, self.(*Host).SecretsList())
	}); err != nil {
		return err
	}
	if err := mRead("Gcp", "secrets_get_metadata", []string{"string"}, "SecretMetadata", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Obj("SecretMetadata", self.(*Host).SecretsGetMetadata(a[0].Str)), nil
	}); err != nil {
		return err
	}
	if err := mPreview("Gcp", "secrets_create_metadata", []string{"string", "string", "int", "StrMap"}, "SecretMetadata", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Obj("SecretMetadata", self.(*Host).SecretsCreateMetadata(a[0].Str, a[1].Str, a[2].Int, extractStrMap(a[3]))), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "secrets_delete", []string{"string"}, "bool", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(self.(*Host).SecretsDelete(a[0].Str)), nil
	}); err != nil {
		return err
	}

	// SecretMetadata methods
	if err := mSnapshot("SecretMetadata", "name", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*SecretMetadata).NameVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("SecretMetadata", "replication_policy", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*SecretMetadata).ReplicationPolicyVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("SecretMetadata", "version_count", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*SecretMetadata).VersionCountVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("SecretMetadata", "enabled_versions", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*SecretMetadata).EnabledVersionsVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("SecretMetadata", "rotation_period_days", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*SecretMetadata).RotationDaysVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("SecretMetadata", "cmek_enabled", nil, "bool", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(s.(*SecretMetadata).CMEKEnabledVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("SecretMetadata", "has_expiration", nil, "bool", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(s.(*SecretMetadata).HasExpirationVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("SecretMetadata", "labels", nil, "StrMap", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return makeStrMap(reg, s.(*SecretMetadata).LabelsVal)
	}); err != nil {
		return err
	}

	// KMS
	if err := mRead("Gcp", "kms_list_keyrings", []string{"string"}, "StrList", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return makeStrList(reg, self.(*Host).KMSListKeyRings(a[0].Str))
	}); err != nil {
		return err
	}
	if err := mRead("Gcp", "kms_list_keys", []string{"string", "string"}, "StrList", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return makeStrList(reg, self.(*Host).KMSListKeys(a[0].Str, a[1].Str))
	}); err != nil {
		return err
	}
	if err := mRead("Gcp", "kms_get_key", []string{"string", "string", "string"}, "KmsKey", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Obj("KmsKey", self.(*Host).KMSGetKey(a[0].Str, a[1].Str, a[2].Str)), nil
	}); err != nil {
		return err
	}
	if err := mPreview("Gcp", "kms_create_key", []string{"string", "string", "string", "string", "int"}, "KmsKey", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Obj("KmsKey", self.(*Host).KMSCreateKey(a[0].Str, a[1].Str, a[2].Str, a[3].Str, a[4].Int)), nil
	}); err != nil {
		return err
	}

	// KmsKey methods
	if err := mSnapshot("KmsKey", "name", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*KmsKey).NameVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("KmsKey", "keyring", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*KmsKey).KeyRingVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("KmsKey", "location", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*KmsKey).LocationVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("KmsKey", "purpose", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*KmsKey).PurposeVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("KmsKey", "rotation_days", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*KmsKey).RotationDaysVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("KmsKey", "primary_state", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*KmsKey).PrimaryStateVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("KmsKey", "protection_level", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*KmsKey).ProtectionLevelVal), nil
	}); err != nil {
		return err
	}

	// --- 9. Container Services (GKE & Artifact Registry) ---
	if err := mRead("Gcp", "gke_list_clusters", []string{"string"}, "StrList", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return makeStrList(reg, self.(*Host).GKEListClusters(a[0].Str))
	}); err != nil {
		return err
	}
	if err := mRead("Gcp", "gke_get_cluster", []string{"string", "string"}, "GkeCluster", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Obj("GkeCluster", self.(*Host).GKEGetCluster(a[0].Str, a[1].Str)), nil
	}); err != nil {
		return err
	}

	// GkeCluster methods
	if err := mSnapshot("GkeCluster", "name", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*GkeCluster).NameVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("GkeCluster", "location", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*GkeCluster).LocationVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("GkeCluster", "status", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*GkeCluster).StatusVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("GkeCluster", "k8s_version", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*GkeCluster).K8sVersionVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("GkeCluster", "release_channel", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*GkeCluster).ReleaseChannelVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("GkeCluster", "autopilot", nil, "bool", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(s.(*GkeCluster).AutopilotVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("GkeCluster", "private_nodes", nil, "bool", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(s.(*GkeCluster).PrivateNodesVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("GkeCluster", "workload_identity", nil, "bool", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(s.(*GkeCluster).WorkloadIdentityVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("GkeCluster", "network_policy", nil, "bool", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(s.(*GkeCluster).NetworkPolicyVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("GkeCluster", "binary_auth", nil, "bool", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(s.(*GkeCluster).BinaryAuthVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("GkeCluster", "node_pool_count", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*GkeCluster).NodePoolCountVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("GkeCluster", "total_nodes", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*GkeCluster).TotalNodesVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("GkeCluster", "labels", nil, "StrMap", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return makeStrMap(reg, s.(*GkeCluster).LabelsVal)
	}); err != nil {
		return err
	}

	// Artifact Registry
	if err := mRead("Gcp", "ar_list_repositories", []string{"string"}, "StrList", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return makeStrList(reg, self.(*Host).ARListRepositories(a[0].Str))
	}); err != nil {
		return err
	}
	if err := mRead("Gcp", "ar_get_repository", []string{"string", "string"}, "ArtifactRepo", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Obj("ArtifactRepo", self.(*Host).ARGetRepository(a[0].Str, a[1].Str)), nil
	}); err != nil {
		return err
	}
	if err := mPreview("Gcp", "ar_create_repository", []string{"string", "string", "string", "bool"}, "ArtifactRepo", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Obj("ArtifactRepo", self.(*Host).ARCreateRepository(a[0].Str, a[1].Str, a[2].Str, a[3].Bool)), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "ar_delete_repository", []string{"string", "string"}, "bool", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(self.(*Host).ARDeleteRepository(a[0].Str, a[1].Str)), nil
	}); err != nil {
		return err
	}

	// ArtifactRepo methods
	if err := mSnapshot("ArtifactRepo", "name", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*ArtifactRepo).NameVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("ArtifactRepo", "location", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*ArtifactRepo).LocationVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("ArtifactRepo", "format", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*ArtifactRepo).FormatVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("ArtifactRepo", "immutable_tags", nil, "bool", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(s.(*ArtifactRepo).ImmutableTagsVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("ArtifactRepo", "cleanup_policy_count", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*ArtifactRepo).CleanupPolicyCountVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("ArtifactRepo", "vulnerability_scanning", nil, "bool", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(s.(*ArtifactRepo).VulnerabilityScanningVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("ArtifactRepo", "size_mb", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*ArtifactRepo).SizeMBVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("ArtifactRepo", "package_count", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*ArtifactRepo).PackageCountVal), nil
	}); err != nil {
		return err
	}

	// --- 10. Cloud DNS ---
	if err := mRead("Gcp", "dns_list_zones", nil, "StrList", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return makeStrList(reg, self.(*Host).DNSListZones())
	}); err != nil {
		return err
	}
	if err := mRead("Gcp", "dns_get_zone", []string{"string"}, "DnsZone", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Obj("DnsZone", self.(*Host).DNSGetZone(a[0].Str)), nil
	}); err != nil {
		return err
	}
	if err := mPreview("Gcp", "dns_create_zone", []string{"string", "string", "string", "bool"}, "DnsZone", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Obj("DnsZone", self.(*Host).DNSCreateZone(a[0].Str, a[1].Str, a[2].Str, a[3].Bool)), nil
	}); err != nil {
		return err
	}
	if err := mRead("Gcp", "dns_list_records", []string{"string"}, "StrList", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return makeStrList(reg, self.(*Host).DNSListRecords(a[0].Str))
	}); err != nil {
		return err
	}
	if err := m("Gcp", "dns_add_record", []string{"string", "string", "string", "int", "string"}, "bool", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(self.(*Host).DNSAddRecord(a[0].Str, a[1].Str, a[2].Str, a[3].Int, a[4].Str)), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "dns_delete_record", []string{"string", "string", "string"}, "bool", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(self.(*Host).DNSDeleteRecord(a[0].Str, a[1].Str, a[2].Str)), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "dns_delete_zone", []string{"string"}, "bool", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(self.(*Host).DNSDeleteZone(a[0].Str)), nil
	}); err != nil {
		return err
	}

	// DnsZone methods
	if err := mSnapshot("DnsZone", "name", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*DnsZone).NameVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("DnsZone", "dns_name", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*DnsZone).DNSNameVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("DnsZone", "visibility", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*DnsZone).VisibilityVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("DnsZone", "dnssec", nil, "bool", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(s.(*DnsZone).DNSSECVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("DnsZone", "record_count", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*DnsZone).RecordCountVal), nil
	}); err != nil {
		return err
	}

	// --- 11. Cloud Logging & Cloud Monitoring ---
	if err := mRead("Gcp", "logging_list_sinks", nil, "StrList", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return makeStrList(reg, self.(*Host).LoggingListSinks())
	}); err != nil {
		return err
	}
	if err := mRead("Gcp", "logging_get_sink", []string{"string"}, "LogSink", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Obj("LogSink", self.(*Host).LoggingGetSink(a[0].Str)), nil
	}); err != nil {
		return err
	}
	if err := mPreview("Gcp", "logging_create_sink", []string{"string", "string", "string"}, "LogSink", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Obj("LogSink", self.(*Host).LoggingCreateSink(a[0].Str, a[1].Str, a[2].Str)), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "logging_delete_sink", []string{"string"}, "bool", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(self.(*Host).LoggingDeleteSink(a[0].Str)), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "logging_write_entry", []string{"string", "string", "string", "StrMap"}, "bool", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(self.(*Host).LoggingWriteEntry(a[0].Str, a[1].Str, a[2].Str, extractStrMap(a[3]))), nil
	}); err != nil {
		return err
	}
	if err := mRead("Gcp", "logging_severity_counts", nil, "StrMap", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return makeStrMap(reg, self.(*Host).LoggingSeverityCounts())
	}); err != nil {
		return err
	}

	// LogSink methods
	if err := mSnapshot("LogSink", "name", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*LogSink).NameVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("LogSink", "destination", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*LogSink).DestinationVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("LogSink", "filter", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*LogSink).FilterVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("LogSink", "disabled", nil, "bool", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(s.(*LogSink).DisabledVal), nil
	}); err != nil {
		return err
	}

	// Monitoring
	if err := mRead("Gcp", "monitoring_list_metrics", nil, "StrList", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return makeStrList(reg, self.(*Host).MonitoringListMetrics())
	}); err != nil {
		return err
	}
	if err := mRead("Gcp", "monitoring_query_series", []string{"string", "int"}, "FloatList", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return makeFloatList(reg, self.(*Host).MonitoringQuerySeries(a[0].Str, a[1].Int))
	}); err != nil {
		return err
	}
	if err := mRead("Gcp", "monitoring_list_alert_policies", nil, "StrList", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return makeStrList(reg, self.(*Host).MonitoringListAlertPolicies())
	}); err != nil {
		return err
	}
	if err := mRead("Gcp", "monitoring_get_alert_policy", []string{"string"}, "AlertPolicy", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Obj("AlertPolicy", self.(*Host).MonitoringGetAlertPolicy(a[0].Str)), nil
	}); err != nil {
		return err
	}
	if err := mPreview("Gcp", "monitoring_create_alert_policy", []string{"string", "string", "float", "int"}, "AlertPolicy", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Obj("AlertPolicy", self.(*Host).MonitoringCreateAlertPolicy(a[0].Str, a[1].Str, a[2].Float, a[3].Int)), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "monitoring_delete_alert_policy", []string{"string"}, "bool", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(self.(*Host).MonitoringDeleteAlertPolicy(a[0].Str)), nil
	}); err != nil {
		return err
	}

	// AlertPolicy methods
	if err := mSnapshot("AlertPolicy", "name", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*AlertPolicy).NameVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("AlertPolicy", "metric_type", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*AlertPolicy).MetricTypeVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("AlertPolicy", "threshold", nil, "float", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Float(s.(*AlertPolicy).ThresholdVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("AlertPolicy", "duration_sec", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*AlertPolicy).DurationSecVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("AlertPolicy", "enabled", nil, "bool", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(s.(*AlertPolicy).EnabledVal), nil
	}); err != nil {
		return err
	}
	if err := mSnapshot("AlertPolicy", "notification_channels", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*AlertPolicy).NotificationChannelsVal), nil
	}); err != nil {
		return err
	}

	// --- 12. Cross-Service Inventory, Compliance & Topology ---
	if err := mRead("Gcp", "inventory_summary", nil, "StrMap", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return makeStrMap(reg, self.(*Host).InventorySummary())
	}); err != nil {
		return err
	}
	if err := mRead("Gcp", "find_prefix", []string{"string"}, "StrList", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return makeStrList(reg, self.(*Host).FindPrefix(a[0].Str))
	}); err != nil {
		return err
	}
	if err := mRead("Gcp", "audit_label_compliance", []string{"StrList"}, "StrMap", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return makeStrMap(reg, self.(*Host).AuditLabelCompliance(extractStrList(a[0])))
	}); err != nil {
		return err
	}
	if err := mRead("Gcp", "resource_graph_edges", nil, "StrList", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return makeStrList(reg, self.(*Host).ResourceGraphEdges())
	}); err != nil {
		return err
	}

	return nil
}

// zeroHostObject returns a non-nil zero value for each schema object type so dry
// preview callbacks satisfy the SDK's concrete-receiver requirement.
func zeroHostObject(typeName string) any {
	switch typeName {
	case "ComputeInstance":
		return &ComputeInstance{}
	case "VpcNetwork":
		return &VpcNetwork{}
	case "Subnet":
		return &Subnet{}
	case "FirewallRule":
		return &FirewallRule{}
	case "StorageBucket":
		return &StorageBucket{}
	case "PubsubTopic":
		return &PubsubTopic{}
	case "PubsubSubscription":
		return &PubsubSubscription{}
	case "CloudRunService":
		return &CloudRunService{}
	case "SqlInstance":
		return &SqlInstance{}
	case "BqDataset":
		return &BqDataset{}
	case "BqTable":
		return &BqTable{}
	case "SecretMetadata":
		return &SecretMetadata{}
	case "KmsKey":
		return &KmsKey{}
	case "ArtifactRepo":
		return &ArtifactRepo{}
	case "DnsZone":
		return &DnsZone{}
	case "LogSink":
		return &LogSink{}
	case "AlertPolicy":
		return &AlertPolicy{}
	case "GkeCluster":
		return &GkeCluster{}
	case "ServiceAccount":
		return &ServiceAccount{}
	default:
		return nil
	}
}
