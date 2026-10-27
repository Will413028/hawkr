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
  static num? positiveMoney(dynamic v) {
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
    final mn = positiveMoney(min);
    final mx = positiveMoney(max);
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
  /// 給「只有一個價格值」的渲染點用（社群貼文商品卡、聊天商品卡、搜尋結果
  /// 列）。它們過去各自寫 `if (price.isNotEmpty)`，而 `"0"` 是非空字串 ——
  /// 判準必須與區間版同一份，否則同一個 `"0"` 在不同畫面有不同讀法。
  static String formatPrice(dynamic value, {String prefix = 'NT\$'}) =>
      formatPriceRange(value, null, prefix: prefix);

  /// 原價刪除線區間。無原價、無有效售價、或最高原價 <= 售價時回空字串。
  ///
  /// 「沒有有效售價」也回空字串：刪除線的語意是「原價比現價高」，現價缺席時
  /// 單獨掛一條被劃掉的原價，讀者無從得知現在賣多少。
  static String formatListPriceRange(
    dynamic listMin,
    dynamic listMax, {
    required dynamic effectivePrice,
    String prefix = '',
  }) {
    final lpMin = positiveMoney(listMin);
    final lpMax = positiveMoney(listMax);
    if (lpMin == null && lpMax == null) return '';
    final ep = positiveMoney(effectivePrice);
    if (ep == null) return '';
    final highest = (lpMax ?? lpMin)!;
    if (highest <= ep) return '';
    return '$prefix${formatRangeRaw(lpMin, lpMax)}';
  }

  /// 區間標籤 ＋ 單價。標籤規則同 [formatPriceRange]（空字串＝不顯示價格）。
  ///
  /// `unitPrice` 只在區間收斂成單一價格時才有值：有區間的商品要進詳情頁選規格
  /// 才知道單價，不能拿區間的一端冒充。
  static (String, double?) priceLabelAndUnit(
    dynamic min,
    dynamic max, {
    String prefix = 'NT\$',
  }) {
    final label = formatPriceRange(min, max, prefix: prefix);
    if (label.isEmpty) return ('', null);
    final mn = positiveMoney(min);
    final mx = positiveMoney(max);
    final single = (mn == null || mx == null || mn == mx)
        ? (mn ?? mx)!.toDouble()
        : null;
    return (label, single);
  }

  /// 純數字區間（不含 prefix）；兩端皆無價格時回空字串。
  static String formatRangeRaw(dynamic min, dynamic max) {
    final mn = positiveMoney(min);
    final mx = positiveMoney(max);
    if (mn == null && mx == null) return '';
    if (mn != null && mx != null) {
      if (mn == mx) return _fmtInt(mn);
      return '${_fmtInt(mn)}~${_fmtInt(mx)}';
    }
    return _fmtInt((mn ?? mx)!);
  }

  /// 原價刪除線顯示條件：listPrice > effectivePrice
  static bool shouldStrikeListPrice(dynamic listPrice, dynamic effectivePrice) {
    final lp = parseMoney(listPrice);
    final ep = parseMoney(effectivePrice);
    if (lp == null || ep == null) return false;
    return lp > ep;
  }

  /// 折扣百分比（四捨五入到整數），無效時回 null。
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
