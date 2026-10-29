import 'dart:async';

import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.pb.dart' as buyer;
import 'package:hawkr_api/images.dart' show ImagePreset, ImageUri;
import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/app_typography.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter/services.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../config/image_config.dart';
import '../../../core/session/session_provider.dart';
import '../../../core/storage/auth_storage.dart';
import '../../../shared/widgets/app_cached_image.dart';
import '../../../shared/widgets/common_top_bar.dart';
import '../../../shared/widgets/common_dialog.dart';
import '../../../shared/widgets/common_snackbar.dart';
import '../../../shared/widgets/hk_button_label.dart';
import '../../../shared/widgets/hk_frosted_card_surface.dart';
import '../../../shared/widgets/fruit_refresh_indicator.dart';
import '../../../shared/widgets/glass_tab_bar.dart';
import '../../../shared/widgets/location_icon.dart';
import '../../cart/data/cart_repository.dart';
import '../../cart/data/cart_error_message.dart';
import '../../cart/presentation/cart_controller.dart';
import '../../follow/data/follow_repository.dart';
import '../../home/presentation/home_widget/product_add_to_cart_sheet.dart';
import '../../product/data/product_repository.dart';
import '../../product/presentation/merchant_item_detail_screen.dart';
import '../../review/data/review_repository.dart';
import '../../seller/data/seller_repository.dart';
import '../../seller/data/store_card_display.dart';
import '../../visit/data/visit_repository.dart';
import '../../../shared/utils/price_display.dart';
import 'package:hawkr_buyer/shared/widgets/hk_brand_loader.dart';
import 'package:hawkr_buyer/shared/widgets/hk_skeleton_shimmer.dart';
import 'package:hawkr_buyer/shared/widgets/hk_placeholder.dart';

class _SubscriptionGlassBackdrop extends StatelessWidget {
  const _SubscriptionGlassBackdrop();

