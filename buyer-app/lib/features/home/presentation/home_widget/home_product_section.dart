import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';
import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../../core/theme/app_typography.dart';
import '../../../../core/theme/hk_tokens.dart';
import '../../../../shared/widgets/hk_empty_state.dart';
import '../../../../shared/widgets/hk_button_label.dart';
import '../../../../shared/widgets/product_grid_spec.dart';
import '../../../../shared/widgets/product_card.dart';
import '../../data/home_product_model.dart';
import '../../../product/presentation/merchant_item_detail_screen.dart';
import 'product_add_to_cart_sheet.dart';

/// 首頁商品推薦 Sliver 區塊。
///
/// This widget must be placed directly in a [CustomScrollView.slivers] list.
/// Using a real [SliverGrid] keeps off-screen product cards lazy instead of
/// eagerly laying out an inner shrink-wrapped grid.
class HomeProductSection extends ConsumerWidget {
  const HomeProductSection({
    super.key,
    required this.isLoading,
    required this.products,
    this.initialErrorMessage,
    this.onRetryInitial,
  });

  static const int previewLimit = 6;

  final bool isLoading;
  final List<HomeProduct> products;

  final String? initialErrorMessage;
  final VoidCallback? onRetryInitial;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return SliverLayoutBuilder(
      builder: (context, constraints) {
        final gridSpec = ProductGridSpec.forWidth(
          constraints.crossAxisExtent,
          textScale: ProductCard.effectiveTextScale(context),
        );

        return SliverMainAxisGroup(
          slivers: [
            SliverToBoxAdapter(
              child: Padding(
                padding: EdgeInsets.symmetric(
                  horizontal: gridSpec.horizontalInset,
                ),
                child: Row(
                  mainAxisAlignment: MainAxisAlignment.spaceBetween,
                  children: [
                    Text(
                      '商品推薦',
                      style: AppTypography.productName.copyWith(
                        color: AppTypography.colorPrimary,
                      ),
                    ),
                    Semantics(
                      button: true,
                      label: '查看更多商品',
                      child: TextButton(
                        key: const ValueKey('home-products-see-more'),
                        style: ButtonStyle(
                          minimumSize: const WidgetStatePropertyAll(
                            Size(0, HkControl.minTap),
                          ),
                          padding: const WidgetStatePropertyAll(
                            EdgeInsets.symmetric(horizontal: HkSpace.xs),
                          ),
                          foregroundColor: const WidgetStatePropertyAll(
                            AppColors.brandPrimaryStrong,
                          ),
                          overlayColor: WidgetStatePropertyAll(
                            AppColors.brandPrimaryStrong.withValues(
                              alpha: 0.08,
                            ),
                          ),
                          shape: WidgetStatePropertyAll(
                            RoundedRectangleBorder(
                              borderRadius: BorderRadius.circular(HkRadius.sm),
                            ),
                          ),
                        ),
                        onPressed: () {
                          context.go('/?tab=1&sort=LATEST');
                        },
                        child: HkButtonLabel(
                          '查看更多',
                          style: AppTypography.meta.copyWith(
                            color: AppColors.brandPrimaryStrong,
                            fontWeight: FontWeight.w700,
                          ),
                        ),
                      ),
                    ),
                  ],
                ),
              ),
            ),
            const SliverToBoxAdapter(child: SizedBox(height: HkSpace.xs)),
            if (initialErrorMessage != null && products.isEmpty)
              SliverToBoxAdapter(
                child: _InitialErrorCard(
                  message: initialErrorMessage!,
                  onRetry: onRetryInitial,
                ),
              )
            else if (isLoading && products.isEmpty)
              SliverPadding(
                padding: gridSpec.padding,
                sliver: SliverGrid(
                  key: const ValueKey('home-product-grid-loading'),
                  gridDelegate: gridSpec.delegate,
                  delegate: SliverChildBuilderDelegate(
                    (_, __) => const _ProductCardSkeleton(),
                    childCount: previewLimit,
                  ),
                ),
              )
            else if (products.isEmpty)
              const SliverToBoxAdapter(
                child: SizedBox(
                  height: 190,
                  child: HkEmptyState(
                    key: ValueKey('home-products-empty-state'),
                    iconKey: ValueKey('home-products-empty-state-icon'),
                    icon: HkIconName.emptyProducts,
                    message: '暫無推薦',
                    topPadding: 24,
                  ),
                ),
              )
            else
              SliverPadding(
                padding: gridSpec.padding,
                sliver: SliverGrid(
                  key: const ValueKey('home-product-grid'),
                  gridDelegate: gridSpec.delegate,
                  delegate: SliverChildBuilderDelegate(
                    (context, index) {
                      final product = products[index];
                      return ProductCard(
                        key: ValueKey('home-product-${product.id}'),
                        name: product.name,
                        price: product.priceMin,
                        priceMax: product.priceMax,
                        listPriceMin: product.listPriceMin,
                        listPriceMax: product.listPriceMax,
                        imageUrl: product.imageUrl,
                        sellerName: product.sellerName,
                        sellerAvatarUrl: product.sellerAvatarUrl,
                        hashtags: product.hashtags,
                        onTap: () => context.push(
                          '/merchant-product',
                          extra: MerchantItemDetailScreen(
                            title: product.name.isNotEmpty
                                ? product.name
                                : '商品',
                            imageUrl: product.imageUrl.isNotEmpty
                                ? product.imageUrl
                                : null,
                            price: product.priceMin.isNotEmpty
                                ? num.tryParse(product.priceMin)
                                : null,
                            skuId: product.id,
                            merchantId: product.sellerAccountId.isEmpty
                                ? null
                                : product.sellerAccountId,
                            merchantName: product.sellerName,
                          ),
                        ),
                        onAddToCartTap: () => showProductAddToCartSheet(
                          context,
                          ref,
                          skuId: product.id,
                          productName: product.name,
                          imageUrl: product.imageUrl,
                          initialPrice: product.priceMin.isNotEmpty
                              ? num.tryParse(product.priceMin)?.toDouble()
                              : null,
                        ),
                      );
                    },
                    childCount: products.length.clamp(0, previewLimit),
                    addAutomaticKeepAlives: false,
                  ),
                ),
              ),
          ],
        );
      },
    );
  }
}

