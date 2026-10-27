import 'package:hawkr_buyer/shared/utils/price_display.dart';
import 'package:flutter_test/flutter_test.dart';

/// 後端契約：價格欄位的 `"0"` ＝ 「無價格 / 無特價」
/// （入站 `parsePrice("0") → null`、出站 `formatAmount(null) → "0"`，雙向一致）。
///
/// 這份契約在 `price_display.dart` 裡原本只被遵守了一半：`hasRangeSale` 認
/// `<= 0` 是缺席，但 `formatPriceRange` 直接 `num.tryParse("0") → 0` 而印出
/// `NT$0` —— 同一個檔案對同一個 `"0"` 有兩套讀法。買家在商品卡上看到的就是
/// 「這家店的商品零元」。
///
/// `num.tryParse("0")` 回的是合法的 0 不是 null，所以 `mn == null` 這種
/// null 判斷完全擋不住它，也不會有任何 analyzer 警告。只能靠斷言釘住。
///
/// ⚠️ 這一份只守 helper。真正會壞的是**各個 widget 自己那份 parse** ——
/// 上一輪就是只修了 helper 而漏掉四條渲染路徑。widget 層的斷言在
/// `test/shared/widgets/product_card_price_test.dart`。
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

  group('PriceDisplay.formatRangeRaw — 與 formatPriceRange 同一套判準', () {
    test('"0" 視為無價格', () {
      expect(PriceDisplay.formatRangeRaw('0', '0'), '');
      expect(PriceDisplay.formatRangeRaw('0', '250'), '250');
    });

    test('正常價格的格式沒有被改動', () {
      expect(PriceDisplay.formatRangeRaw('150', '150'), '150');
      expect(PriceDisplay.formatRangeRaw('150', '200'), '150~200');
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
      expect(PriceDisplay.formatPrice('120', prefix: 'NT\$ '), 'NT\$ 120');
      expect(PriceDisplay.formatPrice(120), 'NT\$120');
    });
  });

  group('PriceDisplay.formatListPriceRange — 刪除線原價', () {
    test('原價為 "0" 時不顯示刪除線', () {
      expect(
        PriceDisplay.formatListPriceRange('0', '0', effectivePrice: '120'),
        '',
      );
    });

    test('售價缺席（"0"）時不單獨掛一條被劃掉的原價', () {
      // 只印一條刪除線原價，讀者無從得知現在賣多少。
      expect(
        PriceDisplay.formatListPriceRange('200', '200', effectivePrice: '0'),
        '',
      );
      expect(
        PriceDisplay.formatListPriceRange('200', '200', effectivePrice: null),
        '',
      );
    });

    test('原價 <= 售價時不顯示', () {
      expect(
        PriceDisplay.formatListPriceRange('120', '120', effectivePrice: '120'),
        '',
      );
      expect(
        PriceDisplay.formatListPriceRange('100', '100', effectivePrice: '120'),
        '',
      );
    });

    test('原價高於售價才顯示，區間格式與售價一致', () {
      expect(
        PriceDisplay.formatListPriceRange('200', '200', effectivePrice: '120'),
        '200',
      );
      expect(
        PriceDisplay.formatListPriceRange('200', '300', effectivePrice: '120'),
        '200~300',
      );
      expect(
        PriceDisplay.formatListPriceRange(
          '200',
          '300',
          effectivePrice: '120',
          prefix: 'NT\$ ',
        ),
        'NT\$ 200~300',
      );
    });
  });

  group('PriceDisplay.priceLabelAndUnit — 搜尋列標籤與單價', () {
    test('"0" 回空標籤且無單價', () {
      expect(PriceDisplay.priceLabelAndUnit('0', '0'), ('', null));
      expect(PriceDisplay.priceLabelAndUnit(null, null), ('', null));
    });

    test('區間收斂成單一價格時才給 unitPrice', () {
      expect(PriceDisplay.priceLabelAndUnit('150', '150', prefix: 'NT\$ '), (
        'NT\$ 150',
        150.0,
      ));
      expect(PriceDisplay.priceLabelAndUnit('150', '0', prefix: 'NT\$ '), (
        'NT\$ 150',
        150.0,
      ));
    });

    test('真有區間時 unitPrice 為 null，不拿一端冒充', () {
      expect(PriceDisplay.priceLabelAndUnit('150', '200', prefix: 'NT\$ '), (
        'NT\$ 150~200',
        null,
      ));
    });
  });
}
