#!/bin/bash

# TinyWasm Simple Benchmark Script  
# Compares Go standard vs TinyGo compilation using shared example

set -e

BENCHMARK_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SHARED_DIR="$BENCHMARK_DIR/../shared"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}TinyWasm Simple Benchmark${NC}"
echo "========================="

# Build function using shared example
build_with_compiler() {
    local compiler="$1"  # "go" or "tinygo"
    
    echo -e "\n${GREEN}Building with $compiler...${NC}"
    cd "$BENCHMARK_DIR"
    
    # Copy shared example
    cp "$SHARED_DIR/main.go" "temp_main.go"
    
    local start_time=$(date +%s%N)
    
    if [ "$compiler" = "tinygo" ]; then
        if ! command -v tinygo &> /dev/null; then
            echo -e "${YELLOW}TinyGo not installed, skipping${NC}"
            return 1
        fi
        tinygo build -o main-$compiler.wasm -target wasm temp_main.go
    else
        GOOS=js GOARCH=wasm go build -o main-$compiler.wasm temp_main.go
    fi
    
    local end_time=$(date +%s%N)
    local build_time=$(( (end_time - start_time) / 1000000 ))
    local file_size=$(stat -c%s main-$compiler.wasm 2>/dev/null || stat -f%z main-$compiler.wasm)
    
    echo "  Build time: ${build_time}ms"
    echo "  File size: ${file_size} bytes"
    
    # Store results
    echo "$compiler,$build_time,$file_size" >> "$BENCHMARK_DIR/simple_results.txt"
    
    # Cleanup
    rm -f main-$compiler.wasm temp_main.go
}

# Main execution
main() {
    # Clean previous results
    rm -f "$BENCHMARK_DIR/simple_results.txt"
    
    echo -e "\n${YELLOW}Testing shared example with both compilers${NC}"
    
    # Test both compilers using shared example
    build_with_compiler "go"
    build_with_compiler "tinygo"
    
    # Show comparison results
    if [[ -f "$BENCHMARK_DIR/simple_results.txt" ]]; then
        echo -e "\n${BLUE}Benchmark Results:${NC}"
        cat "$BENCHMARK_DIR/simple_results.txt"
    fi
    
    echo -e "\n${GREEN}Simple benchmark completed!${NC}"
}

main "$@"
