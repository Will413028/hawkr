import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:dio/dio.dart' show DioException;
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:video_thumbnail/video_thumbnail.dart';

import '../../../../config/flavor.dart';
import '../../../../config/image_config.dart';
import '../../../../core/motion/hk_motion_transitions.dart';
import '../../../../core/networking/dio_provider.dart';
import '../../../../shared/utils/hk_log.dart';
import '../../../../shared/widgets/comments_bottom_sheet.dart';
import '../../../../shared/widgets/common_snackbar.dart';
import '../../../../shared/widgets/hk_button_label.dart';
import '../../../../shared/widgets/hk_icon.dart';
import '../../../../shared/widgets/fruit_refresh_indicator.dart';
import '../../../../shared/widgets/oss_image.dart';
import '../../../../shared/widgets/post_video_player.dart';
import 'merchant_empty_state.dart';
import 'merchant_layout.dart';

/// 商家頁「貼文」Tab。
///
/// 依可用寬度顯示 2–5 欄等比 Grid。
/// - 點擊單格 → 跳轉 `/social/posts` Carousel 詳細頁
/// - 長壓單格 → 彈出就地預覽卡（含按讚/留言/分享）
/// - 雙擊圖片 → 動態愛心動畫並觸發按讚
///
/// 注意：透過 [ref] 參數取得 Riverpod 依賴，保持本 Widget 為純 [StatefulWidget]。
class MerchantHomeTab extends StatefulWidget {
  const MerchantHomeTab({
    super.key,
    this.ref,
    this.merchantId,
    this.merchantName,
    this.merchantAvatar,
    this.outerHandle,
  });

  /// Riverpod [WidgetRef]，由父層 ConsumerWidget 傳入。
  final WidgetRef? ref;
  final String? merchantId;
  final String? merchantName;
  final String? merchantAvatar;

  /// [NestedScrollView] 的 [SliverOverlapAbsorberHandle]，用於補償 pinned header 高度。
  final SliverOverlapAbsorberHandle? outerHandle;

  @override
  State<MerchantHomeTab> createState() => _MerchantHomeTabState();
}

