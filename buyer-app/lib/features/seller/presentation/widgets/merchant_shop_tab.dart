import 'dart:async';

import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:hawkr_buyer/shared/icons/market_favorite_icon.dart';
import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../../config/image_config.dart';
import '../../../../core/networking/dio_provider.dart';
import '../../../../core/networking/oss_service.dart';
import '../../../../core/session/session_provider.dart';
import '../../../../core/storage/auth_storage.dart';
import '../../../../shared/utils/hk_log.dart';
import '../../../../shared/utils/require_login.dart';
import '../../../../shared/widgets/common_snackbar.dart';
import '../../../../shared/widgets/hk_frosted_card_surface.dart';
import '../../../../shared/widgets/fruit_refresh_indicator.dart';
import '../../../../shared/widgets/oss_image.dart';
import '../../../../shared/utils/price_display.dart';
import '../../../cart/data/cart_repository.dart';
import '../../../cart/data/cart_error_message.dart';
import '../../../cart/presentation/cart_controller.dart';
import '../../../home/presentation/home_widget/product_add_to_cart_sheet.dart';
import '../../../product/data/hashtag_extractor.dart';
import '../../../product/data/product_repository.dart';
import '../../../visit/presentation/favorite_products.dart';
import '../../../product/presentation/merchant_item_detail_screen.dart';
import 'merchant_common.dart';
import 'merchant_empty_state.dart';
import 'merchant_layout.dart';
import '../../../visit/data/visit_repository.dart';

const double _shopCardPadding = 10;
const double _shopCardBottomPadding = 4;
const double _shopCardColumnGap = 10;
const double _shopCardSectionGap = 4;
const double _shopCardActionTargetHeight = HkControl.minTap;
const double _shopCardActionVisualHeight = 40;
const double _shopCardStackThreshold = 180;

double _shopCardImageWidth(double cardWidth) =>
    (cardWidth * 0.34).clamp(104.0, 132.0).toDouble();

double _shopCardContentWidth(double cardWidth) =>
    cardWidth -
    (_shopCardPadding * 2) -
    _shopCardColumnGap -
    _shopCardImageWidth(cardWidth);

bool _shopCardStacksActions(double cardWidth, {required double textScale}) =>
    _shopCardContentWidth(cardWidth) <
    _shopCardStackThreshold * textScale.clamp(1, 2);

/// 商家頁「商城」Tab。
///
/// 以垂直 [ListView] 呈現商家商品列表，每個商品以 [MerchantShopCard] 渲染。
/// 支援：
/// - 預快取（[prefetchedProducts] / [prefetchedReady]）避免切換 Tab 時重複請求
/// - 加入喜愛收藏（[visitRepositoryProvider]）
/// - 下拉重整
class MerchantShopTab extends StatefulWidget {
  const MerchantShopTab({
    super.key,
    this.ref,
    this.merchantId,
    this.merchantName,
    this.merchantAvatar,
    this.prefetchedProducts,
    this.prefetchedReady = false,
    this.outerHandle,
  });

  /// Riverpod [WidgetRef]，由父層 ConsumerWidget 傳入。
  final WidgetRef? ref;
  final String? merchantId;
  final String? merchantName;
  final String? merchantAvatar;

  /// 父層預先抓取的商品資料；不為 null 時直接使用，跳過再次請求。
  final List<Map<String, dynamic>>? prefetchedProducts;

  /// 父層是否已完成預抓取（完成才可決定要顯示資料或發起新請求）。
  final bool prefetchedReady;

  /// [NestedScrollView] 的 [SliverOverlapAbsorberHandle]，用於補償 pinned header 高度。
  final SliverOverlapAbsorberHandle? outerHandle;

  @override
  State<MerchantShopTab> createState() => _MerchantShopTabState();
}

