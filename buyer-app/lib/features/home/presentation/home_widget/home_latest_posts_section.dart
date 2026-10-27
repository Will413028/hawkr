import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../../config/image_config.dart';
import '../../../../core/theme/app_colors.dart';
import '../../../../core/theme/app_typography.dart';
import '../../../../core/theme/hk_tokens.dart';
import '../../../../shared/icons/app_icon_colors.dart';
import '../../../../shared/icons/app_icons.dart';
import '../../../../shared/widgets/app_cached_image.dart';
import '../../../../shared/widgets/hk_button_label.dart';
import '../../../../shared/widgets/hk_post_card_surface.dart';
import '../../../../shared/widgets/product_grid_spec.dart';
import '../../../social/data/social_model.dart';
import '../home_latest_posts_controller.dart';

/// 首頁的內容型入口：最多顯示兩則貼文，完整瀏覽交給 social feed。
class HomeLatestPostsSection extends ConsumerWidget {
  const HomeLatestPostsSection({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final postsAsync = ref.watch(homeLatestPostsControllerProvider);

    return SliverLayoutBuilder(
      builder: (context, constraints) {
        final spec = ProductGridSpec.forWidth(constraints.crossAxisExtent);
        final content = postsAsync.when(
          data: (posts) => _PostPreviewLayout(posts: posts),
          error: (_, __) => _PostPreviewError(
            onRetry: () {
              ref.read(homeLatestPostsControllerProvider.notifier).reload();
            },
          ),
          loading: () => const _PostPreviewLoading(),
        );

        return SliverMainAxisGroup(
          slivers: [
            SliverToBoxAdapter(
              child: Padding(
                padding: EdgeInsets.symmetric(horizontal: spec.horizontalInset),
                child: Row(
                  children: [
                    Expanded(
                      child: Text(
                        '最新貼文',
                        style: AppTypography.productName.copyWith(
                          color: AppTypography.colorPrimary,
                        ),
                      ),
                    ),
                    Semantics(
                      button: true,
                      label: '瀏覽全部最新貼文',
                      child: TextButton(
                        key: const ValueKey('home-posts-see-all'),
                        onPressed: () => context.push(
                          '/social/posts',
                          extra: const {'includeGlobalFeed': true},
                        ),
                        style: ButtonStyle(
                          minimumSize: const WidgetStatePropertyAll(
                            Size(0, HkControl.minTap),
                          ),
                          padding: const WidgetStatePropertyAll(
                            EdgeInsets.symmetric(horizontal: HkSpace.xs),
                          ),
                          foregroundColor: const WidgetStatePropertyAll(
                            AppColors.brandPrimaryStrong,
                          ),
                          overlayColor: WidgetStatePropertyAll(
                            AppColors.brandPrimaryStrong.withValues(
                              alpha: 0.08,
                            ),
                          ),
                          shape: WidgetStatePropertyAll(
                            RoundedRectangleBorder(
                              borderRadius: BorderRadius.circular(HkRadius.sm),
                            ),
                          ),
                        ),
                        child: HkButtonLabel(
                          '瀏覽全部',
                          style: AppTypography.meta.copyWith(
                            color: AppColors.brandPrimaryStrong,
                            fontWeight: FontWeight.w700,
                          ),
                        ),
                      ),
                    ),
                  ],
                ),
              ),
            ),
            const SliverToBoxAdapter(child: SizedBox(height: HkSpace.xs)),
            SliverToBoxAdapter(
              child: Padding(
                padding: EdgeInsets.fromLTRB(
                  spec.horizontalInset,
                  0,
                  spec.horizontalInset,
                  HkSpace.sm,
                ),
                child: content,
              ),
            ),
          ],
        );
      },
    );
  }
}

class _PostPreviewLayout extends StatelessWidget {
  const _PostPreviewLayout({required this.posts});

  final List<Post> posts;

  @override
  Widget build(BuildContext context) {
    if (posts.isEmpty) {
      return const SizedBox(
        key: ValueKey('home-posts-empty'),
        height: 80,
        child: Center(child: Text('暫無最新貼文')),
      );
    }

    return LayoutBuilder(
      builder: (context, constraints) {
        final columnCount = constraints.maxWidth >= HkBreakpoints.tablet
            ? 2
            : 1;
        final cardWidth =
            (constraints.maxWidth - HkSpace.md * (columnCount - 1)) /
            columnCount;
        return Wrap(
          key: const ValueKey('home-posts-preview'),
          spacing: HkSpace.md,
          runSpacing: HkSpace.md,
          children: [
            for (final post in posts.take(
              HomeLatestPostsController.previewLimit,
            ))
              SizedBox(
                width: cardWidth,
                child: _HomePostPreviewCard(post: post),
              ),
          ],
        );
      },
    );
  }
}

class _HomePostPreviewCard extends StatelessWidget {
  const _HomePostPreviewCard({required this.post});

