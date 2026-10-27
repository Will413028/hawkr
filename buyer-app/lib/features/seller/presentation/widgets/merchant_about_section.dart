import 'package:hawkr_buyer/core/motion/hk_motion_transitions.dart';
import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/app_typography.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:flutter/material.dart';

import '../../../../shared/widgets/hk_icon.dart';
import '../../../../shared/widgets/hk_tap_target.dart';
import 'merchant_common.dart';
import 'merchant_layout.dart';

const String merchantAboutEmptyLabel = '該商家沒有填寫';

/// Compact seller-description pane used by the persistent storefront header.
///
/// The website keeps this copy to a short summary. Long seller-authored text is
/// shown in a modal sheet so the persistent header stays compact while its
/// two-line summary can still grow with the user's text setting.
class MerchantAboutPanel extends StatelessWidget {
  const MerchantAboutPanel({
    super.key,
    required this.merchantName,
    this.avatarPayload,
    this.description,
  });

  final String merchantName;
  final String? avatarPayload;
  final String? description;

  @override
  Widget build(BuildContext context) {
    final resolvedDescription = description?.trim() ?? '';
    final hasDescription = resolvedDescription.isNotEmpty;
    final summaryStyle = AppTypography.metaMuted.copyWith(height: 1.5);

    return LayoutBuilder(
      builder: (context, constraints) {
        final textWidth = constraints.hasBoundedWidth
            ? (constraints.maxWidth - HkSpace.md * 2).clamp(
                1.0,
                double.infinity,
              )
            : 240.0;
        final painter = TextPainter(
          text: TextSpan(text: resolvedDescription, style: summaryStyle),
          textDirection: Directionality.of(context),
          textScaler: MediaQuery.textScalerOf(context),
          maxLines: 2,
        )..layout(maxWidth: textWidth);
        final canReadMore = hasDescription && painter.didExceedMaxLines;

        return DecoratedBox(
          key: const Key('merchant_about_panel'),
          decoration: const BoxDecoration(color: Colors.transparent),
          child: Padding(
            padding: const EdgeInsets.symmetric(
              horizontal: HkSpace.md,
              vertical: HkSpace.sm,
            ),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    const HkIcon(
                      HkIconName.store,
                      size: 18,
                      color: AppIconColors.active,
                    ),
                    const SizedBox(width: HkSpace.sm),
                    Expanded(
                      child: Text(
                        '商家介紹',
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: AppTypography.productName.copyWith(
                          color: AppColors.textEmphasis,
                        ),
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: HkSpace.xs),
                Text(
                  hasDescription
                      ? resolvedDescription
                      : merchantAboutEmptyLabel,
                  key: const Key('merchant_about_summary'),
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                  style: hasDescription
                      ? summaryStyle
                      : summaryStyle.copyWith(
                          color: AppColors.textSecondary.withValues(
                            alpha: 0.72,
                          ),
                        ),
                ),
                if (canReadMore) ...[
                  const SizedBox(height: HkSpace.xs),
                  Align(
                    alignment: Alignment.centerRight,
                    child: Semantics(
                      button: true,
                      label: '閱讀完整商家介紹',
                      child: ConstrainedBox(
                        constraints: const BoxConstraints(
                          minHeight: HkControl.minTap,
                        ),
                        child: Material(
                          type: MaterialType.transparency,
                          child: InkWell(
                            key: const Key('merchant_about_read_more'),
                            onTap: () => _showFullDescription(
                              context,
                              resolvedDescription,
                              avatarPayload,
                            ),
                            borderRadius: BorderRadius.circular(HkRadius.pill),
                            child: Padding(
                              padding: const EdgeInsets.symmetric(
                                horizontal: HkSpace.sm,
                              ),
                              child: Row(
                                mainAxisSize: MainAxisSize.min,
                                children: [
                                  Text(
                                    '閱讀更多',
                                    style: AppTypography.meta.copyWith(
                                      color: AppColors.actionPrimary,
                                      fontWeight: FontWeight.w700,
                                    ),
                                  ),
                                  const SizedBox(width: HkSpace.xs),
                                  const HkIcon(
                                    HkIconName.chevronRight,
                                    size: 16,
                                    color: AppIconColors.active,
                                  ),
                                ],
                              ),
                            ),
                          ),
                        ),
                      ),
                    ),
                  ),
                ],
              ],
            ),
          ),
        );
      },
    );
  }

  Future<void> _showFullDescription(
    BuildContext context,
    String resolvedDescription,
    String? avatarPayload,
  ) {
    final name = merchantName.trim();
    return showGeneralDialog<void>(
      context: context,
      barrierDismissible: true,
      barrierLabel: MaterialLocalizations.of(context).modalBarrierDismissLabel,
      barrierColor: AppColors.bgOverlay,
      transitionDuration: HkMotionTransitions.durationOf(context),
      pageBuilder: (dialogContext, animation, secondaryAnimation) =>
          _MerchantAboutBannerDialog(
            merchantName: name,
            avatarPayload: avatarPayload,
            description: resolvedDescription,
          ),
      transitionBuilder: (dialogContext, animation, secondaryAnimation, child) {
        return SafeArea(
          child: Padding(
            padding: const EdgeInsets.fromLTRB(
              HkSpace.md,
              HkSpace.md,
              HkSpace.md,
              HkSpace.xxl,
            ),
            child: Align(
              alignment: Alignment.topCenter,
              child: HkMotionTransitions.modal(
                context: dialogContext,
                animation: animation,
                presentation: HkModalPresentation.popup,
                alignment: Alignment.topCenter,
                child: child,
              ),
            ),
          ),
        );
      },
    );
  }
}

