import 'package:hawkr_api/errors.dart'
    show DefaultErrorText, ErrorFacts, errorText;
import 'package:hawkr_api/transport.dart' show ConnectException;

/// 送出訊息失敗的呈現決定：顯示的文字永遠是伺服器的 LocalizedMessage（沒有時是 code 的預設文字，取自共用文案目錄）。
class ChatSendFailure {
  const ChatSendFailure({required this.message, required this.readOnly});

  final String message;

  /// 這個對話不能再送（對方封鎖、店家停業、對方帳號已刪除）：輸入框改為唯讀並顯示 [message]。
  final bool readOnly;
}

/// 這三個 `ErrorReason` 之下對話不能再送。
const _readOnlyReasons = {'CHAT_BLOCKED', 'STORE_CLOSED', 'RECEIVER_DELETED'};

ChatSendFailure classifyChatSendError(
  Object error, {
  required DefaultErrorText defaults,
}) {
  final e = ConnectException.from(error);
  final reason = ErrorFacts.fromException(e).reason;
  return ChatSendFailure(
    message: errorText(error, defaults: defaults),
    readOnly: reason != null && _readOnlyReasons.contains(reason),
  );
}
