import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/core/theme/app_typography.dart';
import 'package:hawkr_seller/core/theme/hk_tokens.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:hawkr_seller/shared/widgets/hk_icon.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../config/image_config.dart';
import '../../../shared/widgets/hk_empty_state.dart';
import '../../../shared/widgets/hk_choice_chip.dart';
import '../../../shared/widgets/hk_frosted_card_surface.dart';
import '../../../shared/widgets/hk_glass_app_bar.dart';
import '../../../shared/widgets/optimized_image.dart';
import '../../../shared/widgets/truck_loading_overlay.dart';
import '../../media/data/media_repository.dart';
import '../data/product_repository.dart';
import 'widgets/review_image_gallery.dart';

/// 賣家商品評論頁：與買家端共用評分、篩選與實拍圖片閱讀層級。
class ProductReviewsScreen extends ConsumerStatefulWidget {
  const ProductReviewsScreen({
    super.key,
    required this.sellerId,
    required this.skuId,
    required this.productName,
  });

  final String sellerId;
  final String skuId;
  final String productName;

  @override
  ConsumerState<ProductReviewsScreen> createState() =>
      _ProductReviewsScreenState();
}

class _ProductReviewsScreenState extends ConsumerState<ProductReviewsScreen> {
  List<Map<String, dynamic>> _reviews = [];
  bool _isLoading = true;
  String _selectedFilter = '全部';
  int _totalCount = 0;
  double _avgRating = 0;
  Map<int, int> _ratingDistribution = {5: 0, 4: 0, 3: 0, 2: 0, 1: 0};

  /// `ReviewListItem.imageKeys`（gateway/internal/handler/review.go）是
  /// **objectKey 字串陣列**，元素不是物件——曾經那串 key/imageKey/url/... 的候選
  /// 清單在任何評論端點都不存在。
  static String? _reviewImageUrl(dynamic value) {
    final source = value?.toString().trim() ?? '';
    return source.isEmpty ? null : MediaRepository.resolveUrl(source);
  }

  @override
  void initState() {
    super.initState();
    _loadReviews();
  }

  Future<void> _loadReviews() async {
    setState(() => _isLoading = true);
    try {
      final list = await ref
          .read(productRepositoryProvider)
          .getSellerProductReviewsPublic(widget.sellerId, widget.skuId);
      final targetSkuId = widget.skuId.trim();
      final filteredList = list.where((review) {
        // `ReviewListItem` 只有 productId（`GET /public/products/{id}/reviews`
        // 本來就是照商品撈的，回應裡沒有賣家欄位）。
        final reviewSkuId = review['productId']?.toString().trim() ?? '';
        return reviewSkuId.isEmpty || reviewSkuId == targetSkuId;
      }).toList();

      var ratedCount = 0;
      var ratingSum = 0.0;
      final distribution = <int, int>{5: 0, 4: 0, 3: 0, 2: 0, 1: 0};
      for (final review in filteredList) {
        final raw = review['rating'] ?? 0;
        final rating = raw is num
            ? raw.toDouble()
            : double.tryParse('$raw') ?? 0;
        if (rating <= 0) continue;
        ratedCount++;
        ratingSum += rating;
        final star = rating.round().clamp(1, 5);
        distribution[star] = (distribution[star] ?? 0) + 1;
      }

      if (!mounted) return;
      setState(() {
        _reviews = filteredList;
        _totalCount = filteredList.length;
        _avgRating = ratedCount > 0 ? ratingSum / ratedCount : 0;
        _ratingDistribution = distribution;
        _isLoading = false;
      });
    } catch (error, stackTrace) {
      debugPrint('[ProductReviewsScreen] $error\n$stackTrace');
      if (!mounted) return;
      setState(() {
        _reviews = [];
        _totalCount = 0;
        _avgRating = 0;
        _ratingDistribution = {5: 0, 4: 0, 3: 0, 2: 0, 1: 0};
        _isLoading = false;
      });
    }
  }

