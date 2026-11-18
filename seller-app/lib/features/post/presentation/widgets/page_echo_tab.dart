import '../../../seller/presentation/seller_store_avatar.dart';
import '../../../../features/seller/data/current_store.dart';
import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../../core/theme/hk_tokens.dart';
import '../../../../shared/utils/hk_log.dart';
import '../../../../shared/utils/floating_toast.dart';
import '../../../../config/media_image.dart';
import '../../../../shared/widgets/app_confirm_dialog.dart';
import '../../../../shared/widgets/hk_frosted_card_surface.dart';
import '../../../../shared/widgets/hk_frosted_icon_button.dart';
import '../../../../shared/widgets/hk_icon.dart';
import '../../../../shared/widgets/echo_comment_node.dart';
import '../../../../shared/widgets/fruit_refresh_indicator.dart';
import '../../../../shared/widgets/seller_floating_navigation_layout.dart';
import '../../../report/data/report_model.dart';
import '../../../report/presentation/widgets/report_sheet.dart';
import '../../data/post_comment.dart';
import '../../data/post_repository.dart';
import 'post_card.dart' show showPostActionMenu;
import 'storefront_tab_glass.dart';

/// 互動回聲 Tab — 顯示買家對貼文的留言與按讚，支援賣家回覆及刪除留言
class PageEchoTab extends ConsumerStatefulWidget {
  const PageEchoTab({
    super.key,
    this.outerHandle,
    required this.sellerId,
    required this.sellerDisplayName,
  });

  /// NestedScrollView 的 overlap absorber handle，用於 SliverOverlapInjector
  final SliverOverlapAbsorberHandle? outerHandle;
  final String sellerId;
  final String sellerDisplayName;

  @override
  ConsumerState<PageEchoTab> createState() => _PageEchoTabState();
}

