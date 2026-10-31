/// App 提供的聯合登入 provider。值對應 identity 的 `LoginProvider`（Google、LINE）；不提供 Apple 登入。
enum OAuthProvider {
  google,
  line;

  String get label => switch (this) {
    google => 'Google',
    line => 'LINE',
  };
}
