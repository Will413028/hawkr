/// 售後案件佐證圖的解析。
///
/// 契約來源是 Gateway `EvidenceResponse`
/// （`gateway/internal/handler/aftersales.go:146-154`），實際 wire 形狀
/// 2026-08-05 對 dev 實打 `GET /app/seller/after_sales/{caseNo}` 逐鍵確認：
///
/// ```json
/// {"assetId":"866540497739859931",
///  "imageKey":"public/aftersale/2623900520/20260719/ce35c925-....png",
///  "imageUrl":"https://test-media.example.com/i/original/public/...",
///  "evidenceType":"8601",
///  "createdAt":"2026-07-19T04:42:34.24775Z"}
/// ```
///
/// 只有 `imageKey` / `imageUrl` 成對存在，且 `imageUrl` 是 Gateway 由 key 組出的
/// 衍生物（`ImageUrl: buildURL(mediaBase, e.ImageKey)`）。SSOT 是 key，
/// 顯示 URL 交回 `buildImageUrl` 唯一組裝入口（49 §6.1）——下游
/// `OptimizedNetworkImage` 內部本來就走 `ImageConfig.urlFor`，
/// 收 objectKey 與收 URL 組出的結果完全相同。
///
/// 這裡刻意不做多鍵 fallback：原本並讀 `imageUrl`/`url`/`images`/`imageUrls`/
/// `photoUrls` 五個候選、`evidenceType`/`evidence_type`/`type` 三個候選，
/// 其中只有各自第一個會命中，其餘八個任何端點都不回。多候選不會讓錯誤浮現，
/// 只會讓「後端改欄名」從一個立刻看得見的空畫面變成一個沒人知道何時壞的猜謎。
library;

/// `evidenceType` 合法值（CodeMeta `AfterSaleEvidenceType` 的 Value）。
///
/// 值是數字碼字串，不是 `BUYER` / `SELLER` 這類角色字面值——
/// 見 `docs/代碼管理（CodeMeta）/內部技術碼.md#aftersaleevidencetype`。
/// 該 enum 標記為「前端可見: Yes」但目前沒有 Flutter 端的 codegen 產物
/// （`lib/core/codemeta/code_meta.dart` 不含它），所以只能在此宣告字面值。
abstract final class AfterSaleEvidenceType {
  /// 8601 PHOTO — 買家申請／補件時上傳的證據照片。
  static const buyerPhoto = '8601';

  /// 8602 INSPECTION_PHOTO — 賣家確認收貨時上傳的驗貨照片。
  static const inspectionPhoto = '8602';
}

/// 從 `evidence` 陣列取出符合 [includeType] 的佐證圖 objectKey。
///
/// 回傳已去重、已去空白的 key 清單，順序維持後端給的順序。
List<String> afterSaleEvidenceImageKeys(
  List<Map<String, dynamic>> evidence, {
  required bool Function(String evidenceType) includeType,
}) {
  final keys = <String>[];
  for (final e in evidence) {
    final type = (e['evidenceType'] ?? '').toString().trim();
    if (!includeType(type)) continue;
    final key = (e['imageKey'] ?? '').toString().trim();
    if (key.isNotEmpty) keys.add(key);
  }
  return uniqueNonEmptyPhotoSources(keys);
}

/// 買家舉證照片（申請／補件）的 objectKey。
List<String> buyerIssueEvidenceImageKeys(List<Map<String, dynamic>> evidence) =>
    afterSaleEvidenceImageKeys(
      evidence,
      includeType: (type) => type == AfterSaleEvidenceType.buyerPhoto,
    );

/// 去空白 + 去重，順序維持首次出現的順序。
///
/// 收的是「圖片來源」而非嚴格的 URL：佐證圖是 objectKey，出貨照片
/// （`shipmentPhotoUrls`）是 Gateway 組好的顯示 URL，兩者混在同一份清單裡，
/// 因為 `OptimizedNetworkImage` 對兩者的處理完全相同。
List<String> uniqueNonEmptyPhotoSources(Iterable<String> sources) {
  final seen = <String>{};
  return sources
      .map((s) => s.trim())
      .where((s) => s.isNotEmpty && seen.add(s))
      .toList();
}
