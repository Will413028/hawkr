import 'package:hawkr_buyer/config/image_config.dart';
import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/app_typography.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:hawkr_buyer/shared/widgets/hk_liquid_glass.dart';
import 'package:hawkr_buyer/shared/widgets/oss_image.dart';
import 'package:hawkr_buyer/shared/widgets/quantity_stepper.dart';
import 'package:flutter/material.dart';
import 'package:hawkr_buyer/shared/widgets/hk_brand_loader.dart';

/// Converts the raw `skus` payload into the one shape shared by the product
/// detail page and the quick-add sheet.
///
/// 欄名以 gateway `SkuItem` 為準：`id` / `skuName` / `price` / `salePrice` /
/// `stockQty` / `unit`，全部 camelCase。這裡曾同時搬運 `sale_price` 那份
/// snake 拼寫，但後端從來沒有吐過它 —— 留著只會讓下一個人以為它可能出現。
List<Map<String, dynamic>> normalizeProductPurchaseSpecs(Object? rawSkus) {
  if (rawSkus is! List) return const <Map<String, dynamic>>[];

  return rawSkus
      .whereType<Map>()
      .map((raw) {
        final sku = Map<String, dynamic>.from(raw);
        final normalized = <String, dynamic>{
          'id': (sku['id'] ?? sku['skuId'] ?? '').toString(),
          'name': (sku['skuName'] ?? sku['name'] ?? '規格').toString(),
          'price': sku['price'] ?? sku['amount'] ?? sku['value'],
          'quantity':
              sku['stockQty'] ??
              sku['quantity'] ??
              sku['stock'] ??
              sku['inventory'] ??
              0,
          'unit': (sku['unit'] ?? '').toString(),
        };

        if (sku.containsKey('salePrice')) {
          normalized['salePrice'] = sku['salePrice'];
        }
        return normalized;
      })
      .toList(growable: false);
}

/// Reads stock from either a normalized spec or a raw API spec.
int purchaseSpecStock(Map<dynamic, dynamic>? spec) {
  if (spec == null) return 0;
  final raw =
      spec['quantity'] ??
      spec['stockQty'] ??
      spec['stock'] ??
      spec['inventory'];
  final parsed = raw is num
      ? raw.toInt()
      : int.tryParse(raw?.toString().trim() ?? '') ?? 0;
  return parsed < 0 ? 0 : parsed;
}

/// Returns the first purchasable spec, or `null` when every spec is sold out.
Map<String, dynamic>? firstAvailablePurchaseSpec(List<Map> specs) {
  for (final spec in specs) {
    if (purchaseSpecStock(spec) > 0) {
      return Map<String, dynamic>.from(spec);
    }
  }
  return null;
}

/// The canonical delivery row used by every product-purchase surface.
///
/// Carrier marks share one natural-height [Wrap], so multiple delivery
/// methods never get clipped.
class ProductDeliveryMetadata extends StatelessWidget {
  const ProductDeliveryMetadata({super.key, required this.shippingMethodCodes});

  final List<String> shippingMethodCodes;

  @override
  Widget build(BuildContext context) {
    return Wrap(
      key: const ValueKey('product-purchase-delivery-metadata'),
      spacing: HkSpace.sm,
      runSpacing: HkSpace.xs,
      crossAxisAlignment: WrapCrossAlignment.center,
      children: [
        Text(
          '配送方式',
          key: const ValueKey('product-purchase-delivery-label'),
          style: AppTypography.meta,
        ),
        for (final code in shippingMethodCodes)
          ShippingMethodMark(
            key: ValueKey('product-purchase-carrier-$code'),
            code: code,
          ),
      ],
    );
  }
}

/// Exact 24px carrier mark shared by the quick-add sheet and product detail.
class ShippingMethodMark extends StatelessWidget {
  const ShippingMethodMark({super.key, required this.code});

  final String code;

