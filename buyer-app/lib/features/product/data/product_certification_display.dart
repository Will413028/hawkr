/// 商品認證標章的顯示解析。
///
/// `certifications[].name` 存的是 `CertificationType` 的**碼值**（`3701`…），不是中文名
/// —— 後端把它當不透明字串原樣轉發，碼值→顯示名的解析責任在前端。
///
/// ⚠️ **這裡不維護第二份中文對照表。** 手寫副本的失敗形狀是 drift 而不是錯誤：CodeMeta
/// 改了 label，副本照樣編譯、沒有測試轉紅，只是從那天起畫面顯示舊字串。標籤一律取自
/// 生成的 [CertificationType]（`lib/core/codemeta/code_meta.dart`）。
library;

import '../../../core/codemeta/code_meta.dart';

/// 碼值 → 顯示名。
///
/// 認不得的值**原樣回傳**（可能是更早期寫入的中文名資料），只有空字串才退成「認證」：
/// 顯示空白等於在畫面上留下一個看不見的法規宣稱。
String certDisplayTitle(String raw) {
  final String trimmed = raw.trim();
  final CertificationType? type = CertificationType.fromValue(trimmed);
  if (type != null) return type.label;
  return trimmed.isEmpty ? '認證' : trimmed;
}
