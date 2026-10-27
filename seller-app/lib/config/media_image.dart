import 'package:hawkr_api/images.dart' show ImagePreset;
import 'flavor.dart';

/// 圖片顯示 preset 白名單（49 號設計書 §4.4）：由 image gateway 的 `presets.ts` 生成（ADR
/// 2026-09-25-api-image-uri-template D4），前端只准傳這些名稱，Worker 對未知 preset 回 400。
export 'package:hawkr_api/images.dart' show ImagePreset;

/// Whether [keyOrUrl] names an animated GIF asset.
///
/// Post APIs expose the object key/URL rather than a separate MIME field, and
/// uploaded media keeps its file extension. Inspecting the URI path also makes
/// this work for Worker URLs with query strings and mixed-case extensions.
bool isAnimatedGifMedia(String keyOrUrl) {
  final value = keyOrUrl.trim();
  if (value.isEmpty) return false;
  final uri = Uri.tryParse(value);
  final path = uri == null || uri.path.isEmpty ? value : uri.path;
  return path.toLowerCase().endsWith('.gif');
}

/// Select the display preset for post media without flattening GIF animation.
///
/// Static post images keep the inexpensive [staticPreset]. GIFs use the
/// Worker's bounded `motion` derivative, matching the web post-card contract.
ImagePreset postImagePresetForMedia(
  String keyOrUrl, {
  ImagePreset staticPreset = ImagePreset.card,
}) => isAnimatedGifMedia(keyOrUrl) ? ImagePreset.motion : staticPreset;

/// Build a post-image URL using [postImagePresetForMedia].
String buildPostImageUrl(
  String keyOrUrl, {
  ImagePreset staticPreset = ImagePreset.card,
}) => buildImageUrl(
  keyOrUrl,
  postImagePresetForMedia(keyOrUrl, staticPreset: staticPreset),
);

/// 媒體域名基底：跟隨當前 flavor（與 gatewayHost 同 flavor 對應，49 §8.5 Image
/// Gateway）。取自 `MEDIA_BASE_HOST`（dev 預設本機單體）。runtime 讀取（F.init()
/// 後才有值；所有 buildImageUrl 呼叫皆在 UI 渲染期，必然已 init）。
String get _mediaBase => F.appFlavor.mediaBaseHost;

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

/// 從 objectKey 或完整 URL 還原「乾淨的」 objectKey，供需要「持久化 key」的呼叫端
/// （如頭像快取回填）使用。相較 [buildImageUrl]（回組顯示 URL），本函式回傳裸 key，
/// 且額外剝除誤入 key 的 Worker preset 前綴 `i/{preset}/`——修正曾把
/// `https://.../i/original/public/x.png` 的 path 直接當 key 存下、下次再組出
/// double-preset `/i/avatar/i/original/...`（404）的污染（issue #1659）。
/// 無法還原（外部域名 / Gateway 動態端點）回 ''。
String mediaObjectKey(String keyOrUrl) {
  final v = keyOrUrl.trim();
  if (v.isEmpty) return '';
  return _stripPresetPrefix(_toObjectKey(v));
}

/// 已知 preset 名稱（用於偵測並剝除誤入 objectKey 的 `i/{preset}/` 前綴），取自生成的 [ImagePreset]。
final Set<String> _knownPresets = {for (final p in ImagePreset.values) p.name};

/// 反覆剝除開頭的 `i/{knownPreset}/` 前綴（處理多重污染的 double preset）。
/// 只在第二段為已知 preset 時剝除，避免誤傷真實以 `i/` 起頭的 key（實務不存在，
/// 但保守）。
String _stripPresetPrefix(String key) {
  var k = _stripLeadingSlash(key);
  while (k.startsWith('i/')) {
    final rest = k.substring(2);
    final slash = rest.indexOf('/');
    if (slash < 0) break;
    if (!_knownPresets.contains(rest.substring(0, slash))) break;
    k = rest.substring(slash + 1);
  }
  return k;
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
