import 'package:hawkr_api/images.dart' show ImagePreset;
import 'flavor.dart';

/// 圖片顯示 preset 白名單（49 號設計書 §4.4）：由 image gateway 的 `presets.ts` 生成（ADR
/// 2026-09-25-api-image-uri-template D4），前端只准傳這些名稱，Worker 對未知 preset 回 400。
export 'package:hawkr_api/images.dart' show ImagePreset;

/// 媒體域名基底：跟隨當前 flavor（與 gatewayHost 同 flavor 對應，49 §8.5 Image
/// Gateway）。取自 `MEDIA_BASE_HOST`（dev 預設本機單體）。runtime 讀取（F.init()
/// 後才有值；所有 buildImageUrl 呼叫皆在 UI 渲染期，必然已 init）。
String get _mediaBase => F.appFlavor.mediaBaseHost;

/// inline 圖片資料的唯一判準：`data:` URI scheme。
///
/// 判「這個字串是不是內嵌圖片」只能看 scheme，**不能用長度猜**。過去有四處寫成
/// 「不是 http 開頭且長度 > N 就當 base64」（門檻 60 / 100 / 120 各自為政），
/// 但 objectKey 本來就是 78~80 字元
/// （`public/{type}/{storeId}/{yyyymmdd}/{uuid}.{ext}`）——門檻 60 的那兩處已經被
/// 撞穿，objectKey 被誤判成 base64、`base64Decode` 拋例外後靜默退回預設圖；
/// storeId 位數或路徑層級一改，120 也會跟著中槍（hawkr_buyer#89）。
///
/// 非 inline 的值（objectKey 或完整 URL）一律交給 [buildImageUrl] 處理。
bool isInlineImageData(String value) => value.trim().startsWith('data:image/');

/// 媒體圖片顯示 URL 組裝的唯一入口（49 號設計書 §4.1 / §8.6）。
///
/// [keyOrUrl] 可為 objectKey（如 `public/products/x.jpg`）或後端回傳的完整顯示
/// URL。[preset] 取自 [ImagePreset]。組 `{mediaBaseHost}/i/{preset}/{objectKey}`；傳入
/// 完整 URL 時先還原 objectKey，無法還原（外部域名如 Google 頭像、或 Gateway
/// 動態端點 `/api/*`）→ 原樣 passthrough，不會產生無效的 `/i/` URL。
String buildImageUrl(String keyOrUrl, ImagePreset preset) {
  final v = keyOrUrl.trim();
  if (v.isEmpty) return '';

  final key = _toObjectKey(v);
  if (key.isEmpty) return v; // 外部 / 動態端點無法還原 key → passthrough。
  if (_mediaBase.isEmpty) return ''; // 沒有設定媒體域名 → 不組出無效 URL。
  return '$_mediaBase/i/${preset.name}/$key';
}

bool _isHttp(String v) => v.startsWith('http://') || v.startsWith('https://');

String _stripLeadingSlash(String v) => v.startsWith('/') ? v.substring(1) : v;

/// 從 objectKey 或完整 URL 還原 objectKey；無法還原回 ''（呼叫端 passthrough）。
String _toObjectKey(String v) {
  if (!_isHttp(v)) return _stripLeadingSlash(v);

  final uri = Uri.tryParse(v);
  if (uri == null) return '';
  final path = uri.path;

  // Gateway 檔案代理：/api/v1/files/{key}
  const filesMarker = '/api/v1/files/';
  final fi = path.indexOf(filesMarker);
  if (fi >= 0) return path.substring(fi + filesMarker.length);

  // 其他 Gateway 動態端點（/api/...，如會員頭像 view）→ 不還原（passthrough）。
  if (path.startsWith('/api/')) return '';

  // 已是 Worker preset URL：/i/{preset}/{key}
  if (path.startsWith('/i/')) {
    final rest = path.substring(3);
    final slash = rest.indexOf('/');
    return slash >= 0 ? rest.substring(slash + 1) : '';
  }

  // 設定的媒體域名（MEDIA_BASE_HOST）→ path 即 objectKey；其餘一律 passthrough。
  //
  // 媒體域名不做新舊相容（backend#2283）：只認當前 flavor 的媒體域名，
  // 已退役的儲存域名與任何其他子網域都不當成媒體來源，
  // 避免死網址被靜默還原成 key 再組成新網址、掩蓋掉真正的資料問題。
  final mediaAuthority = Uri.tryParse(_mediaBase)?.authority ?? '';
  if (mediaAuthority.isNotEmpty && uri.authority == mediaAuthority) {
    return _stripLeadingSlash(path);
  }
  return '';
}