  static const Map<String, String> _labelByCode = {'3001': '宅配', '3002': '店到店'};

  @override
  Widget build(BuildContext context) {
    return Semantics(
      image: true,
      label: _labelByCode[code] ?? '配送方式',
      child: Container(
        width: 24,
        height: 24,
        decoration: BoxDecoration(
          color: AppColors.bgCard,
          borderRadius: BorderRadius.circular(HkRadius.xs),
          border: Border.all(color: AppColors.cardBorder),
        ),
        clipBehavior: Clip.antiAlias,
        child: const Center(
          child: HkIcon(HkIconName.truck, size: 20, color: AppIconColors.muted),
        ),
      ),
    );
  }
}

/// Product identity, delivery metadata and rating used above the purchase
/// controls. It deliberately owns no loading or selection state.
class ProductPurchaseSummaryCard extends StatelessWidget {
  const ProductPurchaseSummaryCard({
    super.key,
    required this.productName,
    required this.imageUrl,
    required this.rating,
    required this.reviewCount,
    required this.shippingMethodCodes,
    required this.loading,
  });

  final String productName;
  final String imageUrl;
  final double rating;
  final int reviewCount;
  final List<String> shippingMethodCodes;
  final bool loading;

  @override
  Widget build(BuildContext context) {
    return Container(
      key: const ValueKey('product-purchase-summary'),
      padding: const EdgeInsets.all(HkSpace.md),
      decoration: BoxDecoration(
        color: AppColors.bgCard,
        borderRadius: BorderRadius.circular(HkRadius.lg),
        border: Border.all(color: AppColors.cardBorder),
        boxShadow: HkShadows.sm,
      ),
      child: LayoutBuilder(
        builder: (context, constraints) {
          final imageExtent = constraints.maxWidth < 300 ? 80.0 : 88.0;
          return Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Container(
                key: const ValueKey('product-purchase-summary-image'),
                width: imageExtent,
                height: imageExtent,
                decoration: BoxDecoration(
                  color: AppColors.primarySoft,
                  borderRadius: BorderRadius.circular(HkRadius.sm),
                ),
                clipBehavior: Clip.antiAlias,
                child: imageUrl.isNotEmpty
                    ? OssImage(
                        imageSource: imageUrl,
                        scene: ImageScene.orderItem,
                        fit: BoxFit.cover,
                        width: imageExtent,
                        height: imageExtent,
                      )
                    : const Icon(
                        AppIcons.imageOutlined,
                        size: 28,
                        color: AppIconColors.muted,
                      ),
              ),
              const SizedBox(width: HkSpace.md),
              Expanded(
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      productName.trim().isEmpty ? '商品' : productName,
                      key: const ValueKey('product-purchase-summary-name'),
                      style: AppTypography.productName,
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                    ),
                    const SizedBox(height: HkSpace.xs),
                    if (loading)
                      const _PurchaseSkeleton(
                        width: 130,
                        height: 22,
                        radius: HkRadius.pill,
                      )
                    else
                      ProductDeliveryMetadata(
                        shippingMethodCodes: shippingMethodCodes,
                      ),
                    const SizedBox(height: HkSpace.xs),
                    if (loading)
                      const _PurchaseSkeleton(width: 100, height: 18)
                    else
                      _ProductRatingMetadata(
                        rating: rating,
                        reviewCount: reviewCount,
                      ),
                  ],
                ),
              ),
            ],
          );
        },
      ),
    );
  }
}

class _ProductRatingMetadata extends StatelessWidget {
  const _ProductRatingMetadata({
    required this.rating,
    required this.reviewCount,
  });

  final double rating;
  final int reviewCount;

