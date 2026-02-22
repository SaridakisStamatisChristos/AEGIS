package contracts

import (
	"crypto/ed25519"
	"time"
)

// Run represents a single agent execution session
type Run struct {
	RunID          string                 `json:"run_id" db:"run_id"`
	OrgID          string                 `json:"org_id" db:"org_id"`
	ParentRunID    *string                `json:"parent_run_id,omitempty" db:"parent_run_id"`
	PolicyRef      PolicyRef              `json:"policy_ref" db:"policy_ref"`
	StateSchemaRef *SchemaRef             `json:"state_schema_ref,omitempty" db:"state_schema_ref"`
	Metadata       map[string]interface{} `json:"metadata" db:"metadata"`
	CreatedAt      time.Time              `json:"created_at" db:"created_at"`
	EndedAt        *time.Time             `json:"ended_at,omitempty" db:"ended_at"`
	Status         RunStatus              `json:"status" db:"status"`
	Outcome        *RunOutcome            `json:"outcome,omitempty" db:"outcome"`
	Counters       RunCounters            `json:"counters" db:"counters"`
	EvidenceHash   *string                `json:"evidence_hash,omitempty" db:"evidence_hash"`
	Signature      *string                `json:"signature,omitempty" db:"signature"`
	SignerKeyID    *string                `json:"signer_key_id,omitempty" db:"signer_key_id"`
}

type PolicyRef struct {
	PolicyID string `json:"policy_id"`
	Version  string `json:"version"`
}

type SchemaRef struct {
	SchemaID string `json:"schema_id"`
	Version  string `json:"version"`
}

type RunStatus string

const (
	RunStatusRunning   RunStatus = "running"
	RunStatusCompleted RunStatus = "completed"
	RunStatusFailed    RunStatus = "failed"
	RunStatusBlocked   RunStatus = "blocked"
	RunStatusCancelled RunStatus = "cancelled"
)

type RunOutcome struct {
	Result     interface{} `json:"result,omitempty"`
	Error      *string     `json:"error,omitempty"`
	ExitReason string      `json:"exit_reason"`
}

type RunCounters struct {
	Steps         int `json:"steps"`
	ToolCalls     int `json:"tool_calls"`
	BytesEgressed int `json:"bytes_egressed"`
	Retries       int `json:"retries"`
	Blocks        int `json:"blocks"`
}

// Step represents a logical unit of work within a run
type Step struct {
	StepID       string                 `json:"step_id" db:"step_id"`
	RunID        string                 `json:"run_id" db:"run_id"`
	ParentStepID *string                `json:"parent_step_id,omitempty" db:"parent_step_id"`
	SeqNo        int                    `json:"seq_no" db:"seq_no"`
	Name         string                 `json:"name" db:"name"`
	StateVector  map[string]interface{} `json:"state_vector" db:"state_vector"`
	StartedAt    time.Time              `json:"started_at" db:"started_at"`
	EndedAt      *time.Time             `json:"ended_at,omitempty" db:"ended_at"`
	Status       StepStatus             `json:"status" db:"status"`
	Error        *string                `json:"error,omitempty" db:"error"`
}

type StepStatus string

const (
	StepStatusRunning   StepStatus = "running"
	StepStatusCompleted StepStatus = "completed"
	StepStatusFailed    StepStatus = "failed"
)

// ToolCall represents a single tool invocation
type ToolCall struct {
	ToolCallID       string                 `json:"tool_call_id" db:"tool_call_id"`
	RunID            string                 `json:"run_id" db:"run_id"`
	StepID           string                 `json:"step_id" db:"step_id"`
	SeqNo            int                    `json:"seq_no" db:"seq_no"`
	ToolName         string                 `json:"tool_name" db:"tool_name"`
	Args             map[string]interface{} `json:"args" db:"args"`
	ArgsRedacted     bool                   `json:"args_redacted" db:"args_redacted"`
	RequestedAt      time.Time              `json:"requested_at" db:"requested_at"`
	RespondedAt      *time.Time             `json:"responded_at,omitempty" db:"responded_at"`
	Decision         Decision               `json:"decision" db:"decision"`
	Response         *ToolResponse          `json:"response,omitempty" db:"response"`
	ResponseRedacted bool                   `json:"response_redacted" db:"response_redacted"`
	Metadata         ToolCallMetadata       `json:"metadata" db:"metadata"`
}

type Decision struct {
	Action       PolicyAction `json:"action"`
	PolicyRuleID string       `json:"policy_rule_id"`
	Reason       string       `json:"reason"`
	ApprovalID   *string      `json:"approval_id,omitempty"`
}

type PolicyAction string

const (
	ActionAllow           PolicyAction = "allow"
	ActionWarn            PolicyAction = "warn"
	ActionRedact          PolicyAction = "redact"
	ActionBlock           PolicyAction = "block"
	ActionRequireApproval PolicyAction = "require_approval"
	ActionDegrade         PolicyAction = "degrade"
)

type ToolResponse struct {
	Result     interface{} `json:"result,omitempty"`
	Error      *string     `json:"error,omitempty"`
	DurationMs float64     `json:"duration_ms"`
}

type ToolCallMetadata struct {
	Executor   string `json:"executor"`
	RetryCount int    `json:"retry_count"`
}

