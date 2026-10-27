import 'dart:math' as math;

// ---------------------------------------------------------------------------
// 優惠券工具函數 — 純 Dart，無 Flutter 依賴。
//
// 從購物車 API 回傳的 coupon Map 中提取顯示資訊、
// 計算折扣金額，供 UI 層使用。
// ---------------------------------------------------------------------------

/// 取得優惠券的唯一識別 ID（嘗試多個欄位名）。
String couponIdentifier(Map<String, dynamic> coupon) {
  final candidates = [
    coupon['templateId'],
    coupon['assignment_id'],
    coupon['assignmentId'],
    coupon['coupon_id'],
    coupon['couponId'],
    coupon['id'],
  ];
  for (final candidate in candidates) {
    if (candidate == null) continue;
    final id = candidate.toString().trim();
    if (id.isNotEmpty) return id;
  }
  return coupon.hashCode.toString();
}

/// 取得優惠券名稱（嘗試多個欄位名）。
String couponDisplayTitle(Map<String, dynamic> coupon) {
  const keys = [
    'couponName',
    'coupon_name',
    'title',
    'name',
    'couponTitle',
    'display_name',
  ];
  for (final key in keys) {
    final value = coupon[key];
    if (value == null) continue;
    final text = value.toString().trim();
    if (text.isNotEmpty) return text;
  }
  return '優惠券';
}

/// 取得優惠券折扣描述文字（折扣金額 / 折數 / 免運 等）。
String couponDiscountText(Map<String, dynamic> coupon) {
  // 固定金額折抵
  final amountCandidate = coupon['discount_amount'] ?? coupon['discountAmount'];
  if (amountCandidate != null) {
    final num? amount = num.tryParse(amountCandidate.toString());
    if (amount != null && amount > 0) {
      return '折抵 NT\$${amount.toStringAsFixed(0)}';
    }
  }
  // 最高折抵上限
  final capCandidate = coupon['discount_cap'] ?? coupon['discountCap'];
  if (capCandidate != null) {
    final num? cap = num.tryParse(capCandidate.toString());
    if (cap != null && cap > 0) {
      return '最高折抵 NT\$${cap.toStringAsFixed(0)}';
    }
  }
  // 折扣比例
  final rateCandidate = coupon['discount_rate'] ?? coupon['discountRate'];
  if (rateCandidate != null) {
    final num? rate = num.tryParse(rateCandidate.toString());
    if (rate != null && rate > 0) {
      double normalized;
      if (rate > 0 && rate < 1) {
        normalized = rate * 10;
      } else if (rate > 10) {
        normalized = rate / 10;
      } else {
        normalized = rate.toDouble();
      }
      final bool isWhole = normalized == normalized.roundToDouble();
      final display = isWhole
          ? normalized.toStringAsFixed(0)
          : normalized.toStringAsFixed(1);
      return '$display折';
    }
  }
  // 規則類型 fallback
  final rule = coupon['rule_type'] ?? coupon['ruleType'];
  if (rule is String && rule.trim().isNotEmpty) {
    return localizeRuleType(rule);
  }
  return '優惠內容';
}

/// 取得優惠券有效期限文字。
String couponExpireText(Map<String, dynamic> coupon) {
  final startCandidate =
      coupon['startAt'] ?? coupon['issue_date'] ?? coupon['start_date'];
  final endCandidate =
      coupon['endAt'] ?? coupon['expire_date'] ?? coupon['end_date'];
  final assignedCandidate = coupon['assigned_at'] ?? coupon['assignedAt'];
  final start = normalizeDateString(startCandidate?.toString());
  final end = normalizeDateString(endCandidate?.toString());
  final assigned = normalizeDateString(assignedCandidate?.toString());
  if (start.isNotEmpty && end.isNotEmpty) {
    return '使用期限：$start - $end';
  }
  if (end.isNotEmpty) {
    return '有效至 $end';
  }
  if (start.isNotEmpty) {
    return '開始於 $start';
  }
  if (assigned.isNotEmpty) {
    return '領取於 $assigned';
  }
  return '';
}

/// 取得優惠券適用商家文字。
String couponMerchantText(Map<String, dynamic> coupon) {
  final name = (coupon['sellerName'] ?? '').toString().trim();
  if (name.isNotEmpty) return '適用商家：$name';
  final id = couponMerchantId(coupon);
  if (id != null && id.isNotEmpty) return '適用商家：商家 #$id';
  return '';
}

/// 取得優惠券適用商家 ID（空 = 全平台券）。
String? couponMerchantId(Map<String, dynamic> coupon) {
  final id = (coupon['applicableStoreId'] ?? '').toString().trim();
  if (id.isNotEmpty) return id;
  return null;
}

