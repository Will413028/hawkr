import 'dart:convert';
import 'dart:ui';

import 'package:hawkr_buyer/shared/widgets/hk_frosted_icon_button.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../config/app_constants.dart';
import '../../../config/flavor.dart';
import '../../../config/image_config.dart';
import '../../../config/media_image.dart';
import '../../../core/theme/app_colors.dart';
import '../../../core/theme/hk_tokens.dart';
import '../../../shared/utils/avatar_utils.dart';
import '../../../shared/utils/hashtag_spans.dart';
import '../../../shared/utils/require_login.dart';
import '../../../shared/widgets/app_cached_image.dart';
import '../../../shared/widgets/comments_bottom_sheet.dart';
import '../../../shared/widgets/hk_button_label.dart';
import '../../../shared/widgets/hk_post_card_surface.dart';
import '../../../shared/widgets/share_bottom_sheet.dart';
import '../../../shared/widgets/truck_loading_overlay.dart';
import '../../moderation/data/report_target_type.dart';
import '../../moderation/presentation/report_block_action_sheet.dart';
import '../../product/data/product_repository.dart';
import '../../product/presentation/merchant_item_detail_screen.dart';
import '../data/social_model.dart';
import 'post_feed_controller.dart';
import 'post_preview_layout.dart';
import '../../../shared/utils/price_display.dart';
import 'package:hawkr_buyer/shared/widgets/hk_placeholder.dart';

const Color _kGray100 = AppColors.textPrimary;
const Color _kMediaForeground = AppColors.textEmphasis;
const Color _kMediaMuted = AppColors.textSecondary;

bool _hasValidProduct(Map<String, dynamic>? product) {
  if (product == null) return false;
  final name = (product['name'] ?? '').toString().trim();
  // `"0"` 是後端「無價格」的表達且非空字串 —— 用 isNotEmpty 判斷會讓
  // 「無名稱＋無價格」的商品節點通過，渲染出一張空卡片。
  final price = PriceDisplay.formatPrice(product['price']);
  return name.isNotEmpty || price.isNotEmpty;
}

int _resolveInitialPostIndex(
  List<Post> posts, {
  required int initialIndex,
  required String? initialPostId,
}) {
  final requestedPostId = (initialPostId ?? '').trim();
  if (requestedPostId.isNotEmpty) {
    final matchedIndex = posts.indexWhere(
      (post) => post.id.toString() == requestedPostId,
    );
    if (matchedIndex >= 0) return matchedIndex;
  }
  return initialIndex.clamp(0, posts.length - 1);
}

class _GsapCardPagePhysics extends PageScrollPhysics {
  const _GsapCardPagePhysics({super.parent});

  @override
  _GsapCardPagePhysics applyTo(ScrollPhysics? ancestor) {
    return _GsapCardPagePhysics(parent: buildParent(ancestor));
  }

  // 略低於臨界阻尼：保留 GSAP Inertia 式的動量，但會穩定 snap 回卡片中心。
  @override
  SpringDescription get spring =>
      const SpringDescription(mass: 0.9, stiffness: 310, damping: 30);
}

class MerchantPostsFeedScreen extends HookConsumerWidget {
  const MerchantPostsFeedScreen({
    super.key,
    required this.merchantId,
    this.merchantName,
    this.merchantAvatar,
    this.initialIndex = 0,
    this.initialPostId,
    this.includeGlobalFeed = false,
  });

  final String merchantId;
  final String? merchantName;
  final String? merchantAvatar;
  final int initialIndex;
  final String? initialPostId;

  /// `true` 時：商家貼文擺前面，後面接全平台其他商家貼文（去重後）。
  /// 從 marketplace「最新貼文」進入用此模式 — 用戶可繼續往下滑看全平台其他貼文。
  /// `false`（預設）只有該商家貼文（merchant_home_tab 進入用此模式）。
  final bool includeGlobalFeed;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final mid = int.tryParse(merchantId);
    final merchantPostsAsync = ref.watch(
      postFeedControllerProvider(merchantId: mid),
    );
    final globalPostsAsync = includeGlobalFeed
        ? ref.watch(postFeedControllerProvider())
        : const AsyncValue<List<Post>>.data(<Post>[]);

    // Merge 兩個來源
    final List<Post>? mergedPosts;
    if (merchantPostsAsync.hasValue && globalPostsAsync.hasValue) {
      final merchantPosts = merchantPostsAsync.value!;
      final globalPosts = globalPostsAsync.value!;
      if (includeGlobalFeed) {
        if (merchantPosts.length > 1) {
          // 商家還有其他貼文：擺前面 + 全平台其他商家貼文（去重）
          final seen = merchantPosts.map((p) => p.id).toSet();
          final others = globalPosts
              .where((p) => !seen.contains(p.id))
              .toList();
          mergedPosts = [...merchantPosts, ...others];
        } else {
          // 商家只有 1 篇（即點擊那篇）→ 切成純全平台 feed，不強制置頂。
          // 若全平台 list 沒包含該篇（後端 pagination 未回），把它前置以保證能找到。
          final hasInitial =
              (initialPostId ?? '').isEmpty ||
              globalPosts.any((p) => p.id.toString() == initialPostId);
          mergedPosts = hasInitial
              ? globalPosts
              : [...merchantPosts, ...globalPosts];
        }
      } else {
        mergedPosts = merchantPosts;
      }
    } else {
      mergedPosts = null;
    }

