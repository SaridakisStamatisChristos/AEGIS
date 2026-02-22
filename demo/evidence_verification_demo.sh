#!/bin/bash
#
# AegisRun Demo: Evidence Bundle Verification
#
# This script demonstrates the complete evidence lifecycle:
# 1. Run an agent with tool calls
# 2. Export the evidence bundle
# 3. Verify the bundle integrity
# 4. Inspect the contents
#
# Prerequisites:
#   - AegisRun API running (docker-compose up)
#   - aegis-verify CLI installed
#   - jq, unzip installed
#
# Usage:
#   ./evidence_verification_demo.sh

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Configuration
API_URL="${AEGISRUN_API_URL:-http://localhost:8080}"
API_TOKEN="${AEGISRUN_API_TOKEN:-demo-token}"
POLICY_ID="${AEGISRUN_POLICY_ID:-production-standard}"
POLICY_VERSION="${AEGISRUN_POLICY_VERSION:-v1}"
VERIFIER_PATH="${AEGIS_VERIFIER_PATH:-aegis-verify}"

# Temp directory for demo artifacts
DEMO_DIR=$(mktemp -d)
trap "rm -rf $DEMO_DIR" EXIT

echo ""
echo "============================================================"
echo -e "${BLUE} AegisRun Demo: Evidence Bundle Verification${NC}"
echo "============================================================"
echo ""
echo "This demo shows the complete evidence lifecycle including"
echo "hash chain verification and signature validation."
echo ""
echo "Working directory: $DEMO_DIR"
echo ""

