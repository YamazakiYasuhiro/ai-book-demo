#!/bin/bash
#
# Code Line Counter Script
# Counts lines of code for source files and test files
# Displays results in a table format with detailed breakdowns
#

set -e

# ============================================================
# Configuration
# ============================================================

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$SCRIPT_DIR"

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# ============================================================
# Functions
# ============================================================

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

count_lines_and_files() {
    local pattern="$1"
    local exclude_paths="$2"
    local count_lines=0
    local count_files=0
    
    # Build find command
    local find_cmd="find \"$PROJECT_ROOT\" -type f -name \"$pattern\""
    
    # Add exclusions
    if [ -n "$exclude_paths" ]; then
        for exclude in $exclude_paths; do
            find_cmd="$find_cmd -not -path \"$exclude\""
        done
    fi
    
    # Use find with -exec to count lines and files
    # This avoids issues with pipes and subshells
    eval "$find_cmd" 2>/dev/null | while IFS= read -r file; do
        if [ -f "$file" ]; then
            lines=$(wc -l < "$file" 2>/dev/null | tr -d ' ' || echo "0")
            count_lines=$((count_lines + lines))
            count_files=$((count_files + 1))
            echo "$count_lines|$count_files"
        fi
    done | tail -1 | {
        IFS='|' read -r lines files
        echo "$lines|$files"
    }
}