class _MerchantShopTabState extends State<MerchantShopTab>
    with AutomaticKeepAliveClientMixin {
  @override
  bool get wantKeepAlive => true;
  List<Map<String, dynamic>> _products = const <Map<String, dynamic>>[];
  bool _loadingProducts = false;
  bool _prefetchApplied = false;

  // ── 初始化 ────────────────────────────────────────────────────────────────

  @override
  void initState() {
    super.initState();
    _startProducts();
  }

  void _startProducts() {
    if (widget.prefetchedProducts != null) {
      _applyPrefetched(widget.prefetchedProducts!);
    } else if (widget.prefetchedReady) {
      unawaited(_fetchProducts());
    } else {
      setState(() => _loadingProducts = true);
    }
  }

  @override
  void didUpdateWidget(covariant MerchantShopTab oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (widget.merchantId != oldWidget.merchantId) {
      _products = const <Map<String, dynamic>>[];
      _prefetchApplied = false;
      _loadingProducts = false;
      _startProducts();
      return;
    }
    if (!_prefetchApplied) {
      if (widget.prefetchedProducts != null) {
        _applyPrefetched(widget.prefetchedProducts!);
      } else if (widget.prefetchedReady) {
        _fetchProducts();
      }
    }
  }

  // ── 資料載入 ──────────────────────────────────────────────────────────────

  Future<void> _loadFavorites() async {
    try {
      final ids = _products
          .map(
            (product) => MerchantFieldHelper.str(product, [
              'skuId',
              'id',
              'productId',
              'spuId',
            ]),
          )
          .map(int.tryParse)
          .whereType<int>();
      await widget.ref?.read(favoriteProductsProvider).check(ids);
    } catch (_) {}
  }

  void _applyPrefetched(List<Map<String, dynamic>> source) {
    final copied = source.map((m) => Map<String, dynamic>.from(m)).toList();
    if (!mounted) return;
    setState(() {
      _products = copied;
      _loadingProducts = false;
    });
    _prefetchApplied = true;
    unawaited(_loadFavorites());
  }

  Future<void> _fetchProducts() async {
    final id = widget.merchantId?.trim() ?? '';
    if (id.isEmpty) {
      if (mounted) {
        setState(() {
          _products = const [];
          _loadingProducts = false;
        });
      }
      _prefetchApplied = true;
      return;
    }
    if (mounted) {
      setState(() {
        _loadingProducts = true;
      });
    }
    try {
      final dio = widget.ref?.read(dioProvider);
      if (dio == null) return;
      final resp = await dio.get<Map<String, dynamic>>(
        '/public/sellers/$id/products',
        queryParameters: {'page': 1, 'page_size': 50, 'status': 1},
      );
      final body = resp.data ?? {};
      HKLog.logD('[MerchantShopTab] _fetchProducts response: $body');
      final list = MerchantApiHelper.extractList(body);
      if (!mounted) return;
      setState(() {
        _products = list;
        _loadingProducts = false;
      });
      _prefetchApplied = true;
      unawaited(_loadFavorites());
    } catch (e) {
      HKLog.logE('[MerchantShopTab] _fetchProducts error: $e', e);
      if (!mounted) return;
      setState(() {
        _loadingProducts = false;
      });
      _prefetchApplied = true;
    }
  }

  // ── 收藏工具 ──────────────────────────────────────────────────────────────

  bool _isFavorited(Map m) {
    final raw = m['isFavorited'];
    if (raw is bool) return raw;
    final normalized = raw?.toString().toLowerCase().trim() ?? '';
    return normalized == 'true' || normalized == '1';
  }

  Future<void> _toggleFavorite({
    required String productId,
    required bool isFav,
  }) async {
    final intId = int.tryParse(productId);
    if (intId == null) return;
    try {
      await widget.ref
          ?.read(favoriteProductsProvider)
          .toggle(intId, current: isFav);
    } catch (_) {
      if (!mounted) return;
      showCommonSnackBar(context, isFav ? '取消喜愛失敗' : '加入喜愛失敗');
    }
  }

  // ── Build ─────────────────────────────────────────────────────────────────

  @override
  Widget build(BuildContext context) {
    super.build(context);
    if (widget.merchantId == null || widget.merchantId!.isEmpty) {
      return Container(
        color: AppColors.bgPrimary,
        child: const Center(
          child: MerchantEmptyState(
            icon: HkIconName.emptyMerchantSearch,
            label: '暫無商家資料',
          ),
        ),
      );
    }

    return FruitRefreshIndicator(
      onRefresh: () async {
        await _fetchProducts();
      },
      topOffset: widget.outerHandle?.layoutExtent ?? 0,
      child: CustomScrollView(
        physics: const AlwaysScrollableScrollPhysics(),
        slivers: [
          if (widget.outerHandle != null)
            SliverOverlapInjector(handle: widget.outerHandle!),
          _buildContentSliver(),
        ],
      ),
    );
  }

  Widget _buildContentSliver() {
    if (_products.isEmpty && _loadingProducts) {
      return SliverLayoutBuilder(
        builder: (context, constraints) => _buildResponsiveProductSliver(
          context,
          constraints.crossAxisExtent,
          itemCount: 4,
          itemBuilder: (_, __) => const MerchantShopCardSkeleton(),
        ),
      );
    }
    if (_products.isEmpty) {
      return SliverFillRemaining(
        hasScrollBody: false,
        child: const Center(
          child: MerchantEmptyState(
            icon: HkIconName.emptyProducts,
            label: '暫無商品',
          ),
        ),
      );
    }
    return SliverLayoutBuilder(
      builder: (context, constraints) => _buildResponsiveProductSliver(
        context,
        constraints.crossAxisExtent,
        itemCount: _products.length,
        itemBuilder: (_, index) => _buildProductCard(index),
      ),
    );
  }

  Widget _buildResponsiveProductSliver(
    BuildContext context,
    double viewportWidth, {
    required int itemCount,
    required IndexedWidgetBuilder itemBuilder,
  }) {
    final spec = MerchantLayoutSpec.forSize(
      Size(viewportWidth, MediaQuery.sizeOf(context).height),
      textScale: MediaQuery.textScalerOf(context).scale(14) / 14,
    );
    final twoColumns = viewportWidth >= 720;
    final maxWidth = twoColumns
        ? MerchantLayoutSpec.storefrontContentMaxWidth
        : 680.0;
    final sideInset = MerchantLayoutSpec.centeredSideInset(
      viewportWidth,
      baseInset: spec.pageHorizontalPadding,
      maxContentWidth: maxWidth,
    );
    final padding = EdgeInsets.fromLTRB(sideInset, 16, sideInset, 24);
    if (twoColumns) {
      return SliverPadding(
        padding: padding,
        sliver: SliverList.separated(
          itemCount: (itemCount + 1) ~/ 2,
          separatorBuilder: (_, __) => const SizedBox(height: HkSpace.lg),
          itemBuilder: (context, rowIndex) {
            final firstIndex = rowIndex * 2;
            final secondIndex = firstIndex + 1;
            return Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Expanded(child: itemBuilder(context, firstIndex)),
                const SizedBox(width: HkSpace.lg),
                Expanded(
                  child: secondIndex < itemCount
                      ? itemBuilder(context, secondIndex)
                      : const SizedBox.shrink(),
                ),
              ],
            );
          },
        ),
      );
    }
    return SliverPadding(
      padding: padding,
      sliver: SliverList.separated(
        itemCount: itemCount,
        separatorBuilder: (_, __) => const SizedBox(height: HkSpace.lg),
        itemBuilder: itemBuilder,
      ),
    );
  }

  Widget _buildProductCard(int index) {
    final m = _products[index];
    final title = MerchantFieldHelper.str(m, [
      'title',
      'name',
      'skuName',
      'spuName',
      'productName',
      'goodsName',
      'label',
      'desc',
    ], '商品');
    final priceNum = MerchantFieldHelper.num_(m, ['priceMin']);
    final priceMaxNum = MerchantFieldHelper.num_(m, ['priceMax'], priceNum);
    final imageUrls = _extractImageUrls(m);
    final defaultBrand = (widget.merchantName?.trim().isNotEmpty == true)
        ? widget.merchantName!
        : 'Hawkr';
    final brand = MerchantFieldHelper.str(m, [
      'merchantName',
      'storeName',
      'shopName',
      'brand',
    ], defaultBrand);
    final sku = MerchantFieldHelper.str(m, [
      'skuId',
      'id',
      'productId',
      'spuId',
    ]);
    final rating = _extractRating(m);
    final favoriteId = int.tryParse(sku);
    final hashtags = BuyerHashtagExtractor.fromProduct(m);
    final listPriceMin = m['listPriceMin']?.toString();
    final listPriceMax = m['listPriceMax']?.toString();
    final salePriceMin = m['salePriceMin']?.toString();
    final salePriceMax = m['salePriceMax']?.toString();
    final isRestockSubscribed = m['isRestockSubscribed'] == true;
    return Consumer(
      builder: (_, favoriteRef, _) {
        // 還沒讀到共用的收藏狀態時，先用商品資料帶的 isFavorited。
        final isFav =
            sku.isNotEmpty &&
            ((favoriteId == null
                    ? null
                    : favoriteRef.watch(favoriteProductProvider(favoriteId))) ??
                _isFavorited(m));
        return MerchantShopCard(
          widgetRef: widget.ref,
          merchantId: widget.merchantId!,
          merchantName: brand,
          merchantAvatar: widget.merchantAvatar,
          sku: sku,
          title: title,
          imageUrls: imageUrls,
          fallbackPrice: priceNum,
          fallbackPriceMax: priceMaxNum,
          rating: rating,
          reviewCount: _extractReviewCount(m),
          inventory: _extractInventory(m),
          grades: _extractGrades(m),
          isFav: isFav,
          hashtags: hashtags,
          listPriceMin: listPriceMin,
          listPriceMax: listPriceMax,
          salePriceMin: salePriceMin,
          salePriceMax: salePriceMax,
          isRestockSubscribed: isRestockSubscribed,
          onToggleFavorite: () => _toggleFavorite(productId: sku, isFav: isFav),
          onFavoriteChanged: (pid, fav) {
            final id = int.tryParse(pid);
            if (id == null) return;
            widget.ref?.read(favoriteProductProvider(id).notifier).set(fav);
          },
        );
      },
    );
  }

  static List<String> _extractImageUrls(Map<String, dynamic> m) {
    final imageUrls = <String>[];
    // 49 §6.1 成對契約：imageKeys（objectKey 陣列）優先，imageUrls 退路。
    final images = m['imageKeys'] ?? m['imageUrls'];
    if (images is List) {
      for (final e in images) {
        if (e is String && e.trim().isNotEmpty) {
          imageUrls.add(e.trim());
        } else if (e is Map) {
          final u = (e['imageKey'] ?? e['imageUrl'] ?? '').toString().trim();
          if (u.isNotEmpty) imageUrls.add(u);
        }
      }
    } else if (images is String && images.isNotEmpty) {
      imageUrls.addAll(
        images.split(',').map((e) => e.trim()).where((e) => e.isNotEmpty),
      );
    }
    if (imageUrls.isEmpty) {
      final img = MerchantFieldHelper.str(m, [
        'image',
        'img',
        'pic',
        'picUrl',
        'skuDefaultImg',
        'spuPic',
        'cover',
        'imageUrl',
      ]);
      if (img.isNotEmpty) imageUrls.add(img);
    }
    return imageUrls;
  }

  static double _extractRating(Map<String, dynamic> m) {
    for (final key in [
      'rating',
      'score',
      'avgRating',
      'stars',
      'averageScore',
      'avgScore',
    ]) {
      final val = m[key];
      if (val == null) continue;
      double? r;
      if (val is num) {
        r = val.toDouble();
      } else if (val is String) {
        r = double.tryParse(val);
      }
      if (r != null) return r.clamp(0.0, 5.0);
    }
    return 0.0;
  }

  static int? _extractReviewCount(Map<String, dynamic> m) {
    for (final key in ['reviewCount']) {
      final raw = m[key];
      final parsed = raw is num
          ? raw.toInt()
          : int.tryParse(raw?.toString() ?? '');
      if (parsed != null && parsed >= 0) return parsed;
    }
    return null;
  }

  static List<Map<String, dynamic>> _extractGrades(Map<String, dynamic> m) {
    final raw = m['skus'];
    if (raw is! List) return const [];
    return raw
        .whereType<Map>()
        .map((item) => Map<String, dynamic>.from(item))
        .toList(growable: false);
  }

  /// List payload 有庫存時直接使用；沒有時保持未知，進入詳情頁再載入。
  static int? _extractInventory(Map<String, dynamic> m) {
    // ProductItem 列表層吐的是 `stock`（字串）；skus[] 才有 stockQty。
    for (final key in ['stock']) {
      final raw = m[key];
      final parsed = raw is num
          ? raw.toInt()
          : int.tryParse(raw?.toString() ?? '');
      if (parsed != null) return parsed;
    }
    final grades = _extractGrades(m);
    if (grades.isEmpty) return null;
    var total = 0;
    var hasStock = false;
    for (final grade in grades) {
      final raw = grade['stockQty'];
      final parsed = raw is num
          ? raw.toInt()
          : int.tryParse(raw?.toString() ?? '');
      if (parsed == null) continue;
      total += parsed;
      hasStock = true;
    }
    return hasStock ? total : null;
  }
}