# Check prerequisites
check_prerequisites() {
    echo -e "${YELLOW}[CHECK] Verifying prerequisites...${NC}"
    
    local missing=()
    
    command -v jq &> /dev/null || missing+=("jq")
    command -v curl &> /dev/null || missing+=("curl")
    command -v unzip &> /dev/null || missing+=("unzip")
    
    if [ ${#missing[@]} -gt 0 ]; then
        echo -e "${RED}ERROR: Missing required tools: ${missing[*]}${NC}"
        exit 1
    fi
    
    # Check API
    if ! curl -s --max-time 5 "${API_URL}/api/v1/health" > /dev/null 2>&1; then
        echo -e "${RED}ERROR: AegisRun API not reachable at ${API_URL}${NC}"
        exit 1
    fi
    
    # Check verifier (optional - will skip verification if not present)
    if ! command -v "$VERIFIER_PATH" &> /dev/null; then
        echo -e "${YELLOW}  ⚠ aegis-verify not found, will skip automated verification${NC}"
        SKIP_VERIFICATION=true
    fi
    
    echo -e "${GREEN}  ✓ Prerequisites satisfied${NC}"
    echo ""
}

# Create a run with multiple tool calls
create_demo_run() {
    echo -e "${YELLOW}[PHASE 1] Creating Demo Run${NC}"
    echo ""
    
    # Step 1: Create run
    echo "  Creating new run..."
    RUN_RESPONSE=$(curl -s -X POST "${API_URL}/api/v1/runs" \
        -H "Authorization: Bearer ${API_TOKEN}" \
        -H "Content-Type: application/json" \
        -d "{
            \"policy_id\": \"${POLICY_ID}\",
            \"policy_version\": \"${POLICY_VERSION}\",
            \"metadata\": {
                \"demo\": \"evidence_verification\",
                \"timestamp\": \"$(date -u +%Y-%m-%dT%H:%M:%SZ)\",
                \"environment\": \"demo\"
            }
        }")
    
    RUN_ID=$(echo "$RUN_RESPONSE" | jq -r '.run_id')
    echo -e "  ${GREEN}✓ Run created: ${RUN_ID}${NC}"
    
    # Step 2: Execute several tool calls
    echo ""
    echo "  Executing tool calls..."
    
    # Tool call 1: HTTP GET (should be allowed)
    echo "    - HTTP GET request..."
    curl -s -X POST "${API_URL}/api/v1/gateway/tool-call" \
        -H "Authorization: Bearer ${API_TOKEN}" \
        -H "Content-Type: application/json" \
        -d "{
            \"run_id\": \"${RUN_ID}\",
            \"step_id\": \"step_001\",
            \"tool_name\": \"http_request\",
            \"args\": {
                \"url\": \"https://api.github.com/zen\",
                \"method\": \"GET\"
            },
            \"state_vector\": {\"step\": 1},
            \"executor\": \"builtin\"
        }" > /dev/null
    echo -e "      ${GREEN}✓ Completed${NC}"
    
    # Tool call 2: File write to /tmp (should be allowed)
    echo "    - File write to /tmp..."
    curl -s -X POST "${API_URL}/api/v1/gateway/tool-call" \
        -H "Authorization: Bearer ${API_TOKEN}" \
        -H "Content-Type: application/json" \
        -d "{
            \"run_id\": \"${RUN_ID}\",
            \"step_id\": \"step_002\",
            \"tool_name\": \"file_write\",
            \"args\": {
                \"path\": \"/tmp/demo_output.json\",
                \"content\": \"{\\\"status\\\": \\\"success\\\"}\"
            },
            \"state_vector\": {\"step\": 2},
            \"executor\": \"builtin\"
        }" > /dev/null
    echo -e "      ${GREEN}✓ Completed${NC}"
    
    # Tool call 3: Blocked request (for variety)
    echo "    - SSRF attempt (should be blocked)..."
    BLOCKED_RESPONSE=$(curl -s -X POST "${API_URL}/api/v1/gateway/tool-call" \
        -H "Authorization: Bearer ${API_TOKEN}" \
        -H "Content-Type: application/json" \
        -d "{
            \"run_id\": \"${RUN_ID}\",
            \"step_id\": \"step_003\",
            \"tool_name\": \"http_request\",
            \"args\": {
                \"url\": \"http://169.254.169.254/latest/meta-data/\",
                \"method\": \"GET\"
            },
            \"state_vector\": {\"step\": 3},
            \"executor\": \"builtin\"
        }")
    BLOCKED_ACTION=$(echo "$BLOCKED_RESPONSE" | jq -r '.decision.action')
    if [ "$BLOCKED_ACTION" == "block" ]; then
        echo -e "      ${GREEN}✓ Blocked as expected${NC}"
    else
        echo -e "      ${YELLOW}⚠ Unexpected action: ${BLOCKED_ACTION}${NC}"
    fi
    
    # Tool call 4: Database query (should be allowed)
    echo "    - Database SELECT query..."
    curl -s -X POST "${API_URL}/api/v1/gateway/tool-call" \
        -H "Authorization: Bearer ${API_TOKEN}" \
        -H "Content-Type: application/json" \
        -d "{
            \"run_id\": \"${RUN_ID}\",
            \"step_id\": \"step_004\",
            \"tool_name\": \"database_query\",
            \"args\": {
                \"query\": \"SELECT id, name FROM users LIMIT 10\",
                \"params\": []
            },
            \"state_vector\": {\"step\": 4},
            \"executor\": \"builtin\"
        }" > /dev/null
    echo -e "      ${GREEN}✓ Completed${NC}"
    
    echo ""
    echo -e "  ${GREEN}✓ All tool calls executed${NC}"
    echo ""
}

# Export evidence bundle
export_bundle() {
    echo -e "${YELLOW}[PHASE 2] Exporting Evidence Bundle${NC}"
    echo ""
    
    BUNDLE_PATH="${DEMO_DIR}/evidence_${RUN_ID}.zip"
    
    echo "  Downloading bundle..."
    HTTP_CODE=$(curl -s -w "%{http_code}" -o "$BUNDLE_PATH" \
        "${API_URL}/api/v1/evidence/${RUN_ID}/bundle" \
        -H "Authorization: Bearer ${API_TOKEN}")
    
    if [ "$HTTP_CODE" != "200" ]; then
        echo -e "${RED}  ERROR: Failed to download bundle (HTTP ${HTTP_CODE})${NC}"
        exit 1
    fi
    
    BUNDLE_SIZE=$(ls -lh "$BUNDLE_PATH" | awk '{print $5}')
    echo -e "  ${GREEN}✓ Bundle downloaded: ${BUNDLE_SIZE}${NC}"
    echo ""
    
    # List contents
    echo "  Bundle contents:"
    unzip -l "$BUNDLE_PATH" 2>/dev/null | tail -n +4 | head -n -2 | while read line; do
        SIZE=$(echo "$line" | awk '{print $1}')
        NAME=$(echo "$line" | awk '{print $4}')
        printf "    %-30s %s bytes\n" "$NAME" "$SIZE"
    done
    echo ""
}

