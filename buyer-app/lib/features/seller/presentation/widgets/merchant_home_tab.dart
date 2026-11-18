import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:flutter/foundation.dart' show listEquals;
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:video_thumbnail/video_thumbnail.dart';

import '../../../../config/flavor.dart';
import '../../../../config/image_config.dart';
import '../../../../core/motion/hk_motion_transitions.dart';
import '../../../../core/session/viewer_scope.dart';
import '../../../../shared/utils/require_login.dart';
import '../../../../shared/widgets/comments_bottom_sheet.dart';
import '../../../../shared/widgets/common_snackbar.dart';
import '../../../../shared/widgets/hk_button_label.dart';
import '../../../../shared/widgets/hk_icon.dart';
import '../../../../shared/widgets/fruit_refresh_indicator.dart';
import '../../../../shared/widgets/oss_image.dart';
import '../../../../shared/widgets/post_video_player.dart';
import '../../../social/data/social_model.dart';
import '../../../social/presentation/post_entity.dart';
import '../../../social/presentation/post_entity_store.dart';
import '../../../social/presentation/post_feed_controller.dart';
import 'merchant_empty_state.dart';
import 'merchant_layout.dart';

/// 商家頁「貼文」Tab。
///
/// 依可用寬度顯示 2–5 欄等比 Grid。
/// - 點擊單格 → 跳轉 `/social/posts` Carousel 詳細頁
/// - 長壓單格 → 彈出就地預覽卡（含按讚/留言/分享）
/// - 雙擊圖片 → 動態愛心動畫並觸發按讚
///
/// 貼文清單與每則貼文的實體都在 Riverpod（[postFeedControllerProvider]、[postSnapshotProvider]），本 Widget 只顯示與發動作。
class MerchantHomeTab extends ConsumerStatefulWidget {
  const MerchantHomeTab({
    super.key,
    this.merchantId,
    this.merchantName,
    this.merchantAvatar,
    this.outerHandle,
  });

  final String? merchantId;
  final String? merchantName;
  final String? merchantAvatar;

  /// [NestedScrollView] 的 [SliverOverlapAbsorberHandle]，用於補償 pinned header 高度。
  final SliverOverlapAbsorberHandle? outerHandle;

  @override
  ConsumerState<MerchantHomeTab> createState() => _MerchantHomeTabState();
}

