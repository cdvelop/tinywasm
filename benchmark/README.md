# TinyWasm Benchmark System

This benchmark system provides automated performance comparison between Go standard compiler and TinyGo compiler for WebAssembly compilation.

## Overview

The benchmark system addresses the need to:
- Compare build times between compilers
- Measure output file sizes
- Avoid code duplication in test scenarios
- Provide automated performance metrics

## Directory Structure

```
benchmark/
├── scripts/
│   ├── unified-benchmark.sh   # ✅ RECOMMENDED - Unified approach avoiding duplication
│   ├── simple-benchmark.sh    # ✅ UPDATED - Simple benchmark using shared/
│   ├── benchmark.sh           # ⚠️ LEGACY - Complex benchmark (being deprecated)
│   ├── clean.sh               # ✅ Cleanup script
│   └── unified_results.txt    # Generated results file
├── shared/
│   └── main.go               # ✅ SINGLE SOURCE - Used by all benchmarks
└── README.md                 # This documentation
```

**Note**: The `examples/` directory has been removed to eliminate code duplication. All benchmarks now use the unified `shared/main.go` file.

## Scripts

### unified-benchmark.sh (⭐ RECOMMENDED)

The unified benchmark script uses the shared example to avoid code duplication while providing comprehensive performance metrics.

**Usage:**
```bash
cd benchmark/scripts
./unified-benchmark.sh
```

### simple-benchmark.sh (✅ UPDATED)

A simplified version that also uses the shared example for quick performance checks.

**Usage:**
```bash
cd benchmark/scripts
./simple-benchmark.sh
```

**Example Output:**
```
TinyWasm Simple Benchmark
=========================

Testing shared example with both compilers

Building with go...
  Build time: 531ms
  File size: 1605526 bytes

Building with tinygo...
  Build time: 2631ms
  File size: 171171 bytes

Benchmark Results:
go,531,1605526
tinygo,2631,171171
```

### benchmark.sh (⚠️ LEGACY)

Complex benchmark with multiple optimization levels. **Being deprecated** in favor of unified approach.

### clean.sh

Removes all generated files and temporary build artifacts.

## Benchmark Results Analysis

### Typical Performance Metrics

| Metric | Go Standard | TinyGo | Difference |
|--------|-------------|--------|------------|
| Build Time | ~200-300ms | ~1000-1500ms | TinyGo ~4-5x slower |
| File Size | ~1.6MB | ~170KB | TinyGo ~90% smaller |
| Use Case | Development | Production | - |

### Results File Format

The `unified_results.txt` file contains CSV format:
```
compiler,build_time_ms,file_size_bytes
go,274,1605518
tinygo,1080,171165
```

## Code Duplication Solution ✅ IMPLEMENTED

The benchmark system **completely eliminates** code duplication through:

1. **Single Source**: `shared/main.go` contains the only example code
2. **No Examples Directory**: Removed redundant `examples/` directory  
3. **Unified Scripts**: Both `unified-benchmark.sh` and `simple-benchmark.sh` use shared code
4. **Consistent Results**: All benchmarks use identical source code

### Before vs After

**❌ Before (Duplicated):**
```
examples/go-standard/main.go    # Duplicate code
examples/tinygo/main.go         # Duplicate code  
shared/main.go                  # Third copy
```

**✅ After (Unified):**
```
shared/main.go                  # SINGLE SOURCE
```

### Implementation Details

All benchmark scripts now use the same pattern:
```bash
# Copy shared example
cp "$SHARED_DIR/main.go" "temp_main.go"

# Compile with chosen compiler
$COMPILER build -o output.wasm temp_main.go

# Clean up
rm temp_main.go
```

## Integration with TinyWasm

The benchmark system integrates with TinyWasm's dynamic compiler selection:

```go
// Example: Run benchmark programmatically
tw := tinywasm.New(config)

// Test with standard Go
tw.SetTinyGoCompiler(false)
// Compile and measure...

// Test with TinyGo  
tw.SetTinyGoCompiler(true)
// Compile and measure...
```

## Best Practices

1. **Run Multiple Times**: Build times can vary, run several iterations
2. **Clean Environment**: Ensure clean state between runs
3. **Consistent Conditions**: Same hardware, OS state for fair comparison
4. **Monitor Resources**: Check CPU, memory usage during benchmarks

## Troubleshooting

### Common Issues

1. **Script Not Executable**
   ```bash
   chmod +x benchmark/scripts/*.sh
   ```

2. **TinyGo Not Found**
   - Ensure TinyGo is installed and in PATH
   - Script will show error if TinyGo unavailable

3. **Permission Issues**
   - Check write permissions in benchmark directory
   - Ensure scripts can create temporary files

### Error Messages

- `tinygo not found`: Install TinyGo or use go-only benchmarks
- `Permission denied`: Run `chmod +x` on script files
- `No such file or directory`: Check working directory and paths


