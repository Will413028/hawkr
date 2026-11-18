import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';

import 'dart:async';
import 'dart:convert';
import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:flutter_hooks/flutter_hooks.dart';

import '../../features/post/data/post_comment.dart';
import '../../features/post/data/post_repository.dart';
import '../utils/floating_toast.dart';
import 'chat_bubble_icon.dart';
import 'hk_tap_target.dart';
import 'hk_frosted_icon_button.dart';
import 'truck_loading_overlay.dart';

/// Figma-aligned comments bottom sheet (node 6976:67372).
/// Opens with showModalBottomSheet(isScrollControlled: true).
class CommentsBottomSheet extends HookConsumerWidget {
  const CommentsBottomSheet({
    super.key,
    required this.postId,
    required this.postRepository,
    required this.initialCommentCount,
    this.sellerAvatarBase64,
    this.sellerAvatarUrl,
    this.onCommentCountChanged,
    this.notice,
    this.canComment = false,
  });
  final int postId;
  final PostRepository postRepository;
  final String? sellerAvatarBase64;
  final String? sellerAvatarUrl;

  /// 開啟時這則貼文的 `comment_count`（呼叫端手上的值）；留言與刪除成功時依它加減一，結果交給 [onCommentCountChanged]。
  /// 伺服器的 `comment_count` 把頂層留言與回覆各算一則，刪除一則留言只讓它少一（底下的回覆仍在），所以每次成功的寫入就是 ±1。
  final int initialCommentCount;
  final ValueChanged<int>? onCommentCountChanged;

  /// 非 null 時買家看不到這則貼文的留言：列表上方顯示這段說明，留言照常載入與刪除。
  final String? notice;

  /// 伺服器的 `Post.viewer_can_comment`：true 才顯示輸入列，與 [notice] 無關。
  final bool canComment;

