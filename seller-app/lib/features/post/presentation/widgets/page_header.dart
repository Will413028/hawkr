import 'dart:typed_data';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart' show SystemUiOverlayStyle;
import 'package:go_router/go_router.dart';

import '../../../../config/image_config.dart';
import '../../../../core/theme/app_colors.dart';
import '../../../../core/theme/app_typography.dart';
import '../../../../core/theme/hk_tokens.dart';
import '../../../../shared/icons/app_icon_colors.dart';
import '../../../../shared/widgets/hk_icon.dart';
import '../../../../shared/widgets/optimized_image.dart';
import '../../../../shared/widgets/seller_identity_avatar_frame.dart';
import '../../../../shared/widgets/storefront_top_bar_icon_button.dart';
import '../../../../shared/widgets/hk_placeholder.dart';

/// 計算圖片是否為亮色背景（用於決定狀態列文字顏色）。
bool isLightBackground(Uint8List bytes) {
  if (bytes.length < 100) return false;
  var sum = 0;
  final sampleSize = bytes.length > 1000 ? 1000 : bytes.length;
  for (var index = 0; index < sampleSize; index++) {
    sum += bytes[index];
  }
  return sum / sampleSize > 180;
}

/// 與買家 MerchantLayoutSpec 相同的店舖 Header 響應式尺寸。
@immutable
class PageHeaderLayoutSpec {
  const PageHeaderLayoutSpec._({
    required this.viewportWidth,
    required this.viewportHeight,
    required this.textScale,
    required this.textScaler,
    required this.textDirection,
  });

  factory PageHeaderLayoutSpec.of(BuildContext context) {
    final size = MediaQuery.sizeOf(context);
    return PageHeaderLayoutSpec._(
      viewportWidth: size.width,
      viewportHeight: size.height,
      textScale: MediaQuery.textScalerOf(context).scale(1).clamp(1, 2),
      textScaler: MediaQuery.textScalerOf(context),
      textDirection: Directionality.of(context),
    );
  }

  final double viewportWidth;
  final double viewportHeight;
  final double textScale;
  final TextScaler textScaler;
  final TextDirection textDirection;

  static const double topBarHeight = 64;
  static const double tabBarHeight = 52;
  static const double profileContentMaxWidth = 720;

  bool get isCompact => viewportWidth < HkBreakpoints.compact;
  bool get isWide => viewportWidth >= HkBreakpoints.tablet;
  bool get isShort => viewportHeight < 560;
  double get pageHorizontalPadding =>
      isCompact ? HkSpace.md : (isWide ? HkSpace.xxl : HkSpace.lg);
  double get avatarSize => isCompact ? 80 : (isWide ? 96 : 88);

  double get expandedHeroHeight {
    if (isShort) return 104;
    final maxHeight = isWide ? 148.0 : 124.0;
    return (viewportWidth * 0.28).clamp(104, maxHeight);
  }

  double profileDetailsPaneHeightFor({
    required bool hasAward,
    required String displayedLocation,
    required String description,
    String? awardText,
    int? fansCount,
  }) {
    final available = (viewportWidth - pageHorizontalPadding * 2).clamp(
      1.0,
      profileContentMaxWidth,
    );
    Size measure(String text, TextStyle style, {double? width, int? lines}) {
      final painter = TextPainter(
        text: TextSpan(text: text, style: style),
        textDirection: textDirection,
        textScaler: textScaler,
        maxLines: lines,
        ellipsis: lines == null ? null : '…',
      )..layout(maxWidth: width ?? double.infinity);
      final size = painter.size;
      painter.dispose();
      return size;
    }

    final meta = AppTypography.meta;
    final nameHeight = measure('店舖', AppTypography.h1, lines: 1).height;
    final locationHeight = measure(
      _shortStoreLocation(displayedLocation) ?? '尚未設定店舖位置',
      meta,
      width: available - 20,
      lines: 2,
    ).height.clamp(18.0, double.infinity);
    // Match the Wrap below, including the unchanged 16px icon + 4px gap.
    // Use the wider rating state so an async rating cannot grow the header.
    final ratingWidth = [
      measure('--', meta).width,
      measure('5.0', meta).width,
    ].reduce((a, b) => a > b ? a : b);
    final labels = <Size>[
      Size(ratingWidth, measure('5.0', meta).height),
      if (fansCount != null)
        measure('${_formatCount(fansCount)} 位訂閱者', meta, width: available - 20),
      if (hasAward)
        measure(awardText ?? '店舖獎項', meta, width: available - 20, lines: 1),
    ];
    var rowWidth = 0.0;
    var rowHeight = 0.0;
    var metadataHeight = 0.0;
    for (final label in labels) {
      final width = label.width + 20;
      if (rowWidth > 0 && rowWidth + HkSpace.md + width > available) {
        metadataHeight += rowHeight + HkSpace.xs;
        rowWidth = 0;
        rowHeight = 0;
      }
      rowWidth += (rowWidth > 0 ? HkSpace.md : 0) + width;
      final height = label.height.clamp(21.0, double.infinity);
      rowHeight = rowHeight > height ? rowHeight : height;
    }
    metadataHeight += rowHeight;
    final height =
        nameHeight + locationHeight + metadataHeight + HkSpace.xs * 3;
    return height.ceilToDouble().clamp(104.0, double.infinity);
  }

