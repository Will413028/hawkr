import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/app_typography.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import 'package:hawkr_api/errors.dart' show errorText;
import 'package:hawkr_api/images.dart' as api;
import 'package:hawkr_api/gen/hawkr/review/v1alpha1/review.pb.dart' as pb;

import '../../../core/l10n/ui_locale.dart';
import '../../review/data/review_display.dart';
import '../../review/data/review_repository.dart';
import '../../../config/image_config.dart';
import '../../../shared/utils/hk_log.dart';
import '../../../shared/widgets/app_cached_image.dart';
import '../../../shared/widgets/common_top_bar.dart';
import '../../../shared/widgets/hk_button_label.dart';
import '../../../shared/widgets/hk_empty_state.dart';
import '../../../shared/widgets/hk_choice_chip.dart';
import '../../../shared/widgets/hk_frosted_card_surface.dart';
import '../../../shared/widgets/truck_loading_overlay.dart';
import 'widgets/review_image_gallery.dart';

/// 商品評論詳細頁面
class ProductReviewsScreen extends ConsumerStatefulWidget {
  const ProductReviewsScreen({
    super.key,
    required this.merchantId,
    required this.skuId,
    required this.productName,
  });

  final String merchantId;
  final String skuId;
  final String productName;

  @override
  ConsumerState<ProductReviewsScreen> createState() =>
      _ProductReviewsScreenState();
}

class _ProductReviewsScreenState extends ConsumerState<ProductReviewsScreen> {
  List<pb.Review> _reviews = [];
  bool _isLoading = true;
  Object? _loadError;
  String _selectedFilter = '全部';

  // 統計數據
  int _totalCount = 0;
  double _avgRating = 0.0;
  Map<int, int> _ratingDistribution = {5: 0, 4: 0, 3: 0, 2: 0, 1: 0};

  @override
  void initState() {
    super.initState();
    _loadReviews();
  }

  Future<void> _loadReviews() async {
    setState(() {
      _isLoading = true;
      _loadError = null;
    });
    try {
      final repo = ref.read(reviewRepositoryProvider);
      final (list, stats) = await (
        repo.listAllProductReviews(widget.skuId),
        repo.getProductReviewStats(widget.skuId),
      ).wait;
      if (mounted) {
        setState(() {
          _reviews = list;
          _totalCount = stats.count;
          _avgRating = stats.average;
          _ratingDistribution = stats.distribution;
          _isLoading = false;
        });
      }
    } catch (e) {
      HKLog.logE('[ProductReviewsScreen] Error: $e');
      if (mounted) {
        setState(() {
          _reviews = [];
          _loadError = e;
          _isLoading = false;
        });
      }
    }
  }

  List<pb.Review> get _filteredReviews {
    if (_selectedFilter == '全部') {
      return _reviews;
    } else if (_selectedFilter == '有圖片') {
      return _reviews.where((r) => r.images.isNotEmpty).toList();
    } else {
      // 星等篩選 (例如 "5星")
      final starCount = int.tryParse(_selectedFilter.replaceAll('星', ''));
      if (starCount != null) {
        return _reviews.where((r) => r.rating == starCount).toList();
      }
    }
    return _reviews;
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: AppColors.bgPrimary,
      appBar: buildCommonTitleAppBar(context, title: '商品評價', showBack: true),
      body: _isLoading
          ? const TruckLoadingOverlay()
          : _loadError != null
          ? _buildLoadFailure(_loadError!)
          : ColoredBox(
              key: const ValueKey('product-reviews-background'),
              color: AppColors.bgPrimary,
              child: LayoutBuilder(
                builder: (context, constraints) {
                  final gutter = HkBreakpoints.pageGutter(constraints.maxWidth);
                  return CustomScrollView(
                    key: const ValueKey('product-reviews-scroll-view'),
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

  /// 讀取失敗：顯示錯誤文字與重試，不當成「沒有評價」。
  Widget _buildLoadFailure(Object error) {
    return Center(
      key: const ValueKey('product-reviews-load-failure'),
      child: Padding(
        padding: const EdgeInsets.all(HkSpace.xl),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(
              errorText(error, defaults: ref.read(defaultErrorTextProvider)),
              textAlign: TextAlign.center,
              style: AppTypography.body,
            ),
            const SizedBox(height: HkSpace.md),
            TextButton(
              key: const ValueKey('product-reviews-retry'),
              onPressed: _loadReviews,
              child: const HkButtonLabel('重試'),
            ),
          ],
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
      key: const ValueKey('product-reviews-summary-card'),
      child: Padding(
        padding: const EdgeInsets.all(HkSpace.xl),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                const _ReviewSectionIcon(
                  icon: AppIcons.starRounded,
                  color: AppIconColors.rating,
                ),
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
      ('有圖片', _getImageReviewCount()),
    ];
    return Column(
      key: const ValueKey('product-reviews-filter-section'),
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
                  key: ValueKey('product-review-filter-${filters[index].$1}'),
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

  int _getImageReviewCount() {
    return _reviews.where((r) => r.images.isNotEmpty).length;
  }

  Widget _buildReviewListHeading() {
    final reviews = _filteredReviews;
    return Row(
      children: [
        Expanded(
          child: Text(
            '顧客回饋',
            key: const ValueKey('product-reviews-list-heading'),
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
              key: ValueKey('product-reviews-empty-state'),
              iconKey: ValueKey('product-reviews-empty-state-icon'),
              icon: HkIconName.emptyReviews,
              message: '暫無符合條件的評論',
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

  Widget _buildUserAvatar(api.Image? avatar) {
    if (avatar != null) {
      return ClipOval(
        child: SizedBox(
          width: 40,
          height: 40,
          child: AppCachedImage.api(
            image: avatar,
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

  Widget _buildReviewItem(pb.Review review) {
    final content = review.content;
    final dateStr = review.createDateText;
    final rating = review.rating.clamp(0, 5);
    final userAvatar = review.authorAvatar;
    final displayName = review.authorName.isNotEmpty
        ? review.authorName
        : anonymousReviewerName;
    final imageList = review.images;
    final reviewKey = review.id;
    return HkFrostedCardSurface(
      key: ValueKey('product-review-card-$reviewKey'),
      child: Padding(
        padding: const EdgeInsets.all(HkSpace.lg),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              crossAxisAlignment: CrossAxisAlignment.center,
              children: [
                _buildUserAvatar(userAvatar),
                const SizedBox(width: HkSpace.md),
                Expanded(
                  child: Text(
                    displayName,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: AppTypography.bodyBold,
                  ),
                ),
                if (dateStr.isNotEmpty) ...[
                  const SizedBox(width: HkSpace.sm),
                  Text(dateStr, style: AppTypography.caption),
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
            if (imageList.isNotEmpty) ...[
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
                    '${imageList.length} 張實拍 · 點擊放大',
                    style: AppTypography.metaMuted,
                  ),
                ],
              ),
              const SizedBox(height: HkSpace.sm),
              ReviewImageGallery(images: imageList),
            ],
          ],
        ),
      ),
    );
  }
}

class _ReviewSectionIcon extends StatelessWidget {
  const _ReviewSectionIcon({required this.icon, required this.color});

  final IconData icon;
  final Color color;

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
        child: Icon(icon, color: color, size: HkControl.icon),
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
          key: const ValueKey('product-reviews-average-rating'),
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
    final ratedCount = distribution.values.fold<int>(0, (sum, count) {
      return sum + count;
    });
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