  final Post post;

  void _open(BuildContext context) {
    final merchantId = post.merchantId;
    if (merchantId == null) {
      context.push('/social/post/${post.id}');
      return;
    }
    context.push(
      '/social/posts',
      extra: {
        'merchantId': merchantId.toString(),
        'merchantName': post.merchantName,
        'merchantAvatar': post.merchantAvatar,
        'initialPostId': post.id.toString(),
        'includeGlobalFeed': true,
      },
    );
  }

  @override
  Widget build(BuildContext context) {
    final merchantName = post.merchantName?.trim().isNotEmpty == true
        ? post.merchantName!.trim()
        : 'Hawkr 商家';
    final content = post.content.trim().isNotEmpty
        ? post.content.trim()
        : '看看商家的最新分享';

    return Semantics(
      key: ValueKey('home-post-${post.id}'),
      button: true,
      label:
          '$merchantName 的貼文，$content，${post.likeCount} 人喜歡，${post.commentCount} 則留言',
      child: ExcludeSemantics(
        child: HkPostCardSurface(
          surfaceKey: ValueKey('home-post-surface-${post.id}'),
          onTap: () => _open(context),
          child: Padding(
            padding: const EdgeInsets.all(HkSpace.md),
            child: LayoutBuilder(
              builder: (context, constraints) {
                final mediaExtent = constraints.maxWidth < 320 ? 92.0 : 108.0;
                return Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    _PostThumbnail(
                      postId: post.id,
                      imageUrl: post.imageUrl,
                      extent: mediaExtent,
                    ),
                    const SizedBox(width: HkSpace.md),
                    Expanded(
                      child: Column(
                        mainAxisSize: MainAxisSize.min,
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          _MerchantMeta(
                            name: merchantName,
                            avatarUrl: post.merchantAvatar ?? '',
                            timeLabel: _formatTimeAgo(post.createdAt),
                          ),
                          const SizedBox(height: HkSpace.xs),
                          Text(
                            content,
                            maxLines: 3,
                            overflow: TextOverflow.ellipsis,
                            style: AppTypography.meta.copyWith(
                              color: AppColors.textPrimary,
                            ),
                          ),
                          const SizedBox(height: HkSpace.sm),
                          Wrap(
                            spacing: HkSpace.md,
                            runSpacing: HkSpace.xs,
                            children: [
                              _PostMetric(
                                icon: post.liked
                                    ? AppIcons.favorite
                                    : AppIcons.favoriteBorder,
                                value: post.likeCount,
                                color: post.liked
                                    ? AppIconColors.favorite
                                    : AppIconColors.muted,
                              ),
                              _PostMetric(
                                icon: AppIcons.chatBubbleOutline,
                                value: post.commentCount,
                                color: AppIconColors.muted,
                              ),
                            ],
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
      ),
    );
  }
}

class _MerchantMeta extends StatelessWidget {
  const _MerchantMeta({
    required this.name,
    required this.avatarUrl,
    required this.timeLabel,
  });

  final String name;
  final String avatarUrl;
  final String timeLabel;

  @override
  Widget build(BuildContext context) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        ClipOval(
          child: avatarUrl.isNotEmpty
              ? AppCachedImage(
                  imageUrl: avatarUrl,
                  width: 28,
                  height: 28,
                  fit: BoxFit.cover,
                  loadingStyle: AppImageLoadingStyle.quiet,
                  scene: ImageScene.avatarSmall,
                  errorWidget: Image.asset(
                    'assets/hawkrlogo.png',
                    width: 28,
                    height: 28,
                    fit: BoxFit.cover,
                  ),
                )
              : Image.asset(
                  'assets/hawkrlogo.png',
                  width: 28,
                  height: 28,
                  fit: BoxFit.cover,
                ),
        ),
        const SizedBox(width: HkSpace.sm),
        Expanded(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                name,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: AppTypography.meta.copyWith(
                  color: AppColors.textEmphasis,
                  fontWeight: FontWeight.w700,
                ),
              ),
              Text(timeLabel, style: AppTypography.caption),
            ],
          ),
        ),
      ],
    );
  }
}

class _PostThumbnail extends StatelessWidget {
  const _PostThumbnail({
    required this.postId,
    required this.imageUrl,
    required this.extent,
  });

