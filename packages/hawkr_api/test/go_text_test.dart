import 'package:hawkr_api/go_text.dart';
import 'package:test/test.dart';

void main() {
  group('trimGoSpace：同 Go strings.TrimSpace（unicode.IsSpace）', () {
    test('Go 認定是空白的 code point 都被修剪', () {
      const spaces = [
        '\u0009',
        '\u000A',
        '\u000B',
        '\u000C',
        '\u000D',
        '\u0020',
        '\u0085',
        '\u00A0',
        '\u1680',
        '\u2000',
        '\u2001',
        '\u2002',
        '\u2003',
        '\u2004',
        '\u2005',
        '\u2006',
        '\u2007',
        '\u2008',
        '\u2009',
        '\u200A',
        '\u2028',
        '\u2029',
        '\u202F',
        '\u205F',
        '\u3000',
      ];
      for (final space in spaces) {
        final reason = 'U+${space.codeUnitAt(0).toRadixString(16)}';
        expect(trimGoSpace('$space字$space'), '字', reason: reason);
        expect(goTrimmedRuneCount(space * 3), 0, reason: reason);
      }
    });

    test('和 Dart trim 不同的 code point：U+FEFF（BOM）Go 不修剪，所以算 1 個字', () {
      const bom = '\uFEFF';
      expect(bom.trim(), '');
      expect(trimGoSpace(bom), bom);
      expect(goTrimmedRuneCount(bom), 1);
      expect(goTrimmedRuneCount('$bom字$bom'), 3);
    });

    test('零寬空白 U+200B 與 U+180E 不是 Go 的空白，不修剪', () {
      expect(trimGoSpace('\u200B字'), '\u200B字');
      expect(trimGoSpace('\u180E字'), '\u180E字');
    });

    test('只修剪頭尾，中間的空白保留；全空白回空字串', () {
      expect(trimGoSpace('\u0020a\u0020b\u0020\u3000'), 'a\u0020b');
      expect(trimGoSpace('\u0020\u000A\u0009\u0020'), '');
      expect(trimGoSpace(''), '');
    });

    test('字數以 code point 計，不是 UTF-16 單位', () {
      expect(goTrimmedRuneCount('\u0020\u{1F600}\u0020'), 1);
      expect(goTrimmedRuneCount('a\u{1F600}b'), 3);
    });
  });
}