class _MerchantHomeTabState extends ConsumerState<MerchantHomeTab>
    with AutomaticKeepAliveClientMixin {
  @override
  bool get wantKeepAlive => true;
  final Map<String, String> _videoThumbCache = <String, String>{};
  BuildContext? _previewContext;
  int _prefetchRun = 0;

  /// 目前店家的貼文清單 provider；店家 id 不是正整數時沒有清單。
  PostFeedControllerProvider? get _feed {
    final mid = int.tryParse(widget.merchantId?.trim() ?? '') ?? 0;
    return mid > 0 ? postFeedControllerProvider(merchantId: mid) : null;
  }

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      final ids = _feed == null ? null : ref.read(_feed!).value?.ids;
      if (ids != null) _prefetch(ids);
    });
  }

  @override
  void didUpdateWidget(covariant MerchantHomeTab oldWidget) {
    super.didUpdateWidget(oldWidget);
    if ((oldWidget.merchantId?.trim() ?? '') !=
        (widget.merchantId?.trim() ?? '')) {
      _closePreview();
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (!mounted) return;
        final ids = _feed == null ? null : ref.read(_feed!).value?.ids;
        if (ids != null) _prefetch(ids);
      });
    }
  }

  /// 關掉開著的預覽卡。
  void _closePreview() {
    final context = _previewContext;
    _previewContext = null;
    if (context == null || !context.mounted) return;
    final route = ModalRoute.of(context);
    if (route != null && route.isActive) {
      Navigator.of(context).removeRoute(route);
    }
  }

  Future<void> _refresh() async {
    final feed = _feed;
    if (feed == null) return;
    ref.invalidate(feed);
    try {
      await ref.read(feed.future);
    } catch (_) {
      // 重讀失敗保留舊清單。
    }
  }

  /// 預快取首屏圖片。清單換了或又被呼叫時，前一輪就停。
  Future<void> _prefetch(List<int> ids) async {
    final run = ++_prefetchRun;
    final store = ref.read(postEntityStoreProvider);
    for (final id in ids.take(9)) {
      final url = _toAbsolute(store.posts[id]?.imageUrl);
      if (url.isEmpty) continue;
      if (!mounted) return;
      try {
        await precacheImage(
          NetworkImage(ImageConfig.urlFor(url, ImageScene.postImage)),
          context,
        );
      } catch (_) {}
      if (!mounted || run != _prefetchRun) return;
    }
  }

  // ── 互動操作 ──────────────────────────────────────────────────────────────

  Future<void> _openCommentSheet(int postId) async {
    if (!mounted) return;
    final post = ref.read(postEntityStoreProvider).posts[postId];
    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      showDragHandle: false,
      backgroundColor: AppColors.bgPrimary,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(16)),
      ),
      builder: (_) => CommentsBottomSheet(
        postId: postId.toString(),
        viewerCanComment: post?.viewerCanComment ?? false,
        viewerActor: post?.viewerActor,
      ),
    );
    if (!mounted) return;
    await ref.read(postEntityStoreProvider.notifier).refresh(postId);
  }

  // ── 圖片 / 影片 URL 萃取工具 ─────────────────────────────────────────────

  /// 貼文封面圖的顯示值（已過 `_toAbsolute`）。
  String _pickImageUrl(Post post) => _toAbsolute(post.imageUrl);

  /// 貼文影片。social 的 posts 只存單一圖片欄，**沒有任何影片欄位** ——
  /// 這裡曾對 videoUrl / video / mediaUrl / medias[].url 等十餘個名字做猜測式
  /// 探測，後端一個都不吐，所以它從第一天起就恆為 null。保留函式簽章讓呼叫端
  /// 的影片分支維持原狀（走圖片路徑），等後端真的支援影片時再從這裡接。
  String? _videoUrlFromPost(Post post) => null;

  Future<String?> _thumbnailForVideo(String videoUrl) async {
    final cached = _videoThumbCache[videoUrl];
    if (cached != null && cached.isNotEmpty) return cached;
    try {
      final path = await VideoThumbnail.thumbnailFile(
        video: videoUrl,
        imageFormat: ImageFormat.JPEG,
        maxWidth: 512,
        quality: 75,
      );
      if (path != null && path.isNotEmpty) {
        _videoThumbCache[videoUrl] = path;
        return path;
      }
    } catch (_) {}
    return null;
  }

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

  // ── Build ─────────────────────────────────────────────────────────────────

  @override
  Widget build(BuildContext context) {
    super.build(context);
    if (widget.merchantId == null || widget.merchantId!.isEmpty) {
      return const Center(child: Text('No merchant'));
    }
    final feed = _feed;
    if (feed != null) {
      ref.listen(feed.select((a) => a.value?.ids), (previous, next) {
        if (next == null) return;
        if (previous != null &&
            listEquals(previous.take(9).toList(), next.take(9).toList())) {
          return;
        }
        _prefetch(next);
      });
    }
    ref.listen(viewerIdProvider, (_, _) => _closePreview());
    return _buildPostsList(feed);
  }

  Widget _buildPostsList(PostFeedControllerProvider? feed) {
    final feedState = feed == null ? null : ref.watch(feed);
    final postIds = feedState?.value?.ids ?? const <int>[];
    final unavailable = watchListUnavailable(ref, postIds);
    final BorderRadius tileRadius = BorderRadius.circular(16);
    final String keySuffix = widget.merchantId ?? 'noid';

    Widget buildSliverGrid({
      required int itemCount,
      required IndexedWidgetBuilder itemBuilder,
    }) => SliverLayoutBuilder(
      builder: (context, constraints) {
        final viewportWidth = constraints.crossAxisExtent;
        final spec = MerchantLayoutSpec.forSize(
          Size(viewportWidth, MediaQuery.sizeOf(context).height),
        );
        final baseInset = spec.pageHorizontalPadding;
        final sideInset = MerchantLayoutSpec.centeredSideInset(
          viewportWidth,
          baseInset: baseInset,
        );
        final contentWidth = viewportWidth - sideInset * 2;
        final columns = spec.postColumnCount(contentWidth);
        final spacing = viewportWidth < 360 ? 8.0 : 12.0;
        return SliverPadding(
          padding: EdgeInsets.fromLTRB(sideInset, 16, sideInset, 40),
          sliver: SliverGrid.builder(
            key: PageStorageKey<String>('echo_grid_$keySuffix'),
            gridDelegate: SliverGridDelegateWithFixedCrossAxisCount(
              crossAxisCount: columns,
              mainAxisSpacing: spacing,
              crossAxisSpacing: spacing,
              childAspectRatio: 1,
            ),
            itemCount: itemCount,
            itemBuilder: itemBuilder,
          ),
        );
      },
    );

    Widget wrapInScrollView(Widget sliver) => FruitRefreshIndicator(
      onRefresh: _refresh,
      topOffset: widget.outerHandle?.layoutExtent ?? 0,
      child: CustomScrollView(
        physics: const AlwaysScrollableScrollPhysics(),
        slivers: [
          if (widget.outerHandle != null)
            SliverOverlapInjector(handle: widget.outerHandle!),
          sliver,
        ],
      ),
    );

    if (feedState != null &&
        (!feedState.hasValue || unavailable) &&
        feedState.isLoading) {
      return wrapInScrollView(
        buildSliverGrid(
          itemCount: 9,
          itemBuilder: (_, __) => ClipRRect(
            borderRadius: tileRadius,
            child: Container(color: AppColors.gray100),
          ),
        ),
      );
    }

    if (feedState != null &&
        (!feedState.hasValue || unavailable) &&
        feedState.hasError) {
      return wrapInScrollView(
        SliverFillRemaining(
          hasScrollBody: false,
          child: Center(
            child: Container(
              margin: const EdgeInsets.all(24),
              padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 20),
              decoration: BoxDecoration(
                color: AppColors.storefrontCardSurface,
                borderRadius: BorderRadius.circular(24),
                border: Border.all(color: AppColors.storefrontBorder),
              ),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  const HkIcon(
                    HkIconName.warning,
                    size: 32,
                    color: AppIconColors.warning,
                  ),
                  const SizedBox(height: 8),
                  Text(
                    '載入失敗',
                    style: const TextStyle(
                      color: AppColors.textPrimary,
                      fontSize: 16,
                      fontWeight: FontWeight.w700,
                    ),
                  ),
                  const SizedBox(height: 12),
                  TextButton.icon(
                    onPressed: _refresh,
                    icon: const HkIcon(HkIconName.refresh, size: 18),
                    label: const HkButtonLabel('重新載入'),
                  ),
                ],
              ),
            ),
          ),
        ),
      );
    }

    if (postIds.isEmpty) {
      return wrapInScrollView(
        SliverFillRemaining(
          hasScrollBody: false,
          child: Center(
            child: MerchantEmptyState(
              key: PageStorageKey<String>('echo_empty_list_$keySuffix'),
              semanticIcon: HkIconName.post,
              size: 36,
              label: '暫無貼文',
            ),
          ),
        ),
      );
    }

    return wrapInScrollView(
      buildSliverGrid(
        itemCount: postIds.length,
        itemBuilder: (_, i) => Consumer(
          builder: (context, ref, _) {
            final postId = postIds[i];
            final post = ref.watch(postSnapshotProvider(postId));
            if (post == null) {
              return ClipRRect(
                borderRadius: tileRadius,
                child: Container(color: AppColors.dividerLight),
              );
            }
            final imageUrl = _pickImageUrl(post);
            final String? vUrl = _videoUrlFromPost(post);

            final Widget tileChild = imageUrl.isNotEmpty
                ? _StableNetImage(url: imageUrl)
                : (vUrl != null && vUrl.isNotEmpty)
                ? FutureBuilder<String?>(
                    future: _thumbnailForVideo(vUrl),
                    builder: (_, snap) {
                      if (snap.connectionState == ConnectionState.waiting) {
                        return Container(color: AppColors.dividerLight);
                      }
                      final p = snap.data;
                      if (p != null && p.isNotEmpty) {
                        return Image.asset(
                          p,
                          fit: BoxFit.cover,
                          errorBuilder: (_, __, ___) =>
                              Container(color: AppColors.dividerLight),
                        );
                      }
                      return _emptyTile();
                    },
                  )
                : _emptyTile();

            return GestureDetector(
              onLongPressStart: (details) =>
                  _showPostPreviewAt(details.globalPosition, postId),
              child: ClipRRect(
                borderRadius: tileRadius,
                child: Material(
                  color: Colors.transparent,
                  child: InkWell(
                    onTap: () {
                      final mid = widget.merchantId;
                      if (mid == null || mid.isEmpty) return;
                      context.push(
                        '/social/posts',
                        extra: {
                          'merchantId': mid,
                          'merchantName': widget.merchantName,
                          'merchantAvatar': widget.merchantAvatar,
                          'initialIndex': i,
                          'initialPostId': postId.toString(),
                        },
                      );
                    },
                    child: SizedBox.expand(
                      child: Hero(
                        tag: 'post-image-$postId',
                        // 飛行中用 destination 的 widget，避免 NetworkImage→cached
                        // 兩種底層 widget 切換導致重新解碼 jank
                        flightShuttleBuilder: _postImageHeroShuttle,
                        child: tileChild,
                      ),
                    ),
                  ),
                ),
              ),
            );
          },
        ),
      ),
    );
  }

  Widget _emptyTile() => Container(
    color: AppColors.dividerLight,
    child: Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(
            AppIcons.brokenImageOutlined,
            color: AppIconColors.onBrand,
            size: 28.sp,
          ),
        ],
      ),
    ),
  );

  // ── 長壓預覽卡 ────────────────────────────────────────────────────────────

  void _showPostPreviewAt(Offset globalPos, int postId) {
    SystemSound.play(SystemSoundType.click);
    final post = ref.read(postEntityStoreProvider).posts[postId];
    if (post == null) return;
    final mn = widget.merchantName;
    final username = (mn != null && mn.trim().isNotEmpty) ? mn.trim() : '使用者';

    showGeneralDialog(
      context: context,
      barrierColor: Colors.transparent,
      barrierDismissible: true,
      barrierLabel: '關閉',
      pageBuilder: (ctx, a1, a2) {
        final size = MediaQuery.of(ctx).size;
        final double panelWidth = size.width <= 380 ? size.width - 16 : 360;
        final double maxHeight = size.height * 0.8;
        double left = (globalPos.dx - panelWidth / 2).clamp(
          8.0,
          size.width - panelWidth - 8,
        );
        double top = globalPos.dy.clamp(8.0, size.height - maxHeight - 8) - 10;
        return Consumer(
          builder: (dialogContext, ref, _) {
            _previewContext = dialogContext;
            final live = ref.watch(postSnapshotProvider(postId));
            if (live == null) {
              _closePreview();
              return const SizedBox.shrink();
            }
            final imageUrl = _pickImageUrl(live);
            final String? vUrl = _videoUrlFromPost(live);
            return Stack(
              children: [
                Positioned(
                  left: left,
                  top: top,
                  child: Material(
                    color: AppColors.bgCard,
                    borderRadius: BorderRadius.circular(16.r),
                    elevation: 8,
                    child: ConstrainedBox(
                      constraints: BoxConstraints(
                        maxWidth: panelWidth,
                        maxHeight: maxHeight,
                      ),
                      child: SingleChildScrollView(
                        child: Padding(
                          padding: EdgeInsets.only(bottom: 8.h),
                          child: _PostPreviewCard(
                            postId: postId,
                            imageUrl: imageUrl,
                            videoUrl: vUrl,
                            caption: live.content,
                            username: username,
                            merchantAvatar: widget.merchantAvatar,
                            liked: live.liked,
                            likeCount: live.likeCount,
                            commentCount: live.commentCount,
                            onLikeToggle: () async {
                              if (!await requireLogin(context, ref)) return;
                              if (!mounted) return;
                              await ref
                                  .read(postEntityStoreProvider.notifier)
                                  .toggleLike(postId);
                            },
                            onComment: () {
                              Navigator.of(context).pop();
                              _openCommentSheet(postId);
                            },
                          ),
                        ),
                      ),
                    ),
                  ),
                ),
              ],
            );
          },
        );
      },
      transitionDuration: HkMotionTransitions.durationOf(context),
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
    ).whenComplete(() => _previewContext = null);
  }
}

