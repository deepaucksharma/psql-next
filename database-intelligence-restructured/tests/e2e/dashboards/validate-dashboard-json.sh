#!/bin/bash

# Script to validate the dashboard JSON structure
set -euo pipefail

DASHBOARD_FILE="${1:-database-intelligence-complete-dashboard.json}"

echo "=== Dashboard JSON Validation ==="
echo "File: $DASHBOARD_FILE"
echo ""

# Check if file exists
if [[ ! -f "$DASHBOARD_FILE" ]]; then
    echo "❌ Error: Dashboard file not found"
    exit 1
fi

# Validate JSON syntax
echo "1. Validating JSON syntax..."
if jq empty "$DASHBOARD_FILE" 2>/dev/null; then
    echo "   ✅ JSON syntax is valid"
else
    echo "   ❌ JSON syntax error:"
    jq empty "$DASHBOARD_FILE"
    exit 1
fi

# Check required fields
echo ""
echo "2. Checking required fields..."
errors=0

# Check dashboard name
name=$(jq -r '.name // empty' "$DASHBOARD_FILE")
if [[ -z "$name" ]]; then
    echo "   ❌ Missing dashboard name"
    ((errors++))
else
    echo "   ✅ Dashboard name: $name"
fi

# Check pages
page_count=$(jq '.pages | length' "$DASHBOARD_FILE")
if [[ "$page_count" -eq 0 ]]; then
    echo "   ❌ No pages defined"
    ((errors++))
else
    echo "   ✅ Pages: $page_count"
fi

# Validate each page
echo ""
echo "3. Validating pages..."
for ((i=0; i<$page_count; i++)); do
    page_name=$(jq -r ".pages[$i].name // empty" "$DASHBOARD_FILE")
    widget_count=$(jq ".pages[$i].widgets | length" "$DASHBOARD_FILE")
    
    if [[ -z "$page_name" ]]; then
        echo "   ❌ Page $((i+1)) has no name"
        ((errors++))
    else
        echo "   ✅ Page $((i+1)): $page_name ($widget_count widgets)"
    fi
done

# Check widget structure
echo ""
echo "4. Validating widgets..."
total_widgets=0
invalid_widgets=0

while IFS= read -r widget; do
    ((total_widgets++))
    
    # Check required widget fields
    title=$(echo "$widget" | jq -r '.title // empty')
    viz=$(echo "$widget" | jq -r '.visualization // empty')
    query=$(echo "$widget" | jq -r '.query // empty')
    
    if [[ -z "$title" ]] || [[ -z "$viz" ]] || [[ -z "$query" ]]; then
        ((invalid_widgets++))
        echo "   ❌ Invalid widget: ${title:-'No title'}"
        [[ -z "$viz" ]] && echo "      - Missing visualization"
        [[ -z "$query" ]] && echo "      - Missing query"
    fi
done < <(jq -c '.pages[].widgets[]' "$DASHBOARD_FILE")

echo "   Total widgets: $total_widgets"
echo "   Valid widgets: $((total_widgets - invalid_widgets))"
echo "   Invalid widgets: $invalid_widgets"

# Check for common NRQL patterns
echo ""
echo "5. Checking NRQL queries..."
instrumentation_queries=$(jq '[.pages[].widgets[].query | select(. != null)] | map(select(contains("instrumentation.provider"))) | length' "$DASHBOARD_FILE")
metric_queries=$(jq '[.pages[].widgets[].query | select(. != null)] | map(select(contains("FROM Metric"))) | length' "$DASHBOARD_FILE")
log_queries=$(jq '[.pages[].widgets[].query | select(. != null)] | map(select(contains("FROM Log"))) | length' "$DASHBOARD_FILE")

echo "   Queries with instrumentation.provider filter: $instrumentation_queries"
echo "   Metric queries: $metric_queries"
echo "   Log queries: $log_queries"

# Summary
echo ""
echo "=== Validation Summary ==="
if [[ $errors -eq 0 ]] && [[ $invalid_widgets -eq 0 ]]; then
    echo "✅ Dashboard JSON is valid and ready for deployment!"
    
    # Show metrics being queried
    echo ""
    echo "Metrics referenced in dashboard:"
    jq -r '[.pages[].widgets[].query | select(. != null)] | map(scan("\\b(db\\.ash\\.[a-z_]+|kernel\\.[a-z.]+|otelcol\\.[a-z.]+|postgres\\.[a-z._]+)\\b")) | unique | sort[]' "$DASHBOARD_FILE" 2>/dev/null | head -20
    
    exit 0
else
    echo "❌ Dashboard has $errors errors and $invalid_widgets invalid widgets"
    echo "   Please fix these issues before deployment."
    exit 1
fi