  @override
  Widget build(BuildContext context) {
    return Row(
      key: const ValueKey('product-purchase-rating'),
      mainAxisSize: MainAxisSize.min,
      children: [
        Icon(
          AppIcons.starRounded,
          size: 16,
          color: rating > 0 ? AppIconColors.gold : AppIconColors.disabled,
        ),
        const SizedBox(width: HkSpace.xs),
        Flexible(
          child: Text(
            rating > 0
                ? '${rating.toStringAsFixed(1)}${reviewCount > 0 ? ' ($reviewCount則評價)' : ''}'
                : '暫無評價',
            maxLines: 2,
            overflow: TextOverflow.ellipsis,
            style: rating > 0 ? AppTypography.meta : AppTypography.metaMuted,
          ),
        ),
      ],
    );
  }
}

/// Canonical spec, price and quantity content.
///
/// The parent remains the single source of truth and receives every selection
/// or quantity event through callbacks.
class ProductPurchaseSelector extends StatelessWidget {
  const ProductPurchaseSelector({
    super.key,
    required this.specs,
    required this.selectedSpecId,
    required this.quantity,
    required this.currentPrice,
    required this.currentListPrice,
    required this.loading,
    required this.onSpecSelected,
    required this.onQuantityChanged,
  });

  final List<Map<String, dynamic>> specs;
  final String? selectedSpecId;
  final int quantity;
  final num? currentPrice;
  final num? currentListPrice;
  final bool loading;
  final ValueChanged<Map<String, dynamic>> onSpecSelected;
  final ValueChanged<int> onQuantityChanged;

  Map<String, dynamic>? get _selectedSpec {
    for (final spec in specs) {
      if ((spec['id'] ?? '').toString() == selectedSpecId) return spec;
    }
    return null;
  }

