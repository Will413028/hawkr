import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/app_typography.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'dart:async';
import 'dart:math' as math;
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../core/networking/dio_provider.dart';
import '../../../core/theme/hk_tokens.dart';
import '../../../config/image_config.dart';
import '../../../shared/widgets/app_cached_image.dart';
import '../../../shared/widgets/category_top_bar.dart';
import '../../../shared/widgets/hk_tap_target.dart';
import '../../../shared/utils/price_display.dart';
import 'merchant_item_detail_screen.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:hawkr_buyer/shared/widgets/hk_skeleton_shimmer.dart';

/// Search 畫面：AI 智能推薦 + 搜尋建議
class SearchScreen extends HookConsumerWidget {
  const SearchScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final controller = useTextEditingController();
    // Trigger rebuild on text changes
    useListenable(controller);
    final isSearching = useState(false);
    final hasSearched = useState(false);
    final resultsError = useState<String?>(null);
    final results = useState<List<_SearchResult>>([]);
    final searchSerial = useRef(0);
    final searchDebounce = useRef<Timer?>(null);
    useEffect(
      () =>
          () => searchDebounce.value?.cancel(),
      const [],
    );

    // 進場預載：個人化商品推薦
    final recommendedProducts = useState<List<_SearchResult>>([]);
    final isLoadingRecommended = useState(false);

    final dio = ref.watch(dioProvider);
    useEffect(() {
      _loadRecommendedProducts(
        isLoading: isLoadingRecommended,
        products: recommendedProducts,
        dio: dio,
      );
      return null;
    }, const []);

    Future<void> performSearch(String raw) async {
      final keyword = raw.trim();
      if (keyword.isEmpty) {
        results.value = [];
        isSearching.value = false;
        hasSearched.value = false;
        resultsError.value = null;
        return;
      }

      final requestId = ++searchSerial.value;
      isSearching.value = true;
      hasSearched.value = true;
      resultsError.value = null;

      try {
        final mapped = await _fetchSearchResults(
          keyword: keyword,
          controller: controller,
          requestId: requestId,
          searchSerial: searchSerial,
          dio: dio,
        );

        if (mapped == null) return; // stale or context gone
        results.value = mapped;
        isSearching.value = false;
      } catch (_) {
        if (searchSerial.value != requestId) return;
        isSearching.value = false;
        resultsError.value = '搜尋失敗，請稍後再試';
      }
    }

    void onChanged(String text) {
      final kw = text.trim();
      if (kw.isEmpty) {
        searchDebounce.value?.cancel();
        results.value = [];
        isSearching.value = false;
        hasSearched.value = false;
        resultsError.value = null;
        return;
      }
      hasSearched.value = false;
      isSearching.value = false;
      resultsError.value = null;
      results.value = [];

      // 注音 IME 中間狀態（含 ㄅ-ㄦ 或聲調符號 ˊˇˋ˙ˉ）直接跳過，不打 API。
      // 之前每按一個鍵 — 「ㄈ → ㄈㄢ → ㄈㄢˉ → ㄈㄢˉ ㄑ ...」— 都會丟一次
      // 搜尋出去，全部查無結果。
      if (_isImeComposingText(kw)) {
        searchDebounce.value?.cancel();
        return;
      }

      // 一般輸入 debounce ~400ms，避免每個 keystroke 都打 API。
      searchDebounce.value?.cancel();
      searchDebounce.value = Timer(const Duration(milliseconds: 400), () {
        // 使用者已經繼續打或清掉了，這次 timer 就 stale，不打
        if (controller.text.trim() != kw) return;
        performSearch(kw);
      });
    }

    final isInSearchMode = controller.text.trim().isNotEmpty;
    final showResults = hasSearched.value || isSearching.value;

