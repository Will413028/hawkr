import '../data/seller_quote_model.dart';

/// 回價表單的欄位鍵；[SellerQuoteValidation.fieldErrors] 用它定位錯誤。
enum SellerQuoteField {
  responseType,
  qualityGrade,
  qualityInspectionNotes,
  unitBasisQuantity,
  unitBasisUnit,
  offeredQuantity,
  offeredQuantityUnit,
  currency,
  goodsAmount,
  shippingAmount,
  taxAmount,
  taxInclusionBasis,
  totalAmount,
  promisedDeliveryAt,
  validUntil,
}

/// decimal string 的加總與比較。
///
/// 金額一律 decimal string，**不經過 `double`**：`0.1 + 0.2` 在 binary float
/// 下不等於 `0.3`，拿它去比對 server 的恆等式會得到假的紅字。這裡用 [BigInt]
/// 做定點運算。
class QuoteDecimal {
  const QuoteDecimal._(this.units, this.scale);

  /// 以 `10^scale` 為單位的整數值。
  final BigInt units;

  /// 小數位數。
  final int scale;

  static final RegExp _pattern = RegExp(r'^\d+(\.\d+)?$');

  /// 解析非負 decimal string；格式不合回傳 null。
  ///
  /// 不接受正負號、科學記號與千分位——這三種都不是 wire 上的 decimal string。
  static QuoteDecimal? tryParse(String raw) {
    final value = raw.trim();
    if (!_pattern.hasMatch(value)) return null;
    final dot = value.indexOf('.');
    if (dot < 0) return QuoteDecimal._(BigInt.parse(value), 0);
    final fraction = value.substring(dot + 1);
    final digits = value.substring(0, dot) + fraction;
    return QuoteDecimal._(BigInt.parse(digits), fraction.length);
  }

  QuoteDecimal _rescale(int target) {
    if (target == scale) return this;
    final factor = BigInt.from(10).pow(target - scale);
    return QuoteDecimal._(units * factor, target);
  }

  QuoteDecimal operator +(QuoteDecimal other) {
    final target = scale > other.scale ? scale : other.scale;
    final a = _rescale(target);
    final b = other._rescale(target);
    return QuoteDecimal._(a.units + b.units, target);
  }

  /// 數值相等（`10` == `10.00`）。
  bool sameValueAs(QuoteDecimal other) {
    final target = scale > other.scale ? scale : other.scale;
    return _rescale(target).units == other._rescale(target).units;
  }

  @override
  String toString() {
    if (scale == 0) return units.toString();
    final digits = units.toString().padLeft(scale + 1, '0');
    final cut = digits.length - scale;
    return '${digits.substring(0, cut)}.${digits.substring(cut)}';
  }

  /// 貨款＋運費＋稅額；任一欄格式不合回傳 null。
  static QuoteDecimal? sumOf(Iterable<String> values) {
    QuoteDecimal? total;
    for (final value in values) {
      final parsed = tryParse(value);
      if (parsed == null) return null;
      total = total == null ? parsed : total + parsed;
    }
    return total;
  }
}

/// RFC3339 判定：必須有日期、`T`、時間，以及 `Z` 或 `±hh:mm` 位移。
bool isRfc3339(String raw) {
  final value = raw.trim();
  if (value.isEmpty) return false;
  if (!RegExp(
    r'^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})$',
  ).hasMatch(value)) {
    return false;
  }
  return DateTime.tryParse(value) != null;
}

/// 把本地時間轉成 wire 要的 RFC3339 UTC 字串（不帶毫秒）。
String toRfc3339Utc(DateTime value) {
  final utc = value.toUtc();
  String two(int n) => n.toString().padLeft(2, '0');
  return '${utc.year.toString().padLeft(4, '0')}-${two(utc.month)}-'
      '${two(utc.day)}T${two(utc.hour)}:${two(utc.minute)}:'
      '${two(utc.second)}Z';
}

/// 表單驗證結果。
class SellerQuoteValidation {
  const SellerQuoteValidation({
    required this.fieldErrors,
    required this.computedTotal,
    required this.amountIdentityHolds,
  });

  /// 欄位 → 該欄的錯誤訊息。空 map ＝ 可以送出。
  final Map<SellerQuoteField, String> fieldErrors;

  /// 貨款＋運費＋稅額的即時計算結果；任一欄格式不合時為 null。
  ///
  /// ⛔ 只用來提示。**不得**拿它覆寫使用者輸入的 `totalAmount`——server 才是權威。
  final String? computedTotal;

  /// `totalAmount == goods + shipping + tax` 是否成立。
  final bool amountIdentityHolds;

  /// 正式提交是否可以送出。
  bool get isSubmittable => fieldErrors.isEmpty;

  /// 還沒填好的必填欄位（[kRequiredQuoteFields] 的子集）。
  List<SellerQuoteField> get missingRequiredFields => kRequiredQuoteFields
      .where(fieldErrors.containsKey)
      .toList(growable: false);

  String? errorFor(SellerQuoteField field) => fieldErrors[field];
}

