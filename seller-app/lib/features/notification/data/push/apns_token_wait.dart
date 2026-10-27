import 'dart:async';

typedef ApnsTokenFetcher = Future<String?> Function();

/// 等待 iOS APNs token 就緒，緩解 firebase_messaging 在 iOS 上的已知時序坑：
/// APNs 註冊是非同步的，若原生尚未把 token 交給裝置就呼叫
/// `FirebaseMessaging.getToken()`，套件會拋出 `apns-token-not-set`
/// （hawkr_seller#2009 App 啟動、#2010 登入後，兩個呼叫點皆踩到同一根因）。
///
/// 有界輪詢（預設最多 5 次、間隔 1 秒），逾時回傳 null 而非無限等待。
Future<String?> waitForApnsToken(
  ApnsTokenFetcher fetchApnsToken, {
  int maxAttempts = 5,
  Duration interval = const Duration(seconds: 1),
  Future<void> Function(Duration) delay = Future<void>.delayed,
}) async {
  for (var attempt = 0; attempt < maxAttempts; attempt++) {
    final token = await fetchApnsToken();
    if (token != null) return token;
    if (attempt < maxAttempts - 1) await delay(interval);
  }
  return null;
}
