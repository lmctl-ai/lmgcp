// SPDX-License-Identifier: MIT

package fakegcp

import (
	"fmt"

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

const DefaultEpoch int64 = 1761350400 // 2025-10-25T00:00:00Z deterministic baseline

func (s *Server) seedBaseline() {
	// 1. VPC Networks
	s.vpcs["prod-vpc"] = &compute.Network{
		Name:                  "prod-vpc",
		AutoCreateSubnetworks: false,
		Mtu:                   1460,
		RoutingConfig:         &compute.NetworkRoutingConfig{RoutingMode: "GLOBAL"},
	}
	s.vpcs["staging-vpc"] = &compute.Network{
		Name:                  "staging-vpc",
		AutoCreateSubnetworks: false,
		Mtu:                   1460,
		RoutingConfig:         &compute.NetworkRoutingConfig{RoutingMode: "REGIONAL"},
	}
	s.vpcs["analytics-vpc"] = &compute.Network{
		Name:                  "analytics-vpc",
		AutoCreateSubnetworks: false,
		Mtu:                   1500,
		RoutingConfig:         &compute.NetworkRoutingConfig{RoutingMode: "GLOBAL"},
	}

	// 2. Subnets
	s.subnets["us-central1/prod-app-subnet"] = &compute.Subnetwork{
		Name:                  "prod-app-subnet",
		Region:                "us-central1",
		Network:               "prod-vpc",
		IpCidrRange:           "10.10.0.0/20",
		PrivateIpGoogleAccess: true,
		EnableFlowLogs:        true,
	}
	s.subnets["us-central1/prod-db-subnet"] = &compute.Subnetwork{
		Name:                  "prod-db-subnet",
		Region:                "us-central1",
		Network:               "prod-vpc",
		IpCidrRange:           "10.10.16.0/22",
		PrivateIpGoogleAccess: true,
		EnableFlowLogs:        true,
	}
	s.subnets["us-east1/prod-dr-subnet"] = &compute.Subnetwork{
		Name:                  "prod-dr-subnet",
		Region:                "us-east1",
		Network:               "prod-vpc",
		IpCidrRange:           "10.20.0.0/21",
		PrivateIpGoogleAccess: true,
		EnableFlowLogs:        true,
	}
	s.subnets["us-central1/staging-app-subnet"] = &compute.Subnetwork{
		Name:                  "staging-app-subnet",
		Region:                "us-central1",
		Network:               "staging-vpc",
		IpCidrRange:           "10.30.0.0/22",
		PrivateIpGoogleAccess: true,
		EnableFlowLogs:        false,
	}
	s.subnets["us-west1/analytics-batch-subnet"] = &compute.Subnetwork{
		Name:                  "analytics-batch-subnet",
		Region:                "us-west1",
		Network:               "analytics-vpc",
		IpCidrRange:           "10.40.0.0/19",
		PrivateIpGoogleAccess: false,
		EnableFlowLogs:        false,
	}

	// 3. Firewall Rules (including compliant & risky open-to-world rules for security review)
	s.firewalls["prod-allow-internal"] = &compute.Firewall{
		Name:         "prod-allow-internal",
		Network:      "prod-vpc",
		Direction:    "INGRESS",
		Priority:     1000,
		SourceRanges: []string{"10.10.0.0/16"},
		TargetTags:   []string{"internal-tier"},
		Allowed:      []*compute.FirewallAllowed{{IPProtocol: "tcp", Ports: []string{"8080", "8443"}}},
		LogConfig:    &compute.FirewallLogConfig{Enable: true},
	}
	s.firewalls["prod-allow-lb-healthcheck"] = &compute.Firewall{
		Name:         "prod-allow-lb-healthcheck",
		Network:      "prod-vpc",
		Direction:    "INGRESS",
		Priority:     900,
		SourceRanges: []string{"35.191.0.0/16", "130.211.0.0/22"},
		TargetTags:   []string{"web-frontend"},
		Allowed:      []*compute.FirewallAllowed{{IPProtocol: "tcp", Ports: []string{"443"}}},
		LogConfig:    &compute.FirewallLogConfig{Enable: true},
	}
	s.firewalls["prod-allow-iap-ssh"] = &compute.Firewall{
		Name:         "prod-allow-iap-ssh",
		Network:      "prod-vpc",
		Direction:    "INGRESS",
		Priority:     950,
		SourceRanges: []string{"35.235.240.0/20"},
		TargetTags:   []string{"bastion"},
		Allowed:      []*compute.FirewallAllowed{{IPProtocol: "tcp", Ports: []string{"22"}}},
		LogConfig:    &compute.FirewallLogConfig{Enable: true},
	}
	s.firewalls["legacy-open-ssh-world"] = &compute.Firewall{
		Name:         "legacy-open-ssh-world",
		Network:      "staging-vpc",
		Direction:    "INGRESS",
		Priority:     1000,
		SourceRanges: []string{"0.0.0.0/0"},
		TargetTags:   []string{"staging-vm"},
		Allowed:      []*compute.FirewallAllowed{{IPProtocol: "tcp", Ports: []string{"22"}}},
		LogConfig:    &compute.FirewallLogConfig{Enable: false},
	}
	s.firewalls["legacy-open-rdp-world"] = &compute.Firewall{
		Name:         "legacy-open-rdp-world",
		Network:      "staging-vpc",
		Direction:    "INGRESS",
		Priority:     1000,
		SourceRanges: []string{"0.0.0.0/0"},
		TargetTags:   []string{"win-jump"},
		Allowed:      []*compute.FirewallAllowed{{IPProtocol: "tcp", Ports: []string{"3389"}}},
		LogConfig:    &compute.FirewallLogConfig{Enable: false},
	}
	s.firewalls["analytics-open-postgres"] = &compute.Firewall{
		Name:         "analytics-open-postgres",
		Network:      "analytics-vpc",
		Direction:    "INGRESS",
		Priority:     1100,
		SourceRanges: []string{"0.0.0.0/0"},
		TargetTags:   []string{"db-replica"},
		Allowed:      []*compute.FirewallAllowed{{IPProtocol: "tcp", Ports: []string{"5432"}}},
		LogConfig:    &compute.FirewallLogConfig{Enable: false},
	}

	// 4. Compute Engine Instances across zones
	addInst := func(zone, name, mtype, status, vpc, ip, extIP, sa string, diskGB int64, vtpm, delProt bool, labels map[string]string) {
		var nis []*compute.NetworkInterface
		ni := &compute.NetworkInterface{
			Network:   vpc,
			NetworkIP: ip,
		}
		if extIP != "" {
			ni.AccessConfigs = []*compute.AccessConfig{{NatIP: extIP, Name: "External NAT"}}
		}
		nis = append(nis, ni)
		s.instances[zone+"/"+name] = &compute.Instance{
			Name:               name,
			Zone:               zone,
			MachineType:        mtype,
			Status:             status,
			CpuPlatform:        "Intel Ice Lake",
			DeletionProtection: delProt,
			Labels:             labels,
			NetworkInterfaces:  nis,
			Disks: []*compute.AttachedDisk{
				{Boot: true, DiskSizeGb: diskGB, Source: name + "-boot"},
			},
			ServiceAccounts: []*compute.ServiceAccount{
				{Email: sa, Scopes: []string{"https://www.googleapis.com/auth/cloud-platform"}},
			},
			ShieldedInstanceConfig: &compute.ShieldedInstanceConfig{
				EnableVtpm:                vtpm,
				EnableIntegrityMonitoring: vtpm,
			},
		}
		s.disks[zone+"/"+name+"-boot"] = &compute.Disk{
			Name:   name + "-boot",
			Zone:   zone,
			SizeGb: diskGB,
			Status: "READY",
			Type:   "pd-balanced",
		}
	}

	addInst("us-central1-a", "prod-api-gw-01", "e2-standard-4", "RUNNING", "prod-vpc", "10.10.1.11", "",
		"sa-prod-api@gcp-proj-masked.iam.gserviceaccount.com", 50, true, true,
		map[string]string{"env": "prod", "owner": "platform", "cost_center": "cc-101", "data_class": "confidential"})
	addInst("us-central1-b", "prod-api-gw-02", "e2-standard-4", "RUNNING", "prod-vpc", "10.10.1.12", "",
		"sa-prod-api@gcp-proj-masked.iam.gserviceaccount.com", 50, true, true,
		map[string]string{"env": "prod", "owner": "platform", "cost_center": "cc-101", "data_class": "confidential"})
	addInst("us-central1-a", "prod-worker-pool-01", "n2-standard-8", "RUNNING", "prod-vpc", "10.10.2.21", "",
		"sa-prod-worker@gcp-proj-masked.iam.gserviceaccount.com", 100, true, true,
		map[string]string{"env": "prod", "owner": "fulfillment", "cost_center": "cc-104", "data_class": "internal"})
	addInst("us-central1-c", "prod-cache-node-01", "e2-medium", "RUNNING", "prod-vpc", "10.10.3.15", "",
		"sa-prod-api@gcp-proj-masked.iam.gserviceaccount.com", 30, true, false,
		map[string]string{"env": "prod", "owner": "platform", "cost_center": "cc-101"}) // missing data_class
	addInst("us-central1-a", "staging-test-runner-01", "e2-standard-2", "RUNNING", "staging-vpc", "10.30.1.10", "34.122.40.19",
		"000000000000-compute@developer.gserviceaccount.com", 40, false, false,
		map[string]string{"env": "staging", "owner": "qa"}) // missing cost_center, data_class
	addInst("us-west1-a", "analytics-etl-node-01", "n2-highmem-4", "RUNNING", "analytics-vpc", "10.40.1.50", "",
		"sa-analytics-etl@gcp-proj-masked.iam.gserviceaccount.com", 200, true, false,
		map[string]string{"env": "prod", "owner": "data-eng", "cost_center": "cc-205", "data_class": "restricted"})
	addInst("us-east1-b", "dr-standby-api-01", "e2-small", "TERMINATED", "prod-vpc", "10.20.1.11", "",
		"sa-prod-api@gcp-proj-masked.iam.gserviceaccount.com", 50, true, true,
		map[string]string{"env": "dr", "owner": "platform", "cost_center": "cc-101", "data_class": "confidential"})

	// 5. Cloud Storage Buckets & Objects
	addBkt := func(name, loc, class string, ver, ubla bool, pap string, delDays, nearDays int64, kms string, objCount, totalBytes int64, labels map[string]string) {
		var rules []*storage.BucketLifecycleRule
		if delDays > 0 {
			d := delDays
			rules = append(rules, &storage.BucketLifecycleRule{
				Action:    &storage.BucketLifecycleRuleAction{Type: "Delete"},
				Condition: &storage.BucketLifecycleRuleCondition{Age: &d},
			})
		}
		if nearDays > 0 {
			n := nearDays
			rules = append(rules, &storage.BucketLifecycleRule{
				Action:    &storage.BucketLifecycleRuleAction{Type: "SetStorageClass", StorageClass: "NEARLINE"},
				Condition: &storage.BucketLifecycleRuleCondition{Age: &n},
			})
		}
		b := &storage.Bucket{
			Name:         name,
			Location:     loc,
			StorageClass: class,
			Labels:       labels,
			Versioning:   &storage.BucketVersioning{Enabled: ver},
			IamConfiguration: &storage.BucketIamConfiguration{
				UniformBucketLevelAccess: &storage.BucketIamConfigurationUniformBucketLevelAccess{Enabled: ubla},
				PublicAccessPrevention:   pap,
			},
			Lifecycle: &storage.BucketLifecycle{Rule: rules},
		}
		if kms != "" {
			b.Encryption = &storage.BucketEncryption{DefaultKmsKeyName: kms}
		}
		s.buckets[name] = b
		s.bucketStats[name] = bucketStat{ObjectCount: objCount, TotalBytes: totalBytes}
		s.objects[name] = make(map[string]storedObject)
	}

	addBkt("prod-customer-invoices-us", "US-CENTRAL1", "STANDARD", true, true, "enforced", 365, 30,
		"projects/gcp-proj-masked/locations/us-central1/keyRings/prod-ring/cryptoKeys/storage-cmek",
		1840, 482000000,
		map[string]string{"env": "prod", "owner": "finance", "cost_center": "cc-102", "data_class": "restricted"})
	addBkt("prod-telemetry-parquet-us", "US", "NEARLINE", true, true, "enforced", 180, 0,
		"", 9420, 2150000000,
		map[string]string{"env": "prod", "owner": "data-eng", "cost_center": "cc-205", "data_class": "internal"})
	addBkt("prod-compliance-ledger-archive", "US-EAST1", "ARCHIVE", true, true, "enforced", 2555, 90,
		"projects/gcp-proj-masked/locations/us-east1/keyRings/dr-ring/cryptoKeys/archive-cmek",
		640, 890000000,
		map[string]string{"env": "prod", "owner": "secops", "cost_center": "cc-301", "data_class": "restricted"})
	addBkt("staging-build-artifacts-c1", "US-CENTRAL1", "STANDARD", false, false, "inherited", 0, 0,
		"", 310, 95000000,
		map[string]string{"env": "staging", "owner": "platform"})
	addBkt("coldline-db-snapshots-us", "US-CENTRAL1", "COLDLINE", true, true, "enforced", 730, 0,
		"", 120, 620000000,
		map[string]string{"env": "prod", "owner": "dba", "cost_center": "cc-108", "data_class": "confidential"})

	// 6. Pub/Sub Topics & Subscriptions
	s.topics["order-events-v1"] = &pubsub.Topic{
		Name:                     "projects/gcp-proj-masked/topics/order-events-v1",
		MessageRetentionDuration: "604800s",
		Labels:                   map[string]string{"env": "prod", "owner": "fulfillment", "cost_center": "cc-104", "data_class": "internal"},
	}
	s.topicStats["order-events-v1"] = 14820
	s.topics["order-events-dlq"] = &pubsub.Topic{
		Name:                     "projects/gcp-proj-masked/topics/order-events-dlq",
		MessageRetentionDuration: "1209600s",
		Labels:                   map[string]string{"env": "prod", "owner": "fulfillment", "cost_center": "cc-104", "data_class": "internal"},
	}
	s.topicStats["order-events-dlq"] = 27
	s.topics["security-audit-stream"] = &pubsub.Topic{
		Name:                     "projects/gcp-proj-masked/topics/security-audit-stream",
		MessageRetentionDuration: "604800s",
		Labels:                   map[string]string{"env": "prod", "owner": "secops", "cost_center": "cc-301", "data_class": "restricted"},
	}
	s.topicStats["security-audit-stream"] = 52400

	s.subscriptions["order-worker-sub"] = &pubsub.Subscription{
		Name:               "projects/gcp-proj-masked/subscriptions/order-worker-sub",
		Topic:              "projects/gcp-proj-masked/topics/order-events-v1",
		AckDeadlineSeconds: 30,
		RetainAckedMessages: false,
		DeadLetterPolicy: &pubsub.DeadLetterPolicy{
			DeadLetterTopic:     "projects/gcp-proj-masked/topics/order-events-dlq",
			MaxDeliveryAttempts: 5,
		},
	}
	s.subBacklog["order-worker-sub"] = 14
	s.subscriptions["security-siem-sub"] = &pubsub.Subscription{
		Name:                "projects/gcp-proj-masked/subscriptions/security-siem-sub",
		Topic:               "projects/gcp-proj-masked/topics/security-audit-stream",
		AckDeadlineSeconds:  60,
		RetainAckedMessages: true,
	}
	s.subBacklog["security-siem-sub"] = 3

	// 7. Cloud Run Services
	addRun := func(region, name, image, ingress string, cpuMilli, memMB, minInst, maxInst, conc, revCount, stablePct, canaryPct int64, labels map[string]string) {
		key := region + "/" + name
		s.runServices[key] = &run.GoogleCloudRunV2Service{
			Name:           fmt.Sprintf("projects/gcp-proj-masked/locations/%s/services/%s", region, name),
			Uri:            fmt.Sprintf("https://%s-masked-%s.a.run.app", name, region),
			Ingress:        ingress,
			LatestReadyRevision: fmt.Sprintf("%s-rev-%03d", name, revCount),
			Labels:         labels,
			Template: &run.GoogleCloudRunV2RevisionTemplate{
				MaxInstanceRequestConcurrency: conc,
				Scaling: &run.GoogleCloudRunV2RevisionScaling{
					MinInstanceCount: minInst,
					MaxInstanceCount: maxInst,
				},
				Containers: []*run.GoogleCloudRunV2Container{
					{
						Image: image,
						Resources: &run.GoogleCloudRunV2ResourceRequirements{
							Limits: map[string]string{
								"cpu":    fmt.Sprintf("%dm", cpuMilli),
								"memory": fmt.Sprintf("%dMi", memMB),
							},
						},
					},
				},
			},
			Traffic: []*run.GoogleCloudRunV2TrafficTarget{
				{Type: "TRAFFIC_TARGET_ALLOCATION_TYPE_REVISION", Revision: fmt.Sprintf("%s-rev-%03d", name, revCount-1), Percent: stablePct},
				{Type: "TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST", Revision: fmt.Sprintf("%s-rev-%03d", name, revCount), Percent: canaryPct},
			},
		}
		s.runMeta[key] = runMetaInfo{
			CPUMilli:      cpuMilli,
			MemoryMB:      memMB,
			RevisionCount: revCount,
			StablePct:     stablePct,
			CanaryPct:     canaryPct,
		}
	}

	addRun("us-central1", "payments-authorizer", "us-central1-docker.pkg.dev/gcp-proj-masked/prod-containers/payments:v2.4.1",
		"INGRESS_TRAFFIC_INTERNAL_LOAD_BALANCER", 2000, 1024, 2, 25, 80, 14, 90, 10,
		map[string]string{"env": "prod", "owner": "finance", "cost_center": "cc-102", "data_class": "restricted"})
	addRun("us-central1", "catalog-search-api", "us-central1-docker.pkg.dev/gcp-proj-masked/prod-containers/catalog:v1.9.0",
		"INGRESS_TRAFFIC_ALL", 1000, 512, 1, 15, 100, 9, 100, 0,
		map[string]string{"env": "prod", "owner": "catalog", "cost_center": "cc-106", "data_class": "public"})
	addRun("us-east1", "payments-authorizer-dr", "us-central1-docker.pkg.dev/gcp-proj-masked/prod-containers/payments:v2.4.1",
		"INGRESS_TRAFFIC_INTERNAL_LOAD_BALANCER", 2000, 1024, 1, 20, 80, 14, 100, 0,
		map[string]string{"env": "dr", "owner": "finance", "cost_center": "cc-102", "data_class": "restricted"})

	// 8. Cloud SQL Instances
	addSQL := func(name, dbVer, tier, region, availType string, backup, pitr, ssl, pubIP bool, sizeGB int64, labels map[string]string) {
		s.sqlInstances[name] = &sqladmin.DatabaseInstance{
			Name:            name,
			DatabaseVersion: dbVer,
			Region:          region,
			State:           "RUNNABLE",
			Settings: &sqladmin.Settings{
				Tier:             tier,
				AvailabilityType: availType,
				DataDiskSizeGb:   sizeGB,
				UserLabels:       labels,
				BackupConfiguration: &sqladmin.BackupConfiguration{
					Enabled:                    backup,
					PointInTimeRecoveryEnabled: pitr,
				},
				IpConfiguration: &sqladmin.IpConfiguration{
					RequireSsl:  ssl,
					Ipv4Enabled: pubIP,
				},
			},
		}
	}

	addSQL("prod-orders-pg15", "POSTGRES_15", "db-custom-4-16384", "us-central1", "REGIONAL",
		true, true, true, false, 250,
		map[string]string{"env": "prod", "owner": "fulfillment", "cost_center": "cc-104", "data_class": "restricted"})
	addSQL("prod-ledger-pg15", "POSTGRES_15", "db-custom-4-16384", "us-east1", "REGIONAL",
		true, true, true, false, 400,
		map[string]string{"env": "prod", "owner": "finance", "cost_center": "cc-102", "data_class": "restricted"})
	addSQL("staging-reporting-mysql8", "MYSQL_8_0", "db-g1-small", "us-central1", "ZONAL",
		true, false, false, true, 50,
		map[string]string{"env": "staging", "owner": "qa"})

	// 9. Firestore & BigQuery
	s.firestoreDocs["feature_flags"] = map[string]map[string]string{
		"checkout_v2": {"enabled": "true", "rollout_pct": "25", "owner": "platform"},
		"strict_mtls": {"enabled": "true", "rollout_pct": "100", "owner": "secops"},
	}

	s.bqDatasets["commerce_warehouse"] = &bigquery.Dataset{
		DatasetReference:         &bigquery.DatasetReference{DatasetId: "commerce_warehouse", ProjectId: "gcp-proj-masked"},
		Location:                 "US",
		DefaultTableExpirationMs: 365 * 86400 * 1000,
		Labels:                   map[string]string{"env": "prod", "owner": "data-eng", "cost_center": "cc-205", "data_class": "confidential"},
	}
	s.bqDatasets["security_telemetry"] = &bigquery.Dataset{
		DatasetReference:         &bigquery.DatasetReference{DatasetId: "security_telemetry", ProjectId: "gcp-proj-masked"},
		Location:                 "US-CENTRAL1",
		DefaultTableExpirationMs: 730 * 86400 * 1000,
		Labels:                   map[string]string{"env": "prod", "owner": "secops", "cost_center": "cc-301", "data_class": "restricted"},
	}
	s.bqTables["commerce_warehouse/fact_orders"] = &bigquery.Table{
		TableReference:   &bigquery.TableReference{DatasetId: "commerce_warehouse", TableId: "fact_orders"},
		TimePartitioning: &bigquery.TimePartitioning{Field: "order_timestamp", Type: "DAY"},
		NumRows:          4820000,
		NumBytes:         1420000000,
		Description:      "order_id:STRING,customer_hash:STRING,amount_usd:FLOAT,order_timestamp:TIMESTAMP",
	}
	s.bqTables["commerce_warehouse/dim_products"] = &bigquery.Table{
		TableReference: &bigquery.TableReference{DatasetId: "commerce_warehouse", TableId: "dim_products"},
		NumRows:        84500,
		NumBytes:       38000000,
		Description:    "sku:STRING,category:STRING,unit_price:FLOAT",
	}
	s.bqTables["security_telemetry/vpc_flow_events"] = &bigquery.Table{
		TableReference:   &bigquery.TableReference{DatasetId: "security_telemetry", TableId: "vpc_flow_events"},
		TimePartitioning: &bigquery.TimePartitioning{Field: "event_time", Type: "HOUR"},
		NumRows:          19500000,
		NumBytes:         4100000000,
		Description:      "src_ip:STRING,dest_ip:STRING,bytes_sent:INTEGER,event_time:TIMESTAMP",
	}

	// 10. IAM Service Accounts & Policy Bindings (strictly masked & no keys exposed)
	addSA := func(name, email, display string, disabled, isDefault bool, keys, keyMaxAge int64, roles []string) {
		s.serviceAccounts[email] = &iam.ServiceAccount{
			Name:        "projects/gcp-proj-masked/serviceAccounts/" + email,
			Email:       email,
			DisplayName: display,
			Disabled:    disabled,
		}
		s.saMeta[email] = saMetaInfo{
			ShortName:       name,
			UserManagedKeys: keys,
			KeyMaxAgeDays:   keyMaxAge,
			IsDefault:       isDefault,
			Roles:           roles,
		}
	}

	addSA("sa-prod-api", "sa-prod-api@gcp-proj-masked.iam.gserviceaccount.com", "Production API Gateway SA",
		false, false, 0, 0, []string{"roles/run.invoker", "roles/pubsub.publisher", "roles/secretmanager.secretAccessor"})
	addSA("sa-prod-worker", "sa-prod-worker@gcp-proj-masked.iam.gserviceaccount.com", "Fulfillment Worker SA",
		false, false, 0, 0, []string{"roles/pubsub.subscriber", "roles/cloudsql.client", "roles/storage.objectUser"})
	addSA("sa-analytics-etl", "sa-analytics-etl@gcp-proj-masked.iam.gserviceaccount.com", "Analytics BigQuery ETL SA",
		false, false, 1, 42, []string{"roles/bigquery.dataEditor", "roles/storage.objectViewer"})
	addSA("sa-legacy-ci", "sa-legacy-ci@gcp-proj-masked.iam.gserviceaccount.com", "Legacy CI Deployer SA",
		false, false, 2, 185, []string{"roles/editor", "roles/storage.admin"})
	addSA("default-compute", "000000000000-compute@developer.gserviceaccount.com", "Compute Engine default service account",
		false, true, 1, 310, []string{"roles/editor"})

	s.iamPolicy = &cloudresourcemanager.Policy{
		Bindings: []*cloudresourcemanager.Binding{
			{
				Role:    "roles/run.invoker",
				Members: []string{"serviceAccount:sa-prod-api@gcp-proj-masked.iam.gserviceaccount.com"},
			},
			{
				Role:    "roles/pubsub.publisher",
				Members: []string{"serviceAccount:sa-prod-api@gcp-proj-masked.iam.gserviceaccount.com"},
			},
			{
				Role:    "roles/pubsub.subscriber",
				Members: []string{"serviceAccount:sa-prod-worker@gcp-proj-masked.iam.gserviceaccount.com"},
			},
			{
				Role:    "roles/cloudsql.client",
				Members: []string{"serviceAccount:sa-prod-worker@gcp-proj-masked.iam.gserviceaccount.com"},
			},
			{
				Role:    "roles/bigquery.dataEditor",
				Members: []string{"serviceAccount:sa-analytics-etl@gcp-proj-masked.iam.gserviceaccount.com"},
			},
			{
				Role: "roles/editor",
				Members: []string{
					"serviceAccount:sa-legacy-ci@gcp-proj-masked.iam.gserviceaccount.com",
					"serviceAccount:000000000000-compute@developer.gserviceaccount.com",
				},
			},
		},
	}

	// 11. Secret Manager Metadata ONLY (no secret values ever stored or returned)
	addSecret := func(name, repl string, rotDays, verCount, enabledVers int64, cmek, hasExp bool, labels map[string]string) {
		sec := &secretmanager.Secret{
			Name:   "projects/gcp-proj-masked/secrets/" + name,
			Labels: labels,
			Replication: &secretmanager.Replication{
				Automatic: &secretmanager.Automatic{},
			},
		}
		if rotDays > 0 {
			sec.Rotation = &secretmanager.Rotation{
				RotationPeriod: fmt.Sprintf("%ds", rotDays*86400),
			}
		}
		s.secrets[name] = sec
		s.secretMeta[name] = secretMetaInfo{
			ReplicationPolicy: repl,
			RotationDays:      rotDays,
			VersionCount:      verCount,
			EnabledVersions:   enabledVers,
			CMEKEnabled:       cmek,
			HasExpiration:     hasExp,
		}
	}

	addSecret("prod-db-app-role-meta", "user-managed-multi-region", 30, 6, 1, true, false,
		map[string]string{"env": "prod", "owner": "fulfillment", "cost_center": "cc-104", "data_class": "restricted"})
	addSecret("prod-stripe-webhook-meta", "user-managed-multi-region", 45, 4, 2, true, false,
		map[string]string{"env": "prod", "owner": "finance", "cost_center": "cc-102", "data_class": "restricted"})
	addSecret("prod-jwt-signing-meta", "automatic", 30, 8, 2, true, false,
		map[string]string{"env": "prod", "owner": "secops", "cost_center": "cc-301", "data_class": "restricted"})
	addSecret("staging-oauth-client-meta", "automatic", 90, 2, 1, false, true,
		map[string]string{"env": "staging", "owner": "qa"})
	addSecret("legacy-partner-webhook-meta", "automatic", 0, 1, 1, false, false,
		map[string]string{"env": "prod", "owner": "integrations", "cost_center": "cc-112", "data_class": "confidential"})

	// 12. Cloud KMS KeyRings & CryptoKeys
	addKMS := func(loc, ring, key, purpose, state, protLevel string, rotDays int64) {
		ringKey := loc + "/" + ring
		s.kmsKeyRings[ringKey] = &cloudkms.KeyRing{
			Name: fmt.Sprintf("projects/gcp-proj-masked/locations/%s/keyRings/%s", loc, ring),
		}
		fullKey := loc + "/" + ring + "/" + key
		s.kmsKeys[fullKey] = &cloudkms.CryptoKey{
			Name:           fmt.Sprintf("projects/gcp-proj-masked/locations/%s/keyRings/%s/cryptoKeys/%s", loc, ring, key),
			Purpose:        purpose,
			RotationPeriod: fmt.Sprintf("%ds", rotDays*86400),
			Primary: &cloudkms.CryptoKeyVersion{
				State:           state,
				ProtectionLevel: protLevel,
			},
			VersionTemplate: &cloudkms.CryptoKeyVersionTemplate{
				ProtectionLevel: protLevel,
			},
		}
		s.kmsRotDays[fullKey] = rotDays
	}

	addKMS("us-central1", "prod-ring", "storage-cmek", "ENCRYPT_DECRYPT", "ENABLED", "HSM", 90)
	addKMS("us-central1", "prod-ring", "sql-cmek", "ENCRYPT_DECRYPT", "ENABLED", "HSM", 90)
	addKMS("us-central1", "prod-ring", "artifact-signing-key", "ASYMMETRIC_SIGN", "ENABLED", "HSM", 0)
	addKMS("us-east1", "dr-ring", "archive-cmek", "ENCRYPT_DECRYPT", "ENABLED", "SOFTWARE", 180)

	// 13. GKE Clusters & Artifact Registry
	addGKE := func(loc, name, k8sVer, channel string, autopilot, priv, wi, netPol, binAuth bool, pools, nodes int64, labels map[string]string) {
		key := loc + "/" + name
		s.gkeClusters[key] = &container.Cluster{
			Name:                  name,
			Location:              loc,
			Status:                "RUNNING",
			CurrentMasterVersion:  k8sVer,
			CurrentNodeCount:      nodes,
			ResourceLabels:        labels,
			ReleaseChannel:        &container.ReleaseChannel{Channel: channel},
			Autopilot:             &container.Autopilot{Enabled: autopilot},
			PrivateClusterConfig:  &container.PrivateClusterConfig{EnablePrivateNodes: priv},
			WorkloadIdentityConfig: &container.WorkloadIdentityConfig{WorkloadPool: "gcp-proj-masked.svc.id.goog"},
			NetworkPolicy:         &container.NetworkPolicy{Enabled: netPol},
			BinaryAuthorization:   &container.BinaryAuthorization{EvaluationMode: "PROJECT_SINGLETON_POLICY_ENFORCE"},
		}
		s.gkeMeta[key] = gkeMetaInfo{
			Autopilot:        autopilot,
			PrivateNodes:     priv,
			WorkloadIdentity: wi,
			NetworkPolicy:    netPol,
			BinaryAuth:       binAuth,
			NodePoolCount:    pools,
			TotalNodes:       nodes,
		}
	}

	addGKE("us-central1", "prod-autopilot-core", "1.31.2-gke.1100", "REGULAR",
		true, true, true, true, true, 3, 18,
		map[string]string{"env": "prod", "owner": "platform", "cost_center": "cc-101", "data_class": "confidential"})
	addGKE("us-east1", "dr-autopilot-standby", "1.31.2-gke.1100", "STABLE",
		true, true, true, true, true, 2, 8,
		map[string]string{"env": "dr", "owner": "platform", "cost_center": "cc-101", "data_class": "confidential"})
	addGKE("us-central1", "staging-standard-k8s", "1.30.5-gke.1020", "RAPID",
		false, false, true, false, false, 2, 6,
		map[string]string{"env": "staging", "owner": "qa"})

	addAR := func(loc, name, format string, immut, vulnScan bool, cleanupPolicies, sizeMB, pkgCount int64) {
		key := loc + "/" + name
		s.arRepos[key] = &artifactregistry.Repository{
			Name:      fmt.Sprintf("projects/gcp-proj-masked/locations/%s/repositories/%s", loc, name),
			Format:    format,
			SizeBytes: sizeMB * 1024 * 1024,
		}
		s.arMeta[key] = arMetaInfo{
			ImmutableTags:         immut,
			CleanupPolicies:       cleanupPolicies,
			VulnerabilityScanning: vulnScan,
			SizeMB:                sizeMB,
			PackageCount:          pkgCount,
		}
	}

	addAR("us-central1", "prod-containers", "DOCKER", true, true, 3, 4280, 64)
	addAR("us-central1", "go-internal-modules", "GO", true, true, 2, 340, 28)
	addAR("us-central1", "staging-scratch-images", "DOCKER", false, false, 0, 1890, 42)

	// 14. Cloud DNS Managed Zones & Record Sets
	s.dnsZones["prod-internal-zone"] = &dns.ManagedZone{
		Name:       "prod-internal-zone",
		DnsName:    "internal.gcp-masked.example.",
		Visibility: "private",
		DnssecConfig: &dns.ManagedZoneDnsSecConfig{
			State: "on",
		},
	}
	s.dnsRecords["prod-internal-zone"] = []*dns.ResourceRecordSet{
		{Name: "api.internal.gcp-masked.example.", Type: "A", Ttl: 60, Rrdatas: []string{"10.10.1.11", "10.10.1.12"}},
		{Name: "db.internal.gcp-masked.example.", Type: "A", Ttl: 60, Rrdatas: []string{"10.10.16.10"}},
		{Name: "cache.internal.gcp-masked.example.", Type: "CNAME", Ttl: 120, Rrdatas: []string{"redis.internal.gcp-masked.example."}},
	}
	s.dnsZones["public-edge-zone"] = &dns.ManagedZone{
		Name:       "public-edge-zone",
		DnsName:    "edge.gcp-masked.example.",
		Visibility: "public",
		DnssecConfig: &dns.ManagedZoneDnsSecConfig{
			State: "on",
		},
	}
	s.dnsRecords["public-edge-zone"] = []*dns.ResourceRecordSet{
		{Name: "edge.gcp-masked.example.", Type: "A", Ttl: 300, Rrdatas: []string{"34.110.200.10"}},
		{Name: "_dmarc.edge.gcp-masked.example.", Type: "TXT", Ttl: 3600, Rrdatas: []string{"\"v=DMARC1; p=reject;\""}},
	}

	// 15. Cloud Logging Sinks & Severity Distribution
	s.logSinks["audit-to-bigquery"] = &logging.LogSink{
		Name:        "audit-to-bigquery",
		Destination: "bigquery.googleapis.com/projects/gcp-proj-masked/datasets/security_telemetry",
		Filter:      "logName:\"cloudaudit.googleapis.com\" OR severity>=WARNING",
		Disabled:    false,
	}
	s.logSinks["compliance-to-archive-gcs"] = &logging.LogSink{
		Name:        "compliance-to-archive-gcs",
		Destination: "storage.googleapis.com/prod-compliance-ledger-archive",
		Filter:      "logName:\"cloudaudit.googleapis.com/activity\"",
		Disabled:    false,
	}
	s.logSinks["siem-realtime-pubsub"] = &logging.LogSink{
		Name:        "siem-realtime-pubsub",
		Destination: "pubsub.googleapis.com/projects/gcp-proj-masked/topics/security-audit-stream",
		Filter:      "severity>=ERROR",
		Disabled:    false,
	}
	s.logSeverityCounts = map[string]int64{
		"DEBUG":    18400,
		"INFO":     94200,
		"NOTICE":   6300,
		"WARNING":  2180,
		"ERROR":    340,
		"CRITICAL": 18,
	}

	// 16. Cloud Monitoring Alert Policies & Time-Series
	s.alertPolicies["high-api-p95-latency"] = &monitoring.AlertPolicy{
		Name:        "projects/gcp-proj-masked/alertPolicies/high-api-p95-latency",
		DisplayName: "high-api-p95-latency",
		Enabled:     true,
		NotificationChannels: []string{"projects/gcp-proj-masked/notificationChannels/pagerduty-oncall", "projects/gcp-proj-masked/notificationChannels/slack-sre"},
	}
	s.alertMeta["high-api-p95-latency"] = alertMetaInfo{
		MetricType:  "run.googleapis.com/request_latencies_p95_ms",
		Threshold:   250.0,
		DurationSec: 300,
	}
	s.alertPolicies["pubsub-dlq-non-zero"] = &monitoring.AlertPolicy{
		Name:        "projects/gcp-proj-masked/alertPolicies/pubsub-dlq-non-zero",
		DisplayName: "pubsub-dlq-non-zero",
		Enabled:     true,
		NotificationChannels: []string{"projects/gcp-proj-masked/notificationChannels/slack-sre"},
	}
	s.alertMeta["pubsub-dlq-non-zero"] = alertMetaInfo{
		MetricType:  "pubsub.googleapis.com/subscription/num_undelivered_messages",
		Threshold:   10.0,
		DurationSec: 120,
	}
	s.alertPolicies["sql-cpu-saturation"] = &monitoring.AlertPolicy{
		Name:        "projects/gcp-proj-masked/alertPolicies/sql-cpu-saturation",
		DisplayName: "sql-cpu-saturation",
		Enabled:     true,
		NotificationChannels: []string{"projects/gcp-proj-masked/notificationChannels/pagerduty-oncall"},
	}
	s.alertMeta["sql-cpu-saturation"] = alertMetaInfo{
		MetricType:  "cloudsql.googleapis.com/database/cpu/utilization",
		Threshold:   0.85,
		DurationSec: 600,
	}

	s.metricSeries["compute.googleapis.com/instance/cpu/utilization"] = []float64{
		0.42, 0.45, 0.51, 0.58, 0.63, 0.61, 0.55, 0.49, 0.46, 0.52, 0.67, 0.71,
	}
	s.metricSeries["run.googleapis.com/request_latencies_p95_ms"] = []float64{
		48.5, 51.2, 54.0, 62.8, 79.4, 92.1, 84.0, 68.3, 59.1, 53.7, 50.4, 49.0,
	}
	s.metricSeries["run.googleapis.com/request_error_rate_pct"] = []float64{
		0.04, 0.05, 0.06, 0.12, 0.28, 0.35, 0.19, 0.09, 0.06, 0.05, 0.04, 0.04,
	}
	s.metricSeries["cloudsql.googleapis.com/database/cpu/utilization"] = []float64{
		0.31, 0.33, 0.36, 0.41, 0.48, 0.52, 0.49, 0.44, 0.39, 0.37, 0.35, 0.34,
	}
}
