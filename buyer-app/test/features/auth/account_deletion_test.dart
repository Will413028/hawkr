import 'dart:async';
import 'dart:io';

import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/identity/v1alpha1/identity.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/identity/v1alpha1/identity.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/identity/v1alpha1/identity.pb.dart';
import 'package:hawkr_api/gen/google/rpc/error_details.pb.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_api/transport.dart'
    show Code, ConnectException, ErrorDetail;
import 'package:hawkr_buyer/features/auth/data/account_deletion.dart';
import 'package:hawkr_buyer/features/auth/data/auth_repository.dart';
import 'package:flutter_test/flutter_test.dart';
import '../../support/default_error_text.dart';

/// 刪除帳號的結果分類：DeleteAccount 的回應、錯誤，以及沒有回應時以 GetSession 確認。
void main() {
  ({AccountDeletion deletion, List<String> calls}) build({
    required FutureOr<DeleteAccountResponse> Function() deleteAccount,
    FutureOr<GetSessionResponse> Function()? getSession,
  }) {
    final calls = <String>[];
    final transport = FakeTransportBuilder()
        .unary(IdentityAppService.deleteAccount, (_, _) {
          calls.add('delete');
          return deleteAccount();
        })
        .unary(IdentityAppService.getSession, (_, _) {
          calls.add('getSession');
          return getSession?.call() ??
              GetSessionResponse(account: Account(id: 'acc'));
        })
        .build();
    final auth = AuthRepository(
      IdentityAppServiceClient(transport),
      BuyerAppServiceClient(FakeTransportBuilder().build()),
      defaults: zhDefaultErrorText,
    );
    return (
      deletion: AccountDeletion(auth: auth, defaults: zhDefaultErrorText),
      calls: calls,
    );
  }

  ConnectException noResponse() =>
      ConnectException.from(const SocketException('offline'));

  ConnectException unauthenticated() =>
      ConnectException(Code.unauthenticated, 'token expired');

  test('DELETED：已刪除', () async {
    final t = build(
      deleteAccount: () => DeleteAccountResponse(
        state: DeleteAccountResponse_DeletionState.DELETION_STATE_DELETED,
      ),
    );

    expect(await t.deletion.run(), isA<AccountDeleted>());
    expect(t.calls, ['delete']);
  });

  test('PENDING_ORDERS：處理中', () async {
    final t = build(
      deleteAccount: () => DeleteAccountResponse(
        state:
            DeleteAccountResponse_DeletionState.DELETION_STATE_PENDING_ORDERS,
      ),
    );

    expect(await t.deletion.run(), isA<AccountDeletionPending>());
  });

  test('未指定的狀態：失敗', () async {
    final t = build(deleteAccount: DeleteAccountResponse.new);

    final outcome = await t.deletion.run();

    expect(
      outcome,
      isA<AccountDeletionFailed>().having(
        (o) => o.message,
        'message',
        deleteAccountFailedMessage,
      ),
    );
  });

  test('ACCOUNT_HAS_OPEN_STORE：要先關閉商店', () async {
    final t = build(
      deleteAccount: () => throw ConnectException(
        Code.failedPrecondition,
        'open store',
        details: [
          ErrorDetail(
            'google.rpc.ErrorInfo',
            (ErrorInfo()
                  ..reason = 'ACCOUNT_HAS_OPEN_STORE'
                  ..domain = 'hawkr')
                .writeToBuffer(),
          ),
        ],
      ),
    );

    expect(await t.deletion.run(), isA<AccountHasOpenStore>());
    expect(t.calls, ['delete']);
  });

  test('其他錯誤：顯示伺服器給的文字', () async {
    final t = build(
      deleteAccount: () => throw ConnectException(
        Code.permissionDenied,
        'denied',
        details: [
          ErrorDetail(
            'google.rpc.LocalizedMessage',
            (LocalizedMessage()
                  ..locale = 'zh-TW'
                  ..message = '帳號已停權')
                .writeToBuffer(),
          ),
        ],
      ),
    );

    expect(
      await t.deletion.run(),
      isA<AccountDeletionFailed>().having((o) => o.message, 'message', '帳號已停權'),
    );
  });

  test('沒有回應：不重送 DeleteAccount，GetSession unauthenticated 視為已刪除', () async {
    final t = build(
      deleteAccount: () => throw noResponse(),
      getSession: () => throw unauthenticated(),
    );

    expect(await t.deletion.run(), isA<AccountDeleted>());
    expect(t.calls, ['delete', 'getSession']);
  });

  test('逾時：GetSession 有回應表示帳號還在', () async {
    final t = build(
      deleteAccount: () =>
          throw ConnectException(Code.deadlineExceeded, 'slow'),
    );

    expect(
      await t.deletion.run(),
      isA<AccountDeletionFailed>().having(
        (o) => o.message,
        'message',
        accountNotDeletedMessage,
      ),
    );
    expect(t.calls, ['delete', 'getSession']);
  });

  test('GetSession 也沒有回應：無法確認', () async {
    final t = build(
      deleteAccount: () => throw noResponse(),
      getSession: () => throw noResponse(),
    );

    expect(
      await t.deletion.run(),
      isA<AccountDeletionFailed>().having(
        (o) => o.message,
        'message',
        accountDeletionUnconfirmedMessage,
      ),
    );
    expect(t.calls, ['delete', 'getSession']);
  });
}
