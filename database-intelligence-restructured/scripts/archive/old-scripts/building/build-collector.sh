#!/bin/bash
# Build script for Database Intelligence OpenTelemetry Collector

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Variables
BUILDER_VERSION="0.105.0"
BUILDER_BINARY="ocb"
CONFIG_FILE="otelcol-builder-config-complete.yaml"
OUTPUT_DIR="distributions/production"

echo -e "${GREEN}Database Intelligence Collector Build Script${NC}"
echo "============================================"

# Function to check if a command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Check for Go
if ! command_exists go; then
    echo -e "${RED}Error: Go is not installed${NC}"
    echo "Please install Go 1.21 or later from https://golang.org/dl/"
    exit 1
fi

# Check Go version
GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
REQUIRED_VERSION="1.21"
if [ "$(printf '%s\n' "$REQUIRED_VERSION" "$GO_VERSION" | sort -V | head -n1)" != "$REQUIRED_VERSION" ]; then
    echo -e "${RED}Error: Go version $REQUIRED_VERSION or later is required${NC}"
    echo "Current version: $GO_VERSION"
    exit 1
fi

# Install or update OpenTelemetry Collector Builder
echo -e "${YELLOW}Installing OpenTelemetry Collector Builder v${BUILDER_VERSION}...${NC}"
go install go.opentelemetry.io/collector/cmd/builder@v${BUILDER_VERSION}

# Rename to ocb for convenience
if command_exists builder; then
    BUILDER_BINARY="builder"
fi

# Verify builder installation
if ! command_exists $BUILDER_BINARY; then
    echo -e "${RED}Error: OpenTelemetry Collector Builder not found${NC}"
    echo "Please ensure $GOPATH/bin is in your PATH"
    exit 1
fi

# Clean previous builds
echo -e "${YELLOW}Cleaning previous builds...${NC}"
rm -rf ${OUTPUT_DIR}/otelcol-*

# Update go.mod and go.sum in workspace
echo -e "${YELLOW}Updating Go modules...${NC}"
go mod tidy

# Build the collector
echo -e "${YELLOW}Building Database Intelligence Collector...${NC}"
echo "Using configuration: ${CONFIG_FILE}"

$BUILDER_BINARY \
    --config="${CONFIG_FILE}" \
    --skip-compilation=false \
    --skip-get-modules=false

# Check if build was successful
if [ -f "${OUTPUT_DIR}/database-intelligence-collector" ]; then
    echo -e "${GREEN}Build successful!${NC}"
    echo "Binary location: ${OUTPUT_DIR}/database-intelligence-collector"
    
    # Make binary executable
    chmod +x "${OUTPUT_DIR}/database-intelligence-collector"
    
    # Show binary info
    echo -e "\n${YELLOW}Binary information:${NC}"
    ls -lh "${OUTPUT_DIR}/database-intelligence-collector"
    
    # Show available components
    echo -e "\n${YELLOW}Available components:${NC}"
    "${OUTPUT_DIR}/database-intelligence-collector" components
else
    echo -e "${RED}Build failed!${NC}"
    echo "Check the build logs above for errors"
    exit 1
fi

# Create a simple run script
cat > "${OUTPUT_DIR}/run-collector.sh" << 'EOF'
#!/bin/bash
# Run the Database Intelligence Collector

# Set config file path
CONFIG_FILE="${CONFIG_FILE:-production-config-enhanced.yaml}"

# Check if config exists
if [ ! -f "$CONFIG_FILE" ]; then
    echo "Error: Configuration file $CONFIG_FILE not found"
    echo "Usage: CONFIG_FILE=your-config.yaml ./run-collector.sh"
    exit 1
fi

# Check if .env file exists
if [ -f ".env" ]; then
    echo "Loading environment variables from .env file..."
    export $(grep -v '^#' .env | xargs)
fi

# Run the collector
echo "Starting Database Intelligence Collector..."
echo "Configuration: $CONFIG_FILE"
./database-intelligence-collector --config="$CONFIG_FILE"
EOF

chmod +x "${OUTPUT_DIR}/run-collector.sh"

echo -e "\n${GREEN}Build complete!${NC}"
echo "To run the collector:"
echo "  1. cd ${OUTPUT_DIR}"
echo "  2. Copy and configure your .env file"
echo "  3. ./run-collector.sh"