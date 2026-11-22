/// 金額的格式化：TWD、0 位小數，兩語系都顯示 `NT$`（ADR 多語系架構「金額」）。
library;

/// 0 位小數的金額數字（無貨幣前綴），小數四捨五入（0.5 進位）；用在前綴另起 TextSpan 的版面。
String formatMoneyAmount(num value) => value.round().toString();

/// `NT$150`。
String formatMoney(num value) => 'NT\$${formatMoneyAmount(value)}';