    return Scaffold(
      backgroundColor: AppColors.bgPrimary,
      body: SafeArea(
        top: false,
        child: Column(
          children: [
            _TopBar(
              controller: controller,
              onChanged: onChanged,
              onSubmitted: performSearch,
            ),
            Expanded(
              child: ListView(
                padding: EdgeInsets.zero,
                keyboardDismissBehavior:
                    ScrollViewKeyboardDismissBehavior.onDrag,
                children: [
                  if (!isInSearchMode)
                    _HomePage(
                      isLoadingRecommended: isLoadingRecommended.value,
                      recommendedProducts: recommendedProducts.value,
                      onProductTap: (result) =>
                          _openResultDetail(context, result),
                    ),
                  if (isInSearchMode && showResults)
                    _ResultsSection(
                      isSearching: isSearching.value,
                      resultsError: resultsError.value,
                      results: results.value,
                      hasSearched: hasSearched.value,
                      onProductTap: (result) =>
                          _openResultDetail(context, result),
                    ),
                  const SizedBox(height: HkSpace.xxl),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }

  void _openResultDetail(BuildContext context, _SearchResult result) {
    context.push(
      '/merchant-product',
      extra: MerchantItemDetailScreen(
        title: result.skuName,
        skuId: result.skuId,
        merchantId: result.merchantId,
        merchantName: result.merchantName,
        imageUrl: result.imageUrl.isEmpty ? null : result.imageUrl,
        price: result.unitPrice,
      ),
    );
  }
}

// ─── Internal helpers (top-level functions) ───

Future<void> _loadRecommendedProducts({
  required ValueNotifier<bool> isLoading,
  required ValueNotifier<List<_SearchResult>> products,
  required Dio dio,
}) async {
  if (isLoading.value) return;
  isLoading.value = true;
  try {
    // 推薦商品：依銷量排序的公開商品列表，訪客與已登入共用，不需要 token。
    final resp = await dio.get<Map<String, dynamic>>(
      '/public/products',
      queryParameters: const {'sort': 'SALES', 'page': 1, 'page_size': 12},
    );
    final flat = <Map<String, dynamic>>[];
    final records = resp.data?['data']?['records'];
    if (records is List) {
      for (final p in records) {
        if (p is Map) flat.add(Map<String, dynamic>.from(p));
      }
    }

    final recommended = <_SearchResult>[];
    final seenIds = <String>{};
    for (final p in flat) {
      if (recommended.length >= 6) break;

      // records[] 是 gateway ProductItem，欄名全 camelCase。
      final skuId = (p['id'] ?? '').toString().trim();
      if (skuId.isEmpty || !seenIds.add(skuId)) continue;

      String skuName = (p['name'] ?? '').toString().trim();
      if (skuName.isEmpty) continue;
      if (skuName.contains('-')) skuName = skuName.split('-').first.trim();

      // 49 §6.1 成對契約：imageKey（objectKey）優先，imageUrl 退路。
      final imageUrl = (p['imageKey'] ?? p['imageUrl'] ?? '').toString().trim();
      // ProductItem 的店名同時出現在 flat 層（sellerName）與巢狀 seller.name。
      final nestedSeller = p['seller'] is Map ? p['seller'] as Map : null;
      final merchantName = (p['sellerName'] ?? nestedSeller?['name'] ?? '')
          .toString()
          .trim();
      final merchantId = (p['storeId'] ?? nestedSeller?['storeId'] ?? '')
          .toString()
          .trim();

      final (priceLabel, unitPrice) = PriceDisplay.priceLabelAndUnit(
        p['priceMin'],
        p['priceMax'],
      );

      recommended.add(
        _SearchResult(
          skuName: skuName,
          skuId: skuId,
          merchantId: merchantId,
          merchantName: merchantName,
          priceLabel: priceLabel,
          imageUrl: imageUrl,
          unitPrice: unitPrice,
        ),
      );
    }
    products.value = recommended;
  } catch (_) {
    // ignore — 推薦失敗不擋整個畫面
  } finally {
    isLoading.value = false;
  }
}

// OpenAPI spec (預設模組.openapi.json → /public/search/products) 規定的 query 是
// `keyword`/`q`、`page`、`page_size`、`sort_by`、`sort_order`、`category_id`。
// 舊版誤傳 `size`（規格無此鍵）跟 `date`（spec 也沒有），backend 會默默忽略。
Future<List<dynamic>> _fetchProductsPage({
  required int page,
  required int pageSize,
  required String keyword,
  required Dio dio,
  String? sortBy,
  String? sortOrder,
  String? categoryId,
}) async {
  try {
    final resp = await dio.get<Map<String, dynamic>>(
      '/public/search/products',
      queryParameters: {
        'page': page,
        'page_size': pageSize,
        'keyword': keyword,
        if (sortBy != null && sortBy.isNotEmpty) 'sort_by': sortBy,
        if (sortOrder != null && sortOrder.isNotEmpty) 'sort_order': sortOrder,
        if (categoryId != null && categoryId.isNotEmpty)
          'category_id': categoryId,
      },
    );
    if (resp.statusCode == 200 && resp.data != null) {
      final data = resp.data!['data'];
      if (data is List) return data;
      // 分頁 key 是 `records`（§16；見 product_repository.searchProducts 同樣 wire）。
      if (data is Map && data['records'] is List) {
        return data['records'] as List;
      }
    }
  } catch (_) {}
  return [];
}

// /public/search/products response 內嵌的 storeId / priceMin / priceMax /
// imageUrl 已足夠建出 _SearchResult；原本 flow 為每個 product 再打
// /public/products/markets 取 merchant，但該 endpoint 後端不存在（會 400
// `For input string: "markets"`，被路由匹配到 /public/products/{id} 後 parse Long 失敗），
// 整段 N+1 aggregation 都是死路。直接用內嵌欄位即可。
_SearchResult? _resultFromProductMap(Map<String, dynamic> map) {
  final skuId = (map['id'] ?? '').toString().trim();
  String skuName = (map['name'] ?? '').toString().trim();
  if (skuId.isEmpty || skuName.isEmpty) return null;
  if (skuName.contains('-')) skuName = skuName.split('-').first.trim();

  final merchantId = (map['storeId'] ?? '').toString().trim();
  final nestedSeller = map['seller'] is Map ? map['seller'] as Map : null;
  final merchantName = (map['sellerName'] ?? nestedSeller?['name'] ?? '')
      .toString()
      .trim();
  // 49 §6.1 成對契約：imageKey（objectKey）優先，imageUrl 退路。
  final imageUrl = (map['imageKey'] ?? map['imageUrl'] ?? '').toString().trim();

  final (priceLabel, unitPrice) = PriceDisplay.priceLabelAndUnit(
    map['priceMin'],
    map['priceMax'],
  );

  return _SearchResult(
    skuName: skuName,
    skuId: skuId,
    merchantId: merchantId,
    merchantName: merchantName,
    priceLabel: priceLabel,
    imageUrl: imageUrl,
    unitPrice: unitPrice,
  );
}

/// 偵測注音 IME 中間狀態：含 ㄅ-ㄦ (U+3105–U+3129) 或聲調符號
/// ˉ(U+02C9) ˇ(U+02C7) ˊ(U+02CA) ˋ(U+02CB) ˙(U+02D9) 都代表還在組字。
bool _isImeComposingText(String s) {
  for (final code in s.codeUnits) {
    if (code >= 0x3105 && code <= 0x3129) return true;
    if (code == 0x02C7 ||
        code == 0x02C9 ||
        code == 0x02CA ||
        code == 0x02CB ||
        code == 0x02D9) {
      return true;
    }
  }
  return false;
}

Future<List<_SearchResult>?> _fetchSearchResults({
  required String keyword,
  required TextEditingController controller,
  required int requestId,
  required ObjectRef<int> searchSerial,
  required Dio dio,
}) async {
  final items = await _fetchProductsPage(
    page: 1,
    pageSize: 60,
    keyword: keyword,
    dio: dio,
  );
  if (searchSerial.value != requestId) return null;
  if (controller.text.trim().isEmpty) return null;

  final mapped = <_SearchResult>[];
  final seenIds = <String>{};

  for (final item in items) {
    Map<String, dynamic>? map;
    if (item is Map<String, dynamic>) {
      map = item;
    } else if (item is Map) {
      map = Map<String, dynamic>.from(item);
    }
    if (map == null) continue;

    final result = _resultFromProductMap(map);
    if (result == null) continue;
    if (!seenIds.add(result.skuId)) continue;
    mapped.add(result);
  }

  if (searchSerial.value != requestId) return null;
  return mapped;
}

// ─── Widgets ───

class _TopBar extends StatelessWidget {
  const _TopBar({
    required this.controller,
    required this.onChanged,
    required this.onSubmitted,
  });
  final TextEditingController controller;
  final ValueChanged<String> onChanged;
  final ValueChanged<String> onSubmitted;

  @override
  Widget build(BuildContext context) {
    return CategoryTopBar(
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
      middle: CategorySearchField(
        controller: controller,
        autofocus: true,
        hintText: '搜尋商品',
        semanticLabel: '商品搜尋',
        semanticHint: '輸入商品名稱或關鍵字',
        onChanged: onChanged,
        onSubmitted: onSubmitted,
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
      ),
    );
  }
}

class _HomePage extends StatelessWidget {
  const _HomePage({
    required this.isLoadingRecommended,
    required this.recommendedProducts,
    required this.onProductTap,
  });
  final bool isLoadingRecommended;
  final List<_SearchResult> recommendedProducts;
  final void Function(_SearchResult) onProductTap;

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(
      builder: (context, constraints) {
        final pageWidth = constraints.maxWidth;
        final horizontalPadding = HkBreakpoints.pageGutter(pageWidth);
        final maxWidth = HkBreakpoints.contentMaxWidth(pageWidth);

        return Align(
          alignment: Alignment.topCenter,
          child: ConstrainedBox(
            constraints: BoxConstraints(maxWidth: maxWidth),
            child: Padding(
              key: const ValueKey('search-home-content'),
              padding: EdgeInsets.fromLTRB(
                horizontalPadding,
                HkSpace.xxl,
                horizontalPadding,
                0,
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  const _SearchSectionHeader(
                    key: ValueKey('search-products-section-title'),
                    title: '人氣商品',
                    subtitle: '為你挑選的新鮮好物',
                  ),
                  const SizedBox(height: HkSpace.md),
                  if (isLoadingRecommended)
                    const _AdaptiveProductLayout(
                      key: ValueKey('search-products-loading'),
                      children: [
                        _SkeletonCard(horizontalInset: false),
                        _SkeletonCard(horizontalInset: false),
                        _SkeletonCard(horizontalInset: false),
                      ],
                    )
                  else if (recommendedProducts.isNotEmpty)
                    _AdaptiveProductLayout(
                      key: const ValueKey('search-recommended-products'),
                      children: [
                        for (final result in recommendedProducts)
                          _SearchResultTile(
                            key: ValueKey('recommended-${result.skuId}'),
                            result: result,
                            onTap: () => onProductTap(result),
                          ),
                      ],
                    )
                  else
                    Padding(
                      padding: const EdgeInsets.symmetric(
                        vertical: HkSpace.xxl,
                      ),
                      child: Center(
                        child: Column(
                          mainAxisSize: MainAxisSize.min,
                          children: [
                            const HkIcon(HkIconName.emptyProducts, size: 64),
                            const SizedBox(height: HkSpace.sm),
                            Text('暫無推薦商品', style: AppTypography.metaMuted),
                          ],
                        ),
                      ),
                    ),
                ],
              ),
            ),
          ),
        );
      },
    );
  }
}

class _SearchSectionHeader extends StatelessWidget {
  const _SearchSectionHeader({
    super.key,
    required this.title,
    required this.subtitle,
  });

  final String title;
  final String subtitle;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          title,
          style: AppTypography.sectionTitle.copyWith(
            color: AppColors.textEmphasis,
          ),
        ),
        const SizedBox(height: HkSpace.xs),
        Text(subtitle, style: AppTypography.metaMuted),
      ],
    );
  }
}