  @override
  Widget build(BuildContext context) {
    return Column(
      key: const ValueKey('product-purchase-selector'),
      mainAxisSize: MainAxisSize.min,
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('選擇規格', style: AppTypography.sectionTitle),
        const SizedBox(height: HkSpace.sm),
        if (loading)
          const _SpecSkeleton()
        else if (specs.isEmpty)
          Text('此商品暫無規格資訊', style: AppTypography.metaMuted)
        else
          Wrap(
            key: const ValueKey('product-purchase-specs'),
            spacing: HkSpace.sm,
            runSpacing: HkSpace.sm,
            children: [for (final spec in specs) _buildSpec(spec)],
          ),
        const SizedBox(height: HkSpace.sm),
        if (loading)
          const _PurchaseSkeleton(
            width: double.infinity,
            height: 56,
            radius: HkRadius.md,
          )
        else
          _buildPriceAndQuantity(),
      ],
    );
  }

  Widget _buildSpec(Map<String, dynamic> spec) {
    final id = (spec['id'] ?? '').toString();
    final name = (spec['name'] ?? '').toString().trim();
    final selected = id == selectedSpecId;
    final soldOut = purchaseSpecStock(spec) <= 0;

    return Semantics(
      button: true,
      enabled: !soldOut,
      selected: selected,
      label: name.isEmpty ? id : name,
      child: ExcludeSemantics(
        child: GestureDetector(
          behavior: HitTestBehavior.opaque,
          onTap: soldOut ? null : () => onSpecSelected(spec),
          child: ConstrainedBox(
            key: ValueKey('product-purchase-spec-target-$id'),
            constraints: const BoxConstraints(
              minWidth: 96,
              minHeight: HkControl.minTap,
            ),
            child: Align(
              widthFactor: 1,
              heightFactor: 1,
              alignment: Alignment.center,
              child: AnimatedContainer(
                key: ValueKey('product-purchase-spec-$id'),
                duration: HkMotion.duration,
                curve: HkMotion.ease,
                padding: const EdgeInsets.symmetric(
                  horizontal: HkSpace.xl,
                  vertical: HkSpace.sm,
                ),
                decoration: BoxDecoration(
                  color: soldOut
                      ? AppColors.gray100
                      : selected
                      ? AppColors.primarySoft
                      : AppColors.surfacePanel,
                  borderRadius: BorderRadius.circular(HkRadius.pill),
                  border: Border.all(
                    color: soldOut
                        ? AppColors.dividerLight
                        : selected
                        ? AppColors.primary
                        : AppColors.cardBorder,
                    width: 1.5,
                  ),
                ),
                child: Text(
                  name.isEmpty ? id : name,
                  textAlign: TextAlign.center,
                  style: AppTypography.meta.copyWith(
                    height: 1.2,
                    fontWeight: selected ? FontWeight.w700 : FontWeight.w400,
                    color: soldOut ? AppColors.textDisabled : AppColors.primary,
                    decoration: soldOut
                        ? TextDecoration.lineThrough
                        : TextDecoration.none,
                  ),
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildPriceAndQuantity() {
    final selectedSpec = _selectedSpec;
    final stock = purchaseSpecStock(selectedSpec);
    final displayQuantity = quantity < 1 ? 1 : quantity;
    final total = currentPrice == null ? null : currentPrice! * displayQuantity;
    final listTotal = currentListPrice == null
        ? null
        : currentListPrice! * displayQuantity;

    final priceBlock = Column(
      key: const ValueKey('product-purchase-price'),
      mainAxisSize: MainAxisSize.min,
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text.rich(
          TextSpan(
            children: [
              TextSpan(text: 'NT\$ ', style: AppTypography.priceUnit),
              TextSpan(
                text: total == null ? '--' : _formatMoney(total),
                style: AppTypography.priceTotal.copyWith(
                  fontSize: 20,
                  height: 1,
                ),
              ),
              if (listTotal != null && total != null && listTotal > total) ...[
                TextSpan(text: '  ', style: AppTypography.caption),
                TextSpan(
                  text: _formatMoney(listTotal),
                  style: AppTypography.caption.copyWith(
                    color: AppColors.textSecondary,
                    decoration: TextDecoration.lineThrough,
                    decorationColor: AppColors.textSecondary,
                  ),
                ),
              ],
            ],
          ),
        ),
        if (currentPrice != null && displayQuantity > 1) ...[
          const SizedBox(height: HkSpace.xs),
          Text(
            '單價 NT\$ ${_formatMoney(currentPrice!)} × $displayQuantity',
            style: AppTypography.metaMuted,
          ),
        ],
      ],
    );

    final quantityBlock = Row(
      key: const ValueKey('product-purchase-quantity'),
      mainAxisSize: MainAxisSize.min,
      crossAxisAlignment: CrossAxisAlignment.center,
      children: [
        Text('數量', style: AppTypography.body),
        const SizedBox(width: HkSpace.sm),
        QuantityStepper(
          count: displayQuantity,
          minCount: 1,
          maxCount: stock > 0 ? stock : 1,
          disabled: selectedSpec == null || stock <= 0,
          reversed: false,
          onIncrement: () => onQuantityChanged(displayQuantity + 1),
          onDecrement: () => onQuantityChanged(displayQuantity - 1),
        ),
      ],
    );

    return LayoutBuilder(
      builder: (context, constraints) {
        if (constraints.maxWidth < 292) {
          return Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              priceBlock,
              const SizedBox(height: HkSpace.sm),
              Align(alignment: Alignment.centerRight, child: quantityBlock),
            ],
          );
        }
        return Row(
          crossAxisAlignment: CrossAxisAlignment.center,
          children: [
            Expanded(child: priceBlock),
            const SizedBox(width: HkSpace.sm),
            quantityBlock,
          ],
        );
      },
    );
  }

  static String _formatMoney(num value) {
    final asDouble = value.toDouble();
    return asDouble == asDouble.roundToDouble()
        ? asDouble.toInt().toString()
        : asDouble.toStringAsFixed(2);
  }
}

/// Canonical stock row and purchase CTAs. The parent owns the actual actions.
class ProductPurchaseFooter extends StatelessWidget {
  const ProductPurchaseFooter({
    super.key,
    required this.hasSelection,
    required this.stock,
    required this.enabled,
    required this.addLoading,
    required this.buyLoading,
    required this.onAdd,
    required this.onBuy,
    this.addLabel = '加入購物車',
    this.buyLabel = '立即結帳',
    this.floating = false,
    this.integrated = false,
  });

  final bool hasSelection;
  final int stock;
  final bool enabled;
  final bool addLoading;
  final bool buyLoading;
  final VoidCallback onAdd;
  final VoidCallback onBuy;
  final String addLabel;
  final String buyLabel;
  final bool floating;

  /// Renders the footer inside a parent card without adding another surface.
  /// The parent owns the separator and clipping in this mode.
  final bool integrated;

  @override
  Widget build(BuildContext context) {
    final canAct = enabled && hasSelection && stock > 0;
    final stockLabel = !hasSelection
        ? '選擇規格'
        : stock <= 0
        ? '已售完'
        : '庫存：$stock 件';

    final content = Container(
      key: const ValueKey('product-purchase-footer'),
      padding: const EdgeInsets.fromLTRB(
        HkSpace.xl,
        HkSpace.md,
        HkSpace.xl,
        HkSpace.lg,
      ),
      decoration: floating || integrated
          ? null
          : BoxDecoration(
              color: AppColors.bgCard,
              border: Border(
                top: BorderSide(
                  color: AppColors.textPrimary.withValues(alpha: 0.08),
                ),
              ),
            ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              const Icon(
                AppIcons.inventory2Outlined,
                size: 22,
                color: AppIconColors.muted,
              ),
              const SizedBox(width: HkSpace.sm),
              Flexible(
                child: Text(
                  stockLabel,
                  key: const ValueKey('product-purchase-stock'),
                  style: AppTypography.body.copyWith(
                    color: hasSelection && stock <= 0
                        ? AppColors.danger
                        : AppColors.textPrimary,
                  ),
                ),
              ),
            ],
          ),
          const SizedBox(height: HkSpace.sm),
          LayoutBuilder(
            builder: (context, constraints) {
              final addButton = _ProductPurchaseActionButton(
                key: const ValueKey('product-purchase-add'),
                label: addLabel,
                icon: HkIconName.cart,
                variant: _ProductPurchaseActionVariant.cart,
                enabled: canAct && !addLoading && !buyLoading,
                loading: addLoading,
                onTap: onAdd,
              );
              final buyButton = _ProductPurchaseActionButton(
                key: const ValueKey('product-purchase-buy'),
                label: buyLabel,
                icon: HkIconName.checkoutCoins,
                variant: _ProductPurchaseActionVariant.checkout,
                enabled: canAct && !addLoading && !buyLoading,
                loading: buyLoading,
                onTap: onBuy,
              );
              if (constraints.maxWidth < 320) {
                return Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    addButton,
                    const SizedBox(height: HkSpace.sm),
                    buyButton,
                  ],
                );
              }
              return Row(
                children: [
                  Expanded(child: addButton),
                  const SizedBox(width: HkSpace.sm),
                  Expanded(child: buyButton),
                ],
              );
            },
          ),
        ],
      ),
    );
    if (!floating) return content;
    return HkLiquidGlassSurface(
      surfaceKey: const ValueKey('product-purchase-floating-surface'),
      variant: HkLiquidGlassVariant.regular,
      backgroundColor: AppColors.neutralCream,
      surfaceOpacity: 0.82,
      blurSigma: HkGlass.navBarBlurSigma,
      borderRadius: BorderRadius.circular(HkRadius.xxl),
      shadows: HkShadows.lg,
      child: content,
    );
  }
}

