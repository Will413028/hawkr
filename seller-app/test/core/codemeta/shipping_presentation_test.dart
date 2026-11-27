import 'package:hawkr_seller/core/codemeta/code_meta.dart';
import 'package:hawkr_seller/core/codemeta/shipping_presentation.dart';
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

    test('3003 / 3005 → unsupported（不得落到黑貓或 7-11）', () {
      for (final value in const ['3003', '3005']) {
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
        expect(
          ShippingPresentation.values,
          contains(shippingPresentationOf(method.value)),
          reason:
              '${method.value} (${method.code}) 沒有被 shippingPresentationOf 覆蓋',
        );
      }
      final supported = ShippingMethod.values
          .where((m) => shippingPresentationOf(m.value).isSupported)
          .map((m) => m.value)
          .toList();
      expect(supported, ['3001', '3002']);
    });
  });
}
