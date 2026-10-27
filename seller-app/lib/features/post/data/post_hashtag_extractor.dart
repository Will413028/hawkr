class PostHashtagExtractor {
  PostHashtagExtractor._();

  static const int maxCount = 10;

  static final RegExp _pattern = RegExp(
    r'#([A-Za-z0-9\u3400-\u4DBF\u4E00-\u9FFF\uF900-\uFAFF]+)',
    unicode: true,
  );

  static List<String> extractFromText(String text) {
    if (text.isEmpty) return const [];
    final result = <String>[];
    final seen = <String>{};

    for (final match in _pattern.allMatches(text)) {
      final raw = match.group(1);
      if (raw == null || raw.isEmpty) continue;
      final canonical = raw.toLowerCase();
      if (!seen.add(canonical)) continue;
      result.add(raw);
      if (result.length >= maxCount) break;
    }
    return result;
  }

  static int countInText(String text) => extractFromText(text).length;

  static PostHashtagTaggingMode? detectTaggingMode(
    String textBeforeCaret, {
    int triggerAfterChars = 1,
  }) {
    if (textBeforeCaret.isEmpty) return null;

    for (int i = textBeforeCaret.length - 1; i >= 0; i--) {
      final ch = textBeforeCaret[i];
      if (ch == '#') {
        if (i > 0) {
          final prev = textBeforeCaret[i - 1];
          if (_isHashtagChar(prev) || prev == '#') return null;
        }
        final prefix = textBeforeCaret.substring(i + 1);
        if (prefix.length < triggerAfterChars) return null;
        return PostHashtagTaggingMode(hashAt: i, prefix: prefix);
      }
      if (!_isHashtagChar(ch)) return null;
    }
    return null;
  }

  static bool _isHashtagChar(String ch) {
    if (ch.length != 1) return false;
    final code = ch.codeUnitAt(0);
    if (code >= 0x30 && code <= 0x39) return true;
    if (code >= 0x41 && code <= 0x5A) return true;
    if (code >= 0x61 && code <= 0x7A) return true;
    if (code >= 0x3400 && code <= 0x4DBF) return true;
    if (code >= 0x4E00 && code <= 0x9FFF) return true;
    if (code >= 0xF900 && code <= 0xFAFF) return true;
    return false;
  }
}

class PostHashtagTaggingMode {
  const PostHashtagTaggingMode({required this.hashAt, required this.prefix});

  final int hashAt;
  final String prefix;
}