  double expandedAppBarHeightFor({
    required bool hasAward,
    required String displayedLocation,
    required String description,
    String? awardText,
    int? fansCount,
  }) {
    return expandedProfileHeightFor(
          hasAward: hasAward,
          displayedLocation: displayedLocation,
          description: description,
          awardText: awardText,
          fansCount: fansCount,
        ) +
        tabBarHeight;
  }

  /// 完整展開的店舖身份區高度，不含店舖頁底部 Tab。
  ///
  /// 設定頁直接使用這個高度與 [PageHeader]，避免兩頁各自維護封面、
  /// 頭像與資訊面板尺寸後再次產生偏差。
  double expandedProfileHeightFor({
    required bool hasAward,
    required String displayedLocation,
    required String description,
    String? awardText,
    int? fansCount,
  }) {
    return expandedHeroHeight +
        28 +
        profileDetailsPaneHeightFor(
          hasAward: hasAward,
          displayedLocation: displayedLocation,
          description: description,
          awardText: awardText,
          fansCount: fansCount,
        );
  }
}

/// 賣家自有店舖 Header。
///
/// 封面、置中頭像、店名、地點與公開數據直接對齊買家公開店舖頁；
/// 僅省略買家專屬的購物車與訂閱操作。
class PageHeader extends StatelessWidget {
  const PageHeader({
    super.key,
    this.coverUrl,
    required this.sellerName,
    this.fansCount,
    this.ratingAverage,
    this.reviewCount,
    this.location,
    this.prize,
    this.description,
    this.avatarUrl,
    this.title,
    this.onBackPressed,
    this.showNotifications = false,
    this.onNotifications,
    this.onChat,
    this.onPickCover,
    this.onPickAvatar,
    this.isUploadingAvatar = false,
  });

  final String? coverUrl;
  final String sellerName;

  /// 粉絲數；讀不到時是 null，不顯示（不當成 0）。
  final int? fansCount;

  /// 平均評分與評價數；沒有評價或讀不到時是 null。
  final double? ratingAverage;
  final int? reviewCount;
  final String? location;
  final String? prize;
  final String? description;
  final String? avatarUrl;
  final String? title;
  final VoidCallback? onBackPressed;
  final bool showNotifications;
  final VoidCallback? onNotifications;
  final VoidCallback? onChat;
  final VoidCallback? onPickCover;
  final VoidCallback? onPickAvatar;
  final bool isUploadingAvatar;

