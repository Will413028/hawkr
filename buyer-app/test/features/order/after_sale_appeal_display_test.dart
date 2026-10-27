import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_buyer/core/codemeta/code_meta.dart';
import 'package:hawkr_buyer/features/order/data/after_sale_model.dart';
import 'package:hawkr_buyer/features/order/data/after_sale_order_lookup.dart';
import 'package:hawkr_buyer/features/order/data/appeal_data.dart';

/// 售後申訴狀態折疊顯示（#1233）：列表卡 / 訂單卡需把 appealState 折疊成
/// 「申訴仲裁中」，否則案件進入平台仲裁後仍顯示已拒絕 / 待處理。
void main() {
  group('AfterSaleStatus appeal-aware display (#1233)', () {
    test('isUnderAppeal 只在 APPEALED(10)/INVESTIGATING(20) 為真，NONE/空為否', () {
      expect(AfterSaleStatus.isUnderAppeal('10'), isTrue);
      expect(AfterSaleStatus.isUnderAppeal('20'), isTrue);
      // DB NOT NULL DEFAULT '0' → 每筆都帶 '0'，不可誤判為申訴
      expect(AfterSaleStatus.isUnderAppeal('0'), isFalse);
      expect(AfterSaleStatus.isUnderAppeal(''), isFalse);
      // RESOLVED(30) 已裁決，不算仲裁中
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

    test('displayLabel 仲裁中覆寫 raw status，否則維持原 label', () {
      // 買家申訴(60) 與自動升級(0) 都折疊成 申訴仲裁中
      expect(AfterSaleStatus.displayLabel('60', '10'), '申訴仲裁中');
      expect(AfterSaleStatus.displayLabel('0', '20'), '申訴仲裁中');
      // 未申訴 → 維持 raw status label
      expect(AfterSaleStatus.displayLabel('60', '0'), '已拒絕');
      expect(AfterSaleStatus.displayLabel('0', ''), '待處理');
      // 已裁決 → 回 raw status
      expect(AfterSaleStatus.displayLabel('60', '30'), '已拒絕');
      // Name 不再被折疊，維持 raw status
      expect(AfterSaleStatus.displayLabel('60', 'APPEALED'), '已拒絕');
    });
  });

  group('ArbitrationState 只吃 Value', () {
    test('常數對齊 CodeMeta AfterSaleAppealState 的 Value', () {
      expect(ArbitrationState.appealed, AfterSaleAppealState.appealed.value);
      expect(
        ArbitrationState.investigating,
        AfterSaleAppealState.investigating.value,
      );
      expect(ArbitrationState.resolved, AfterSaleAppealState.resolved.value);
    });

    test('label 認得後端實際吐出的 Value，Name 一律未知狀態', () {
      // 後端 ArbitrationData.state = after_sale_cases.appeal_state（Value）
      expect(ArbitrationState.label('10'), '申訴中，等待平台裁決');
      expect(ArbitrationState.label('20'), '平台調查中');
      expect(ArbitrationState.label('30'), '已裁決');
      // Name 不是合法 wire 值
      expect(ArbitrationState.label('APPEALED'), '未知狀態');
      expect(ArbitrationState.label('RESOLVED'), '未知狀態');
    });
  });

  group('AfterSaleCaseSnapshot appeal-aware (#1233)', () {
    test('statusLabel / isUnderAppeal 跟著 appealState 折疊', () {
      const appealed = AfterSaleCaseSnapshot(
        caseNo: 'A1',
        status: '60',
        appealState: '10',
      );
      expect(appealed.isUnderAppeal, isTrue);
      expect(appealed.statusLabel, '申訴仲裁中');

      const rejected = AfterSaleCaseSnapshot(
        caseNo: 'A2',
        status: '60',
        appealState: '0',
      );
      expect(rejected.isUnderAppeal, isFalse);
      expect(rejected.statusLabel, '已拒絕');
    });
  });
}
