import 'dart:async';

import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/transport.dart';
import 'package:hawkr_seller/config/flavor.dart';
import 'package:hawkr_seller/core/session/post_login_side_effects.dart';
import 'package:hawkr_seller/core/session/session_provider.dart';
import 'package:hawkr_seller/features/seller/data/current_store.dart';
import 'package:hawkr_seller/features/seller/data/my_store_repository.dart';
import 'package:firebase_core/firebase_core.dart';
import 'package:firebase_core_platform_interface/test.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../../support/fake_my_store.dart';
import '../../support/my_store_fixtures.dart';

/// 店家（`GetMyStore`）的狀態在 `currentStoreProvider`，session 只管登入的身分。
void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUpAll(() async {
    if (!F.isInitialized) F.init(Flavor.dev);

    // Session.login() → _syncToStaticServices() reaches
    // PushNotificationManager.instance, whose field initializer calls
    // FirebaseMessaging.instance (→ Firebase.app()) — throws `core/no-app`
    // unless a Firebase app is registered first.
    setupFirebaseCoreMocks();
    await Firebase.initializeApp();

    // login() completes without a MissingPluginException in the test binary.
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(
          const MethodChannel('plugins.it_nomads.com/flutter_secure_storage'),
          (call) async {
            switch (call.method) {
              case 'readAll':
                return <String, String>{};
              case 'containsKey':
                return false;
              default:
                return null;
            }
          },
        );
  });

  setUp(() {
    SharedPreferences.setMockInitialValues({});
  });

  ProviderContainer containerWith(FakeMyStoreServer server) {
    final container = ProviderContainer(
      overrides: [
        myStoreRepositoryProvider.overrideWithValue(server.repository),
        postLoginSideEffectsProvider.overrideWithValue(
          const NoopPostLoginSideEffects(),
        ),
      ],
    );
    addTearDown(container.dispose);
    final subscription = container.listen(currentStoreProvider, (_, _) {});
    addTearDown(subscription.close);
    return container;
  }

  Future<void> signIn(ProviderContainer container, String accountId) async {
    await container.read(sessionProvider.notifier).login(accountId: accountId);
    await container.read(currentStoreProvider.future);
  }

  test(
    'GetMyStore not_found（沒有店家）→ needsOnboarding 為 true，店家快照是「沒有店家」',
    () async {
      final container = containerWith(FakeMyStoreServer.absent());
      await signIn(container, 'acc-1');

      final snapshot = container.read(myStoreSnapshotProvider);
      expect(container.read(needsOnboardingProvider), isTrue);
      expect(snapshot, isNotNull);
      expect(snapshot!.hasStore, isFalse);
      expect(container.read(sellerStoreIdProvider), isNull);
      expect(container.read(storeUnavailableProvider), isFalse);
    },
  );

  test('登入本身就等 GetMyStore：login 回傳時 needsOnboarding 已有答案', () async {
    final container = containerWith(FakeMyStoreServer.absent());

    expect(container.read(needsOnboardingProvider), isNull);
    await signIn(container, 'acc-1');

    expect(container.read(needsOnboardingProvider), isTrue);
  });

  test('有店家但尚未完成新手引導 → needsOnboarding 為 true，storeId 來自店家', () async {
    final container = containerWith(
      FakeMyStoreServer(response: myStoreResponse(onboardingCompleted: false)),
    );
    await signIn(container, 'acc-1');

    expect(container.read(needsOnboardingProvider), isTrue);
    expect(container.read(sellerStoreIdProvider), 'store-1');
    expect(container.read(sessionProvider).accountId, 'acc-1');
  });

  test('有店家且已完成新手引導 → needsOnboarding 為 false', () async {
    final container = containerWith(FakeMyStoreServer());
    await signIn(container, 'acc-1');

    final snapshot = container.read(myStoreSnapshotProvider)!;
    expect(container.read(needsOnboardingProvider), isFalse);
    expect(snapshot.isSuspended, isFalse);
    expect(snapshot.isPendingDeletion, isFalse);
  });

  test('停權的店家 → 帶停權與白名單能力', () async {
    final container = containerWith(
      FakeMyStoreServer(response: myStoreResponse(suspended: true)),
    );
    await signIn(container, 'acc-1');

    final snapshot = container.read(myStoreSnapshotProvider)!;
    expect(snapshot.isSuspended, isTrue);
    expect(snapshot.isPendingDeletion, isFalse);
    expect(container.read(needsOnboardingProvider), isFalse);
    expect(
      snapshot.capabilities,
      isNot(contains(SellerCapability.SELLER_CAPABILITY_PRODUCT_MANAGE)),
    );
  });

  test('等待刪除的店家 → isPendingDeletion', () async {
    final container = containerWith(
      FakeMyStoreServer(
        response: myStoreResponse(
          status: StoreStatus.STORE_STATUS_PENDING_DELETION,
        ),
      ),
    );
    await signIn(container, 'acc-1');

    final snapshot = container.read(myStoreSnapshotProvider)!;
    expect(snapshot.isPendingDeletion, isTrue);
    expect(snapshot.isSuspended, isFalse);
  });

  test(
    '暫時失敗且沒有快照：不是「沒有店家」，也不放行（needsOnboarding 不明、storeUnavailable）',
    () async {
      final container = containerWith(FakeMyStoreServer.unavailable());
      await signIn(container, 'acc-1');

      expect(container.read(storeUnavailableProvider), isTrue);
      expect(container.read(needsOnboardingProvider), isNull);
      expect(container.read(myStoreSnapshotProvider), isNull);
      expect(container.read(sellerStoreIdProvider), isNull);
      expect(
        await container.read(currentStoreProvider.notifier).refresh(),
        isFalse,
      );
    },
  );

  test('失敗之後重試成功 → 恢復正常，不再 storeUnavailable', () async {
    final server = FakeMyStoreServer.unavailable();
    final container = containerWith(server);
    await signIn(container, 'acc-1');
    expect(container.read(storeUnavailableProvider), isTrue);

    server.failure = null;
    final ok = await container.read(currentStoreProvider.notifier).refresh();

    expect(ok, isTrue);
    expect(container.read(storeUnavailableProvider), isFalse);
    expect(container.read(needsOnboardingProvider), isFalse);
    expect(container.read(sellerStoreIdProvider), 'store-1');
  });

  test('重新讀取失敗時保留上一次的店家快照，不 storeUnavailable', () async {
    final server = FakeMyStoreServer(
      response: myStoreResponse(suspended: true),
    );
    final container = containerWith(server);
    await signIn(container, 'acc-1');
    final before = container.read(myStoreSnapshotProvider);

    server.failure = ConnectException(Code.unavailable, 'try later');
    final ok = await container.read(currentStoreProvider.notifier).refresh();

    expect(ok, isFalse);
    expect(container.read(myStoreSnapshotProvider), before);
    expect(container.read(myStoreSnapshotProvider)!.isSuspended, isTrue);
    expect(container.read(storeUnavailableProvider), isFalse);
  });

  test('重新讀取反映伺服器的新答案：解除停權後回到正常', () async {
    final server = FakeMyStoreServer(
      response: myStoreResponse(suspended: true),
    );
    final container = containerWith(server);
    await signIn(container, 'acc-1');
    expect(container.read(myStoreSnapshotProvider)!.isSuspended, isTrue);

    server.response = myStoreResponse();
    await container.read(currentStoreProvider.notifier).refresh();

    expect(container.read(myStoreSnapshotProvider)!.isSuspended, isFalse);
  });

  test('onboarding 寫入後重讀店家：伺服器答案變成已完成後 needsOnboarding 為 false', () async {
    final server = FakeMyStoreServer(
      response: myStoreResponse(onboardingCompleted: false),
    );
    final container = containerWith(server);
    await signIn(container, 'acc-1');
    expect(container.read(needsOnboardingProvider), isTrue);

    server.response = myStoreResponse();
    await container.read(currentStoreProvider.notifier).refresh();

    expect(container.read(needsOnboardingProvider), isFalse);
    expect(server.calls, 2);
  });

  group('onSellerSuspendedSignal', () {
    test('重讀 GetMyStore；讀完前的重複訊號不再重打', () async {
      final gate = Completer<void>();
      final server = FakeMyStoreServer();
      final container = containerWith(server);
      final notifier = container.read(sessionProvider.notifier);
      await signIn(container, 'acc-1');
      expect(server.calls, 1);

      server
        ..gate = gate.future
        ..response = myStoreResponse(suspended: true);
      notifier
        ..onSellerSuspendedSignal()
        ..onSellerSuspendedSignal();
      gate.complete();
      await pumpEventQueue();

      expect(server.calls, 2);
      expect(container.read(myStoreSnapshotProvider)!.isSuspended, isTrue);
    });

    test('已經停權時再收到訊號仍重讀（能力快照可能過期）', () async {
      final server = FakeMyStoreServer(
        response: myStoreResponse(suspended: true),
      );
      final container = containerWith(server);
      final notifier = container.read(sessionProvider.notifier);
      await signIn(container, 'acc-1');
      expect(server.calls, 1);

      server.response = myStoreResponse();
      notifier.onSellerSuspendedSignal();
      await pumpEventQueue();

      expect(server.calls, 2);
      expect(container.read(myStoreSnapshotProvider)!.isSuspended, isFalse);
    });

    test('沒有登入時不讀', () async {
      final server = FakeMyStoreServer();
      final container = containerWith(server);

      container.read(sessionProvider.notifier).onSellerSuspendedSignal();
      await pumpEventQueue();

      expect(server.calls, 0);
    });
  });

  group('帳號與重疊的讀取', () {
    test('讀取回來前已登出 → 結果不寫進來', () async {
      final gate = Completer<void>();
      final server = FakeMyStoreServer(gate: gate.future);
      final container = containerWith(server);
      final login = container
          .read(sessionProvider.notifier)
          .login(accountId: 'acc-1');
      await pumpEventQueue();

      container.read(sessionProvider.notifier).state = const SessionModel();
      gate.complete();
      await login;
      await pumpEventQueue();

      expect(container.read(myStoreSnapshotProvider), isNull);
      expect(container.read(sellerStoreIdProvider), isNull);
      expect(container.read(needsOnboardingProvider), isNull);
      expect(container.read(storeUnavailableProvider), isFalse);
    });

    test('換帳號：舊帳號較晚才回來的讀取不會蓋掉新帳號的店家', () async {
      final gate = Completer<void>();
      final server = FakeMyStoreServer(
        response: myStoreResponse(id: 'store-a'),
        gate: gate.future,
      );
      final container = containerWith(server);
      final notifier = container.read(sessionProvider.notifier);
      final loginA = signIn(container, 'acc-a');
      await pumpEventQueue();

      notifier.state = const SessionModel();
      server
        ..gate = null
        ..response = myStoreResponse(id: 'store-b');
      await signIn(container, 'acc-b');
      expect(container.read(sellerStoreIdProvider), 'store-b');

      gate.complete();
      await loginA;
      await pumpEventQueue();

      expect(container.read(sellerStoreIdProvider), 'store-b');
      expect(container.read(sessionProvider).accountId, 'acc-b');
      expect(container.read(needsOnboardingProvider), isFalse);
    });

    test('換帳號：新帳號讀取失敗時不沿用舊帳號的快照（storeUnavailable）', () async {
      final server = FakeMyStoreServer(
        response: myStoreResponse(id: 'store-a'),
      );
      final container = containerWith(server);
      final notifier = container.read(sessionProvider.notifier);
      await signIn(container, 'acc-a');
      expect(container.read(sellerStoreIdProvider), 'store-a');

      notifier.state = const SessionModel();
      server.failure = ConnectException(Code.unavailable, 'try later');
      await signIn(container, 'acc-b');

      expect(container.read(myStoreSnapshotProvider), isNull);
      expect(container.read(sellerStoreIdProvider), isNull);
      expect(container.read(storeUnavailableProvider), isTrue);
      expect(container.read(needsOnboardingProvider), isNull);
    });

    test('重疊的讀取：較舊的讀取較晚回來也不會蓋掉較新的', () async {
      final gate = Completer<void>();
      final server = FakeMyStoreServer(
        response: myStoreResponse(id: 'store-old'),
        gate: gate.future,
      );
      final container = containerWith(server);
      final login = signIn(container, 'acc-1');
      await pumpEventQueue();

      server
        ..gate = null
        ..response = myStoreResponse(id: 'store-new');
      await container.read(currentStoreProvider.notifier).refresh();
      expect(container.read(sellerStoreIdProvider), 'store-new');

      gate.complete();
      await login;
      await pumpEventQueue();

      expect(container.read(sellerStoreIdProvider), 'store-new');
      expect(container.read(myStoreSnapshotProvider)!.storeName, '阿明果園');
      expect(server.calls, 2);
    });
  });
}
