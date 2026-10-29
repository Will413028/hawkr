import 'dart:convert';
import 'dart:ui' as ui;

import 'package:hawkr_buyer/shared/widgets/hk_frosted_icon_button.dart';
import 'package:hawkr_buyer/config/app_constants.dart';
import 'package:hawkr_buyer/config/flavor.dart';
import 'package:hawkr_buyer/config/image_config.dart';
import 'package:hawkr_buyer/config/media_image.dart';
import 'package:hawkr_buyer/core/exceptions/app_exception.dart';
import 'package:hawkr_buyer/core/extensions/url_extensions.dart';
import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/features/product/data/product_repository.dart';
import 'package:hawkr_buyer/features/product/presentation/merchant_item_detail_screen.dart';
import 'package:hawkr_buyer/features/seller/data/seller_repository.dart';
import 'package:hawkr_buyer/features/seller/data/store_card_display.dart';
import 'package:hawkr_buyer/features/seller/data/store_card_provider.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:hawkr_buyer/shared/utils/hashtag_spans.dart';
import 'package:hawkr_buyer/shared/utils/require_login.dart';
import 'package:hawkr_buyer/shared/widgets/app_cached_image.dart';
import 'package:hawkr_buyer/shared/widgets/comments_bottom_sheet.dart';
import 'package:hawkr_buyer/shared/widgets/hk_button_label.dart';
import 'package:hawkr_buyer/shared/widgets/hk_post_card_surface.dart';
import 'package:hawkr_buyer/shared/widgets/fruit_refresh_indicator.dart';
import 'package:hawkr_buyer/shared/widgets/share_bottom_sheet.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../moderation/data/report_target_type.dart';
import '../../moderation/presentation/report_block_action_sheet.dart';
import 'post_detail_controller.dart';
import 'post_preview_layout.dart';
import '../../../shared/utils/price_display.dart';
import 'package:hawkr_buyer/shared/widgets/hk_brand_loader.dart';

const _mediaForeground = AppColors.textEmphasis;
const _mediaMuted = AppColors.textSecondary;

bool _hasValidProduct(Map<String, dynamic>? product) {
  if (product == null) return false;
  final name = (product['name'] ?? '').toString().trim();
  // `"0"` 是後端「無價格」的表達且非空字串 —— 用 isNotEmpty 判斷會讓
  // 「無名稱＋無價格」的商品節點通過，渲染出一張空卡片。
  final price = PriceDisplay.formatPrice(product['price']);
  return name.isNotEmpty || price.isNotEmpty;
}

String _toAbsolute(String? value) {
  final source = (value ?? '').trim();
  if (source.isEmpty) return '';
  if (source.startsWith('http://') || source.startsWith('https://')) {
    return source;
  }
  final base = Uri.parse(F.appFlavor.apiBaseUrl);
  final origin =
      '${base.scheme}://${base.host}${base.hasPort ? ':${base.port}' : ''}';
  if (source.startsWith('//')) return '${base.scheme}:$source';
  if (source.startsWith('/')) return '$origin$source';
  return '$origin/$source';
}

String _formatDate(dynamic value) {
  if (value == null) return '';
  DateTime? date;
  if (value is String) date = DateTime.tryParse(value);
  if (value is int) {
    final milliseconds = value < 10000000000 ? value * 1000 : value;
    date = DateTime.fromMillisecondsSinceEpoch(milliseconds);
  }
  if (date == null) return '';
  return '${date.year}/${date.month.toString().padLeft(2, '0')}/'
      '${date.day.toString().padLeft(2, '0')}';
}

class PostDetailScreen extends HookConsumerWidget {
  const PostDetailScreen({
    super.key,
    required this.postId,
    this.merchantId,
    this.merchantName,
    this.merchantAvatar,
  });

