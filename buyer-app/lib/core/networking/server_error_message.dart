import 'package:hawkr_api/errors.dart' show ErrorFacts;
import 'package:hawkr_api/transport.dart' show ConnectException;

/// 伺服器給使用者的文字（LocalizedMessage，是文字的唯一來源）；不是 Connect 錯誤（連線問題）或沒有帶文字時 null，
/// 由呼叫端決定通用句。
String? serverErrorMessage(Object error) {
  if (error is! ConnectException) return null;
  final text = ErrorFacts.fromException(error).localizedMessage;
  return text == null || text.isEmpty ? null : text;
}
