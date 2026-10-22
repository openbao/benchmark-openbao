test "transit_encrypt" "transit" {
    weight = 100
    config {
      payload_len = 128
      context_len = 32

      keys {
        type = "external-key"
        external_key_ref = "transit:aes-key"
      }
    }
}
