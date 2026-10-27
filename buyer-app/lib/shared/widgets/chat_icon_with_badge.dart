import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'dart:async';
import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import '../../features/chat/data/chat_repository.dart';
import '../utils/require_login.dart';
import '../widgets/hk_icon.dart';
import 'notification_count_badge.dart';

class ChatIconWithBadge extends ConsumerStatefulWidget {
  const ChatIconWithBadge({
    super.key,
    this.iconSize = 24.0,
    this.badgeColor = AppColors.danger,
    this.badgeTextColor = AppColors.bgCard,
    this.compact = false,
  });

  final double iconSize;
  final Color badgeColor;
  final Color badgeTextColor;

  /// 緊湊模式：移除 IconButton 的預設 padding / min size，視覺寬度 = iconSize
  final bool compact;

  @override
  ConsumerState<ChatIconWithBadge> createState() => _ChatIconWithBadgeState();
}

class _ChatIconWithBadgeState extends ConsumerState<ChatIconWithBadge> {
  int _unread = 0;
  bool _loading = false;
  StreamSubscription<int>? _unreadSub;
  StreamSubscription<ChatConnectionState>? _stateSub;

  @override
  void initState() {
    super.initState();
    _load();
    // 後端推送的權威未讀數:一律 SET(取代每則訊息 REST refetch)
    _unreadSub = ref.read(chatRepositoryProvider).onUnreadCountChange.listen((
      n,
    ) {
      if (!mounted) return;
      setState(() => _unread = n);
    });
    // 重連 resync:補抓斷線期間漏掉的 unread_changed
    _stateSub = ref.read(chatRepositoryProvider).connectionStateStream.listen((
      s,
    ) {
      if (s == ChatConnectionState.connected && mounted) _load();
    });
  }

  Future<void> _load() async {
    if (_loading) return;
    _loading = true;
    try {
      final n = await ref.read(chatRepositoryProvider).getUnreadCount();
      if (!mounted) return;
      setState(() => _unread = n);
    } catch (_) {
      if (!mounted) return;
      setState(() => _unread = 0);
    } finally {
      _loading = false;
    }
  }

  @override
  void dispose() {
    _unreadSub?.cancel();
    _stateSub?.cancel();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final show = _unread > 0;
    final iconChild = HkIcon(
      HkIconName.chat,
      key: const ValueKey('topbar-chat-icon'),
      size: widget.iconSize,
    );

    if (widget.compact) {
      return Semantics(
        label: show ? '訊息，$_unread 則未讀' : '訊息',
        button: true,
        onTap: _handleTap,
        child: ExcludeSemantics(
          child: GestureDetector(
            behavior: HitTestBehavior.opaque,
            onTap: _handleTap,
            child: SizedBox.square(
              dimension: 48,
              child: Stack(
                clipBehavior: Clip.none,
                alignment: Alignment.center,
                children: [
                  iconChild,
                  if (show)
                    Positioned(
                      right: 3,
                      top: 3,
                      child: NotificationCountBadge(count: _unread),
                    ),
                ],
              ),
            ),
          ),
        ),
      );
    }

    return Stack(
      clipBehavior: Clip.none,
      children: [
        IconButton(tooltip: '訊息', icon: iconChild, onPressed: _handleTap),
        if (show)
          Positioned(
            right: 4,
            top: 4,
            child: GestureDetector(
              behavior: HitTestBehavior.opaque,
              onTap: _handleTap,
              child: NotificationCountBadge(count: _unread),
            ),
          ),
      ],
    );
  }

  Future<void> _handleTap() async {
    await _goToChat();
  }

  Future<void> _goToChat() async {
    if (!await requireLogin(context, ref)) return;
    if (!mounted) return;
    await context.push('/chat');
    if (mounted) _load();
  }
}