  List<Map<String, dynamic>> get _filteredReviews {
    if (_selectedFilter == '全部') return _reviews;
    if (_selectedFilter == '有圖片') {
      return _reviews
          .where((review) => _reviewImages(review).isNotEmpty)
          .toList();
    }
    final star = int.tryParse(_selectedFilter.replaceAll('星', ''));
    if (star == null) return _reviews;
    return _reviews.where((review) {
      final raw = review['rating'] ?? 0;
      final rating = raw is num ? raw.round() : int.tryParse('$raw') ?? 0;
      return rating == star;
    }).toList();
  }

  static List<String> _reviewImages(Map<String, dynamic> review) {
    final images = review['imageKeys'];
    if (images is! List) return const [];
    return images
        .map(_reviewImageUrl)
        .whereType<String>()
        .toList(growable: false);
  }

  int get _imageReviewCount =>
      _reviews.where((review) => _reviewImages(review).isNotEmpty).length;

  static String _formatDate(dynamic dateValue) {
    if (dateValue is! String) return '';
    try {
      final date = DateTime.parse(dateValue);
      return '${date.year}-${date.month.toString().padLeft(2, '0')}-${date.day.toString().padLeft(2, '0')}';
    } catch (_) {
      return '';
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: AppColors.bgPrimary,
      appBar: HkGlassAppBar(
        title: const Text('商品評價'),
        onBackPressed: () => Navigator.of(context).maybePop(),
      ),
      body: _isLoading
          ? const TruckLoadingOverlay()
          : ColoredBox(
              key: const ValueKey('seller-product-reviews-background'),
              color: AppColors.bgPrimary,
              child: LayoutBuilder(
                builder: (context, constraints) {
                  final gutter = HkBreakpoints.pageGutter(constraints.maxWidth);
                  return CustomScrollView(
                    key: const ValueKey('seller-product-reviews-scroll-view'),
                    slivers: [
                      _buildSectionSliver(
                        horizontalPadding: gutter,
                        topPadding: HkSpace.lg,
                        child: _buildRatingSummaryCard(),
                      ),
                      _buildSectionSliver(
                        horizontalPadding: gutter,
                        topPadding: HkSpace.lg,
                        child: _buildFilterButtons(),
                      ),
                      _buildSectionSliver(
                        horizontalPadding: gutter,
                        topPadding: HkSpace.sm,
                        child: _buildReviewListHeading(),
                      ),
                      _buildReviewsSliver(gutter),
                      SliverToBoxAdapter(
                        child: SizedBox(
                          height:
                              MediaQuery.paddingOf(context).bottom +
                              HkSpace.section,
                        ),
                      ),
                    ],
                  );
                },
              ),
            ),
    );
  }