  final int postId;
  final String? merchantId;
  final String? merchantName;
  final String? merchantAvatar;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final detailAsync = ref.watch(postDetailControllerProvider(postId));
    final resolvedMerchantName = useState(merchantName ?? '');
    final resolvedMerchantAvatar = useState(merchantAvatar ?? '');
    final captionExpanded = useState(false);

    useEffect(() {
      final sellerId = merchantId?.trim() ?? '';
      if (resolvedMerchantName.value.isNotEmpty || sellerId.isEmpty) {
        return null;
      }
      () async {
        try {
          final info = await ref
              .read(sellerRepositoryProvider)
              .getMerchantInfo(sellerId);
          if (info == null) return;
          // getMerchantInfo 回的是**整個 envelope**（`{code,message,data}`），
          // 要先鑽進 data；舊寫法直接讀頂層，欄位名與巢狀層級兩者皆錯。
          final data = info['data'];
          final seller = data is Map<String, dynamic>
              ? data
              : info.cast<String, dynamic>();
          // gateway `PublicSellerResponse`：店名是 storeName / sellerName，
          // 頭像是成對的 logoKey（裸 objectKey，SSOT）+ logoUrl。
          // 舊的 `name` / `avatarBase64` / `avatar` 三個 key wire 上都不存在。
          final name = (seller['storeName'] ?? seller['sellerName'] ?? '')
              .toString();
          final avatar = (seller['logoKey'] ?? seller['logoUrl'] ?? '')
              .toString();
          if (name.isNotEmpty) resolvedMerchantName.value = name;
          if (avatar.isNotEmpty) resolvedMerchantAvatar.value = avatar;
        } catch (_) {}
      }();
      return null;
    }, [merchantId]);

    useEffect(() {
      captionExpanded.value = false;
      return null;
    }, [postId]);

    void closeDetail() {
      if (context.canPop()) {
        context.pop();
      } else {
        context.go('/?tab=1');
      }
    }

    Widget pageBody;
    pageBody = detailAsync.when(
      loading: () => _PostDetailLoading(onClose: closeDetail),
      error: (error, _) => _PostDetailError(
        message: error.toString(),
        onClose: closeDetail,
        // 找不到不是暫時的失敗，重試只會得到同一個結果。
        onRetry: error is NotFoundException
            ? null
            : () => ref.invalidate(postDetailControllerProvider(postId)),
      ),
      data: (state) {
        final post = state.post;
        final apiName = (post.merchantName ?? '').trim();
        final apiAvatar = (post.merchantAvatar ?? '').trim();
        final displayName = apiName.isNotEmpty
            ? apiName
            : resolvedMerchantName.value.isNotEmpty
            ? resolvedMerchantName.value
            : 'Hawkr';
        final displayAvatar = apiAvatar.isNotEmpty
            ? apiAvatar
            : resolvedMerchantAvatar.value;
        final sellerId =
            post.merchantId?.toString() ?? merchantId?.trim() ?? '';
        // 詳情端點不帶 storeCode：sellerId 就是 store_id，經店家卡片（BatchGetStores）取得；
        // 沒有營業中的店面網址時不給連結，也不退回用 id 定址。
        final card = sellerId.isEmpty
            ? null
            : ref.watch(storeCardProvider(sellerId)).value;
        final storeCode = post.storeCode.isNotEmpty
            ? post.storeCode
            : (card != null && card.isOpen ? card.storeCode : '');
        final imageUrl = _toAbsolute(post.imageUrl);
        final date = _formatDate(post.createdAt);

        Future<void> openComments() async {
          await showModalBottomSheet<void>(
            context: context,
            isScrollControlled: true,
            useSafeArea: true,
            showDragHandle: false,
            backgroundColor: Colors.transparent,
            barrierColor: AppColors.neutralBlack.withValues(alpha: 0.48),
            builder: (_) => CommentsBottomSheet(
              postId: postId.toString(),
              postOwnerName: displayName,
              merchantAvatar: displayAvatar.isEmpty ? null : displayAvatar,
            ),
          );
          ref.invalidate(postDetailControllerProvider(postId));
        }

        Future<void> sharePost() async {
          final data = await ref
              .read(productRepositoryProvider)
              .getShareUrl(type: 'post', id: postId.toString());
          if (!context.mounted) return;
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
              title: (data?['title'] ?? displayName).toString(),
              description: (data?['description'] ?? post.content).toString(),
              imageUrl: (data?['imageUrl'] ?? imageUrl).toString(),
              url: url.toString(),
            ),
          );
        }

