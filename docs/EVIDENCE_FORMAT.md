# AegisRun Evidence Bundle Format

**Version**: 1.0.0  
**Last Updated**: 2026-02-03

---

## 1. Overview

Evidence bundles provide tamper-evident records of agent execution. Each bundle contains:

- Complete run manifest (all steps, tool calls, decisions)
- Content-addressed blobs (input/output data)
- Merkle tree root for integrity verification
- Ed25519 digital signature

Bundles are designed for:
- Post-hoc auditing
- Regulatory compliance (SOC 2, HIPAA, etc.)
- Forensic investigation
- Chain-of-custody documentation

---

## 2. Bundle Structure

```
evidence-bundle/
├── manifest.json      # Run metadata + tree structure
├── signature.json     # Ed25519 signature over manifest
├── blobs/
│   ├── abc123...      # Content-addressed data blobs
│   ├── def456...
│   └── ...
└── merkle_proof.json  # Optional: Sparse proof for specific items
```

---

## 3. Manifest Format

```json
{
  "version": "1.0.0",
  "bundle_id": "01JQZX3K2FGH9VWBCD4BUNDLEID",
  "created_at": "2026-02-03T12:34:56.789Z",
  "run": {
    "id": "01JQZX3K2FGH9VWBCD45EFGHIJ",
    "org_id": "01JPKDEF456OrgExample123",
    "policy_id": "01JQZX3K2FGH9VWBCDPOLICYID",
    "status": "completed",
    "started_at": "2026-02-03T12:00:00.000Z",
    "finished_at": "2026-02-03T12:34:56.789Z",
    "metadata": {
      "agent_name": "customer-support-v2",
      "environment": "production",
      "user_id": "user_12345"
    }
  },
  "policy_snapshot": {
    "id": "01JQZX3K2FGH9VWBCDPOLICYID",
    "name": "production-policy",
    "version": "v1",
    "hash": "sha256:abc123...",
    "spec": { /* full policy spec at time of run */ }
  },
  "steps": [
    {
      "id": "01JQZX3K2FGH9VWBCDSTEP0001",
      "sequence": 0,
      "type": "agent_action",
      "hash": "sha256:step0hash...",
      "tool_calls": [
        {
          "id": "01JQZX3K2FGH9VWBCDTOOLCALL",
          "tool_name": "http_request",
          "arguments_blob": "sha256:args123...",
          "response_blob": "sha256:resp456...",
          "decision": {
            "action": "allow",
            "policy_rule_id": "tool.http_request",
            "evaluated_at": "2026-02-03T12:00:01.234Z"
          },
          "timing": {
            "queued_at": "2026-02-03T12:00:01.000Z",
            "started_at": "2026-02-03T12:00:01.234Z",
            "finished_at": "2026-02-03T12:00:02.567Z",
            "duration_ms": 1333
          }
        }
      ]
    }
  ],
  "events": [
    {
      "id": "01JQZX3K2FGH9VWBCDEVENT001",
      "type": "run.started",
      "timestamp": "2026-02-03T12:00:00.000Z",
      "payload_blob": "sha256:evt123..."
    }
  ],
  "merkle_root": "sha256:merkleroot123...",
  "counters": {
    "steps": 15,
    "tool_calls": 42,
    "events": 87,
    "bytes_input": 125000,
    "bytes_output": 340000,
    "approvals_requested": 2,
    "approvals_granted": 2,
    "policy_blocks": 3
  }
}
```

---

## 4. Content-Addressed Blobs

All data is stored as content-addressed blobs using SHA-256:

```
blob_id = sha256(content)
```

### 4.1 Blob Naming

```
blobs/sha256:a1b2c3d4e5f6...  (64 hex chars)
```

### 4.2 Blob Contents

Blobs contain raw JSON or binary data:

```json
// blobs/sha256:args123...
{
  "url": "https://api.example.com/data",
  "method": "GET",
  "headers": {
    "Authorization": "[REDACTED:API_KEY]"
  }
}
```