  static ImageProvider? _resolveCommentAvatar(
    PostComment c, {
    required String? sellerAvatarBase64,
  }) {
    if (c.authorAvatar.startsWith('http')) {
      return NetworkImage(c.authorAvatar);
    }
    if (c.isMine &&
        sellerAvatarBase64 != null &&
        sellerAvatarBase64.isNotEmpty) {
      try {
        return MemoryImage(base64Decode(sellerAvatarBase64));
      } catch (_) {}
    }
    return null;
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final ctrl = useTextEditingController();
    final focus = useFocusNode();
    final scroll = useScrollController();
    final hasText = useState(false);

    final comments = useState<List<PostComment>>(<PostComment>[]);
    final loading = useState(true);
    final sending = useState(false);
    final replyToCommentId = useState<String?>(null);
    final replyToName = useState<String?>(null);

    final repliesByParentRef = useRef(<String, List<PostComment>>{});
    final repliesLoadingRef = useRef(<String>{});
    // 讀取成功過的串；只有讀取成功才算載入，送出回覆不算（先前讀取失敗時，既有的回覆還要讀）。
    final repliesLoadedRef = useRef(<String>{});
    // 讀取失敗的串：不在每次重繪時重讀，等下一次送出回覆或重新載入再試。
    final repliesFailedRef = useRef(<String>{});

    // Listen to text changes for send button opacity
    useEffect(() {
      void listener() {
        hasText.value = ctrl.text.trim().isNotEmpty;
      }

      ctrl.addListener(listener);
      return () => ctrl.removeListener(listener);
    }, [ctrl]);

    final commentCount = useRef(initialCommentCount);
    void adjustCommentCount(int delta) {
      commentCount.value = math.max(0, commentCount.value + delta);
      onCommentCountChanged?.call(commentCount.value);
    }

    void scheduleScrollToBottom({bool animated = true}) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (!context.mounted || !scroll.hasClients) return;
        final maxExtent = scroll.position.maxScrollExtent;
        if (animated) {
          scroll.animateTo(
            maxExtent,
            duration: const Duration(milliseconds: 250),
            curve: Curves.easeOut,
          );
        } else {
          scroll.jumpTo(maxExtent);
        }
      });
    }

    Future<void> load() async {
      loading.value = true;
      try {
        final list = await postRepository.listComments(postId);
        if (!context.mounted) return;
        repliesByParentRef.value.clear();
        repliesLoadingRef.value.clear();
        repliesLoadedRef.value.clear();
        repliesFailedRef.value.clear();
        comments.value = list;
        scheduleScrollToBottom(animated: false);
      } catch (_) {
        if (context.mounted) FloatingToast.error(context, '留言載入失敗');
      } finally {
        if (context.mounted) loading.value = false;
      }
    }

    useEffect(() {
      load();
      return null;
    }, const []);

    Future<void> send() async {
      final text = ctrl.text.trim();
      if (text.isEmpty || sending.value) return;
      final parentId = replyToCommentId.value;
      sending.value = true;
      try {
        final created = await postRepository.createComment(
          postId,
          body: text,
          parentId: parentId,
        );
        adjustCommentCount(1);
        if (!context.mounted) return;
        ctrl.clear();
        if (parentId != null) {
          final list = repliesByParentRef.value[parentId] ?? <PostComment>[];
          repliesByParentRef.value[parentId] = [...list, created];
          repliesFailedRef.value.remove(parentId);
          replyToCommentId.value = null;
          replyToName.value = null;
          comments.value = List.of(comments.value);
        } else {
          comments.value = [...comments.value, created];
        }
        scheduleScrollToBottom();
      } catch (_) {
        if (context.mounted) {
          FloatingToast.error(context, parentId != null ? '回覆失敗' : '留言失敗');
        }
      } finally {
        if (context.mounted) sending.value = false;
      }
    }

    void onTapReply(String name, String? parentId) {
      if (sending.value) return;
      replyToCommentId.value = parentId;
      replyToName.value = name;
      final mention = '@$name ';
      ctrl.text = mention;
      ctrl.selection = TextSelection.fromPosition(
        TextPosition(offset: ctrl.text.length),
      );
      FocusScope.of(context).requestFocus(focus);
    }

    void cancelReply() {
      if (sending.value) return;
      final name = replyToName.value;
      final prefix = name == null ? '' : '@$name ';
      final remaining = prefix.isNotEmpty && ctrl.text.startsWith(prefix)
          ? ctrl.text.substring(prefix.length)
          : ctrl.text;
      replyToCommentId.value = null;
      replyToName.value = null;
      ctrl.value = TextEditingValue(
        text: remaining,
        selection: TextSelection.collapsed(offset: remaining.length),
      );
    }

    Future<bool> deleteComment(PostComment target) async {
      try {
        await postRepository.deleteComment(target.id, etag: target.etag);
      } catch (_) {
        if (context.mounted) FloatingToast.error(context, '刪除留言失敗');
        return false;
      }
      adjustCommentCount(-1);
      if (!context.mounted) return false;
      await load();
      if (!context.mounted) return true;
      FloatingToast.success(context, '已刪除留言');
      return true;
    }

    final likesInFlight = useRef(<String>{});
    PostComment? currentOf(String id) {
      for (final c in comments.value) {
        if (c.id == id) return c;
      }
      for (final list in repliesByParentRef.value.values) {
        for (final r in list) {
          if (r.id == id) return r;
        }
      }
      return null;
    }

    Future<void> toggleLike(PostComment stale) async {
      if (!likesInFlight.value.add(stale.id)) return;
      final target = currentOf(stale.id) ?? stale;
      final liked = !target.viewerLiked;
      try {
        if (liked) {
          await postRepository.likeComment(target.id);
        } else {
          await postRepository.unlikeComment(target.id);
        }
      } catch (_) {
        likesInFlight.value.remove(target.id);
        if (context.mounted) FloatingToast.error(context, '操作失敗');
        return;
      }
      likesInFlight.value.remove(target.id);
      if (!context.mounted) return;
      PostComment swap(PostComment c) =>
          c.id == target.id ? c.withLiked(liked) : c;
      comments.value = [for (final c in comments.value) swap(c)];
      repliesByParentRef.value.updateAll(
        (_, list) => [for (final r in list) swap(r)],
      );
    }

    Future<bool> showDeleteConfirmationDialog() async {
      final bool? confirm = await showDialog<bool>(
        context: context,
        barrierDismissible: false,
        builder: (ctx) => Dialog(
          backgroundColor: Colors.transparent,
          insetPadding: EdgeInsets.symmetric(horizontal: 40.w),
          child: Container(
            decoration: BoxDecoration(
              color: AppColors.dividerLight,
              borderRadius: BorderRadius.circular(24.r),
            ),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Container(
                  width: double.infinity,
                  padding: EdgeInsets.symmetric(
                    vertical: 24.h,
                    horizontal: 16.w,
                  ),
                  decoration: BoxDecoration(
                    color: AppColors.bgCard,
                    borderRadius: BorderRadius.vertical(
                      top: Radius.circular(24.r),
                    ),
                  ),
                  child: Text(
                    '確定要刪除此留言嗎？\n刪除後將無法復原。',
                    textAlign: TextAlign.center,
                    style: TextStyle(
                      fontSize: 18.sp,
                      color: AppColors.textEmphasis,
                      height: 1.6,
                    ),
                  ),
                ),
                const Divider(
                  height: 0,
                  thickness: 1,
                  color: AppColors.gray300,
                ),
                Row(
                  children: [
                    Expanded(
                      child: Material(
                        color: AppColors.bgCard,
                        borderRadius: BorderRadius.only(
                          bottomLeft: Radius.circular(24.r),
                        ),
                        child: InkWell(
                          borderRadius: BorderRadius.only(
                            bottomLeft: Radius.circular(24.r),
                          ),
                          onTap: () => Navigator.of(ctx).pop(false),
                          child: SizedBox(
                            height: 54.h,
                            child: Center(
                              child: Text(
                                '取消',
                                style: TextStyle(
                                  fontSize: 20.sp,
                                  fontWeight: FontWeight.w700,
                                  color: AppColors.textEmphasis,
                                ),
                              ),
                            ),
                          ),
                        ),
                      ),
                    ),
                    Container(width: 1, height: 54.h, color: AppColors.gray300),
                    Expanded(
                      child: Material(
                        color: AppColors.bgCard,
                        borderRadius: BorderRadius.only(
                          bottomRight: Radius.circular(24.r),
                        ),
                        child: InkWell(
                          borderRadius: BorderRadius.only(
                            bottomRight: Radius.circular(24.r),
                          ),
                          onTap: () => Navigator.of(ctx).pop(true),
                          child: SizedBox(
                            height: 54.h,
                            child: Center(
                              child: Text(
                                '刪除',
                                style: TextStyle(
                                  fontSize: 20.sp,
                                  fontWeight: FontWeight.w700,
                                  color: AppColors.danger,
                                ),
                              ),
                            ),
                          ),
                        ),
                      ),
                    ),
                  ],
                ),
              ],
            ),
          ),
        ),
      );
      return confirm == true;
    }

    Future<void> confirmAndDeleteComment(PostComment target) async {
      final confirmed = await showDeleteConfirmationDialog();
      if (!confirmed) return;
      await deleteComment(target);
    }

    String resolveDisplayName(PostComment c) =>
        c.authorName.isEmpty ? '用戶' : c.authorName;

    final mq = MediaQuery.of(context);
    final bottom = mq.viewInsets.bottom;

    return AnimatedPadding(
      duration: const Duration(milliseconds: 150),
      padding: EdgeInsets.only(bottom: bottom),
      child: FractionallySizedBox(
        heightFactor: 0.65,
        child: Material(
          color: AppColors.bgCard,
          borderRadius: BorderRadius.vertical(top: Radius.circular(24.r)),
          child: Column(
            children: [
              // ── Drag handle + close ──
              SizedBox(
                height: 56,
                child: Stack(
                  alignment: Alignment.topCenter,
                  children: [
                    Padding(
                      padding: EdgeInsets.only(top: 20.h),
                      child: Container(
                        width: 84.w,
                        height: 5.h,
                        decoration: BoxDecoration(
                          color: AppColors.dragHandle,
                          borderRadius: BorderRadius.circular(999),
                        ),
                      ),
                    ),
                    Positioned(
                      top: 4,
                      right: 8,
                      child: HkTapTarget(
                        key: const ValueKey('comments-sheet-close'),
                        semanticLabel: '關閉留言',
                        hint: '關閉留言面板',
                        tooltip: '關閉留言',
                        onTap: () => Navigator.of(context).pop(),
                        child: Icon(
                          AppIcons.close,
                          size: 20.sp,
                          color: AppIconColors.strong,
                        ),
                      ),
                    ),
                  ],
                ),
              ),

              // ── Comments list ──
              if (notice != null)
                Padding(
                  key: const ValueKey('comments-sheet-notice'),
                  padding: EdgeInsets.fromLTRB(16.w, 0, 16.w, 8.h),
                  child: Text(
                    notice!,
                    style: TextStyle(
                      fontSize: 14.sp,
                      color: AppColors.textSecondary,
                    ),
                  ),
                ),
              Expanded(
                child: loading.value
                    ? const TruckLoadingOverlay()
                    : (comments.value.isEmpty
                          ? Center(
                              child: Text(
                                '還沒有留言，來當第一個吧！',
                                style: TextStyle(
                                  fontSize: 14.sp,
                                  color: AppColors.textPrimary,
                                ),
                              ),
                            )
                          : ListView.builder(
                              controller: scroll,
                              padding: EdgeInsets.only(bottom: 8.h),
                              itemCount: comments.value.length,
                              itemBuilder: (context, i) {
                                final c = comments.value[i];
                                final id = c.id;

                                // Lazy-load replies
                                if (c.replyCount > 0 &&
                                    !repliesLoadedRef.value.contains(id) &&
                                    !repliesFailedRef.value.contains(id) &&
                                    !repliesLoadingRef.value.contains(id)) {
                                  repliesLoadingRef.value.add(id);
                                  Future.microtask(() async {
                                    try {
                                      final items = await postRepository
                                          .listComments(postId, parentId: id);
                                      if (!context.mounted) return;
                                      final created =
                                          repliesByParentRef.value[id] ??
                                          const <PostComment>[];
                                      final known = {
                                        for (final r in items) r.id,
                                      };
                                      repliesByParentRef.value[id] = [
                                        ...items,
                                        ...created.where(
                                          (r) => !known.contains(r.id),
                                        ),
                                      ];
                                      repliesLoadedRef.value.add(id);
                                    } catch (_) {
                                      if (!context.mounted) return;
                                      repliesFailedRef.value.add(id);
                                      FloatingToast.error(context, '回覆載入失敗');
                                    } finally {
                                      repliesLoadingRef.value.remove(id);
                                    }
                                    if (!context.mounted) return;
                                    comments.value = List.of(comments.value);
                                  });
                                }

                                final replies =
                                    repliesByParentRef.value[id] ??
                                    const <PostComment>[];

                                return Column(
                                  children: [
                                    // Top-level comment
                                    if (c.placeholderText case final text?)
                                      _CommentPlaceholder(
                                        commentId: id,
                                        text: text,
                                      )
                                    else
                                      _CommentTile(
                                        comment: c,
                                        displayName: resolveDisplayName(c),
                                        avatar: _resolveCommentAvatar(
                                          c,
                                          sellerAvatarBase64:
                                              sellerAvatarBase64,
                                        ),
                                        onReply: c.viewerCanReply
                                            ? () => onTapReply(
                                                resolveDisplayName(c),
                                                id,
                                              )
                                            : null,
                                        onLongPress: c.viewerCanDelete
                                            ? () => confirmAndDeleteComment(c)
                                            : null,
                                        onLike: () => toggleLike(c),
                                      ),
                                    // Replies (indented)
                                    ...replies.map(
                                      (r) => _ReplyTile(
                                        reply: r,
                                        displayName: resolveDisplayName(r),
                                        avatar: _resolveCommentAvatar(
                                          r,
                                          sellerAvatarBase64:
                                              sellerAvatarBase64,
                                        ),
                                        onLongPress: r.viewerCanDelete
                                            ? () => confirmAndDeleteComment(r)
                                            : null,
                                        onLike: () => toggleLike(r),
                                      ),
                                    ),
                                    // Divider after each comment group
                                    if (i < comments.value.length - 1)
                                      Divider(
                                        height: 1,
                                        thickness: 1,
                                        color: AppColors.gray200,
                                        indent: 16.w,
                                        endIndent: 16.w,
                                      ),
                                  ],
                                );
                              },
                            )),
              ),

              // ── Input bar ──
              if (canComment)
                SafeArea(
                  top: false,
                  child: Padding(
                    padding: EdgeInsets.symmetric(
                      horizontal: 32.w,
                      vertical: 12.h,
                    ),
                    child: TextFieldTapRegion(
                      child: Column(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          if (replyToCommentId.value != null &&
                              replyToName.value != null) ...[
                            Row(
                              children: [
                                Expanded(
                                  child: Text(
                                    '正在回覆 @${replyToName.value}',
                                    maxLines: 1,
                                    overflow: TextOverflow.ellipsis,
                                    style: TextStyle(
                                      fontSize: 14.sp,
                                      fontWeight: FontWeight.w700,
                                      color: AppColors.primary,
                                    ),
                                  ),
                                ),
                                HkTapTarget(
                                  key: const ValueKey('cancel-comment-reply'),
                                  semanticLabel: '取消回覆',
                                  hint: '返回新增留言模式',
                                  tooltip: '取消回覆',
                                  onTap: sending.value ? null : cancelReply,
                                  child: Icon(
                                    AppIcons.close,
                                    size: 16.sp,
                                    color: AppIconColors.active,
                                  ),
                                ),
                              ],
                            ),
                            SizedBox(height: 4.h),
                          ],
                          Row(
                            children: [
                              // Text field — pill shape
                              Expanded(
                                child: Container(
                                  // The decorated border consumes 1dp on each edge.
                                  // Keep the editable semantics node itself at 48dp.
                                  height: 50,
                                  decoration: BoxDecoration(
                                    color: AppColors.bgCard,
                                    border: Border.all(
                                      color: const Color(
                                        0xFF42493F,
                                      ).withValues(alpha: 0.5),
                                    ),
                                    borderRadius: BorderRadius.circular(999),
                                  ),
                                  child: TextField(
                                    controller: ctrl,
                                    focusNode: focus,
                                    readOnly: sending.value,
                                    textInputAction: TextInputAction.send,
                                    onSubmitted: (_) => send(),
                                    style: TextStyle(
                                      fontSize: 14.sp,
                                      color: AppColors.textPrimary,
                                    ),
                                    decoration: InputDecoration(
                                      hintText: replyToCommentId.value != null
                                          ? '回覆留言'
                                          : '新增貼文留言',
                                      hintStyle: TextStyle(
                                        fontSize: 14.sp,
                                        color: const Color(
                                          0xFF42493F,
                                        ).withValues(alpha: 0.5),
                                      ),
                                      isDense: true,
                                      filled: false,
                                      border: InputBorder.none,
                                      enabledBorder: InputBorder.none,
                                      focusedBorder: InputBorder.none,
                                      disabledBorder: InputBorder.none,
                                      errorBorder: InputBorder.none,
                                      focusedErrorBorder: InputBorder.none,
                                      contentPadding: EdgeInsets.symmetric(
                                        horizontal: 12.w,
                                        vertical: 12.h,
                                      ),
                                    ),
                                  ),
                                ),
                              ),
                              SizedBox(width: 12.w),
                              // Send button
                              HkFrostedIconButton(
                                key: const ValueKey('send-comment-action'),
                                label: '送出留言',
                                semanticHint: hasText.value
                                    ? '送出目前輸入的留言'
                                    : '請先輸入留言',
                                icon: AppIcons.send,
                                onPressed: hasText.value ? send : null,
                                loading: sending.value,
                                loadingLabel: '正在送出留言',
                              ),
                            ],
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
    );
  }
}

/// Single top-level comment tile matching Figma design.
/// Layout: [Avatar 30x30] [Name / Content / Reply button] [Heart + Count]
class _CommentTile extends StatelessWidget {
  const _CommentTile({
    required this.comment,
    required this.displayName,
    required this.avatar,
    required this.onReply,
    this.onLongPress,
    this.onLike,
  });
  final PostComment comment;
  final String displayName;
  final ImageProvider? avatar;

  /// null 時（`viewer_can_reply` 是 false）不顯示回覆鈕。
  final VoidCallback? onReply;
  final VoidCallback? onLongPress;
  final VoidCallback? onLike;

  @override
  Widget build(BuildContext context) {
    final text = comment.body;
    final likeCount = comment.likeCount;
    final isLiked = comment.viewerLiked;

    return GestureDetector(
      onLongPress: onLongPress,
      behavior: HitTestBehavior.opaque,
      child: Padding(
        padding: EdgeInsets.symmetric(horizontal: 16.w, vertical: 4.h),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Padding(
              padding: EdgeInsets.only(top: 4.h),
              child: _buildAvatar(avatar, 30),
            ),
            SizedBox(width: 12.w),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    displayName,
                    style: TextStyle(
                      fontSize: 16.sp,
                      fontWeight: FontWeight.w400,
                      color: AppColors.textEmphasis,
                    ),
                  ),
                  SizedBox(height: 4.h),
                  Text(
                    text,
                    style: TextStyle(
                      fontSize: 14.sp,
                      color: AppColors.textPrimary,
                    ),
                  ),
                  if (onReply != null) ...[
                    SizedBox(height: 4.h),
                    HkTapTarget(
                      key: ValueKey('comment-reply-${comment.id}'),
                      semanticLabel: '回覆 $displayName 的留言',
                      hint: '在輸入框撰寫回覆',
                      tooltip: '回覆',
                      onTap: onReply,
                      child: Row(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          ChatBubbleIcon(
                            size: 24.sp,
                            color: AppIconColors.defaultColor,
                          ),
                          SizedBox(width: 6.w),
                          Text(
                            '回覆',
                            style: TextStyle(
                              fontSize: 14.sp,
                              color: AppColors.textPrimary,
                            ),
                          ),
                        ],
                      ),
                    ),
                  ],
                ],
              ),
            ),
            // Heart + count — outline when not liked, filled when liked
            HkTapTarget(
              key: ValueKey('comment-like-${comment.id}'),
              semanticLabel: isLiked
                  ? '取消喜歡 $displayName 的留言'
                  : '喜歡 $displayName 的留言',
              hint: isLiked ? '移除這個留言的喜歡' : '喜歡這個留言',
              tooltip: isLiked ? '取消喜歡' : '喜歡',
              toggled: isLiked,
              onTap: onLike,
              child: Column(
                children: [
                  Icon(
                    isLiked ? AppIcons.favorite : AppIcons.favoriteBorder,
                    size: 24.sp,
                    color: isLiked
                        ? AppIconColors.favorite
                        : AppIconColors.defaultColor,
                  ),
                  SizedBox(height: 2.h),
                  Text(
                    '$likeCount',
                    style: TextStyle(
                      fontSize: 14.sp,
                      color: AppColors.textPrimary,
                    ),
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}

/// Reply tile — card with border, indented, compact.
class _ReplyTile extends StatelessWidget {
  const _ReplyTile({
    required this.reply,
    required this.displayName,
    required this.avatar,
    this.onLongPress,
    this.onLike,
  });
  final PostComment reply;
  final String displayName;
  final ImageProvider? avatar;
  final VoidCallback? onLongPress;
  final VoidCallback? onLike;

  @override
  Widget build(BuildContext context) {
    final text = reply.body;
    final likeCount = reply.likeCount;
    final isLiked = reply.viewerLiked;

    return GestureDetector(
      onLongPress: onLongPress,
      behavior: HitTestBehavior.opaque,
      child: Container(
        margin: EdgeInsets.symmetric(horizontal: 16.w, vertical: 4.h),
        padding: EdgeInsets.symmetric(horizontal: 16.w, vertical: 4.h),
        decoration: BoxDecoration(
          color: AppColors.bgPrimary,
          borderRadius: BorderRadius.circular(16.r),
          border: Border.all(
            color: AppColors.textPrimary.withValues(alpha: 0.2),
          ),
        ),
        child: Row(
          children: [
            // Left indent (42px) + avatar + content
            SizedBox(width: 42.w),
            Padding(
              padding: EdgeInsets.only(top: 4.h),
              child: _buildAvatar(avatar, 30),
            ),
            SizedBox(width: 12.w),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    displayName,
                    style: TextStyle(
                      fontSize: 16.sp,
                      fontWeight: FontWeight.w400,
                      color: AppColors.textEmphasis,
                    ),
                  ),
                  SizedBox(height: 4.h),
                  Text(
                    text,
                    style: TextStyle(
                      fontSize: 14.sp,
                      color: AppColors.textPrimary,
                    ),
                  ),
                ],
              ),
            ),
            // Heart + count
            HkTapTarget(
              key: ValueKey('reply-like-${reply.id}'),
              semanticLabel: isLiked
                  ? '取消喜歡 $displayName 的回覆'
                  : '喜歡 $displayName 的回覆',
              hint: isLiked ? '移除這個回覆的喜歡' : '喜歡這個回覆',
              tooltip: isLiked ? '取消喜歡' : '喜歡',
              toggled: isLiked,
              onTap: onLike,
              child: Column(
                children: [
                  Icon(
                    isLiked ? AppIcons.favorite : AppIcons.favoriteBorder,
                    size: 24.sp,
                    color: isLiked
                        ? AppIconColors.favorite
                        : AppIconColors.defaultColor,
                  ),
                  SizedBox(height: 2.h),
                  Text(
                    '$likeCount',
                    style: TextStyle(
                      fontSize: 14.sp,
                      color: AppColors.textPrimary,
                    ),
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}

/// 被刪除或被隱藏、只因底下還有可見回覆而留著的頂層留言。
class _CommentPlaceholder extends StatelessWidget {
  const _CommentPlaceholder({required this.commentId, required this.text});
  final String commentId;
  final String text;

  @override
  Widget build(BuildContext context) => Padding(
    key: ValueKey('comment-placeholder-$commentId'),
    padding: EdgeInsets.symmetric(horizontal: 16.w, vertical: 8.h),
    child: Text(
      text,
      style: TextStyle(fontSize: 14.sp, color: AppColors.textSecondary),
    ),
  );
}

Widget _buildAvatar(ImageProvider? avatar, double size) {
  return Container(
    width: size.w,
    height: size.w,
    decoration: const BoxDecoration(
      shape: BoxShape.circle,
      color: AppColors.gray100,
    ),
    child: ClipOval(
      child: avatar != null
          ? Image(
              image: avatar,
              fit: BoxFit.cover,
              width: size.w,
              height: size.w,
              gaplessPlayback: true,
              errorBuilder: (_, _, _) => Icon(
                AppIcons.personOutline,
                size: (size * 0.6).sp,
                color: AppIconColors.onBrand,
              ),
            )
          : Icon(
              AppIcons.personOutline,
              size: (size * 0.6).sp,
              color: AppIconColors.onBrand,
            ),
    ),
  );
}
