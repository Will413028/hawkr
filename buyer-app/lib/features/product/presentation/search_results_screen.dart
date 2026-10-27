import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'dart:async';
import 'package:flutter/material.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:infinite_scroll_pagination/infinite_scroll_pagination.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import '../../product/presentation/search_controller.dart' as ctrl;
import '../../category/data/product_item_model.dart';
import '../../../config/image_config.dart';
import '../../../core/analytics/analytics_providers.dart';
import '../../../core/analytics/ga4_item.dart';
import '../../../core/theme/app_typography.dart';
import '../../../core/theme/hk_tokens.dart';
import '../../../shared/widgets/app_cached_image.dart';
import '../../../shared/widgets/cart_icon_with_badge.dart';
import '../../../shared/widgets/category_top_bar.dart';
import '../../../shared/widgets/chat_icon_with_badge.dart';
import '../../../shared/widgets/hk_button_label.dart';
import '../../../shared/widgets/hk_empty_state.dart';
import '../../../shared/widgets/hk_tap_target.dart';
import '../../../shared/widgets/product_card_hero.dart';
import '../../../shared/widgets/truck_loading_overlay.dart';
import '../../../shared/utils/price_display.dart';
import '../../home/presentation/home_widget/product_add_to_cart_sheet.dart';
import 'merchant_item_detail_screen.dart';

GA4Item _searchProductToGa4Item(ProductItem product, {int? index}) => GA4Item(
  itemId: product.id,
  itemName: product.name,
  price: PriceDisplay.parseMoney(product.priceMin),
  itemBrand: (product.sellerName?.trim().isEmpty ?? true)
      ? null
      : product.sellerName,
  itemCategory: product.categoryId,
  index: index,
);

class SearchResultsScreen extends HookConsumerWidget {
  const SearchResultsScreen({super.key, required this.query});
  final String query;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final searchCtrl = useTextEditingController(text: query);
    final showSuggestions = useState(false);
    final suggestions = useState<List<String>>([]);
    final debounce = useRef<Timer?>(null);
    final currentKeyword = useRef(query);

    // Paging controller for infinite scroll
    final pagingController = useMemoized(
      () => PagingController<int, ProductItem>(firstPageKey: 1),
    );

    useEffect(() {
      void listener(int pageKey) async {
        try {
          final page = await ref
              .read(ctrl.searchControllerProvider.notifier)
              .fetchSearchPage(currentKeyword.value, pageKey);
          final items = page.records;
          if (page.isLastPage) {
            pagingController.appendLastPage(items);
          } else {
            pagingController.appendPage(items, page.page + 1);
          }
          // GA4: view_item_list — fired once per loaded page, not on every rebuild
          if (items.isNotEmpty) {
            final startIndex = (pageKey - 1) * ctrl.SearchController.pageSize;
            final ga4Items = items.indexed
                .map(
                  (e) =>
                      _searchProductToGa4Item(e.$2, index: startIndex + e.$1),
                )
                .toList();
            ref
                .read(analyticsEventsProvider)
                .trackViewItemList(
                  ga4Items,
                  listId: 'search',
                  listName: '搜尋結果',
                );
          }
        } catch (e) {
          pagingController.error = e;
        }
      }

      pagingController.addPageRequestListener(listener);
      return () {
        pagingController.removePageRequestListener(listener);
        pagingController.dispose();
      };
    }, []);

    void onSearchChanged(String text) {
      final kw = text.trim();
      showSuggestions.value = kw.isNotEmpty;
      debounce.value?.cancel();
      debounce.value = Timer(const Duration(milliseconds: 500), () async {
        if (kw.isEmpty) {
          suggestions.value = [];
          return;
        }
        try {
          final result = await ref
              .read(ctrl.searchControllerProvider.notifier)
              .getSuggestions(kw);
          suggestions.value = result.take(10).toList();
        } catch (_) {
          suggestions.value = [];
        }
      });
    }

    void submitSearch(String keyword) {
      final kw = keyword.trim();
      if (kw.isEmpty) return;
      showSuggestions.value = false;
      currentKeyword.value = kw;
      pagingController.refresh();
    }

