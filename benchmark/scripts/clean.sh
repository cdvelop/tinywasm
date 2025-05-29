#!/bin/bash

# TinyWasm Benchmark Cleanup Script
# Removes generated files and temporary build artifacts

echo "TinyWasm Benchmark Cleanup"
echo "========================="

# Change to benchmark directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BENCHMARK_DIR="$(dirname "$SCRIPT_DIR")"

# Remove result files
echo "Removing result files..."
rm -f "$SCRIPT_DIR/unified_results.txt"
rm -f "$SCRIPT_DIR/simple_results.txt"
rm -f "$SCRIPT_DIR/benchmark_results.txt"

# Remove generated WASM files
echo "Removing WASM files..."
find "$BENCHMARK_DIR" -name "*.wasm" -delete
find "$BENCHMARK_DIR" -name "main-*.wasm" -delete

# Remove temporary files
echo "Removing temporary files..."
find "$BENCHMARK_DIR" -name "*.tmp" -delete
find "$BENCHMARK_DIR" -name "temp_*" -delete

# Remove generated JavaScript files (if any)
echo "Removing temporary JS files..."
find "$BENCHMARK_DIR" -name "wasm_exec_temp.js" -delete

echo "✓ Cleanup completed!"
echo "All generated files have been removed."
