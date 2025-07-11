#!/bin/bash

# Fix account IDs in the user dashboard JSON
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INPUT_FILE="$SCRIPT_DIR/database-intelligence-user-session-nerdgraph.json"
ACCOUNT_ID="${1:-3630072}"

echo "Fixing account IDs in user dashboard JSON..."
echo "Account ID: $ACCOUNT_ID"

# Use jq to update all accountId fields
jq --arg accountId "$ACCOUNT_ID" '
  .pages |= map({
    name: .name,
    description: .description,
    widgets: .widgets | map({
      title: .title,
      layout: .layout,
      rawConfiguration: (.rawConfiguration | 
        .nrqlQueries[0].accountId = ($accountId | tonumber)
      ),
      visualization: .visualization
    })
  })
' "$INPUT_FILE" > "${INPUT_FILE}.tmp" && mv "${INPUT_FILE}.tmp" "$INPUT_FILE"

echo "✅ Updated all account IDs to: $ACCOUNT_ID"

# Verify the update
count=$(jq '[.pages[].widgets[].rawConfiguration.nrqlQueries[0].accountId] | map(select(. == 0)) | length' "$INPUT_FILE")
if [[ "$count" -eq 0 ]]; then
    echo "✅ Verification passed: No accountId fields with value 0"
else
    echo "❌ Warning: Found $count widgets with accountId still set to 0"
fi