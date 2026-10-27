import '../../../../features/seller/data/seller_status_refresh.dart';
import '../../../../features/seller/data/current_store.dart';
import '../../../../features/seller/data/onboarding_tasks.dart';
import '../../../../features/seller/presentation/onboarding_progress_card.dart';
import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../../core/analytics/analytics_providers.dart';
import '../../../../core/router/app_router.dart';
import '../../../../core/theme/hk_tokens.dart';
import '../../../../config/image_config.dart';
import '../../../../shared/utils/hk_log.dart';
import '../../../../shared/utils/floating_toast.dart';
import '../../../../shared/utils/price_display.dart';
import '../../../../shared/utils/product_image_resolver.dart';
import '../../../../shared/widgets/hk_disabled_share.dart';
import '../../../../shared/widgets/hk_disabled_with_hint.dart';
import '../../../../shared/widgets/hk_icon.dart';
import '../../../../shared/widgets/fruit_refresh_indicator.dart';
import '../../../../shared/widgets/fruit_spinner.dart';
import '../../../../shared/widgets/optimized_image.dart';
import '../../../../shared/widgets/seller_floating_navigation_layout.dart';
import '../../../../shared/widgets/share_bottom_sheet.dart';
import '../../../product/data/hashtag_extractor.dart';
import '../../../product/data/product_repository.dart';
import '../../../product/data/product_upload_service.dart';
import '../../data/post_repository.dart';
import '../shop_tab_controller.dart';
import 'storefront_tab_glass.dart';

/// 商城 Tab — 管理商品、開通流程、上架狀態
class PageShopTab extends ConsumerStatefulWidget {
  const PageShopTab({super.key, this.outerHandle, required this.sellerId});

  /// NestedScrollView 的 overlap absorber handle，用於 SliverOverlapInjector
  final SliverOverlapAbsorberHandle? outerHandle;
  final String sellerId;

  @override
  ConsumerState<PageShopTab> createState() => PageShopTabState();
}

class PageShopTabState extends ConsumerState<PageShopTab>
    with AutomaticKeepAliveClientMixin {
  @override
  bool get wantKeepAlive => true;

  /// 供外層在 tab 切換時呼叫，重新載入認證 / 物流狀態與商品
  void reload() {
    _refreshReadiness();
    _loadOneProduct();
  }

  int _productsRefreshTick = 0;
  StreamSubscription<List<ProductUploadTask>>? _uploadSub;

  ShopTabState get _shopState =>
      ref.read(shopTabControllerProvider(widget.sellerId));
  List<Map<String, dynamic>> get _products => _shopState.products;

  @override
  void initState() {
    super.initState();
    Future.microtask(_loadOneProduct);
    // 監聽背景上傳完成，刷新商品列表
    _uploadSub = ProductUploadService().taskStream.listen((tasks) {
      final ownSellerId = ref.read(sellerStoreIdProvider);
      final isOwn = widget.sellerId == ownSellerId;
      final hasNewSuccess = tasks.any((t) => t.status == UploadStatus.success);
      if (hasNewSuccess && isOwn) {
        _loadOneProduct();
        ProductUploadService().clearCompletedTasks();
      }
    });
  }

  @override
  void dispose() {
    _uploadSub?.cancel();
    super.dispose();
  }

  void _refreshReadiness() {
    ref.invalidate(onboardingInputsProvider);
    unawaited(ref.read(sellerStatusRefreshProvider.notifier).refresh());
  }

  Future<void> _loadOneProduct() async {
    if (widget.sellerId.isEmpty) {
      HKLog.logD('[Shop] No sellerId; skip fetching products');
      return;
    }
    try {
      await ref
          .read(shopTabControllerProvider(widget.sellerId).notifier)
          .refreshProducts();
      if (!mounted) return;
      if (_products.isNotEmpty) {
        final p = _products.first;
        HKLog.logD(
          '[Shop] Products loaded (${_products.length}) for '
          '${widget.sellerId}; first => id=${p['id']}, '
          'skuId=${p['skuId']}, price=${p['price']}',
        );
      } else {
        HKLog.logD('[Shop] No products for sellerId=${widget.sellerId}');
      }
    } catch (e) {
      HKLog.logD('[Shop] Load products failed: $e');
    }
  }

  Future<void> _openProductPicker() async {
    final id = ref.read(sellerStoreIdProvider);
    if (id == null) {
      FloatingToast.error(context, '無法取得商家ID，請重新登入');
      return;
    }
    final result = await context.push<dynamic>(ProductRoutes.picker);
    if (result == null) return;
    if (!mounted) return;
    if (result == true) {
      await _loadOneProduct();
      _refreshReadiness();
    }
  }

  @override
  Widget build(BuildContext context) {
    super.build(context);
    ref.watch(shopTabControllerProvider(widget.sellerId));
    return _productsSection();
  }

  Widget _productsSection() {
    final navigationClearance = sellerFloatingNavigationClearance(context);
    Future<void> onRefresh() async {
      _refreshReadiness();
      await _loadOneProduct();
      if (mounted) setState(() => _productsRefreshTick++);
    }

    return Stack(
      children: [
        Positioned.fill(
          child: FruitRefreshIndicator(
            onRefresh: onRefresh,
            child: CustomScrollView(
              slivers: [
                if (widget.outerHandle != null)
                  SliverOverlapInjector(handle: widget.outerHandle!),
                SliverPadding(
                  padding: EdgeInsets.fromLTRB(16.w, 16.h, 16.w, 0),
                  sliver: const SliverToBoxAdapter(
                    child: OnboardingProgressCard(),
                  ),
                ),
                if (_products.isNotEmpty)
                  SliverPadding(
                    padding: EdgeInsets.fromLTRB(
                      16.w,
                      16.h,
                      16.w,
                      navigationClearance + HkControl.minTap + HkSpace.lg,
                    ),
                    sliver: SliverList(
                      delegate: SliverChildBuilderDelegate(
                        (_, i) {
                          final p = _products[i];
                          final keySku = (p['skuId'] ?? p['sku_id'] ?? p['id'])
                              .toString();
                          return Padding(
                            padding: EdgeInsets.only(bottom: 12.h),
                            child: _AddedProductCard(
                              key: ValueKey(
                                'p_${_productsRefreshTick}_$keySku',
                              ),
                              product: p,
                              onEditCompleted: () async {
                                await _loadOneProduct();
                                if (mounted) {
                                  setState(() => _productsRefreshTick++);
                                }
                              },
                              onDeleted: () async {
                                ref
                                    .read(
                                      shopTabControllerProvider(
                                        widget.sellerId,
                                      ).notifier,
                                    )
                                    .removeProduct(keySku);
                                await _loadOneProduct();
                                _refreshReadiness();
                              },
                            ),
                          );
                        },
                        childCount: _products.length,
                        addRepaintBoundaries: false,
                      ),
                    ),
                  )
                else
                  SliverPadding(
                    padding: EdgeInsets.only(
                      top: 16.h,
                      bottom:
                          navigationClearance + HkControl.minTap + HkSpace.lg,
                    ),
                    sliver: SliverToBoxAdapter(child: _buildEmptyState()),
                  ),
              ],
            ),
          ),
        ),
        Positioned(
          left: 0,
          right: 0,
          bottom: navigationClearance,
          child: Center(
            child: StorefrontGlassActionButton(
              key: const ValueKey('seller-add-product-floating-action'),
              icon: HkIconName.add,
              label: '新增商品',
              onTap: _openProductPicker,
            ),
          ),
        ),
      ],
    );
  }

  Widget _buildEmptyState() {
    return const StorefrontGlassEmptyPanel(
      icon: HkIconName.goods,
      message: '暫無商品',
      description: '商品上架後會顯示在這裡。',
    );
  }
}

