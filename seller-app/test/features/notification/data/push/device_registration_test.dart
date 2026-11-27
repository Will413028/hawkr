import 'dart:async';

import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/notification/v1alpha1/notification.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/notification/v1alpha1/notification.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/notification/v1alpha1/notification.pb.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:hawkr_seller/features/notification/data/push/device_registrar.dart';

NotificationDeviceRegistrar _registrar(FakeTransportBuilder t) =>
    NotificationDeviceRegistrar(NotificationAppServiceClient(t.build()));

void main() {
  group('NotificationDeviceRegistrar', () {
    test(
      'register sends the token and SELLER_APP, no platform field',
      () async {
        final requests = <RegisterDeviceTokenRequest>[];
        final registrar = _registrar(
          FakeTransportBuilder().unary(
            NotificationAppService.registerDeviceToken,
            (r, _) {
              requests.add(r);
              return RegisterDeviceTokenResponse();
            },
          ),
        );

        final outcome = await registrar.registerDevice(
          fcmToken: 'device-token',
        );

        expect(outcome.isSuccess, isTrue);
        expect(requests.single.token, 'device-token');
        expect(
          requests.single.app,
          NotificationApp.NOTIFICATION_APP_SELLER_APP,
        );
        expect(
          requests.single.info_.fieldInfo.values.map((f) => f.name),
          isNot(contains('platform')),
        );
      },
    );

    test('remove sends DeleteDeviceToken with the token', () async {
      final requests = <DeleteDeviceTokenRequest>[];
      final registrar = _registrar(
        FakeTransportBuilder().unary(NotificationAppService.deleteDeviceToken, (
          r,
          _,
        ) {
          requests.add(r);
          return DeleteDeviceTokenResponse();
        }),
      );

      final outcome = await registrar.removeDevice(fcmToken: 'device-token');

      expect(outcome.isSuccess, isTrue);
      expect(requests.single.token, 'device-token');
    });

    test('empty tokens are skipped without a request', () async {
      final registrar = _registrar(FakeTransportBuilder());
      expect(
        (await registrar.registerDevice(fcmToken: '')).disposition,
        DeviceMutationDisposition.skipped,
      );
      expect(
        (await registrar.removeDevice(fcmToken: '')).disposition,
        DeviceMutationDisposition.skipped,
      );
    });

    test('unavailable is retryable, invalid_argument is rejected', () async {
      for (final (code, expected) in [
        (Code.unavailable, DeviceMutationDisposition.retryableFailure),
        (Code.invalidArgument, DeviceMutationDisposition.rejected),
      ]) {
        final registrar = _registrar(
          FakeTransportBuilder().unary(
            NotificationAppService.registerDeviceToken,
            (r, _) => throw ConnectException(code, 'x'),
          ),
        );
        final outcome = await registrar.registerDevice(fcmToken: 't');
        expect(outcome.disposition, expected, reason: '$code');
      }
    });
  });

  group('DeviceRegistrationCoordinator', () {
    test('deduplicates concurrent registration for the same token', () async {
      final pending = Completer<DeviceMutationOutcome>();
      final gateway = _FakeGateway(onRegister: (_) => pending.future);
      final coordinator = DeviceRegistrationCoordinator(
        gateway: gateway,
        retryDelays: const [],
      );

      final first = coordinator.register('fcm-token');
      final second = coordinator.register('fcm-token');

      expect(identical(first, second), isTrue);
      expect(gateway.registerCalls, ['fcm-token']);

      pending.complete(const DeviceMutationOutcome.success());
      expect((await first).isSuccess, isTrue);
      expect((await second).isSuccess, isTrue);
      expect(gateway.registerCalls, hasLength(1));
    });

    test('uses capped backoff and records only eventual success', () async {
      final delays = <Duration>[];
      final now = DateTime.utc(2026, 7, 27, 8);
      final gateway = _SequenceGateway([
        const DeviceMutationOutcome.retryableFailure(),
        const DeviceMutationOutcome.retryableFailure(),
        const DeviceMutationOutcome.success(),
      ]);
      final coordinator = DeviceRegistrationCoordinator(
        gateway: gateway,
        clock: () => now,
        delay: (duration) async => delays.add(duration),
        retryDelays: const [Duration(milliseconds: 250), Duration(seconds: 1)],
      );

      final outcome = await coordinator.register('fcm-token');

      expect(outcome.isSuccess, isTrue);
      expect(gateway.registerCalls, 3);
      expect(delays, const [Duration(milliseconds: 250), Duration(seconds: 1)]);
      expect(coordinator.lastSuccessfulAt, now);
      expect(coordinator.shouldThrottle('fcm-token'), isTrue);
    });

    test('4xx failure is retried on a later resume, not throttled', () async {
      final gateway = _SequenceGateway([
        const DeviceMutationOutcome.rejected(),
        const DeviceMutationOutcome.success(),
      ]);
      final coordinator = DeviceRegistrationCoordinator(
        gateway: gateway,
        retryDelays: const [
          Duration(milliseconds: 1),
          Duration(milliseconds: 2),
        ],
        delay: (_) async {},
      );

      final rejected = await coordinator.register('fcm-token');

      expect(rejected.disposition, DeviceMutationDisposition.rejected);
      expect(gateway.registerCalls, 1);
      expect(coordinator.lastSuccessfulAt, isNull);
      expect(coordinator.shouldThrottle('fcm-token'), isFalse);

      final resumed = await coordinator.register('fcm-token');
      expect(resumed.isSuccess, isTrue);
      expect(gateway.registerCalls, 2);
      expect(coordinator.shouldThrottle('fcm-token'), isTrue);
    });

    test('token rotation bypasses an existing success throttle', () async {
      final gateway = _SequenceGateway([
        const DeviceMutationOutcome.success(),
        const DeviceMutationOutcome.success(),
      ]);
      final coordinator = DeviceRegistrationCoordinator(
        gateway: gateway,
        retryDelays: const [],
      );

      await coordinator.register('old-token');
      expect(coordinator.shouldThrottle('old-token'), isTrue);
      expect(coordinator.shouldThrottle('new-token'), isFalse);

      await coordinator.register('new-token');
      expect(coordinator.lastSuccessfulToken, 'new-token');
      expect(gateway.registerCalls, 2);
    });

    test('collapses concurrent rotations to the newest queued token', () async {
      final active = Completer<DeviceMutationOutcome>();
      var invocation = 0;
      final gateway = _FakeGateway(
        onRegister: (_) {
          invocation++;
          if (invocation == 1) return active.future;
          return Future.value(const DeviceMutationOutcome.success());
        },
      );
      final coordinator = DeviceRegistrationCoordinator(
        gateway: gateway,
        retryDelays: const [],
      );

      final old = coordinator.register('old-token');
      final superseded = coordinator.register('middle-token');
      final newest = coordinator.register('newest-token');

      expect((await superseded).disposition, DeviceMutationDisposition.skipped);
      expect(gateway.registerCalls, ['old-token']);

      active.complete(const DeviceMutationOutcome.success());
      await old;
      expect((await newest).isSuccess, isTrue);

      expect(gateway.registerCalls, ['old-token', 'newest-token']);
      expect(coordinator.lastSuccessfulToken, 'newest-token');
    });

    test('removal of an empty token never reaches the gateway', () async {
      final gateway = _FakeGateway(
        onRegister: (_) async => const DeviceMutationOutcome.success(),
      );
      final coordinator = DeviceRegistrationCoordinator(
        gateway: gateway,
        retryDelays: const [],
      );

      final outcome = await coordinator.remove('');

      expect(outcome.disposition, DeviceMutationDisposition.skipped);
      expect(gateway.removeCalls, isEmpty);
    });

    test('401 removal is terminal — no retry, exactly one call', () async {
      final delays = <Duration>[];
      final gateway = _FakeGateway(
        onRegister: (_) async => const DeviceMutationOutcome.success(),
        onRemove: (_) async => const DeviceMutationOutcome.rejected(),
      );
      final coordinator = DeviceRegistrationCoordinator(
        gateway: gateway,
        delay: (duration) async => delays.add(duration),
        retryDelays: const [Duration(milliseconds: 250), Duration(seconds: 1)],
      );

      final outcome = await coordinator.remove('fcm-token');

      expect(outcome.disposition, DeviceMutationDisposition.rejected);
      expect(gateway.removeCalls, ['fcm-token']);
      expect(delays, isEmpty);
    });

    test('removal retries 5xx with capped backoff', () async {
      final delays = <Duration>[];
      var attempt = 0;
      final gateway = _FakeGateway(
        onRegister: (_) async => const DeviceMutationOutcome.success(),
        onRemove: (_) async {
          attempt++;
          if (attempt < 3) {
            return const DeviceMutationOutcome.retryableFailure();
          }
          return const DeviceMutationOutcome.success();
        },
      );
      final coordinator = DeviceRegistrationCoordinator(
        gateway: gateway,
        delay: (duration) async => delays.add(duration),
        retryDelays: const [Duration(milliseconds: 250), Duration(seconds: 1)],
      );

      final outcome = await coordinator.remove('fcm-token');

      expect(outcome.isSuccess, isTrue);
      expect(gateway.removeCalls, hasLength(3));
      expect(delays, const [Duration(milliseconds: 250), Duration(seconds: 1)]);
    });

    test('deduplicates concurrent removal for the same token', () async {
      final pending = Completer<DeviceMutationOutcome>();
      final gateway = _FakeGateway(
        onRegister: (_) async => const DeviceMutationOutcome.success(),
        onRemove: (_) => pending.future,
      );
      final coordinator = DeviceRegistrationCoordinator(
        gateway: gateway,
        retryDelays: const [],
      );

      final first = coordinator.remove('fcm-token');
      final second = coordinator.remove('fcm-token');

      expect(identical(first, second), isTrue);
      expect(gateway.removeCalls, ['fcm-token']);

      pending.complete(const DeviceMutationOutcome.success());
      expect((await first).isSuccess, isTrue);
      expect((await second).isSuccess, isTrue);
      expect(gateway.removeCalls, hasLength(1));
    });

    test('breaker short-circuits register and removal past the window '
        'budget, and logout-time reset does not refund it', () async {
      final now = DateTime.utc(2026, 7, 29, 8);
      final gateway = _FakeGateway(
        onRegister: (_) async => const DeviceMutationOutcome.success(),
        onRemove: (_) async => const DeviceMutationOutcome.rejected(),
      );
      final coordinator = DeviceRegistrationCoordinator(
        gateway: gateway,
        clock: () => now,
        retryDelays: const [],
        maxCallsPerWindow: 2,
        breakerWindow: const Duration(minutes: 5),
      );

      await coordinator.register('fcm-token');
      final secondRemoval = await coordinator.remove('fcm-token');
      expect(secondRemoval.disposition, DeviceMutationDisposition.rejected);

      coordinator.reset();
      final shorted = await coordinator.remove('fcm-token');
      final shortedRegister = await coordinator.register('fcm-token');

      expect(shorted.disposition, DeviceMutationDisposition.skipped);
      expect(shortedRegister.disposition, DeviceMutationDisposition.skipped);
      expect(gateway.registerCalls, hasLength(1));
      expect(gateway.removeCalls, hasLength(1));
    });

    test('breaker budget recovers after the window elapses', () async {
      var now = DateTime.utc(2026, 7, 29, 8);
      final gateway = _FakeGateway(
        onRegister: (_) async => const DeviceMutationOutcome.success(),
        onRemove: (_) async => const DeviceMutationOutcome.success(),
      );
      final coordinator = DeviceRegistrationCoordinator(
        gateway: gateway,
        clock: () => now,
        retryDelays: const [],
        maxCallsPerWindow: 1,
        breakerWindow: const Duration(minutes: 5),
      );

      await coordinator.remove('fcm-token');
      expect(
        (await coordinator.remove('fcm-token')).disposition,
        DeviceMutationDisposition.skipped,
      );

      now = now.add(const Duration(minutes: 6));
      expect((await coordinator.remove('fcm-token')).isSuccess, isTrue);
      expect(gateway.removeCalls, hasLength(2));
    });

    /// register 與 remove 在後端寫的是同一把 (user_id, user_type, token) 的相反
    /// 值（Upsert is_active=true / Deactivate is_active=false）。交錯落地最惡性
    /// 的順序是「登出的 DELETE 蓋掉重新登入的註冊」：帳號已登入卻整台裝置收不到
    /// 推播，且 client 以為註冊成功、24h 節流內不會再試。
    test('登出的註銷不會搶在進行中的註冊之前送出', () async {
      final order = <String>[];
      final pending = Completer<DeviceMutationOutcome>();
      final gateway = _FakeGateway(
        onRegister: (_) {
          order.add('register');
          return pending.future;
        },
        onRemove: (_) async {
          order.add('remove');
          return const DeviceMutationOutcome.success();
        },
      );
      final coordinator = DeviceRegistrationCoordinator(
        gateway: gateway,
        retryDelays: const [],
      );

      final registration = coordinator.register('fcm-token');
      final removal = coordinator.remove('fcm-token');
      await Future<void>.delayed(Duration.zero);

      expect(order, ['register']);

      pending.complete(const DeviceMutationOutcome.success());
      await registration;
      expect((await removal).isSuccess, isTrue);
      expect(order, ['register', 'remove']);
    });

    test('重新登入的註冊不會搶在進行中的註銷之前送出', () async {
      final order = <String>[];
      final pending = Completer<DeviceMutationOutcome>();
      final gateway = _FakeGateway(
        onRegister: (_) async {
          order.add('register');
          return const DeviceMutationOutcome.success();
        },
        onRemove: (_) {
          order.add('remove');
          return pending.future;
        },
      );
      final coordinator = DeviceRegistrationCoordinator(
        gateway: gateway,
        retryDelays: const [],
      );

      final removal = coordinator.remove('fcm-token');
      coordinator.reset();
      final registration = coordinator.register('fcm-token');
      await Future<void>.delayed(Duration.zero);

      expect(order, ['remove']);

      pending.complete(const DeviceMutationOutcome.success());
      await removal;
      expect((await registration).isSuccess, isTrue);
      expect(order, ['remove', 'register']);
      expect(coordinator.lastSuccessfulToken, 'fcm-token');
    });

    test('reset rejects late success from the previous session', () async {
      final pending = Completer<DeviceMutationOutcome>();
      final gateway = _FakeGateway(onRegister: (_) => pending.future);
      final coordinator = DeviceRegistrationCoordinator(
        gateway: gateway,
        retryDelays: const [],
      );

      final operation = coordinator.register('old-session-token');
      coordinator.reset();
      pending.complete(const DeviceMutationOutcome.success());
      await operation;

      expect(coordinator.lastSuccessfulAt, isNull);
      expect(coordinator.lastSuccessfulToken, isNull);
    });
  });
}

