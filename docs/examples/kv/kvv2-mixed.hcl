test "kvv2_read" "kvv2_read_test" {
    weight = 50
    config {
        numkvs = 100
        kvsize = 1000
    }
}

test "kvv2_write" "kvv2_write_test" {
    weight = 50
    config {
        numkvs = 100
        kvsize = 1000
    }
}
