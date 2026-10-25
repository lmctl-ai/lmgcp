// SPDX-License-Identifier: MIT

package host

import "strings"

// ComputeInstance wraps a GCP Compute Engine VM instance snapshot.
type ComputeInstance struct {
	NameVal               string
	ZoneVal               string
	MachineTypeVal        string
	StatusVal             string
	NetworkVal            string
	InternalIPVal         string
	ExternalIPVal         string
	CPUPlatformVal        string
	DiskGBVal             int64
	VCPUsVal              int64
	MemoryMBVal           int64
	LabelsVal             map[string]string
	ServiceAccountVal     string
	ShieldedVTPMVal       bool
	DeletionProtectionVal bool
}

// VpcNetwork wraps a GCP VPC network snapshot.
type VpcNetwork struct {
	NameVal              string
	AutoCreateSubnetsVal bool
	RoutingModeVal       string
	MTUVal               int64
	SubnetCountVal       int64
}

// Subnet wraps a GCP VPC subnetwork snapshot.
type Subnet struct {
	NameVal                  string
	RegionVal                string
	NetworkVal               string
	CIDRVal                  string
	PrivateIPGoogleAccessVal bool
	FlowLogsVal              bool
}

// FirewallRule wraps a GCP Compute Engine firewall rule snapshot.
type FirewallRule struct {
	NameVal       string
	NetworkVal    string
	DirectionVal  string
	PriorityVal   int64
	ActionVal     string
	ProtocolVal   string
	PortsVal      string
	SourceCIDRVal string
	TargetTagsVal string
	DisabledVal   bool
	LogEnabledVal bool
}

func (f *FirewallRule) IsOpenToWorld() bool {
	if f == nil || f.DisabledVal {
		return false
	}
	return strings.Contains(f.SourceCIDRVal, "0.0.0.0/0") || strings.Contains(f.SourceCIDRVal, "::/0")
}

func (f *FirewallRule) RiskScore() int64 {
	if f == nil || f.DisabledVal {
		return 0
	}
	var score int64 = 10
	if f.IsOpenToWorld() {
		score += 55
		if strings.Contains(f.PortsVal, "22") || strings.Contains(f.PortsVal, "3389") {
			score += 30
		} else if strings.Contains(f.PortsVal, "5432") || strings.Contains(f.PortsVal, "3306") || strings.Contains(f.PortsVal, "6379") {
			score += 25
		}
	}
	if !f.LogEnabledVal {
		score += 5
	}
	if score > 100 {
		score = 100
	}
	return score
}

// StorageBucket wraps a GCP Cloud Storage bucket snapshot.
type StorageBucket struct {
	NameVal                   string
	LocationVal               string
	StorageClassVal           string
	VersioningVal             bool
	UniformAccessVal          bool
	PublicAccessPreventionVal string
	LifecycleDeleteDaysVal    int64
	LifecycleNearlineDaysVal  int64
	EncryptionTypeVal         string
	ObjectCountVal            int64
	TotalBytesVal             int64
	LabelsVal                 map[string]string
}

// PubsubTopic wraps a GCP Pub/Sub topic snapshot.
type PubsubTopic struct {
	NameVal           string
	RetentionSecVal   int64
	PublishedCountVal int64
	LabelsVal         map[string]string
}

// PubsubSubscription wraps a GCP Pub/Sub subscription snapshot.
type PubsubSubscription struct {
	NameVal                string
	TopicVal               string
	AckDeadlineSecVal      int64
	DeadLetterTopicVal     string
	MaxDeliveryAttemptsVal int64
	BacklogCountVal        int64
	RetainAckedVal         bool
}

// CloudRunService wraps a GCP Cloud Run v2 service snapshot.
type CloudRunService struct {
	NameVal             string
	RegionVal           string
	ImageVal            string
	URIVal              string
	CPUMilliVal         int64
	MemoryMBVal         int64
	MinInstancesVal     int64
	MaxInstancesVal     int64
	ConcurrencyVal      int64
	IngressVal          string
	LatestRevisionVal   string
	RevisionCountVal    int64
	StableTrafficPctVal int64
	CanaryTrafficPctVal int64
	LabelsVal           map[string]string
}

