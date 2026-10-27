import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:flutter_svg/flutter_svg.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';

import '../../config/image_config.dart';
import '../../core/theme/app_colors.dart';
import '../../core/theme/app_typography.dart';
import '../../core/theme/hk_tokens.dart';
import '../utils/price_display.dart';
import 'hk_liquid_glass.dart';
import 'oss_image.dart';
import 'product_card_hero.dart';

enum ProductFavoriteSurfaceStyle {
  /// 首頁等共用卡片使用的 36dp 清透玻璃圓面。
  elevated,

  /// 市集小卡使用的 33dp 緊湊清透玻璃圓面。
  hashtag,

  /// 只顯示愛心 icon。
  none,
}

/// 商品卡片 — 統一首頁 / Marketplace 共用元件。
///
/// 寬度與總高由父級控制；Web 4:3 圖片規格在一般字級下建議總高為
/// `cardWidth * 3 / 4 + 72`。文字放大時卡片會把少量高度讓給文字區。
///
/// 商家名稱與頭像**只**由呼叫端傳入（`sellerName` / `sellerAvatarUrl`）。
///
/// 卡片內部曾經在這兩個值為空時，用 `sellerAccountId` 逐張去打
/// `/public/sellers/{id}` 補撈——那是列表頁的 N+1（20 張卡＝最多 20 次請求），
/// 而且補不到東西：`/public/sellers/{id}` 回的是 `storeName` / `logoKey`，
/// 舊的 `Seller` model 讀的是 `name` / `avatar`，解析必定落空。
/// `/public/products`、`/public/marketplace`、分類列表三個來源都成對回
/// `sellerName` + `sellerLogoKey`，補撈路徑因此整條移除。
class ProductCard extends StatefulWidget {
  final String name;
  final String price;
  final String priceMax;
  // PayUni #758: 原價區間（刪除線）。當 listPriceMin/Max > priceMin/Max 才顯示。
  final String listPriceMin;
  final String listPriceMax;
  final String imageUrl;

  /// 商家名稱（由商品 payload 的 `sellerName` 直接傳入）。空字串不渲染商家列。
  final String sellerName;

  /// 商家頭像來源，objectKey 或完整 URL 皆可（`OssImage` dual-mode）。
  final String sellerAvatarUrl;

  /// hashtag 名稱（不含 `#` 前綴）。空 list 不渲染，卡片依 Web 規格最多顯示 2 個。
  /// v1 純裝飾，用 IgnorePointer 讓事件穿透給卡片 onTap。
  final List<String> hashtags;

  /// Sold-out products remain navigable, but the floating add action is
  /// disabled and the media is visually de-emphasised.
  final bool isSoldOut;

  /// 點擊整張卡片觸發（通常導到商品詳情）
  final VoidCallback? onTap;

  /// 與商品詳情頁共享的唯一 Hero tag。
  ///
  /// 同一個列表若可能重複出現相同商品，呼叫端需把列表 surface 納入 tag。
  final Object? heroTag;

  /// 點擊「購買」按鈕觸發。null 時退回 [onTap]。
  final VoidCallback? onAddToCartTap;

  /// Web 市集卡片右上角的收藏狀態與操作。
  final bool isFavorited;
  final VoidCallback? onFavoriteTap;

  /// 收藏 icon 的可見表面；不影響外層 48dp 點擊範圍。
  final ProductFavoriteSurfaceStyle favoriteSurfaceStyle;

  /// Accessibility scale used by both this card's text panel and the parent
  /// `ProductGridSpec`.
  ///
  /// A [TextScaler] may be nonlinear, so probing it with a one-pixel font does
  /// not describe the 16px product-name/price role that determines card height.
  /// Keeping this calculation here also prevents a grid delegate and its card
  /// children from reserving different vertical extents.
  static double effectiveTextScale(BuildContext context) {
    final fontSize = AppTypography.productName.fontSize!;
    return (MediaQuery.textScalerOf(context).scale(fontSize) / fontSize)
        .clamp(1.0, 1.4)
        .toDouble();
  }

