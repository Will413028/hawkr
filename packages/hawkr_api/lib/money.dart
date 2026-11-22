/// 金額的格式化：TWD、0 位小數、千分位，兩語系都顯示 `NT$ `（ADR 多語系架構「金額」，每端一個 `formatMoney`）。
library;

/// 金額四捨五入到整數元（0.5 進位）。單價先經過這裡，總額與比較都以四捨五入後的單價計。
int roundMoney(num value) => value.round();

/// 四捨五入後單價乘數量。
int moneyTotal(num unitPrice, int quantity) => roundMoney(unitPrice) * quantity;

/// 折扣百分比，兩個價格先四捨五入再算；原價不高於售價、原價或售價四捨五入後不是正數回 null。
int? moneyDiscountPercent(num listPrice, num salePrice) {
  final list = roundMoney(listPrice);
  final sale = roundMoney(salePrice);
  if (list <= 0 || sale <= 0 || sale >= list) return null;
  return (((list - sale) / list) * 100).round();
}

/// 0 位小數、千分位的金額數字（無貨幣前綴）；用在前綴另起 TextSpan 的版面。
String formatMoneyAmount(num value) {
  final n = roundMoney(value);
  final digits = n.abs().toString();
  final buf = StringBuffer();
  for (var i = 0; i < digits.length; i++) {
    if (i > 0 && (digits.length - i) % 3 == 0) buf.write(',');
    buf.write(digits[i]);
  }
  return n < 0 ? '-$buf' : buf.toString();
}

/// `NT$ 1,234`。
String formatMoney(num value) => 'NT\$ ${formatMoneyAmount(value)}';
