import 'package:hawkr_api/errors.dart';
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/error_reason.pbenum.dart';
import 'package:hawkr_api/transport.dart' show Code, ConnectException;

/// 停權的訊號：`permission_denied` 加上 domain 是 hawkr 的 `ACCOUNT_SUSPENDED`。其他的 `permission_denied`
/// （例如不是買家）不是停權。
bool isAccountSuspended(ConnectException error) {
  if (error.code != Code.permissionDenied) return false;
  final decision = classifyError(ErrorFacts.fromException(error));
  return decision.action == ErrorAction.reason &&
      decision.reason == ErrorReason.ACCOUNT_SUSPENDED;
}

/// 使用者沒有主動觸發的寫入（背景呼叫）。這些請求收到 `ACCOUNT_SUSPENDED` 時靜默略過：不進受限畫面、不跳提示
/// （A7-D17）。不在清單內的請求一律視為使用者主動，收到 `ACCOUNT_SUSPENDED` 就進受限狀態並提示。
///
/// 新增背景寫入（瀏覽計數、裝置註冊之類）時加在這裡。
const backgroundConnectProcedures = {
  '/hawkr.buyer.v1alpha1.BuyerAppService/RecordVisit',
  '/hawkr.buyer.v1alpha1.BuyerAppService/ClearVisits',
};

/// 背景寫入的 REST 路徑（舊 gateway）：裝置 token 的註冊與註銷。
const backgroundRestPaths = {'/app/notifications/device-token'};

bool isBackgroundConnectProcedure(String procedure) =>
    backgroundConnectProcedures.contains(procedure);

bool isBackgroundRestPath(String path) => backgroundRestPaths.contains(path);