/// 優惠券折扣計算結果。
class CouponEffect {
  const CouponEffect({
    required this.deduction,
    required this.label,
    required this.display,
  });

  final double deduction;
  final String label;
  final String display;
}

/// 根據優惠券資料和訂單基礎金額，計算實際折扣效果。
///
/// 回傳 null 表示無法計算或折扣為 0。
CouponEffect? resolveCouponEffect(
  Map<String, dynamic> coupon,
  double baseAmount,
) {
  if (baseAmount <= 0) return null;

  final discountText = couponDiscountText(coupon);
  final dynamic ruleCandidate = coupon['rule_type'] ?? coupon['ruleType'];
  final ruleLabel = discountText.isNotEmpty
      ? discountText
      : (ruleCandidate is String && ruleCandidate.trim().isNotEmpty
            ? localizeRuleType(ruleCandidate)
            : '');
  final label = ruleLabel.isNotEmpty ? '現金折抵（$ruleLabel）' : '現金折抵';

  double deduction = 0;
  String display = '';

  // 折扣上限
  double? cap;
  final capCandidate =
      coupon['discount_cap'] ??
      coupon['discountCap'] ??
      coupon['max_discount_amount'];
  if (capCandidate != null) {
    final num? capNum = num.tryParse(capCandidate.toString());
    if (capNum != null && capNum > 0) cap = capNum.toDouble();
  }

  double clampDeduction(double value) {
    var result = math.min(value, baseAmount);
    if (cap != null) result = math.min(result, cap);
    return math.max(0, result);
  }

  // 固定金額折抵
  final amountCandidate =
      coupon['discount_amount'] ?? coupon['discountAmount'] ?? coupon['amount'];
  if (amountCandidate != null) {
    final num? amountNum = num.tryParse(amountCandidate.toString());
    if (amountNum != null && amountNum > 0) {
      deduction = clampDeduction(amountNum.toDouble());
      if (deduction > 0) {
        display = '-NT\$${deduction.toStringAsFixed(0)}';
      }
    }
  }

  // 比例折扣 fallback
  if (deduction <= 0) {
    final rateCandidate =
        coupon['discount_rate'] ??
        coupon['discountRate'] ??
        coupon['percentage'] ??
        coupon['discount_value'];
    if (rateCandidate != null) {
      final num? rateNum = num.tryParse(rateCandidate.toString());
      if (rateNum != null && rateNum > 0) {
        double multiplier;
        final double r = rateNum.toDouble();
        if (r > 0 && r < 1) {
          multiplier = r;
        } else if (r > 1 && r <= 10) {
          multiplier = r / 10;
        } else if (r > 10 && r <= 100) {
          multiplier = r / 100;
        } else {
          multiplier = 1;
        }
        final double boundedMultiplier = multiplier.clamp(0.0, 1.0).toDouble();
        final rawDeduction = baseAmount * (1 - boundedMultiplier);
        deduction = clampDeduction(rawDeduction);
        if (deduction > 0) {
          final double fold = boundedMultiplier <= 0
              ? 0
              : boundedMultiplier * 10;
          final bool isWhole = fold == fold.roundToDouble();
          final displayRate = boundedMultiplier <= 0
              ? '0折'
              : '${isWhole ? fold.toStringAsFixed(0) : fold.toStringAsFixed(1)}折';
          display = '-$displayRate';
        }
      }
    }
  }

  if (deduction <= 0) return null;
  final effectiveDisplay = display.isNotEmpty
      ? display
      : '-NT\$${deduction.toStringAsFixed(0)}';
  return CouponEffect(
    deduction: deduction,
    label: label,
    display: effectiveDisplay,
  );
}

/// 將日期字串正規化為 YYYY-MM-DD 格式（去掉 T 後面的時間部分）。
String normalizeDateString(String? raw) {
  if (raw == null) return '';
  final trimmed = raw.trim();
  if (trimmed.isEmpty) return '';
  if (trimmed.contains('T')) {
    return trimmed.split('T').first;
  }
  return trimmed;
}

/// 將優惠券規則類型轉換為中文顯示文字。
String localizeRuleType(String value) {
  final normalized = value.trim().toLowerCase();
  switch (normalized) {
    case 'percentage':
    case 'percent':
    case 'rate':
      return '折扣優惠';
    case 'amount':
    case 'fixed_amount':
    case 'cash':
      return '現折優惠';
    case 'bogo':
    case 'buy_x_get_y':
      return '滿額贈優惠';
    case 'gift':
      return '贈品優惠';
    case 'cashback':
      return '現金回饋';
    default:
      final label = value.replaceAll('_', ' ').trim();
      return label.isEmpty ? '其他優惠' : '其他優惠（$label）';
  }
}
