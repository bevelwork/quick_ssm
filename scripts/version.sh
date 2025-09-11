#!/bin/bash

# Version management script for quick_ssm

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_color() {
    local color=$1
    local message=$2
    echo -e "${color}${message}${NC}"
}

# Function to read version from version/version.go
read_version() {
    if [ ! -f "version/version.go" ]; then
        print_color $RED "version/version.go not found!"
        exit 1
    fi

    local major=$(grep '^\s*Major\s*=\s*' version/version.go | sed -E 's/.*Major\s*=\s*([0-9]+).*/\1/')
    local minor=$(grep '^\s*Minor\s*=\s*' version/version.go | sed -E 's/.*Minor\s*=\s*([0-9]+).*/\1/')
    local patch=$(grep '^\s*PatchDate\s*=\s*' version/version.go | sed -E 's/.*PatchDate\s*=\s*"([0-9]+)".*/\1/')
    echo "${major}.${minor}.${patch}"
}

# Function to generate full version with date
generate_version() {
    local base=$(read_version)
    echo "v${base}"
}

# Function to show current version
show_version() {
    local base_version=$(read_version)
    local full_version=$(generate_version)
    print_color $BLUE "Base version: $base_version"
    print_color $GREEN "Full version: $full_version"
}

# Print only the full version string (for tooling)
print_full_version() {
    generate_version
}

# Function to update major version
update_major() {
    local new_major=$1
    if [ -z "$new_major" ]; then
        print_color $RED "Major version number required"
        print_color $YELLOW "Usage: $0 major <number>"
        exit 1
    fi
    
    # Update version/version.go (Major and PatchDate, and Full)
    local today=$(date +%Y%m%d)
    sed -i "s/^\(\s*Major\s*=\s*\).*/\1${new_major}/" version/version.go
    sed -i "s/^\(\s*PatchDate\s*=\s*\)\"[0-9]*\"/\1\"${today}\"/" version/version.go
    # Recompute Full
    local minor=$(grep '^\s*Minor\s*=\s*' version/version.go | sed -E 's/.*Minor\s*=\s*([0-9]+).*/\1/')
    sed -i "s/^\(\s*var Full = \)\"v[0-9]+\.[0-9]+\.[0-9]+\"/\1\"v${new_major}.${minor}.${today}\"/" version/version.go
    print_color $GREEN "Updated major version to: $new_major"
    show_version
}

# Function to update minor version
update_minor() {
    local new_minor=$1
    if [ -z "$new_minor" ]; then
        print_color $RED "Minor version number required"
        print_color $YELLOW "Usage: $0 minor <number>"
        exit 1
    fi
    
    # Update version/version.go (Minor and PatchDate, and Full)
    local today=$(date +%Y%m%d)
    sed -i "s/^\(\s*Minor\s*=\s*\).*/\1${new_minor}/" version/version.go
    sed -i "s/^\(\s*PatchDate\s*=\s*\)\"[0-9]*\"/\1\"${today}\"/" version/version.go
    # Recompute Full
    local major=$(grep '^\s*Major\s*=\s*' version/version.go | sed -E 's/.*Major\s*=\s*([0-9]+).*/\1/')
    sed -i "s/^\(\s*var Full = \)\"v[0-9]+\.[0-9]+\.[0-9]+\"/\1\"v${major}.${new_minor}.${today}\"/" version/version.go
    print_color $GREEN "Updated minor version to: $new_minor"
    show_version
}

# Function to build release binary
build_release() {
    # Ensure PatchDate and Full are up to date before build
    local today=$(date +%Y%m%d)
    local major=$(grep '^\s*Major\s*=\s*' version/version.go | sed -E 's/.*Major\s*=\s*([0-9]+).*/\1/')
    local minor=$(grep '^\s*Minor\s*=\s*' version/version.go | sed -E 's/.*Minor\s*=\s*([0-9]+).*/\1/')
    sed -i "s/^\(\s*PatchDate\s*=\s*\)\"[0-9]*\"/\1\"${today}\"/" version/version.go
    sed -i "s/^\(\s*var Full = \)\"v[0-9]+\.[0-9]+\.[0-9]+\"/\1\"v${major}.${minor}.${today}\"/" version/version.go
    local version=$(generate_version)
    local binary_name="quick_ssm-${version}-linux-amd64"
    
    print_color $BLUE "Building release binary: $binary_name"
    
    # Build the binary
    go build -v -o "$binary_name" .
    
    # Generate checksum
    sha256sum "$binary_name" > "${binary_name}.sha256"
    
    print_color $GREEN "Built: $binary_name"
    print_color $GREEN "Checksum: ${binary_name}.sha256"
    
    # Show file info
    ls -lh "$binary_name"
    cat "${binary_name}.sha256"
}

# Main script logic
case "${1:-help}" in
    "current"|"version")
        show_version
        ;;
    "print-full")
        print_full_version
        ;;
    "major")
        update_major "$2"
        ;;
    "minor")
        update_minor "$2"
        ;;
    "build")
        build_release
        ;;
    "help"|*)
        print_color $BLUE "Quick SSM Version Management"
        echo
        print_color $YELLOW "Usage: $0 <command> [options]"
        echo
        print_color $GREEN "Commands:"
        print_color $YELLOW "  current, version    Show current version"
        print_color $YELLOW "  major <number>      Update major version"
        print_color $YELLOW "  minor <number>      Update minor version"
        print_color $YELLOW "  build               Build release binary with current date"
        print_color $YELLOW "  help                Show this help message"
        echo
        print_color $GREEN "Examples:"
        print_color $YELLOW "  $0 current          # Show current version"
        print_color $YELLOW "  $0 major 2          # Update to major version 2"
        print_color $YELLOW "  $0 minor 1          # Update to minor version 1"
        print_color $YELLOW "  $0 build            # Build with current date"
        echo
        print_color $GREEN "Version format: major.minor.YYYYMMDD"
        print_color $YELLOW "Example: v1.0.20241201"
        ;;
esac
