import 'package:riverpod_annotation/riverpod_annotation.dart';
import '../../../core/session/session_provider.dart';
import '../data/wallet_model.dart';
import '../data/wallet_repository.dart';

part 'wallet_controller.g.dart';

Duration? _disableCouponAutomaticRetry(int _, Object __) => null;

@Riverpod(keepAlive: true, retry: _disableCouponAutomaticRetry)
class CouponController extends _$CouponController {
  @override
  Future<List<WalletCouponCardState>> build({int status = 0}) async {
    ref.keepAlive();
    // 未登入（訪客／登出後）不打需登入的 API，避免 401 風暴；
    // session 變動會自動重建。
    final session = ref.watch(sessionProvider).value;
    if (!hasUsableSession(session)) {
      return const [];
    }
    final coupons = await ref
        .watch(walletRepositoryProvider)
        .getMyCoupons(status: status);
    return List.unmodifiable(coupons.map(WalletCouponCardState.fromCoupon));
  }

  Future<void> refresh() async {
    ref.invalidateSelf();
    await future;
  }
}

final class WalletCouponCardState {
  const WalletCouponCardState({
    required this.id,
    required this.title,
    required this.content,
    required this.rule,
    required this.expire,
    required this.discountLabel,
    required this.sellerName,
    required this.storeId,
    required this.sellerAvatarUrl,
    required this.ruleDescription,
  });

  factory WalletCouponCardState.fromCoupon(Coupon coupon) {
    final threshold = coupon.thresholdAmount;
    final discount = coupon.discountAmount;
    final percent = coupon.discountPercent;
    final title = coupon.name.trim().isEmpty ? '優惠券' : coupon.name.trim();

    String content;
    String discountLabel;
    if (discount != null && discount > 0) {
      content = threshold != null && threshold > 0
          ? '消費滿 ${_currency(threshold)} 可折抵 ${_currency(discount)}'
          : '現金折抵 ${_currency(discount)}';
      discountLabel = _currency(discount);
    } else if (percent != null && percent > 0) {
      final fold = percent > 1 ? percent : percent * 10;
      final foldLabel = _decimal(fold);
      content = threshold != null && threshold > 0
          ? '消費滿 ${_currency(threshold)} 享 $foldLabel折'
          : '$foldLabel折優惠';
      discountLabel = '$foldLabel 折';
    } else {
      content = '優惠券';
      discountLabel = '';
    }

    final ruleParts = <String>[
      if (threshold != null && threshold > 0) '消費滿 ${_currency(threshold)} 可用',
      if (coupon.usageLimitPerAccount case final limit? when limit > 0)
        '每人限用 ${_whole(discount: limit)} 次',
    ];

    return WalletCouponCardState(
      id: coupon.id,
      title: title,
      content: content,
      rule: ruleParts.join('  '),
      expire: _expiryLabel(coupon.endAt),
      discountLabel: discountLabel,
      sellerName: (coupon.sellerName ?? '').trim(),
      storeId: (coupon.storeId ?? '').trim(),
      sellerAvatarUrl: (coupon.sellerAvatarUrl ?? '').trim(),
      ruleDescription: _ruleDescription(coupon),
    );
  }

  final String id;
  final String title;
  final String content;
  final String rule;
  final String expire;
  final String discountLabel;
  final String sellerName;
  final String storeId;
  final String sellerAvatarUrl;
  final String ruleDescription;

  static String _whole({required num discount}) => discount.toStringAsFixed(0);

  static String _currency(num value) {
    final whole = value.toStringAsFixed(0);
    final negative = whole.startsWith('-');
    final digits = negative ? whole.substring(1) : whole;
    final buffer = StringBuffer();
    for (var index = 0; index < digits.length; index++) {
      if (index > 0 && (digits.length - index) % 3 == 0) {
        buffer.write(',');
      }
      buffer.write(digits[index]);
    }
    return '${negative ? '-' : ''}NT\$ $buffer';
  }

  static String _decimal(num value) {
    return value % 1 == 0 ? value.toStringAsFixed(0) : value.toStringAsFixed(1);
  }

  static String _expiryLabel(String? raw) {
    final value = raw?.trim() ?? '';
    if (value.isEmpty) return '';
    try {
      final date = DateTime.parse(value).toLocal();
      return '有效期至 ${date.year}/${date.month.toString().padLeft(2, '0')}/${date.day.toString().padLeft(2, '0')}';
    } on FormatException {
      final fallback = value.length >= 10
          ? value.substring(0, 10).replaceAll('-', '/')
          : value;
      return '有效期至 $fallback';
    }
  }

  /// 規則說明只由 wire 真的有的欄位組：`thresholdAmount` / `discountAmount` /
  /// `discountPercent` / `maxDiscountAmount` / `usageLimitPerAccount`。
  /// 舊版讀 `ruleType` / `conditionType` / `discountRate` / `minAmount`，
  /// 四個欄位後端一個都不吐，那三行從來沒有出現過。
  static String _ruleDescription(Coupon coupon) {
    final lines = <String>[];
    final minimum = coupon.thresholdAmount;
    if (minimum != null && minimum > 0) {
      lines.add('使用門檻：滿 NT\$${minimum.toStringAsFixed(0)}');
    }
    final amount = coupon.discountAmount;
    if (amount != null && amount > 0) {
      lines.add('折抵金額：NT\$${amount.toStringAsFixed(0)}');
    }
    final percent = coupon.discountPercent;
    if (percent != null && percent > 0) {
      final fold = percent > 1 ? percent : percent * 10;
      lines.add('折扣：${_decimal(fold)} 折');
    }
    final cap = coupon.maxDiscountAmount;
    if (cap != null && cap > 0) {
      lines.add('折抵上限：NT\$${cap.toStringAsFixed(0)}');
    }
    final limit = coupon.usageLimitPerAccount;
    if (limit != null && limit > 0) {
      lines.add('每人限用：${limit.toStringAsFixed(0)} 次');
    }
    return lines.isEmpty ? '無使用規則說明' : lines.join('\n');
  }
}