        return _PostDetailSurface(
          postId: postId,
          imageUrl: imageUrl,
          merchantId: sellerId,
          merchantName: displayName,
          merchantAvatar: displayAvatar,
          date: date,
          content: post.content,
          product: post.product,
          captionExpanded: captionExpanded.value,
          liked: post.liked,
          likeCount: post.likeCount,
          commentCount: post.commentCount,
          shareCount: post.shareCount,
          onClose: closeDetail,
          onMore: () async {
            if (!await requireLogin(context, ref)) return;
            if (!context.mounted) return;
            showReportBlockActionSheet(
              context,
              reportTargetType: ReportTargetType.post,
              reportTargetId: postId.toString(),
              blockUserId: sellerId.isNotEmpty ? sellerId : null,
              blockUserLabel: displayName,
            );
          },
          onMerchantTap: storeCode.isEmpty
              ? null
              : () => context.push('/merchant/$storeCode'),
          onCaptionToggle: () {
            captionExpanded.value = !captionExpanded.value;
          },
          onLikeTap: () async {
            if (!await requireLogin(context, ref)) return;
            await ref
                .read(postDetailControllerProvider(postId).notifier)
                .likePost();
          },
          onCommentTap: () async {
            if (!await requireLogin(context, ref)) return;
            if (!context.mounted) return;
            await openComments();
          },
          onShareTap: sharePost,
          onRefresh: () async {
            ref.invalidate(postDetailControllerProvider(postId));
            await ref.read(postDetailControllerProvider(postId).future);
          },
        );
      },
    );

    return AnnotatedRegion<SystemUiOverlayStyle>(
      value: AppColors.glassTopOverlay.copyWith(
        statusBarColor: Colors.transparent,
        systemNavigationBarColor: AppColors.navBarBackground,
        systemNavigationBarIconBrightness: Brightness.dark,
      ),
      child: pageBody,
    );
  }
}

class _PostDetailSurface extends StatelessWidget {
  const _PostDetailSurface({
    required this.postId,
    required this.imageUrl,
    required this.merchantId,
    required this.merchantName,
    required this.merchantAvatar,
    required this.date,
    required this.content,
    required this.product,
    required this.captionExpanded,
    required this.liked,
    required this.likeCount,
    required this.commentCount,
    required this.shareCount,
    required this.onClose,
    required this.onMore,
    required this.onMerchantTap,
    required this.onCaptionToggle,
    required this.onLikeTap,
    required this.onCommentTap,
    required this.onShareTap,
    required this.onRefresh,
  });

  final int postId;
  final String imageUrl;
  final String merchantId;
  final String merchantName;
  final String merchantAvatar;
  final String date;
  final String content;
  final Map<String, dynamic>? product;
  final bool captionExpanded;
  final bool liked;
  final int likeCount;
  final int commentCount;
  final int shareCount;
  final VoidCallback onClose;
  final VoidCallback onMore;
  final VoidCallback? onMerchantTap;
  final VoidCallback onCaptionToggle;
  final VoidCallback onLikeTap;
  final VoidCallback onCommentTap;
  final VoidCallback onShareTap;
  final Future<void> Function() onRefresh;

