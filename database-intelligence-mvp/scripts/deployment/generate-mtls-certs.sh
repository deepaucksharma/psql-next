#!/bin/bash
# Generate mTLS certificates for secure OTel collector communication

set -euo pipefail

# Configuration
CERT_DIR="${CERT_DIR:-./certs}"
VALIDITY_DAYS="${VALIDITY_DAYS:-365}"
KEY_SIZE="${KEY_SIZE:-4096}"
NAMESPACE="${NAMESPACE:-otel}"
CLUSTER_DOMAIN="${CLUSTER_DOMAIN:-svc.cluster.local}"

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${GREEN}=== Generating mTLS Certificates for OTel Collectors ===${NC}"

# Create certificate directory
mkdir -p "$CERT_DIR"
cd "$CERT_DIR"

# Generate CA private key
echo -e "${YELLOW}Generating CA private key...${NC}"
openssl genrsa -out ca-key.pem $KEY_SIZE

# Generate CA certificate
echo -e "${YELLOW}Generating CA certificate...${NC}"
cat > ca-cert.conf <<EOF
[req]
distinguished_name = req_distinguished_name
x509_extensions = v3_ca
prompt = no

[req_distinguished_name]
C = US
ST = California
L = San Francisco
O = Database Intelligence
OU = Observability Platform
CN = OTel Collector CA

[v3_ca]
basicConstraints = critical,CA:TRUE
keyUsage = critical,digitalSignature,keyCertSign,cRLSign
subjectKeyIdentifier = hash
authorityKeyIdentifier = keyid:always,issuer
EOF

openssl req -new -x509 -days $VALIDITY_DAYS -key ca-key.pem -out ca-cert.pem -config ca-cert.conf

# Function to generate certificate
generate_cert() {
    local name=$1
    local cn=$2
    local san=$3
    
    echo -e "${YELLOW}Generating certificate for $name...${NC}"
    
    # Generate private key
    openssl genrsa -out "$name-key.pem" $KEY_SIZE
    
    # Generate certificate request
    cat > "$name.conf" <<EOF
[req]
distinguished_name = req_distinguished_name
req_extensions = v3_req
prompt = no

[req_distinguished_name]
C = US
ST = California
L = San Francisco
O = Database Intelligence
OU = Observability Platform
CN = $cn

[v3_req]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth, clientAuth
subjectAltName = $san
EOF

    # Generate CSR
    openssl req -new -key "$name-key.pem" -out "$name.csr" -config "$name.conf"
    
    # Sign certificate
    openssl x509 -req -in "$name.csr" -CA ca-cert.pem -CAkey ca-key.pem \
        -CAcreateserial -out "$name-cert.pem" -days $VALIDITY_DAYS \
        -extensions v3_req -extfile "$name.conf"
    
    # Clean up
    rm "$name.csr" "$name.conf"
    
    # Set appropriate permissions
    chmod 400 "$name-key.pem"
    chmod 444 "$name-cert.pem"
}

# Generate Gateway certificate (acts as both server and client)
generate_cert "gateway" \
    "otel-gateway.$NAMESPACE.$CLUSTER_DOMAIN" \
    "DNS:otel-gateway,DNS:otel-gateway.$NAMESPACE,DNS:otel-gateway.$NAMESPACE.$CLUSTER_DOMAIN,DNS:localhost,IP:127.0.0.1"

# Generate Agent certificate (client only)
generate_cert "agent" \
    "otel-agent.$NAMESPACE.$CLUSTER_DOMAIN" \
    "DNS:otel-agent,DNS:otel-agent.$NAMESPACE,DNS:otel-agent.$NAMESPACE.$CLUSTER_DOMAIN,DNS:localhost,IP:127.0.0.1"