// SqlInstance wraps a GCP Cloud SQL instance snapshot.
type SqlInstance struct {
	NameVal            string
	DBVersionVal       string
	TierVal            string
	RegionVal          string
	StateVal           string
	HAEnabledVal       bool
	BackupEnabledVal   bool
	PITREnabledVal     bool
	RequireSSLVal      bool
	PublicIPEnabledVal bool
	StorageGBVal       int64
	LabelsVal          map[string]string
}

// BqDataset wraps a GCP BigQuery dataset snapshot.
type BqDataset struct {
	DatasetIDVal             string
	LocationVal              string
	TableCountVal            int64
	DefaultExpirationDaysVal int64
	LabelsVal                map[string]string
}

// BqTable wraps a GCP BigQuery table snapshot.
type BqTable struct {
	DatasetIDVal      string
	TableIDVal        string
	SchemaSpecVal     string
	PartitionFieldVal string
	RowCountVal       int64
	BytesVal          int64
}

// ServiceAccount wraps a GCP IAM service account snapshot (metadata only).
type ServiceAccount struct {
	NameVal            string
	EmailVal           string
	DisplayNameVal     string
	DisabledVal        bool
	UserManagedKeysVal int64
	KeyMaxAgeDaysVal   int64
	IsDefaultVal       bool
	RolesVal           []string
}

// SecretMetadata wraps a GCP Secret Manager secret metadata snapshot (never secret values).
type SecretMetadata struct {
	NameVal              string
	ReplicationPolicyVal string
	VersionCountVal      int64
	EnabledVersionsVal   int64
	RotationDaysVal      int64
	CMEKEnabledVal       bool
	HasExpirationVal     bool
	LabelsVal            map[string]string
}

// KmsKey wraps a GCP Cloud KMS crypto key snapshot.
type KmsKey struct {
	NameVal            string
	KeyRingVal         string
	LocationVal        string
	PurposeVal         string
	RotationDaysVal    int64
	PrimaryStateVal    string
	ProtectionLevelVal string
}

// GkeCluster wraps a GCP Kubernetes Engine cluster snapshot.
type GkeCluster struct {
	NameVal             string
	LocationVal         string
	StatusVal           string
	K8sVersionVal       string
	ReleaseChannelVal   string
	AutopilotVal        bool
	PrivateNodesVal     bool
	WorkloadIdentityVal bool
	NetworkPolicyVal    bool
	BinaryAuthVal       bool
	NodePoolCountVal    int64
	TotalNodesVal       int64
	LabelsVal           map[string]string
}

// ArtifactRepo wraps a GCP Artifact Registry repository snapshot.
type ArtifactRepo struct {
	NameVal                  string
	LocationVal              string
	FormatVal                string
	ImmutableTagsVal         bool
	CleanupPolicyCountVal    int64
	VulnerabilityScanningVal bool
	SizeMBVal                int64
	PackageCountVal          int64
}

// DnsZone wraps a GCP Cloud DNS managed zone snapshot.
type DnsZone struct {
	NameVal        string
	DNSNameVal     string
	VisibilityVal  string
	DNSSECVal      bool
	RecordCountVal int64
}

// LogSink wraps a GCP Cloud Logging sink snapshot.
type LogSink struct {
	NameVal        string
	DestinationVal string
	FilterVal      string
	DisabledVal    bool
}

// AlertPolicy wraps a GCP Cloud Monitoring alert policy snapshot.
type AlertPolicy struct {
	NameVal                 string
	MetricTypeVal           string
	ThresholdVal            float64
	DurationSecVal          int64
	EnabledVal              bool
	NotificationChannelsVal int64
}
