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

  late List<Duration> delays;

  NotificationSignalService make() {
    streams = [];
    events = [];
    delays = [];
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
      delay: (d) async => delays.add(d),
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
        'resync',
        'inbox:3:STATE_REFRESH_HINT_ORDER',
        'workspace:SELLER_WORKSPACE_CHANGE_AFTER_SALE',
      ]);

      streams[0].addError(ConnectException(Code.unavailable, 'drop'));
      await _settle();
      expect(streams, hasLength(2), reason: '斷線後重連');
      expect(events.last, 'resync', reason: '重連後先全部重讀一次');
      expect(events.where((e) => e == 'resync'), hasLength(2));
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

  test('the first connect resyncs before any signal arrives', () async {
    final service = make()..start();
    await _settle();
    expect(events, ['resync']);
    service.stop();
  });

  test('repeated start keeps a single stream', () async {
    final service = make()
      ..start()
      ..start();
    await _settle();
    service.start();
    await _settle();
    expect(streams, hasLength(1));
    service.stop();
  });

  test('permission_denied stops the stream without reconnecting', () async {
    final service = make()..start();
    await _settle();
    streams[0].addError(ConnectException(Code.permissionDenied, 'x'));
    await _settle();
    expect(streams, hasLength(1));
    expect(service.isRunning, isFalse);
    expect(delays, isEmpty);
  });

  test('not_found and resource_exhausted wait at least slowBackoff', () async {
    final service = make()..start();
    await _settle();
    final codes = [Code.notFound, Code.resourceExhausted, Code.notFound];
    for (var i = 0; i < codes.length; i++) {
      streams[i].addError(ConnectException(codes[i], 'slow'));
      await _settle();
    }
    expect(delays, hasLength(3));
    for (final d in delays) {
      expect(d, greaterThanOrEqualTo(const Duration(minutes: 1)));
      expect(d, lessThan(const Duration(seconds: 75)));
    }
    service.stop();
  });

  test(
    'other errors back off exponentially within [base, min(max, 2^n)]',
    () async {
      final service = make()..start();
      await _settle();
      for (var i = 0; i < 7; i++) {
        streams[i].addError(ConnectException(Code.unavailable, 'drop'));
        await _settle();
      }
      expect(delays, hasLength(7));
      const ceilings = [1, 2, 4, 8, 16, 30, 30];
      for (var i = 0; i < delays.length; i++) {
        expect(delays[i], greaterThanOrEqualTo(const Duration(seconds: 1)));
        expect(delays[i], lessThanOrEqualTo(Duration(seconds: ceilings[i])));
      }
      expect(delays[0], const Duration(seconds: 1));
      service.stop();
    },
  );

  test('a stream that ends without any signal counts as a failure', () async {
    final service = make()..start();
    await _settle();
    await streams[0].close();
    await _settle();
    expect(delays, hasLength(1));
    expect(streams, hasLength(2));
    service.stop();
  });
}
