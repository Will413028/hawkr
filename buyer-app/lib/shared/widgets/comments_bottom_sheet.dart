import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/app_typography.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.pb.dart' as pb;
import 'package:hawkr_api/images.dart' show ImagePreset, ImageUri;
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:uuid/uuid.dart';

import '../../core/networking/suspension_signal.dart';
import '../../core/session/session_provider.dart';
import '../../features/social/data/social_content_repository.dart';
import '../utils/require_login.dart';
import 'chat_bubble_icon.dart';
import 'login_to_comment_prompt.dart';
import 'fruit_refresh_indicator.dart';
import 'hk_button_label.dart';
import 'hk_empty_state.dart';
import 'hk_tap_target.dart';
import 'hk_frosted_icon_button.dart';
import 'common_snackbar.dart';
import 'truck_loading_overlay.dart';

const _deletedCommentText = '此留言已刪除';
const _hiddenCommentText = '此留言已被隱藏';
const _anonymousName = '匿名用戶';

/// 貼文的留言。打開與下拉時讀取，不即時推送。留言與回覆直接用生成的 [pb.Comment]；能不能留言、回覆、刪除
/// 只讀伺服器的 `viewer_can_*`，留言身分讀 `Post.viewer_actor`。
class CommentsBottomSheet extends ConsumerStatefulWidget {
  const CommentsBottomSheet({super.key, required this.postId});

  final String postId;

  @override
  ConsumerState<CommentsBottomSheet> createState() =>
      _CommentsBottomSheetState();
}

class _CommentsBottomSheetState extends ConsumerState<CommentsBottomSheet> {
  final TextEditingController _controller = TextEditingController();
  final FocusNode _focusNode = FocusNode();
  final ScrollController _scrollController = ScrollController();
  static const Uuid _uuid = Uuid();

  final Map<String, List<pb.Comment>> _repliesByParent =
      <String, List<pb.Comment>>{};
  final Set<String> _repliesLoading = <String>{};

  final Map<String, int> _commentLikeCounts = <String, int>{};
  final Set<String> _likedCommentIds = <String>{};

  List<pb.Comment> _comments = <pb.Comment>[];
  String _nextPageToken = '';
  bool _loading = true;
  bool _sending = false;

  /// 觀看者在這篇貼文的留言權限與身分（`GetPost`）；讀不到時不能留言。
  bool _viewerCanComment = false;
  pb.Actor? _viewerActor;

  /// Reddit-style thread 的根留言 ID，新回覆會掛在這個 thread 下。
  String? _replyToCommentId;

  /// 實際被回覆的留言 ID，可能是根留言或某則回覆。
  String? _replyToTargetId;
  String? _replyToName;

  @override
  void initState() {
    super.initState();
    _controller.addListener(() {
      if (mounted) setState(() {});
    });
    _loadComments();
  }

  @override
  void dispose() {
    _controller.dispose();
    _focusNode.dispose();
    _scrollController.dispose();
    super.dispose();
  }

  SocialContentRepository get _repo =>
      ref.read(socialContentRepositoryProvider);

  void _remember(pb.Comment comment) {
    _commentLikeCounts[comment.id] = comment.likeCount.toInt();
    if (comment.viewerLiked) {
      _likedCommentIds.add(comment.id);
    } else {
      _likedCommentIds.remove(comment.id);
    }
  }

  Future<void> _loadViewer() async {
    try {
      final post = await ref
          .read(socialContentRepositoryProvider)
          .getPost(int.parse(widget.postId));
      _viewerCanComment = post.viewerCanComment;
      _viewerActor = post.viewerActor;
    } catch (_) {
      // 貼文不公開時留言仍讀得到，只是不能留言。
      _viewerCanComment = false;
      _viewerActor = null;
    }
  }

