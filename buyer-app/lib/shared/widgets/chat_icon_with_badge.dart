import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'dart:async';
import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import '../../features/chat/data/chat_realtime.dart';
import '../../features/chat/data/chat_repository.dart';
import '../../core/session/session_provider.dart';
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
  int _epoch = 0;
  StreamSubscription<int>? _unreadSub;

  @override
  void initState() {
    super.initState();
    _attach();
    // 訪客登入之後才連：訪客沒有聊天。
    ref.listenManual(sessionProvider, (prev, next) {
      if ((prev?.value?.userId ?? '') != (next.value?.userId ?? '')) {
        _onSessionChanged();
      }
    });
  }

  void _onSessionChanged() {
    // 登出或換帳號：上一個帳號的數字不留，換上新帳號時重讀。
    _epoch++;
    _loading = false;
    unawaited(_unreadSub?.cancel());
    _unreadSub = null;
    if (mounted) setState(() => _unread = 0);
    _attach();
  }

  bool get _signedIn => hasUsableSession(ref.read(sessionProvider).value);

  void _attach() {
    if (_unreadSub != null || !_signedIn) return;
    _load();
    // 串流的 Ready 與 Unread 事件帶總未讀數（權威值，一律 SET）；串流重連後的 Ready 也補上斷線期間漏掉的變化。
    _unreadSub = ref.read(chatRealtimeProvider).unreadCount.listen((n) {
      if (!mounted) return;
      setState(() => _unread = n);
    });
  }

  Future<void> _load() async {
    if (_loading) return;
    // 訪客沒有聊天：不呼叫（登入後的 service 會把訪客的呼叫當成 session 失效）。
    if (!_signedIn) return;
    _loading = true;
    final epoch = _epoch;
    try {
      final n = await ref.read(chatRepositoryProvider).getUnreadCount();
      if (!mounted || epoch != _epoch) return;
      setState(() => _unread = n);
    } catch (_) {
      // 讀取失敗時保留目前的數字；串流連上之後會以 Ready 的值為準。
    } finally {
      if (epoch == _epoch) _loading = false;
    }
  }

  @override
  void dispose() {
    _unreadSub?.cancel();
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
