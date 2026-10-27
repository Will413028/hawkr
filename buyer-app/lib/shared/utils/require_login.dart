import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../core/session/session_provider.dart';
import '../widgets/common_dialog.dart';

/// Apple App Review 合規 — guest mode 下，帳號相關操作的統一登入閘門。
///
/// 用法：
/// ```dart
/// onTap: () async {
///   if (!await requireLogin(context, ref)) return;
///   await _addToCart();
/// }
/// ```
///
/// 已登入：直接回 true 並執行原操作。
/// 未登入：跳出確認 dialog；使用者按「前往登入」才導向 `/login?continue=<currentPath>`，
/// 登入成功 router 會自動帶回原頁面。按「取消」回 false，原操作取消。
Future<bool> requireLogin(BuildContext context, WidgetRef ref) async {
  final session = ref.read(sessionProvider).value;
  if (hasUsableSession(session)) return true;

  final router = GoRouter.of(context);
  final currentPath = router.routerDelegate.currentConfiguration.uri.toString();
  final currentRoute = ModalRoute.of(context);

  final goLogin = await showLoginRequiredDialog(context);
  if (!context.mounted) return false;

  if (goLogin == true && currentRoute is PopupRoute) {
    final nav = Navigator.of(context);
    if (nav.canPop()) nav.pop();
  }

  if (goLogin != true) return false;

  final encoded = Uri.encodeComponent(currentPath);
  router.push('/login?continue=$encoded');
  return false;
}

/// 訪客與失效 session（強制登出）共用的登入提示，確保文案、按鈕與 dismiss 行為一致。
Future<bool?> showLoginRequiredDialog(BuildContext context, {String? message}) {
  return showCommonDialog(
    context: context,
    title: '需要登入',
    content: message ?? '此操作需要登入後才能繼續，是否前往登入？',
    confirmText: '前往登入',
    cancelText: '取消',
    barrierDismissible: true,
  );
}