class _FakeGateway implements DeviceRegistrationGateway {
  _FakeGateway({required this.onRegister, this.onRemove});

  final Future<DeviceMutationOutcome> Function(String token) onRegister;
  final Future<DeviceMutationOutcome> Function(String token)? onRemove;
  final List<String> registerCalls = [];
  final List<String> removeCalls = [];

  @override
  Future<DeviceMutationOutcome> registerDevice({required String fcmToken}) {
    registerCalls.add(fcmToken);
    return onRegister(fcmToken);
  }

  @override
  Future<DeviceMutationOutcome> removeDevice({required String fcmToken}) {
    removeCalls.add(fcmToken);
    final handler = onRemove;
    if (handler != null) return handler(fcmToken);
    return Future.value(const DeviceMutationOutcome.success());
  }
}

class _SequenceGateway implements DeviceRegistrationGateway {
  _SequenceGateway(this._outcomes);

  final List<DeviceMutationOutcome> _outcomes;
  int registerCalls = 0;

  @override
  Future<DeviceMutationOutcome> registerDevice({
    required String fcmToken,
  }) async {
    final index = registerCalls++;
    return _outcomes[index];
  }

  @override
  Future<DeviceMutationOutcome> removeDevice({required String fcmToken}) async {
    return const DeviceMutationOutcome.success();
  }
}