  Widget _buildSectionSliver({
    required double horizontalPadding,
    required double topPadding,
    required Widget child,
  }) {
    return SliverPadding(
      padding: EdgeInsets.fromLTRB(
        horizontalPadding,
        topPadding,
        horizontalPadding,
        0,
      ),
      sliver: SliverToBoxAdapter(
        child: Align(
          alignment: Alignment.topCenter,
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 720),
            child: child,
          ),
        ),
      ),
    );
  }

  Widget _buildRatingSummaryCard() {
    return HkFrostedCardSurface(
      key: const ValueKey('seller-product-reviews-summary-card'),
      child: Padding(
        padding: const EdgeInsets.all(HkSpace.xl),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                const _ReviewSectionIcon(),
                const SizedBox(width: HkSpace.md),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text('整體評分', style: AppTypography.sectionTitle),
                      if (widget.productName.trim().isNotEmpty)
                        Text(
                          widget.productName.trim(),
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: AppTypography.metaMuted,
                        ),
                    ],
                  ),
                ),
              ],
            ),
            const SizedBox(height: HkSpace.lg),
            const Divider(height: 1, color: AppColors.dividerLight),
            const SizedBox(height: HkSpace.lg),
            LayoutBuilder(
              builder: (context, constraints) {
                final average = _AverageRatingBlock(
                  rating: _avgRating,
                  totalCount: _totalCount,
                );
                final distribution = _RatingDistribution(
                  distribution: _ratingDistribution,
                );
                if (constraints.maxWidth < 300) {
                  return Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      average,
                      const SizedBox(height: HkSpace.xl),
                      distribution,
                    ],
                  );
                }
                return Row(
                  crossAxisAlignment: CrossAxisAlignment.center,
                  children: [
                    SizedBox(width: 116, child: average),
                    const SizedBox(width: HkSpace.xl),
                    Expanded(child: distribution),
                  ],
                );
              },
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildFilterButtons() {
    final filters = <(String, int)>[
      ('全部', _totalCount),
      ('5星', _ratingDistribution[5] ?? 0),
      ('4星', _ratingDistribution[4] ?? 0),
      ('3星', _ratingDistribution[3] ?? 0),
      ('2星', _ratingDistribution[2] ?? 0),
      ('1星', _ratingDistribution[1] ?? 0),
      ('有圖片', _imageReviewCount),
    ];
    return Column(
      key: const ValueKey('seller-product-reviews-filter-section'),
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('篩選評價', style: AppTypography.sectionTitle),
        const SizedBox(height: HkSpace.sm),
        SingleChildScrollView(
          scrollDirection: Axis.horizontal,
          child: Row(
            children: [
              for (var index = 0; index < filters.length; index++) ...[
                if (index > 0) const SizedBox(width: HkSpace.sm),
                HkChoiceChip(
                  key: ValueKey(
                    'seller-product-review-filter-${filters[index].$1}',
                  ),
                  label: '${filters[index].$1} (${filters[index].$2})',
                  selected: _selectedFilter == filters[index].$1,
                  onTap: () {
                    setState(() => _selectedFilter = filters[index].$1);
                  },
                ),
              ],
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildReviewListHeading() {
    final reviews = _filteredReviews;
    return Row(
      children: [
        Expanded(
          child: Text(
            '顧客回饋',
            key: const ValueKey('seller-product-reviews-list-heading'),
            style: AppTypography.sectionTitle,
          ),
        ),
        Text('${reviews.length} 則', style: AppTypography.metaMuted),
      ],
    );
  }

  Widget _buildReviewsSliver(double gutter) {
    final reviews = _filteredReviews;
    if (reviews.isEmpty) {
      return SliverPadding(
        padding: EdgeInsets.fromLTRB(gutter, HkSpace.lg, gutter, HkSpace.xxl),
        sliver: const SliverToBoxAdapter(
          child: SizedBox(
            height: 180,
            child: HkEmptyState(
              key: ValueKey('seller-product-reviews-empty-state'),
              iconKey: ValueKey('seller-product-reviews-empty-state-icon'),
              icon: HkIconName.star,
              message: '暫無符合條件的評論',
              description: '買家完成評價後會顯示在這裡。',
              topPadding: HkSpace.lg,
              iconDimension: HkEmptyState.compactIconSize,
              haloDimension: HkEmptyState.compactHaloSize,
            ),
          ),
        ),
      );
    }

    return SliverPadding(
      padding: EdgeInsets.fromLTRB(gutter, HkSpace.md, gutter, 0),
      sliver: SliverList.builder(
        itemCount: reviews.length,
        itemBuilder: (context, index) {
          return Align(
            alignment: Alignment.topCenter,
            child: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 720),
              child: Padding(
                padding: const EdgeInsets.only(bottom: HkSpace.md),
                child: _buildReviewItem(reviews[index]),
              ),
            ),
          );
        },
      ),
    );
  }

  Widget _buildUserAvatar(String source) {
    if (source.isNotEmpty) {
      return ClipOval(
        child: SizedBox.square(
          dimension: 40,
          child: OptimizedNetworkImage(
            imageUrl: MediaRepository.resolveUrl(source),
            scene: ImageScene.avatarSmall,
            fit: BoxFit.cover,
            errorWidget: _defaultAvatar(),
          ),
        ),
      );
    }
    return _defaultAvatar();
  }

  Widget _defaultAvatar() {
    return SizedBox.square(
      dimension: 40,
      child: DecoratedBox(
        decoration: const BoxDecoration(
          color: AppColors.storefrontMutedSurface,
          shape: BoxShape.circle,
          border: Border.fromBorderSide(
            BorderSide(color: AppColors.borderSoft),
          ),
        ),
        child: const Center(
          child: Icon(
            AppIcons.personRounded,
            size: 22,
            color: AppIconColors.secondary,
          ),
        ),
      ),
    );
  }

  Widget _buildReviewItem(Map<String, dynamic> review) {
    final buyerId = (review['buyerId'] ?? '').toString().trim();
    final username = (review['username'] ?? '').toString().trim();
    // `GET /public/products/{id}/reviews` 契約成對回 `avatarKey` + `avatarUrl`
    // （gateway review.go `ReviewListItem`），SSOT 是 key。舊欄位
    // `userAvatarKey` / `userAvatarUrl` / `userAvatar` 皆已不存在。
    final avatar = (review['avatarKey'] ?? review['avatarUrl'] ?? '')
        .toString()
        .trim();
    final displayName = username.isNotEmpty
        ? username
        : (buyerId.length >= 4
              ? '會員 ${buyerId.substring(buyerId.length - 4)}'
              : (buyerId.isNotEmpty ? '會員 $buyerId' : '匿名會員'));
    final content = (review['content'] ?? '').toString();
    final date = _formatDate(review['createdAt']);
    final rating = int.tryParse('${review['rating'] ?? ''}')?.clamp(0, 5) ?? 0;
    final images = _reviewImages(review);
    final reviewKey = (review['reviewId'] ?? review['id'] ?? '$buyerId-$date')
        .toString();

    return HkFrostedCardSurface(
      key: ValueKey('seller-product-review-card-$reviewKey'),
      child: Padding(
        padding: const EdgeInsets.all(HkSpace.lg),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                _buildUserAvatar(avatar),
                const SizedBox(width: HkSpace.md),
                Expanded(
                  child: Text(
                    displayName,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: AppTypography.bodyBold,
                  ),
                ),
                if (date.isNotEmpty) ...[
                  const SizedBox(width: HkSpace.sm),
                  Text(date, style: AppTypography.caption),
                ],
              ],
            ),
            const SizedBox(height: HkSpace.md),
            Row(
              children: [
                _RatingStars(rating: rating.toDouble(), size: 18),
                const SizedBox(width: HkSpace.sm),
                Text(
                  rating > 0 ? '$rating.0' : '尚未評分',
                  style: AppTypography.meta.copyWith(
                    fontWeight: FontWeight.w700,
                  ),
                ),
              ],
            ),
            const SizedBox(height: HkSpace.md),
            Text(
              content.isNotEmpty ? content : '這位顧客分享了實拍圖片。',
              style: content.isNotEmpty
                  ? AppTypography.body
                  : AppTypography.metaMuted,
            ),
            if (images.isNotEmpty) ...[
              const SizedBox(height: HkSpace.lg),
              Row(
                children: [
                  const Icon(
                    AppIcons.imageOutlined,
                    size: 18,
                    color: AppIconColors.secondary,
                  ),
                  const SizedBox(width: HkSpace.xs),
                  Text(
                    '${images.length} 張實拍 · 點擊放大',
                    style: AppTypography.metaMuted,
                  ),
                ],
              ),
              const SizedBox(height: HkSpace.sm),
              ReviewImageGallery(images: images),
            ],
          ],
        ),
      ),
    );
  }
}