    TextSpan buildHighlightedSpan(String text, String keyword) {
      if (keyword.isEmpty) {
        return TextSpan(
          text: text,
          style: const TextStyle(color: AppColors.textEmphasis),
        );
      }
      final lowerText = text.toLowerCase();
      final lowerKey = keyword.toLowerCase();
      final idx = lowerText.indexOf(lowerKey);
      if (idx < 0) {
        return TextSpan(
          text: text,
          style: const TextStyle(color: AppColors.textEmphasis),
        );
      }
      final pre = text.substring(0, idx);
      final match = text.substring(idx, idx + keyword.length);
      final post = text.substring(idx + keyword.length);
      return TextSpan(
        children: [
          TextSpan(
            text: pre,
            style: const TextStyle(color: AppColors.textSecondary),
          ),
          TextSpan(
            text: match,
            style: const TextStyle(
              color: AppColors.textEmphasis,
              fontWeight: FontWeight.w700,
            ),
          ),
          TextSpan(
            text: post,
            style: const TextStyle(color: AppColors.textSecondary),
          ),
        ],
      );
    }

    Widget buildSuggestionsList() {
      final kw = searchCtrl.text.trim();
      if (!showSuggestions.value) return const SizedBox.shrink();
      if (kw.isEmpty || suggestions.value.isEmpty) {
        return Padding(
          padding: EdgeInsets.fromLTRB(12.w, 0, 12.w, 8.h),
          child: const Text(
            '可以搜尋',
            style: TextStyle(color: AppColors.textSecondary),
          ),
        );
      }
      return Container(
        margin: EdgeInsets.fromLTRB(12.w, 0, 12.w, 8.h),
        decoration: BoxDecoration(
          color: AppColors.bgCard,
          borderRadius: BorderRadius.circular(12.r),
          border: Border.all(color: AppColors.gray300),
          boxShadow: const [
            BoxShadow(
              color: AppColors.cardShadow,
              blurRadius: 8,
              offset: Offset(0, 2),
            ),
          ],
        ),
        child: ListView.separated(
          shrinkWrap: true,
          physics: const NeverScrollableScrollPhysics(),
          padding: EdgeInsets.symmetric(vertical: 8.h),
          itemBuilder: (context, i) {
            final label = suggestions.value[i];
            return InkWell(
              onTap: () {
                searchCtrl.text = label;
                showSuggestions.value = false;
                FocusScope.of(context).unfocus();
                currentKeyword.value = label;
                pagingController.refresh();
              },
              child: Padding(
                padding: EdgeInsets.symmetric(horizontal: 12.w, vertical: 12.h),
                child: Row(
                  children: [
                    Icon(
                      AppIcons.search,
                      size: 18.sp,
                      color: AppIconColors.muted,
                    ),
                    SizedBox(width: 8.w),
                    Expanded(
                      child: RichText(
                        text: buildHighlightedSpan(label, kw),
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                      ),
                    ),
                  ],
                ),
              ),
            );
          },
          separatorBuilder: (_, __) =>
              Divider(height: 1, color: AppColors.gray200),
          itemCount: suggestions.value.length,
        ),
      );
    }

