import 'dart:async';
import 'dart:convert';
import 'dart:math' as math;
import 'dart:ui' as ui;

import 'package:hawkr_buyer/shared/widgets/hk_frosted_icon_button.dart';
import 'package:hawkr_buyer/core/codemeta/code_meta.dart';
import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:url_launcher/url_launcher.dart';

import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../data/product_repository.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart' as seller;
import 'package:hawkr_api/images.dart' show ImagePreset, ImageUri;
import '../../seller/data/seller_repository.dart';
import '../../seller/data/store_card_display.dart';
import '../../review/data/review_repository.dart';
import '../../cart/data/cart_repository.dart';
import '../../cart/data/cart_error_message.dart';
import '../../cart/presentation/cart_controller.dart';
import '../../../config/app_constants.dart';
import '../../../core/session/session_provider.dart';
import '../../../core/storage/auth_storage.dart';
import '../../../core/networking/oss_service.dart';
import '../../../core/routing/floating_detail_page.dart';
import '../../../config/media_image.dart';
import '../../../core/theme/app_typography.dart';
import '../../../core/theme/hk_tokens.dart';

import '../../../shared/widgets/hk_tap_target.dart';
import '../../../shared/widgets/hk_liquid_glass.dart';

import '../../../shared/widgets/common_dialog.dart';
import '../../../shared/widgets/common_snackbar.dart';
import '../../../config/image_config.dart';
import '../../../shared/widgets/app_cached_image.dart';
import '../../../shared/widgets/oss_image.dart';
import 'widgets/product_image_thumbnail_grid.dart';
import 'widgets/product_purchase_panel.dart';

import '../../../shared/utils/hk_log.dart';
import '../../moderation/presentation/report_block_action_sheet.dart';
import '../../../shared/utils/price_display.dart';
import '../../../shared/utils/require_login.dart';
import '../../../shared/widgets/share_bottom_sheet.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

import '../../../core/analytics/analytics_providers.dart';
import '../../../core/analytics/ga4_item.dart';
import '../../../shared/utils/avatar_wire.dart';
import '../../../shared/widgets/product_card_hero.dart';
import 'product_detail_preview_layout.dart';
import 'package:hawkr_buyer/shared/widgets/hk_brand_loader.dart';
import 'package:hawkr_buyer/shared/widgets/hk_placeholder.dart';

/// 商家商品詳細頁（從商家頁商品卡片進入）
/// - 上方綠色 banner 與商家頁一致色系
/// - 下方導航欄沿用前頁 icon 與樣式
class MerchantItemDetailScreen extends ConsumerStatefulWidget {
  const MerchantItemDetailScreen({
    super.key,
    this.title = '商品',
    this.imageUrl,
    this.price,
    this.skuId,
    this.merchantName,
    this.merchantId,
    this.heroTag,
    this.autoFocusSpecs = false,
    this.initialRestockSubscribed = false,
    this.onResultChanged,
  });

  final String title;
  final String? imageUrl;
  final num? price; // 未稅或含稅無硬性規定，僅展示
  final String? skuId;
  final String? merchantName;
  final String? merchantId;
  final Object? heroTag;
  final bool initialRestockSubscribed;
  final ValueChanged<Map<String, dynamic>>? onResultChanged;

  /// 進入頁面後自動滾動到規格選擇區
  final bool autoFocusSpecs;

  @override
  ConsumerState<MerchantItemDetailScreen> createState() =>
      _MerchantItemDetailScreenState();
}