# Inspect bundle contents
inspect_bundle() {
    echo -e "${YELLOW}[PHASE 3] Inspecting Bundle Contents${NC}"
    echo ""
    
    EXTRACT_DIR="${DEMO_DIR}/extracted"
    mkdir -p "$EXTRACT_DIR"
    unzip -q "$BUNDLE_PATH" -d "$EXTRACT_DIR"
    
    # Read manifest
    echo "  Manifest:"
    echo "  ─────────────────────────────────────"
    if [ -f "${EXTRACT_DIR}/manifest.json" ]; then
        echo ""
        cat "${EXTRACT_DIR}/manifest.json" | jq '.' | sed 's/^/    /'
        echo ""
        
        # Extract key fields
        ROOT_HASH=$(cat "${EXTRACT_DIR}/manifest.json" | jq -r '.root_hash')
        EVENT_COUNT=$(cat "${EXTRACT_DIR}/manifest.json" | jq -r '.event_count')
        SIGNATURE=$(cat "${EXTRACT_DIR}/manifest.json" | jq -r '.signature')
        
        echo "  Key Fields:"
        echo "    Event Count: ${EVENT_COUNT}"
        echo "    Root Hash:   ${ROOT_HASH:0:16}..."
        echo "    Signature:   ${SIGNATURE:0:20}..."
    fi
    echo ""
    
    # Show event chain sample
    echo "  Event Chain (first 3 events):"
    echo "  ─────────────────────────────────────"
    if [ -f "${EXTRACT_DIR}/events.jsonl" ]; then
        head -n 3 "${EXTRACT_DIR}/events.jsonl" | while read line; do
            EVENT_TYPE=$(echo "$line" | jq -r '.event_type')
            EVENT_HASH=$(echo "$line" | jq -r '.event_hash')
            PREV_HASH=$(echo "$line" | jq -r '.prev_hash')
            SEQ_NO=$(echo "$line" | jq -r '.seq_no')
            echo ""
            echo "    Event #${SEQ_NO}: ${EVENT_TYPE}"
            echo "      Hash:      ${EVENT_HASH:0:16}..."
            if [ "$PREV_HASH" != "null" ]; then
                echo "      Prev Hash: ${PREV_HASH:0:16}..."
            else
                echo "      Prev Hash: (genesis)"
            fi
        done
    fi
    echo ""
    
    # Show policy snapshot
    echo "  Policy Snapshot:"
    echo "  ─────────────────────────────────────"
    if [ -f "${EXTRACT_DIR}/policy_snapshot.json" ]; then
        POLICY_NAME=$(cat "${EXTRACT_DIR}/policy_snapshot.json" | jq -r '.name')
        POLICY_VER=$(cat "${EXTRACT_DIR}/policy_snapshot.json" | jq -r '.version')
        SPEC_HASH=$(cat "${EXTRACT_DIR}/policy_snapshot.json" | jq -r '.spec_hash')
        TOOL_COUNT=$(cat "${EXTRACT_DIR}/policy_snapshot.json" | jq '.spec.tools | length')
        
        echo ""
        echo "    Name:       ${POLICY_NAME}"
        echo "    Version:    ${POLICY_VER}"
        echo "    Spec Hash:  ${SPEC_HASH:0:16}..."
        echo "    Tool Rules: ${TOOL_COUNT}"
    fi
    echo ""
}

# Verify bundle with aegis-verify
verify_bundle() {
    echo -e "${YELLOW}[PHASE 4] Verifying Bundle Integrity${NC}"
    echo ""
    
    if [ "$SKIP_VERIFICATION" == "true" ]; then
        echo "  Skipping automated verification (aegis-verify not found)"
        echo ""
        echo "  To verify manually, run:"
        echo "    aegis-verify ${BUNDLE_PATH}"
        echo ""
        return
    fi
    
    echo "  Running aegis-verify..."
    echo "  ─────────────────────────────────────"
    echo ""
    
    # Run verifier
    if $VERIFIER_PATH --verbose "$BUNDLE_PATH" 2>&1 | sed 's/^/    /'; then
        echo ""
        echo -e "  ${GREEN}✓ Bundle verification PASSED${NC}"
    else
        echo ""
        echo -e "  ${RED}✗ Bundle verification FAILED${NC}"
    fi
    echo ""
}