// ── 商品卡片 ──────────────────────────────────────────────────────────────────

@visibleForTesting
Widget buildSellerProductManagementCardForTest({
  required Map<String, dynamic> product,
}) => _AddedProductCard(product: product);

class _AddedProductCard extends ConsumerStatefulWidget {
  final Map<String, dynamic> product;
  final Future<void> Function()? onEditCompleted;
  final Future<void> Function()? onDeleted;
  const _AddedProductCard({
    super.key,
    required this.product,
    this.onEditCompleted,
    this.onDeleted,
  });

  @override
  ConsumerState<_AddedProductCard> createState() => _AddedProductCardState();
}

class _AddedProductCardState extends ConsumerState<_AddedProductCard> {
  final PageController _pageController = PageController();
  int _pageIndex = 0;
  List<String> _imageUrls = const [];
  double? _avgRating;
  int? _ratingCount;
  int? _minPrice;
  int? _maxPrice;
  // PayUni #758: list price 用於 strikethrough；hasSale true 才顯示
  int? _listMin;
  int? _listMax;
  bool _hasSale = false;
  int? _stockQty;

  String _status = ''; // 4001=draft, 4002=review, 4003=unlisted, 4004=listed
  bool _isTogglingStatus = false;

  StreamSubscription<List<ProductUploadTask>>? _uploadSubscription;
  bool _isUploading = false;
  double _uploadProgress = 0.0;

  @override
  void initState() {
    super.initState();
    _minPrice = int.tryParse(widget.product['priceMin']?.toString() ?? '');
    _maxPrice = int.tryParse(widget.product['priceMax']?.toString() ?? '');
    // 後端規則：salePriceMin/Max == "0" 表示全部無特價（不顯示刪除線）
    _hasSale = PriceDisplay.hasRangeSale(
      salePriceMin: widget.product['salePriceMin'],
      salePriceMax: widget.product['salePriceMax'],
    );
    _listMin = int.tryParse(widget.product['listPriceMin']?.toString() ?? '');
    _listMax = int.tryParse(widget.product['listPriceMax']?.toString() ?? '');
    _stockQty = _parseTotalStock(widget.product);
    _status = (widget.product['status'] ?? '').toString();
    _loadDetails();
    _setupUploadListener();
  }

  void _setupUploadListener() {
    final productName = _getProductName();
    _updateUploadStatus(productName);
    _uploadSubscription = ProductUploadService().taskStream.listen((_) {
      if (mounted) {
        _updateUploadStatus(productName);
      }
    });
  }

  String _getProductName() {
    final p = widget.product;
    return (p['productName'] ?? p['name'] ?? p['skuName'] ?? '').toString();
  }

  void _updateUploadStatus(String productName) {
    final service = ProductUploadService();
    final isUploading = service.isUploading(productName);
    final progress = service.getUploadProgress(productName) ?? 0.0;

    if (_isUploading != isUploading || _uploadProgress != progress) {
      setState(() {
        _isUploading = isUploading;
        _uploadProgress = progress;
      });

      if (!isUploading && _isUploading) {
        _loadDetails();
      }
    }
  }

