import 'package:flutter/material.dart';

import '../../../../core/theme/app_colors.dart';
import '../../../../core/theme/app_typography.dart';
import '../../../../core/theme/hk_tokens.dart';
import '../../../../shared/icons/app_icon_colors.dart';
import '../../../../shared/widgets/hk_icon.dart';

enum WalletCouponVisualStatus { available, used, expired }

/// Buyer wallet coupon rendered as a single, accessible ticket.
///
/// The value rail is part of the card layout instead of a positioned raster
/// badge, so long titles, larger text and compact screens retain their own
/// content space.
class WalletCouponTicket extends StatefulWidget {
  const WalletCouponTicket({
    super.key,
    required this.title,
    required this.content,
    required this.rule,
    required this.expiry,
    required this.discountLabel,
    required this.issuerLabel,
    required this.issuerAvatar,
    required this.status,
    this.onPressed,
  });

  final String title;
  final String content;
  final String rule;
  final String expiry;
  final String discountLabel;
  final String issuerLabel;
  final Widget issuerAvatar;
  final WalletCouponVisualStatus status;
  final VoidCallback? onPressed;

  @override
  State<WalletCouponTicket> createState() => _WalletCouponTicketState();
}

class _WalletCouponTicketState extends State<WalletCouponTicket> {
  static const double _compactRailWidth = 88;
  static const double _railWidth = 96;
  static const double _avatarSize = HkSpace.large;
  static const double _notchSize = HkSpace.xl;
  static const double _metaIconSize = HkSpace.xl;
  static const double _statusIconSize = HkSpace.lg;

  bool _pressed = false;

