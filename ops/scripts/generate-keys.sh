#!/usr/bin/env bash
# AegisRun Ed25519 Key Generation Script
# Generates signing keys for evidence bundles

set -euo pipefail

# Configuration
KEYS_DIR="${KEYS_DIR:-./keys}"
KEY_NAME="${KEY_NAME:-aegisrun-signing}"
KEY_ID="${KEY_ID:-$(date +%Y%m%d%H%M%S)}"
OUTPUT_FORMAT="${OUTPUT_FORMAT:-pem}"  # pem, raw, both

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check for required tools
check_dependencies() {
    log_info "Checking dependencies..."
    
    local has_openssl=false
    local has_python=false
    
    if command -v openssl &> /dev/null; then
        has_openssl=true
        OPENSSL_VERSION=$(openssl version)
        log_info "Found OpenSSL: ${OPENSSL_VERSION}"
    fi
    
    if command -v python3 &> /dev/null; then
        has_python=true
        PYTHON_VERSION=$(python3 --version)
        log_info "Found Python: ${PYTHON_VERSION}"
    fi
    
    if [ "$has_openssl" = false ] && [ "$has_python" = false ]; then
        log_error "Neither OpenSSL (1.1.1+) nor Python3 found. Please install one."
        exit 1
    fi
    
    # Prefer OpenSSL if available and supports Ed25519
    if [ "$has_openssl" = true ]; then
        if openssl genpkey -algorithm ed25519 2>/dev/null | head -1 | grep -q "BEGIN"; then
            USE_OPENSSL=true
            log_success "Using OpenSSL for key generation"
        else
            USE_OPENSSL=false
            log_warn "OpenSSL doesn't support Ed25519, falling back to Python"
        fi
    else
        USE_OPENSSL=false
    fi
}

# Create keys directory
setup_directory() {
    log_info "Setting up keys directory: ${KEYS_DIR}"
    
    if [ ! -d "${KEYS_DIR}" ]; then
        mkdir -p "${KEYS_DIR}"
        chmod 700 "${KEYS_DIR}"
        log_success "Created keys directory"
    else
        log_info "Keys directory already exists"
    fi
}

# Generate keys using OpenSSL
generate_keys_openssl() {
    log_info "Generating Ed25519 key pair using OpenSSL..."
    
    local private_key_file="${KEYS_DIR}/${KEY_NAME}-${KEY_ID}.key"
    local public_key_file="${KEYS_DIR}/${KEY_NAME}-${KEY_ID}.pub"
    
    # Check if key already exists
    if [ -f "${private_key_file}" ]; then
        log_warn "Key already exists: ${private_key_file}"
        read -r -p "Overwrite? (y/N): " confirm
        if [[ ! "$confirm" =~ ^[Yy]$ ]]; then
            log_info "Key generation cancelled"
            exit 0
        fi
    fi
    
    # Generate private key
    openssl genpkey -algorithm ed25519 -out "${private_key_file}"
    chmod 600 "${private_key_file}"
    log_success "Private key generated: ${private_key_file}"
    
    # Extract public key
    openssl pkey -in "${private_key_file}" -pubout -out "${public_key_file}"
    chmod 644 "${public_key_file}"
    log_success "Public key generated: ${public_key_file}"
    
    # Generate raw format if requested
    if [ "$OUTPUT_FORMAT" = "raw" ] || [ "$OUTPUT_FORMAT" = "both" ]; then
        generate_raw_format "${private_key_file}" "${public_key_file}"
    fi
    
    PRIVATE_KEY_FILE="${private_key_file}"
    PUBLIC_KEY_FILE="${public_key_file}"
}

