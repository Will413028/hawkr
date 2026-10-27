import 'package:hawkr_api/gen/hawkr/identity/v1alpha1/identity.pb.dart';
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:hawkr_seller/core/session/clerk_session_provider.dart';
import 'package:hawkr_seller/core/session/session_provider.dart';
import 'package:hawkr_seller/features/auth/data/auth_repository.dart';
import 'package:hawkr_seller/features/seller/data/current_store.dart';
import 'package:hawkr_seller/features/seller/data/my_store_repository.dart';
import 'package:hawkr_seller/features/seller/data/version_conflict.dart';
import 'package:hawkr_seller/features/seller/presentation/profile_account_closure.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

import '../../../support/fake_clerk_session.dart';
import '../../../support/connect_errors.dart';
import '../../../support/fake_my_store.dart';
import '../../../support/my_store_fixtures.dart';

void main() {
  group('刪除帳號', () {
    Future<List<String>> deleteWith(
      WidgetTester tester, {
      FakeMyStoreServer? store,
      VoidCallback? onGoToCloseStore,
      DeleteAccountResponse_DeletionState state =
          DeleteAccountResponse_DeletionState.DELETION_STATE_DELETED,
      Object? deleteError,
      void Function()? afterFlow,
    }) async {
      final events = <String>[];
      final router = GoRouter(
        routes: [
          GoRoute(
            path: '/',
            builder: (_, _) => Scaffold(
              body: Center(
                child: ProfileDeleteAccountSection(
                  onGoToCloseStore: onGoToCloseStore,
                ),
              ),
            ),
          ),
          GoRoute(
            path: '/auth',
            builder: (_, _) => const Scaffold(body: Text('AUTH')),
          ),
        ],
      );
      addTearDown(router.dispose);
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            myStoreRepositoryProvider.overrideWithValue(
              (store ?? FakeMyStoreServer.absent()).repository,
            ),
            authRepositoryProvider.overrideWithValue(
              _AuthRecorder(events, state, deleteError),
            ),
            sessionProvider.overrideWith(() => _LogoutRecorder(events)),
          ],
          child: MaterialApp.router(routerConfig: router),
        ),
      );

      await tester.tap(find.text('刪除帳號').last);
      await tester.pumpAndSettle();
      await tester.tap(find.text('確認刪除'));
      await tester.pumpAndSettle();
      afterFlow?.call();
      await tester.pump(const Duration(seconds: 6));
      await tester.pumpAndSettle();
      return events;
    }

    testWidgets('刪除、登出', (tester) async {
      final events = await deleteWith(tester);

      expect(events, ['delete', 'logout']);
    });

    testWidgets('還有未完成的訂單：不登出', (tester) async {
      final events = await deleteWith(
        tester,
        state:
            DeleteAccountResponse_DeletionState.DELETION_STATE_PENDING_ORDERS,
      );

      expect(events, ['delete']);
    });

    testWidgets('ACCOUNT_HAS_OPEN_STORE：引導去關店，不登出', (tester) async {
      var wentToClose = 0;
      final events = await deleteWith(
        tester,
        store: FakeMyStoreServer(),
        deleteError: reasonError(
          Code.failedPrecondition,
          'ACCOUNT_HAS_OPEN_STORE',
        ),
        onGoToCloseStore: () => wentToClose++,
        afterFlow: () {
          expect(find.text('你的帳號還有商店，請先關閉商店，再刪除帳號。'), findsOneWidget);
        },
      );

      expect(events, ['delete']);
      expect(wentToClose, 0);
    });

    testWidgets('ACCOUNT_HAS_OPEN_STORE：按「前往關閉商店」', (tester) async {
      var wentToClose = 0;
      final events = <String>[];
      final router = GoRouter(
        routes: [
          GoRoute(
            path: '/',
            builder: (_, _) => Scaffold(
              body: ProfileDeleteAccountSection(
                onGoToCloseStore: () => wentToClose++,
              ),
            ),
          ),
        ],
      );
      addTearDown(router.dispose);
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            myStoreRepositoryProvider.overrideWithValue(
              FakeMyStoreServer().repository,
            ),
            authRepositoryProvider.overrideWithValue(
              _AuthRecorder(
                events,
                DeleteAccountResponse_DeletionState.DELETION_STATE_DELETED,
                reasonError(Code.failedPrecondition, 'ACCOUNT_HAS_OPEN_STORE'),
              ),
            ),
            sessionProvider.overrideWith(_LoggedIn.new),
          ],
          child: MaterialApp.router(routerConfig: router),
        ),
      );

      await tester.tap(find.text('刪除帳號').last);
      await tester.pumpAndSettle();
      await tester.tap(find.text('確認刪除'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('前往關閉商店'));
      await tester.pumpAndSettle();

      expect(wentToClose, 1);
      expect(events, ['delete']);
    });
  });

  Future<ProviderContainer> raceWith(
    WidgetTester tester,
    FakeMyStoreServer server,
    void Function() onDelete,
  ) async {
    final router = GoRouter(
      routes: [
        GoRoute(
          path: '/',
          builder: (_, _) =>
              const Scaffold(body: ProfileDeleteAccountSection()),
        ),
      ],
    );
    addTearDown(router.dispose);
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          clerkSessionProvider.overrideWith(
            (ref) async => FakeClerkSession(signedIn: true),
          ),
          sessionProvider.overrideWith(_LoggedIn.new),
          myStoreRepositoryProvider.overrideWithValue(server.repository),
          authRepositoryProvider.overrideWithValue(_OpenStoreRace(onDelete)),
        ],
        child: MaterialApp.router(routerConfig: router),
      ),
    );
    final container = ProviderScope.containerOf(
      tester.element(find.byType(ProfileDeleteAccountSection)),
    );
    final sub = container.listen(currentStoreProvider, (_, _) {});
    addTearDown(sub.close);
    await container.read(currentStoreProvider.future);
    expect(container.read(myStoreSnapshotProvider)!.hasStore, isFalse);

    await tester.tap(find.text('刪除帳號').last);
    await tester.pumpAndSettle();
    await tester.tap(find.text('確認刪除'));
    await tester.pumpAndSettle();
    return container;
  }

  testWidgets('DeleteAccount 回 ACCOUNT_HAS_OPEN_STORE：先重讀店家，再引導關店', (
    tester,
  ) async {
    final server = FakeMyStoreServer.absent();
    final container = await raceWith(tester, server, () {
      server.failure = null;
      server.response = myStoreResponse();
    });

    expect(find.text('你的帳號還有商店，請先關閉商店，再刪除帳號。'), findsOneWidget);
    expect(find.text('前往關閉商店'), findsOneWidget);
    expect(container.read(myStoreSnapshotProvider)!.hasStore, isTrue);
  });

  testWidgets('重讀店家失敗：顯示無法確認商店狀態，不顯示引導', (tester) async {
    final server = FakeMyStoreServer.absent();
    await raceWith(tester, server, () {
      server.failure = ConnectException(Code.unavailable, 'try later');
    });

    expect(find.text(storeStateUnavailableNotice), findsOneWidget);
    expect(find.text('前往關閉商店'), findsNothing);
    await tester.pump(const Duration(seconds: 6));
  });

  testWidgets('重讀後沒有 ACCOUNT_CLOSE：不出現「前往關閉商店」', (tester) async {
    final server = FakeMyStoreServer.absent();
    await raceWith(tester, server, () {
      server.failure = null;
      server.response = myStoreResponse(
        capabilities: pendingDeletionCapabilities,
      );
    });

    expect(find.text('前往關閉商店'), findsNothing);
    expect(find.text('你的帳號還有商店，請先關閉商店，再刪除帳號。'), findsOneWidget);
    await tester.pump(const Duration(seconds: 6));
  });
}

class _LoggedIn extends Session {
  @override
  SessionModel build() =>
      const SessionModel(accountId: 'acc-1', isLoggedIn: true);
}

class _OpenStoreRace extends Fake implements AuthRepository {
  _OpenStoreRace(this.onDelete);

  final void Function() onDelete;

  @override
  Future<DeleteAccountResponse_DeletionState> deleteAccount() async {
    onDelete();
    throw reasonError(Code.failedPrecondition, 'ACCOUNT_HAS_OPEN_STORE');
  }
}

class _AuthRecorder extends Fake implements AuthRepository {
  _AuthRecorder(this.events, this.state, [this.error]);

  final List<String> events;
  final DeleteAccountResponse_DeletionState state;
  final Object? error;

  @override
  Future<DeleteAccountResponse_DeletionState> deleteAccount() async {
    events.add('delete');
    if (error case final error?) throw error;
    return state;
  }
}

class _LogoutRecorder extends Session {
  _LogoutRecorder(this.events);

  final List<String> events;

  @override
  SessionModel build() =>
      const SessionModel(accountId: 'acc-1', isLoggedIn: true);

  @override
  Future<void> logout() async => events.add('logout');
}
