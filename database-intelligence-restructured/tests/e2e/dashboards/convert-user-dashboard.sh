#!/bin/bash

# Convert dashboard format for NerdGraph API
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INPUT_FILE="$SCRIPT_DIR/database-intelligence-user-session-dashboard.json"
OUTPUT_FILE="$SCRIPT_DIR/database-intelligence-user-session-nerdgraph.json"

echo "Converting dashboard format for NerdGraph API..."

# Convert using jq to transform widget structure
jq '
  # Transform each widget to NerdGraph format
  .pages |= map({
    name: .name,
    description: .description,
    widgets: .widgets | map({
      title: .title,
      layout: .layout,
      rawConfiguration: (
        {
          nrqlQueries: [
            {
              accountId: 0,  # Will be replaced by API
              query: .query
            }
          ],
          platformOptions: {
            ignoreTimeRange: false
          }
        } + 
        if .configuration and .configuration.thresholds then
          {
            thresholds: {
              isLabelVisible: true,
              thresholds: .configuration.thresholds | map({
                value: .value,
                severity: .alertSeverity
              })
            }
          }
        else
          {}
        end
      ),
      visualization: {
        id: .visualization
      }
    })
  })
' "$INPUT_FILE" > "$OUTPUT_FILE"

echo "✅ Converted dashboard saved to: $OUTPUT_FILE"

# Validate the output
if jq empty "$OUTPUT_FILE" 2>/dev/null; then
    echo "✅ Output JSON is valid"
    
    # Show summary
    page_count=$(jq '.pages | length' "$OUTPUT_FILE")
    widget_count=$(jq '[.pages[].widgets | length] | add' "$OUTPUT_FILE")
    
    echo ""
    echo "Dashboard summary:"
    echo "- Pages: $page_count"
    echo "- Total widgets: $widget_count"
else
    echo "❌ Output JSON is invalid"
    exit 1
fi