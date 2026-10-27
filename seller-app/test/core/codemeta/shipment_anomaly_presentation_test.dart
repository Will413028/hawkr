import 'package:hawkr_seller/core/codemeta/code_meta.dart';
import 'package:hawkr_seller/core/codemeta/shipment_anomaly_presentation.dart';
import 'package:flutter_test/flutter_test.dart';

/// 🔴 **賣家畫面不得出現 raw enum code。**
///
/// ## 這一支要防的具體事故
///
/// `features/shipment_anomaly/data/shipment_anomaly_model.dart` 曾有一份**手抄的**
/// `ShipmentAnomalyType`，停在 7 個值而 CodeMeta 有 11 個（後端 2026-09 新增 4 個
/// 值）。它的 `labelOf` 對認不得的碼值 `return value` ⇒ 後端一旦沒回
/// `anomalyTypeLabel`，賣家畫面就會出現英文大寫底線的 raw code。
///
/// 那份漂移**沒有任何 gate 擋得住**：CodeMeta 的檢查只看生成物（11 個值），
/// 不看手寫 class，所以 7 vs 11 在 gate 上一路是綠的。
///
/// ## 鑑別力
///
/// 第一支測試**窮舉 CodeMeta 的每一個成員**，所以「App 沒跟上 CodeMeta 新增碼值」
/// 會在這裡轉紅，而不是在賣家的螢幕上。
/// 第二支是那個 bug 本身的反面：認不得的碼值**不得**原樣回傳。
void main() {
  group('anomalyTypeLabelOf', () {
    test('CodeMeta 的每一個碼值都有中文 label，且不等於碼值本身', () {
      for (final type in ShipmentAnomalyType.values) {
        final label = anomalyTypeLabelOf(type.value);
        expect(label, isNotEmpty, reason: '${type.value} 沒有 label ⇒ 賣家看到空白');
        expect(
          label,
          isNot(type.value),
          reason: '${type.value} 的 label 等於碼值本身 ⇒ 賣家看到 raw enum code',
        );
        expect(
          label,
          isNot(unknownAnomalyTypeLabel),
          reason: '${type.value} 落到了 unknown 分支 ⇒ App 沒跟上 CodeMeta',
        );
      }
    });

    test('🔴 認不得的碼值回一句完整的話，不得回原字串', () {
      for (final unknown in <String>['WHATEVER_NEW_CODE', 'xyz', '']) {
        expect(anomalyTypeLabelOf(unknown), unknownAnomalyTypeLabel);
      }
      expect(anomalyTypeLabelOf(null), unknownAnomalyTypeLabel);
    });

    test('後端回了 label 就用後端的（它可能比 App 新）', () {
      expect(
        anomalyTypeLabelOf('WHATEVER_NEW_CODE', backendLabel: '某個新的異常'),
        '某個新的異常',
      );
      // 空白的 backendLabel 不算數——否則畫面會顯示一片空白。
      expect(
        anomalyTypeLabelOf('PACKAGE_LOST', backendLabel: '   '),
        isNot('   '),
      );
    });

    test('code 與 value 兩種寫法都查得到（後端在不同端點回不同形式）', () {
      final type = ShipmentAnomalyType.values.first;
      expect(anomalyTypeLabelOf(type.value), type.label);
      expect(anomalyTypeLabelOf(type.code.toLowerCase()), type.label);
    });
  });

  group('anomalySeverityLabelOf', () {
    test('CodeMeta 的每一個嚴重度都有中文 label，且不等於碼值本身', () {
      for (final severity in ShipmentAnomalySeverity.values) {
        final label = anomalySeverityLabelOf(severity.value);
        expect(label, isNotEmpty);
        expect(label, isNot(severity.value));
        expect(label, isNot(unknownAnomalySeverityLabel));
      }
    });

    test('🔴 認不得的嚴重度不得原樣顯示', () {
      expect(anomalySeverityLabelOf('CRITICAL'), unknownAnomalySeverityLabel);
      expect(anomalySeverityLabelOf(null), unknownAnomalySeverityLabel);
    });
  });
}