class _ReviewSectionIcon extends StatelessWidget {
  const _ReviewSectionIcon();

  @override
  Widget build(BuildContext context) {
    return SizedBox.square(
      dimension: HkControl.minTap,
      child: DecoratedBox(
        decoration: BoxDecoration(
          color: AppColors.brandSecondary.withValues(alpha: 0.12),
          shape: BoxShape.circle,
          border: Border.all(
            color: AppColors.brandSecondary.withValues(alpha: 0.32),
          ),
        ),
        child: const Icon(
          AppIcons.starRounded,
          color: AppIconColors.rating,
          size: HkControl.icon,
        ),
      ),
    );
  }
}

class _AverageRatingBlock extends StatelessWidget {
  const _AverageRatingBlock({required this.rating, required this.totalCount});

  final double rating;
  final int totalCount;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          rating > 0 ? rating.toStringAsFixed(1) : '—',
          key: const ValueKey('seller-product-reviews-average-rating'),
          style: AppTypography.display.copyWith(color: AppColors.textEmphasis),
        ),
        const SizedBox(height: HkSpace.xs),
        _RatingStars(rating: rating, size: 18),
        const SizedBox(height: HkSpace.sm),
        Text('共 $totalCount 則評價', style: AppTypography.metaMuted),
      ],
    );
  }
}