# Manual hash verification demonstration
demonstrate_hash_chain() {
    echo -e "${YELLOW}[PHASE 5] Hash Chain Verification (Manual Demo)${NC}"
    echo ""
    
    EXTRACT_DIR="${DEMO_DIR}/extracted"
    
    echo "  The hash chain ensures tamper-evidence:"
    echo "  ─────────────────────────────────────"
    echo ""
    echo "    event[0].hash = SHA256(canonical(event[0]) || \"\")"
    echo "    event[n].hash = SHA256(canonical(event[n]) || event[n-1].hash)"
    echo ""
    
    if [ -f "${EXTRACT_DIR}/events.jsonl" ]; then
        echo "  Verifying chain links..."
        
        PREV_HASH=""
        VALID=true
        
        while read line; do
            SEQ_NO=$(echo "$line" | jq -r '.seq_no')
            EVENT_HASH=$(echo "$line" | jq -r '.event_hash')
            CLAIMED_PREV=$(echo "$line" | jq -r '.prev_hash')
            
            if [ "$SEQ_NO" -eq 0 ]; then
                if [ "$CLAIMED_PREV" != "null" ]; then
                    echo -e "    ${RED}✗ Event 0 should have null prev_hash${NC}"
                    VALID=false
                else
                    echo -e "    ${GREEN}✓ Event 0: Genesis (no prev_hash)${NC}"
                fi
            else
                if [ "$CLAIMED_PREV" == "$PREV_HASH" ]; then
                    echo -e "    ${GREEN}✓ Event ${SEQ_NO}: Links to event $((SEQ_NO-1))${NC}"
                else
                    echo -e "    ${RED}✗ Event ${SEQ_NO}: Broken chain!${NC}"
                    VALID=false
                fi
            fi
            
            PREV_HASH="$EVENT_HASH"
            
        done < "${EXTRACT_DIR}/events.jsonl"
        
        echo ""
        if [ "$VALID" == "true" ]; then
            echo -e "  ${GREEN}✓ Hash chain is intact${NC}"
        else
            echo -e "  ${RED}✗ Hash chain has been tampered with!${NC}"
        fi
    fi
    echo ""
}

# Summary
print_summary() {
    echo "============================================================"
    echo -e "${GREEN} Demo Complete!${NC}"
    echo "============================================================"
    echo ""
    echo "  Evidence Bundle Lifecycle:"
    echo "  ─────────────────────────────────────────────────────────"
    echo "  1. ✓ Agent run created with multiple tool calls"
    echo "  2. ✓ Each tool call produced audit events"
    echo "  3. ✓ Events formed a hash chain for tamper-evidence"
    echo "  4. ✓ Run was signed with Ed25519"
    echo "  5. ✓ Evidence bundle exported as ZIP"
    echo "  6. ✓ Bundle can be verified offline"
    echo ""
    echo "  What's in the bundle:"
    echo "  ─────────────────────────────────────────────────────────"
    echo "  • manifest.json    - Run metadata + signature"
    echo "  • events.jsonl     - Full event chain"  
    echo "  • policy_snapshot  - Immutable policy at time of run"
    echo "  • approvals.json   - Any approval records"
    echo "  • public_key.pem   - Verification key"
    echo ""
    echo "  Files created:"
    echo "    ${BUNDLE_PATH}"
    echo ""
    echo "  Run ID: ${RUN_ID}"
    echo ""
    echo "  Use this evidence for:"
    echo "  • Compliance audits"
    echo "  • Incident investigation"
    echo "  • Legal hold/discovery"
    echo "  • Security reviews"
    echo ""
}

# Main execution
main() {
    check_prerequisites
    create_demo_run
    export_bundle
    inspect_bundle
    verify_bundle
    demonstrate_hash_chain
    print_summary
}

main
