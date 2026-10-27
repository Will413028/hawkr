import 'package:flutter_quill/quill_delta.dart';

/// runs（02 §3.6 inline-run 模型）⇄ Quill Delta 轉換 adapter。
///
/// 後端 DB 一律存 `runs`（`[{text,bold,italic,underline,color,href}]`）；
/// 賣家 app 編輯器內部用 flutter_quill 的 Delta。此 adapter 是唯一橋接點，
/// DB 格式不因編輯器換成 Quill 而改變（對齊 web 的 descriptionEditorJs.ts）。
class RunsDeltaAdapter {
  const RunsDeltaAdapter._();

  /// runs → Quill Delta（Document 用）。Quill 文件必以換行收尾。
  static Delta runsToDelta(List<dynamic>? runs) {
    final delta = Delta();
    if (runs != null) {
      for (final r in runs) {
        if (r is! Map) continue;
        final text = r['text']?.toString() ?? '';
        if (text.isEmpty) continue;
        final attrs = _attrsFromRun(r);
        delta.insert(text, attrs.isEmpty ? null : attrs);
      }
    }
    // 確保結尾換行（Quill Document 要求）
    delta.insert('\n');
    return delta;
  }

  /// Quill Delta → runs（存檔用）。丟棄純換行 / 空 run，合併相鄰同樣式（canonical）。
  static List<Map<String, dynamic>> deltaToRuns(Delta delta) {
    final raw = <Map<String, dynamic>>[];
    for (final op in delta.toList()) {
      final data = op.data;
      if (data is! String) continue; // 略過 embed（圖片等，本編輯器不會有）
      // 去掉文件結尾強制換行；段內換行轉為空白（單段落 block）
      final text = data.replaceAll('\n', ' ');
      if (text.isEmpty) continue;
      final run = <String, dynamic>{'text': text};
      final attrs = op.attributes;
      if (attrs != null) {
        if (attrs['bold'] == true) run['bold'] = true;
        if (attrs['italic'] == true) run['italic'] = true;
        if (attrs['underline'] == true) run['underline'] = true;
        final color = attrs['color'];
        if (color is String && _isHex6(color)) run['color'] = color;
        final link = attrs['link'];
        if (link is String && _isHttp(link)) run['href'] = link;
      }
      raw.add(run);
    }
    return _normalize(raw);
  }

  static Map<String, dynamic> _attrsFromRun(Map<dynamic, dynamic> r) {
    final attrs = <String, dynamic>{};
    if (r['bold'] == true) attrs['bold'] = true;
    if (r['italic'] == true) attrs['italic'] = true;
    if (r['underline'] == true) attrs['underline'] = true;
    final color = r['color'];
    if (color is String && _isHex6(color)) attrs['color'] = color;
    final href = r['href'];
    if (href is String && _isHttp(href)) attrs['link'] = href;
    return attrs;
  }

  /// 相鄰且樣式旗標完全相同的 run 合併、丟空 —— 與 web 同一正規形（保 round-trip idempotent）。
  static List<Map<String, dynamic>> _normalize(
    List<Map<String, dynamic>> runs,
  ) {
    final out = <Map<String, dynamic>>[];
    for (final r in runs) {
      final text = r['text']?.toString() ?? '';
      if (text.isEmpty) continue;
      if (out.isNotEmpty && _sameStyle(out.last, r)) {
        out.last['text'] = (out.last['text'] as String) + text;
      } else {
        out.add(Map<String, dynamic>.from(r));
      }
    }
    return out;
  }

  static bool _sameStyle(Map<String, dynamic> a, Map<String, dynamic> b) {
    return a['bold'] == b['bold'] &&
        a['italic'] == b['italic'] &&
        a['underline'] == b['underline'] &&
        a['color'] == b['color'] &&
        a['href'] == b['href'];
  }

  static bool _isHex6(String s) => RegExp(r'^#[0-9a-fA-F]{6}$').hasMatch(s);

  static bool _isHttp(String s) =>
      s.startsWith('http://') || s.startsWith('https://');
}
