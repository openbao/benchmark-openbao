test "acl_policy_read" "acl_read_test" {
    weight = 100
    config {
      policies = 100
      paths = 25
      path_length = 150
    }
}
