test "jwt_auth" "jwt_auth1" {
  weight = 100
  config {
    auth {}

    role {
      name            = "my-jwt-role"
      role_type       = "jwt"
      bound_audiences = ["https://vault.plugin.auth.jwt.test"]
      user_claim      = "https://vault/user"
    }
  }
}
