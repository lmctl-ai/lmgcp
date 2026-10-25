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

	// --- Core & Safety on Gcp ---
	if err := m("Gcp", "project", nil, "string", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(self.(*Host).Project()), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "masked_project", nil, "string", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(self.(*Host).MaskedProject()), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "set_project", []string{"string"}, "Gcp", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		hs := self.(*Host)
		hs.SetProject(a[0].Str)
		return lmcode.Obj("Gcp", hs), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "region", nil, "string", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(self.(*Host).Region()), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "set_region", []string{"string"}, "Gcp", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		hs := self.(*Host)
		hs.SetRegion(a[0].Str)
		return lmcode.Obj("Gcp", hs), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "zone", nil, "string", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(self.(*Host).Zone()), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "set_zone", []string{"string"}, "Gcp", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		hs := self.(*Host)
		hs.SetZone(a[0].Str)
		return lmcode.Obj("Gcp", hs), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "is_live", nil, "bool", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(self.(*Host).IsLive()), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "auth_mode", nil, "string", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(self.(*Host).AuthMode()), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "endpoint", nil, "string", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(self.(*Host).Endpoint()), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "run_id", nil, "string", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(self.(*Host).RunID()), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "set_run_id", []string{"string"}, "Gcp", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		hs := self.(*Host)
		hs.SetRunID(a[0].Str)
		return lmcode.Obj("Gcp", hs), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "last_error", nil, "string", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(self.(*Host).LastError()), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "redact", []string{"string"}, "string", func(_ any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(safety.RedactSecrets(a[0].Str)), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "set_auto_cleanup", []string{"bool"}, "Gcp", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		hs := self.(*Host)
		hs.SetAutoCleanup(a[0].Bool)
		return lmcode.Obj("Gcp", hs), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "ledger_count", nil, "int", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(self.(*Host).LedgerCount()), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "ledger_entries", nil, "StrList", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return makeStrList(reg, self.(*Host).LedgerEntries())
	}); err != nil {
		return err
	}
	if err := m("Gcp", "ledger_owns", []string{"string", "string"}, "bool", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(self.(*Host).LedgerOwns(a[0].Str, a[1].Str)), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "cost_cap_usd", nil, "float", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Float(self.(*Host).CostCapUSD()), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "set_cost_cap_usd", []string{"float"}, "bool", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(self.(*Host).SetCostCapUSD(a[0].Float)), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "estimated_cost_usd", nil, "float", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Float(self.(*Host).EstimatedCostUSD()), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "cost_report", nil, "string", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
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
	if err := m("Gcp", "sweep_dry_run", []string{"int"}, "StrList", func(self any, a []lmcode.Value) (lmcode.Value, error) {
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
	if err := m("Gcp", "compute_list_instances", []string{"string"}, "StrList", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return makeStrList(reg, self.(*Host).ComputeListInstances(a[0].Str))
	}); err != nil {
		return err
	}
	if err := m("Gcp", "compute_get_instance", []string{"string", "string"}, "ComputeInstance", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Obj("ComputeInstance", self.(*Host).ComputeGetInstance(a[0].Str, a[1].Str)), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "compute_instance_count", []string{"string"}, "int", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(int64(len(self.(*Host).ComputeListInstances(a[0].Str)))), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "compute_list_disks", []string{"string"}, "StrList", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return makeStrList(reg, self.(*Host).ComputeListDisks(a[0].Str))
	}); err != nil {
		return err
	}
	if err := m("Gcp", "compute_create_instance", []string{"string", "string", "string", "string"}, "ComputeInstance", func(self any, a []lmcode.Value) (lmcode.Value, error) {
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
	if err := m("ComputeInstance", "name", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*ComputeInstance).NameVal), nil
	}); err != nil {
		return err
	}
	if err := m("ComputeInstance", "zone", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*ComputeInstance).ZoneVal), nil
	}); err != nil {
		return err
	}
	if err := m("ComputeInstance", "machine_type", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*ComputeInstance).MachineTypeVal), nil
	}); err != nil {
		return err
	}
	if err := m("ComputeInstance", "status", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*ComputeInstance).StatusVal), nil
	}); err != nil {
		return err
	}
	if err := m("ComputeInstance", "network", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*ComputeInstance).NetworkVal), nil
	}); err != nil {
		return err
	}
	if err := m("ComputeInstance", "internal_ip", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*ComputeInstance).InternalIPVal), nil
	}); err != nil {
		return err
	}
	if err := m("ComputeInstance", "external_ip", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*ComputeInstance).ExternalIPVal), nil
	}); err != nil {
		return err
	}
	if err := m("ComputeInstance", "cpu_platform", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*ComputeInstance).CPUPlatformVal), nil
	}); err != nil {
		return err
	}
	if err := m("ComputeInstance", "disk_gb", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*ComputeInstance).DiskGBVal), nil
	}); err != nil {
		return err
	}
	if err := m("ComputeInstance", "vcpus", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*ComputeInstance).VCPUsVal), nil
	}); err != nil {
		return err
	}
	if err := m("ComputeInstance", "memory_mb", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*ComputeInstance).MemoryMBVal), nil
	}); err != nil {
		return err
	}
	if err := m("ComputeInstance", "labels", nil, "StrMap", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return makeStrMap(reg, s.(*ComputeInstance).LabelsVal)
	}); err != nil {
		return err
	}
	if err := m("ComputeInstance", "has_label", []string{"string"}, "bool", func(s any, a []lmcode.Value) (lmcode.Value, error) {
		_, ok := s.(*ComputeInstance).LabelsVal[a[0].Str]
		return lmcode.Bool(ok), nil
	}); err != nil {
		return err
	}
	if err := m("ComputeInstance", "label", []string{"string"}, "string", func(s any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*ComputeInstance).LabelsVal[a[0].Str]), nil
	}); err != nil {
		return err
	}
	if err := m("ComputeInstance", "service_account", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*ComputeInstance).ServiceAccountVal), nil
	}); err != nil {
		return err
	}
	if err := m("ComputeInstance", "shielded_vtpm", nil, "bool", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(s.(*ComputeInstance).ShieldedVTPMVal), nil
	}); err != nil {
		return err
	}
	if err := m("ComputeInstance", "deletion_protection", nil, "bool", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(s.(*ComputeInstance).DeletionProtectionVal), nil
	}); err != nil {
		return err
	}

	// --- 2. Network, Subnets & Firewalls ---
	if err := m("Gcp", "network_list_vpcs", nil, "StrList", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return makeStrList(reg, self.(*Host).NetworkListVPCs())
	}); err != nil {
		return err
	}
	if err := m("Gcp", "network_get_vpc", []string{"string"}, "VpcNetwork", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Obj("VpcNetwork", self.(*Host).NetworkGetVPC(a[0].Str)), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "network_create_vpc", []string{"string", "bool"}, "VpcNetwork", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Obj("VpcNetwork", self.(*Host).NetworkCreateVPC(a[0].Str, a[1].Bool)), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "network_delete_vpc", []string{"string"}, "bool", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(self.(*Host).NetworkDeleteVPC(a[0].Str)), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "network_list_subnets", []string{"string"}, "StrList", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return makeStrList(reg, self.(*Host).NetworkListSubnets(a[0].Str))
	}); err != nil {
		return err
	}
	if err := m("Gcp", "network_get_subnet", []string{"string", "string"}, "Subnet", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Obj("Subnet", self.(*Host).NetworkGetSubnet(a[0].Str, a[1].Str)), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "network_create_subnet", []string{"string", "string", "string", "string", "bool"}, "Subnet", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Obj("Subnet", self.(*Host).NetworkCreateSubnet(a[0].Str, a[1].Str, a[2].Str, a[3].Str, a[4].Bool)), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "network_delete_subnet", []string{"string", "string"}, "bool", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(self.(*Host).NetworkDeleteSubnet(a[0].Str, a[1].Str)), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "network_list_firewalls", nil, "StrList", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return makeStrList(reg, self.(*Host).NetworkListFirewalls())
	}); err != nil {
		return err
	}
	if err := m("Gcp", "network_get_firewall", []string{"string"}, "FirewallRule", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Obj("FirewallRule", self.(*Host).NetworkGetFirewall(a[0].Str)), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "network_create_firewall", []string{"string", "string", "string", "int", "string", "string", "string"}, "FirewallRule", func(self any, a []lmcode.Value) (lmcode.Value, error) {
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
	if err := m("VpcNetwork", "name", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*VpcNetwork).NameVal), nil
	}); err != nil {
		return err
	}
	if err := m("VpcNetwork", "auto_create_subnets", nil, "bool", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(s.(*VpcNetwork).AutoCreateSubnetsVal), nil
	}); err != nil {
		return err
	}
	if err := m("VpcNetwork", "routing_mode", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*VpcNetwork).RoutingModeVal), nil
	}); err != nil {
		return err
	}
	if err := m("VpcNetwork", "mtu", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*VpcNetwork).MTUVal), nil
	}); err != nil {
		return err
	}
	if err := m("VpcNetwork", "subnet_count", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*VpcNetwork).SubnetCountVal), nil
	}); err != nil {
		return err
	}

	// Subnet methods
	if err := m("Subnet", "name", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*Subnet).NameVal), nil
	}); err != nil {
		return err
	}
	if err := m("Subnet", "region", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*Subnet).RegionVal), nil
	}); err != nil {
		return err
	}
	if err := m("Subnet", "network", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*Subnet).NetworkVal), nil
	}); err != nil {
		return err
	}
	if err := m("Subnet", "cidr", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*Subnet).CIDRVal), nil
	}); err != nil {
		return err
	}
	if err := m("Subnet", "private_ip_google_access", nil, "bool", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(s.(*Subnet).PrivateIPGoogleAccessVal), nil
	}); err != nil {
		return err
	}
	if err := m("Subnet", "flow_logs", nil, "bool", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(s.(*Subnet).FlowLogsVal), nil
	}); err != nil {
		return err
	}

	// FirewallRule methods
	if err := m("FirewallRule", "name", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*FirewallRule).NameVal), nil
	}); err != nil {
		return err
	}
	if err := m("FirewallRule", "network", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*FirewallRule).NetworkVal), nil
	}); err != nil {
		return err
	}
	if err := m("FirewallRule", "direction", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*FirewallRule).DirectionVal), nil
	}); err != nil {
		return err
	}
	if err := m("FirewallRule", "priority", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*FirewallRule).PriorityVal), nil
	}); err != nil {
		return err
	}
	if err := m("FirewallRule", "action", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*FirewallRule).ActionVal), nil
	}); err != nil {
		return err
	}
	if err := m("FirewallRule", "protocol", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*FirewallRule).ProtocolVal), nil
	}); err != nil {
		return err
	}
	if err := m("FirewallRule", "ports", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*FirewallRule).PortsVal), nil
	}); err != nil {
		return err
	}
	if err := m("FirewallRule", "source_cidr", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*FirewallRule).SourceCIDRVal), nil
	}); err != nil {
		return err
	}
	if err := m("FirewallRule", "target_tags", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*FirewallRule).TargetTagsVal), nil
	}); err != nil {
		return err
	}
	if err := m("FirewallRule", "disabled", nil, "bool", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(s.(*FirewallRule).DisabledVal), nil
	}); err != nil {
		return err
	}
	if err := m("FirewallRule", "log_enabled", nil, "bool", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(s.(*FirewallRule).LogEnabledVal), nil
	}); err != nil {
		return err
	}
	if err := m("FirewallRule", "is_open_to_world", nil, "bool", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(s.(*FirewallRule).IsOpenToWorld()), nil
	}); err != nil {
		return err
	}
	if err := m("FirewallRule", "risk_score", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*FirewallRule).RiskScore()), nil
	}); err != nil {
		return err
	}

	// --- 3. Cloud Storage (GCS) ---
	if err := m("Gcp", "storage_list_buckets", nil, "StrList", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return makeStrList(reg, self.(*Host).StorageListBuckets())
	}); err != nil {
		return err
	}
	if err := m("Gcp", "storage_get_bucket", []string{"string"}, "StorageBucket", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Obj("StorageBucket", self.(*Host).StorageGetBucket(a[0].Str)), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "storage_create_bucket", []string{"string", "string", "string"}, "StorageBucket", func(self any, a []lmcode.Value) (lmcode.Value, error) {
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
	if err := m("Gcp", "storage_list_objects", []string{"string", "string"}, "StrList", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return makeStrList(reg, self.(*Host).StorageListObjects(a[0].Str, a[1].Str))
	}); err != nil {
		return err
	}
	if err := m("Gcp", "storage_read_object", []string{"string", "string"}, "string", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(self.(*Host).StorageReadObject(a[0].Str, a[1].Str)), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "storage_object_size", []string{"string", "string"}, "int", func(self any, a []lmcode.Value) (lmcode.Value, error) {
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
	if err := m("StorageBucket", "name", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*StorageBucket).NameVal), nil
	}); err != nil {
		return err
	}
	if err := m("StorageBucket", "location", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*StorageBucket).LocationVal), nil
	}); err != nil {
		return err
	}
	if err := m("StorageBucket", "storage_class", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*StorageBucket).StorageClassVal), nil
	}); err != nil {
		return err
	}
	if err := m("StorageBucket", "versioning", nil, "bool", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(s.(*StorageBucket).VersioningVal), nil
	}); err != nil {
		return err
	}
	if err := m("StorageBucket", "uniform_access", nil, "bool", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(s.(*StorageBucket).UniformAccessVal), nil
	}); err != nil {
		return err
	}
	if err := m("StorageBucket", "public_access_prevention", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*StorageBucket).PublicAccessPreventionVal), nil
	}); err != nil {
		return err
	}
	if err := m("StorageBucket", "lifecycle_delete_days", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*StorageBucket).LifecycleDeleteDaysVal), nil
	}); err != nil {
		return err
	}
	if err := m("StorageBucket", "lifecycle_nearline_days", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*StorageBucket).LifecycleNearlineDaysVal), nil
	}); err != nil {
		return err
	}
	if err := m("StorageBucket", "encryption_type", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*StorageBucket).EncryptionTypeVal), nil
	}); err != nil {
		return err
	}
	if err := m("StorageBucket", "object_count", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*StorageBucket).ObjectCountVal), nil
	}); err != nil {
		return err
	}
	if err := m("StorageBucket", "total_bytes", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*StorageBucket).TotalBytesVal), nil
	}); err != nil {
		return err
	}
	if err := m("StorageBucket", "labels", nil, "StrMap", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return makeStrMap(reg, s.(*StorageBucket).LabelsVal)
	}); err != nil {
		return err
	}
	if err := m("StorageBucket", "label", []string{"string"}, "string", func(s any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*StorageBucket).LabelsVal[a[0].Str]), nil
	}); err != nil {
		return err
	}

	// --- 4. Pub/Sub ---
	if err := m("Gcp", "pubsub_list_topics", nil, "StrList", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return makeStrList(reg, self.(*Host).PubsubListTopics())
	}); err != nil {
		return err
	}
	if err := m("Gcp", "pubsub_get_topic", []string{"string"}, "PubsubTopic", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Obj("PubsubTopic", self.(*Host).PubsubGetTopic(a[0].Str)), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "pubsub_create_topic", []string{"string"}, "PubsubTopic", func(self any, a []lmcode.Value) (lmcode.Value, error) {
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
	if err := m("Gcp", "pubsub_list_subscriptions", nil, "StrList", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return makeStrList(reg, self.(*Host).PubsubListSubscriptions())
	}); err != nil {
		return err
	}
	if err := m("Gcp", "pubsub_get_subscription", []string{"string"}, "PubsubSubscription", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Obj("PubsubSubscription", self.(*Host).PubsubGetSubscription(a[0].Str)), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "pubsub_create_subscription", []string{"string", "string", "int", "string"}, "PubsubSubscription", func(self any, a []lmcode.Value) (lmcode.Value, error) {
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
	if err := m("PubsubTopic", "name", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*PubsubTopic).NameVal), nil
	}); err != nil {
		return err
	}
	if err := m("PubsubTopic", "retention_sec", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*PubsubTopic).RetentionSecVal), nil
	}); err != nil {
		return err
	}
	if err := m("PubsubTopic", "published_count", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*PubsubTopic).PublishedCountVal), nil
	}); err != nil {
		return err
	}
	if err := m("PubsubTopic", "labels", nil, "StrMap", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return makeStrMap(reg, s.(*PubsubTopic).LabelsVal)
	}); err != nil {
		return err
	}
	if err := m("PubsubSubscription", "name", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*PubsubSubscription).NameVal), nil
	}); err != nil {
		return err
	}
	if err := m("PubsubSubscription", "topic", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*PubsubSubscription).TopicVal), nil
	}); err != nil {
		return err
	}
	if err := m("PubsubSubscription", "ack_deadline_sec", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*PubsubSubscription).AckDeadlineSecVal), nil
	}); err != nil {
		return err
	}
	if err := m("PubsubSubscription", "dead_letter_topic", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*PubsubSubscription).DeadLetterTopicVal), nil
	}); err != nil {
		return err
	}
	if err := m("PubsubSubscription", "max_delivery_attempts", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*PubsubSubscription).MaxDeliveryAttemptsVal), nil
	}); err != nil {
		return err
	}
	if err := m("PubsubSubscription", "backlog_count", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*PubsubSubscription).BacklogCountVal), nil
	}); err != nil {
		return err
	}
	if err := m("PubsubSubscription", "retain_acked", nil, "bool", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(s.(*PubsubSubscription).RetainAckedVal), nil
	}); err != nil {
		return err
	}

	// --- 5. Cloud Run v2 ---
	if err := m("Gcp", "run_list_services", []string{"string"}, "StrList", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return makeStrList(reg, self.(*Host).RunListServices(a[0].Str))
	}); err != nil {
		return err
	}
	if err := m("Gcp", "run_get_service", []string{"string", "string"}, "CloudRunService", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Obj("CloudRunService", self.(*Host).RunGetService(a[0].Str, a[1].Str)), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "run_deploy_service", []string{"string", "string", "string", "int", "int", "int"}, "CloudRunService", func(self any, a []lmcode.Value) (lmcode.Value, error) {
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
	if err := m("CloudRunService", "name", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*CloudRunService).NameVal), nil
	}); err != nil {
		return err
	}
	if err := m("CloudRunService", "region", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*CloudRunService).RegionVal), nil
	}); err != nil {
		return err
	}
	if err := m("CloudRunService", "image", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*CloudRunService).ImageVal), nil
	}); err != nil {
		return err
	}
	if err := m("CloudRunService", "uri", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*CloudRunService).URIVal), nil
	}); err != nil {
		return err
	}
	if err := m("CloudRunService", "cpu_milli", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*CloudRunService).CPUMilliVal), nil
	}); err != nil {
		return err
	}
	if err := m("CloudRunService", "memory_mb", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*CloudRunService).MemoryMBVal), nil
	}); err != nil {
		return err
	}
	if err := m("CloudRunService", "min_instances", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*CloudRunService).MinInstancesVal), nil
	}); err != nil {
		return err
	}
	if err := m("CloudRunService", "max_instances", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*CloudRunService).MaxInstancesVal), nil
	}); err != nil {
		return err
	}
	if err := m("CloudRunService", "concurrency", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*CloudRunService).ConcurrencyVal), nil
	}); err != nil {
		return err
	}
	if err := m("CloudRunService", "ingress", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*CloudRunService).IngressVal), nil
	}); err != nil {
		return err
	}
	if err := m("CloudRunService", "latest_revision", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*CloudRunService).LatestRevisionVal), nil
	}); err != nil {
		return err
	}
	if err := m("CloudRunService", "revision_count", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*CloudRunService).RevisionCountVal), nil
	}); err != nil {
		return err
	}
	if err := m("CloudRunService", "stable_traffic_pct", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*CloudRunService).StableTrafficPctVal), nil
	}); err != nil {
		return err
	}
	if err := m("CloudRunService", "canary_traffic_pct", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*CloudRunService).CanaryTrafficPctVal), nil
	}); err != nil {
		return err
	}
	if err := m("CloudRunService", "labels", nil, "StrMap", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return makeStrMap(reg, s.(*CloudRunService).LabelsVal)
	}); err != nil {
		return err
	}

	// --- 6. Databases & Analytics (SQL, Firestore, BigQuery) ---
	if err := m("Gcp", "sql_list_instances", nil, "StrList", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return makeStrList(reg, self.(*Host).SQLListInstances())
	}); err != nil {
		return err
	}
	if err := m("Gcp", "sql_get_instance", []string{"string"}, "SqlInstance", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Obj("SqlInstance", self.(*Host).SQLGetInstance(a[0].Str)), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "sql_create_instance", []string{"string", "string", "string", "string", "bool"}, "SqlInstance", func(self any, a []lmcode.Value) (lmcode.Value, error) {
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
	if err := m("SqlInstance", "name", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*SqlInstance).NameVal), nil
	}); err != nil {
		return err
	}
	if err := m("SqlInstance", "db_version", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*SqlInstance).DBVersionVal), nil
	}); err != nil {
		return err
	}
	if err := m("SqlInstance", "tier", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*SqlInstance).TierVal), nil
	}); err != nil {
		return err
	}
	if err := m("SqlInstance", "region", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*SqlInstance).RegionVal), nil
	}); err != nil {
		return err
	}
	if err := m("SqlInstance", "state", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*SqlInstance).StateVal), nil
	}); err != nil {
		return err
	}
	if err := m("SqlInstance", "ha_enabled", nil, "bool", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(s.(*SqlInstance).HAEnabledVal), nil
	}); err != nil {
		return err
	}
	if err := m("SqlInstance", "backup_enabled", nil, "bool", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(s.(*SqlInstance).BackupEnabledVal), nil
	}); err != nil {
		return err
	}
	if err := m("SqlInstance", "pitr_enabled", nil, "bool", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(s.(*SqlInstance).PITREnabledVal), nil
	}); err != nil {
		return err
	}
	if err := m("SqlInstance", "require_ssl", nil, "bool", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(s.(*SqlInstance).RequireSSLVal), nil
	}); err != nil {
		return err
	}
	if err := m("SqlInstance", "public_ip_enabled", nil, "bool", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(s.(*SqlInstance).PublicIPEnabledVal), nil
	}); err != nil {
		return err
	}
	if err := m("SqlInstance", "storage_gb", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*SqlInstance).StorageGBVal), nil
	}); err != nil {
		return err
	}
	if err := m("SqlInstance", "labels", nil, "StrMap", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return makeStrMap(reg, s.(*SqlInstance).LabelsVal)
	}); err != nil {
		return err
	}

	// Firestore
	if err := m("Gcp", "firestore_list_collections", nil, "StrList", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return makeStrList(reg, self.(*Host).FirestoreListCollections())
	}); err != nil {
		return err
	}
	if err := m("Gcp", "firestore_put_doc", []string{"string", "string", "StrMap"}, "bool", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(self.(*Host).FirestorePutDoc(a[0].Str, a[1].Str, extractStrMap(a[2]))), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "firestore_get_doc", []string{"string", "string"}, "StrMap", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return makeStrMap(reg, self.(*Host).FirestoreGetDoc(a[0].Str, a[1].Str))
	}); err != nil {
		return err
	}
	if err := m("Gcp", "firestore_list_docs", []string{"string"}, "StrList", func(self any, a []lmcode.Value) (lmcode.Value, error) {
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
	if err := m("Gcp", "bq_list_datasets", nil, "StrList", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return makeStrList(reg, self.(*Host).BQListDatasets())
	}); err != nil {
		return err
	}
	if err := m("Gcp", "bq_get_dataset", []string{"string"}, "BqDataset", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Obj("BqDataset", self.(*Host).BQGetDataset(a[0].Str)), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "bq_create_dataset", []string{"string", "string"}, "BqDataset", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Obj("BqDataset", self.(*Host).BQCreateDataset(a[0].Str, a[1].Str)), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "bq_create_table", []string{"string", "string", "string", "string"}, "BqTable", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Obj("BqTable", self.(*Host).BQCreateTable(a[0].Str, a[1].Str, a[2].Str, a[3].Str)), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "bq_get_table", []string{"string", "string"}, "BqTable", func(self any, a []lmcode.Value) (lmcode.Value, error) {
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
	if err := m("BqDataset", "dataset_id", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*BqDataset).DatasetIDVal), nil
	}); err != nil {
		return err
	}
	if err := m("BqDataset", "location", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*BqDataset).LocationVal), nil
	}); err != nil {
		return err
	}
	if err := m("BqDataset", "default_expiration_days", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*BqDataset).DefaultExpirationDaysVal), nil
	}); err != nil {
		return err
	}
	if err := m("BqDataset", "labels", nil, "StrMap", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return makeStrMap(reg, s.(*BqDataset).LabelsVal)
	}); err != nil {
		return err
	}
	if err := m("BqTable", "dataset_id", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*BqTable).DatasetIDVal), nil
	}); err != nil {
		return err
	}
	if err := m("BqTable", "table_id", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*BqTable).TableIDVal), nil
	}); err != nil {
		return err
	}
	if err := m("BqTable", "schema_spec", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*BqTable).SchemaSpecVal), nil
	}); err != nil {
		return err
	}
	if err := m("BqTable", "partition_field", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*BqTable).PartitionFieldVal), nil
	}); err != nil {
		return err
	}
	if err := m("BqTable", "row_count", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*BqTable).RowCountVal), nil
	}); err != nil {
		return err
	}
	if err := m("BqTable", "bytes", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*BqTable).BytesVal), nil
	}); err != nil {
		return err
	}

	// --- 7. IAM ---
	if err := m("Gcp", "iam_list_service_accounts", nil, "StrList", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return makeStrList(reg, self.(*Host).IAMListServiceAccounts())
	}); err != nil {
		return err
	}
	if err := m("Gcp", "iam_get_service_account", []string{"string"}, "ServiceAccount", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Obj("ServiceAccount", self.(*Host).IAMGetServiceAccount(a[0].Str)), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "iam_list_bindings", nil, "StrList", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return makeStrList(reg, self.(*Host).IAMListBindings())
	}); err != nil {
		return err
	}

	// ServiceAccount methods
	if err := m("ServiceAccount", "name", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*ServiceAccount).NameVal), nil
	}); err != nil {
		return err
	}
	if err := m("ServiceAccount", "email", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*ServiceAccount).EmailVal), nil
	}); err != nil {
		return err
	}
	if err := m("ServiceAccount", "display_name", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*ServiceAccount).DisplayNameVal), nil
	}); err != nil {
		return err
	}
	if err := m("ServiceAccount", "disabled", nil, "bool", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(s.(*ServiceAccount).DisabledVal), nil
	}); err != nil {
		return err
	}
	if err := m("ServiceAccount", "user_managed_keys", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*ServiceAccount).UserManagedKeysVal), nil
	}); err != nil {
		return err
	}
	if err := m("ServiceAccount", "key_max_age_days", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*ServiceAccount).KeyMaxAgeDaysVal), nil
	}); err != nil {
		return err
	}
	if err := m("ServiceAccount", "is_default_compute", nil, "bool", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(s.(*ServiceAccount).IsDefaultVal), nil
	}); err != nil {
		return err
	}
	if err := m("ServiceAccount", "roles", nil, "StrList", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return makeStrList(reg, s.(*ServiceAccount).RolesVal)
	}); err != nil {
		return err
	}

	// --- 8. Secret Manager Metadata & KMS ---
	if err := m("Gcp", "secrets_list", nil, "StrList", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return makeStrList(reg, self.(*Host).SecretsList())
	}); err != nil {
		return err
	}
	if err := m("Gcp", "secrets_get_metadata", []string{"string"}, "SecretMetadata", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Obj("SecretMetadata", self.(*Host).SecretsGetMetadata(a[0].Str)), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "secrets_create_metadata", []string{"string", "string", "int", "StrMap"}, "SecretMetadata", func(self any, a []lmcode.Value) (lmcode.Value, error) {
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
	if err := m("SecretMetadata", "name", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*SecretMetadata).NameVal), nil
	}); err != nil {
		return err
	}
	if err := m("SecretMetadata", "replication_policy", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*SecretMetadata).ReplicationPolicyVal), nil
	}); err != nil {
		return err
	}
	if err := m("SecretMetadata", "version_count", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*SecretMetadata).VersionCountVal), nil
	}); err != nil {
		return err
	}
	if err := m("SecretMetadata", "enabled_versions", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*SecretMetadata).EnabledVersionsVal), nil
	}); err != nil {
		return err
	}
	if err := m("SecretMetadata", "rotation_period_days", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*SecretMetadata).RotationDaysVal), nil
	}); err != nil {
		return err
	}
	if err := m("SecretMetadata", "cmek_enabled", nil, "bool", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(s.(*SecretMetadata).CMEKEnabledVal), nil
	}); err != nil {
		return err
	}
	if err := m("SecretMetadata", "has_expiration", nil, "bool", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(s.(*SecretMetadata).HasExpirationVal), nil
	}); err != nil {
		return err
	}
	if err := m("SecretMetadata", "labels", nil, "StrMap", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return makeStrMap(reg, s.(*SecretMetadata).LabelsVal)
	}); err != nil {
		return err
	}

	// KMS
	if err := m("Gcp", "kms_list_keyrings", []string{"string"}, "StrList", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return makeStrList(reg, self.(*Host).KMSListKeyRings(a[0].Str))
	}); err != nil {
		return err
	}
	if err := m("Gcp", "kms_list_keys", []string{"string", "string"}, "StrList", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return makeStrList(reg, self.(*Host).KMSListKeys(a[0].Str, a[1].Str))
	}); err != nil {
		return err
	}
	if err := m("Gcp", "kms_get_key", []string{"string", "string", "string"}, "KmsKey", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Obj("KmsKey", self.(*Host).KMSGetKey(a[0].Str, a[1].Str, a[2].Str)), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "kms_create_key", []string{"string", "string", "string", "string", "int"}, "KmsKey", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Obj("KmsKey", self.(*Host).KMSCreateKey(a[0].Str, a[1].Str, a[2].Str, a[3].Str, a[4].Int)), nil
	}); err != nil {
		return err
	}

	// KmsKey methods
	if err := m("KmsKey", "name", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*KmsKey).NameVal), nil
	}); err != nil {
		return err
	}
	if err := m("KmsKey", "keyring", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*KmsKey).KeyRingVal), nil
	}); err != nil {
		return err
	}
	if err := m("KmsKey", "location", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*KmsKey).LocationVal), nil
	}); err != nil {
		return err
	}
	if err := m("KmsKey", "purpose", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*KmsKey).PurposeVal), nil
	}); err != nil {
		return err
	}
	if err := m("KmsKey", "rotation_days", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*KmsKey).RotationDaysVal), nil
	}); err != nil {
		return err
	}
	if err := m("KmsKey", "primary_state", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*KmsKey).PrimaryStateVal), nil
	}); err != nil {
		return err
	}
	if err := m("KmsKey", "protection_level", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*KmsKey).ProtectionLevelVal), nil
	}); err != nil {
		return err
	}

	// --- 9. Container Services (GKE & Artifact Registry) ---
	if err := m("Gcp", "gke_list_clusters", []string{"string"}, "StrList", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return makeStrList(reg, self.(*Host).GKEListClusters(a[0].Str))
	}); err != nil {
		return err
	}
	if err := m("Gcp", "gke_get_cluster", []string{"string", "string"}, "GkeCluster", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Obj("GkeCluster", self.(*Host).GKEGetCluster(a[0].Str, a[1].Str)), nil
	}); err != nil {
		return err
	}

	// GkeCluster methods
	if err := m("GkeCluster", "name", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*GkeCluster).NameVal), nil
	}); err != nil {
		return err
	}
	if err := m("GkeCluster", "location", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*GkeCluster).LocationVal), nil
	}); err != nil {
		return err
	}
	if err := m("GkeCluster", "status", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*GkeCluster).StatusVal), nil
	}); err != nil {
		return err
	}
	if err := m("GkeCluster", "k8s_version", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*GkeCluster).K8sVersionVal), nil
	}); err != nil {
		return err
	}
	if err := m("GkeCluster", "release_channel", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*GkeCluster).ReleaseChannelVal), nil
	}); err != nil {
		return err
	}
	if err := m("GkeCluster", "autopilot", nil, "bool", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(s.(*GkeCluster).AutopilotVal), nil
	}); err != nil {
		return err
	}
	if err := m("GkeCluster", "private_nodes", nil, "bool", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(s.(*GkeCluster).PrivateNodesVal), nil
	}); err != nil {
		return err
	}
	if err := m("GkeCluster", "workload_identity", nil, "bool", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(s.(*GkeCluster).WorkloadIdentityVal), nil
	}); err != nil {
		return err
	}
	if err := m("GkeCluster", "network_policy", nil, "bool", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(s.(*GkeCluster).NetworkPolicyVal), nil
	}); err != nil {
		return err
	}
	if err := m("GkeCluster", "binary_auth", nil, "bool", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(s.(*GkeCluster).BinaryAuthVal), nil
	}); err != nil {
		return err
	}
	if err := m("GkeCluster", "node_pool_count", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*GkeCluster).NodePoolCountVal), nil
	}); err != nil {
		return err
	}
	if err := m("GkeCluster", "total_nodes", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*GkeCluster).TotalNodesVal), nil
	}); err != nil {
		return err
	}
	if err := m("GkeCluster", "labels", nil, "StrMap", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return makeStrMap(reg, s.(*GkeCluster).LabelsVal)
	}); err != nil {
		return err
	}

	// Artifact Registry
	if err := m("Gcp", "ar_list_repositories", []string{"string"}, "StrList", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return makeStrList(reg, self.(*Host).ARListRepositories(a[0].Str))
	}); err != nil {
		return err
	}
	if err := m("Gcp", "ar_get_repository", []string{"string", "string"}, "ArtifactRepo", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Obj("ArtifactRepo", self.(*Host).ARGetRepository(a[0].Str, a[1].Str)), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "ar_create_repository", []string{"string", "string", "string", "bool"}, "ArtifactRepo", func(self any, a []lmcode.Value) (lmcode.Value, error) {
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
	if err := m("ArtifactRepo", "name", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*ArtifactRepo).NameVal), nil
	}); err != nil {
		return err
	}
	if err := m("ArtifactRepo", "location", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*ArtifactRepo).LocationVal), nil
	}); err != nil {
		return err
	}
	if err := m("ArtifactRepo", "format", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*ArtifactRepo).FormatVal), nil
	}); err != nil {
		return err
	}
	if err := m("ArtifactRepo", "immutable_tags", nil, "bool", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(s.(*ArtifactRepo).ImmutableTagsVal), nil
	}); err != nil {
		return err
	}
	if err := m("ArtifactRepo", "cleanup_policy_count", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*ArtifactRepo).CleanupPolicyCountVal), nil
	}); err != nil {
		return err
	}
	if err := m("ArtifactRepo", "vulnerability_scanning", nil, "bool", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(s.(*ArtifactRepo).VulnerabilityScanningVal), nil
	}); err != nil {
		return err
	}
	if err := m("ArtifactRepo", "size_mb", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*ArtifactRepo).SizeMBVal), nil
	}); err != nil {
		return err
	}
	if err := m("ArtifactRepo", "package_count", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*ArtifactRepo).PackageCountVal), nil
	}); err != nil {
		return err
	}

	// --- 10. Cloud DNS ---
	if err := m("Gcp", "dns_list_zones", nil, "StrList", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return makeStrList(reg, self.(*Host).DNSListZones())
	}); err != nil {
		return err
	}
	if err := m("Gcp", "dns_get_zone", []string{"string"}, "DnsZone", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Obj("DnsZone", self.(*Host).DNSGetZone(a[0].Str)), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "dns_create_zone", []string{"string", "string", "string", "bool"}, "DnsZone", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Obj("DnsZone", self.(*Host).DNSCreateZone(a[0].Str, a[1].Str, a[2].Str, a[3].Bool)), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "dns_list_records", []string{"string"}, "StrList", func(self any, a []lmcode.Value) (lmcode.Value, error) {
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
	if err := m("DnsZone", "name", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*DnsZone).NameVal), nil
	}); err != nil {
		return err
	}
	if err := m("DnsZone", "dns_name", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*DnsZone).DNSNameVal), nil
	}); err != nil {
		return err
	}
	if err := m("DnsZone", "visibility", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*DnsZone).VisibilityVal), nil
	}); err != nil {
		return err
	}
	if err := m("DnsZone", "dnssec", nil, "bool", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(s.(*DnsZone).DNSSECVal), nil
	}); err != nil {
		return err
	}
	if err := m("DnsZone", "record_count", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*DnsZone).RecordCountVal), nil
	}); err != nil {
		return err
	}

	// --- 11. Cloud Logging & Cloud Monitoring ---
	if err := m("Gcp", "logging_list_sinks", nil, "StrList", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return makeStrList(reg, self.(*Host).LoggingListSinks())
	}); err != nil {
		return err
	}
	if err := m("Gcp", "logging_get_sink", []string{"string"}, "LogSink", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Obj("LogSink", self.(*Host).LoggingGetSink(a[0].Str)), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "logging_create_sink", []string{"string", "string", "string"}, "LogSink", func(self any, a []lmcode.Value) (lmcode.Value, error) {
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
	if err := m("Gcp", "logging_severity_counts", nil, "StrMap", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return makeStrMap(reg, self.(*Host).LoggingSeverityCounts())
	}); err != nil {
		return err
	}

	// LogSink methods
	if err := m("LogSink", "name", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*LogSink).NameVal), nil
	}); err != nil {
		return err
	}
	if err := m("LogSink", "destination", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*LogSink).DestinationVal), nil
	}); err != nil {
		return err
	}
	if err := m("LogSink", "filter", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*LogSink).FilterVal), nil
	}); err != nil {
		return err
	}
	if err := m("LogSink", "disabled", nil, "bool", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(s.(*LogSink).DisabledVal), nil
	}); err != nil {
		return err
	}

	// Monitoring
	if err := m("Gcp", "monitoring_list_metrics", nil, "StrList", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return makeStrList(reg, self.(*Host).MonitoringListMetrics())
	}); err != nil {
		return err
	}
	if err := m("Gcp", "monitoring_query_series", []string{"string", "int"}, "FloatList", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return makeFloatList(reg, self.(*Host).MonitoringQuerySeries(a[0].Str, a[1].Int))
	}); err != nil {
		return err
	}
	if err := m("Gcp", "monitoring_list_alert_policies", nil, "StrList", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return makeStrList(reg, self.(*Host).MonitoringListAlertPolicies())
	}); err != nil {
		return err
	}
	if err := m("Gcp", "monitoring_get_alert_policy", []string{"string"}, "AlertPolicy", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Obj("AlertPolicy", self.(*Host).MonitoringGetAlertPolicy(a[0].Str)), nil
	}); err != nil {
		return err
	}
	if err := m("Gcp", "monitoring_create_alert_policy", []string{"string", "string", "float", "int"}, "AlertPolicy", func(self any, a []lmcode.Value) (lmcode.Value, error) {
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
	if err := m("AlertPolicy", "name", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*AlertPolicy).NameVal), nil
	}); err != nil {
		return err
	}
	if err := m("AlertPolicy", "metric_type", nil, "string", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Str(s.(*AlertPolicy).MetricTypeVal), nil
	}); err != nil {
		return err
	}
	if err := m("AlertPolicy", "threshold", nil, "float", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Float(s.(*AlertPolicy).ThresholdVal), nil
	}); err != nil {
		return err
	}
	if err := m("AlertPolicy", "duration_sec", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*AlertPolicy).DurationSecVal), nil
	}); err != nil {
		return err
	}
	if err := m("AlertPolicy", "enabled", nil, "bool", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Bool(s.(*AlertPolicy).EnabledVal), nil
	}); err != nil {
		return err
	}
	if err := m("AlertPolicy", "notification_channels", nil, "int", func(s any, _ []lmcode.Value) (lmcode.Value, error) {
		return lmcode.Int(s.(*AlertPolicy).NotificationChannelsVal), nil
	}); err != nil {
		return err
	}

	// --- 12. Cross-Service Inventory, Compliance & Topology ---
	if err := m("Gcp", "inventory_summary", nil, "StrMap", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return makeStrMap(reg, self.(*Host).InventorySummary())
	}); err != nil {
		return err
	}
	if err := m("Gcp", "audit_label_compliance", []string{"StrList"}, "StrMap", func(self any, a []lmcode.Value) (lmcode.Value, error) {
		return makeStrMap(reg, self.(*Host).AuditLabelCompliance(extractStrList(a[0])))
	}); err != nil {
		return err
	}
	if err := m("Gcp", "resource_graph_edges", nil, "StrList", func(self any, _ []lmcode.Value) (lmcode.Value, error) {
		return makeStrList(reg, self.(*Host).ResourceGraphEdges())
	}); err != nil {
		return err
	}

	return nil
}
