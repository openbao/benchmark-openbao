test "acl_policy_read" "acl_read_test" {
    weight = 50
    config {
      policies = 100
      paths = 25
      path_length = 150
    }
}

test "acl_policy_write" "acl_write_test" {
    weight = 50
    config {
      policies = 100
      paths = 25
      path_length = 150
    }
}
