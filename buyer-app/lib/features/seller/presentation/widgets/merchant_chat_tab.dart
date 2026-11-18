import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.pb.dart' as pb;
import 'package:hawkr_api/images.dart' show ImagePreset, ImageUri;
import 'package:uuid/uuid.dart';
import '../../../../core/networking/suspension_signal.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../../core/theme/app_typography.dart';
import '../../../../shared/utils/hk_log.dart';
import '../../../../shared/widgets/chat_bubble_icon.dart';
import '../../../../shared/widgets/common_snackbar.dart';
import '../../../../shared/widgets/hk_button_label.dart';
import '../../../../shared/widgets/hk_frosted_icon_button.dart';
import '../../../../shared/widgets/hk_empty_state.dart';
import '../../../../shared/widgets/fruit_refresh_indicator.dart';
import '../../../../shared/widgets/truck_loading_overlay.dart';
import '../../../social/data/social_content_repository.dart';
import 'merchant_layout.dart';

// ---------------------------------------------------------------------------
// 資料模型
// ---------------------------------------------------------------------------

/// 討論區項目節點（支援樹狀回覆結構）。包著生成的 [pb.ForumEntry]：能不能編輯、刪除、回覆只讀它的
/// `viewer_can_*`。[replies] 在懶加載回覆時更新。
class _CommentNode {
  _CommentNode({
    required this.entry,
    required this.userName,
    required this.userAvatar,
    required this.createdAt,
    this.replyToUserName = '',
    required this.replies,
  });

  final pb.ForumEntry entry;
  final String userName;
  final String userAvatar;
  final DateTime createdAt;
  final String replyToUserName;
  final List<_CommentNode> replies;

  String get id => entry.id;
  String get content => entry.body;
  int get replyCount => entry.replyCount.toInt();

  /// 已刪除或被隱藏的項目（只在底下有可見回覆時出現）只顯示佔位文字。
  bool get isPlaceholder =>
      entry.state != pb.CommentState.COMMENT_STATE_VISIBLE;
}

const _deletedEntryText = '此留言已刪除';
const _hiddenEntryText = '此留言已被隱藏';

// ---------------------------------------------------------------------------
// MerchantChatTab
// ---------------------------------------------------------------------------

/// 商家頁「留言」Tab。
///
/// 以樹狀結構呈現商家留言板（最多兩層），支援：
/// - 發表頂層留言 / 回覆特定留言
/// - 編輯、刪除留言（伺服器說可以時，長壓或點 ⋯ 觸發選單；討論區沒有按讚）
/// - 懶加載子回覆及發言者資料
///
/// 注意：透過 [ref] 參數取得 Riverpod 依賴，保持本 Widget 為純 [StatefulWidget]。
class MerchantChatTab extends StatefulWidget {
  const MerchantChatTab({
    super.key,
    this.ref,
    this.merchantId,
    required this.merchantName,
    this.outerHandle,
  });

  /// Riverpod [WidgetRef]，由父層 ConsumerWidget 傳入。
  final WidgetRef? ref;
  final String? merchantId;
  final String merchantName;

  /// [NestedScrollView] 的 [SliverOverlapAbsorberHandle]，用於補償 pinned header 高度。
  final SliverOverlapAbsorberHandle? outerHandle;

  @override
  State<MerchantChatTab> createState() => _MerchantChatTabState();
}