// Event represents a ledger entry with hash chaining
type Event struct {
	EventID   string                 `json:"event_id" db:"event_id"`
	RunID     string                 `json:"run_id" db:"run_id"`
	SeqNo     int                    `json:"seq_no" db:"seq_no"`
	EventType EventType              `json:"event_type" db:"event_type"`
	Timestamp time.Time              `json:"timestamp" db:"timestamp"`
	Payload   map[string]interface{} `json:"payload" db:"payload"`
	PrevHash  *string                `json:"prev_hash" db:"prev_hash"`
	EventHash string                 `json:"event_hash" db:"event_hash"`
}

type EventType string

const (
	EventRunStarted       EventType = "run.started"
	EventRunEnded         EventType = "run.ended"
	EventStepStarted      EventType = "step.started"
	EventStepEnded        EventType = "step.ended"
	EventToolRequested    EventType = "tool.requested"
	EventToolDecided      EventType = "tool.decided"
	EventToolResponded    EventType = "tool.responded"
	EventDecisionOverride EventType = "decision.overridden"
	EventStateUpdated     EventType = "state.updated"
)

// Policy represents runtime constraints
type Policy struct {
	PolicyID   string       `json:"policy_id" db:"policy_id"`
	OrgID      string       `json:"org_id" db:"org_id"`
	Name       string       `json:"name" db:"name"`
	Version    string       `json:"version" db:"version"`
	Status     PolicyStatus `json:"status" db:"status"`
	CreatedAt  time.Time    `json:"created_at" db:"created_at"`
	ApprovedAt *time.Time   `json:"approved_at,omitempty" db:"approved_at"`
	ApprovedBy []string     `json:"approved_by,omitempty" db:"approved_by"`
	Spec       PolicySpec   `json:"spec" db:"spec"`
	SpecHash   string       `json:"spec_hash" db:"spec_hash"`
}

type PolicyStatus string

const (
	PolicyStatusDraft      PolicyStatus = "draft"
	PolicyStatusReview     PolicyStatus = "review"
	PolicyStatusApproved   PolicyStatus = "approved"
	PolicyStatusDeployed   PolicyStatus = "deployed"
	PolicyStatusDeprecated PolicyStatus = "deprecated"
)

type PolicySpec struct {
	Tools          []ToolPolicy     `json:"tools"`
	Budgets        Budgets          `json:"budgets"`
	EgressControls *EgressControls  `json:"egress_controls,omitempty"`
	Redaction      *RedactionConfig `json:"redaction,omitempty"`
}

type ToolPolicy struct {
	Name         string                 `json:"name"`
	Action       PolicyAction           `json:"action"`
	ArgSchema    map[string]interface{} `json:"arg_schema,omitempty"`
	OutputSchema map[string]interface{} `json:"output_schema,omitempty"`
	Conditions   []string               `json:"conditions,omitempty"`
}

type Budgets struct {
	MaxToolCalls     *int `json:"max_tool_calls,omitempty"`
	MaxWallClockSec  *int `json:"max_wall_clock_sec,omitempty"`
	MaxRetries       *int `json:"max_retries,omitempty"`
	MaxBytesEgressed *int `json:"max_bytes_egressed,omitempty"`
}

type EgressControls struct {
	DomainAllowlist []string `json:"domain_allowlist,omitempty"`
	DomainDenylist  []string `json:"domain_denylist,omitempty"`
	BlockPrivateIPs bool     `json:"block_private_ips"`
}

type RedactionConfig struct {
	Patterns     []string     `json:"patterns"`
	MaskStrategy MaskStrategy `json:"mask_strategy"`
}

type MaskStrategy string

const (
	MaskHash     MaskStrategy = "hash"
	MaskRedact   MaskStrategy = "redact"
	MaskTruncate MaskStrategy = "truncate"
)

// Approval represents a policy approval record
type Approval struct {
	ApprovalID string    `json:"approval_id" db:"approval_id"`
	PolicyID   string    `json:"policy_id" db:"policy_id"`
	Version    string    `json:"version" db:"version"`
	ApproverID string    `json:"approver_id" db:"approver_id"`
	Decision   string    `json:"decision" db:"decision"` // "approved" | "rejected"
	Comment    *string   `json:"comment,omitempty" db:"comment"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
}

// SigningKey represents a cryptographic key for evidence signing
type SigningKey struct {
	KeyID      string             `json:"key_id" db:"key_id"`
	OrgID      string             `json:"org_id" db:"org_id"`
	PublicKey  ed25519.PublicKey  `json:"public_key" db:"public_key"`
	PrivateKey ed25519.PrivateKey `json:"-" db:"private_key"` // never exposed in JSON
	CreatedAt  time.Time          `json:"created_at" db:"created_at"`
	Status     KeyStatus          `json:"status" db:"status"`
}

type KeyStatus string

const (
	KeyStatusActive     KeyStatus = "active"
	KeyStatusDeprecated KeyStatus = "deprecated"
)

// EvidenceBundleManifest metadata
type EvidenceBundleManifest struct {
	BundleVersion string    `json:"bundle_version"`
	RunID         string    `json:"run_id"`
	ExportedAt    time.Time `json:"exported_at"`
	EventCount    int       `json:"event_count"`
	RootHash      string    `json:"root_hash"`
	Signature     string    `json:"signature"`
	SignerKeyID   string    `json:"signer_key_id"`
}