    final pageController = usePageController(initialPage: initialIndex);
    final currentIndex = useState(initialIndex);

    useEffect(() {
      if (mergedPosts == null) return null;
      final posts = mergedPosts;
      int target = initialIndex;
      if ((initialPostId ?? '').isNotEmpty) {
        final idx = posts.indexWhere((p) => p.id.toString() == initialPostId);
        if (idx >= 0) target = idx;
      }
      if (target < 0) target = 0;
      if (posts.isEmpty) return null;
      if (target >= posts.length) target = posts.length - 1;
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (!pageController.hasClients) return;
        if (target == currentIndex.value) return;
        pageController.jumpToPage(target);
        currentIndex.value = target;
      });
      return null;
    }, [mergedPosts == null]);

    // Loading / error 狀態：兩個 async 任一還沒就緒就 loading；任一錯就顯示錯誤。
    if (merchantPostsAsync.isLoading ||
        (includeGlobalFeed && globalPostsAsync.isLoading)) {
      return Scaffold(
        backgroundColor: AppColors.bgPrimary,
        body: const TruckLoadingOverlay(),
      );
    }
    final err =
        merchantPostsAsync.error ??
        (includeGlobalFeed ? globalPostsAsync.error : null);
    if (err != null) {
      return Scaffold(
        backgroundColor: AppColors.bgPrimary,
        body: _ErrorState(
          message: err.toString(),
          onRetry: () {
            ref.invalidate(postFeedControllerProvider(merchantId: mid));
            if (includeGlobalFeed) {
              ref.invalidate(postFeedControllerProvider());
            }
          },
        ),
      );
    }

    final posts = mergedPosts ?? const <Post>[];
    if (posts.isEmpty) {
      return const Scaffold(
        backgroundColor: AppColors.bgPrimary,
        body: _EmptyState(),
      );
    }

    final activeIndex = currentIndex.value.clamp(0, posts.length - 1);
    final backdropIndex = _resolveInitialPostIndex(
      posts,
      initialIndex: initialIndex,
      initialPostId: initialPostId,
    );
    final backdropImageUrl = _toAbsolute(posts[backdropIndex].imageUrl);
    final reduceMotion = MediaQuery.disableAnimationsOf(context);

    Future<void> openComments(Post post) async {
      final username = (post.merchantName ?? merchantName ?? 'Hawkr').trim();
      await showModalBottomSheet<void>(
        context: context,
        isScrollControlled: true,
        useSafeArea: true,
        showDragHandle: false,
        backgroundColor: AppColors.neutralBlack.withValues(alpha: 0),
        barrierColor: AppColors.neutralBlack.withValues(alpha: 0.42),
        builder: (_) => CommentsBottomSheet(
          postId: post.id.toString(),
          postOwnerName: username,
          merchantAvatar: post.merchantAvatar ?? merchantAvatar,
        ),
      );
    }

    Future<void> toggleLike(Post post) async {
      if (!await requireLogin(context, ref)) return;
      await ref
          .read(
            postFeedControllerProvider(
              // 跨商家貼文使用 global scope，才能更新正確的 feed。
              merchantId: post.merchantId == mid ? mid : null,
            ).notifier,
          )
          .likePost(post.id);
    }

    return AnnotatedRegion<SystemUiOverlayStyle>(
      value: AppColors.glassTopOverlay.copyWith(
        statusBarColor: Colors.transparent,
        systemNavigationBarColor: AppColors.navBarBackground,
        systemNavigationBarIconBrightness: Brightness.dark,
      ),
      child: Scaffold(
        backgroundColor: AppColors.bgPrimary,
        body: ColoredBox(
          key: const ValueKey('post-preview-surface'),
          color: AppColors.bgPrimary,
          child: Stack(
            children: [
              Positioned.fill(
                child: _PostPreviewBackdrop(
                  key: const ValueKey('post-static-backdrop'),
                  imageUrl: backdropImageUrl,
                ),
              ),
              PageView.builder(
                key: const ValueKey('post-card-page-view'),
                scrollDirection: Axis.horizontal,
                controller: pageController,
                physics: const _GsapCardPagePhysics(
                  parent: BouncingScrollPhysics(),
                ),
                itemCount: posts.length,
                onPageChanged: (i) {
                  currentIndex.value = i;
                  // 左右滑動預載：距尾端 ≤ 3 篇時觸發 loadMore。
                  if (i >= posts.length - 3) {
                    if (includeGlobalFeed) {
                      ref
                          .read(postFeedControllerProvider().notifier)
                          .loadMore();
                    } else if (mid != null) {
                      ref
                          .read(
                            postFeedControllerProvider(
                              merchantId: mid,
                            ).notifier,
                          )
                          .loadMore();
                    }
                  }
                },
                itemBuilder: (_, i) => AnimatedBuilder(
                  key: ValueKey('post-card-motion-${posts[i].id}'),
                  animation: pageController,
                  child: _PostPage(
                    post: posts[i],
                    fallbackMerchantName: merchantName,
                    fallbackMerchantAvatar: merchantAvatar,
                    parentMerchantId: merchantId,
                    onLikeTap: () => toggleLike(posts[i]),
                    onCommentTap: () => openComments(posts[i]),
                    onShareTap: () => _shareCurrent(context, ref, posts[i]),
                  ),
                  builder: (context, child) {
                    if (reduceMotion ||
                        !pageController.hasClients ||
                        !pageController.position.hasContentDimensions) {
                      return child!;
                    }
                    final page = pageController.page ?? activeIndex.toDouble();
                    final signedDistance = (page - i).clamp(-1.0, 1.0);
                    final distance = signedDistance.abs();
                    final easedDistance = Curves.easeOutCubic.transform(
                      distance,
                    );
                    final perspective = Matrix4.identity()
                      ..setEntry(3, 2, 0.0009)
                      ..rotateY(-signedDistance * 0.035);
                    return Opacity(
                      key: ValueKey('post-card-depth-${posts[i].id}'),
                      opacity: 1 - (easedDistance * 0.16),
                      child: Transform.translate(
                        offset: Offset(0, easedDistance * 10),
                        child: Transform(
                          alignment: Alignment.center,
                          transform: perspective,
                          child: Transform.scale(
                            scale: 1 - (easedDistance * 0.035),
                            child: child,
                          ),
                        ),
                      ),
                    );
                  },
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Future<void> _shareCurrent(
    BuildContext context,
    WidgetRef ref,
    Post post,
  ) async {
    final postId = post.id.toString();
    final username = (post.merchantName ?? merchantName ?? '').trim();
    final imageUrl = post.imageUrl;
    final data = await ref
        .read(productRepositoryProvider)
        .getShareUrl(type: 'post', id: postId);
    if (!context.mounted) return;
    final storeCode = post.storeCode;
    final url =
        data?['url'] ??
        (storeCode.isEmpty
            ? null
            : AppConstants.shareFallbackUrl(
                '/sellers/$storeCode/posts#post-$postId',
              ));
    if (url == null) return;
    showShareSheet(
      context,
      ShareData(
        title: (data?['title'] ?? username).toString(),
        description: (data?['description'] ?? post.content).toString(),
        imageUrl: (data?['imageUrl'] ?? imageUrl).toString(),
        url: url.toString(),
      ),
    );
  }
}

// ─── 單篇貼文（Figma Design 1：image + 商家 + 日期 + 內文 + 留言 + bottom bar） ──
class _PostPage extends HookConsumerWidget {
  const _PostPage({
    required this.post,
    required this.parentMerchantId,
    required this.onLikeTap,
    required this.onCommentTap,
    required this.onShareTap,
    this.fallbackMerchantName,
    this.fallbackMerchantAvatar,
  });

  final Post post;
  final String parentMerchantId;
  final VoidCallback onLikeTap;
  final VoidCallback onCommentTap;
  final VoidCallback onShareTap;
  final String? fallbackMerchantName;
  final String? fallbackMerchantAvatar;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final imageUrl = _toAbsolute(post.imageUrl);
    final username = (post.merchantName ?? fallbackMerchantName ?? 'Hawkr')
        .trim();
    final dateStr = _formatDate(post.createdAt);
    final topPadding = MediaQuery.paddingOf(context).top;
    final bottomPadding = MediaQuery.paddingOf(context).bottom;
    final reduceMotion = MediaQuery.disableAnimationsOf(context);
    final motionDuration = reduceMotion
        ? Duration.zero
        : const Duration(milliseconds: 320);
    final sellerId = post.merchantId?.toString() ?? parentMerchantId;

    final captionExpanded = useState(false);

    void handleBack() {
      if (context.canPop()) {
        context.pop();
      } else {
        context.go('/?tab=1');
      }
    }

    useEffect(() {
      captionExpanded.value = false;
      return null;
    }, [post.id]);

    return LayoutBuilder(
      builder: (context, constraints) {
        final actionBarBottom =
            bottomPadding + PostPreviewLayout.actionBarBottomInset;
        final previewTop = topPadding + PostPreviewLayout.previewTopInset;
        final previewBottom = actionBarBottom;
        final previewHeight =
            constraints.maxHeight - previewTop - previewBottom;
        final mediaHeight = previewHeight - PostPreviewLayout.actionBarHeight;
        final panelMaxHeight = (mediaHeight * 0.60).clamp(280.0, 520.0);

        return Stack(
          children: [
            Positioned(
              top: previewTop,
              left: PostPreviewLayout.previewHorizontalInset,
              right: PostPreviewLayout.previewHorizontalInset,
              bottom: previewBottom,
              child: DecoratedBox(
                decoration: BoxDecoration(
                  borderRadius: BorderRadius.circular(HkRadius.xxl),
                  boxShadow: HkShadows.modal,
                ),
                child: ClipRRect(
                  borderRadius: BorderRadius.circular(HkRadius.xxl),
                  child: DecoratedBox(
                    key: ValueKey('post-preview-window-${post.id}'),
                    decoration: BoxDecoration(
                      color: AppColors.bgPrimary,
                      borderRadius: BorderRadius.circular(HkRadius.xxl),
                      border: Border.all(
                        color: AppColors.neutralWhite.withValues(alpha: 0.96),
                      ),
                    ),
                    child: Stack(
                      children: [
                        Positioned.fill(
                          child: _ImmersivePostMedia(
                            key: ValueKey('post-media-${post.id}'),
                            postId: post.id,
                            imageUrl: imageUrl,
                          ),
                        ),
                        Positioned.fill(
                          child: IgnorePointer(
                            ignoring: !captionExpanded.value,
                            child: Semantics(
                              button: captionExpanded.value,
                              label: captionExpanded.value ? '收起貼文說明' : null,
                              child: GestureDetector(
                                key: ValueKey(
                                  'post-caption-outside-dismiss-${post.id}',
                                ),
                                behavior: HitTestBehavior.opaque,
                                onTap: captionExpanded.value
                                    ? () => captionExpanded.value = false
                                    : null,
                                child: AnimatedOpacity(
                                  key: ValueKey('post-photo-scrim-${post.id}'),
                                  duration: motionDuration,
                                  curve: HkMotion.ease,
                                  opacity: captionExpanded.value ? 1 : 0,
                                  child: BackdropFilter(
                                    filter: ImageFilter.blur(
                                      sigmaX: 4,
                                      sigmaY: 4,
                                    ),
                                    child: ColoredBox(
                                      color: AppColors.neutralWhite.withValues(
                                        alpha: 0.38,
                                      ),
                                    ),
                                  ),
                                ),
                              ),
                            ),
                          ),
                        ),
                        Positioned(
                          left: PostPreviewLayout.panelHorizontalInset,
                          right: PostPreviewLayout.panelHorizontalInset,
                          bottom: PostPreviewLayout.panelBottomInset,
                          child: HkPostCardBodySurface(
                            surfaceKey: ValueKey(
                              'post-card-body-surface-${post.id}',
                            ),
                            child: Column(
                              mainAxisSize: MainAxisSize.min,
                              children: [
                                AnimatedSize(
                                  key: ValueKey('post-glass-panel-${post.id}'),
                                  duration: motionDuration,
                                  curve: HkMotion.ease,
                                  alignment: Alignment.bottomCenter,
                                  child: ConstrainedBox(
                                    constraints: BoxConstraints(
                                      maxHeight: panelMaxHeight,
                                    ),
                                    child: _PostContentPanel(
                                      post: post,
                                      username: username,
                                      dateStr: dateStr,
                                      sellerId: sellerId,
                                      fallbackMerchantAvatar:
                                          fallbackMerchantAvatar,
                                      captionExpanded: captionExpanded.value,
                                      onCaptionToggle: () {
                                        captionExpanded.value =
                                            !captionExpanded.value;
                                      },
                                    ),
                                  ),
                                ),
                                _BottomActionBar(
                                  key: ValueKey('post-action-bar-${post.id}'),
                                  postId: post.id,
                                  liked: post.liked,
                                  likeCount: post.likeCount,
                                  commentCount: post.commentCount,
                                  shareCount: post.shareCount,
                                  onLikeTap: onLikeTap,
                                  onCommentTap: onCommentTap,
                                  onShareTap: onShareTap,
                                ),
                              ],
                            ),
                          ),
                        ),
                        Positioned(
                          top: 12.h,
                          left: 12.w,
                          child: _CircleIconButton(
                            key: const ValueKey('post-back-button'),
                            semanticLabel: '關閉貼文',
                            onTap: handleBack,
                            child: const Icon(
                              AppIcons.close,
                              size: 30,
                              color: _kMediaForeground,
                            ),
                          ),
                        ),
                        Positioned(
                          top: 12.h,
                          right: 12.w,
                          child: HkFrostedIconButton(
                            label: '更多貼文操作',
                            onPressed: () async {
                              if (!await requireLogin(context, ref)) return;
                              if (!context.mounted) return;
                              showReportBlockActionSheet(
                                context,
                                reportTargetType: ReportTargetType.post,
                                reportTargetId: post.id.toString(),
                                blockUserId: sellerId.isNotEmpty
                                    ? sellerId
                                    : null,
                                blockUserLabel: username,
                              );
                            },
                            icon: AppIcons.moreHoriz,
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
      },
    );
  }
}

class _PostPreviewBackdrop extends StatelessWidget {
  const _PostPreviewBackdrop({super.key, required this.imageUrl});

  final String imageUrl;

  @override
  Widget build(BuildContext context) {
    return Stack(
      fit: StackFit.expand,
      children: [
        const ColoredBox(color: AppColors.bgPrimary),
        if (imageUrl.isNotEmpty)
          ImageFiltered(
            imageFilter: ImageFilter.blur(sigmaX: 34, sigmaY: 34),
            child: Transform.scale(
              scale: 1.22,
              child: AppCachedImage(
                imageUrl: imageUrl,
                fit: BoxFit.cover,
                memCacheWidth: 720,
                scene: ImageScene.fullscreen,
              ),
            ),
          ),
        Positioned.fill(
          child: ColoredBox(
            color: AppColors.neutralWhite.withValues(alpha: 0.66),
          ),
        ),
      ],
    );
  }
}

class _ImmersivePostMedia extends StatelessWidget {
  const _ImmersivePostMedia({
    super.key,
    required this.postId,
    required this.imageUrl,
  });

  final int postId;
  final String imageUrl;

  @override
  Widget build(BuildContext context) {
    return RepaintBoundary(
      child: Stack(
        fit: StackFit.expand,
        children: [
          const ColoredBox(color: AppColors.bgPrimary),
          if (imageUrl.isNotEmpty)
            ImageFiltered(
              imageFilter: ImageFilter.blur(sigmaX: 24, sigmaY: 24),
              child: Transform.scale(
                scale: 1.12,
                child: AppCachedImage(
                  imageUrl: imageUrl,
                  fit: BoxFit.cover,
                  memCacheWidth: 960,
                  scene: ImageScene.fullscreen,
                ),
              ),
            ),
          Positioned.fill(
            child: IgnorePointer(
              child: ColoredBox(
                color: AppColors.neutralWhite.withValues(alpha: 0.30),
              ),
            ),
          ),
          Hero(
            key: ValueKey('post-centered-media-$postId'),
            tag: 'post-image-$postId',
            flightShuttleBuilder: _heroShuttle,
            child: imageUrl.isEmpty
                ? Center(
                    child: Icon(
                      AppIcons.brokenImageOutlined,
                      size: 56.sp,
                      color: AppIconColors.muted,
                    ),
                  )
                : AppCachedImage(
                    key: ValueKey('post-image-content-$postId'),
                    imageUrl: imageUrl,
                    fit: BoxFit.contain,
                    memCacheWidth: 1440,
                    scene: ImageScene.fullscreen,
                  ),
          ),
          Positioned(
            left: 0,
            right: 0,
            top: 0,
            height: 180.h,
            child: IgnorePointer(
              child: DecoratedBox(
                decoration: BoxDecoration(
                  gradient: LinearGradient(
                    begin: Alignment.topCenter,
                    end: Alignment.bottomCenter,
                    colors: [
                      AppColors.neutralWhite.withValues(alpha: 0.76),
                      AppColors.neutralWhite.withValues(alpha: 0),
                    ],
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

class _PostContentPanel extends StatelessWidget {
  const _PostContentPanel({
    required this.post,
    required this.username,
    required this.dateStr,
    required this.sellerId,
    required this.fallbackMerchantAvatar,
    required this.captionExpanded,
    required this.onCaptionToggle,
  });

  final Post post;
  final String username;
  final String dateStr;
  final String sellerId;
  final String? fallbackMerchantAvatar;
  final bool captionExpanded;
  final VoidCallback onCaptionToggle;

  @override
  Widget build(BuildContext context) {
    final hasProduct = _hasValidProduct(post.product);
    return SingleChildScrollView(
      key: const ValueKey('post-content-scroll'),
      primary: false,
      physics: captionExpanded
          ? const BouncingScrollPhysics()
          : const NeverScrollableScrollPhysics(),
      padding: EdgeInsets.fromLTRB(12.w, 12.h, 12.w, 12.h),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          _MerchantLink(
            postId: post.id,
            username: username,
            dateStr: dateStr,
            avatarUrl: post.merchantAvatar ?? fallbackMerchantAvatar,
            storeCode: post.storeCode,
          ),
          if (hasProduct) ...[
            SizedBox(height: 8.h),
            _ProductOverlayChip(
              key: ValueKey('related-product-chip-${post.id}'),
              product: post.product!,
              merchantId: sellerId,
              merchantName: post.merchantName,
            ),
          ],
          if (post.content.isNotEmpty) ...[
            SizedBox(height: 8.h),
            Padding(
              padding: EdgeInsets.symmetric(horizontal: 4.w),
              child: _ExpandablePostText(
                key: ValueKey('post-caption-${post.id}'),
                content: post.content,
                expanded: captionExpanded,
                onToggle: onCaptionToggle,
              ),
            ),
          ],
        ],
      ),
    );
  }
}

class _MerchantLink extends StatelessWidget {
  const _MerchantLink({
    required this.postId,
    required this.username,
    required this.dateStr,
    required this.avatarUrl,
    required this.storeCode,
  });

  final int postId;
  final String username;
  final String dateStr;
  final String? avatarUrl;

  /// 店面網址的路徑段；空字串時連結停用（不退回用 id 定址）。
  final String storeCode;

  @override
  Widget build(BuildContext context) {
    final enabled = storeCode.isNotEmpty;

    return Semantics(
      button: enabled,
      label: enabled ? '進入$username店舖' : username,
      child: Material(
        color: Colors.transparent,
        child: InkWell(
          key: ValueKey('post-merchant-link-$postId'),
          onTap: enabled ? () => context.push('/merchant/$storeCode') : null,
          borderRadius: BorderRadius.circular(HkRadius.md),
          child: ConstrainedBox(
            constraints: const BoxConstraints(minHeight: HkControl.minTap),
            child: Padding(
              padding: EdgeInsets.symmetric(horizontal: 4.w, vertical: 4.h),
              child: Row(
                children: [
                  _Avatar(size: 42.sp, url: avatarUrl),
                  SizedBox(width: 12.w),
                  Expanded(
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          username,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: const TextStyle(
                            color: _kMediaForeground,
                            fontSize: 16,
                            fontWeight: FontWeight.w700,
                            height: 20 / 16,
                          ),
                        ),
                        if (dateStr.isNotEmpty)
                          Text(
                            dateStr,
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: const TextStyle(
                              color: _kMediaMuted,
                              fontSize: 14,
                              height: 18 / 14,
                            ),
                          ),
                      ],
                    ),
                  ),
                  if (enabled) ...[
                    SizedBox(width: 8.w),
                    const Text(
                      '進入店舖',
                      style: TextStyle(
                        color: _kMediaForeground,
                        fontSize: 14,
                        fontWeight: FontWeight.w700,
                      ),
                    ),
                    SizedBox(width: 4.w),
                    const Icon(
                      AppIcons.chevronRight,
                      size: 20,
                      color: _kMediaForeground,
                    ),
                  ],
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class _ExpandablePostText extends StatelessWidget {
  const _ExpandablePostText({
    super.key,
    required this.content,
    required this.expanded,
    required this.onToggle,
  });

  final String content;
  final bool expanded;
  final VoidCallback onToggle;

  @override
  Widget build(BuildContext context) {
    const textStyle = TextStyle(
      fontSize: 16,
      color: _kMediaForeground,
      height: 24 / 16,
    );

    return LayoutBuilder(
      builder: (context, constraints) {
        final span = TextSpan(children: buildHashtagSpans(content, textStyle));
        final collapsedPainter = TextPainter(
          text: span,
          maxLines: 1,
          textDirection: Directionality.of(context),
          textScaler: MediaQuery.textScalerOf(context),
        )..layout(maxWidth: constraints.maxWidth);
        final canToggle = collapsedPainter.didExceedMaxLines || expanded;

        final text = Text.rich(
          span,
          key: const ValueKey('post-caption-text'),
          maxLines: expanded ? null : 1,
          overflow: expanded ? TextOverflow.visible : TextOverflow.ellipsis,
        );

        if (!canToggle) return text;

        if (!expanded) {
          return Semantics(
            button: true,
            label: '查看貼文說明',
            value: '已收合',
            child: ConstrainedBox(
              constraints: const BoxConstraints(minHeight: HkControl.minTap),
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.center,
                children: [
                  Expanded(child: text),
                  SizedBox(width: 10.w),
                  Material(
                    color: Colors.transparent,
                    child: InkWell(
                      key: const ValueKey('post-caption-toggle'),
                      onTap: onToggle,
                      borderRadius: BorderRadius.circular(HkRadius.pill),
                      child: ConstrainedBox(
                        constraints: const BoxConstraints(
                          minHeight: HkControl.minTap,
                        ),
                        child: const Padding(
                          padding: EdgeInsets.only(left: 8),
                          child: Row(
                            mainAxisSize: MainAxisSize.min,
                            children: [
                              Text(
                                '查看說明',
                                style: TextStyle(
                                  color: AppColors.actionPrimary,
                                  fontSize: 14,
                                  fontWeight: FontWeight.w700,
                                  letterSpacing: 0.5,
                                  height: 20 / 14,
                                ),
                              ),
                              SizedBox(width: HkSpace.xs),
                              Icon(
                                AppIcons.keyboardArrowUp,
                                size: 18,
                                color: AppIconColors.active,
                              ),
                            ],
                          ),
                        ),
                      ),
                    ),
                  ),
                ],
              ),
            ),
          );
        }

        return Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            text,
            Align(
              alignment: Alignment.centerRight,
              child: Semantics(
                button: true,
                label: '收起貼文說明',
                value: '已展開',
                child: InkWell(
                  key: const ValueKey('post-caption-toggle'),
                  onTap: onToggle,
                  borderRadius: BorderRadius.circular(HkRadius.pill),
                  child: const SizedBox(
                    height: HkControl.minTap,
                    child: Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Text(
                          '收起說明',
                          style: TextStyle(
                            color: AppColors.actionPrimary,
                            fontSize: 14,
                            fontWeight: FontWeight.w700,
                            letterSpacing: 0.5,
                          ),
                        ),
                        SizedBox(width: HkSpace.xs),
                        Icon(
                          AppIcons.keyboardArrowDown,
                          size: 18,
                          color: AppIconColors.active,
                        ),
                      ],
                    ),
                  ),
                ),
              ),
            ),
          ],
        );
      },
    );
  }
}

// ─── 貼文卡內建互動列 ───────────────────────────────────────
class _BottomActionBar extends StatelessWidget {
  const _BottomActionBar({
    super.key,
    required this.postId,
    required this.liked,
    required this.likeCount,
    required this.commentCount,
    required this.shareCount,
    required this.onLikeTap,
    required this.onCommentTap,
    required this.onShareTap,
  });

  final int postId;
  final bool liked;
  final int likeCount;
  final int commentCount;
  final int shareCount;
  final VoidCallback onLikeTap;
  final VoidCallback onCommentTap;
  final VoidCallback onShareTap;

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      height: PostPreviewLayout.actionBarHeight,
      child: Material(
        color: Colors.transparent,
        child: Padding(
          key: ValueKey('post-action-toolbar-$postId'),
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
          child: Row(
            children: [
              Expanded(
                child: _IconCountAction(
                  key: ValueKey('post-like-action-$postId'),
                  semanticLabel: liked ? '取消按讚' : '按讚',
                  selected: liked,
                  iconBuilder: (size) => Icon(
                    liked ? AppIcons.favorite : AppIcons.favoriteBorder,
                    size: size,
                    color: liked ? AppIconColors.favorite : _kMediaForeground,
                  ),
                  count: likeCount,
                  onTap: onLikeTap,
                ),
              ),
              Expanded(
                child: _IconCountAction(
                  key: ValueKey('post-comment-action-$postId'),
                  semanticLabel: '查看留言',
                  iconBuilder: (size) => Icon(
                    AppIcons.chatBubbleOutline,
                    size: size,
                    color: _kMediaForeground,
                  ),
                  count: commentCount,
                  onTap: onCommentTap,
                ),
              ),
              Expanded(
                child: _IconCountAction(
                  key: ValueKey('post-share-action-$postId'),
                  semanticLabel: '分享貼文',
                  iconBuilder: (size) => Icon(
                    AppIcons.shareOutlined,
                    size: size,
                    color: _kMediaForeground,
                  ),
                  count: shareCount > 0 ? shareCount : null,
                  onTap: onShareTap,
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _IconCountAction extends StatelessWidget {
  const _IconCountAction({
    super.key,
    required this.iconBuilder,
    required this.semanticLabel,
    this.count,
    this.selected = false,
    required this.onTap,
  });
  final Widget Function(double size) iconBuilder;
  final String semanticLabel;
  final int? count;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Semantics(
      button: true,
      selected: selected,
      label: semanticLabel,
      child: GestureDetector(
        onTap: onTap,
        behavior: HitTestBehavior.opaque,
        child: AnimatedContainer(
          duration: const Duration(milliseconds: 180),
          constraints: const BoxConstraints(minHeight: HkControl.minTap),
          padding: EdgeInsets.symmetric(horizontal: 8.w, vertical: 8.h),
          child: Row(
            mainAxisAlignment: MainAxisAlignment.center,
            mainAxisSize: MainAxisSize.min,
            children: [
              iconBuilder(22.sp),
              if (count != null) ...[
                SizedBox(width: 4.w),
                Text(
                  '$count',
                  style: TextStyle(
                    fontSize: 14,
                    fontWeight: FontWeight.w500,
                    color: selected
                        ? AppColors.favoriteActive
                        : _kMediaForeground,
                    height: 20 / 14,
                  ),
                ),
              ],
            ],
          ),
        ),
      ),
    );
  }
}

// ─── 圓形 overlay icon（白色霧面玻璃）────────────────────────────────
class _CircleIconButton extends StatelessWidget {
  const _CircleIconButton({
    super.key,
    required this.child,
    required this.semanticLabel,
    required this.onTap,
  });

  final Widget child;
  final String semanticLabel;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Semantics(
      button: true,
      label: semanticLabel,
      child: SizedBox.square(
        dimension: HkControl.minTap,
        child: Center(
          child: Material(
            color: AppColors.neutralWhite.withValues(alpha: 0.74),
            shape: CircleBorder(
              side: BorderSide(
                color: AppColors.neutralWhite.withValues(alpha: 0.94),
              ),
            ),
            clipBehavior: Clip.antiAlias,
            child: InkWell(
              onTap: onTap,
              customBorder: const CircleBorder(),
              child: SizedBox.square(
                dimension: HkControl.minTap - HkSpace.xs,
                child: Center(child: child),
              ),
            ),
          ),
        ),
      ),
    );
  }
}

// ─── 商品卡片（圖片下方，inline） ─────────────────────────────────────
class _ProductOverlayChip extends StatelessWidget {
  const _ProductOverlayChip({
    super.key,
    required this.product,
    this.merchantId,
    this.merchantName,
  });

  final Map<String, dynamic> product;
  final String? merchantId;
  final String? merchantName;

  @override
  Widget build(BuildContext context) {
    final name = (product['name'] ?? '').toString();
    // 判準走 PriceDisplay：`"0"` = 無價格，不能印成 `NT$ 0`。
    final price = PriceDisplay.formatPrice(product['price'], prefix: 'NT\$ ');
    final imageUrl = _toAbsolute((product['image'] ?? '').toString());

    void openProduct() {
      context.push(
        '/merchant-product',
        extra: MerchantItemDetailScreen(
          title: name,
          imageUrl: imageUrl,
          price: num.tryParse(price),
          skuId: (product['id'] ?? '').toString(),
          merchantName: merchantName,
          merchantId: merchantId,
        ),
      );
    }

    return Semantics(
      button: true,
      label: '查看關聯商品 $name',
      child: Material(
        color: Colors.transparent,
        child: InkWell(
          onTap: openProduct,
          borderRadius: BorderRadius.circular(HkRadius.sm),
          child: ConstrainedBox(
            constraints: const BoxConstraints(minHeight: HkControl.minTap),
            child: Padding(
              padding: EdgeInsets.symmetric(horizontal: 4.w, vertical: 4.h),
              child: Row(
                children: [
                  ClipRRect(
                    borderRadius: BorderRadius.circular(HkRadius.sm),
                    child: ColoredBox(
                      color: AppColors.gray100,
                      child: SizedBox.square(
                        dimension: 44.w,
                        child: imageUrl.isNotEmpty
                            ? AppCachedImage(
                                imageUrl: imageUrl,
                                fit: BoxFit.cover,
                                scene: ImageScene.productThumbnail,
                              )
                            : Icon(
                                AppIcons.imageOutlined,
                                size: 20.sp,
                                color: AppIconColors.muted,
                              ),
                      ),
                    ),
                  ),
                  SizedBox(width: 8.w),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Text(
                          name,
                          style: const TextStyle(
                            fontSize: 14,
                            fontWeight: FontWeight.w700,
                            color: AppColors.textEmphasis,
                            height: 20 / 14,
                          ),
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                        ),
                        if (price.isNotEmpty)
                          Text(
                            price,
                            style: const TextStyle(
                              fontSize: 14,
                              fontWeight: FontWeight.w700,
                              color: AppColors.brandPrimaryStrong,
                              height: 20 / 14,
                            ),
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                          ),
                      ],
                    ),
                  ),
                  SizedBox(width: 8.w),
                  Icon(
                    AppIcons.chevronRight,
                    size: 16.sp,
                    color: AppIconColors.active,
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}

// ─── 圓形頭像（支援 base64 / URL / 預設） ────────────────────────────
class _Avatar extends StatelessWidget {
  const _Avatar({required this.size, this.url});
  final double size;
  final String? url;

  @override
  Widget build(BuildContext context) {
    final s = (url ?? '').trim();
    final border = BoxDecoration(
      shape: BoxShape.circle,
      border: Border.all(color: AppColors.gray300),
    );
    Widget defaultAvatar = ClipOval(
      child: SizedBox(
        width: size,
        height: size,
        child: HkPlaceholder(HkPlaceholderKind.buyerAvatar),
      ),
    );
    if (s.isEmpty) {
      return Container(
        width: size,
        height: size,
        decoration: border,
        clipBehavior: Clip.antiAlias,
        child: defaultAvatar,
      );
    }
    // inline 判準只看 `data:` scheme（media_image.dart isInlineImageData）。
    // 舊寫法「不是 http 且長度 > 60」會把 78~80 字元的 objectKey 誤判成 base64。
    final looksBase64 = isInlineImageData(s);
    Widget img;
    if (looksBase64) {
      try {
        final b64 = s.contains(',') ? s.split(',').last : s;
        final bytes = base64Decode(b64);
        img = Image.memory(
          bytes,
          fit: BoxFit.cover,
          width: size,
          height: size,
          gaplessPlayback: true,
        );
      } catch (_) {
        img = defaultAvatar;
      }
    } else {
      final resolved = (() {
        if (s.startsWith('http://') || s.startsWith('https://')) return s;
        if (s.startsWith('/member/avatar/view') ||
            s.startsWith('member/avatar/view') ||
            s.startsWith('/member-avatars') ||
            s.startsWith('member-avatars') ||
            s.startsWith('avatars/')) {
          return AvatarUtils.normalizeAvatarUrl(s);
        }
        return _toAbsolute(s);
      })();
      img = AppCachedImage(
        imageUrl: resolved,
        fit: BoxFit.cover,
        width: size,
        height: size,
        errorWidget: defaultAvatar,
        scene: ImageScene.avatarSmall,
      );
    }
    return Container(
      width: size,
      height: size,
      decoration: border,
      clipBehavior: Clip.antiAlias,
      child: ClipOval(child: img),
    );
  }
}

class _EmptyState extends StatelessWidget {
  const _EmptyState();
  @override
  Widget build(BuildContext context) {
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          const HkIcon(HkIconName.emptyPosts, size: 64),
          SizedBox(height: 12.h),
          Text('尚無貼文', style: TextStyle(fontSize: 16, color: _kGray100)),
        ],
      ),
    );
  }
}

class _ErrorState extends StatelessWidget {
  const _ErrorState({required this.message, required this.onRetry});
  final String message;
  final VoidCallback onRetry;
  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: EdgeInsets.symmetric(horizontal: 24.w),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Text(
              message,
              textAlign: TextAlign.center,
              style: TextStyle(fontSize: 14, color: _kGray100),
            ),
            SizedBox(height: 16.h),
            ElevatedButton(
              onPressed: onRetry,
              child: const HkButtonLabel('重試'),
            ),
          ],
        ),
      ),
    );
  }
}

// ─── helpers ──────────────────────────────────────────────────────────
String _toAbsolute(String? u) {
  if (u == null) return '';
  final s = u.trim();
  if (s.isEmpty) return '';
  if (s.startsWith('http://') || s.startsWith('https://')) return s;
  final b = Uri.parse(F.appFlavor.apiBaseUrl);
  final origin = '${b.scheme}://${b.host}${b.hasPort ? ':${b.port}' : ''}';
  if (s.startsWith('//')) return '${b.scheme}:$s';
  if (s.startsWith('/')) return '$origin$s';
  return '$origin/$s';
}

String _formatDate(dynamic dateVal) {
  if (dateVal == null) return '';
  DateTime? dt;
  if (dateVal is String) dt = DateTime.tryParse(dateVal);
  if (dateVal is int) {
    final ms = dateVal < 10000000000 ? dateVal * 1000 : dateVal;
    dt = DateTime.fromMillisecondsSinceEpoch(ms);
  }
  if (dt == null) return '';
  return '${dt.year}/${dt.month.toString().padLeft(2, '0')}/'
      '${dt.day.toString().padLeft(2, '0')}';
}

/// Hero flight shuttle — 飛行中固定用 destination 的 widget，避免兩端
/// （grid `_StableNetImage(NetworkImage)` vs feed `AppCachedImage`）在飛行
/// 過程中切換 widget 重新解碼造成 jank
Widget _heroShuttle(
  BuildContext flightContext,
  Animation<double> animation,
  HeroFlightDirection flightDirection,
  BuildContext fromHeroContext,
  BuildContext toHeroContext,
) {
  final Hero toHero = toHeroContext.widget as Hero;
  return toHero.child;
}
