test "acl_policy_write" "acl_write_test" {
    weight = 100
    config {
      policies = 100
      paths = 25
      path_length = 150
    }
}