# Generate additional gateway client certificate for gateway-to-gateway communication
generate_cert "gateway-client" \
    "otel-gateway-client.$NAMESPACE.$CLUSTER_DOMAIN" \
    "DNS:otel-gateway-client,DNS:otel-gateway-client.$NAMESPACE,DNS:otel-gateway-client.$NAMESPACE.$CLUSTER_DOMAIN"

# Create verification script
cat > verify-certs.sh <<'EOF'
#!/bin/bash
# Verify certificate chain

echo "=== Certificate Chain Verification ==="

# Verify CA certificate
echo "CA Certificate:"
openssl x509 -in ca-cert.pem -noout -subject -issuer -dates

# Verify Gateway certificate
echo -e "\nGateway Certificate:"
openssl verify -CAfile ca-cert.pem gateway-cert.pem
openssl x509 -in gateway-cert.pem -noout -subject -ext subjectAltName -dates

# Verify Agent certificate
echo -e "\nAgent Certificate:"
openssl verify -CAfile ca-cert.pem agent-cert.pem
openssl x509 -in agent-cert.pem -noout -subject -ext subjectAltName -dates

# Test TLS connection (requires gateway to be running)
echo -e "\nTo test mTLS connection:"
echo "openssl s_client -connect localhost:4317 -cert agent-cert.pem -key agent-key.pem -CAfile ca-cert.pem -showcerts"
EOF

chmod +x verify-certs.sh

# Create Kubernetes secret manifest
cat > mtls-secrets.yaml <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: otel-mtls-certs
  namespace: $NAMESPACE
type: Opaque
data:
  ca-cert.pem: $(base64 -w0 < ca-cert.pem)
  gateway-cert.pem: $(base64 -w0 < gateway-cert.pem)
  gateway-key.pem: $(base64 -w0 < gateway-key.pem)
  agent-cert.pem: $(base64 -w0 < agent-cert.pem)
  agent-key.pem: $(base64 -w0 < agent-key.pem)
  gateway-client-cert.pem: $(base64 -w0 < gateway-client-cert.pem)
  gateway-client-key.pem: $(base64 -w0 < gateway-client-key.pem)
EOF

# Create certificate renewal script
cat > renew-certs.sh <<'EOF'
#!/bin/bash
# Renew certificates before expiry

DAYS_BEFORE_EXPIRY=${DAYS_BEFORE_EXPIRY:-30}

check_cert_expiry() {
    local cert=$1
    local expiry_date=$(openssl x509 -in "$cert" -noout -enddate | cut -d= -f2)
    local expiry_epoch=$(date -d "$expiry_date" +%s)
    local current_epoch=$(date +%s)
    local days_left=$(( (expiry_epoch - current_epoch) / 86400 ))
    
    echo "$cert expires in $days_left days"
    
    if [ $days_left -lt $DAYS_BEFORE_EXPIRY ]; then
        echo "WARNING: $cert needs renewal!"
        return 1
    fi
    return 0
}

# Check all certificates
for cert in *-cert.pem; do
    check_cert_expiry "$cert"
done
EOF

chmod +x renew-certs.sh

# Summary
echo -e "${GREEN}=== Certificate Generation Complete ===${NC}"
echo "Generated files in $CERT_DIR:"
echo "  - ca-cert.pem, ca-key.pem (CA certificate and key)"
echo "  - gateway-cert.pem, gateway-key.pem (Gateway server certificate)"
echo "  - agent-cert.pem, agent-key.pem (Agent client certificate)"
echo "  - gateway-client-cert.pem, gateway-client-key.pem (Gateway client certificate)"
echo "  - mtls-secrets.yaml (Kubernetes secret manifest)"
echo "  - verify-certs.sh (Certificate verification script)"
echo "  - renew-certs.sh (Certificate renewal check script)"
echo ""
echo "To deploy to Kubernetes:"
echo "  kubectl apply -f $CERT_DIR/mtls-secrets.yaml"
echo ""
echo "To verify certificates:"
echo "  cd $CERT_DIR && ./verify-certs.sh"