class _AdaptiveProductLayout extends StatelessWidget {
  const _AdaptiveProductLayout({super.key, required this.children});

  final List<Widget> children;

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(
      builder: (context, constraints) {
        final useTwoColumns = constraints.maxWidth >= HkBreakpoints.tablet;
        if (!useTwoColumns) {
          return Column(
            children: [
              for (var i = 0; i < children.length; i++) ...[
                children[i],
                if (i != children.length - 1)
                  const SizedBox(height: HkSpace.md),
              ],
            ],
          );
        }

        const spacing = HkSpace.md;
        final itemWidth = (constraints.maxWidth - spacing) / 2;
        return Wrap(
          spacing: spacing,
          runSpacing: spacing,
          children: [
            for (final child in children)
              SizedBox(width: itemWidth, child: child),
          ],
        );
      },
    );
  }
}

class _ResultsSection extends StatelessWidget {
  const _ResultsSection({
    required this.isSearching,
    required this.resultsError,
    required this.results,
    required this.hasSearched,
    required this.onProductTap,
  });
  final bool isSearching;
  final String? resultsError;
  final List<_SearchResult> results;
  final bool hasSearched;
  final void Function(_SearchResult) onProductTap;

  @override
  Widget build(BuildContext context) {
    if (isSearching) {
      return Column(children: _buildSkeletonCards(context));
    }

    if (resultsError != null) {
      return Padding(
        padding: EdgeInsets.symmetric(vertical: 16.h),
        child: Center(
          child: Text(
            resultsError!,
            style: const TextStyle(color: AppColors.danger),
            textAlign: TextAlign.center,
          ),
        ),
      );
    }

    if (results.isEmpty) {
      if (hasSearched) return _searchHintBox('尚無相關商品');
      return const SizedBox.shrink();
    }

    return _buildProductResults();
  }

