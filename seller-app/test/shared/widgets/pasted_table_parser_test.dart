import 'package:hawkr_seller/shared/widgets/pasted_table_parser.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('detectPastedTable', () {
    test('標準 tab 矩陣（多列多欄）→ 正確二維', () {
      final r = detectPastedTable('品名\t產地\n蘋果\t台南\n檸檬\t屏東');
      expect(r, isNotNull);
      expect(r!.rows, [
        ['品名', '產地'],
        ['蘋果', '台南'],
        ['檸檬', '屏東'],
      ]);
      expect(r.truncated, isFalse);
    });

    test('單列多欄（a\\tb\\tc）→ 表格', () {
      final r = detectPastedTable('品名\t產地\t保存');
      expect(r, isNotNull);
      expect(r!.rows, [
        ['品名', '產地', '保存'],
      ]);
    });

    test('欄數不齊 → 補空正規化為最大欄數', () {
      final r = detectPastedTable('a\tb\tc\nx\ty');
      expect(r, isNotNull);
      expect(r!.rows, [
        ['a', 'b', 'c'],
        ['x', 'y', ''],
      ]);
    });

    test('純多行、無 tab → null（不誤判散文）', () {
      expect(detectPastedTable('第一行\n第二行\n第三行'), isNull);
    });

    test('單行無 tab → null', () {
      expect(detectPastedTable('這是一段商品說明文字'), isNull);
    });

    test('空字串 → null', () {
      expect(detectPastedTable(''), isNull);
    });

    test('尾端多餘換行被去除', () {
      final r = detectPastedTable('a\tb\nc\td\n\n\n');
      expect(r, isNotNull);
      expect(r!.rows, [
        ['a', 'b'],
        ['c', 'd'],
      ]);
    });

    test('CRLF 換行正規化', () {
      final r = detectPastedTable('a\tb\r\nc\td');
      expect(r, isNotNull);
      expect(r!.rows.length, 2);
    });

    test('超過列上限 → 截斷 + truncated=true', () {
      final line = '欄1\t欄2';
      final text = List.filled(60, line).join('\n');
      final r = detectPastedTable(text, maxRows: 50);
      expect(r, isNotNull);
      expect(r!.rows.length, 50);
      expect(r.truncated, isTrue);
    });

    test('超過欄上限 → 截斷至 maxCols', () {
      final cells = List.generate(15, (i) => 'c$i').join('\t');
      final r = detectPastedTable(cells, maxCols: 10);
      expect(r, isNotNull);
      expect(r!.rows.first.length, 10);
      expect(r.truncated, isTrue);
    });

    test('單格超過字數上限 → 截斷', () {
      final big = 'x' * 250;
      final r = detectPastedTable('$big\tb', maxCellLen: 200);
      expect(r, isNotNull);
      expect(r!.rows.first.first.length, 200);
      expect(r.truncated, isTrue);
    });

    test('保留儲存格內空白（不 trim cell 內容）', () {
      final r = detectPastedTable('  前後空白  \tb');
      expect(r, isNotNull);
      expect(r!.rows.first.first, '  前後空白  ');
    });

    // 混合內容分離（issue #1246）：網站複製常帶前導敘述行（無 tab），
    // 不可吞進表格變成錯誤的標題列。
    test('前導無 tab 敘述行 → 進 leadingText，不進表格', () {
      final r = detectPastedTable('經典原味\n成分\t發酵奶油\n產地\t臺灣');
      expect(r, isNotNull);
      expect(r!.leadingText, '經典原味');
      expect(r.rows.length, 2);
      expect(r.rows.first, ['成分', '發酵奶油']);
    });

    test('尾隨無 tab 行 → 進 trailingText', () {
      final r = detectPastedTable('成分\t發酵奶油\n產地\t臺灣\n以上為官方資料');
      expect(r, isNotNull);
      expect(r!.trailingText, '以上為官方資料');
      expect(r.rows.length, 2);
    });

    test('前後都無敘述行 → leading/trailing 為空', () {
      final r = detectPastedTable('a\tb\nc\td');
      expect(r, isNotNull);
      expect(r!.leadingText, '');
      expect(r.trailingText, '');
    });

    test('只有敘述行、無任何 tab 行 → 不算表格', () {
      expect(detectPastedTable('第一行\n第二行'), isNull);
    });
  });
}
