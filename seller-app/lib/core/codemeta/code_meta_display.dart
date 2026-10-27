import 'code_meta.dart';

typedef _LabelResolver = String? Function(String normalized);

class CodeMetaDisplay {
  const CodeMetaDisplay._();

  static String orderStatus(String? raw, {String fallback = '處理中'}) {
    return _resolveLabel(
      raw,
      resolvers: [
        (value) => OrderStatus.fromValue(value)?.label,
        (value) => OrderStatus.fromCode(value.toUpperCase())?.label,
      ],
      fallback: fallback,
    );
  }

  static String paymentMethod(String? raw, {String fallback = ''}) {
    return _resolveLabel(
      raw,
      resolvers: [
        (value) => PaymentMethod.fromValue(value)?.label,
        (value) => PaymentMethod.fromCode(value.toUpperCase())?.label,
      ],
      fallback: fallback,
    );
  }

  static String shippingMethod(String? raw, {String fallback = '未指定'}) {
    return _resolveLabel(
      raw,
      resolvers: [
        (value) => ShippingMethod.fromValue(value)?.label,
        (value) => ShippingMethod.fromCode(value.toUpperCase())?.label,
      ],
      fallback: fallback,
    );
  }

  static String shippingStatus(String? raw, {String fallback = ''}) {
    return _resolveLabel(
      raw,
      resolvers: [
        (value) => ShippingStatus.fromValue(value)?.label,
        (value) => ShippingStatus.fromCode(value.toUpperCase())?.label,
      ],
      fallback: fallback,
    );
  }

  static String couponType(String? raw, {String fallback = ''}) {
    final type = couponTypeMeta(raw);
    if (type != null) return type.label;
    return _fallback(raw, fallback);
  }

  static CouponType? couponTypeMeta(String? raw) {
    final normalized = _normalize(raw);
    if (normalized == null) return null;

    return CouponType.fromValue(normalized) ??
        CouponType.fromCode(normalized.toUpperCase()) ??
        _legacyCouponType(normalized);
  }

  static String couponBadgeVariant(String? raw) {
    return switch (couponTypeMeta(raw)) {
      CouponType.percent => 'DISCOUNT',
      CouponType.cash => 'CASH',
      null => 'CASH',
    };
  }

  static String refundReason(String? raw, {String fallback = ''}) {
    return _resolveLabel(
      raw,
      resolvers: [
        (value) => RefundReason.fromValue(value)?.label,
        (value) => RefundReason.fromCode(value.toUpperCase())?.label,
      ],
      fallback: fallback,
    );
  }

  static String afterSaleCaseStatus(String? raw, {String fallback = '未知'}) {
    return _resolveLabel(
      raw,
      resolvers: [
        (value) => AfterSaleCaseStatus.fromValue(value)?.label,
        (value) => AfterSaleCaseStatus.fromCode(value.toUpperCase())?.label,
      ],
      fallback: fallback,
    );
  }

  static String afterSaleResolutionType(String? raw, {String fallback = ''}) {
    return _resolveLabel(
      raw,
      resolvers: [
        (value) => AfterSaleResolutionType.fromValue(value)?.label,
        (value) => AfterSaleResolutionType.fromCode(value.toUpperCase())?.label,
      ],
      fallback: fallback,
    );
  }

  static String afterSaleRejectReason(String? raw, {String fallback = ''}) {
    return _resolveLabel(
      raw,
      resolvers: [
        (value) => AfterSaleRejectReason.fromValue(value)?.label,
        (value) => AfterSaleRejectReason.fromCode(value.toUpperCase())?.label,
      ],
      fallback: fallback,
    );
  }

  static String _resolveLabel(
    String? raw, {
    required List<_LabelResolver> resolvers,
    required String fallback,
  }) {
    final normalized = _normalize(raw);
    if (normalized == null) return fallback;

    for (final resolver in resolvers) {
      final label = resolver(normalized);
      if (label != null && label.isNotEmpty) return label;
    }

    return normalized;
  }

  static CouponType? _legacyCouponType(String normalized) {
    return switch (normalized.toUpperCase()) {
      'CASH' => CouponType.cash,
      'DISCOUNT' || 'PERCENT' => CouponType.percent,
      _ => null,
    };
  }

  static String? _normalize(String? raw) {
    if (raw == null) return null;
    final value = raw.trim();
    if (value.isEmpty || value == 'null') return null;
    return value;
  }

  static String _fallback(String? raw, String fallback) {
    final normalized = _normalize(raw);
    return normalized ?? fallback;
  }
}
