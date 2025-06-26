#!/bin/bash
set -e

# PostgreSQL Unified Collector Installation Script

INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/postgres-collector"
DATA_DIR="/var/lib/postgres-collector"
LOG_DIR="/var/log/postgres-collector"
USER="postgres-collector"
GROUP="postgres-collector"
BINARY_NAME="postgres-unified-collector"

echo "Installing PostgreSQL Unified Collector..."

# Check if running as root
if [[ $EUID -ne 0 ]]; then
   echo "This script must be run as root"
   exit 1
fi

# Create user and group
if ! id "$USER" &>/dev/null; then
    echo "Creating user $USER..."
    useradd --system --no-create-home --shell /bin/false "$USER"
fi

# Create directories
echo "Creating directories..."
mkdir -p "$CONFIG_DIR" "$DATA_DIR" "$LOG_DIR"

# Set ownership
chown -R "$USER:$GROUP" "$DATA_DIR" "$LOG_DIR"
chmod 755 "$CONFIG_DIR"
chmod 700 "$DATA_DIR" "$LOG_DIR"

# Copy binary
if [[ -f "target/release/$BINARY_NAME" ]]; then
    echo "Installing binary..."
    cp "target/release/$BINARY_NAME" "$INSTALL_DIR/"
    chmod 755 "$INSTALL_DIR/$BINARY_NAME"
else
    echo "Binary not found. Please build the project first."
    exit 1
fi

# Copy additional binaries for compatibility
for binary in nri-postgresql postgres-otel-collector; do
    if [[ -f "target/release/$binary" ]]; then
        cp "target/release/$binary" "$INSTALL_DIR/"
        chmod 755 "$INSTALL_DIR/$binary"
    fi
done

# Copy configuration
if [[ -f "configs/collector-config.toml" ]]; then
    echo "Installing configuration..."
    cp "configs/collector-config.toml" "$CONFIG_DIR/config.toml"
    chmod 644 "$CONFIG_DIR/config.toml"
fi

# Copy systemd service
echo "Installing systemd service..."
cp "deployments/systemd/postgres-unified-collector.service" /etc/systemd/system/
systemctl daemon-reload

# Create environment file
cat > /etc/default/postgres-collector <<EOF
# PostgreSQL Unified Collector environment variables
# POSTGRES_HOST=localhost
# POSTGRES_PORT=5432
# POSTGRES_USER=postgres
# POSTGRES_PASSWORD=
# NEW_RELIC_LICENSE_KEY=
# OTLP_ENDPOINT=http://localhost:4317
EOF
chmod 600 /etc/default/postgres-collector

echo "Installation complete!"
echo ""
echo "Next steps:"
echo "1. Edit the configuration file: $CONFIG_DIR/config.toml"
echo "2. Set environment variables in: /etc/default/postgres-collector"
echo "3. Enable the service: systemctl enable postgres-unified-collector"
echo "4. Start the service: systemctl start postgres-unified-collector"
echo "5. Check status: systemctl status postgres-unified-collector"
echo "6. View logs: journalctl -u postgres-unified-collector -f"