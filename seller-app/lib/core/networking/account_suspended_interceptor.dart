import 'package:hawkr_api/errors.dart'
    show ErrorAction, ErrorFacts, classifyError;
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/error_reason.pbenum.dart'
    show ErrorReason;
import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.connect.spec.dart'
    show SocialPublicService;
import 'package:hawkr_api/transport.dart';

/// 背景寫入的方法（使用者沒有主動操作的打點）：帳號停權時收到 `ACCOUNT_SUSPENDED` 靜默略過，不轉到受限頁。
final Set<String> backgroundWriteProcedures = {
  SocialPublicService.recordPostView.procedure,
};

/// 帳號停權（`permission_denied` 加 `ACCOUNT_SUSPENDED`）：前景請求呼叫 [onSuspended]，錯誤原樣丟出；
/// [silentProcedures] 內的背景寫入不呼叫。
Interceptor accountSuspendedInterceptor(
  void Function() onSuspended, {
  Set<String>? silentProcedures,
}) {
  final silent = silentProcedures ?? backgroundWriteProcedures;
  return <I extends Object, O extends Object>(next) {
    return (req) async {
      try {
        return await next(req);
      } catch (raw) {
        final decision = classifyError(
          ErrorFacts.fromException(ConnectException.from(raw)),
        );
        if (decision.action == ErrorAction.reason &&
            decision.reason == ErrorReason.ACCOUNT_SUSPENDED &&
            !silent.contains(req.spec.procedure)) {
          onSuspended();
        }
        rethrow;
      }
    };
  };
}
