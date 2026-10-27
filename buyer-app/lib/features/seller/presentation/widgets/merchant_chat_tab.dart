import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../../config/image_config.dart';
import '../../../../core/networking/dio_provider.dart';
import '../../../../core/theme/app_typography.dart';
import '../../../../shared/utils/hk_log.dart';
import '../../../../shared/widgets/chat_bubble_icon.dart';
import '../../../../shared/widgets/common_snackbar.dart';
import '../../../../shared/widgets/hk_button_label.dart';
import '../../../../shared/widgets/hk_frosted_icon_button.dart';
import '../../../../shared/utils/avatar_wire.dart';
import '../../../../shared/widgets/hk_empty_state.dart';
import '../../../../shared/widgets/fruit_refresh_indicator.dart';
import '../../../../shared/widgets/oss_image.dart';
import '../../../../shared/widgets/truck_loading_overlay.dart';
import 'merchant_layout.dart';

// ---------------------------------------------------------------------------
// 資料模型
// ---------------------------------------------------------------------------

/// 留言節點（支援樹狀回覆結構）。
///
/// 欄位均由 [_MerchantChatTabState._parseCommentNode] 解析填入。
/// [userName]、[userAvatar]、[likeCount] 為可變欄位，支援懶加載更新。
class _CommentNode {
  _CommentNode({
    required this.id,
    required this.isMine,
    required this.userName,
    required this.userAvatar,
    required this.content,
    required this.createdAt,
    required this.likeCount,
    this.replyCount = 0,
    this.depth = 0,
    this.parentId = '',
    this.rootId = '',
    this.replyToUserName = '',
    required this.replies,
  });

  final String id;
  final bool isMine;
  String userName;
  String userAvatar;
  final String content;
  final DateTime createdAt;
  int likeCount;
  final int replyCount;
  final int depth;
  final String parentId;
  final String rootId;
  final String replyToUserName;
  final List<_CommentNode> replies;
}

// ---------------------------------------------------------------------------
// MerchantChatTab
// ---------------------------------------------------------------------------

