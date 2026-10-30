import 'dart:io';

import 'package:hawkr_api/gen/hawkr/identity/v1alpha1/identity.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/identity/v1alpha1/identity.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/identity/v1alpha1/identity.pb.dart';
import 'package:hawkr_api/gen/google/rpc/error_details.pb.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_api/transport.dart';
import 'package:hawkr_seller/features/auth/data/auth_repository.dart';
import 'package:flutter_test/flutter_test.dart';
import '../../../support/default_error_text.dart';

/// 以假的 IdentityAppService 建 repository；handler 沒給的方法呼叫到就失敗。
AuthRepository _repo({GetSessionResponse Function()? getSession}) {
  var builder = FakeTransportBuilder();
  if (getSession != null) {
    builder = builder.unary(
      IdentityAppService.getSession,
      (_, _) => getSession(),
    );
  }
  return AuthRepository(
    IdentityAppServiceClient(builder.build()),
    defaults: zhDefaultErrorText,
  );
}

ErrorDetail _localized(String message) => ErrorDetail(
  'google.rpc.LocalizedMessage',
  (LocalizedMessage()
        ..locale = 'zh-TW'
        ..message = message)
      .writeToBuffer(),
);

void main() {
  group('currentAccountId（GetSession）', () {
    test('回傳不透明的帳號 ID', () async {
      final repo = _repo(
        getSession: () =>
            GetSessionResponse(account: Account(id: 'acc-opaque-1')),
      );

      expect(await repo.currentAccountId(), 'acc-opaque-1');
    });

    test('回應沒有帳號 → MalformedAuthResponse', () async {
      final repo = _repo(getSession: GetSessionResponse.new);

      await expectLater(
        repo.currentAccountId(),
        throwsA(isA<MalformedAuthResponse>()),
      );
    });

    test('停權（permission_denied）→ 顯示伺服器的文字', () async {
      final repo = _repo(
        getSession: () => throw ConnectException(
          Code.permissionDenied,
          'suspended',
          details: [_localized('帳號已停權')],
        ),
      );

      await expectLater(
        repo.currentAccountId(),
        throwsA(
          isA<AuthRequestException>()
              .having((e) => e.code, 'code', Code.permissionDenied)
              .having((e) => e.toString(), 'message', '帳號已停權'),
        ),
      );
    });

    test('停權但伺服器沒給句子 → 該 code 的預設文字，不顯示開發者訊息', () async {
      final repo = _repo(
        getSession: () =>
            throw ConnectException(Code.permissionDenied, 'DEV-ONLY detail'),
      );

      await expectLater(
        repo.currentAccountId(),
        throwsA(
          isA<AuthRequestException>().having(
            (e) => e.toString(),
            'message',
            zhDefaultErrorText(Code.permissionDenied),
          ),
        ),
      );
    });

    test('被取消 → internal 的預設文字，沒有寫死的句子', () async {
      final repo = _repo(
        getSession: () => throw ConnectException(Code.canceled, 'DEV-ONLY'),
      );

      await expectLater(
        repo.currentAccountId(),
        throwsA(
          isA<AuthRequestException>().having(
            (e) => e.toString(),
            'message',
            zhDefaultErrorText(Code.internal),
          ),
        ),
      );
    });

    test('沒收到回應 → 網路錯誤的文字', () async {
      final repo = _repo(
        getSession: () =>
            throw ConnectException.from(const SocketException('offline')),
      );

      await expectLater(
        repo.currentAccountId(),
        throwsA(
          isA<AuthRequestException>()
              .having((e) => e.noResponse, 'noResponse', isTrue)
              .having((e) => e.message, 'message', '網路連線失敗，請檢查網路設定'),
        ),
      );
    });
  });
}