  List<Widget> _buildSkeletonCards(BuildContext context) {
    final viewportHeight = MediaQuery.of(context).size.height;
    const estimatedCardHeight = 118.0;
    final skeletonCount = math.max(
      5,
      (viewportHeight / estimatedCardHeight).ceil(),
    );
    return List.generate(
      skeletonCount,
      (i) => _SkeletonCard(isLast: i == skeletonCount - 1),
    );
  }

  Widget _buildProductResults() {
    return LayoutBuilder(
      builder: (context, constraints) {
        final pageWidth = constraints.maxWidth;
        final maxWidth = HkBreakpoints.contentMaxWidth(pageWidth);
        final horizontalPadding = HkBreakpoints.pageGutter(pageWidth);
        return Align(
          alignment: Alignment.topCenter,
          child: ConstrainedBox(
            constraints: BoxConstraints(maxWidth: maxWidth),
            child: Padding(
              padding: EdgeInsets.fromLTRB(
                horizontalPadding,
                HkSpace.xxl,
                horizontalPadding,
                0,
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  Row(
                    children: [
                      Text('搜尋結果', style: AppTypography.sectionTitle),
                      const Spacer(),
                      Text(
                        '共 ${results.length} 項',
                        style: AppTypography.metaMuted,
                      ),
                    ],
                  ),
                  const SizedBox(height: HkSpace.md),
                  _AdaptiveProductLayout(
                    children: [
                      for (final result in results)
                        _SearchResultTile(
                          key: ValueKey('result-${result.skuId}'),
                          result: result,
                          onTap: () => onProductTap(result),
                        ),
                    ],
                  ),
                ],
              ),
            ),
          ),
        );
      },
    );
  }

  Widget _searchHintBox(String text) {
    return Container(
      margin: EdgeInsets.fromLTRB(16.w, 24.h, 16.w, 0),
      padding: EdgeInsets.symmetric(vertical: 24.h, horizontal: 12.w),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.center,
        children: [
          const HkIcon(HkIconName.emptySearch, size: 64),
          SizedBox(height: 12.h),
          Text(
            text,
            style: const TextStyle(color: AppColors.textSecondary),
            textAlign: TextAlign.center,
          ),
        ],
      ),
    );
  }
}

