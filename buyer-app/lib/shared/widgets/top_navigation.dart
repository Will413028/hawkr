import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';
import 'package:flutter/material.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../core/theme/hk_tokens.dart';
import '../../features/notification/presentation/notification_controller.dart';
import '../utils/require_login.dart';
import 'notification_count_badge.dart';

/// 通知鈴鐺 — 帶未讀 badge 的獨立 widget。
class NotificationBellButton extends ConsumerWidget {
  const NotificationBellButton({super.key});

  Future<void> _openNotifications(BuildContext context, WidgetRef ref) async {
    if (!await requireLogin(context, ref)) return;
    if (!context.mounted) return;
    await context.push('/notifications');
    ref.invalidate(notificationUnreadCountProvider);
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final count = ref.watch(notificationUnreadCountProvider).value ?? 0;
    return Semantics(
      label: count > 0 ? '通知，$count 則未讀' : '通知',
      button: true,
      onTap: () => _openNotifications(context, ref),
      child: ExcludeSemantics(
        child: GestureDetector(
          onTap: () => _openNotifications(context, ref),
          behavior: HitTestBehavior.opaque,
          child: SizedBox.square(
            dimension: HkControl.minTap,
            child: Stack(
              clipBehavior: Clip.none,
              alignment: Alignment.center,
              children: [
                HkIcon(
                  HkIconName.notification,
                  key: const ValueKey('topbar-notification-icon'),
                  size: HkControl.icon,
                ),
                if (count > 0)
                  Positioned(
                    right: 3,
                    top: 3,
                    child: NotificationCountBadge(count: count),
                  ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
