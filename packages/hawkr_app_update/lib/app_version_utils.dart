import 'package:package_info_plus/package_info_plus.dart';

/// 取得格式化的 App 版本字串
///
/// 返回格式：`"v1.0.0(7)"`（含版次，且不含前綴 0）或 `"v1.0.0"`（無版次）
/// 失敗時返回空字串 `""`
Future<String> fetchAppVersion() async {
  try {
    final info = await PackageInfo.fromPlatform();
    final version = info.version.trim();
    final build = info.buildNumber.trim();
    if (version.isEmpty) return '';

    final cleanBuild = int.tryParse(build)?.toString() ?? build;
    return cleanBuild.isNotEmpty ? '$version($cleanBuild)' : version;
  } catch (_) {
    return '';
  }
}

/// 取得 bare semver 版本字串（`"1.0.3"`，**不含** build number）
///
/// 專供 App 版本 gate 與 `X-App-Version` header 使用（`packages/hawkr_app_update/docs/spec.md`）：
/// 後端以三段數字比對版本，**不可**餵 [fetchAppVersion] 的顯示字串
/// `"1.0.3(7)"`——那會讓比對全部失敗 → fail-open → 強更靜默永不觸發。
///
/// 失敗時返回空字串 `""`（後端收到空字串一樣走 fail-open）。
Future<String> fetchRawAppVersion() async {
  try {
    final info = await PackageInfo.fromPlatform();
    return info.version.trim();
  } catch (_) {
    return '';
  }
}
