import 'dart:async';

import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';
import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../core/analytics/analytics_providers.dart';
import '../../../core/analytics/ga4_item.dart';
import '../../../core/routing/market_route.dart';
import '../../../core/theme/app_typography.dart';
import '../../../core/theme/hk_tokens.dart';
import '../../../config/image_config.dart';
import '../../../shared/utils/hk_log.dart';
import '../../../shared/utils/require_login.dart';
import '../../../shared/widgets/app_cached_image.dart';
import '../../../shared/widgets/buyer_navigation_clearance_sliver.dart';
import '../../../shared/widgets/category_top_bar.dart';
import '../../../shared/widgets/hk_choice_chip.dart';
import '../../../shared/widgets/hk_empty_state.dart';
import '../../../shared/widgets/hk_button_label.dart';
import '../../../shared/widgets/hk_sheet_drag_handle.dart';
import '../../../shared/widgets/main_top_bar.dart';
import '../../../shared/widgets/product_card.dart';
import '../../../shared/widgets/product_grid_spec.dart';
import '../../../shared/widgets/top_bar_provider.dart';
import '../../../shared/widgets/truck_loading_overlay.dart';
import '../../product/presentation/merchant_item_detail_screen.dart';
import '../../visit/presentation/favorite_products.dart';
import '../data/category_model.dart';
import '../data/category_repository.dart';
import '../data/product_item_model.dart';
import '../../home/presentation/home_widget/product_add_to_cart_sheet.dart';
import '../../cart/presentation/widgets/temperature_tags.dart';
import 'package:hawkr_buyer/shared/widgets/hk_brand_loader.dart';

/// Hawkr 市集商品列表。
///
/// 行動版互動與 Web `/market/products` 對齊：搜尋、L1/L2/L3 多選分類、
/// 溫層、價格、認證、熱門 hashtag、排序與無限載入共用
/// `GET /public/products` contract。
class CategoryScreen extends ConsumerStatefulWidget {
  const CategoryScreen({
    super.key,
    this.initialType,
    this.initialKeyword = '',
    this.initialSort = '',
  });
  final String? initialType;
  final String initialKeyword;
  final String initialSort;

  @override
  ConsumerState<CategoryScreen> createState() => _CategoryScreenState();
}