/// 商家頁「留言」Tab。
///
/// 以樹狀結構呈現商家留言板（最多兩層），支援：
/// - 發表頂層留言 / 回覆特定留言
/// - 編輯、刪除自己的留言（長壓或點 ⋯ 觸發選單）
/// - 按讚留言（樂觀更新）
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

  Future<void> _loadComments() async {
    if (widget.merchantId == null || widget.merchantId!.isEmpty) return;
    setState(() => _loading = true);
    try {
      final dio = widget.ref?.read(dioProvider);
      if (dio == null) {
        if (mounted) setState(() => _loading = false);
        return;
      }
      final resp = await dio.get<Map<String, dynamic>>(
        '/public/sellers/${widget.merchantId}/forum',
      );
      final body = resp.data ?? {};
      final data = body['data'];
      final List list;
      if (data is Map<String, dynamic>) {
        list = (data['records'] as List?) ?? [];
      } else if (data is List) {
        list = data;
      } else {
        list = [];
      }
      final messages = list.map((e) => e as Map<String, dynamic>).toList();
      final nodes = _buildCommentTree(messages);
      if (mounted) {
        setState(() {
          _comments = nodes;
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
      final dio = widget.ref?.read(dioProvider);
      if (dio == null) return;
      final resp = await dio.get<Map<String, dynamic>>(
        '/public/sellers/forum/${node.id}/replies',
      );
      final data = resp.data?['data'];
      final List replies;
      if (data is Map) {
        replies = (data['records'] as List?) ?? [];
      } else if (data is List) {
        replies = data;
      } else {
        replies = [];
      }
      if (replies.isEmpty || !mounted) return;
      final newReplies = <_CommentNode>[];
      for (final r in replies) {
        if (r is Map<String, dynamic>) {
          newReplies.add(
            _parseCommentNode(r, depth: 1, parentName: node.userName),
          );
        }
      }
      setState(() {
        node.replies.clear();
        node.replies.addAll(newReplies);
      });
    } catch (_) {}
  }

  // ── 互動操作 ──────────────────────────────────────────────────────────────

  Future<void> _submitComment() async {
    final text = _inputController.text.trim();
    if (_submitting || text.isEmpty || widget.merchantId == null) return;
    final dio = widget.ref?.read(dioProvider);
    if (dio == null) return;
    setState(() => _submitting = true);
    try {
      if (_replyTarget != null) {
        await dio.post<Map<String, dynamic>>(
          '/app/sellers/forum/${_replyTarget!.id}/reply',
          data: {'content': text},
        );
        if (!mounted) return;
        _inputController.clear();
        setState(() => _replyTarget = null);
      } else {
        await dio.post<Map<String, dynamic>>(
          '/app/sellers/${widget.merchantId}/forum',
          data: {'content': text},
        );
        if (!mounted) return;
        _inputController.clear();
      }
      await _loadComments();
    } catch (e) {
      HKLog.logE('[MerchantChatTab] _submitComment error: $e', e);
      if (mounted) showCommonSnackBar(context, '傳送失敗，請稍後再試');
    } finally {
      if (mounted) setState(() => _submitting = false);
    }
  }

  Future<void> _submitEditedComment() async {
    if (_submitting || _editingNode == null) return;
    final content = _inputController.text.trim();
    if (content.isEmpty) return;
    final dio = widget.ref?.read(dioProvider);
    if (dio == null) return;
    setState(() => _submitting = true);
    try {
      await dio.put<Map<String, dynamic>>(
        '/app/sellers/forum/${_editingNode!.id}',
        data: {'content': content},
      );
      if (mounted) {
        showCommonSnackBar(context, '留言已更新');
        setState(() {
          _editingNode = null;
          _inputController.clear();
        });
        await _loadComments();
      }
    } catch (e) {
      if (mounted) showCommonSnackBar(context, '更新失敗: $e');
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

  Future<void> _onLikeComment(_CommentNode node) async {
    if (widget.merchantId == null) return;
    try {
      final dio = widget.ref?.read(dioProvider);
      if (dio == null) return;
      final resp = await dio.post<Map<String, dynamic>>(
        '/app/posts/comments/${node.id}/like',
      );
      final liked = resp.data?['data']?['liked'] as bool? ?? true;
      setState(() {
        if (liked) {
          node.likeCount++;
        } else if (node.likeCount > 0) {
          node.likeCount--;
        }
      });
    } catch (_) {}
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
      final dio = widget.ref?.read(dioProvider);
      if (dio == null) return;
      await dio.delete<Map<String, dynamic>>('/app/sellers/forum/${node.id}');
      if (mounted) {
        showCommonSnackBar(context, '留言已刪除');
        _loadComments();
      }
    } catch (e) {
      if (mounted) showCommonSnackBar(context, '刪除失敗: $e');
    }
  }

  // ── 解析工具 ──────────────────────────────────────────────────────────────

  List<_CommentNode> _buildCommentTree(List<Map<String, dynamic>> messages) {
    final allNodes = <String, _CommentNode>{};
    final rootNodes = <_CommentNode>[];

    for (final msg in messages) {
      final node = _parseCommentNode(msg);
      allNodes[node.id] = node;
    }

    final hasNestedReplies = messages.any(
      (m) => (m['replies'] as List?)?.isNotEmpty == true,
    );
    if (hasNestedReplies) {
      return allNodes.values
          .where(
            (n) =>
                n.parentId.isEmpty || n.parentId == 'null' || n.parentId == '0',
          )
          .toList();
    }

    for (final node in allNodes.values) {
      if (node.parentId.isEmpty ||
          node.parentId == 'null' ||
          node.parentId == '0') {
        rootNodes.add(node);
      } else {
        final parent = allNodes[node.parentId];
        if (parent != null) {
          parent.replies.add(node);
        } else {
          rootNodes.add(node);
        }
      }
    }
    return rootNodes;
  }

  _CommentNode _parseCommentNode(
    Map<String, dynamic> m, {
    int depth = 0,
    String parentName = '',
  }) {
    // ForumRecord：userName / isMine（皆 camelCase，見 gateway seller_forum.go）。
    String userName = (m['userName'] ?? '').toString().trim();
    if (userName.isEmpty) userName = '匿名用戶';

    // 契約成對回 `avatarKey` + `avatarUrl`（gateway seller_forum.go
    // `ForumCommentItem`，49 §6.6 收斂後扁平成對），SSOT 是 key。舊名
    // （userAvatarKey / user_avatar_key / logo / header / avatar…）在後端都不存在，
    // 留著只會在哪天有同名欄位時搶先命中並蓋掉正確值。
    final String avatar = resolveAvatarKey(m) ?? '';

    DateTime createdAt = DateTime.now();
    final timeRaw = m['createdAt'];
    if (timeRaw is int) {
      createdAt = DateTime.fromMillisecondsSinceEpoch(
        timeRaw < 10000000000 ? timeRaw * 1000 : timeRaw,
      );
    } else if (timeRaw != null) {
      try {
        createdAt = DateTime.parse(timeRaw.toString());
      } catch (_) {}
    }

    final rawReplies = m['replies'];
    final List<_CommentNode> childReplies = [];
    if (rawReplies is List) {
      for (final r in rawReplies) {
        if (r is Map<String, dynamic>) {
          childReplies.add(
            _parseCommentNode(r, depth: depth + 1, parentName: userName),
          );
        }
      }
    }

    return _CommentNode(
      id: (m['id'] ?? '').toString(),
      isMine: m['isMine'] == true,
      userName: userName,
      userAvatar: avatar,
      content: (m['content'] ?? '').toString(),
      createdAt: createdAt,
      // ForumRecord 沒有按讚欄位（討論區留言不支援按讚）。
      likeCount: 0,
      replyCount: int.tryParse((m['replyCount'] ?? '0').toString()) ?? 0,
      depth: depth,
      parentId: (m['parentId'] ?? '').toString(),
      // ForumRecord 沒有 rootId；巢狀結構由 parentId + replies 表達。
      rootId: '',
      replyToUserName: parentName,
      replies: childReplies,
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
      onLike: () => _onLikeComment(n),
      formatTimeAgo: _formatTimeAgo,
      onEdit: _onEditComment,
      onDelete: _onDeleteComment,
      actionsEnabled: !_submitting,
    );

    Widget buildAvatar(_CommentNode n) => ClipOval(
      child: n.userAvatar.isNotEmpty
          ? OssImage(
              imageSource: n.userAvatar,
              scene: ImageScene.avatarSmall,
              width: 32,
              height: 32,
              fit: BoxFit.cover,
              errorWidget: _defaultAvatarBox(n.userName),
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
/// 自己的留言（[_isOwnComment]）支援長壓或點 ⋯ 觸發編輯/刪除選單。
class _CommentCard extends StatelessWidget {
  const _CommentCard({
    required this.node,
    required this.onReply,
    required this.onLike,
    required this.formatTimeAgo,
    required this.onEdit,
    required this.onDelete,
    required this.actionsEnabled,
  });

  final _CommentNode node;
  final VoidCallback onReply;
  final VoidCallback onLike;
  final String Function(DateTime) formatTimeAgo;
  final void Function(_CommentNode) onEdit;
  final void Function(_CommentNode) onDelete;
  final bool actionsEnabled;

  bool get _isOwnComment => node.isMine;

  Future<void> _showActionMenu(
    BuildContext context,
    TapDownDetails details,
  ) async {
    if (!_isOwnComment || !actionsEnabled) return;
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
            onEdit: () => navigator.pop('edit'),
            onDelete: () => navigator.pop('delete'),
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
      onLongPressStart: _isOwnComment && actionsEnabled
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
                if (_isOwnComment) ...[
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
            Text(node.content, style: AppTypography.body),
            SizedBox(height: 8.h),
            // 操作列
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
  const _ActionMenuContent({required this.onEdit, required this.onDelete});

  final VoidCallback onEdit;
  final VoidCallback onDelete;

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
          _menuItem(
            context,
            icon: AppIcons.editOutlined,
            label: '編輯留言',
            onTap: onEdit,
          ),
          Center(
            child: Container(
              width: 110.w,
              height: 1,
              color: AppColors.cardBorder,
            ),
          ),
          _menuItem(
            context,
            icon: AppIcons.deleteOutline,
            label: '刪除留言',
            onTap: onDelete,
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