# Simpler approach: use find with -exec wc -l and sum the results
count_lines_simple() {
    local pattern="$1"
    local exclude_paths="$2"
    local total=0
    local temp_result
    
    # Build find command - use -prune for common exclusions to speed up
    local find_cmd="find \"$PROJECT_ROOT\""
    
    # Add -prune for directories that should be skipped entirely
    find_cmd="$find_cmd \\( -name node_modules -o -name dist -o -name vendor -o -name .git \\) -prune -o"
    
    # Add file pattern and type
    find_cmd="$find_cmd -type f -name \"$pattern\""
    
    # Add additional exclusions using -not -path
    if [ -n "$exclude_paths" ]; then
        for exclude in $exclude_paths; do
            # Skip if already handled by -prune
            case "$exclude" in
                */node_modules/*|*/dist/*|*/vendor/*|*/.git/*) continue ;;
            esac
            find_cmd="$find_cmd -not -path \"$exclude\""
        done
    fi
    
    # Execute find and sum line counts
    temp_result=$(eval "$find_cmd -exec wc -l {} + 2>/dev/null" | tail -1)
    if [ -n "$temp_result" ] && [ "$temp_result" != "total" ]; then
        total=$(echo "$temp_result" | awk '{print $1}' || echo "0")
    fi
    
    echo "${total:-0}"
}

count_files_simple() {
    local pattern="$1"
    local exclude_paths="$2"
    local count=0
    
    # Build find command - use -prune for common exclusions
    local find_cmd="find \"$PROJECT_ROOT\""
    
    # Add -prune for directories that should be skipped entirely
    find_cmd="$find_cmd \\( -name node_modules -o -name dist -o -name vendor -o -name .git \\) -prune -o"
    
    # Add file pattern and type
    find_cmd="$find_cmd -type f -name \"$pattern\""
    
    # Add additional exclusions
    if [ -n "$exclude_paths" ]; then
        for exclude in $exclude_paths; do
            # Skip if already handled by -prune
            case "$exclude" in
                */node_modules/*|*/dist/*|*/vendor/*|*/.git/*) continue ;;
            esac
            find_cmd="$find_cmd -not -path \"$exclude\""
        done
    fi
    
    # Count files
    count=$(eval "$find_cmd -print 2>/dev/null" | wc -l | tr -d ' ')
    echo "${count:-0}"
}

# ============================================================
# Count Lines by Category
# ============================================================

log_info "Counting lines of code..."

# Common exclusion patterns
EXCLUDE_BASE="*/node_modules/* */dist/* */go.sum */yarn.lock */package-lock.json */settings/secrets/* */prompts/* */assets/*"

# Go - Source files (backend only, exclude tests)
GO_SOURCE_LINES=$(count_lines_simple "*.go" "$EXCLUDE_BASE */tests/* */*_test.go */backend/*/vendor/*")
GO_SOURCE_FILES=$(count_files_simple "*.go" "$EXCLUDE_BASE */tests/* */*_test.go */backend/*/vendor/*")

# Go - Test files (backend only)
GO_TEST_LINES=$(count_lines_simple "*_test.go" "$EXCLUDE_BASE */tests/* */backend/*/vendor/*")
GO_TEST_FILES=$(count_files_simple "*_test.go" "$EXCLUDE_BASE */tests/* */backend/*/vendor/*")

# Go - Integration tests (tests/ directory only)
GO_INTEGRATION_LINES=$(count_lines_simple "*.go" "$EXCLUDE_BASE */backend/* */frontend/*")
GO_INTEGRATION_FILES=$(count_files_simple "*.go" "$EXCLUDE_BASE */backend/* */frontend/*")

# TypeScript - Source files (exclude test files)
TS_SOURCE_LINES=$(count_lines_simple "*.ts" "$EXCLUDE_BASE */__tests__/* */*.test.ts */*.test.tsx")
TS_SOURCE_FILES=$(count_files_simple "*.ts" "$EXCLUDE_BASE */__tests__/* */*.test.ts */*.test.tsx")
TSX_SOURCE_LINES=$(count_lines_simple "*.tsx" "$EXCLUDE_BASE */__tests__/* */*.test.ts */*.test.tsx")
TSX_SOURCE_FILES=$(count_files_simple "*.tsx" "$EXCLUDE_BASE */__tests__/* */*.test.ts */*.test.tsx")

# TypeScript - Test files (.test.ts/.test.tsx)
TS_TEST_LINES=$(count_lines_simple "*.test.ts" "$EXCLUDE_BASE")
TS_TEST_FILES=$(count_files_simple "*.test.ts" "$EXCLUDE_BASE")
TSX_TEST_LINES=$(count_lines_simple "*.test.tsx" "$EXCLUDE_BASE")
TSX_TEST_FILES=$(count_files_simple "*.test.tsx" "$EXCLUDE_BASE")

# TypeScript - Test files in __tests__ directory
TS_TEST_DIR_LINES=0
TS_TEST_DIR_FILES=0
TSX_TEST_DIR_LINES=0
TSX_TEST_DIR_FILES=0
# Use find with path filter for __tests__ directory
TS_TEST_DIR_LINES=$(find "$PROJECT_ROOT" -type f -path "*/__tests__/*.ts" -not -path "*/node_modules/*" -not -path "*/dist/*" -not -name "*.test.ts" -exec wc -l {} + 2>/dev/null | tail -1 | awk '{print $1}' || echo "0")
TS_TEST_DIR_FILES=$(find "$PROJECT_ROOT" -type f -path "*/__tests__/*.ts" -not -path "*/node_modules/*" -not -path "*/dist/*" -not -name "*.test.ts" 2>/dev/null | wc -l | tr -d ' ' || echo "0")
TSX_TEST_DIR_LINES=$(find "$PROJECT_ROOT" -type f -path "*/__tests__/*.tsx" -not -path "*/node_modules/*" -not -path "*/dist/*" -not -name "*.test.tsx" -exec wc -l {} + 2>/dev/null | tail -1 | awk '{print $1}' || echo "0")
TSX_TEST_DIR_FILES=$(find "$PROJECT_ROOT" -type f -path "*/__tests__/*.tsx" -not -path "*/node_modules/*" -not -path "*/dist/*" -not -name "*.test.tsx" 2>/dev/null | wc -l | tr -d ' ' || echo "0")

# JavaScript - Source files (exclude config and test files)
JS_SOURCE_LINES=$(count_lines_simple "*.js" "$EXCLUDE_BASE */*.config.js */*.test.js */*.test.jsx */__tests__/*")
JS_SOURCE_FILES=$(count_files_simple "*.js" "$EXCLUDE_BASE */*.config.js */*.test.js */*.test.jsx */__tests__/*")
JSX_SOURCE_LINES=$(count_lines_simple "*.jsx" "$EXCLUDE_BASE */*.config.jsx */*.test.js */*.test.jsx */__tests__/*")
JSX_SOURCE_FILES=$(count_files_simple "*.jsx" "$EXCLUDE_BASE */*.config.jsx */*.test.js */*.test.jsx */__tests__/*")

# JavaScript - Test files
JS_TEST_LINES=$(count_lines_simple "*.test.js" "$EXCLUDE_BASE")
JS_TEST_FILES=$(count_files_simple "*.test.js" "$EXCLUDE_BASE")
JSX_TEST_LINES=$(count_lines_simple "*.test.jsx" "$EXCLUDE_BASE")
JSX_TEST_FILES=$(count_files_simple "*.test.jsx" "$EXCLUDE_BASE")

# CSS files
CSS_LINES=$(count_lines_simple "*.css" "$EXCLUDE_BASE")
CSS_FILES=$(count_files_simple "*.css" "$EXCLUDE_BASE")

# Calculate totals
TOTAL_GO=$((GO_SOURCE_LINES + GO_TEST_LINES + GO_INTEGRATION_LINES))
TOTAL_TS=$((TS_SOURCE_LINES + TSX_SOURCE_LINES + TS_TEST_LINES + TSX_TEST_LINES + TS_TEST_DIR_LINES + TSX_TEST_DIR_LINES))
TOTAL_JS=$((JS_SOURCE_LINES + JSX_SOURCE_LINES + JS_TEST_LINES + JSX_TEST_LINES))
TOTAL_CSS=$CSS_LINES
TOTAL_LINES=$((TOTAL_GO + TOTAL_TS + TOTAL_JS + TOTAL_CSS))

# ============================================================
# Display Results
# ============================================================

echo ""
echo "========================================"
echo "      Code Line Count Report"
echo "========================================"
echo ""

# Language breakdown
echo -e "${CYAN}By Language:${NC}"
printf "%-15s %10s %10s\n" "Language" "Lines" "Files"
echo "----------------------------------------"
printf "%-15s %10d %10d\n" "Go" "$TOTAL_GO" "$((GO_SOURCE_FILES + GO_TEST_FILES + GO_INTEGRATION_FILES))"
printf "%-15s %10d %10d\n" "TypeScript" "$TOTAL_TS" "$((TS_SOURCE_FILES + TSX_SOURCE_FILES + TS_TEST_FILES + TSX_TEST_FILES + TS_TEST_DIR_FILES + TSX_TEST_DIR_FILES))"
printf "%-15s %10d %10d\n" "JavaScript" "$TOTAL_JS" "$((JS_SOURCE_FILES + JSX_SOURCE_FILES + JS_TEST_FILES + JSX_TEST_FILES))"
printf "%-15s %10d %10d\n" "CSS" "$TOTAL_CSS" "$CSS_FILES"
echo "----------------------------------------"
printf "%-15s %10d\n" "TOTAL" "$TOTAL_LINES"
echo ""

# Directory breakdown
echo -e "${CYAN}By Directory:${NC}"
printf "%-20s %10s %10s\n" "Directory" "Lines" "Files"
echo "----------------------------------------"

# Backend (Go source + tests, excluding integration tests)
BACKEND_LINES=$((GO_SOURCE_LINES + GO_TEST_LINES))
BACKEND_FILES=$((GO_SOURCE_FILES + GO_TEST_FILES))
printf "%-20s %10d %10d\n" "backend/" "$BACKEND_LINES" "$BACKEND_FILES"

# Frontend (TypeScript + JavaScript + CSS)
FRONTEND_LINES=$((TOTAL_TS + TOTAL_JS + TOTAL_CSS))
FRONTEND_FILES=$((TS_SOURCE_FILES + TSX_SOURCE_FILES + TS_TEST_FILES + TSX_TEST_FILES + TS_TEST_DIR_FILES + TSX_TEST_DIR_FILES + JS_SOURCE_FILES + JSX_SOURCE_FILES + JS_TEST_FILES + JSX_TEST_FILES + CSS_FILES))
printf "%-20s %10d %10d\n" "frontend/" "$FRONTEND_LINES" "$FRONTEND_FILES"

# Tests (Integration tests)
printf "%-20s %10d %10d\n" "tests/" "$GO_INTEGRATION_LINES" "$GO_INTEGRATION_FILES"
echo "----------------------------------------"
printf "%-20s %10d\n" "TOTAL" "$TOTAL_LINES"
echo ""

# Source vs Test breakdown
echo -e "${CYAN}By File Type:${NC}"
printf "%-15s %10s %10s\n" "Type" "Lines" "Files"
echo "----------------------------------------"

SOURCE_LINES=$((GO_SOURCE_LINES + TS_SOURCE_LINES + TSX_SOURCE_LINES + JS_SOURCE_LINES + JSX_SOURCE_LINES + CSS_LINES))
SOURCE_FILES=$((GO_SOURCE_FILES + TS_SOURCE_FILES + TSX_SOURCE_FILES + JS_SOURCE_FILES + JSX_SOURCE_FILES + CSS_FILES))
printf "%-15s %10d %10d\n" "Source" "$SOURCE_LINES" "$SOURCE_FILES"

TEST_LINES=$((GO_TEST_LINES + GO_INTEGRATION_LINES + TS_TEST_LINES + TSX_TEST_LINES + TS_TEST_DIR_LINES + TSX_TEST_DIR_LINES + JS_TEST_LINES + JSX_TEST_LINES))
TEST_FILES=$((GO_TEST_FILES + GO_INTEGRATION_FILES + TS_TEST_FILES + TSX_TEST_FILES + TS_TEST_DIR_FILES + TSX_TEST_DIR_FILES + JS_TEST_FILES + JSX_TEST_FILES))
printf "%-15s %10d %10d\n" "Test" "$TEST_LINES" "$TEST_FILES"
echo "----------------------------------------"
printf "%-15s %10d\n" "TOTAL" "$TOTAL_LINES"
echo ""

# Detailed breakdown
echo -e "${CYAN}Detailed Breakdown:${NC}"
echo ""
echo -e "${YELLOW}Go:${NC}"
printf "  %-30s %10d lines (%d files)\n" "Source files" "$GO_SOURCE_LINES" "$GO_SOURCE_FILES"
printf "  %-30s %10d lines (%d files)\n" "Test files" "$GO_TEST_LINES" "$GO_TEST_FILES"
printf "  %-30s %10d lines (%d files)\n" "Integration tests" "$GO_INTEGRATION_LINES" "$GO_INTEGRATION_FILES"
echo ""

echo -e "${YELLOW}TypeScript:${NC}"
printf "  %-30s %10d lines (%d files)\n" "Source files (.ts)" "$TS_SOURCE_LINES" "$TS_SOURCE_FILES"
printf "  %-30s %10d lines (%d files)\n" "Source files (.tsx)" "$TSX_SOURCE_LINES" "$TSX_SOURCE_FILES"
printf "  %-30s %10d lines (%d files)\n" "Test files (.test.ts)" "$TS_TEST_LINES" "$TS_TEST_FILES"
printf "  %-30s %10d lines (%d files)\n" "Test files (.test.tsx)" "$TSX_TEST_LINES" "$TSX_TEST_FILES"
printf "  %-30s %10d lines (%d files)\n" "Test files (__tests__/)" "$((TS_TEST_DIR_LINES + TSX_TEST_DIR_LINES))" "$((TS_TEST_DIR_FILES + TSX_TEST_DIR_FILES))"
echo ""

echo -e "${YELLOW}JavaScript:${NC}"
printf "  %-30s %10d lines (%d files)\n" "Source files (.js)" "$JS_SOURCE_LINES" "$JS_SOURCE_FILES"
printf "  %-30s %10d lines (%d files)\n" "Source files (.jsx)" "$JSX_SOURCE_LINES" "$JSX_SOURCE_FILES"
printf "  %-30s %10d lines (%d files)\n" "Test files (.test.js)" "$JS_TEST_LINES" "$JS_TEST_FILES"
printf "  %-30s %10d lines (%d files)\n" "Test files (.test.jsx)" "$JSX_TEST_LINES" "$JSX_TEST_FILES"
echo ""

echo -e "${YELLOW}CSS:${NC}"
printf "  %-30s %10d lines (%d files)\n" "CSS files" "$CSS_LINES" "$CSS_FILES"
echo ""

echo "========================================"
log_success "Counting completed!"
echo "========================================"
echo ""