class _PageEchoTabState extends ConsumerState<PageEchoTab>
    with AutomaticKeepAliveClientMixin {
  @override
  bool get wantKeepAlive => true;
  // Echo data
  List<StoreForumEntry> _echoMessages = [];
  List<EchoCommentNode> _echoCommentNodes = [];
  bool _loadingEcho = false;
  bool _echoLoaded = false;
  final Set<String> _deletingMessageIds = {};

  // Reply cache
  final Map<String, List<StoreForumEntry>> _repliesByMessageId = {};
  final Set<String> _repliesLoading = {};

  // Inline input bar controllers
  final TextEditingController _echoInputCtrl = TextEditingController();
  final FocusNode _echoFocusNode = FocusNode();

  bool _echoSubmitting = false;
  EchoCommentNode? _replyTargetNode;
  EchoCommentNode? _editingNode;

  @override
  void initState() {
    super.initState();
    Future.microtask(_loadEchoMessages);
  }

  @override
  void dispose() {
    _echoInputCtrl.dispose();
    _echoFocusNode.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    super.build(context);
    // 店家快照晚到時，自己的留言與頭像要補上。
    ref.listen(myStoreSnapshotProvider, (_, _) => _rebuildEchoTree());
    return _echoList();
  }

  /// 將討論區的頂層發言與已載入的回覆轉換為樹狀節點結構
  List<EchoCommentNode> _buildEchoCommentTree(
    List<StoreForumEntry> messages,
    Map<String, List<StoreForumEntry>> repliesByMessageId,
  ) {
    return [
      for (final m in messages)
        _buildEchoNode(m)
          ..replies.addAll([
            for (final r
                in repliesByMessageId[m.id] ?? const <StoreForumEntry>[])
              _buildEchoNode(r),
          ]),
    ];
  }

  /// 建構單個留言節點
  EchoCommentNode _buildEchoNode(StoreForumEntry m) {
    // 自己的發言名稱與頭像讀不到時用店家的快照。
    var userName = m.authorName;
    if (userName.isEmpty && m.isMine) userName = widget.sellerDisplayName;
    if (userName.isEmpty) userName = '用戶';
    var avatar = m.authorAvatar;
    if (avatar.isEmpty && m.isMine) {
      final logo = ref.read(myStoreSnapshotProvider)?.logoUrl ?? '';
      avatar = logo.isEmpty ? '' : buildImageUrl(logo, ImagePreset.avatar);
    }
    return EchoCommentNode(
      id: m.id,
      etag: m.etag,
      isMine: m.isMine,
      userName: userName,
      userAvatar: avatar,
      content: m.body,
      createdAt: m.createTime,
      placeholderText: m.placeholderText,
      viewerCanEdit: m.viewerCanEdit,
      viewerCanDelete: m.viewerCanDelete,
      viewerCanReply: m.viewerCanReply,
    );
  }

  /// 格式化時間顯示
  String _formatEchoTimeAgo(DateTime? dt) {
    if (dt == null) return '';
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

  /// 重建樹狀結構
  void _rebuildEchoTree() {
    if (!mounted) return;
    setState(() {
      _echoCommentNodes = _buildEchoCommentTree(
        _echoMessages,
        _repliesByMessageId,
      );
    });
  }

  Future<void> _loadEchoMessages() async {
    try {
      HKLog.logD('[EchoUI] _loadEchoMessages() start');
    } catch (_) {}
    if (_loadingEcho) return;
    setState(() {
      _loadingEcho = true;
    });
    try {
      final list = await ref
          .read(postRepositoryProvider)
          .listForumEntries(widget.sellerId);
      if (!mounted) return;
      setState(() {
        _echoMessages = list;
        _echoLoaded = true;
      });
      try {
        HKLog.logD('[EchoUI] _loadEchoMessages() loaded count=${list.length}');
      } catch (_) {}
      for (final m in list) {
        if (m.replyCount > 0 && !_repliesLoading.contains(m.id)) {
          Future.microtask(() => _loadRepliesForMessage(m.id));
        }
      }
    } catch (e) {
      try {
        HKLog.logD('[Echo] load messages failed: $e');
      } catch (_) {}
    } finally {
      if (mounted) {
        setState(() {
          _loadingEcho = false;
        });
        // 載入完成後重建樹狀結構
        _rebuildEchoTree();
      } else {
        _loadingEcho = false;
      }
    }
  }

  Future<void> _loadRepliesForMessage(String messageId) async {
    if (_repliesLoading.contains(messageId)) return;
    _repliesLoading.add(messageId);
    try {
      HKLog.logD('[EchoUI] load replies for messageId=$messageId');
    } catch (_) {}
    try {
      final list = await ref
          .read(postRepositoryProvider)
          .listForumEntries(widget.sellerId, parentId: messageId);
      if (!mounted) return;
      setState(() {
        _repliesByMessageId[messageId] = list;
        _repliesLoading.remove(messageId);
      });
      try {
        HKLog.logD(
          '[EchoUI] replies loaded messageId=$messageId count=${list.length}',
        );
      } catch (_) {}
      // 回覆載入後重建樹狀結構
      _rebuildEchoTree();
    } catch (e) {
      try {
        HKLog.logD('[EchoUI] load replies error messageId=$messageId e=$e');
      } catch (_) {}
      if (!mounted) return;
      setState(() {
        _repliesLoading.remove(messageId);
      });
    }
  }

  Future<void> _refreshEcho() async {
    try {
      HKLog.logD('[EchoUI] refresh Echo');
    } catch (_) {}
    // 可視需要清空回覆快取，避免舊資料殘留
    setState(() {
      _repliesByMessageId.clear();
      _repliesLoading.clear();
      _echoLoaded = false;
    });
    await _loadEchoMessages();
  }

  // Echo tab list wired to real API: GET /api/message/merchant/messages
  // 重寫為樹狀結構設計
  Widget _echoList() {
    if (!_echoLoaded && !_loadingEcho) {
      Future.microtask(() => _loadEchoMessages());
    }

    final storeNotPublic = ref.watch(storeShareProvider) is StoreShareDisabled;

    return ColoredBox(
      color: Colors.transparent,
      child: LayoutBuilder(
        builder: (context, constraints) => Column(
          children: [
            // 留言列表
            Expanded(
              child: _echoCommentNodes.isEmpty
                  ? FruitRefreshIndicator(
                      onRefresh: _refreshEcho,
                      child: CustomScrollView(
                        slivers: [
                          if (widget.outerHandle != null)
                            SliverOverlapInjector(handle: widget.outerHandle!),
                          SliverToBoxAdapter(child: SizedBox(height: 24.h)),
                          SliverToBoxAdapter(
                            child: StorefrontGlassEmptyPanel(
                              icon: HkIconName.chat,
                              message: '暫無留言',
                              description: storeNotPublic
                                  ? '店家公開後，買家的迴響會顯示在這裡。'
                                  : '買家留言後會顯示在這裡。',
                            ),
                          ),
                        ],
                      ),
                    )
                  : FruitRefreshIndicator(
                      onRefresh: _refreshEcho,
                      child: CustomScrollView(
                        slivers: [
                          if (widget.outerHandle != null)
                            SliverOverlapInjector(handle: widget.outerHandle!),
                          SliverPadding(
                            padding: EdgeInsets.only(
                              left: 16.w,
                              right: 16.w,
                              top: 12.h,
                              bottom: 24,
                            ),
                            sliver: SliverList(
                              delegate: SliverChildBuilderDelegate((
                                context,
                                index,
                              ) {
                                final isLast =
                                    index == _echoCommentNodes.length - 1;
                                return Padding(
                                  padding: EdgeInsets.only(bottom: 12.h),
                                  child: HkFrostedCardSurface(
                                    radius: HkRadius.lg,
                                    child: Padding(
                                      padding: EdgeInsets.all(16.w),
                                      child: _buildEchoCommentItem(
                                        _echoCommentNodes[index],
                                        0,
                                        isLast: isLast,
                                      ),
                                    ),
                                  ),
                                );
                              }, childCount: _echoCommentNodes.length),
                            ),
                          ),
                        ],
                      ),
                    ),
            ),
            // 底部輸入欄
            _buildEchoInputBar(availableHeight: constraints.maxHeight),
          ],
        ),
      ),
    );
  }

  /// 建構單個留言項目 - 含垂直引導線
  Widget _buildEchoCommentItem(
    EchoCommentNode node,
    int depth, {
    bool isLast = false,
  }) {
    final hasReplies = node.replies.isNotEmpty;
    const lineColor = AppColors.cardBorder;

    // 無回覆：簡單渲染
    if (!hasReplies) {
      return Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          _buildEchoAvatar(node),
          SizedBox(width: 6.w),
          Expanded(child: _buildEchoCommentContent(node, depth)),
        ],
      );
    }

    final nonLastReplies = node.replies.sublist(0, node.replies.length - 1);
    final lastReply = node.replies.last;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        // 父留言 + 非最後回覆（含直線引導線）
        IntrinsicHeight(
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              // 頭像 + 垂直引導線向下延伸
              Column(
                children: [
                  _buildEchoAvatar(node),
                  Expanded(child: Container(width: 1, color: lineColor)),
                ],
              ),
              SizedBox(width: 6.w),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    _buildEchoCommentContent(node, depth),
                    // 非最後回覆（各自含向下引導線）
                    for (final reply in nonLastReplies)
                      _buildEchoReplyItem(reply),
                  ],
                ),
              ),
            ],
          ),
        ),
        // 最後回覆（含弧形連接線）
        _buildLastReplyWithCurve(
          lastReply,
          lineColor,
          nonLastReplies.isNotEmpty,
        ),
      ],
    );
  }

  /// 建構非最後回覆項目（含向下引導線）
  Widget _buildEchoReplyItem(EchoCommentNode node) {
    const lineColor = AppColors.cardBorder;

    return IntrinsicHeight(
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Column(
            children: [
              _buildEchoAvatar(node),
              Expanded(child: Container(width: 1, color: lineColor)),
            ],
          ),
          SizedBox(width: 6.w),
          Expanded(child: _buildEchoCommentContent(node, 1)),
        ],
      ),
    );
  }

  /// 建構最後一則回覆（含弧形連接線從父引導線彎到回覆頭像）
  Widget _buildLastReplyWithCurve(
    EchoCommentNode node,
    Color lineColor,
    bool hasNonLastReplies,
  ) {
    final gapWidth = 6.w;
    final replyIndent = 30.0 + gapWidth;
    const parentLineX = 15.0;
    final replyAvatarCenterX = replyIndent + 15.0;
    const replyAvatarCenterY = 15.0;

    return CustomPaint(
      painter: _CurveConnectorPainter(
        parentLineX: parentLineX,
        replyAvatarCenterX: replyAvatarCenterX,
        replyAvatarCenterY: replyAvatarCenterY,
        color: lineColor,
        hasNonLastReplies: hasNonLastReplies,
      ),
      child: Padding(
        padding: EdgeInsets.only(left: replyIndent),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            _buildEchoAvatar(node),
            SizedBox(width: 6.w),
            Expanded(child: _buildEchoCommentContent(node, 1)),
          ],
        ),
      ),
    );
  }

  /// 顯示他人留言的操作選單（檢舉，與伺服器允許時的刪除；公開留言不帶留言者 id）
  Future<void> _showEchoOtherUserMenu(
    BuildContext context,
    EchoCommentNode node,
  ) async {
    await showModalBottomSheet(
      context: context,
      // 留言 tab 位於 `StatefulShellRoute` 分支底下，分支 Navigator 在浮動導覽列
      // 之下；不掛根 Navigator 的話 sheet 底部會被導覽列蓋住（#132）。
      useRootNavigator: true,
      backgroundColor: AppColors.bgCard,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(16.r)),
      ),
      builder: (ctx) => SafeArea(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            SizedBox(height: 8.h),
            Container(
              width: 36.w,
              height: 4.h,
              decoration: BoxDecoration(
                color: AppColors.gray200,
                borderRadius: BorderRadius.circular(2.r),
              ),
            ),
            SizedBox(height: 12.h),
            if (node.viewerCanDelete)
              ListTile(
                leading: Icon(
                  AppIcons.deleteOutline,
                  color: AppIconColors.defaultColor,
                  size: 22.sp,
                ),
                title: Text('刪除留言', style: TextStyle(fontSize: 16.sp)),
                onTap: () {
                  Navigator.pop(ctx);
                  _onEchoDeleteComment(node);
                },
              ),
            ListTile(
              leading: Icon(
                AppIcons.reportOutlined,
                color: AppIconColors.defaultColor,
                size: 22.sp,
              ),
              title: Text('檢舉留言', style: TextStyle(fontSize: 16.sp)),
              onTap: () {
                Navigator.pop(ctx);
                // 本頁是店家討論區（`listForumEntries`）——所以是 SELLER_FORUM，
                // 不是 POST_COMMENT（那是貼文底下的留言）。
                showReportSheet(
                  context,
                  ref,
                  targetType: ReportTargetType.sellerForum,
                  targetId: node.id,
                );
              },
            ),
            SizedBox(height: 8.h),
          ],
        ),
      ),
    );
  }

  /// 建構留言內容（不含頭像）
  Widget _buildEchoCommentContent(EchoCommentNode node, int depth) {
    final isSelf = node.isMine;
    final placeholder = node.placeholderText;
    if (placeholder != null) {
      return Padding(
        key: ValueKey('echo-placeholder-${node.id}'),
        padding: EdgeInsets.only(bottom: 12.h),
        child: Text(
          placeholder,
          style: TextStyle(
            fontSize: 14.sp,
            color: AppColors.textPrimary.withValues(alpha: 0.5),
          ),
        ),
      );
    }

    return Padding(
      padding: EdgeInsets.only(bottom: 12.h),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // 名稱與時間在剩餘寬度內換行；更多按鈕保留固定操作尺寸。
          // 此內容位於回覆引導線的 IntrinsicHeight 下，使用 Wrap 避免
          // LayoutBuilder 無法提供 intrinsic 尺寸的限制。
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Expanded(
                child: Wrap(
                  spacing: HkSpace.sm,
                  runSpacing: HkSpace.xs,
                  crossAxisAlignment: WrapCrossAlignment.center,
                  children: [
                    Text(
                      node.userName,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(
                        fontWeight: FontWeight.w400,
                        fontSize: 16.sp,
                        color: AppColors.textEmphasis,
                      ),
                    ),
                    Text(
                      _formatEchoTimeAgo(node.createdAt),
                      style: TextStyle(
                        fontSize: 14.sp,
                        color: AppColors.textPrimary.withValues(alpha: 0.5),
                      ),
                    ),
                  ],
                ),
              ),
              // 更多按鈕：自己的留言依 viewer_can_edit／viewer_can_delete 編輯、刪除；他人留言可檢舉（封鎖入口見上方選單的說明）
              if (isSelf && (node.viewerCanEdit || node.viewerCanDelete))
                Builder(
                  builder: (btnCtx) => HkFrostedIconButton(
                    label: '留言更多操作',
                    icon: AppIcons.moreHoriz,
                    loading: _deletingMessageIds.contains(node.id),
                    loadingLabel: '正在刪除留言',
                    onPressed: _echoSubmitting
                        ? null
                        : () => showPostActionMenu(
                            btnCtx,
                            editLabel: '編輯留言',
                            deleteLabel: '刪除留言',
                            onEdit: node.viewerCanEdit
                                ? () => _onEchoEditComment(node)
                                : null,
                            onDelete: node.viewerCanDelete
                                ? () => _onEchoDeleteComment(node)
                                : null,
                          ),
                  ),
                )
              else if (!isSelf)
                Builder(
                  builder: (btnCtx) => HkFrostedIconButton(
                    label: '留言更多操作',
                    icon: AppIcons.moreHoriz,
                    onPressed: () => _showEchoOtherUserMenu(btnCtx, node),
                  ),
                ),
            ],
          ),
          SizedBox(height: 4.h),
          // 留言內容
          Text(
            node.content,
            style: TextStyle(
              fontSize: 16.sp,
              color: AppColors.textPrimary,
              height: 1.5,
            ),
          ),
          SizedBox(height: 4.h),
          // 回覆按鈕（依 viewer_can_reply）
          if (node.viewerCanReply)
            GestureDetector(
              behavior: HitTestBehavior.opaque,
              onTap: () => _setEchoReplyTarget(node),
              child: ConstrainedBox(
                constraints: const BoxConstraints(
                  minWidth: HkControl.minTap,
                  minHeight: HkControl.minTap,
                ),
                child: Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    HkIcon(
                      HkIconName.chat,
                      size: 24.sp,
                      color: AppIconColors.defaultColor,
                    ),
                    SizedBox(width: 6.w),
                    Text(
                      '回覆',
                      style: TextStyle(
                        fontSize: 16.sp,
                        color: AppColors.textPrimary,
                      ),
                    ),
                  ],
                ),
              ),
            ),
        ],
      ),
    );
  }

  /// 建構頭像
  Widget _buildEchoAvatar(EchoCommentNode node) {
    if (node.placeholderText != null) {
      return const CircleAvatar(radius: 15, backgroundColor: AppColors.gray100);
    }
    // 如果是自己的留言，使用商家頭像
    final isSelf = node.isMine;

    if (isSelf) {
      return SellerStoreAvatar(size: 30);
    }

    ImageProvider? avatar;
    if (node.userAvatar.isNotEmpty && node.userAvatar.startsWith('http')) {
      avatar = NetworkImage(node.userAvatar);
    }

    // Figma 設計：30x30px 頭像
    return CircleAvatar(
      radius: 15,
      backgroundColor: AppColors.gray100,
      backgroundImage: avatar,
      child: avatar == null
          ? Text(
              node.userName.isNotEmpty ? node.userName[0].toUpperCase() : '?',
              style: TextStyle(
                fontSize: 12.sp,
                fontWeight: FontWeight.w700,
                color: AppColors.textSecondary,
              ),
            )
          : null,
    );
  }

  /// 回覆與編輯共用同一個輸入框，避免兩個 composer 重疊、焦點分離。
  void _setEchoReplyTarget(EchoCommentNode node) {
    if (_echoSubmitting || _deletingMessageIds.contains(node.id)) {
      return;
    }
    setState(() {
      if (_replyTargetNode?.id != node.id || _editingNode != null) {
        _echoInputCtrl.clear();
      }
      _replyTargetNode = node;
      _editingNode = null;
    });
    _focusEchoInput();
  }

  void _onEchoEditComment(EchoCommentNode node) {
    if (_echoSubmitting || _deletingMessageIds.contains(node.id)) {
      return;
    }
    setState(() {
      _editingNode = node;
      _replyTargetNode = null;
      _echoInputCtrl.value = TextEditingValue(
        text: node.content,
        selection: TextSelection.collapsed(offset: node.content.length),
      );
    });
    _focusEchoInput();
  }

  void _focusEchoInput() {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted) _echoFocusNode.requestFocus();
    });
  }

  void _clearEchoComposer() {
    _echoFocusNode.unfocus();
    setState(() {
      _editingNode = null;
      _replyTargetNode = null;
      _echoInputCtrl.clear();
    });
  }

  /// 確認後刪除發言。
  Future<void> _onEchoDeleteComment(EchoCommentNode node) async {
    final msgId = node.id;
    if (_echoSubmitting || !mounted || _deletingMessageIds.contains(msgId)) {
      return;
    }
    final confirmed = await showAppConfirmDialog(
      context,
      title: '確定要刪除此留言嗎？',
      message: '刪除後將無法復原。',
      confirmText: '刪除',
      confirmTextColor: AppColors.danger,
    );
    if (!confirmed ||
        !mounted ||
        _echoSubmitting ||
        _deletingMessageIds.contains(msgId)) {
      return;
    }
    setState(() => _deletingMessageIds.add(msgId));
    try {
      await ref
          .read(postRepositoryProvider)
          .deleteForumEntry(msgId, etag: node.etag);
      if (!mounted) return;
      if (_editingNode?.id == node.id || _replyTargetNode?.id == node.id) {
        _clearEchoComposer();
      }
      FloatingToast.success(context, '留言已刪除');
      await _refreshEcho();
    } catch (_) {
      if (mounted) FloatingToast.error(context, '刪除失敗');
    } finally {
      if (mounted) setState(() => _deletingMessageIds.remove(msgId));
    }
  }

  /// 商家只能回覆留言；選擇目標後啟用輸入，編輯也沿用同一列。
  Widget _buildEchoInputBar({required double availableHeight}) {
    final isEditing = _editingNode != null;
    final canCompose = isEditing || _replyTargetNode != null;
    final hintText = isEditing
        ? '編輯留言...'
        : _replyTargetNode != null
        ? '回覆 ${_replyTargetNode!.userName}...'
        : '請選擇留言回覆';
    final navigationClearance = sellerFloatingNavigationClearance(context);
    final lineHeight = MediaQuery.textScalerOf(context).scale(16.sp) * 1.5;
    // Short windows and the keyboard reduce the space left above navigation.
    // Keep one scrollable field and its draft; only reduce its visible lines.
    final inputBudget =
        availableHeight -
        navigationClearance -
        HkSpace.md * 4 -
        (canCompose ? HkControl.minTap : 0) -
        HkControl.minTap;
    final maxLines = (inputBudget / lineHeight).floor().clamp(1, 4);

    return Padding(
      padding: EdgeInsets.only(
        left: HkSpace.lg,
        right: HkSpace.lg,
        top: HkSpace.md,
        bottom: navigationClearance + HkSpace.md,
      ),
      child: TextFieldTapRegion(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            if (canCompose)
              Row(
                children: [
                  Expanded(
                    child: Text(
                      isEditing ? '編輯留言' : '回覆 @${_replyTargetNode!.userName}',
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(
                        color: AppColors.primary,
                        fontSize: 14.sp,
                      ),
                    ),
                  ),
                  IconButton(
                    tooltip: isEditing ? '取消編輯' : '取消回覆',
                    onPressed: _echoSubmitting ? null : _clearEchoComposer,
                    icon: Icon(
                      AppIcons.close,
                      size: 18.sp,
                      color: AppIconColors.muted,
                    ),
                  ),
                ],
              ),
            Row(
              crossAxisAlignment: CrossAxisAlignment.end,
              children: [
                Expanded(
                  child: Container(
                    constraints: const BoxConstraints(
                      minHeight: HkControl.minTap,
                    ),
                    padding: const EdgeInsets.symmetric(
                      horizontal: HkSpace.lg,
                      vertical: HkSpace.md,
                    ),
                    decoration: BoxDecoration(
                      color: AppColors.neutralWhite.withValues(alpha: 0.62),
                      borderRadius: BorderRadius.circular(HkRadius.xl),
                      border: Border.all(color: AppColors.borderSoft),
                    ),
                    child: TextField(
                      controller: _echoInputCtrl,
                      focusNode: _echoFocusNode,
                      readOnly: !canCompose || _echoSubmitting,
                      canRequestFocus: canCompose,
                      onTapOutside: (_) => _echoFocusNode.unfocus(),
                      contextMenuBuilder: (context, editableTextState) {
                        return AdaptiveTextSelectionToolbar.editableText(
                          editableTextState: editableTextState,
                        );
                      },
                      decoration: InputDecoration(
                        isDense: true,
                        contentPadding: EdgeInsets.zero,
                        hintText: hintText,
                        hintStyle: TextStyle(
                          color: AppColors.textSecondary,
                          fontSize: 16.sp,
                          fontWeight: FontWeight.w400,
                          height: 1.5,
                        ),
                        // 外層已有圓角與底色，停用全站表單的各狀態邊框。
                        filled: false,
                        border: InputBorder.none,
                        enabledBorder: InputBorder.none,
                        focusedBorder: InputBorder.none,
                        disabledBorder: InputBorder.none,
                        errorBorder: InputBorder.none,
                        focusedErrorBorder: InputBorder.none,
                      ),
                      style: TextStyle(
                        fontSize: 16.sp,
                        color: AppColors.textPrimary,
                        fontWeight: FontWeight.w400,
                        height: 1.5,
                      ),
                      minLines: 1,
                      maxLines: maxLines,
                      keyboardType: TextInputType.multiline,
                      textInputAction: TextInputAction.send,
                      onSubmitted: (_) => _submitEchoComment(),
                    ),
                  ),
                ),
                const SizedBox(width: HkSpace.sm),
                ValueListenableBuilder<TextEditingValue>(
                  valueListenable: _echoInputCtrl,
                  builder: (context, value, child) {
                    final enabled =
                        canCompose &&
                        !_echoSubmitting &&
                        value.text.trim().isNotEmpty;
                    return HkFrostedIconButton(
                      label: isEditing ? '儲存留言' : '送出留言',
                      icon: isEditing ? AppIcons.check : AppIcons.send,
                      onPressed: enabled ? _submitEchoComment : null,
                      loading: _echoSubmitting,
                      loadingLabel: isEditing ? '留言儲存中' : '留言傳送中',
                    );
                  },
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }

  Future<void> _submitEchoComment() async {
    // IME 的送出也會走這裡，不能只停用畫面上的按鈕。
    if (_echoSubmitting) return;
    final text = _echoInputCtrl.text.trim();
    final target = _editingNode ?? _replyTargetNode;
    if (text.isEmpty || target == null) return;
    final isEditing = _editingNode != null;
    if (_deletingMessageIds.contains(target.id)) return;

    setState(() => _echoSubmitting = true);
    try {
      final repository = ref.read(postRepositoryProvider);
      if (isEditing) {
        await repository.updateForumEntry(
          target.id,
          etag: target.etag,
          body: text,
        );
      } else {
        await repository.replyForumEntry(
          widget.sellerId,
          parentId: target.id,
          body: text,
        );
      }
      if (!mounted) return;
      _clearEchoComposer();
      if (isEditing) FloatingToast.success(context, '已更新留言');
      await _refreshEcho();
    } catch (e) {
      HKLog.logD('[Echo] submit error: $e');
      if (mounted) {
        FloatingToast.error(context, isEditing ? '更新失敗' : '回覆失敗');
      }
    } finally {
      if (mounted) setState(() => _echoSubmitting = false);
    }
  }
}

// =============================================================================
// Echo 留言區塊 - 樹狀結構設計
// =============================================================================

/// 弧形連接線畫筆：從父引導線彎曲連接到最後回覆頭像
class _CurveConnectorPainter extends CustomPainter {
  final double parentLineX;
  final double replyAvatarCenterX;
  final double replyAvatarCenterY;
  final Color color;
  final bool hasNonLastReplies;

  _CurveConnectorPainter({
    required this.parentLineX,
    required this.replyAvatarCenterX,
    required this.replyAvatarCenterY,
    required this.color,
    this.hasNonLastReplies = false,
  });

  @override
  void paint(Canvas canvas, Size size) {
    final paint = Paint()
      ..color = color
      ..strokeWidth = 1
      ..style = PaintingStyle.stroke;

    // 弧形：從父引導線 (parentLineX, 0) 向下彎到最後回覆頭像中心
    final curveRadius = 15.0;
    final path = Path();
    path.moveTo(parentLineX, 0);
    // 直線向下到弧形起點
    if (replyAvatarCenterY > curveRadius) {
      path.lineTo(parentLineX, replyAvatarCenterY - curveRadius);
    }
    // 弧形（垂直轉水平）
    path.quadraticBezierTo(
      parentLineX,
      replyAvatarCenterY,
      parentLineX + curveRadius,
      replyAvatarCenterY,
    );
    // 水平線到回覆頭像中心
    path.lineTo(replyAvatarCenterX, replyAvatarCenterY);
    canvas.drawPath(path, paint);

    // 如果有非最後回覆，補一段垂直短線（連接上方回覆的引導線到最後回覆頭像）
    if (hasNonLastReplies) {
      canvas.drawLine(
        Offset(replyAvatarCenterX, 0),
        Offset(replyAvatarCenterX, replyAvatarCenterY),
        paint,
      );
    }
  }

  @override
  bool shouldRepaint(covariant _CurveConnectorPainter oldDelegate) =>
      parentLineX != oldDelegate.parentLineX ||
      replyAvatarCenterX != oldDelegate.replyAvatarCenterX ||
      replyAvatarCenterY != oldDelegate.replyAvatarCenterY ||
      color != oldDelegate.color ||
      hasNonLastReplies != oldDelegate.hasNonLastReplies;
}