/// 正式提交的必填欄位。
///
/// ⛔ 這份清單**不看 `responseType`**：v6.0 逐字要求，不論接受非價格條款或提出
/// 不同條件，都必須建立含實際品相、unit basis、goods／shipping／tax／total 與
/// validUntil 的 immutable sellerResponseVersion。
/// `ACCEPT_NON_PRICE_TERMS_WITH_QUOTE` 不是「少填金額的捷徑」。
const List<SellerQuoteField> kRequiredQuoteFields = <SellerQuoteField>[
  SellerQuoteField.responseType,
  SellerQuoteField.qualityGrade,
  SellerQuoteField.qualityInspectionNotes,
  SellerQuoteField.unitBasisQuantity,
  SellerQuoteField.unitBasisUnit,
  SellerQuoteField.offeredQuantity,
  SellerQuoteField.offeredQuantityUnit,
  SellerQuoteField.goodsAmount,
  SellerQuoteField.shippingAmount,
  SellerQuoteField.taxAmount,
  SellerQuoteField.taxInclusionBasis,
  SellerQuoteField.totalAmount,
  SellerQuoteField.validUntil,
];

/// 驗證一份正式提交用的 payload。
///
/// 草稿**不走這裡**：草稿可不限次改，不計正式一次，也不驗金額恆等式。
SellerQuoteValidation validateSellerQuote(
  SellerQuotePayload quote, {
  DateTime? now,
}) {
  final errors = <SellerQuoteField, String>{};

  if (quote.responseTypeValue == null) {
    errors[SellerQuoteField.responseType] = '請選擇回應方式';
  }

  // ⛔ 產品規則 1：實際品相不得由平台或 LLM 代填。
  if (quote.offeredQuality.grade.trim().isEmpty) {
    errors[SellerQuoteField.qualityGrade] = '請填寫這批貨的實際品相等級';
  }
  if (quote.offeredQuality.inspectionNotes.trim().isEmpty) {
    errors[SellerQuoteField.qualityInspectionNotes] = '請填寫驗收說明';
  }

  if (quote.unitBasis.quantity.trim().isEmpty) {
    errors[SellerQuoteField.unitBasisQuantity] = '請填寫計價基準數量';
  } else if (QuoteDecimal.tryParse(quote.unitBasis.quantity) == null) {
    errors[SellerQuoteField.unitBasisQuantity] = '計價基準數量只能是數字';
  }
  if (quote.unitBasis.unit.trim().isEmpty) {
    errors[SellerQuoteField.unitBasisUnit] = '請填寫計價基準單位';
  }

  if (quote.offeredQuantity.trim().isEmpty) {
    errors[SellerQuoteField.offeredQuantity] = '請填寫供應數量';
  } else if (QuoteDecimal.tryParse(quote.offeredQuantity) == null) {
    errors[SellerQuoteField.offeredQuantity] = '供應數量只能是數字';
  }
  if (quote.offeredQuantityUnit.trim().isEmpty) {
    errors[SellerQuoteField.offeredQuantityUnit] = '請填寫供應數量的單位';
  }

  if (quote.currency != kQuoteCurrencyTwd) {
    errors[SellerQuoteField.currency] = '目前只支援 $kQuoteCurrencyTwd';
  }

  const amountFields = <SellerQuoteField, String>{
    SellerQuoteField.goodsAmount: '貨款',
    SellerQuoteField.shippingAmount: '運費',
    SellerQuoteField.taxAmount: '稅額',
    SellerQuoteField.totalAmount: '總金額',
  };
  final amountValues = <SellerQuoteField, String>{
    SellerQuoteField.goodsAmount: quote.goodsAmount,
    SellerQuoteField.shippingAmount: quote.shippingAmount,
    SellerQuoteField.taxAmount: quote.taxAmount,
    SellerQuoteField.totalAmount: quote.totalAmount,
  };
  amountFields.forEach((field, label) {
    final raw = amountValues[field]!;
    if (raw.trim().isEmpty) {
      errors[field] = '請填寫$label';
    } else if (QuoteDecimal.tryParse(raw) == null) {
      errors[field] = '$label只能是非負數字，例如 1200 或 1200.50';
    }
  });

  if (quote.taxInclusionBasisValue == null) {
    errors[SellerQuoteField.taxInclusionBasis] = '請選擇報價含稅或未稅';
  }

  final computed = QuoteDecimal.sumOf([
    quote.goodsAmount,
    quote.shippingAmount,
    quote.taxAmount,
  ]);
  final declared = QuoteDecimal.tryParse(quote.totalAmount);
  final identityHolds =
      computed != null && declared != null && declared.sameValueAs(computed);
  if (computed != null && declared != null && !identityHolds) {
    errors[SellerQuoteField.totalAmount] = '總金額必須等於貨款＋運費＋稅額（目前三項合計為 $computed）';
  }

  final reference = now ?? DateTime.now();
  final validUntil = quote.validUntil.trim();
  if (validUntil.isEmpty) {
    errors[SellerQuoteField.validUntil] = '請設定報價有效期限';
  } else if (!isRfc3339(validUntil)) {
    errors[SellerQuoteField.validUntil] = '有效期限格式不正確';
  } else if (!DateTime.parse(validUntil).isAfter(reference)) {
    errors[SellerQuoteField.validUntil] = '有效期限必須晚於現在';
  }

  final promised = quote.promisedDeliveryAt.trim();
  if (promised.isNotEmpty && !isRfc3339(promised)) {
    errors[SellerQuoteField.promisedDeliveryAt] = '承諾交貨時間格式不正確';
  }

  return SellerQuoteValidation(
    fieldErrors: Map.unmodifiable(errors),
    computedTotal: computed?.toString(),
    amountIdentityHolds: identityHolds,
  );
}