// ---------------------------------------------------------------------------
// MerchantShopCard — 商城商品卡
// ---------------------------------------------------------------------------

/// 商城商品卡片（水平佈局）。
///
/// 左側為可滑動多圖 [PageView]（或單圖），右側縱向排列：
/// - 商品名稱 + 收藏圖示
/// - 評分
/// - 商城列表 payload 已提供的售價與原價區間
/// - 「加入購物車」按鈕
///
/// 售完時顯示圖片角標與禁用操作，對齊商品列表卡片樣式。
class MerchantShopCard extends StatefulWidget {
  const MerchantShopCard({
    super.key,
    this.widgetRef,
    required this.merchantId,
    required this.merchantName,
    this.merchantAvatar,
    required this.sku,
    required this.title,
    required this.imageUrls,
    required this.fallbackPrice,
    this.fallbackPriceMax,
    required this.rating,
    this.reviewCount,
    this.inventory,
    this.grades = const [],
    required this.isFav,
    required this.onToggleFavorite,
    this.onFavoriteChanged,
    // PayUni #758: hashtag + listPrice 區間（從 product map 推出）
    this.hashtags = const [],
    this.listPriceMin,
    this.listPriceMax,
    this.salePriceMin,
    this.salePriceMax,
    this.isRestockSubscribed = false,
  });

