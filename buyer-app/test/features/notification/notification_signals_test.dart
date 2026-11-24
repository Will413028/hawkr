import 'dart:async';
import 'dart:math' as math;

import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/notification/v1alpha1/notification.pb.dart'
    as pb;
import 'package:hawkr_api/gen/hawkr/notification/v1alpha1/notification.pbenum.dart';
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:hawkr_buyer/features/notification/data/notification_signals.dart';
import 'package:hawkr_buyer/features/notification/data/state_refresh.dart';
import 'package:hawkr_buyer/features/order/data/after_sale_status_notifier.dart';
import 'package:hawkr_buyer/features/order/data/order_status_notifier.dart';
import 'package:hawkr_buyer/features/order/data/refresh_coalescer.dart';

pb.WatchNotificationsResponse _inbox({
  int unread = 1,
  List<StateRefreshHint> hints = const [],
}) => pb.WatchNotificationsResponse(
  inbox: pb.InboxSignal(unreadCount: unread, stateRefreshHints: hints),
);

class _Harness {
  _Harness() {
    signals = NotificationSignals(
      open: () {
        final controller = StreamController<pb.WatchNotificationsResponse>(
          onCancel: () => cancelled++,
        );
        streams.add(controller);
        return controller.stream;
      },
      onInbox: inboxSignals.add,
      onHints: (hints) {
        hintBatches.add(hints);
        dispatchStateRefreshHints(hints);
      },
      onResync: () {
        resyncs++;
        dispatchFullStateRefresh();
      },
      random: math.Random(1),
      wait: (delay) {
        final completer = Completer<void>();
        waits.add(completer);
        return completer.future;
      },
    );
  }

  late final NotificationSignals signals;
  final streams = <StreamController<pb.WatchNotificationsResponse>>[];
  final inboxSignals = <pb.InboxSignal>[];
  final hintBatches = <List<StateRefreshHint>>[];
  final waits = <Completer<void>>[];
  var resyncs = 0;
  var cancelled = 0;
}

void main() {
  late _Harness h;
  setUp(() {
    OrderStatusNotifier.instance.coalesceWindow = Duration.zero;
    AfterSaleStatusNotifier.instance.coalesceWindow = Duration.zero;
    addTearDown(() {
      OrderStatusNotifier.instance.coalesceWindow =
          RefreshCoalescer.defaultWindow;
      AfterSaleStatusNotifier.instance.coalesceWindow =
          RefreshCoalescer.defaultWindow;
    });
  });

  late List<OrderStatusEvent> orderEvents;
  late List<AfterSaleStatusEvent> afterSaleEvents;

  setUp(() {
    h = _Harness();
    orderEvents = [];
    afterSaleEvents = [];
    final a = OrderStatusNotifier.instance.eventStream.listen(orderEvents.add);
    final b = AfterSaleStatusNotifier.instance.eventStream.listen(
      afterSaleEvents.add,
    );
    addTearDown(() async {
      h.signals.stop();
      await a.cancel();
      await b.cancel();
    });
  });

  test('inbox 訊號通知收件匣；ORDER 與 AFTER_SALE 提示各叫對應的通知器', () async {
    h.signals.start();
    await pumpEventQueue();

    h.streams.single.add(_inbox());
    await pumpEventQueue();
    expect(h.inboxSignals, hasLength(1));
    expect(orderEvents, isEmpty);
    expect(afterSaleEvents, isEmpty);

    h.streams.single.add(
      _inbox(hints: [StateRefreshHint.STATE_REFRESH_HINT_ORDER]),
    );
    await pumpEventQueue();
    expect(orderEvents, hasLength(1));
    expect(afterSaleEvents, isEmpty);

    h.streams.single.add(
      _inbox(
        hints: [
          StateRefreshHint.STATE_REFRESH_HINT_AFTER_SALE,
          StateRefreshHint.STATE_REFRESH_HINT_SELLER_STATUS,
        ],
      ),
    );
    await pumpEventQueue();
    expect(orderEvents, hasLength(1));
    expect(afterSaleEvents, hasLength(1));
    expect(h.inboxSignals, hasLength(3));
  });

  test('斷線後等 backoff 再重連，重連前全部重讀；第一次連線不重讀', () async {
    h.signals.start();
    await pumpEventQueue();
    expect(h.resyncs, 0);

    h.streams.single.addError(ConnectException(Code.unavailable, 'down'));
    await pumpEventQueue();
    expect(h.streams, hasLength(1));
    expect(h.waits, hasLength(1));

    h.waits.single.complete();
    await pumpEventQueue();
    expect(h.streams, hasLength(2));
    expect(h.resyncs, 1);
    expect(orderEvents, hasLength(1));
    expect(afterSaleEvents, hasLength(1));

    // 伺服器正常結束也是重連。
    await h.streams.last.close();
    await pumpEventQueue();
    expect(h.waits, hasLength(2));
    h.waits.last.complete();
    await pumpEventQueue();
    expect(h.streams, hasLength(3));
    expect(h.resyncs, 2);
  });

  test('permission_denied 與 unauthenticated 停止，不重連', () async {
    for (final code in [Code.permissionDenied, Code.unauthenticated]) {
      final local = _Harness();
      addTearDown(local.signals.stop);
      local.signals.start();
      await pumpEventQueue();

      local.streams.single.addError(ConnectException(code, 'no'));
      await pumpEventQueue();

      expect(local.signals.isRunning, isFalse, reason: code.name);
      expect(local.waits, isEmpty, reason: code.name);
      expect(local.streams, hasLength(1), reason: code.name);
      expect(local.resyncs, 0, reason: code.name);
    }
  });

  test('重複 start 不開第二條串流；stop 取消串流，再 start 重新連線', () async {
    h.signals.start();
    h.signals.start();
    await pumpEventQueue();
    h.signals.start();
    expect(h.streams, hasLength(1));

    h.signals.stop();
    await pumpEventQueue();
    expect(h.cancelled, 1);
    expect(h.signals.isRunning, isFalse);

    h.signals.start();
    await pumpEventQueue();
    expect(h.streams, hasLength(2));
    // 停掉之前的串流不會再送訊號。
    h.streams.first.add(_inbox());
    await pumpEventQueue();
    expect(h.inboxSignals, isEmpty);
  });

  test('backoff 等待中 stop，之後不會再開串流', () async {
    h.signals.start();
    await pumpEventQueue();
    h.streams.single.addError(ConnectException(Code.unavailable, 'down'));
    await pumpEventQueue();

    h.signals.stop();
    h.waits.single.complete();
    await pumpEventQueue();

    expect(h.streams, hasLength(1));
  });

  test('重連的等待時間有上限 30 秒', () {
    final random = math.Random(3);
    for (var attempt = 0; attempt < 20; attempt++) {
      expect(
        notificationReconnectDelay(attempt, random),
        lessThanOrEqualTo(const Duration(seconds: 30)),
      );
    }
  });
}
