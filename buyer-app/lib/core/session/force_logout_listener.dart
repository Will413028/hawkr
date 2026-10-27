import 'package:flutter/material.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../shared/utils/require_login.dart';
import 'force_logout_provider.dart';
import 'session_provider.dart';

/// 監聽 [forceLogoutProvider]，當 session 失效時
/// 將失效 session 降級為訪客，並顯示與未登入完全相同的登入提示。
///
/// 放在 MaterialApp 的 `builder` 內，並透過 root navigator key 取得 Overlay。
class ForceLogoutListener extends ConsumerStatefulWidget {
  const ForceLogoutListener({
    super.key,
    required this.child,
    required this.navigatorKey,
    required this.currentPath,
    required this.onLoginRequired,
  });

  final Widget child;
  final GlobalKey<NavigatorState> navigatorKey;
  final String Function() currentPath;
  final ValueChanged<String> onLoginRequired;

  @override
  ConsumerState<ForceLogoutListener> createState() =>
      _ForceLogoutListenerState();
}

class _ForceLogoutListenerState extends ConsumerState<ForceLogoutListener> {
  bool _dialogShowing = false;

  @override
  Widget build(BuildContext context) {
    final forceLogoutPending = ref.watch(forceLogoutProvider);
    if (forceLogoutPending && !_dialogShowing) {
      _dialogShowing = true;
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (mounted) _showDialog();
      });
    }
    return widget.child;
  }

  Future<void> _showDialog() async {
    // MaterialApp.builder 位於 Router / Navigator 上方，不能用本 widget 的
    // context 呼叫 showDialog。改從 root Navigator 的 Overlay 取得有效 context。
    await Future<void>.delayed(Duration.zero);
    if (!mounted) {
      _dialogShowing = false;
      return;
    }

    final dialogContext = widget.navigatorKey.currentState?.overlay?.context;
    if (dialogContext == null || !dialogContext.mounted) {
      _dialogShowing = false;
      // Router 尚未完成第一幀時保留 force-logout 旗標，下一幀再嘗試。
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (mounted && ref.read(forceLogoutProvider)) _showDialog();
      });
      WidgetsBinding.instance.scheduleFrame();
      return;
    }

    final continuePath = widget.currentPath();
    final message = ref.read(forceLogoutProvider.notifier).message;
    final goLogin = await showLoginRequiredDialog(
      dialogContext,
      message: message,
    );
    if (!mounted) return;
    // 不論取消或前往登入，失效憑證都必須清除，後續行為與訪客一致。
    await ref.read(sessionProvider.notifier).logout();
    if (!mounted) return;
    ref.read(forceLogoutProvider.notifier).consume();
    _dialogShowing = false;
    if (mounted && goLogin == true) {
      widget.onLoginRequired(continuePath);
    }
  }
}
