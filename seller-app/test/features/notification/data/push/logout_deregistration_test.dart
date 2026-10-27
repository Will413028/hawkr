import 'dart:async';

import 'package:hawkr_seller/config/flavor.dart';
import 'package:hawkr_seller/features/notification/data/push/device_registrar.dart';
import 'package:hawkr_seller/features/notification/data/push/manager.dart';
import 'package:firebase_core/firebase_core.dart';
import 'package:firebase_core_platform_interface/test.dart';
import 'package:flutter_test/flutter_test.dart';

/// hawkr_seller#106 的守衛層回歸防線。
///
/// 迴圈長這樣：登出 → 送裝置註銷 → 憑證已失效 → 401 → interceptor 強制登出
/// → 再送一次註銷……dev 環境 9 小時 14 萬次 DELETE。`Session.logout` 的
/// `_loggingOut` 只擋得住「同一輪重入」，擋不住「state 清空後由任何其他 401
/// 再觸發的下一輪」——那些輪次唯一的防線就是本檔驗的這道：
/// **沒有憑證 / 未登入就不送註銷**。
void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUpAll(() async {
    if (!F.isInitialized) F.init(Flavor.dev);
    // PushNotificationManager 的欄位初始化會碰 FirebaseMessaging.instance。
    setupFirebaseCoreMocks();
    await Firebase.initializeApp();
  });

  // 延後到 Firebase mock 就緒後才碰 singleton：它的欄位初始化會取
  // FirebaseMessaging.instance，在 main() body 直接取會 [core/no-app]。
  late final PushNotificationManager manager;
  setUpAll(() => manager = PushNotificationManager.instance);

  tearDown(() => manager.seedLifecycleStateForTest());

  DeviceRegistrationCoordinator coordinatorFor(_RecordingGateway gateway) =>
      DeviceRegistrationCoordinator(gateway: gateway, retryDelays: const []);

  group('PushNotificationManager.onUserLoggedOut 無憑證守衛', () {
    test('已登入但呼叫端說沒有憑證 → 不送註銷', () async {
      final gateway = _RecordingGateway();
      manager.seedLifecycleStateForTest(
        registration: coordinatorFor(gateway),
        fcmToken: 'fcm-token',
        isUserLoggedIn: true,
      );

      await manager.onUserLoggedOut(hasCredential: false);

      expect(gateway.removeCalls, isEmpty);
    });

    test('有憑證但 manager 認定未登入 → 不送註銷', () async {
      final gateway = _RecordingGateway();
      manager.seedLifecycleStateForTest(
        registration: coordinatorFor(gateway),
        fcmToken: 'fcm-token',
      );

      await manager.onUserLoggedOut(hasCredential: true);

      expect(gateway.removeCalls, isEmpty);
    });

    test('已登入且有憑證 → 註銷剛好送一次（對照組，確保上面兩條不是空測）', () async {
      final gateway = _RecordingGateway();
      manager.seedLifecycleStateForTest(
        registration: coordinatorFor(gateway),
        fcmToken: 'fcm-token',
        isUserLoggedIn: true,
      );

      await manager.onUserLoggedOut(hasCredential: true);

      expect(gateway.removeCalls, ['fcm-token']);
    });

    test('登出後 token 與登入旗標被清空 → 第二次登出不再送註銷', () async {
      final gateway = _RecordingGateway();
      manager.seedLifecycleStateForTest(
        registration: coordinatorFor(gateway),
        fcmToken: 'fcm-token',
        isUserLoggedIn: true,
      );

      await manager.onUserLoggedOut(hasCredential: true);
      // 第二輪：即使呼叫端仍宣稱有憑證（例如 refresh 期間的殘留 state）。
      await manager.onUserLoggedOut(hasCredential: true);

      expect(gateway.removeCalls, hasLength(1));
      expect(manager.fcmToken, isNull);
    });
  });

  group('PushNotificationManager.onUserLoggedOut 不阻塞登出', () {
    testWidgets('後端 hang 住時，登出在 deadline 內返回，請求續留背景', (tester) async {
      final pending = Completer<DeviceMutationOutcome>();
      final gateway = _RecordingGateway(onRemove: (_) => pending.future);
      manager.seedLifecycleStateForTest(
        registration: coordinatorFor(gateway),
        fcmToken: 'fcm-token',
        isUserLoggedIn: true,
      );

      var released = false;
      unawaited(
        manager.onUserLoggedOut(hasCredential: true).then((_) {
          released = true;
        }),
      );

      await tester.pump(
        PushNotificationManager.logoutDeregistrationDeadline -
            const Duration(milliseconds: 1),
      );
      expect(released, isFalse, reason: 'deadline 之前應照常等待，健康網路不改行為');

      await tester.pump(const Duration(milliseconds: 1));
      expect(released, isTrue, reason: '逾時後必須放行，不得把 15s~46s 攤到使用者身上');

      // 逾時不是取消：請求已經送出，後端慢慢回來時仍然完成註銷。
      expect(gateway.removeCalls, ['fcm-token']);
      pending.complete(const DeviceMutationOutcome.success(statusCode: 204));
      await tester.pump();
    });

    /// 互等（register/remove 不得交錯落地）× deadline（登出不得被網路拖住）的
    /// 交叉點：註銷排在 in-flight 註冊之後，但那個等待**不得**把註冊自己的
    /// 逾時（prod 15s）攤到使用者身上。少了任一半，這條就會紅。
    testWidgets('in-flight 註冊卡住時登出，仍在 deadline 內放行', (tester) async {
      final registerPending = Completer<DeviceMutationOutcome>();
      final gateway = _RecordingGateway(
        onRegister: (_) => registerPending.future,
      );
      final coordinator = coordinatorFor(gateway);
      // 例如回前景重註冊剛送出就卡住，使用者接著按登出。
      unawaited(coordinator.register('fcm-token'));
      manager.seedLifecycleStateForTest(
        registration: coordinator,
        fcmToken: 'fcm-token',
        isUserLoggedIn: true,
      );

      var released = false;
      unawaited(
        manager.onUserLoggedOut(hasCredential: true).then((_) {
          released = true;
        }),
      );

      await tester.pump(
        PushNotificationManager.logoutDeregistrationDeadline -
            const Duration(milliseconds: 1),
      );
      expect(released, isFalse);

      await tester.pump(const Duration(milliseconds: 1));
      expect(released, isTrue, reason: '互等不得讓登出被卡住的註冊連帶拖過 deadline');

      // 已知取捨：註銷排在註冊之後，此刻尚未送出——反過來讓 DELETE 先走會
      // 製造「註冊蓋掉註銷」的錯誤終態，那個不會自癒（見 _settleOpposing）。
      expect(gateway.removeCalls, isEmpty);

      // 註冊落地後，註銷才在背景依序送出。
      registerPending.complete(
        const DeviceMutationOutcome.success(statusCode: 200),
      );
      await tester.pump();
      expect(gateway.removeCalls, ['fcm-token']);
    });
  });
}

class _RecordingGateway implements DeviceRegistrationGateway {
  _RecordingGateway({this.onRegister, this.onRemove});

  final Future<DeviceMutationOutcome> Function(String token)? onRegister;
  final Future<DeviceMutationOutcome> Function(String token)? onRemove;
  final List<String> registerCalls = [];
  final List<String> removeCalls = [];

  @override
  Future<DeviceMutationOutcome> registerDevice({required String fcmToken}) {
    registerCalls.add(fcmToken);
    final handler = onRegister;
    if (handler != null) return handler(fcmToken);
    return Future.value(const DeviceMutationOutcome.success(statusCode: 200));
  }

  @override
  Future<DeviceMutationOutcome> removeDevice({required String fcmToken}) {
    removeCalls.add(fcmToken);
    final handler = onRemove;
    if (handler != null) return handler(fcmToken);
    return Future.value(const DeviceMutationOutcome.success(statusCode: 204));
  }
}