  @override
  Widget build(BuildContext context) {
    final reduceMotion =
        MediaQuery.maybeOf(context)?.disableAnimations ?? false;
    final textScale = MediaQuery.textScalerOf(context).scale(1);
    final visual = _CouponStatusVisual.from(widget.status);
    final benefit = _CouponBenefit.from(widget.discountLabel, widget.content);
    final semanticsLabel = _semanticsLabel(visual.label);

    return LayoutBuilder(
      builder: (context, constraints) {
        final compact = constraints.maxWidth < 328;
        final railWidth = compact ? _compactRailWidth : _railWidth;
        final baseMinHeight = compact ? 160.0 : 168.0;
        final extraTextHeight = ((textScale - 1).clamp(0, 1)) * 64;
        final minHeight = baseMinHeight + extraTextHeight;
        final duration = reduceMotion
            ? Duration.zero
            : const Duration(milliseconds: 140);

        return Semantics(
          container: true,
          button: widget.onPressed != null,
          enabled: widget.onPressed != null,
          label: semanticsLabel,
          hint: widget.onPressed == null ? null : '點兩下查看使用範圍與規則',
          onTap: widget.onPressed,
          excludeSemantics: true,
          child: AnimatedContainer(
            key: const ValueKey('coupon-ticket-surface'),
            duration: duration,
            curve: Curves.easeOutCubic,
            constraints: BoxConstraints(minHeight: minHeight),
            decoration: BoxDecoration(
              color: visual.surfaceColor,
              borderRadius: BorderRadius.circular(HkRadius.lg),
              border: Border.all(color: visual.borderColor),
              boxShadow: _pressed ? HkShadows.xs : HkShadows.card,
            ),
            child: Stack(
              clipBehavior: Clip.none,
              children: [
                Padding(
                  padding: EdgeInsets.only(right: railWidth),
                  child: Padding(
                    padding: const EdgeInsets.all(HkSpace.lg),
                    child: _CouponTicketContent(
                      title: widget.title,
                      content: widget.content,
                      rule: widget.rule,
                      expiry: widget.expiry,
                      issuerLabel: widget.issuerLabel,
                      issuerAvatar: SizedBox.square(
                        dimension: _avatarSize,
                        child: widget.issuerAvatar,
                      ),
                    ),
                  ),
                ),
                Positioned(
                  top: 0,
                  right: 0,
                  bottom: 0,
                  width: railWidth,
                  child: AnimatedContainer(
                    key: const ValueKey('coupon-ticket-value-rail'),
                    duration: duration,
                    curve: Curves.easeOutCubic,
                    padding: const EdgeInsets.symmetric(
                      horizontal: HkSpace.sm,
                      vertical: HkSpace.lg,
                    ),
                    decoration: BoxDecoration(
                      color: visual.railColor,
                      borderRadius: const BorderRadius.horizontal(
                        right: Radius.circular(HkRadius.lg),
                      ),
                    ),
                    child: _CouponValueRail(
                      statusLabel: visual.label,
                      statusIcon: visual.icon,
                      benefit: benefit,
                      valueColor: visual.valueColor,
                      compactValue:
                          compact ||
                          textScale >= 1.15 ||
                          benefit.value.length > 6,
                    ),
                  ),
                ),
                Positioned(
                  top: HkSpace.lg,
                  right: railWidth,
                  bottom: HkSpace.lg,
                  width: 1,
                  child: CustomPaint(
                    painter: _VerticalDashPainter(
                      color: AppColors.neutralCream.withValues(alpha: 0.42),
                    ),
                  ),
                ),
                Positioned.fill(
                  child: IgnorePointer(
                    child: AnimatedOpacity(
                      duration: duration,
                      curve: Curves.easeOutCubic,
                      opacity: _pressed ? 1 : 0,
                      child: const DecoratedBox(
                        decoration: BoxDecoration(
                          color: AppColors.primaryHover,
                          borderRadius: BorderRadius.all(
                            Radius.circular(HkRadius.lg),
                          ),
                        ),
                      ),
                    ),
                  ),
                ),
                Positioned.fill(
                  child: Material(
                    color: Colors.transparent,
                    borderRadius: BorderRadius.circular(HkRadius.lg),
                    child: InkWell(
                      onTap: widget.onPressed,
                      onHighlightChanged: widget.onPressed == null
                          ? null
                          : (highlighted) {
                              if (_pressed == highlighted) return;
                              setState(() => _pressed = highlighted);
                            },
                      mouseCursor: widget.onPressed == null
                          ? MouseCursor.defer
                          : SystemMouseCursors.click,
                      customBorder: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(HkRadius.lg),
                      ),
                      splashFactory: NoSplash.splashFactory,
                      overlayColor: const WidgetStatePropertyAll(
                        Colors.transparent,
                      ),
                    ),
                  ),
                ),
                Positioned(
                  top: -_notchSize / 2,
                  right: railWidth - _notchSize / 2,
                  child: const _TicketNotch(),
                ),
                Positioned(
                  bottom: -_notchSize / 2,
                  right: railWidth - _notchSize / 2,
                  child: const _TicketNotch(),
                ),
              ],
            ),
          ),
        );
      },
    );
  }

  String _semanticsLabel(String statusLabel) {
    final pieces = <String>[
      statusLabel,
      widget.title.trim(),
      if (widget.discountLabel.trim().isNotEmpty)
        widget.discountLabel.trim()
      else if (widget.content.trim().isNotEmpty)
        widget.content.trim(),
      widget.rule.trim(),
      widget.expiry.trim().isEmpty ? '無使用期限' : widget.expiry.trim(),
      widget.issuerLabel.trim(),
    ].where((piece) => piece.isNotEmpty);
    return pieces.join('，');
  }
}

class _CouponTicketContent extends StatelessWidget {
  const _CouponTicketContent({
    required this.title,
    required this.content,
    required this.rule,
    required this.expiry,
    required this.issuerLabel,
    required this.issuerAvatar,
  });

  final String title;
  final String content;
  final String rule;
  final String expiry;
  final String issuerLabel;
  final Widget issuerAvatar;

