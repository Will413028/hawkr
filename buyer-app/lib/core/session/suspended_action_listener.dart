import 'package:flutter/material.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../core/theme/app_typography.dart';
import '../../core/theme/hk_tokens.dart';
import '../../shared/widgets/hk_button_label.dart';
import 'account_restriction.dart';

/// 使用者主動的寫入收到 `ACCOUNT_SUSPENDED` 時，顯示 bottom sheet「帳號停權中，無法執行此動作」，
/// 「查看詳情」開受限狀態頁。放在 MaterialApp 的 `builder` 內，從 root navigator 取得 Overlay。
class SuspendedActionListener extends ConsumerStatefulWidget {
  const SuspendedActionListener({
    super.key,
    required this.child,
    required this.navigatorKey,
    required this.onViewDetails,
  });

  static const message = '帳號停權中，無法執行此動作';

  final Widget child;
  final GlobalKey<NavigatorState> navigatorKey;
  final VoidCallback onViewDetails;

  @override
  ConsumerState<SuspendedActionListener> createState() =>
      _SuspendedActionListenerState();
}

class _SuspendedActionListenerState
    extends ConsumerState<SuspendedActionListener> {
  bool _showing = false;

  @override
  Widget build(BuildContext context) {
    ref.listen(suspendedActionNoticeProvider, (_, _) {
      if (_showing) return;
      _showing = true;
      WidgetsBinding.instance.addPostFrameCallback((_) => _show());
    });
    return widget.child;
  }

  Future<void> _show() async {
    final context = widget.navigatorKey.currentState?.overlay?.context;
    if (!mounted || context == null || !context.mounted) {
      _showing = false;
      return;
    }
    final viewDetails = await showModalBottomSheet<bool>(
      context: context,
      showDragHandle: true,
      builder: (sheet) => SafeArea(
        child: Padding(
          padding: const EdgeInsets.fromLTRB(
            HkSpace.lg,
            0,
            HkSpace.lg,
            HkSpace.lg,
          ),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Text(
                SuspendedActionListener.message,
                style: AppTypography.sectionTitle,
              ),
              const SizedBox(height: HkSpace.lg),
              FilledButton(
                key: const ValueKey('suspended-action-view-details'),
                onPressed: () => Navigator.of(sheet).pop(true),
                child: const HkButtonLabel('查看詳情'),
              ),
            ],
          ),
        ),
      ),
    );
    _showing = false;
    if (viewDetails == true && mounted) widget.onViewDetails();
  }
}
