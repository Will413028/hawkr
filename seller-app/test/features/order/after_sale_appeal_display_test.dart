import 'package:hawkr_seller/core/codemeta/code_meta.dart';
import 'package:hawkr_seller/features/order/data/after_sale_model.dart';
import 'package:flutter_test/flutter_test.dart';

/// 售後申訴狀態折疊顯示（#1233）：賣家訂單卡售後區需把 appealState 折疊成
/// 「申訴仲裁中」，否則案件進入平台仲裁後仍顯示已拒絕 / 待處理。
void main() {
  group('AfterSaleStatus appeal-aware display (#1233)', () {
    test('isUnderAppeal 只在 APPEALED(10)/INVESTIGATING(20) 為真，NONE/空/已裁決為否', () {
      expect(AfterSaleStatus.isUnderAppeal('10'), isTrue);
      expect(AfterSaleStatus.isUnderAppeal('20'), isTrue);
      // DB NOT NULL DEFAULT '0' → 每筆都帶 '0'，不可誤判
      expect(AfterSaleStatus.isUnderAppeal('0'), isFalse);
      expect(AfterSaleStatus.isUnderAppeal(''), isFalse);
      expect(AfterSaleStatus.isUnderAppeal('30'), isFalse);
    });

    test('isUnderAppeal 只吃 Value，CodeMeta Name 一律不算申訴', () {
      // wire 上只會出現 AfterSaleAppealState 的 Value：gateway 收到 Name 直接
      // 400、aftersales-service 落庫也是 Value。舊的雙軌比對
      //（== '10' || == 'APPEALED'）已物理刪除，Name 必須比不中。
      expect(AfterSaleStatus.isUnderAppeal('APPEALED'), isFalse);
      expect(AfterSaleStatus.isUnderAppeal('INVESTIGATING'), isFalse);
      expect(AfterSaleStatus.isUnderAppeal('RESOLVED'), isFalse);
      expect(AfterSaleStatus.isUnderAppeal('NONE'), isFalse);
      // Value 對照表即 CodeMeta 鏡像，兩者必須一致
      expect(
        AfterSaleStatus.isUnderAppeal(AfterSaleAppealState.appealed.value),
        isTrue,
      );
      expect(
        AfterSaleStatus.isUnderAppeal(AfterSaleAppealState.investigating.value),
        isTrue,
      );
      expect(
        AfterSaleStatus.isUnderAppeal(AfterSaleAppealState.resolved.value),
        isFalse,
      );
      expect(
        AfterSaleStatus.isUnderAppeal(AfterSaleAppealState.none.value),
        isFalse,
      );
    });

    test('displayLabel 仲裁中覆寫成 申訴仲裁中（不經 codemeta）', () {
      // 買家申訴(60) 與自動升級(0) 都折疊成 申訴仲裁中
      expect(AfterSaleStatus.displayLabel('60', '10'), '申訴仲裁中');
      expect(AfterSaleStatus.displayLabel('0', '20'), '申訴仲裁中');
      // Name 不再被折疊，維持 raw status
      expect(
        AfterSaleStatus.displayLabel('60', 'APPEALED'),
        AfterSaleStatus.label('60'),
      );
    });

    test('未申訴 / 已裁決 不折疊（委派 raw status label，非申訴文字）', () {
      // 委派 label()，至少保證不會誤蓋成申訴文字
      expect(AfterSaleStatus.displayLabel('60', '0'), isNot('申訴仲裁中'));
      expect(AfterSaleStatus.displayLabel('60', '30'), isNot('申訴仲裁中'));
      expect(
        AfterSaleStatus.displayLabel('60', '0'),
        AfterSaleStatus.label('60'),
      );
    });
  });
}
