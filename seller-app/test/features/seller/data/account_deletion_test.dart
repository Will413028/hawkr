import 'dart:io';

import 'package:hawkr_api/gen/hawkr/identity/v1alpha1/identity.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/identity/v1alpha1/identity.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/identity/v1alpha1/identity.pb.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_api/transport.dart';
import 'package:hawkr_seller/features/auth/data/auth_repository.dart';
import 'package:hawkr_seller/features/seller/data/account_deletion.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../../support/connect_errors.dart';
import '../../../support/default_error_text.dart';

ConnectException _offline() =>
    ConnectException.from(const SocketException('offline'));

void main() {
  late List<String> calls;
  late Object? Function() deleteResult;
  late Object? Function() sessionResult;

  AccountDeletion build() {
    final transport = FakeTransportBuilder()
        .unary(IdentityAppService.deleteAccount, (_, _) {
          calls.add('delete');
          final result = deleteResult();
          if (result is Exception) throw result;
          return DeleteAccountResponse(
            state: result as DeleteAccountResponse_DeletionState,
          );
        })
        .unary(IdentityAppService.getSession, (_, _) {
          calls.add('session');
          final result = sessionResult();
          if (result is Exception) throw result;
          return GetSessionResponse(account: Account(id: 'acc-1'));
        })
        .build();
    return AccountDeletion(
      auth: AuthRepository(
        IdentityAppServiceClient(transport),
        defaults: zhDefaultErrorText,
      ),
      defaults: zhDefaultErrorText,
    );
  }

  setUp(() {
    calls = [];
    deleteResult = () =>
        DeleteAccountResponse_DeletionState.DELETION_STATE_DELETED;
    sessionResult = () => 'ok';
  });

  group('順序', () {
    test('直接刪除', () async {
      final outcome = await build().run();

      expect(outcome, isA<AccountDeleted>());
      expect(calls, ['delete']);
    });
  });

  group('DeleteAccount 的結果', () {
    test('還有未完成的訂單', () async {
      deleteResult = () =>
          DeleteAccountResponse_DeletionState.DELETION_STATE_PENDING_ORDERS;

      expect(await build().run(), isA<AccountDeletionPending>());
    });

    test('狀態未指定：失敗', () async {
      deleteResult = () =>
          DeleteAccountResponse_DeletionState.DELETION_STATE_UNSPECIFIED;

      final outcome = await build().run();

      expect(
        outcome,
        isA<AccountDeletionFailed>().having(
          (o) => o.message,
          'message',
          deleteAccountFailedMessage,
        ),
      );
    });

    test('ACCOUNT_HAS_OPEN_STORE（讀店之後才開的店）：引導關店', () async {
      deleteResult = () =>
          reasonError(Code.failedPrecondition, 'ACCOUNT_HAS_OPEN_STORE');

      final outcome = await build().run();

      expect(outcome, isA<AccountHasOpenStore>());
      expect(calls, ['delete']);
    });

    test('其他錯誤：沒有訊息時用預設文字', () async {
      deleteResult = () => ConnectException(Code.failedPrecondition, 'x');

      final outcome = await build().run();

      expect(outcome, isA<AccountDeletionFailed>());
      expect(calls, ['delete']);
    });
  });

  group('沒收到 DeleteAccount 的回應', () {
    setUp(() => deleteResult = _offline);

    test('GetSession 回 unauthenticated：已刪除', () async {
      sessionResult = () => ConnectException(Code.unauthenticated, 'gone');

      expect(await build().run(), isA<AccountDeleted>());
      expect(calls, ['delete', 'session']);
    });

    test('GetSession 有回應：帳號還在', () async {
      final outcome = await build().run();

      expect(
        outcome,
        isA<AccountDeletionFailed>().having(
          (o) => o.message,
          'message',
          accountNotDeletedMessage,
        ),
      );
    });

    test('client 逾時（deadline_exceeded，沒有 cause）：同樣以 GetSession 確認', () async {
      deleteResult = () =>
          ConnectException(Code.deadlineExceeded, 'operation timed out');
      sessionResult = () => ConnectException(Code.unauthenticated, 'gone');

      expect(await build().run(), isA<AccountDeleted>());
      expect(calls, ['delete', 'session']);
    });

    test('伺服器送來 deadline_exceeded：帳號還在時顯示尚未刪除', () async {
      deleteResult = () => reasonError(Code.deadlineExceeded, 'SOMETHING');

      final outcome = await build().run();

      expect(
        outcome,
        isA<AccountDeletionFailed>().having(
          (o) => o.message,
          'message',
          accountNotDeletedMessage,
        ),
      );
      expect(calls, ['delete', 'session']);
    });

    test('GetSession 逾時：無法確認', () async {
      deleteResult = () => ConnectException(Code.deadlineExceeded, 'timeout');
      sessionResult = () => ConnectException(Code.deadlineExceeded, 'timeout');

      final outcome = await build().run();

      expect(
        outcome,
        isA<AccountDeletionFailed>().having(
          (o) => o.message,
          'message',
          accountDeletionUnconfirmedMessage,
        ),
      );
    });

    test('GetSession 也沒有回應：無法確認', () async {
      sessionResult = _offline;

      final outcome = await build().run();

      expect(
        outcome,
        isA<AccountDeletionFailed>().having(
          (o) => o.message,
          'message',
          accountDeletionUnconfirmedMessage,
        ),
      );
    });
  });
}
