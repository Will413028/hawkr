import 'dart:io';

import 'package:connectrpc/connect.dart';
import 'package:hawkr_api/account.dart';
import 'package:hawkr_api/gen/hawkr/identity/v1alpha1/identity.pb.dart';
import 'package:test/test.dart';

const _deleted = DeleteAccountResponse_DeletionState.DELETION_STATE_DELETED;
const _pending =
    DeleteAccountResponse_DeletionState.DELETION_STATE_PENDING_ORDERS;

Object _noResponse() => ConnectException.from(const SocketException('down'));

Future<AccountDeletionResult> _run(
  Future<DeleteAccountResponse_DeletionState> Function() delete, {
  Future<bool> Function()? exists,
}) => classifyAccountDeletion(
  deleteAccount: delete,
  accountExists: exists ?? () async => throw StateError('不該確認'),
  defaults: _defaults,
);

String _defaults(Code code) => '預設:${code.name}';

void main() {
  group('classifyAccountDeletion', () {
    test('DELETED、PENDING_ORDERS', () async {
      expect(await _run(() async => _deleted), isA<AccountDeletionDeleted>());
      expect(
        await _run(() async => _pending),
        isA<AccountDeletionPendingOrders>(),
      );
    });

    test('未指定的狀態是失敗，沒有伺服器文字', () async {
      final r = await _run(
        () async =>
            DeleteAccountResponse_DeletionState.DELETION_STATE_UNSPECIFIED,
      );
      expect(r, isA<AccountDeletionFailure>());
      expect((r as AccountDeletionFailure).message, isNull);
    });

    test('沒收到回應：GetSession unauthenticated 是已刪除', () async {
      expect(
        await _run(() async => throw _noResponse(), exists: () async => false),
        isA<AccountDeletionDeleted>(),
      );
    });

    test('逾時：GetSession 有回應是尚未刪除', () async {
      expect(
        await _run(
          () async => throw ConnectException(Code.deadlineExceeded, 'slow'),
          exists: () async => true,
        ),
        isA<AccountDeletionNotDeleted>(),
      );
    });

    test('沒收到回應，GetSession 也沒收到回應', () async {
      expect(
        await _run(
          () async => throw _noResponse(),
          exists: () async => throw _noResponse(),
        ),
        isA<AccountDeletionUnconfirmed>(),
      );
    });

    test('其他錯誤帶伺服器的文字，沒有文字時為 null；DeleteAccount 只呼叫一次', () async {
      var calls = 0;
      final r = await _run(() async {
        calls++;
        throw ConnectException(Code.internal, 'boom');
      });
      expect(r, isA<AccountDeletionFailure>());
      expect(calls, 1);
    });

    test('onError 收到每一步的失敗', () async {
      final steps = <String>[];
      await classifyAccountDeletion(
        deleteAccount: () async => throw _noResponse(),
        accountExists: () async => throw _noResponse(),
        defaults: _defaults,
        onError: (step, _) => steps.add(step),
      );
      expect(steps, ['DeleteAccount', 'GetSession']);
    });
  });
}
