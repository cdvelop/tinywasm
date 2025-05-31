#!/bin/bash

# TinyWasm Advanced Benchmark
# Comprehensive benchmark comparing Go standard vs TinyGo for WASM development

set -e

BENCHMARK_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SHARED_DIR="$BENCHMARK_DIR/../shared"
RESULTS_FILE="$BENCHMARK_DIR/advanced_results.txt"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
CYAN='\033[0;36m'
NC='\033[0m'

# Configuration
ITERATIONS=5
echo -e "${BLUE}TinyWasm Advanced Benchmark${NC}"
echo "=============================="
echo -e "Iterations per compiler: ${CYAN}$ITERATIONS${NC}"
echo -e "Source file: ${CYAN}$SHARED_DIR/main.go${NC}"
echo ""

# Initialize results
> "$RESULTS_FILE"
echo "# TinyWasm Benchmark Results - $(date)" >> "$RESULTS_FILE"
echo "# Format: compiler,iteration,build_time_ms,file_size_bytes,success" >> "$RESULTS_FILE"

# Arrays to store results
declare -a go_times=()
declare -a go_sizes=()
declare -a tinygo_times=()
declare -a tinygo_sizes=()

# Test function for multiple iterations
test_compiler_multiple() {
    local compiler="$1"
    local results_array_times="$2"
    local results_array_sizes="$3"
    
    echo -e "\n${GREEN}Testing $compiler compiler ($ITERATIONS iterations)...${NC}"
    cd "$SHARED_DIR"
    
    local success_count=0
    
    for i in $(seq 1 $ITERATIONS); do
        echo -ne "  Iteration $i/$ITERATIONS... "
        
        # Clean previous builds
        rm -f main-$compiler.wasm 2>/dev/null || true
        
        local start_time=$(date +%s%N)
        local success=false
        local error_msg=""
        
        if [ "$compiler" = "tinygo" ]; then
            if command -v tinygo &> /dev/null; then
                if tinygo build -o main-$compiler.wasm -target wasm --no-debug main.go 2>/dev/null; then
                    success=true
                else
                    error_msg="TinyGo build failed"
                fi
            else
                error_msg="TinyGo not installed"
            fi
        else
            if GOOS=js GOARCH=wasm go build -o main-$compiler.wasm -tags dev main.go 2>/dev/null; then
                success=true
            else
                error_msg="Go build failed"
            fi
        fi
        
        if [ "$success" = true ]; then
            local end_time=$(date +%s%N)
            local build_time=$(( (end_time - start_time) / 1000000 ))
            local file_size=$(stat -c%s main-$compiler.wasm 2>/dev/null || stat -f%z main-$compiler.wasm)
            
            # Store results
            if [ "$compiler" = "go" ]; then
                go_times+=($build_time)
                go_sizes+=($file_size)
            else
                tinygo_times+=($build_time)
                tinygo_sizes+=($file_size)
            fi
            
            echo -e "${GREEN}✓${NC} ${build_time}ms, ${file_size} bytes"
            echo "$compiler,$i,$build_time,$file_size,true" >> "$RESULTS_FILE"
            success_count=$((success_count + 1))
        else
            echo -e "${RED}✗${NC} $error_msg"
            echo "$compiler,$i,0,0,false" >> "$RESULTS_FILE"
        fi
    done
    
    echo -e "  ${CYAN}Success rate: $success_count/$ITERATIONS${NC}"
    # Store success count for later use
    eval "${results_array_times}_success=$success_count"
}