class _ProductPurchaseActionButton extends StatelessWidget {
  const _ProductPurchaseActionButton({
    super.key,
    required this.label,
    required this.enabled,
    required this.loading,
    required this.onTap,
    required this.icon,
    required this.variant,
  });

  final String label;
  final HkIconName icon;
  final _ProductPurchaseActionVariant variant;
  final bool enabled;
  final bool loading;
  final VoidCallback onTap;

  void _handleTap() {
    if (!enabled || loading) return;
    onTap();
  }

  @override
  Widget build(BuildContext context) {
    final backgroundColor = switch (variant) {
      _ProductPurchaseActionVariant.cart => AppColors.commerceCartAction,
      _ProductPurchaseActionVariant.checkout =>
        AppColors.commerceCheckoutAction,
    };
    final foregroundColor = switch (variant) {
      _ProductPurchaseActionVariant.cart => AppColors.commerceCartForeground,
      _ProductPurchaseActionVariant.checkout =>
        AppColors.commerceCheckoutForeground,
    };
    return Semantics(
      button: true,
      enabled: enabled,
      label: label,
      onTap: enabled ? _handleTap : null,
      child: ExcludeSemantics(
        child: MouseRegion(
          cursor: enabled ? SystemMouseCursors.click : SystemMouseCursors.basic,
          child: GestureDetector(
            behavior: HitTestBehavior.opaque,
            onTap: enabled ? _handleTap : null,
            child: AnimatedOpacity(
              duration: HkMotion.duration,
              opacity: enabled || loading ? 1 : 0.50,
              child: Container(
                key: ValueKey('product-purchase-${variant.name}-surface'),
                height: HkControl.minTap,
                decoration: BoxDecoration(
                  color: backgroundColor,
                  borderRadius: BorderRadius.circular(HkRadius.pill),
                ),
                child: LayoutBuilder(
                  builder: (context, constraints) {
                    final horizontalPadding = constraints.maxWidth < 160
                        ? HkSpace.sm
                        : HkSpace.xl;
                    return Padding(
                      padding: EdgeInsets.symmetric(
                        horizontal: horizontalPadding,
                      ),
                      child: Center(
                        child: loading
                            ? SizedBox(
                                width: 20,
                                height: 20,
                                child: HkBrandLoader(
                                  tone: HkBrandLoaderTone.mono,
                                  color: foregroundColor,
                                ),
                              )
                            : Row(
                                mainAxisSize: MainAxisSize.min,
                                mainAxisAlignment: MainAxisAlignment.center,
                                children: [
                                  HkIcon(
                                    icon,
                                    key: ValueKey(
                                      'product-purchase-${icon.name}-icon',
                                    ),
                                    size: 22,
                                    color: foregroundColor,
                                  ),
                                  const SizedBox(width: HkSpace.sm),
                                  Flexible(
                                    child: Text(
                                      label,
                                      maxLines: 1,
                                      overflow: TextOverflow.ellipsis,
                                      style: AppTypography.buttonLarge.copyWith(
                                        height: 1,
                                        color: foregroundColor,
                                      ),
                                    ),
                                  ),
                                ],
                              ),
                      ),
                    );
                  },
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}

enum _ProductPurchaseActionVariant { cart, checkout }

class _SpecSkeleton extends StatelessWidget {
  const _SpecSkeleton();

  @override
  Widget build(BuildContext context) {
    return const Wrap(
      spacing: HkSpace.sm,
      runSpacing: HkSpace.sm,
      children: [
        _PurchaseSkeleton(width: 72, height: 40, radius: HkRadius.pill),
        _PurchaseSkeleton(width: 72, height: 40, radius: HkRadius.pill),
        _PurchaseSkeleton(width: 72, height: 40, radius: HkRadius.pill),
      ],
    );
  }
}

class _PurchaseSkeleton extends StatelessWidget {
  const _PurchaseSkeleton({
    required this.width,
    required this.height,
    this.radius,
  });

  final double width;
  final double height;
  final double? radius;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: width,
      height: height,
      decoration: BoxDecoration(
        color: AppColors.gray100,
        borderRadius: BorderRadius.circular(radius ?? HkRadius.xs),
      ),
    );
  }
}