  @override
  Widget build(BuildContext context) {
    final spec = PageHeaderLayoutSpec.of(context);
    final settings = context
        .dependOnInheritedWidgetOfExactType<FlexibleSpaceBarSettings>();
    final minExtent = settings?.minExtent ?? 0;
    final maxExtent = settings?.maxExtent ?? 1;
    final currentExtent = settings?.currentExtent ?? maxExtent;
    final range = (maxExtent - minExtent).abs();
    final t = range < 0.001
        ? 1.0
        : ((currentExtent - minExtent) / range).clamp(0.0, 1.0);
    final safe = MediaQuery.viewPaddingOf(context);
    final bannerHeight =
        PageHeaderLayoutSpec.topBarHeight +
        (spec.expandedHeroHeight - PageHeaderLayoutSpec.topBarHeight) * t;
    final bannerBottom = safe.top + bannerHeight;
    final hasCustomCover = coverUrl?.trim().isNotEmpty == true;
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
          // Keep light status icons attached to the actual dark cover. The
          // settings header scrolls away; annotating the whole page would leave
          // white icons above the cream form after the cover is offscreen.
          child: AnnotatedRegion<SystemUiOverlayStyle>(
            key: const ValueKey('seller-store-banner-system-ui'),
            value: SystemUiOverlayStyle.light.copyWith(
              statusBarColor: Colors.transparent,
            ),
            child: ClipRRect(
              borderRadius: const BorderRadius.only(
                bottomLeft: Radius.circular(HkRadius.xxl),
              ),
              child: Stack(
                fit: StackFit.expand,
                children: [
                  _buildBannerImage(),
                  if (hasCustomCover)
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
                          maxWidth: PageHeaderLayoutSpec.profileContentMaxWidth,
                        ),
                        child: _buildExpandedInfo(spec),
                      ),
                    ),
                  ),
                ),
              ),
            ),
          ),
        if (onPickAvatar != null && expandedOpacity > 0.01)
          Positioned(
            top: bannerBottom - spec.avatarSize + HkSpace.xxl,
            left: 0,
            right: 0,
            height: spec.avatarSize,
            child: IgnorePointer(
              ignoring: t < 0.48 || isUploadingAvatar,
              child: Opacity(
                opacity: expandedOpacity,
                child: Center(
                  child: Semantics(
                    button: !isUploadingAvatar,
                    label: '更換店舖頭像',
                    child: SizedBox.square(
                      dimension: spec.avatarSize,
                      child: Material(
                        type: MaterialType.transparency,
                        shape: const CircleBorder(),
                        child: InkWell(
                          onTap: onPickAvatar,
                          customBorder: const CircleBorder(),
                        ),
                      ),
                    ),
                  ),
                ),
              ),
            ),
          ),
        if (collapsedOpacity > 0.01)
          Positioned(
            top: safe.top + HkSpace.sm,
            left: safe.left + (onBackPressed == null ? HkSpace.md : 56),
            right: safe.right + 56,
            height: HkControl.minTap,
            child: IgnorePointer(
              ignoring: t > 0.30,
              child: Opacity(
                opacity: collapsedOpacity,
                child: _buildCollapsedRow(),
              ),
            ),
          ),
        if (title?.trim().isNotEmpty == true && expandedOpacity > 0.01)
          Positioned(
            top: safe.top + HkSpace.sm,
            left: safe.left + 112,
            right: safe.right + 112,
            height: HkControl.minTap,
            child: IgnorePointer(
              child: Opacity(
                opacity: expandedOpacity,
                child: Center(
                  child: Text(
                    title!.trim(),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: AppTypography.h1.copyWith(
                      color: AppColors.textInverse,
                      shadows: const [
                        Shadow(
                          color: AppColors.neutralInk,
                          blurRadius: HkSpace.xs,
                        ),
                      ],
                    ),
                  ),
                ),
              ),
            ),
          ),
        if (onBackPressed != null)
          Positioned(
            top: safe.top + HkSpace.sm,
            left: safe.left + HkSpace.xs,
            child: StorefrontTopBarIconButton(
              semanticLabel: '返回',
              onPressed: onBackPressed,
              icon: HkIconName.back,
            ),
          ),
        if (onPickCover != null && expandedOpacity > 0.01)
          Positioned(
            top: safe.top + PageHeaderLayoutSpec.topBarHeight,
            right: safe.right + HkSpace.xs,
            child: IgnorePointer(
              ignoring: t < 0.48,
              child: Opacity(
                opacity: expandedOpacity,
                child: StorefrontTopBarIconButton(
                  semanticLabel: '更換封面',
                  onPressed: onPickCover,
                  icon: HkIconName.photoAdd,
                ),
              ),
            ),
          ),
        Positioned(
          top: safe.top + HkSpace.sm,
          right: safe.right + HkSpace.xs,
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              if (showNotifications)
                StorefrontTopBarIconButton(
                  semanticLabel: '通知中心',
                  onPressed: onNotifications,
                  icon: HkIconName.notification,
                ),
              StorefrontTopBarIconButton(
                semanticLabel: '聊天',
                onPressed: onChat ?? () => context.push('/chat'),
                icon: HkIconName.chat,
              ),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildBannerImage() {
    final resolvedCover = coverUrl?.trim() ?? '';
    if (resolvedCover.isNotEmpty) {
      return OptimizedNetworkImage(
        imageUrl: resolvedCover,
        scene: ImageScene.merchantBanner,
        fit: BoxFit.cover,
        alignment: Alignment.center,
        width: double.infinity,
        errorWidget: _buildDefaultBanner(),
      );
    }
    return _buildDefaultBanner();
  }

  Widget _buildDefaultBanner() {
    return const SizedBox(
      width: double.infinity,
      child: HkPlaceholder(HkPlaceholderKind.storeCover),
    );
  }

  Widget _buildExpandedInfo(PageHeaderLayoutSpec spec) {
    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        _buildAvatarArea(spec.avatarSize),
        const SizedBox(height: 2),
        _buildProfileDetails(spec),
      ],
    );
  }

  Widget _buildProfileDetails(PageHeaderLayoutSpec spec) {
    final displayedLocation = _shortLocation ?? '尚未設定店舖位置';
    final detailsHeight = spec.profileDetailsPaneHeightFor(
      hasAward: prize?.trim().isNotEmpty == true,
      displayedLocation: displayedLocation,
      description: description ?? '',
      awardText: prize,
      fansCount: fansCount,
    );
    return SizedBox(
      key: const Key('seller_store_centered_identity'),
      height: detailsHeight,
      child: _buildCenteredIdentity(displayedLocation),
    );
  }

  Widget _buildCenteredIdentity(String displayedLocation) {
    final award = prize?.trim() ?? '';
    return Column(
      mainAxisSize: MainAxisSize.min,
      crossAxisAlignment: CrossAxisAlignment.center,
      children: [
        Text(
          sellerName.trim().isEmpty ? '尚未設定店舖名稱' : sellerName.trim(),
          key: const Key('seller_store_name'),
          maxLines: 1,
          overflow: TextOverflow.ellipsis,
          textAlign: TextAlign.center,
          style: AppTypography.h1.copyWith(color: AppColors.textEmphasis),
        ),
        const SizedBox(height: HkSpace.xs),
        _StoreLocationRow(location: displayedLocation),
        const SizedBox(height: HkSpace.xs),
        Wrap(
          alignment: WrapAlignment.center,
          crossAxisAlignment: WrapCrossAlignment.center,
          spacing: HkSpace.md,
          runSpacing: HkSpace.xs,
          children: [
            _MetadataRow(
              icon: HkIconName.star,
              iconColor: AppIconColors.gold,
              child: _SellerRatingText(
                average: ratingAverage,
                count: reviewCount,
              ),
            ),
            if (fansCount != null)
              _MetadataRow(
                icon: HkIconName.users,
                iconColor: AppIconColors.active,
                child: Semantics(
                  label: '$fansCount 位訂閱者',
                  child: ExcludeSemantics(
                    child: Text(
                      '${_formatCount(fansCount!)} 位訂閱者',
                      style: AppTypography.meta,
                    ),
                  ),
                ),
              ),
            if (award.isNotEmpty)
              _MetadataRow(
                icon: HkIconName.award,
                iconColor: AppIconColors.gold,
                child: Text(
                  award,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: AppTypography.meta,
                ),
              ),
          ],
        ),
      ],
    );
  }

  Widget _buildAvatarArea(double avatarSize) {
    final avatar = SizedBox.square(
      dimension: avatarSize,
      child: Stack(
        clipBehavior: Clip.none,
        children: [
          SellerIdentityAvatarFrame(
            size: avatarSize,
            ringWidth: 0.5,
            imageUrl: avatarUrl,
            frameKey: const ValueKey('store-identity-avatar-frame'),
          ),
          if (onPickAvatar != null)
            Positioned(
              right: -1,
              bottom: -1,
              child: DecoratedBox(
                decoration: BoxDecoration(
                  color: AppColors.actionPrimary,
                  shape: BoxShape.circle,
                  border: Border.all(
                    color: AppColors.storefrontProfileSurface,
                    width: 2,
                  ),
                ),
                child: const SizedBox.square(
                  dimension: 28,
                  child: Center(
                    child: HkIcon(
                      HkIconName.photoAdd,
                      size: 15,
                      color: AppIconColors.onBrand,
                    ),
                  ),
                ),
              ),
            ),
          if (isUploadingAvatar)
            Positioned.fill(
              child: DecoratedBox(
                decoration: BoxDecoration(
                  shape: BoxShape.circle,
                  color: AppColors.neutralInk.withValues(alpha: 0.42),
                ),
                child: const Center(
                  child: SizedBox.square(
                    dimension: HkControl.icon,
                    child: CircularProgressIndicator(
                      strokeWidth: 2,
                      color: AppColors.textInverse,
                    ),
                  ),
                ),
              ),
            ),
        ],
      ),
    );

    return SizedBox(
      height: 26,
      width: double.infinity,
      child: Stack(
        clipBehavior: Clip.none,
        alignment: Alignment.topCenter,
        children: [Positioned(top: -(avatarSize - HkSpace.xxl), child: avatar)],
      ),
    );
  }

  Widget _buildCollapsedRow() {
    return Row(
      children: [
        SellerIdentityAvatarFrame(
          size: 36,
          ringWidth: 0.5,
          imageUrl: avatarUrl,
        ),
        const SizedBox(width: HkSpace.sm),
        Expanded(
          child: Text(
            sellerName,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            style: const TextStyle(
              color: AppColors.textInverse,
              fontSize: 16,
              fontWeight: FontWeight.w700,
              shadows: [
                Shadow(color: AppColors.neutralInk, blurRadius: HkSpace.xs),
              ],
            ),
          ),
        ),
      ],
    );
  }

  String? get _shortLocation => _shortStoreLocation(location);
}

