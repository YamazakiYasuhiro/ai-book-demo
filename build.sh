#!/bin/bash
#
# CI/CD Build Script
# Executes the complete build and test pipeline
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
NC='\033[0m'

# Default flags
SKIP_TESTS=false
SKIP_BUILD=false
SKIP_START=false
SKIP_TEST=false

# ============================================================
# Functions
# ============================================================

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

show_help() {
    cat << EOF
Usage: ./build.sh [OPTIONS]

CI/CD Build Script - Executes the complete build and test pipeline

Options:
    --skip-tests    Skip unit tests
    --skip-build    Skip container build
    --skip-start    Skip container startup
    --skip-test     Skip integration tests
    --help          Show this help message

Process:
    1. Stop running containers
    2. Run unit tests (backend and frontend)
    3. Build containers
    4. Start containers
    5. Run integration tests

Examples:
    ./build.sh                      # Run full pipeline
    ./build.sh --skip-tests         # Skip unit tests
    ./build.sh --skip-build         # Skip container build
    ./build.sh --skip-tests --skip-build  # Skip both
EOF
}

step_stop_containers() {
    log_info "Step 1: Stopping containers..."
    cd "$PROJECT_ROOT"
    docker compose down --remove-orphans 2>/dev/null || true
    log_success "Containers stopped"
}

step_unit_tests() {
    if [ "$SKIP_TESTS" = true ]; then
        log_warning "Skipping unit tests (--skip-tests)"
        return 0
    fi

    log_info "Step 2: Running unit tests..."

    # Backend tests
    log_info "Running backend tests..."
    cd "$PROJECT_ROOT/backend"
    go test ./... -v

    # Frontend tests
    log_info "Running frontend tests..."
    cd "$PROJECT_ROOT/frontend"
    # Ensure dependencies are installed
    if [ ! -d "node_modules" ]; then
        log_info "Installing frontend dependencies..."
        yarn install
    fi
    # Use node_modules/.bin/jest directly for Windows compatibility
    # On Windows Git Bash, we need to use the full path
    if [ -f "node_modules/.bin/jest" ]; then
        ./node_modules/.bin/jest --watchAll=false
    elif [ -f "node_modules/.bin/jest.cmd" ]; then
        # Windows Git Bash may need .cmd extension
        ./node_modules/.bin/jest.cmd --watchAll=false
    else
        log_error "jest not found in node_modules/.bin"
        log_info "Attempting to install dependencies..."
        yarn install
        if [ -f "node_modules/.bin/jest" ]; then
            ./node_modules/.bin/jest --watchAll=false
        elif [ -f "node_modules/.bin/jest.cmd" ]; then
            ./node_modules/.bin/jest.cmd --watchAll=false
        else
            log_error "Failed to find jest after installation"
            exit 1
        fi
    fi

    log_success "Unit tests passed"
}

step_build_containers() {
    if [ "$SKIP_BUILD" = true ]; then
        log_warning "Skipping container build (--skip-build)"
        return 0
    fi

    log_info "Step 3: Building containers..."
    cd "$PROJECT_ROOT"
    docker compose build
    log_success "Containers built"
}

step_start_containers() {
    if [ "$SKIP_START" = true ]; then
        log_warning "Skipping container startup (--skip-start)"
        return 0
    fi

    log_info "Step 4: Starting containers..."
    cd "$PROJECT_ROOT"
    docker compose up -d

    # Wait for health check
    log_info "Waiting for server to be healthy..."
    local max_attempts=30
    local attempt=0
    while [ $attempt -lt $max_attempts ]; do
        if curl -s http://localhost:8080/health > /dev/null 2>&1; then
            log_success "Server is healthy"
            return 0
        fi
        attempt=$((attempt + 1))
        sleep 1
    done

    log_error "Server failed to become healthy within ${max_attempts} seconds"
    exit 1
}

step_integration_tests() {
    if [ "$SKIP_TEST" = true ]; then
        log_warning "Skipping integration tests (--skip-test)"
        return 0
    fi

    log_info "Step 5: Running integration tests..."
    cd "$PROJECT_ROOT/tests"
    go test ./integration/... -v
    log_success "Integration tests passed"
}

# ============================================================
# Main
# ============================================================

main() {
    # Parse arguments
    for arg in "$@"; do
        case $arg in
            --skip-tests) SKIP_TESTS=true ;;
            --skip-build) SKIP_BUILD=true ;;
            --skip-start) SKIP_START=true ;;
            --skip-test)  SKIP_TEST=true ;;
            --help)       show_help; exit 0 ;;
            *)
                log_error "Unknown option: $arg"
                show_help
                exit 1
                ;;
        esac
    done

    echo ""
    echo "========================================"
    echo "       CI/CD Pipeline Starting"
    echo "========================================"
    echo ""

    local start_time=$(date +%s)

    # Execute pipeline steps
    step_stop_containers
    step_unit_tests
    step_build_containers
    step_start_containers
    step_integration_tests

    local end_time=$(date +%s)
    local duration=$((end_time - start_time))

    echo ""
    echo "========================================"
    log_success "CI/CD Pipeline Completed Successfully!"
    echo "        Duration: ${duration} seconds"
    echo "========================================"
    echo ""
}

main "$@"