  final WidgetRef? widgetRef;
  final String merchantId;
  final String merchantName;
  final String? merchantAvatar;
  final String sku;
  final String title;
  final List<String> imageUrls;

  /// 規格未載入前的後備顯示價格。
  final num fallbackPrice;
  final num? fallbackPriceMax;
  final double rating;
  final int? reviewCount;
  final int? inventory;
  final List<Map<String, dynamic>> grades;
  final bool isFav;
  final VoidCallback onToggleFavorite;
  final void Function(String productId, bool isFavorite)? onFavoriteChanged;

  /// PayUni #758: hashtag list（從 product hashtags 或 description 抽取）
  final List<String> hashtags;

  /// 原價區間（用於刪除線）。任一邊 > priceMin 才顯示刪除線。
  final String? listPriceMin;
  final String? listPriceMax;

  /// 特價區間（"0" 或空 = 無特價）。
  final String? salePriceMin;
  final String? salePriceMax;
  final bool isRestockSubscribed;

  @override
  State<MerchantShopCard> createState() => _MerchantShopCardState();
}

class _MerchantShopCardState extends State<MerchantShopCard> {
  int? _inventory;
  bool _inventoryLoaded = false;
  int _currentImageIndex = 0;
  List<String> _loadedImageUrls = [];
  double? _loadedRating;
  int? _loadedReviewCount;
  num? _minPrice;
  num? _maxPrice;
  // PayUni #758: 原價區間（刪除線）+ 是否有特價
  num? _listMin;
  num? _listMax;
  bool _hasSale = false;
  bool _restockLoading = false;
  bool _restockSubscribed = false;
  bool? _localRestockOverride;
  List<Map<String, dynamic>> _grades = [];

  @override
  void initState() {
    super.initState();
    _hydrateFromListPayload();
  }