class _MerchantHomeTabState extends State<MerchantHomeTab>
    with AutomaticKeepAliveClientMixin {
  @override
  bool get wantKeepAlive => true;
  List<Map<String, dynamic>> _posts = <Map<String, dynamic>>[];
  bool _loadingPosts = false;
  String? _postErr;
  final Map<String, String> _videoThumbCache = <String, String>{};
  bool _initialFetchScheduled = false;
  final ScrollController _feedScrollController = ScrollController();

  @override
  void initState() {
    super.initState();
    _scheduleInitialFetch();
  }

  @override
  void didUpdateWidget(covariant MerchantHomeTab oldWidget) {
    super.didUpdateWidget(oldWidget);
    final String oldId = oldWidget.merchantId?.trim() ?? '';
    final String newId = widget.merchantId?.trim() ?? '';
    if (newId != oldId) {
      _initialFetchScheduled = false;
      if (mounted) {
        setState(() {
          _posts = <Map<String, dynamic>>[];
          _postErr = null;
        });
      }
      _scheduleInitialFetch();
    } else {
      _scheduleInitialFetch();
    }
  }

  @override
  void dispose() {
    _feedScrollController.dispose();
    super.dispose();
  }

  void _scheduleInitialFetch() {
    final mid = widget.merchantId?.trim();
    if (mid == null || mid.isEmpty || _initialFetchScheduled) return;
    _initialFetchScheduled = true;
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      _fetchPosts();
    });
  }

  // ── 資料載入 ──────────────────────────────────────────────────────────────

  Future<void> _fetchPosts() async {
    final idStr = widget.merchantId?.trim();
    if (idStr == null || idStr.isEmpty) return;
    if (mounted) {
      setState(() {
        _loadingPosts = true;
        _postErr = null;
      });
    }

    try {
      final dio = widget.ref?.read(dioProvider);
      if (dio == null) return;
      final mid = int.tryParse(idStr) ?? 0;
      List<Map<String, dynamic>> list = [];

      if (mid > 0) {
        try {
          final resp = await dio.get<Map<String, dynamic>>(
            '/public/sellers/$mid/posts',
          );
          final body = resp.data ?? {};
          final rawData = body['data'];
          final List rawList;
          if (rawData is Map<String, dynamic>) {
            rawList = (rawData['records'] as List?) ?? [];
          } else if (rawData is List) {
            rawList = rawData;
          } else {
            rawList = [];
          }
          list = rawList
              .whereType<Map>()
              .map((e) => Map<String, dynamic>.from(e))
              .toList();
        } on DioException catch (de) {
          // 404 表示商家暫無貼文，視為空列表
          if (de.response?.statusCode != 404) rethrow;
        }
      }

      await _hydratePostProducts(dio, list);

      if (mounted) {
        setState(() {
          _posts = list.map((m) {
            final id = '${m['id'] ?? m['postId'] ?? ''}';
            final liked = m['isLiked'] == true;
            final lc = (m['likeCount'] is num)
                ? (m['likeCount'] as num).toInt()
                : 0;
            final cc = (m['commentCount'] is num)
                ? (m['commentCount'] as num).toInt()
                : 0;
            return {
              ...m,
              '_id': id,
              '_liked': liked,
              '_likeCount': lc,
              '_commentCount': cc,
              '_hydrated': true,
            };
          }).toList();
        });
      }

      // 預快取首屏圖片
      try {
        if (mounted && _posts.isNotEmpty) {
          final int prefetchCount = _posts.length < 9 ? _posts.length : 9;
          for (int i = 0; i < prefetchCount; i++) {
            final u = _firstImageUrlFromPost(_posts[i]);
            if (u.isEmpty) continue;
            final abs = _toAbsolute(u);
            if (abs.isEmpty) continue;
            try {
              await precacheImage(
                NetworkImage(ImageConfig.urlFor(abs, ImageScene.postImage)),
                context,
              );
            } catch (_) {}
          }
        }
      } catch (_) {}

      if (mounted) {
        setState(() {
          _loadingPosts = false;
        });
      }
    } catch (e, st) {
      HKLog.logE('_fetchPosts error: $e', e);
      debugPrint('$st');
      if (mounted) {
        setState(() {
          _postErr = '載入失敗';
          _loadingPosts = false;
        });
      }
    }
  }

  /// 針對僅有 id 但無詳細資訊的商品節點，補充抓取商品名稱/圖片/價格。
  Future<void> _hydratePostProducts(
    dynamic dio,
    List<Map<String, dynamic>> records,
  ) async {
    if (dio == null) return;
    final futures = <Future>[];
    for (final raw in records) {
      final product = raw['product'];
      if (product is! Map) continue;
      final pid = product['id'];
      final name = (product['name'] ?? '').toString().trim();
      if (pid == null || pid == 0 || name.isNotEmpty) continue;
      futures.add(() async {
        try {
          final resp = await dio.get<dynamic>('/public/products/$pid');
          final data = resp.data;
          if (data is Map) {
            final inner = data['data'] ?? data;
            if (inner is Map) {
              raw['product'] = {
                ...Map<String, dynamic>.from(product),
                'name': (inner['name'] ?? '').toString(),
                // 49 §6.1 成對契約：imageKey（objectKey）優先，imageUrl 退路。
                'image': (inner['imageKey'] ?? inner['imageUrl'] ?? '')
                    .toString(),
                'price': (inner['priceMin'] ?? '').toString(),
              };
            }
          }
        } catch (_) {}
      }());
    }
    if (futures.isNotEmpty) await Future.wait(futures);
  }

  // ── 互動操作 ──────────────────────────────────────────────────────────────

  Future<void> _toggleLike(int index) async {
    final p = _posts[index];
    final id = (p['_id'] ?? '').toString();
    if (id.isEmpty) return;

    final prevLiked = p['_liked'] == true;
    final prevCount = (p['_likeCount'] is int) ? (p['_likeCount'] as int) : 0;
    final nextLiked = !prevLiked;
    final nextCount = (prevCount + (nextLiked ? 1 : -1)).clamp(0, 1 << 30);
    setState(() {
      _posts[index] = {...p, '_liked': nextLiked, '_likeCount': nextCount};
    });

    try {
      final dio = widget.ref?.read(dioProvider);
      if (dio == null) return;
      final resp = await dio.post<dynamic>('/app/posts/$id/like');
      if (!mounted) return;
      final data = resp.data;
      if (data is Map) {
        final liked = data['liked'] == true;
        final likeCount = (data['likeCount'] is num)
            ? (data['likeCount'] as num).toInt()
            : nextCount;
        setState(() {
          _posts[index] = {
            ..._posts[index],
            '_liked': liked,
            '_likeCount': likeCount,
          };
        });
      }
    } catch (_) {
      if (!mounted) return;
      setState(() {
        _posts[index] = {
          ..._posts[index],
          '_liked': prevLiked,
          '_likeCount': prevCount,
        };
      });
    }
  }

  /// 重抓單一貼文的留言數。
  ///
  /// 資料來源＝貼文詳情 `GET /public/posts/{id}` 的 `comment_count`；Gateway 沒有
  /// `/public/posts/{id}/comments/count` 這條路由（`routes_post.go` 只註冊
  /// `/:id`、`/:id/comments`、`/comments/:id/replies`、`/:id/comments/stream`）。
  Future<void> _refreshCommentCountAt(int index) async {
    if (index < 0 || index >= _posts.length) return;
    final id = (_posts[index]['_id'] ?? '').toString();
    if (id.isEmpty) return;
    try {
      final dio = widget.ref?.read(dioProvider);
      if (dio == null) return;
      final resp = await dio.get<dynamic>('/public/posts/$id');
      final body = resp.data;
      final detail = (body is Map ? body['data'] : null) ?? body;
      if (detail is! Map) return;
      final raw = detail['commentCount'];
      final commentCnt = raw is num
          ? raw.toInt()
          : int.tryParse((raw ?? '').toString()) ?? 0;
      if (!mounted) return;
      setState(() {
        _posts[index] = {..._posts[index], '_commentCount': commentCnt};
      });
    } catch (_) {}
  }

  Future<void> _openCommentSheet(String postId) async {
    final id = postId.trim();
    if (!mounted || id.isEmpty) return;
    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      showDragHandle: false,
      backgroundColor: AppColors.bgPrimary,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(16)),
      ),
      builder: (_) => CommentsBottomSheet(
        postId: id,
        postOwnerName: widget.merchantName,
        merchantAvatar: widget.merchantAvatar,
      ),
    );
    if (!mounted) return;
    final idx = _posts.indexWhere(
      (p) => ((p['_id'] ?? p['id'] ?? p['postId'] ?? '').toString() == id),
    );
    if (idx >= 0) await _refreshCommentCountAt(idx);
  }

  // ── 圖片 / 影片 URL 萃取工具 ─────────────────────────────────────────────

  /// 貼文封面圖。`PostListItemData` 的圖片欄只有 `imageKeys` / `imageUrls` 兩個
  /// 陣列（49 §6.1；無圖為 `[]`，欄位一定存在）—— 單數的 `imageKey` / `imageUrl`
  /// 已從後端移除。key 優先，顯示端 buildImageUrl dual-mode 兩種都吃得下。
  String _firstImageUrlFromPost(Map<String, dynamic> m) {
    for (final field in const ['imageKeys', 'imageUrls']) {
      final value = m[field];
      if (value is! List) continue;
      for (final e in value) {
        final s = (e ?? '').toString().trim();
        if (s.isNotEmpty) return s;
      }
    }
    return '';
  }

  /// 同上，但回傳已過 `_toAbsolute` 的顯示值（格狀縮圖用）。
  String _pickImageUrl(dynamic m) {
    if (m is! Map) return '';
    final raw = _firstImageUrlFromPost(Map<String, dynamic>.from(m));
    return raw.isEmpty ? '' : _toAbsolute(raw);
  }

  /// 貼文影片。social 的 posts 只存單一圖片欄，**沒有任何影片欄位** ——
  /// 這裡曾對 videoUrl / video / mediaUrl / medias[].url 等十餘個名字做猜測式
  /// 探測，後端一個都不吐，所以它從第一天起就恆為 null。保留函式簽章讓呼叫端
  /// 的影片分支維持原狀（走圖片路徑），等後端真的支援影片時再從這裡接。
  String? _videoUrlFromPost(dynamic m) => null;

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
    return _buildPostsList();
  }

  Widget _buildPostsList() {
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
      onRefresh: _fetchPosts,
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

    if (_loadingPosts) {
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

    if (_postErr != null) {
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
                    _postErr!,
                    style: const TextStyle(
                      color: AppColors.textPrimary,
                      fontSize: 16,
                      fontWeight: FontWeight.w700,
                    ),
                  ),
                  const SizedBox(height: 12),
                  TextButton.icon(
                    onPressed: _fetchPosts,
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

    if (_posts.isEmpty) {
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
        itemCount: _posts.length,
        itemBuilder: (_, i) {
          final m = _posts[i];
          final imageUrl = _pickImageUrl(m);
          final title = (m['title'] ?? m['text'] ?? '').toString();
          final String? vUrl = _videoUrlFromPost(m);

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
                    return _emptyTile(title);
                  },
                )
              : _emptyTile(title);

          final postId = (m['_id'] ?? m['id'] ?? m['postId'] ?? '').toString();
          return GestureDetector(
            onLongPressStart: (details) =>
                _showPostPreviewAt(details.globalPosition, i),
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
                        'initialPostId': postId.isNotEmpty ? postId : null,
                      },
                    );
                  },
                  child: SizedBox.expand(
                    child: postId.isNotEmpty
                        ? Hero(
                            tag: 'post-image-$postId',
                            // 飛行中用 destination 的 widget，避免 NetworkImage→cached
                            // 兩種底層 widget 切換導致重新解碼 jank
                            flightShuttleBuilder: _postImageHeroShuttle,
                            child: tileChild,
                          )
                        : tileChild,
                  ),
                ),
              ),
            ),
          );
        },
      ),
    );
  }

  Widget _emptyTile(String title) => Container(
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
          if (title.isNotEmpty) ...[
            SizedBox(height: 6.h),
            Padding(
              padding: EdgeInsets.symmetric(horizontal: 8.w),
              child: Text(
                title,
                maxLines: 2,
                overflow: TextOverflow.ellipsis,
                textAlign: TextAlign.center,
                style: TextStyle(
                  color: AppColors.bgCard,
                  fontSize: 12,
                  fontWeight: FontWeight.w700,
                ),
              ),
            ),
          ],
        ],
      ),
    ),
  );

  // ── 長壓預覽卡 ────────────────────────────────────────────────────────────

  void _showPostPreviewAt(Offset globalPos, int index) {
    SystemSound.play(SystemSoundType.click);
    if (index < 0 || index >= _posts.length) return;
    final m = _posts[index];

    String content = '';
    final c = m['content'];
    if (c is String && c.toString().trim().isNotEmpty) {
      content = c.toString();
    } else if (c is Map) {
      for (final k in ['text', 'caption', 'body', 'description', 'content']) {
        final v = (c[k] ?? '').toString().trim();
        if (v.isNotEmpty) {
          content = v;
          break;
        }
      }
    }
    if (content.isEmpty) {
      for (final k in ['text', 'body', 'description', 'caption', 'title']) {
        final v = (m[k] ?? '').toString().trim();
        if (v.isNotEmpty) {
          content = v;
          break;
        }
      }
    }

    final imageUrl = _pickImageUrl(m);
    final String? vUrl = _videoUrlFromPost(m);
    var dialogLiked = m['_liked'] == true;
    var dialogLikeCount = (m['_likeCount'] is int)
        ? (m['_likeCount'] as int)
        : 0;
    final commentCount = (m['_commentCount'] is int)
        ? (m['_commentCount'] as int)
        : 0;
    final postId = (m['_id'] ?? m['id'] ?? m['postId'] ?? '').toString();
    final mn = widget.merchantName;
    final username = (mn != null && mn.trim().isNotEmpty)
        ? mn.trim()
        : (m['authorName'] ?? m['username'] ?? m['author'] ?? '使用者').toString();

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
        return StatefulBuilder(
          builder: (dialogContext, setDialogState) {
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
                            index: index,
                            postId: postId,
                            imageUrl: imageUrl,
                            videoUrl: vUrl,
                            caption: content,
                            username: username,
                            merchantAvatar: widget.merchantAvatar,
                            liked: dialogLiked,
                            likeCount: dialogLikeCount,
                            commentCount: commentCount,
                            onLikeToggle: () {
                              setDialogState(() {
                                dialogLiked = !dialogLiked;
                                dialogLikeCount += dialogLiked ? 1 : -1;
                              });
                              _toggleLike(index);
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
    );
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
    required this.index,
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

  final int index;
  final String postId;
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
