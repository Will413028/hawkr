/// Hashtag 從 product description 抽取的 client-side fallback。
///
/// 後端 issue #759 計畫把 hashtag 從 description blocks 抽離成獨立 `hashtags`
/// 欄位回傳，但在那之前 (或舊資料未回填時)，前端先自己抽，免得 buyer 卡片
/// 看不到 hashtag。
///
/// 規則對齊賣家側 (`hawkr_seller/features/product/data/hashtag_extractor.dart`)：
/// - 合法字元 `\p{L}\p{N}_`（中英混合 OK，排除空白 / 標點 / emoji）
/// - dedup case-insensitive
/// - 上限 10
class BuyerHashtagExtractor {
  BuyerHashtagExtractor._();

  static const int maxCount = 10;

  static final RegExp _pattern = RegExp(r'#([\p{L}\p{N}_]+)', unicode: true);

  /// 從一段純文字抽 hashtag。
  static List<String> fromText(String text) {
    if (text.isEmpty) return const [];
    final result = <String>[];
    final seen = <String>{};
    for (final m in _pattern.allMatches(text)) {
      final raw = m.group(1);
      if (raw == null || raw.isEmpty) continue;
      final key = raw.toLowerCase();
      if (!seen.add(key)) continue;
      result.add(raw);
      if (result.length >= maxCount) break;
    }
    return result;
  }

  /// 從 description blocks 結構抽（`[{type: 'text', content: '...'}, ...]`）。
  static List<String> fromDescriptionBlocks(dynamic blocks) {
    if (blocks is! List || blocks.isEmpty) return const [];
    final buf = StringBuffer();
    for (final b in blocks) {
      if (b is! Map) continue;
      final type = b['type']?.toString() ?? 'text';
      if (type != 'text') continue;
      final content = b['content']?.toString() ?? b['text']?.toString() ?? '';
      if (content.isEmpty) continue;
      buf.write(content);
      buf.write('\n');
    }
    return fromText(buf.toString());
  }

  /// 從 product map 自動推 hashtags 出來。優先使用 backend 已回傳的 `hashtags`，
  /// 找不到就退而求其次從 `description.blocks` 抽。
  ///
  /// 容忍三種 hashtags 形態：
  /// - `["foodie", "紐西蘭"]`（字串陣列）
  /// - `[{name, productCount, ...}]`（HashtagItem 陣列）
  /// - `null` / 空陣列 → 從 description 抽
  static List<String> fromProduct(Map product) {
    final raw = product['hashtags'];
    if (raw is List && raw.isNotEmpty) {
      final list = raw
          .map((h) => h is Map ? h['name']?.toString() : h?.toString())
          .whereType<String>()
          .where((s) => s.isNotEmpty)
          .toList(growable: false);
      if (list.isNotEmpty) return list;
    }
    // fallback：從 description 抽
    final desc = product['description'];
    if (desc is Map) {
      final blocks = desc['blocks'];
      return fromDescriptionBlocks(blocks);
    }
    if (desc is String) {
      return fromText(desc);
    }
    return const [];
  }
}
