import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../../core/router/app_router.dart';
import '../../../../core/router/tab_refresh_provider.dart';
import '../../../../core/motion/hk_motion_transitions.dart';
import '../../../../core/theme/app_colors.dart';
import '../../../../core/theme/app_typography.dart';
import '../../../../core/theme/hk_tokens.dart';
import '../../../../shared/utils/hk_log.dart';
import '../../../../config/media_image.dart';
import '../../../../shared/utils/floating_toast.dart';
import '../../../../shared/widgets/comments_bottom_sheet.dart';
import '../../../../shared/widgets/hk_icon.dart';
import '../../../../shared/widgets/hk_tap_target.dart';
import '../../../../shared/widgets/fruit_refresh_indicator.dart';
import '../../../../shared/widgets/linked_product_card.dart';
import '../../../../shared/widgets/post_video_player.dart';
import '../../../../shared/widgets/post_video_thumbnail.dart';
import '../../../../shared/widgets/seller_floating_navigation_layout.dart';
import '../../../seller/data/current_store.dart';
import '../../data/post_comments_availability.dart';
import '../../data/post_hidden.dart';
import '../../data/post_model.dart';
import '../../data/post_repository.dart';
import 'hidden_post_badge.dart';
import 'storefront_tab_glass.dart';

/// 社群(首頁) Tab — 貼文九宮格、長按預覽卡、按讚/留言邏輯。
///
/// 貼文的全螢幕瀏覽與編輯/刪除入口都在 `/post/feed`（[PageHomeFeedScreen]），
/// 點格子進去；本 Tab 只負責 grid 與預覽。
class PageHomeTab extends ConsumerStatefulWidget {
  const PageHomeTab({
    super.key,
    this.outerHandle,
    this.sellerName,
    required this.onReloadSeller,
  });

  /// NestedScrollView 的 overlap absorber handle，用於 SliverOverlapInjector
  final SliverOverlapAbsorberHandle? outerHandle;
  final String? sellerName;

  /// Callback to refresh seller data in the parent (e.g. pull-to-refresh)
  final Future<void> Function() onReloadSeller;

  @override
  ConsumerState<PageHomeTab> createState() => _PageHomeTabState();
}