  @override
  Widget build(BuildContext context) {
    final ruleParts = _ruleParts(rule, fallback: content);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisAlignment: MainAxisAlignment.center,
      children: [
        Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            DecoratedBox(
              decoration: BoxDecoration(
                shape: BoxShape.circle,
                border: Border.all(color: AppColors.borderSoft),
              ),
              child: ClipOval(child: issuerAvatar),
            ),
            const SizedBox(width: HkSpace.md),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    title.trim().isEmpty ? '優惠券' : title.trim(),
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                    style: AppTypography.sectionTitle,
                  ),
                  const SizedBox(height: HkSpace.xs),
                  Text(
                    issuerLabel,
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                    style: AppTypography.caption.copyWith(
                      color: AppColors.textPrimary,
                    ),
                  ),
                ],
              ),
            ),
          ],
        ),
        const SizedBox(height: HkSpace.lg),
        Wrap(
          spacing: HkSpace.sm,
          runSpacing: HkSpace.sm,
          children: [
            for (final part in ruleParts) _CouponRuleChip(label: part),
          ],
        ),
        const SizedBox(height: HkSpace.lg),
        Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const Padding(
              padding: EdgeInsets.only(top: HkSpace.xs),
              child: HkIcon(
                HkIconName.clock,
                size: _WalletCouponTicketState._metaIconSize,
                color: AppIconColors.defaultColor,
              ),
            ),
            const SizedBox(width: HkSpace.sm),
            Expanded(
              child: Text(
                expiry.trim().isEmpty ? '無使用期限' : expiry.trim(),
                maxLines: 2,
                overflow: TextOverflow.ellipsis,
                style: AppTypography.meta.copyWith(
                  color: AppColors.textPrimary,
                ),
              ),
            ),
          ],
        ),
      ],
    );
  }

  static List<String> _ruleParts(String rule, {required String fallback}) {
    final normalized = rule.replaceFirst(RegExp(r'^[・•]\s*'), '').trim();
    final source = normalized.isEmpty ? fallback.trim() : normalized;
    if (source.isEmpty) return const ['請查看使用規則'];

    final parts = source
        .split(RegExp(r'\s{2,}|[・•]\s*'))
        .map((part) => part.trim())
        .where((part) => part.isNotEmpty)
        .toList(growable: false);
    return parts.isEmpty ? const ['請查看使用規則'] : parts;
  }
}

class _CouponRuleChip extends StatelessWidget {
  const _CouponRuleChip({required this.label});

  final String label;

  @override
  Widget build(BuildContext context) {
    return DecoratedBox(
      decoration: BoxDecoration(
        color: AppColors.primarySubtle,
        borderRadius: BorderRadius.circular(HkRadius.pill),
        border: Border.all(color: AppColors.borderSubtle),
      ),
      child: Padding(
        padding: const EdgeInsets.symmetric(
          horizontal: HkSpace.sm,
          vertical: HkSpace.xs,
        ),
        child: Text(
          label,
          style: AppTypography.caption.copyWith(
            color: AppColors.textPrimary,
            fontWeight: FontWeight.w500,
          ),
        ),
      ),
    );
  }
}

class _CouponValueRail extends StatelessWidget {
  const _CouponValueRail({
    required this.statusLabel,
    required this.statusIcon,
    required this.benefit,
    required this.valueColor,
    required this.compactValue,
  });

  final String statusLabel;
  final HkIconName statusIcon;
  final _CouponBenefit benefit;
  final Color valueColor;
  final bool compactValue;

  @override
  Widget build(BuildContext context) {
    final valueStyle =
        (compactValue ? AppTypography.sectionTitle : AppTypography.display)
            .copyWith(
              color: valueColor,
              height: 1.1,
              fontWeight: FontWeight.w900,
            );

    return Column(
      mainAxisAlignment: MainAxisAlignment.spaceBetween,
      children: [
        Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            HkIcon(
              statusIcon,
              size: _WalletCouponTicketState._statusIconSize,
              color: AppIconColors.onBrand,
            ),
            const SizedBox(width: HkSpace.xs),
            Flexible(
              child: Text(
                statusLabel,
                key: const ValueKey('coupon-ticket-status-label'),
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: AppTypography.caption.copyWith(
                  color: AppColors.textInverse,
                  fontWeight: FontWeight.w700,
                ),
              ),
            ),
          ],
        ),
        Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(
              benefit.kind,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: AppTypography.caption.copyWith(
                color: AppColors.textInverse,
                fontWeight: FontWeight.w700,
              ),
            ),
            if (benefit.unit.isNotEmpty) ...[
              const SizedBox(height: HkSpace.xs),
              Text(
                benefit.unit,
                style: AppTypography.caption.copyWith(
                  color: AppColors.textInverse,
                ),
              ),
            ],
            Text(
              benefit.value,
              key: const ValueKey('coupon-ticket-value'),
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
              textAlign: TextAlign.center,
              style: valueStyle,
            ),
          ],
        ),
        Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(
              '規則',
              style: AppTypography.caption.copyWith(
                color: AppColors.textInverse,
                fontWeight: FontWeight.w700,
              ),
            ),
            const SizedBox(width: HkSpace.xs),
            const HkIcon(
              HkIconName.chevronRight,
              size: _WalletCouponTicketState._statusIconSize,
              color: AppIconColors.onBrand,
            ),
          ],
        ),
      ],
    );
  }
}

