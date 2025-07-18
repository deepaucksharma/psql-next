#!/bin/bash

# Script to verify current module paths before making changes

echo "Current module path usage analysis:"
echo "=================================="
echo ""

OLD_MODULE="github.com/deepaksharma/db-otel"

echo "1. go.mod files containing $OLD_MODULE:"
echo "---------------------------------------"
find . -name "go.mod" -type f -exec grep -l "$OLD_MODULE" {} \; 2>/dev/null | sort

echo ""
echo "2. go.work file:"
echo "----------------"
if [ -f "go.work" ]; then
    grep -c "$OLD_MODULE" go.work 2>/dev/null && echo "go.work contains $(grep -c "$OLD_MODULE" go.work) occurrences"
fi

echo ""
echo "3. Go source files containing $OLD_MODULE:"
echo "-----------------------------------------"
find . -name "*.go" -type f -exec grep -l "$OLD_MODULE" {} \; 2>/dev/null | sort

echo ""
echo "4. Total occurrences by file type:"
echo "----------------------------------"
echo "go.mod files: $(find . -name "go.mod" -type f -exec grep -c "$OLD_MODULE" {} \; 2>/dev/null | paste -sd+ | bc)"
echo "go.work: $(grep -c "$OLD_MODULE" go.work 2>/dev/null || echo 0)"
echo ".go files: $(find . -name "*.go" -type f -exec grep -c "$OLD_MODULE" {} \; 2>/dev/null | paste -sd+ | bc)"

echo ""
echo "5. Sample imports that will be changed:"
echo "--------------------------------------"
find . -name "*.go" -type f -exec grep -h "import.*$OLD_MODULE" {} \; 2>/dev/null | head -10