# Generate keys using Python
generate_keys_python() {
    log_info "Generating Ed25519 key pair using Python..."
    
    local private_key_file="${KEYS_DIR}/${KEY_NAME}-${KEY_ID}.key"
    local public_key_file="${KEYS_DIR}/${KEY_NAME}-${KEY_ID}.pub"
    
    # Check if key already exists
    if [ -f "${private_key_file}" ]; then
        log_warn "Key already exists: ${private_key_file}"
        read -r -p "Overwrite? (y/N): " confirm
        if [[ ! "$confirm" =~ ^[Yy]$ ]]; then
            log_info "Key generation cancelled"
            exit 0
        fi
    fi
    
    python3 << EOF
import sys
try:
    from cryptography.hazmat.primitives import serialization
    from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
except ImportError:
    print("Installing cryptography package...")
    import subprocess
    subprocess.check_call([sys.executable, "-m", "pip", "install", "cryptography>=41.0.0"])
    from cryptography.hazmat.primitives import serialization
    from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey

# Generate key pair
private_key = Ed25519PrivateKey.generate()
public_key = private_key.public_key()

# Serialize private key (PEM format)
private_pem = private_key.private_bytes(
    encoding=serialization.Encoding.PEM,
    format=serialization.PrivateFormat.PKCS8,
    encryption_algorithm=serialization.NoEncryption()
)

# Serialize public key (PEM format)
public_pem = public_key.public_bytes(
    encoding=serialization.Encoding.PEM,
    format=serialization.PublicFormat.SubjectPublicKeyInfo
)

# Write keys to files
with open("${private_key_file}", "wb") as f:
    f.write(private_pem)

with open("${public_key_file}", "wb") as f:
    f.write(public_pem)

print("Keys generated successfully")
EOF
    
    chmod 600 "${private_key_file}"
    chmod 644 "${public_key_file}"
    
    log_success "Private key generated: ${private_key_file}"
    log_success "Public key generated: ${public_key_file}"
    
    # Generate raw format if requested
    if [ "$OUTPUT_FORMAT" = "raw" ] || [ "$OUTPUT_FORMAT" = "both" ]; then
        generate_raw_format_python "${private_key_file}" "${public_key_file}"
    fi
    
    PRIVATE_KEY_FILE="${private_key_file}"
    PUBLIC_KEY_FILE="${public_key_file}"
}

# Generate raw (binary/base64) format
generate_raw_format() {
    local private_key_file="$1"
    local public_key_file="$2"
    
    log_info "Generating raw format keys..."
    
    local private_raw="${KEYS_DIR}/${KEY_NAME}-${KEY_ID}.raw.key"
    local public_raw="${KEYS_DIR}/${KEY_NAME}-${KEY_ID}.raw.pub"
    local private_b64="${KEYS_DIR}/${KEY_NAME}-${KEY_ID}.b64.key"
    local public_b64="${KEYS_DIR}/${KEY_NAME}-${KEY_ID}.b64.pub"
    
    # Extract raw keys and encode to base64
    openssl pkey -in "${private_key_file}" -outform DER | tail -c 32 > "${private_raw}"
    openssl pkey -in "${private_key_file}" -pubout -outform DER | tail -c 32 > "${public_raw}"
    
    base64 < "${private_raw}" > "${private_b64}"
    base64 < "${public_raw}" > "${public_b64}"
    
    chmod 600 "${private_raw}" "${private_b64}"
    chmod 644 "${public_raw}" "${public_b64}"
    
    log_success "Raw format keys generated"
}

# Generate raw format using Python
generate_raw_format_python() {
    local private_key_file="$1"
    local public_key_file="$2"
    
    log_info "Generating raw format keys using Python..."
    
    python3 << EOF
import base64
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey

# Load private key
with open("${private_key_file}", "rb") as f:
    private_key = serialization.load_pem_private_key(f.read(), password=None)

# Get raw bytes
private_raw = private_key.private_bytes(
    encoding=serialization.Encoding.Raw,
    format=serialization.PrivateFormat.Raw,
    encryption_algorithm=serialization.NoEncryption()
)

public_raw = private_key.public_key().public_bytes(
    encoding=serialization.Encoding.Raw,
    format=serialization.PublicFormat.Raw
)

# Write raw files
with open("${KEYS_DIR}/${KEY_NAME}-${KEY_ID}.raw.key", "wb") as f:
    f.write(private_raw)

with open("${KEYS_DIR}/${KEY_NAME}-${KEY_ID}.raw.pub", "wb") as f:
    f.write(public_raw)

# Write base64 files
with open("${KEYS_DIR}/${KEY_NAME}-${KEY_ID}.b64.key", "w") as f:
    f.write(base64.b64encode(private_raw).decode())

with open("${KEYS_DIR}/${KEY_NAME}-${KEY_ID}.b64.pub", "w") as f:
    f.write(base64.b64encode(public_raw).decode())

print("Raw format keys generated")
EOF
    
    chmod 600 "${KEYS_DIR}/${KEY_NAME}-${KEY_ID}.raw.key" "${KEYS_DIR}/${KEY_NAME}-${KEY_ID}.b64.key"
    chmod 644 "${KEYS_DIR}/${KEY_NAME}-${KEY_ID}.raw.pub" "${KEYS_DIR}/${KEY_NAME}-${KEY_ID}.b64.pub"
    
    log_success "Raw format keys generated"
}