final class _CouponBenefit {
  const _CouponBenefit({
    required this.kind,
    required this.unit,
    required this.value,
  });

  factory _CouponBenefit.from(String discountLabel, String fallback) {
    final label = discountLabel.trim();
    if (label.startsWith(r'NT$')) {
      return _CouponBenefit(
        kind: '折抵',
        unit: r'NT$',
        value: label.substring(3).trim(),
      );
    }
    if (label.startsWith(r'$')) {
      return _CouponBenefit(
        kind: '折抵',
        unit: r'NT$',
        value: label.substring(1).trim(),
      );
    }
    if (label.contains('折')) {
      return _CouponBenefit(kind: '折扣', unit: '', value: label);
    }
    final value = label.isNotEmpty ? label : fallback.trim();
    return _CouponBenefit(
      kind: '優惠',
      unit: '',
      value: value.isEmpty ? '專屬' : value,
    );
  }

  final String kind;
  final String unit;
  final String value;
}

final class _CouponStatusVisual {
  const _CouponStatusVisual({
    required this.label,
    required this.icon,
    required this.surfaceColor,
    required this.railColor,
    required this.valueColor,
    required this.borderColor,
  });

  factory _CouponStatusVisual.from(WalletCouponVisualStatus status) {
    return switch (status) {
      WalletCouponVisualStatus.available => const _CouponStatusVisual(
        label: '可使用',
        icon: HkIconName.tag,
        surfaceColor: AppColors.bgCard,
        railColor: AppColors.brandPrimaryStrong,
        valueColor: AppColors.brandSecondarySoft,
        borderColor: AppColors.borderSoft,
      ),
      WalletCouponVisualStatus.used => const _CouponStatusVisual(
        label: '已使用',
        icon: HkIconName.check,
        surfaceColor: AppColors.surfaceMuted,
        railColor: AppColors.textPrimary,
        valueColor: AppColors.textInverse,
        borderColor: AppColors.borderSubtle,
      ),
      WalletCouponVisualStatus.expired => const _CouponStatusVisual(
        label: '已過期',
        icon: HkIconName.clock,
        surfaceColor: AppColors.neutralWhisper,
        railColor: AppColors.textPrimary,
        valueColor: AppColors.textInverse,
        borderColor: AppColors.borderSubtle,
      ),
    };
  }

  final String label;
  final HkIconName icon;
  final Color surfaceColor;
  final Color railColor;
  final Color valueColor;
  final Color borderColor;
}

class _TicketNotch extends StatelessWidget {
  const _TicketNotch();

  @override
  Widget build(BuildContext context) {
    return const SizedBox.square(
      dimension: _WalletCouponTicketState._notchSize,
      child: DecoratedBox(
        decoration: BoxDecoration(
          color: AppColors.bgPrimary,
          shape: BoxShape.circle,
        ),
      ),
    );
  }
}

class _VerticalDashPainter extends CustomPainter {
  const _VerticalDashPainter({required this.color});

  final Color color;

  @override
  void paint(Canvas canvas, Size size) {
    final paint = Paint()
      ..color = color
      ..strokeWidth = 1
      ..strokeCap = StrokeCap.round;
    const dash = HkSpace.xs;
    const gap = HkSpace.xs;
    for (double y = 0; y < size.height; y += dash + gap) {
      canvas.drawLine(
        Offset(0.5, y),
        Offset(0.5, (y + dash).clamp(0, size.height)),
        paint,
      );
    }
  }

  @override
  bool shouldRepaint(covariant _VerticalDashPainter oldDelegate) {
    return oldDelegate.color != color;
  }
}
