/// 寄件人 / 收件人姓名「顯示長度」驗證工具
///
/// 對齊後端 PayUni 約束（backend-payuni#1187）：
/// 顯示長度規則為「英文 / 半形算 1，中文 / 全形算 2」。
/// 黑貓宅配寄件人姓名 ≤ 30、超商 ≤ 10、退貨收件人 ≤ 30（皆為顯示長度，min=1 非空）。
class TextLengthValidator {
  TextLengthValidator._();

  /// 計算字串的「顯示長度」：逐 rune 計算，
  /// code point <= 0x7F（ASCII / 半形）算 1，其餘（中文 / 全形等）算 2。
  static int displayLength(String s) {
    var length = 0;
    for (final r in s.runes) {
      length += r <= 0x7F ? 1 : 2;
    }
    return length;
  }

  /// 驗證姓名顯示長度是否合法。
  ///
  /// - 空字串（trim 後）回傳「請輸入$fieldLabel」
  /// - 顯示長度超過 [maxDisplayLength] 回傳「$fieldLabel長度過長…」
  /// - 合法回傳 null
  static String? validateName(
    String? value, {
    required int maxDisplayLength,
    required String fieldLabel,
  }) {
    final trimmed = (value ?? '').trim();
    if (trimmed.isEmpty) {
      return '請輸入$fieldLabel';
    }
    if (displayLength(trimmed) > maxDisplayLength) {
      return '$fieldLabel長度過長，最多 $maxDisplayLength 個字（中文算 2、英文算 1）';
    }
    return null;
  }
}