    return Scaffold(
      backgroundColor: AppColors.bgPrimary,
      body: SafeArea(
        top: false,
        child: Column(
          children: [
            CategoryTopBar(
              backgroundColor: AppColors.topBarBackground,
              systemOverlayStyle: AppColors.topBarOverlay,
              leading: HkTapTarget(
                semanticLabel: '返回',
                onTap: () => context.pop(),
                child: const HkIcon(
                  HkIconName.back,
                  size: HkControl.icon,
                  color: AppIconColors.strong,
                ),
              ),
              middle: _SearchInput(
                controller: searchCtrl,
                onChanged: onSearchChanged,
                onSubmitted: submitSearch,
              ),
              actions: const [
                CartIconWithBadge(compact: true, iconSize: HkControl.icon),
                ChatIconWithBadge(compact: true, iconSize: HkControl.icon),
              ],
            ),
            buildSuggestionsList(),
            Expanded(
              child: PagedListView<int, ProductItem>.separated(
                pagingController: pagingController,
                padding: EdgeInsets.only(bottom: 16.h),
                separatorBuilder: (_, __) => const Divider(height: 1),
                builderDelegate: PagedChildBuilderDelegate<ProductItem>(
                  itemBuilder: (context, product, index) {
                    final productId = product.id.trim();
                    final heroTag = productId.isEmpty
                        ? null
                        : 'product-card:search:$productId';
                    return _ProductTile(
                      product: product,
                      heroTag: heroTag,
                      onOpen: () {
                        ref
                            .read(analyticsEventsProvider)
                            .trackSelectItem(
                              _searchProductToGa4Item(product, index: index),
                              listId: 'search',
                              listName: '搜尋結果',
                            );
                        context.push(
                          '/merchant-product',
                          extra: MerchantItemDetailScreen(
                            title: product.name,
                            imageUrl: product.displayImage.isEmpty
                                ? null
                                : product.displayImage,
                            price: PriceDisplay.parseMoney(product.priceMin),
                            skuId: product.id,
                            merchantName: product.sellerName,
                            merchantId: product.storeId,
                            heroTag: heroTag,
                          ),
                        );
                      },
                      onAddToCart: () => showProductAddToCartSheet(
                        context,
                        ref,
                        skuId: product.id,
                        productName: product.name,
                        imageUrl: product.displayImage,
                        initialPrice: PriceDisplay.parseMoney(
                          product.priceMin,
                        )?.toDouble(),
                      ),
                    );
                  },
                  firstPageProgressIndicatorBuilder: (_) =>
                      const TruckLoadingOverlay(message: '載入中...'),
                  newPageProgressIndicatorBuilder: (_) => Padding(
                    padding: EdgeInsets.all(16.w),
                    child: const TruckLoadingOverlay(message: '載入中...'),
                  ),
                  noItemsFoundIndicatorBuilder: (_) => const HkEmptyState(
                    key: ValueKey('search-results-empty-state'),
                    iconKey: ValueKey('search-results-empty-state-icon'),
                    icon: HkIconName.emptySearch,
                    message: '沒有找到相關商品',
                  ),
                  firstPageErrorIndicatorBuilder: (context) => Center(
                    child: Column(
                      mainAxisAlignment: MainAxisAlignment.center,
                      children: [
                        Text(
                          '搜尋失敗',
                          style: TextStyle(color: AppColors.textSecondary),
                        ),
                        SizedBox(height: 8.h),
                        ElevatedButton(
                          onPressed: () =>
                              pagingController.retryLastFailedRequest(),
                          child: const HkButtonLabel('重試'),
                        ),
                      ],
                    ),
                  ),
                  newPageErrorIndicatorBuilder: (context) => Center(
                    child: TextButton(
                      onPressed: () =>
                          pagingController.retryLastFailedRequest(),
                      child: const HkButtonLabel('載入失敗，點擊重試'),
                    ),
                  ),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _ProductTile extends StatelessWidget {
  const _ProductTile({
    required this.product,
    required this.heroTag,
    required this.onOpen,
    required this.onAddToCart,
  });

  final ProductItem product;
  final Object? heroTag;
  final VoidCallback onOpen;
  final VoidCallback onAddToCart;

  @override
  Widget build(BuildContext context) {
    final isSoldOut = product.isSoldOut;
    final actionLabel = isSoldOut ? '已售完' : '加入購物車';
    final priceLabel = PriceDisplay.formatPriceRange(
      product.priceMin,
      product.priceMax,
      prefix: 'NT\$ ',
    );
    final semanticLabel = <String>[
      '查看商品：${product.name}',
      if (priceLabel.isNotEmpty) '價格：$priceLabel',
      if (product.sellerName?.trim().isNotEmpty ?? false)
        '商家：${product.sellerName}',
      if (product.stockQty case final stock?) stock > 0 ? '庫存：$stock 件' : '已售完',
    ].join('，');

    return Padding(
      padding: const EdgeInsets.symmetric(
        horizontal: HkSpace.md,
        vertical: HkSpace.sm,
      ),
      child: LayoutBuilder(
        builder: (context, constraints) {
          final compact = constraints.maxWidth < HkBreakpoints.compact;
          final summary = Semantics(
            button: true,
            label: semanticLabel,
            excludeSemantics: true,
            child: Material(
              color: Colors.transparent,
              child: InkWell(
                onTap: onOpen,
                borderRadius: BorderRadius.circular(HkRadius.md),
                child: Padding(
                  padding: const EdgeInsets.symmetric(vertical: HkSpace.xs),
                  child: _ProductSummary(product: product, heroTag: heroTag),
                ),
              ),
            ),
          );
          final action = SizedBox(
            width: compact ? double.infinity : 124,
            height: HkControl.minTap,
            child: ElevatedButton(
              onPressed: isSoldOut ? null : onAddToCart,
              style: ElevatedButton.styleFrom(
                backgroundColor: AppColors.textEmphasis,
                foregroundColor: AppColors.bgCard,
                padding: const EdgeInsets.symmetric(horizontal: HkSpace.md),
                shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(HkRadius.md),
                ),
              ),
              child: HkButtonLabel(actionLabel),
            ),
          );

          if (compact) {
            return Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                summary,
                const SizedBox(height: HkSpace.sm),
                action,
              ],
            );
          }
          return Row(
            crossAxisAlignment: CrossAxisAlignment.center,
            children: [
              Expanded(child: summary),
              const SizedBox(width: HkSpace.md),
              action,
            ],
          );
        },
      ),
    );
  }
}

class _ProductSummary extends StatelessWidget {
  const _ProductSummary({required this.product, required this.heroTag});

  final ProductItem product;
  final Object? heroTag;

  @override
  Widget build(BuildContext context) {
    final price = PriceDisplay.formatPriceRange(
      product.priceMin,
      product.priceMax,
      prefix: 'NT\$ ',
    );
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        ProductMediaHero(
          tag: heroTag,
          child: Container(
            width: 72,
            height: 72,
            decoration: BoxDecoration(
              border: Border.all(color: AppColors.gray300),
              color: AppColors.gray100,
              borderRadius: BorderRadius.circular(HkRadius.sm),
            ),
            clipBehavior: Clip.antiAlias,
            child: product.displayImage.isNotEmpty
                ? AppCachedImage(
                    imageUrl: product.displayImage,
                    fit: BoxFit.cover,
                    width: 72,
                    height: 72,
                    borderRadius: BorderRadius.circular(HkRadius.sm),
                    scene: ImageScene.searchResult,
                  )
                : Icon(AppIcons.imageOutlined, color: AppIconColors.muted),
          ),
        ),
        const SizedBox(width: HkSpace.md),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                product.name,
                maxLines: 2,
                overflow: TextOverflow.ellipsis,
                style: AppTypography.productName,
              ),
              if (price.isNotEmpty) ...[
                const SizedBox(height: HkSpace.xs),
                Text(
                  price,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: AppTypography.meta.copyWith(
                    color: AppColors.textEmphasis,
                    fontWeight: FontWeight.w700,
                  ),
                ),
              ],
              if (product.sellerName case final merchantName?) ...[
                const SizedBox(height: HkSpace.xs),
                Text(
                  merchantName,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: AppTypography.caption.copyWith(color: AppColors.info),
                ),
              ],
              if (product.stockQty case final stock?) ...[
                const SizedBox(height: HkSpace.xs),
                Text(
                  stock > 0 ? '剩餘 $stock 件' : '已售完',
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: AppTypography.caption.copyWith(
                    color: AppColors.danger,
                  ),
                ),
              ],
            ],
          ),
        ),
      ],
    );
  }
}

class _SearchInput extends StatelessWidget {
  const _SearchInput({
    required this.controller,
    required this.onChanged,
    this.onSubmitted,
  });
  final TextEditingController controller;
  final ValueChanged<String> onChanged;
  final ValueChanged<String>? onSubmitted;

  @override
  Widget build(BuildContext context) {
    return CategorySearchField(
      controller: controller,
      hintText: '搜尋商品',
      semanticLabel: '商品搜尋',
      semanticHint: '輸入商品名稱或關鍵字',
      onChanged: onChanged,
      onSubmitted: (value) {
        FocusScope.of(context).unfocus();
        onSubmitted?.call(value);
      },
      suffixIcon: controller.text.isEmpty
          ? null
          : Semantics(
              label: '清除搜尋',
              button: true,
              child: GestureDetector(
                behavior: HitTestBehavior.opaque,
                onTap: () {
                  controller.clear();
                  onChanged('');
                },
                child: const SizedBox.square(
                  dimension: HkControl.minTap,
                  child: Center(
                    child: HkIcon(
                      HkIconName.close,
                      size: 20,
                      color: AppIconColors.muted,
                    ),
                  ),
                ),
              ),
            ),
    );
  }
}
