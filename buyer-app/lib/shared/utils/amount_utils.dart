/// 金額處理工具 — 統一字串 / 數字 / null 的轉換邏輯
///
/// 後端規格（B-109 / issue #582）：
/// - API 一律回傳字串（避免 int / String 分歧）
/// - PayUNi 要求整數 NTD（不可有小數）
/// - 前端應用 [parseToInt] 取金額做運算與送出
///
/// 範例：
/// ```dart
/// final total = AmountUtils.parseToInt(order.totalActualAmount); // 999
/// final display = formatMoney(AmountUtils.parseToInt(order.totalActualAmount)); // "NT$ 999"
/// final isEqual = AmountUtils.equals(a, b); // 字串/數字皆可比
/// ```
class AmountUtils {
  AmountUtils._();

  /// 解析金額為整數（NTD 標準）— 接受 String / int / double / null
  /// 小數會四捨五入。空字串 / 無法 parse → 0
  static int parseToInt(dynamic value) {
    if (value == null) return 0;
    if (value is int) return value;
    if (value is double) return value.round();
    if (value is num) return value.round();
    final s = value.toString().trim();
    if (s.isEmpty) return 0;
    final n = num.tryParse(s);
    return n?.round() ?? 0;
  }

  /// 解析金額為 num（含小數）— 用於需保留精度的中間運算
  /// 最後送出 / 顯示前應改用 [parseToInt]
  static num parseToNum(dynamic value) {
    if (value == null) return 0;
    if (value is num) return value;
    return num.tryParse(value.toString().trim()) ?? 0;
  }

  /// 比較兩金額是否相等（任意 String / num）
  static bool equals(dynamic a, dynamic b) => parseToInt(a) == parseToInt(b);

  /// a > b
  static bool greaterThan(dynamic a, dynamic b) =>
      parseToInt(a) > parseToInt(b);

  /// 加總多個金額（容錯 String / num / null）
  static int sum(List<dynamic> values) =>
      values.fold(0, (acc, v) => acc + parseToInt(v));

  /// 是否為 0 元（含 "0" / "0.0" / 0 / 0.0 / null / ""）
  static bool isZero(dynamic value) => parseToInt(value) == 0;
}
