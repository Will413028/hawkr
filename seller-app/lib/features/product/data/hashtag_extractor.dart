/// Hashtag 抽取 / normalize 工具。
///
/// 從 rich text editor 的純文字 / structured blocks 內把 `#xxx` 抽出來，
/// 上架前傳給後端的 `hashtags: string[]` 欄位。
///
/// 規則對齊 issue #759 + OpenAPI:
/// - 合法字元：Unicode letter + digit + underscore（中英混合 OK，排除空白、標點、emoji）
/// - case-insensitive dedup（中文不變、英文 lowercase 後比對）
/// - 同一段文字內重複 hashtag 只算一次
/// - 全文上限 [maxCount]（預設 10）
class HashtagExtractor {
  HashtagExtractor._();

  /// 上限（與 UI 計數器同步）
  static const int maxCount = 10;

  /// hashtag 命中規則：`#` 後跟 1+ 個 Unicode letter / digit / underscore。
  /// 用 unicode 修飾才能吃中文；連續 `##` 因為第二個 `#` 不是 letter/digit 不會擴展。
  static final RegExp _pattern = RegExp(r'#([\p{L}\p{N}_]+)', unicode: true);

  /// 從純文字抽 hashtag。回傳 normalize 後、去重、依序排好、上限 [maxCount] 的 list。
  static List<String> extractFromText(String text) {
    if (text.isEmpty) return const [];
    final result = <String>[];
    final seen = <String>{}; // 已收進的 canonical key（小寫）

    for (final match in _pattern.allMatches(text)) {
      final raw = match.group(1);
      if (raw == null || raw.isEmpty) continue;
      final canonical = _canonical(raw);
      if (canonical.isEmpty) continue;
      if (!seen.add(canonical)) continue; // dedup
      result.add(raw); // 保留原樣（含中文 / 大小寫），server 端會 normalize
      if (result.length >= maxCount) break;
    }
    return result;
  }

  /// 取 text block 的純文字：inline-run 模型（`runs: [{text,...}]`）優先，
  /// legacy `content` / `text` 欄位相容。編輯器新格式只輸出 runs——只認
  /// legacy 欄位會恆為空，導致存檔時 hashtag 被全量取代洗掉（issue #1246）。
  static String blockPlainText(Map<String, dynamic> b) {
    final runs = b['runs'];
    if (runs is List && runs.isNotEmpty) {
      final buf = StringBuffer();
      for (final r in runs) {
        if (r is Map) buf.write(r['text']?.toString() ?? '');
      }
      final s = buf.toString();
      if (s.isNotEmpty) return s;
    }
    return b['content']?.toString() ?? b['text']?.toString() ?? '';
  }

  /// 從 rich text editor 的 blocks 抽 hashtag。
  /// 走訪所有 `type: 'text'` block 把文字（runs 優先、legacy content 相容）
  /// 串起來再抽。
  static List<String> extractFromBlocks(List<Map<String, dynamic>>? blocks) {
    if (blocks == null || blocks.isEmpty) return const [];
    final buf = StringBuffer();
    for (final b in blocks) {
      final type = b['type']?.toString() ?? 'text';
      if (type != 'text') continue;
      final content = blockPlainText(b);
      if (content.isEmpty) continue;
      buf.write(content);
      buf.write('\n'); // 區塊間插斷字元，避免「#hashtag」誤跟下一個 block 黏起來
    }
    return extractFromText(buf.toString());
  }

  /// 計算目前 [text] 已使用的 hashtag 數量（dedup 後）。給 UI 計數器用。
  static int countInText(String text) => extractFromText(text).length;

  /// 從 product map 推 hashtags。優先 `hashtags` 欄位，找不到從 description 抽。
  /// 容忍 `["foodie"]` / `[{name: ...}]` / null。
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
    final desc = product['description'];
    if (desc is Map) {
      final blocks = desc['blocks'];
      if (blocks is List) {
        return extractFromBlocks(
          blocks.whereType<Map<String, dynamic>>().toList(),
        );
      }
    }
    if (desc is String) return extractFromText(desc);
    return const [];
  }

  /// 給定一段 caret 之前的字串，判斷使用者「正在輸入 hashtag」中。
  /// 條件：caret 前面找得到最近一個 `#`，且 `#` 跟 caret 之間沒有空白 / 標點
  /// （只能是 letter / digit / underscore）。
  ///
  /// 回傳 [HashtagTaggingMode] 含 `prefix` 字串（不含 `#`），UI 用來 query autocomplete。
  /// 不在 tagging 狀態回 null。
  static HashtagTaggingMode? detectTaggingMode(
    String textBeforeCaret, {
    int triggerAfterChars = 1,
  }) {
    if (textBeforeCaret.isEmpty) return null;
    // 從右往左找 `#`，途中遇到不合法字元就退出
    final n = textBeforeCaret.length;
    for (int i = n - 1; i >= 0; i--) {
      final c = textBeforeCaret.codeUnitAt(i);
      final ch = textBeforeCaret[i];
      if (ch == '#') {
        // 找到 `#`。檢查它前面是不是 word boundary（開頭 / 空白 / 非 hashtag char）。
        if (i > 0) {
          final prev = textBeforeCaret[i - 1];
          if (_isHashtagChar(prev) || prev == '#') return null;
        }
        final prefix = textBeforeCaret.substring(i + 1);
        if (prefix.length < triggerAfterChars) return null; // 至少要 N 個字才觸發
        return HashtagTaggingMode(hashAt: i, prefix: prefix);
      }
      // 不是 `#` 也不是 hashtag char → 中途有空白/標點，不在 tagging mode
      if (!_isHashtagChar(ch)) return null;
      // 避免 unused warning
      // ignore: unused_local_variable
      final _ = c;
    }
    return null;
  }

  static String _canonical(String s) {
    // 英文小寫，其他語言（中文等）原樣保留作為 key
    return s.toLowerCase();
  }

  static bool _isHashtagChar(String ch) {
    if (ch.length != 1) return false;
    // ASCII letter / digit / underscore 走快路徑
    final code = ch.codeUnitAt(0);
    if (code == 0x5F) return true; // _
    if (code >= 0x30 && code <= 0x39) return true; // 0-9
    if (code >= 0x41 && code <= 0x5A) return true; // A-Z
    if (code >= 0x61 && code <= 0x7A) return true; // a-z
    // 其他用 regex 判斷一次（吃 unicode letter / digit）
    return RegExp(r'[\p{L}\p{N}_]', unicode: true).hasMatch(ch);
  }
}

/// 描述「使用者正在輸入 hashtag」的瞬時狀態。
class HashtagTaggingMode {
  const HashtagTaggingMode({required this.hashAt, required this.prefix});

  /// `#` 字元在原字串中的 index
  final int hashAt;

  /// `#` 後到 caret 之間的字串（不含 `#`），給 autocomplete query 用
  final String prefix;

  @override
  String toString() => 'HashtagTaggingMode(#@$hashAt, prefix="$prefix")';
}