class _CategoryScreenState extends ConsumerState<CategoryScreen>
    with TopBarProvider {
  // Category tree
  List<Category> _l1Categories = [];
  Set<String> _selectedCategoryIds = <String>{};
  List<String> _popularHashtags = const [];
  List<MarketSellerPreview> _recommendedSellers = const [];
  _MarketStoryTab _storyTab = _MarketStoryTab.recommended;

  // Products
  List<ProductItem> _products = [];
  int _total = 0;
  int _page = 1;
  bool _hasMore = true;

  // Seller info cache: storeId → (name, logo)
  final Map<String, ({String name, String logo})> _sellerCache = {};

  // Loading
  bool _loadingCategories = true;
  bool _loadingProducts = false;

  // Web /public/products filter contract.
  String _keyword = '';
  final TextEditingController _searchCtrl = TextEditingController();
  Timer? _searchDebounce;

  // Sort / delivery temperature / price range.
  String _sortBy = '';
  String _deliveryTemp = '';
  Set<String> _selectedHashtags = <String>{};
  Set<String> _selectedCertificationTypes = <String>{};
  String _priceMin = '';
  String _priceMax = '';
  String? _priceError;
  final TextEditingController _priceMinCtrl = TextEditingController();
  final TextEditingController _priceMaxCtrl = TextEditingController();

  // Ignore stale responses when a user changes filters quickly.
  int _productRequestEpoch = 0;

  final ScrollController _productScrollCtrl = ScrollController();

  // ─── Lifecycle ─────────────────────────────────────

  @override
  void initState() {
    super.initState();
    _keyword = normalizeMarketKeyword(widget.initialKeyword);
    _sortBy = normalizeMarketSort(widget.initialSort);
    _searchCtrl.text = _keyword;
    _loadCategories();
    _loadPopularHashtags();
    _loadRecommendedSellers();
    _productScrollCtrl.addListener(_onProductScroll);
  }

  @override
  void didUpdateWidget(covariant CategoryScreen oldWidget) {
    super.didUpdateWidget(oldWidget);
    final nextKeyword = normalizeMarketKeyword(widget.initialKeyword);
    final nextSort = normalizeMarketSort(widget.initialSort);
    if (nextKeyword == normalizeMarketKeyword(oldWidget.initialKeyword) &&
        nextSort == normalizeMarketSort(oldWidget.initialSort)) {
      return;
    }

    _searchDebounce?.cancel();
    _searchCtrl.value = TextEditingValue(
      text: nextKeyword,
      selection: TextSelection.collapsed(offset: nextKeyword.length),
    );
    setState(() {
      _keyword = nextKeyword;
      _sortBy = nextSort;
    });
    if (!_loadingCategories) {
      unawaited(_loadProducts(reset: true));
    }
  }

  @override
  void dispose() {
    _searchDebounce?.cancel();
    _searchCtrl.dispose();
    _priceMinCtrl.dispose();
    _priceMaxCtrl.dispose();
    _productScrollCtrl.dispose();
    super.dispose();
  }

  // ─── Top bar (TopBarProvider) ──────────────────────

  @override
  PreferredSizeWidget buildTopBar(BuildContext context) {
    return buildMainTopBar(context);
  }

  // ─── Data loading ─────────────────────────────────

  Future<void> _loadCategories() async {
    try {
      final repo = ref.read(categoryRepositoryProvider);
      final tree = await repo.fetchCategoryTree();
      if (!mounted) return;
      setState(() {
        _l1Categories = tree;
        _loadingCategories = false;
      });
      _loadProducts(reset: true);
    } catch (e) {
      HKLog.logE('[CategoryScreen] _loadCategories error: $e', e);
      if (!mounted) return;
      setState(() => _loadingCategories = false);
    }
  }

  Future<void> _loadPopularHashtags() async {
    try {
      final values = await ref
          .read(categoryRepositoryProvider)
          .fetchPopularHashtags();
      if (!mounted) return;
      setState(() => _popularHashtags = values);
    } catch (error) {
      // Hashtags are an optional enhancement. A transient failure must not
      // block the product list or the rest of the filters.
      HKLog.logE('[CategoryScreen] _loadPopularHashtags error: $error', error);
    }
  }

  Future<void> _loadRecommendedSellers() async {
    try {
      final sellers = await ref
          .read(categoryRepositoryProvider)
          .fetchRecommendedSellers();
      if (!mounted) return;
      setState(() => _recommendedSellers = sellers);
    } catch (error) {
      // The seller rail is independently streamed on Web too. Keep product
      // discovery usable when the marketplace aggregation endpoint is down.
      HKLog.logE(
        '[CategoryScreen] _loadRecommendedSellers error: $error',
        error,
      );
    }
  }

  String? get _selectedCategoryId =>
      _selectedCategoryIds.isEmpty ? null : _selectedCategoryIds.join(',');

  Future<void> _loadProducts({bool reset = false}) async {
    if (_loadingProducts && !reset) return;
    final categoryId = _selectedCategoryId;
    final requestEpoch = reset ? ++_productRequestEpoch : _productRequestEpoch;
    final requestedPage = reset ? 1 : _page;

    if (reset) {
      _page = 1;
      _hasMore = true;
      if (mounted) {
        setState(() {
          _products = [];
          _loadingProducts = true;
        });
      }
    } else {
      if (!_hasMore) return;
      if (mounted) setState(() => _loadingProducts = true);
    }

    try {
      final repo = ref.read(categoryRepositoryProvider);
      final result = await repo.fetchProducts(
        categoryId: categoryId,
        page: requestedPage,
        pageSize: 20,
        keyword: _keyword.isEmpty ? null : _keyword,
        sort: _sortBy.isEmpty ? null : _sortBy,
        deliveryTemp: _deliveryTemp.isEmpty ? null : _deliveryTemp,
        priceMin: _priceMin.isEmpty ? null : _priceMin,
        priceMax: _priceMax.isEmpty ? null : _priceMax,
        hashtags: _selectedHashtags.toList(growable: false),
        certificationTypes: _selectedCertificationTypes.toList(growable: false),
        followedSellerOnly: _storyTab == _MarketStoryTab.following,
      );

      if (!mounted || requestEpoch != _productRequestEpoch) return;
      final startIndex = reset ? 0 : _products.length;
      setState(() {
        if (reset) {
          _products = result.records;
        } else {
          _products.addAll(result.records);
        }
        _total = result.total;
        _page = requestedPage + 1;
        _hasMore = result.records.isNotEmpty && _products.length < result.total;
        _loadingProducts = false;
      });

      unawaited(
        _loadFavorites(
          result.records
              .map((product) => int.tryParse(product.id))
              .whereType<int>(),
        ),
      );

      // GA4: view_item_list — fired once per loaded page, not on every rebuild
      if (result.records.isNotEmpty) {
        final listId =
            'category_${_selectedCategoryId ?? widget.initialType ?? 'all'}';
        final ga4Items = result.records.indexed
            .map(
              (e) => GA4Item(
                itemId: e.$2.id,
                itemName: e.$2.name,
                price: double.tryParse(e.$2.priceMin ?? ''),
                itemBrand: e.$2.sellerName,
                itemCategory: e.$2.categoryId?.toString(),
                index: startIndex + e.$1,
              ),
            )
            .toList();
        ref
            .read(analyticsEventsProvider)
            .trackViewItemList(ga4Items, listId: listId, listName: '市集');
      }

      // Batch-fetch seller info for new products.
      _fetchSellerInfoForProducts(result.records);
    } catch (e) {
      HKLog.logE('[CategoryScreen] _loadProducts error: $e', e);
      if (!mounted || requestEpoch != _productRequestEpoch) return;
      setState(() => _loadingProducts = false);
    }
  }

  /// Fetch seller info for products that have storeId but aren't cached yet.
  Future<void> _fetchSellerInfoForProducts(List<ProductItem> products) async {
    final repo = ref.read(categoryRepositoryProvider);
    final uncachedIds = products
        .map((p) => p.storeId)
        .where(
          (id) => id != null && id.isNotEmpty && !_sellerCache.containsKey(id),
        )
        .toSet();

    if (uncachedIds.isEmpty) return;

    // Fetch in parallel
    final futures = uncachedIds.map((id) async {
      try {
        final info = await repo.fetchSellerInfo(id!);
        return MapEntry(id, info);
      } catch (e) {
        HKLog.logE('[CategoryScreen] fetchSellerInfo($id) error: $e', e);
        return MapEntry(id!, (name: '', logo: ''));
      }
    });

    final results = await Future.wait(futures);
    if (!mounted) return;

    setState(() {
      for (final entry in results) {
        _sellerCache[entry.key] = entry.value;
      }
    });
  }

  Future<void> _loadFavorites(Iterable<int> productIds) async {
    try {
      await ref.read(favoriteProductsProvider).check(productIds);
    } catch (error) {
      HKLog.logE('[CategoryScreen] _loadFavorites error: $error', error);
    }
  }

  Future<void> _toggleFavorite(String productId) async {
    if (!await requireLogin(context, ref)) return;
    if (!mounted) return;
    final skuId = int.tryParse(productId);
    if (skuId == null) return;
    try {
      await ref
          .read(favoriteProductsProvider)
          .toggle(
            skuId,
            current: ref.read(favoriteProductProvider(skuId)) ?? false,
          );
    } catch (error) {
      HKLog.logE(
        '[CategoryScreen] _toggleFavorite($skuId) error: $error',
        error,
      );
    }
  }

  void _onProductScroll() {
    if (_productScrollCtrl.position.pixels >=
        _productScrollCtrl.position.maxScrollExtent - 200) {
      if (!_loadingProducts && _hasMore) {
        _loadProducts();
      }
    }
  }

  void _setCategoryIds(Set<String> categoryIds) {
    if (categoryIds.length == _selectedCategoryIds.length &&
        categoryIds.containsAll(_selectedCategoryIds)) {
      return;
    }
    setState(() => _selectedCategoryIds = Set<String>.from(categoryIds));
    _loadProducts(reset: true);
  }

  void _toggleHashtag(String hashtag) {
    final next = Set<String>.from(_selectedHashtags);
    next.contains(hashtag) ? next.remove(hashtag) : next.add(hashtag);
    setState(() => _selectedHashtags = next);
    _loadProducts(reset: true);
  }

  void _toggleCertification(String certificationType) {
    final next = Set<String>.from(_selectedCertificationTypes);
    next.contains(certificationType)
        ? next.remove(certificationType)
        : next.add(certificationType);
    setState(() => _selectedCertificationTypes = next);
    _loadProducts(reset: true);
  }

  void _onSearchChanged(String query) {
    final trimmed = query.trim();
    if (trimmed == _keyword) return;
    setState(() => _keyword = trimmed);
    _searchDebounce?.cancel();
    _searchDebounce = Timer(
      const Duration(milliseconds: 350),
      () => _loadProducts(reset: true),
    );
  }

  void _submitSearch(String query) {
    _searchDebounce?.cancel();
    final trimmed = query.trim();
    if (trimmed != _keyword) setState(() => _keyword = trimmed);
    _loadProducts(reset: true);
  }

  void _setSort(String sort) {
    if (sort == _sortBy) return;
    setState(() => _sortBy = sort);
    _loadProducts(reset: true);
  }

  Future<void> _setStoryTab(_MarketStoryTab tab) async {
    if (tab == _storyTab) return;
    if (tab == _MarketStoryTab.following) {
      if (!await requireLogin(context, ref)) return;
      if (!mounted) return;
    }
    final changesProductScope =
        tab == _MarketStoryTab.following ||
        _storyTab == _MarketStoryTab.following;
    setState(() => _storyTab = tab);
    if (changesProductScope) _loadProducts(reset: true);
  }

  void _setDeliveryTemp(String temp) {
    final next = temp == _deliveryTemp ? '' : temp;
    setState(() => _deliveryTemp = next);
    _loadProducts(reset: true);
  }

  void _applyPriceRange() {
    final minText = _priceMinCtrl.text.trim();
    final maxText = _priceMaxCtrl.text.trim();
    final min = minText.isEmpty ? null : num.tryParse(minText);
    final max = maxText.isEmpty ? null : num.tryParse(maxText);
    String? error;
    if ((minText.isNotEmpty && (min == null || min < 0)) ||
        (maxText.isNotEmpty && (max == null || max < 0))) {
      error = '請輸入有效的非負價格';
    } else if (min != null && max != null && min > max) {
      error = '最高價不可低於最低價';
    }
    if (error != null) {
      setState(() => _priceError = error);
      return;
    }
    setState(() {
      _priceMin = minText;
      _priceMax = maxText;
      _priceError = null;
    });
    _loadProducts(reset: true);
  }

  void _clearFilters() {
    _searchDebounce?.cancel();
    _searchCtrl.clear();
    _priceMinCtrl.clear();
    _priceMaxCtrl.clear();
    setState(() {
      _keyword = '';
      _selectedCategoryIds = <String>{};
      _deliveryTemp = '';
      _selectedHashtags = <String>{};
      _selectedCertificationTypes = <String>{};
      _priceMin = '';
      _priceMax = '';
      _priceError = null;
    });
    _loadProducts(reset: true);
  }

  Future<void> _openCategoryPicker() async {
    final selected = await showModalBottomSheet<Set<String>>(
      context: context,
      isScrollControlled: true,
      useSafeArea: true,
      showDragHandle: false,
      backgroundColor: Colors.transparent,
      builder: (context) => _CategoryPickerSheet(
        categories: _l1Categories,
        initiallySelected: _selectedCategoryIds,
      ),
    );
    if (selected != null && mounted) _setCategoryIds(selected);
  }

  Future<void> _openFilterSheet() async {
    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      useSafeArea: true,
      showDragHandle: false,
      backgroundColor: Colors.transparent,
      builder: (sheetContext) => FractionallySizedBox(
        heightFactor: 0.9,
        child: Material(
          key: const ValueKey('market-filter-sheet-surface'),
          color: AppColors.bgPrimary,
          borderRadius: const BorderRadius.vertical(
            top: Radius.circular(HkRadius.xl),
          ),
          clipBehavior: Clip.antiAlias,
          child: Column(
            children: [
              const HkSheetDragHandle(
                key: ValueKey('market-filter-drag-handle'),
              ),
              Expanded(
                child: StatefulBuilder(
                  builder: (context, setSheetState) => SingleChildScrollView(
                    padding: const EdgeInsets.fromLTRB(
                      HkSpace.md,
                      HkSpace.xs,
                      HkSpace.md,
                      HkSpace.md,
                    ),
                    child: _MarketFilterPanel(
                      expanded: true,
                      activeFilterLabels: _activeFilterLabels,
                      selectedCategoryCount: _selectedCategoryIds.length,
                      deliveryTemp: _deliveryTemp,
                      popularHashtags: _popularHashtags,
                      selectedHashtags: _selectedHashtags,
                      selectedCertificationTypes: _selectedCertificationTypes,
                      priceMinController: _priceMinCtrl,
                      priceMaxController: _priceMaxCtrl,
                      priceError: _priceError,
                      onToggleExpanded: () => Navigator.of(sheetContext).pop(),
                      onOpenCategoryPicker: () async {
                        await _openCategoryPicker();
                        setSheetState(() {});
                      },
                      onDeliveryTempSelected: (value) {
                        _setDeliveryTemp(value);
                        setSheetState(() {});
                      },
                      onHashtagSelected: (value) {
                        _toggleHashtag(value);
                        setSheetState(() {});
                      },
                      onCertificationSelected: (value) {
                        _toggleCertification(value);
                        setSheetState(() {});
                      },
                      onApplyPrice: () {
                        _applyPriceRange();
                        setSheetState(() {});
                      },
                      onClearAll: () {
                        _clearFilters();
                        setSheetState(() {});
                      },
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

  Future<void> _openSortSheet() async {
    final selected = await showModalBottomSheet<_MarketSortSelection>(
      context: context,
      useSafeArea: true,
      backgroundColor: Colors.transparent,
      isScrollControlled: true,
      showDragHandle: false,
      builder: (context) =>
          _MarketSortSheet(selectedStoryTab: _storyTab, selectedSort: _sortBy),
    );
    if (selected == null || !mounted) return;
    if (selected.storyTab != null) {
      await _setStoryTab(selected.storyTab!);
      return;
    }
    if (selected.sort != null) _setSort(selected.sort!);
  }

  // ─── Build ────────────────────────────────────────

  @override
  Widget build(BuildContext context) {
    if (_loadingCategories) {
      return const Scaffold(
        backgroundColor: AppColors.bgPrimary,
        body: TruckLoadingOverlay(),
      );
    }

    return Scaffold(
      backgroundColor: AppColors.bgPrimary,
      body: SafeArea(
        top: false,
        bottom: false,
        child: LayoutBuilder(
          builder: (context, constraints) {
            final horizontalInset = _marketHorizontalInset(
              constraints.maxWidth,
            );
            return CustomScrollView(
              key: const ValueKey('market-scroll-view'),
              controller: _productScrollCtrl,
              keyboardDismissBehavior: ScrollViewKeyboardDismissBehavior.onDrag,
              slivers: [
                SliverPadding(
                  padding: EdgeInsets.fromLTRB(
                    horizontalInset,
                    HkSpace.md,
                    horizontalInset,
                    HkSpace.sm,
                  ),
                  sliver: SliverToBoxAdapter(
                    child: CategorySearchField(
                      controller: _searchCtrl,
                      hintText: '搜尋番茄、雞蛋、虱目魚…',
                      semanticLabel: '市集商品搜尋',
                      semanticHint: '輸入商品名稱或關鍵字',
                      onChanged: _onSearchChanged,
                      onSubmitted: (value) {
                        FocusScope.of(context).unfocus();
                        _submitSearch(value);
                      },
                      suffixIcon: _keyword.isEmpty
                          ? null
                          : _ClearSearchButton(
                              onTap: () {
                                _searchCtrl.clear();
                                _submitSearch('');
                              },
                            ),
                    ),
                  ),
                ),
                SliverPadding(
                  padding: EdgeInsets.fromLTRB(
                    horizontalInset,
                    0,
                    horizontalInset,
                    HkSpace.sm,
                  ),
                  sliver: SliverToBoxAdapter(
                    child: _MarketResultsToolbar(
                      total: _total,
                      selectedSort: _sortBy,
                      selectedStoryTab: _storyTab,
                      loading: _loadingProducts,
                      activeFilterCount: _activeFilterLabels.length,
                      onFilterTap: _openFilterSheet,
                      onSortTap: _openSortSheet,
                    ),
                  ),
                ),
                if (_recommendedSellers.isNotEmpty)
                  SliverToBoxAdapter(
                    child: _MarketSellerRail(
                      sellers: _recommendedSellers,
                      onSellerSelected: (seller) =>
                          context.push('/merchant/${seller.id}'),
                    ),
                  ),
                ..._buildProductSlivers(),
                const BuyerNavigationClearanceSliver(
                  key: ValueKey('market-navigation-clearance'),
                ),
              ],
            );
          },
        ),
      ),
    );
  }

  double _marketHorizontalInset(double width) {
    final base = HkBreakpoints.pageGutter(width);
    final centred = (width - HkBreakpoints.contentMaxWidth(width)) / 2;
    return base > centred ? base : centred;
  }

  List<String> get _activeFilterLabels {
    final labels = <String>[];
    if (_keyword.isNotEmpty) labels.add('關鍵字：$_keyword');
    for (final categoryId in _selectedCategoryIds) {
      final path = _findCategoryPath(_l1Categories, categoryId);
      labels.add('分類：${path?.join(' › ') ?? categoryId}');
    }
    final tempLabel = _deliveryTempOptions
        .where((option) => option.value == _deliveryTemp)
        .map((option) => option.label)
        .firstOrNull;
    if (tempLabel != null) labels.add('溫層：$tempLabel');
    if (_priceMin.isNotEmpty || _priceMax.isNotEmpty) {
      labels.add(
        '價格：${_priceMin.isEmpty ? '0' : _priceMin}–${_priceMax.isEmpty ? '不限' : _priceMax}',
      );
    }
    for (final hashtag in _selectedHashtags) {
      labels.add('#$hashtag');
    }
    for (final certification in _selectedCertificationTypes) {
      final label = _certificationOptions
          .where((option) => option.value == certification)
          .map((option) => option.label)
          .firstOrNull;
      labels.add('認證：${label ?? certification}');
    }
    return labels;
  }

  List<Widget> _buildProductSlivers() {
    if (_products.isEmpty && _loadingProducts) {
      return const [
        SliverFillRemaining(hasScrollBody: false, child: TruckLoadingOverlay()),
      ];
    }
    if (_products.isEmpty) {
      return [
        SliverFillRemaining(
          hasScrollBody: false,
          child: const HkEmptyState(
            key: ValueKey('market-products-empty-state'),
            iconKey: ValueKey('market-products-empty-state-icon'),
            icon: HkIconName.emptyProducts,
            message: '目前沒有符合條件的商品',
            description: '換個分類、價格範圍或搜尋關鍵字再試一次。',
          ),
        ),
      ];
    }

    return [
      SliverLayoutBuilder(
        builder: (context, constraints) {
          final spec = ProductGridSpec.forWidth(
            constraints.crossAxisExtent,
            textScale: ProductCard.effectiveTextScale(context),
          );
          return SliverPadding(
            padding: EdgeInsets.fromLTRB(
              spec.horizontalInset,
              0,
              spec.horizontalInset,
              spec.mainAxisSpacing,
            ),
            sliver: SliverGrid.builder(
              key: const ValueKey('market-product-grid'),
              gridDelegate: spec.delegate,
              itemCount: _products.length,
              itemBuilder: (context, index) =>
                  _buildProductCard(_products[index], index),
            ),
          );
        },
      ),
      if (_loadingProducts)
        const SliverToBoxAdapter(
          child: Padding(
            padding: EdgeInsets.all(HkSpace.lg),
            child: Center(
              child: SizedBox.square(dimension: 24, child: HkBrandLoader()),
            ),
          ),
        )
      else if (!_hasMore)
        const SliverToBoxAdapter(
          child: Padding(
            padding: EdgeInsets.all(HkSpace.lg),
            child: Center(
              child: Text(
                '已顯示全部商品',
                style: TextStyle(fontSize: 12, color: AppColors.textSecondary),
              ),
            ),
          ),
        ),
    ];
  }

  Widget _buildProductCard(ProductItem product, int index) {
    final sellerInfo = product.storeId != null
        ? _sellerCache[product.storeId]
        : null;
    final sellerName = product.sellerName ?? sellerInfo?.name ?? '';
    final sellerAvatar = product.sellerAvatar ?? sellerInfo?.logo ?? '';
    final heroTag = product.id.isEmpty
        ? null
        : 'product-card:category:${product.id}';
    final favoriteId = int.tryParse(product.id);
    return Consumer(
      builder: (_, favoriteRef, _) => ProductCard(
        key: ValueKey('market-product-${product.id}'),
        name: product.name,
        imageUrl: product.displayImage,
        price: product.priceMin ?? '',
        priceMax: product.priceMax ?? '',
        listPriceMin: product.listPriceMin ?? '',
        listPriceMax: product.listPriceMax ?? '',
        sellerName: sellerName,
        sellerAvatarUrl: sellerAvatar,
        hashtags: product.hashtags,
        heroTag: heroTag,
        isSoldOut: product.isSoldOut,
        isFavorited:
            favoriteId != null &&
            (favoriteRef.watch(favoriteProductProvider(favoriteId)) ?? false),
        favoriteSurfaceStyle: ProductFavoriteSurfaceStyle.hashtag,
        onFavoriteTap: product.id.isEmpty
            ? null
            : () => _toggleFavorite(product.id),
        onTap: () {
          final listId =
              'category_${_selectedCategoryId ?? widget.initialType ?? 'all'}';
          ref
              .read(analyticsEventsProvider)
              .trackSelectItem(
                GA4Item(
                  itemId: product.id,
                  itemName: product.name,
                  price: double.tryParse(product.priceMin ?? ''),
                  itemBrand: product.sellerName,
                  itemCategory: product.categoryId?.toString(),
                  index: index,
                ),
                listId: listId,
                listName: '市集',
              );
          context.push(
            '/merchant-product',
            extra: MerchantItemDetailScreen(
              title: product.name,
              imageUrl: product.displayImage.isNotEmpty
                  ? product.displayImage
                  : null,
              price: double.tryParse(product.priceMin ?? ''),
              skuId: product.id,
              heroTag: heroTag,
              merchantId: product.storeId,
              merchantName: sellerName,
            ),
          );
        },
        onAddToCartTap: product.id.isEmpty
            ? null
            : () => showProductAddToCartSheet(
                context,
                ref,
                skuId: product.id,
                productName: product.name,
                imageUrl: product.displayImage,
                initialPrice: double.tryParse(product.priceMin ?? ''),
              ),
      ),
    );
  }
}

class _FilterOption {
  final String value;
  final String label;
  const _FilterOption(this.value, this.label);
}

const _sortOptions = [
  _FilterOption('', '綜合推薦'),
  _FilterOption('LATEST', '最新上架'),
  _FilterOption('SALES', '銷量最高'),
  _FilterOption('PRICE_ASC', '價格低到高'),
  _FilterOption('PRICE_DESC', '價格高到低'),
  _FilterOption('RATING', '評分最高'),
];

const _deliveryTempOptions = [
  _FilterOption('3201', '常溫'),
  _FilterOption('3202', '冷藏'),
  _FilterOption('3203', '冷凍'),
];

const _certificationOptions = [
  _FilterOption('3701', '有機農產品'),
  _FilterOption('3702', '產銷履歷'),
  _FilterOption('3703', 'CAS'),
  _FilterOption('3704', 'HACCP'),
  _FilterOption('3705', 'ISO'),
  _FilterOption('3706', 'GMP'),
];

enum _MarketStoryTab {
  recommended('推薦'),
  following('追蹤中'),
  nearby('附近');

  const _MarketStoryTab(this.label);
  final String label;
}

List<String>? _findCategoryPath(
  List<Category> categories,
  String categoryId, [
  List<String> ancestors = const [],
]) {
  for (final category in categories) {
    final path = [...ancestors, category.name];
    if (category.id == categoryId) return path;
    final nested = _findCategoryPath(category.children, categoryId, path);
    if (nested != null) return nested;
  }
  return null;
}

class _MarketSellerRail extends StatelessWidget {
  const _MarketSellerRail({
    required this.sellers,
    required this.onSellerSelected,
  });

  final List<MarketSellerPreview> sellers;
  final ValueChanged<MarketSellerPreview> onSellerSelected;

  @override
  Widget build(BuildContext context) {
    return Padding(
      key: const ValueKey('market-seller-stories'),
      padding: const EdgeInsets.only(top: HkSpace.sm, bottom: HkSpace.xs),
      child: SizedBox(
        height: 102,
        child: ListView.separated(
          key: const ValueKey('market-seller-story-list'),
          scrollDirection: Axis.horizontal,
          padding: const EdgeInsets.symmetric(horizontal: HkSpace.md),
          itemCount: sellers.length,
          separatorBuilder: (_, __) => const SizedBox(width: HkSpace.sm),
          itemBuilder: (context, index) {
            final seller = sellers[index];
            return Semantics(
              button: true,
              label: '前往商家 ${seller.name}',
              child: InkWell(
                key: ValueKey('market-seller-story-${seller.id}'),
                borderRadius: BorderRadius.circular(HkRadius.md),
                onTap: () => onSellerSelected(seller),
                child: SizedBox(
                  width: 76,
                  child: Column(
                    children: [
                      Container(
                        key: ValueKey('market-seller-ring-${seller.id}'),
                        width: 68,
                        height: 68,
                        padding: const EdgeInsets.all(1.5),
                        decoration: BoxDecoration(
                          shape: BoxShape.circle,
                          color: AppColors.neutralWhite.withValues(alpha: 0.72),
                          border: Border.all(
                            color: AppColors.neutralWhite.withValues(
                              alpha: 0.88,
                            ),
                            width: 1.5,
                          ),
                        ),
                        child: ClipOval(
                          child: seller.avatarUrl.isEmpty
                              ? Image.asset(
                                  'assets/hawkrlogo.png',
                                  fit: BoxFit.cover,
                                )
                              : AppCachedImage(
                                  imageUrl: seller.avatarUrl,
                                  width: 65,
                                  height: 65,
                                  fit: BoxFit.cover,
                                  scene: ImageScene.merchantAvatar,
                                  errorWidget: Image.asset(
                                    'assets/hawkrlogo.png',
                                    fit: BoxFit.cover,
                                  ),
                                ),
                        ),
                      ),
                      const SizedBox(height: 4),
                      Text(
                        seller.name,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        textAlign: TextAlign.center,
                        style: const TextStyle(
                          fontSize: 12,
                          color: AppColors.textPrimary,
                        ),
                      ),
                    ],
                  ),
                ),
              ),
            );
          },
        ),
      ),
    );
  }
}

class _ClearSearchButton extends StatelessWidget {
  const _ClearSearchButton({required this.onTap});

  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Semantics(
      label: '清除搜尋',
      button: true,
      child: InkWell(
        key: const ValueKey('market-clear-search'),
        borderRadius: BorderRadius.circular(HkRadius.pill),
        onTap: onTap,
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
    );
  }
}

class _MarketFilterPanel extends StatelessWidget {
  const _MarketFilterPanel({
    required this.expanded,
    required this.activeFilterLabels,
    required this.selectedCategoryCount,
    required this.deliveryTemp,
    required this.popularHashtags,
    required this.selectedHashtags,
    required this.selectedCertificationTypes,
    required this.priceMinController,
    required this.priceMaxController,
    required this.priceError,
    required this.onToggleExpanded,
    required this.onOpenCategoryPicker,
    required this.onDeliveryTempSelected,
    required this.onHashtagSelected,
    required this.onCertificationSelected,
    required this.onApplyPrice,
    required this.onClearAll,
  });

  final bool expanded;
  final List<String> activeFilterLabels;
  final int selectedCategoryCount;
  final String deliveryTemp;
  final List<String> popularHashtags;
  final Set<String> selectedHashtags;
  final Set<String> selectedCertificationTypes;
  final TextEditingController priceMinController;
  final TextEditingController priceMaxController;
  final String? priceError;
  final VoidCallback onToggleExpanded;
  final VoidCallback onOpenCategoryPicker;
  final ValueChanged<String> onDeliveryTempSelected;
  final ValueChanged<String> onHashtagSelected;
  final ValueChanged<String> onCertificationSelected;
  final VoidCallback onApplyPrice;
  final VoidCallback onClearAll;

  @override
  Widget build(BuildContext context) {
    final activeCount = activeFilterLabels.length;
    return Material(
      key: const ValueKey('market-filter-panel'),
      color: AppColors.bgCard,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(HkRadius.lg),
        side: const BorderSide(color: AppColors.borderSubtle),
      ),
      child: Column(
        children: [
          InkWell(
            key: const ValueKey('market-filter-toggle'),
            borderRadius: BorderRadius.circular(HkRadius.lg),
            onTap: onToggleExpanded,
            child: Padding(
              padding: const EdgeInsets.symmetric(
                horizontal: HkSpace.lg,
                vertical: HkSpace.md,
              ),
              child: Row(
                children: [
                  const HkIcon(
                    HkIconName.filter,
                    size: HkControl.icon,
                    color: AppIconColors.active,
                  ),
                  const SizedBox(width: HkSpace.sm),
                  const Expanded(
                    child: Text(
                      '篩選與分類',
                      style: TextStyle(
                        fontSize: 16,
                        fontWeight: FontWeight.w700,
                        color: AppColors.textEmphasis,
                      ),
                    ),
                  ),
                  if (activeCount > 0)
                    Container(
                      key: const ValueKey('market-active-filter-count'),
                      constraints: const BoxConstraints(minWidth: 24),
                      padding: const EdgeInsets.symmetric(
                        horizontal: HkSpace.sm,
                        vertical: HkSpace.xs,
                      ),
                      decoration: BoxDecoration(
                        color: AppColors.primarySoft,
                        borderRadius: BorderRadius.circular(HkRadius.pill),
                      ),
                      alignment: Alignment.center,
                      child: Text(
                        '$activeCount',
                        style: const TextStyle(
                          fontSize: 12,
                          fontWeight: FontWeight.w700,
                          color: AppColors.textAccentOnTint,
                        ),
                      ),
                    ),
                  const SizedBox(width: HkSpace.sm),
                  HkIcon(
                    expanded ? HkIconName.chevronUp : HkIconName.chevronDown,
                    size: 20,
                    color: AppIconColors.muted,
                  ),
                ],
              ),
            ),
          ),
          AnimatedSize(
            duration: HkMotion.duration,
            curve: HkMotion.ease,
            alignment: Alignment.topCenter,
            child: expanded
                ? Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      const Divider(height: 1, color: AppColors.borderSubtle),
                      Padding(
                        padding: const EdgeInsets.all(HkSpace.lg),
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            if (activeCount > 0) ...[
                              Row(
                                children: [
                                  const Expanded(
                                    child: Text(
                                      '已選篩選',
                                      style: TextStyle(
                                        fontSize: 12,
                                        fontWeight: FontWeight.w700,
                                        color: AppColors.textSecondary,
                                      ),
                                    ),
                                  ),
                                  TextButton(
                                    key: const ValueKey(
                                      'market-clear-all-filters',
                                    ),
                                    onPressed: onClearAll,
                                    child: const HkButtonLabel('清除全部'),
                                  ),
                                ],
                              ),
                              Wrap(
                                spacing: HkSpace.xs,
                                runSpacing: HkSpace.xs,
                                children: [
                                  for (final label in activeFilterLabels)
                                    _AppliedFilterChip(label: label),
                                ],
                              ),
                              const SizedBox(height: HkSpace.lg),
                            ],
                            _FilterSection(
                              title: '分類',
                              child: OutlinedButton(
                                key: const ValueKey('market-category-picker'),
                                onPressed: onOpenCategoryPicker,
                                style: OutlinedButton.styleFrom(
                                  minimumSize: const Size.fromHeight(
                                    HkControl.minTap,
                                  ),
                                  alignment: Alignment.centerLeft,
                                  foregroundColor: AppColors.textPrimary,
                                  side: const BorderSide(
                                    color: AppColors.borderSoft,
                                  ),
                                  shape: RoundedRectangleBorder(
                                    borderRadius: BorderRadius.circular(
                                      HkRadius.md,
                                    ),
                                  ),
                                ),
                                child: Row(
                                  children: [
                                    const Expanded(
                                      child: HkButtonLabel(
                                        '選擇商品分類',
                                        textAlign: TextAlign.start,
                                      ),
                                    ),
                                    if (selectedCategoryCount > 0)
                                      HkButtonLabel(
                                        '已選 $selectedCategoryCount 項',
                                        style: const TextStyle(
                                          color: AppColors.actionPrimary,
                                          fontWeight: FontWeight.w700,
                                        ),
                                      ),
                                    const SizedBox(width: HkSpace.xs),
                                    const HkIcon(
                                      HkIconName.chevronRight,
                                      size: 18,
                                      color: AppIconColors.muted,
                                    ),
                                  ],
                                ),
                              ),
                            ),
                            const SizedBox(height: HkSpace.lg),
                            _FilterSection(
                              title: '配送溫層',
                              child: Wrap(
                                spacing: HkSpace.sm,
                                runSpacing: HkSpace.sm,
                                children: [
                                  for (final option in _deliveryTempOptions)
                                    TemperatureChoiceChip(
                                      key: ValueKey(
                                        'market-temp-${option.value}',
                                      ),
                                      code: option.value,
                                      label: option.label,
                                      selected: deliveryTemp == option.value,
                                      onTap: () =>
                                          onDeliveryTempSelected(option.value),
                                    ),
                                ],
                              ),
                            ),
                            const SizedBox(height: HkSpace.lg),
                            _FilterSection(
                              title: '價格範圍',
                              child: _MarketPriceRange(
                                minimumController: priceMinController,
                                maximumController: priceMaxController,
                                errorText: priceError,
                                onApply: onApplyPrice,
                              ),
                            ),
                            const SizedBox(height: HkSpace.lg),
                            _FilterSection(
                              title: '認證',
                              child: Wrap(
                                spacing: HkSpace.sm,
                                runSpacing: HkSpace.sm,
                                children: [
                                  for (final option in _certificationOptions)
                                    HkChoiceChip(
                                      key: ValueKey(
                                        'market-cert-${option.value}',
                                      ),
                                      label: option.label,
                                      selected: selectedCertificationTypes
                                          .contains(option.value),
                                      onTap: () =>
                                          onCertificationSelected(option.value),
                                    ),
                                ],
                              ),
                            ),
                            if (popularHashtags.isNotEmpty) ...[
                              const SizedBox(height: HkSpace.lg),
                              _FilterSection(
                                title: '熱門 hashtag',
                                child: Wrap(
                                  spacing: HkSpace.sm,
                                  runSpacing: HkSpace.sm,
                                  children: [
                                    for (final hashtag in popularHashtags)
                                      HkChoiceChip(
                                        key: ValueKey(
                                          'market-hashtag-$hashtag',
                                        ),
                                        label: '#$hashtag',
                                        selected: selectedHashtags.contains(
                                          hashtag,
                                        ),
                                        onTap: () => onHashtagSelected(hashtag),
                                      ),
                                  ],
                                ),
                              ),
                            ],
                          ],
                        ),
                      ),
                    ],
                  )
                : const SizedBox.shrink(),
          ),
        ],
      ),
    );
  }
}

class _CategoryPickerSheet extends StatefulWidget {
  const _CategoryPickerSheet({
    required this.categories,
    required this.initiallySelected,
  });

  final List<Category> categories;
  final Set<String> initiallySelected;

  @override
  State<_CategoryPickerSheet> createState() => _CategoryPickerSheetState();
}

class _CategoryPickerSheetState extends State<_CategoryPickerSheet> {
  late Set<String> _selected;
  late Set<String> _expandedL2;
  int _activeL1 = 0;

  @override
  void initState() {
    super.initState();
    _selected = Set<String>.from(widget.initiallySelected);
    _expandedL2 = <String>{};
    for (final l1 in widget.categories) {
      for (final l2 in l1.children) {
        if (l2.children.any((l3) => _selected.contains(l3.id))) {
          _expandedL2.add(l2.id);
        }
      }
    }
  }

  void _toggleSelected(String id, {bool expandChildren = false}) {
    setState(() {
      if (_selected.remove(id)) {
        if (expandChildren) _expandedL2.remove(id);
      } else {
        _selected.add(id);
        if (expandChildren) _expandedL2.add(id);
      }
    });
  }

  int _selectedCount(Category category) {
    var count = _selected.contains(category.id) ? 1 : 0;
    for (final child in category.children) {
      count += _selectedCount(child);
    }
    return count;
  }

  @override
  Widget build(BuildContext context) {
    final categories = widget.categories;
    final active = categories.isEmpty
        ? null
        : categories[_activeL1.clamp(0, categories.length - 1)];
    return FractionallySizedBox(
      heightFactor: 0.86,
      child: Material(
        key: const ValueKey('market-category-sheet'),
        color: AppColors.bgCard,
        borderRadius: const BorderRadius.vertical(
          top: Radius.circular(HkRadius.xl),
        ),
        clipBehavior: Clip.antiAlias,
        child: Column(
          children: [
            const HkSheetDragHandle(
              key: ValueKey('market-category-drag-handle'),
            ),
            Padding(
              padding: const EdgeInsets.fromLTRB(
                HkSpace.lg,
                HkSpace.sm,
                HkSpace.sm,
                HkSpace.sm,
              ),
              child: Row(
                children: [
                  const Expanded(
                    child: Text(
                      '選擇商品分類',
                      style: TextStyle(
                        fontSize: 18,
                        fontWeight: FontWeight.w700,
                        color: AppColors.textEmphasis,
                      ),
                    ),
                  ),
                  IconButton(
                    tooltip: '關閉',
                    onPressed: () => Navigator.of(context).pop(),
                    icon: const HkIcon(
                      HkIconName.close,
                      color: AppIconColors.muted,
                    ),
                  ),
                ],
              ),
            ),
            const Divider(height: 1, color: AppColors.borderSubtle),
            Expanded(
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  SizedBox(
                    width: 112,
                    child: ColoredBox(
                      color: AppColors.primarySubtle,
                      child: ListView.builder(
                        padding: const EdgeInsets.all(HkSpace.sm),
                        itemCount: categories.length,
                        itemBuilder: (context, index) {
                          final category = categories[index];
                          final selected = index == _activeL1;
                          final count = _selectedCount(category);
                          return Padding(
                            padding: const EdgeInsets.only(bottom: HkSpace.xs),
                            child: Material(
                              color: selected
                                  ? AppColors.actionPrimary
                                  : Colors.transparent,
                              borderRadius: BorderRadius.circular(HkRadius.sm),
                              child: InkWell(
                                key: ValueKey(
                                  'market-category-l1-${category.id}',
                                ),
                                borderRadius: BorderRadius.circular(
                                  HkRadius.sm,
                                ),
                                onTap: () => setState(() => _activeL1 = index),
                                child: Padding(
                                  padding: const EdgeInsets.symmetric(
                                    horizontal: HkSpace.sm,
                                    vertical: HkSpace.sm,
                                  ),
                                  child: Row(
                                    children: [
                                      Expanded(
                                        child: Text(
                                          category.name,
                                          maxLines: 2,
                                          overflow: TextOverflow.ellipsis,
                                          style: TextStyle(
                                            fontSize: 14,
                                            fontWeight: FontWeight.w700,
                                            color: selected
                                                ? AppColors.textInverse
                                                : AppColors.textPrimary,
                                          ),
                                        ),
                                      ),
                                      if (count > 0)
                                        DecoratedBox(
                                          decoration: BoxDecoration(
                                            color: selected
                                                ? AppColors.bgCard.withValues(
                                                    alpha: 0.2,
                                                  )
                                                : AppColors.primarySoft,
                                            borderRadius: BorderRadius.circular(
                                              HkRadius.pill,
                                            ),
                                          ),
                                          child: Padding(
                                            padding: const EdgeInsets.all(
                                              HkSpace.xs,
                                            ),
                                            child: Text(
                                              '$count',
                                              style: TextStyle(
                                                fontSize: 12,
                                                fontWeight: FontWeight.w700,
                                                color: selected
                                                    ? AppColors.textInverse
                                                    : AppColors.actionPrimary,
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
                      ),
                    ),
                  ),
                  const VerticalDivider(
                    width: 1,
                    color: AppColors.borderSubtle,
                  ),
                  Expanded(
                    child: active == null || active.children.isEmpty
                        ? const Center(
                            child: Text(
                              '這個分類目前沒有子分類',
                              style: TextStyle(color: AppColors.textSecondary),
                            ),
                          )
                        : ListView.builder(
                            key: const ValueKey('market-category-l2-list'),
                            padding: const EdgeInsets.all(HkSpace.sm),
                            itemCount: active.children.length,
                            itemBuilder: (context, index) {
                              final l2 = active.children[index];
                              final expanded = _expandedL2.contains(l2.id);
                              return _CategoryOption(
                                category: l2,
                                selected: _selected.contains(l2.id),
                                expanded: expanded,
                                selectedChildren: l2.children
                                    .where((l3) => _selected.contains(l3.id))
                                    .length,
                                onToggle: () => _toggleSelected(
                                  l2.id,
                                  expandChildren: l2.children.isNotEmpty,
                                ),
                                onExpand: l2.children.isEmpty
                                    ? null
                                    : () => setState(() {
                                        expanded
                                            ? _expandedL2.remove(l2.id)
                                            : _expandedL2.add(l2.id);
                                      }),
                                children: expanded
                                    ? [
                                        for (final l3 in l2.children)
                                          _CategoryCheckboxRow(
                                            key: ValueKey(
                                              'market-category-l3-${l3.id}',
                                            ),
                                            label: l3.name,
                                            selected: _selected.contains(l3.id),
                                            nested: true,
                                            onChanged: () =>
                                                _toggleSelected(l3.id),
                                          ),
                                      ]
                                    : const [],
                              );
                            },
                          ),
                  ),
                ],
              ),
            ),
            const Divider(height: 1, color: AppColors.borderSubtle),
            Padding(
              padding: const EdgeInsets.fromLTRB(
                HkSpace.md,
                HkSpace.sm,
                HkSpace.md,
                HkSpace.md,
              ),
              child: Row(
                children: [
                  TextButton(
                    key: const ValueKey('market-category-clear'),
                    onPressed: _selected.isEmpty
                        ? null
                        : () => setState(() => _selected.clear()),
                    child: const HkButtonLabel('清除'),
                  ),
                  const SizedBox(width: HkSpace.xs),
                  Expanded(
                    child: Text(
                      '已選 ${_selected.length} 項',
                      style: const TextStyle(
                        fontSize: 14,
                        color: AppColors.textSecondary,
                      ),
                    ),
                  ),
                  FilledButton(
                    key: const ValueKey('market-category-done'),
                    onPressed: () =>
                        Navigator.of(context).pop(Set<String>.from(_selected)),
                    style: FilledButton.styleFrom(
                      backgroundColor: AppColors.actionPrimary,
                      foregroundColor: AppColors.textInverse,
                      minimumSize: const Size(96, HkControl.minTap),
                    ),
                    child: const HkButtonLabel('完成'),
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _CategoryOption extends StatelessWidget {
  const _CategoryOption({
    required this.category,
    required this.selected,
    required this.expanded,
    required this.selectedChildren,
    required this.onToggle,
    required this.onExpand,
    required this.children,
  });

  final Category category;
  final bool selected;
  final bool expanded;
  final int selectedChildren;
  final VoidCallback onToggle;
  final VoidCallback? onExpand;
  final List<Widget> children;

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        Row(
          children: [
            Expanded(
              child: _CategoryCheckboxRow(
                key: ValueKey('market-category-l2-${category.id}'),
                label: category.name,
                selected: selected,
                onChanged: onToggle,
              ),
            ),
            if (onExpand != null)
              IconButton(
                key: ValueKey('market-category-expand-${category.id}'),
                tooltip: expanded
                    ? '收合 ${category.name}'
                    : '展開 ${category.name}',
                onPressed: onExpand,
                icon: Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    if (selectedChildren > 0)
                      Text(
                        '$selectedChildren',
                        style: const TextStyle(
                          fontSize: 12,
                          fontWeight: FontWeight.w700,
                          color: AppColors.actionPrimary,
                        ),
                      ),
                    HkIcon(
                      expanded
                          ? HkIconName.chevronDown
                          : HkIconName.chevronRight,
                      size: 18,
                      color: AppIconColors.muted,
                    ),
                  ],
                ),
              ),
          ],
        ),
        if (children.isNotEmpty)
          Padding(
            padding: const EdgeInsets.only(left: HkSpace.md),
            child: Column(children: children),
          ),
      ],
    );
  }
}

class _CategoryCheckboxRow extends StatelessWidget {
  const _CategoryCheckboxRow({
    super.key,
    required this.label,
    required this.selected,
    required this.onChanged,
    this.nested = false,
  });

  final String label;
  final bool selected;
  final VoidCallback onChanged;
  final bool nested;

  @override
  Widget build(BuildContext context) {
    return Semantics(
      checked: selected,
      button: true,
      child: InkWell(
        onTap: onChanged,
        borderRadius: BorderRadius.circular(HkRadius.sm),
        child: ConstrainedBox(
          constraints: const BoxConstraints(minHeight: HkControl.minTap),
          child: Row(
            children: [
              Checkbox(
                value: selected,
                onChanged: (_) => onChanged(),
                visualDensity: VisualDensity.compact,
              ),
              Expanded(
                child: Text(
                  label,
                  style: TextStyle(
                    fontSize: 14,
                    fontWeight: selected ? FontWeight.w700 : FontWeight.w500,
                    color: selected
                        ? AppColors.actionPrimary
                        : AppColors.textPrimary,
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

class _FilterSection extends StatelessWidget {
  const _FilterSection({required this.title, required this.child});

  final String title;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          title,
          style: const TextStyle(
            fontSize: 14,
            height: 1.3,
            fontWeight: FontWeight.w700,
            color: AppColors.textSecondary,
          ),
        ),
        const SizedBox(height: HkSpace.sm),
        child,
      ],
    );
  }
}

class _AppliedFilterChip extends StatelessWidget {
  const _AppliedFilterChip({required this.label});

  final String label;

  @override
  Widget build(BuildContext context) {
    return DecoratedBox(
      decoration: BoxDecoration(
        color: AppColors.primarySubtle,
        border: Border.all(color: AppColors.borderSoft),
        borderRadius: BorderRadius.circular(HkRadius.pill),
      ),
      child: Padding(
        padding: const EdgeInsets.symmetric(
          horizontal: HkSpace.sm,
          vertical: HkSpace.xs,
        ),
        child: Text(
          label,
          style: const TextStyle(
            fontSize: 12,
            fontWeight: FontWeight.w500,
            color: AppColors.textPrimary,
          ),
        ),
      ),
    );
  }
}

class _MarketPriceRange extends StatelessWidget {
  const _MarketPriceRange({
    required this.minimumController,
    required this.maximumController,
    required this.errorText,
    required this.onApply,
  });

  final TextEditingController minimumController;
  final TextEditingController maximumController;
  final String? errorText;
  final VoidCallback onApply;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            Expanded(
              child: _PriceField(
                key: const ValueKey('market-price-min'),
                controller: minimumController,
                hintText: '最低',
              ),
            ),
            const Padding(
              padding: EdgeInsets.symmetric(horizontal: HkSpace.sm),
              child: Text(
                '–',
                style: TextStyle(color: AppColors.textSecondary),
              ),
            ),
            Expanded(
              child: _PriceField(
                key: const ValueKey('market-price-max'),
                controller: maximumController,
                hintText: '最高',
              ),
            ),
            const SizedBox(width: HkSpace.sm),
            SizedBox(
              height: HkControl.minTap,
              child: FilledButton(
                key: const ValueKey('market-apply-price'),
                onPressed: onApply,
                style: FilledButton.styleFrom(
                  backgroundColor: AppColors.actionPrimary,
                  foregroundColor: AppColors.textInverse,
                  padding: const EdgeInsets.symmetric(horizontal: HkSpace.md),
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(HkRadius.sm),
                  ),
                ),
                child: const HkButtonLabel('套用'),
              ),
            ),
          ],
        ),
        if (errorText != null) ...[
          const SizedBox(height: HkSpace.xs),
          Text(
            errorText!,
            key: const ValueKey('market-price-error'),
            style: const TextStyle(fontSize: 12, color: AppColors.danger),
          ),
        ],
      ],
    );
  }
}

class _PriceField extends StatelessWidget {
  const _PriceField({
    super.key,
    required this.controller,
    required this.hintText,
  });

  final TextEditingController controller;
  final String hintText;

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      height: HkControl.searchFieldHeight,
      child: TextField(
        controller: controller,
        keyboardType: const TextInputType.numberWithOptions(decimal: true),
        textInputAction: TextInputAction.next,
        style: const TextStyle(fontSize: 14, color: AppColors.textPrimary),
        decoration: InputDecoration(
          hintText: hintText,
          hintStyle: const TextStyle(color: AppColors.textHint),
          contentPadding: const EdgeInsets.symmetric(horizontal: HkSpace.sm),
          filled: true,
          fillColor: AppColors.bgPrimary,
          enabledBorder: OutlineInputBorder(
            borderRadius: BorderRadius.circular(HkRadius.sm),
            borderSide: const BorderSide(color: AppColors.borderInput),
          ),
          focusedBorder: OutlineInputBorder(
            borderRadius: BorderRadius.circular(HkRadius.sm),
            borderSide: const BorderSide(color: AppColors.actionPrimary),
          ),
        ),
      ),
    );
  }
}

class _MarketResultsToolbar extends StatelessWidget {
  const _MarketResultsToolbar({
    required this.total,
    required this.selectedSort,
    required this.selectedStoryTab,
    required this.loading,
    required this.activeFilterCount,
    required this.onFilterTap,
    required this.onSortTap,
  });

  final int total;
  final String selectedSort;
  final _MarketStoryTab selectedStoryTab;
  final bool loading;
  final int activeFilterCount;
  final VoidCallback onFilterTap;
  final VoidCallback onSortTap;

  @override
  Widget build(BuildContext context) {
    final browseAndSortCount =
        (selectedStoryTab == _MarketStoryTab.recommended ? 0 : 1) +
        (selectedSort.isEmpty ? 0 : 1);
    return SizedBox(
      key: const ValueKey('market-results-toolbar'),
      height: HkControl.minTap,
      child: Row(
        children: [
          Expanded(
            child: Text(
              '$total 件商品${loading ? '・載入中' : ''}',
              key: const ValueKey('market-result-count'),
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: AppTypography.metaMuted.copyWith(
                fontWeight: FontWeight.w700,
              ),
            ),
          ),
          HkChoiceChip(
            key: const ValueKey('market-filter-button'),
            label: activeFilterCount == 0 ? '篩選' : '篩選 $activeFilterCount',
            selected: activeFilterCount > 0,
            onTap: onFilterTap,
          ),
          const SizedBox(width: HkSpace.xs),
          HkChoiceChip(
            key: const ValueKey('market-sort-button'),
            label: browseAndSortCount == 0 ? '排序' : '排序 $browseAndSortCount',
            selected: browseAndSortCount > 0,
            onTap: onSortTap,
          ),
        ],
      ),
    );
  }
}

class _MarketSortSelection {
  const _MarketSortSelection.story(this.storyTab) : sort = null;
  const _MarketSortSelection.sort(this.sort) : storyTab = null;

  final _MarketStoryTab? storyTab;
  final String? sort;
}

class _MarketSortSheet extends StatelessWidget {
  const _MarketSortSheet({
    required this.selectedStoryTab,
    required this.selectedSort,
  });

  final _MarketStoryTab selectedStoryTab;
  final String selectedSort;

  @override
  Widget build(BuildContext context) {
    final sheetHeight = (MediaQuery.sizeOf(context).height * 0.72)
        .clamp(360.0, 600.0)
        .toDouble();
    return Material(
      key: const ValueKey('market-sort-sheet'),
      color: AppColors.bgCard,
      borderRadius: const BorderRadius.vertical(
        top: Radius.circular(HkRadius.xl),
      ),
      clipBehavior: Clip.antiAlias,
      child: SizedBox(
        height: sheetHeight,
        child: SafeArea(
          top: false,
          child: Column(
            children: [
              const HkSheetDragHandle(key: ValueKey('market-sort-drag-handle')),
              Padding(
                padding: const EdgeInsets.fromLTRB(
                  HkSpace.lg,
                  HkSpace.sm,
                  HkSpace.sm,
                  HkSpace.sm,
                ),
                child: Row(
                  children: [
                    Expanded(
                      child: Text('商品瀏覽與排序', style: AppTypography.sectionTitle),
                    ),
                    IconButton(
                      tooltip: '關閉',
                      onPressed: () => Navigator.of(context).pop(),
                      icon: const HkIcon(
                        HkIconName.close,
                        size: 20,
                        color: AppIconColors.strong,
                      ),
                    ),
                  ],
                ),
              ),
              const Divider(height: 1, color: AppColors.borderSubtle),
              Expanded(
                child: ListView(
                  key: const ValueKey('market-sort-options'),
                  padding: const EdgeInsets.fromLTRB(
                    HkSpace.lg,
                    HkSpace.md,
                    HkSpace.lg,
                    HkSpace.xxl,
                  ),
                  children: [
                    const _MarketSortSectionHeader(
                      title: '瀏覽範圍',
                      description: '選擇要瀏覽的商品來源',
                    ),
                    const SizedBox(height: HkSpace.sm),
                    Wrap(
                      key: const ValueKey('market-browse-options'),
                      // 與同一張表單裡的其他選擇列一致（認證／hashtag 都是 sm），
                      // 也對齊網頁的 --hkui-filter-gap-sm。4px 在 32px 膠囊之間
                      // 會讀成整排黏在一起。
                      spacing: HkSpace.sm,
                      runSpacing: HkSpace.sm,
                      children: [
                        for (final tab in _MarketStoryTab.values)
                          HkChoiceChip(
                            key: ValueKey('market-browse-${tab.name}'),
                            label: tab.label,
                            selected: tab == selectedStoryTab,
                            onTap: () => Navigator.of(
                              context,
                            ).pop(_MarketSortSelection.story(tab)),
                          ),
                      ],
                    ),
                    const Padding(
                      padding: EdgeInsets.symmetric(vertical: HkSpace.md),
                      child: Divider(height: 1, color: AppColors.borderSubtle),
                    ),
                    const _MarketSortSectionHeader(
                      title: '排序方式',
                      description: '調整商品的顯示順序',
                    ),
                    const SizedBox(height: HkSpace.sm),
                    for (final option in _sortOptions) ...[
                      _MarketSortOptionTile(
                        key: ValueKey(
                          'market-sort-${option.value.isEmpty ? 'recommended' : option.value.toLowerCase()}',
                        ),
                        label: option.label,
                        selected: option.value == selectedSort,
                        onTap: () => Navigator.of(
                          context,
                        ).pop(_MarketSortSelection.sort(option.value)),
                      ),
                      if (option != _sortOptions.last)
                        const SizedBox(height: HkSpace.xs),
                    ],
                  ],
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _MarketSortSectionHeader extends StatelessWidget {
  const _MarketSortSectionHeader({
    required this.title,
    required this.description,
  });

  final String title;
  final String description;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          title,
          style: AppTypography.meta.copyWith(
            color: AppColors.textEmphasis,
            fontWeight: FontWeight.w700,
          ),
        ),
        const SizedBox(height: HkSpace.xs),
        Text(description, style: AppTypography.caption),
      ],
    );
  }
}

class _MarketSortOptionTile extends StatelessWidget {
  const _MarketSortOptionTile({
    super.key,
    required this.label,
    required this.selected,
    required this.onTap,
  });

  final String label;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Semantics(
      button: true,
      selected: selected,
      label: selected ? '$label，已選取' : label,
      child: Material(
        color: selected ? AppColors.primarySubtle : Colors.transparent,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(HkRadius.md),
          side: BorderSide(
            color: selected ? AppColors.borderSoft : Colors.transparent,
          ),
        ),
        clipBehavior: Clip.antiAlias,
        child: InkWell(
          onTap: onTap,
          child: ConstrainedBox(
            constraints: const BoxConstraints(minHeight: HkControl.minTap),
            child: Padding(
              padding: const EdgeInsets.symmetric(horizontal: HkSpace.md),
              child: Row(
                children: [
                  Expanded(
                    child: Text(
                      label,
                      style: AppTypography.body.copyWith(
                        color: selected
                            ? AppColors.actionPrimary
                            : AppColors.textPrimary,
                        fontWeight: selected
                            ? FontWeight.w700
                            : FontWeight.w400,
                      ),
                    ),
                  ),
                  if (selected)
                    const HkIcon(
                      HkIconName.check,
                      size: 20,
                      color: AppIconColors.active,
                    ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}