# Calculate statistics
calculate_stats() {
    local array_name="$1"
    local -n arr=$array_name
    
    if [ ${#arr[@]} -eq 0 ]; then
        echo "0,0,0,0"
        return
    fi
    
    local sum=0
    local min=${arr[0]}
    local max=${arr[0]}
    
    for val in "${arr[@]}"; do
        sum=$((sum + val))
        if [ $val -lt $min ]; then min=$val; fi
        if [ $val -gt $max ]; then max=$val; fi
    done
    
    local avg=$((sum / ${#arr[@]}))
    echo "$avg,$min,$max,$sum"
}

# Main benchmark execution
echo -e "\n${YELLOW}Starting benchmark execution...${NC}"

# Test Go standard compiler
test_compiler_multiple "go" "go_times" "go_sizes"
go_success=${go_times_success:-0}

# Test TinyGo compiler  
test_compiler_multiple "tinygo" "tinygo_times" "tinygo_sizes"
tinygo_success=${tinygo_times_success:-0}

# Calculate statistics
echo -e "\n${BLUE}Calculating statistics...${NC}"

go_time_stats=$(calculate_stats go_times)
go_size_stats=$(calculate_stats go_sizes)
tinygo_time_stats=$(calculate_stats tinygo_times)
tinygo_size_stats=$(calculate_stats tinygo_sizes)

# Extract individual values
IFS=',' read -r go_avg_time go_min_time go_max_time go_total_time <<< "$go_time_stats"
IFS=',' read -r go_avg_size go_min_size go_max_size go_total_size <<< "$go_size_stats"
IFS=',' read -r tinygo_avg_time tinygo_min_time tinygo_max_time tinygo_total_time <<< "$tinygo_time_stats"
IFS=',' read -r tinygo_avg_size tinygo_min_size tinygo_max_size tinygo_total_size <<< "$tinygo_size_stats"

# Generate summary
echo -e "\n${BLUE}BENCHMARK RESULTS SUMMARY${NC}"
echo "=========================="

echo -e "\n${GREEN}Go Standard Compiler:${NC}"
echo "  Build Time: avg=${go_avg_time}ms, min=${go_min_time}ms, max=${go_max_time}ms"
echo "  File Size:  avg=${go_avg_size} bytes, min=${go_min_size} bytes, max=${go_max_size} bytes"
echo "  Success:    $go_success/$ITERATIONS builds"

echo -e "\n${GREEN}TinyGo Compiler:${NC}"
echo "  Build Time: avg=${tinygo_avg_time}ms, min=${tinygo_min_time}ms, max=${tinygo_max_time}ms"  
echo "  File Size:  avg=${tinygo_avg_size} bytes, min=${tinygo_min_size} bytes, max=${tinygo_max_size} bytes"
echo "  Success:    $tinygo_success/$ITERATIONS builds"

# Comparison
if [ $go_success -gt 0 ] && [ $tinygo_success -gt 0 ]; then
    echo -e "\n${CYAN}COMPARISON:${NC}"
    
    # Speed comparison
    if [ $go_avg_time -gt 0 ] && [ $tinygo_avg_time -gt 0 ]; then
        if [ $go_avg_time -lt $tinygo_avg_time ]; then
            speed_ratio=$((tinygo_avg_time * 100 / go_avg_time))
            echo "  Speed: Go is ${speed_ratio}% faster than TinyGo for development"
        else
            speed_ratio=$((go_avg_time * 100 / tinygo_avg_time))
            echo "  Speed: TinyGo is ${speed_ratio}% faster than Go"
        fi
    fi
    
    # Size comparison
    if [ $go_avg_size -gt 0 ] && [ $tinygo_avg_size -gt 0 ]; then
        if [ $tinygo_avg_size -lt $go_avg_size ]; then
            size_ratio=$((go_avg_size * 100 / tinygo_avg_size))
            echo "  Size: TinyGo produces ${size_ratio}% smaller binaries than Go"
        else
            size_ratio=$((tinygo_avg_size * 100 / go_avg_size))
            echo "  Size: Go produces ${size_ratio}% smaller binaries than TinyGo"
        fi
    fi
fi

# Development recommendation
echo -e "\n${YELLOW}DEVELOPMENT STRATEGY RECOMMENDATION:${NC}"
if [ $go_success -gt 0 ] && [ $tinygo_success -gt 0 ]; then
    if [ $go_avg_time -lt $tinygo_avg_time ]; then
        echo "  ✅ Use Go standard for DEVELOPMENT (faster builds)"
        echo "  ✅ Use TinyGo for PRODUCTION (smaller binaries)"
    else
        echo "  ⚠️  TinyGo is faster, consider using it for both development and production"
    fi
else
    echo "  ⚠️  Incomplete benchmark data - check compiler installations"
fi

# Cleanup
cd "$SHARED_DIR"
rm -f main-go.wasm main-tinygo.wasm 2>/dev/null || true

echo -e "\n${BLUE}Results saved to: ${CYAN}$RESULTS_FILE${NC}"
echo -e "${GREEN}Benchmark complete!${NC}"
