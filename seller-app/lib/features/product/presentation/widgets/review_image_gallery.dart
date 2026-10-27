import 'dart:ui' as ui;

import 'package:flutter/material.dart';

import '../../../../config/image_config.dart';
import '../../../../core/motion/hk_motion_transitions.dart';
import '../../../../core/theme/app_colors.dart';
import '../../../../core/theme/app_typography.dart';
import '../../../../core/theme/hk_tokens.dart';
import '../../../../shared/icons/app_icon_colors.dart';
import '../../../../shared/icons/app_icons.dart';
import '../../../../shared/widgets/hk_frosted_card_surface.dart';
import '../../../../shared/widgets/optimized_image.dart';

typedef ReviewImageBuilder =
    Widget Function(BuildContext context, String source, BoxFit fit);

Widget _defaultReviewImageBuilder(
  BuildContext context,
  String source,
  BoxFit fit,
) {
  return OptimizedNetworkImage(
    imageUrl: source,
    scene: fit == BoxFit.cover ? ImageScene.reviewPhoto : ImageScene.fullscreen,
    fit: fit,
  );
}

Future<void> showReviewImageViewer(
  BuildContext context, {
  required List<String> images,
  int initialIndex = 0,
  ReviewImageBuilder imageBuilder = _defaultReviewImageBuilder,
}) {
  if (images.isEmpty) return Future<void>.value();
  final safeIndex = initialIndex.clamp(0, images.length - 1);
  return showGeneralDialog<void>(
    context: context,
    useRootNavigator: true,
    barrierDismissible: true,
    barrierLabel: MaterialLocalizations.of(context).modalBarrierDismissLabel,
    barrierColor: AppColors.textEmphasis.withValues(alpha: 0.18),
    transitionDuration: HkMotionTransitions.durationOf(
      context,
      presentation: HkModalPresentation.photoZoom,
    ),
    pageBuilder: (_, __, ___) => ReviewImageViewerDialog(
      images: images,
      initialIndex: safeIndex,
      imageBuilder: imageBuilder,
    ),
    transitionBuilder: (_, animation, __, child) {
      return HkMotionTransitions.modal(
        context: context,
        animation: animation,
        presentation: HkModalPresentation.photoZoom,
        child: child,
      );
    },
  );
}

class ReviewImageGallery extends StatelessWidget {
  const ReviewImageGallery({
    super.key,
    required this.images,
    this.imageBuilder = _defaultReviewImageBuilder,
  });

  final List<String> images;
  final ReviewImageBuilder imageBuilder;

