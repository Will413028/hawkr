import 'package:hawkr_app_update/hawkr_app_update.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/networking/app_version_headers.dart'
    show currentRawAppVersion;
import '../../../core/networking/connect_provider.dart';
import '../../../shared/utils/hk_log.dart';
import '../../../shared/utils/platform_io.dart' show Platform;

part 'force_update_provider.g.dart';

/// App 強制更新 gate 的單一狀態來源（`packages/hawkr_app_update/docs/spec.md`）；攔截一律由 GoRouter 頂層 redirect 執行（見
/// `core/router/redirect_logic.dart`）。
///
/// 檢查的規則（只在 iOS、3 秒逾時、失敗一律放行、回到前景的節流）在兩支 App 共用的
/// `hawkr_app_update`，這裡只把結果放進 state。state 是 freeze 過的 CheckAppVersionResponse，有值相等，
/// 結果沒變就不會白白通知 router 重判。CheckAppVersion 是公開的 service，不帶身分。
@Riverpod(keepAlive: true)
class ForceUpdateGate extends _$ForceUpdateGate {
  late final VersionGate _gate = VersionGate(
    client: ref.read(contentClientProvider),
    appType: AppType.APP_TYPE_SELLER,
    isIOS: () => Platform.isIOS,
    clientVersion: () => currentRawAppVersion,
    log: (message) => HKLog.logW('[AppVersionGate] $message'),
  );

  @override
  CheckAppVersionResponse build() => versionGateAllowed;

  /// 冷啟動檢查（不看節流）：使用者更新完 App 後最需要立刻重判的時候。
  Future<void> checkOnStartup() async => state = await _gate.checkOnStartup();

  /// 回到前景的檢查：距上次成功的檢查未滿 10 分鐘就跳過。
  Future<void> checkOnResume() async => state = await _gate.checkOnResume();
}
