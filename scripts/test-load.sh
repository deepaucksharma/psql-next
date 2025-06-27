#!/bin/bash
# Symlink to run-load-test.sh for compatibility

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
exec "$SCRIPT_DIR/run-load-test.sh" "$@"