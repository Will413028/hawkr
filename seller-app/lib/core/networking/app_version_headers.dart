import '../../shared/utils/app_version_utils.dart';
import '../codemeta/code_meta.dart';

/// 每個請求都帶的 header（含 WebSocket 握手）。
///
/// - 版本（`packages/hawkr_app_update/docs/spec.md`）：`X-App-Version`／`X-App-Type`（App 只發 iOS，不帶平台），供 access log
///   統計版本分佈。**純可觀測性用途**：伺服器不因缺這些 header 拒絕任何請求，也不拿它們當授權依據。
/// - 視角：舊 REST、SSE、WebSocket 握手與 chat ws-ticket 另外帶 [kActiveRoleHeader]（固定是 [kAppActiveRole]），舊
///   gateway 依它選擇以買家或賣家身分動作；只是視角、不是授權。Connect 不送它（ADR
///   2026-10-07-connect-without-perspective），所以 Connect 用 [appConnectHeaders]、舊 REST 用 [appVersionHeaders]。
///   隨最後一個讀 loginRole 的 gateway handler 或 IM 消費端退場刪除。
const String kAppVersionHeader = 'X-App-Version';
const String kAppTypeHeader = 'X-App-Type';

String _cachedRawAppVersion = '';

/// 目前 App 的 bare semver（`"1.0.3"`）；尚未初始化或讀取失敗時為空字串。
///
/// `PackageInfo.fromPlatform()` 是非同步的，而 Dio 的 `BaseOptions.headers`
/// 在建立 instance 當下就要定值，所以在 bootstrap 先 [initAppVersionInfo]
/// 快取一次，之後全部同步讀這裡。
String get currentRawAppVersion => _cachedRawAppVersion;

/// 這支 App 的身分：賣家版。
const AppType kCurrentAppType = AppType.seller;

/// 舊 REST 的視角 header（舊 gateway 依它選擇身分）；Connect 不送。
const String kActiveRoleHeader = 'X-Active-Role';

/// 這支 App 的視角（[kActiveRoleHeader] 的值）：賣家。
const String kAppActiveRole = 'SELLER';

/// bootstrap 呼叫一次，把 bare semver 快取起來供同步讀取。
///
/// 失敗不拋例外（維持空字串）——版本 header 缺失只影響可觀測性，
/// 不得讓 App 起不來。
Future<void> initAppVersionInfo() async {
  _cachedRawAppVersion = await fetchRawAppVersion();
}

/// Connect 每個請求都帶的 header map（版本與 App 類型）；版本還沒讀到時不送（不送空 header）。不含 [kActiveRoleHeader]。
Map<String, String> appConnectHeaders() {
  final headers = <String, String>{kAppTypeHeader: kCurrentAppType.value};
  if (_cachedRawAppVersion.isNotEmpty) {
    headers[kAppVersionHeader] = _cachedRawAppVersion;
  }
  return headers;
}

/// 舊 REST、SSE、WebSocket 握手每個請求都帶的 header map：[appConnectHeaders] 加上 [kActiveRoleHeader]。
Map<String, String> appVersionHeaders() => {
  ...appConnectHeaders(),
  kActiveRoleHeader: kAppActiveRole,
};