class _MerchantItemDetailScreenState
    extends ConsumerState<MerchantItemDetailScreen> {
  late List<String> _images;
  late final PageController _pageController;
  final ScrollController _scrollController = ScrollController();
  final GlobalKey _specSectionKey = GlobalKey();
  int _currentIndex = 0;

  // Size Spec selection state
  Map<String, dynamic>? _selectedGrade; // selected grade map
  String? _selectedGradeId; // selected grade id
  int _selectedQty = 1; // purchase surfaces share a minimum quantity of one
  bool _favoriteLoading = false;
  bool _shareLoading = false;
  bool _isFavorite = false;
  bool _selfOwned = false;
  bool _addingToCart = false;
  bool _directingCheckout = false;
  bool _viewItemSent = false;
  // 補貨通知訂閱狀態（#942）— 售完時 PDP 顯示鈴鐺；初始值由 _loadProductDetail 從 raw map 讀
  bool _isRestockSubscribed = false;
  bool _restockLoading = false;
  OverlayEntry? _checkoutToastEntry;

  // 由 _loadProductDetail 從 seller.sellerAccountId / seller.name 解析
  String _resolvedMerchantId = '';
  String _resolvedMerchantName = '';

  // 商品詳情（由 _loadProductDetail 填入）
  String _productDescription = '';
  List<Map<String, dynamic>> _productDescriptionBlocks = [];
  // B-132 三層分類名稱（後端會在 /public/products/{id} 回傳）
  String _categoryLv1 = '';
  String _category = ''; // = categoryLv2Name
  String _subCategory = ''; // = categoryLv3Name
  String _origin = '';

  /// 完整分類路徑（Lv1 → Lv2 → Lv3，未來商品詳情可能用於 breadcrumb）
  // ignore: unused_element
  String get _categoryBreadcrumb {
    final parts = [
      _categoryLv1,
      _category,
      _subCategory,
    ].where((s) => s.isNotEmpty).toList();
    return parts.join(' › ');
  }

  String _location = '';
  String _storeCode = '';
  String _storeAvatarUrl = '';
  String _storeName = '';
  double _storeRating = 0.0;
  List<String> _shippingMethodCodes = const [];
  // 由 /public/products/{id} 的 skus 載入的規格列表
  List<Map<String, dynamic>>? _grades;

  @override
  void initState() {
    super.initState();
    // 移除示意用假圖，僅使用 API 與（必要時）單一備援網址
    _images = [];
    _pageController = PageController(initialPage: 0);
    _resolvedMerchantId = widget.merchantId?.trim() ?? '';
    _resolvedMerchantName = widget.merchantName?.trim() ?? '';
    _isRestockSubscribed = widget.initialRestockSubscribed;

    // 異步載入初始圖片（獲取預簽名 URL）
    _loadInitialImage();

    // 載入商品詳情（含圖片、收藏狀態）
    // _recordProductVisit 在 _loadProductDetail 後執行，確保 _resolvedMerchantId 已填入
    _loadProductDetail().then((_) => _recordProductVisit());
  }

  Future<void> _recordProductVisit() async {
    // 瀏覽 beacon 一律送出，登入的呼叫者由伺服器略過。
    final viewedProductId = (widget.skuId ?? '').trim();
    if (viewedProductId.isNotEmpty) {
      unawaited(
        ref.read(sellerRepositoryProvider).recordProductView(viewedProductId),
      );
    }
    // 訪客（無 token）打 /app/buyers/me/browse-history 必 401，直接 skip。
    final session = ref.read(sessionProvider).value;
    if (!hasUsableSession(session)) {
      return;
    }
    final mid = _resolvedMerchantId.isNotEmpty
        ? _resolvedMerchantId
        : (widget.merchantId ?? '').trim();
    final mname = _resolvedMerchantName.isNotEmpty
        ? _resolvedMerchantName
        : (widget.merchantName ?? '').trim();
    final pid = (widget.skuId ?? '').trim();
    final pname = (widget.title).trim();
    if (mid.isEmpty || pid.isEmpty) {
      HKLog.logW(
        '[VisitAPI] SKIP product visit: missing field mid="$mid" pid="$pid"',
      );
      return;
    }
    HKLog.logD(
      '[VisitAPI] record product visit -> mid=$mid, mname=$mname, pid=$pid, pname=$pname',
    );
    await ref
        .read(productRepositoryProvider)
        .visitProduct(
          merchantId: mid,
          merchantName: mname,
          productId: pid,
          productName: pname,
        );
  }

  Future<void> _toggleFavorite() async {
    if (!await requireLogin(context, ref)) return;
    if (!mounted) return;
    final pid = (widget.skuId ?? '').trim();
    if (pid.isEmpty) return;
    if (_favoriteLoading) return;
    final bool target = !_isFavorite;
    setState(() {
      _isFavorite = target;
      _favoriteLoading = true;
    });
    try {
      bool ok = false;
      if (target) {
        ok = await ref
            .read(productRepositoryProvider)
            .addFavoriteProduct(productId: pid);
      } else {
        ok = await ref
            .read(productRepositoryProvider)
            .removeFavoriteProduct(pid);
      }
      if (!ok && mounted) {
        setState(() => _isFavorite = !target);
      }
      // Analytics: add_to_wishlist — only on successful add (not remove)
      if (target && ok) {
        ref
            .read(analyticsEventsProvider)
            .trackAddToWishlist(
              GA4Item(
                itemId: pid,
                itemName: widget.title,
                price: PriceDisplay.parseMoney(widget.price),
                itemBrand: _resolvedMerchantName.isNotEmpty
                    ? _resolvedMerchantName
                    : null,
              ),
            );
      }
    } catch (_) {
      if (mounted) setState(() => _isFavorite = !target);
    } finally {
      if (mounted) setState(() => _favoriteLoading = false);
    }
  }

  /// 商品是否全規格售完（_grades 已載入且每個規格 quantity <= 0）— 用於 PDP 售完區塊判斷
  bool get _isProductOutOfStock {
    final grades = _grades;
    if (grades == null || grades.isEmpty) return false;
    int qtyOf(Map<String, dynamic> m) {
      final qv = m['quantity'];
      return (qv is num) ? qv.toInt() : int.tryParse('$qv') ?? 0;
    }

    return grades.every((g) => qtyOf(g) <= 0);
  }

  /// 訂閱 / 取消補貨通知 — POST / DELETE /app/products/{productId}/restock_subscriptions（#942）
  Future<void> _toggleRestockSubscription() async {
    if (!await requireLogin(context, ref)) return;
    if (!mounted) return;
    final pid = (widget.skuId ?? '').trim();
    if (pid.isEmpty) return;
    if (_restockLoading) return;
    final bool target = !_isRestockSubscribed;
    setState(() {
      _isRestockSubscribed = target;
      _restockLoading = true;
    });
    try {
      final repo = ref.read(productRepositoryProvider);
      final ok = target
          ? await repo.subscribeRestock(pid)
          : await repo.unsubscribeRestock(pid);
      if (!mounted) return;
      if (!ok) {
        setState(() => _isRestockSubscribed = !target);
        showFloatingToast(context, '操作失敗，請稍後再試', isSuccess: false);
        return;
      }
      showFloatingToast(
        context,
        target ? '已開啟補貨通知，補貨時會通知你' : '已取消補貨通知',
        isSuccess: true,
      );
    } catch (_) {
      if (mounted) {
        setState(() => _isRestockSubscribed = !target);
        showFloatingToast(context, '操作失敗，請稍後再試', isSuccess: false);
      }
    } finally {
      if (mounted) setState(() => _restockLoading = false);
    }
  }

  Future<void> _handleShare() async {
    if (_shareLoading) return;
    final skuId = (widget.skuId ?? '').trim();
    setState(() => _shareLoading = true);
    try {
      final data = await ref
          .read(productRepositoryProvider)
          .getShareUrl(type: 'product', id: skuId);
      if (!mounted) return;
      final url =
          data?['url'] ?? AppConstants.shareFallbackUrl('/products/$skuId');
      if (url == null) return;
      showShareSheet(
        context,
        ShareData(
          title: (data?['title'] ?? widget.title).toString(),
          description: (data?['description'] ?? widget.merchantName ?? '')
              .toString(),
          imageUrl: (data?['imageUrl'] ?? widget.imageUrl ?? '').toString(),
          url: url.toString(),
        ),
      );
    } catch (_) {
      // silently ignore — share sheet not shown
    } finally {
      if (mounted) setState(() => _shareLoading = false);
    }
  }

  /// 載入初始圖片（解析 objectKey 或 URL）
  void _loadInitialImage() {
    final url = (widget.imageUrl ?? '').trim();
    if (url.isEmpty) return;

    final resolved = OssService.resolveImageUrl(url);
    if (resolved != null && resolved.isNotEmpty && mounted && _images.isEmpty) {
      setState(() {
        _images = [resolved];
        _currentIndex = 0;
      });
    }
  }

  static String _firstNonEmpty(List<dynamic> values) {
    for (final v in values) {
      final s = (v ?? '').toString().trim();
      if (s.isNotEmpty) return s;
    }
    return '';
  }

  static String _buildSellerLocation(Map<String, dynamic>? info) {
    if (info == null || info.isEmpty) return '';
    final city = (info['city'] ?? '').toString().trim();
    final district = (info['district'] ?? '').toString().trim();
    if (city.isNotEmpty && district.isNotEmpty) return '$city$district';
    return city.isNotEmpty ? city : district;
  }

  static List<String> _categoryNamesFromPath(Map<String, dynamic> pathData) {
    final rawPath = pathData['path'] ?? pathData['categories'];
    if (rawPath is List) {
      return rawPath
          .whereType<Map>()
          .map((e) => (e['name'] ?? e['categoryName'] ?? '').toString().trim())
          .where((name) => name.isNotEmpty)
          .toList();
    }
    return [
          pathData['categoryLv1Name'],
          pathData['categoryLv2Name'],
          pathData['categoryLv3Name'],
          pathData['name'],
        ]
        .map((e) => (e ?? '').toString().trim())
        .where((e) => e.isNotEmpty)
        .toList();
  }

  /// GET /public/products/{skuId} — 載入完整商品資料（圖片、規格、分類、說明）
  Future<void> _loadProductDetail() async {
    final sid = (widget.skuId ?? '').trim();
    if (sid.isEmpty) return;
    try {
      final info = await ref
          .read(productRepositoryProvider)
          .getProductFullInfo(sid);
      if (!mounted || info.isEmpty) return;

      // 圖片：49 §6.1 成對契約，優先 key 陣列 imageKeys，其次 imageUrls；
      // imageKey/imageUrl 作為 fallback 封面圖。值一律過 buildImageUrl 顯示。
      final imagesList = info['imageKeys'] ?? info['imageUrls'];
      final List<String> imgs = [];
      if (imagesList is List) {
        for (final img in imagesList) {
          final url = img.toString().trim();
          if (url.isNotEmpty && url != 'null') imgs.add(url);
        }
      }
      if (imgs.isEmpty) {
        final cover = (info['imageKey'] ?? info['imageUrl'] ?? '')
            .toString()
            .trim();
        if (cover.isNotEmpty && cover != 'null') imgs.add(cover);
      }

      // 說明：description.blocks[].content 串接
      final desc = info['description'];
      String descText = '';
      List<Map<String, dynamic>> descBlocks = [];
      if (desc is Map) {
        final blocks = desc['blocks'];
        if (blocks is List) {
          descBlocks = blocks
              .whereType<Map>()
              .map((b) => Map<String, dynamic>.from(b))
              .toList();
          descText = descBlocks
              .map((b) => (b['content'] ?? '').toString())
              .where((s) => s.isNotEmpty)
              .join('\n');
        }
      } else if (desc is String) {
        descText = desc;
      }

      // 配送資訊保留後端原始 code，與快速選購彈窗共用同一個語意元件。
      final shippingMethodCodes = <String>[];
      final shippingMethods = info['shippingMethods'];
      if (shippingMethods is List) {
        shippingMethodCodes.addAll(
          shippingMethods
              .map(
                (method) => method is Map
                    ? (method['type'] ?? method['shippingMethod'] ?? '')
                          .toString()
                    : method.toString(),
              )
              .map((code) => code.trim())
              .where((code) => code.isNotEmpty),
        );
      }

      // 規格：skus 陣列（直接使用 API 欄位，不呼叫額外端點）
      final grades = normalizeProductPurchaseSpecs(info['skus']);
      final initialGrade =
          firstAvailablePurchaseSpec(grades) ??
          (grades.isNotEmpty ? grades.first : null);

      // 從 seller 解析 merchantId / merchantName
      final sellerMap = info['seller'];
      final sellerInfo = sellerMap is Map
          ? Map<String, dynamic>.from(sellerMap)
          : <String, dynamic>{};
      // 44 號身分分離：商品詳情的賣家鍵是 storeId（store surrogate），
      // 巢狀 seller 物件與 flat 層都有。
      final sellerId = _firstNonEmpty([sellerInfo['storeId'], info['storeId']]);
      final sellerName = (sellerInfo['name'] ?? '').toString().trim();
      var sellerLocation = _buildSellerLocation(sellerInfo);
      final storeCode = _firstNonEmpty([
        sellerInfo['storeCode'],
        info['storeCode'],
      ]);
      seller.GetStoreResponse? store;
      if (storeCode.isNotEmpty) {
        try {
          store = await ref.read(sellerRepositoryProvider).getStore(storeCode);
        } catch (_) {
          store = null;
        }
      }
      if (sellerLocation.isEmpty && store != null) {
        sellerLocation = townLocation(store.store.townCode);
      }
      var categoryLv1 = (info['categoryLv1Name'] ?? '').toString();
      var categoryLv2 = _firstNonEmpty([
        info['categoryLv2Name'],
        info['categoryName'],
      ]);
      var categoryLv3 = _firstNonEmpty([
        info['categoryLv3Name'],
        info['categoryLv2Name'],
      ]);
      if (categoryLv1.isEmpty && categoryLv2.isEmpty && categoryLv3.isEmpty) {
        final categoryId = int.tryParse((info['categoryId'] ?? '').toString());
        if (categoryId != null) {
          final pathData = await ref
              .read(productRepositoryProvider)
              .getCategoryPath(categoryId);
          final names = _categoryNamesFromPath(pathData);
          if (names.isNotEmpty) {
            categoryLv1 = names.length > 2 ? names[names.length - 3] : '';
            categoryLv2 = names.length > 1 ? names[names.length - 2] : '';
            categoryLv3 = names.last;
          }
        }
      }
      if (!mounted) return;
      final isFavorited = (() {
        final raw = info['isFavorited'];
        if (raw is bool) return raw;
        final normalized = raw?.toString().toLowerCase().trim() ?? '';
        return normalized == 'true' || normalized == '1';
      })();
      final isSelfOwned = (() {
        final raw = info['selfOwned'];
        if (raw is bool) return raw;
        final normalized = raw?.toString().toLowerCase().trim() ?? '';
        return normalized == 'true' || normalized == '1';
      })();
      final isRestockSubscribed = (() {
        final raw = info['isRestockSubscribed'];
        if (raw is bool) return raw;
        final normalized = raw?.toString().toLowerCase().trim() ?? '';
        return normalized == 'true' || normalized == '1';
      })();

      setState(() {
        if (imgs.isNotEmpty) {
          _images = imgs;
          _currentIndex = 0;
        }
        // B-132 三層分類（後端 /public/products/{id} 補欄位）
        _categoryLv1 = categoryLv1;
        _category = categoryLv2;
        _subCategory = categoryLv3;
        _origin = (info['origin'] ?? '').toString();
        _location = sellerLocation;
        if (store != null) {
          _storeCode = store.store.storeCode;
          _storeName = store.store.storeName.trim();
          _storeAvatarUrl = store.store.hasLogo()
              ? store.store.logo.uri(ImagePreset.avatar)
              : '';
          _storeRating = store.stats.hasRatingAverage()
              ? store.stats.ratingAverage
              : 0.0;
        }
        _shippingMethodCodes = shippingMethodCodes.toSet().toList(
          growable: false,
        );
        if (descText.isNotEmpty) _productDescription = descText;
        if (descBlocks.isNotEmpty) _productDescriptionBlocks = descBlocks;
        _grades = grades;
        _selectedGrade = initialGrade;
        _selectedGradeId = initialGrade == null
            ? null
            : (initialGrade['id'] ?? '').toString();
        _selectedQty = 1;
        _isFavorite = isFavorited;
        _selfOwned = isSelfOwned;
        _isRestockSubscribed = isRestockSubscribed;
        if (_resolvedMerchantId.isEmpty && sellerId.isNotEmpty) {
          _resolvedMerchantId = sellerId;
        }
        if (_resolvedMerchantName.isEmpty && sellerName.isNotEmpty) {
          _resolvedMerchantName = sellerName;
        }
      });

      // Analytics: view_item — fire once when product detail finishes loading
      if (!_viewItemSent) {
        _viewItemSent = true;
        final brandName = _resolvedMerchantName.isNotEmpty
            ? _resolvedMerchantName
            : sellerName.isNotEmpty
            ? sellerName
            : null;
        // Use first grade's effective price if available; fall back to widget.price
        final firstGradeRawPrice = grades.isNotEmpty
            ? (grades.first['salePrice'] ?? grades.first['price'])
            : null;
        final effectivePrice = firstGradeRawPrice != null
            ? PriceDisplay.parseMoney(firstGradeRawPrice)
            : PriceDisplay.parseMoney(widget.price);
        ref
            .read(analyticsEventsProvider)
            .trackViewItem(
              GA4Item(
                itemId: (widget.skuId ?? '').trim(),
                itemName: widget.title,
                price: effectivePrice,
                itemBrand: brandName,
              ),
            );
      }

      // autoFocusSpecs：滾動到規格選擇區
      if (widget.autoFocusSpecs && _grades != null && _grades!.isNotEmpty) {
        WidgetsBinding.instance.addPostFrameCallback((_) {
          _scrollToSpecSection();
        });
      }

      // E2E / ui-verify：直開商品時自動滾到底部（商品說明區）
      if (const String.fromEnvironment('E2E_INITIAL_PRODUCT').isNotEmpty) {
        WidgetsBinding.instance.addPostFrameCallback((_) {
          Future.delayed(const Duration(milliseconds: 900), () {
            if (_scrollController.hasClients) {
              _scrollController.animateTo(
                _scrollController.position.maxScrollExtent,
                duration: const Duration(milliseconds: 700),
                curve: Curves.easeOut,
              );
            }
          });
        });
      }
    } catch (_) {}
  }

  void _jumpTo(int i) {
    if (i == _currentIndex) return;
    _pageController.animateToPage(
      i,
      duration: const Duration(milliseconds: 250),
      curve: Curves.easeInOut,
    );
  }

  /// 滾動到規格選擇區
  void _scrollToSpecSection() {
    final ctx = _specSectionKey.currentContext;
    if (ctx == null || !_scrollController.hasClients) return;
    Scrollable.ensureVisible(
      ctx,
      duration: const Duration(milliseconds: 400),
      curve: Curves.easeInOut,
      alignment: 0.1, // 區段頂部稍偏下顯示
    );
  }

  Map<String, dynamic> get _detailResult => {
    'productId': widget.skuId,
    'isFavorite': _isFavorite,
    'isRestockSubscribed': _isRestockSubscribed,
  };

  void _closeDetail() => context.pop(_detailResult);

  @override
  void dispose() {
    _scrollController.dispose();
    _pageController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    // 計算當前選擇的單價和總價（PayUni #758：salePrice 有效時用 salePrice）
    double? currentUnitPrice;
    double? currentListPrice; // 原價（刪除線用），無特價時為 null
    if (_selectedGrade != null) {
      final rawPrice =
          _selectedGrade!['price'] ??
          _selectedGrade!['amount'] ??
          _selectedGrade!['value'];
      final rawSale = _selectedGrade!['salePrice'];
      final eff = PriceDisplay.effectiveSkuPrice(
        price: rawPrice,
        salePrice: rawSale,
      );
      currentUnitPrice = eff?.toDouble();
      if (PriceDisplay.hasSkuSale(price: rawPrice, salePrice: rawSale)) {
        currentListPrice = PriceDisplay.parseMoney(rawPrice)?.toDouble();
      }
    }
    final currentStock = purchaseSpecStock(_selectedGrade);
    final favoriteEnabled =
        !_favoriteLoading && (widget.skuId ?? '').trim().isNotEmpty;
    final detailRoute = ModalRoute.of(context);
    final supportsNativeBack = detailRoute is FloatingDetailBackGestureRoute;
    return PopScope(
      canPop: supportsNativeBack,
      onPopInvokedWithResult: (didPop, result) {
        if (didPop) {
          widget.onResultChanged?.call(_detailResult);
          return;
        }
        _closeDetail();
      },
      child: Scaffold(
        key: const ValueKey('product-detail-floating-shell'),
        backgroundColor: Colors.transparent,
        body: AnnotatedRegion<SystemUiOverlayStyle>(
          value: AppColors.glassTopOverlay,
          child: _ProductFloatingDetailSurface(
            onClose: _closeDetail,
            isFavorite: _isFavorite,
            favoriteEnabled: favoriteEnabled,
            onToggleFavorite: _toggleFavorite,
            onShare: _shareLoading ? null : _handleShare,
            purchaseDock: !_selfOwned && _isProductOutOfStock
                ? _RestockSubscribeButton(
                    subscribed: _isRestockSubscribed,
                    loading: _restockLoading,
                    onPressed: _toggleRestockSubscription,
                  )
                : ProductPurchaseFooter(
                    hasSelection: _selectedGrade != null,
                    stock: currentStock,
                    enabled: !_selfOwned,
                    addLoading: _addingToCart,
                    buyLoading: _directingCheckout,
                    integrated: true,
                    onAdd: () => _handleAddToCart(context),
                    onBuy: () => _handleDirectCheckout(context),
                  ),
            child: SingleChildScrollView(
              key: const ValueKey('product-detail-scroll'),
              controller: _scrollController,
              physics: const BouncingScrollPhysics(
                parent: AlwaysScrollableScrollPhysics(),
              ),
              child: Column(
                children: [
                  _ProductImageSection(
                    heroTag: widget.heroTag,
                    images: _images,
                    currentIndex: _currentIndex,
                    controller: _pageController,
                    onChanged: (i) => setState(() => _currentIndex = i),
                    onSelectImage: _jumpTo,
                  ),
                  const _ProductDetailSectionDivider(),
                  _MerchantInfoCardNew(
                    merchantName: _resolvedMerchantName.isNotEmpty
                        ? _resolvedMerchantName
                        : (widget.merchantName ?? ''),
                    storeCode: _storeCode,
                    storeName: _storeName,
                    avatarUrl: _storeAvatarUrl,
                    rating: _storeRating,
                  ),
                  const _ProductDetailSectionDivider(),
                  _ProductInfoCardNew(
                    productName: widget.title,
                    category: _category,
                    subCategory: _subCategory,
                    origin: _origin,
                    location: _location,
                    shippingMethodCodes: _shippingMethodCodes,
                  ),
                  if (_selfOwned) ...[
                    const SizedBox(height: HkSpace.sm),
                    const _SelfOwnedProtectionNotice(),
                  ],
                  const _ProductDetailSectionDivider(),
                  Padding(
                    key: _specSectionKey,
                    padding: const EdgeInsets.all(HkSpace.lg),
                    child: KeyedSubtree(
                      key: const ValueKey(
                        'product-detail-purchase-selector-card',
                      ),
                      child: ProductPurchaseSelector(
                        specs: _grades ?? const <Map<String, dynamic>>[],
                        selectedSpecId: _selectedGradeId,
                        quantity: _selectedQty,
                        currentPrice: currentUnitPrice,
                        currentListPrice: currentListPrice,
                        loading: _grades == null,
                        onSpecSelected: (grade) {
                          final stock = purchaseSpecStock(grade);
                          if (stock <= 0) return;
                          setState(() {
                            _selectedGrade = Map<String, dynamic>.from(grade);
                            _selectedGradeId = (grade['id'] ?? '').toString();
                            _selectedQty = 1;
                          });
                        },
                        onQuantityChanged: (quantity) {
                          final stock = purchaseSpecStock(_selectedGrade);
                          if (stock <= 0) return;
                          setState(
                            () =>
                                _selectedQty = quantity.clamp(1, stock).toInt(),
                          );
                        },
                      ),
                    ),
                  ),
                  const _ProductDetailSectionDivider(),
                  _ProductDescriptionCard(
                    description: _productDescription,
                    blocks: _productDescriptionBlocks,
                  ),
                  const _ProductDetailSectionDivider(),
                  _ProductReviewCardNew(
                    merchantId: _resolvedMerchantId.isNotEmpty
                        ? _resolvedMerchantId
                        : widget.merchantId,
                    skuId: widget.skuId,
                    productName: widget.title,
                  ),
                  const SizedBox(height: HkSpace.sm),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }

  Future<void> _handleDirectCheckout(BuildContext context) async {
    if (!await requireLogin(context, ref)) return;
    if (!context.mounted) return;
    if (_selfOwned) {
      _showSelfOwnedMessage(context);
      return;
    }
    final g = _selectedGrade;
    if (g == null) {
      showCommonDialog(context: context, title: '提醒', content: '請先選擇規格或等級。');
      return;
    }
    final skuStr = (_selectedGradeId?.trim().isNotEmpty == true)
        ? _selectedGradeId!.trim()
        : (widget.skuId ?? '').trim();
    int? sidInt =
        int.tryParse(skuStr) ??
        int.tryParse(RegExp(r'^[0-9]+').stringMatch(skuStr) ?? '');
    if (sidInt == null) {
      await showCommonDialog(
        context: context,
        title: '結帳失敗',
        content: '找不到商品，無法結帳',
        confirmText: '知道了',
      );
      return;
    }
    int qtyOf(Map<String, dynamic> m) {
      final qv = m['quantity'];
      return (qv is num) ? qv.toInt() : int.tryParse('$qv') ?? 0;
    }

    final int avail = qtyOf(g);
    if (avail <= 0) {
      showCommonDialog(context: context, title: '提醒', content: '商品已售完。');
      return;
    }
    if (_selectedQty <= 0) {
      showCommonSnackBar(context, '請選擇數量。');
      return;
    }

    setState(() => _directingCheckout = true);

    // Show the animated checkout toast immediately (no API wait)
    _checkoutToastEntry?.remove();
    _checkoutToastEntry = OverlayEntry(
      builder: (_) => const _CheckoutTransitionToast(),
    );
    Overlay.of(context).insert(_checkoutToastEntry!);

    try {
      final session = ref.read(sessionProvider).value;
      final guestId = hasUsableSession(session)
          ? null
          : await AuthStorage.guestCartId();
      await ref
          .read(cartRepositoryProvider)
          .addToCart(sidInt, _selectedQty, guestId: guestId);
      ref.invalidate(cartCountProvider);
    } catch (e) {
      HKLog.logE('[Cart] directCheckout addToCart error: $e');
      _checkoutToastEntry?.remove();
      _checkoutToastEntry = null;
      if (mounted) setState(() => _directingCheckout = false);
      if (context.mounted) {
        showCommonSnackBar(context, addToCartErrorMessage(e));
      }
      return;
    }

    _checkoutToastEntry?.remove();
    _checkoutToastEntry = null;
    if (mounted) setState(() => _directingCheckout = false);
    if (context.mounted) {
      context.push('/cart', extra: {'preSelectedSkuId': sidInt});
    }
  }

  Future<void> _handleAddToCart(BuildContext context) async {
    if (!await requireLogin(context, ref)) return;
    if (!context.mounted) return;
    if (_addingToCart) return;
    if (_selfOwned) {
      _showSelfOwnedMessage(context);
      return;
    }

    final g = _selectedGrade;
    if (g == null) {
      showCommonDialog(context: context, title: '提醒', content: '請先選擇規格或等級。');
      return;
    }
    int qtyOf(Map<String, dynamic> m) {
      final qv = m['quantity'];
      return (qv is num) ? qv.toInt() : int.tryParse('$qv') ?? 0;
    }

    final int avail = qtyOf(g);
    if (avail <= 0) {
      showCommonDialog(context: context, title: '提醒', content: '商品已售完。');
      return;
    }
    if (_selectedQty <= 0) {
      showCommonSnackBar(context, '請選擇數量。');
      return;
    }

    // Use the selected grade's SKU ID (variant ID), not the product ID from widget.skuId
    final skuStr = (_selectedGradeId?.trim().isNotEmpty == true)
        ? _selectedGradeId!.trim()
        : (widget.skuId ?? '').trim();
    int? sidInt =
        int.tryParse(skuStr) ??
        int.tryParse(RegExp(r'^[0-9]+').stringMatch(skuStr) ?? '');
    if (sidInt == null) {
      await showCommonDialog(
        context: context,
        title: '加入購物車失敗',
        content: '找不到商品，無法加入購物車',
        confirmText: '知道了',
      );
      return;
    }

    setState(() => _addingToCart = true);
    bool success = false;
    try {
      final session = ref.read(sessionProvider).value;
      final guestId = hasUsableSession(session)
          ? null
          : await AuthStorage.guestCartId();
      await ref
          .read(cartRepositoryProvider)
          .addToCart(sidInt, _selectedQty, guestId: guestId);
      ref.invalidate(cartCountProvider);
      success = true;
      // Analytics: add_to_cart
      final rawPrice = _selectedGrade?['salePrice'] ?? _selectedGrade?['price'];
      ref
          .read(analyticsEventsProvider)
          .trackAddToCart(
            GA4Item(
              itemId: skuStr,
              itemName: widget.title,
              price: PriceDisplay.parseMoney(rawPrice),
              quantity: _selectedQty,
              itemBrand: _resolvedMerchantName.isNotEmpty
                  ? _resolvedMerchantName
                  : null,
            ),
          );
    } catch (e) {
      HKLog.logE('[Cart] addToCart error: $e');
      success = false;
      if (context.mounted) {
        showCommonSnackBar(context, addToCartErrorMessage(e));
      }
    } finally {
      if (mounted) setState(() => _addingToCart = false);
    }
    if (!context.mounted) return;
    if (success) {
      showFloatingToast(context, '成功加入購物車！', isSuccess: true);
    }
  }

  void _showSelfOwnedMessage(BuildContext context) {
    showCommonSnackBar(context, cannotAddOwnProductToCartMessage);
  }
}

// ---------------------------------------------------------------------------
// Checkout transition toast (animated cart icon overlay)
// ---------------------------------------------------------------------------
class _CheckoutTransitionToast extends StatefulWidget {
  const _CheckoutTransitionToast();

  @override
  State<_CheckoutTransitionToast> createState() =>
      _CheckoutTransitionToastState();
}

class _CheckoutTransitionToastState extends State<_CheckoutTransitionToast>
    with TickerProviderStateMixin {
  late final AnimationController _enterCtrl;
  late final AnimationController _bounceCtrl;
  late final Animation<double> _enterAnim;

  @override
  void initState() {
    super.initState();
    // One-shot entrance: fade + slide up in 280ms
    _enterCtrl = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 280),
    )..forward();
    _enterAnim = CurvedAnimation(parent: _enterCtrl, curve: Curves.easeOut);

    // Continuous sine bounce for icon
    _bounceCtrl = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 1200),
    )..repeat();
  }

  @override
  void dispose() {
    _enterCtrl.dispose();
    _bounceCtrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Positioned(
      bottom: 120,
      left: 0,
      right: 0,
      child: Center(
        child: FadeTransition(
          opacity: _enterAnim,
          child: SlideTransition(
            position: Tween<Offset>(
              begin: const Offset(0, 0.5),
              end: Offset.zero,
            ).animate(_enterAnim),
            child: Material(
              color: Colors.transparent,
              child: Container(
                width: 200.w,
                padding: EdgeInsets.symmetric(horizontal: 24.w, vertical: 24.h),
                decoration: BoxDecoration(
                  color: AppColors.bgCard,
                  borderRadius: BorderRadius.circular(24.r),
                  boxShadow: [
                    BoxShadow(
                      color: AppColors.cardShadow,
                      blurRadius: 32,
                      offset: const Offset(0, 8),
                    ),
                  ],
                ),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    _BouncingCartIcon(controller: _bounceCtrl),
                    SizedBox(height: 14.h),
                    Text(
                      '來去結帳',
                      style: TextStyle(
                        fontSize: 16,
                        fontWeight: FontWeight.w500,
                        color: AppColors.textSecondary,
                        letterSpacing: 0.5,
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

class _BouncingCartIcon extends StatelessWidget {
  const _BouncingCartIcon({required this.controller});
  final AnimationController controller;

  @override
  Widget build(BuildContext context) {
    return AnimatedBuilder(
      animation: controller,
      builder: (_, child) {
        final dy = -5.0 * math.sin(controller.value * 2 * math.pi);
        return Transform.translate(offset: Offset(0, dy), child: child);
      },
      child: Icon(
        AppIcons.shoppingCartOutlined,
        size: 64.sp,
        color: AppIconColors.muted,
      ),
    );
  }
}

class _ProductFloatingDetailSurface extends StatelessWidget {
  const _ProductFloatingDetailSurface({
    required this.child,
    required this.purchaseDock,
    required this.onClose,
    required this.isFavorite,
    required this.favoriteEnabled,
    required this.onToggleFavorite,
    this.onShare,
  });

  final Widget child;
  final Widget purchaseDock;
  final VoidCallback onClose;
  final bool isFavorite;
  final bool favoriteEnabled;
  final VoidCallback onToggleFavorite;
  final VoidCallback? onShare;

  @override
  Widget build(BuildContext context) {
    final mediaPadding = MediaQuery.paddingOf(context);

    return Stack(
      children: [
        const Positioned.fill(
          child: RepaintBoundary(
            key: ValueKey('product-detail-backdrop'),
            child: ColoredBox(color: Colors.transparent),
          ),
        ),
        Positioned.fill(
          child: Padding(
            padding: EdgeInsets.fromLTRB(
              ProductDetailPreviewLayout.previewHorizontalInset,
              mediaPadding.top + ProductDetailPreviewLayout.previewTopInset,
              ProductDetailPreviewLayout.previewHorizontalInset,
              mediaPadding.bottom +
                  ProductDetailPreviewLayout.purchaseBottomInset,
            ),
            child: DecoratedBox(
              decoration: BoxDecoration(
                borderRadius: BorderRadius.circular(HkRadius.xxl),
                boxShadow: HkShadows.modal,
              ),
              child: Material(
                key: const ValueKey('product-detail-preview-window'),
                color: AppColors.bgCard,
                clipBehavior: Clip.antiAlias,
                shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(HkRadius.xxl),
                  side: const BorderSide(color: AppColors.cardBorder),
                ),
                child: Column(
                  children: [
                    Expanded(
                      child: Stack(
                        children: [
                          Positioned.fill(child: child),
                          Positioned(
                            key: const ValueKey(
                              'product-detail-overlay-controls',
                            ),
                            top: HkSpace.md,
                            left: HkSpace.md,
                            right: HkSpace.md,
                            child: Row(
                              children: [
                                _ProductOverlayIconButton(
                                  key: const ValueKey('product-detail-close'),
                                  semanticLabel: '關閉商品詳情',
                                  onTap: onClose,
                                  child: const Icon(
                                    AppIcons.close,
                                    size: 30,
                                    color: AppIconColors.strong,
                                  ),
                                ),
                                const Spacer(),
                                _ProductOverlayIconButton(
                                  key: const ValueKey(
                                    'product-detail-favorite',
                                  ),
                                  semanticLabel: isFavorite ? '取消收藏' : '加入收藏',
                                  onTap: favoriteEnabled
                                      ? onToggleFavorite
                                      : null,
                                  child: Icon(
                                    isFavorite
                                        ? AppIcons.favorite
                                        : AppIcons.favoriteBorder,
                                    size: HkControl.icon,
                                    color: isFavorite
                                        ? AppIconColors.favorite
                                        : AppIconColors.strong,
                                  ),
                                ),
                                const SizedBox(width: HkSpace.sm),
                                _ProductOverlayIconButton(
                                  key: const ValueKey('product-detail-share'),
                                  semanticLabel: '分享商品',
                                  onTap: onShare,
                                  child: const Icon(
                                    AppIcons.shareOutlined,
                                    size: HkControl.icon,
                                    color: AppIconColors.strong,
                                  ),
                                ),
                              ],
                            ),
                          ),
                        ],
                      ),
                    ),
                    DecoratedBox(
                      decoration: const BoxDecoration(
                        border: Border(
                          top: BorderSide(color: AppColors.dividerLight),
                        ),
                      ),
                      child: KeyedSubtree(
                        key: const ValueKey('product-detail-purchase-dock'),
                        child: purchaseDock,
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ),
        ),
      ],
    );
  }
}

class _ProductOverlayIconButton extends StatelessWidget {
  const _ProductOverlayIconButton({
    super.key,
    required this.semanticLabel,
    required this.onTap,
    required this.child,
  });

  final String semanticLabel;
  final VoidCallback? onTap;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    return HkLiquidGlassIconButton(
      semanticLabel: semanticLabel,
      onPressed: onTap,
      child: child,
    );
  }
}

class _ProductSourceImage extends StatelessWidget {
  const _ProductSourceImage({required this.source, required this.fit});

  final String source;
  final BoxFit fit;

  @override
  Widget build(BuildContext context) {
    if (source.startsWith('assets/')) {
      return Image.asset(source, fit: fit);
    }
    return AppCachedImage(
      imageUrl: source,
      fit: fit,
      scene: ImageScene.productDetail,
      loadingStyle: AppImageLoadingStyle.quiet,
    );
  }
}

// ============================================================================
// 商品內容元件
// ============================================================================

/// 商品預覽媒體：置中的原圖疊在同圖模糊背景上，維持不同商品比例時的沉浸感。
class _ProductImageSection extends StatelessWidget {
  const _ProductImageSection({
    required this.heroTag,
    required this.images,
    required this.currentIndex,
    required this.controller,
    required this.onChanged,
    required this.onSelectImage,
  });

  final Object? heroTag;
  final List<String> images;
  final int currentIndex;
  final PageController controller;
  final ValueChanged<int> onChanged;
  final ValueChanged<int> onSelectImage;

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(
      builder: (context, constraints) {
        final viewportHeight = (constraints.maxWidth * 0.88).clamp(
          280.0,
          420.0,
        );
        return Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            ProductMediaHero(
              tag: heroTag,
              child: SizedBox(
                key: const ValueKey('product-image-viewport'),
                width: double.infinity,
                height: viewportHeight,
                child: Stack(
                  fit: StackFit.expand,
                  children: [
                    if (images.isEmpty)
                      const ColoredBox(
                        color: AppColors.storefrontMutedSurface,
                        child: Center(
                          child: Icon(
                            AppIcons.imageOutlined,
                            color: AppIconColors.disabled,
                            size: 56,
                          ),
                        ),
                      )
                    else
                      PageView.builder(
                        controller: controller,
                        onPageChanged: onChanged,
                        itemCount: images.length,
                        itemBuilder: (_, i) =>
                            _ImmersiveProductMedia(source: images[i]),
                      ),
                    Positioned(
                      left: 0,
                      right: 0,
                      top: 0,
                      height: 150,
                      child: IgnorePointer(
                        child: DecoratedBox(
                          decoration: BoxDecoration(
                            gradient: LinearGradient(
                              begin: Alignment.topCenter,
                              end: Alignment.bottomCenter,
                              colors: [
                                AppColors.neutralCream.withValues(alpha: 0.70),
                                AppColors.neutralCream.withValues(alpha: 0),
                              ],
                            ),
                          ),
                        ),
                      ),
                    ),
                  ],
                ),
              ),
            ),
            if (images.length > 1) ...[
              const SizedBox(height: HkSpace.md),
              Padding(
                padding: const EdgeInsets.symmetric(horizontal: HkSpace.md),
                child: ProductImageThumbnailGrid(
                  images: images,
                  currentIndex: currentIndex,
                  onSelected: onSelectImage,
                ),
              ),
              const SizedBox(height: HkSpace.sm),
            ],
          ],
        );
      },
    );
  }
}

class _ImmersiveProductMedia extends StatelessWidget {
  const _ImmersiveProductMedia({required this.source});

  final String source;

  @override
  Widget build(BuildContext context) {
    if (source.isEmpty) {
      return const ColoredBox(
        color: AppColors.storefrontMutedSurface,
        child: Center(
          child: Icon(
            AppIcons.imageOutlined,
            color: AppIconColors.disabled,
            size: 56,
          ),
        ),
      );
    }
    return RepaintBoundary(
      child: Stack(
        fit: StackFit.expand,
        children: [
          ImageFiltered(
            imageFilter: ui.ImageFilter.blur(sigmaX: 22, sigmaY: 22),
            child: Transform.scale(
              scale: 1.14,
              child: _ProductSourceImage(source: source, fit: BoxFit.cover),
            ),
          ),
          Positioned.fill(
            child: ColoredBox(
              color: AppColors.neutralWhite.withValues(alpha: 0.38),
            ),
          ),
          _ProductSourceImage(source: source, fit: BoxFit.contain),
        ],
      ),
    );
  }
}

/// The product detail is one card; section rhythm comes from whitespace and a
/// quiet inset rule instead of stacking more cards and shadows inside it.
class _ProductDetailSectionDivider extends StatelessWidget {
  const _ProductDetailSectionDivider();

  @override
  Widget build(BuildContext context) {
    return const Padding(
      padding: EdgeInsets.symmetric(horizontal: HkSpace.lg),
      child: Divider(height: 1, thickness: 1, color: AppColors.dividerLight),
    );
  }
}

/// 商品資訊區段。
class _ProductInfoCardNew extends StatelessWidget {
  const _ProductInfoCardNew({
    required this.productName,
    required this.category,
    required this.subCategory,
    required this.origin,
    required this.location,
    required this.shippingMethodCodes,
  });

  final String productName;
  final String category;
  final String subCategory;
  final String origin;
  final String location;
  final List<String> shippingMethodCodes;

  @override
  Widget build(BuildContext context) {
    // 顯示真實資料，無資料時顯示 '--'
    // 「商品分類」改成「L2 - L3」單行格式（不再分商品分類 / 詳細分類兩欄）
    final String categoryDisplay;
    if (category.isNotEmpty && subCategory.isNotEmpty) {
      categoryDisplay = '$category - $subCategory';
    } else if (category.isNotEmpty) {
      categoryDisplay = category;
    } else if (subCategory.isNotEmpty) {
      categoryDisplay = subCategory;
    } else {
      categoryDisplay = '--';
    }
    final originDisplay = origin.isNotEmpty ? origin : '--';
    final locationDisplay = location.isNotEmpty ? location : '--';

    return Padding(
      key: const ValueKey('product-detail-info-section'),
      padding: EdgeInsets.all(16.w),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(productName, style: AppTypography.sectionTitle),
          SizedBox(height: 8.h),
          // 商品分類：L2 - L3 合併單行顯示（移除原本的「詳細分類」獨立欄位）
          Text(
            '商品分類：$categoryDisplay',
            style: AppTypography.body,
            overflow: TextOverflow.ellipsis,
          ),
          SizedBox(height: 4.h),
          Text('產地：$originDisplay', style: AppTypography.body),
          SizedBox(height: 8.h),
          Row(
            children: [
              Icon(AppIcons.location, size: 20.sp),
              SizedBox(width: 5.w),
              SizedBox(
                width: 212.w,
                child: Text(locationDisplay, style: AppTypography.body),
              ),
            ],
          ),
          SizedBox(height: 8.h),
          ProductDeliveryMetadata(shippingMethodCodes: shippingMethodCodes),
        ],
      ),
    );
  }
}

class _SelfOwnedProtectionNotice extends StatelessWidget {
  const _SelfOwnedProtectionNotice();

  @override
  Widget build(BuildContext context) {
    return Container(
      width: double.infinity,
      margin: EdgeInsets.symmetric(horizontal: 16.w),
      padding: EdgeInsets.symmetric(horizontal: 16.w, vertical: 12.h),
      decoration: BoxDecoration(
        color: AppColors.warningSoft,
        borderRadius: BorderRadius.circular(12.r),
        border: Border.all(color: const Color(0x33E0C44C)),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(
            AppIcons.infoOutline,
            size: 18.sp,
            color: AppIconColors.warningText,
          ),
          SizedBox(width: 8.w),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  '自買防護已啟用',
                  style: AppTypography.bodyBold.copyWith(
                    color: AppColors.warningText,
                  ),
                ),
                SizedBox(height: 2.h),
                Text(
                  '這是您上架的商品，無法加入購物車或直接購買。',
                  style: AppTypography.metaMuted.copyWith(
                    color: AppColors.warningText,
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

/// 商品說明區段。
class _ProductDescriptionCard extends StatelessWidget {
  const _ProductDescriptionCard({
    required this.description,
    this.blocks = const [],
  });
  final String description;
  final List<Map<String, dynamic>> blocks;

  List<Widget> _buildContent() {
    if (blocks.isEmpty) {
      return [
        Text(
          description.isNotEmpty ? description : '--',
          style: AppTypography.body,
        ),
      ];
    }

    final widgets = <Widget>[];
    for (final block in blocks) {
      final type = block['type']?.toString() ?? 'text';
      if (type == 'image') {
        // 49 §6.1 成對契約：描述區塊的圖片欄已由 objectKey/url 收斂成
        // imageKey/imageUrl。OssImage 兩種輸入都吃，key 優先。
        final url = (block['imageKey'] ?? block['imageUrl'] ?? '').toString();
        if (url.isNotEmpty) {
          widgets.add(
            Padding(
              padding: EdgeInsets.symmetric(vertical: 4.h),
              child: ClipRRect(
                borderRadius: BorderRadius.circular(8.r),
                child: OssImage(
                  imageSource: url,
                  scene: ImageScene.productDetail,
                  fit: BoxFit.contain,
                ),
              ),
            ),
          );
        }
        continue;
      }

      // 表格 block（02 §3.6）：橫向捲動渲染
      if (type == 'table') {
        final tableWidget = _buildDescTable(block['tableJson']);
        if (tableWidget != null) {
          widgets.add(
            Padding(
              padding: EdgeInsets.symmetric(vertical: 4.h),
              child: tableWidget,
            ),
          );
        }
        continue;
      }

      // 清單 block（02 §3.6）：有序帶序號 / 無序帶項目符號
      if (type == 'list') {
        final listWidget = _buildDescList(block['listJson']);
        if (listWidget != null) {
          widgets.add(
            Padding(
              padding: EdgeInsets.symmetric(vertical: 4.h),
              child: listWidget,
            ),
          );
        }
        continue;
      }

      // 文字 block：runs 為主（inline 粗體/斜體/底線/顏色/連結），legacy content 相容
      final styleMap = block['style'] as Map<String, dynamic>? ?? {};
      final blockFontSize = _parseFontSize(styleMap['fontSize']);

      FontWeight blockFontWeight = FontWeight.w400;
      if (styleMap['fontWeight'] is num) {
        final wVal = (styleMap['fontWeight'] as num).toInt();
        final wIndex = (wVal ~/ 100) - 1;
        if (wIndex >= 0 && wIndex < FontWeight.values.length) {
          blockFontWeight = FontWeight.values[wIndex];
        }
      }

      TextAlign textAlign = TextAlign.left;
      if (styleMap['alignment'] is String) {
        switch (styleMap['alignment'] as String) {
          case 'center':
            textAlign = TextAlign.center;
            break;
          case 'right':
            textAlign = TextAlign.right;
            break;
        }
      }

      const defaultColor = AppColors.textPrimary;
      final baseStyle = TextStyle(
        fontSize: blockFontSize,
        fontWeight: blockFontWeight,
        color: defaultColor,
        height: 1.5,
      );

      final runs = block['runs'];
      if (runs is List && runs.isNotEmpty) {
        widgets.add(
          SizedBox(
            width: double.infinity,
            child: RichText(
              textAlign: textAlign,
              text: TextSpan(children: _runSpans(runs, baseStyle)),
            ),
          ),
        );
        continue;
      }

      // legacy content（backfill 前舊資料，整塊樣式）
      final content = (block['content'] ?? '').toString();
      if (content.isEmpty) continue;
      final isBold = styleMap['bold'] == true;
      final isItalic = styleMap['italic'] == true;
      Color legacyColor = defaultColor;
      if (styleMap['color'] is String) {
        final c = _parseHexColor(styleMap['color'] as String);
        if (c != null) legacyColor = c;
      }
      widgets.add(
        SizedBox(
          width: double.infinity,
          child: Text(
            content,
            textAlign: textAlign,
            style: baseStyle.copyWith(
              fontWeight: isBold ? FontWeight.bold : blockFontWeight,
              fontStyle: isItalic ? FontStyle.italic : FontStyle.normal,
              color: legacyColor,
            ),
          ),
        ),
      );
    }

    return widgets.isEmpty ? [Text('--', style: AppTypography.body)] : widgets;
  }

  /// fontSize 可能為數字或 px 字串（web 存 "26px"）→ 統一解析為 double。
  double _parseFontSize(dynamic v) {
    double raw = 14;
    if (v is num) raw = v.toDouble();
    if (v is String) {
      final m = RegExp(r'(\d+(?:\.\d+)?)').firstMatch(v);
      if (m != null) raw = double.tryParse(m.group(1)!) ?? 14;
    }
    if (raw <= 12) return 12;
    if (raw <= 14) return 14;
    if (raw <= 16) return 16;
    if (raw <= 18) return 18;
    if (raw <= 20) return 20;
    return 24;
  }

  Color? _parseHexColor(String raw) {
    final hex = raw.replaceFirst('#', '');
    if (hex.length == 6 || hex.length == 8) {
      final argb = hex.length == 6 ? 'FF$hex' : hex;
      final v = int.tryParse(argb, radix: 16);
      if (v != null) return Color(v);
    }
    return null;
  }

  /// inline runs（02 §3.6）→ TextSpan 清單：逐 run 套粗體/斜體/底線/顏色/連結。
  List<InlineSpan> _runSpans(List<dynamic> runs, TextStyle base) {
    final spans = <InlineSpan>[];
    for (final r in runs) {
      if (r is! Map) continue;
      final text = r['text']?.toString() ?? '';
      if (text.isEmpty) continue;
      var style = base;
      if (r['bold'] == true) {
        style = style.copyWith(fontWeight: FontWeight.bold);
      }
      if (r['italic'] == true) {
        style = style.copyWith(fontStyle: FontStyle.italic);
      }
      final color = r['color'] is String
          ? _parseHexColor(r['color'] as String)
          : null;
      if (color != null) style = style.copyWith(color: color);

      final href = r['href'] is String ? r['href'] as String : null;
      final isLink =
          href != null &&
          (href.startsWith('http://') || href.startsWith('https://'));
      final decorations = <TextDecoration>[
        if (r['underline'] == true) TextDecoration.underline,
        if (isLink) TextDecoration.underline,
      ];
      if (isLink) {
        style = style.copyWith(
          color: AppColors.success,
          decoration: TextDecoration.combine(decorations),
        );
        spans.add(
          TextSpan(
            text: text,
            style: style,
            recognizer: TapGestureRecognizer()
              ..onTap = () {
                launchUrl(
                  Uri.parse(href),
                  mode: LaunchMode.externalApplication,
                );
              },
          ),
        );
      } else {
        if (decorations.isNotEmpty) {
          style = style.copyWith(
            decoration: TextDecoration.combine(decorations),
          );
        }
        spans.add(TextSpan(text: text, style: style));
      }
    }
    return spans;
  }

  /// 表格 block（tableJson）→ 橫向捲動 Table（使用者核准橫向捲動）。
  Widget? _buildDescTable(dynamic tableJson) {
    if (tableJson is! String || tableJson.isEmpty) return null;
    Map<String, dynamic> model;
    try {
      model = jsonDecode(tableJson) as Map<String, dynamic>;
    } catch (_) {
      return null;
    }
    final rows = model['rows'];
    if (rows is! List || rows.isEmpty) return null;
    final withHeadings = model['withHeadings'] == true;

    final baseStyle = TextStyle(
      fontSize: 14,
      color: AppColors.textPrimary,
      height: 1.4,
    );
    // 各列欄數對齊（Table 要求所有列同欄數）
    var maxCols = 0;
    for (final row in rows) {
      if (row is List && row.length > maxCols) maxCols = row.length;
    }
    if (maxCols == 0) return null;

    final tableRows = <TableRow>[];
    for (var i = 0; i < rows.length; i++) {
      final row = rows[i];
      if (row is! List) continue;
      final isHead = withHeadings && i == 0;
      final cellStyle = isHead
          ? baseStyle.copyWith(
              fontWeight: FontWeight.bold,
              color: AppColors.textEmphasis,
            )
          : baseStyle;
      final cells = <Widget>[];
      for (var c = 0; c < maxCols; c++) {
        final cell = c < row.length ? row[c] : null;
        final cellRuns = (cell is Map && cell['runs'] is List)
            ? List<dynamic>.from(cell['runs'] as List)
            : const <dynamic>[];
        cells.add(
          Padding(
            padding: EdgeInsets.symmetric(horizontal: 8.w, vertical: 8.h),
            child: RichText(
              text: TextSpan(children: _runSpans(cellRuns, cellStyle)),
            ),
          ),
        );
      }
      tableRows.add(
        TableRow(
          decoration: isHead
              ? const BoxDecoration(color: AppColors.gray100)
              : null,
          children: cells,
        ),
      );
    }
    if (tableRows.isEmpty) return null;

    return SingleChildScrollView(
      scrollDirection: Axis.horizontal,
      child: Table(
        border: TableBorder.all(color: AppColors.dividerLight, width: 1),
        defaultColumnWidth: const IntrinsicColumnWidth(),
        children: tableRows,
      ),
    );
  }

  /// 清單 block（listJson `{ordered, items:[{runs}]}`）→ 有序帶序號 / 無序帶項目符號。
  /// 每個 item 的 runs 複用 _runSpans 渲染（同 table cell）。
  Widget? _buildDescList(dynamic listJson) {
    if (listJson is! String || listJson.isEmpty) return null;
    Map<String, dynamic> model;
    try {
      model = jsonDecode(listJson) as Map<String, dynamic>;
    } catch (_) {
      return null;
    }
    final items = model['items'];
    if (items is! List || items.isEmpty) return null;
    final ordered = model['ordered'] == true;

    final baseStyle = TextStyle(
      fontSize: 14,
      color: AppColors.textPrimary,
      height: 1.5,
    );

    final rows = <Widget>[];
    for (var i = 0; i < items.length; i++) {
      final item = items[i];
      final itemRuns = (item is Map && item['runs'] is List)
          ? List<dynamic>.from(item['runs'] as List)
          : const <dynamic>[];
      final marker = ordered ? '${i + 1}.' : '•';
      rows.add(
        Padding(
          padding: EdgeInsets.symmetric(vertical: 4.h),
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.baseline,
            textBaseline: TextBaseline.alphabetic,
            children: [
              SizedBox(
                width: ordered ? 22.w : 16.w,
                child: Text(
                  marker,
                  textAlign: ordered ? TextAlign.right : TextAlign.left,
                  style: baseStyle,
                ),
              ),
              SizedBox(width: 6.w),
              Expanded(
                child: RichText(
                  text: TextSpan(children: _runSpans(itemRuns, baseStyle)),
                ),
              ),
            ],
          ),
        ),
      );
    }
    if (rows.isEmpty) return null;
    return Column(crossAxisAlignment: CrossAxisAlignment.start, children: rows);
  }

  @override
  Widget build(BuildContext context) {
    return Padding(
      key: const ValueKey('product-detail-description-section'),
      padding: EdgeInsets.all(16.w),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            '商品說明',
            style: TextStyle(
              fontSize: 16,
              fontWeight: FontWeight.bold,
              color: AppColors.textEmphasis,
            ),
          ),
          SizedBox(height: 8.h),
          const Divider(color: AppColors.gray100, height: 1),
          SizedBox(height: 8.h),
          ..._buildContent(),
        ],
      ),
    );
  }
}

/// 商家資訊區段。店家資料來自 GetStore；[storeCode] 空字串表示讀不到店面，不顯示進店連結。
class _MerchantInfoCardNew extends StatelessWidget {
  const _MerchantInfoCardNew({
    required this.merchantName,
    this.storeCode = '',
    this.storeName = '',
    this.avatarUrl = '',
    this.rating = 0.0,
  });

  final String merchantName;
  final String storeCode;
  final String storeName;
  final String avatarUrl;
  final double rating;

  @override
  Widget build(BuildContext context) {
    final avatarRaw = avatarUrl;

    void enterStore() {
      if (storeCode.isEmpty) return;
      context.push('/merchant/$storeCode');
    }

    // Figma: 40x40 圓形頭像
    Widget avatar = Container(
      width: 40.w,
      height: 40.h,
      decoration: const BoxDecoration(shape: BoxShape.circle),
      clipBehavior: Clip.antiAlias,
      child: SizedBox(
        width: 40.w,
        height: 40.h,
        child: HkPlaceholder(HkPlaceholderKind.merchantAvatar),
      ),
    );

    if (avatarRaw.isNotEmpty) {
      // inline 判準只看 `data:` scheme（media_image.dart isInlineImageData）。
      // 舊寫法「不是 http 且長度 > 60」會把 78~80 字元的 objectKey 誤判成 base64。
      final bool looksBase64 = isInlineImageData(avatarRaw);
      if (looksBase64) {
        try {
          final b64 = avatarRaw.contains(',')
              ? avatarRaw.split(',').last
              : avatarRaw;
          final bytes = base64Decode(b64);
          avatar = Container(
            width: 40.w,
            height: 40.h,
            decoration: const BoxDecoration(shape: BoxShape.circle),
            clipBehavior: Clip.antiAlias,
            child: Image.memory(
              bytes,
              width: 40.w,
              height: 40.h,
              fit: BoxFit.cover,
              gaplessPlayback: true,
            ),
          );
        } catch (_) {}
      } else {
        avatar = Container(
          width: 40.w,
          height: 40.h,
          decoration: const BoxDecoration(shape: BoxShape.circle),
          clipBehavior: Clip.antiAlias,
          child: AppCachedImage(
            imageUrl: avatarRaw,
            width: 40.w,
            height: 40.h,
            fit: BoxFit.cover,
            scene: ImageScene.merchantAvatar,
          ),
        );
      }
    }

    final merchantIdentity = Row(
      children: [
        avatar,
        SizedBox(width: 7.w),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            mainAxisSize: MainAxisSize.min,
            children: [
              Text(
                storeName.isNotEmpty
                    ? storeName
                    : (merchantName.isNotEmpty ? merchantName : '商家'),
                style: AppTypography.sectionTitle,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
              ),
              Row(
                children: [
                  Icon(AppIcons.star, size: 12.sp, color: AppIconColors.gold),
                  SizedBox(width: 4.w),
                  Flexible(
                    child: Text(
                      rating > 0 ? rating.toStringAsFixed(1) : '尚無評分',
                      style: AppTypography.body,
                      overflow: TextOverflow.ellipsis,
                    ),
                  ),
                ],
              ),
            ],
          ),
        ),
      ],
    );
    final storeButton = MouseRegion(
      cursor: storeCode.isEmpty
          ? SystemMouseCursors.basic
          : SystemMouseCursors.click,
      child: HkTapTarget(
        semanticLabel: '進入店舖',
        onTap: storeCode.isEmpty ? null : enterStore,
        child: Container(
          key: const ValueKey('product-enter-store-button'),
          height: 40,
          alignment: Alignment.center,
          padding: const EdgeInsets.symmetric(horizontal: HkSpace.md),
          decoration: BoxDecoration(
            color: AppColors.actionPrimary,
            borderRadius: BorderRadius.circular(HkRadius.md),
            boxShadow: HkShadows.sm,
          ),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              HkIcon(
                HkIconName.merchantShop,
                key: const ValueKey('product-enter-store-icon'),
                size: 20,
                color: AppIconColors.onBrand,
              ),
              const SizedBox(width: HkSpace.sm),
              Text(
                '進入店舖',
                textAlign: TextAlign.center,
                style: AppTypography.buttonMedium.copyWith(
                  color: AppColors.textInverse,
                  height: 1,
                ),
              ),
            ],
          ),
        ),
      ),
    );

    return Padding(
      key: const ValueKey('product-detail-merchant-section'),
      padding: EdgeInsets.symmetric(horizontal: 20.w, vertical: 16.h),
      child: LayoutBuilder(
        builder: (context, constraints) {
          if (constraints.maxWidth < 300) {
            return Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                merchantIdentity,
                const SizedBox(height: HkSpace.md),
                storeButton,
              ],
            );
          }
          return Row(
            children: [
              Expanded(child: merchantIdentity),
              const SizedBox(width: HkSpace.md),
              storeButton,
            ],
          );
        },
      ),
    );
  }
}

/// 商品評論卡片
class _ProductReviewCardNew extends ConsumerStatefulWidget {
  const _ProductReviewCardNew({this.merchantId, this.skuId, this.productName});
  final String? merchantId;
  final String? skuId;
  final String? productName;

  @override
  ConsumerState<_ProductReviewCardNew> createState() =>
      _ProductReviewCardNewState();
}

class _ProductReviewCardNewState extends ConsumerState<_ProductReviewCardNew> {
  List<Map<String, dynamic>>? _reviewsList;
  bool _isLoading = true;

  @override
  void initState() {
    super.initState();
    _loadReviews();
  }

  @override
  void didUpdateWidget(covariant _ProductReviewCardNew oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.merchantId != widget.merchantId ||
        oldWidget.skuId != widget.skuId) {
      _loadReviews();
    }
  }

  Future<void> _loadReviews() async {
    final mid = (widget.merchantId ?? '').trim();
    final sid = (widget.skuId ?? '').trim();
    HKLog.logD('[ProductReviewCard] _loadReviews called: mid=$mid, sid=$sid');
    if (mid.isEmpty || sid.isEmpty) {
      HKLog.logW('[ProductReviewCard] mid or sid is empty, skipping');
      if (mounted) {
        setState(() {
          _reviewsList = null;
          _isLoading = false;
        });
      }
      return;
    }
    try {
      final list = await ref
          .read(reviewRepositoryProvider)
          .getMerchantProductReviews(sid);
      HKLog.logD('[ProductReviewCard] API returned ${list.length} reviews');
      if (list.isNotEmpty) {
        HKLog.logD(
          '[ProductReviewCard] First review keys: ${list.first.keys.toList()}',
        );
      }

      // 過濾只顯示該商品的評論
      final filteredList = list.where((review) {
        // ReviewListItem 每則評論帶 productId（#1155）。
        final reviewSkuId = (review['productId'] ?? '').toString().trim();
        if (reviewSkuId.isEmpty) return true;
        return reviewSkuId == sid;
      }).toList();
      HKLog.logD(
        '[ProductReviewCard] After filtering: ${filteredList.length} reviews for skuId=$sid',
      );

      if (mounted) {
        setState(() {
          _reviewsList = filteredList;
          _isLoading = false;
        });
      }
    } catch (e) {
      HKLog.logE('[ProductReviewCard] Error loading reviews: $e');
      if (mounted) {
        setState(() {
          _reviewsList = null;
          _isLoading = false;
        });
      }
    }
  }

  String _formatDate(dynamic dateValue) {
    if (dateValue == null) return '';
    try {
      DateTime date;
      if (dateValue is String) {
        date = DateTime.parse(dateValue);
      } else {
        return '';
      }
      return '${date.year}-${date.month.toString().padLeft(2, '0')}-${date.day.toString().padLeft(2, '0')}';
    } catch (_) {
      return '';
    }
  }

  @override
  Widget build(BuildContext context) {
    final mid = (widget.merchantId ?? '').trim();
    final sid = (widget.skuId ?? '').trim();
    HKLog.logD(
      '[ProductReviewCard] build: mid=$mid, sid=$sid, isLoading=$_isLoading, reviewsCount=${_reviewsList?.length ?? 0}',
    );

    if (mid.isEmpty || sid.isEmpty) {
      HKLog.logD('[ProductReviewCard] Hiding: mid or sid empty');
      return const SizedBox.shrink(
        key: ValueKey('product-detail-review-section'),
      );
    }

    if (_isLoading) {
      return Padding(
        key: const ValueKey('product-detail-review-section'),
        padding: EdgeInsets.fromLTRB(16.w, 16.h, 16.w, 20.h),
        child: Center(
          child: SizedBox(
            width: 20,
            height: 20.h,
            child: const HkBrandLoader(
              tone: HkBrandLoaderTone.mono,
              color: AppColors.primary,
            ),
          ),
        ),
      );
    }

    final reviews = _reviewsList ?? [];
    final reviewCount = reviews.length;

    // 只顯示前2筆評論預覽
    final displayReviews = reviews.take(2).toList();

    return Padding(
      key: const ValueKey('product-detail-review-section'),
      padding: EdgeInsets.all(16.w),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          // 標題列：商品評價 (數量) + 查看全部
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Expanded(
                child: Text(
                  '商品評價 ($reviewCount)',
                  style: const TextStyle(
                    fontSize: 16,
                    fontWeight: FontWeight.bold,
                    color: AppColors.textEmphasis,
                  ),
                ),
              ),
              const SizedBox(width: HkSpace.sm),
              GestureDetector(
                onTap: () {
                  context.push(
                    '/review/product',
                    extra: {
                      'merchantId': widget.merchantId ?? '',
                      'skuId': widget.skuId ?? '',
                      'productName': widget.productName ?? '商品評價',
                    },
                  );
                },
                child: const Text(
                  '查看全部 >',
                  style: TextStyle(fontSize: 14, color: AppColors.primary),
                ),
              ),
            ],
          ),
          SizedBox(height: 8.h),
          const Divider(color: AppColors.gray100, height: 1),
          SizedBox(height: 16.h),
          // 評論列表
          reviewCount == 0
              ? Text('暫無評論', style: AppTypography.metaMuted)
              : Column(
                  children: displayReviews.asMap().entries.map((entry) {
                    final isLast = entry.key == displayReviews.length - 1;
                    return _buildReviewItemNew(
                      entry.value,
                      showDivider: !isLast,
                    );
                  }).toList(),
                ),
        ],
      ),
    );
  }

  List<String> _extractImages(Map<String, dynamic> review) {
    // 49 §6.1 成對契約：評論附圖 imageKeys（objectKey）優先，imageUrls 退路。
    final raw = review['imageKeys'] ?? review['imageUrls'];
    if (raw is List) {
      return raw
          .map((e) => e.toString().trim())
          .where((s) => s.isNotEmpty)
          .toList();
    }
    return [];
  }

  Widget _buildReviewItemNew(
    Map<String, dynamic> review, {
    bool showDivider = true,
  }) {
    // 取得用戶名稱
    String userName = '';
    final nameKeys = [
      'username', // API 實際回傳欄位（後端已做遮罩）
      'userName',
      'memberName',
      'name',
      'nickname',
      'user',
      'member',
      'reviewer',
      'author',
    ];
    for (final key in nameKeys) {
      final val = review[key];
      if (val != null && val.toString().trim().isNotEmpty) {
        userName = val.toString().trim();
        break;
      }
    }
    // API 回傳的 username 後端已遮罩（P** C** YA**），不再做二次遮罩
    final displayName = userName.isNotEmpty ? userName : '匿名';

    // 取得評論者頭像：49 §6.1 成對契約，SSOT 是 avatarKey（裸 objectKey），
    // 由 AppCachedImage 的 scene 走 buildImageUrl 組 avatar preset URL。
    final avatarUrl = resolveAvatarKey(review) ?? '';

    // 取得日期
    final dateStr = _formatDate(review['createdAt']);

    // 取得評論 ID 與作者 buyerId（供檢舉／封鎖使用）
    final reviewId = (review['reviewId'] ?? '').toString();
    final reviewUserId = (review['buyerId'] ?? '').toString();

    // 取得星星數
    final ratingValue =
        review['rating'] ??
        review['score'] ??
        review['stars'] ??
        review['star'] ??
        5;
    final rating = (ratingValue is num)
        ? ratingValue.toInt()
        : int.tryParse('$ratingValue') ?? 5;

    // 取得評論內容
    final content =
        (review['content'] ?? review['comment'] ?? review['text'] ?? '')
            .toString();

    return Container(
      padding: EdgeInsets.only(bottom: 16.h),
      decoration: showDivider
          ? const BoxDecoration(
              border: Border(
                bottom: BorderSide(color: AppColors.bgPrimary, width: 0.564),
              ),
            )
          : null,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisSize: MainAxisSize.min,
        children: [
          // 用戶資訊列
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Expanded(
                child: Row(
                  children: [
                    // 用戶頭像 (24x24 圓形) — 有 URL 載圖片，沒有則顯示名字首字
                    ClipOval(
                      child: SizedBox(
                        width: 24.w,
                        height: 24.h,
                        child: avatarUrl.isNotEmpty
                            ? AppCachedImage(
                                imageUrl: avatarUrl,
                                width: 24.w,
                                height: 24.h,
                                fit: BoxFit.cover,
                                scene: ImageScene.avatarSmall,
                              )
                            : Container(
                                color: AppColors.bgPrimary,
                                alignment: Alignment.center,
                                child: Text(
                                  displayName.isNotEmpty
                                      ? displayName[0].toUpperCase()
                                      : 'U',
                                  style: TextStyle(
                                    fontSize: 12,
                                    fontWeight: FontWeight.w700,
                                    color: AppColors.primary,
                                    letterSpacing: 0.1172,
                                  ),
                                ),
                              ),
                      ),
                    ),
                    SizedBox(width: 8.w),
                    // 用戶名與日期共用可伸縮區域，保留更多操作的完整點擊範圍。
                    Flexible(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            displayName,
                            style: AppTypography.body,
                            overflow: TextOverflow.ellipsis,
                          ),
                          if (dateStr.isNotEmpty)
                            Text(
                              dateStr,
                              style: AppTypography.meta.copyWith(
                                color: AppColors.textSecondary,
                              ),
                            ),
                        ],
                      ),
                    ),
                  ],
                ),
              ),
              // 日期 + 檢舉/封鎖操作
              Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  if (reviewId.isNotEmpty) ...[
                    SizedBox(width: 8.w),
                    HkFrostedIconButton(
                      label: '更多評價操作',
                      onPressed: () async {
                        if (!await requireLogin(context, ref)) return;
                        if (!mounted) return;
                        await showReportBlockActionSheet(
                          context,
                          reportTargetType: ReportTargetType.review,
                          reportTargetId: reviewId,
                          blockUserId: reviewUserId.isNotEmpty
                              ? reviewUserId
                              : null,
                          blockUserLabel: displayName,
                        );
                      },
                      icon: AppIcons.moreHoriz,
                    ),
                  ],
                ],
              ),
            ],
          ),
          SizedBox(height: 8.h),
          // 星星評分 (12x12)
          Row(
            children: List.generate(rating.clamp(0, 5), (index) {
              return Padding(
                padding: EdgeInsets.only(right: 4.w),
                child: Icon(
                  AppIcons.starRounded,
                  size: 12.sp,
                  color: AppIconColors.gold,
                ),
              );
            }),
          ),
          // 評論內容
          if (content.isNotEmpty) ...[
            SizedBox(height: 8.h),
            Text(
              content,
              style: AppTypography.body,
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
            ),
          ],
          // 評論圖片
          if (_extractImages(review).isNotEmpty) ...[
            SizedBox(height: 8.h),
            SizedBox(
              height: 60.w,
              child: ListView.separated(
                scrollDirection: Axis.horizontal,
                itemCount: _extractImages(review).length,
                separatorBuilder: (_, __) => SizedBox(width: 6.w),
                itemBuilder: (_, i) {
                  final imgUrl = _extractImages(review)[i];
                  return ClipRRect(
                    borderRadius: BorderRadius.circular(6.r),
                    child: AppCachedImage(
                      imageUrl: imgUrl,
                      width: 60.w,
                      height: 60.w,
                      fit: BoxFit.cover,
                      scene: ImageScene.reviewPhoto,
                      errorWidget: SizedBox(
                        width: 60.w,
                        height: 60.w,
                        child: HkPlaceholder(HkPlaceholderKind.photo),
                      ),
                    ),
                  );
                },
              ),
            ),
          ],
        ],
      ),
    );
  }
}

