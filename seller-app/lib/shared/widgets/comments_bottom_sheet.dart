import '../../features/seller/data/current_store.dart';
import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'dart:convert';
import 'package:flutter/material.dart';
import 'package:hawkr_seller/shared/widgets/hk_icon.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import '../../config/media_image.dart';
import '../../features/post/data/post_repository.dart';
import '../../features/post/presentation/widgets/storefront_tab_glass.dart';
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
    this.postOwnerName,
    this.sellerAvatarBase64,
    this.sellerAvatarUrl,
    this.onCommentCountChanged,
    this.unavailableMessage,
  });
  final int postId;
  final PostRepository postRepository;
  final String? postOwnerName;
  final String? sellerAvatarBase64;
  final String? sellerAvatarUrl;
  final ValueChanged<int>? onCommentCountChanged;

  /// 非 null 時這則貼文買家看不到，公開的留言讀不到它：不載入留言、不顯示輸入列，
  /// 列表區改顯示這段說明，外框（拖曳把手、關閉鍵）不變。
  final String? unavailableMessage;

  static ImageProvider? _resolveCommentAvatar(
    Map<String, dynamic> c, {
    required String? myHeaderUrl,
    required String? sellerAvatarBase64,
  }) {
    // 契約成對回 `avatarKey` + `avatarUrl`（gateway post.go `CommentItem`，
    // 讀取與寫入端點同形狀），SSOT 是 key。舊欄位 `userAvatarKey` /
    // `userAvatarUrl` / `user_avatar_key` / `userAvatar` 皆已不存在。
    final String? avSource = (c['avatarKey'] ?? c['avatarUrl'])?.toString();
    if (avSource != null && avSource.isNotEmpty) {
      final String u = buildImageUrl(avSource, ImagePreset.avatar);
      if (u.startsWith('http')) return NetworkImage(u);
    }
    // 是不是自己所發由伺服器的 isMine 決定，藉此決定要不要顯示賣家自己的頭像。
    final bool isSelf = c['isMine'] == true;
    if (isSelf &&
        myHeaderUrl != null &&
        myHeaderUrl.isNotEmpty &&
        myHeaderUrl.startsWith('http')) {
      return NetworkImage(myHeaderUrl);
    }
    if (isSelf && sellerAvatarBase64 != null && sellerAvatarBase64.isNotEmpty) {
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

    final comments = useState<List<Map<String, dynamic>>>(
      <Map<String, dynamic>>[],
    );
    final loading = useState(true);
    final sending = useState(false);
    final myHeaderUrl = useState<String?>(null);
    final replyToCommentId = useState<int?>(null);
    final replyToName = useState<String?>(null);

    final repliesByParentRef = useRef(<int, List<Map<String, dynamic>>>{});
    final repliesLoadingRef = useRef(<int>{});

    // Listen to text changes for send button opacity
    useEffect(() {
      void listener() {
        hasText.value = ctrl.text.trim().isNotEmpty;
      }

      ctrl.addListener(listener);
      return () => ctrl.removeListener(listener);
    }, [ctrl]);

    void notifyCommentCount() {
      if (!context.mounted) return;
      final cb = onCommentCountChanged;
      if (cb == null) return;
      int total = comments.value.length;
      for (final list in repliesByParentRef.value.values) {
        total += list.length;
      }
      cb(total);
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
        final list = await postRepository.listCommentsRaw(postId);
        if (!context.mounted) return;
        repliesByParentRef.value.clear();
        final List<Map<String, dynamic>> top = [];
        for (final c in list) {
          final rawPid = c['parentId'];
          final int? pid = (rawPid is num)
              ? rawPid.toInt()
              : int.tryParse('${rawPid ?? ''}');
          if (pid != null && pid > 0) {
            final l = repliesByParentRef.value[pid] ?? <Map<String, dynamic>>[];
            l.add(c);
            repliesByParentRef.value[pid] = l;
          } else {
            top.add(c);
          }
        }
        comments.value = top;
        notifyCommentCount();
        scheduleScrollToBottom(animated: false);
      } finally {
        if (context.mounted) loading.value = false;
      }
    }

    useEffect(() {
      if (unavailableMessage == null) load();
      return null;
    }, const []);

    Future<void> send() async {
      final text = ctrl.text.trim();
      if (text.isEmpty || sending.value) return;
      final parentId = replyToCommentId.value;
      sending.value = true;
      try {
        if (parentId != null) {
          final created = await postRepository.replyToComment(
            parentId,
            postId: postId,
            content: text,
          );
          if (!context.mounted) return;
          if (created != null) {
            created['isMine'] = true;
            final meName =
                (postOwnerName != null && postOwnerName!.trim().isNotEmpty)
                ? postOwnerName!.trim()
                : (ref.read(myStoreSnapshotProvider)?.storeName ?? '我');
            created['userName'] = created['userName'] ?? meName;
            ctrl.clear();
            final pid = parentId;
            final list =
                repliesByParentRef.value[pid] ?? <Map<String, dynamic>>[];
            repliesByParentRef.value[pid] = [...list, created];
            replyToCommentId.value = null;
            replyToName.value = null;
            comments.value = List.from(comments.value);
            notifyCommentCount();
            scheduleScrollToBottom();
          } else {
            FloatingToast.error(context, '回覆失敗');
          }
        } else {
          final created = await postRepository.commentOnPostRaw(
            postId,
            content: text,
          );
          if (!context.mounted) return;
          if (created != null) {
            ctrl.clear();
            created['isMine'] = true;
            final meName =
                (postOwnerName != null && postOwnerName!.trim().isNotEmpty)
                ? postOwnerName!.trim()
                : (ref.read(myStoreSnapshotProvider)?.storeName ?? '我');
            created['userName'] = created['userName'] ?? meName;
            comments.value = [...comments.value, created];
            notifyCommentCount();
            scheduleScrollToBottom();
          } else {
            FloatingToast.error(context, '留言失敗');
          }
        }
      } catch (_) {
        if (context.mounted) {
          FloatingToast.error(context, parentId != null ? '回覆失敗' : '留言失敗');
        }
      } finally {
        if (context.mounted) sending.value = false;
      }
    }

    void onTapReply(String name, int? parentId) {
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

    Future<bool> deleteComment(int commentId) async {
      final ok = await postRepository.deleteComment(commentId);
      if (!context.mounted) return false;
      if (ok) {
        final updatedComments = List<Map<String, dynamic>>.from(comments.value);
        updatedComments.removeWhere((c) {
          final raw = c['id'];
          final id = raw is int ? raw : int.tryParse('${raw ?? ''}');
          return id == commentId;
        });
        repliesByParentRef.value.updateAll((pid, list) {
          list.removeWhere((r) {
            final raw = r['id'];
            final id = raw is int ? raw : int.tryParse('${raw ?? ''}');
            return id == commentId;
          });
          return list;
        });
        repliesByParentRef.value.removeWhere((_, list) => list.isEmpty);
        comments.value = updatedComments;
        notifyCommentCount();
        await load();
        if (!context.mounted) return true;
        try {
          FloatingToast.success(context, '已刪除留言');
        } catch (_) {}
        return true;
      } else {
        try {
          FloatingToast.error(context, '刪除留言失敗');
        } catch (_) {}
        return false;
      }
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

    Future<void> confirmAndDeleteComment(int commentId) async {
      final confirmed = await showDeleteConfirmationDialog();
      if (!confirmed) return;
      await deleteComment(commentId);
    }

    String resolveDisplayName(Map<String, dynamic> c) {
      final origName = (c['userName'] ?? '').toString();
      if (origName.isNotEmpty) return origName;
      return '用戶';
    }

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
              Expanded(
                child: unavailableMessage != null
                    ? Center(
                        child: SingleChildScrollView(
                          child: StorefrontGlassEmptyPanel(
                            icon: HkIconName.chat,
                            message: unavailableMessage!,
                          ),
                        ),
                      )
                    : loading.value
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
                                final rawId = c['id'];
                                final id = rawId is int
                                    ? rawId
                                    : int.tryParse('${rawId ?? ''}');

                                // Lazy-load replies
                                if (id != null &&
                                    !repliesByParentRef.value.containsKey(id) &&
                                    !repliesLoadingRef.value.contains(id)) {
                                  repliesLoadingRef.value.add(id);
                                  Future.microtask(() async {
                                    final items = await postRepository
                                        .listCommentReplies(id);
                                    if (!context.mounted) return;
                                    final existing =
                                        repliesByParentRef.value[id] ??
                                        const <Map<String, dynamic>>[];
                                    final Map<int, Map<String, dynamic>> byId =
                                        {};
                                    for (final r in [...existing, ...items]) {
                                      final rawRid = r['id'];
                                      final rid = rawRid is int
                                          ? rawRid
                                          : int.tryParse('${rawRid ?? ''}');
                                      if (rid == null) continue;
                                      byId[rid] = r;
                                    }
                                    repliesByParentRef.value[id] = byId.values
                                        .toList();
                                    repliesLoadingRef.value.remove(id);
                                    comments.value = List.from(comments.value);
                                  });
                                }

                                final replies = (id != null)
                                    ? (repliesByParentRef.value[id] ??
                                          const <Map<String, dynamic>>[])
                                    : const <Map<String, dynamic>>[];

                                return Column(
                                  children: [
                                    // Top-level comment
                                    _CommentTile(
                                      comment: c,
                                      displayName: resolveDisplayName(c),
                                      avatar: _resolveCommentAvatar(
                                        c,
                                        myHeaderUrl: myHeaderUrl.value,
                                        sellerAvatarBase64: sellerAvatarBase64,
                                      ),
                                      onReply: () =>
                                          onTapReply(resolveDisplayName(c), id),
                                      onLongPress: id != null
                                          ? () => confirmAndDeleteComment(id)
                                          : null,
                                      onLike: id != null
                                          ? () async {
                                              final liked = await postRepository
                                                  .likeComment(id);
                                              if (!context.mounted) return;
                                              c['liked'] = liked;
                                              c['likeCount'] =
                                                  (c['likeCount'] ?? 0) +
                                                  (liked ? 1 : -1);
                                              comments.value = List.from(
                                                comments.value,
                                              );
                                            }
                                          : null,
                                    ),
                                    // Replies (indented)
                                    ...replies.map((r) {
                                      final rid = _parseId(r);
                                      return _ReplyTile(
                                        reply: r,
                                        displayName: resolveDisplayName(r),
                                        avatar: _resolveCommentAvatar(
                                          r,
                                          myHeaderUrl: myHeaderUrl.value,
                                          sellerAvatarBase64:
                                              sellerAvatarBase64,
                                        ),
                                        onLongPress: rid != null
                                            ? () => confirmAndDeleteComment(rid)
                                            : null,
                                        onLike: rid != null
                                            ? () async {
                                                final liked =
                                                    await postRepository
                                                        .likeComment(rid);
                                                if (!context.mounted) return;
                                                r['liked'] = liked;
                                                r['likeCount'] =
                                                    (r['likeCount'] ?? 0) +
                                                    (liked ? 1 : -1);
                                                comments.value = List.from(
                                                  comments.value,
                                                );
                                              }
                                            : null,
                                      );
                                    }),
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
              if (unavailableMessage == null)
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

  static int? _parseId(Map<String, dynamic> m) {
    final raw = m['id'];
    return raw is int ? raw : int.tryParse('${raw ?? ''}');
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
  final Map<String, dynamic> comment;
  final String displayName;
  final ImageProvider? avatar;
  final VoidCallback onReply;
  final VoidCallback? onLongPress;
  final VoidCallback? onLike;

  @override
  Widget build(BuildContext context) {
    final text = (comment['content'] ?? '').toString();
    // `likeCount` / `isLiked` 是 wire 欄位；`liked` 是按讚後本地樂觀覆蓋的旗標。
    final likeCount = (comment['likeCount'] ?? 0);
    final isLiked = comment['liked'] == true || comment['isLiked'] == true;

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
                  SizedBox(height: 4.h),
                  HkTapTarget(
                    key: ValueKey('comment-reply-${comment['id']}'),
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
              ),
            ),
            // Heart + count — outline when not liked, filled when liked
            HkTapTarget(
              key: ValueKey('comment-like-${comment['id']}'),
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
  final Map<String, dynamic> reply;
  final String displayName;
  final ImageProvider? avatar;
  final VoidCallback? onLongPress;
  final VoidCallback? onLike;

  @override
  Widget build(BuildContext context) {
    final text = (reply['content'] ?? '').toString();
    // 同 _CommentTile：`liked` 為本地樂觀旗標，其餘為 wire 欄位。
    final likeCount = (reply['likeCount'] ?? 0);
    final isLiked = reply['liked'] == true || reply['isLiked'] == true;
    final parentName = reply['replyToName'] ?? '';

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
                    parentName.toString().isNotEmpty ? '@$parentName' : text,
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
              key: ValueKey('reply-like-${reply['id']}'),
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
              errorBuilder: (_, __, ___) => Icon(
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
