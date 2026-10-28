import 'package:hawkr_api/gen/hawkr/identity/v1alpha1/identity.pb.dart';
import 'package:hawkr_seller/core/session/session_provider.dart';
import 'package:hawkr_seller/features/auth/data/auth_repository.dart';
import 'package:hawkr_seller/features/seller/data/my_store_repository.dart';
import 'package:hawkr_seller/features/seller/presentation/profile_account_closure.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

import '../../../support/fake_my_store.dart';

void main() {
  group('刪除帳號', () {
    Future<List<String>> deleteWith(
      WidgetTester tester, {
      FakeMyStoreServer? store,
      DeleteAccountResponse_DeletionState state =
          DeleteAccountResponse_DeletionState.DELETION_STATE_DELETED,
      Object? deleteError,
      void Function()? onDialog,
      void Function()? afterFlow,
    }) async {
      final events = <String>[];
      final router = GoRouter(
        routes: [
          GoRoute(
            path: '/',
            builder: (_, _) => Scaffold(
              body: Center(
                child: ProfileDeleteAccountSection(),
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
      onDialog?.call();
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

    testWidgets('確認框告知商店會一起關閉', (tester) async {
      await deleteWith(
        tester,
        onDialog: () => expect(
          find.textContaining('你的商店也會一起關閉，商店與商品不再公開，賣家身分會移除。'),
          findsOneWidget,
        ),
      );
    });

    testWidgets('還有未完成的訂單：不登出', (tester) async {
      final events = await deleteWith(
        tester,
        state:
            DeleteAccountResponse_DeletionState.DELETION_STATE_PENDING_ORDERS,
      );

      expect(events, ['delete']);
    });
  });
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