class _InitialErrorCard extends StatelessWidget {
  const _InitialErrorCard({required this.message, required this.onRetry});

  final String message;
  final VoidCallback? onRetry;

  @override
  Widget build(BuildContext context) {
    return Padding(
      key: const ValueKey('home-products-initial-error'),
      padding: const EdgeInsets.fromLTRB(
        HkSpace.md,
        HkSpace.lg,
        HkSpace.md,
        HkSpace.xl,
      ),
      child: Column(
        children: [
          Text(
            message,
            textAlign: TextAlign.center,
            style: AppTypography.body.copyWith(color: AppColors.textSecondary),
          ),
          const SizedBox(height: HkSpace.sm),
          FilledButton(
            key: const ValueKey('home-products-retry-initial'),
            onPressed: onRetry,
            style: const ButtonStyle(
              minimumSize: WidgetStatePropertyAll(Size(0, HkControl.minTap)),
            ),
            child: const HkButtonLabel('重新載入'),
          ),
        ],
      ),
    );
  }
}

// ── 骨架屏卡片 ──────────────────────────────────────────────────────────────

class _ProductCardSkeleton extends StatelessWidget {
  const _ProductCardSkeleton();

  @override
  Widget build(BuildContext context) {
    return Container(
      key: const ValueKey('home-product-skeleton'),
      decoration: BoxDecoration(
        color: AppColors.bgCard,
        borderRadius: BorderRadius.circular(HkRadius.md),
        boxShadow: HkShadows.sm,
      ),
      foregroundDecoration: BoxDecoration(
        borderRadius: BorderRadius.circular(HkRadius.md),
        border: Border.all(color: AppColors.borderSubtle),
      ),
      clipBehavior: Clip.antiAlias,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          AspectRatio(
            key: const ValueKey('home-product-skeleton-media'),
            aspectRatio: ProductGridSpec.mediaAspectRatio,
            child: Padding(
              padding: const EdgeInsets.all(HkSpace.xs),
              child: DecoratedBox(
                decoration: BoxDecoration(
                  color: AppColors.gray100,
                  borderRadius: BorderRadius.circular(HkRadius.sm),
                ),
              ),
            ),
          ),
          Expanded(
            child: Padding(
              padding: const EdgeInsets.symmetric(horizontal: HkSpace.sm),
              child: Column(
                mainAxisAlignment: MainAxisAlignment.center,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      const CircleAvatar(
                        radius: 8,
                        backgroundColor: AppColors.gray100,
                      ),
                      const SizedBox(width: HkSpace.xs),
                      Container(
                        height: 8,
                        width: 48,
                        decoration: BoxDecoration(
                          color: AppColors.gray100,
                          borderRadius: BorderRadius.circular(HkRadius.xs),
                        ),
                      ),
                    ],
                  ),
                  const SizedBox(height: 2),
                  Container(
                    height: 12,
                    width: 80,
                    decoration: BoxDecoration(
                      color: AppColors.gray100,
                      borderRadius: BorderRadius.circular(HkRadius.xs),
                    ),
                  ),
                  const SizedBox(height: 4),
                  Container(
                    height: 14,
                    width: 70,
                    decoration: BoxDecoration(
                      color: AppColors.gray100,
                      borderRadius: BorderRadius.circular(HkRadius.xs),
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
}
