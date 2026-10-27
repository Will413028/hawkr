import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/app_typography.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../review/data/review_repository.dart';
import '../../../config/image_config.dart';
import '../../../shared/utils/hk_log.dart';
import '../../../shared/widgets/app_cached_image.dart';
import '../../../shared/widgets/common_top_bar.dart';
import '../../../shared/utils/avatar_wire.dart';
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
  List<Map<String, dynamic>> _reviews = [];
  bool _isLoading = true;
  String _selectedFilter = '全部';

  // 統計數據
  int _totalCount = 0;
  double _avgRating = 0.0;
  Map<int, int> _ratingDistribution = {5: 0, 4: 0, 3: 0, 2: 0, 1: 0};

  static String? _reviewImageSource(dynamic value) {
    if (value is String) {
      final source = value.trim();
      return source.isEmpty ? null : source;
    }

    if (value is Map) {
      // 49 §6.1 成對契約：優先裸 key。ReviewListItem.imageKeys/imageUrls 目前是
      // 字串陣列，Map 形式僅供未來擴充。
      const keys = ['imageKey', 'imageUrl'];
      for (final key in keys) {
        final source = value[key]?.toString().trim() ?? '';
        if (source.isNotEmpty) return source;
      }
    }

    return null;
  }

  @override
  void initState() {
    super.initState();
    _loadReviews();
  }

  Future<void> _loadReviews() async {
    setState(() => _isLoading = true);
    HKLog.logD('[ProductReviewsScreen] ===== LOADING REVIEWS =====');
    HKLog.logD(
      '[ProductReviewsScreen] merchantId="${widget.merchantId}", skuId="${widget.skuId}"',
    );
    try {
      final list = await ref
          .read(reviewRepositoryProvider)
          .getMerchantProductReviews(widget.skuId);
      HKLog.logD('[ProductReviewsScreen] API returned ${list.length} reviews');

      // /public/products/{id}/reviews 已經是指定商品的 review，不需要再過濾
      // （回傳欄位僅 reviewId/buyerId/username/avatarKey/avatarUrl/rating/content/createdAt/images，
      //  沒有 merchantId / skuId，舊版濾掉是因為前端條件寫死要求這些欄位存在）
      final filteredList = list;

      // 計算統計數據
      int count = 0;
      double sum = 0.0;
      final Map<int, int> dist = {5: 0, 4: 0, 3: 0, 2: 0, 1: 0};

      for (final review in filteredList) {
        final r = review['rating'] ?? review['score'] ?? review['stars'] ?? 0;
        final v = (r is num) ? r.toDouble() : double.tryParse('$r') ?? 0.0;
        if (v > 0) {
          sum += v;
          count++;
          final k = v.round().clamp(1, 5);
          dist[k] = (dist[k] ?? 0) + 1;
        }
      }

      if (mounted) {
        setState(() {
          _reviews = filteredList;
          _totalCount = filteredList.length;
          _avgRating = count > 0 ? sum / count : 0.0;
          _ratingDistribution = dist;
          _isLoading = false;
        });
      }
    } catch (e) {
      HKLog.logE('[ProductReviewsScreen] Error: $e');
      if (mounted) {
        setState(() {
          _reviews = [];
          _isLoading = false;
        });
      }
    }
  }

  List<Map<String, dynamic>> get _filteredReviews {
    if (_selectedFilter == '全部') {
      return _reviews;
    } else if (_selectedFilter == '有圖片') {
      return _reviews.where((r) {
        final images = r['imageKeys'] ?? r['imageUrls'] ?? [];
        return images is List &&
            images.any((image) => _reviewImageSource(image) != null);
      }).toList();
    } else {
      // 星等篩選 (例如 "5星")
      final starCount = int.tryParse(_selectedFilter.replaceAll('星', ''));
      if (starCount != null) {
        return _reviews.where((r) {
          final rating = r['rating'] ?? 0;
          final v = (rating is num)
              ? rating.round()
              : int.tryParse('$rating') ?? 0;
          return v == starCount;
        }).toList();
      }
    }
    return _reviews;
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
    return Scaffold(
      backgroundColor: AppColors.bgPrimary,
      appBar: buildCommonTitleAppBar(context, title: '商品評價', showBack: true),
      body: _isLoading
          ? const TruckLoadingOverlay()
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
    return _reviews.where((r) {
      final images = r['imageKeys'] ?? r['imageUrls'] ?? [];
      return images is List &&
          images.any((image) => _reviewImageSource(image) != null);
    }).length;
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

  Widget _buildUserAvatar(String url) {
    if (url.isNotEmpty) {
      return ClipOval(
        child: SizedBox(
          width: 40,
          height: 40,
          child: AppCachedImage(
            imageUrl: url,
            // 49 號成對契約：avatarKey 為裸 objectKey，scene 令其走
            // buildImageUrl dual-mode（avatar preset）。
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
    final content = (review['content'] ?? '').toString();
    final dateStr = _formatDate(review['createdAt']);
    final rating = int.tryParse('${review['rating'] ?? ''}')?.clamp(0, 5) ?? 0;

    final username = (review['username'] ?? '').toString().trim();
    // 49 §6.1 成對契約：SSOT 是 avatarKey（裸 objectKey），顯示端由
    // AppCachedImage 的 scene 走 buildImageUrl 組 avatar preset URL。
    final userAvatar = resolveAvatarKey(review) ?? '';

    final displayName = username.isNotEmpty
        ? username
        : (buyerId.length >= 4
              ? '會員 ${buyerId.substring(buyerId.length - 4)}'
              : (buyerId.isNotEmpty ? '會員 $buyerId' : '匿名會員'));

    final images = review['imageKeys'] ?? review['imageUrls'];
    final imageList = (images is List)
        ? images
              .map(_reviewImageSource)
              .whereType<String>()
              .toList(growable: false)
        : <String>[];

    final reviewKey =
        (review['reviewId'] ?? review['id'] ?? '$buyerId-$dateStr').toString();
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
