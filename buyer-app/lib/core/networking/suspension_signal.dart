import 'dart:convert';

import 'package:dio/dio.dart';
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

bool isBackgroundConnectProcedure(String procedure) =>
    backgroundConnectProcedures.contains(procedure);

/// 舊 gateway 的 REST 停權訊號：403，且 JSON envelope 的 `reason` 是 `ACCOUNT_SUSPENDED`（A7-D18）。停權只擋寫入，
/// 所以不必看 HTTP method。[data] 是已解碼的 body（Map），或未解碼的 JSON 字串。
bool isRestSuspensionBody(int? status, Object? data) {
  if (status != 403) return false;
  Object? body = data;
  if (body is String) {
    try {
      body = jsonDecode(body);
    } catch (_) {
      return false;
    }
  }
  return body is Map && body['reason'] == ErrorReason.ACCOUNT_SUSPENDED.name;
}

/// 同 [isRestSuspensionBody]，另外讀得了串流 body（`ResponseType.stream` 的回應，body 是 [ResponseBody]）。
Future<bool> isRestSuspension(Response<dynamic> response) async {
  final status = response.statusCode;
  if (status != 403) return false;
  final data = response.data;
  if (data is ResponseBody) {
    try {
      final text = await utf8.decodeStream(data.stream);
      return isRestSuspensionBody(status, text);
    } catch (_) {
      return false;
    }
  }
  return isRestSuspensionBody(status, data);
}

/// 掛在自己處理非 2xx 回應（`validateStatus` 放行）的 Dio 上：使用者主動的寫入收到 REST 停權時呼叫 [onSuspended]。
class RestSuspensionInterceptor extends Interceptor {
  RestSuspensionInterceptor(this.onSuspended);

  final void Function() onSuspended;

  @override
  Future<void> onResponse(
    Response<dynamic> response,
    ResponseInterceptorHandler handler,
  ) async {
    if (await isRestSuspension(response)) onSuspended();
    handler.next(response);
  }
}
