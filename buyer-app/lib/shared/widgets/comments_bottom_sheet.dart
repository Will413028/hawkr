import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/app_typography.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'dart:async';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:uuid/uuid.dart';

import '../../features/social/data/comment_stream_service.dart';
import '../../features/social/data/social_repository.dart';
import '../../features/auth/data/auth_repository.dart';
import '../../core/session/session_provider.dart';
import '../utils/avatar_wire.dart';
import '../utils/require_login.dart';
import '../../core/extensions/url_extensions.dart';
import '../../config/flavor.dart';
import '../../config/image_config.dart';
import 'chat_bubble_icon.dart';
import 'hk_empty_state.dart';
import 'hk_tap_target.dart';
import 'hk_frosted_icon_button.dart';
import 'common_snackbar.dart';
import 'truck_loading_overlay.dart';

class CommentsBottomSheet extends ConsumerStatefulWidget {
  const CommentsBottomSheet({
    super.key,
    required this.postId,
    this.postOwnerName,
    this.merchantAvatar,
  });

  final String postId;
  final String? postOwnerName;
  final String? merchantAvatar;

  @override
  ConsumerState<CommentsBottomSheet> createState() =>
      _CommentsBottomSheetState();
}

class _CommentsBottomSheetState extends ConsumerState<CommentsBottomSheet>
    with WidgetsBindingObserver {
  final TextEditingController _controller = TextEditingController();
  final FocusNode _focusNode = FocusNode();
  final ScrollController _scrollController = ScrollController();
  static const Uuid _uuid = Uuid();

  final Map<String, List<Map<String, dynamic>>> _repliesByParent =
      <String, List<Map<String, dynamic>>>{};
  final Set<String> _repliesLoading = <String>{};

  final Map<String, int> _commentLikeCounts = <String, int>{};
  final Set<String> _likedCommentIds = <String>{};

  List<Map<String, dynamic>> _comments = <Map<String, dynamic>>[];
  bool _loading = true;
  bool _sending = false;

  /// Reddit-style thread 的根留言 ID，新回覆會掛在這個 thread 下。
  String? _replyToCommentId;

  /// 實際被回覆的留言 ID，可能是根留言或某則回覆。
  String? _replyToTargetId;
  String? _replyToName;
  String? _inputAvatar;
  String? _myDisplayName;

  CommentStreamService? _streamService;
  bool _degradedNoticeShown = false;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    _controller.addListener(() {
      if (mounted) setState(() {});
    });
    _initInputAvatar();
    _loadComments();
    _startStream();
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    _stopStream();
    _controller.dispose();
    _focusNode.dispose();
    _scrollController.dispose();
    super.dispose();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    super.didChangeAppLifecycleState(state);
    if (!mounted) return;
    switch (state) {
      case AppLifecycleState.paused:
      case AppLifecycleState.detached:
      case AppLifecycleState.hidden:
        _stopStream();
        break;
      case AppLifecycleState.resumed:
        if (_streamService == null) {
          _startStream();
          // resume 後預設保險 refresh,避免漏接到 buffer 視窗外的事件
          _loadComments();
        }
        break;
      case AppLifecycleState.inactive:
        break;
    }
  }

  void _startStream() {
    debugPrint('[SSE-DEBUG] _startStream called postId=${widget.postId}');
    if (_streamService != null) {
      debugPrint('[SSE-DEBUG] _startStream skipped — already running');
      return;
    }
    final postIdInt = int.tryParse(widget.postId);
    if (postIdInt == null) {
      debugPrint('[SSE-DEBUG] _startStream skipped — postId not int');
      return;
    }
    debugPrint(
      '[SSE-DEBUG] _startStream constructing service postIdInt=$postIdInt apiBaseUrl=${F.appFlavor.apiBaseUrl}',
    );
    final service = CommentStreamService(
      apiBaseUrl: F.appFlavor.apiBaseUrl,
      postId: postIdInt,
      onCommentEvent: _handleStreamEvent,
      onBufferMiss: () {
        if (!mounted) return;
        _loadComments();
      },
      onConnectionDegraded: () {
        if (!mounted || _degradedNoticeShown) return;
        _degradedNoticeShown = true;
        final messenger = ScaffoldMessenger.maybeOf(context);
        messenger?.showSnackBar(
          const SnackBar(
            content: Text('即時同步暫時不可用'),
            duration: Duration(seconds: 3),
          ),
        );
      },
      onConnectionRecovered: () {
        _degradedNoticeShown = false;
      },
    );
    _streamService = service;
    service.start();
  }

  void _stopStream() {
    _streamService?.dispose();
    _streamService = null;
  }

  Future<void> _initInputAvatar() async {
    try {
      final session = ref.read(sessionProvider).value;
      if (!mounted) return;
      if (hasUsableSession(session)) {
        final activeSession = session!;
        setState(() => _myDisplayName = activeSession.nickname);
        try {
          final info = await ref.read(authRepositoryProvider).tryGetProfile();
          if (!mounted) return;
          if (info != null) {
            if (_inputAvatar == null || _inputAvatar!.isEmpty) {
              final avatar = info.avatarUrl;
              if (avatar.isNotEmpty) {
                setState(() => _inputAvatar = avatar);
                return;
              }
            }
          }
        } catch (_) {}
      }
    } catch (_) {}
    if (!mounted) return;
    if (_inputAvatar != null && _inputAvatar!.isNotEmpty) return;
    final fallback = widget.merchantAvatar?.trim();
    if (fallback != null && fallback.isNotEmpty) {
      setState(() => _inputAvatar = fallback);
    }
  }

  Future<void> _loadComments() async {
    setState(() => _loading = true);
    try {
      final list = await ref
          .read(socialRepositoryProvider)
          .getCommentsRaw(widget.postId);
      final List<Map<String, dynamic>> topLevel = <Map<String, dynamic>>[];
      final Map<String, List<Map<String, dynamic>>> replies =
          <String, List<Map<String, dynamic>>>{};
      final Map<String, int> likeCounts = <String, int>{};
      final Set<String> likedIds = <String>{};
      final List<Map<String, dynamic>> flattened = <Map<String, dynamic>>[];

      for (final raw in list) {
        final item = Map<String, dynamic>.from(raw);
        flattened.add(item);
        final String? parentId = _commentIdOf(item);
        final embedded = item['replies'];
        if (parentId != null && embedded is List) {
          for (final rawReply in embedded.whereType<Map>()) {
            final reply = Map<String, dynamic>.from(rawReply);
            reply['parentId'] ??= parentId;
            flattened.add(reply);
          }
        }
      }

      final byId = <String, Map<String, dynamic>>{
        for (final item in flattened)
          if (_commentIdOf(item) case final String id) id: item,
      };
      final processedIds = <String>{};

      for (final item in flattened) {
        final String? cid = _commentIdOf(item);
        if (cid == null || !processedIds.add(cid)) continue;

        final String? parentId = _parentIdOf(item);
        if (parentId == null) {
          topLevel.add(item);
        } else {
          String rootId = parentId;
          final visited = <String>{cid};
          while (visited.add(rootId)) {
            final parent = byId[rootId];
            final nextParentId = parent == null ? null : _parentIdOf(parent);
            if (nextParentId == null) break;
            rootId = nextParentId;
          }
          (replies[rootId] ??= <Map<String, dynamic>>[]).add(item);
        }

        final int? likeCount = _likeCountOf(item);
        if (likeCount != null) likeCounts[cid] = likeCount;
        if (_isLiked(item)) likedIds.add(cid);
      }

      if (!mounted) return;
      setState(() {
        _comments = topLevel;
        _repliesByParent
          ..clear()
          ..addAll(replies);
        _commentLikeCounts
          ..removeWhere((_, __) => true)
          ..addAll(likeCounts);
        _likedCommentIds
          ..clear()
          ..addAll(likedIds);
      });
    } catch (_) {
      if (!mounted) return;
      setState(() => _comments = <Map<String, dynamic>>[]);
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  Future<void> _ensureReplies(String parentId) async {
    if (_repliesLoading.contains(parentId)) return;
    _repliesLoading.add(parentId);
    try {
      final list = await ref
          .read(socialRepositoryProvider)
          .getCommentRepliesRaw(parentId);
      if (!mounted) return;
      setState(() {
        final existing = _repliesByParent[parentId] ?? <Map<String, dynamic>>[];
        final Map<String, Map<String, dynamic>> merged =
            <String, Map<String, dynamic>>{};
        for (final m in existing) {
          final id = _commentIdOf(m);
          if (id != null) merged[id] = m;
        }
        for (final raw in list) {
          final Map<String, dynamic> item = Map<String, dynamic>.from(raw);
          final String? cid = _commentIdOf(item);
          if (cid == null) continue;
          merged[cid] = item;
          final int? likeCount = _likeCountOf(item);
          if (likeCount != null) _commentLikeCounts[cid] = likeCount;
          if (_isLiked(item)) _likedCommentIds.add(cid);
        }
        _repliesByParent[parentId] = merged.values.toList();
      });
    } finally {
      _repliesLoading.remove(parentId);
    }
  }

  Future<void> _send() async {
    final text = _controller.text.trim();
    if (_sending || text.isEmpty) return;
    if (!await requireLogin(context, ref)) return;
    if (!mounted || _sending) return;
    setState(() => _sending = true);
    final String clientRequestId = _uuid.v4();
    final replyCommentId = _replyToCommentId;
    final replyTargetId = _replyToTargetId;
    try {
      final socialRepo = ref.read(socialRepositoryProvider);
      final bool isReply = replyCommentId != null;
      Map<String, dynamic>? created;
      if (isReply) {
        created = await socialRepo.replyToCommentRaw(
          replyCommentId,
          text,
          postId: widget.postId,
          replyToId: replyTargetId ?? replyCommentId,
          clientRequestId: clientRequestId,
        );
      } else {
        created = await socialRepo.addCommentRaw(
          widget.postId,
          text,
          clientRequestId: clientRequestId,
        );
      }
      if (created != null && mounted) {
        _controller.clear();
        final Map<String, dynamic> map = Map<String, dynamic>.from(created);
        map['clientRequestId'] ??= clientRequestId;
        map['isMine'] = true;
        final String myName = _myDisplayName ?? widget.postOwnerName ?? '我';
        map['userName'] ??= myName;
        map['username'] ??= myName;
        // 樂觀更新：寫入回應現在已與讀取端同形狀（都回 avatarKey），
        // 這裡只在後端沒帶到時補上自己的頭像。
        if (_inputAvatar != null && _inputAvatar!.isNotEmpty) {
          map['avatarKey'] ??= _inputAvatar;
        }
        if (isReply) {
          map['parentId'] ??= replyCommentId;
          map['replyToId'] ??= replyTargetId ?? replyCommentId;
        }
        final String? id = _commentIdOf(map);
        // SSE 可能比 POST 回應先送達 → 已在列表中就不重複 append
        final bool alreadyPresent = id != null && _hasComment(id);
        setState(() {
          if (!alreadyPresent) {
            if (isReply) {
              final parentId = replyCommentId;
              final replies =
                  _repliesByParent[parentId] ?? <Map<String, dynamic>>[];
              _repliesByParent[parentId] = [...replies, map];
            } else {
              _comments = [..._comments, map];
            }
            if (id != null) {
              final int? likeCount = _likeCountOf(map);
              if (likeCount != null) _commentLikeCounts[id] = likeCount;
              if (_isLiked(map)) _likedCommentIds.add(id);
            }
          }
          if (isReply) {
            _replyToCommentId = null;
            _replyToTargetId = null;
            _replyToName = null;
          }
        });
        if (!alreadyPresent) _scheduleScrollToBottom();
      } else if (mounted) {
        showCommonSnackBar(context, '傳送失敗，請稍後再試');
      }
    } catch (_) {
      if (mounted) showCommonSnackBar(context, '傳送失敗，請稍後再試');
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

  // ===========================================================================
  // SSE event handlers
  // ===========================================================================

  void _handleStreamEvent(String eventName, Map<String, dynamic> data) {
    if (!mounted) return;
    switch (eventName) {
      case 'comment.created':
      case 'comment.replied':
        _onCommentCreatedOrReplied(data);
        break;
      case 'comment.updated':
        _onCommentUpdated(data);
        break;
      case 'comment.deleted':
        _onCommentDeleted(data);
        break;
      case 'comment.like_changed':
        _onCommentLikeChanged(data);
        break;
      default:
        // 未識別事件忽略,避免漏接時誤動畫面
        break;
    }
  }

  void _onCommentCreatedOrReplied(Map<String, dynamic> data) {
    final raw = data['comment'];
    if (raw is! Map) return;
    final Map<String, dynamic> map = Map<String, dynamic>.from(raw);
    final String? clientRequestId =
        (data['clientRequestId'] ?? map['clientRequestId'])?.toString();
    if (clientRequestId != null && clientRequestId.isNotEmpty) {
      map['clientRequestId'] = clientRequestId;
    }
    final String? id = _commentIdOf(map);
    if (id == null) return;
    if (_hasComment(id)) return; // POST 回應或前一筆 SSE 已塞入
    if (clientRequestId != null &&
        clientRequestId.isNotEmpty &&
        _hasClientRequestId(clientRequestId)) {
      return;
    }
    final String? parentIdStr = _parentIdOf(map);
    final String? rootParentId = parentIdStr == null
        ? null
        : _rootThreadId(parentIdStr);
    setState(() {
      if (rootParentId == null) {
        _comments = [..._comments, map];
      } else {
        final list = _repliesByParent[rootParentId] ?? <Map<String, dynamic>>[];
        if (!list.any((c) => _commentIdOf(c) == id)) {
          _repliesByParent[rootParentId] = [...list, map];
        }
        // 把母留言的 replyCount +1(若該欄位存在於 _comments 中對應項)
        for (int i = 0; i < _comments.length; i++) {
          if (_commentIdOf(_comments[i]) == rootParentId) {
            final parent = Map<String, dynamic>.from(_comments[i]);
            final int prev = _replyCountOf(parent);
            parent['replyCount'] = prev + 1;
            final newList = [..._comments];
            newList[i] = parent;
            _comments = newList;
            break;
          }
        }
      }
      final int? likeCount = _likeCountOf(map);
      if (likeCount != null) _commentLikeCounts[id] = likeCount;
      if (_isLiked(map)) _likedCommentIds.add(id);
    });
    if (rootParentId == null) _scheduleScrollToBottom();
  }

  void _onCommentUpdated(Map<String, dynamic> data) {
    final String? id = (data['commentId'])?.toString();
    if (id == null || id.isEmpty) return;
    final String? content = data['content']?.toString();
    final dynamic updatedAt = data['updatedAt'];
    setState(() {
      _mutateCommentById(id, (m) {
        if (content != null) m['content'] = content;
        if (updatedAt != null) m['updatedAt'] = updatedAt;
      });
    });
  }

  void _onCommentDeleted(Map<String, dynamic> data) {
    final String? id = (data['commentId'])?.toString();
    if (id == null || id.isEmpty) return;
    final String? parentIdStr = _parentIdOf(data);
    final String? rootParentId = parentIdStr == null
        ? null
        : _rootThreadId(parentIdStr);
    final bool isTopLevel =
        rootParentId == null || rootParentId.isEmpty || rootParentId == '0';
    setState(() {
      if (isTopLevel) {
        _comments = _comments.where((c) => _commentIdOf(c) != id).toList();
        _repliesByParent.remove(id);
      } else {
        final list = _repliesByParent[rootParentId];
        if (list != null) {
          _repliesByParent[rootParentId] = list
              .where((c) => _commentIdOf(c) != id)
              .toList();
        }
        // 母留言 replyCount -1
        for (int i = 0; i < _comments.length; i++) {
          if (_commentIdOf(_comments[i]) == rootParentId) {
            final parent = Map<String, dynamic>.from(_comments[i]);
            final int prev = _replyCountOf(parent);
            parent['replyCount'] = prev > 0 ? prev - 1 : 0;
            final newList = [..._comments];
            newList[i] = parent;
            _comments = newList;
            break;
          }
        }
      }
      _commentLikeCounts.remove(id);
      _likedCommentIds.remove(id);
    });
  }

  void _onCommentLikeChanged(Map<String, dynamic> data) {
    final String? id = data['commentId']?.toString();
    if (id == null || id.isEmpty) return;
    final dynamic likeCountRaw = data['likeCount'];
    final int? likeCount = _toInt(likeCountRaw);
    final String action = (data['action'] ?? '').toString().toLowerCase();
    final bool isLikeAction = action == 'liked' || action == 'like';
    final bool isUnlikeAction = action == 'unliked' || action == 'unlike';
    final bool fromMe = data['likedByViewer'] == true;
    setState(() {
      if (likeCount != null) {
        _commentLikeCounts[id] = likeCount;
      }
      if (fromMe) {
        if (isLikeAction) {
          _likedCommentIds.add(id);
        } else if (isUnlikeAction) {
          _likedCommentIds.remove(id);
        }
      }
    });
  }

  String _rootThreadId(String parentId) {
    if (_comments.any((comment) => _commentIdOf(comment) == parentId)) {
      return parentId;
    }
    for (final entry in _repliesByParent.entries) {
      if (entry.value.any((reply) => _commentIdOf(reply) == parentId)) {
        return entry.key;
      }
    }
    return parentId;
  }

  bool _hasComment(String id) {
    for (final c in _comments) {
      if (_commentIdOf(c) == id) return true;
    }
    for (final list in _repliesByParent.values) {
      for (final c in list) {
        if (_commentIdOf(c) == id) return true;
      }
    }
    return false;
  }

  bool _hasClientRequestId(String reqId) {
    bool check(Iterable<Map<String, dynamic>> list) {
      for (final c in list) {
        final v = c['clientRequestId'];
        if (v != null && v.toString() == reqId) return true;
      }
      return false;
    }

    if (check(_comments)) return true;
    for (final list in _repliesByParent.values) {
      if (check(list)) return true;
    }
    return false;
  }

  /// 在 `_comments` 或 `_repliesByParent` 中找到對應 id 的留言並就地修改。
  void _mutateCommentById(
    String id,
    void Function(Map<String, dynamic>) apply,
  ) {
    for (int i = 0; i < _comments.length; i++) {
      if (_commentIdOf(_comments[i]) == id) {
        final updated = Map<String, dynamic>.from(_comments[i]);
        apply(updated);
        final newList = [..._comments];
        newList[i] = updated;
        _comments = newList;
        return;
      }
    }
    for (final parentId in _repliesByParent.keys) {
      final list = _repliesByParent[parentId]!;
      for (int i = 0; i < list.length; i++) {
        if (_commentIdOf(list[i]) == id) {
          final updated = Map<String, dynamic>.from(list[i]);
          apply(updated);
          final newList = [...list];
          newList[i] = updated;
          _repliesByParent[parentId] = newList;
          return;
        }
      }
    }
  }

  void _scheduleScrollToBottom({bool animated = true}) {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!_scrollController.hasClients) return;
      final double target = _scrollController.position.maxScrollExtent;
      if (animated) {
        _scrollController.animateTo(
          target,
          duration: const Duration(milliseconds: 250),
          curve: Curves.easeOut,
        );
      } else {
        _scrollController.jumpTo(target);
      }
    });
  }

  void _toggleLike(Map<String, dynamic> comment) async {
    if (!await requireLogin(context, ref)) return;
    if (!mounted) return;
    final String? id = _commentIdOf(comment);
    if (id == null) return;
    final bool wasLiked = _likedCommentIds.contains(id);
    final int currentCount =
        _commentLikeCounts[id] ?? _likeCountOf(comment) ?? 0;
    // Optimistic update
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
      // API 是 toggle: POST /app/posts/comments/{id}/like → { code, data: { liked } }
      final resp = await ref.read(socialRepositoryProvider).likeCommentRaw(id);
      if (!mounted) return;
      // 解析 { code, data: { liked } }
      final data = resp?['data'];
      final bool? serverLiked = data is Map ? data['liked'] : resp?['liked'];
      if (serverLiked != null) {
        setState(() {
          if (serverLiked) {
            _likedCommentIds.add(id);
          } else {
            _likedCommentIds.remove(id);
          }
        });
      }
    } catch (_) {
      // Revert on error
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

  @override
  Widget build(BuildContext context) {
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
                      : (_comments.isEmpty
                            ? const HkEmptyState(
                                key: ValueKey('comments-empty-state'),
                                iconKey: ValueKey('comments-empty-state-icon'),
                                icon: HkIconName.emptyChat,
                                message: '還沒有留言，來當第一個吧',
                              )
                            : ListView.separated(
                                controller: _scrollController,
                                padding: EdgeInsets.fromLTRB(0, 8.h, 0, 16.h),
                                itemBuilder: (context, index) =>
                                    _buildComment(_comments[index]),
                                separatorBuilder: (_, __) =>
                                    SizedBox(height: 8.h),
                                itemCount: _comments.length,
                              )),
                ),
                // Divider above input
                Divider(height: 1, color: AppColors.borderSubtle),
                // Input area
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
                                    hintText: _replyToName != null
                                        ? '寫下回覆…'
                                        : '參與討論…',
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

  Widget _buildComment(Map<String, dynamic> comment) {
    final String? commentId = _commentIdOf(comment);
    final String author = _userNameOf(comment);
    final String text = _commentTextOf(comment);
    final String? avatar = _pickAvatarFromMap(comment);
    final List<Map<String, dynamic>> replies = commentId != null
        ? (_repliesByParent[commentId] ?? const <Map<String, dynamic>>[])
        : const <Map<String, dynamic>>[];
    final int likeCount =
        _commentLikeCounts[commentId] ?? _likeCountOf(comment) ?? 0;
    final bool liked =
        commentId != null && _likedCommentIds.contains(commentId);
    final int replyCount = _replyCountOf(comment);

    return Padding(
      padding: EdgeInsets.symmetric(horizontal: 16.w, vertical: 4.h),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              // Avatar aligned to top
              Padding(
                padding: EdgeInsets.only(top: 4.h),
                child: _avatarCircle(avatar, size: 30),
              ),
              SizedBox(width: 12.w),
              // Name + content + reply
              Expanded(
                child: Column(
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
                      text,
                      style: TextStyle(
                        fontSize: 14,
                        fontWeight: FontWeight.w400,
                        color: AppColors.textPrimary,
                      ),
                    ),
                    SizedBox(height: 4.h),
                    HkTapTarget(
                      key: ValueKey('comment-reply-$commentId'),
                      semanticLabel: '回覆 $author 的留言',
                      hint: '開始撰寫回覆',
                      tooltip: '回覆',
                      onTap: _sending || commentId == null
                          ? null
                          : () => _beginReply(
                              rootCommentId: commentId,
                              targetCommentId: commentId,
                              targetName: author,
                            ),
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
                              fontSize: 14,
                              color: AppColors.textPrimary,
                            ),
                          ),
                        ],
                      ),
                    ),
                  ],
                ),
              ),
              // Like button + count
              Column(
                children: [
                  HkTapTarget(
                    key: ValueKey('comment-like-$commentId'),
                    semanticLabel: liked
                        ? '取消對 $author 留言的讚'
                        : '按讚 $author 的留言',
                    hint: '$likeCount 個讚',
                    tooltip: liked ? '取消讚' : '按讚',
                    toggled: liked,
                    onTap: commentId == null
                        ? null
                        : () => _toggleLike(comment),
                    child: Icon(
                      liked ? AppIcons.favorite : AppIcons.favoriteBorder,
                      color: liked
                          ? AppIconColors.favorite
                          : AppIconColors.defaultColor,
                      size: 24.sp,
                    ),
                  ),
                  SizedBox(height: 2.h),
                  Text(
                    _formatCount(likeCount),
                    style: TextStyle(
                      fontSize: 14,
                      color: AppColors.textPrimary,
                    ),
                    textAlign: TextAlign.center,
                  ),
                ],
              ),
            ],
          ),
          // Replies
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
                        (reply) =>
                            _buildReply(reply, rootCommentId: commentId!),
                      )
                      .toList(),
                ),
              ),
            ),
          // Load more replies
          if (commentId != null &&
              replyCount > 0 &&
              replies.length < replyCount)
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

  Widget _buildReply(
    Map<String, dynamic> reply, {
    required String rootCommentId,
  }) {
    final String? replyId = _commentIdOf(reply);
    final String author = _userNameOf(reply);
    final String text = _commentTextOf(reply);
    final String? avatar = _pickAvatarFromMap(reply);
    final int likeCount = replyId != null
        ? (_commentLikeCounts[replyId] ?? _likeCountOf(reply) ?? 0)
        : 0;
    final bool liked = replyId != null && _likedCommentIds.contains(replyId);

    return Padding(
      padding: EdgeInsets.symmetric(vertical: 8.h),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Padding(
            padding: EdgeInsets.only(top: 4.h),
            child: _avatarCircle(avatar, size: 28),
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
                if (text.isNotEmpty)
                  Text(
                    text,
                    style: const TextStyle(
                      fontSize: 14,
                      color: AppColors.textPrimary,
                      height: 20 / 14,
                    ),
                  ),
                SizedBox(height: 4.h),
                HkTapTarget(
                  key: ValueKey('comment-reply-$replyId'),
                  semanticLabel: '回覆 $author',
                  hint: '開始撰寫回覆',
                  tooltip: '回覆',
                  onTap: _sending || replyId == null
                      ? null
                      : () => _beginReply(
                          rootCommentId: rootCommentId,
                          targetCommentId: replyId,
                          targetName: author,
                        ),
                  child: Text(
                    '回覆',
                    style: TextStyle(
                      fontSize: 12,
                      fontWeight: FontWeight.w700,
                      color: AppColors.textSecondary,
                    ),
                  ),
                ),
              ],
            ),
          ),
          Column(
            children: [
              HkTapTarget(
                key: ValueKey('comment-like-$replyId'),
                semanticLabel: liked ? '取消對 $author 回覆的讚' : '按讚 $author 的回覆',
                hint: '$likeCount 個讚',
                tooltip: liked ? '取消讚' : '按讚',
                toggled: liked,
                onTap: replyId == null ? null : () => _toggleLike(reply),
                child: Icon(
                  liked ? AppIcons.favorite : AppIcons.favoriteBorder,
                  color: liked
                      ? AppIconColors.favorite
                      : AppIconColors.defaultColor,
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
          ),
        ],
      ),
    );
  }

  Widget _avatarCircle(String? avatar, {double size = 32}) {
    Widget buildPlaceholder() {
      return CircleAvatar(
        radius: size / 2,
        child: Icon(AppIcons.personOutline, size: size / 2),
      );
    }

    final String? effective = avatar?.trim();

    if (effective == null || effective.isEmpty) {
      return buildPlaceholder();
    }

    try {
      ImageProvider provider;
      if (_looksLikeBase64(effective)) {
        final bytes = base64Decode(
          effective.contains(',') ? effective.split(',').last : effective,
        );
        provider = MemoryImage(bytes);
      } else if (effective.startsWith('http://') ||
          effective.startsWith('https://')) {
        provider = NetworkImage(
          ImageConfig.urlFor(effective, ImageScene.avatarSmall),
        );
      } else if (effective.startsWith('/member/avatar/view') ||
          effective.startsWith('member/avatar/view') ||
          effective.startsWith('/member-avatars') ||
          effective.startsWith('member-avatars') ||
          effective.startsWith('avatars/')) {
        provider = NetworkImage(
          ImageConfig.urlFor(
            effective.normalizeAvatarUrl(),
            ImageScene.avatarSmall,
          ),
        );
      } else if (effective.startsWith('/')) {
        provider = NetworkImage(
          ImageConfig.urlFor(_toAbsolute(effective), ImageScene.avatarSmall),
        );
      } else if (_looksLikeObjectKey(effective)) {
        // 49 號成對契約（S4）：SSE 留言 avatarKey = 裸 R2 objectKey（如
        // public/social_comment/x.jpg），過 buildImageUrl dual-mode 組頭像 URL。
        provider = NetworkImage(
          ImageConfig.urlFor(effective, ImageScene.avatarSmall),
        );
      } else {
        final bytes = base64Decode(effective);
        provider = MemoryImage(bytes);
      }
      return CircleAvatar(radius: size / 2, backgroundImage: provider);
    } catch (_) {
      return buildPlaceholder();
    }
  }

  String _formatCount(int count) {
    if (count >= 1000000) return '${(count / 1000000).toStringAsFixed(1)}M';
    if (count >= 1000) return '${(count / 1000).toStringAsFixed(1)}K';
    return count.toString();
  }

  /// 留言者顯示名。REST（`/public/posts/{id}/comments`）與 SSE
  /// `comment.created` 收斂後同名 `userName`，沒有第二個來源。
  String _userNameOf(Map<String, dynamic> comment) {
    final value = comment['userName'];
    if (value is String && value.trim().isNotEmpty) return value.trim();
    if (comment['isMine'] == true && _myDisplayName != null) {
      return _myDisplayName!;
    }
    return widget.postOwnerName ?? _myDisplayName ?? 'User';
  }

  String _commentTextOf(Map<String, dynamic> comment) {
    for (final key in const ['content', 'text', 'body', 'message']) {
      final value = comment[key];
      if (value is String && value.isNotEmpty) return value;
    }
    return '';
  }

  /// 留言 ID。REST 回 `id`，SSE `comment.created` 回 `commentId`。
  String? _commentIdOf(Map<String, dynamic> comment) {
    for (final key in const ['id', 'commentId']) {
      final value = comment[key];
      if (value == null) continue;
      final String s = value.toString();
      if (s.trim().isNotEmpty) return s.trim();
    }
    return null;
  }

  /// 父留言 ID。REST 與 SSE 皆為 `parentId`；`replyToId` 是二層回覆的被回覆者，
  /// 只在 `parentId` 缺值時當退路（頂層留言兩者皆為 "0"）。
  String? _parentIdOf(Map<String, dynamic> comment) {
    for (final key in const ['parentId', 'replyToId']) {
      final value = comment[key];
      if (value == null) continue;
      final String s = value.toString().trim();
      if (s.isEmpty || s == '0' || s.toLowerCase() == 'null') continue;
      return s;
    }
    return null;
  }

  int? _likeCountOf(Map<String, dynamic> comment) {
    return _toInt(comment['likeCount']);
  }

  int _replyCountOf(Map<String, dynamic> comment) {
    final dynamic value = comment['replyCount'] ?? comment['replies'];
    if (value is List) return value.length;
    final int? parsed = _toInt(value);
    return parsed ?? 0;
  }

  int? _toInt(dynamic value) {
    if (value is int) return value;
    if (value is double) return value.toInt();
    if (value is num) return value.toInt();
    if (value is String) {
      final int? parsed = int.tryParse(value);
      if (parsed != null) return parsed;
    }
    return null;
  }

  bool _isLiked(Map<String, dynamic> comment) {
    final liked = comment['isLiked'];
    if (liked is bool) return liked;
    if (liked is int) return liked == 1;
    if (liked is String) return liked.toLowerCase() == 'true' || liked == '1';
    return false;
  }

  /// 取頭像。三個來源在 49 §6.6 收斂後同名：REST 留言列表／寫入回應與
  /// GET /app/buyers/me 皆回成對的 `avatarKey`＋`avatarUrl`，SSE
  /// `comment.created` 只回 `avatarKey`（S4，不經 Gateway 組裝）。
  /// 一律以 `avatarKey`（裸 objectKey）為準，顯示時過 buildImageUrl。
  String? _pickAvatarFromMap(Map<String, dynamic>? map) =>
      resolveAvatarKey(map);

  bool _looksLikeBase64(String value) {
    if (value.startsWith('data:image/')) return true;
    if (value.length < 60) return false;
    final RegExp base64Pattern = RegExp(r'^[A-Za-z0-9+/=]+$');
    return base64Pattern.hasMatch(value);
  }

  /// 判斷是否為 R2 媒體 objectKey（相對路徑帶副檔名，如
  /// `public/social_comment/uuid.jpg`）。base64 字元集不含 `.`，故有副檔名即可
  /// 安全區分於裸 base64 字串。
  bool _looksLikeObjectKey(String value) {
    if (!value.contains('/')) return false;
    final segment = value.split('/').last;
    return segment.contains('.');
  }

  String _toAbsolute(String path) {
    final Uri base = Uri.parse(F.appFlavor.apiBaseUrl);
    final String origin =
        '${base.scheme}://${base.host}${base.hasPort ? ':${base.port}' : ''}';
    if (path.startsWith('//')) return '${base.scheme}:$path';
    if (path.startsWith('/')) return '$origin$path';
    return '$origin/$path';
  }
}