### 4.3 Blob References

The manifest references blobs by their hash:

```json
{
  "arguments_blob": "sha256:a1b2c3d4...",
  "response_blob": "sha256:e5f6g7h8..."
}
```

---

## 5. Merkle Tree Construction

### 5.1 Leaf Nodes

Each item (step, tool call, event) becomes a leaf node:

```
leaf_hash = sha256(canonical_json(item))
```

### 5.2 Tree Structure

```
                    Root
                   /    \
                  /      \
                 /        \
              Node1      Node2
             /    \     /    \
            L1    L2   L3    L4
           Step  Step Tool  Tool
            0     1   Call  Call
```

### 5.3 Construction Algorithm

```go
func BuildMerkleRoot(items []HashableItem) string {
    if len(items) == 0 {
        return sha256("")
    }
    
    leaves := make([]string, len(items))
    for i, item := range items {
        leaves[i] = sha256(canonicalJSON(item))
    }
    
    for len(leaves) > 1 {
        var nextLevel []string
        for i := 0; i < len(leaves); i += 2 {
            if i+1 < len(leaves) {
                nextLevel = append(nextLevel, sha256(leaves[i] + leaves[i+1]))
            } else {
                nextLevel = append(nextLevel, leaves[i])  // Odd node promoted
            }
        }
        leaves = nextLevel
    }
    
    return leaves[0]
}
```

---

## 6. Signature Format

```json
{
  "algorithm": "Ed25519",
  "key_id": "01JQZX3K2FGH9VWBCDKEYID001",
  "key_fingerprint": "sha256:keyfingerprint...",
  "signed_at": "2026-02-03T12:34:56.789Z",
  "message_hash": "sha256:manifestsummary...",
  "signature": "base64:MEUCIQDw3k8..."
}
```

### 6.1 Signed Message

The signature is computed over:

```go
message := fmt.Sprintf(
    "aegisrun-evidence-v1\n%s\n%s\n%s",
    bundleID,
    merkleRoot,
    createdAt.Format(time.RFC3339Nano),
)

signature := ed25519.Sign(privateKey, []byte(message))
```

### 6.2 Verification

```go
func VerifySignature(bundle Bundle, publicKey ed25519.PublicKey) error {
    message := fmt.Sprintf(
        "aegisrun-evidence-v1\n%s\n%s\n%s",
        bundle.ID,
        bundle.MerkleRoot,
        bundle.CreatedAt.Format(time.RFC3339Nano),
    )
    
    signature, err := base64.StdEncoding.DecodeString(bundle.Signature.Signature)
    if err != nil {
        return err
    }
    
    if !ed25519.Verify(publicKey, []byte(message), signature) {
        return errors.New("invalid signature")
    }
    
    return nil
}
```

---

## 7. Hashing Scheme

### 7.1 Canonical JSON (RFC 8785)

All JSON is canonicalized before hashing:

1. Keys sorted lexicographically
2. No whitespace
3. Unicode normalized (NFC)
4. Numbers as minimal decimal

```go
import "github.com/gibson042/canonicaljson-go"

func CanonicalHash(v interface{}) string {
    canonical, _ := canonicaljson.Marshal(v)
    hash := sha256.Sum256(canonical)
    return fmt.Sprintf("sha256:%x", hash)
}
```

### 7.2 Run Hash Computation

```go
type RunHashInput struct {
    ID        string            `json:"id"`
    OrgID     string            `json:"org_id"`
    PolicyID  string            `json:"policy_id"`
    StartedAt time.Time         `json:"started_at"`
    Metadata  map[string]string `json:"metadata"`
}

runHash := CanonicalHash(RunHashInput{...})
```

### 7.3 Step Hash Computation

