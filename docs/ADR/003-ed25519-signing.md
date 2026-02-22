# ADR-003: Ed25519 for Evidence Signing

**Status**: Accepted  
**Date**: 2026-02-03  
**Authors**: AegisRun Team

---

## Context

AegisRun evidence bundles require digital signatures for:

1. **Integrity**: Detect tampering with historical records
2. **Non-repudiation**: Prove AegisRun generated the bundle
3. **Compliance**: Meet audit requirements (SOC 2, etc.)

We needed to choose a cryptographic signing algorithm.

## Decision

We use **Ed25519** (Edwards-curve Digital Signature Algorithm) for all evidence signatures.

## Rationale

### Why Ed25519

1. **Security Strength**
   - 128-bit security level (equivalent to RSA-3072)
   - Resistant to timing attacks by design
   - No known practical attacks
   - Recommended by NIST (as part of EdDSA family)

2. **Performance**
   - Sign: ~70,000 signatures/second on modern hardware
   - Verify: ~25,000 verifications/second
   - Negligible CPU impact vs. RSA

3. **Key Size**
   - Private key: 32 bytes
   - Public key: 32 bytes
   - Signature: 64 bytes
   - Compact storage and transmission

4. **Simplicity**
   - No parameters to choose (curve is fixed)
   - Deterministic signatures (same input → same signature)
   - Widely supported in standard libraries

5. **Modern Standard**
   - Used by: SSH, TLS 1.3, WireGuard, Signal, Tor
   - Go standard library support (`crypto/ed25519`)
   - OpenSSL support since 1.1.1

### Implementation Details

```go
import (
    "crypto/ed25519"
    "encoding/base64"
)

// Key generation
publicKey, privateKey, _ := ed25519.GenerateKey(rand.Reader)

// Signing
message := []byte("aegisrun-evidence-v1\n" + bundleID + "\n" + merkleRoot)
signature := ed25519.Sign(privateKey, message)

// Verification
valid := ed25519.Verify(publicKey, message, signature)
```

### Signature Format

```
Message:
  aegisrun-evidence-v1
  {bundle_id}
  {merkle_root}
  {timestamp}

Signature: base64-encoded 64-byte Ed25519 signature
```

## Consequences

### Positive

- Fast signing (70K/s) enables real-time bundle generation
- Small signatures (64 bytes) minimize storage overhead
- No algorithm configuration reduces misconfiguration risk
- Broad library support simplifies offline verification

### Negative

- No quantum resistance (neither does RSA/ECDSA)
- Key rotation requires public key distribution
- Cannot use HSM that only supports RSA/ECDSA

### Mitigations

- Document key rotation procedures
- Support multiple public keys for verification (key not deprecated until grace period)
- Consider Ed448 or hybrid schemes for post-quantum future

## Alternatives Considered

### RSA-2048/4096

Rejected because:
- Much slower signing (~1000/s for RSA-2048)
- Larger keys (256+ bytes public, 2048+ signature)
- More complex parameter selection
- Variable timing attacks possible with naive implementations

### ECDSA (P-256)

Rejected because:
- Requires secure random for each signature (k-value)
- Historical implementation vulnerabilities (Sony PlayStation hack)
- More complex than Ed25519
- Slightly slower

### HMAC (Symmetric)

Rejected because:
- Cannot share verification capability without sharing signing capability
- Anyone with key can forge signatures
- No non-repudiation properties

## Key Management

### Storage

```yaml
# Production: Use secrets manager
SIGNING_KEY_PATH: vault://secret/aegisrun/signing-key

# Development: File-based
SIGNING_KEY_PATH: /keys/signing.key
```

### Rotation Procedure

1. Generate new key pair
2. Add new public key to verifier's trusted keys
3. Start signing with new private key
4. After grace period (e.g., 90 days), remove old public key
5. Securely destroy old private key

### Backup

- Private key must be backed up securely (encrypted, offline)
- Public key can be distributed freely
- Consider using key splitting (Shamir's Secret Sharing) for private key backup

## Related Decisions

- [ADR-001: Single Binary API](001-single-binary-api.md)
- [ADR-002: PostgreSQL as Queue](002-postgres-as-queue.md)
- [ADR-004: CEL Subset for Policy Expressions](004-cel-subset.md)

---

**End of ADR-003**