  const ProductCard({
    super.key,
    required this.name,
    this.price = '',
    this.priceMax = '',
    this.listPriceMin = '',
    this.listPriceMax = '',
    this.imageUrl = '',
    this.sellerName = '',
    this.sellerAvatarUrl = '',
    this.hashtags = const [],
    this.isSoldOut = false,
    this.onTap,
    this.heroTag,
    this.onAddToCartTap,
    this.isFavorited = false,
    this.onFavoriteTap,
    this.favoriteSurfaceStyle = ProductFavoriteSurfaceStyle.elevated,
  });

  @override
  State<ProductCard> createState() => _ProductCardState();
}

class _ProductCardState extends State<ProductCard> {
  double _scale = 1.0;

  /// 售價區間（不含 `NT$` 前綴 —— 前綴由 build 另起一個 TextSpan 渲染）。
  ///
  /// 交給 [PriceDisplay.formatRangeRaw]，不自己 `num.tryParse`：後端契約以
  /// `"0"` 表達「無價格」，而 `num.tryParse("0")` 拿到 0 會印成 `NT$ 0`。
  /// 這張卡是市集 / 首頁推薦 / 分類三個列表共用的渲染點。
  String get _priceDisplay =>
      PriceDisplay.formatRangeRaw(widget.price, widget.priceMax);

  /// PayUni #758: 原價刪除線。只在 listPrice > price 時顯示。
  String get _listPriceDisplay => PriceDisplay.formatListPriceRange(
    widget.listPriceMin,
    widget.listPriceMax,
    effectivePrice: widget.price,
  );

  /// 與 Web 商品卡一致：清理 `#`、去重後最多呈現兩個標籤。
  /// 卡片空間有限，多於兩個時應在商品詳情呈現，不能在縮圖上硬裁半顆 chip。
  List<String> get _displayHashtags {
    final result = <String>[];
    final seen = <String>{};
    for (final raw in widget.hashtags) {
      final tag = raw.trim().replaceFirst(RegExp(r'^#+'), '');
      if (tag.isEmpty || !seen.add(tag.toLowerCase())) continue;
      result.add(tag);
      if (result.length == 2) break;
    }
    return result;
  }

  Icon _favoriteIcon({required double size, Key? key}) => Icon(
    widget.isFavorited ? AppIcons.favorite : AppIcons.favoriteBorder,
    key: key,
    size: size,
    color: widget.isFavorited ? AppIconColors.favorite : AppIconColors.active,
  );

  Widget _favoriteGlassSurface({
    required double dimension,
    required List<BoxShadow> shadows,
  }) => HkLiquidGlassSurface(
    key: const ValueKey('product-card-favorite-surface'),
    surfaceKey: const ValueKey('product-card-favorite-glass-fill'),
    variant: HkLiquidGlassVariant.clear,
    // Keep the clear material's compact blur and specular highlight, while a
    // real material tint keeps busy product photos from showing through too
    // strongly.
    surfaceOpacity: HkGlass.regularSurfaceOpacity,
    paintSurfaceColorBeneathGradient: true,
    width: dimension,
    height: dimension,
    borderRadius: BorderRadius.circular(HkRadius.pill),
    shadows: shadows,
    child: Center(child: _favoriteIcon(size: 19)),
  );

  Widget _favoriteVisual() => switch (widget.favoriteSurfaceStyle) {
    ProductFavoriteSurfaceStyle.elevated => _favoriteGlassSurface(
      dimension: 36,
      shadows: HkShadows.sm,
    ),
    ProductFavoriteSurfaceStyle.hashtag => _favoriteGlassSurface(
      dimension: 33,
      shadows: HkShadows.xs,
    ),
    ProductFavoriteSurfaceStyle.none => _favoriteIcon(
      key: const ValueKey('product-card-favorite-icon'),
      size: 22,
    ),
  };

  Widget _imagePlaceholder() => Container(
    color: AppColors.primarySoft,
    child: Center(
      child: Icon(
        AppIcons.imageOutlined,
        size: 36,
        color: AppIconColors.disabled,
      ),
    ),
  );

