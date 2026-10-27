import 'package:hawkr_buyer/core/codemeta/code_meta.dart';
import 'package:hawkr_buyer/core/codemeta/shipping_presentation.dart';
import 'package:flutter_test/flutter_test.dart';

/// `shippingPresentationOf` 的值域契約。
///
/// 值域來源是 CodeMeta 生成檔 `ShippingMethod`，不是手寫清單 —— 「每個成員都要有
/// 明確歸屬」這條斷言是遍歷 `ShippingMethod.values` 得到的，所以 CodeMeta 新增碼值
/// 而沒有人來本檔決定它的歸屬時，這支測試會連同編譯錯誤一起把人擋下來。
void main() {
  group('shippingPresentationOf', () {
    test('3001 黑貓宅急便 → homeDelivery', () {
      expect(shippingPresentationOf('3001'), ShippingPresentation.homeDelivery);
      expect(
        shippingPresentationOf('TAQBIN'),
        ShippingPresentation.homeDelivery,
      );
    });

    test('3002 7-11 店到店 → cvsStorePickup', () {
      expect(
        shippingPresentationOf('3002'),
        ShippingPresentation.cvsStorePickup,
      );
      expect(
        shippingPresentationOf('SEVEN_ELEVEN_STORE_PICKUP'),
        ShippingPresentation.cvsStorePickup,
      );
    });

    test('3003 / 3004 / 3005 → unsupported（不得落到黑貓或 7-11）', () {
      // 3003 是已刪除的 Uber 即時配送（值域仍在 CodeMeta）。
      for (final value in const ['3003', 'UBER_INSTANT', '3004', '3005']) {
        expect(
          shippingPresentationOf(value),
          ShippingPresentation.unsupported,
          reason: '$value 被靜默當成已支援的物流商 —— 這正是本次要移除的二元 fallback',
        );
      }
    });

    test('空字串 / null / 不在值域內的碼值 → unsupported', () {
      for (final value in const [null, '', '   ', 'null', '9999', 'FOO']) {
        expect(
          shippingPresentationOf(value),
          ShippingPresentation.unsupported,
          reason: '未知值 ${value ?? "null"} 必須 fail-safe',
        );
      }
    });

    test('CodeMeta 每個 ShippingMethod 成員都有明確歸屬（無漏網）', () {
      for (final method in ShippingMethod.values) {
        // 不做「等於某個特定值」的斷言 —— 這裡要證明的是「查得到、不會拋例外」，
        // 各成員該落到哪一格由上面三支測試逐一釘住。
        expect(
          ShippingPresentation.values,
          contains(shippingPresentationOf(method.value)),
          reason:
              '${method.value} (${method.code}) 沒有被 shippingPresentationOf 覆蓋',
        );
      }
      // 目前 3001 / 3002 有專屬 UI；新增支援時本斷言要一起改，
      // 它的作用是讓「悄悄多支援一個 provider」不會無聲通過。
      final supported = ShippingMethod.values
          .where((m) => shippingPresentationOf(m.value).isSupported)
          .map((m) => m.value)
          .toList();
      expect(supported, ['3001', '3002']);
    });
  });

  group('usesRecipientAddress（收件端是買家地址而不是門市）', () {
    test('3001 宅配要收件地址', () {
      expect(
        shippingPresentationOf('3001').usesRecipientAddress,
        isTrue,
        reason: '宅配送到府',
      );
    });

    test('3002 超商取貨與未支援的配送方式都不帶收件地址', () {
      expect(shippingPresentationOf('3002').usesRecipientAddress, isFalse);
      expect(shippingPresentationOf('3003').usesRecipientAddress, isFalse);
      expect(shippingPresentationOf('3004').usesRecipientAddress, isFalse);
      expect(shippingPresentationOf('9999').usesRecipientAddress, isFalse);
    });
  });
}
