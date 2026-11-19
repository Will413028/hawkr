import 'package:flutter/widgets.dart' show StringCharacters;
import 'package:flutter/services.dart'
    show TextInputFormatter, TextEditingValue, TextSelection;

/// 輸入超過 [max] 個 rune（伺服器的計長單位）時截斷。
///
/// - 以字素叢集為單位丟棄：跨過上限的叢集（例如 emoji 的 ZWJ 序列）整個丟掉，不拆開。
/// - 輸入法組字中不截斷，組字結束後的那次更新才套用上限。
/// - 游標不移到尾端，只在超出截斷後的文字長度時夾回。
class RuneLimitFormatter extends TextInputFormatter {
  const RuneLimitFormatter(this.max);

  final int max;

  @override
  TextEditingValue formatEditUpdate(
    TextEditingValue oldValue,
    TextEditingValue newValue,
  ) {
    if (newValue.composing.isValid && !newValue.composing.isCollapsed) {
      return newValue;
    }
    if (newValue.text.runes.length <= max) return newValue;
    final kept = StringBuffer();
    var runes = 0;
    for (final cluster in newValue.text.characters) {
      final next = runes + cluster.runes.length;
      if (next > max) break;
      kept.write(cluster);
      runes = next;
    }
    final text = kept.toString();
    int clamp(int offset) =>
        offset < 0 ? offset : offset.clamp(0, text.length).toInt();
    final selection = newValue.selection;
    return TextEditingValue(
      text: text,
      selection: TextSelection(
        baseOffset: clamp(selection.baseOffset),
        extentOffset: clamp(selection.extentOffset),
        affinity: selection.affinity,
      ),
    );
  }
}