# Create symlinks to latest key
create_symlinks() {
    log_info "Creating symlinks to latest key..."
    
    local latest_private="${KEYS_DIR}/${KEY_NAME}.key"
    local latest_public="${KEYS_DIR}/${KEY_NAME}.pub"
    
    # Remove old symlinks
    rm -f "${latest_private}" "${latest_public}"
    
    # Create new symlinks
    ln -s "$(basename "${PRIVATE_KEY_FILE}")" "${latest_private}"
    ln -s "$(basename "${PUBLIC_KEY_FILE}")" "${latest_public}"
    
    log_success "Symlinks created: ${latest_private}, ${latest_public}"
}

# Generate key metadata
generate_metadata() {
    log_info "Generating key metadata..."
    
    local metadata_file="${KEYS_DIR}/${KEY_NAME}-${KEY_ID}.meta.json"
    
    cat > "${metadata_file}" << EOF
{
    "key_id": "${KEY_ID}",
    "key_name": "${KEY_NAME}",
    "algorithm": "Ed25519",
    "format": "${OUTPUT_FORMAT}",
    "created_at": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
    "created_by": "$(whoami)",
    "hostname": "$(hostname)",
    "private_key_file": "${KEY_NAME}-${KEY_ID}.key",
    "public_key_file": "${KEY_NAME}-${KEY_ID}.pub",
    "status": "active",
    "notes": "AegisRun evidence signing key"
}
EOF
    
    chmod 644 "${metadata_file}"
    log_success "Metadata generated: ${metadata_file}"
}

# Print key info
print_key_info() {
    echo ""
    echo "============================================"
    echo "  Ed25519 Key Generation Complete"
    echo "============================================"
    echo ""
    echo "Key ID:       ${KEY_ID}"
    echo "Key Name:     ${KEY_NAME}"
    echo "Algorithm:    Ed25519"
    echo "Directory:    ${KEYS_DIR}"
    echo ""
    echo "Files generated:"
    echo "  Private key: ${PRIVATE_KEY_FILE}"
    echo "  Public key:  ${PUBLIC_KEY_FILE}"
    
    if [ "$OUTPUT_FORMAT" = "raw" ] || [ "$OUTPUT_FORMAT" = "both" ]; then
        echo "  Raw private: ${KEYS_DIR}/${KEY_NAME}-${KEY_ID}.raw.key"
        echo "  Raw public:  ${KEYS_DIR}/${KEY_NAME}-${KEY_ID}.raw.pub"
        echo "  B64 private: ${KEYS_DIR}/${KEY_NAME}-${KEY_ID}.b64.key"
        echo "  B64 public:  ${KEYS_DIR}/${KEY_NAME}-${KEY_ID}.b64.pub"
    fi
    
    echo ""
    echo "Public key (PEM):"
    cat "${PUBLIC_KEY_FILE}"
    echo ""
    
    log_warn "IMPORTANT: Keep the private key secure!"
    log_warn "Never commit private keys to version control."
    echo ""
}

# Print usage
print_usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  -d, --dir DIR       Keys directory (default: ./keys)"
    echo "  -n, --name NAME     Key name prefix (default: aegisrun-signing)"
    echo "  -i, --id ID         Key ID (default: timestamp)"
    echo "  -f, --format FMT    Output format: pem, raw, both (default: pem)"
    echo "  -h, --help          Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0                           # Generate with defaults"
    echo "  $0 -d /etc/aegisrun/keys     # Custom directory"
    echo "  $0 -n prod-signing -f both   # Custom name and format"
}

# Parse arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            -d|--dir)
                KEYS_DIR="$2"
                shift 2
                ;;
            -n|--name)
                KEY_NAME="$2"
                shift 2
                ;;
            -i|--id)
                KEY_ID="$2"
                shift 2
                ;;
            -f|--format)
                OUTPUT_FORMAT="$2"
                shift 2
                ;;
            -h|--help)
                print_usage
                exit 0
                ;;
            *)
                log_error "Unknown option: $1"
                print_usage
                exit 1
                ;;
        esac
    done
}

# Main execution
main() {
    parse_args "$@"
    
    echo ""
    echo "============================================"
    echo "  AegisRun Ed25519 Key Generation"
    echo "============================================"
    echo ""
    
    check_dependencies
    setup_directory
    
    if [ "$USE_OPENSSL" = true ]; then
        generate_keys_openssl
    else
        generate_keys_python
    fi
    
    create_symlinks
    generate_metadata
    print_key_info
}

# Run main function
main "$@"
