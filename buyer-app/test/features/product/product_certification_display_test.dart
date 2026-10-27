import 'package:hawkr_buyer/core/codemeta/code_meta.dart';
import 'package:hawkr_buyer/features/product/data/product_certification_display.dart';
import 'package:flutter_test/flutter_test.dart';

/// 守的是 hawkr-website#507 在 App 側的同一個失效：**碼值漏了對照就靜默顯示成數字**。
///
/// 買家看到的「3701」不會拋例外、不會有任何既有測試轉紅 —— 認證是法規宣稱，顯示成一串
/// 數字等於這個宣稱對買家完全無效，而唯一會發現的人是買家自己。
void main() {
  group('certDisplayTitle', () {
    test('每個碼值都解析得出顯示名，不會退回碼值本身', () {
      for (final type in CertificationType.values) {
        final title = certDisplayTitle(type.value);
        expect(title, type.label);
        // 這一條才是重點：退回碼值 ＝ 買家看到數字。
        expect(title, isNot(type.value));
      }
    });

    test('前後空白不影響解析', () {
      expect(certDisplayTitle('  3701 '), '有機農產品');
    });

    test('認不得的值原樣顯示，不吞掉', () {
      // 更早期寫入的中文名資料（dev 實測存在一筆「產銷履歷」）仍要顯示得出來。
      expect(certDisplayTitle('產銷履歷'), '產銷履歷');
      expect(certDisplayTitle('9999'), '9999');
    });

    test('只有空值才退成「認證」', () {
      expect(certDisplayTitle(''), '認證');
      expect(certDisplayTitle('   '), '認證');
    });
  });
}
