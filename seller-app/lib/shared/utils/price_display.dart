/// SKU 特價顯示與判斷共用 helper（PayUni issue #758）
///
/// 規則來源：docs/payuni_sale_price_two_app_integration_plan.md §2.1
class PriceDisplay {
  PriceDisplay._();

  /// 安全解析金錢（接受 num、"150"、"150.0"、null）。
  static num? parseMoney(dynamic v) {
    if (v == null) return null;
    if (v is num) return v;
    final s = v.toString().trim();
    if (s.isEmpty) return null;
    return num.tryParse(s);
  }

  /// SKU 主價格：salePrice 有效時用 salePrice，否則用 price。
  static num? effectiveSkuPrice({dynamic price, dynamic salePrice}) {
    final p = parseMoney(price);
    if (hasSkuSale(price: price, salePrice: salePrice)) {
      return parseMoney(salePrice);
    }
    return p;
  }

  /// SKU 是否有有效特價：salePrice > 0 且 salePrice < price。
  static bool hasSkuSale({dynamic price, dynamic salePrice}) {
    final p = parseMoney(price);
    final s = parseMoney(salePrice);
    if (p == null || s == null) return false;
    if (s <= 0) return false;
    return s < p;
  }

  /// 商品卡是否有特價區間。
  /// 後端規則：全部無特價時 salePriceMin/Max 為 "0"。
  static bool hasRangeSale({dynamic salePriceMin, dynamic salePriceMax}) {
    final min = parseMoney(salePriceMin);
    final max = parseMoney(salePriceMax);
    if (min == null && max == null) return false;
    if ((min ?? 0) <= 0 && (max ?? 0) <= 0) return false;
    return true;
  }

  /// 區間顯示用的金額解析：後端契約以 `"0"` 表達「無價格 / 無特價」
  /// （入站 `parsePrice("0") → null`、出站 `formatAmount(null) → "0"`），
  /// 所以 <= 0 一律視為缺席，不能印成 `NT$0`。
  ///
  /// 同檔 [hasRangeSale] 早就是這個判準，只有區間格式化漏掉 —— 一個檔案裡
  /// 對同一個 `"0"` 有兩套讀法。[parseMoney] 維持原樣（它另有 SKU 比價的
  /// 呼叫端，那些地方 0 與 null 的區別是有意義的）。
  static num? _positiveMoney(dynamic v) {
    final n = parseMoney(v);
    if (n == null || n <= 0) return null;
    return n;
  }

  /// `NT$150` 或 `NT$150~200`；兩端皆無價格時回空字串。
  static String formatPriceRange(
    dynamic min,
    dynamic max, {
    String prefix = 'NT\$',
  }) {
    final mn = _positiveMoney(min);
    final mx = _positiveMoney(max);
    if (mn == null && mx == null) return '';
    if (mn != null && mx != null) {
      final mnStr = _fmtInt(mn);
      final mxStr = _fmtInt(mx);
      if (mn == mx) return '$prefix$mnStr';
      return '$prefix$mnStr~$mxStr';
    }
    return '$prefix${_fmtInt((mn ?? mx)!)}';
  }

  /// 單一價格；`"0"` / null / <= 0 一律視為無價格，回空字串。
  ///
  /// 給「只有一個價格值」的渲染點用。它們過去各自寫 `?? 0` 再插字串，而
  /// `"0"` 會印成 `NT$0` —— 判準必須與區間版同一份。
  static String formatPrice(dynamic value, {String prefix = 'NT\$'}) =>
      formatPriceRange(value, null, prefix: prefix);

  /// 商品卡主價格：優先用 priceMin/Max 區間，區間缺席時才退回單一 SKU 的
  /// `salePrice ?? price`。全部缺席（含後端以 `"0"` 表達的無價格）回空字串，
  /// 呼叫端據此整條價格列不畫。
  ///
  /// 抽成純函式而非留在 widget 裡：`_AddedProductCard` 是私有 + initState 打
  /// 網路，widget test 綁不住；把判準留在那裡等於沒有任何測試守得到。
  static String productCardPrice({
    dynamic priceMin,
    dynamic priceMax,
    dynamic skuPrice,
    dynamic skuSalePrice,
    String prefix = 'NT\$',
  }) {
    final range = formatPriceRange(priceMin, priceMax, prefix: prefix);
    if (range.isNotEmpty) return range;
    return formatPrice(
      effectiveSkuPrice(price: skuPrice, salePrice: skuSalePrice),
      prefix: prefix,
    );
  }

  /// 原價刪除線顯示條件：listPrice > effectivePrice
  static bool shouldStrikeListPrice(dynamic listPrice, dynamic effectivePrice) {
    final lp = parseMoney(listPrice);
    final ep = parseMoney(effectivePrice);
    if (lp == null || ep == null) return false;
    return lp > ep;
  }

  /// 折扣百分比（四捨五入到整數），無效時回 null。
  /// 例：原價 320、特價 240 -> 25%
  static int? discountPercent({dynamic listPrice, dynamic salePrice}) {
    final lp = parseMoney(listPrice);
    final sp = parseMoney(salePrice);
    if (lp == null || sp == null || lp <= 0 || sp <= 0 || sp >= lp) return null;
    return (((lp - sp) / lp) * 100).round();
  }

  static String _fmtInt(num v) {
    if (v == v.roundToDouble()) return v.toInt().toString();
    return v.toStringAsFixed(2);
  }
}
