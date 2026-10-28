import '../../shared/utils/app_version_utils.dart';
import '../codemeta/code_meta.dart';

/// 每個請求都帶的 header：客戶端版本（`packages/hawkr_app_update/docs/spec.md`）。
///
/// 兩支 App 對**所有打 Gateway 的請求**（含 SSE / WebSocket 握手、Connect）帶
/// `X-App-Version` / `X-App-Type`，Gateway access log
/// 記錄後才答得出「線上各版本 App 的請求佔比」。
///
/// - 兩欄是**純可觀測性用途**，不是授權依據。
/// - 買家 App 不送 `X-Active-Role`：入口在提示不是 SELLER 時一律推導為買家，送 BUYER 與不送效果相同
///   （ADR 2026-10-07-connect-without-perspective）。
/// - 版本值是 bare semver（`fetchRawAppVersion`），**不含 build number**。
/// - [initialize] 必須在 `_bootstrapApplication` 開頭先跑完；`PackageInfo`
///   是 async，而各 client 建構點只能同步讀，故在此快取一份。未初始化時
///   [headers] 是空的（請求照送，只是少了可觀測欄位）。
class AppVersionHeaders {
  AppVersionHeaders._();

  static const String versionHeader = 'X-App-Version';
  static const String typeHeader = 'X-App-Type';

  /// CodeMeta [AppType]——本 repo 是買家 App。
  static String get appType => AppType.buyer.value;

  static String _version = '';

  /// bare semver（例 `1.0.3`）；未初始化或讀取失敗為空字串。
  static String get version => _version;

  static Map<String, String> _headers = const {};

  /// 供各 client（Connect、舊 REST、SSE、WebSocket 握手）展開（`...AppVersionHeaders.headers`）的固定欄位
  /// （App 只發 iOS，不帶平台）。
  static Map<String, String> get headers => _headers;

  static Future<void> initialize() async {
    _version = await fetchRawAppVersion();
    _headers = Map.unmodifiable(<String, String>{
      versionHeader: _version,
      typeHeader: appType,
    });
  }
}