class _MerchantChatTabState extends State<MerchantChatTab>
    with AutomaticKeepAliveClientMixin {
  @override
  bool get wantKeepAlive => true;
  List<_CommentNode> _comments = [];
  bool _loading = true;
  bool _submitting = false;
  final TextEditingController _inputController = TextEditingController();
  final FocusNode _inputFocusNode = FocusNode();
  _CommentNode? _replyTarget;
  _CommentNode? _editingNode;
  static const Uuid _uuid = Uuid();

  /// 觀看者能不能新增頂層留言（`ListForumEntries` 回應）；回覆與編輯另看項目的 `viewer_can_*`。
  bool _viewerCanComment = false;

  // ── 初始化 ────────────────────────────────────────────────────────────────

  @override
  void initState() {
    super.initState();
    _loadComments();
  }

  @override
  void dispose() {
    _inputController.dispose();
    _inputFocusNode.dispose();
    super.dispose();
  }

  // ── 資料載入 ──────────────────────────────────────────────────────────────

  SocialContentRepository? get _repo =>
      widget.ref?.read(socialContentRepositoryProvider);

  Future<void> _loadComments() async {
    if (widget.merchantId == null || widget.merchantId!.isEmpty) return;
    setState(() => _loading = true);
    try {
      final repo = _repo;
      if (repo == null) {
        if (mounted) setState(() => _loading = false);
        return;
      }
      final response = await repo.listForumEntries(widget.merchantId!);
      final nodes = response.entries.map((e) => _nodeOf(e)).toList();
      if (mounted) {
        setState(() {
          _comments = nodes;
          _viewerCanComment = response.viewerCanComment;
          _loading = false;
        });
      }
      for (final node in nodes) {
        if (node.replyCount > 0 && node.replies.isEmpty) _loadRepliesFor(node);
      }
    } catch (e) {
      HKLog.logE('[MerchantChatTab] _loadComments error: $e', e);
      if (mounted) setState(() => _loading = false);
    }
  }

  Future<void> _loadRepliesFor(_CommentNode node) async {
    try {
      final repo = _repo;
      if (repo == null) return;
      final entries = <pb.ForumEntry>[];
      var token = '';
      do {
        final response = await repo.listForumEntries(
          widget.merchantId!,
          parentId: node.id,
          pageToken: token,
        );
        entries.addAll(response.entries);
        token = response.nextPageToken;
      } while (token.isNotEmpty);
      if (entries.isEmpty || !mounted) return;
      final newReplies = entries
          .map((e) => _nodeOf(e, parentName: node.userName))
          .toList();
      setState(() {
        node.replies
          ..clear()
          ..addAll(newReplies);
      });
    } catch (_) {}
  }

  // ── 互動操作 ──────────────────────────────────────────────────────────────

  void _reportFailure(Object error, String message) {
    if (!mounted) return;
    // 停權由 transport 的 interceptor 提示，這裡不再疊一則失敗訊息。
    if (isAccountSuspendedError(error)) return;
    showCommonSnackBar(context, message);
  }

  Future<void> _submitComment() async {
    final text = _inputController.text.trim();
    if (_submitting || text.isEmpty || widget.merchantId == null) return;
    final repo = _repo;
    if (repo == null) return;
    setState(() => _submitting = true);
    try {
      final target = _replyTarget;
      await repo.createForumEntry(
        widget.merchantId!,
        text,
        parentId: target?.id,
        requestId: _uuid.v4(),
      );
      if (!mounted) return;
      _inputController.clear();
      if (target != null) setState(() => _replyTarget = null);
      await _loadComments();
    } catch (e) {
      HKLog.logE('[MerchantChatTab] _submitComment error: $e', e);
      _reportFailure(e, '傳送失敗，請稍後再試');
    } finally {
      if (mounted) setState(() => _submitting = false);
    }
  }

  Future<void> _submitEditedComment() async {
    if (_submitting || _editingNode == null) return;
    final content = _inputController.text.trim();
    if (content.isEmpty) return;
    final repo = _repo;
    if (repo == null) return;
    setState(() => _submitting = true);
    try {
      await repo.updateForumEntry(_editingNode!.entry, content);
      if (mounted) {
        showCommonSnackBar(context, '留言已更新');
        setState(() {
          _editingNode = null;
          _inputController.clear();
        });
        await _loadComments();
      }
    } catch (e) {
      _reportFailure(e, '更新失敗，請稍後再試');
    } finally {
      if (mounted) setState(() => _submitting = false);
    }
  }

  void _cancelEdit() => setState(() {
    _editingNode = null;
    _inputController.clear();
  });

  void _setReplyTarget(_CommentNode? node) {
    if (_submitting) return;
    setState(() {
      if (_editingNode != null) _inputController.clear();
      _editingNode = null;
      _replyTarget = node;
    });
    if (node != null) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (mounted) _inputFocusNode.requestFocus();
      });
    }
  }

  void _onEditComment(_CommentNode node) {
    if (_submitting) return;
    setState(() {
      _editingNode = node;
      _replyTarget = null;
      _inputController.text = node.content;
    });
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted) _inputFocusNode.requestFocus();
    });
  }

  Future<void> _onDeleteComment(_CommentNode node) async {
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
    if (confirm != true) return;
    try {
      final repo = _repo;
      if (repo == null) return;
      await repo.deleteForumEntry(node.entry);
      if (mounted) {
        showCommonSnackBar(context, '留言已刪除');
        _loadComments();
      }
    } catch (e) {
      _reportFailure(e, '刪除失敗，請稍後再試');
    }
  }

  // ── 解析工具 ──────────────────────────────────────────────────────────────

  _CommentNode _nodeOf(pb.ForumEntry entry, {String parentName = ''}) {
    final author = entry.author;
    final name = entry.hasAuthor() && author.hasName()
        ? author.name.trim()
        : '';
    final avatar = entry.hasAuthor() && author.hasAvatar()
        ? author.avatar.uri(ImagePreset.avatar)
        : '';
    return _CommentNode(
      entry: entry,
      userName: name.isEmpty ? '匿名用戶' : name,
      userAvatar: avatar,
      createdAt: entry.hasCreateTime()
          ? entry.createTime.toDateTime().toLocal()
          : DateTime.now(),
      replyToUserName: parentName,
      replies: [],
    );
  }

  String _formatTimeAgo(DateTime dt) {
    final now = DateTime.now();
    final diff = now.difference(dt);
    if (diff.inHours < 24) {
      if (diff.inHours > 0) return '${diff.inHours}小時前';
      if (diff.inMinutes > 0) return '${diff.inMinutes}分鐘前';
      return '剛剛';
    }
    if (diff.inDays < 7) return '${diff.inDays}天前';
    if (dt.year == now.year) {
      return '${dt.month.toString().padLeft(2, '0')}-${dt.day.toString().padLeft(2, '0')}';
    }
    return '${dt.year}/${dt.month.toString().padLeft(2, '0')}/${dt.day.toString().padLeft(2, '0')}';
  }

  // ── Build ─────────────────────────────────────────────────────────────────

  @override
  Widget build(BuildContext context) {
    super.build(context);
    if (widget.merchantId == null || widget.merchantId!.isEmpty) {
      return const Center(child: Text('尚未選擇商家'));
    }
    return ColoredBox(
      color: AppColors.bgPrimary,
      child: Column(
        children: [
          Expanded(
            child: FruitRefreshIndicator(
              onRefresh: _loadComments,
              topOffset: widget.outerHandle?.layoutExtent ?? 0,
              child: CustomScrollView(
                key: PageStorageKey<String>(
                  'merchant_comments_${widget.merchantId}',
                ),
                physics: const AlwaysScrollableScrollPhysics(),
                slivers: [
                  if (widget.outerHandle != null)
                    SliverOverlapInjector(handle: widget.outerHandle!),
                  _buildCommentListSlivers(),
                ],
              ),
            ),
          ),
          _buildInputBar(),
        ],
      ),
    );
  }

  /// 留言列表（以 Sliver 形式），依狀態輸出骨架、空狀態或實際列表。
  Widget _buildCommentListSlivers() {
    if (_loading) {
      return const SliverFillRemaining(
        hasScrollBody: false,
        child: TruckLoadingOverlay(message: '載入中...'),
      );
    }
    if (_comments.isEmpty) {
      return const SliverFillRemaining(
        hasScrollBody: false,
        child: HkEmptyState(
          key: Key('merchant-comments-empty-state'),
          iconKey: Key('merchant-comments-empty-icon'),
          icon: HkIconName.chat,
          message: '暫無留言',
          iconDimension: HkEmptyState.compactIconSize,
          haloDimension: HkEmptyState.compactHaloSize,
        ),
      );
    }
    return SliverLayoutBuilder(
      builder: (context, constraints) {
        final spec = MerchantLayoutSpec.forSize(
          Size(constraints.crossAxisExtent, MediaQuery.sizeOf(context).height),
        );
        final sideInset = MerchantLayoutSpec.centeredSideInset(
          constraints.crossAxisExtent,
          baseInset: spec.pageHorizontalPadding,
          maxContentWidth: MerchantLayoutSpec.profileContentMaxWidth,
        );
        return SliverPadding(
          padding: EdgeInsets.fromLTRB(sideInset, 16, sideInset, 24),
          sliver: SliverList.builder(
            itemCount: _comments.length,
            itemBuilder: (_, index) => _buildCommentItem(
              _comments[index],
              0,
              isLast: index == _comments.length - 1,
            ),
          ),
        );
      },
    );
  }

  Widget _buildCommentItem(
    _CommentNode node,
    int depth, {
    bool isLast = false,
  }) {
    final hasReplies = node.replies.isNotEmpty;
    const Color lineColor = AppColors.cardBorder;

    Widget commentCard(_CommentNode n) => _CommentCard(
      node: n,
      onReply: () => _setReplyTarget(n),
      formatTimeAgo: _formatTimeAgo,
      onEdit: _onEditComment,
      onDelete: _onDeleteComment,
      actionsEnabled: !_submitting,
    );

    Widget buildAvatar(_CommentNode n) => ClipOval(
      child: n.userAvatar.isNotEmpty && !n.isPlaceholder
          ? Image.network(
              n.userAvatar,
              width: 32,
              height: 32,
              fit: BoxFit.cover,
              errorBuilder: (_, __, ___) => _defaultAvatarBox(n.userName),
            )
          : _defaultAvatarBox(n.userName),
    );

    if (!hasReplies) {
      return Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          buildAvatar(node),
          const SizedBox(width: 8),
          Expanded(child: commentCard(node)),
        ],
      );
    }

    final nonLastReplies = node.replies.sublist(0, node.replies.length - 1);
    final lastReply = node.replies.last;

    // 用 Stack + Positioned 畫垂直連線，避免 IntrinsicHeight 觸發 parentDataDirty
    // (IntrinsicHeight 在 ListView/CustomScrollView 內會與 layout/semantics 衝突)。
    const double avatarSize = 32.0;
    final double lineX = avatarSize / 2 - 0.5; // 線位於頭像中央
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Stack(
          children: [
            // 外層垂直線（從父頭像底部延伸到 Stack 底）
            Positioned(
              left: lineX,
              top: avatarSize,
              bottom: 0,
              child: Container(width: 1, color: lineColor),
            ),
            Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                buildAvatar(node),
                const SizedBox(width: 8),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      commentCard(node),
                      for (final reply in nonLastReplies)
                        Stack(
                          children: [
                            Positioned(
                              left: lineX,
                              top: avatarSize,
                              bottom: 0,
                              child: Container(width: 1, color: lineColor),
                            ),
                            Row(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                buildAvatar(reply),
                                const SizedBox(width: 8),
                                Expanded(child: commentCard(reply)),
                              ],
                            ),
                          ],
                        ),
                    ],
                  ),
                ),
              ],
            ),
          ],
        ),
        CustomPaint(
          painter: _ReplyLinePainter(
            lineColor: lineColor,
            parentLineX: 16,
            replyAvatarCenterX: 16 + 8 + 16,
            replyAvatarCenterY: 16,
            hasNonLastReplies: nonLastReplies.isNotEmpty,
          ),
          child: Padding(
            padding: const EdgeInsets.only(left: 40),
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                buildAvatar(lastReply),
                const SizedBox(width: 8),
                Expanded(child: commentCard(lastReply)),
              ],
            ),
          ),
        ),
      ],
    );
  }

  Widget _defaultAvatarBox(String userName) => Container(
    width: 32,
    height: 32,
    color: AppColors.gray100,
    child: Center(
      child: Text(
        userName.isNotEmpty ? userName[0].toUpperCase() : '?',
        style: TextStyle(fontSize: 14, color: AppColors.textSecondary),
      ),
    ),
  );

  // ── 輸入欄 ────────────────────────────────────────────────────────────────

  Widget _buildInputBar() {
    final isEditing = _editingNode != null;
    final hintText = isEditing
        ? '編輯留言...'
        : _replyTarget != null
        ? '回覆 ${_replyTarget!.userName}...'
        : '留言給這個賣家...';

    final hasText = _inputController.text.trim().isNotEmpty;
    final modeLabel = isEditing
        ? '編輯留言'
        : _replyTarget != null
        ? '回覆 @${_replyTarget!.userName}'
        : null;
    final cancelMode = isEditing ? _cancelEdit : () => _setReplyTarget(null);

    final spec = MerchantLayoutSpec.of(context);
    // 新增頂層留言看 viewer_can_comment；回覆與編輯是另一種動作，進入該模式就要有輸入欄。
    if (!_viewerCanComment && _replyTarget == null && !isEditing) {
      return const SizedBox.shrink();
    }
    return TextFieldTapRegion(
      key: const Key('merchant-comment-composer'),
      child: SafeArea(
        top: false,
        child: Center(
          child: ConstrainedBox(
            constraints: const BoxConstraints(
              maxWidth: MerchantLayoutSpec.profileContentMaxWidth,
            ),
            child: Padding(
              padding: EdgeInsets.fromLTRB(
                spec.pageHorizontalPadding,
                HkSpace.sm,
                spec.pageHorizontalPadding,
                HkSpace.sm,
              ),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  if (modeLabel != null) ...[
                    DecoratedBox(
                      key: const Key('merchant-comment-mode-banner'),
                      decoration: BoxDecoration(
                        color: AppColors.primarySoft,
                        borderRadius: BorderRadius.circular(HkRadius.pill),
                      ),
                      child: Padding(
                        padding: const EdgeInsets.fromLTRB(
                          HkSpace.md,
                          HkSpace.xs,
                          HkSpace.xs,
                          HkSpace.xs,
                        ),
                        child: Row(
                          children: [
                            Icon(
                              isEditing
                                  ? AppIcons.editOutlined
                                  : AppIcons.replyOutlined,
                              size: 16,
                              color: AppIconColors.secondary,
                            ),
                            const SizedBox(width: HkSpace.sm),
                            Expanded(
                              child: Text(
                                modeLabel,
                                maxLines: 1,
                                overflow: TextOverflow.ellipsis,
                                style: AppTypography.meta.copyWith(
                                  color: AppColors.textPrimary,
                                  fontWeight: FontWeight.w700,
                                ),
                              ),
                            ),
                            Semantics(
                              button: true,
                              label: isEditing ? '取消編輯' : '取消回覆',
                              child: InkWell(
                                onTap: _submitting ? null : cancelMode,
                                customBorder: const CircleBorder(),
                                child: const SizedBox.square(
                                  dimension: 36,
                                  child: Icon(
                                    AppIcons.close,
                                    size: 18,
                                    color: AppIconColors.muted,
                                  ),
                                ),
                              ),
                            ),
                          ],
                        ),
                      ),
                    ),
                    const SizedBox(height: 6),
                  ],
                  Row(
                    key: const Key('merchant-comment-input-surface'),
                    crossAxisAlignment: CrossAxisAlignment.end,
                    children: [
                      Expanded(
                        child: TextField(
                          key: const Key('merchant-comment-input'),
                          controller: _inputController,
                          focusNode: _inputFocusNode,
                          readOnly: _submitting,
                          onChanged: (_) => setState(() {}),
                          minLines: 1,
                          maxLines: 4,
                          maxLength: 500,
                          keyboardType: TextInputType.multiline,
                          textInputAction: TextInputAction.newline,
                          decoration: InputDecoration(
                            hintText: hintText,
                            hintStyle: AppTypography.body.copyWith(
                              color: AppColors.textHint,
                            ),
                            filled: true,
                            fillColor: AppColors.bgCard,
                            border: OutlineInputBorder(
                              borderRadius: BorderRadius.circular(HkRadius.md),
                              borderSide: const BorderSide(
                                color: AppColors.borderInput,
                              ),
                            ),
                            enabledBorder: OutlineInputBorder(
                              borderRadius: BorderRadius.circular(HkRadius.md),
                              borderSide: const BorderSide(
                                color: AppColors.borderInput,
                              ),
                            ),
                            focusedBorder: OutlineInputBorder(
                              borderRadius: BorderRadius.circular(HkRadius.md),
                              borderSide: const BorderSide(
                                color: AppColors.actionPrimary,
                                width: 1.5,
                              ),
                            ),
                            disabledBorder: OutlineInputBorder(
                              borderRadius: BorderRadius.circular(HkRadius.md),
                              borderSide: const BorderSide(
                                color: AppColors.borderSubtle,
                              ),
                            ),
                            counterText: '',
                            isDense: true,
                            contentPadding: const EdgeInsets.symmetric(
                              horizontal: HkSpace.lg,
                              vertical: HkSpace.md,
                            ),
                          ),
                          style: AppTypography.body.copyWith(
                            color: AppColors.textPrimary,
                          ),
                        ),
                      ),
                      const SizedBox(width: HkSpace.sm),
                      KeyedSubtree(
                        key: const Key('merchant-comment-submit-surface'),
                        child: HkFrostedIconButton(
                          key: const Key('merchant-comment-submit'),
                          label: isEditing ? '儲存留言' : '送出留言',
                          icon: isEditing ? AppIcons.check : AppIcons.send,
                          onPressed: hasText && !_submitting
                              ? (isEditing
                                    ? _submitEditedComment
                                    : _submitComment)
                              : null,
                          loading: _submitting,
                          loadingLabel: isEditing ? '留言儲存中' : '留言傳送中',
                        ),
                      ),
                    ],
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

// ---------------------------------------------------------------------------
// _CommentCard — 單則留言卡片
// ---------------------------------------------------------------------------

/// 單則留言的展示卡片。
///
/// 顯示：使用者名稱、發布時間、回覆對象（若有）、內文、按讚/回覆操作列。
/// 自己的留言（[_hasMenu]）支援長壓或點 ⋯ 觸發編輯/刪除選單。
class _CommentCard extends StatelessWidget {
  const _CommentCard({
    required this.node,
    required this.onReply,
    required this.formatTimeAgo,
    required this.onEdit,
    required this.onDelete,
    required this.actionsEnabled,
  });

  final _CommentNode node;
  final VoidCallback onReply;
  final String Function(DateTime) formatTimeAgo;
  final void Function(_CommentNode) onEdit;
  final void Function(_CommentNode) onDelete;
  final bool actionsEnabled;

  /// 有任何一個選單動作可做：只讀伺服器的 `viewer_can_edit`、`viewer_can_delete`。
  bool get _hasMenu =>
      !node.isPlaceholder &&
      (node.entry.viewerCanEdit || node.entry.viewerCanDelete);

  Future<void> _showActionMenu(
    BuildContext context,
    TapDownDetails details,
  ) async {
    if (!_hasMenu || !actionsEnabled) return;
    final navigator = Navigator.of(context);
    final overlay = Overlay.of(context).context.findRenderObject() as RenderBox;
    final position = RelativeRect.fromRect(
      Rect.fromLTWH(details.globalPosition.dx, details.globalPosition.dy, 0, 0),
      Offset.zero & overlay.size,
    );
    final action = await showMenu<String>(
      context: context,
      position: position,
      elevation: 0,
      color: Colors.transparent,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12.r)),
      items: [
        PopupMenuItem<String>(
          enabled: false,
          padding: EdgeInsets.zero,
          child: _ActionMenuContent(
            onEdit: node.entry.viewerCanEdit
                ? () => navigator.pop('edit')
                : null,
            onDelete: node.entry.viewerCanDelete
                ? () => navigator.pop('delete')
                : null,
          ),
        ),
      ],
    );
    if (!context.mounted) return;
    if (action == 'edit') onEdit(node);
    if (action == 'delete') onDelete(node);
  }

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onLongPressStart: _hasMenu && actionsEnabled
          ? (details) => _showActionMenu(
              context,
              TapDownDetails(globalPosition: details.globalPosition),
            )
          : null,
      child: Padding(
        padding: EdgeInsets.only(bottom: 16.h),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // 用戶名稱 + 時間 + ⋯ 按鈕
            Row(
              children: [
                Expanded(
                  child: Text(
                    node.userName.isNotEmpty ? node.userName : '匿名用戶',
                    style: AppTypography.bodyBold.copyWith(
                      fontWeight: FontWeight.w700,
                    ),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  ),
                ),
                const SizedBox(width: 8),
                Text(
                  formatTimeAgo(node.createdAt),
                  maxLines: 1,
                  textAlign: TextAlign.right,
                  style: AppTypography.body.copyWith(
                    color: AppTypography.colorDisabled,
                    fontSize: 12,
                  ),
                ),
                if (_hasMenu) ...[
                  SizedBox(width: 8.w),
                  Builder(
                    builder: (buttonContext) => HkFrostedIconButton(
                      label: '更多留言操作',
                      icon: AppIcons.moreHoriz,
                      onPressed: !actionsEnabled
                          ? null
                          : () {
                              final box =
                                  buttonContext.findRenderObject()!
                                      as RenderBox;
                              _showActionMenu(
                                context,
                                TapDownDetails(
                                  globalPosition: box.localToGlobal(
                                    box.size.bottomRight(Offset.zero),
                                  ),
                                ),
                              );
                            },
                    ),
                  ),
                ],
              ],
            ),
            SizedBox(height: 4.h),
            // 回覆對象（若有）
            if (node.replyToUserName.isNotEmpty)
              Padding(
                padding: EdgeInsets.only(bottom: 4.h),
                child: Text.rich(
                  TextSpan(
                    children: [
                      TextSpan(
                        text: '回覆 ',
                        style: TextStyle(
                          color: AppColors.textSecondary,
                          fontSize: 14,
                        ),
                      ),
                      TextSpan(
                        text: '@${node.replyToUserName}',
                        style: TextStyle(
                          color: AppColors.primary,
                          fontSize: 14,
                        ),
                      ),
                    ],
                  ),
                ),
              ),
            // 留言內文
            Text(
              switch (node.entry.state) {
                pb.CommentState.COMMENT_STATE_DELETED => _deletedEntryText,
                pb.CommentState.COMMENT_STATE_HIDDEN => _hiddenEntryText,
                _ => node.content,
              },
              style: node.isPlaceholder
                  ? AppTypography.body.copyWith(color: AppColors.textSecondary)
                  : AppTypography.body,
            ),
            SizedBox(height: 8.h),
            // 操作列：回覆只看伺服器的 viewer_can_reply；討論區沒有按讚。
            if (!node.isPlaceholder && node.entry.viewerCanReply)
              Row(
                children: [
                  GestureDetector(
                    onTap: onReply,
                    child: Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        ChatBubbleIcon(
                          size: 16.sp,
                          color: AppIconColors.defaultColor.withValues(
                            alpha: 0.6,
                          ),
                        ),
                        SizedBox(width: 4.w),
                        Text(
                          '回覆',
                          style: TextStyle(
                            color: AppColors.textPrimary.withValues(alpha: 0.6),
                            fontSize: 14,
                          ),
                        ),
                      ],
                    ),
                  ),
                ],
              ),
          ],
        ),
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// _ActionMenuContent — 編輯/刪除彈出選單內容
// ---------------------------------------------------------------------------

/// 留言的編輯 / 刪除操作選單內容區域。
///
/// 以自訂容器（白底圓角 + 細邊框 + 陰影）取代系統預設 PopupMenu 背景。
class _ActionMenuContent extends StatelessWidget {
  const _ActionMenuContent({this.onEdit, this.onDelete});

  final VoidCallback? onEdit;
  final VoidCallback? onDelete;

  @override
  Widget build(BuildContext context) {
    return Container(
      decoration: BoxDecoration(
        color: AppColors.bgCard,
        borderRadius: BorderRadius.circular(12.r),
        border: Border.all(color: AppColors.textPrimary.withValues(alpha: 0.2)),
        boxShadow: const [
          BoxShadow(
            color: AppColors.cardShadow,
            blurRadius: 5.70,
            offset: Offset(-1, -1),
          ),
        ],
      ),
      padding: EdgeInsets.symmetric(vertical: 8.h),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          if (onEdit != null)
            _menuItem(
              context,
              icon: AppIcons.editOutlined,
              label: '編輯留言',
              onTap: onEdit!,
            ),
          if (onEdit != null && onDelete != null)
            Center(
              child: Container(
                width: 110.w,
                height: 1,
                color: AppColors.cardBorder,
              ),
            ),
          if (onDelete != null)
            _menuItem(
              context,
              icon: AppIcons.deleteOutline,
              label: '刪除留言',
              onTap: onDelete!,
            ),
        ],
      ),
    );
  }

  Widget _menuItem(
    BuildContext context, {
    required IconData icon,
    required String label,
    required VoidCallback onTap,
  }) {
    return InkWell(
      onTap: onTap,
      child: Padding(
        padding: EdgeInsets.symmetric(horizontal: 12.w, vertical: 8.h),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(icon, size: 24.sp, color: AppIconColors.defaultColor),
            SizedBox(width: 6.w),
            Text(
              label,
              style: TextStyle(
                color: AppColors.textPrimary,
                fontSize: 16,
                fontWeight: FontWeight.w400,
                height: 1.50,
              ),
            ),
          ],
        ),
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// _ReplyLinePainter — 回覆引導線繪製
// ---------------------------------------------------------------------------

/// 繪製留言回覆的弧形引導線。
///
/// 從父留言的垂直引導線向下，以 Bezier 曲線彎折指向最後一則回覆的頭像中心。
class _ReplyLinePainter extends CustomPainter {
  const _ReplyLinePainter({
    required this.lineColor,
    this.parentLineX = 16,
    this.replyAvatarCenterX = 51.0,
    this.replyAvatarCenterY = 16,
    this.hasNonLastReplies = false,
  });

  final Color lineColor;
  final double parentLineX;
  final double replyAvatarCenterX;
  final double replyAvatarCenterY;
  final bool hasNonLastReplies;

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
