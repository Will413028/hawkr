import 'package:hawkr_buyer/config/image_config.dart';
import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/shared/widgets/app_cached_image.dart';
import 'package:flutter/material.dart';

/// 單列、可左右滑動的 4:3 商品縮圖列。
///
/// 畫面寬度容納 5 張縮圖；最多顯示正式商品圖庫上限 9 張。
class ProductImageThumbnailGrid extends StatelessWidget {
  const ProductImageThumbnailGrid({
    super.key,
    required this.images,
    required this.currentIndex,
    required this.onSelected,
  }) : assert(currentIndex >= 0 && currentIndex < images.length);

  static const int columnCount = 5;
  static const int maxImages = 9;
  static const double spacing = HkSpace.xs;
  static const double aspectRatio = 4 / 3;

  final List<String> images;
  final int currentIndex;
  final ValueChanged<int> onSelected;

  @override
  Widget build(BuildContext context) {
    final visibleImages = images.take(maxImages).toList(growable: false);
    return LayoutBuilder(
      builder: (context, constraints) {
        final itemWidth =
            (constraints.maxWidth - spacing * (columnCount - 1)) / columnCount;
        final contentWidth =
            itemWidth * visibleImages.length +
            spacing * (visibleImages.length - 1);
        final horizontalInset = ((constraints.maxWidth - contentWidth) / 2)
            .clamp(0.0, double.infinity);
        return SizedBox(
          height: itemWidth / aspectRatio,
          child: ListView.separated(
            key: const ValueKey('product-image-thumbnail-list'),
            padding: EdgeInsets.symmetric(horizontal: horizontalInset),
            scrollDirection: Axis.horizontal,
            physics: const BouncingScrollPhysics(),
            itemCount: visibleImages.length,
            separatorBuilder: (_, _) => const SizedBox(width: spacing),
            itemBuilder: (context, index) {
              final selected = index == currentIndex;
              return Semantics(
                button: true,
                selected: selected,
                label: '查看商品圖片 ${index + 1}',
                child: SizedBox(
                  key: ValueKey('product-image-thumbnail-slot-$index'),
                  width: itemWidth,
                  child: Center(
                    child: AnimatedScale(
                      scale: selected ? 1 : 0.94,
                      duration: HkMotion.duration,
                      curve: HkMotion.ease,
                      child: Material(
                        color: Colors.transparent,
                        child: InkWell(
                          key: ValueKey(
                            'product-image-thumbnail-target-$index',
                          ),
                          borderRadius: BorderRadius.circular(HkRadius.md),
                          onTap: () => onSelected(index),
                          child: AnimatedContainer(
                            key: ValueKey('product-image-thumbnail-$index'),
                            duration: HkMotion.duration,
                            curve: HkMotion.ease,
                            decoration: BoxDecoration(
                              color: AppColors.neutralCream,
                              borderRadius: BorderRadius.circular(HkRadius.md),
                              border: Border.all(
                                color: selected
                                    ? AppColors.actionPrimary
                                    : AppColors.borderInput.withValues(
                                        alpha: 0.72,
                                      ),
                                width: 2,
                              ),
                              boxShadow: selected
                                  ? [
                                      BoxShadow(
                                        color: AppColors.actionPrimary
                                            .withValues(alpha: 0.16),
                                        blurRadius: 14,
                                        offset: const Offset(0, 5),
                                      ),
                                      BoxShadow(
                                        color: AppColors.neutralWhite
                                            .withValues(alpha: 0.82),
                                        blurRadius: 2,
                                        offset: const Offset(0, -1),
                                      ),
                                    ]
                                  : HkShadows.xs,
                            ),
                            padding: const EdgeInsets.all(3),
                            child: AspectRatio(
                              aspectRatio: aspectRatio,
                              child: ClipRRect(
                                borderRadius: BorderRadius.circular(
                                  HkRadius.sm,
                                ),
                                child: _ThumbnailImage(
                                  source: visibleImages[index],
                                ),
                              ),
                            ),
                          ),
                        ),
                      ),
                    ),
                  ),
                ),
              );
            },
          ),
        );
      },
    );
  }
}

class _ThumbnailImage extends StatelessWidget {
  const _ThumbnailImage({required this.source});

  final String source;

  @override
  Widget build(BuildContext context) {
    if (source.startsWith('assets/')) {
      return Image.asset(
        source,
        fit: BoxFit.cover,
        alignment: Alignment.center,
      );
    }
    return AppCachedImage(
      imageUrl: source,
      fit: BoxFit.cover,
      scene: ImageScene.productThumbnail,
      loadingStyle: AppImageLoadingStyle.quiet,
    );
  }
}
