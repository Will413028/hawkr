import 'dart:async';
import 'dart:io';

import 'package:hawkr_api/gen/hawkr/identity/v1alpha1/identity.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/identity/v1alpha1/identity.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/identity/v1alpha1/identity.pb.dart';
import 'package:hawkr_api/gen/google/rpc/error_details.pb.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_api/transport.dart'
    show Code, ConnectException, ErrorDetail;
import 'package:hawkr_buyer/core/exceptions/app_exception.dart';
import 'package:hawkr_buyer/features/auth/data/auth_repository.dart';
import 'package:flutter_test/flutter_test.dart';
import '../../support/default_error_text.dart';

/// 登入之後的帳號（IdentityAppService/GetSession）：帳號 ID 與錯誤的轉換。
void main() {
  AuthRepository repositoryWith(
    FutureOr<GetSessionResponse> Function(GetSessionRequest) getSession,
  ) {
    final transport = FakeTransportBuilder()
        .unary(IdentityAppService.getSession, (req, _) => getSession(req))
        .build();
    return AuthRepository(
      IdentityAppServiceClient(transport),
      BuyerAppServiceClient(FakeTransportBuilder().build()),
      defaults: zhDefaultErrorText,
    );
  }

  test('回傳不透明的帳號 ID', () async {
    final repository = repositoryWith(
      (_) => GetSessionResponse(account: Account(id: '01J8ZQ4Y7K3M9V2N6P0R')),
    );

    expect(await repository.currentAccountId(), '01J8ZQ4Y7K3M9V2N6P0R');
  });

  test('停權：顯示伺服器的文字', () async {
    final repository = repositoryWith(
      (_) => throw ConnectException(
        Code.permissionDenied,
        'account suspended',
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

    await expectLater(
      repository.currentAccountId(),
      throwsA(
        isA<SuspendedException>().having((e) => e.message, 'message', '帳號已停權'),
      ),
    );
  });

  test('停權但伺服器沒給句子：顯示該 code 的預設文字，不顯示開發者訊息', () async {
    final repository = repositoryWith(
      (_) => throw ConnectException(Code.permissionDenied, 'DEV-ONLY detail'),
    );

    await expectLater(
      repository.currentAccountId(),
      throwsA(
        isA<SuspendedException>()
            .having(
              (e) => e.message,
              'message',
              zhDefaultErrorText(Code.permissionDenied),
            )
            .having(
              (e) => e.message,
              'no developer text',
              isNot(contains('DEV-ONLY')),
            ),
      ),
    );
  });

  test('getSession 被取消：文字是 internal 的預設文字，沒有動作專屬的寫死句子', () async {
    final repository = repositoryWith(
      (_) => throw ConnectException(Code.canceled, 'DEV-ONLY cancel'),
    );
    await expectLater(
      repository.currentAccountId(),
      throwsA(
        isA<NetworkException>().having(
          (e) => e.message,
          'message',
          zhDefaultErrorText(Code.internal),
        ),
      ),
    );
  });

  test(
    'GetProfile 被取消：不顯示開發者訊息，文字是 internal 的預設文字（目錄沒有 canceled 的文字）',
    () async {
      final repository = AuthRepository(
        IdentityAppServiceClient(FakeTransportBuilder().build()),
        BuyerAppServiceClient(
          FakeTransportBuilder()
              .unary(
                BuyerAppService.getProfile,
                (_, _) =>
                    throw ConnectException(Code.canceled, 'DEV-ONLY cancel'),
              )
              .build(),
        ),
        defaults: zhDefaultErrorText,
      );

      await expectLater(
        repository.getProfile(),
        throwsA(
          isA<ServerException>().having(
            (e) => e.message,
            'message',
            zhDefaultErrorText(Code.internal),
          ),
        ),
      );
    },
  );

  for (final code in [
    Code.canceled,
    Code.unknown,
    Code.deadlineExceeded,
    Code.resourceExhausted,
    Code.unimplemented,
    Code.internal,
    Code.unavailable,
    Code.dataLoss,
    Code.failedPrecondition,
    Code.aborted,
    Code.invalidArgument,
  ]) {
    test('$code 保留可恢復 session 的錯誤分類', () async {
      final repository = repositoryWith(
        (_) => throw ConnectException(code, 'retry'),
      );
      await expectLater(
        repository.currentAccountId(),
        throwsA(isA<NetworkException>()),
      );
    });
  }

  test('token 無效：UnauthorizedException', () async {
    final repository = repositoryWith(
      (_) => throw ConnectException(Code.unauthenticated, 'invalid token'),
    );

    await expectLater(
      repository.currentAccountId(),
      throwsA(isA<UnauthorizedException>()),
    );
  });

  test('沒收到回應：網路錯誤', () async {
    final repository = repositoryWith(
      (_) => throw ConnectException.from(const SocketException('offline')),
    );

    await expectLater(
      repository.currentAccountId(),
      throwsA(isA<NetworkException>()),
    );
  });

  test('回應沒有帳號：不符契約，往外丟', () async {
    final repository = repositoryWith((_) => GetSessionResponse());

    await expectLater(
      repository.currentAccountId(),
      throwsA(isA<MalformedAuthResponse>()),
    );
  });
}
