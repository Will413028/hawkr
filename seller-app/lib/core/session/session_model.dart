import 'package:freezed_annotation/freezed_annotation.dart';

part 'session_model.freezed.dart';

/// Session 資料模型：只放登入的身分。店家（`GetMyStore`）的狀態在 `currentStoreProvider`。
///
/// 使用 Freezed 確保不可變性與 copyWith 支援。
@Freezed(toStringOverride: false)
abstract class SessionModel with _$SessionModel {
  const factory SessionModel({
    /// 帳號 ID（identity 的 opaque 字串，不解析成數字）
    String? accountId,

    /// 是否已登入：Clerk 登入中，而且有帳號 ID（見 `Session`）。憑證是 Clerk 的 session token，不在這裡。
    @Default(false) bool isLoggedIn,
  }) = _SessionModel;
}

extension SessionSafeLogX on SessionModel {
  /// Explicit, secret-free representation for diagnostics.
  String toSafeLogString() =>
      'SessionModel(accountId: $accountId, isLoggedIn: $isLoggedIn)';
}