class _RatingDistribution extends StatelessWidget {
  const _RatingDistribution({required this.distribution});

  final Map<int, int> distribution;

  @override
  Widget build(BuildContext context) {
    final ratedCount = distribution.values.fold<int>(
      0,
      (sum, count) => sum + count,
    );
    return Column(
      children: [
        for (final star in const [5, 4, 3, 2, 1])
          Padding(
            padding: const EdgeInsets.symmetric(vertical: HkSpace.xs),
            child: Row(
              children: [
                SizedBox(
                  width: 16,
                  child: Text(
                    '$star',
                    textAlign: TextAlign.center,
                    style: AppTypography.caption.copyWith(
                      color: AppColors.textPrimary,
                    ),
                  ),
                ),
                const SizedBox(width: HkSpace.xs),
                const Icon(
                  AppIcons.starRounded,
                  size: 16,
                  color: AppIconColors.rating,
                ),
                const SizedBox(width: HkSpace.sm),
                Expanded(
                  child: ClipRRect(
                    borderRadius: BorderRadius.circular(HkRadius.pill),
                    child: LinearProgressIndicator(
                      value: ratedCount > 0
                          ? (distribution[star] ?? 0) / ratedCount
                          : 0,
                      minHeight: 6,
                      backgroundColor: AppColors.neutralWhisper,
                      valueColor: const AlwaysStoppedAnimation<Color>(
                        AppColors.storefrontRating,
                      ),
                    ),
                  ),
                ),
                const SizedBox(width: HkSpace.sm),
                SizedBox(
                  width: 24,
                  child: Text(
                    '${distribution[star] ?? 0}',
                    textAlign: TextAlign.end,
                    style: AppTypography.caption,
                  ),
                ),
              ],
            ),
          ),
      ],
    );
  }
}

class _RatingStars extends StatelessWidget {
  const _RatingStars({required this.rating, required this.size});

  final double rating;
  final double size;

  @override
  Widget build(BuildContext context) {
    final filledCount = rating.round().clamp(0, 5);
    return Semantics(
      label: '評分 ${rating.toStringAsFixed(1)} 分，滿分 5 分',
      excludeSemantics: true,
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          for (var index = 0; index < 5; index++) ...[
            if (index > 0) const SizedBox(width: HkSpace.xs),
            Icon(
              index < filledCount
                  ? AppIcons.starRounded
                  : AppIcons.starOutlineRounded,
              size: size,
              color: index < filledCount
                  ? AppIconColors.rating
                  : AppIconColors.disabled,
            ),
          ],
        ],
      ),
    );
  }
}