class _MerchantAboutBannerDialog extends StatelessWidget {
  const _MerchantAboutBannerDialog({
    required this.merchantName,
    required this.avatarPayload,
    required this.description,
  });

  final String merchantName;
  final String? avatarPayload;
  final String description;

  @override
  Widget build(BuildContext context) {
    final viewport = MediaQuery.sizeOf(context);
    final maxHeight = (viewport.height * 0.72).clamp(320.0, 680.0);

    return ConstrainedBox(
      constraints: BoxConstraints(
        maxWidth: MerchantLayoutSpec.profileContentMaxWidth,
        maxHeight: maxHeight,
      ),
      child: DecoratedBox(
        decoration: BoxDecoration(
          color: AppColors.neutralWhite,
          borderRadius: BorderRadius.circular(HkRadius.xl),
          boxShadow: HkShadows.lg,
        ),
        child: Material(
          key: const Key('merchant_about_banner'),
          color: AppColors.neutralWhite,
          surfaceTintColor: Colors.transparent,
          clipBehavior: Clip.antiAlias,
          borderRadius: BorderRadius.circular(HkRadius.xl),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Padding(
                padding: const EdgeInsets.fromLTRB(
                  HkSpace.xl,
                  HkSpace.lg,
                  HkSpace.md,
                  HkSpace.md,
                ),
                child: Row(
                  children: [
                    SizedBox.square(
                      key: const Key('merchant_about_avatar'),
                      dimension: 40,
                      child: ClipRRect(
                        borderRadius: BorderRadius.circular(HkRadius.md),
                        child: MerchantImageHelper.buildAvatarSmall(
                          avatarPayload,
                          fallback: ColoredBox(
                            color: AppColors.primarySoft,
                            child: const Center(
                              child: HkIcon(
                                HkIconName.store,
                                size: 20,
                                color: AppIconColors.active,
                              ),
                            ),
                          ),
                        ),
                      ),
                    ),
                    const SizedBox(width: HkSpace.md),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            'BRAND STORY',
                            style: AppTypography.caption.copyWith(
                              color: AppColors.actionPrimary,
                              fontWeight: FontWeight.w700,
                              letterSpacing: 1.8,
                            ),
                          ),
                          Text('商家介紹', style: AppTypography.sectionTitle),
                          if (merchantName.isNotEmpty)
                            Text(
                              merchantName,
                              maxLines: 1,
                              overflow: TextOverflow.ellipsis,
                              style: AppTypography.metaMuted,
                            ),
                        ],
                      ),
                    ),
                    HkTapTarget(
                      semanticLabel: '關閉商家介紹',
                      onTap: () => Navigator.of(context).pop(),
                      child: Container(
                        width: 36,
                        height: 36,
                        alignment: Alignment.center,
                        decoration: BoxDecoration(
                          color: AppColors.neutralWhite,
                          borderRadius: BorderRadius.circular(HkRadius.md),
                          border: Border.all(color: AppColors.storefrontBorder),
                        ),
                        child: const HkIcon(
                          HkIconName.close,
                          size: 18,
                          color: AppIconColors.strong,
                        ),
                      ),
                    ),
                  ],
                ),
              ),
              const Divider(height: 1, color: AppColors.storefrontBorder),
              Flexible(
                child: SingleChildScrollView(
                  padding: const EdgeInsets.fromLTRB(
                    HkSpace.xl,
                    HkSpace.lg,
                    HkSpace.xl,
                    HkSpace.xxl,
                  ),
                  child: SelectionArea(
                    child: Text(
                      description,
                      key: const Key('merchant_about_full_text'),
                      style: AppTypography.body.copyWith(height: 1.75),
                    ),
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

/// Standalone wrapper retained for previews and focused widget tests.
class MerchantAboutSection extends StatelessWidget {
  const MerchantAboutSection({
    super.key,
    required this.merchantName,
    required this.location,
    this.avatarPayload,
    this.description,
  });

  final String merchantName;
  final String? avatarPayload;

  /// Kept for source compatibility; empty descriptions no longer invent
  /// location-based marketing copy.
  final String location;
  final String? description;

  @override
  Widget build(BuildContext context) {
    final spec = MerchantLayoutSpec.of(context);
    final viewportWidth = MediaQuery.sizeOf(context).width;
    final sideInset = MerchantLayoutSpec.centeredSideInset(
      viewportWidth,
      baseInset: spec.pageHorizontalPadding,
      maxContentWidth: MerchantLayoutSpec.profileContentMaxWidth,
    );

    return Padding(
      padding: EdgeInsets.fromLTRB(
        sideInset,
        HkSpace.md,
        sideInset,
        HkSpace.md,
      ),
      child: KeyedSubtree(
        key: const Key('merchant_about_card'),
        child: MerchantAboutPanel(
          merchantName: merchantName,
          avatarPayload: avatarPayload,
          description: description,
        ),
      ),
    );
  }
}