/// PDP 售完時的補貨通知 CTA（#942），沿用共用購買 footer 的尺寸與字體。
class _RestockSubscribeButton extends StatelessWidget {
  const _RestockSubscribeButton({
    required this.subscribed,
    required this.loading,
    required this.onPressed,
  });

  final bool subscribed;
  final bool loading;
  final VoidCallback onPressed;

  @override
  Widget build(BuildContext context) {
    final bgColor = subscribed ? AppColors.primary : AppColors.bgCard;
    final fgColor = subscribed ? AppColors.bgCard : AppColors.primary;
    final label = subscribed ? '已開啟補貨通知' : '通知我補貨';
    final content = Container(
      width: double.infinity,
      padding: const EdgeInsets.fromLTRB(
        HkSpace.xl,
        HkSpace.md,
        HkSpace.xl,
        HkSpace.lg,
      ),
      decoration: BoxDecoration(color: Colors.transparent),
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
              Text(
                '已售完',
                style: AppTypography.body.copyWith(color: AppColors.danger),
              ),
            ],
          ),
          const SizedBox(height: HkSpace.sm),
          Semantics(
            button: true,
            enabled: !loading,
            label: label,
            onTap: loading ? null : onPressed,
            child: ExcludeSemantics(
              child: Material(
                color: bgColor,
                borderRadius: BorderRadius.circular(HkRadius.pill),
                child: InkWell(
                  onTap: loading ? null : onPressed,
                  borderRadius: BorderRadius.circular(HkRadius.pill),
                  child: Container(
                    width: double.infinity,
                    height: 56,
                    decoration: BoxDecoration(
                      borderRadius: BorderRadius.circular(HkRadius.pill),
                      border: Border.all(color: AppColors.primary),
                    ),
                    padding: const EdgeInsets.symmetric(horizontal: HkSpace.xl),
                    child: Center(
                      child: loading
                          ? SizedBox(
                              width: 20,
                              height: 20,
                              child: HkBrandLoader(
                                tone: HkBrandLoaderTone.mono,
                                color: fgColor,
                              ),
                            )
                          : Row(
                              mainAxisSize: MainAxisSize.min,
                              mainAxisAlignment: MainAxisAlignment.center,
                              children: [
                                Icon(
                                  subscribed
                                      ? AppIcons.notificationsActive
                                      : AppIcons.notificationsOutlined,
                                  size: 22,
                                  color: fgColor,
                                ),
                                const SizedBox(width: HkSpace.sm),
                                Flexible(
                                  child: Text(
                                    label,
                                    maxLines: 1,
                                    overflow: TextOverflow.ellipsis,
                                    style: AppTypography.buttonLarge.copyWith(
                                      color: fgColor,
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
        ],
      ),
    );
    return KeyedSubtree(
      key: const ValueKey('product-restock-integrated-surface'),
      child: content,
    );
  }
}
