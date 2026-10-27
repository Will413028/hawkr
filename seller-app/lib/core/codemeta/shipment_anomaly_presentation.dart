/// 配送異常碼值 → 賣家看得懂的中文，**唯一**的判定點。
///
/// ## 為什麼需要它
///
/// 這個檔案取代的是 `features/shipment_anomaly/data/shipment_anomaly_model.dart`
/// 裡一份**手抄的** `ShipmentAnomalyType`：它停在 7 個值，而 CodeMeta 已經有 11 個
/// （後端 2026-09 新增 4 個值）。手抄版的 `labelOf` 對認不得的碼值
/// `return value` —— 也就是把英文大寫底線的 raw code 原樣印在賣家畫面上。
///
/// 🔴 那個漂移**沒有任何 gate 擋得住**：CodeMeta 的檢查只看生成物（11 個值），
/// 不看手寫 class，所以 7 vs 11 在 gate 上是綠的。
/// 唯一的症狀是賣家看到一串英文大寫底線字 —— 而那只在後端沒回 `anomalyTypeLabel`
/// 時才發生，平常測不出來。
///
/// ## 使用規約
///
/// 1. 畫面層一律呼叫本檔，**禁止**自己寫 `switch (anomalyType)`。
/// 2. 後端有回 `anomalyTypeLabel` 就用後端的（它是同一份 CodeMeta 的 label）；
///    空字串才落到這裡。兩者同源，所以不會出現兩套說法。
/// 3. 認不得的碼值回 [unknownAnomalyTypeLabel]，**不得回原字串** ——
///    賣家看到「配送異常」還能求助，看到一串 raw code 只會困惑。
///
/// ## 這個機制擋得住什麼（誠實標明邊界）
///
/// 它擋得住「CodeMeta 新增碼值而 App 沒跟上」——新值自動有 label，不必改本檔。
/// 它**擋不住**「畫面層自己另外寫一次 switch」，那一種只能靠
/// `test/core/codemeta/shipment_anomaly_presentation_test.dart` 的窮舉與 review。
library;

import 'code_meta.dart';

/// 認不得的碼值要顯示的字。
///
/// 刻意是一句**完整的話**而不是「未知」兩個字：賣家看到它時仍然知道
/// 「這是一筆配送異常、我該點進去看」，而不是以為畫面壞了。
const String unknownAnomalyTypeLabel = '配送異常（請點入查看詳情）';

/// 認不得的嚴重度要顯示的字。
const String unknownAnomalySeverityLabel = '—';

/// 由後端回的 `anomalyType` 原始值取中文 label。
///
/// [backendLabel] 是後端隨資料回來的 `anomalyTypeLabel`；非空就直接用它
/// （同一份 CodeMeta 的 label，且後端可能比 App 新）。
String anomalyTypeLabelOf(String? rawAnomalyType, {String? backendLabel}) {
  final fromBackend = backendLabel?.trim() ?? '';
  if (fromBackend.isNotEmpty) return fromBackend;

  final raw = rawAnomalyType?.trim() ?? '';
  if (raw.isEmpty) return unknownAnomalyTypeLabel;
  final type =
      ShipmentAnomalyType.fromValue(raw) ??
      ShipmentAnomalyType.fromCode(raw.toUpperCase());
  return type?.label ?? unknownAnomalyTypeLabel;
}

/// 由後端回的 `severity` 原始值取中文 label。
String anomalySeverityLabelOf(String? rawSeverity) {
  final raw = rawSeverity?.trim() ?? '';
  if (raw.isEmpty) return unknownAnomalySeverityLabel;
  final severity =
      ShipmentAnomalySeverity.fromValue(raw) ??
      ShipmentAnomalySeverity.fromCode(raw.toUpperCase());
  return severity?.label ?? unknownAnomalySeverityLabel;
}
