# System Mount Configuration Options

This benchmark tests the performance of mounting auth and secret engines.

## Test Parameters

### Configuration `config`

- `mount_type` `(string: "secret")` - type of plugin to mount; either `secret`
  or `auth`.
- `plugin` `(string: "kv-v2")` - plugin engine to create.

## Example configuration

```hcl
test "mount" "mount_test" {
    weight = 100
    config {
      mount_type = "secret"
      plugin = "pki"
    }
}
```