```go
type StepHashInput struct {
    ID            string `json:"id"`
    RunID         string `json:"run_id"`
    Sequence      int    `json:"sequence"`
    PreviousHash  string `json:"previous_hash"`  // Chain linking
    Type          string `json:"type"`
    ToolCallsHash string `json:"tool_calls_hash"`
}

stepHash := CanonicalHash(StepHashInput{...})
```

### 7.4 Step Chain Linking

Steps form a hash chain for tamper detection:

```
Step 0: hash = H(step0_data | "")
Step 1: hash = H(step1_data | step0_hash)
Step 2: hash = H(step2_data | step1_hash)
...
```

---

## 8. Sparse Proofs

For partial verification (e.g., prove a specific tool call):

```json
{
  "target_hash": "sha256:targetitem...",
  "path": [
    {
      "position": "left",
      "hash": "sha256:sibling1..."
    },
    {
      "position": "right", 
      "hash": "sha256:sibling2..."
    }
  ],
  "root": "sha256:merkleroot123..."
}
```

### 8.1 Proof Verification

```go
func VerifyProof(proof MerkleProof) bool {
    currentHash := proof.TargetHash
    
    for _, node := range proof.Path {
        if node.Position == "left" {
            currentHash = sha256(node.Hash + currentHash)
        } else {
            currentHash = sha256(currentHash + node.Hash)
        }
    }
    
    return currentHash == proof.Root
}
```

---

## 9. Export Formats

### 9.1 Full Bundle (Default)

```bash
GET /api/v1/runs/{run_id}/export?format=bundle

# Returns: application/zip
evidence-bundle-{run_id}.zip
```

### 9.2 JSON Manifest Only

```bash
GET /api/v1/runs/{run_id}/export?format=manifest

# Returns: application/json
{
  "version": "1.0.0",
  "manifest": {...},
  "signature": {...}
}
```

### 9.3 Attestation Document

```bash
GET /api/v1/runs/{run_id}/export?format=attestation

# Returns: application/pdf
# Human-readable PDF with signatures
```

---

## 10. Retention & Compliance

### 10.1 Retention Policies

| Data Type | Default Retention | Configurable |
|-----------|-------------------|--------------|
| Run metadata | 90 days | Yes |
| Tool call arguments | 30 days | Yes |
| Tool call responses | 30 days | Yes |
| Evidence bundles | 7 years | Yes |
| Signatures | Indefinite | No |

### 10.2 Compliance Mappings

| Framework | Evidence Requirement | AegisRun Support |
|-----------|---------------------|------------------|
| SOC 2 | Access logs, change history | ✅ Full audit trail |
| HIPAA | PHI access logging | ✅ Redacted PII + audit |
| GDPR | Data processing records | ✅ Complete lineage |
| PCI-DSS | Cardholder data protection | ✅ Credit card redaction |

---

## 11. Verification CLI

```bash
# Verify bundle integrity
aegis verify bundle evidence-bundle.zip

# Output:
✓ Manifest schema valid
✓ All blob references resolved
✓ Merkle root verified
✓ Signature valid (key: 01JQZX3K2FGH9...)
✓ Step chain valid (15 steps)

Bundle verified successfully.
```

### 11.1 Verification Steps

1. **Schema validation**: Manifest matches expected schema
2. **Blob resolution**: All referenced blobs exist
3. **Hash verification**: Blob contents match their hashes
4. **Merkle verification**: Recomputed root matches declared root
5. **Signature verification**: Ed25519 signature valid
6. **Chain verification**: Step hashes form valid chain

---

## 12. Security Considerations

1. **Key rotation**: Rotate signing keys annually; old signatures remain valid
2. **Offline verification**: Bundles are self-contained for air-gapped verification
3. **Timestamping**: Consider RFC 3161 timestamps for legal validity
4. **Multi-signature**: Support multiple signers for high-assurance bundles
5. **Redaction**: Sensitive data redacted but hash chain preserved

---

**End of EVIDENCE_FORMAT.md**
