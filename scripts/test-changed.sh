#!/bin/bash

echo "🧪 Testing only changed files..."
echo "================================"
echo ""

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

cd "$(git rev-parse --show-toplevel)" || exit 1

# Get changed Go files (excluding test files)
CHANGED_FILES=$(git diff --name-only origin/main..HEAD | grep "\.go$" | grep -v "_test\.go$")

if [ -z "$CHANGED_FILES" ]; then
    echo -e "${YELLOW}No Go files changed, skipping tests${NC}"
    exit 0
fi

echo -e "${YELLOW}Changed Go files:${NC}"
echo "$CHANGED_FILES"
echo ""

# Get unique modules from changed files
MODULES=$(echo "$CHANGED_FILES" | grep -o "internal/modules/[^/]*" | sort -u | sed 's/internal\/modules\///')

if [ -z "$MODULES" ]; then
    # If no modules found, test the whole package
    echo -e "${YELLOW}Testing all packages...${NC}"
    go test -v ./...
    exit $?
fi

# Test only changed modules
echo -e "${YELLOW}Testing changed modules:${NC}"
echo "$MODULES"
echo ""

HAS_FAILURE=0

for module in $MODULES; do
    echo -e "${BLUE}📦 Testing module: $module${NC}"
    
    # Check if test file exists for this module
    TEST_FILE="tests/unit/${module}_test.go"
    if [ -f "$TEST_FILE" ]; then
        echo -e "${YELLOW}  Running tests from: $TEST_FILE${NC}"
        if ! go test -v "$TEST_FILE" 2>&1; then
            echo -e "${RED}  ❌ Tests failed for module: $module${NC}"
            HAS_FAILURE=1
        else
            echo -e "${GREEN}  ✅ Tests passed for module: $module${NC}"
        fi
    else
        echo -e "${YELLOW}  ⚠️ No test file found for module: $module${NC}"
        echo -e "${YELLOW}  Create: $TEST_FILE${NC}"
    fi
    echo ""
done

if [ $HAS_FAILURE -eq 1 ]; then
    echo -e "${RED}❌ Some tests failed! Push aborted.${NC}"
    exit 1
fi

echo -e "${GREEN}🎉 All changed modules tested successfully!${NC}"