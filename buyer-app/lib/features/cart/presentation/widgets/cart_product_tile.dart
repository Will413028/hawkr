import 'dart:math' as math;

import '../cart_typography.dart';
import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/shared/widgets/quantity_stepper.dart';
import 'package:flutter/material.dart';

import '../../data/cart_model.dart';

/// 購物車商品列。
///
/// 手機版將數量與價格放在縮圖旁，品名與規格保留完整寬度；
/// 寬螢幕再合併為橫向商品列。
class CartProductTile extends StatelessWidget {
  const CartProductTile({
    super.key,
    required this.item,
    required this.isChecked,
    this.remainingStock,
    this.imageProvider,
    required this.onCheckChanged,
    required this.onQuantityChanged,
  });

  final CartItem item;
  final bool isChecked;
  final int? remainingStock;
  final ImageProvider? imageProvider;
  final ValueChanged<bool> onCheckChanged;
  final ValueChanged<int> onQuantityChanged;

  // 後端 issue #1127：cart 透過 purchasable + unavailableReason 表達 row 級可購買性。
  static const Map<String, String> _unavailableLabel = {
    'DELISTED': '已下架',
    'OUT_OF_STOCK': '已售完',
    'INSUFFICIENT_STOCK': '庫存不足',
    'LOOKUP_FAILED': '暫時無法確認',
  };

  String get _title =>
      item.title?.trim().isNotEmpty == true ? item.title!.trim() : '商品';

  @override
  Widget build(BuildContext context) {
    final unavailable = !item.purchasable;
    final badgeLabel =
        _unavailableLabel[item.unavailableReason ?? ''] ?? '目前不可購買';

    final tileContent = Padding(
      key: ValueKey('cart-product-tile-${item.skuId}'),
      padding: const EdgeInsets.symmetric(vertical: HkSpace.sm),
      child: LayoutBuilder(
        builder: (context, constraints) {
          if (constraints.maxWidth >= 480) {
            return _buildWideLayout(unavailable);
          }
          return _buildCompactLayout(
            context,
            constraints.maxWidth,
            unavailable,
          );
        },
      ),
    );

    if (!unavailable) return tileContent;

    // 不可購買商品仍保留外層 IosSwipeAction，讓使用者可以左滑移除。
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Align(
          alignment: Alignment.centerLeft,
          child: Semantics(
            key: ValueKey('cart-product-unavailable-${item.skuId}'),
            label: badgeLabel,
            child: ExcludeSemantics(
              child: DecoratedBox(
                decoration: BoxDecoration(
                  color: AppColors.textEmphasis.withValues(alpha: 0.88),
                  borderRadius: BorderRadius.circular(HkRadius.xs),
                ),
                child: Padding(
                  padding: const EdgeInsets.symmetric(
                    horizontal: HkSpace.sm,
                    vertical: HkSpace.xs,
                  ),
                  child: Text(
                    badgeLabel,
                    style: CartTypography.supportMuted.copyWith(
                      color: AppColors.textInverse,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                ),
              ),
            ),
          ),
        ),
        Opacity(
          opacity: 0.55,
          child: ColorFiltered(
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
            child: tileContent,
          ),
        ),
      ],
    );
  }