  Future<void> _loadDetails() async {
    final urls = extractProductImageUrls(widget.product);
    if (mounted && urls.isNotEmpty) {
      setState(() => _imageUrls = urls);
    }

    // sellerId
    final mRaw =
        widget.product['merchantId'] ??
        widget.product['merchant_id'] ??
        ref.read(sellerStoreIdProvider);
    final String? sellerId = mRaw?.toString();

    // skuId
    final skuRaw =
        widget.product['skuId'] ??
        widget.product['sku_id'] ??
        widget.product['id'];
    final Object? skuId = skuRaw;
    if (skuId == null || sellerId == null) return;

    // 評分（rating summary + reviews）+ grades 各自隔離失敗，互不影響
    // reviews list 跟 buyer merchant_shop_tab 一致：當 source of truth
    // 拿評論數，避免 rating summary endpoint 給錯數量造成「商品有評論但顯示 0」
    final results = await Future.wait<dynamic>([
      ref
          .read(productRepositoryProvider)
          .getPublicProductRatingSummary(skuId)
          .catchError((_) => <String, num>{}),
      ref
          .read(productRepositoryProvider)
          .getSellerProductGrades(sellerId, skuId)
          .catchError((_) => <Map<String, dynamic>>[]),
      ref
          .read(productRepositoryProvider)
          .getSellerProductReviewsPublic(sellerId, skuId)
          .catchError((_) => <Map<String, dynamic>>[]),
    ]);
    if (!mounted) return;

    final rateResp = results[0] is Map
        ? (results[0] as Map<String, dynamic>)
        : <String, dynamic>{};
    final rateData = rateResp['data'] is Map
        ? (rateResp['data'] as Map<String, dynamic>)
        : rateResp;
    final grades = results[1] is List
        ? (results[1] as List).whereType<Map<String, dynamic>>().toList()
        : <Map<String, dynamic>>[];
    final reviewsList = results[2] is List
        ? (results[2] as List).whereType<Map<String, dynamic>>().toList()
        : <Map<String, dynamic>>[];

    double parseNum(dynamic v) {
      if (v == null) return 0;
      if (v is num) return v.toDouble();
      return double.tryParse(v.toString()) ?? 0;
    }

    int? minP;
    int? maxP;
    int? minList;
    int? maxList;
    bool anySale = false;
    int stockTotal = 0;
    bool hasStockValue = false;
    for (final g in grades) {
      // PayUni #758: main price = salePrice ?? price；list price 用於 strikethrough
      final priceVal = g['price'];
      final saleVal = g['salePrice'] ?? g['sale_price'];
      final effective = PriceDisplay.effectiveSkuPrice(
        price: priceVal,
        salePrice: saleVal,
      );
      if (effective != null && effective > 0) {
        final p = effective.toInt();
        if (minP == null || p < minP) minP = p;
        if (maxP == null || p > maxP) maxP = p;
      }
      if (PriceDisplay.hasSkuSale(price: priceVal, salePrice: saleVal)) {
        anySale = true;
      }
      final list = PriceDisplay.parseMoney(priceVal);
      if (list != null && list > 0) {
        final lp = list.toInt();
        if (minList == null || lp < minList) minList = lp;
        if (maxList == null || lp > maxList) maxList = lp;
      }
      final stock = _parseStockValue(g);
      if (stock != null) {
        hasStockValue = true;
        stockTotal += stock;
      }
    }

    // 從 reviews list 算 avg + count（buyer pattern）
    double reviewsAvg = 0;
    int validRated = 0;
    for (final r in reviewsList) {
      final v = parseNum(r['rating'] ?? r['score'] ?? r['stars']);
      if (v > 0) {
        reviewsAvg += v;
        validRated++;
      }
    }
    if (validRated > 0) reviewsAvg /= validRated;

    setState(() {
      // reviews list 有資料就採用（更準）；否則 fallback rating summary endpoint
      if (reviewsList.isNotEmpty) {
        _avgRating = reviewsAvg;
        _ratingCount = reviewsList.length;
      } else {
        _avgRating = parseNum(rateData['rating'] ?? rateData['avg']);
        _ratingCount = parseNum(
          rateData['reviewCount'] ?? rateData['count'],
        ).toInt();
      }
      if (minP != null) _minPrice = minP;
      if (maxP != null) _maxPrice = maxP;
      if (minList != null) _listMin = minList;
      if (maxList != null) _listMax = maxList;
      if (anySale) _hasSale = true;
      if (hasStockValue) _stockQty = stockTotal;
    });
  }

  static int? _parseTotalStock(Map<dynamic, dynamic> source) {
    final direct = _parseStockValue(source);
    if (direct != null) return direct;

    for (final key in [
      'skus',
      'skuList',
      'sku_list',
      'grades',
      'variants',
      'items',
      'productSkus',
      'product_skus',
    ]) {
      final value = source[key];
      if (value is! List) continue;

      var total = 0;
      var hasStockValue = false;
      for (final item in value) {
        if (item is! Map) continue;
        final stock = _parseStockValue(item);
        if (stock == null) continue;
        hasStockValue = true;
        total += stock;
      }
      if (hasStockValue) return total;
    }

    return null;
  }

  static int? _parseStockValue(Map<dynamic, dynamic> source) {
    for (final key in [
      'stockQty',
      'stock_qty',
      'availableQty',
      'available_qty',
      'stock',
      'inventory',
      'qty',
      'quantity',
      'remainingQty',
      'remaining_qty',
      'remainingQuantity',
      'remaining_quantity',
      'totalStock',
      'total_stock',
    ]) {
      final value = source[key];
      if (value == null) continue;
      if (value is int) return value;
      if (value is num) return value.toInt();
      final parsed = int.tryParse(value.toString());
      if (parsed != null) return parsed;
    }
    return null;
  }

  @override
  void dispose() {
    _uploadSubscription?.cancel();
    _pageController.dispose();
    super.dispose();
  }

  bool get _isListed => _status == '4004';

  bool get _isSoldOut {
    if (_isSoldOutByStatus(widget.product)) return true;
    return _stockQty != null && _stockQty! <= 0;
  }

  static bool _isSoldOutByStatus(Map<dynamic, dynamic> source) {
    const soldOutStatuses = {
      'sold_out',
      'soldout',
      'out_of_stock',
      'outofstock',
      'sold-out',
      '已售完',
      '售完',
      '缺貨',
      '無庫存',
      '商品缺貨',
      '9202',
    };
    for (final key in [
      'status',
      'productStatus',
      'product_status',
      'stockStatus',
      'stock_status',
      'inventoryStatus',
      'inventory_status',
    ]) {
      final normalized = source[key]?.toString().toLowerCase().trim() ?? '';
      if (soldOutStatuses.contains(normalized)) return true;
    }

    for (final key in [
      'skus',
      'skuList',
      'sku_list',
      'grades',
      'variants',
      'items',
      'productSkus',
      'product_skus',
    ]) {
      final value = source[key];
      if (value is! List || value.isEmpty) continue;
      final mapItems = value.whereType<Map<dynamic, dynamic>>().toList();
      if (mapItems.isEmpty) continue;
      if (mapItems.every(_isSoldOutByStatus)) return true;
    }

    return false;
  }

