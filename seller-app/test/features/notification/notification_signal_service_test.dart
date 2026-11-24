import 'dart:async';

import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/notification/v1alpha1/notification.pb.dart';
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:hawkr_seller/features/notification/data/notification_signal_service.dart';

Future<void> _settle() async {
  for (var i = 0; i < 10; i++) {
    await Future<void>.delayed(Duration.zero);
  }
}

void main() {
  late List<StreamController<WatchNotificationsResponse>> streams;
  late List<String> events;

  NotificationSignalService make() {
    streams = [];
    events = [];
    return NotificationSignalService(
      open: () {
        final c = StreamController<WatchNotificationsResponse>();
        streams.add(c);
        return c.stream;
      },
      onInbox: (s) => events.add(
        'inbox:${s.unreadCount}:${s.stateRefreshHints.map((h) => h.name).join(',')}',
      ),
      onWorkspace: (c) => events.add('workspace:${c.name}'),
      onResync: () => events.add('resync'),
      delay: (_) async {},
    );
  }

  test(
    'signals are forwarded; a dropped stream reconnects and resyncs',
    () async {
      final service = make()..start();
      await _settle();
      streams[0].add(
        WatchNotificationsResponse(
          inbox: InboxSignal(
            unreadCount: 3,
            stateRefreshHints: [StateRefreshHint.STATE_REFRESH_HINT_ORDER],
          ),
        ),
      );
      streams[0].add(
        WatchNotificationsResponse(
          sellerWorkspace: SellerWorkspaceSignal(
            change: SellerWorkspaceChange.SELLER_WORKSPACE_CHANGE_AFTER_SALE,
          ),
        ),
      );
      await _settle();
      expect(events, [
        'inbox:3:STATE_REFRESH_HINT_ORDER',
        'workspace:SELLER_WORKSPACE_CHANGE_AFTER_SALE',
      ]);

      streams[0].addError(ConnectException(Code.unavailable, 'drop'));
      await _settle();
      expect(streams, hasLength(2), reason: '斷線後重連');
      expect(events.last, 'resync', reason: '重連後先全部重讀一次');
      service.stop();
    },
  );

  test('unauthenticated stops the stream without reconnecting', () async {
    final service = make()..start();
    await _settle();
    streams[0].addError(ConnectException(Code.unauthenticated, 'x'));
    await _settle();
    expect(streams, hasLength(1));
    expect(service.isRunning, isFalse);
  });

  test('stop cancels the stream and does not reconnect', () async {
    final service = make()..start();
    await _settle();
    service.stop();
    await _settle();
    expect(streams.single.hasListener, isFalse);
    expect(streams, hasLength(1));
  });
}
