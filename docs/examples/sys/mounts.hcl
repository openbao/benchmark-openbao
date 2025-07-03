test "mount" "mount_test" {
    weight = 100
    config {
      mount_type = "secret"
      plugin = "pki"
    }
}
