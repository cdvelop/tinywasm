#!/bin/bash

# TinyWasm Benchmark Script
# Compares performance between Go standard compiler and TinyGo compiler

set -e

BENCHMARK_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$BENCHMARK_DIR")" 
SHARED_DIR="$PROJECT_ROOT/shared"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}TinyWasm Compiler Benchmark${NC}"
echo "=============================="

# Cleanup function
cleanup() {
    echo -e "\n${YELLOW}Cleaning up...${NC}"
    rm -f "$EXAMPLES_DIR"/*/main.wasm
    rm -f "$EXAMPLES_DIR"/*/main.js
}

# Set trap for cleanup on exit
trap cleanup EXIT

# Function to build with Go standard compiler
build_go_standard() {
    local example_dir="$1"
    local example_name="$(basename "$example_dir")"
    
    echo -e "\n${GREEN}Building $example_name with Go standard compiler...${NC}"
    cd "$example_dir"
    
    local start_time=$(date +%s%N)
    GOOS=js GOARCH=wasm go build -o main.wasm main.go
    local end_time=$(date +%s%N)
    
    local build_time=$(( (end_time - start_time) / 1000000 ))
    local file_size=$(stat -c%s main.wasm 2>/dev/null || stat -f%z main.wasm)
    
    echo "  Build time: ${build_time}ms"
    echo "  File size: ${file_size} bytes"
    
    cd "$BENCHMARK_DIR/scripts"
    echo "$build_time,$file_size" > "${example_name}_go_standard.txt"
}

# Function to build with TinyGo compiler
build_tinygo() {
    local example_dir="$1"
    local example_name="$(basename "$example_dir")"
    
    echo -e "\n${GREEN}Building $example_name with TinyGo compiler...${NC}"
    cd "$example_dir"
    
    # Check if TinyGo is installed
    if ! command -v tinygo &> /dev/null; then
        echo -e "${RED}TinyGo is not installed. Skipping TinyGo benchmark.${NC}"
        return 1
    fi
    
    local start_time=$(date +%s%N)
    tinygo build -o main.wasm -target wasm main.go
    local end_time=$(date +%s%N)
    
    local build_time=$(( (end_time - start_time) / 1000000 ))
    local file_size=$(stat -c%s main.wasm 2>/dev/null || stat -f%z main.wasm)
    
    echo "  Build time: ${build_time}ms"
    echo "  File size: ${file_size} bytes"
    
    cd "$BENCHMARK_DIR/scripts"
    echo "$build_time,$file_size" > "${example_name}_tinygo.txt"
    return 0
}

# Function to compare results
compare_results() {
    local example_name="$1"
    
    local go_file="${example_name}_go_standard.txt"
    local tinygo_file="${example_name}_tinygo.txt"
    
    if [[ -f "$go_file" && -f "$tinygo_file" ]]; then
        local go_time=$(cut -d',' -f1 "$go_file")
        local go_size=$(cut -d',' -f2 "$go_file")
        local tinygo_time=$(cut -d',' -f1 "$tinygo_file")
        local tinygo_size=$(cut -d',' -f2 "$tinygo_file")
        
        echo -e "\n${BLUE}Comparison for $example_name:${NC}"
        echo "Go Standard: ${go_time}ms, ${go_size} bytes"
        echo "TinyGo:      ${tinygo_time}ms, ${tinygo_size} bytes"
        
        local time_diff=$(( tinygo_time - go_time ))
        local size_diff=$(( tinygo_size - go_size ))
        
        if [ $time_diff -lt 0 ]; then
            echo -e "Time: TinyGo is ${GREEN}$((-time_diff))ms faster${NC}"
        else
            echo -e "Time: Go standard is ${GREEN}${time_diff}ms faster${NC}"
        fi
        
        if [ $size_diff -lt 0 ]; then
            echo -e "Size: TinyGo is ${GREEN}$((-size_diff)) bytes smaller${NC}"
        else
            echo -e "Size: Go standard is ${GREEN}${size_diff} bytes smaller${NC}"
        fi
    fi
}

# Main benchmark execution
main() {
    echo -e "${YELLOW}Starting benchmark...${NC}"
    
    # Find all example directories
    for example_dir in "$EXAMPLES_DIR"/*; do
        if [[ -d "$example_dir" && -f "$example_dir/main.go" ]]; then
            local example_name=$(basename "$example_dir")
            
            echo -e "\n${YELLOW}Benchmarking: $example_name${NC}"
            echo "==============================="
            
            # Build with both compilers
            build_go_standard "$example_dir"
            if build_tinygo "$example_dir"; then
                compare_results "$example_name"
            fi
            
            # Clean up WASM files after each test
            rm -f "$example_dir"/*.wasm
        fi
    done
    
    echo -e "\n${GREEN}Benchmark completed!${NC}"
}

# Run main function
main "$@"
