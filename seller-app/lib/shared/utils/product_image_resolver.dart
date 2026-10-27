import '../../config/media_image.dart';

/// 商品圖來源。契約（49 §6.1）SSOT 是 objectKey，**只有兩個欄位名**
/// （2026-08-05 實打 dev 逐支確認，非照型別宣告推寫）：
///
/// - **商品端點** —— 列表 `GET /app/products?view=manage`、詳情 `GET /app/products/{id}`、
///   公開詳情 `GET /public/products/{id}` —— 回 `imageKeys`（objectKey 陣列，`omitempty`）。
///   同一份回應裡的 `gallery` / `imageKey` / `imageUrls` / `imageUrl` **都是它的衍生物、
///   不是獨立來源**：product-service 逐項攤平 `gallery` 產生 `image_keys`、
///   `image_key = image_keys[0]`（`ProductProtoConverter.java` 的 `addAllImageKeys` 兩處），
///   Gateway 再由 `image_keys` 組出 `imageUrls` / `imageUrl`
///   （`gateway/internal/handler/product.go` 的 `buildMediaURLs(base, p.ImageKeys)`）。
///   讀 `imageKeys` 一個欄位 ＝ 讀那四個，順序與內容完全一致。
/// - **貼文的關聯商品**（`PostProductData`，`gateway/internal/handler/post.go`；
///   列表回單數物件 `product`、詳情回陣列 `products`）**只有單數 `imageKey`**，
///   沒有複數、也沒有 `gallery`。故單數是第二個、也是最後一個必要欄位。
///
/// 其餘曾列入候選的 20 個欄位名（`images` / `productImages` / `thumbnail` /
/// `thumbnailUrl` / `mainImage` / `coverUrl` / `pic` / `previewUrl` / `image` /
/// `objectKey` 與各自的 snake_case 版本）在上述任一端點都不存在，純屬猜欄位，
/// 2026-08-05 已移除。**新增候選前必須先實打端點確認。**
///
/// 另一個曾列入候選的 `_loadedImages` 是 App 自己塞的預載清單
/// （`page_shop_tab.dart` 寫入 → `product_setup_image_screen.dart` 讀取），
/// 那條路徑本來就直接讀該欄位、不經本檔，故一併移除。
List<String> _productImageKeys(Map<String, dynamic> product) {
  final keys = <String>[];
  final seen = <String>{};

  void add(dynamic raw) {
    // 值依契約恆為 objectKey；仍過 [mediaObjectKey] 還原裸 key，順帶剝掉誤入的
    // `i/{preset}/` 前綴，避免 #1659 的 double-preset 404。無法還原（外部域名）
    // 視同無圖。
    final key = mediaObjectKey((raw ?? '').toString());
    if (key.isEmpty) return;
    if (seen.add(key)) keys.add(key);
  }

  final list = product['imageKeys'];
  if (list is List) {
    for (final raw in list) {
      add(raw);
    }
  }
  // 貼文關聯商品只有單數欄位；商品端點的單數 == imageKeys[0]，會被 seen 去重。
  add(product['imageKey']);

  return keys;
}

/// 商品所有圖片的顯示 URL（依 [_productImageKeys] 的欄位判定），一律經
/// [buildImageUrl] 這個唯一組裝入口（49 §8.6）產生。
List<String> extractProductImageUrls(Map<String, dynamic> product) {
  return _productImageKeys(product)
      .map((key) => buildImageUrl(key, ImagePreset.original))
      .where((url) => url.isNotEmpty)
      .toList();
}

/// 商品主圖顯示 URL（＝ [extractProductImageUrls] 的第一張）。
String? extractPrimaryProductImageUrl(Map<String, dynamic> product) {
  final urls = extractProductImageUrls(product);
  return urls.isEmpty ? null : urls.first;
}

/// 商品主圖 **objectKey**（不是 URL）。用於把商品圖回填進其他資料結構時
/// ——49 §6.1 要求跨層傳遞只帶 objectKey，回填 URL 會讓下游再也分不清哪個是 SSOT。
String? extractPrimaryProductImageKey(Map<String, dynamic> product) {
  final keys = _productImageKeys(product);
  return keys.isEmpty ? null : keys.first;
}
