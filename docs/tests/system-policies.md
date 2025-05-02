# System ACL Policy Configuration Options

This benchmark tests the performance of ACL policy reading and writing. It
writes a series of policies to unique different paths, reading them back or
modifying them.

## Test Parameters

### Configuration `config`

- `policies` `(int: 10)` - the number of policies in the working set to read
  or write.
- `path_length` `(int: 25)` - the length of the paths within the ACL policy.
- `paths` `(int: 1)` - how many paths within each policy.
- `capabilities` `([]string: ["create", "read", "update", "delete", "list", "sudo"])` - capabilities
  for each path.

## Example configuration

```hcl
test "acl_policy_write" "acl_write_test" {
    weight = 100
    config {
      policies = 100
      paths = 25
      path_length = 150
    }
}
```