  Future<void> _toggleStatus() async {
    final productId = (widget.product['id'] ?? '').toString();
    if (productId.isEmpty) return;

    if (_isListed) {
      // 下架需要確認
      final confirmed = await _showDelistDialog();
      if (confirmed != true || !mounted) return;
    }

    setState(() => _isTogglingStatus = true);
    try {
      final newStatus = _isListed ? '4003' : '4004';
      final result = await ref
          .read(productRepositoryProvider)
          .setProductStatus(productId, status: newStatus);
      if (!mounted) return;
      if (result['code'] == 0) {
        setState(() => _status = newStatus);
        unawaited(
          ref
              .read(analyticsEventsProvider)
              .setProductStatus(
                productId,
                newStatus == '4004' ? 'listed' : 'delisted',
              ),
        );
        if (context.mounted) {
          FloatingToast.success(context, _isListed ? '商品已上架' : '商品已下架');
        }
        if (widget.onEditCompleted != null) {
          await widget.onEditCompleted!();
        }
      } else {
        if (context.mounted) {
          FloatingToast.error(context, '操作失敗');
        }
      }
    } catch (e) {
      if (mounted && context.mounted) {
        FloatingToast.error(context, '操作失敗，請稍後再試');
      }
    } finally {
      if (mounted) setState(() => _isTogglingStatus = false);
    }
  }

