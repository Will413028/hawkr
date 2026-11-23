import 'package:hawkr_api/taipei_time.dart';
import 'package:hawkr_seller/shared/time/taipei_display.dart';
import 'package:hawkr_seller/l10n/l10n.dart';
import 'dart:async' as async;

import 'package:hawkr_seller/shared/widgets/hk_button_label.dart';
import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/core/theme/app_typography.dart';
import 'package:hawkr_seller/core/theme/hk_tokens.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:hawkr_seller/shared/widgets/hk_icon.dart';
import 'package:hawkr_api/errors.dart' show errorText;
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../config/image_config.dart';
import '../../../shared/widgets/hk_empty_state.dart';
import '../../../shared/widgets/hk_choice_chip.dart';
import '../../../shared/widgets/hk_frosted_card_surface.dart';
import '../../../shared/widgets/hk_glass_app_bar.dart';
import '../../../shared/widgets/optimized_image.dart';
import '../../../shared/widgets/truck_loading_overlay.dart';
import '../../../core/l10n/ui_locale.dart';
import '../../../shared/utils/floating_toast.dart';
import '../../review/data/review_repository.dart';
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

/// `Future.wait` 的記錄版在任一邊失敗時丟 [ParallelWaitError]（錯誤包在 `errors` 裡）：取出第一個真正的錯誤。
Object _firstCause(Object raw) {
  if (raw is ParallelWaitError) {
    final errors = raw.errors;
    if (errors is (async.AsyncError?, async.AsyncError?)) {
      return (errors.$1 ?? errors.$2)?.error ?? raw;
    }
  }
  return raw;
}

class _ProductReviewsScreenState extends ConsumerState<ProductReviewsScreen> {
  List<ReviewItem> _reviews = [];
  bool _isLoading = true;
  Object? _loadError;
  bool _loadingMore = false;
  String _nextPageToken = '';
  String _selectedFilter = '全部';
  ReviewStats _stats = ReviewStats.fromProto(0, 0, const []);

  int get _totalCount => _stats.count;
  double get _avgRating => _stats.average;
  Map<int, int> get _ratingDistribution => _stats.distribution;

  @override
  void initState() {
    super.initState();
    _loadReviews();
  }

  /// 篩選對應的伺服器參數：星等與「有圖片」都由伺服器篩，不在已載入的頁上篩。
  (int?, bool) get _serverFilter {
    if (_selectedFilter == '全部') return (null, false);
    if (_selectedFilter == '有圖片') return (null, true);
    return (int.tryParse(_selectedFilter.replaceAll('星', '')), false);
  }

  Future<void> _loadReviews() async {
    setState(() {
      _isLoading = true;
      _loadError = null;
    });
    final repo = ref.read(reviewRepositoryProvider);
    final (rating, withImages) = _serverFilter;
    try {
      final (stats, page) = await (
        repo.productStats(widget.skuId.trim()),
        repo.listProductReviews(
          widget.skuId.trim(),
          rating: rating,
          withImages: withImages,
        ),
      ).wait;
      if (!mounted) return;
      setState(() {
        _stats = stats;
        _reviews = page.reviews;
        _nextPageToken = page.nextPageToken;
        _isLoading = false;
      });
    } catch (raw, stackTrace) {
      // 並行讀取任一邊失敗時 `.wait` 丟 ParallelWaitError：取出第一個真正的錯誤，錯誤文字才依原因顯示。
      final error = _firstCause(raw);
      debugPrint('[ProductReviewsScreen] $error\n$stackTrace');
      if (!mounted) return;
      setState(() {
        _loadError = error;
        _isLoading = false;
      });
    }
  }

  Future<void> _loadMore() async {
    if (_loadingMore || _nextPageToken.isEmpty) return;
    setState(() => _loadingMore = true);
    final (rating, withImages) = _serverFilter;
    try {
      final page = await ref
          .read(reviewRepositoryProvider)
          .listProductReviews(
            widget.skuId.trim(),
            pageToken: _nextPageToken,
            rating: rating,
            withImages: withImages,
          );
      if (!mounted) return;
      setState(() {
        _reviews = [..._reviews, ...page.reviews];
        _nextPageToken = page.nextPageToken;
        _loadingMore = false;
      });
    } catch (error) {
      if (!mounted) return;
      setState(() => _loadingMore = false);
      FloatingToast.error(
        context,
        errorText(error, defaults: ref.read(defaultErrorTextProvider)),
      );
    }
  }

  List<ReviewItem> get _filteredReviews => _reviews;

  static String _formatDate(DateTime? date, TaipeiDisplay t) {
    if (date == null) return '';
    final d = date.taipeiWall;
    return t.labeled(
      '${d.year}-${d.month.toString().padLeft(2, '0')}-${d.day.toString().padLeft(2, '0')}',
    );
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
                      if (_loadError == null)
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
    final filters = <(String, int?)>[
      ('全部', _totalCount),
      ('5星', _ratingDistribution[5] ?? 0),
      ('4星', _ratingDistribution[4] ?? 0),
      ('3星', _ratingDistribution[3] ?? 0),
      ('2星', _ratingDistribution[2] ?? 0),
      ('1星', _ratingDistribution[1] ?? 0),
      ('有圖片', null),
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
                  label: filters[index].$2 == null
                      ? filters[index].$1
                      : '${filters[index].$1} (${filters[index].$2})',
                  selected: _selectedFilter == filters[index].$1,
                  onTap: () {
                    _selectedFilter = filters[index].$1;
                    _loadReviews();
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
    if (_loadError case final error?) {
      return SliverPadding(
        padding: EdgeInsets.fromLTRB(gutter, HkSpace.lg, gutter, HkSpace.xxl),
        sliver: SliverToBoxAdapter(
          child: Column(
            children: [
              HkEmptyState(
                key: const ValueKey('seller-product-reviews-error-state'),
                icon: HkIconName.warning,
                message: errorText(
                  error,
                  defaults: ref.read(defaultErrorTextProvider),
                ),
                topPadding: HkSpace.lg,
                iconDimension: HkEmptyState.compactIconSize,
                haloDimension: HkEmptyState.compactHaloSize,
              ),
              TextButton(
                key: const ValueKey('seller-product-reviews-retry'),
                onPressed: _loadReviews,
                child: HkButtonLabel(context.l10n.commonReload),
              ),
            ],
          ),
        ),
      );
    }
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
        itemCount: reviews.length + (_nextPageToken.isEmpty ? 0 : 1),
        itemBuilder: (context, index) {
          if (index == reviews.length) {
            return TextButton(
              key: const ValueKey('seller-product-reviews-load-more'),
              onPressed: _loadingMore ? null : _loadMore,
              child: HkButtonLabel(context.l10n.commonLoadMore),
            );
          }
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
            imageUrl: source,
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

  Widget _buildReviewItem(ReviewItem review) {
    final displayName = review.authorName.isNotEmpty
        ? review.authorName
        : '匿名會員';
    final avatar = review.authorAvatarUrl;
    final content = review.content;
    final date = _formatDate(review.createTime, context.taipeiDisplay);
    final rating = review.rating.clamp(0, 5);
    final images = review.imageUrls;
    final reviewKey = review.id;

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
                  ConstrainedBox(
                    constraints: BoxConstraints(
                      maxWidth: MediaQuery.sizeOf(context).width * 0.4,
                    ),
                    child: Text(
                      date,
                      textAlign: TextAlign.end,
                      style: AppTypography.caption,
                    ),
                  ),
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