String? _shortStoreLocation(String? location) {
  final text = location?.trim();
  return text == null || text.isEmpty ? null : text;
}

class _StoreLocationRow extends StatelessWidget {
  const _StoreLocationRow({required this.location});

  final String location;

  @override
  Widget build(BuildContext context) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      mainAxisAlignment: MainAxisAlignment.center,
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const Padding(
          padding: EdgeInsets.only(top: 2),
          child: HkIcon(
            HkIconName.location,
            size: 16,
            color: AppIconColors.active,
          ),
        ),
        const SizedBox(width: HkSpace.xs),
        Flexible(
          child: Text(
            location,
            maxLines: 2,
            overflow: TextOverflow.ellipsis,
            textAlign: TextAlign.center,
            style: AppTypography.meta,
          ),
        ),
      ],
    );
  }
}

class _MetadataRow extends StatelessWidget {
  const _MetadataRow({
    required this.icon,
    required this.iconColor,
    required this.child,
  });

  final HkIconName icon;
  final Color iconColor;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    return ConstrainedBox(
      constraints: const BoxConstraints(minHeight: 21),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Padding(
            padding: const EdgeInsets.only(top: 2),
            child: HkIcon(icon, size: 16, color: iconColor),
          ),
          const SizedBox(width: HkSpace.xs),
          Flexible(child: child),
        ],
      ),
    );
  }
}

class _SellerRatingText extends StatelessWidget {
  const _SellerRatingText({required this.average, required this.count});

  final double? average;
  final int? count;

  @override
  Widget build(BuildContext context) {
    final value = average?.clamp(0, 5).toDouble() ?? 0;
    final semanticLabel = value <= 0
        ? '尚無評分'
        : '${value.toStringAsFixed(1)} (${count ?? 0}則評價)';
    final compactLabel = value <= 0 ? '--' : value.toStringAsFixed(1);
    return Semantics(
      label: semanticLabel,
      child: ExcludeSemantics(
        child: Text(compactLabel, style: AppTypography.meta),
      ),
    );
  }
}

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