  @override
  Widget build(BuildContext context) {
    return RepaintBoundary(
      key: const ValueKey('subscription-glass-backdrop'),
      child: DecoratedBox(
        decoration: const BoxDecoration(
          gradient: LinearGradient(
            begin: Alignment.topLeft,
            end: Alignment.bottomRight,
            colors: [
              AppColors.neutralPage,
              AppColors.neutralCream,
              AppColors.neutralWhisper,
            ],
            stops: [0, 0.54, 1],
          ),
        ),
        child: Stack(
          fit: StackFit.expand,
          children: [
            Align(
              alignment: const Alignment(1.25, -0.72),
              child: Container(
                width: 240,
                height: 240,
                decoration: BoxDecoration(
                  shape: BoxShape.circle,
                  gradient: RadialGradient(
                    colors: [
                      AppColors.brandSecondary.withValues(alpha: 0.12),
                      AppColors.brandSecondary.withValues(alpha: 0),
                    ],
                  ),
                ),
              ),
            ),
            Align(
              alignment: const Alignment(-1.35, 0.32),
              child: Container(
                width: 280,
                height: 280,
                decoration: BoxDecoration(
                  shape: BoxShape.circle,
                  gradient: RadialGradient(
                    colors: [
                      AppColors.brandPrimaryStrong.withValues(alpha: 0.10),
                      AppColors.brandPrimaryStrong.withValues(alpha: 0),
                    ],
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

// ===== 魚骨圖載入效果元件 =====
class _SkeletonBar extends StatelessWidget {
  const _SkeletonBar({
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

/// 訂閱卡片的骨架圖
class _FollowingSkeletonCard extends StatelessWidget {
  const _FollowingSkeletonCard();

  @override
  Widget build(BuildContext context) {
    return HkSkeletonShimmer(
      child: Padding(
        padding: const EdgeInsets.only(bottom: HkSpace.lg),
        child: HkFrostedCardSurface(
          shadows: HkShadows.xs,
          child: Padding(
            padding: const EdgeInsets.all(HkSpace.md),
            child: Row(
              children: [
                // 頭像骨架
                Container(
                  width: 56.w,
                  height: 56.h,
                  decoration: BoxDecoration(
                    color: AppColors.surfaceMuted.withValues(alpha: 0.72),
                    shape: BoxShape.circle,
                  ),
                ),
                SizedBox(width: 12.w),
                // 資訊骨架
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      _SkeletonBar(width: 120, height: 16.h),
                      SizedBox(height: 8.h),
                      _SkeletonBar(width: 160, height: 12.h),
                      SizedBox(height: 6.h),
                      _SkeletonBar(width: 100, height: 12.h),
                    ],
                  ),
                ),
                SizedBox(width: 12.w),
                // 按鈕骨架
                _SkeletonBar(width: 80, height: 32.h, radius: 16),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

class _ProductSkeletonCard extends StatelessWidget {
  const _ProductSkeletonCard();

  @override
  Widget build(BuildContext context) {
    return HkSkeletonShimmer(
      child: Padding(
        padding: const EdgeInsets.only(bottom: HkSpace.lg),
        child: HkFrostedCardSurface(
          shadows: HkShadows.xs,
          child: Padding(
            padding: const EdgeInsets.all(HkSpace.md),
            child: Column(
              children: [
                Row(
                  children: [
                    Container(
                      width: 38,
                      height: 38,
                      decoration: BoxDecoration(
                        color: AppColors.surfaceMuted.withValues(alpha: 0.72),
                        shape: BoxShape.circle,
                      ),
                    ),
                    const SizedBox(width: HkSpace.sm),
                    const Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        _SkeletonBar(width: 104, height: 14),
                        SizedBox(height: HkSpace.xs),
                        _SkeletonBar(width: 142, height: 10),
                      ],
                    ),
                  ],
                ),
                const SizedBox(height: HkSpace.md),
                LayoutBuilder(
                  builder: (context, constraints) {
                    final imageWidth = constraints.maxWidth < 300
                        ? 112.0
                        : 128.0;
                    return Row(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        SizedBox(
                          width: imageWidth,
                          child: AspectRatio(
                            aspectRatio: 4 / 3,
                            child: DecoratedBox(
                              decoration: BoxDecoration(
                                color: AppColors.surfaceMuted,
                                borderRadius: BorderRadius.circular(
                                  HkRadius.md,
                                ),
                              ),
                            ),
                          ),
                        ),
                        const SizedBox(width: HkSpace.md),
                        const Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              _SkeletonBar(width: double.infinity, height: 16),
                              SizedBox(height: HkSpace.sm),
                              _SkeletonBar(width: 92, height: 12),
                              SizedBox(height: HkSpace.xxl),
                              _SkeletonBar(width: 110, height: 18),
                            ],
                          ),
                        ),
                      ],
                    );
                  },
                ),
                const SizedBox(height: HkSpace.md),
                const Row(
                  children: [
                    Expanded(
                      child: _SkeletonBar(
                        width: double.infinity,
                        height: HkControl.minTap,
                        radius: HkRadius.pill,
                      ),
                    ),
                    SizedBox(width: HkSpace.sm),
                    Expanded(
                      child: _SkeletonBar(
                        width: double.infinity,
                        height: HkControl.minTap,
                        radius: HkRadius.pill,
                      ),
                    ),
                    SizedBox(width: HkSpace.sm),
                    Expanded(
                      child: _SkeletonBar(
                        width: double.infinity,
                        height: HkControl.minTap,
                        radius: HkRadius.pill,
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
}

/// 訂閱的骨架圖列表
class _FollowingSkeletonList extends StatelessWidget {
  const _FollowingSkeletonList({this.count = 5, this.productCards = false});

  final int count;
  final bool productCards;

  @override
  Widget build(BuildContext context) {
    return ListView.builder(
      padding: EdgeInsets.symmetric(horizontal: 16.w, vertical: 8.h),
      itemCount: count,
      itemBuilder: (context, index) => productCards
          ? const _ProductSkeletonCard()
          : const _FollowingSkeletonCard(),
    );
  }
}

class MySubscriptionScreen extends ConsumerStatefulWidget {
  const MySubscriptionScreen({super.key, this.initialTab = 0});
  final int initialTab; // 0: 訂閱, 1: 收藏, 2: 瀏覽

  @override
  ConsumerState<MySubscriptionScreen> createState() =>
      _MySubscriptionScreenState();
}

class _MySubscriptionScreenState extends ConsumerState<MySubscriptionScreen> {
  late int _tabIndex;
  List<Map<String, dynamic>> _following = [];
  bool _loadingFollowing = false;
  bool _showSkeletonFollowing = false;
  Timer? _skeletonTimerFollowing;
  List<Map<String, dynamic>> _likes = [];
  bool _loadingLikes = false;
  bool _showSkeletonLikes = false;
  Timer? _skeletonTimerLikes;
  List<Map<String, dynamic>> _recentViews = [];
  bool _loadingRecent = false;
  bool _showSkeletonRecent = false;
  Timer? _skeletonTimerRecent;
  bool _refreshingInPlace = false;
  // cursor 分頁：token 空字串表示沒有下一頁；generation 讓重新整理之後才回來的
  // 舊 loadMore 結果直接丟掉，不會接到新列表後面。
  String _followingPageToken = '';
  bool _loadingMoreFollowing = false;
  int _followingTotal = 0;
  int _followingGeneration = 0;
  String _likesPageToken = '';
  bool _loadingMoreLikes = false;
  int _likesTotal = 0;
  int _likesGeneration = 0;
  String _recentPageToken = '';
  bool _loadingMoreRecent = false;
  int _recentGeneration = 0;
  final Set<String> _recentSeen = {};
  Set<String> _favoriteIds = {};
  String? _buyingProductId;

  @override
  void initState() {
    super.initState();
    _tabIndex = widget.initialTab;
    if (_tabIndex == 0) {
      _loadFollowing();
    } else if (_tabIndex == 1) {
      _loadLikes();
    } else {
      _loadRecentViews();
    }
  }

  @override
  void dispose() {
    _skeletonTimerFollowing?.cancel();
    _skeletonTimerLikes?.cancel();
    _skeletonTimerRecent?.cancel();
    super.dispose();
  }

  double _parseRatingValue(dynamic value) {
    if (value == null) return 0;
    final raw = value is num
        ? value.toDouble()
        : double.tryParse(value.toString());
    if (raw == null || raw <= 0) return 0;
    final rating = raw > 5 && raw <= 50 ? raw / 10 : raw;
    return rating.clamp(0, 5).toDouble();
  }

  int _parseIntValue(dynamic value) {
    if (value == null) return 0;
    if (value is int) return value;
    if (value is num) return value.toInt();
    return int.tryParse(value.toString()) ?? 0;
  }

  int _reviewCountFromItem(Map<String, dynamic> item) {
    return _parseIntValue(item['reviewCount']);
  }

  void _normalizeProductReviewStats(Map<String, dynamic> item) {
    var rating = 0.0;
    // 收藏卡片吐 ratingX10（十倍整數），其他卡片吐 rating（浮點）。
    for (final key in const ['rating', 'ratingX10']) {
      rating = _parseRatingValue(item[key]);
      if (rating > 0) break;
    }
    if (rating > 0) item['rating'] = rating;

    final reviewCount = _reviewCountFromItem(item);
    if (reviewCount > 0) item['reviewCount'] = reviewCount;
  }

  void _applyReviewStatsFromReviews(
    Map<String, dynamic> item,
    List<Map<String, dynamic>> reviews,
  ) {
    item['reviewCount'] = reviews.length;
    if (reviews.isEmpty) {
      item['rating'] = 0.0;
      return;
    }

    var total = 0.0;
    var validCount = 0;
    for (final review in reviews) {
      final rating = _parseRatingValue(review['rating']);
      if (rating <= 0) continue;
      total += rating;
      validCount++;
    }
    item['rating'] = validCount > 0 ? total / validCount : 0.0;
  }

  Future<void> _enrichProductReviewStats(
    Map<String, dynamic> item,
    String productId,
  ) async {
    _normalizeProductReviewStats(item);

    final hasReviewCount = _reviewCountFromItem(item) > 0;
    final hasRating = _parseRatingValue(item['rating']) > 0;
    if (hasReviewCount && hasRating) return;

    try {
      final reviewRepository = ref.read(reviewRepositoryProvider);
      final summary = await reviewRepository.getMerchantProductReviewSummary(
        productId,
      );
      final hasSummaryTotal = summary['hasTotal'] == true;
      final summaryTotal = _parseIntValue(summary['total']);
      final summaryRating = _parseRatingValue(summary['rating']);
      if (hasSummaryTotal) item['reviewCount'] = summaryTotal;
      if (summaryRating > 0) item['rating'] = summaryRating;
      if (hasSummaryTotal &&
          (summaryTotal <= 0 || summaryRating > 0 || hasRating)) {
        return;
      }

      final reviews = await reviewRepository.getMerchantProductReviews(
        productId,
      );
      if (reviews.isEmpty) {
        if (!hasReviewCount) item['reviewCount'] = 0;
        if (!hasRating) item['rating'] = 0.0;
        return;
      }
      _applyReviewStatsFromReviews(item, reviews);
      if (hasSummaryTotal) item['reviewCount'] = summaryTotal;
    } catch (_) {}
  }

  /// 追蹤店家卡片的顯示資料，全部來自 API 回的店家卡片。
  Map<String, dynamic> _followingItem(buyer.Follow row) {
    final card = row.store;
    return {
      'merchantId': row.storeId,
      'name': card.displayName,
      'avatar': card.logoUri,
      'address': card.location,
      'rating': card.hasRatingStats ? card.ratingAverage.toDouble() : 0.0,
      'reviewCount': card.hasRatingStats ? card.reviewCount : 0,
      'storeCode': card.isOpen ? card.storeCode : '',
      'statusLabel': card.statusLabel ?? '',
    };
  }

  Map<String, dynamic> _favoriteItem(buyer.Favorite favorite) {
    final product = favorite.product;
    final store = product.store;
    return {
      'id': product.id,
      'productId': product.id,
      'productName': product.name,
      'imageKey': product.hasImage() ? product.image.uri(ImagePreset.card) : '',
      'sellerId': product.storeId,
      'sellerName': store.displayName,
      'storeName': store.displayName,
      'sellerAvatar': store.logoUri,
      'distance': store.location,
      'storeCode': store.isOpen ? store.storeCode : '',
      'priceMin': product.minPrice.toInt(),
      'priceMax': product.maxPrice.toInt(),
      'ratingX10': product.hasRating() ? (product.rating * 10).round() : null,
      'available': product.available,
      'createdAt': favorite.hasCreateTime()
          ? favorite.createTime.toDateTime().millisecondsSinceEpoch
          : null,
    };
  }

  Map<String, dynamic> _visitItem(buyer.Visit visit) {
    final product = visit.product;
    return {
      'id': product.id,
      'productId': product.id,
      'productName': product.name,
      'imageKey': product.hasImage() ? product.image.uri(ImagePreset.card) : '',
      'sellerId': product.storeId,
      'sellerName': product.store.displayName,
      'storeName': product.store.displayName,
      'sellerAvatar': product.store.logoUri,
      'distance': product.store.location,
      'storeCode': product.store.isOpen ? product.store.storeCode : '',
      'minPrice': product.minPrice.toString(),
      'maxPrice': product.maxPrice.toString(),
      'rating': product.hasRating() ? product.rating.toString() : '',
      'available': product.available,
      'visitedAt': visit.hasVisitTime()
          ? visit.visitTime.toDateTime().toIso8601String()
          : '',
    };
  }

  void _startSkeleton(int tab) {
    switch (tab) {
      case 0:
        _skeletonTimerFollowing?.cancel();
        _skeletonTimerFollowing = Timer(const Duration(milliseconds: 300), () {
          if (mounted && _loadingFollowing && !_refreshingInPlace) {
            setState(() => _showSkeletonFollowing = true);
          }
        });
      case 1:
        _skeletonTimerLikes?.cancel();
        _skeletonTimerLikes = Timer(const Duration(milliseconds: 300), () {
          if (mounted && _loadingLikes && !_refreshingInPlace) {
            setState(() => _showSkeletonLikes = true);
          }
        });
      default:
        _skeletonTimerRecent?.cancel();
        _skeletonTimerRecent = Timer(const Duration(milliseconds: 300), () {
          if (mounted && _loadingRecent && !_refreshingInPlace) {
            setState(() => _showSkeletonRecent = true);
          }
        });
    }
  }

  /// 讀一頁追蹤店家。
  Future<({List<Map<String, dynamic>> items, String next, int total})>
  _fetchFollowingPage(String pageToken) async {
    final page = await ref
        .read(followRepositoryProvider)
        .listFollows(pageToken: pageToken);
    final mapped = [
      for (final row in page.follows)
        if (row.storeId.trim().isNotEmpty) _followingItem(row),
    ];
    return (items: mapped, next: page.nextPageToken, total: page.totalSize);
  }

  Future<void> _loadFollowing() async {
    final generation = ++_followingGeneration;
    setState(() {
      _loadingFollowing = true;
      _loadingMoreFollowing = false;
      _showSkeletonFollowing = false;
    });
    _startSkeleton(0);
    try {
      final page = await _fetchFollowingPage('');
      _skeletonTimerFollowing?.cancel();
      if (!mounted || generation != _followingGeneration) return;
      setState(() {
        _following = page.items;
        _followingPageToken = page.next;
        _followingTotal = page.total;
        _loadingFollowing = false;
        _showSkeletonFollowing = false;
      });
    } catch (_) {
      _skeletonTimerFollowing?.cancel();
      if (!mounted || generation != _followingGeneration) return;
      setState(() {
        _loadingFollowing = false;
        _showSkeletonFollowing = false;
      });
    }
  }

  Future<void> _loadMoreFollowing() async {
    if (_followingPageToken.isEmpty ||
        _loadingFollowing ||
        _loadingMoreFollowing) {
      return;
    }
    final generation = _followingGeneration;
    setState(() => _loadingMoreFollowing = true);
    try {
      final page = await _fetchFollowingPage(_followingPageToken);
      if (!mounted || generation != _followingGeneration) return;
      setState(() {
        final seen = {for (final m in _following) m['merchantId']};
        _following = [
          ..._following,
          ...page.items.where((m) => !seen.contains(m['merchantId'])),
        ];
        _followingPageToken = page.next;
        _followingTotal = page.total;
      });
    } catch (_) {
      // 保留 token，下次捲到底再試。
    } finally {
      if (mounted && generation == _followingGeneration) {
        setState(() => _loadingMoreFollowing = false);
      }
    }
  }

  /// 讀一頁收藏並補齊評價。
  Future<({List<Map<String, dynamic>> items, String next, int total})>
  _fetchLikesPage(String pageToken) async {
    final page = await ref
        .read(visitRepositoryProvider)
        .listFavorites(pageToken: pageToken);
    final enriched = [for (final f in page.favorites) _favoriteItem(f)];

    await Future.wait([
      for (final item in enriched)
        if ((item['productId'] ?? '').toString().trim().isNotEmpty)
          _enrichProductReviewStats(
            item,
            (item['productId'] ?? '').toString().trim(),
          ),
    ]);

    return (items: enriched, next: page.nextPageToken, total: page.totalSize);
  }

  Set<String> _productIdsOf(Iterable<Map<String, dynamic>> items) => {
    for (final m in items)
      if ((m['productId'] ?? '').toString().trim().isNotEmpty)
        (m['productId'] ?? '').toString().trim(),
  };

  Future<void> _loadLikes() async {
    final generation = ++_likesGeneration;
    setState(() {
      _loadingLikes = true;
      _loadingMoreLikes = false;
      _showSkeletonLikes = false;
    });
    _startSkeleton(1);
    try {
      final page = await _fetchLikesPage('');
      _skeletonTimerLikes?.cancel();
      if (!mounted || generation != _likesGeneration) return;
      setState(() {
        _likes = page.items;
        // 收藏 tab 的商品全部都是已收藏的，同步 _favoriteIds
        _favoriteIds = _productIdsOf(page.items);
        _likesPageToken = page.next;
        _likesTotal = page.total;
        _loadingLikes = false;
        _showSkeletonLikes = false;
      });
    } catch (_) {
      _skeletonTimerLikes?.cancel();
      if (!mounted || generation != _likesGeneration) return;
      setState(() {
        _loadingLikes = false;
        _showSkeletonLikes = false;
      });
    }
  }

  Future<void> _loadMoreLikes() async {
    if (_likesPageToken.isEmpty || _loadingLikes || _loadingMoreLikes) return;
    final generation = _likesGeneration;
    setState(() => _loadingMoreLikes = true);
    try {
      final page = await _fetchLikesPage(_likesPageToken);
      if (!mounted || generation != _likesGeneration) return;
      setState(() {
        final seen = _productIdsOf(_likes);
        _likes = [
          ..._likes,
          ...page.items.where(
            (m) => !seen.contains((m['productId'] ?? '').toString().trim()),
          ),
        ];
        _favoriteIds.addAll(_productIdsOf(page.items));
        _likesPageToken = page.next;
        _likesTotal = page.total;
      });
    } catch (_) {
      // 保留 token，下次捲到底再試。
    } finally {
      if (mounted && generation == _likesGeneration) {
        setState(() => _loadingMoreLikes = false);
      }
    }
  }

  /// 讀一頁瀏覽紀錄，補齊商品與商家資訊，並只對這一頁的商品查收藏狀態。
  Future<({List<Map<String, dynamic>> items, String next, Set<String> liked})>
  _fetchRecentPage(String pageToken, Set<String> seen) async {
    final page = await ref
        .read(visitRepositoryProvider)
        .listVisits(pageToken: pageToken);
    // 去除重複的商品（根據 seller_id + product_id 組合判斷，跨頁也算）
    // seller_id == 0 為已刪除商品的幽靈紀錄（欄位全空），直接排除
    final uniqueVisits = <Map<String, dynamic>>[];
    for (final visit in page.visits) {
      final m = _visitItem(visit);
      final mid = (m['sellerId'] ?? '').toString().trim();
      final pid = (m['productId'] ?? '').toString().trim();
      final key = '$mid|$pid';
      if (mid.isNotEmpty && mid != '0' && pid.isNotEmpty && seen.add(key)) {
        uniqueVisits.add(m);
      }
    }
    final enriched = uniqueVisits;

    final liked = <String>{};
    try {
      final ids = [
        for (final pid in _productIdsOf(enriched))
          if (int.tryParse(pid) != null) int.parse(pid),
      ];
      if (ids.isNotEmpty) {
        final matched = await ref
            .read(visitRepositoryProvider)
            .batchCheckFavorites(ids);
        liked.addAll(matched.map((id) => id.toString()));
      }
    } catch (_) {}

    await Future.wait(
      enriched.asMap().entries.map((entry) async {
        final i = entry.key;
        final m = entry.value;
        final mid = (m['sellerId'] ?? '').toString().trim();
        final pid = (m['productId'] ?? '').toString().trim();
        if (mid.isEmpty || pid.isEmpty) return;

        // 載入商品圖片（若 API 未回傳則補抓）；49 號成對契約 key 優先。
        if ((m['imageKey'] ?? m['imageUrl'] ?? m['productImage'] ?? '')
            .toString()
            .isEmpty) {
          try {
            final imgs = await ref
                .read(productRepositoryProvider)
                .getProductImages(pid);
            if (imgs.isNotEmpty) {
              enriched[i]['productImage'] = imgs.first.toString().trim();
            }
          } catch (_) {}
        }

        // 載入商品規格價格範圍（若 API 未回傳則補抓）
        if ((m['minPrice'] ?? '').toString().isEmpty) {
          try {
            final grades = await ref
                .read(productRepositoryProvider)
                .getMerchantProductGrades(mid, pid);
            if (grades.isNotEmpty) {
              num? minPrice;
              num? maxPrice;
              for (final grade in grades) {
                final priceVal =
                    grade['price'] ?? grade['unitPrice'] ?? grade['salePrice'];
                if (priceVal != null) {
                  num price;
                  if (priceVal is num) {
                    price = priceVal;
                  } else {
                    final parsed = num.tryParse(priceVal.toString());
                    if (parsed == null) continue;
                    price = parsed;
                  }
                  if (price > 0) {
                    if (minPrice == null || price < minPrice) {
                      minPrice = price;
                    }
                    if (maxPrice == null || price > maxPrice) {
                      maxPrice = price;
                    }
                  }
                }
              }
              if (minPrice != null) {
                enriched[i]['minPrice'] = minPrice.toString();
                enriched[i]['maxPrice'] = (maxPrice ?? minPrice).toString();
              }
            }
          } catch (_) {}
        }

        await _enrichProductReviewStats(enriched[i], pid);
      }),
    );

    return (items: enriched, next: page.nextPageToken, liked: liked);
  }

  void _mergeFavoriteState(
    Iterable<Map<String, dynamic>> items,
    Set<String> liked,
  ) {
    for (final pid in _productIdsOf(items)) {
      if (liked.contains(pid)) {
        _favoriteIds.add(pid);
      } else {
        _favoriteIds.remove(pid);
      }
    }
  }

  Future<void> _loadRecentViews() async {
    final generation = ++_recentGeneration;
    setState(() {
      _loadingRecent = true;
      _loadingMoreRecent = false;
      _showSkeletonRecent = false;
    });
    _startSkeleton(2);
    try {
      final seen = <String>{};
      final page = await _fetchRecentPage('', seen);
      _skeletonTimerRecent?.cancel();
      if (!mounted || generation != _recentGeneration) return;
      setState(() {
        _recentSeen
          ..clear()
          ..addAll(seen);
        _recentViews = page.items;
        _mergeFavoriteState(page.items, page.liked);
        _recentPageToken = page.next;
        _loadingRecent = false;
        _showSkeletonRecent = false;
      });
    } catch (_) {
      _skeletonTimerRecent?.cancel();
      if (!mounted || generation != _recentGeneration) return;
      setState(() {
        _loadingRecent = false;
        _showSkeletonRecent = false;
      });
    }
  }

  Future<void> _loadMoreRecentViews() async {
    if (_recentPageToken.isEmpty || _loadingRecent || _loadingMoreRecent) {
      return;
    }
    final generation = _recentGeneration;
    setState(() => _loadingMoreRecent = true);
    try {
      final seen = {..._recentSeen};
      final page = await _fetchRecentPage(_recentPageToken, seen);
      if (!mounted || generation != _recentGeneration) return;
      setState(() {
        _recentSeen.addAll(seen);
        _recentViews = [..._recentViews, ...page.items];
        _mergeFavoriteState(page.items, page.liked);
        _recentPageToken = page.next;
      });
    } catch (_) {
      // 保留 token，下次捲到底再試。
    } finally {
      if (mounted && generation == _recentGeneration) {
        setState(() => _loadingMoreRecent = false);
      }
    }
  }

  Future<void> _unfollowAt(int index) async {
    if (index < 0 || index >= _following.length) return;
    final mid = (_following[index]['merchantId'] ?? '').toString().trim();
    final name = (_following[index]['name'] ?? '').toString().trim();
    if (mid.isEmpty) return;

    final confirmed = await showCommonDialog(
      context: context,
      title: '取消訂閱？',
      content: '取消後將不再優先看到「${name.isNotEmpty ? name : '此商家'}」的更新。',
      confirmText: '取消訂閱',
      cancelText: '繼續訂閱',
      confirmColor: AppColors.danger,
      barrierDismissible: true,
    );
    if (confirmed != true) return;

    final ok = await ref.read(sellerRepositoryProvider).unfollowMerchant(mid);
    if (!mounted) return;
    if (ok) {
      HapticFeedback.lightImpact();
      setState(() {
        final before = _following.length;
        _following.removeWhere(
          (item) => (item['merchantId'] ?? '').toString().trim() == mid,
        );
        _followingTotal -= before - _following.length;
        if (_followingTotal < _following.length) {
          _followingTotal = _following.length;
        }
      });
      showFloatingToast(context, '已取消訂閱');
    } else {
      showFloatingToast(context, '取消訂閱失敗，請稍後再試', isSuccess: false);
    }
  }

  Future<void> _toggleFavorite(String productId) async {
    final pid = productId.trim();
    if (pid.isEmpty) return;
    final bool wasFavorite = _favoriteIds.contains(pid);
    final bool target = !wasFavorite;

    // 樂觀 UI 更新
    setState(() {
      if (target) {
        _favoriteIds.add(pid);
      } else {
        _favoriteIds.remove(pid);
      }
    });

    try {
      bool ok;
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
        // API 失敗 → 回滾
        setState(() {
          if (wasFavorite) {
            _favoriteIds.add(pid);
          } else {
            _favoriteIds.remove(pid);
          }
        });
        showFloatingToast(context, '收藏狀態更新失敗', isSuccess: false);
      }
      // 收藏 tab：取消收藏成功時，從列表中移除該商品
      if (!target && ok && mounted) {
        HapticFeedback.lightImpact();
        setState(() => _removeLike(pid));
        showFloatingToast(context, '已從收藏移除');
      } else if (target && ok && mounted) {
        HapticFeedback.lightImpact();
        showFloatingToast(context, '已加入收藏');
      }
    } catch (_) {
      if (mounted) {
        setState(() {
          if (wasFavorite) {
            _favoriteIds.add(pid);
          } else {
            _favoriteIds.remove(pid);
          }
        });
        showFloatingToast(context, '收藏狀態更新失敗', isSuccess: false);
      }
    }
  }

  void _handleFavoriteResult(Map<String, dynamic>? result) {
    if (!mounted || result == null) return;
    final pid = (result['productId'] ?? '').toString().trim();
    final isFav = result['isFavorite'] == true;
    if (pid.isEmpty) return;
    setState(() {
      if (isFav) {
        _favoriteIds.add(pid);
      } else {
        _favoriteIds.remove(pid);
        // 收藏 tab：取消收藏時，從列表中移除
        _removeLike(pid);
      }
    });
  }

  void _removeLike(String pid) {
    final before = _likes.length;
    _likes.removeWhere(
      (item) => (item['productId'] ?? '').toString().trim() == pid,
    );
    _likesTotal -= before - _likes.length;
    if (_likesTotal < _likes.length) _likesTotal = _likes.length;
  }

  Future<void> _handleBuyNow(String productId) async {
    if (_buyingProductId != null) return;
    final pid = productId.trim();
    if (pid.isEmpty) {
      showFloatingToast(context, '商品資料不完整，請稍後再試', isSuccess: false);
      return;
    }

    setState(() => _buyingProductId = pid);
    String toastMsg = '購買失敗';
    bool success = false;
    int? selectedSkuId;
    try {
      final info = await ref
          .read(productRepositoryProvider)
          .getProductFullInfo(pid);
      final skus = info['skus'];
      final skuList = (skus is List)
          ? skus
                .whereType<Map>()
                .map((s) => Map<String, dynamic>.from(s))
                .toList()
          : <Map<String, dynamic>>[];
      if (skuList.isEmpty) {
        toastMsg = '此商品目前無可選規格';
      } else {
        int? skuId;
        for (final sku in skuList) {
          final available =
              int.tryParse(sku['stockQty']?.toString() ?? '') ?? 0;
          if (available > 0) {
            skuId = int.tryParse(sku['id']?.toString() ?? '');
            break;
          }
        }
        if (skuId == null) {
          toastMsg = '商品已售完';
        } else {
          final session = ref.read(sessionProvider).value;
          final guestId = hasUsableSession(session)
              ? null
              : await AuthStorage.guestCartId();
          await ref
              .read(cartRepositoryProvider)
              .addToCart(skuId, 1, guestId: guestId);
          ref.invalidate(cartCountProvider);
          selectedSkuId = skuId;
          success = true;
        }
      }
    } catch (e) {
      debugPrint('[MySubscription] buyNow error: $e');
      toastMsg = addToCartErrorMessage(e, fallback: toastMsg);
    } finally {
      if (mounted) {
        setState(() => _buyingProductId = null);
        if (success && selectedSkuId != null) {
          context.push('/cart', extra: {'preSelectedSkuId': selectedSkuId});
        } else {
          showFloatingToast(context, toastMsg, isSuccess: false);
        }
      }
    }
  }

  void _handleBack() {
    context.go('/?tab=4');
  }

  String get _pageTitle => switch (_tabIndex) {
    0 => '我的訂閱',
    1 => '我的收藏',
    _ => '瀏覽紀錄',
  };

  // 訂閱與收藏用 API 的 total_size（分頁只載入一部分）；瀏覽紀錄沒有總數，顯示已載入的筆數。
  int get _currentItemCount => switch (_tabIndex) {
    0 => _followingTotal,
    1 => _likesTotal,
    _ => _recentViews.length,
  };

  String get _sectionTitle => switch (_tabIndex) {
    0 => '你關注的店家',
    1 => '你的收藏牆',
    _ => '最近足跡',
  };

  String get _sectionDescription => switch (_tabIndex) {
    0 => '從熟悉的店家開始，接著探索今天的新鮮事。',
    1 => '收好喜歡的食材，想買的時候再回來。',
    _ => '沿著最近看過的商品，繼續剛才的探索。',
  };

  IconData get _sectionIcon => switch (_tabIndex) {
    0 => AppIcons.people,
    1 => AppIcons.favorite,
    _ => AppIcons.historyToggleOffOutlined,
  };

  EdgeInsets _listPadding(BuildContext context) {
    final width = MediaQuery.sizeOf(context).width;
    final horizontal = width > 760 ? (width - 720) / 2 : HkSpace.lg;
    return EdgeInsets.fromLTRB(horizontal, 0, horizontal, HkSpace.xxl);
  }

  Future<void> _refreshCurrentTab() async {
    if (_refreshingInPlace) return;
    setState(() => _refreshingInPlace = true);
    try {
      if (_tabIndex == 0) {
        await _loadFollowing();
      } else if (_tabIndex == 1) {
        await _loadLikes();
      } else {
        await _loadRecentViews();
      }
    } finally {
      if (mounted) setState(() => _refreshingInPlace = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return PopScope(
      canPop: false,
      onPopInvokedWithResult: (didPop, _) {
        if (!didPop) context.go('/?tab=4');
      },
      child: Scaffold(
        backgroundColor: Colors.transparent,
        appBar: buildCommonTitleAppBar(
          context,
          title: _pageTitle,
          onBack: _handleBack,
        ),
        body: SafeArea(
          top: false,
          child: Stack(
            children: [
              const Positioned.fill(child: _SubscriptionGlassBackdrop()),
              Column(
                children: [
                  _buildTabSelector(),
                  const SizedBox(height: HkSpace.sm),
                  // Persistent shell tabs swap in place without a page-like
                  // fade; item-level feedback animations remain local.
                  Expanded(
                    child: KeyedSubtree(
                      key: ValueKey('subscription-tab-content-$_tabIndex'),
                      child: _buildContent(),
                    ),
                  ),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildTabSelector() {
    return HkGlassTabBar(
      labels: const ['訂閱', '收藏', '瀏覽'],
      selectedIndex: _tabIndex,
      labelKeyPrefix: 'subscription_tab_label_',
      onTap: (index) {
        if (index == _tabIndex) return;
        setState(() => _tabIndex = index);
        if (index == 0 && _following.isEmpty && !_loadingFollowing) {
          _loadFollowing();
        } else if (index == 1 && _likes.isEmpty && !_loadingLikes) {
          _loadLikes();
        } else if (index == 2 && _recentViews.isEmpty && !_loadingRecent) {
          _loadRecentViews();
        }
      },
    );
  }

  Widget _buildContent() => switch (_tabIndex) {
    0 => _buildFollowingList(),
    1 => _buildLikesList(),
    _ => _buildRecentViewsList(),
  };

  Widget _buildSectionIntro() {
    return Padding(
      key: Key('subscription_section_header_$_tabIndex'),
      padding: const EdgeInsets.only(top: HkSpace.lg, bottom: HkSpace.lg),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          HkFrostedCardSurface(
            radius: HkRadius.pill,
            shadows: HkShadows.sm,
            blurSigma: HkGlass.tabIndicatorBlurSigma,
            child: SizedBox.square(
              dimension: HkControl.minTap,
              child: Icon(_sectionIcon, color: AppIconColors.active, size: 22),
            ),
          ),
          const SizedBox(width: HkSpace.md),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Flexible(
                      child: Text(
                        _sectionTitle,
                        style: AppTypography.sectionTitle,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                      ),
                    ),
                    const SizedBox(width: HkSpace.sm),
                    HkFrostedCardSurface(
                      radius: HkRadius.pill,
                      shadows: HkShadows.xs,
                      blurSigma: HkGlass.tabIndicatorBlurSigma,
                      child: Padding(
                        padding: const EdgeInsets.symmetric(
                          horizontal: HkSpace.sm,
                          vertical: HkSpace.xs,
                        ),
                        child: Text(
                          '$_currentItemCount',
                          style: AppTypography.caption.copyWith(
                            color: AppColors.primary,
                            fontWeight: FontWeight.w700,
                          ),
                        ),
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: HkSpace.xs),
                Text(
                  _sectionDescription,
                  style: AppTypography.metaMuted,
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildEmptyState({
    required HkIconName icon,
    required String title,
    required String description,
  }) {
    final padding = _listPadding(context);
    return FruitRefreshIndicator(
      onRefresh: _refreshCurrentTab,
      child: CustomScrollView(
        physics: const AlwaysScrollableScrollPhysics(),
        slivers: [
          SliverPadding(
            padding: EdgeInsets.only(left: padding.left, right: padding.right),
            sliver: SliverToBoxAdapter(child: _buildSectionIntro()),
          ),
          SliverFillRemaining(
            hasScrollBody: false,
            child: Padding(
              padding: const EdgeInsets.fromLTRB(
                HkSpace.xxl,
                HkSpace.section,
                HkSpace.xxl,
                HkSpace.sectionLarge,
              ),
              child: Column(
                mainAxisAlignment: MainAxisAlignment.center,
                children: [
                  HkIcon(icon, size: 80),
                  const SizedBox(height: HkSpace.md),
                  Text(title, style: AppTypography.sectionTitle),
                  const SizedBox(height: HkSpace.xs),
                  Text(
                    description,
                    textAlign: TextAlign.center,
                    style: AppTypography.metaMuted,
                  ),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildFollowingList() {
    if (_showSkeletonFollowing && !_refreshingInPlace) {
      return const _FollowingSkeletonList(count: 5);
    }
    if (_loadingFollowing && !_refreshingInPlace) {
      return const SizedBox.shrink();
    }
    if (_following.isEmpty) {
      return _buildEmptyState(
        icon: HkIconName.emptySubscriptions,
        title: '還沒有訂閱店家',
        description: '遇到喜歡的店家時按下訂閱，之後就能從這裡快速回訪。',
      );
    }
    return FruitRefreshListView(
      padding: _listPadding(context),
      onRefresh: _refreshCurrentTab,
      onLoadMore: _loadMoreFollowing,
      hasMore: _followingPageToken.isNotEmpty,
      itemCount: _following.length + 1,
      itemBuilder: (context, index) {
        if (index == 0) return _buildSectionIntro();
        final itemIndex = index - 1;
        return _buildFollowingCard(_following[itemIndex], itemIndex);
      },
    );
  }

  Widget _buildFollowingCard(Map<String, dynamic> item, int index) {
    final name = (item['name'] ?? '').toString().trim();
    final avatar = (item['avatar'] ?? '').toString().trim();
    final address = (item['address'] ?? '').toString().trim();
    final rating = _parseRatingValue(item['rating']);
    final reviewCount = _parseIntValue(item['reviewCount']);
    final merchantId = (item['merchantId'] ?? '').toString().trim();
    final merchantName = name.isNotEmpty ? name : closedStoreName;
    final storeCode = (item['storeCode'] ?? '').toString().trim();
    final statusLabel = (item['statusLabel'] ?? '').toString().trim();
    final canOpenMerchant = storeCode.isNotEmpty;
    final VoidCallback? onOpenMerchant = canOpenMerchant
        ? () => context.push('/merchant/$storeCode')
        : null;

    return Padding(
      padding: const EdgeInsets.only(bottom: HkSpace.lg),
      child: HkFrostedCardSurface(
        key: ValueKey<String>('following_$merchantId'),
        child: Material(
          type: MaterialType.transparency,
          child: Semantics(
            container: true,
            explicitChildNodes: true,
            button: true,
            enabled: canOpenMerchant,
            label: '查看 $merchantName 店家',
            onTap: onOpenMerchant,
            child: InkWell(
              excludeFromSemantics: true,
              onTap: onOpenMerchant,
              child: Padding(
                padding: const EdgeInsets.all(HkSpace.md),
                child: Row(
                  children: [
                    Stack(
                      clipBehavior: Clip.none,
                      children: [
                        Container(
                          width: 56,
                          height: 56,
                          padding: const EdgeInsets.all(2),
                          decoration: BoxDecoration(
                            shape: BoxShape.circle,
                            border: Border.all(color: AppColors.primaryLight),
                          ),
                          child: ClipOval(child: _buildAvatar(avatar)),
                        ),
                        Positioned(
                          right: 1,
                          bottom: 1,
                          child: Container(
                            width: 14,
                            height: 14,
                            decoration: BoxDecoration(
                              color: AppColors.success,
                              shape: BoxShape.circle,
                              border: Border.all(
                                color: AppColors.bgCard,
                                width: 2,
                              ),
                            ),
                          ),
                        ),
                      ],
                    ),
                    const SizedBox(width: HkSpace.md),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            merchantName,
                            style: AppTypography.productName.copyWith(
                              color: AppColors.textEmphasis,
                            ),
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                          ),
                          if (statusLabel.isNotEmpty) ...[
                            const SizedBox(height: HkSpace.xs),
                            Text(statusLabel, style: AppTypography.metaMuted),
                          ],
                          const SizedBox(height: HkSpace.xs),
                          Row(
                            children: [
                              HawkrLocationIcon(
                                size: 15,
                                color: AppIconColors.muted,
                              ),
                              const SizedBox(width: HkSpace.xs),
                              Expanded(
                                child: Text(
                                  address.isNotEmpty ? address : '在地店家',
                                  style: AppTypography.metaMuted,
                                  maxLines: 1,
                                  overflow: TextOverflow.ellipsis,
                                ),
                              ),
                            ],
                          ),
                          const SizedBox(height: HkSpace.xs),
                          Row(
                            children: [
                              const Icon(
                                AppIcons.star,
                                size: 15,
                                color: AppIconColors.gold,
                              ),
                              const SizedBox(width: HkSpace.xs),
                              Expanded(
                                child: Text(
                                  rating > 0
                                      ? '${rating.toStringAsFixed(1)} · $reviewCount 則評價'
                                      : '尚無評價',
                                  style: AppTypography.caption,
                                  maxLines: 1,
                                  overflow: TextOverflow.ellipsis,
                                ),
                              ),
                            ],
                          ),
                        ],
                      ),
                    ),
                    const SizedBox(width: HkSpace.sm),
                    Semantics(
                      container: true,
                      button: true,
                      label: '取消訂閱 $merchantName',
                      child: ExcludeSemantics(
                        child: OutlinedButton.icon(
                          onPressed: () => _unfollowAt(index),
                          icon: const Icon(AppIcons.check, size: 16),
                          label: const HkButtonLabel('已訂閱'),
                          style: OutlinedButton.styleFrom(
                            minimumSize: const Size(0, HkControl.minTap),
                            padding: const EdgeInsets.symmetric(
                              horizontal: HkSpace.md,
                            ),
                            foregroundColor: AppColors.primary,
                            backgroundColor: AppColors.primarySubtle,
                            side: const BorderSide(color: AppColors.borderSoft),
                            shape: const StadiumBorder(),
                            textStyle: AppTypography.meta.copyWith(
                              fontWeight: FontWeight.w700,
                            ),
                          ),
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
    );
  }

  Widget _buildAvatar(String avatarUrl) {
    final url = avatarUrl.trim();
    // 後端回的是裸 objectKey（49 §6.1，例 `public/user_avatar/…png`），不是 http
    // URL。原本拿 `startsWith('http')` 當有效性判準 → objectKey 一律被判無效而
    // 落到預設圖。下方 AppCachedImage 帶了 scene，會自己走 ImageConfig.urlFor
    // 把 objectKey / 完整 URL 都組成顯示 URL，這裡只需擋空字串。
    if (url.isEmpty) {
      return const HkPlaceholder(HkPlaceholderKind.merchantAvatar);
    }
    return AppCachedImage(
      imageUrl: url,
      fit: BoxFit.cover,
      width: double.infinity,
      height: double.infinity,
      scene: ImageScene.merchantAvatar,
    );
  }

  Widget _buildLikesList() {
    if (_showSkeletonLikes && !_refreshingInPlace) {
      return const _FollowingSkeletonList(count: 4, productCards: true);
    }
    if (_loadingLikes && !_refreshingInPlace) {
      return const SizedBox.shrink();
    }
    if (_likes.isEmpty) {
      return _buildEmptyState(
        icon: HkIconName.emptyFavorites,
        title: '收藏牆還是空的',
        description: '看到心動商品就按下愛心，慢慢整理出自己的新鮮選物。',
      );
    }
    return FruitRefreshListView(
      padding: _listPadding(context),
      onRefresh: _refreshCurrentTab,
      onLoadMore: _loadMoreLikes,
      hasMore: _likesPageToken.isNotEmpty,
      itemCount: _likes.length + 1,
      itemBuilder: (context, index) {
        if (index == 0) return _buildSectionIntro();
        return _buildSocialProductCard(_likes[index - 1]);
      },
    );
  }

  Widget _buildRecentViewsList() {
    if (_showSkeletonRecent && !_refreshingInPlace) {
      return const _FollowingSkeletonList(count: 4, productCards: true);
    }
    if (_loadingRecent && !_refreshingInPlace) {
      return const SizedBox.shrink();
    }
    if (_recentViews.isEmpty) {
      return _buildEmptyState(
        icon: HkIconName.emptyRecentViews,
        title: '還沒有最近足跡',
        description: '逛過的商品會留在這裡，方便你稍後接著看。',
      );
    }
    return FruitRefreshListView(
      padding: _listPadding(context),
      onRefresh: _refreshCurrentTab,
      onLoadMore: _loadMoreRecentViews,
      hasMore: _recentPageToken.isNotEmpty,
      itemCount: _recentViews.length + 1,
      itemBuilder: (context, index) {
        if (index == 0) return _buildSectionIntro();
        return _buildSocialProductCard(
          _recentViews[index - 1],
          fromHistory: true,
        );
      },
    );
  }

  String _priceDisplay(Map<String, dynamic> item) {
    // 判準走 PriceDisplay：`"0"` 是後端「無價格」的表達，
    // `int.tryParse("0")` 拿到 0 會印成 `NT$ 0`。
    final label = PriceDisplay.formatPriceRange(
      item['minPrice'] ?? item['priceMin'],
      item['maxPrice'] ?? item['priceMax'],
      prefix: 'NT\$ ',
    );
    return label.isEmpty ? '價格請見詳情' : label;
  }

  DateTime? _activityTime(Map<String, dynamic> item) {
    for (final key in const ['visitedAt', 'createdAt']) {
      final value = item[key];
      if (value == null) continue;
      if (value is num) {
        final milliseconds = value > 1000000000000
            ? value.toInt()
            : value.toInt() * 1000;
        return DateTime.fromMillisecondsSinceEpoch(milliseconds).toLocal();
      }
      final raw = value.toString().trim();
      final numeric = int.tryParse(raw);
      if (numeric != null) {
        final milliseconds = numeric > 1000000000000 ? numeric : numeric * 1000;
        return DateTime.fromMillisecondsSinceEpoch(milliseconds).toLocal();
      }
      final parsed = DateTime.tryParse(raw);
      if (parsed != null) return parsed.toLocal();
    }
    return null;
  }

  String _historyLabel(Map<String, dynamic> item) {
    final time = _activityTime(item);
    if (time == null) return '最近看過';
    final difference = DateTime.now().difference(time);
    if (difference.isNegative || difference.inMinutes < 1) return '剛剛看過';
    if (difference.inHours < 1) return '${difference.inMinutes} 分鐘前看過';
    if (difference.inDays < 1) return '${difference.inHours} 小時前看過';
    if (difference.inDays == 1) return '昨天看過';
    if (difference.inDays < 7) return '${difference.inDays} 天前看過';
    return '${time.month}/${time.day} 看過';
  }

  void _openMerchant(String storeCode) {
    final code = storeCode.trim();
    if (code.isNotEmpty) context.push('/merchant/$code');
  }

  void _openProduct(Map<String, dynamic> item) {
    final merchantId = (item['sellerId'] ?? '').toString().trim();
    final productId = (item['productId'] ?? '').toString().trim();
    final productName = (item['productName'] ?? '').toString().trim();
    final merchantName = (item['storeName'] ?? '').toString().trim();
    final imageUrl =
        (item['imageKey'] ?? item['imageUrl'] ?? item['productImage'] ?? '')
            .toString()
            .trim();
    final priceValue = item['minPrice'] ?? item['priceMin'];
    final price = priceValue is num
        ? priceValue
        : num.tryParse(priceValue?.toString() ?? '') ?? 0;
    context
        .push<Map<String, dynamic>>(
          '/merchant-product',
          extra: MerchantItemDetailScreen(
            title: productName,
            imageUrl: imageUrl,
            price: price,
            skuId: productId,
            merchantName: merchantName,
            merchantId: merchantId,
            onResultChanged: _handleFavoriteResult,
          ),
        )
        .then(_handleFavoriteResult);
  }

  void _openAddToCart(Map<String, dynamic> item) {
    final productId = (item['productId'] ?? '').toString().trim();
    if (productId.isEmpty) return;
    final priceValue = item['minPrice'] ?? item['priceMin'];
    final price = priceValue is num
        ? priceValue.toDouble()
        : double.tryParse(priceValue?.toString() ?? '');
    showProductAddToCartSheet(
      context,
      ref,
      skuId: productId,
      productName: (item['productName'] ?? '').toString(),
      imageUrl:
          (item['imageKey'] ?? item['imageUrl'] ?? item['productImage'] ?? '')
              .toString(),
      initialPrice: price,
    );
  }

  Widget _buildFavoriteAction(String productId) {
    final id = productId.trim();
    final isFavorite = _favoriteIds.contains(id);
    final reduceMotion =
        MediaQuery.maybeOf(context)?.disableAnimations ?? false;
    return Semantics(
      button: true,
      label: isFavorite ? '取消收藏' : '加入收藏',
      child: ExcludeSemantics(
        child: TextButton.icon(
          onPressed: id.isEmpty ? null : () => _toggleFavorite(id),
          icon: AnimatedSwitcher(
            duration: reduceMotion ? Duration.zero : HkMotion.duration,
            transitionBuilder: (child, animation) =>
                ScaleTransition(scale: animation, child: child),
            child: Icon(
              isFavorite ? AppIcons.favorite : AppIcons.favoriteBorder,
              key: ValueKey<bool>(isFavorite),
              size: 20,
              color: isFavorite ? AppIconColors.favorite : AppIconColors.muted,
            ),
          ),
          label: HkButtonLabel(isFavorite ? '已收藏' : '收藏'),
          style: TextButton.styleFrom(
            minimumSize: const Size(0, HkControl.minTap),
            padding: const EdgeInsets.symmetric(horizontal: HkSpace.sm),
            foregroundColor: isFavorite
                ? AppColors.favoriteActive
                : AppColors.textSecondary,
            textStyle: AppTypography.meta.copyWith(fontWeight: FontWeight.w700),
          ),
        ),
      ),
    );
  }

  Widget _buildCommerceActionButton({
    required Key key,
    required Key iconKey,
    required String label,
    required HkIconName icon,
    required VoidCallback? onPressed,
    required Color backgroundColor,
    required Color foregroundColor,
    bool loading = false,
  }) {
    return FilledButton(
      key: key,
      onPressed: onPressed,
      style: FilledButton.styleFrom(
        minimumSize: const Size(0, HkControl.minTap),
        padding: const EdgeInsets.symmetric(horizontal: HkSpace.xs),
        backgroundColor: backgroundColor,
        foregroundColor: foregroundColor,
        disabledBackgroundColor: backgroundColor.withValues(alpha: 0.45),
        disabledForegroundColor: foregroundColor.withValues(alpha: 0.75),
        shape: const StadiumBorder(),
        textStyle: AppTypography.meta.copyWith(fontWeight: FontWeight.w700),
      ),
      child: loading
          ? SizedBox.square(
              key: ValueKey<String>(
                'subscription-product-${icon.name}-loading',
              ),
              dimension: 18,
              child: HkBrandLoader(
                tone: HkBrandLoaderTone.mono,
                color: foregroundColor,
                semanticsLabel: '$label，處理中',
              ),
            )
          : Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                HkIcon(
                  icon,
                  key: iconKey,
                  size: 18,
                  color: onPressed == null
                      ? foregroundColor.withValues(alpha: 0.75)
                      : foregroundColor,
                ),
                const SizedBox(width: HkSpace.xs),
                Flexible(
                  child: HkButtonLabel(
                    label,
                    style: const TextStyle(height: 1),
                  ),
                ),
              ],
            ),
    );
  }

  Widget _buildSocialProductCard(
    Map<String, dynamic> item, {
    bool fromHistory = false,
  }) {
    final storeCode = (item['storeCode'] ?? '').toString().trim();
    final productId = (item['productId'] ?? '').toString().trim();
    final merchantName = (item['storeName'] ?? '').toString().trim();
    final sellerAvatar = (item['sellerAvatar'] ?? '').toString().trim();
    final productName = (item['productName'] ?? '').toString().trim();
    final imageUrl =
        (item['imageKey'] ?? item['imageUrl'] ?? item['productImage'] ?? '')
            .toString()
            .trim();
    final distance = (item['distance'] ?? '').toString().trim();
    final rating = _parseRatingValue(item['rating'] ?? item['ratingX10']);
    final reviewCount = _reviewCountFromItem(item);
    final displayMerchant = merchantName.isNotEmpty ? merchantName : 'Hawkr 店家';
    final contextLabel = fromHistory ? _historyLabel(item) : '收藏商品';

    return Padding(
      padding: const EdgeInsets.only(bottom: HkSpace.lg),
      child: HkFrostedCardSurface(
        key: ValueKey<String>(
          '${fromHistory ? 'history' : 'favorite'}_$productId',
        ),
        child: Material(
          type: MaterialType.transparency,
          child: Column(
            children: [
              InkWell(
                onTap: storeCode.isEmpty
                    ? null
                    : () => _openMerchant(storeCode),
                child: Padding(
                  padding: const EdgeInsets.fromLTRB(
                    HkSpace.md,
                    HkSpace.md,
                    HkSpace.sm,
                    HkSpace.sm,
                  ),
                  child: Row(
                    children: [
                      Container(
                        width: 38,
                        height: 38,
                        padding: const EdgeInsets.all(1.5),
                        decoration: BoxDecoration(
                          shape: BoxShape.circle,
                          border: Border.all(color: AppColors.primaryLight),
                        ),
                        child: ClipOval(child: _buildAvatar(sellerAvatar)),
                      ),
                      const SizedBox(width: HkSpace.sm),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(
                              displayMerchant,
                              style: AppTypography.meta.copyWith(
                                color: AppColors.textEmphasis,
                                fontWeight: FontWeight.w700,
                              ),
                              maxLines: 1,
                              overflow: TextOverflow.ellipsis,
                            ),
                            Text(
                              distance.isEmpty
                                  ? contextLabel
                                  : '$contextLabel · $distance',
                              style: AppTypography.caption,
                              maxLines: 1,
                              overflow: TextOverflow.ellipsis,
                            ),
                          ],
                        ),
                      ),
                      const Icon(
                        AppIcons.chevronRight,
                        size: 18,
                        color: AppIconColors.muted,
                      ),
                    ],
                  ),
                ),
              ),
              InkWell(
                onTap: () => _openProduct(item),
                child: Padding(
                  padding: const EdgeInsets.fromLTRB(
                    HkSpace.md,
                    HkSpace.xs,
                    HkSpace.md,
                    HkSpace.md,
                  ),
                  child: LayoutBuilder(
                    builder: (context, constraints) {
                      final imageWidth = constraints.maxWidth < 340
                          ? 112.0
                          : 128.0;
                      return Row(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          SizedBox(
                            key: ValueKey<String>(
                              'subscription-product-image-$productId',
                            ),
                            width: imageWidth,
                            child: AspectRatio(
                              aspectRatio: 4 / 3,
                              child: ClipRRect(
                                borderRadius: BorderRadius.circular(
                                  HkRadius.md,
                                ),
                                child: ColoredBox(
                                  color: AppColors.surfaceMuted,
                                  child: imageUrl.isNotEmpty
                                      ? AppCachedImage(
                                          imageUrl: imageUrl,
                                          fit: BoxFit.cover,
                                          width: double.infinity,
                                          height: double.infinity,
                                          scene:
                                              ImageScene.productLandscapeCard,
                                        )
                                      : const Center(
                                          child: Icon(
                                            AppIcons.image,
                                            color: AppIconColors.muted,
                                            size: 32,
                                          ),
                                        ),
                                ),
                              ),
                            ),
                          ),
                          const SizedBox(width: HkSpace.md),
                          Expanded(
                            child: Column(
                              mainAxisSize: MainAxisSize.min,
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                Text(
                                  productName.isNotEmpty ? productName : '商品名稱',
                                  style: AppTypography.productName.copyWith(
                                    color: AppColors.textEmphasis,
                                  ),
                                  maxLines: 2,
                                  overflow: TextOverflow.ellipsis,
                                ),
                                const SizedBox(height: HkSpace.sm),
                                Row(
                                  children: [
                                    const Icon(
                                      AppIcons.star,
                                      size: 15,
                                      color: AppIconColors.gold,
                                    ),
                                    const SizedBox(width: HkSpace.xs),
                                    Expanded(
                                      child: Text(
                                        rating > 0
                                            ? '${rating.toStringAsFixed(1)}${reviewCount > 0 ? ' · $reviewCount 則' : ''}'
                                            : '尚無評價',
                                        style: AppTypography.caption,
                                        maxLines: 1,
                                        overflow: TextOverflow.ellipsis,
                                      ),
                                    ),
                                  ],
                                ),
                                const SizedBox(height: HkSpace.sm),
                                Text(
                                  _priceDisplay(item),
                                  style: AppTypography.priceTotal,
                                  maxLines: 1,
                                  overflow: TextOverflow.ellipsis,
                                ),
                              ],
                            ),
                          ),
                        ],
                      );
                    },
                  ),
                ),
              ),
              Padding(
                padding: const EdgeInsets.fromLTRB(
                  HkSpace.xs,
                  0,
                  HkSpace.xs,
                  HkSpace.xs,
                ),
                child: HkFrostedCardSurface(
                  key: ValueKey<String>(
                    'subscription-product-actions-$productId',
                  ),
                  radius: HkRadius.lg,
                  shadows: HkShadows.xs,
                  blurSigma: HkGlass.tabIndicatorBlurSigma,
                  child: Row(
                    children: [
                      Expanded(child: _buildFavoriteAction(productId)),
                      const SizedBox(width: HkSpace.xs),
                      Expanded(
                        child: _buildCommerceActionButton(
                          key: const ValueKey('subscription-product-add'),
                          iconKey: const ValueKey(
                            'subscription-product-add-icon',
                          ),
                          label: '加入購物車',
                          icon: HkIconName.cart,
                          onPressed: _buyingProductId != null
                              ? null
                              : () => _openAddToCart(item),
                          backgroundColor: AppColors.commerceCartAction,
                          foregroundColor: AppColors.commerceCartForeground,
                        ),
                      ),
                      const SizedBox(width: HkSpace.xs),
                      Expanded(
                        child: _buildCommerceActionButton(
                          key: const ValueKey('subscription-product-buy'),
                          iconKey: const ValueKey(
                            'subscription-product-buy-icon',
                          ),
                          label: '立即結帳',
                          icon: HkIconName.checkoutCoins,
                          onPressed: _buyingProductId != null
                              ? null
                              : () => _handleBuyNow(productId),
                          backgroundColor: AppColors.commerceCheckoutAction,
                          foregroundColor: AppColors.commerceCheckoutForeground,
                          loading: _buyingProductId == productId,
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
    );
  }
}
