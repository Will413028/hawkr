/// 收件人姓名「顯示長度」驗證工具 — 對齊後端 PayUni 約束（issue #1187）
///
/// 顯示長度規則：
/// - 英文 / 半形字元（rune <= 0x7F）算 1
/// - 中文 / 全形字元算 2
///
/// 後端限制（顯示長度）：
/// - 超商取貨收件人姓名：[4, 10]
/// - 買家地址本收件人姓名：[4, 30]
///
/// 範例：
/// ```dart
/// RecipientNameValidator.displayLength('王小明');   // 6（3 個全形）
/// RecipientNameValidator.displayLength('Tom');      // 3（3 個半形）
/// RecipientNameValidator.validateStorePickup('王'); // '收件人姓名長度不足...'
/// ```
class RecipientNameValidator {
  RecipientNameValidator._();

  /// 超商取貨收件人姓名顯示長度下限
  static const int storePickupMin = 4;

  /// 超商取貨收件人姓名顯示長度上限
  static const int storePickupMax = 10;

  /// 地址本收件人姓名顯示長度下限
  static const int addressMin = 4;

  /// 地址本收件人姓名顯示長度上限
  static const int addressMax = 30;

  /// 計算字串的「顯示長度」：逐 rune 累加，半形（<= 0x7F）算 1、其餘算 2。
  static int displayLength(String s) {
    var len = 0;
    for (final r in s.runes) {
      len += r <= 0x7F ? 1 : 2;
    }
    return len;
  }

  /// 通用顯示長度驗證 — 合法回傳 `null`，不合法回傳繁體中文錯誤訊息。
  ///
  /// 空字串視為「未填寫」，回傳「請輸入收件人姓名」。
  static String? validate(String value, {required int min, required int max}) {
    final trimmed = value.trim();
    if (trimmed.isEmpty) {
      return '請輸入收件人姓名';
    }
    final len = displayLength(trimmed);
    if (len < min) {
      return '收件人姓名過短，需 $min-$max 字（中文 1 字算 2）';
    }
    if (len > max) {
      return '收件人姓名過長，需 $min-$max 字（中文 1 字算 2）';
    }
    return null;
  }

  /// 超商取貨收件人姓名驗證（顯示長度 [4, 10]）。
  static String? validateStorePickup(String value) =>
      validate(value, min: storePickupMin, max: storePickupMax);

  /// 地址本收件人姓名驗證（顯示長度 [4, 30]）。
  static String? validateAddress(String value) =>
      validate(value, min: addressMin, max: addressMax);
}
