import 'dart:async';

import 'package:hawkr_buyer/core/exceptions/app_exception.dart';
import 'package:hawkr_buyer/core/session/force_logout_provider.dart';
import 'package:hawkr_buyer/core/session/clerk_session_provider.dart';
import 'package:hawkr_buyer/core/session/session_provider.dart';
import 'package:hawkr_buyer/features/auth/data/auth_repository.dart';
import 'package:hawkr_buyer/features/notification/data/push/manager.dart';
import 'package:hawkr_buyer/features/notification/data/push/background_handler.dart';
import 'package:hawkr_buyer/features/order/data/receipt_photo_local_service.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../../support/fake_clerk_session.dart';
import '../../support/firebase_core_mock.dart';
import '../../support/secure_storage_mock.dart';

class _AccountRepository extends Fake implements AuthRepository {
  _AccountRepository(this.answer, this.profile, this.profileAnswer);

  final BuyerProfile? profile;
  final Future<BuyerProfile?> Function()? profileAnswer;

  final Future<String> Function() answer;

  @override
  Future<String> currentAccountId() => answer();

  @override
  Future<BuyerProfile?> tryGetProfile() async =>
      profileAnswer == null ? profile : await profileAnswer!();
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUp(() {
    mockFirebaseCore();
    PushNotificationManager.e2eDisabled = true;
    addTearDown(() => PushNotificationManager.e2eDisabled = false);
    SharedPreferences.setMockInitialValues({});
    mockSecureStorage({});
  });

  ProviderContainer containerWith(
    Future<FakeClerkSession> Function() clerk,
    Future<String> Function() account, {
    BuyerProfile? profile = const BuyerProfile(
      id: '1',
      etag: 'v1',
      displayName: '冷啟動暱稱',
      avatarUrl: '',
      phone: '',
      gender: null,
      birthDate: null,
      city: '',
      job: '',
    ),
    Future<BuyerProfile?> Function()? profileAnswer,
  }) {
    final container = ProviderContainer(
      overrides: [
        clerkSessionProvider.overrideWith((ref) => clerk()),
        authRepositoryProvider.overrideWith(
          (ref) => _AccountRepository(account, profile, profileAnswer),
        ),
      ],
    );
    addTearDown(container.dispose);
    return container;
  }

  test('冷啟動：Clerk 登入中，以 GetSession 取得帳號', () async {
    var calls = 0;
    final c = containerWith(
      () async => FakeClerkSession(signedIn: true),
      () async {
        calls++;
        return 'acc_01J8Z';
      },
    );

    final session = await c.read(sessionProvider.future);

    expect(calls, 1);
    expect(session?.userId, 'acc_01J8Z');
    await Future<void>.delayed(Duration.zero);
    expect(c.read(sessionProvider).value?.nickname, '冷啟動暱稱');
    expect(hasUsableSession(session), isTrue);
  });

  test('GetSession 完成即恢復，個人資料在背景完成', () async {
    final profile = Completer<BuyerProfile?>();
    final c = containerWith(
      () async => FakeClerkSession(signedIn: true),
      () async => 'acc-1',
      profileAnswer: () => profile.future,
    );
    final session = await c.read(sessionProvider.future);
    expect(session?.userId, 'acc-1');
    expect(session?.nickname, isNull);
    expect(profile.isCompleted, isFalse);
    profile.complete(
      const BuyerProfile(
        id: '1',
        etag: 'v1',
        displayName: '恢復後',
        avatarUrl: '',
        phone: '',
        gender: null,
        birthDate: null,
        city: '',
        job: '',
      ),
    );
    await Future<void>.delayed(Duration.zero);
    expect(c.read(sessionProvider).value?.nickname, '恢復後');
  });

  test('冷啟動暫時失敗會自動退避重試', () async {
    final photos = SharedPreferencesReceiptPhotoLocalService();
    await photos.write('order-1', 'cached-photo');
    final prefs = await SharedPreferences.getInstance();
    await prefs.setStringList(fcmProcessedIdsKey, ['message-1']);
    var calls = 0;
    final clerk = FakeClerkSession(signedIn: true);
    final c = containerWith(() async => clerk, () async {
      if (++calls < 2) throw const NetworkException();
      return 'recovered';
    });
    expect(await c.read(sessionProvider.future), isNull);
    expect(calls, 1);
    await Future<void>.delayed(const Duration(milliseconds: 1600));
    expect(c.read(sessionProvider).value?.userId, 'recovered');
    expect(calls, 2);
    expect(clerk.calls, isEmpty);
    expect(await photos.read('order-1'), 'cached-photo');
    expect(prefs.getStringList(fcmProcessedIdsKey), ['message-1']);
  });

