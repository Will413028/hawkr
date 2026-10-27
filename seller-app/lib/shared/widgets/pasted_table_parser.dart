/// 貼上內容的表格偵測（方案 A：tab 分隔純文字）。
///
/// Excel / Google Sheet 複製時一定帶「tab 分欄、換行分列」的純文字，
/// 本模組把這種純文字偵測並正規化成二維字串矩陣，供編輯器插入為表格 block。
///
/// 設計取捨：
/// - **只認 tab**（不認逗號），避免把一般散文誤判成表格。
/// - 純多行、無 tab 的文字視為一般段落（回傳 null），不觸發表格。
/// - 上限對齊商品描述表格規範（欄 10 / 列 50 / 單格 200 字），超過即截斷。
library;

/// 偵測結果：含正規化矩陣與是否曾因上限而截斷。
class DetectedTable {
  const DetectedTable({
    required this.rows,
    required this.truncated,
    this.leadingText = '',
    this.trailingText = '',
  });

  /// 正規化後的二維字串矩陣（每列欄數已補齊為一致）。
  final List<List<String>> rows;

  /// 是否因超過列 / 欄 / 單格上限而截斷。
  final bool truncated;

  /// 表格區之前的無 tab 敘述行（保留為段落，不吞進表格——issue #1246
  /// 混合內容剪貼簿：如「經典原味↵成分⇥發酵奶油…」的首行標題）。
  final String leadingText;

  /// 表格區之後的無 tab 行（保留為段落）。
  final String trailingText;
}

/// 偵測貼上的純文字是否為 tab 分隔表格。
///
/// 回傳 [DetectedTable]（矩陣已正規化、上限已套用）；若不像表格則回傳 `null`。
///
/// 判定為表格的條件（全部成立）：
/// - 內含至少一個 tab（`\t`）
/// - 正規化後欄數 >= 2
DetectedTable? detectPastedTable(
  String text, {
  int maxRows = 50,
  int maxCols = 10,
  int maxCellLen = 200,
}) {
  if (text.isEmpty || !text.contains('\t')) return null;

  // 統一換行、去掉結尾多餘空行（Excel / Sheets 常在尾端帶換行）。
  final normalized = text.replaceAll('\r\n', '\n').replaceAll('\r', '\n');
  final rawLines = normalized.split('\n');
  while (rawLines.isNotEmpty && rawLines.last.trim().isEmpty) {
    rawLines.removeLast();
  }
  if (rawLines.isEmpty) return null;

  // 混合內容分離：開頭/結尾連續「無 tab 行」是敘述文字，不屬於表格——
  // 吞進表格會變成錯誤的標題列/資料列。表格區 = 第一個含 tab 的行起、
  // 最後一個含 tab 的行止；區間內夾雜的無 tab 行維持原樣（視為單欄列）。
  var start = 0;
  while (start < rawLines.length && !rawLines[start].contains('\t')) {
    start++;
  }
  var end = rawLines.length - 1;
  while (end >= 0 && !rawLines[end].contains('\t')) {
    end--;
  }
  if (start > end) return null;
  final leadingText = rawLines
      .sublist(0, start)
      .where((l) => l.trim().isNotEmpty)
      .join('\n');
  final trailingText = rawLines
      .sublist(end + 1)
      .where((l) => l.trim().isNotEmpty)
      .join('\n');

  var truncated = false;

  // 列上限。
  List<String> lines = rawLines.sublist(start, end + 1);
  if (lines.length > maxRows) {
    lines = lines.sublist(0, maxRows);
    truncated = true;
  }

  // 每列以 tab 分欄，記錄最大欄數。
  final parsed = <List<String>>[];
  var cols = 0;
  for (final line in lines) {
    final cells = line.split('\t');
    if (cells.length > cols) cols = cells.length;
    parsed.add(cells);
  }

  // 欄上限。
  if (cols > maxCols) {
    cols = maxCols;
    truncated = true;
  }

  // 需要真的有「欄」（>=2）才算表格；否則只是多行純文字。
  if (cols < 2) return null;

  // 正規化：補齊欄數 + 單格長度上限。
  final rows = <List<String>>[];
  for (final cells in parsed) {
    final row = <String>[];
    for (var c = 0; c < cols; c++) {
      var v = c < cells.length ? cells[c] : '';
      if (v.length > maxCellLen) {
        v = v.substring(0, maxCellLen);
        truncated = true;
      }
      row.add(v);
    }
    rows.add(row);
  }

  return DetectedTable(
    rows: rows,
    truncated: truncated,
    leadingText: leadingText,
    trailingText: trailingText,
  );
}
