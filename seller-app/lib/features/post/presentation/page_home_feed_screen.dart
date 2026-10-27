import '../../seller/presentation/seller_store_avatar.dart';
import '../../../features/seller/data/current_store.dart';
import 'dart:math' as math;
import 'dart:ui' as ui;

import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:hawkr_api/taipei_time.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:hawkr_seller/shared/widgets/hk_icon.dart';
import 'package:flutter/services.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../config/image_config.dart';
import '../../../core/exceptions/app_exception.dart';
import '../../../core/theme/hk_tokens.dart';
import '../../../shared/utils/floating_toast.dart';
import '../../../config/media_image.dart';
import '../../../shared/widgets/hk_empty_state.dart';
import '../../../shared/widgets/hk_button_label.dart';
import '../../../shared/widgets/hk_frosted_icon_button.dart';
import '../../../shared/widgets/hk_frosted_card_surface.dart';
import '../../../shared/widgets/hk_post_card_surface.dart';
import '../../../shared/widgets/hk_tap_target.dart';
import '../../../shared/widgets/fruit_spinner.dart';
import '../../../shared/utils/product_image_resolver.dart';
import '../../../shared/widgets/optimized_image.dart';
import '../../../shared/widgets/share_bottom_sheet.dart';
import '../../report/data/report_model.dart';
import '../../report/presentation/widgets/report_sheet.dart';
import '../data/post_comments_availability.dart';
import '../data/post_hidden.dart';
import '../data/post_model.dart';
import '../data/post_publish.dart';
import '../data/post_repository.dart';
import 'post_controller.dart';
import 'widgets/post_status_badge.dart';
import 'widgets/storefront_tab_glass.dart';
import '../../../shared/widgets/app_confirm_dialog.dart';
import '../../../shared/widgets/hk_disabled_share.dart';
import '../../../shared/widgets/hk_disabled_with_hint.dart';
import '../../../shared/widgets/hk_status_badge.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pbenum.dart'
    show SellerCapability;

const Color _kGray100 = AppColors.textPrimary;
const Color _kGray50 = AppColors.textSecondary; // rgba(86,86,86,0.5)
const Color _kBorder = Color(0x1442493F);

// ─── 留言 VM / 分組（對齊 buyer merchant_posts_feed_screen；資料層走
// seller API：留言用 parent_id 分組 + listCommentReplies 補齊回覆） ────────
class _CommentVM {
  const _CommentVM({
    required this.id,
    required this.parentId,
    required this.isMine,
    required this.name,
    required this.avatar,
    required this.content,
    required this.createdAt,
  });

  final int id;
  final int parentId;
  final bool isMine;
  final String name;
  final String avatar;
  final String content;
  final dynamic createdAt;

  static _CommentVM fromRaw(Map<String, dynamic> raw) {
    int pInt(dynamic v) {
      if (v is int) return v;
      if (v is num) return v.toInt();
      if (v is String) return int.tryParse(v) ?? 0;
      return 0;
    }

    String s(dynamic v) => (v ?? '').toString();

    return _CommentVM(
      id: pInt(raw['id']),
      parentId: pInt(raw['parentId']),
      isMine: raw['isMine'] == true,
      name: s(raw['userName']),
      // 契約成對回 `avatarKey` + `avatarUrl`（gateway post.go `CommentItem`），
      // SSOT 是 key。舊欄位 `userAvatarKey` / `userAvatarUrl` / `userAvatar` /
      // `avatar` 皆已不存在。
      avatar: s(raw['avatarKey'] ?? raw['avatarUrl']),
      content: s(raw['content'] ?? raw['comment'] ?? raw['text']),
      createdAt: raw['createdAt'],
    );
  }
}

class _CommentThread {
  const _CommentThread({required this.parent, required this.replies});
  final _CommentVM parent;
  final List<_CommentVM> replies;

  int get total => 1 + replies.length;
}

final _postCommentsProvider = FutureProvider.autoDispose
    .family<List<_CommentThread>, int>((ref, postId) async {
      final repo = ref.watch(postRepositoryProvider);
      final raw = await repo.listCommentsRaw(postId);

      final tops = <_CommentVM>[];
      final byParent = <int, List<_CommentVM>>{};
      for (final m in raw) {
        final c = _CommentVM.fromRaw(m);
        if (c.parentId > 0) {
          byParent.putIfAbsent(c.parentId, () => <_CommentVM>[]).add(c);
        } else {
          tops.add(c);
        }
      }

      // 各 top-level 留言補抓回覆（與主列表 merge、以 id 去重）
      await Future.wait(
        tops.map((t) async {
          try {
            final items = await repo.listCommentReplies(t.id);
            final list = byParent.putIfAbsent(t.id, () => <_CommentVM>[]);
            final seen = list.map((e) => e.id).toSet();
            for (final m in items) {
              final r = _CommentVM.fromRaw(m);
              if (seen.add(r.id)) list.add(r);
            }
          } catch (_) {}
        }),
      );

      return [
        for (final t in tops)
          _CommentThread(
            parent: t,
            replies: byParent[t.id] ?? const <_CommentVM>[],
          ),
      ];
    });

/// 賣家店舖貼文管理面：沉浸媒體、可展開說明與固定操作列（數字、留言、分享、發布與下架）。
///
/// [initialPostId] 給了就停在那一篇（推播與列表點進來），找不到停在第一篇；沒給用 [initialIndex]。
class PageHomeFeedScreen extends HookConsumerWidget {
  const PageHomeFeedScreen({
    super.key,
    this.initialIndex = 0,
    this.initialPostId,
    required this.sellerName,
  });

  final int initialIndex;
  final int? initialPostId;
  final String sellerName;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final postsAsync = ref.watch(postListProvider);
    final share = ref.watch(storeShareProvider);

    return Scaffold(
      backgroundColor: AppColors.bgPrimary,
      resizeToAvoidBottomInset: false,
      body: AnnotatedRegion<SystemUiOverlayStyle>(
        value: SystemUiOverlayStyle.dark.copyWith(
          statusBarColor: Colors.transparent,
        ),
        child: postsAsync.when(
          // 重讀失敗時 state 帶著舊列表：繼續顯示它，不換成錯誤畫面。
          skipError: true,
          data: (posts) {
            if (posts.isEmpty) return _buildEmptyState();
            return _FeedPager(
              posts: posts,
              initialIndex: initialPostId == null
                  ? initialIndex.clamp(0, posts.length - 1)
                  : feedIndexForPost(posts, initialPostId),
              sellerName: sellerName,
              share: share,
              publishDisabled: postsAsync.hasError,
              onShare: (post) => _onShare(context, ref, post),
              onEdit: (post) => _onEdit(context, ref, post),
              onDelete: (post) => _confirmAndDelete(context, ref, post.id),
            );
          },
          loading: () => const Center(child: FruitSpinner(size: 64)),
          error: (e, _) => Center(child: Text('Error: $e')),
        ),
      ),
    );
  }

  Future<void> _onEdit(BuildContext context, WidgetRef ref, Post post) async {
    final result = await context.push<bool>(
      '/post/edit',
      extra: {'post': post},
    );
    if (result == true) {
      await ref.read(postListProvider.notifier).silentRefresh();
    }
  }

  Future<void> _onShare(BuildContext context, WidgetRef ref, Post post) async {
    final data = await ref
        .read(postRepositoryProvider)
        .getShareUrl(type: 'post', id: post.id.toString());
    if (!context.mounted) return;
    if (data == null) {
      FloatingToast.error(context, kShareFailedMessage);
      return;
    }
    showShareSheet(context, data);
  }

  Widget _buildEmptyState() {
    return const HkEmptyState(
      key: ValueKey('seller-posts-empty-state'),
      iconKey: ValueKey('seller-posts-empty-state-icon'),
      icon: HkIconName.post,
      message: '暫無貼文',
      description: '發布第一則動態，和買家分享今天的好食材。',
      topPadding: 96,
    );
  }

  Future<void> _confirmAndDelete(
    BuildContext context,
    WidgetRef ref,
    int postId,
  ) async {
    final bool? confirm = await showDialog<bool>(
      context: context,
      barrierDismissible: false,
      builder: (ctx) => _buildDeleteDialog(ctx),
    );
    if (confirm != true) return;

    try {
      await ref.read(postListProvider.notifier).deletePost(postId);
      if (context.mounted) FloatingToast.success(context, '已刪除貼文');
    } catch (e) {
      if (context.mounted) {
        FloatingToast.error(context, '刪除失敗：${_mutationErrorMessage(e)}');
      }
    }
  }

  Widget _buildDeleteDialog(BuildContext context) {
    return AlertDialog(
      backgroundColor: AppColors.bgElevated,
      surfaceTintColor: Colors.transparent,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(HkRadius.xl),
      ),
      title: const Text('刪除貼文？'),
      content: const Text('刪除後將無法復原。'),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(false),
          child: const HkButtonLabel('取消'),
        ),
        TextButton(
          onPressed: () => Navigator.of(context).pop(true),
          child: const HkButtonLabel(
            '刪除',
            style: TextStyle(color: AppColors.danger),
          ),
        ),
      ],
    );
  }
}

