# identity/internal/oidc

App 原生登入的 ID token 驗證。

## 不變量

- Google App 登入的 ID token 驗 `iss`（兩種寫法）、`aud` 為 web client ID、`azp` 在允許清單、`nonce`；LINE 以 ES256 與 JWKS 驗。守門：`TestGoogleAppLoginVerifiesTheIDToken`、`TestLineAppLoginVerifiesTheES256IDTokenWithTheJWKS`（`internal/provider`）。規格：§1。
- nonce 先比對 ID token、通過才單次消耗；`RevokeSession` 的重新驗證期限是 `session.ReauthWindow`。守門：`TestRevokeSessionNeedsRecentAuthentication`（`internal/httpapi/identity`）。規格：§1。
