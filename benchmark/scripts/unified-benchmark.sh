#!/bin/bash

# TinyWasm Unified Benchmark
# Uses shared main.go with both compilers

set -e

BENCHMARK_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SHARED_DIR="$BENCHMARK_DIR/../shared"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${BLUE}TinyWasm Unified Benchmark${NC}"
echo "=========================="

# Test function for both compilers
test_compiler() {
    local compiler="$1"
    
    echo -e "\n${GREEN}Testing $compiler compiler...${NC}"
    cd "$SHARED_DIR"
    
    local start_time=$(date +%s%N)
    local success=false
    
    if [ "$compiler" = "tinygo" ]; then
        if command -v tinygo &> /dev/null; then
            if tinygo build -o main-$compiler.wasm -target wasm main.go 2>/dev/null; then
                success=true
            fi
        else
            echo -e "${YELLOW}TinyGo not installed${NC}"
            return 1
        fi
    else
        if GOOS=js GOARCH=wasm go build -o main-$compiler.wasm main.go 2>/dev/null; then
            success=true
        fi
    fi
    
    if [ "$success" = true ]; then
        local end_time=$(date +%s%N)
        local build_time=$(( (end_time - start_time) / 1000000 ))
        local file_size=$(stat -c%s main-$compiler.wasm 2>/dev/null || stat -f%z main-$compiler.wasm)
        
        echo "  ✓ Build time: ${build_time}ms"
        echo "  ✓ File size: ${file_size} bytes"
        
        # Store results
        echo "$compiler,$build_time,$file_size" >> "$BENCHMARK_DIR/unified_results.txt"
        
        # Cleanup
        rm -f main-$compiler.wasm
        return 0
    else
        echo -e "${RED}  ✗ Compilation failed${NC}"
        return 1
    fi
}

# Compare results
compare_results() {
    local results_file="$BENCHMARK_DIR/scripts/unified_results.txt"
    
    if [[ -f "$results_file" ]]; then
        echo -e "\n${BLUE}Benchmark Results:${NC}"
        echo "=================="
        
        local go_line=$(grep "^go," "$results_file" 2>/dev/null || echo "")
        local tinygo_line=$(grep "^tinygo," "$results_file" 2>/dev/null || echo "")
        
        if [[ -n "$go_line" ]]; then
            local go_time=$(echo "$go_line" | cut -d',' -f2)
            local go_size=$(echo "$go_line" | cut -d',' -f3)
            echo "Go Standard: ${go_time}ms, ${go_size} bytes"
        fi
        
        if [[ -n "$tinygo_line" ]]; then
            local tinygo_time=$(echo "$tinygo_line" | cut -d',' -f2)
            local tinygo_size=$(echo "$tinygo_line" | cut -d',' -f3)
            echo "TinyGo:      ${tinygo_time}ms, ${tinygo_size} bytes"
        fi
        
        if [[ -n "$go_line" && -n "$tinygo_line" ]]; then
            local size_diff=$(( tinygo_size - go_size ))
            local time_diff=$(( tinygo_time - go_time ))
            
            echo -e "\n${YELLOW}Comparison:${NC}"
            if [ $size_diff -lt 0 ]; then
                echo "  Size: TinyGo is $((-size_diff)) bytes smaller"
            else
                echo "  Size: Go standard is $size_diff bytes smaller"
            fi
            
            if [ $time_diff -lt 0 ]; then
                echo "  Speed: TinyGo is $((-time_diff))ms faster"
            else
                echo "  Speed: Go standard is ${time_diff}ms faster"
            fi
        fi
    fi
}

# Main execution
main() {
    # Clean previous results
    rm -f "$BENCHMARK_DIR/scripts/unified_results.txt"
    
    # Test both compilers
    test_compiler "go"
    test_compiler "tinygo"
    
    # Show comparison
    compare_results
    
    echo -e "\n${GREEN}Unified benchmark completed!${NC}"
}

main "$@"
