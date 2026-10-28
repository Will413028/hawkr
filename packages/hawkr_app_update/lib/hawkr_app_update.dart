/// App 版本門檻的檢查邏輯（規格見 `docs/spec.md`），buyer-app 與 seller-app 共用（ADR
/// `backend/docs/adr/2026-09-27-app-version-gate-shared-package.md`）。強制更新的畫面、路由的攔截與
/// provider 留在各自的 App。
///
/// 版本比較只在後端（ContentPublicService 的 CheckAppVersion），App 只讀 `updateRequired`：
/// - 只在 iOS 檢查（App 只發 iOS），讀不到自己的版本也不檢查；
/// - 整個呼叫最多 [versionGateTimeout]，弱網下啟動畫面不能卡到共用的逾時才放行；
/// - 任何失敗一律放行（fail-open）並保留上一次的結果：後端故障不能把使用者鎖在門外；
/// - 冷啟動一定檢查；回到前景時，距上次**成功**的檢查未滿 [versionGateRecheckInterval] 就跳過，失敗不更新
///   這個錨點，下次回到前景立刻重試。
library;

import 'dart:async';

import 'package:hawkr_api/gen/hawkr/content/v1alpha1/content.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/content/v1alpha1/content.pb.dart';
import 'package:hawkr_api/transport.dart';

export 'package:hawkr_api/gen/hawkr/content/v1alpha1/content.pb.dart'
    show AppType, CheckAppVersionResponse;

/// 一次檢查（含重試）的上限。
const Duration versionGateTimeout = Duration(seconds: 3);

/// 回到前景時重新檢查的節流間隔：`resumed` 每次切回前景都會觸發，不節流會打出請求風暴，
/// 也會排擠同一個 IP 的其他匿名公開讀取。
const Duration versionGateRecheckInterval = Duration(minutes: 10);

/// 放行的結果：還沒檢查、或檢查不了時的狀態。回應都 freeze 成唯讀，訊息有值相等，結果沒變時不會白白通知
/// router 重判。
final CheckAppVersionResponse versionGateAllowed = CheckAppVersionResponse()
  ..freeze();

/// 發起版本檢查、保存結果。App 的 provider 把 [checkOnStartup]、[checkOnResume] 的回傳值放進自己的 state。
class VersionGate {
  VersionGate({
    required ContentPublicServiceClient client,
    required AppType appType,
    required bool Function() isIOS,
    required FutureOr<String> Function() clientVersion,
    void Function(String message)? log,
    DateTime Function() now = DateTime.now,
  }) : _client = client,
       _appType = appType,
       _isIOS = isIOS,
       _clientVersion = clientVersion,
       _log = log ?? ((_) {}),
       _now = now;

  final ContentPublicServiceClient _client;
  final AppType _appType;
  final bool Function() _isIOS;
  final FutureOr<String> Function() _clientVersion;
  final void Function(String) _log;
  final DateTime Function() _now;

  CheckAppVersionResponse _result = versionGateAllowed;
  DateTime? _lastSuccessAt;

  /// 目前的結果。
  CheckAppVersionResponse get result => _result;

  /// 冷啟動的檢查：不看節流，那是使用者更新完 App 後最需要立刻重判的時候。
  Future<CheckAppVersionResponse> checkOnStartup() =>
      _check(respectThrottle: false);

  /// 回到前景的檢查：距上次成功的檢查未滿 [versionGateRecheckInterval] 就不檢查。
  Future<CheckAppVersionResponse> checkOnResume() =>
      _check(respectThrottle: true);

  Future<CheckAppVersionResponse> _check({
    required bool respectThrottle,
  }) async {
    if (!_isIOS()) return _result;
    final last = _lastSuccessAt;
    if (respectThrottle &&
        last != null &&
        _now().difference(last) < versionGateRecheckInterval) {
      return _result;
    }
    try {
      final version = await _clientVersion();
      if (version.isEmpty) {
        _log('讀不到 App 的版本，不檢查（放行）');
        return _result;
      }
      final res = await withTimeout(
        versionGateTimeout,
        (signal) => _client.checkAppVersion(
          CheckAppVersionRequest(appType: _appType, clientVersion: version),
          signal: signal,
        ),
      );
      _lastSuccessAt = _now();
      _result = res..freeze();
    } catch (e) {
      // 逾時、連線失敗、回應不對一律放行：網路不能用時，使用者沒辦法自救。
      _log('版本檢查失敗，放行：$e');
    }
    return _result;
  }
}