  Future<void> _loadComments() async {
    setState(() => _loading = true);
    try {
      final results = await Future.wait([
        _loadViewer(),
        _repo.listComments(widget.postId),
      ]);
      final response = results[1] as pb.ListCommentsResponse;
      if (!mounted) return;
      setState(() {
        _comments = response.comments.toList();
        _nextPageToken = response.nextPageToken;
        _repliesByParent.clear();
        _commentLikeCounts.clear();
        _likedCommentIds.clear();
        _comments.forEach(_remember);
      });
    } catch (_) {
      if (!mounted) return;
      setState(() => _comments = <pb.Comment>[]);
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  Future<void> _loadMoreComments() async {
    if (_nextPageToken.isEmpty) return;
    try {
      final response = await _repo.listComments(
        widget.postId,
        pageToken: _nextPageToken,
      );
      if (!mounted) return;
      setState(() {
        _comments = [..._comments, ...response.comments];
        _nextPageToken = response.nextPageToken;
        response.comments.forEach(_remember);
      });
    } catch (_) {
      if (mounted) showCommonSnackBar(context, '載入失敗，請稍後再試');
    }
  }

  Future<void> _ensureReplies(String parentId) async {
    if (_repliesLoading.contains(parentId)) return;
    _repliesLoading.add(parentId);
    try {
      final all = <pb.Comment>[];
      var token = '';
      do {
        final response = await _repo.listComments(
          widget.postId,
          parentId: parentId,
          pageToken: token,
        );
        all.addAll(response.comments);
        token = response.nextPageToken;
      } while (token.isNotEmpty);
      if (!mounted) return;
      setState(() {
        _repliesByParent[parentId] = all;
        all.forEach(_remember);
      });
    } catch (_) {
      if (mounted) showCommonSnackBar(context, '載入失敗，請稍後再試');
    } finally {
      _repliesLoading.remove(parentId);
    }
  }

  void _reportFailure(Object error, String message) {
    if (!mounted) return;
    // 停權由 transport 的 interceptor 提示，這裡不再疊一則失敗訊息。
    if (isAccountSuspendedError(error)) return;
    showCommonSnackBar(context, message);
  }

  Future<void> _send() async {
    final text = _controller.text.trim();
    if (_sending || text.isEmpty) return;
    if (!await requireLogin(context, ref)) return;
    if (!mounted || _sending) return;
    setState(() => _sending = true);
    final replyCommentId = _replyToCommentId;
    final replyTargetId = _replyToTargetId;
    try {
      final isReply = replyCommentId != null;
      final created = await _repo.createComment(
        widget.postId,
        text,
        parentId: replyCommentId,
        replyToId: isReply ? (replyTargetId ?? replyCommentId) : null,
        requestId: _uuid.v4(),
      );
      if (!mounted) return;
      _controller.clear();
      setState(() {
        _remember(created);
        if (isReply) {
          _repliesByParent[replyCommentId] = [
            ...?_repliesByParent[replyCommentId],
            created,
          ];
          _replyToCommentId = null;
          _replyToTargetId = null;
          _replyToName = null;
        } else {
          _comments = [..._comments, created];
        }
      });
      _scheduleScrollToBottom();
    } catch (e) {
      _reportFailure(e, '傳送失敗，請稍後再試');
    } finally {
      if (mounted) setState(() => _sending = false);
    }
  }

  void _beginReply({
    required String rootCommentId,
    required String targetCommentId,
    required String targetName,
  }) {
    if (_sending) return;
    setState(() {
      _replyToCommentId = rootCommentId;
      _replyToTargetId = targetCommentId;
      _replyToName = targetName;
    });
    _focusNode.requestFocus();
  }

  void _cancelReply() {
    if (_sending) return;
    setState(() {
      _replyToCommentId = null;
      _replyToTargetId = null;
      _replyToName = null;
    });
  }

  Future<void> _delete(pb.Comment comment) async {
    final confirm = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('刪除留言'),
        content: const Text('確定要刪除這則留言嗎？'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx, false),
            child: const HkButtonLabel('取消'),
          ),
          TextButton(
            onPressed: () => Navigator.pop(ctx, true),
            child: const HkButtonLabel(
              '刪除',
              style: TextStyle(color: AppColors.danger),
            ),
          ),
        ],
      ),
    );
    if (confirm != true || !mounted) return;
    try {
      await _repo.deleteComment(comment);
      if (!mounted) return;
      await _loadComments();
    } catch (e) {
      _reportFailure(e, '刪除失敗，請稍後再試');
    }
  }

  void _scheduleScrollToBottom() {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!_scrollController.hasClients) return;
      _scrollController.animateTo(
        _scrollController.position.maxScrollExtent,
        duration: const Duration(milliseconds: 250),
        curve: Curves.easeOut,
      );
    });
  }

  Future<void> _toggleLike(pb.Comment comment) async {
    if (!await requireLogin(context, ref)) return;
    if (!mounted) return;
    final id = comment.id;
    final bool wasLiked = _likedCommentIds.contains(id);
    final int currentCount = _commentLikeCounts[id] ?? 0;
    setState(() {
      if (wasLiked) {
        _likedCommentIds.remove(id);
        _commentLikeCounts[id] = currentCount > 0 ? currentCount - 1 : 0;
      } else {
        _likedCommentIds.add(id);
        _commentLikeCounts[id] = currentCount + 1;
      }
    });
    try {
      await _repo.setCommentLiked(id, liked: !wasLiked);
    } catch (e) {
      if (!mounted) return;
      setState(() {
        if (wasLiked) {
          _likedCommentIds.add(id);
        } else {
          _likedCommentIds.remove(id);
        }
        _commentLikeCounts[id] = currentCount;
      });
    }
  }

  int get _visibleCommentCount =>
      _comments.length +
      _repliesByParent.values.fold<int>(
        0,
        (sum, replies) => sum + replies.length,
      );

  /// 以店家身分留言時，輸入框提示帶店名（C2、C3）；其他身分用一般提示。
  String get _hintText {
    final actor = _viewerActor;
    final storeName =
        actor != null &&
            actor.kind == pb.ActorKind.ACTOR_KIND_STORE &&
            actor.hasName() &&
            actor.name.trim().isNotEmpty
        ? actor.name.trim()
        : null;
    final replying = _replyToName != null;
    if (storeName != null) {
      return replying ? '以「$storeName」身分回覆…' : '以「$storeName」身分留言…';
    }
    return replying ? '寫下回覆…' : '參與討論…';
  }

  @override
  Widget build(BuildContext context) {
    final guest = isGuestSession(ref.watch(sessionProvider));
    return SafeArea(
      top: false,
      child: AnimatedPadding(
        duration: const Duration(milliseconds: 150),
        padding: EdgeInsets.only(
          bottom: MediaQuery.of(context).viewInsets.bottom,
        ),
        child: FractionallySizedBox(
          heightFactor: 0.88,
          child: Container(
            key: const ValueKey('comments-panel'),
            decoration: BoxDecoration(
              color: AppColors.bgCard,
              borderRadius: BorderRadius.vertical(
                top: Radius.circular(HkRadius.xxl),
              ),
              boxShadow: const [
                BoxShadow(
                  color: AppColors.cardShadow,
                  blurRadius: 24,
                  offset: Offset(0, -4),
                ),
              ],
            ),
            child: Column(
              children: [
                // Drag handle
                Container(
                  margin: EdgeInsets.only(top: 8.h, bottom: 8.h),
                  width: 40.w,
                  height: 4.h,
                  decoration: BoxDecoration(
                    color: AppColors.dragHandle,
                    borderRadius: BorderRadius.circular(HkRadius.pill),
                  ),
                ),
                Padding(
                  padding: EdgeInsets.fromLTRB(20.w, 4.h, 12.w, 12.h),
                  child: Row(
                    children: [
                      Text(
                        '留言',
                        style: const TextStyle(
                          fontSize: 24,
                          fontWeight: FontWeight.w700,
                          color: AppColors.textEmphasis,
                        ),
                      ),
                      SizedBox(width: 8.w),
                      Container(
                        padding: EdgeInsets.symmetric(
                          horizontal: 8.w,
                          vertical: 4.h,
                        ),
                        decoration: BoxDecoration(
                          color: AppColors.primarySoft,
                          borderRadius: BorderRadius.circular(HkRadius.pill),
                        ),
                        child: Text(
                          '$_visibleCommentCount',
                          style: const TextStyle(
                            fontSize: 12,
                            fontWeight: FontWeight.w700,
                            color: AppColors.textAccentOnTint,
                          ),
                        ),
                      ),
                      const Spacer(),
                      HkTapTarget(
                        key: const ValueKey('comments-panel-close'),
                        semanticLabel: '關閉留言',
                        hint: '返回貼文',
                        tooltip: '關閉',
                        onTap: () => Navigator.of(context).pop(),
                        child: Icon(
                          AppIcons.close,
                          size: 20.sp,
                          color: AppIconColors.strong,
                        ),
                      ),
                    ],
                  ),
                ),
                Divider(height: 1, color: AppColors.borderSubtle),
                // Comment list

                Expanded(
                  child: _loading
                      ? const TruckLoadingOverlay(message: '載入中...')
                      : FruitRefreshIndicator(
                          onRefresh: _loadComments,
                          child: _comments.isEmpty
                              ? ListView(
                                  controller: _scrollController,
                                  physics:
                                      const AlwaysScrollableScrollPhysics(),
                                  children: const [
                                    SizedBox(height: 48),
                                    HkEmptyState(
                                      key: ValueKey('comments-empty-state'),
                                      iconKey: ValueKey(
                                        'comments-empty-state-icon',
                                      ),
                                      icon: HkIconName.emptyChat,
                                      message: '還沒有留言，來當第一個吧',
                                    ),
                                  ],
                                )
                              : ListView.separated(
                                  controller: _scrollController,
                                  physics:
                                      const AlwaysScrollableScrollPhysics(),
                                  padding: EdgeInsets.fromLTRB(0, 8.h, 0, 16.h),
                                  itemBuilder: (context, index) {
                                    if (index == _comments.length) {
                                      return Center(
                                        child: HkTapTarget(
                                          key: const ValueKey(
                                            'comments-load-more',
                                          ),
                                          semanticLabel: '載入更多留言',
                                          hint: '載入下一頁留言',
                                          tooltip: '載入更多留言',
                                          onTap: _loadMoreComments,
                                          child: Text(
                                            '查看更多留言',
                                            style: TextStyle(
                                              fontSize: 14,
                                              color: AppColors.primary,
                                            ),
                                          ),
                                        ),
                                      );
                                    }
                                    return _buildComment(_comments[index]);
                                  },
                                  separatorBuilder: (_, __) =>
                                      SizedBox(height: 8.h),
                                  itemCount:
                                      _comments.length +
                                      (_nextPageToken.isEmpty ? 0 : 1),
                                ),
                        ),
                ),
                if (_viewerCanComment || guest)
                  Divider(height: 1, color: AppColors.borderSubtle),
                if (!_viewerCanComment && guest) const LoginToCommentPrompt(),
                // Input area
                if (_viewerCanComment)
                  Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      if (_replyToName != null)
                        Padding(
                          padding: EdgeInsets.fromLTRB(16.w, 8.h, 16.w, 0),
                          child: Container(
                            key: const ValueKey('comment-reply-context'),
                            padding: EdgeInsets.symmetric(
                              horizontal: 12.w,
                              vertical: 8.h,
                            ),
                            decoration: BoxDecoration(
                              color: AppColors.primarySubtle,
                              borderRadius: BorderRadius.circular(HkRadius.md),
                            ),
                            child: Row(
                              children: [
                                Expanded(
                                  child: Text(
                                    '回覆 @$_replyToName',
                                    style: const TextStyle(
                                      fontSize: 14,
                                      fontWeight: FontWeight.w700,
                                      color: AppColors.primary,
                                    ),
                                    maxLines: 1,
                                    overflow: TextOverflow.ellipsis,
                                  ),
                                ),
                                HkTapTarget(
                                  key: const ValueKey('cancel-comment-reply'),
                                  semanticLabel: '取消回覆',
                                  hint: '清除目前的回覆對象',
                                  tooltip: '取消回覆',
                                  onTap: _sending ? null : _cancelReply,
                                  child: Icon(
                                    AppIcons.close,
                                    size: 16.sp,
                                    color: AppIconColors.active,
                                  ),
                                ),
                              ],
                            ),
                          ),
                        ),
                      TextFieldTapRegion(
                        child: Padding(
                          padding: EdgeInsets.symmetric(
                            horizontal: 16.w,
                            vertical: 12.h,
                          ),
                          child: Row(
                            crossAxisAlignment: CrossAxisAlignment.end,
                            children: [
                              Expanded(
                                child: Container(
                                  constraints: const BoxConstraints(
                                    minHeight: HkControl.minTap,
                                  ),
                                  decoration: BoxDecoration(
                                    color: AppColors.bgPrimary,
                                    borderRadius: BorderRadius.circular(
                                      HkRadius.pill,
                                    ),
                                    border: Border.all(
                                      color: AppColors.borderInput,
                                    ),
                                  ),
                                  child: TextField(
                                    key: const ValueKey('comments-panel-input'),
                                    controller: _controller,
                                    focusNode: _focusNode,
                                    readOnly: _sending,
                                    minLines: 1,
                                    maxLines: 4,
                                    textInputAction: TextInputAction.send,
                                    onSubmitted: (_) => _send(),
                                    style: AppTypography.body.copyWith(
                                      color: AppColors.textPrimary,
                                    ),
                                    decoration: InputDecoration(
                                      hintText: _hintText,
                                      hintStyle: AppTypography.body.copyWith(
                                        color: AppColors.textHint,
                                      ),
                                      filled: false,
                                      fillColor: Colors.transparent,
                                      contentPadding: EdgeInsets.symmetric(
                                        horizontal: 16.w,
                                        vertical: 12.h,
                                      ),
                                      border: InputBorder.none,
                                      enabledBorder: InputBorder.none,
                                      focusedBorder: InputBorder.none,
                                      disabledBorder: InputBorder.none,
                                      errorBorder: InputBorder.none,
                                      focusedErrorBorder: InputBorder.none,
                                      isDense: true,
                                    ),
                                  ),
                                ),
                              ),
                              const SizedBox(width: HkSpace.sm),
                              HkFrostedIconButton(
                                key: const ValueKey('send-comment-action'),
                                label: _replyToName == null ? '送出留言' : '送出回覆',
                                semanticHint: _sending
                                    ? '留言傳送中'
                                    : _controller.text.trim().isNotEmpty
                                    ? '送出目前輸入的內容'
                                    : '請先輸入內容',
                                icon: AppIcons.sendRounded,
                                onPressed:
                                    !_sending &&
                                        _controller.text.trim().isNotEmpty
                                    ? _send
                                    : null,
                                loading: _sending,
                                loadingLabel: '留言傳送中',
                              ),
                            ],
                          ),
                        ),
                      ),
                    ],
                  ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  String _authorName(pb.Comment comment) {
    final author = comment.author;
    final name = comment.hasAuthor() && author.hasName()
        ? author.name.trim()
        : '';
    return name.isEmpty ? _anonymousName : name;
  }

  String? _avatarUri(pb.Comment comment) {
    if (!comment.hasAuthor() || !comment.author.hasAvatar()) return null;
    final uri = comment.author.avatar.uri(ImagePreset.avatar);
    return uri.isEmpty ? null : uri;
  }

  /// 已刪除或被隱藏的項目（只在底下有可見回覆時出現）只顯示佔位文字，沒有作者與動作。
  String? _placeholderText(pb.Comment comment) => switch (comment.state) {
    pb.CommentState.COMMENT_STATE_DELETED => _deletedCommentText,
    pb.CommentState.COMMENT_STATE_HIDDEN => _hiddenCommentText,
    _ => null,
  };

  Widget _buildComment(pb.Comment comment) {
    final String commentId = comment.id;
    final String? placeholder = _placeholderText(comment);
    final String author = _authorName(comment);
    final List<pb.Comment> replies =
        _repliesByParent[commentId] ?? const <pb.Comment>[];
    final int likeCount = _commentLikeCounts[commentId] ?? 0;
    final bool liked = _likedCommentIds.contains(commentId);
    final int replyCount = comment.replyCount.toInt();

    return Padding(
      padding: EdgeInsets.symmetric(horizontal: 16.w, vertical: 4.h),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              if (placeholder == null) ...[
                Padding(
                  padding: EdgeInsets.only(top: 4.h),
                  child: _avatarCircle(_avatarUri(comment), size: 30),
                ),
                SizedBox(width: 12.w),
              ],
              Expanded(
                child: placeholder != null
                    ? Text(
                        placeholder,
                        key: ValueKey('comment-placeholder-$commentId'),
                        style: TextStyle(
                          fontSize: 14,
                          color: AppColors.textSecondary,
                        ),
                      )
                    : Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            author,
                            style: TextStyle(
                              fontSize: 16,
                              fontWeight: FontWeight.w400,
                              color: AppColors.textEmphasis,
                            ),
                          ),
                          SizedBox(height: 4.h),
                          Text(
                            comment.body,
                            style: TextStyle(
                              fontSize: 14,
                              fontWeight: FontWeight.w400,
                              color: AppColors.textPrimary,
                            ),
                          ),
                          SizedBox(height: 4.h),
                          _buildActions(
                            comment,
                            author: author,
                            rootCommentId: commentId,
                          ),
                        ],
                      ),
              ),
              if (placeholder == null)
                _likeColumn(comment, author, liked, likeCount),
            ],
          ),
          if (replies.isNotEmpty)
            Padding(
              padding: EdgeInsets.only(left: 20.w, top: 4.h),
              child: Container(
                decoration: const BoxDecoration(
                  border: Border(
                    left: BorderSide(color: AppColors.borderSoft, width: 2),
                  ),
                ),
                padding: EdgeInsets.only(left: 12.w),
                child: Column(
                  children: replies
                      .map(
                        (reply) => _buildReply(reply, rootCommentId: commentId),
                      )
                      .toList(),
                ),
              ),
            ),
          if (replyCount > 0 && replies.length < replyCount)
            Padding(
              padding: EdgeInsets.only(left: 48.w),
              child: HkTapTarget(
                key: ValueKey('comment-more-replies-$commentId'),
                semanticLabel: '查看更多 $author 的回覆',
                hint: '載入尚未顯示的回覆',
                tooltip: '查看更多回覆',
                onTap: () => _ensureReplies(commentId),
                child: Text(
                  '查看更多回覆',
                  style: TextStyle(fontSize: 14, color: AppColors.primary),
                ),
              ),
            ),
        ],
      ),
    );
  }

  /// 回覆與刪除的入口：只看伺服器的 `viewer_can_reply`、`viewer_can_delete`。
  Widget _buildActions(
    pb.Comment comment, {
    required String author,
    required String rootCommentId,
  }) {
    final id = comment.id;
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        if (comment.viewerCanReply)
          HkTapTarget(
            key: ValueKey('comment-reply-$id'),
            semanticLabel: '回覆 $author 的留言',
            hint: '開始撰寫回覆',
            tooltip: '回覆',
            onTap: _sending
                ? null
                : () => _beginReply(
                    rootCommentId: rootCommentId,
                    targetCommentId: id,
                    targetName: author,
                  ),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                ChatBubbleIcon(size: 24.sp, color: AppIconColors.defaultColor),
                SizedBox(width: 6.w),
                Text(
                  '回覆',
                  style: TextStyle(fontSize: 14, color: AppColors.textPrimary),
                ),
              ],
            ),
          ),
        if (comment.viewerCanReply && comment.viewerCanDelete)
          SizedBox(width: 16.w),
        if (comment.viewerCanDelete)
          HkTapTarget(
            key: ValueKey('comment-delete-$id'),
            semanticLabel: '刪除 $author 的留言',
            hint: '刪除這則留言',
            tooltip: '刪除留言',
            onTap: _sending ? null : () => _delete(comment),
            child: Text(
              '刪除',
              style: TextStyle(fontSize: 14, color: AppColors.danger),
            ),
          ),
      ],
    );
  }

  Widget _likeColumn(
    pb.Comment comment,
    String author,
    bool liked,
    int likeCount,
  ) {
    return Column(
      children: [
        HkTapTarget(
          key: ValueKey('comment-like-${comment.id}'),
          semanticLabel: liked ? '取消對 $author 留言的讚' : '按讚 $author 的留言',
          hint: '$likeCount 個讚',
          tooltip: liked ? '取消讚' : '按讚',
          toggled: liked,
          onTap: () => _toggleLike(comment),
          child: Icon(
            liked ? AppIcons.favorite : AppIcons.favoriteBorder,
            color: liked ? AppIconColors.favorite : AppIconColors.defaultColor,
            size: 24.sp,
          ),
        ),
        SizedBox(height: 2.h),
        Text(
          _formatCount(likeCount),
          style: TextStyle(fontSize: 14, color: AppColors.textPrimary),
          textAlign: TextAlign.center,
        ),
      ],
    );
  }

  Widget _buildReply(pb.Comment reply, {required String rootCommentId}) {
    final String? placeholder = _placeholderText(reply);
    final String author = _authorName(reply);
    final int likeCount = _commentLikeCounts[reply.id] ?? 0;
    final bool liked = _likedCommentIds.contains(reply.id);

    return Padding(
      padding: EdgeInsets.symmetric(vertical: 8.h),
      child: placeholder != null
          ? Text(
              placeholder,
              key: ValueKey('comment-placeholder-${reply.id}'),
              style: TextStyle(fontSize: 14, color: AppColors.textSecondary),
            )
          : Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Padding(
                  padding: EdgeInsets.only(top: 4.h),
                  child: _avatarCircle(_avatarUri(reply), size: 28),
                ),
                SizedBox(width: 8.w),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        author,
                        style: TextStyle(
                          fontSize: 14,
                          fontWeight: FontWeight.w700,
                          color: AppColors.textEmphasis,
                        ),
                      ),
                      SizedBox(height: 4.h),
                      if (reply.body.isNotEmpty)
                        Text(
                          reply.body,
                          style: const TextStyle(
                            fontSize: 14,
                            color: AppColors.textPrimary,
                            height: 20 / 14,
                          ),
                        ),
                      SizedBox(height: 4.h),
                      _buildActions(
                        reply,
                        author: author,
                        rootCommentId: rootCommentId,
                      ),
                    ],
                  ),
                ),
                _likeColumn(reply, author, liked, likeCount),
              ],
            ),
    );
  }

  Widget _avatarCircle(String? uri, {double size = 32}) {
    if (uri == null) {
      return CircleAvatar(
        radius: size / 2,
        child: Icon(AppIcons.personOutline, size: size / 2),
      );
    }
    return CircleAvatar(radius: size / 2, backgroundImage: NetworkImage(uri));
  }

  String _formatCount(int count) {
    if (count >= 1000000) return '${(count / 1000000).toStringAsFixed(1)}M';
    if (count >= 1000) return '${(count / 1000).toStringAsFixed(1)}K';
    return count.toString();
  }
}
