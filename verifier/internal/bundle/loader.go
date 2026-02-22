// Package bundle provides evidence bundle loading and parsing
package bundle

import (
	"archive/zip"
	"crypto/ed25519"
	"fmt"
	"time"
)

// Bundle represents a complete evidence bundle
type Bundle struct {
	Manifest  *Manifest
	Events    []Event
	Policy    *Policy
	Run       *Run
	PublicKey ed25519.PublicKey
}

// Manifest represents the bundle metadata
type Manifest struct {
	BundleVersion string    `json:"bundle_version"`
	RunID         string    `json:"run_id"`
	ExportedAt    time.Time `json:"exported_at"`
	EventCount    int       `json:"event_count"`
	RootHash      string    `json:"root_hash"`
	Signature     string    `json:"signature"`
	SignerKeyID   string    `json:"signer_key_id"`
}

// Event represents a single event in the chain
type Event struct {
	EventID   string                 `json:"event_id"`
	RunID     string                 `json:"run_id"`
	SeqNo     int                    `json:"seq_no"`
	EventType string                 `json:"event_type"`
	Timestamp time.Time              `json:"timestamp"`
	Payload   map[string]interface{} `json:"payload"`
	PrevHash  *string                `json:"prev_hash"`
	EventHash string                 `json:"event_hash"`
}

// Policy represents the policy snapshot
type Policy struct {
	PolicyID  string                 `json:"policy_id"`
	OrgID     string                 `json:"org_id"`
	Name      string                 `json:"name"`
	Version   string                 `json:"version"`
	Status    string                 `json:"status"`
	Spec      map[string]interface{} `json:"spec"`
	SpecHash  string                 `json:"spec_hash"`
	CreatedAt time.Time              `json:"created_at"`
}

// Run represents the run metadata
type Run struct {
	RunID        string                 `json:"run_id"`
	OrgID        string                 `json:"org_id"`
	PolicyRef    PolicyRef              `json:"policy_ref"`
	Metadata     map[string]interface{} `json:"metadata"`
	CreatedAt    time.Time              `json:"created_at"`
	EndedAt      *time.Time             `json:"ended_at"`
	Status       string                 `json:"status"`
	Outcome      map[string]interface{} `json:"outcome"`
	Counters     RunCounters            `json:"counters"`
	EvidenceHash *string                `json:"evidence_hash"`
	Signature    *string                `json:"signature"`
}

// PolicyRef represents a policy reference
type PolicyRef struct {
	PolicyID string `json:"policy_id"`
	Version  string `json:"version"`
}

// RunCounters represents run counters
type RunCounters struct {
	Steps         int `json:"steps"`
	ToolCalls     int `json:"tool_calls"`
	BytesEgressed int `json:"bytes_egressed"`
	Retries       int `json:"retries"`
	Blocks        int `json:"blocks"`
}

// Load loads an evidence bundle from a ZIP file
func Load(bundlePath string) (*Bundle, error) {
	// Open ZIP file
	reader, err := zip.OpenReader(bundlePath)
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}
	defer reader.Close()

	bundle := &Bundle{}

	// Parse each file in the bundle
	for _, file := range reader.File {
		switch file.Name {
		case "manifest.json":
			manifest, err := parseManifest(file)
			if err != nil {
				return nil, fmt.Errorf("parse manifest: %w", err)
			}
			bundle.Manifest = manifest

		case "events.jsonl":
			events, err := parseEvents(file)
			if err != nil {
				return nil, fmt.Errorf("parse events: %w", err)
			}
			bundle.Events = events

		case "policy_snapshot.json":
			policy, err := parsePolicy(file)
			if err != nil {
				return nil, fmt.Errorf("parse policy: %w", err)
			}
			bundle.Policy = policy

		case "run.json":
			run, err := parseRun(file)
			if err != nil {
				return nil, fmt.Errorf("parse run: %w", err)
			}
			bundle.Run = run

		case "public_key.pem":
			key, err := parsePublicKey(file)
			if err != nil {
				// Public key is optional - just warn
				fmt.Printf("Warning: failed to parse public key: %v\n", err)
			} else {
				bundle.PublicKey = key
			}
		}
	}

	// Validate required components
	if bundle.Manifest == nil {
		return nil, fmt.Errorf("bundle missing manifest.json")
	}

	if len(bundle.Events) == 0 {
		return nil, fmt.Errorf("bundle missing events.jsonl or has no events")
	}

	if bundle.Policy == nil {
		return nil, fmt.Errorf("bundle missing policy_snapshot.json")
	}

	return bundle, nil
}
