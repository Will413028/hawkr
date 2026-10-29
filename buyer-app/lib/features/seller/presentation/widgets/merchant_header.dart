import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/app_typography.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import 'merchant_about_section.dart';
import 'merchant_common.dart';
import 'merchant_layout.dart';

String _formatCount(int count) {
  if (count >= 100000000) {
    return '${(count / 100000000).toStringAsFixed(1)}億';
  }
  if (count >= 10000) {
    return '${(count / 10000).toStringAsFixed(count >= 100000 ? 0 : 1)}萬';
  }
  if (count >= 1000) return '${(count / 1000).toStringAsFixed(1)}K';
  return count.toString();
}

/// 原商家頁的可折疊版型：Banner、置中頭像／商家資訊、右側訂閱鈕。
///
/// 僅將固定設計稿尺寸改為 viewport-aware logical pixels，保留原本視覺結構。
class MerchantHeader extends StatelessWidget {
  const MerchantHeader({
    super.key,
    this.bannerUrl,
    this.avatarPayload,
    this.merchantName,
    this.merchantAddress,
    this.merchantLocation,
    this.merchantDescription,
    this.merchantAward,
    this.rating,
    this.ratingCount,
    this.followerCount,
    this.isFollowing = false,
    this.isLoading = false,
    this.onToggleFollow,
    this.onChat,
    this.onCart,
  });

  final String? bannerUrl;
  final String? avatarPayload;
  final String? merchantName;
  final String? merchantAddress;
  final String? merchantLocation;
  final String? merchantDescription;
  final String? merchantAward;
  final double? rating;
  final int? ratingCount;
  final int? followerCount;
  final bool isFollowing;
  final bool isLoading;
  final VoidCallback? onToggleFollow;
  final VoidCallback? onChat;
  final VoidCallback? onCart;