  @override
  Widget build(BuildContext context) {
    final pd = _priceDisplay;
    final displayHashtags = _displayHashtags;
    final compactMedia = MediaQuery.sizeOf(context).width <= 840;
    final cardRadius = compactMedia ? HkRadius.md : HkRadius.lg;
    // Keep component geometry in logical pixels. Only the text panel grows a
    // little with the system text scale so a wrapped original price does not
    // overflow at the app-supported 1.4x accessibility setting.
    final textScale = ProductCard.effectiveTextScale(context);
    final textPanelHeight = 72.0 + ((textScale - 1.0) * 20.0);
    final showListPrice =
        _listPriceDisplay.isNotEmpty &&
        MediaQuery.sizeOf(context).width >= HkBreakpoints.tablet;
    final reduceMotion =
        (MediaQuery.maybeDisableAnimationsOf(context) ?? false) ||
        View.of(context).platformDispatcher.accessibilityFeatures.reduceMotion;

    // 商家名稱 / 頭像一律由呼叫端給（三個列表 API 都成對回 sellerName +
    // sellerLogoKey）；此處不再有任何 per-card 網路補撈。
    final String resolvedSellerName = widget.sellerName;
    final String resolvedSellerAvatarUrl = widget.sellerAvatarUrl;

    final displayName = widget.name.trim().isEmpty ? '商品' : widget.name.trim();
    final cardSemanticLabel = <String>[
      '查看商品：$displayName',
      if (resolvedSellerName.isNotEmpty) '商家：$resolvedSellerName',
      if (pd.isNotEmpty) '價格：新台幣 $pd 元',
      if (widget.isSoldOut) '已售完',
    ].join('，');
    final addAction = widget.isSoldOut
        ? null
        : widget.onAddToCartTap ?? widget.onTap;
    final addSemanticLabel = widget.isSoldOut
        ? '商品已售完：$displayName'
        : widget.onAddToCartTap != null
        ? '加入購物車：$displayName'
        : '查看商品：$displayName';
    final baseProductImage = widget.imageUrl.isNotEmpty
        ? OssImage(
            imageSource: widget.imageUrl,
            scene: ImageScene.homeProductCard,
            fit: BoxFit.cover,
            errorWidget: _imagePlaceholder(),
          )
        : _imagePlaceholder();
    final productImage = widget.isSoldOut
        ? ColorFiltered(
            colorFilter: const ColorFilter.matrix(<double>[
              0.2126,
              0.7152,
              0.0722,
              0,
              0,
              0.2126,
              0.7152,
              0.0722,
              0,
              0,
              0.2126,
              0.7152,
              0.0722,
              0,
              0,
              0,
              0,
              0,
              1,
              0,
            ]),
            child: Opacity(opacity: 0.68, child: baseProductImage),
          )
        : baseProductImage;

    final card = AnimatedScale(
      scale: reduceMotion ? 1 : _scale,
      duration: reduceMotion
          ? Duration.zero
          : const Duration(milliseconds: 150),
      curve: HkMotion.ease,
      // 媒體、文字與操作共用單一外層卡面；不再為圖片或下方資訊加第二層卡片。
      child: Stack(
        clipBehavior: Clip.none,
        children: [
          Semantics(
            container: true,
            button: widget.onTap != null,
            label: cardSemanticLabel,
            onTap: widget.onTap,
            child: ExcludeSemantics(
              child: GestureDetector(
                behavior: HitTestBehavior.opaque,
                onTapDown: widget.onTap == null || reduceMotion
                    ? null
                    : (_) => setState(() => _scale = 0.97),
                onTapUp: widget.onTap == null || reduceMotion
                    ? null
                    : (_) => setState(() => _scale = 1.0),
                onTapCancel: widget.onTap == null || reduceMotion
                    ? null
                    : () => setState(() => _scale = 1.0),
                onTap: widget.onTap,
                child: Container(
                  key: const ValueKey('product-card-surface'),
                  decoration: BoxDecoration(
                    color: AppColors.bgCard,
                    borderRadius: BorderRadius.circular(cardRadius),
                    boxShadow: HkShadows.sm,
                  ),
                  foregroundDecoration: BoxDecoration(
                    borderRadius: BorderRadius.circular(cardRadius),
                    border: Border.all(color: AppColors.borderSubtle),
                  ),
                  clipBehavior: Clip.antiAlias,
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      // ── 4:3 媒體區：貼齊卡面，與下方資訊形成單一連續表面。──
                      AspectRatio(
                        key: const ValueKey('product-card-media'),
                        aspectRatio: 4 / 3,
                        child: Stack(
                          key: const ValueKey('product-card-image'),
                          fit: StackFit.expand,
                          children: [
                            ProductMediaHero(
                              tag: widget.heroTag,
                              child: ClipRSuperellipse(
                                key: const ValueKey('product-card-media-frame'),
                                borderRadius: BorderRadius.only(
                                  topLeft: Radius.circular(cardRadius),
                                  topRight: Radius.circular(cardRadius),
                                ),
                                child: Stack(
                                  fit: StackFit.expand,
                                  children: [productImage],
                                ),
                              ),
                            ),
                            if (widget.isSoldOut)
                              Positioned(
                                key: const ValueKey('product-card-sold-out'),
                                top: widget.onFavoriteTap == null ? 8 : 46,
                                right: 8,
                                child: DecoratedBox(
                                  decoration: BoxDecoration(
                                    color: AppColors.textEmphasis.withValues(
                                      alpha: 0.86,
                                    ),
                                    borderRadius: BorderRadius.circular(
                                      HkRadius.pill,
                                    ),
                                  ),
                                  child: const Padding(
                                    padding: EdgeInsets.symmetric(
                                      horizontal: 8,
                                      vertical: HkSpace.xs,
                                    ),
                                    child: Text(
                                      '已售完',
                                      style: TextStyle(
                                        color: AppColors.textInverse,
                                        fontSize: 12,
                                        height: 1,
                                        fontWeight: FontWeight.w700,
                                      ),
                                    ),
                                  ),
                                ),
                              ),
                            // ── Hashtag chips（Web parity：最多兩顆、空間不足往上換行）──
                            if (displayHashtags.isNotEmpty)
                              Positioned(
                                left: 8,
                                // 保留完整 48dp 給右側加購操作，兩者永不重疊。
                                right: 56,
                                bottom: 8,
                                child: IgnorePointer(
                                  key: const ValueKey(
                                    'product-card-hashtag-layer',
                                  ),
                                  child: Wrap(
                                    alignment: WrapAlignment.start,
                                    spacing: 4,
                                    runSpacing: 4,
                                    children: [
                                      for (
                                        var i = 0;
                                        i < displayHashtags.length;
                                        i++
                                      )
                                        _HashtagChip(
                                          key: ValueKey(
                                            'product-card-hashtag-$i',
                                          ),
                                          text: '#${displayHashtags[i]}',
                                        ),
                                    ],
                                  ),
                                ),
                              ),
                          ],
                        ),
                      ),
                      // ── 文字區（下方）──
                      SizedBox(
                        key: const ValueKey('product-card-body'),
                        height: textPanelHeight,
                        child: Padding(
                          padding: const EdgeInsets.symmetric(
                            horizontal: HkSpace.sm,
                          ),
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            // Four equal spaces: media→seller,
                            // seller→product, product→price and price→card.
                            // This remains proportional when accessibility
                            // text scaling changes each row's intrinsic height.
                            mainAxisAlignment: MainAxisAlignment.spaceEvenly,
                            children: [
                              // 商家資訊從圖片移到圖文分界，讓照片保持原色。
                              // 右側預留加購鈕跨界的 40dp 視覺面，避免大字級文字重疊。
                              if (resolvedSellerName.isNotEmpty) ...[
                                Padding(
                                  padding: const EdgeInsets.only(right: 44),
                                  child: Row(
                                    key: const ValueKey(
                                      'product-card-seller-row',
                                    ),
                                    children: [
                                      Container(
                                        key: const ValueKey(
                                          'product-card-seller-avatar',
                                        ),
                                        width: 14,
                                        height: 14,
                                        decoration: BoxDecoration(
                                          color: AppColors.primarySoft,
                                          shape: BoxShape.circle,
                                          border: Border.all(
                                            color: AppColors.borderSubtle,
                                            width: 0.5,
                                          ),
                                        ),
                                        clipBehavior: Clip.antiAlias,
                                        child:
                                            resolvedSellerAvatarUrl.isNotEmpty
                                            ? OssImage(
                                                imageSource:
                                                    resolvedSellerAvatarUrl,
                                                scene:
                                                    ImageScene.merchantAvatar,
                                                width: 14,
                                                height: 14,
                                                fit: BoxFit.cover,
                                              )
                                            : Center(
                                                child: SizedBox(
                                                  width: 8,
                                                  height: 8,
                                                  child: SvgPicture.asset(
                                                    'assets/brand/logo-grass.svg',
                                                    colorFilter:
                                                        const ColorFilter.mode(
                                                          AppColors.primary,
                                                          BlendMode.srcIn,
                                                        ),
                                                  ),
                                                ),
                                              ),
                                      ),
                                      const SizedBox(width: HkSpace.xs),
                                      Expanded(
                                        child: Text(
                                          resolvedSellerName,
                                          maxLines: 1,
                                          overflow: TextOverflow.ellipsis,
                                          style: AppTypography.meta.copyWith(
                                            color: AppColors.textSecondary,
                                            fontWeight: FontWeight.w500,
                                            height: 1,
                                          ),
                                        ),
                                      ),
                                    ],
                                  ),
                                ),
                              ],
                              SizedBox(
                                key: const ValueKey('product-card-title'),
                                width: double.infinity,
                                child: Text(
                                  displayName,
                                  style: AppTypography.meta.copyWith(
                                    color: AppColors.textEmphasis,
                                    fontWeight: FontWeight.w500,
                                    height: 1.2,
                                  ),
                                  maxLines: 1,
                                  overflow: TextOverflow.ellipsis,
                                ),
                              ),
                              if (pd.isNotEmpty)
                                SizedBox(
                                  key: const ValueKey('product-card-price-row'),
                                  width: double.infinity,
                                  child: Row(
                                    crossAxisAlignment: CrossAxisAlignment.end,
                                    children: [
                                      Flexible(
                                        // Narrow two-column cards must preserve
                                        // the current price range first.  The
                                        // struck-through list price is secondary
                                        // metadata and yields space before the
                                        // actionable price is truncated.
                                        flex: 4,
                                        child: Text.rich(
                                          TextSpan(
                                            children: [
                                              TextSpan(
                                                text: 'NT\$ ',
                                                style: AppTypography.caption
                                                    .copyWith(
                                                      color: AppColors.primary,
                                                      height: 1,
                                                    ),
                                              ),
                                              TextSpan(
                                                text: pd,
                                                style: AppTypography.productName
                                                    .copyWith(
                                                      color: AppColors.primary,
                                                      height: 1,
                                                    ),
                                              ),
                                            ],
                                          ),
                                          maxLines: 1,
                                          overflow: TextOverflow.ellipsis,
                                        ),
                                      ),
                                      // On a two-column phone card the full
                                      // active range is more important than
                                      // squeezed or ellipsized comparison
                                      // metadata. Wider layouts have room for
                                      // both values, matching the Web card.
                                      if (showListPrice) ...[
                                        const SizedBox(width: HkSpace.xs),
                                        Flexible(
                                          flex: 2,
                                          child: Text(
                                            // The active price already carries
                                            // the NT$ unit. Repeating a currency
                                            // symbol here wastes scarce width on
                                            // two-column cards and can turn a
                                            // complete range into an ellipsis.
                                            _listPriceDisplay,
                                            maxLines: 1,
                                            overflow: TextOverflow.ellipsis,
                                            style: AppTypography.caption
                                                .copyWith(
                                                  color:
                                                      AppColors.textSecondary,
                                                  height: 1,
                                                  decoration: TextDecoration
                                                      .lineThrough,
                                                  decorationColor:
                                                      AppColors.textSecondary,
                                                  decorationThickness: 1.2,
                                                ),
                                          ),
                                        ),
                                      ],
                                    ],
                                  ),
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
          ),
          if (widget.onFavoriteTap != null)
            Positioned(
              top: 0,
              right: 0,
              child: Semantics(
                button: true,
                enabled: true,
                toggled: widget.isFavorited,
                label: widget.isFavorited
                    ? '取消收藏 $displayName'
                    : '收藏 $displayName',
                onTap: widget.onFavoriteTap,
                child: ExcludeSemantics(
                  child: GestureDetector(
                    key: const ValueKey('product-card-favorite'),
                    behavior: HitTestBehavior.opaque,
                    onTap: widget.onFavoriteTap,
                    child: SizedBox.square(
                      dimension: HkControl.minTap,
                      child: Center(child: _favoriteVisual()),
                    ),
                  ),
                ),
              ),
            ),
          // 「+ 加購」的中心精準對齊圖文分界，但仍留在同一張卡面內。
          Positioned(
            right: 0,
            // 48dp 點擊範圍的中心在 seam：body 72dp - 半徑 24dp。
            bottom: textPanelHeight - (HkControl.minTap / 2),
            child: Semantics(
              button: true,
              enabled: !widget.isSoldOut && addAction != null,
              label: addSemanticLabel,
              onTap: addAction,
              child: ExcludeSemantics(
                child: GestureDetector(
                  key: const ValueKey('product-card-add-action'),
                  behavior: HitTestBehavior.opaque,
                  onTap: addAction,
                  child: SizedBox.square(
                    dimension: HkControl.minTap,
                    child: Center(
                      child: Container(
                        key: const ValueKey('product-card-add-surface'),
                        // Keep the floating control visually compact while the
                        // surrounding gesture target remains a full 48dp.
                        width: 40,
                        height: 40,
                        decoration: BoxDecoration(
                          color: widget.isSoldOut
                              ? AppColors.gray600
                              : AppColors.primary,
                          shape: BoxShape.circle,
                          border: Border.all(color: AppColors.bgCard, width: 2),
                          boxShadow: const [
                            BoxShadow(
                              color: AppColors.cardShadow,
                              blurRadius: 4,
                              offset: Offset(0, 2),
                            ),
                          ],
                        ),
                        child: const Center(
                          child: ExcludeSemantics(
                            child: Icon(
                              AppIcons.add,
                              size: 18,
                              color: AppIconColors.onBrand,
                            ),
                          ),
                        ),
                      ),
                    ),
                  ),
                ),
              ),
            ),
          ),
        ],
      ),
    );
    return card;
  }
}

/// 圖片上的 hashtag chip。尺寸由文字內容決定，避免短標籤被撐成等寬膠囊。
class _HashtagChip extends StatelessWidget {
  const _HashtagChip({super.key, required this.text});
  final String text;

  @override
  Widget build(BuildContext context) {
    return Container(
      constraints: const BoxConstraints(minHeight: 22),
      padding: const EdgeInsets.symmetric(horizontal: 8),
      decoration: BoxDecoration(
        color: AppColors.bgCard,
        borderRadius: BorderRadius.circular(HkRadius.pill),
        border: Border.all(color: AppColors.borderSubtle, width: 0.5),
        boxShadow: HkShadows.xs,
      ),
      child: Center(
        widthFactor: 1,
        heightFactor: 1,
        child: Text(
          text,
          maxLines: 1,
          overflow: TextOverflow.ellipsis,
          softWrap: false,
          style: AppTypography.caption.copyWith(
            color: AppColors.brandPrimaryStrong,
            fontWeight: FontWeight.w700,
            height: 1,
          ),
        ),
      ),
    );
  }
}