  Widget _buildCompactLayout(
    BuildContext context,
    double maxWidth,
    bool unavailable,
  ) {
    // QuantityStepper keeps its 48px buttons and scaled count; give it its
    // actual text width before sharing the remaining space with the thumbnail.
    final countPainter = TextPainter(
      text: TextSpan(
        text: '${item.count}',
        style: DefaultTextStyle.of(context).style.merge(CartTypography.button),
      ),
      textDirection: Directionality.of(context),
      textScaler: MediaQuery.textScalerOf(context),
    )..layout();
    final stepperWidth =
        HkControl.minTap * 2 +
        math.max(32, countPainter.width + HkSpace.xs * 2);
    countPainter.dispose();
    // Keep the thumbnail and controls inline only when their measured widths fit.
    // A very large count gets its own row, with a stacked stepper if necessary.
    if (stepperWidth + HkSpace.sm + 48 > maxWidth) {
      return Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              _buildSelectionControl(unavailable, 64),
              const SizedBox(width: HkSpace.sm),
              _buildProductImage(64, 64),
              const SizedBox(width: HkSpace.sm),
              Expanded(child: _buildPriceAndStock()),
            ],
          ),
          const SizedBox(height: HkSpace.sm),
          _buildProductSummary(expanded: true),
          const SizedBox(height: HkSpace.sm),
          Align(
            alignment: Alignment.centerRight,
            child: _buildQuantityStepper(
              unavailable,
              stacked: stepperWidth > maxWidth,
            ),
          ),
        ],
      );
    }
    final selectionBesideImage =
        maxWidth - HkControl.minTap - HkSpace.sm - stepperWidth >= 48;
    final imageWidth = math.min(
      88.0,
      math.max(
        48.0,
        maxWidth -
            (selectionBesideImage ? HkControl.minTap : 0) -
            HkSpace.sm -
            stepperWidth,
      ),
    );
    final imageHeight = math.min(80.0, imageWidth);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        if (!selectionBesideImage) ...[
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              _buildSelectionControl(unavailable, HkControl.minTap),
              const SizedBox(width: HkSpace.sm),
              Expanded(child: _buildProductSummary(expanded: true)),
            ],
          ),
          const SizedBox(height: HkSpace.sm),
        ],
        Row(
          crossAxisAlignment: CrossAxisAlignment.end,
          children: [
            if (selectionBesideImage)
              _buildSelectionControl(unavailable, math.max(72, imageHeight)),
            _buildProductImage(imageWidth, imageHeight),
            const SizedBox(width: HkSpace.sm),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.end,
                children: [
                  _buildPriceAndStock(),
                  const SizedBox(height: HkSpace.xs),
                  _buildQuantityStepper(unavailable),
                ],
              ),
            ),
          ],
        ),
        if (selectionBesideImage) ...[
          const SizedBox(height: HkSpace.sm),
          _buildProductSummary(expanded: true),
        ],
      ],
    );
  }

  Widget _buildWideLayout(bool unavailable) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        _buildSelectionControl(unavailable, 90),
        const SizedBox(width: HkSpace.sm),
        _buildProductImage(120, 90),
        const SizedBox(width: HkSpace.lg),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              _buildProductSummary(),
              const SizedBox(height: HkSpace.sm),
              LayoutBuilder(
                builder: (context, constraints) =>
                    _buildPurchaseControls(constraints.maxWidth, unavailable),
              ),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildSelectionControl(bool unavailable, double alignmentHeight) {
    final enabled = !unavailable;
    return Semantics(
      button: true,
      enabled: enabled,
      checked: isChecked,
      label: '${isChecked ? '取消選取' : '選取'}$_title',
      onTap: enabled ? () => onCheckChanged(!isChecked) : null,
      child: ExcludeSemantics(
        child: GestureDetector(
          behavior: HitTestBehavior.opaque,
          onTap: enabled ? () => onCheckChanged(!isChecked) : null,
          child: SizedBox(
            width: HkControl.minTap,
            height: alignmentHeight < HkControl.minTap
                ? HkControl.minTap
                : alignmentHeight,
            child: Center(
              child: SizedBox.square(
                dimension: 24,
                child: Checkbox(
                  value: isChecked,
                  onChanged: enabled
                      ? (value) => onCheckChanged(value ?? false)
                      : null,
                  activeColor: AppColors.brandPrimaryStrong,
                  checkColor: AppColors.bgCard,
                  side: BorderSide(
                    color: isChecked
                        ? AppColors.brandPrimaryStrong
                        : AppColors.gray300,
                    width: 1.5,
                  ),
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(HkRadius.xs),
                  ),
                  materialTapTargetSize: MaterialTapTargetSize.shrinkWrap,
                  visualDensity: VisualDensity.compact,
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildProductImage(double width, double height) {
    return Semantics(
      key: ValueKey('cart-product-image-${item.skuId}'),
      image: true,
      label: '$_title商品圖片',
      child: ExcludeSemantics(
        child: DecoratedBox(
          decoration: BoxDecoration(
            color: AppColors.primarySoft,
            borderRadius: BorderRadius.circular(HkRadius.sm),
            boxShadow: HkShadows.xs,
          ),
          child: ClipRRect(
            borderRadius: BorderRadius.circular(HkRadius.sm),
            child: SizedBox(
              width: width,
              height: height,
              child: imageProvider != null
                  ? Image(
                      image: imageProvider!,
                      fit: BoxFit.cover,
                      gaplessPlayback: true,
                    )
                  : const ColoredBox(color: AppColors.primarySoft),
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildProductSummary({bool expanded = false}) {
    final spec = item.specName?.trim() ?? '';
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          _title,
          style: CartTypography.subtitle.copyWith(
            color: AppColors.textEmphasis,
          ),
          maxLines: expanded ? null : 2,
          overflow: expanded ? TextOverflow.visible : TextOverflow.ellipsis,
        ),
        if (spec.isNotEmpty) ...[
          const SizedBox(height: HkSpace.xs),
          Text(
            '規格：$spec',
            key: ValueKey('cart-product-spec-${item.skuId}'),
            style: CartTypography.supportMuted,
            maxLines: expanded ? null : 2,
            overflow: expanded ? TextOverflow.visible : TextOverflow.ellipsis,
          ),
        ],
      ],
    );
  }

  Widget _buildPurchaseControls(double availableWidth, bool unavailable) {
    final priceAndStock = _buildPriceAndStock();
    final stepper = _buildQuantityStepper(unavailable);

    if (availableWidth < 280) {
      return Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          priceAndStock,
          const SizedBox(height: HkSpace.sm),
          Align(alignment: Alignment.centerRight, child: stepper),
        ],
      );
    }

    return Row(
      crossAxisAlignment: CrossAxisAlignment.center,
      children: [
        Expanded(child: priceAndStock),
        const SizedBox(width: HkSpace.sm),
        stepper,
      ],
    );
  }

  Widget _buildQuantityStepper(bool unavailable, {bool stacked = false}) {
    return QuantityStepper(
      count: item.count,
      countTextStyle: CartTypography.button,
      stacked: stacked,
      // A retained sold-out row can report zero stock; the disabled stepper
      // still renders its existing quantity with a valid minimum of one.
      maxCount: remainingStock != null && remainingStock! < 1
          ? 1
          : remainingStock,
      disabled: unavailable,
      onIncrement: () => onQuantityChanged(item.count + 1),
      onDecrement: () => onQuantityChanged(item.count - 1),
    );
  }

  Widget _buildPriceAndStock() {
    final lowStock = remainingStock != null && remainingStock! <= 5;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Wrap(
          crossAxisAlignment: WrapCrossAlignment.center,
          spacing: HkSpace.sm,
          runSpacing: HkSpace.xs,
          children: [
            Text(
              'NT\$${item.price}',
              key: ValueKey('cart-product-price-${item.skuId}'),
              style: CartTypography.subtitle.copyWith(
                color: AppColors.textEmphasis,
              ),
            ),
            if (item.listPrice != null && item.listPrice! > item.price)
              Text(
                'NT\$${item.listPrice}',
                style: CartTypography.supportMuted.copyWith(
                  decoration: TextDecoration.lineThrough,
                  decorationColor: AppColors.textSecondary,
                ),
              ),
          ],
        ),
        const SizedBox(height: HkSpace.xs),
        Text(
          '庫存 ${remainingStock ?? '—'} 件',
          style: CartTypography.support.copyWith(
            color: lowStock ? AppColors.danger : AppColors.textSecondary,
            fontWeight: lowStock ? FontWeight.w600 : FontWeight.w400,
          ),
        ),
      ],
    );
  }
}