  @override
  Widget build(BuildContext context) {
    if (images.isEmpty) return const SizedBox.shrink();
    return SizedBox(
      key: const ValueKey('review-image-gallery'),
      height: 88,
      child: ListView.separated(
        scrollDirection: Axis.horizontal,
        itemCount: images.length,
        separatorBuilder: (_, __) => const SizedBox(width: HkSpace.sm),
        itemBuilder: (context, index) {
          final source = images[index];
          return Semantics(
            button: true,
            label: '查看第 ${index + 1} 張評價圖片，共 ${images.length} 張',
            child: SizedBox.square(
              dimension: 88,
              child: Material(
                color: AppColors.storefrontMutedSurface,
                shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(HkRadius.lg),
                  side: const BorderSide(color: AppColors.borderSoft),
                ),
                clipBehavior: Clip.antiAlias,
                child: InkWell(
                  key: ValueKey('review-image-thumbnail-$index'),
                  onTap: () => showReviewImageViewer(
                    context,
                    images: images,
                    initialIndex: index,
                    imageBuilder: imageBuilder,
                  ),
                  child: Stack(
                    fit: StackFit.expand,
                    children: [
                      imageBuilder(context, source, BoxFit.cover),
                      Positioned(
                        right: HkSpace.xs,
                        bottom: HkSpace.xs,
                        child: IgnorePointer(
                          child: DecoratedBox(
                            decoration: BoxDecoration(
                              color: AppColors.neutralWhite.withValues(
                                alpha: 0.88,
                              ),
                              shape: BoxShape.circle,
                              border: Border.all(color: AppColors.neutralWhite),
                              boxShadow: HkShadows.sm,
                            ),
                            child: const SizedBox.square(
                              dimension: 28,
                              child: Icon(
                                AppIcons.imageOutlined,
                                size: 16,
                                color: AppIconColors.active,
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
          );
        },
      ),
    );
  }
}

class ReviewImageViewerDialog extends StatefulWidget {
  const ReviewImageViewerDialog({
    super.key,
    required this.images,
    required this.initialIndex,
    this.imageBuilder = _defaultReviewImageBuilder,
  }) : assert(images.length > 0);

  final List<String> images;
  final int initialIndex;
  final ReviewImageBuilder imageBuilder;

  @override
  State<ReviewImageViewerDialog> createState() =>
      _ReviewImageViewerDialogState();
}

class _ReviewImageViewerDialogState extends State<ReviewImageViewerDialog> {
  late final PageController _pageController;
  late int _currentIndex;

  @override
  void initState() {
    super.initState();
    _currentIndex = widget.initialIndex.clamp(0, widget.images.length - 1);
    _pageController = PageController(initialPage: _currentIndex);
  }

  @override
  void dispose() {
    _pageController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Semantics(
      scopesRoute: true,
      namesRoute: true,
      explicitChildNodes: true,
      label: '評價圖片檢視',
      child: SafeArea(
        child: BackdropFilter(
          filter: ui.ImageFilter.blur(
            sigmaX: HkGlass.dialogBlurSigma,
            sigmaY: HkGlass.dialogBlurSigma,
          ),
          child: Center(
            child: Padding(
              padding: const EdgeInsets.all(HkSpace.md),
              child: ConstrainedBox(
                constraints: const BoxConstraints(
                  maxWidth: 760,
                  maxHeight: 760,
                ),
                child: HkFrostedCardSurface(
                  key: const ValueKey('review-image-viewer-dialog'),
                  radius: HkRadius.xl,
                  shadows: HkShadows.modal,
                  child: Padding(
                    padding: const EdgeInsets.all(HkSpace.md),
                    child: Column(
                      children: [
                        _ViewerHeader(
                          currentIndex: _currentIndex,
                          count: widget.images.length,
                          onClose: () => Navigator.of(context).pop(),
                        ),
                        const SizedBox(height: HkSpace.sm),
                        Expanded(
                          child: ClipRRect(
                            borderRadius: BorderRadius.circular(HkRadius.lg),
                            child: ColoredBox(
                              color: AppColors.neutralWhisper,
                              child: Stack(
                                children: [
                                  PageView.builder(
                                    key: const ValueKey(
                                      'review-image-viewer-pages',
                                    ),
                                    controller: _pageController,
                                    physics: const PageScrollPhysics(
                                      parent: BouncingScrollPhysics(),
                                    ),
                                    pageSnapping: true,
                                    itemCount: widget.images.length,
                                    onPageChanged: (index) {
                                      setState(() => _currentIndex = index);
                                    },
                                    itemBuilder: (context, index) {
                                      return LayoutBuilder(
                                        builder: (context, constraints) {
                                          return InteractiveViewer(
                                            key: ValueKey(
                                              'review-image-zoom-$index',
                                            ),
                                            minScale: 1,
                                            maxScale: 4,
                                            child: SizedBox(
                                              width: constraints.maxWidth,
                                              height: constraints.maxHeight,
                                              child: widget.imageBuilder(
                                                context,
                                                widget.images[index],
                                                BoxFit.contain,
                                              ),
                                            ),
                                          );
                                        },
                                      );
                                    },
                                  ),
                                ],
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
        ),
      ),
    );
  }
}

class _ViewerHeader extends StatelessWidget {
  const _ViewerHeader({
    required this.currentIndex,
    required this.count,
    required this.onClose,
  });

  final int currentIndex;
  final int count;
  final VoidCallback onClose;

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      height: HkControl.minTap,
      child: Row(
        children: [
          Expanded(
            child: Align(
              alignment: Alignment.centerLeft,
              child: Text('評價圖片', style: AppTypography.productName),
            ),
          ),
          if (count > 1)
            Padding(
              padding: const EdgeInsets.only(right: HkSpace.sm),
              child: Text(
                '${currentIndex + 1} / $count',
                key: const ValueKey('review-image-counter'),
                style: AppTypography.metaMuted.copyWith(
                  fontWeight: FontWeight.w700,
                ),
              ),
            ),
          IconButton(
            key: const ValueKey('review-image-viewer-close'),
            tooltip: '關閉圖片',
            onPressed: onClose,
            icon: const Icon(AppIcons.close, color: AppIconColors.defaultColor),
          ),
        ],
      ),
    );
  }
}