class _FeedPager extends HookWidget {
  const _FeedPager({
    required this.posts,
    required this.initialIndex,
    required this.sellerName,
    required this.share,
    required this.publishDisabled,
    required this.onShare,
    required this.onEdit,
    required this.onDelete,
  });

  final List<Post> posts;
  final int initialIndex;
  final String sellerName;
  final StoreShare share;

  /// 列表重讀失敗後，畫面上的發布狀態不可信：發布與下架停用到下一次重讀成功。
  final bool publishDisabled;
  final void Function(Post post) onShare;
  final void Function(Post post) onEdit;
  final void Function(Post post) onDelete;

  @override
  Widget build(BuildContext context) {
    final pageController = usePageController(initialPage: initialIndex);
    return Stack(
      children: [
        Positioned.fill(
          child: _PostPreviewBackdrop(
            imageSource: posts[initialIndex].imageKeys.firstOrNull ?? '',
          ),
        ),
        // 左右切換店舖貼文；說明與留言各自保留可捲動空間。
        Positioned.fill(
          child: PageView.builder(
            key: const ValueKey('seller-post-card-page-view'),
            controller: pageController,
            scrollDirection: Axis.horizontal,
            physics: const BouncingScrollPhysics(),
            itemCount: posts.length,
            itemBuilder: (_, i) {
              final post = posts[i];
              return _PostPage(
                key: ValueKey('seller-post-page-${post.id}'),
                post: post,
                sellerName: sellerName,
                publishDisabled: publishDisabled,
                onShareTap: switch (share) {
                  StoreShareEnabled() => () => onShare(post),
                  _ => null,
                },
                shareHint: share is StoreShareDisabled
                    ? kStoreShareDisabledHint
                    : null,
                onEditTap: () => onEdit(post),
                onDeleteTap: () => onDelete(post),
                onProductTap: post.linkedProduct != null
                    ? () => context.push(
                        '/product/preview',
                        extra: {'product': post.linkedProduct!},
                      )
                    : null,
              );
            },
          ),
        ),
      ],
    );
  }
}

String _mutationErrorMessage(Object error) {
  return error is AppException ? error.message : '請稍後再試';
}

// ─── 單篇預覽（對齊 buyer MerchantPostsFeedScreen）──────────────────
class _PostPage extends HookConsumerWidget {
  const _PostPage({
    super.key,
    required this.post,
    required this.sellerName,
    required this.publishDisabled,
    required this.onShareTap,
    required this.shareHint,
    required this.onEditTap,
    required this.onDeleteTap,
    this.onProductTap,
  });

  final Post post;
  final String sellerName;
  final bool publishDisabled;
  final VoidCallback? onShareTap;
  final String? shareHint;
  final VoidCallback onEditTap;
  final VoidCallback onDeleteTap;
  final VoidCallback? onProductTap;