class _PageHomeTabState extends ConsumerState<PageHomeTab>
    with AutomaticKeepAliveClientMixin {
  @override
  bool get wantKeepAlive => true;
  PostRepository get _postRepo => ref.read(postRepositoryProvider);

  static const _postsPageSize = 20;

  List<Map<String, dynamic>> _posts = [];
  bool _loadingPosts = false;
  bool _loadingMorePosts = false;
  bool _hasMorePosts = true;
  int _postsPage = 1;

  final Map<int, int> _postCommentCount = <int, int>{};
  final Set<int> _commentsInitRequested = <int>{};
  final Map<int, ValueNotifier<int>> _commentCountNotifiers =
      <int, ValueNotifier<int>>{};

  Offset? _lastTapDownPosition;

  @override
  void initState() {
    super.initState();
    Future.microtask(_loadPosts);
  }

  @override
  void dispose() {
    for (final n in _commentCountNotifiers.values) {
      n.dispose();
    }
    super.dispose();
  }

  ValueNotifier<int> _ensureCommentNotifier(int postId, int initial) {
    final existing = _commentCountNotifiers[postId];
    if (existing != null) {
      if (existing.value != initial) existing.value = initial;
      return existing;
    }
    final notifier = ValueNotifier<int>(initial);
    _commentCountNotifiers[postId] = notifier;
    return notifier;
  }

  void _applyCommentCount(int postId, int count) {
    _postCommentCount[postId] = count;
    final notifier = _commentCountNotifiers[postId];
    if (notifier != null) {
      if (notifier.value != count) notifier.value = count;
    } else {
      _commentCountNotifiers[postId] = ValueNotifier<int>(count);
    }
  }

  void _pruneCommentNotifiers(Set<int> activeIds) {
    final staleIds = _commentCountNotifiers.keys
        .where((id) => !activeIds.contains(id))
        .toList();
    for (final id in staleIds) {
      _commentCountNotifiers.remove(id)?.dispose();
      _postCommentCount.remove(id);
    }
  }

  /// 同步留言數 notifier；`prune=true`（整頁重置時）才清掉不在新清單裡的 id，
  /// 分頁 append 時只會新增，不能拿「這一頁」當全集去清掉前面頁面的 notifier。
  void _syncCommentNotifiers(
    List<Map<String, dynamic>> posts, {
    required bool prune,
  }) {
    final Map<int, int> nextCommentCounts = <int, int>{};
    final Set<int> activeIds = <int>{};
    for (final p in posts) {
      final dynamic rawId = p['id'] ?? p['postId'];
      final int? postId = (rawId is num)
          ? rawId.toInt()
          : int.tryParse('${rawId ?? ''}');
      if (postId == null) continue;
      activeIds.add(postId);
      nextCommentCounts[postId] = postCommentCount(p);
    }
    if (prune) _pruneCommentNotifiers(activeIds);
    for (final id in activeIds) {
      final initial = nextCommentCounts[id] ?? _postCommentCount[id] ?? 0;
      _ensureCommentNotifier(id, initial);
    }
    for (final entry in nextCommentCounts.entries) {
      _applyCommentCount(entry.key, entry.value);
    }
  }

  /// 整頁重置：拉第 1 頁，取代 `_posts`。用於初次載入、下拉重新整理、
  /// 新增/編輯/刪除貼文後——這些情境都預期看到最新狀態，重置回第 1 頁合理。
  Future<void> _loadPosts() async {
    if (_loadingPosts) return;
    setState(() => _loadingPosts = true);
    try {
      final result = await _postRepo.listMyPostsRaw(
        page: 1,
        pageSize: _postsPageSize,
      );
      if (!mounted) return;
      setState(() {
        _posts = result.posts;
        _postsPage = 1;
        _hasMorePosts = result.hasMore;
        _loadingPosts = false;
        _syncCommentNotifiers(result.posts, prune: true);
      });
      HKLog.logD('[Home] Posts loaded (${_posts.length})');
    } catch (e) {
      HKLog.logD('[Home] Load posts failed: $e');
      if (mounted) setState(() => _loadingPosts = false);
    }
  }

  /// 捲到接近底部觸發：拉下一頁 append 到 `_posts`，不重置既有資料/捲動位置。
  Future<void> _loadMorePosts() async {
    if (_loadingPosts || _loadingMorePosts || !_hasMorePosts) return;
    setState(() => _loadingMorePosts = true);
    final nextPage = _postsPage + 1;
    try {
      final result = await _postRepo.listMyPostsRaw(
        page: nextPage,
        pageSize: _postsPageSize,
      );
      if (!mounted) return;
      setState(() {
        _posts = [..._posts, ...result.posts];
        _postsPage = nextPage;
        _hasMorePosts = result.hasMore;
        _loadingMorePosts = false;
        _syncCommentNotifiers(result.posts, prune: false);
      });
      HKLog.logD(
        '[Home] Loaded more posts, page $nextPage (${_posts.length} total)',
      );
    } catch (e) {
      HKLog.logD('[Home] Load more posts failed: $e');
      if (mounted) setState(() => _loadingMorePosts = false);
    }
  }

  Future<void> _openPostComposer() async {
    final created = await context.push<Map<String, dynamic>>(
      '/post/media-picker',
    );
    if (created != null) {
      await _loadPosts();
      if (mounted) {
        FloatingToast.success(context, '已發佈');
      }
    }
  }

  void _openFullScreenFeedForPost(Map<String, dynamic> post) {
    final rawId = post['id'] ?? post['postId'];
    final int? postId = rawId is int ? rawId : int.tryParse('${rawId ?? ''}');
    context
        .push(
          PostRoutes.feed,
          extra: {'postId': postId, 'sellerName': widget.sellerName ?? ''},
        )
        .then((_) async {
          await _loadPosts();
        });
  }

  void _showHiddenReasonSheet(BuildContext context, String? reason) {
    showModalBottomSheet<void>(
      context: context,
      // 貼文 tab 位於 `StatefulShellRoute` 分支底下，分支 Navigator 在浮動導覽列
      // 之下；不掛根 Navigator 的話 sheet 底部會被導覽列蓋住（#132 同型）。
      useRootNavigator: true,
      backgroundColor: AppColors.bgCard,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(16.r)),
      ),
      builder: (_) => SafeArea(
        child: Padding(
          padding: EdgeInsets.all(20.w),
          child: Container(
            width: double.infinity,
            padding: EdgeInsets.all(16.w),
            decoration: BoxDecoration(
              color: AppColors.bgCard,
              borderRadius: BorderRadius.circular(12.r),
            ),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  '貼文已下架',
                  style: TextStyle(
                    fontSize: 14.sp,
                    fontWeight: FontWeight.w700,
                    color: AppColors.danger,
                  ),
                ),
                SizedBox(height: 8.h),
                Text(
                  reason?.isNotEmpty == true
                      ? reason!
                      : '此貼文因違反平台規範已被下架，如有疑問請聯繫客服。',
                  style: TextStyle(fontSize: 14.sp, color: AppColors.gray1),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  void _showPostPreviewAt(Offset globalPos, Map<String, dynamic> post) {
    final size = MediaQuery.of(context).size;
    const double margin = 12;
    double popWidth = size.width - margin * 2;
    if (popWidth > 360) popWidth = 360;
    final double maxHeight = size.height * 0.8;

    double left = globalPos.dx - popWidth / 2;
    if (left < margin) left = margin;
    if (left + popWidth > size.width - margin) {
      left = size.width - popWidth - margin;
    }

    double top = globalPos.dy - maxHeight / 2;
    if (top < margin) top = margin;
    if (top + maxHeight > size.height - margin) {
      top = size.height - maxHeight - margin;
    }

    showGeneralDialog(
      context: context,
      barrierLabel: 'PostPreview',
      barrierDismissible: true,
      barrierColor: AppColors.textEmphasis.withValues(alpha: 0.2),
      // iOS 的預覽從按下的位置展開，像 contextual preview。
      transitionDuration: HkMotionTransitions.durationOf(context),
      pageBuilder: (ctx, anim, secAnim) {
        return Builder(
          builder: (dialogContext) {
            return Material(
              type: MaterialType.transparency,
              child: Stack(
                children: [
                  Positioned.fill(
                    child: GestureDetector(
                      onTap: () => Navigator.of(ctx).pop(),
                    ),
                  ),
                  Positioned(
                    left: left,
                    top: top,
                    child: Container(
                      width: popWidth,
                      constraints: BoxConstraints(maxHeight: maxHeight),
                      child: Material(
                        color: AppColors.bgCard,
                        elevation: 8,
                        borderRadius: BorderRadius.circular(16.r),
                        clipBehavior: Clip.antiAlias,
                        child: SingleChildScrollView(
                          child: ConstrainedBox(
                            constraints: BoxConstraints(minWidth: popWidth),
                            child: Padding(
                              padding: EdgeInsets.only(bottom: 8.h),
                              child: Consumer(
                                builder: (context, ref, _) =>
                                    _igPostCardFromPost(
                                      post,
                                      ref.watch(storeShareProvider),
                                    ),
                              ),
                            ),
                          ),
                        ),
                      ),
                    ),
                  ),
                ],
              ),
            );
          },
        );
      },
      transitionBuilder: (transitionContext, animation, __, child) =>
          HkMotionTransitions.modal(
            context: transitionContext,
            animation: animation,
            presentation: HkModalPresentation.popup,
            alignment: HkMotionTransitions.alignmentForGlobalPosition(
              transitionContext,
              globalPos,
            ),
            offset: Offset.zero,
            beginScale: 0.96,
            child: child,
          ),
    );
  }

  ImageProvider? _imageFromPost(Map<String, dynamic> p) {
    // 欄位形狀與收斂順序見 [postImageSources]：詳情與更新端點只回複數欄位，
    // 只讀單數會整片掉圖。
    final sources = postImageSources(p);
    if (sources.isEmpty) return null;
    return NetworkImage(buildPostImageUrl(sources.first));
  }

  String? _videoUrlFromPost(Map<String, dynamic> p) {
    String? url = p['videoUrl']?.toString();
    if (url == null || url.isEmpty) {
      url = p['video_url']?.toString();
    }
    final dynamic contentAny = p['content'];
    if ((url == null || url.isEmpty) && contentAny is Map) {
      url =
          contentAny['videoUrl']?.toString() ??
          contentAny['video_url']?.toString();
    }
    if (url != null && url.isNotEmpty) {
      // 影片走 original preset（Worker 不轉換，非圖片檔的唯一合法選項）。
      final resolved = buildImageUrl(url, ImagePreset.original);
      return resolved.isEmpty ? null : resolved;
    }
    return null;
  }

  @override
  Widget build(BuildContext context) {
    super.build(context);
    ref.listen<int>(pageTabRefreshProvider, (_, __) {
      _loadPosts();
    });
    return _homeWithFab();
  }

  Widget _homeWithFab() {
    final Widget content = _homeGrid();
    final navigationClearance = sellerFloatingNavigationClearance(context);
    return Stack(
      children: [
        Positioned.fill(child: content),
        Positioned(
          left: 0,
          right: 0,
          bottom: navigationClearance,
          child: Center(
            child: StorefrontGlassActionButton(
              icon: HkIconName.add,
              label: '新增貼文',
              onTap: _openPostComposer,
            ),
          ),
        ),
      ],
    );
  }

  /// 捲動接近底部（扣掉留白區還剩不到一個螢幕高）時觸發下一頁載入。
  /// 用 NotificationListener 而非自帶 ScrollController——這個 CustomScrollView
  /// 吃的是外層 NestedScrollView 的 PrimaryScrollController，監聽冒泡通知不用
  /// 管實際 controller 是誰持有。
  bool _onGridScrollNotification(ScrollNotification notification) {
    if (notification.metrics.extentAfter < 600) {
      _loadMorePosts();
    }
    return false;
  }

  Widget _homeGrid() {
    return FruitRefreshIndicator(
      onRefresh: widget.onReloadSeller,
      child: NotificationListener<ScrollNotification>(
        onNotification: _onGridScrollNotification,
        child: CustomScrollView(
          key: const PageStorageKey('storefront-post-grid-scroll'),
          slivers: [
            if (widget.outerHandle != null)
              SliverOverlapInjector(handle: widget.outerHandle!),
            SliverToBoxAdapter(child: SizedBox(height: 24.h)),
            _buildPostGridSliver(),
            _buildPostGridFooterSliver(),
            SliverToBoxAdapter(child: SizedBox(height: 80.h)),
          ],
        ),
      ),
    );
  }

  Widget _buildPostGridFooterSliver() {
    if (_posts.isEmpty) {
      return const SliverToBoxAdapter(child: SizedBox.shrink());
    }
    if (_loadingMorePosts) {
      return SliverToBoxAdapter(
        child: Padding(
          padding: EdgeInsets.symmetric(vertical: 16.h),
          child: const Center(
            child: SizedBox(
              width: 20,
              height: 20,
              child: CircularProgressIndicator(strokeWidth: 2),
            ),
          ),
        ),
      );
    }
    if (!_hasMorePosts) {
      return SliverToBoxAdapter(
        child: Padding(
          padding: EdgeInsets.symmetric(vertical: 16.h),
          child: Center(child: Text('已顯示全部貼文', style: AppTypography.caption)),
        ),
      );
    }
    return const SliverToBoxAdapter(child: SizedBox.shrink());
  }

  Widget _buildPostGridSliver() {
    const delegate = SliverGridDelegateWithFixedCrossAxisCount(
      crossAxisCount: 3,
      mainAxisSpacing: 12,
      crossAxisSpacing: 12,
      childAspectRatio: 1,
    );
    final padding = EdgeInsets.fromLTRB(16.w, 0, 16.w, 16.h);

    if (_posts.isEmpty && _loadingPosts) {
      return SliverPadding(
        padding: padding,
        sliver: SliverGrid(
          gridDelegate: delegate,
          delegate: SliverChildBuilderDelegate(
            (_, index) => ClipRRect(
              key: ValueKey('storefront-post-grid-skeleton-$index'),
              borderRadius: BorderRadius.circular(HkRadius.lg),
              child: const SizedBox.expand(
                child: DecoratedBox(
                  decoration: BoxDecoration(color: AppColors.dividerLight),
                ),
              ),
            ),
            childCount: 9,
          ),
        ),
      );
    }

    if (_posts.isEmpty) {
      return const SliverToBoxAdapter(
        child: StorefrontGlassEmptyPanel(
          icon: HkIconName.post,
          message: '暫無貼文',
          description: '發布貼文後會顯示在這裡。',
        ),
      );
    }

    return SliverPadding(
      padding: padding,
      sliver: SliverGrid(
        gridDelegate: delegate,
        delegate: SliverChildBuilderDelegate(
          (_, index) => _buildPostGridTile(_posts[index]),
          childCount: _posts.length,
        ),
      ),
    );
  }

  Widget _buildPostGridTile(Map<String, dynamic> post) {
    final img = _imageFromPost(post);
    final ct = post['contentType']?.toString();
    final hidden = isPostHidden(post);
    final postId = (post['id'] ?? post['postId'] ?? '').toString();
    final borderRadius = BorderRadius.circular(HkRadius.lg);

    return Material(
      key: ValueKey('storefront-post-grid-item-$postId'),
      color: Colors.transparent,
      borderRadius: borderRadius,
      child: InkWell(
        borderRadius: borderRadius,
        onTap: hidden
            ? () => _showHiddenReasonSheet(context, hiddenReasonOf(post))
            : () => _openFullScreenFeedForPost(post),
        onTapDown: (details) => _lastTapDownPosition = details.globalPosition,
        onLongPress: () {
          final pos =
              _lastTapDownPosition ??
              (Offset.zero & MediaQuery.of(context).size).center;
          _showPostPreviewAt(pos, post);
        },
        child: ClipRRect(
          borderRadius: borderRadius,
          child: SizedBox.expand(
            child: Stack(
              fit: StackFit.expand,
              children: [
                _buildPostGridMedia(post, img, ct),
                if (hidden) ...[
                  Positioned.fill(
                    child: ColoredBox(
                      color: AppColors.textEmphasis.withValues(alpha: 0.45),
                    ),
                  ),
                  Positioned(
                    top: 6.h,
                    left: 6.w,
                    child: const HiddenPostBadge(),
                  ),
                ],
              ],
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildPostGridMedia(
    Map<String, dynamic> post,
    ImageProvider<Object>? image,
    String? contentType,
  ) {
    if (contentType == 'video') {
      if (image != null) {
        return Stack(
          fit: StackFit.expand,
          children: [
            Image(image: image, fit: BoxFit.cover),
            _buildVideoBadge(),
          ],
        );
      }
      final videoUrl = _videoUrlFromPost(post);
      if (videoUrl != null && videoUrl.isNotEmpty) {
        return Stack(
          fit: StackFit.expand,
          children: [
            PostVideoThumbnail(url: videoUrl),
            _buildVideoBadge(),
          ],
        );
      }
      return const DecoratedBox(
        decoration: BoxDecoration(color: AppColors.dividerLight),
        child: Center(
          child: Icon(AppIcons.videocam, color: AppIconColors.onBrand),
        ),
      );
    }

    if (image == null) {
      return const DecoratedBox(
        decoration: BoxDecoration(color: AppColors.dividerLight),
        child: Center(
          child: Icon(
            AppIcons.imageNotSupportedOutlined,
            color: AppIconColors.onBrand,
          ),
        ),
      );
    }
    return Image(image: image, fit: BoxFit.cover);
  }

  Widget _buildVideoBadge() {
    return Align(
      alignment: Alignment.bottomRight,
      child: Padding(
        padding: EdgeInsets.all(4.w),
        child: Container(
          decoration: BoxDecoration(
            color: AppColors.textSecondary,
            borderRadius: BorderRadius.circular(HkRadius.md),
          ),
          padding: EdgeInsets.all(4.w),
          child: Icon(
            AppIcons.playArrowRounded,
            color: AppIconColors.onBrand,
            size: 16.sp,
          ),
        ),
      ),
    );
  }

  /// 長按九宮格格子時彈出的預覽卡（[_showPostPreviewAt] 是唯一呼叫端）。
  /// 卡片本身不帶賣家 header——預覽已經在該賣家的頁面內，重複顯示店名沒有意義。
  Widget _igPostCardFromPost(Map<String, dynamic> p, StoreShare share) {
    final img = _imageFromPost(p);
    final title = p['title']?.toString() ?? '';
    final ct = p['contentType']?.toString();
    final content = p['content']?.toString();
    final postId = (p['id'] is num)
        ? (p['id'] as num).toInt()
        : int.tryParse('${p['id'] ?? ''}');
    // 讚數直接讀列表 payload 的契約鍵，不另打端點（見 [postLikeCount]）；店主不能按自己貼文的讚，只顯示。
    final int displayLikeCount = postLikeCount(p);
    final int commentCount = postCommentCount(p);
    ValueNotifier<int>? commentNotifier;
    if (postId != null) {
      final int initialCount = _postCommentCount[postId] ?? commentCount;
      commentNotifier = _ensureCommentNotifier(postId, initialCount);
    }
    final commentsAvailability = postCommentsAvailabilityOfMap(share, p);
    final unavailableMessage = commentsAvailability.message;
    if (postId != null &&
        unavailableMessage == null &&
        !_commentsInitRequested.contains(postId)) {
      _commentsInitRequested.add(postId);
      Future.microtask(() async {
        final list = await _postRepo.listComments(postId);
        if (!mounted) return;
        setState(() => _applyCommentCount(postId, list.length));
      });
    }

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        AspectRatio(
          aspectRatio: 1,
          child: GestureDetector(
            behavior: HitTestBehavior.opaque,
            onTap: () => _openFullScreenFeedForPost(p),
            child: (ct == 'video')
                ? (() {
                    final vurl = _videoUrlFromPost(p);
                    if (vurl != null) {
                      return PostVideoPlayer(url: vurl);
                    }
                    return Container(
                      color: AppColors.gray300,
                      child: Center(
                        child: Icon(
                          AppIcons.videocam,
                          color: AppIconColors.onBrand,
                          size: 48.sp,
                        ),
                      ),
                    );
                  })()
                : (img == null
                      ? Container(
                          color: AppColors.gray300,
                          child: Center(
                            child: Icon(
                              AppIcons.image,
                              color: AppIconColors.onBrand,
                              size: 48.sp,
                            ),
                          ),
                        )
                      : Image(
                          image: img,
                          fit: BoxFit.cover,
                          frameBuilder: (context, child, frame, wasSync) {
                            if (wasSync) return child;
                            return AnimatedOpacity(
                              opacity: frame == null ? 0 : 1,
                              duration: const Duration(milliseconds: 250),
                              child: child,
                            );
                          },
                        )),
          ),
        ),
        if (p['linkedProduct'] is Map)
          LinkedProductCard(
            linkedProduct: Map<String, dynamic>.from(p['linkedProduct'] as Map),
          ),
        Padding(
          padding: EdgeInsets.symmetric(horizontal: 12.w, vertical: 12.h),
          child: Row(
            children: [
              _PostPreviewCountAction(count: displayLikeCount, isLike: true),
              SizedBox(width: 24.w),
              _PostPreviewCountAction(
                count: commentNotifier?.value ?? 0,
                countNotifier: commentNotifier,
                onTap: postId == null
                    ? null
                    : () async {
                        await showModalBottomSheet(
                          context: context,
                          // 同上：分支 Navigator 在浮動導覽列之下，留言輸入列
                          // 會被蓋住（#132 同型）。
                          useRootNavigator: true,
                          isScrollControlled: true,
                          backgroundColor: AppColors.bgPrimary,
                          shape: RoundedRectangleBorder(
                            borderRadius: BorderRadius.vertical(
                              top: Radius.circular(16.r),
                            ),
                          ),
                          builder: (_) => CommentsBottomSheet(
                            postId: postId,
                            postRepository: _postRepo,
                            postOwnerName: widget.sellerName,
                            unavailableMessage: unavailableMessage,
                            onCommentCountChanged: (count) {
                              if (!mounted) return;
                              _applyCommentCount(postId, count);
                              setState(() {});
                            },
                          ),
                        );
                      },
              ),
            ],
          ),
        ),
        Padding(
          padding: EdgeInsets.symmetric(horizontal: 12.w),
          child: Text(
            '$displayLikeCount 個讚',
            style: const TextStyle(
              fontWeight: FontWeight.w700,
              color: AppColors.textEmphasis,
            ),
          ),
        ),
        if (title.isNotEmpty ||
            (ct == 'text' && content != null && content.isNotEmpty)) ...[
          SizedBox(height: 6.h),
          Padding(
            padding: EdgeInsets.symmetric(horizontal: 12.w),
            child: Text.rich(
              TextSpan(
                style: DefaultTextStyle.of(context).style,
                children: [
                  TextSpan(
                    text: '${widget.sellerName ?? ''} ',
                    style: const TextStyle(fontWeight: FontWeight.w700),
                  ),
                  if (title.isNotEmpty) TextSpan(text: title),
                  if (ct == 'text' && content != null && content.isNotEmpty)
                    TextSpan(text: title.isNotEmpty ? ' $content' : content),
                ],
              ),
            ),
          ),
        ],
        SizedBox(height: 12.h),
      ],
    );
  }
}

/// Keep preview icons compact while matching the feed's count policy; likes are display-only.
class _PostPreviewCountAction extends StatelessWidget {
  const _PostPreviewCountAction({
    required this.count,
    this.countNotifier,
    this.isLike = false,
    this.onTap,
  });

  final int count;
  final ValueNotifier<int>? countNotifier;
  final bool isLike;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    final notifier = countNotifier;
    if (notifier != null) {
      return ValueListenableBuilder<int>(
        valueListenable: notifier,
        builder: (_, value, _) => _buildCount(value),
      );
    }
    return _buildCount(count);
  }

  Widget _buildCount(int value) => SizedBox(
    width: 96,
    child: HkTapTarget(
      semanticLabel: isLike ? '共 $value 個讚' : '留言，共 $value 則',
      button: !isLike,
      onTap: onTap,
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(
            isLike ? AppIcons.favoriteBorder : AppIcons.chatBubbleOutline,
            size: isLike ? 22.sp : 20.sp,
            color: AppIconColors.muted,
          ),
          SizedBox(width: 4.w),
          Text(
            value > 99 ? '99+' : '$value',
            style: TextStyle(fontSize: 14.sp, color: AppColors.textSecondary),
          ),
        ],
      ),
    ),
  );
}
