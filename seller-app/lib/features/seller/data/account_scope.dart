import '../../../core/session/session_provider.dart';

/// 讀取失敗立刻成為 error 狀態，不自動重試：呼叫端自己決定怎麼處理失敗。
Duration? noRetry(int retryCount, Object error) => null;

/// 目前登入帳號的 id；沒有登入時是 null。以帳號為鍵的 provider 用它取鍵。
String? signedInAccountId(SessionModel session) =>
    session.isLoggedIn ? session.accountId : null;