class _SearchResultTile extends StatelessWidget {
  const _SearchResultTile({
    super.key,
    required this.result,
    required this.onTap,
  });
  final _SearchResult result;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final merchantName = result.merchantName.trim();
    return Semantics(
      key: ValueKey('search-result-tile-${result.skuId}'),
      button: true,
      label: '查看商品：${result.skuName}',
      onTap: onTap,
      child: ExcludeSemantics(
        child: DecoratedBox(
          decoration: BoxDecoration(
            color: AppColors.bgCard,
            borderRadius: BorderRadius.circular(HkRadius.lg),
            border: Border.all(color: AppColors.borderSubtle),
            boxShadow: HkShadows.sm,
          ),
          child: Material(
            color: Colors.transparent,
            borderRadius: BorderRadius.circular(HkRadius.lg),
            clipBehavior: Clip.antiAlias,
            child: InkWell(
              onTap: onTap,
              child: Padding(
                padding: const EdgeInsets.all(HkSpace.md),
                child: Row(
                  crossAxisAlignment: CrossAxisAlignment.center,
                  children: [
                    SizedBox.square(
                      dimension: 80,
                      child: result.imageUrl.isNotEmpty
                          ? AppCachedImage(
                              imageUrl: result.imageUrl,
                              fit: BoxFit.cover,
                              width: 80,
                              height: 80,
                              borderRadius: BorderRadius.circular(HkRadius.md),
                              scene: ImageScene.searchResult,
                            )
                          : DecoratedBox(
                              decoration: BoxDecoration(
                                color: AppColors.primarySoft,
                                borderRadius: BorderRadius.circular(
                                  HkRadius.md,
                                ),
                              ),
                              child: Center(
                                child: Icon(
                                  AppIcons.imageOutlined,
                                  color: AppIconColors.muted,
                                  size: 28,
                                ),
                              ),
                            ),
                    ),
                    const SizedBox(width: HkSpace.md),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          Text(
                            result.skuName,
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: AppTypography.productName.copyWith(
                              color: AppColors.textEmphasis,
                            ),
                          ),
                          if (result.priceLabel.isNotEmpty) ...[
                            const SizedBox(height: HkSpace.xs),
                            Text(
                              result.priceLabel,
                              maxLines: 1,
                              overflow: TextOverflow.ellipsis,
                              style: AppTypography.meta.copyWith(
                                color: AppColors.primary,
                                fontWeight: FontWeight.w700,
                              ),
                            ),
                          ],
                          if (merchantName.isNotEmpty) ...[
                            const SizedBox(height: HkSpace.xs),
                            Text(
                              merchantName,
                              maxLines: 1,
                              overflow: TextOverflow.ellipsis,
                              style: AppTypography.metaMuted,
                            ),
                          ],
                        ],
                      ),
                    ),
                    const SizedBox(width: HkSpace.sm),
                    Container(
                      key: const ValueKey('search-product-action'),
                      height: 40,
                      padding: const EdgeInsets.symmetric(
                        horizontal: HkSpace.sm,
                      ),
                      decoration: BoxDecoration(
                        color: AppColors.primarySoft,
                        borderRadius: BorderRadius.circular(HkRadius.pill),
                      ),
                      child: Row(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          Text(
                            '查看',
                            style: AppTypography.meta.copyWith(
                              color: AppColors.textAccentOnTint,
                              fontWeight: FontWeight.w700,
                            ),
                          ),
                          const SizedBox(width: HkSpace.xs),
                          Icon(
                            AppIcons.chevronRight,
                            size: 16,
                            color: AppIconColors.active,
                          ),
                        ],
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
}

class _SkeletonCard extends StatelessWidget {
  const _SkeletonCard({this.isLast = false, this.horizontalInset = true});
  final bool isLast;
  final bool horizontalInset;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: horizontalInset
          ? EdgeInsets.fromLTRB(
              HkSpace.lg,
              0,
              HkSpace.lg,
              isLast ? 0 : HkSpace.md,
            )
          : EdgeInsets.zero,
      child: HkSkeletonShimmer(
        child: Container(
          decoration: BoxDecoration(
            color: AppColors.bgElevated,
            borderRadius: BorderRadius.circular(HkRadius.lg),
            border: Border.all(color: AppColors.borderSubtle),
            boxShadow: HkShadows.sm,
          ),
          padding: const EdgeInsets.all(HkSpace.md),
          child: Row(
            children: [
              Container(
                width: 80,
                height: 80,
                decoration: BoxDecoration(
                  color: AppColors.bgPrimary,
                  borderRadius: BorderRadius.circular(HkRadius.md),
                ),
              ),
              const SizedBox(width: HkSpace.md),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    const _SkeletonLine(width: double.infinity, height: 16),
                    const SizedBox(height: HkSpace.sm),
                    const _SkeletonLine(width: 72, height: 14),
                    const SizedBox(height: HkSpace.sm),
                    const _SkeletonLine(width: 96, height: 12),
                  ],
                ),
              ),
              const SizedBox(width: HkSpace.sm),
              const _SkeletonLine(width: 60, height: 40, radius: HkRadius.pill),
            ],
          ),
        ),
      ),
    );
  }
}

class _SkeletonLine extends StatelessWidget {
  const _SkeletonLine({
    required this.width,
    required this.height,
    this.radius = 8,
  });
  final double width;
  final double height;
  final double radius;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: width,
      height: height,
      decoration: BoxDecoration(
        color: AppColors.dividerLight,
        borderRadius: BorderRadius.circular(radius),
      ),
    );
  }
}

class _SearchResult {
  const _SearchResult({
    required this.skuName,
    required this.skuId,
    required this.merchantId,
    required this.merchantName,
    required this.priceLabel,
    required this.imageUrl,
    this.unitPrice,
  });
  final String skuName;
  final String skuId;
  final String merchantId;
  final String merchantName;
  final String priceLabel;
  final String imageUrl;
  final double? unitPrice;
}