// ---------------------------------------------------------------------------
// _PostPreviewCard — 長壓預覽卡（媒體 + 互動列）
// ---------------------------------------------------------------------------

/// 貼文長壓預覽卡。
///
/// 顯示正方形媒體（圖片/影片）以及按讚、留言、分享互動列。
/// [liked] 與 [likeCount] 由外層 [StatefulBuilder] 管理以實現對話框內即時更新。
class _PostPreviewCard extends StatelessWidget {
  const _PostPreviewCard({
    required this.postId,
    required this.imageUrl,
    this.videoUrl,
    required this.caption,
    required this.username,
    this.merchantAvatar,
    required this.liked,
    required this.likeCount,
    required this.commentCount,
    required this.onLikeToggle,
    required this.onComment,
  });

  final int postId;
  final String imageUrl;
  final String? videoUrl;
  final String caption;
  final String username;
  final String? merchantAvatar;
  final bool liked;
  final int likeCount;
  final int commentCount;
  final VoidCallback onLikeToggle;
  final VoidCallback onComment;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        AspectRatio(
          aspectRatio: 1,
          child: _DoubleTapLikeMedia(
            onDoubleLike: onLikeToggle,
            child: (videoUrl != null && videoUrl!.isNotEmpty)
                ? PostVideoPlayer(url: videoUrl!)
                : imageUrl.isNotEmpty
                ? OssImage(
                    imageSource: imageUrl,
                    scene: ImageScene.postImage,
                    fit: BoxFit.cover,
                  )
                : Container(
                    color: AppColors.dividerLight,
                    child: const Center(
                      child: Icon(AppIcons.brokenImageOutlined),
                    ),
                  ),
          ),
        ),
        Padding(
          padding: EdgeInsets.symmetric(horizontal: 12.w, vertical: 12.h),
          child: Row(
            children: [
              GestureDetector(
                onTap: onLikeToggle,
                child: Row(
                  children: [
                    Icon(
                      liked ? AppIcons.favorite : AppIcons.favoriteBorder,
                      size: 22.w,
                      color: liked
                          ? AppIconColors.favorite
                          : AppIconColors.muted,
                    ),
                    SizedBox(width: 4.w),
                    Text(
                      '$likeCount',
                      style: TextStyle(
                        fontSize: 14,
                        color: AppColors.textSecondary,
                      ),
                    ),
                  ],
                ),
              ),
              SizedBox(width: 24.w),
              GestureDetector(
                onTap: onComment,
                child: Row(
                  children: [
                    Icon(
                      AppIcons.chatBubbleOutline,
                      size: 20.w,
                      color: AppIconColors.muted,
                    ),
                    SizedBox(width: 4.w),
                    Text(
                      '$commentCount',
                      style: TextStyle(
                        fontSize: 14,
                        color: AppColors.textSecondary,
                      ),
                    ),
                  ],
                ),
              ),
              SizedBox(width: 24.w),
              GestureDetector(
                onTap: () => showFloatingToast(context, '分享功能即將上線'),
                child: Row(
                  children: [
                    Icon(
                      AppIcons.shareOutlined,
                      size: 20.w,
                      color: AppIconColors.muted,
                    ),
                    SizedBox(width: 4.w),
                    Text(
                      '0',
                      style: TextStyle(
                        fontSize: 14,
                        color: AppColors.textSecondary,
                      ),
                    ),
                  ],
                ),
              ),
            ],
          ),
        ),
        Padding(
          padding: EdgeInsets.symmetric(horizontal: 12.w),
          child: Text(
            '$likeCount 個讚',
            style: const TextStyle(
              fontWeight: FontWeight.w700,
              color: AppColors.textEmphasis,
            ),
          ),
        ),
        if (caption.isNotEmpty) ...[
          SizedBox(height: 6.h),
          Padding(
            padding: EdgeInsets.symmetric(horizontal: 12.w),
            child: RichText(
              text: TextSpan(
                style: DefaultTextStyle.of(context).style,
                children: [
                  TextSpan(
                    text: '$username ',
                    style: const TextStyle(fontWeight: FontWeight.w700),
                  ),
                  TextSpan(text: caption),
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

// ---------------------------------------------------------------------------
// _StableNetImage — 預快取後才顯示的網路圖片，避免格子切換閃爍
// ---------------------------------------------------------------------------
// Hero flight shuttle — 飛行中用 destination 的 widget（避免兩端不同 image
// widget 在飛行途中切換造成重新解碼 jank）。push 時 toHero=feed 詳細頁，pop
// 時 toHero=grid tile，雙向都用「即將到達端」的 widget，視覺較順
Widget _postImageHeroShuttle(
  BuildContext flightContext,
  Animation<double> animation,
  HeroFlightDirection flightDirection,
  BuildContext fromHeroContext,
  BuildContext toHeroContext,
) {
  final Hero toHero = toHeroContext.widget as Hero;
  return toHero.child;
}

/// 預快取後才渲染的網路圖片。
///
/// 在圖片快取完成前顯示灰色佔位，完成後以 [gaplessPlayback] 方式切換，
/// 防止 Grid 捲動時出現閃爍。
class _StableNetImage extends StatefulWidget {
  const _StableNetImage({required this.url});
  final String url;

  @override
  State<_StableNetImage> createState() => _StableNetImageState();
}

class _StableNetImageState extends State<_StableNetImage> {
  bool _ready = false;
  ImageProvider? _provider;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    _prefetch();
  }

  @override
  void didUpdateWidget(covariant _StableNetImage oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.url != widget.url) {
      _ready = false;
      _provider = null;
      _prefetch();
    }
  }

  void _prefetch() {
    final u = widget.url.trim();
    if (u.isEmpty) return;
    final p = NetworkImage(ImageConfig.urlFor(u, ImageScene.postImage));
    _provider = p;
    precacheImage(p, context)
        .then((_) {
          if (mounted) setState(() => _ready = true);
        })
        .catchError((_) {});
  }

  @override
  Widget build(BuildContext context) {
    if (_ready && _provider != null) {
      return Image(
        image: _provider!,
        fit: BoxFit.cover,
        gaplessPlayback: true,
        errorBuilder: (_, __, ___) => Container(color: AppColors.dividerLight),
      );
    }
    return Container(color: AppColors.dividerLight);
  }
}

// ---------------------------------------------------------------------------
// _DoubleTapLikeMedia — 雙擊愛心動畫
// ---------------------------------------------------------------------------

/// 雙擊媒體觸發愛心噴發動畫的包裝器。
///
/// 每次雙擊生成 3 顆錯位愛心，以 [TweenAnimationBuilder] 執行上浮淡出動畫，
/// 並回調 [onDoubleLike] 通知父層執行按讚邏輯。
class _DoubleTapLikeMedia extends StatefulWidget {
  const _DoubleTapLikeMedia({required this.child, required this.onDoubleLike});

  final Widget child;
  final VoidCallback onDoubleLike;

  @override
  State<_DoubleTapLikeMedia> createState() => _DoubleTapLikeMediaState();
}

class _DoubleTapLikeMediaState extends State<_DoubleTapLikeMedia> {
  int _counter = 0;
  final List<_HeartPulse> _hearts = <_HeartPulse>[];

  void _spawnHearts() {
    const delays = [0, 120, 240];
    const dxs = [-18.0, 0.0, 18.0];
    const sizes = [84.0, 96.0, 88.0];
    for (var i = 0; i < delays.length; i++) {
      Future.delayed(Duration(milliseconds: delays[i]), () {
        if (!mounted) return;
        setState(() {
          _hearts.add(_HeartPulse(id: _counter++, dx: dxs[i], size: sizes[i]));
        });
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    return Stack(
      fit: StackFit.expand,
      children: [
        widget.child,
        ..._hearts.map((h) => _buildHeart(h)),
        Positioned.fill(
          child: GestureDetector(
            behavior: HitTestBehavior.translucent,
            onDoubleTap: () {
              _spawnHearts();
              widget.onDoubleLike();
            },
          ),
        ),
      ],
    );
  }

  Widget _buildHeart(_HeartPulse h) {
    return Center(
      child: TweenAnimationBuilder<double>(
        key: ValueKey(h.id),
        tween: Tween(begin: 0.0, end: 1.0),
        duration: const Duration(milliseconds: 800),
        onEnd: () {
          if (mounted) {
            setState(() {
              _hearts.removeWhere((e) => e.id == h.id);
            });
          }
        },
        builder: (_, t, child) {
          return Opacity(
            opacity: 1.0 - t,
            child: Transform.translate(
              offset: Offset(h.dx * (1 - t * 0.4), -70 * t),
              child: Transform.scale(scale: 0.9 + 0.3 * t, child: child),
            ),
          );
        },
        child: Icon(
          AppIcons.favorite,
          color: AppIconColors.favorite,
          size: h.size,
        ),
      ),
    );
  }
}

/// 愛心噴發粒子資料模型。
class _HeartPulse {
  const _HeartPulse({required this.id, required this.dx, required this.size});
  final int id;
  final double dx;
  final double size;
}