  @override
  Widget build(BuildContext context) {
    final topPadding = MediaQuery.paddingOf(context).top;
    final bottomPadding = MediaQuery.paddingOf(context).bottom;
    final reduceMotion = MediaQuery.disableAnimationsOf(context);
    final motionDuration = reduceMotion
        ? Duration.zero
        : const Duration(milliseconds: 320);

    return Scaffold(
      key: const ValueKey('post-detail-surface'),
      backgroundColor: Colors.transparent,
      body: SafeArea(
        top: false,
        bottom: false,
        child: LayoutBuilder(
          builder: (context, constraints) {
            final actionBarBottom =
                bottomPadding + PostPreviewLayout.actionBarBottomInset;
            final previewTop = topPadding + PostPreviewLayout.previewTopInset;
            final previewBottom = actionBarBottom;
            final previewHeight =
                constraints.maxHeight - previewTop - previewBottom;
            final mediaHeight =
                previewHeight - PostPreviewLayout.actionBarHeight;
            final panelMaxHeight = (mediaHeight * 0.58).clamp(280.0, 520.0);

            return Stack(
              children: [
                Positioned.fill(
                  child: const ColoredBox(color: Colors.transparent),
                ),
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
                      child: FruitRefreshIndicator(
                        onRefresh: onRefresh,
                        child: CustomScrollView(
                          key: const ValueKey('post-detail-scroll'),
                          physics: const AlwaysScrollableScrollPhysics(
                            parent: BouncingScrollPhysics(),
                          ),
                          slivers: [
                            SliverFillRemaining(
                              hasScrollBody: false,
                              child: DecoratedBox(
                                key: const ValueKey(
                                  'post-detail-preview-window',
                                ),
                                decoration: BoxDecoration(
                                  color: AppColors.bgPrimary,
                                  borderRadius: BorderRadius.circular(
                                    HkRadius.xxl,
                                  ),
                                  border: Border.all(
                                    color: AppColors.neutralWhite.withValues(
                                      alpha: 0.96,
                                    ),
                                  ),
                                ),
                                child: Stack(
                                  children: [
                                    Positioned.fill(
                                      child: _ImmersivePostMedia(
                                        postId: postId,
                                        imageUrl: imageUrl,
                                      ),
                                    ),
                                    Positioned.fill(
                                      child: IgnorePointer(
                                        ignoring: !captionExpanded,
                                        child: Semantics(
                                          button: captionExpanded,
                                          label: captionExpanded
                                              ? '收起貼文說明'
                                              : null,
                                          child: GestureDetector(
                                            key: const ValueKey(
                                              'post-detail-caption-outside-dismiss',
                                            ),
                                            behavior: HitTestBehavior.opaque,
                                            onTap: captionExpanded
                                                ? onCaptionToggle
                                                : null,
                                            child: AnimatedOpacity(
                                              key: const ValueKey(
                                                'post-detail-photo-scrim',
                                              ),
                                              duration: motionDuration,
                                              curve: HkMotion.ease,
                                              opacity: captionExpanded ? 1 : 0,
                                              child: BackdropFilter(
                                                filter: ui.ImageFilter.blur(
                                                  sigmaX: 4,
                                                  sigmaY: 4,
                                                ),
                                                child: ColoredBox(
                                                  color: AppColors.neutralWhite
                                                      .withValues(alpha: 0.38),
                                                ),
                                              ),
                                            ),
                                          ),
                                        ),
                                      ),
                                    ),
                                    Positioned(
                                      left: PostPreviewLayout
                                          .panelHorizontalInset,
                                      right: PostPreviewLayout
                                          .panelHorizontalInset,
                                      bottom:
                                          PostPreviewLayout.panelBottomInset,
                                      child: HkPostCardBodySurface(
                                        surfaceKey: const ValueKey(
                                          'post-detail-card-body-surface',
                                        ),
                                        child: Column(
                                          mainAxisSize: MainAxisSize.min,
                                          children: [
                                            AnimatedSize(
                                              key: const ValueKey(
                                                'post-detail-glass-panel',
                                              ),
                                              duration: motionDuration,
                                              curve: HkMotion.ease,
                                              alignment: Alignment.bottomCenter,
                                              child: ConstrainedBox(
                                                constraints: BoxConstraints(
                                                  maxHeight: panelMaxHeight,
                                                ),
                                                child: _PostContentPanel(
                                                  postId: postId,
                                                  merchantId: merchantId,
                                                  merchantName: merchantName,
                                                  merchantAvatar:
                                                      merchantAvatar,
                                                  date: date,
                                                  content: content,
                                                  product: product,
                                                  captionExpanded:
                                                      captionExpanded,
                                                  onMerchantTap: onMerchantTap,
                                                  onCaptionToggle:
                                                      onCaptionToggle,
                                                ),
                                              ),
                                            ),
                                            _PostActionBar(
                                              liked: liked,
                                              likeCount: likeCount,
                                              commentCount: commentCount,
                                              shareCount: shareCount,
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
                                      child: _OverlayIconButton(
                                        key: const ValueKey(
                                          'post-detail-close',
                                        ),
                                        semanticLabel: '關閉貼文',
                                        onTap: onClose,
                                        child: const Icon(
                                          AppIcons.close,
                                          size: 30,
                                          color: _mediaForeground,
                                        ),
                                      ),
                                    ),
                                    Positioned(
                                      top: 12.h,
                                      right: 12.w,
                                      child: HkFrostedIconButton(
                                        key: const ValueKey('post-detail-more'),
                                        label: '更多貼文操作',
                                        onPressed: onMore,
                                        icon: AppIcons.moreHoriz,
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
                  ),
                ),
              ],
            );
          },
        ),
      ),
    );
  }
}

class _ImmersivePostMedia extends StatelessWidget {
  const _ImmersivePostMedia({required this.postId, required this.imageUrl});

  final int postId;
  final String imageUrl;

  @override
  Widget build(BuildContext context) {
    return RepaintBoundary(
      key: ValueKey('post-detail-media-$postId'),
      child: Stack(
        fit: StackFit.expand,
        children: [
          const ColoredBox(color: AppColors.bgPrimary),
          if (imageUrl.isNotEmpty)
            ImageFiltered(
              imageFilter: ui.ImageFilter.blur(sigmaX: 24, sigmaY: 24),
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
            key: ValueKey('post-detail-centered-media-$postId'),
            tag: 'post-image-$postId',
            child: imageUrl.isEmpty
                ? const Center(
                    child: Icon(
                      AppIcons.imageNotSupportedOutlined,
                      size: 56,
                      color: _mediaMuted,
                    ),
                  )
                : AppCachedImage(
                    key: ValueKey('post-detail-image-$postId'),
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
    required this.postId,
    required this.merchantId,
    required this.merchantName,
    required this.merchantAvatar,
    required this.date,
    required this.content,
    required this.product,
    required this.captionExpanded,
    required this.onMerchantTap,
    required this.onCaptionToggle,
  });

  final int postId;
  final String merchantId;
  final String merchantName;
  final String merchantAvatar;
  final String date;
  final String content;
  final Map<String, dynamic>? product;
  final bool captionExpanded;
  final VoidCallback? onMerchantTap;
  final VoidCallback onCaptionToggle;

  @override
  Widget build(BuildContext context) {
    final hasProduct = _hasValidProduct(product);
    return SingleChildScrollView(
      key: const ValueKey('post-detail-content-scroll'),
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
            postId: postId,
            merchantName: merchantName,
            merchantAvatar: merchantAvatar,
            date: date,
            enabled: onMerchantTap != null,
            onTap: onMerchantTap,
          ),
          if (hasProduct) ...[
            SizedBox(height: 8.h),
            _RelatedProductChip(
              key: ValueKey('related-product-chip-$postId'),
              product: product!,
              merchantId: merchantId,
              merchantName: merchantName,
            ),
          ],
          if (content.isNotEmpty) ...[
            SizedBox(height: 8.h),
            Padding(
              padding: EdgeInsets.symmetric(horizontal: 4.w),
              child: _ExpandableCaption(
                content: content,
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
    required this.merchantName,
    required this.merchantAvatar,
    required this.date,
    required this.enabled,
    required this.onTap,
  });

  final int postId;
  final String merchantName;
  final String merchantAvatar;
  final String date;
  final bool enabled;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    return Semantics(
      button: enabled,
      label: enabled ? '進入$merchantName店舖' : merchantName,
      child: Material(
        color: Colors.transparent,
        child: InkWell(
          key: ValueKey('post-merchant-link-$postId'),
          onTap: onTap,
          borderRadius: BorderRadius.circular(HkRadius.md),
          child: ConstrainedBox(
            constraints: const BoxConstraints(minHeight: HkControl.minTap),
            child: Padding(
              padding: EdgeInsets.symmetric(horizontal: 4.w, vertical: 4.h),
              child: Row(
                children: [
                  _MerchantAvatar(
                    key: ValueKey('post-merchant-avatar-$postId'),
                    url: merchantAvatar,
                    size: 42,
                  ),
                  SizedBox(width: 12.w),
                  Expanded(
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          merchantName,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: const TextStyle(
                            color: _mediaForeground,
                            fontSize: 16,
                            fontWeight: FontWeight.w700,
                            height: 20 / 16,
                          ),
                        ),
                        if (date.isNotEmpty)
                          Text(
                            date,
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: const TextStyle(
                              color: _mediaMuted,
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
                        color: _mediaForeground,
                        fontSize: 14,
                        fontWeight: FontWeight.w700,
                      ),
                    ),
                    SizedBox(width: 4.w),
                    const Icon(
                      AppIcons.chevronRight,
                      size: 20,
                      color: _mediaForeground,
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

class _ExpandableCaption extends StatelessWidget {
  const _ExpandableCaption({
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
      color: _mediaForeground,
      fontSize: 16,
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
          key: const ValueKey('post-detail-caption-text'),
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
                      key: const ValueKey('post-detail-caption-toggle'),
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
                  key: const ValueKey('post-detail-caption-toggle'),
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

class _RelatedProductChip extends StatelessWidget {
  const _RelatedProductChip({
    super.key,
    required this.product,
    required this.merchantId,
    required this.merchantName,
  });

  final Map<String, dynamic> product;
  final String merchantId;
  final String merchantName;

  @override
  Widget build(BuildContext context) {
    final name = (product['name'] ?? '').toString().trim();
    // 判準走 PriceDisplay：`"0"` = 無價格，不能印成 `NT$ 0`。
    final price = PriceDisplay.formatPrice(product['price'], prefix: 'NT\$ ');
    final image = _toAbsolute((product['image'] ?? '').toString());

    return Semantics(
      button: true,
      label: '查看關聯商品 $name',
      child: Material(
        color: Colors.transparent,
        child: InkWell(
          onTap: () {
            context.push(
              '/merchant-product',
              extra: MerchantItemDetailScreen(
                title: name,
                imageUrl: image,
                price: num.tryParse(price),
                skuId: (product['id'] ?? '').toString(),
                merchantName: merchantName,
                merchantId: merchantId,
              ),
            );
          },
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
                        dimension: 42.w,
                        child: image.isEmpty
                            ? const Icon(AppIcons.imageOutlined, size: 20)
                            : AppCachedImage(
                                imageUrl: image,
                                fit: BoxFit.cover,
                                scene: ImageScene.productThumbnail,
                              ),
                      ),
                    ),
                  ),
                  SizedBox(width: 8.w),
                  Expanded(
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          name,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: const TextStyle(
                            color: AppColors.textEmphasis,
                            fontSize: 14,
                            fontWeight: FontWeight.w700,
                            height: 18 / 14,
                          ),
                        ),
                        if (price.isNotEmpty)
                          Text(
                            price,
                            maxLines: 1,
                            style: const TextStyle(
                              color: AppColors.brandPrimaryStrong,
                              fontSize: 14,
                              fontWeight: FontWeight.w700,
                              height: 18 / 14,
                            ),
                          ),
                      ],
                    ),
                  ),
                  SizedBox(width: 8.w),
                  const Icon(
                    AppIcons.chevronRight,
                    size: 18,
                    color: AppIconColors.strong,
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

class _PostActionBar extends StatelessWidget {
  const _PostActionBar({
    required this.liked,
    required this.likeCount,
    required this.commentCount,
    required this.shareCount,
    required this.onLikeTap,
    required this.onCommentTap,
    required this.onShareTap,
  });

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
      key: const ValueKey('post-detail-action-bar'),
      height: PostPreviewLayout.actionBarHeight,
      child: Material(
        color: Colors.transparent,
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
          child: Row(
            children: [
              Expanded(
                child: _PostAction(
                  key: const ValueKey('post-detail-like'),
                  semanticLabel: liked ? '取消按讚' : '按讚',
                  count: likeCount,
                  selected: liked,
                  onTap: onLikeTap,
                  icon: Icon(
                    liked ? AppIcons.favorite : AppIcons.favoriteBorder,
                    size: 22,
                    color: liked ? AppIconColors.favorite : _mediaForeground,
                  ),
                ),
              ),
              Expanded(
                child: _PostAction(
                  key: const ValueKey('post-detail-comment'),
                  semanticLabel: '查看留言',
                  count: commentCount,
                  onTap: onCommentTap,
                  icon: const Icon(
                    AppIcons.chatBubbleOutline,
                    size: 22,
                    color: _mediaForeground,
                  ),
                ),
              ),
              Expanded(
                child: _PostAction(
                  key: const ValueKey('post-detail-share'),
                  semanticLabel: '分享貼文',
                  count: shareCount,
                  onTap: onShareTap,
                  icon: const Icon(
                    AppIcons.shareOutlined,
                    size: 22,
                    color: _mediaForeground,
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

class _PostAction extends StatelessWidget {
  const _PostAction({
    super.key,
    required this.semanticLabel,
    required this.count,
    required this.icon,
    required this.onTap,
    this.selected = false,
  });

  final String semanticLabel;
  final int count;
  final Widget icon;
  final VoidCallback onTap;
  final bool selected;

  @override
  Widget build(BuildContext context) {
    return Semantics(
      button: true,
      selected: selected,
      label: semanticLabel,
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(HkRadius.pill),
        child: ConstrainedBox(
          constraints: const BoxConstraints(minHeight: HkControl.minTap),
          child: Row(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              icon,
              SizedBox(width: 4.w),
              Text(
                '$count',
                style: TextStyle(
                  color: selected ? AppColors.favoriteActive : _mediaForeground,
                  fontSize: 14,
                  fontWeight: FontWeight.w500,
                  height: 20 / 14,
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _OverlayIconButton extends StatelessWidget {
  const _OverlayIconButton({
    super.key,
    required this.semanticLabel,
    required this.onTap,
    required this.child,
  });

  final String semanticLabel;
  final VoidCallback onTap;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    return Semantics(
      button: true,
      label: semanticLabel,
      child: RepaintBoundary(
        child: DecoratedBox(
          decoration: BoxDecoration(
            shape: BoxShape.circle,
            boxShadow: HkShadows.sm,
          ),
          child: ClipOval(
            child: BackdropFilter(
              filter: ui.ImageFilter.blur(
                sigmaX: HkGlass.tabIndicatorBlurSigma,
                sigmaY: HkGlass.tabIndicatorBlurSigma,
              ),
              child: DecoratedBox(
                decoration: BoxDecoration(
                  color: AppColors.neutralWhite.withValues(alpha: 0.74),
                  shape: BoxShape.circle,
                  border: Border.all(
                    color: AppColors.neutralWhite.withValues(alpha: 0.94),
                  ),
                ),
                child: Material(
                  color: Colors.transparent,
                  child: InkWell(
                    onTap: onTap,
                    customBorder: const CircleBorder(),
                    child: SizedBox.square(
                      dimension: HkControl.minTap,
                      child: Center(child: child),
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

class _MerchantAvatar extends StatelessWidget {
  const _MerchantAvatar({super.key, required this.url, required this.size});

  final String url;
  final double size;

  @override
  Widget build(BuildContext context) {
    final source = url.trim();
    Widget image;
    if (source.isEmpty) {
      image = const Icon(
        AppIcons.storefrontOutlined,
        color: AppIconColors.onBrand,
      );
    } else if (isInlineImageData(source)) {
      // inline 判準只看 `data:` scheme。舊寫法是「不是 http 且長度 > 60」，而
      // objectKey 本來就 78~80 字元（media_image.dart isInlineImageData 的註解），
      // 商家 logoKey 一路被誤判成 base64、decode 失敗後靜默退回預設 icon。
      try {
        final bytes = base64Decode(
          source.contains(',') ? source.split(',').last : source,
        );
        image = Image.memory(bytes, fit: BoxFit.cover, gaplessPlayback: true);
      } catch (_) {
        image = const Icon(
          AppIcons.storefrontOutlined,
          color: AppIconColors.onBrand,
        );
      }
    } else {
      final normalized =
          source.startsWith('/member/avatar/view') ||
              source.startsWith('member/avatar/view') ||
              source.startsWith('/member-avatars') ||
              source.startsWith('member-avatars') ||
              source.startsWith('avatars/')
          ? source.normalizeAvatarUrl()
          : _toAbsolute(source);
      image = AppCachedImage(
        imageUrl: normalized,
        fit: BoxFit.cover,
        scene: ImageScene.avatarSmall,
        errorWidget: const Icon(
          AppIcons.storefrontOutlined,
          color: AppIconColors.onBrand,
        ),
      );
    }

    return DecoratedBox(
      decoration: BoxDecoration(
        shape: BoxShape.circle,
        border: Border.all(color: AppColors.borderSoft, width: 1.5),
      ),
      child: ClipOval(
        child: SizedBox.square(dimension: size, child: image),
      ),
    );
  }
}

class _PostDetailLoading extends StatelessWidget {
  const _PostDetailLoading({required this.onClose});

  final VoidCallback onClose;

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.transparent,
      body: SafeArea(
        child: Stack(
          children: [
            const Center(child: HkBrandLoader(size: HkBrandLoader.page)),
            Positioned(
              top: 12.h,
              left: 16.w,
              child: _OverlayIconButton(
                semanticLabel: '關閉貼文',
                onTap: onClose,
                child: const Icon(
                  AppIcons.close,
                  size: 30,
                  color: _mediaForeground,
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _PostDetailError extends StatelessWidget {
  const _PostDetailError({
    required this.message,
    required this.onClose,
    required this.onRetry,
  });

  final String message;
  final VoidCallback onClose;
  final VoidCallback? onRetry;

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.transparent,
      body: SafeArea(
        child: Stack(
          children: [
            Center(
              child: Padding(
                padding: EdgeInsets.symmetric(horizontal: 32.w),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Text(
                      message,
                      textAlign: TextAlign.center,
                      style: const TextStyle(color: _mediaMuted),
                    ),
                    if (onRetry != null) ...[
                      SizedBox(height: 16.h),
                      FilledButton(
                        onPressed: onRetry,
                        child: const HkButtonLabel('重試'),
                      ),
                    ],
                  ],
                ),
              ),
            ),
            Positioned(
              top: 12.h,
              left: 16.w,
              child: _OverlayIconButton(
                semanticLabel: '關閉貼文',
                onTap: onClose,
                child: const Icon(
                  AppIcons.close,
                  size: 30,
                  color: _mediaForeground,
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