  @override
  Widget build(BuildContext context) {
    final spec = MerchantLayoutSpec.of(context);
    final settings = context
        .dependOnInheritedWidgetOfExactType<FlexibleSpaceBarSettings>();
    if (settings == null) {
      return _buildNaturalHeader(context, spec);
    }
    final minExtent = settings.minExtent;
    final maxExtent = settings.maxExtent;
    final currentExtent = settings.currentExtent;
    final range = (maxExtent - minExtent).abs();
    final t = range < 0.001
        ? 1.0
        : ((currentExtent - minExtent) / range).clamp(0.0, 1.0);

    final safe = MediaQuery.viewPaddingOf(context);
    final hasCustomBanner = MerchantImageHelper.isCustomBanner(bannerUrl);
    final bannerHeight =
        MerchantLayoutSpec.topBarHeight +
        (spec.expandedHeroHeight - MerchantLayoutSpec.topBarHeight) * t;
    final bannerBottom = safe.top + bannerHeight;
    final expandedOpacity = const Interval(
      0.48,
      1,
      curve: Curves.easeOutCubic,
    ).transform(t);
    final collapsedOpacity = t >= 0.30
        ? 0.0
        : Curves.easeOut.transform(((0.30 - t) / 0.30).clamp(0.0, 1.0));

    return Stack(
      fit: StackFit.expand,
      clipBehavior: Clip.hardEdge,
      children: [
        Positioned(
          top: 0,
          left: 0,
          right: 0,
          height: bannerBottom,
          child: ClipRRect(
            borderRadius: const BorderRadius.only(
              bottomLeft: Radius.circular(32),
            ),
            child: Stack(
              fit: StackFit.expand,
              children: [
                MerchantImageHelper.buildBanner(bannerUrl),
                if (hasCustomBanner)
                  DecoratedBox(
                    decoration: BoxDecoration(
                      gradient: LinearGradient(
                        begin: Alignment.topCenter,
                        end: Alignment.bottomCenter,
                        colors: [
                          AppColors.neutralInk.withValues(alpha: 0.34),
                          AppColors.neutralInk.withValues(
                            alpha: 0.04 + (1 - t) * 0.30,
                          ),
                        ],
                      ),
                    ),
                  ),
              ],
            ),
          ),
        ),
        if (expandedOpacity > 0.01)
          Positioned(
            top: bannerBottom,
            left: 0,
            right: 0,
            child: IgnorePointer(
              ignoring: t < 0.48,
              child: Opacity(
                opacity: expandedOpacity,
                child: Transform.translate(
                  offset: Offset(0, 10 * (1 - expandedOpacity)),
                  child: Padding(
                    padding: EdgeInsets.symmetric(
                      horizontal: spec.pageHorizontalPadding,
                    ),
                    child: Center(
                      child: ConstrainedBox(
                        constraints: const BoxConstraints(
                          maxWidth: MerchantLayoutSpec.profileContentMaxWidth,
                        ),
                        child: _buildExpandedInfo(spec),
                      ),
                    ),
                  ),
                ),
              ),
            ),
          ),
        if (expandedOpacity > 0.01)
          Positioned(
            top: safe.top + MerchantLayoutSpec.topBarHeight,
            right: safe.right + 12,
            child: IgnorePointer(
              ignoring: t < 0.48,
              child: Opacity(
                opacity: expandedOpacity,
                child: _buildSubscribeButton(),
              ),
            ),
          ),
        if (collapsedOpacity > 0.01)
          Positioned(
            top: safe.top + 8,
            left: safe.left + 56,
            right: safe.right + 104,
            height: 48,
            child: IgnorePointer(
              ignoring: t > 0.30,
              child: Opacity(
                opacity: collapsedOpacity,
                child: _buildCollapsedRow(onCustomBanner: hasCustomBanner),
              ),
            ),
          ),
        Positioned(
          top: safe.top + 8,
          left: safe.left + 4,
          child: _TopBarIconButton(
            onTap: () => Navigator.of(context).maybePop(),
            icon: HkIconName.back,
            iconColor: AppIconColors.active,
            semanticLabel: '返回',
          ),
        ),
        Positioned(
          top: safe.top + 8,
          right: safe.right + 4,
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              _TopBarIconButton(
                onTap: onCart ?? () => context.push('/cart'),
                icon: HkIconName.cart,
                iconSize: HkControl.icon,
                semanticLabel: '購物車',
              ),
              _TopBarIconButton(
                onTap: onChat,
                icon: HkIconName.chat,
                iconSize: HkControl.icon,
                semanticLabel: '聊天',
              ),
            ],
          ),
        ),
      ],
    );
  }

  /// Natural-height storefront header used by the production sliver layout.
  ///
  /// Seller copy and live API data determine their own height. This avoids a
  /// manually estimated FlexibleSpaceBar extent changing after data arrives.
  Widget _buildNaturalHeader(BuildContext context, MerchantLayoutSpec spec) {
    final safe = MediaQuery.viewPaddingOf(context);
    final hasCustomBanner = MerchantImageHelper.isCustomBanner(bannerUrl);
    final heroHeight = safe.top + spec.expandedHeroHeight;

    return ColoredBox(
      key: const Key('merchant_natural_header'),
      color: AppColors.storefrontPageSurface,
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          SizedBox(
            key: const Key('merchant_natural_hero'),
            height: heroHeight,
            child: ClipRRect(
              borderRadius: const BorderRadius.only(
                bottomLeft: Radius.circular(32),
              ),
              child: Stack(
                fit: StackFit.expand,
                children: [
                  MerchantImageHelper.buildBanner(bannerUrl),
                  if (hasCustomBanner)
                    DecoratedBox(
                      decoration: BoxDecoration(
                        gradient: LinearGradient(
                          begin: Alignment.topCenter,
                          end: Alignment.bottomCenter,
                          colors: [
                            AppColors.neutralInk.withValues(alpha: 0.34),
                            AppColors.neutralInk.withValues(alpha: 0.04),
                          ],
                        ),
                      ),
                    ),
                  Positioned(
                    top: safe.top + 8,
                    left: safe.left + 4,
                    child: _TopBarIconButton(
                      onTap: () => Navigator.of(context).maybePop(),
                      icon: HkIconName.back,
                      iconColor: AppIconColors.active,
                      semanticLabel: '返回',
                    ),
                  ),
                  Positioned(
                    top: safe.top + 8,
                    right: safe.right + 4,
                    child: Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        _TopBarIconButton(
                          onTap: onCart ?? () => context.push('/cart'),
                          icon: HkIconName.cart,
                          iconSize: HkControl.icon,
                          semanticLabel: '購物車',
                        ),
                        _TopBarIconButton(
                          onTap: onChat,
                          icon: HkIconName.chat,
                          iconSize: HkControl.icon,
                          semanticLabel: '聊天',
                        ),
                      ],
                    ),
                  ),
                  Positioned(
                    top: safe.top + MerchantLayoutSpec.topBarHeight,
                    right: safe.right + 12,
                    child: _buildSubscribeButton(),
                  ),
                ],
              ),
            ),
          ),
          Padding(
            padding: EdgeInsets.symmetric(
              horizontal: spec.pageHorizontalPadding,
            ),
            child: Center(
              child: ConstrainedBox(
                constraints: const BoxConstraints(
                  maxWidth: MerchantLayoutSpec.profileContentMaxWidth,
                ),
                child: _buildExpandedInfo(spec),
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildExpandedInfo(MerchantLayoutSpec spec) {
    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        _buildAvatarArea(spec.avatarSize),
        const SizedBox(height: 2),
        _buildProfileDetails(spec),
      ],
    );
  }

  Widget _buildProfileDetails(MerchantLayoutSpec spec) {
    final hasAward = merchantAward?.trim().isNotEmpty == true;
    final displayedLocation = _displayedLocation.isEmpty
        ? '尚未設定店舖位置'
        : _displayedLocation;
    final minPaneHeight = spec.profileDetailsPaneHeightFor(
      hasAward: hasAward,
      displayedLocation: displayedLocation,
      description: merchantDescription ?? '',
    );
    final general = _buildGeneralInfoPanel();
    final about = MerchantAboutPanel(
      merchantName: merchantName?.trim() ?? '',
      avatarPayload: avatarPayload,
      description: merchantDescription,
    );

    if (spec.stacksProfileDetails) {
      return KeyedSubtree(
        key: const Key('merchant_profile_details_stacked'),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [general, const SizedBox(height: 4), about],
        ),
      );
    }

    return ConstrainedBox(
      key: const Key('merchant_profile_details_two_column'),
      constraints: BoxConstraints(minHeight: minPaneHeight),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Expanded(child: general),
          const SizedBox(width: HkSpace.sm),
          Expanded(child: about),
        ],
      ),
    );
  }

  Widget _buildGeneralInfoPanel() {
    final displayedLocation = _displayedLocation;
    final award = merchantAward?.trim() ?? '';
    return DecoratedBox(
      key: const Key('merchant_general_panel'),
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
            if (isLoading || (merchantName?.trim().isEmpty ?? true))
              const MerchantSkeletonBox(width: 112, height: 20)
            else
              Text(
                merchantName!.trim(),
                key: const Key('merchant_name_in_general_panel'),
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: AppTypography.productName.copyWith(
                  color: AppColors.textEmphasis,
                ),
              ),
            const SizedBox(height: HkSpace.xs),
            Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const Icon(
                  AppIcons.location,
                  size: 16,
                  color: AppIconColors.active,
                ),
                const SizedBox(width: HkSpace.xs),
                Expanded(
                  child: Text(
                    displayedLocation.isEmpty ? '尚未設定店舖位置' : displayedLocation,
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                    style: AppTypography.meta.copyWith(
                      color: displayedLocation.isEmpty
                          ? AppColors.textSecondary
                          : AppColors.textPrimary,
                    ),
                  ),
                ),
              ],
            ),
            const SizedBox(height: HkSpace.xs),
            Wrap(
              spacing: HkSpace.md,
              runSpacing: HkSpace.sm,
              children: [
                _MetaItem(
                  icon: AppIcons.star,
                  iconColor: AppIconColors.gold,
                  text: (rating ?? 0) <= 0
                      ? '尚無評分'
                      : '${rating!.toStringAsFixed(1)} (${ratingCount ?? 0}則評價)',
                ),
                _MetaItem(
                  icon: AppIcons.people,
                  iconColor: AppIconColors.active,
                  text: '${_formatCount(followerCount ?? 0)}位訂閱者',
                ),
                if (award.isNotEmpty)
                  _MetaItem(
                    semanticIcon: HkIconName.award,
                    iconColor: AppIconColors.gold,
                    text: award,
                  ),
              ],
            ),
          ],
        ),
      ),
    );
  }

  String get _displayedLocation {
    final location = merchantLocation?.trim() ?? '';
    if (location.isNotEmpty) return location;
    final address = merchantAddress?.trim() ?? '';
    return address;
  }

  Widget _buildAvatarArea(double avatarSize) {
    return SizedBox(
      height: 26,
      width: double.infinity,
      child: Stack(
        clipBehavior: Clip.none,
        alignment: Alignment.topCenter,
        children: [
          Positioned(
            top: -(avatarSize - 24),
            child: SizedBox.square(
              key: const Key('merchant_profile_avatar'),
              dimension: avatarSize,
              child: Stack(
                clipBehavior: Clip.none,
                children: [
                  Container(
                    key: const Key('merchant_profile_avatar_frame'),
                    padding: const EdgeInsets.all(0.5),
                    decoration: BoxDecoration(
                      shape: BoxShape.circle,
                      color: AppColors.storefrontProfileSurface,
                      border: Border.all(
                        color: AppColors.storefrontBorder,
                        width: 0.5,
                      ),
                      boxShadow: const [
                        BoxShadow(
                          color: AppColors.cardShadow,
                          blurRadius: 8,
                          offset: Offset(0, 4),
                        ),
                      ],
                    ),
                    child: ClipOval(
                      child: MerchantImageHelper.buildAvatarLarge(
                        avatarPayload,
                      ),
                    ),
                  ),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildSubscribeButton() {
    final background = isFollowing
        ? AppColors.storefrontProfileSurface.withValues(alpha: 0.94)
        : AppColors.actionPrimary;
    final foreground = isFollowing
        ? AppColors.actionPrimary
        : AppIconColors.onBrand;
    return Semantics(
      button: true,
      label: isFollowing ? '取消訂閱' : '訂閱店舖',
      child: SizedBox(
        height: 48,
        child: Material(
          type: MaterialType.transparency,
          child: InkWell(
            onTap: onToggleFollow,
            borderRadius: BorderRadius.circular(8),
            child: Center(
              child: Container(
                height: 36,
                padding: const EdgeInsets.symmetric(horizontal: 8),
                decoration: BoxDecoration(
                  color: background,
                  borderRadius: BorderRadius.circular(8),
                  border: Border.all(
                    color: isFollowing
                        ? AppColors.borderStrong
                        : AppColors.storefrontProfileSurface,
                  ),
                ),
                child: Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Icon(
                      isFollowing
                          ? AppIcons.verifiedOutlined
                          : AppIcons.personAddOutlined,
                      size: 16,
                      color: foreground,
                    ),
                    const SizedBox(width: 4),
                    Text(
                      isFollowing ? '已訂閱' : '訂閱',
                      style: TextStyle(
                        color: foreground,
                        fontSize: 12,
                        fontWeight: FontWeight.w700,
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildCollapsedRow({required bool onCustomBanner}) {
    final foreground = onCustomBanner
        ? AppColors.storefrontProfileSurface
        : AppColors.textEmphasis;
    return Row(
      children: [
        Container(
          width: 36,
          height: 36,
          padding: const EdgeInsets.all(4),
          decoration: BoxDecoration(
            shape: BoxShape.circle,
            color: AppColors.storefrontProfileSurface,
            border: Border.all(color: AppColors.storefrontBorder),
          ),
          child: ClipOval(
            child: MerchantImageHelper.buildAvatarSmall(avatarPayload),
          ),
        ),
        const SizedBox(width: 8),
        Expanded(
          child: Text(
            merchantName?.trim() ?? '',
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: TextStyle(
              color: foreground,
              fontSize: 16,
              fontWeight: FontWeight.w700,
              shadows: onCustomBanner
                  ? const [Shadow(color: AppColors.neutralInk, blurRadius: 4)]
                  : null,
            ),
          ),
        ),
      ],
    );
  }
}

class _MetaItem extends StatelessWidget {
  const _MetaItem({
    this.icon,
    this.semanticIcon,
    required this.iconColor,
    required this.text,
  }) : assert((icon == null) != (semanticIcon == null));

  final IconData? icon;
  final HkIconName? semanticIcon;
  final Color iconColor;
  final String text;

  @override
  Widget build(BuildContext context) {
    return ConstrainedBox(
      constraints: const BoxConstraints(minHeight: 21, maxWidth: 180),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          if (semanticIcon != null)
            HkIcon(semanticIcon!, size: 16, color: iconColor)
          else
            Icon(icon, size: 16, color: iconColor),
          const SizedBox(width: HkSpace.xs),
          Flexible(
            child: Text(
              text,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: AppTypography.meta,
            ),
          ),
        ],
      ),
    );
  }
}

class _TopBarIconButton extends StatelessWidget {
  const _TopBarIconButton({
    required this.semanticLabel,
    required this.icon,
    this.iconSize = 18,
    this.iconColor,
    this.onTap,
  });

  final HkIconName icon;
  final double iconSize;
  final Color? iconColor;
  final String semanticLabel;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    return Semantics(
      label: semanticLabel,
      button: true,
      enabled: onTap != null,
      onTap: onTap,
      child: SizedBox.square(
        dimension: 48,
        child: Material(
          type: MaterialType.transparency,
          child: InkWell(
            onTap: onTap,
            customBorder: const CircleBorder(),
            child: Center(
              child: Container(
                width: 32,
                height: 32,
                alignment: Alignment.center,
                decoration: BoxDecoration(
                  shape: BoxShape.circle,
                  color: AppColors.storefrontControlSurface,
                  border: Border.all(color: AppColors.borderSubtle),
                ),
                child: HkIcon(icon, size: iconSize, color: iconColor),
              ),
            ),
          ),
        ),
      ),
    );
  }
}
