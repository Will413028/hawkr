import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../../../../config/image_config.dart';
import '../../../../core/theme/app_colors.dart';
import '../../../../core/theme/app_typography.dart';
import '../../../../core/theme/hk_tokens.dart';
import '../../../../shared/widgets/hk_tap_target.dart';
import '../../../../shared/widgets/oss_image.dart';
import 'package:hawkr_buyer/shared/widgets/hk_placeholder.dart';

/// 首頁會員卡區（網站 light-mode 奶油白 + 淺品牌色雙層卡片）
class HomeMemberCardSection extends StatelessWidget {
  const HomeMemberCardSection({
    super.key,
    required this.userName,
    required this.couponCount,
    required this.memberCardCount,
    required this.merchantAvatars,
    required this.onCouponTap,
    required this.onMemberCardTap,
  });

  final String userName;
  final int couponCount;
  final int memberCardCount;
  final List<String> merchantAvatars;
  final VoidCallback onCouponTap;
  final VoidCallback onMemberCardTap;

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(
      builder: (context, constraints) {
        final gutter = HkBreakpoints.pageGutter(constraints.maxWidth);
        return Padding(
          padding: EdgeInsets.symmetric(horizontal: gutter),
          child: Center(
            child: ConstrainedBox(
              constraints: const BoxConstraints(
                maxWidth: HkControl.chromeMaxWidth,
              ),
              child: SizedBox(
                key: const ValueKey('home-member-card'),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    DecoratedBox(
                      decoration: const BoxDecoration(
                        color: AppColors.bgCard,
                        border: Border(
                          top: BorderSide(color: AppColors.cardBorder),
                          left: BorderSide(color: AppColors.cardBorder),
                          right: BorderSide(color: AppColors.cardBorder),
                        ),
                        borderRadius: BorderRadius.only(
                          topLeft: Radius.circular(HkRadius.lg),
                          topRight: Radius.circular(HkRadius.lg),
                        ),
                      ),
                      child: SizedBox(
                        height: HkControl.minTap,
                        child: Padding(
                          padding: const EdgeInsets.symmetric(
                            horizontal: HkSpace.md,
                          ),
                          child: LayoutBuilder(
                            builder: (context, rowConstraints) {
                              final couponWidth =
                                  (rowConstraints.maxWidth * 0.52)
                                      .clamp(112.0, 148.0)
                                      .toDouble();
                              return Row(
                                children: [
                                  HkTapTarget(
                                    semanticLabel: '編輯個人資料',
                                    hint: '開啟個人資料設定',
                                    tooltip: '編輯個人資料',
                                    onTap: () =>
                                        context.push('/settings/profile'),
                                    child: DecoratedBox(
                                      decoration: BoxDecoration(
                                        shape: BoxShape.circle,
                                        border: Border.all(
                                          color: AppColors.cardBorder,
                                        ),
                                      ),
                                      child: const ClipOval(
                                        child: SizedBox(
                                          width: 28,
                                          height: 28,
                                          child: HkPlaceholder(
                                            HkPlaceholderKind.buyerAvatar,
                                          ),
                                        ),
                                      ),
                                    ),
                                  ),
                                  const SizedBox(width: HkSpace.sm),
                                  Expanded(
                                    child: Text(
                                      'Hi, $userName',
                                      maxLines: 1,
                                      overflow: TextOverflow.ellipsis,
                                      style: AppTypography.productName.copyWith(
                                        color: AppColors.textEmphasis,
                                      ),
                                    ),
                                  ),
                                  const SizedBox(width: HkSpace.xs),
                                  HkTapTarget(
                                    semanticLabel: '查看可用優惠券，共 $couponCount 張',
                                    hint: '開啟優惠券列表',
                                    tooltip: '查看優惠券',
                                    onTap: onCouponTap,
                                    child: SizedBox(
                                      width: couponWidth,
                                      height: HkControl.minTap,
                                      child: Row(
                                        mainAxisAlignment:
                                            MainAxisAlignment.end,
                                        children: [
                                          Expanded(
                                            child: Text(
                                              '可用$couponCount張優惠券',
                                              maxLines: 1,
                                              overflow: TextOverflow.ellipsis,
                                              textAlign: TextAlign.end,
                                              style: AppTypography.meta
                                                  .copyWith(
                                                    color:
                                                        AppColors.textEmphasis,
                                                  ),
                                            ),
                                          ),
                                          const SizedBox(width: HkSpace.xs),
                                          const Icon(
                                            AppIcons.chevronRight,
                                            color: AppIconColors.active,
                                            size: 20,
                                          ),
                                        ],
                                      ),
                                    ),
                                  ),
                                ],
                              );
                            },
                          ),
                        ),
                      ),
                    ),
                    HkTapTarget(
                      semanticLabel: '查看會員卡，共 $memberCardCount 張',
                      hint: '開啟會員卡列表',
                      tooltip: '查看會員卡',
                      onTap: onMemberCardTap,
                      child: DecoratedBox(
                        decoration: const BoxDecoration(
                          color: AppColors.memberCardSurface,
                          border: Border(
                            bottom: BorderSide(color: AppColors.cardBorder),
                            left: BorderSide(color: AppColors.cardBorder),
                            right: BorderSide(color: AppColors.cardBorder),
                          ),
                          borderRadius: BorderRadius.only(
                            bottomLeft: Radius.circular(HkRadius.lg),
                            bottomRight: Radius.circular(HkRadius.lg),
                          ),
                        ),
                        child: SizedBox(
                          height: HkControl.minTap,
                          child: Padding(
                            padding: const EdgeInsets.symmetric(
                              horizontal: HkSpace.md,
                            ),
                            child: Row(
                              children: [
                                _buildMemberAvatars(),
                                const Spacer(),
                                Text(
                                  '$memberCardCount張會員卡',
                                  maxLines: 1,
                                  overflow: TextOverflow.ellipsis,
                                  style: AppTypography.meta.copyWith(
                                    color: AppColors.textEmphasis,
                                  ),
                                ),
                                const SizedBox(width: HkSpace.sm),
                                const Icon(
                                  AppIcons.chevronRight,
                                  color: AppIconColors.active,
                                  size: 20,
                                ),
                              ],
                            ),
                          ),
                        ),
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ),
        );
      },
    );
  }

  Widget _buildMemberAvatars() {
    final count = merchantAvatars.isNotEmpty
        ? merchantAvatars.length.clamp(0, 5)
        : memberCardCount.clamp(0, 5);
    if (count == 0) return const SizedBox.shrink();
    return SizedBox(
      width: _calculateAvatarsWidth(count),
      height: 24,
      child: Stack(
        children: merchantAvatars.isNotEmpty
            ? _buildAvatarStack()
            : _buildPlaceholderAvatarStack(count),
      ),
    );
  }

  double _calculateAvatarsWidth(int count) {
    return 24 + (count - 1) * 17.0;
  }

  List<Widget> _buildAvatarStack() {
    final count = merchantAvatars.length.clamp(0, 5);
    return List.generate(count, (i) {
      return Positioned(
        left: i * 17.0,
        child: Container(
          width: 24,
          height: 24,
          decoration: BoxDecoration(
            shape: BoxShape.circle,
            border: Border.all(color: AppColors.bgCard, width: 1),
            color: AppColors.bgCard,
          ),
          child: ClipOval(child: _buildMerchantAvatar(merchantAvatars[i])),
        ),
      );
    });
  }

  List<Widget> _buildPlaceholderAvatarStack(int count) {
    return List.generate(count, (i) {
      return Positioned(
        left: i * 17.0,
        child: Container(
          width: 24,
          height: 24,
          decoration: BoxDecoration(
            color: AppColors.gray300,
            shape: BoxShape.circle,
            border: Border.all(color: AppColors.bgCard),
          ),
          child: const Icon(
            AppIcons.store,
            size: 13,
            color: AppIconColors.muted,
          ),
        ),
      );
    });
  }

  Widget _buildMerchantAvatar(String avatarUrl) {
    final trimmed = avatarUrl.trim();
    if (trimmed.isEmpty) {
      return const SizedBox(
        width: 22,
        height: 22,
        child: HkPlaceholder(HkPlaceholderKind.merchantAvatar),
      );
    }
    return OssImage(
      imageSource: trimmed,
      scene: ImageScene.homeAvatar,
      width: 22,
      height: 22,
      fit: BoxFit.cover,
      errorWidget: const SizedBox(
        width: 22,
        height: 22,
        child: HkPlaceholder(HkPlaceholderKind.merchantAvatar),
      ),
    );
  }
}