  test('背景重試晚到的帳號不能覆寫主動登入', () async {
    final answer = Completer<String>();
    var calls = 0;
    final c = containerWith(
      () async => FakeClerkSession(signedIn: true),
      () async {
        if (++calls == 1) throw const NetworkException();
        return answer.future;
      },
    );
    expect(await c.read(sessionProvider.future), isNull);
    await Future<void>.delayed(const Duration(milliseconds: 1600));
    expect(calls, 2);
    await c.read(sessionProvider.notifier).login(userId: 'new-owner');
    answer.complete('old-owner');
    await Future<void>.delayed(Duration.zero);
    expect(c.read(sessionProvider).value?.userId, 'new-owner');
  });

  test('已排程停權提醒不被後續 401 清空', () {
    final c = containerWith(
      () async => FakeClerkSession(),
      () async => 'acc-1',
    );
    final force = c.read(forceLogoutProvider.notifier);
    force.trigger(message: '帳號已停權');
    force.trigger();
    expect(force.message, '帳號已停權');
    expect(c.read(forceLogoutProvider), isTrue);
    force.consume();
    expect(force.message, isNull);
  });

  test('冷啟動個人資料尚未知時暱稱是 null', () async {
    final c = containerWith(
      () async => FakeClerkSession(signedIn: true),
      () async => 'acc-1',
      profile: null,
    );
    final session = await c.read(sessionProvider.future);
    expect(session?.userId, 'acc-1');
    expect(session?.nickname, isNull);
  });

  test('冷啟動：Clerk 沒有 session，不呼叫 GetSession', () async {
    var calls = 0;
    final c = containerWith(() async => FakeClerkSession(), () async {
      calls++;
      return 'acc_01J8Z';
    });

    expect(await c.read(sessionProvider.future), isNull);
    expect(calls, 0);
  });

  test('冷啟動：GetSession 暫時失敗，保留 Clerk session', () async {
    final clerk = FakeClerkSession(signedIn: true);
    final c = containerWith(
      () async => clerk,
      () async => throw StateError('backend unavailable'),
    );

    expect(await c.read(sessionProvider.future), isNull);
    expect(c.read(sessionProvider).hasError, isFalse);
    expect(clerk.calls, isEmpty);
  });

  for (final error in [
    const UnauthorizedException(),
    const SuspendedException('帳號已停權'),
  ]) {
    test('冷啟動拒絕憑證：$error', () async {
      final clerk = FakeClerkSession(signedIn: true);
      final c = containerWith(() async => clerk, () async => throw error);
      expect(await c.read(sessionProvider.future), isNull);
      expect(clerk.isSignedIn, isFalse);
      expect(c.read(sessionProvider).hasError, isFalse);
      if (error is SuspendedException) {
        expect(c.read(forceLogoutProvider.notifier).message, '帳號已停權');
        expect(c.read(forceLogoutProvider), isTrue);
      }
    });
  }

  test('暫時故障後可重新取得帳號', () async {
    var failed = true;
    final c = containerWith(
      () async => FakeClerkSession(signedIn: true),
      () async {
        if (failed) throw const NetworkException();
        return 'recovered';
      },
    );
    expect(await c.read(sessionProvider.future), isNull);
    failed = false;
    c.invalidate(sessionProvider);
    expect((await c.read(sessionProvider.future))?.userId, 'recovered');
  });

  for (final error in [
    const MalformedAuthResponse('missing account'),
    const ServerException('unknown error'),
  ]) {
    test('非拒絕失敗保留 Clerk：$error', () async {
      final clerk = FakeClerkSession(signedIn: true);
      final c = containerWith(() async => clerk, () async => throw error);
      expect(await c.read(sessionProvider.future), isNull);
      expect(clerk.isSignedIn, isTrue);
      expect(clerk.calls, isEmpty);
    });
  }

  test('冷啟動：Clerk 無法啟動，以訪客開始', () async {
    final c = containerWith(
      () async => throw StateError('keychain'),
      () async => 'acc_01J8Z',
    );

    expect(await c.read(sessionProvider.future), isNull);
  });

  test('登出：結束 Clerk session 並清除記憶體帳號', () async {
    final clerk = FakeClerkSession(signedIn: true);
    final c = containerWith(() async => clerk, () async => 'acc_01J8Z');
    await c.read(sessionProvider.future);

    await c.read(sessionProvider.notifier).logout();

    expect(clerk.calls, ['signOut']);
    expect(c.read(sessionProvider).value, isNull);
  });

  test('登入後更新暱稱只改目前 session', () async {
    final c = containerWith(
      () async => FakeClerkSession(),
      () async => 'acc_01J8Z',
    );
    await c.read(sessionProvider.future);
    final notifier = c.read(sessionProvider.notifier);

    await notifier.login(userId: 'acc_01J8Z', nickname: '');
    await notifier.updateNickname('Cora');

    expect(c.read(sessionProvider).value?.nickname, 'Cora');
  });
}
