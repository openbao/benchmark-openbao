test "acl_policy_list" "acl_list_test" {
    weight = 100
    config {
      policies = 100
      paths = 25
      path_length = 150
    }
}