  @override
  void didUpdateWidget(covariant MerchantShopCard oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.sku != widget.sku) _localRestockOverride = null;
    _hydrateFromListPayload();
    // The seller-list payload can remain stale after a successful mutation.
    // Keep the last confirmed local value in either direction until this card
    // is remounted or begins representing a different product.
    final localOverride = _localRestockOverride;
    if (localOverride != null) _restockSubscribed = localOverride;
  }

  /// 商城列表只使用 `/public/sellers/{id}/products` 已帶回的摘要資料。
  ///
  /// 舊版每張卡首次顯示會額外打兩次 product detail、一次 reviews，並
  /// 呼叫已下線的 seller product images endpoint。缺少的規格、評論與庫存
  /// 改在使用者進入商品詳情時再載入，避免 N 張卡放大成 4N 個請求。
  void _hydrateFromListPayload() {
    _inventory = widget.inventory;
    _inventoryLoaded = widget.inventory != null;
    _loadedImageUrls = widget.imageUrls
        .map((url) => OssService.resolveImageUrl(url) ?? url)
        .where((url) => url.trim().isNotEmpty)
        .toList(growable: false);
    if (_currentImageIndex >= _loadedImageUrls.length) {
      _currentImageIndex = 0;
    }
    _loadedRating = widget.rating;
    _loadedReviewCount = widget.reviewCount;
    _minPrice = widget.fallbackPrice;
    final fallbackMax = widget.fallbackPriceMax;
    _maxPrice = fallbackMax != null && fallbackMax >= widget.fallbackPrice
        ? fallbackMax
        : widget.fallbackPrice;
    _hasSale = PriceDisplay.hasRangeSale(
      salePriceMin: widget.salePriceMin,
      salePriceMax: widget.salePriceMax,
    );
    _listMin = null;
    _listMax = null;
    final lpMin = PriceDisplay.parseMoney(widget.listPriceMin);
    final lpMax = PriceDisplay.parseMoney(widget.listPriceMax);
    if (lpMin != null && lpMin > 0) _listMin = lpMin;
    if (lpMax != null && lpMax > 0) _listMax = lpMax;
    _hasSale =
        _hasSale ||
        PriceDisplay.shouldStrikeListPrice(_listMax ?? _listMin, _minPrice);
    _grades = widget.grades
        .map((grade) => Map<String, dynamic>.from(grade))
        .toList(growable: false);
    _restockSubscribed = widget.isRestockSubscribed;
  }

  // ── 加入購物車 / 導航 ─────────────────────────────────────────────────────

  Future<void> _addToCart() async {
    if (widget.sku.isEmpty) return;
    final ref = widget.widgetRef;
    if (ref == null) return;
    if (!await requireLogin(context, ref)) return;
    if (!mounted) return;
    final imageUrl = _loadedImageUrls.isNotEmpty ? _loadedImageUrls.first : '';
    showProductAddToCartSheet(
      context,
      ref,
      skuId: widget.sku,
      productName: widget.title,
      imageUrl: imageUrl,
      initialPrice: _minPrice?.toDouble() ?? widget.fallbackPrice.toDouble(),
    );
  }

  Future<void> _subscribeToRestock() async {
    if (widget.sku.isEmpty || _restockLoading || _restockSubscribed) return;
    final ref = widget.widgetRef;
    if (ref == null || !await requireLogin(context, ref)) return;
    if (!mounted) return;
    setState(() => _restockLoading = true);
    try {
      final subscribed = await ref
          .read(productRepositoryProvider)
          .subscribeRestock(widget.sku);
      if (!mounted) return;
      if (subscribed) {
        setState(() {
          _restockSubscribed = true;
          _localRestockOverride = true;
        });
      }
      showCommonSnackBar(
        context,
        subscribed ? '已開啟補貨通知，補貨時會通知你' : '操作失敗，請稍後再試',
      );
    } catch (_) {
      if (mounted) showCommonSnackBar(context, '操作失敗，請稍後再試');
    } finally {
      if (mounted) setState(() => _restockLoading = false);
    }
  }

  /// 直接購買：跟詳情頁 _handleDirectCheckout 相同邏輯 —
  /// 單規格時直接加車 + 導到購物車（帶 preSelectedSkuId）；多規格則進詳情頁讓使用者選
  Future<void> _buyNow() async {
    if (widget.sku.isEmpty) return;
    final ref = widget.widgetRef;
    if (ref == null) return;
    if (!await requireLogin(context, ref)) return;
    if (!mounted) return;
    if (_grades.length != 1) {
      _navigateToDetail();
      return;
    }
    final grade = _grades.first;
    final skuId = int.tryParse(
      (grade['id'] ?? grade['gradeId'] ?? grade['skuId'] ?? widget.sku)
          .toString(),
    );
    if (skuId == null) {
      if (mounted) showCommonSnackBar(context, '購買失敗');
      return;
    }
    try {
      final session = ref.read(sessionProvider).value;
      final guestId = hasUsableSession(session)
          ? null
          : await AuthStorage.guestCartId();
      await ref
          .read(cartRepositoryProvider)
          .addToCart(skuId, 1, guestId: guestId);
      ref.invalidate(cartCountProvider);
      if (!mounted) return;
      context.push('/cart', extra: {'preSelectedSkuId': skuId});
    } catch (e) {
      HKLog.logE('[MerchantShopTab] buyNow addToCart error: $e');
      if (mounted) showCommonSnackBar(context, addToCartErrorMessage(e));
    }
  }

  void _navigateToDetail() {
    void applyResult(Map<String, dynamic>? result) {
      if (!mounted || result == null) return;
      final pid = (result['productId'] ?? '').toString().trim();
      if (pid.isEmpty || pid != widget.sku) return;

      final restockResult = result['isRestockSubscribed'];
      if (restockResult is bool) {
        setState(() {
          _restockSubscribed = restockResult;
          _localRestockOverride = restockResult;
        });
      }

      if (result.containsKey('isFavorite')) {
        widget.onFavoriteChanged?.call(pid, result['isFavorite'] == true);
      }
    }

    context
        .push<Map<String, dynamic>>(
          '/merchant-product',
          extra: MerchantItemDetailScreen(
            title: widget.title,
            imageUrl: _loadedImageUrls.isNotEmpty
                ? _loadedImageUrls.first
                : null,
            price: _minPrice ?? widget.fallbackPrice,
            skuId: widget.sku.isEmpty ? null : widget.sku,
            merchantName: widget.merchantName,
            merchantId: widget.merchantId,
            initialRestockSubscribed: _restockSubscribed,
            onResultChanged: applyResult,
          ),
        )
        .then(applyResult);
  }

  // ── 價格顯示 ──────────────────────────────────────────────────────────────

  String get _priceDisplayText {
    if (_minPrice == null) {
      return PriceDisplay.formatPriceRange(
        widget.fallbackPrice,
        widget.fallbackPriceMax,
        prefix: 'NT\$ ',
      );
    }
    return PriceDisplay.formatPriceRange(_minPrice, _maxPrice, prefix: 'NT\$ ');
  }

  /// PayUni #758: 原價刪除線顯示字串（無資料或原價 <= 主價時回空字串）
  String get _listPriceDisplayText {
    if (!_hasSale) return '';
    final lpMin = _listMin;
    final lpMax = _listMax;
    if (lpMin == null && lpMax == null) return '';
    final pMin = _minPrice ?? widget.fallbackPrice;
    final highest = (lpMax ?? lpMin)!;
    if (highest <= pMin) return '';
    return PriceDisplay.formatPriceRange(lpMin, lpMax, prefix: 'NT\$ ');
  }

  // ── Build ─────────────────────────────────────────────────────────────────

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(
      builder: (context, constraints) {
        final isSoldOut = _inventoryLoaded && (_inventory == 0);
        final inventoryKnown = _inventoryLoaded;
        final cardWidth = constraints.hasBoundedWidth
            ? constraints.maxWidth
            : MediaQuery.sizeOf(context).width - 32;
        final imgW = _shopCardImageWidth(cardWidth);
        final actionTextScale = MediaQuery.textScalerOf(context).scale(12) / 12;
        final stackActions = _shopCardStacksActions(
          cardWidth,
          textScale: actionTextScale,
        );
        final strikeListDisplay = _listPriceDisplayText;
        final resolvedRating = _loadedRating ?? widget.rating;
        final ratingText = resolvedRating <= 0 && (_loadedReviewCount ?? 0) == 0
            ? '尚無評分'
            : _loadedReviewCount == null
            ? resolvedRating.toStringAsFixed(1)
            : '${resolvedRating.toStringAsFixed(1)} ($_loadedReviewCount則評價)';

        Widget buildImage() {
          return ClipRRect(
            borderRadius: BorderRadius.circular(12),
            child: SizedBox(
              key: const ValueKey('merchant-shop-image'),
              width: imgW,
              child: AspectRatio(
                aspectRatio: 4 / 3,
                child: Stack(
                  fit: StackFit.expand,
                  children: [
                    isSoldOut
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
                            child: Opacity(
                              opacity: 0.65,
                              child: _buildImageSection(),
                            ),
                          )
                        : _buildImageSection(),
                    if (isSoldOut)
                      const Positioned(top: 6, left: 6, child: _SoldOutPill()),
                    // PayUni #758: hashtag chip overlay（左下，僅顯示第一個）
                    if (widget.hashtags.isNotEmpty) ...[
                      Positioned(
                        left: 0,
                        right: 0,
                        bottom: 0,
                        height: 28,
                        child: IgnorePointer(
                          child: Opacity(
                            opacity: 0.22,
                            child: Container(
                              decoration: const BoxDecoration(
                                gradient: LinearGradient(
                                  begin: Alignment.bottomCenter,
                                  end: Alignment.topCenter,
                                  colors: [
                                    AppColors.textEmphasis,
                                    Color(0x00C0C0C0),
                                  ],
                                ),
                              ),
                            ),
                          ),
                        ),
                      ),
                      Positioned(
                        left: 2,
                        right: 2,
                        bottom: 2,
                        child: IgnorePointer(
                          child: Align(
                            alignment: Alignment.centerLeft,
                            child: _ShopHashtagChip(
                              text: '#${widget.hashtags.first}',
                            ),
                          ),
                        ),
                      ),
                    ],
                  ],
                ),
              ),
            ),
          );
        }

        return Semantics(
          container: true,
          explicitChildNodes: true,
          button: true,
          label: '查看商品：${widget.title}，價格：$_priceDisplayText',
          onTap: _navigateToDetail,
          child: GestureDetector(
            onTap: _navigateToDetail,
            child: HkFrostedCardSurface(
              key: const ValueKey('merchant-shop-card-surface'),
              child: Padding(
                padding: const EdgeInsets.fromLTRB(
                  _shopCardPadding,
                  _shopCardPadding,
                  _shopCardPadding,
                  _shopCardBottomPadding,
                ),
                child: Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    buildImage(),
                    const SizedBox(width: _shopCardColumnGap),
                    // 文字區
                    Expanded(
                      child: KeyedSubtree(
                        key: const ValueKey('merchant-shop-content'),
                        child: Column(
                          mainAxisSize: MainAxisSize.min,
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            // 收藏按鈕不再撐高標題列；標題保留右側安全區。
                            SizedBox(
                              width: double.infinity,
                              child: Stack(
                                key: const ValueKey('merchant-shop-title-row'),
                                clipBehavior: Clip.none,
                                children: [
                                  Padding(
                                    padding: const EdgeInsets.only(right: 40),
                                    child: Text(
                                      widget.title,
                                      key: const ValueKey(
                                        'merchant-shop-title',
                                      ),
                                      style: TextStyle(
                                        color: isSoldOut
                                            ? AppColors.textSecondary
                                            : AppColors.textPrimary,
                                        fontSize: 16,
                                        fontWeight: FontWeight.w700,
                                        height: 1.3,
                                      ),
                                      maxLines: 2,
                                      overflow: TextOverflow.ellipsis,
                                    ),
                                  ),
                                  Positioned(
                                    top: 0,
                                    right: 0,
                                    child: _ShopFavoriteButton(
                                      isFavorite: widget.isFav,
                                      onTap: () async {
                                        final ref = widget.widgetRef;
                                        if (ref != null &&
                                            !await requireLogin(context, ref)) {
                                          return;
                                        }
                                        widget.onToggleFavorite();
                                      },
                                    ),
                                  ),
                                ],
                              ),
                            ),
                            const SizedBox(height: _shopCardSectionGap),
                            // 評分
                            Row(
                              key: const ValueKey('merchant-shop-rating-row'),
                              mainAxisSize: MainAxisSize.min,
                              crossAxisAlignment: CrossAxisAlignment.center,
                              children: [
                                const Icon(
                                  AppIcons.star,
                                  key: ValueKey('merchant-shop-rating-star'),
                                  size: 13,
                                  color: AppIconColors.rating,
                                ),
                                const SizedBox(width: 4),
                                Flexible(
                                  child: Text(
                                    ratingText,
                                    key: const ValueKey(
                                      'merchant-shop-rating-label',
                                    ),
                                    style: TextStyle(
                                      color: AppColors.neutralMutedText
                                          .withValues(
                                            alpha: isSoldOut ? 0.55 : 1,
                                          ),
                                      fontSize: 12,
                                      fontWeight: FontWeight.w400,
                                      height: 1.1,
                                    ),
                                    maxLines: 1,
                                    overflow: TextOverflow.ellipsis,
                                  ),
                                ),
                              ],
                            ),
                            const SizedBox(height: _shopCardSectionGap),
                            Wrap(
                              key: const ValueKey('merchant-shop-price-row'),
                              spacing: 6,
                              runSpacing: 2,
                              crossAxisAlignment: WrapCrossAlignment.center,
                              children: [
                                Text(
                                  _priceDisplayText,
                                  key: const ValueKey(
                                    'merchant-shop-sale-price',
                                  ),
                                  maxLines: 1,
                                  softWrap: false,
                                  style: TextStyle(
                                    color: isSoldOut
                                        ? AppColors.textSecondary
                                        : AppColors.actionPrimary,
                                    fontSize: 18,
                                    fontWeight: FontWeight.w700,
                                    height: 1.15,
                                  ),
                                ),
                                if (strikeListDisplay.isNotEmpty)
                                  _ShopListPrice(text: strikeListDisplay),
                              ],
                            ),
                            const SizedBox(height: _shopCardSectionGap),
                            _buildActionButtons(
                              context,
                              inventoryKnown: inventoryKnown,
                              isSoldOut: isSoldOut,
                              stacked: stackActions,
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
        );
      },
    );
  }

  Widget _buildActionButtons(
    BuildContext context, {
    required bool inventoryKnown,
    required bool isSoldOut,
    required bool stacked,
  }) {
    if (!inventoryKnown) {
      return SizedBox(
        width: double.infinity,
        child: _ShopCardActionButton(
          key: const ValueKey('merchant-shop-view-action'),
          label: '查看商品',
          variant: _ShopCardActionVariant.outline,
          icon: AppIcons.shoppingBagOutlined,
          onTap: _navigateToDetail,
        ),
      );
    }
    final first = isSoldOut
        ? _ShopCardActionButton(
            key: const ValueKey('merchant-shop-sold-out-action'),
            label: '已售完',
            variant: _ShopCardActionVariant.outline,
            disabled: true,
            icon: AppIcons.block,
            onTap: () {},
          )
        : _ShopCardActionButton(
            key: const ValueKey('merchant-shop-add-action'),
            label: '加入購物車',
            variant: _ShopCardActionVariant.cart,
            icon: AppIcons.shoppingCartOutlined,
            onTap: _addToCart,
          );
    final second = isSoldOut
        ? _ShopCardActionButton(
            key: const ValueKey('merchant-shop-restock-action'),
            label: _restockSubscribed
                ? '已開啟補貨通知'
                : _restockLoading
                ? '開啟中…'
                : '補貨通知',
            variant: _ShopCardActionVariant.muted,
            disabled: _restockLoading || _restockSubscribed,
            icon: AppIcons.notificationsNoneRounded,
            onTap: _subscribeToRestock,
          )
        : _ShopCardActionButton(
            key: const ValueKey('merchant-shop-buy-action'),
            label: '立即結帳',
            variant: _ShopCardActionVariant.checkout,
            icon: AppIcons.paymentsOutlined,
            onTap: _buyNow,
          );

    if (stacked) {
      return Column(
        key: const ValueKey('merchant-shop-actions-stacked'),
        mainAxisSize: MainAxisSize.min,
        children: [
          SizedBox(width: double.infinity, child: first),
          const SizedBox(height: _shopCardSectionGap),
          SizedBox(width: double.infinity, child: second),
        ],
      );
    }
    return Row(
      key: const ValueKey('merchant-shop-actions-inline'),
      children: [
        Expanded(child: first),
        const SizedBox(width: _shopCardSectionGap),
        Expanded(child: second),
      ],
    );
  }

  Widget _buildImageSection() {
    if (_loadedImageUrls.isEmpty) {
      return Container(
        color: AppColors.gray200,
        child: Icon(AppIcons.image, size: 40, color: AppIconColors.muted),
      );
    }
    if (_loadedImageUrls.length == 1) {
      return OssImage(
        imageSource: _loadedImageUrls.first,
        scene: ImageScene.reviewPhoto,
        fit: BoxFit.cover,
        errorWidget: Container(
          color: AppColors.gray200,
          child: Icon(AppIcons.image, size: 40, color: AppIconColors.muted),
        ),
      );
    }
    return Stack(
      children: [
        PageView.builder(
          itemCount: _loadedImageUrls.length,
          onPageChanged: (index) => setState(() {
            _currentImageIndex = index;
          }),
          itemBuilder: (_, index) => OssImage(
            imageSource: _loadedImageUrls[index],
            scene: ImageScene.reviewPhoto,
            fit: BoxFit.cover,
            errorWidget: Container(
              color: AppColors.gray200,
              child: Icon(AppIcons.image, size: 40, color: AppIconColors.muted),
            ),
          ),
        ),
        Positioned(
          bottom: 6,
          left: 0,
          right: 0,
          child: Row(
            mainAxisAlignment: MainAxisAlignment.center,
            children: List.generate(_loadedImageUrls.length, (index) {
              return Container(
                width: 6,
                height: 6,
                margin: const EdgeInsets.symmetric(horizontal: 4),
                decoration: BoxDecoration(
                  shape: BoxShape.circle,
                  color: _currentImageIndex == index
                      ? AppColors.bgCard
                      : AppColors.bgCard.withAlpha(128),
                ),
              );
            }),
          ),
        ),
      ],
    );
  }
}

class _ShopFavoriteButton extends StatefulWidget {
  const _ShopFavoriteButton({required this.isFavorite, required this.onTap});

  final bool isFavorite;
  final VoidCallback onTap;

  @override
  State<_ShopFavoriteButton> createState() => _ShopFavoriteButtonState();
}

class _ShopFavoriteButtonState extends State<_ShopFavoriteButton> {
  static const _pressDuration = Duration(milliseconds: 150);
  static const _pressedScale = 0.9;

  bool _pressed = false;

  void _setPressed(bool value) {
    if (_pressed == value) return;
    setState(() => _pressed = value);
  }

  @override
  Widget build(BuildContext context) {
    final reduceMotion =
        MediaQuery.maybeOf(context)?.disableAnimations ?? false;
    return Semantics(
      button: true,
      selected: widget.isFavorite,
      label: widget.isFavorite ? '取消收藏' : '加入收藏',
      onTap: widget.onTap,
      child: ExcludeSemantics(
        child: MouseRegion(
          cursor: SystemMouseCursors.click,
          child: Listener(
            onPointerDown: (_) {
              if (!reduceMotion) _setPressed(true);
            },
            onPointerUp: (_) => _setPressed(false),
            onPointerCancel: (_) => _setPressed(false),
            child: GestureDetector(
              key: const ValueKey('merchant-shop-favorite-action'),
              behavior: HitTestBehavior.opaque,
              onTap: widget.onTap,
              child: SizedBox.square(
                dimension: HkControl.minTap,
                child: Align(
                  alignment: Alignment.topRight,
                  child: AnimatedScale(
                    key: const ValueKey('merchant-shop-favorite-press-scale'),
                    scale: reduceMotion || !_pressed ? 1 : _pressedScale,
                    duration: reduceMotion ? Duration.zero : _pressDuration,
                    curve: Curves.easeOut,
                    child: MarketFavoriteIcon(
                      key: ValueKey(
                        widget.isFavorite
                            ? 'merchant-shop-favorite-filled'
                            : 'merchant-shop-favorite-outline',
                      ),
                      size: 24,
                      isFavorite: widget.isFavorite,
                      color: widget.isFavorite
                          ? AppIconColors.favorite
                          : AppIconColors.muted,
                    ),
                  ),
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// MerchantShopCardSkeleton — 商城骨架屏卡片
// ---------------------------------------------------------------------------

/// 商城商品載入期間的骨架屏佔位卡片。
///
/// 視覺結構與 [MerchantShopCard] 對齊（左圖右文本橫列佈局）。
class MerchantShopCardSkeleton extends StatelessWidget {
  const MerchantShopCardSkeleton({super.key});

  @override
  Widget build(BuildContext context) {
    final baseColor = AppColors.gray200;
    final highlightColor = AppColors.gray300;

    Widget bar({double height = 14, double? width}) => Container(
      width: width ?? double.infinity,
      height: height,
      decoration: BoxDecoration(
        color: baseColor,
        borderRadius: BorderRadius.circular(6),
      ),
    );

    return LayoutBuilder(
      builder: (context, constraints) {
        final cardWidth = constraints.hasBoundedWidth
            ? constraints.maxWidth
            : MediaQuery.sizeOf(context).width - 32;
        final imageWidth = _shopCardImageWidth(cardWidth);
        return HkFrostedCardSurface(
          key: const ValueKey('merchant-shop-card-skeleton-surface'),
          child: Padding(
            padding: const EdgeInsets.fromLTRB(
              _shopCardPadding,
              _shopCardPadding,
              _shopCardPadding,
              _shopCardBottomPadding,
            ),
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                SizedBox(
                  width: imageWidth,
                  child: AspectRatio(
                    aspectRatio: 4 / 3,
                    child: Container(
                      key: const ValueKey('merchant-shop-skeleton-image'),
                      decoration: BoxDecoration(
                        color: highlightColor,
                        borderRadius: BorderRadius.circular(HkRadius.lg),
                      ),
                    ),
                  ),
                ),
                const SizedBox(width: _shopCardColumnGap),
                Expanded(
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      bar(height: 18),
                      const SizedBox(height: _shopCardSectionGap),
                      bar(width: 110),
                      const SizedBox(height: _shopCardSectionGap),
                      bar(width: 90),
                      const SizedBox(height: _shopCardSectionGap),
                      bar(height: _shopCardActionVisualHeight),
                    ],
                  ),
                ),
              ],
            ),
          ),
        );
      },
    );
  }
}

class _ShopListPrice extends StatelessWidget {
  const _ShopListPrice({required this.text});

  final String text;

  @override
  Widget build(BuildContext context) {
    const prefix = 'NT\$ ';
    final hasPrefix = text.startsWith(prefix);
    final numericText = hasPrefix ? text.substring(prefix.length) : text;
    const baseStyle = TextStyle(
      fontSize: 12,
      color: AppColors.textSecondary,
      fontWeight: FontWeight.w400,
      height: 1.2,
      letterSpacing: 0.2,
    );

    return KeyedSubtree(
      key: const ValueKey('merchant-shop-list-price-row'),
      child: Text.rich(
        TextSpan(
          children: [
            if (hasPrefix) const TextSpan(text: prefix, style: baseStyle),
            TextSpan(
              text: numericText,
              style: baseStyle.copyWith(
                decoration: TextDecoration.lineThrough,
                decorationColor: AppColors.textSecondary,
                decorationThickness: 1.2,
              ),
            ),
          ],
        ),
        key: const ValueKey('merchant-shop-list-price'),
        maxLines: 1,
        softWrap: false,
      ),
    );
  }
}

/// 商品卡底部的操作按鈕：加入購物車（outlined）/ 直接購買（filled）。
class _ShopCardActionButton extends StatefulWidget {
  const _ShopCardActionButton({
    super.key,
    required this.label,
    required this.variant,
    required this.onTap,
    this.disabled = false,
    this.icon,
  });

  final String label;
  final _ShopCardActionVariant variant;
  final bool disabled;
  final IconData? icon;
  final VoidCallback onTap;

  @override
  State<_ShopCardActionButton> createState() => _ShopCardActionButtonState();
}

class _ShopCardActionButtonState extends State<_ShopCardActionButton> {
  @override
  Widget build(BuildContext context) {
    final bg = switch (widget.variant) {
      _ShopCardActionVariant.cart => AppColors.commerceCartAction,
      _ShopCardActionVariant.checkout => AppColors.commerceCheckoutAction,
      _ShopCardActionVariant.outline ||
      _ShopCardActionVariant.muted => AppColors.bgCard,
    };
    final fg = switch (widget.variant) {
      _ShopCardActionVariant.cart => AppColors.commerceCartForeground,
      _ShopCardActionVariant.checkout => AppColors.commerceCheckoutForeground,
      _ShopCardActionVariant.outline => AppColors.primary,
      _ShopCardActionVariant.muted => AppColors.neutralMutedText,
    };
    final outlined =
        widget.variant == _ShopCardActionVariant.outline ||
        widget.variant == _ShopCardActionVariant.muted;
    return Semantics(
      button: true,
      enabled: !widget.disabled,
      excludeSemantics: true,
      label: widget.label,
      child: MouseRegion(
        cursor: widget.disabled
            ? SystemMouseCursors.basic
            : SystemMouseCursors.click,
        child: GestureDetector(
          behavior: HitTestBehavior.opaque,
          onTap: widget.disabled ? null : widget.onTap,
          child: SizedBox(
            height: _shopCardActionTargetHeight,
            child: Align(
              alignment: Alignment.center,
              child: AnimatedOpacity(
                duration: HkMotion.duration,
                opacity: widget.disabled ? 0.5 : 1,
                child: Container(
                  key: ValueKey('merchant-shop-${widget.variant.name}-surface'),
                  height: _shopCardActionVisualHeight,
                  alignment: Alignment.center,
                  padding: const EdgeInsets.symmetric(horizontal: 6),
                  decoration: BoxDecoration(
                    color: bg,
                    borderRadius: BorderRadius.circular(HkRadius.pill),
                    border: outlined
                        ? Border.all(
                            color:
                                widget.variant == _ShopCardActionVariant.muted
                                ? AppColors.cardBorder
                                : AppColors.primary,
                            width: 1,
                          )
                        : null,
                  ),
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    mainAxisAlignment: MainAxisAlignment.center,
                    children: [
                      if (widget.icon case final icon?) ...[
                        Icon(
                          icon,
                          key: ValueKey(
                            'merchant-shop-${widget.variant.name}-icon',
                          ),
                          size: 16,
                          color: fg,
                        ),
                        const SizedBox(width: 4),
                      ],
                      Flexible(
                        child: Text(
                          widget.label,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          textAlign: TextAlign.center,
                          style: TextStyle(
                            color: fg,
                            fontSize: 12,
                            fontWeight: FontWeight.w700,
                            height: 1.2,
                          ),
                        ),
                      ),
                    ],
                  ),
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}

enum _ShopCardActionVariant { cart, checkout, outline, muted }

/// Hashtag chip overlay — 對齊買家 ProductCard / 賣家 page_shop_tab。
class _ShopHashtagChip extends StatelessWidget {
  const _ShopHashtagChip({required this.text});
  final String text;

  @override
  Widget build(BuildContext context) {
    return DecoratedBox(
      key: const ValueKey('merchant-shop-hashtag-chip'),
      decoration: BoxDecoration(
        color: const Color(0xCCFFFFFF),
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: const Color(0x33000000), width: 0.5),
        boxShadow: const [
          BoxShadow(
            color: AppColors.cardShadow,
            blurRadius: 4,
            offset: Offset(0, 1),
          ),
        ],
      ),
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 5),
        child: Text(
          text,
          maxLines: 1,
          overflow: TextOverflow.ellipsis,
          softWrap: false,
          style: TextStyle(
            color: AppColors.brandPrimaryStrong,
            fontSize: 12,
            fontWeight: FontWeight.w700,
            height: 1.0,
          ),
        ),
      ),
    );
  }
}

class _SoldOutPill extends StatelessWidget {
  const _SoldOutPill();

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
      decoration: BoxDecoration(
        color: const Color(0xCC191C18),
        borderRadius: BorderRadius.circular(8),
      ),
      child: Text(
        '已售完',
        style: TextStyle(
          color: AppColors.bgCard,
          fontSize: 12,
          fontWeight: FontWeight.w700,
          height: 1.2,
        ),
      ),
    );
  }
}
