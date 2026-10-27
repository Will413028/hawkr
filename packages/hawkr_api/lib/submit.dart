/// 表單送出失敗時要做的事（ADR backend/docs/adr/2026-09-25-api-error-model.md 的 D7）。
///
/// 與 `packages/hawkr-api-ts/src/connect/submit.ts` 同形：分類沿用 [classifyError]，這裡只把結果收斂成
/// 表單要處理的三種情況。
library;

import 'package:connectrpc/connect.dart';

import 'errors.dart';
import 'gen/hawkr/common/v1alpha1/error_reason.pbenum.dart';

/// 表單送出失敗時要做的事。
sealed class SubmitError {
  const SubmitError();
}

/// 版本衝突：保留使用者的輸入，顯示最新版本讓使用者對照後再送出。
final class SubmitConflict extends SubmitError {
  const SubmitConflict();
}

/// 欄位錯誤：[fields] 的 key 是請求訊息的 proto 路徑（例如 `banner.title`）。
final class SubmitFields extends SubmitError {
  const SubmitFields(this.fields, {this.message});

  final Map<String, String> fields;

  /// 伺服器給使用者的整體說明（LocalizedMessage）；沒有時為 null。
  final String? message;
}

/// 其他錯誤：[message] 是要顯示給使用者的文字。
final class SubmitMessage extends SubmitError {
  const SubmitMessage(this.message);

  final String message;
}

/// 把送出失敗的 [error] 分類成表單要做的事。[idempotency] 與 [request] 與 [ErrorFacts.fromException] 相同。
///
/// 沒有伺服器文字時的預設文字由 [defaults] 給（目錄在 `hawkr_ui_text`，不能由本套件依賴）；欄位沒有文字時用
/// [Code.invalidArgument] 的預設文字，取消用 [Code.internal] 的。
SubmitError submitError(
  Object error, {
  required DefaultErrorText defaults,
  Idempotency? idempotency,
  Object? request,
}) {
  final e = ConnectException.from(error);
  final d = classifyError(
    ErrorFacts.fromException(e, idempotency: idempotency, request: request),
  );
  if (d.action == ErrorAction.reason &&
      d.reason == ErrorReason.VERSION_CONFLICT) {
    return const SubmitConflict();
  }
  if (d.action == ErrorAction.fields && d.fields.isNotEmpty) {
    return SubmitFields({
      for (final f in d.fields)
        f.field: f.message ?? defaults(Code.invalidArgument),
    }, message: d.message);
  }
  return SubmitMessage(
    d.message ?? defaults(e.code == Code.canceled ? Code.internal : e.code),
  );
}

/// 某個畫面欄位的錯誤文字：第一個符合的路徑，或以它開頭的子路徑（`banner.action` 也涵蓋 `banner.action.url`）。
String? fieldMessage(Map<String, String> fields, List<String> paths) {
  for (final path in paths) {
    for (final MapEntry(key: field, value: message) in fields.entries) {
      if (field == path ||
          field.startsWith('$path.') ||
          field.startsWith('$path[')) {
        return message;
      }
    }
  }
  return null;
}