  String _formatDate(DateTime? d) {
    if (d == null) return '';
    final l = d.toLocal();
    final yyyy = l.year.toString().padLeft(4, '0');
    final mm = l.month.toString().padLeft(2, '0');
    final dd = l.day.toString().padLeft(2, '0');
    return '$yyyy/$mm/$dd';
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    // objectKey 直接交給 OptimizedNetworkImage，由 buildImageUrl（唯一組裝入口）
    // 依 postImage 場景組顯示 URL；不要在這裡自行拼域名。
    final imageSource = post.imageKeys.isEmpty ? '' : post.imageKeys.first;
    final dateStr = _formatDate(post.createdAt);
    final statusBadge = postStatusBadgeOf(post);

    final commentCtrl = useTextEditingController();
    final commentFocus = useFocusNode();
    final submitting = useState(false);
    final deletingCommentIds = useRef(<int>{});
    // 回覆對象：apiParentId 固定掛 top-level 留言（seller API 用 parent_id 串
    // thread），displayName 顯示被回覆者
    final replyTarget = useState<_PostReplyTarget?>(null);
    final captionExpanded = useState(false);
    final commentsOpen = useRef(false);

    void setReplyTo(_CommentThread thread, _CommentVM tapped) {
      if (submitting.value) return;
      replyTarget.value = (
        apiParentId: thread.parent.id,
        name: tapped.name.trim().isEmpty ? '匿名' : tapped.name.trim(),
        isReplyToReply: tapped.id != thread.parent.id,
      );
      commentFocus.requestFocus();
    }

    void syncCommentCount(int delta) {
      // The sheet keeps these callbacks while the underlying feed rebuilds.
      // Read the latest count so consecutive successful mutations accumulate.
      final current = ref
          .read(postListProvider)
          .value
          ?.where((item) => item.id == post.id)
          .firstOrNull;
      final next = (current?.commentCount ?? post.commentCount) + delta;
      ref
          .read(postListProvider.notifier)
          .updateCommentCount(post.id, next < 0 ? 0 : next);
    }

    Future<void> submitComment() async {
      final raw = commentCtrl.text.trim();
      if (raw.isEmpty || submitting.value) return;
      submitting.value = true;
      try {
        final target = replyTarget.value;
        Map<String, dynamic>? created;
        if (target != null) {
          // 回覆巢狀留言時補 @name 前綴，讓對象在 thread 內可辨識
          final text = target.isReplyToReply ? '@${target.name} $raw' : raw;
          created = await ref
              .read(postRepositoryProvider)
              .replyToComment(
                target.apiParentId,
                postId: post.id,
                content: text,
              );
        } else {
          created = await ref
              .read(postRepositoryProvider)
              .commentOnPostRaw(post.id, content: raw);
        }
        if (!context.mounted) return;
        if (created != null) {
          commentCtrl.clear();
          replyTarget.value = null;
          commentFocus.unfocus();
          syncCommentCount(1);
          ref.invalidate(_postCommentsProvider(post.id));
        } else {
          FloatingToast.error(context, target != null ? '回覆失敗' : '留言失敗');
        }
      } catch (e) {
        if (context.mounted) FloatingToast.error(context, '留言失敗');
      } finally {
        if (context.mounted) submitting.value = false;
      }
    }

    Future<void> deleteComment(_CommentVM c) async {
      if (!context.mounted || !deletingCommentIds.value.add(c.id)) return;
      try {
        final ok = await ref.read(postRepositoryProvider).deleteComment(c.id);
        if (!context.mounted) return;
        if (ok) {
          FloatingToast.success(context, '已刪除留言');
          syncCommentCount(-1);
          ref.invalidate(_postCommentsProvider(post.id));
        } else {
          FloatingToast.error(context, '刪除留言失敗');
        }
      } catch (_) {
        if (context.mounted) FloatingToast.error(context, '刪除留言失敗');
      } finally {
        deletingCommentIds.value.remove(c.id);
      }
    }

    final commentsAvailability = postCommentsAvailabilityOfPost(
      ref.watch(storeShareProvider),
      post,
    );
    final store = ref.watch(myStoreSnapshotProvider);
    final publishing = useState(false);
    final target = postPublishTarget(post.publishStatus);
    // 能力還沒讀到（store 為 null）時不下結論，伺服器是最後一道。
    final canPublish =
        store?.can(SellerCapability.SELLER_CAPABILITY_MARKETING) ?? true;
    final canUnpublish =
        store?.can(SellerCapability.SELLER_CAPABILITY_CONTENT_CONTRACT) ?? true;

    Future<void> togglePublish(bool publish) async {
      if (publishing.value) return;
      if (publish && post.imageKeys.isEmpty) {
        FloatingToast.error(
          context,
          '$kPostImageRequiredTitle\n$kPostPublishImageRequiredHint',
        );
        return;
      }
      final confirmed = await showAppConfirmDialog(
        context,
        title: publish ? kPostPublishConfirm : kPostUnpublishConfirm,
        message: publish && post.scheduledPublishAt != null
            ? kPostScheduledPublishWarning
            : null,
        confirmText: publish ? kPostPublishLabel : kPostUnpublishLabel,
      );
      if (!confirmed || !context.mounted) return;
      publishing.value = true;
      try {
        final refreshed = await ref
            .read(postListProvider.notifier)
            .setPublished(post.id, publish: publish);
        if (!refreshed && context.mounted) {
          // 發布或下架已成功，只是重讀列表失敗：不說「失敗」，只交代重讀的原因。
          final error = ref.read(postListProvider).error;
          FloatingToast.error(context, _mutationErrorMessage(error ?? ''));
        }
      } on AppException catch (error) {
        if (context.mounted) {
          final prefix = publish ? '發布失敗' : '下架失敗';
          FloatingToast.error(context, '$prefix：${error.message}');
        }
      } finally {
        if (context.mounted) publishing.value = false;
      }
    }

    final publishControl = target == null
        ? null
        : _PostPublishButton(
            key: ValueKey('seller-post-publish-${post.id}'),
            publish: target == PostPublishTarget.publish,
            hidden: post.isHidden,
            busy: publishing.value,
            disabled: publishDisabled,
            onTap: () => togglePublish(target == PostPublishTarget.publish),
          );
    final showPublishControl =
        publishControl != null &&
        (target == PostPublishTarget.publish ? canPublish : canUnpublish);

    Future<void> openComments() async {
      if (commentsOpen.value || !context.mounted) return;
      commentsOpen.value = true;
      final unavailableMessage = commentsAvailability.message;
      final sheet = showModalBottomSheet<void>(
        context: context,
        useRootNavigator: true,
        useSafeArea: true,
        isScrollControlled: true,
        showDragHandle: false,
        requestFocus: false,
        backgroundColor: Colors.transparent,
        barrierColor: AppColors.neutralBlack.withValues(alpha: 0.42),
        builder: (_) => _PostCommentsSheet(
          postId: post.id,
          controller: commentCtrl,
          focusNode: commentFocus,
          submitting: submitting,
          replyTarget: replyTarget,
          unavailableMessage: unavailableMessage,
          onReply: setReplyTo,
          onDelete: deleteComment,
          onSubmit: submitComment,
          onCancelReply: () {
            if (!submitting.value) replyTarget.value = null;
          },
        ),
      );
      if (replyTarget.value != null) {
        WidgetsBinding.instance.addPostFrameCallback((_) {
          if (context.mounted &&
              commentsOpen.value &&
              commentFocus.context != null) {
            commentFocus.requestFocus();
          }
        });
      }
      try {
        await sheet;
      } finally {
        commentsOpen.value = false;
        if (context.mounted) commentFocus.unfocus();
      }
    }

    final insets = MediaQuery.paddingOf(context);
    final reduceMotion = MediaQuery.disableAnimationsOf(context);
    final duration = reduceMotion ? Duration.zero : HkMotion.duration;
    return Padding(
      padding: EdgeInsets.fromLTRB(
        insets.left + 8,
        insets.top + 8,
        insets.right + 8,
        insets.bottom + 12,
      ),
      child: DecoratedBox(
        key: ValueKey('seller-post-card-${post.id}'),
        decoration: BoxDecoration(
          borderRadius: BorderRadius.circular(HkRadius.xxl),
          boxShadow: HkShadows.modal,
        ),
        child: ClipRRect(
          borderRadius: BorderRadius.circular(HkRadius.xxl),
          child: DecoratedBox(
            decoration: BoxDecoration(
              color: AppColors.bgPrimary,
              border: Border.all(
                color: AppColors.neutralWhite.withValues(alpha: 0.96),
              ),
              borderRadius: BorderRadius.circular(HkRadius.xxl),
            ),
            child: LayoutBuilder(
              builder: (context, constraints) {
                // Keep the top controls and the 64px action row reachable even
                // in short landscape windows. Only the content panel scrolls.
                final contentSpace = math.max(0.0, constraints.maxHeight - 64);
                final panelMaxHeight = math.min(
                  math.min(contentSpace * 0.60, 520.0),
                  math.max(0.0, contentSpace - 72),
                );
                return Stack(
                  children: [
                    Positioned.fill(
                      child: _PostImmersiveMedia(
                        postId: post.id,
                        imageSource: imageSource,
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
                              'seller-post-caption-outside-dismiss-${post.id}',
                            ),
                            behavior: HitTestBehavior.opaque,
                            onTap: captionExpanded.value
                                ? () => captionExpanded.value = false
                                : null,
                            child: AnimatedOpacity(
                              duration: duration,
                              opacity: captionExpanded.value ? 1 : 0,
                              child: BackdropFilter(
                                filter: ui.ImageFilter.blur(
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
                      left: 0,
                      right: 0,
                      bottom: 0,
                      child: HkPostCardBodySurface(
                        surfaceKey: ValueKey(
                          'seller-post-card-body-${post.id}',
                        ),
                        child: Column(
                          mainAxisSize: MainAxisSize.min,
                          children: [
                            ConstrainedBox(
                              constraints: BoxConstraints(
                                maxHeight: panelMaxHeight,
                              ),
                              child: AnimatedSize(
                                duration: duration,
                                alignment: Alignment.bottomCenter,
                                curve: HkMotion.ease,
                                child: SingleChildScrollView(
                                  key: ValueKey(
                                    'seller-post-content-scroll-${post.id}',
                                  ),
                                  primary: false,
                                  physics: const BouncingScrollPhysics(),
                                  padding: const EdgeInsets.all(12),
                                  child: Column(
                                    key: ValueKey(
                                      'seller-post-primary-content-${post.id}',
                                    ),
                                    mainAxisSize: MainAxisSize.min,
                                    crossAxisAlignment:
                                        CrossAxisAlignment.stretch,
                                    children: [
                                      Padding(
                                        padding: const EdgeInsets.all(4),
                                        child: Row(
                                          children: [
                                            const SellerStoreAvatar(size: 42),
                                            const SizedBox(width: 12),
                                            Expanded(
                                              child: Column(
                                                crossAxisAlignment:
                                                    CrossAxisAlignment.start,
                                                children: [
                                                  Text(
                                                    sellerName,
                                                    maxLines: 1,
                                                    overflow:
                                                        TextOverflow.ellipsis,
                                                    style: const TextStyle(
                                                      fontSize: 16,
                                                      fontWeight:
                                                          FontWeight.w700,
                                                      color: AppColors
                                                          .textEmphasis,
                                                      height: 20 / 16,
                                                    ),
                                                  ),
                                                  if (dateStr.isNotEmpty)
                                                    Text(
                                                      dateStr,
                                                      maxLines: 1,
                                                      overflow:
                                                          TextOverflow.ellipsis,
                                                      style: const TextStyle(
                                                        fontSize: 14,
                                                        color: _kGray50,
                                                        height: 18 / 14,
                                                      ),
                                                    ),
                                                  if (statusBadge.scheduledAt !=
                                                      null)
                                                    Text(
                                                      '排程於 ${formatTaipeiDateTime(statusBadge.scheduledAt!)} 發布',
                                                      key: ValueKey(
                                                        'seller-post-scheduled-${post.id}',
                                                      ),
                                                      maxLines: 1,
                                                      overflow:
                                                          TextOverflow.ellipsis,
                                                      style: const TextStyle(
                                                        fontSize: 14,
                                                        color: _kGray50,
                                                        height: 18 / 14,
                                                      ),
                                                    ),
                                                ],
                                              ),
                                            ),
                                            if (statusBadge.label != null) ...[
                                              const SizedBox(width: 8),
                                              HkStatusBadge(
                                                key: ValueKey(
                                                  'seller-post-status-${post.id}',
                                                ),
                                                label: statusBadge.label!,
                                                tone: statusBadge.tone,
                                              ),
                                            ],
                                          ],
                                        ),
                                      ),
                                      if (post.linkedProduct != null) ...[
                                        const SizedBox(height: 8),
                                        _PostLinkedProduct(
                                          key: ValueKey(
                                            'seller-post-linked-product-${post.id}',
                                          ),
                                          product: post.linkedProduct!,
                                          onTap: onProductTap,
                                        ),
                                      ],
                                      if (post.content.isNotEmpty) ...[
                                        const SizedBox(height: 8),
                                        Padding(
                                          padding: const EdgeInsets.symmetric(
                                            horizontal: 4,
                                          ),
                                          child: _PostCaption(
                                            postId: post.id,
                                            content: post.content,
                                            expanded: captionExpanded.value,
                                            onToggle: () =>
                                                captionExpanded.value =
                                                    !captionExpanded.value,
                                          ),
                                        ),
                                      ],
                                    ],
                                  ),
                                ),
                              ),
                            ),
                            _PostActionBar(
                              key: ValueKey(
                                'seller-post-action-bar-${post.id}',
                              ),
                              post: post,
                              onCommentTap: openComments,
                              showShare: postIsPublishedAndVisible(post),
                              onShareTap: onShareTap,
                              shareHint: shareHint,
                              publishControl: showPublishControl
                                  ? publishControl
                                  : null,
                            ),
                          ],
                        ),
                      ),
                    ),
                    Positioned(
                      top: 12,
                      left: 12,
                      child: HkFrostedIconButton(
                        key: const ValueKey('seller-post-back-button'),
                        label: '返回店舖',
                        icon: AppIcons.close,
                        onPressed: () => context.pop(),
                      ),
                    ),
                    Positioned(
                      top: 12,
                      right: 12,
                      child: Row(
                        children: [
                          _PostManagementButton(
                            key: ValueKey('seller-post-edit-${post.id}'),
                            label: '編輯貼文',
                            icon: AppIcons.editSquareOutlined,
                            color: AppIconColors.active,
                            onPressed: onEditTap,
                          ),
                          const SizedBox(width: HkSpace.sm),
                          _PostManagementButton(
                            key: ValueKey('seller-post-delete-${post.id}'),
                            label: '刪除貼文',
                            icon: AppIcons.deleteOutline,
                            color: AppIconColors.danger,
                            onPressed: onDeleteTap,
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

typedef _PostReplyTarget = ({
  int apiParentId,
  String name,
  bool isReplyToReply,
});

class _PostPreviewBackdrop extends StatelessWidget {
  const _PostPreviewBackdrop({required this.imageSource});
  final String imageSource;

  @override
  Widget build(BuildContext context) => Stack(
    fit: StackFit.expand,
    children: [
      const ColoredBox(color: AppColors.bgPrimary),
      if (imageSource.isNotEmpty)
        ImageFiltered(
          imageFilter: ui.ImageFilter.blur(sigmaX: 34, sigmaY: 34),
          child: Transform.scale(
            scale: 1.22,
            child: OptimizedNetworkImage(
              imageUrl: imageSource,
              scene: ImageScene.postImage,
              fit: BoxFit.cover,
              errorWidget: const SizedBox.expand(),
            ),
          ),
        ),
      ColoredBox(color: AppColors.neutralWhite.withValues(alpha: 0.66)),
    ],
  );
}

class _PostImmersiveMedia extends StatelessWidget {
  const _PostImmersiveMedia({required this.postId, required this.imageSource});
  final int postId;
  final String imageSource;

  @override
  Widget build(BuildContext context) => RepaintBoundary(
    key: ValueKey('seller-post-media-$postId'),
    child: Stack(
      fit: StackFit.expand,
      children: [
        const ColoredBox(color: AppColors.bgPrimary),
        if (imageSource.isNotEmpty)
          ImageFiltered(
            imageFilter: ui.ImageFilter.blur(sigmaX: 24, sigmaY: 24),
            child: Transform.scale(
              scale: 1.12,
              child: OptimizedNetworkImage(
                imageUrl: imageSource,
                scene: ImageScene.postImage,
                fit: BoxFit.cover,
                errorWidget: const SizedBox.expand(),
              ),
            ),
          ),
        ColoredBox(color: AppColors.neutralWhite.withValues(alpha: 0.30)),
        if (imageSource.isNotEmpty)
          OptimizedNetworkImage(
            key: ValueKey('seller-post-image-$postId'),
            imageUrl: imageSource,
            scene: ImageScene.postImage,
            fit: BoxFit.contain,
            errorWidget: const Center(
              child: Icon(
                AppIcons.brokenImageOutlined,
                size: 56,
                color: AppIconColors.muted,
              ),
            ),
          )
        else
          const Center(
            child: Icon(
              AppIcons.imageOutlined,
              size: 56,
              color: AppIconColors.muted,
            ),
          ),
        Positioned(
          top: 0,
          left: 0,
          right: 0,
          height: 180,
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
        if (isAnimatedGifMedia(imageSource))
          Positioned(
            top: 72,
            left: 12,
            child: Semantics(
              label: 'GIF 動圖',
              excludeSemantics: true,
              child: DecoratedBox(
                decoration: BoxDecoration(
                  color: AppColors.bgElevated,
                  borderRadius: BorderRadius.circular(HkRadius.pill),
                ),
                child: const Padding(
                  padding: EdgeInsets.symmetric(horizontal: 8, vertical: 4),
                  child: Text(
                    'GIF',
                    style: TextStyle(
                      color: AppColors.textEmphasis,
                      fontSize: 12,
                      fontWeight: FontWeight.w700,
                    ),
                  ),
                ),
              ),
            ),
          ),
      ],
    ),
  );
}

class _PostManagementButton extends StatelessWidget {
  const _PostManagementButton({
    super.key,
    required this.label,
    required this.icon,
    required this.color,
    required this.onPressed,
  });
  final String label;
  final IconData icon;
  final Color color;
  final VoidCallback onPressed;

  @override
  Widget build(BuildContext context) => Semantics(
    label: label,
    button: true,
    enabled: true,
    onTap: onPressed,
    child: ExcludeSemantics(
      child: SizedBox.square(
        dimension: HkControl.minTap,
        child: HkFrostedCardSurface(
          radius: HkRadius.pill,
          shadows: HkShadows.sm,
          child: Material(
            color: Colors.transparent,
            shape: const CircleBorder(),
            clipBehavior: Clip.antiAlias,
            child: IconButton(
              tooltip: label,
              onPressed: onPressed,
              style: IconButton.styleFrom(
                shape: const CircleBorder(),
                side: BorderSide.none,
                padding: EdgeInsets.zero,
              ),
              icon: Icon(icon, size: 22, color: color),
            ),
          ),
        ),
      ),
    ),
  );
}

class _PostLinkedProduct extends StatelessWidget {
  const _PostLinkedProduct({super.key, required this.product, this.onTap});
  final Map<String, dynamic> product;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    final name = product['name']?.toString() ?? '';
    if (name.isEmpty) return const SizedBox.shrink();
    // Same seller price and image resolution as LinkedProductCard.
    final priceRaw = product['price'];
    final price = priceRaw is num
        ? priceRaw.toInt()
        : int.tryParse('${priceRaw ?? ''}') ?? 0;
    final imageUrl = extractPrimaryProductImageUrl(product) ?? '';
    return Semantics(
      label: '查看關聯商品 $name',
      button: true,
      enabled: onTap != null,
      onTap: onTap,
      child: ExcludeSemantics(
        child: Material(
          color: Colors.transparent,
          child: InkWell(
            onTap: onTap,
            borderRadius: BorderRadius.circular(HkRadius.sm),
            child: Padding(
              padding: const EdgeInsets.symmetric(horizontal: 4, vertical: 4),
              child: Row(
                children: [
                  ClipRRect(
                    borderRadius: BorderRadius.circular(HkRadius.sm),
                    child: SizedBox.square(
                      dimension: 44,
                      child: ColoredBox(
                        color: AppColors.gray100,
                        child: imageUrl.isEmpty
                            ? const Icon(
                                AppIcons.imageOutlined,
                                color: AppIconColors.muted,
                              )
                            : OptimizedNetworkImage(
                                imageUrl: imageUrl,
                                // Decode one axis only; a square decode budget
                                // stretches non-square photos before cover crops.
                                width: 44,
                                maxPixels: 240,
                                fit: BoxFit.cover,
                              ),
                      ),
                    ),
                  ),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Text(
                          name,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: const TextStyle(
                            fontSize: 14,
                            fontWeight: FontWeight.w700,
                            color: AppColors.textEmphasis,
                            height: 20 / 14,
                          ),
                        ),
                        Text(
                          'NT\$ $price',
                          style: const TextStyle(
                            fontSize: 14,
                            fontWeight: FontWeight.w700,
                            color: AppColors.brandPrimaryStrong,
                            height: 20 / 14,
                          ),
                        ),
                      ],
                    ),
                  ),
                  const SizedBox(width: 8),
                  const Icon(
                    AppIcons.chevronRight,
                    size: 16,
                    color: AppIconColors.defaultColor,
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

class _PostCaption extends StatelessWidget {
  const _PostCaption({
    required this.postId,
    required this.content,
    required this.expanded,
    required this.onToggle,
  });
  final int postId;
  final String content;
  final bool expanded;
  final VoidCallback onToggle;

  @override
  Widget build(BuildContext context) => LayoutBuilder(
    builder: (context, constraints) {
      const style = TextStyle(
        fontSize: 16,
        color: AppColors.textEmphasis,
        height: 24 / 16,
      );
      final span = TextSpan(children: _buildHashtagSpans(content, style));
      final painter = TextPainter(
        text: span,
        maxLines: 1,
        textDirection: Directionality.of(context),
        textScaler: MediaQuery.textScalerOf(context),
      )..layout(maxWidth: constraints.maxWidth);
      final canToggle = painter.didExceedMaxLines || expanded;
      painter.dispose();
      final text = Text.rich(
        span,
        key: ValueKey('seller-post-caption-text-$postId'),
        maxLines: expanded ? null : 1,
        overflow: expanded ? TextOverflow.visible : TextOverflow.ellipsis,
      );
      if (!canToggle) return text;
      final toggle = HkTapTarget(
        key: ValueKey('seller-post-caption-toggle-$postId'),
        semanticLabel: expanded ? '收起貼文說明' : '查看貼文說明',
        onTap: onToggle,
        child: Padding(
          padding: const EdgeInsets.only(left: 8),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              Text(
                expanded ? '收起說明' : '查看說明',
                style: const TextStyle(
                  fontSize: 14,
                  fontWeight: FontWeight.w700,
                  color: AppColors.actionPrimary,
                ),
              ),
              const SizedBox(width: 4),
              Icon(
                expanded
                    ? AppIcons.keyboardArrowDown
                    : AppIcons.keyboardArrowUp,
                size: 18,
                color: AppIconColors.active,
              ),
            ],
          ),
        ),
      );
      if (!expanded) {
        return Row(
          children: [
            Expanded(child: text),
            const SizedBox(width: 10),
            toggle,
          ],
        );
      }
      return Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          text,
          Row(mainAxisAlignment: MainAxisAlignment.end, children: [toggle]),
        ],
      );
    },
  );
}

class _PostActionBar extends StatelessWidget {
  const _PostActionBar({
    super.key,
    required this.post,
    required this.onCommentTap,
    required this.showShare,
    required this.onShareTap,
    required this.shareHint,
    required this.publishControl,
  });
  final Post post;
  final VoidCallback onCommentTap;

  /// 這篇本身已發布、沒被平台下架才出現分享入口；店家還不能公開時入口停用並附 [shareHint]
  /// （同店內商品分享）。伺服器仍是最後一道。
  final bool showShare;
  final VoidCallback? onShareTap;
  final String? shareHint;
  final Widget? publishControl;

  Widget _shareAction() {
    final label = post.shareCount > 0 ? '分享貼文，共 ${post.shareCount} 次' : '分享貼文';
    final onTap = onShareTap;
    if (onTap == null) {
      return HkDisabledWithHint(
        semanticLabel: '分享貼文',
        hint: shareHint,
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(
              AppIcons.shareOutlined,
              size: 22,
              color: AppIconColors.disabled,
            ),
            if (post.shareCount > 0) ...[
              const SizedBox(width: HkSpace.xs),
              Text(
                _visibleEngagementCount(post.shareCount),
                style: const TextStyle(
                  fontSize: 14,
                  color: AppIconColors.disabled,
                  height: 20 / 14,
                ),
              ),
            ],
          ],
        ),
      );
    }
    return _AssetCountAction(
      semanticLabel: label,
      iconBuilder: (size) =>
          Icon(AppIcons.shareOutlined, size: size, color: _kGray100),
      count: post.shareCount > 0 ? post.shareCount : null,
      onTap: onTap,
    );
  }

  /// 讚數只顯示：店主不能按自己貼文的讚。
  Widget _likeCount() {
    final count = post.likeCount;
    return Semantics(
      container: true,
      label: '共 $count 個讚',
      child: ExcludeSemantics(
        child: ConstrainedBox(
          constraints: const BoxConstraints(minHeight: HkControl.minTap),
          child: Center(
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                const Icon(
                  AppIcons.favoriteBorder,
                  size: 22,
                  color: AppIconColors.defaultColor,
                ),
                const SizedBox(width: HkSpace.xs),
                Text(
                  _visibleEngagementCount(count),
                  style: const TextStyle(
                    fontSize: 14,
                    color: _kGray100,
                    height: 20 / 14,
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) => SizedBox(
    height: 64,
    child: Padding(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
      child: Row(
        children: [
          Expanded(child: _likeCount()),
          Expanded(
            child: _AssetCountAction(
              semanticLabel: '留言，共 ${post.commentCount} 則',
              iconBuilder: (size) => Icon(
                AppIcons.chatBubbleOutline,
                size: size,
                color: _kGray100,
              ),
              count: post.commentCount,
              onTap: onCommentTap,
            ),
          ),
          if (showShare) Expanded(child: _shareAction()),
          if (publishControl != null) ...[
            const SizedBox(width: HkSpace.sm),
            publishControl!,
          ],
        ],
      ),
    ),
  );
}

/// 發布與下架：一顆按鈕，方向由狀態決定。被平台下架時發布停用並說明原因（點一下顯示）；
/// 送出中顯示進行中文字並停用。
class _PostPublishButton extends StatelessWidget {
  const _PostPublishButton({
    super.key,
    required this.publish,
    required this.hidden,
    required this.busy,
    required this.disabled,
    required this.onTap,
  });
  final bool publish;
  final bool hidden;
  final bool busy;

  /// 列表重讀失敗後停用：按鈕依據的發布狀態已不可信。
  final bool disabled;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final label = publish ? kPostPublishLabel : kPostUnpublishLabel;
    final shown = busy
        ? (publish ? kPostPublishPendingLabel : kPostUnpublishPendingLabel)
        : label;
    final blocked = publish && hidden;
    final button = OutlinedButton(
      onPressed: blocked || busy || disabled ? null : onTap,
      style: OutlinedButton.styleFrom(
        minimumSize: const Size(0, 40),
        padding: const EdgeInsets.symmetric(horizontal: HkSpace.lg),
        foregroundColor: publish
            ? AppColors.actionPrimary
            : AppColors.textPrimary,
      ),
      child: HkButtonLabel(shown),
    );
    if (!blocked) return button;
    return HkDisabledWithHint(
      semanticLabel: label,
      hint: kPostPlatformHiddenPublishReason,
      child: IgnorePointer(child: button),
    );
  }
}

class _PostCommentsSheet extends HookConsumerWidget {
  const _PostCommentsSheet({
    required this.postId,
    required this.controller,
    required this.focusNode,
    required this.submitting,
    required this.replyTarget,
    required this.unavailableMessage,
    required this.onReply,
    required this.onDelete,
    required this.onSubmit,
    required this.onCancelReply,
  });
  final int postId;
  final TextEditingController controller;
  final FocusNode focusNode;
  final ValueListenable<bool> submitting;
  final ValueListenable<_PostReplyTarget?> replyTarget;

  /// 非 null 時不載入留言、不顯示輸入列，列表區改顯示這段說明；外框不變。
  final String? unavailableMessage;
  final void Function(_CommentThread, _CommentVM) onReply;
  final void Function(_CommentVM) onDelete;
  final Future<void> Function() onSubmit;
  final VoidCallback onCancelReply;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final unavailable = unavailableMessage;
    final commentsAsync = unavailable == null
        ? ref.watch(_postCommentsProvider(postId))
        : const AsyncData<List<_CommentThread>>(<_CommentThread>[]);
    final threads = commentsAsync.value ?? const <_CommentThread>[];
    final bottom = MediaQuery.viewInsetsOf(context).bottom;
    final viewport = MediaQuery.sizeOf(context);
    useEffect(() {
      // A short keyboard viewport changes the sheet's scrolling arrangement.
      // Keep the existing focused field visible after its new parent lays out.
      WidgetsBinding.instance.addPostFrameCallback((_) {
        final fieldContext = focusNode.context;
        if (context.mounted && focusNode.hasFocus && fieldContext != null) {
          Scrollable.ensureVisible(fieldContext, alignment: 1);
        }
      });
      return null;
    }, [bottom, viewport]);
    return LayoutBuilder(
      builder: (context, constraints) {
        final height = math.max(
          0.0,
          math.min(
            MediaQuery.sizeOf(context).height * 0.88,
            constraints.maxHeight - bottom,
          ),
        );
        final short = height < 240;
        final header = SizedBox(
          height: 72,
          child: Column(
            children: [
              Padding(
                padding: const EdgeInsets.only(top: 8, bottom: 4),
                child: SizedBox(
                  width: 36,
                  height: 4,
                  child: DecoratedBox(
                    decoration: BoxDecoration(
                      color: AppColors.borderSubtle,
                      borderRadius: BorderRadius.circular(HkRadius.pill),
                    ),
                  ),
                ),
              ),
              Padding(
                padding: const EdgeInsets.symmetric(horizontal: 16),
                child: Row(
                  children: [
                    Expanded(
                      child: Text(
                        unavailable != null
                            ? '留言'
                            : '留言 ${threads.fold<int>(0, (sum, thread) => sum + thread.total)}',
                        style: const TextStyle(
                          fontSize: 18,
                          fontWeight: FontWeight.w700,
                          color: AppColors.textEmphasis,
                        ),
                      ),
                    ),
                    HkFrostedIconButton(
                      key: ValueKey('seller-post-close-comments-$postId'),
                      label: '關閉留言',
                      icon: AppIcons.close,
                      onPressed: () => Navigator.of(context).pop(),
                    ),
                  ],
                ),
              ),
            ],
          ),
        );
        final commentContent = unavailable != null
            ? Padding(
                padding: const EdgeInsets.symmetric(vertical: HkSpace.xxl),
                child: StorefrontGlassEmptyPanel(
                  icon: HkIconName.chat,
                  message: unavailable,
                ),
              )
            : commentsAsync.when(
                loading: () => const Padding(
                  padding: EdgeInsets.all(24),
                  child: Center(child: FruitSpinner(size: 40)),
                ),
                error: (_, _) => Padding(
                  padding: const EdgeInsets.all(16),
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      const Text('留言載入失敗'),
                      TextButton(
                        onPressed: () =>
                            ref.invalidate(_postCommentsProvider(postId)),
                        child: const HkButtonLabel('重試'),
                      ),
                    ],
                  ),
                ),
                data: (_) => threads.isEmpty
                    ? const Padding(
                        padding: EdgeInsets.all(24),
                        child: Center(child: Text('尚無留言')),
                      )
                    : Column(
                        children: [
                          for (final thread in threads)
                            _CommentGroup(
                              thread: thread,
                              onReply: (comment) => onReply(thread, comment),
                              onDelete: onDelete,
                            ),
                        ],
                      ),
              );
        final composer = ValueListenableBuilder<bool>(
          valueListenable: submitting,
          builder: (context, busy, _) =>
              ValueListenableBuilder<_PostReplyTarget?>(
                valueListenable: replyTarget,
                builder: (context, target, _) => _PostCommentComposer(
                  key: ValueKey('seller-post-action-composer-$postId'),
                  postId: postId,
                  controller: controller,
                  focusNode: focusNode,
                  onSubmit: onSubmit,
                  submitting: busy,
                  replyName: target?.name,
                  onCancelReply: onCancelReply,
                  maxLines: short ? 1 : 4,
                ),
              ),
        );
        return Padding(
          padding: EdgeInsets.only(bottom: bottom),
          child: SizedBox(
            height: height,
            child: ClipRRect(
              borderRadius: const BorderRadius.vertical(
                top: Radius.circular(HkRadius.xxl),
              ),
              child: ColoredBox(
                key: ValueKey('seller-post-comments-sheet-$postId'),
                color: AppColors.bgElevated,
                child: SafeArea(
                  top: false,
                  child: short
                      ? SingleChildScrollView(
                          key: ValueKey('seller-post-comments-scroll-$postId'),
                          keyboardDismissBehavior:
                              ScrollViewKeyboardDismissBehavior.onDrag,
                          child: Column(
                            mainAxisSize: MainAxisSize.min,
                            children: [
                              header,
                              commentContent,
                              if (unavailable == null) composer,
                            ],
                          ),
                        )
                      : Column(
                          children: [
                            header,
                            Expanded(
                              child: SingleChildScrollView(
                                key: ValueKey(
                                  'seller-post-comments-scroll-$postId',
                                ),
                                keyboardDismissBehavior:
                                    ScrollViewKeyboardDismissBehavior.onDrag,
                                child: commentContent,
                              ),
                            ),
                            if (unavailable == null)
                              ConstrainedBox(
                                constraints: BoxConstraints(
                                  maxHeight: math.max(48, (height - 72) * 0.62),
                                ),
                                child: SingleChildScrollView(child: composer),
                              ),
                          ],
                        ),
                ),
              ),
            ),
          ),
        );
      },
    );
  }
}

class _PostCommentComposer extends StatelessWidget {
  const _PostCommentComposer({
    super.key,
    required this.postId,
    required this.controller,
    required this.focusNode,
    required this.onSubmit,
    required this.submitting,
    required this.replyName,
    required this.onCancelReply,
    required this.maxLines,
  });
  final int postId;
  final TextEditingController controller;
  final FocusNode focusNode;
  final Future<void> Function() onSubmit;
  final bool submitting;
  final String? replyName;
  final VoidCallback onCancelReply;
  final int maxLines;

  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.fromLTRB(16, 8, 16, 12),
    child: Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        if (replyName != null)
          Row(
            children: [
              const Text(
                '回覆 ',
                style: TextStyle(fontSize: 14, color: _kGray50),
              ),
              Expanded(
                child: Text(
                  '@$replyName',
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(
                    fontSize: 14,
                    color: AppColors.primary,
                    fontWeight: FontWeight.w700,
                  ),
                ),
              ),
              HkTapTarget(
                semanticLabel: '取消回覆',
                onTap: submitting ? null : onCancelReply,
                child: const Icon(AppIcons.close, size: 18, color: _kGray100),
              ),
            ],
          ),
        Row(
          crossAxisAlignment: CrossAxisAlignment.end,
          children: [
            Expanded(
              child: _InlineCommentField(
                key: ValueKey('seller-post-comment-input-$postId'),
                controller: controller,
                focusNode: focusNode,
                onSubmit: onSubmit,
                enabled: !submitting,
                hintText: replyName != null ? '回覆 $replyName...' : '新增貼文留言',
                maxLines: maxLines,
              ),
            ),
            const SizedBox(width: 8),
            ValueListenableBuilder<TextEditingValue>(
              valueListenable: controller,
              builder: (context, value, _) => HkFrostedIconButton(
                label: '送出留言',
                loading: submitting,
                loadingLabel: '留言送出中',
                icon: AppIcons.sendRounded,
                onPressed: !submitting && value.text.trim().isNotEmpty
                    ? onSubmit
                    : null,
              ),
            ),
          ],
        ),
      ],
    ),
  );
}

// ─── Inline 留言輸入框（鍵盤 send 鍵觸發送出；對齊 buyer） ─────────────
class _InlineCommentField extends StatelessWidget {
  const _InlineCommentField({
    super.key,
    required this.controller,
    required this.focusNode,
    required this.onSubmit,
    this.enabled = true,
    this.hintText = '新增貼文留言',
    this.maxLines = 4,
  });

  final TextEditingController controller;
  final FocusNode focusNode;
  final Future<void> Function() onSubmit;
  final bool enabled;
  final String hintText;
  final int maxLines;

  @override
  Widget build(BuildContext context) {
    return Semantics(
      hint: '輸入內容後，按鍵盤送出鍵發表留言',
      child: Container(
        constraints: const BoxConstraints(minHeight: HkControl.minTap),
        padding: const EdgeInsets.symmetric(
          horizontal: HkSpace.md,
          vertical: HkSpace.sm,
        ),
        decoration: BoxDecoration(
          color: AppColors.surfaceMuted,
          border: Border.all(color: AppColors.borderSubtle),
          borderRadius: BorderRadius.circular(HkRadius.pill),
        ),
        child: TextField(
          controller: controller,
          focusNode: focusNode,
          enabled: enabled,
          minLines: 1,
          maxLines: maxLines,
          textInputAction: TextInputAction.send,
          onSubmitted: (_) => onSubmit(),
          style: const TextStyle(
            fontSize: 14,
            color: _kGray100,
            height: 20 / 14,
          ),
          decoration: InputDecoration(
            filled: false,
            isDense: true,
            contentPadding: EdgeInsets.zero,
            border: InputBorder.none,
            enabledBorder: InputBorder.none,
            focusedBorder: InputBorder.none,
            disabledBorder: InputBorder.none,
            errorBorder: InputBorder.none,
            focusedErrorBorder: InputBorder.none,
            hintText: hintText,
            hintStyle: const TextStyle(
              fontSize: 14,
              color: _kGray50,
              height: 20 / 14,
            ),
          ),
        ),
      ),
    );
  }
}

class _AssetCountAction extends StatefulWidget {
  const _AssetCountAction({
    required this.semanticLabel,
    required this.iconBuilder,
    this.count,
    required this.onTap,
  });
  final String semanticLabel;
  final Widget Function(double size) iconBuilder;
  final int? count;
  final VoidCallback onTap;

  @override
  State<_AssetCountAction> createState() => _AssetCountActionState();
}

class _AssetCountActionState extends State<_AssetCountAction> {
  bool _pressed = false;

  @override
  Widget build(BuildContext context) {
    final reduceMotion = MediaQuery.maybeDisableAnimationsOf(context) ?? false;
    return Semantics(
      button: true,
      label: widget.semanticLabel,
      onTap: widget.onTap,
      child: ExcludeSemantics(
        child: Material(
          color: Colors.transparent,
          child: InkWell(
            onHighlightChanged: (value) => setState(() => _pressed = value),
            onTap: () {
              HapticFeedback.selectionClick();
              widget.onTap();
            },
            borderRadius: BorderRadius.circular(HkRadius.pill),
            child: ConstrainedBox(
              constraints: const BoxConstraints(
                minWidth: HkControl.minTap,
                minHeight: HkControl.minTap,
              ),
              child: Center(
                child: AnimatedScale(
                  duration: reduceMotion
                      ? Duration.zero
                      : const Duration(milliseconds: 140),
                  curve: Curves.easeOutCubic,
                  scale: _pressed && !reduceMotion ? 0.94 : 1,
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      widget.iconBuilder(22),
                      if (widget.count != null) ...[
                        const SizedBox(width: HkSpace.xs),
                        Text(
                          _visibleEngagementCount(widget.count!),
                          style: const TextStyle(
                            fontSize: 14,
                            color: _kGray100,
                            height: 20 / 14,
                          ),
                        ),
                      ],
                    ],
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

String _visibleEngagementCount(int count) {
  final safeCount = count < 0 ? 0 : count;
  return safeCount > 99 ? '99+' : '$safeCount';
}

// ─── 留言單則（對齊 buyer _CommentRow：avatar 30 + name + 時間 + ⋯ +
// 內文 + 「💬 回覆」；reply 左縮排，引導線由 _CommentGroup 統一畫） ─────────
class _CommentRow extends ConsumerWidget {
  const _CommentRow({
    required this.comment,
    required this.dateStr,
    required this.isReply,
    required this.onReply,
    required this.onDelete,
  });

  final _CommentVM comment;
  final String dateStr;
  final bool isReply;
  final VoidCallback onReply;
  final VoidCallback onDelete;

  bool get _isSelf => comment.isMine;

  Future<void> _showActionMenu(BuildContext context, WidgetRef ref) async {
    final overlay = Overlay.of(context).context.findRenderObject() as RenderBox;
    final box = context.findRenderObject() as RenderBox;
    final position = RelativeRect.fromRect(
      Rect.fromPoints(
        box.localToGlobal(Offset.zero, ancestor: overlay),
        box.localToGlobal(box.size.bottomRight(Offset.zero), ancestor: overlay),
      ),
      Offset.zero & overlay.size,
    );
    // 賣家在自己貼文內可管理所有留言 → 一律提供刪除；
    // 非自己的留言額外提供檢舉（公開留言不帶留言者 id，目前沒有封鎖入口）
    final v = await showMenu<String>(
      context: context,
      position: position,
      color: AppColors.bgCard,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12.r)),
      items: [
        PopupMenuItem<String>(
          value: 'delete',
          child: Text(
            '刪除留言',
            style: TextStyle(fontSize: 16.sp, color: AppColors.error),
          ),
        ),
        if (!_isSelf) ...[
          PopupMenuItem<String>(
            value: 'report',
            child: Text('檢舉留言', style: TextStyle(fontSize: 16.sp)),
          ),
        ],
      ],
    );
    if (!context.mounted) return;
    if (v == 'delete') onDelete();
    if (v == 'report' && context.mounted) {
      // 本頁的留言來自 `/public/posts/{id}/comments`（`listCommentsRaw`），
      // 落在 `cms_comment`——所以是 POST_COMMENT，不是 SELLER_FORUM
      // （那是賣家頁「迴響」留言板，打到 `cms_seller_forum`）。
      await showReportSheet(
        context,
        ref,
        targetType: ReportTargetType.postComment,
        targetId: '${comment.id}',
      );
    }
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final name = comment.name.trim().isEmpty ? '匿名' : comment.name.trim();
    final leftPad = isReply ? 46.w : 10.w;

    return Padding(
      padding: EdgeInsets.fromLTRB(leftPad, 8.h, 12.w, 8.h),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          _CommentAvatar(size: 30.sp, url: comment.avatar),
          SizedBox(width: 6.w),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Expanded(
                      child: Text(
                        name,
                        style: TextStyle(
                          fontSize: 16.sp,
                          fontWeight: FontWeight.w700,
                          color: AppColors.textEmphasis,
                          height: 24 / 16,
                        ),
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                      ),
                    ),
                    SizedBox(width: 8.w),
                    Builder(
                      builder: (innerContext) => HkFrostedIconButton(
                        label: '留言更多操作',
                        icon: AppIcons.moreHoriz,
                        onPressed: () => _showActionMenu(innerContext, ref),
                      ),
                    ),
                  ],
                ),
                if (dateStr.isNotEmpty)
                  Text(
                    dateStr,
                    style: TextStyle(
                      fontSize: 14.sp,
                      color: _kGray50,
                      height: 20 / 14,
                    ),
                  ),
                if (comment.content.isNotEmpty) ...[
                  SizedBox(height: 4.h),
                  Text(
                    comment.content,
                    style: TextStyle(
                      fontSize: 14.sp,
                      color: _kGray100,
                      height: 20 / 14,
                    ),
                  ),
                ],
                SizedBox(height: 6.h),
                Align(
                  alignment: Alignment.centerLeft,
                  child: SizedBox(
                    width: 80,
                    child: HkTapTarget(
                      semanticLabel: '回覆 $name',
                      onTap: onReply,
                      child: Row(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          Icon(
                            AppIcons.chatBubbleOutline,
                            size: 16.sp,
                            color: _kGray100.withValues(alpha: 0.6),
                          ),
                          SizedBox(width: 4.w),
                          Text(
                            '回覆',
                            style: TextStyle(
                              fontSize: 14.sp,
                              color: _kGray100.withValues(alpha: 0.6),
                              height: 18 / 13,
                            ),
                          ),
                        ],
                      ),
                    ),
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

/// Comment group：parent + replies（seller API 以 parent_id 串 thread）
/// 引導線：parent avatar 中心垂直往下 → 彎弧形連到 reply avatar 邊緣
/// （對齊 buyer _CommentGroup / _ReplyLinePainter）
class _CommentGroup extends StatelessWidget {
  const _CommentGroup({
    required this.thread,
    required this.onReply,
    required this.onDelete,
  });

  final _CommentThread thread;
  final void Function(_CommentVM) onReply;
  final void Function(_CommentVM) onDelete;

  @override
  Widget build(BuildContext context) {
    Widget rowFor(_CommentVM c, {required bool isReply}) => _CommentRow(
      comment: c,
      dateStr: _relativeDate(c.createdAt),
      isReply: isReply,
      onReply: () => onReply(c),
      onDelete: () => onDelete(c),
    );

    final parent = thread.parent;
    final replies = thread.replies;
    if (replies.isEmpty) return rowFor(parent, isReply: false);

    // 父 avatar 中心 X = 10w (row left pad) + 15.sp (avatar radius)
    final parentLineX = 10.w + 15.sp;
    // reply avatar 中心 X = 46w (row left pad) + 15.sp (avatar radius)
    final replyAvatarCenterX = 46.w + 15.sp;
    // reply row 內 avatar 中心 Y = 8h (row top pad) + 15.sp (avatar radius)
    final replyAvatarCenterY = 8.h + 15.sp;

    return Stack(
      children: [
        // 主垂直線：從 parent avatar 底部延伸到 group 底（穿過所有 replies）
        Positioned(
          left: parentLineX - 0.5,
          top: 8.h + 30.sp, // parent avatar bottom (row top pad + avatar size)
          bottom: 0,
          child: Container(width: 1, color: _kBorder),
        ),
        Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            rowFor(parent, isReply: false),
            for (final r in replies)
              CustomPaint(
                painter: _ReplyLinePainter(
                  lineColor: _kBorder,
                  parentLineX: parentLineX,
                  replyAvatarCenterX: replyAvatarCenterX,
                  replyAvatarCenterY: replyAvatarCenterY,
                ),
                child: rowFor(r, isReply: true),
              ),
          ],
        ),
      ],
    );
  }
}

/// 從 parent 垂直引導線在 reply row 內彎弧形連到 reply avatar 中心
class _ReplyLinePainter extends CustomPainter {
  const _ReplyLinePainter({
    required this.lineColor,
    required this.parentLineX,
    required this.replyAvatarCenterX,
    required this.replyAvatarCenterY,
  });

  final Color lineColor;
  final double parentLineX;
  final double replyAvatarCenterX;
  final double replyAvatarCenterY;

  @override
  void paint(Canvas canvas, Size size) {
    final paint = Paint()
      ..color = lineColor
      ..strokeWidth = 1
      ..style = PaintingStyle.stroke;

    const curveRadius = 15.0;
    final path = Path();
    path.moveTo(parentLineX, 0);
    if (replyAvatarCenterY > curveRadius) {
      path.lineTo(parentLineX, replyAvatarCenterY - curveRadius);
    }
    path.quadraticBezierTo(
      parentLineX,
      replyAvatarCenterY,
      parentLineX + curveRadius,
      replyAvatarCenterY,
    );
    path.lineTo(replyAvatarCenterX, replyAvatarCenterY);
    canvas.drawPath(path, paint);
  }

  @override
  bool shouldRepaint(covariant _ReplyLinePainter oldDelegate) {
    return oldDelegate.lineColor != lineColor ||
        oldDelegate.parentLineX != parentLineX ||
        oldDelegate.replyAvatarCenterX != replyAvatarCenterX ||
        oldDelegate.replyAvatarCenterY != replyAvatarCenterY;
  }
}

/// 留言者頭像（objectKey 或完整 URL 一律過 [buildImageUrl] 唯一組裝入口，
/// 49 號 §8.6；失敗 fallback 預設人像 — 對齊 CommentsBottomSheet 邏輯）
class _CommentAvatar extends StatelessWidget {
  const _CommentAvatar({required this.size, required this.url});

  final double size;
  final String url;

  @override
  Widget build(BuildContext context) {
    final s = url.trim();
    final defaultAvatar = Icon(
      AppIcons.personOutline,
      size: size * 0.6,
      color: AppIconColors.onBrand,
    );

    final String resolved = s.isEmpty
        ? ''
        : buildImageUrl(s, ImagePreset.avatar);

    return Container(
      width: size,
      height: size,
      decoration: const BoxDecoration(
        shape: BoxShape.circle,
        color: AppColors.gray100,
      ),
      clipBehavior: Clip.antiAlias,
      child: resolved.startsWith('http')
          ? Image(
              image: NetworkImage(resolved),
              fit: BoxFit.cover,
              width: size,
              height: size,
              gaplessPlayback: true,
              errorBuilder: (_, __, ___) => defaultAvatar,
            )
          : defaultAvatar,
    );
  }
}

/// 留言用相對時間：剛剛 / N 分鐘前 / N 小時前 / N 天前 / 否則 yyyy/MM/dd
String _relativeDate(dynamic dateVal) {
  if (dateVal == null) return '';
  DateTime? dt;
  if (dateVal is String) {
    dt = DateTime.tryParse(dateVal);
    if (dt == null) {
      final n = int.tryParse(dateVal);
      if (n != null) {
        dt = DateTime.fromMillisecondsSinceEpoch(
          n < 10000000000 ? n * 1000 : n,
        );
      }
    }
  }
  if (dateVal is int) {
    final ms = dateVal < 10000000000 ? dateVal * 1000 : dateVal;
    dt = DateTime.fromMillisecondsSinceEpoch(ms);
  }
  if (dt == null) return '';
  final diff = DateTime.now().difference(dt);
  if (diff.inSeconds < 60) return '剛剛';
  if (diff.inMinutes < 60) return '${diff.inMinutes} 分鐘前';
  if (diff.inHours < 24) return '${diff.inHours} 小時前';
  if (diff.inDays < 7) return '${diff.inDays} 天前';
  return '${dt.year}/${dt.month.toString().padLeft(2, '0')}/${dt.day.toString().padLeft(2, '0')}';
}

/// 把 `#xxx` token 切出來，上品牌深綠 + w700（跟商品上架的 ExtendedTextField
/// + HashtagSpanBuilder 對齊）。其他文字用 baseStyle。
final RegExp _kHashtagPattern = RegExp(r'#[\p{L}\p{N}_]+', unicode: true);

List<InlineSpan> _buildHashtagSpans(String content, TextStyle baseStyle) {
  final spans = <InlineSpan>[];
  final tagStyle = baseStyle.copyWith(
    color: AppColors.brandPrimaryStrong,
    fontWeight: FontWeight.w700,
  );
  int last = 0;
  for (final m in _kHashtagPattern.allMatches(content)) {
    if (m.start > last) {
      spans.add(
        TextSpan(text: content.substring(last, m.start), style: baseStyle),
      );
    }
    spans.add(TextSpan(text: m.group(0)!, style: tagStyle));
    last = m.end;
  }
  if (last < content.length) {
    spans.add(TextSpan(text: content.substring(last), style: baseStyle));
  }
  return spans;
}