  Future<bool?> _showDelistDialog() {
    return showDialog<bool>(
      context: context,
      barrierDismissible: true,
      builder: (dCtx) => Dialog(
        backgroundColor: Colors.transparent,
        insetPadding: const EdgeInsets.symmetric(horizontal: 48),
        child: SingleChildScrollView(
          child: Container(
            decoration: BoxDecoration(
              color: AppColors.bgCard,
              borderRadius: BorderRadius.circular(12),
            ),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Padding(
                  padding: EdgeInsets.fromLTRB(20.w, 24.h, 20.w, 20.h),
                  child: Column(
                    children: [
                      Text(
                        '請問是否要下架呢?',
                        textAlign: TextAlign.center,
                        style: TextStyle(
                          color: AppColors.textEmphasis,
                          fontSize: 14.sp,
                          fontWeight: FontWeight.w700,
                          height: 20 / 14,
                          letterSpacing: 1,
                        ),
                      ),
                      SizedBox(height: 8.h),
                      Padding(
                        padding: EdgeInsets.only(left: 8.w),
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(
                              '• 下架後顧客無法購買商品！',
                              style: TextStyle(
                                color: AppColors.textEmphasis,
                                fontSize: 14.sp,
                                fontWeight: FontWeight.w400,
                                height: 1.6,
                                letterSpacing: 1,
                              ),
                            ),
                            Text(
                              '• 下架後即可編輯商品，編輯後可再度上架',
                              style: TextStyle(
                                color: AppColors.textEmphasis,
                                fontSize: 14.sp,
                                fontWeight: FontWeight.w400,
                                height: 1.6,
                                letterSpacing: 1,
                              ),
                            ),
                          ],
                        ),
                      ),
                    ],
                  ),
                ),
                Container(
                  width: double.infinity,
                  height: 1,
                  color: AppColors.textSecondary,
                ),
                Row(
                  children: [
                    Expanded(
                      child: InkWell(
                        onTap: () => Navigator.of(dCtx).pop(false),
                        child: SizedBox(
                          height: HkControl.minTap,
                          child: Center(
                            child: Text(
                              '取消',
                              textAlign: TextAlign.center,
                              style: TextStyle(
                                color: AppColors.textPrimary,
                                fontSize: 16.sp,
                                fontWeight: FontWeight.w700,
                                height: 1.23,
                                letterSpacing: 1,
                              ),
                            ),
                          ),
                        ),
                      ),
                    ),
                    Container(
                      width: 1,
                      height: HkControl.minTap,
                      color: AppColors.textSecondary,
                    ),
                    Expanded(
                      child: InkWell(
                        onTap: () => Navigator.of(dCtx).pop(true),
                        child: SizedBox(
                          height: HkControl.minTap,
                          child: Center(
                            child: Text(
                              '下架',
                              textAlign: TextAlign.center,
                              style: TextStyle(
                                color: AppColors.textPrimary,
                                fontSize: 16.sp,
                                fontWeight: FontWeight.w700,
                                height: 1.23,
                                letterSpacing: 1,
                              ),
                            ),
                          ),
                        ),
                      ),
                    ),
                  ],
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Future<void> _confirmDelete() async {
    final confirmed = await showDialog<bool>(
      context: context,
      barrierDismissible: true,
      builder: (dCtx) => Dialog(
        backgroundColor: Colors.transparent,
        insetPadding: const EdgeInsets.symmetric(horizontal: 48),
        child: SingleChildScrollView(
          child: Container(
            decoration: BoxDecoration(
              color: AppColors.bgCard,
              borderRadius: BorderRadius.circular(12),
            ),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Container(
                  width: double.infinity,
                  padding: const EdgeInsets.symmetric(
                    vertical: 32,
                    horizontal: 16,
                  ),
                  child: Text(
                    '請問是否要刪除此商品',
                    textAlign: TextAlign.center,
                    style: TextStyle(
                      color: AppColors.textEmphasis,
                      fontSize: 14.sp,
                      fontWeight: FontWeight.w400,
                      height: 1.6,
                      letterSpacing: 1,
                    ),
                  ),
                ),
                Container(
                  width: double.infinity,
                  height: 1,
                  color: AppColors.textSecondary,
                ),
                Row(
                  children: [
                    Expanded(
                      child: InkWell(
                        onTap: () => Navigator.of(dCtx).pop(false),
                        child: SizedBox(
                          height: HkControl.minTap,
                          child: Center(
                            child: Text(
                              '取消',
                              textAlign: TextAlign.center,
                              style: TextStyle(
                                color: AppColors.textPrimary,
                                fontSize: 16.sp,
                                fontWeight: FontWeight.w700,
                                height: 1.23,
                                letterSpacing: 1,
                              ),
                            ),
                          ),
                        ),
                      ),
                    ),
                    Container(
                      width: 1,
                      height: HkControl.minTap,
                      color: AppColors.textSecondary,
                    ),
                    Expanded(
                      child: InkWell(
                        onTap: () => Navigator.of(dCtx).pop(true),
                        child: SizedBox(
                          height: HkControl.minTap,
                          child: Center(
                            child: Text(
                              '刪除',
                              textAlign: TextAlign.center,
                              style: TextStyle(
                                color: AppColors.textPrimary,
                                fontSize: 16.sp,
                                fontWeight: FontWeight.w700,
                                height: 1.23,
                                letterSpacing: 1,
                              ),
                            ),
                          ),
                        ),
                      ),
                    ),
                  ],
                ),
              ],
            ),
          ),
        ),
      ),
    );
    if (confirmed != true || !mounted) return;

    final productId = (widget.product['id'] ?? '').toString();
    if (productId.isEmpty) {
      if (context.mounted) FloatingToast.error(context, '缺少商品資訊');
      return;
    }
    try {
      final result = await ref
          .read(productRepositoryProvider)
          .deleteProduct(productId);
      if (!mounted) return;
      if (result['code'] == 0) {
        if (context.mounted) FloatingToast.success(context, '商品已刪除');
        if (widget.onDeleted != null) await widget.onDeleted!();
      } else {
        if (context.mounted) FloatingToast.error(context, '刪除失敗');
      }
    } catch (e) {
      if (mounted && context.mounted) {
        FloatingToast.error(context, '刪除失敗，請稍後再試');
      }
    }
  }

  Future<void> _editProduct() async {
    // PayUni #758 編輯閘門：ACTIVE(4004) 需先下架；PENDING_REVIEW(4002) 禁編輯
    if (_status == '4002') {
      if (context.mounted) FloatingToast.error(context, '審核中不可編輯');
      return;
    }
    if (_status == '4004') {
      final ok = await _showNeedDelistBeforeEditDialog();
      if (ok != true || !mounted) return;
      final productId = (widget.product['id'] ?? '').toString();
      if (productId.isEmpty) return;
      setState(() => _isTogglingStatus = true);
      try {
        final result = await ref
            .read(productRepositoryProvider)
            .setProductStatus(productId, status: '4003');
        if (!mounted) return;
        if (result['code'] != 0) {
          if (context.mounted) FloatingToast.error(context, '下架失敗');
          return;
        }
        setState(() => _status = '4003');
        if (widget.onEditCompleted != null) {
          await widget.onEditCompleted!();
        }
      } catch (_) {
        if (mounted && context.mounted) {
          FloatingToast.error(context, '下架失敗，請稍後再試');
        }
        return;
      } finally {
        if (mounted) setState(() => _isTogglingStatus = false);
      }
      if (!mounted) return;
    }

    final enrichedProduct = Map<String, dynamic>.from(widget.product);
    enrichedProduct['_loadedImages'] = _imageUrls;
    enrichedProduct['_avgRating'] = _avgRating;
    enrichedProduct['_ratingCount'] = _ratingCount;
    enrichedProduct['_minPrice'] = _minPrice;
    enrichedProduct['_maxPrice'] = _maxPrice;
    enrichedProduct['_productId'] = widget.product['id'];
    enrichedProduct['_needLoadCategory'] = true;
    enrichedProduct['_needLoadGrades'] = true;

    final result = await context.push<dynamic>(
      '/product/picker',
      extra: {'isEditMode': true, 'existingProduct': enrichedProduct},
    );
    if (!mounted) return;
    if (result == true) {
      if (widget.onEditCompleted != null) {
        await widget.onEditCompleted!();
      }
      if (mounted) await _loadDetails();
    }
  }

  Future<bool?> _showNeedDelistBeforeEditDialog() {
    return showDialog<bool>(
      context: context,
      barrierDismissible: true,
      builder: (dCtx) => Dialog(
        backgroundColor: Colors.transparent,
        insetPadding: const EdgeInsets.symmetric(horizontal: 48),
        child: SingleChildScrollView(
          child: Container(
            decoration: BoxDecoration(
              color: AppColors.bgCard,
              borderRadius: BorderRadius.circular(12),
            ),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Padding(
                  padding: EdgeInsets.fromLTRB(20.w, 24.h, 20.w, 20.h),
                  child: Text(
                    '商品需先下架後再編輯',
                    textAlign: TextAlign.center,
                    style: TextStyle(
                      color: AppColors.textEmphasis,
                      fontSize: 14.sp,
                      fontWeight: FontWeight.w700,
                      height: 20 / 14,
                      letterSpacing: 1,
                    ),
                  ),
                ),
                Container(
                  width: double.infinity,
                  height: 1,
                  color: AppColors.textSecondary,
                ),
                Row(
                  children: [
                    Expanded(
                      child: InkWell(
                        onTap: () => Navigator.of(dCtx).pop(false),
                        child: SizedBox(
                          height: HkControl.minTap,
                          child: Center(
                            child: Text(
                              '取消',
                              style: TextStyle(
                                color: AppColors.textPrimary,
                                fontSize: 16.sp,
                                fontWeight: FontWeight.w700,
                                letterSpacing: 1,
                              ),
                            ),
                          ),
                        ),
                      ),
                    ),
                    Container(
                      width: 1,
                      height: HkControl.minTap,
                      color: AppColors.textSecondary,
                    ),
                    Expanded(
                      child: InkWell(
                        onTap: () => Navigator.of(dCtx).pop(true),
                        child: SizedBox(
                          height: HkControl.minTap,
                          child: Center(
                            child: Text(
                              '先下架',
                              style: TextStyle(
                                color: AppColors.textPrimary,
                                fontSize: 16.sp,
                                fontWeight: FontWeight.w700,
                                letterSpacing: 1,
                              ),
                            ),
                          ),
                        ),
                      ),
                    ),
                  ],
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildShareControl({
    required String title,
    required String priceDisplay,
  }) {
    final share = ref.watch(storeShareProvider);
    if (share is! StoreShareEnabled) {
      return HkDisabledWithHint(
        key: const ValueKey('seller-product-share'),
        semanticLabel: '分享商品',
        hint: share is StoreShareDisabled ? kProductShareDisabledHint : null,
        child: HkIcon(
          HkIconName.externalLink,
          size: 18.sp,
          color: AppIconColors.disabled,
        ),
      );
    }
    return GestureDetector(
      onTap: () async {
        final productId = (widget.product['id'] ?? '').toString();
        final data = await ref
            .read(postRepositoryProvider)
            .getShareUrl(type: 'product', id: productId);
        if (!mounted) return;
        if (data == null) {
          FloatingToast.error(context, kShareFailedMessage);
          return;
        }
        showShareSheet(context, data);
      },
      behavior: HitTestBehavior.opaque,
      child: SizedBox(
        key: const ValueKey('seller-product-share'),
        width: HkControl.minTap,
        height: HkControl.minTap,
        child: Center(
          child: HkIcon(
            HkIconName.externalLink,
            size: 18.sp,
            color: AppIconColors.muted,
            semanticLabel: '分享商品',
          ),
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final p = widget.product;
    final String title = (p['productName'] ?? p['name'] ?? p['skuName'] ?? '商品')
        .toString();

    final double avg = (_avgRating ?? 0).toDouble();
    final double clamped = avg.isNaN ? 0 : avg.clamp(0, 5).toDouble();
    final int ratingCount =
        _ratingCount ?? (p['ratingCount'] is int ? p['ratingCount'] as int : 0);
    final bool isSoldOut = _isSoldOut;

    // 判準走 PriceDisplay：後端契約以 `"0"` 表達「無價格」，
    // `int.tryParse("0")` 拿到 0 會印成 `NT $0`。空字串＝不顯示價格列。
    // PayUni #758: 區間缺席才退回 SKU 的 salePrice ?? price。
    final String priceDisplay = PriceDisplay.productCardPrice(
      priceMin: _minPrice,
      priceMax: _maxPrice,
      skuPrice: p['price'],
      skuSalePrice: p['salePrice'] ?? p['sale_price'],
      prefix: 'NT \$',
    );
    // listPrice strikethrough：salePriceMin != "0" 或 per-SKU 偵測到 sale
    String? strikeListDisplay;
    if (_hasSale) {
      final strike = PriceDisplay.formatPriceRange(
        _listMin,
        _listMax,
        prefix: 'NT \$',
      );
      if (strike.isNotEmpty) strikeListDisplay = strike;
    }

    final hashtags = HashtagExtractor.fromProduct(widget.product);

    Widget buildImage(double width) {
      return ClipRRect(
        borderRadius: BorderRadius.circular(12.r),
        child: SizedBox(
          width: width,
          height: width * 3 / 4,
          child: Stack(
            fit: StackFit.expand,
            children: [
              if (_imageUrls.isEmpty)
                Container(
                  color: AppColors.bgPrimary,
                  child: Center(
                    child: Icon(
                      AppIcons.imageOutlined,
                      size: 28.sp,
                      color: AppIconColors.disabled,
                    ),
                  ),
                )
              else
                PageView.builder(
                  controller: _pageController,
                  onPageChanged: (i) => setState(() => _pageIndex = i),
                  itemCount: _imageUrls.length,
                  itemBuilder: (_, i) {
                    final url = _imageUrls[i];
                    final image = OptimizedNetworkImage(
                      imageUrl: url,
                      scene: ImageScene.productDetail,
                      fit: BoxFit.cover,
                      errorWidget: Container(
                        color: AppColors.bgPrimary,
                        child: const Center(
                          child: Icon(
                            AppIcons.brokenImageOutlined,
                            color: AppIconColors.muted,
                          ),
                        ),
                      ),
                    );
                    if (!isSoldOut) return image;
                    return ColorFiltered(
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
                      child: Opacity(opacity: 0.65, child: image),
                    );
                  },
                ),
              if (isSoldOut)
                Positioned(top: 6.h, left: 6.w, child: _SellerSoldOutPill()),
              if (_imageUrls.length > 1)
                Positioned(
                  bottom: 4,
                  left: 0,
                  right: 0,
                  child: LayoutBuilder(
                    builder: (context, constraints) {
                      // 圓點寬度固定（含左右 margin），數量超過可視寬度容納上限時
                      // 只畫前 N 顆，避免這個沒有 Expanded/Flexible 保護的 Row 被撐爆
                      // （margin 未隨 ScreenUtil 縮放，圖片張數多或縮放比例偏低都會超版）。
                      const dotMargin = 4.0;
                      final perDot = 5.w + dotMargin * 2;
                      final maxFit = (constraints.maxWidth / perDot).floor();
                      final visibleCount = _imageUrls.length < maxFit
                          ? _imageUrls.length
                          : (maxFit < 1 ? 1 : maxFit);
                      return Row(
                        mainAxisAlignment: MainAxisAlignment.center,
                        children: List.generate(
                          visibleCount,
                          (i) => Container(
                            margin: const EdgeInsets.symmetric(
                              horizontal: dotMargin,
                            ),
                            width: 5.w,
                            height: 5.h,
                            decoration: BoxDecoration(
                              color: i == _pageIndex
                                  ? AppColors.bgCard
                                  : AppColors.bgCard.withValues(alpha: 0.5),
                              shape: BoxShape.circle,
                            ),
                          ),
                        ),
                      );
                    },
                  ),
                ),
              // hashtag 底部暗化漸層（增強對比）
              if (hashtags.isNotEmpty)
                Positioned(
                  left: 0,
                  right: 0,
                  bottom: 0,
                  height: 28.h,
                  child: IgnorePointer(
                    child: Opacity(
                      opacity: 0.22,
                      child: Container(
                        decoration: const BoxDecoration(
                          gradient: LinearGradient(
                            begin: Alignment.bottomCenter,
                            end: Alignment.topCenter,
                            colors: [AppColors.textEmphasis, Color(0x00C0C0C0)],
                          ),
                        ),
                      ),
                    ),
                  ),
                ),
              // hashtag chip overlay（左下，僅顯示第一個避免擠到）
              if (hashtags.isNotEmpty)
                Positioned(
                  left: 4.w,
                  right: 4.w,
                  bottom: 4.h,
                  child: IgnorePointer(
                    child: ClipRect(
                      clipBehavior: Clip.hardEdge,
                      child: Row(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          Flexible(
                            child: _SellerHashtagChip(
                              text: '#${hashtags.first}',
                            ),
                          ),
                        ],
                      ),
                    ),
                  ),
                ),
              if (_isUploading)
                Positioned.fill(
                  child: Container(
                    color: const Color(0x99000000),
                    child: Column(
                      mainAxisAlignment: MainAxisAlignment.center,
                      children: [
                        FruitSpinner(size: 24.w),
                        SizedBox(height: 4.h),
                        Text(
                          '${(_uploadProgress * 100).toInt()}%',
                          style: TextStyle(
                            fontSize: 12.sp,
                            color: AppColors.bgCard,
                          ),
                        ),
                      ],
                    ),
                  ),
                ),
            ],
          ),
        ),
      );
    }

    Widget buildActions() {
      return Row(
        children: [
          Expanded(
            child: _buildPillButton(
              label: _isListed ? '下架' : '上架',
              color: _isListed ? AppColors.primary : AppColors.warning,
              onTap: _isTogglingStatus ? null : _toggleStatus,
            ),
          ),
          SizedBox(width: 8.w),
          Expanded(
            child: _buildPillButton(
              label: '刪除',
              color: AppColors.primary,
              onTap: _confirmDelete,
            ),
          ),
          SizedBox(width: 8.w),
          Expanded(
            child: _buildPillButton(
              label: '編輯',
              color: AppColors.primary,
              opacity: _status == '4002' ? 0.5 : 1.0,
              onTap: _status == '4002' ? null : _editProduct,
            ),
          ),
        ],
      );
    }

    return GestureDetector(
      onTap: () {
        context.push('/product/preview', extra: {'product': widget.product});
      },
      child: Container(
        decoration: BoxDecoration(
          gradient: LinearGradient(
            begin: Alignment.topLeft,
            end: Alignment.bottomRight,
            colors: [
              AppColors.neutralWhite.withValues(alpha: 0.94),
              AppColors.neutralCream.withValues(alpha: 0.82),
            ],
          ),
          borderRadius: BorderRadius.circular(16.r),
          border: Border.all(
            color: AppColors.neutralWhite.withValues(alpha: 0.96),
          ),
          boxShadow: HkShadows.card,
        ),
        clipBehavior: Clip.antiAlias,
        child: Padding(
          padding: EdgeInsets.all(12.w),
          child: LayoutBuilder(
            builder: (context, constraints) {
              final imageWidth = (constraints.maxWidth * 0.36).clamp(
                96.0,
                140.0,
              );
              final imageHeight = imageWidth * 3 / 4;
              return Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Row(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      buildImage(imageWidth),
                      SizedBox(width: 10.w),
                      // 右側資訊
                      Expanded(
                        child: ConstrainedBox(
                          constraints: BoxConstraints(minHeight: imageHeight),
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            mainAxisAlignment: MainAxisAlignment.spaceBetween,
                            children: [
                              // 標題 + 分享
                              Row(
                                crossAxisAlignment: CrossAxisAlignment.start,
                                children: [
                                  Expanded(
                                    child: Text(
                                      title,
                                      maxLines: 2,
                                      overflow: TextOverflow.ellipsis,
                                      style: TextStyle(
                                        fontSize: 14.sp,
                                        fontWeight: FontWeight.w700,
                                        color: isSoldOut
                                            ? AppColors.textSecondary
                                            : AppColors.textPrimary,
                                      ),
                                    ),
                                  ),
                                  SizedBox(width: 4.w),
                                  _buildShareControl(
                                    title: title,
                                    priceDisplay: priceDisplay,
                                  ),
                                ],
                              ),
                              // 評分
                              Row(
                                children: [
                                  Icon(
                                    AppIcons.star,
                                    size: 14.sp,
                                    color: AppIconColors.gold,
                                  ),
                                  SizedBox(width: 3.w),
                                  Expanded(
                                    child: Text(
                                      '${clamped.toStringAsFixed(1)} ($ratingCount則評價)',
                                      maxLines: 1,
                                      overflow: TextOverflow.ellipsis,
                                      style: TextStyle(
                                        fontSize: 12.sp,
                                        fontWeight: FontWeight.w400,
                                        color: isSoldOut
                                            ? AppColors.textSecondary
                                            : AppColors.textPrimary,
                                      ),
                                    ),
                                  ),
                                ],
                              ),
                            ],
                          ),
                        ),
                      ),
                    ],
                  ),
                  if (priceDisplay.isNotEmpty) ...[
                    SizedBox(height: 8.h),
                    Align(
                      alignment: Alignment.centerLeft,
                      child: Wrap(
                        spacing: 8.w,
                        runSpacing: 4.h,
                        crossAxisAlignment: WrapCrossAlignment.center,
                        children: [
                          Text(
                            priceDisplay,
                            key: const ValueKey('seller-product-price'),
                            style: TextStyle(
                              fontSize: 16.sp,
                              fontWeight: FontWeight.w700,
                              color: isSoldOut
                                  ? AppColors.textSecondary
                                  : AppColors.primary,
                            ),
                          ),
                          if (strikeListDisplay != null)
                            Text(
                              strikeListDisplay.replaceFirst('NT\$', '\$'),
                              key: const ValueKey(
                                'seller-product-original-price',
                              ),
                              style: TextStyle(
                                fontSize: 12.sp,
                                color: AppColors.textSecondary,
                                fontWeight: FontWeight.w400,
                                letterSpacing: 0.2,
                                decoration: TextDecoration.lineThrough,
                                decorationColor: AppColors.textSecondary,
                                decorationThickness: 1.2,
                              ),
                            ),
                        ],
                      ),
                    ),
                  ],
                  SizedBox(height: 4.h),
                  buildActions(),
                  if (isSoldOut) ...[
                    SizedBox(height: 10.h),
                    _SellerSoldOutNotice(
                      actionLabel: _isListed ? '先下架' : '補貨',
                      onAction: _isListed
                          ? (_isTogglingStatus ? null : _toggleStatus)
                          : _editProduct,
                    ),
                  ],
                ],
              );
            },
          ),
        ),
      ),
    );
  }

  Widget _buildPillButton({
    required String label,
    required Color color,
    double opacity = 1.0,
    VoidCallback? onTap,
  }) {
    // 符合 Material 48dp / Apple HIG 44pt 最小可點擊範圍：
    // 視覺高度 32h，外層額外 hit area 用 InkWell behavior
    return Opacity(
      opacity: opacity,
      child: Material(
        color: Colors.transparent,
        child: InkWell(
          key: ValueKey('seller-product-action-$label'),
          onTap: onTap,
          borderRadius: BorderRadius.circular(999),
          child: SizedBox(
            height: HkControl.minTap,
            child: Center(
              child: Container(
                width: double.infinity,
                height: 32.h,
                alignment: Alignment.center,
                padding: EdgeInsets.symmetric(horizontal: 12.w),
                decoration: BoxDecoration(
                  color: color,
                  borderRadius: BorderRadius.circular(999),
                ),
                child: Text(
                  label,
                  style: TextStyle(
                    fontSize: 14.sp,
                    fontWeight: FontWeight.w700,
                    color: color.computeLuminance() > 0.4
                        ? AppColors.textEmphasis
                        : AppColors.bgCard,
                    height: 1.2,
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

/// 圖片 overlay 上的 hashtag chip — 對齊買家 ProductCard 的 _HashtagChip。
class _SellerHashtagChip extends StatelessWidget {
  const _SellerHashtagChip({required this.text});
  final String text;

  @override
  Widget build(BuildContext context) {
    return Container(
      height: 22.h,
      padding: EdgeInsets.symmetric(horizontal: 8.w),
      alignment: Alignment.center,
      decoration: BoxDecoration(
        color: const Color(0xCCFFFFFF),
        borderRadius: BorderRadius.circular(12.r),
        border: Border.all(color: const Color(0x33000000), width: 0.5),
        boxShadow: const [
          BoxShadow(
            color: AppColors.cardShadow,
            blurRadius: 4,
            offset: Offset(0, 1),
          ),
        ],
      ),
      child: Text(
        text,
        maxLines: 1,
        overflow: TextOverflow.ellipsis,
        softWrap: false,
        style: TextStyle(
          color: AppColors.brandPrimaryStrong,
          fontSize: 12.sp,
          fontWeight: FontWeight.w700,
          height: 1.0,
        ),
      ),
    );
  }
}

class _SellerSoldOutPill extends StatelessWidget {
  const _SellerSoldOutPill();

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: EdgeInsets.symmetric(horizontal: 8.w, vertical: 4.h),
      decoration: BoxDecoration(
        color: const Color(0xCC191C18),
        borderRadius: BorderRadius.circular(8.r),
      ),
      child: Text(
        '已售完',
        style: TextStyle(
          color: AppColors.bgCard,
          fontSize: 12.sp,
          fontWeight: FontWeight.w700,
          height: 1.2,
        ),
      ),
    );
  }
}

class _SellerSoldOutNotice extends StatelessWidget {
  const _SellerSoldOutNotice({
    required this.actionLabel,
    required this.onAction,
  });

  final String actionLabel;
  final VoidCallback? onAction;

  @override
  Widget build(BuildContext context) {
    final enabled = onAction != null;

    return Container(
      width: double.infinity,
      padding: EdgeInsets.symmetric(horizontal: 12.w, vertical: 12.h),
      decoration: BoxDecoration(
        color: AppColors.bgCard,
        borderRadius: BorderRadius.circular(12.r),
        border: Border.all(color: const Color(0x33E0C44C)),
      ),
      child: Row(
        children: [
          Container(
            width: 28.w,
            height: 28.w,
            decoration: const BoxDecoration(
              color: AppColors.warning,
              shape: BoxShape.circle,
            ),
            child: HkIcon(
              HkIconName.inventory,
              color: AppIconColors.strong,
              size: 16.sp,
            ),
          ),
          SizedBox(width: 10.w),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(
                  '已售完，買家端已停止購買',
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(
                    fontSize: 14.sp,
                    fontWeight: FontWeight.w700,
                    color: AppColors.textPrimary,
                    height: 1.25,
                  ),
                ),
                SizedBox(height: 2.h),
                Text(
                  '請補貨或下架商品，避免買家誤會。',
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(
                    fontSize: 12.sp,
                    fontWeight: FontWeight.w400,
                    color: const Color(0x9942493F),
                    height: 1.25,
                  ),
                ),
              ],
            ),
          ),
          SizedBox(width: 8.w),
          GestureDetector(
            onTap: enabled ? onAction : null,
            child: Opacity(
              opacity: enabled ? 1 : 0.5,
              child: Container(
                padding: EdgeInsets.symmetric(horizontal: 12.w, vertical: 8.h),
                decoration: BoxDecoration(
                  color: enabled ? AppColors.warning : AppColors.gray200,
                  borderRadius: BorderRadius.circular(999),
                ),
                child: Text(
                  actionLabel,
                  style: TextStyle(
                    fontSize: 12.sp,
                    fontWeight: FontWeight.w700,
                    color: AppColors.textEmphasis,
                    height: 1.2,
                  ),
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }
}
