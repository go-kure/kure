# Validation Examples

This directory contains examples demonstrating Kure's built-in validation features.

## Bundle Interval Validation

**File**: `bundle-intervals.yaml`

Demonstrates proper configuration of time interval fields in Bundle resources:

- **Valid Examples**: Recommended patterns and edge cases
- **Invalid Examples**: Common mistakes and validation errors (commented out)
- **Error Messages**: Examples of validation error output

### Key Validation Rules

- **Format**: Go `time.Duration` syntax (`1s`, `5m`, `1h`, `1h30m`)
- **Range**: 1 second minimum, 24 hours maximum  
- **Fields**: `interval`, `timeout`, `retryInterval`

### Best Practices

- **Reconciliation**: Use `5m` to `30m` for most applications
- **Timeouts**: Set 2-3x longer than expected deployment time
- **Retry Intervals**: Use `1m` to `5m` for faster failure recovery
- **Production**: Avoid very short intervals (`<1m`) to reduce API load

### Testing Validation

To test validation with these examples:

```bash
# This will validate the YAML and show any errors
kure validate examples/validation/bundle-intervals.yaml

# Or test programmatically
go run examples/validation/test-validation.go
```

For more information, see the main [README.md](../../README.md#configuration-validation) validation section.