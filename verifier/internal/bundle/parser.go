// Package bundle provides evidence bundle loading and parsing
package bundle

import (
	"archive/zip"
	"bufio"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"strings"
)

// parseManifest parses the manifest.json file from the bundle
func parseManifest(file *zip.File) (*Manifest, error) {
	rc, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	var manifest Manifest
	if err := json.NewDecoder(rc).Decode(&manifest); err != nil {
		return nil, err
	}

	return &manifest, nil
}

// parseEvents parses the events.jsonl file (JSON Lines format)
func parseEvents(file *zip.File) ([]Event, error) {
	rc, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	var events []Event
	scanner := bufio.NewScanner(rc)

	// Increase scanner buffer for large events
	const maxCapacity = 10 * 1024 * 1024 // 10MB
	buf := make([]byte, maxCapacity)
	scanner.Buffer(buf, maxCapacity)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		var event Event
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return nil, fmt.Errorf("parse event at line %d: %w", lineNum, err)
		}
		events = append(events, event)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read events: %w", err)
	}

	return events, nil
}

// parsePolicy parses the policy_snapshot.json file
func parsePolicy(file *zip.File) (*Policy, error) {
	rc, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	var policy Policy
	if err := json.NewDecoder(rc).Decode(&policy); err != nil {
		return nil, err
	}

	return &policy, nil
}

// parseRun parses the run.json file
func parseRun(file *zip.File) (*Run, error) {
	rc, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	var run Run
	if err := json.NewDecoder(rc).Decode(&run); err != nil {
		return nil, err
	}

	return &run, nil
}

// parsePublicKey parses a PEM-encoded Ed25519 public key.
// It first attempts PKIX/ASN.1 parsing (standard 44-byte DER), then falls
// back to raw 32-byte Ed25519 key bytes for non-PKIX PEM files.
func parsePublicKey(file *zip.File) (ed25519.PublicKey, error) {
	rc, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, err
	}

	// Try standard PEM + PKIX parsing first
	block, _ := pem.Decode(data)
	if block != nil {
		pub, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err == nil {
			if edKey, ok := pub.(ed25519.PublicKey); ok {
				return edKey, nil
			}
			return nil, fmt.Errorf("PKIX key is not Ed25519")
		}
		// PKIX parse failed; the DER bytes may be a raw 32-byte key
		if len(block.Bytes) == ed25519.PublicKeySize {
			return ed25519.PublicKey(block.Bytes), nil
		}
		return nil, fmt.Errorf("unsupported PEM key: PKIX parse failed (%v) and size %d is not %d",
			err, len(block.Bytes), ed25519.PublicKeySize)
	}

	// Fallback: manual base64 stripping for bare PEM without proper headers
	pemStr := string(data)
	pemStr = strings.TrimPrefix(pemStr, "-----BEGIN PUBLIC KEY-----")
	pemStr = strings.TrimSuffix(pemStr, "-----END PUBLIC KEY-----")
	pemStr = strings.TrimSpace(pemStr)
	pemStr = strings.ReplaceAll(pemStr, "\n", "")
	pemStr = strings.ReplaceAll(pemStr, "\r", "")

	keyBytes, err := base64.StdEncoding.DecodeString(pemStr)
	if err != nil {
		return nil, fmt.Errorf("decode base64: %w", err)
	}

	// Try PKIX on the raw decoded bytes as well
	pub, pkixErr := x509.ParsePKIXPublicKey(keyBytes)
	if pkixErr == nil {
		if edKey, ok := pub.(ed25519.PublicKey); ok {
			return edKey, nil
		}
		return nil, fmt.Errorf("PKIX key is not Ed25519")
	}

	// Final fallback: raw 32-byte key
	if len(keyBytes) == ed25519.PublicKeySize {
		return ed25519.PublicKey(keyBytes), nil
	}

	return nil, fmt.Errorf("invalid public key: PKIX parse failed (%v) and size %d is not %d",
		pkixErr, len(keyBytes), ed25519.PublicKeySize)
}