  final int postId;
  final String imageUrl;
  final double extent;

  @override
  Widget build(BuildContext context) {
    final fallback = ColoredBox(
      color: AppColors.surfaceMuted,
      child: Center(
        child: Icon(
          AppIcons.imageOutlined,
          size: HkControl.icon,
          color: AppIconColors.muted,
        ),
      ),
    );

    return ClipRRect(
      borderRadius: BorderRadius.circular(HkRadius.md),
      child: SizedBox.square(
        dimension: extent,
        child: Hero(
          tag: 'post-image-$postId',
          child: imageUrl.isEmpty
              ? fallback
              : AppCachedImage(
                  imageUrl: imageUrl,
                  fit: BoxFit.cover,
                  loadingStyle: AppImageLoadingStyle.quiet,
                  scene: ImageScene.postImage,
                  errorWidget: fallback,
                ),
        ),
      ),
    );
  }
}

class _PostMetric extends StatelessWidget {
  const _PostMetric({
    required this.icon,
    required this.value,
    required this.color,
  });

  final IconData icon;
  final int value;
  final Color color;

  @override
  Widget build(BuildContext context) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Icon(icon, size: 18, color: color),
        const SizedBox(width: HkSpace.xs),
        Text('$value', style: AppTypography.caption),
      ],
    );
  }
}

class _PostPreviewLoading extends StatelessWidget {
  const _PostPreviewLoading();

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(
      builder: (context, constraints) {
        final columnCount = constraints.maxWidth >= HkBreakpoints.tablet
            ? 2
            : 1;
        final cardWidth =
            (constraints.maxWidth - HkSpace.md * (columnCount - 1)) /
            columnCount;
        return Wrap(
          key: const ValueKey('home-posts-loading'),
          spacing: HkSpace.md,
          runSpacing: HkSpace.md,
          children: List.generate(
            HomeLatestPostsController.previewLimit,
            (_) => Container(
              width: cardWidth,
              height: 132,
              decoration: BoxDecoration(
                color: AppColors.bgElevated,
                borderRadius: BorderRadius.circular(HkRadius.lg),
                border: Border.all(color: AppColors.borderSoft),
              ),
            ),
          ),
        );
      },
    );
  }
}

class _PostPreviewError extends StatelessWidget {
  const _PostPreviewError({required this.onRetry});

  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    return Material(
      key: const ValueKey('home-posts-error'),
      color: AppColors.surfaceMuted,
      borderRadius: BorderRadius.circular(HkRadius.md),
      child: Padding(
        padding: const EdgeInsets.symmetric(
          horizontal: HkSpace.md,
          vertical: HkSpace.sm,
        ),
        child: Row(
          children: [
            Expanded(child: Text('最新貼文目前無法載入', style: AppTypography.metaMuted)),
            TextButton(
              key: const ValueKey('home-posts-retry'),
              onPressed: onRetry,
              style: const ButtonStyle(
                minimumSize: WidgetStatePropertyAll(Size(0, HkControl.minTap)),
              ),
              child: const HkButtonLabel('重試'),
            ),
          ],
        ),
      ),
    );
  }
}

String _formatTimeAgo(dynamic value) {
  final DateTime? date = switch (value) {
    int timestamp => DateTime.fromMillisecondsSinceEpoch(
      timestamp > 1000000000000 ? timestamp : timestamp * 1000,
    ),
    num timestamp => DateTime.fromMillisecondsSinceEpoch(
      timestamp.toInt() > 1000000000000
          ? timestamp.toInt()
          : timestamp.toInt() * 1000,
    ),
    String text => DateTime.tryParse(text),
    _ => null,
  };
  if (date == null) return '剛剛';

  final difference = DateTime.now().difference(date);
  if (difference.isNegative || difference.inMinutes < 1) return '剛剛';
  if (difference.inMinutes < 60) return '${difference.inMinutes}分鐘前';
  if (difference.inHours < 24) return '${difference.inHours}小時前';
  if (difference.inDays < 7) return '${difference.inDays}天前';
  return '${date.month}/${date.day}';
}
