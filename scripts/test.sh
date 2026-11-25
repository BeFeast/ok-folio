#!/bin/bash
# Test runner script for PhotoPrism Extractor

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
COVERAGE_DIR="coverage"
COVERAGE_FILE="$COVERAGE_DIR/coverage.out"
MIN_COVERAGE=80

# Create coverage directory
mkdir -p "$COVERAGE_DIR"

# Function to print section headers
print_header() {
    echo -e "\n${BLUE}=== $1 ===${NC}"
}

# Function to run tests
run_tests() {
    print_header "Running Tests"
    if go test ./... -coverprofile="$COVERAGE_FILE" -v 2>&1 | tee "$COVERAGE_DIR/test.log"; then
        echo -e "${GREEN}✓ Tests passed${NC}"
        return 0
    else
        echo -e "${YELLOW}⚠ Some tests failed${NC}"
        return 1
    fi
}

# Function to generate coverage report
coverage_report() {
    print_header "Coverage Report"

    # Package-level coverage
    echo -e "\n${BLUE}Package Coverage:${NC}"
    go tool cover -func="$COVERAGE_FILE" 2>/dev/null | \
        grep -E "^ok-folio/(pkg|internal|cmd)" | \
        awk -F'[/:]' '{
            pkg=$2"/"$3;
            gsub(/\t+/, " ");
            split($0, parts, " ");
            coverage=parts[length(parts)];

            # Color based on coverage
            if (coverage ~ /^[0-9]/) {
                cov_num = substr(coverage, 1, length(coverage)-1);
                if (cov_num >= 80) color="\033[0;32m";      # Green
                else if (cov_num >= 60) color="\033[1;33m"; # Yellow
                else if (cov_num > 0) color="\033[0;33m";   # Orange
                else color="\033[0;31m";                    # Red
                printf "%s%-30s %6s\033[0m\n", color, pkg, coverage;
            }
        }' | sort -u

    # Overall coverage
    echo -e "\n${BLUE}Overall Coverage:${NC}"
    TOTAL=$(go tool cover -func="$COVERAGE_FILE" 2>/dev/null | tail -1 | awk '{print $3}')
    TOTAL_NUM=$(echo "$TOTAL" | sed 's/%//')

    if (( $(echo "$TOTAL_NUM >= $MIN_COVERAGE" | bc -l) )); then
        echo -e "${GREEN}✓ $TOTAL (Goal: ${MIN_COVERAGE}%)${NC}"
    elif (( $(echo "$TOTAL_NUM >= 60" | bc -l) )); then
        echo -e "${YELLOW}⚠ $TOTAL (Goal: ${MIN_COVERAGE}%)${NC}"
    else
        echo -e "${RED}✗ $TOTAL (Goal: ${MIN_COVERAGE}%)${NC}"
    fi
}

# Function to show test summary
test_summary() {
    print_header "Test Summary"

    # Count tests
    TOTAL_TESTS=$(grep -c "^=== RUN" "$COVERAGE_DIR/test.log" 2>/dev/null || echo "0")
    PASSED_TESTS=$(grep -c "^--- PASS:" "$COVERAGE_DIR/test.log" 2>/dev/null || echo "0")
    FAILED_TESTS=$(grep -c "^--- FAIL:" "$COVERAGE_DIR/test.log" 2>/dev/null || echo "0")

    echo "Total Tests:  $TOTAL_TESTS"
    echo -e "Passed:       ${GREEN}$PASSED_TESTS${NC}"
    if [ "$FAILED_TESTS" -gt 0 ]; then
        echo -e "Failed:       ${RED}$FAILED_TESTS${NC}"
    else
        echo -e "Failed:       ${GREEN}$FAILED_TESTS${NC}"
    fi
}

# Main execution
echo -e "${BLUE}"
echo "╔═══════════════════════════════════════════╗"
echo "║   PhotoPrism Extractor - Test Suite      ║"
echo "╔═══════════════════════════════════════════╝"
echo -e "${NC}"

# Run tests
if ! run_tests; then
    TEST_FAILED=true
fi

# Generate reports
coverage_report
test_summary

# Generate HTML report if requested
if [ "$1" == "--html" ]; then
    print_header "Generating HTML Report"
    go tool cover -html="$COVERAGE_FILE" -o "$COVERAGE_DIR/coverage.html"
    echo -e "${GREEN}✓ HTML report: $COVERAGE_DIR/coverage.html${NC}"
fi

# Exit with appropriate code
if [ "$TEST_FAILED" == "true" ]; then
    exit 1
fi

exit 0
