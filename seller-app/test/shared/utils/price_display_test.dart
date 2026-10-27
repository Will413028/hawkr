import 'package:hawkr_seller/shared/utils/price_display.dart';
import 'package:flutter_test/flutter_test.dart';

/// 後端契約：價格欄位的 `"0"` ＝ 「無價格 / 無特價」
/// （入站 `parsePrice("0") → null`、出站 `formatAmount(null) → "0"`，雙向一致）。
///
/// 這份契約在 `price_display.dart` 裡原本只被遵守了一半：`hasRangeSale` 認
/// `<= 0` 是缺席，但 `formatPriceRange` 直接 `num.tryParse("0") → 0` 而印出
/// `NT$0` —— 同一個檔案對同一個 `"0"` 有兩套讀法。
///
/// `formatPriceRange` 現在由 `productCardPrice` 承載，經賣家商城卡片
/// （`page_shop_tab.dart` `_AddedProductCard`）真的上畫面 —— 它先前沒有呼叫點，
/// 而卡片自己 `int.tryParse` 再拼字串，印得出 `NT $0`。
/// 買家端的對應測試在 `hawkr_buyer/test/shared/utils/price_display_test.dart`，
/// 兩邊的期望值必須逐條相同。
///
/// `num.tryParse("0")` 回的是合法的 0 不是 null，所以 `mn == null` 這種
/// null 判斷完全擋不住它，也不會有任何 analyzer 警告。只能靠斷言釘住。
void main() {
  group('PriceDisplay.formatPriceRange — "0" 是無價格', () {
    test('兩端皆為 "0" 回空字串，不得出現 NT\$0', () {
      expect(PriceDisplay.formatPriceRange('0', '0'), '');
      expect(PriceDisplay.formatPriceRange('0', null), '');
      expect(PriceDisplay.formatPriceRange(null, '0'), '');
      expect(PriceDisplay.formatPriceRange(0, 0), '');
    });

    test('單邊為 "0" 時那一邊視為缺席，只印有價格的一邊', () {
      expect(PriceDisplay.formatPriceRange('0', '250'), 'NT\$250');
      expect(PriceDisplay.formatPriceRange('250', '0'), 'NT\$250');
    });

    test('負數同樣視為無價格', () {
      expect(PriceDisplay.formatPriceRange('-100', '-50'), '');
    });

    test('缺席與非數字回空字串', () {
      expect(PriceDisplay.formatPriceRange(null, null), '');
      expect(PriceDisplay.formatPriceRange('', ''), '');
      expect(PriceDisplay.formatPriceRange('abc', 'abc'), '');
    });

    test('正常價格的格式沒有被改動', () {
      expect(PriceDisplay.formatPriceRange('150', '150'), 'NT\$150');
      expect(PriceDisplay.formatPriceRange('150', '200'), 'NT\$150~200');
      expect(
        PriceDisplay.formatPriceRange('150', '200', prefix: 'NT\$ '),
        'NT\$ 150~200',
      );
    });
  });

  group('PriceDisplay.formatPrice — 單一價格', () {
    test('"0" / 0 / 負數 / 缺席一律回空字串', () {
      expect(PriceDisplay.formatPrice('0'), '');
      expect(PriceDisplay.formatPrice(0), '');
      expect(PriceDisplay.formatPrice('-1'), '');
      expect(PriceDisplay.formatPrice(null), '');
      expect(PriceDisplay.formatPrice(''), '');
    });

    test('正常價格帶 prefix', () {
      expect(PriceDisplay.formatPrice('120', prefix: 'NT \$'), 'NT \$120');
    });
  });

  /// 賣家商城卡片（`page_shop_tab.dart` `_AddedProductCard`）的價格判準。
  ///
  /// 那張卡是私有 widget 且 initState 會打網路，widget test 綁不住，所以判準
  /// 全部抽到這裡 —— 卡片內已不剩任何 parse，這組斷言就是它的守門人。
  group('PriceDisplay.productCardPrice — 賣家商城卡片主價格', () {
    test('priceMin/Max 都是 "0" 且無 SKU 價格時回空字串（整條價格列不畫）', () {
      expect(PriceDisplay.productCardPrice(priceMin: '0', priceMax: '0'), '');
      expect(
        PriceDisplay.productCardPrice(
          priceMin: 0,
          priceMax: 0,
          skuPrice: '0',
          skuSalePrice: '0',
        ),
        '',
      );
    });

    test('區間有效時優先用區間', () {
      expect(
        PriceDisplay.productCardPrice(
          priceMin: 120,
          priceMax: 250,
          skuPrice: '999',
          prefix: 'NT \$',
        ),
        'NT \$120~250',
      );
      expect(
        PriceDisplay.productCardPrice(priceMin: 120, priceMax: 120),
        'NT\$120',
      );
    });

    test('區間缺席（"0"）時退回 SKU 的 salePrice ?? price', () {
      expect(
        PriceDisplay.productCardPrice(
          priceMin: 0,
          priceMax: 0,
          skuPrice: '200',
          skuSalePrice: '150',
          prefix: 'NT \$',
        ),
        'NT \$150',
      );
      // salePrice "0" ＝ 無特價，退回原價而不是印 0
      expect(
        PriceDisplay.productCardPrice(
          priceMin: null,
          priceMax: null,
          skuPrice: '200',
          skuSalePrice: '0',
          prefix: 'NT \$',
        ),
        'NT \$200',
      );
    });

    test('SKU 價格也是 "0" 時不得印成 NT \$0', () {
      expect(
        PriceDisplay.productCardPrice(
          priceMin: null,
          priceMax: null,
          skuPrice: '0',
          skuSalePrice: null,
          prefix: 'NT \$',
        ),
        '',
      );
    });
  });

  group('parseMoney 的語意不受影響', () {
    test('SKU 比價路徑仍然分得出 0 與缺席', () {
      // effectiveSkuPrice / hasSkuSale / discountPercent 靠 0 與 null 的區別
      // 做判斷，區間顯示的修正不得外溢到它們身上。
      expect(PriceDisplay.parseMoney('0'), 0);
      expect(PriceDisplay.parseMoney(null), isNull);
      expect(PriceDisplay.hasSkuSale(price: '200', salePrice: '0'), isFalse);
      expect(PriceDisplay.hasSkuSale(price: '200', salePrice: '150'), isTrue);
      expect(PriceDisplay.effectiveSkuPrice(price: '200', salePrice: '0'), 200);
    });
  });
}
