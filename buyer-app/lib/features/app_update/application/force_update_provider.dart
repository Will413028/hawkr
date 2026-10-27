import 'dart:io' show Platform;

import 'package:hawkr_app_update/hawkr_app_update.dart';
import 'package:flutter/foundation.dart' show kIsWeb;
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/networking/connect_provider.dart';
import '../../../shared/utils/app_version_utils.dart';
import '../../../shared/utils/hk_log.dart';

part 'force_update_provider.g.dart';

/// 版本門檻的結果，供 GoRouter redirect 讀取（設計書 54 §4.1）；攔截一律由 router redirect 執行。
///
/// 檢查的規則（只在 iOS、3 秒逾時、失敗一律放行、回到前景的節流）在兩支 App 共用的
/// `hawkr_app_update`，這裡只把結果放進 state。CheckAppVersion 是公開的 service，不帶身分：token 過期時
/// 不會觸發 refresh。
@Riverpod(keepAlive: true)
class ForceUpdate extends _$ForceUpdate {
  late final VersionGate _gate = VersionGate(
    client: ref.read(contentClientProvider),
    appType: AppType.APP_TYPE_BUYER,
    isIOS: () => !kIsWeb && Platform.isIOS,
    clientVersion: fetchRawAppVersion,
    log: (message) => HKLog.logW('[ForceUpdate] $message'),
  );

  @override
  CheckAppVersionResponse build() => versionGateAllowed;

  /// 冷啟動檢查（不看節流）。失敗一律放行，不會丟出例外。
  Future<void> check() async => state = await _gate.checkOnStartup();

  /// 回到前景的檢查：距上次成功的檢查未滿 10 分鐘就跳過。
  Future<void> checkThrottled() async => state = await _gate.checkOnResume();
}
