import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/session/session_provider.dart';
import '../presentation/notification_controller.dart';
import 'notification_repository.dart';
import 'notification_signals.dart';
import 'state_refresh.dart';

part 'notification_signals_provider.g.dart';

/// 登入中（session 還原或登入完成）連上通知串流，登出或換帳號就斷線重連。
@Riverpod(keepAlive: true)
NotificationSignals notificationSignals(Ref ref) {
  final signals = NotificationSignals(
    open: () => ref.read(notificationRepositoryProvider).watch(),
    onInbox: (_) =>
        ref.read(notificationListControllerProvider.notifier).silentRefresh(),
    onHints: dispatchStateRefreshHints,
    onResync: () {
      ref.read(notificationListControllerProvider.notifier).silentRefresh();
      dispatchFullStateRefresh();
    },
  );
  ref.onDispose(signals.stop);
  ref.listen(sessionProvider, (prev, next) {
    final was = prev?.value;
    final now = next.value;
    if (hasUsableSession(was) &&
        hasUsableSession(now) &&
        was!.userId != now!.userId) {
      signals.stop();
    }
    hasUsableSession(now) ? signals.start() : signals.stop();
  }, fireImmediately: true);
  return signals;
}
