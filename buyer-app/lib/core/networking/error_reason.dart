import 'package:hawkr_api/errors.dart';
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/error_reason.pbenum.dart';
import 'package:hawkr_api/transport.dart' show ConnectException;

/// [error] 帶的 hawkr `ErrorReason`；沒有（或不認得）回 null。畫面依它決定流程（例如「已評價」），
/// 顯示的文字仍由 `errorText` 取伺服器的 LocalizedMessage。
ErrorReason? errorReasonOf(Object error) {
  final decision = classifyError(
    ErrorFacts.fromException(ConnectException.from(error)),
  );
  return decision.action == ErrorAction.reason ? decision.reason : null;
}
