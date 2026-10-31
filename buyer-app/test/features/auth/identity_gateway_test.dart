import 'dart:async';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/google/rpc/error_details.pb.dart';
import 'package:hawkr_api/gen/hawkr/identity/v1alpha1/identity.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/identity/v1alpha1/identity.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/identity/v1alpha1/identity.pb.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_api/transport.dart'
    show Code, ConnectException, ErrorDetail;
import 'package:hawkr_auth/hawkr_auth.dart' as auth;
import 'package:hawkr_buyer/core/exceptions/app_exception.dart';
import 'package:hawkr_buyer/features/auth/data/identity_gateway.dart';

import '../../support/default_error_text.dart';

/// 登入時換發 session（IdentityPublicService）：請求欄位、回應對應與錯誤轉換。
void main() {
  final exchanges = <ExchangeIdTokenRequest>[];
  var nonceCalls = 0;

  IdentityLoginGateway gatewayWith({
    FutureOr<IssueAppLoginNonceResponse> Function()? issue,
    FutureOr<ExchangeIdTokenResponse> Function(ExchangeIdTokenRequest)?
    exchange,
  }) {
    final transport = FakeTransportBuilder()
        .unary(IdentityPublicService.issueAppLoginNonce, (_, _) {
          nonceCalls++;
          return issue!();
        })
        .unary(IdentityPublicService.exchangeIdToken, (req, _) {
          exchanges.add(req);
          return exchange!(req);
        })
        .build();
    return IdentityLoginGateway(
      IdentityPublicServiceClient(transport),
      defaults: zhDefaultErrorText,
    );
  }

  setUp(() {
    exchanges.clear();
    nonceCalls = 0;
  });

  group('issueNonce', () {
    test('回傳伺服器發的 nonce', () async {
      final gateway = gatewayWith(
        issue: () => IssueAppLoginNonceResponse(nonce: 'nonce-1'),
      );

      expect(await gateway.issueNonce(), 'nonce-1');
      expect(nonceCalls, 1);
    });

    test('失敗：ServerException，文字取自伺服器的 LocalizedMessage', () async {
      final gateway = gatewayWith(
        issue: () => throw ConnectException(
          Code.resourceExhausted,
          'DEV-ONLY limit',
          details: [
            ErrorDetail(
              'google.rpc.LocalizedMessage',
              (LocalizedMessage()
                    ..locale = 'zh-TW'
                    ..message = '請稍後再試')
                  .writeToBuffer(),
            ),
          ],
        ),
      );

      await expectLater(
        gateway.issueNonce(),
        throwsA(
          isA<ServerException>().having((e) => e.message, 'message', '請稍後再試'),
        ),
      );
    });
  });

  group('exchange', () {
    for (final (provider, expected) in [
      (auth.OAuthProvider.google, LoginProvider.LOGIN_PROVIDER_GOOGLE),
      (auth.OAuthProvider.line, LoginProvider.LOGIN_PROVIDER_LINE),
    ]) {
      test(
        '${provider.name}：送出對應的 provider、ID token、nonce，回傳 session token',
        () async {
          final gateway = gatewayWith(
            exchange: (_) => ExchangeIdTokenResponse(sessionToken: 'session-1'),
          );

          final token = await gateway.exchange(
            provider: provider,
            idToken: 'id-token-1',
            nonce: 'nonce-1',
          );

          expect(token, 'session-1');
          expect(exchanges, hasLength(1));
          expect(exchanges.single.provider, expected);
          expect(exchanges.single.idToken, 'id-token-1');
          expect(exchanges.single.nonce, 'nonce-1');
        },
      );
    }

    test('回應沒有 session token：MalformedLoginResponse', () async {
      final gateway = gatewayWith(exchange: (_) => ExchangeIdTokenResponse());

      await expectLater(
        gateway.exchange(
          provider: auth.OAuthProvider.google,
          idToken: 't',
          nonce: 'n',
        ),
        throwsA(isA<MalformedLoginResponse>()),
      );
    });

    test('unauthenticated 且伺服器沒給句子：預設文字，不含開發者訊息', () async {
      final gateway = gatewayWith(
        exchange: (_) =>
            throw ConnectException(Code.unauthenticated, 'DEV-ONLY bad nonce'),
      );

      await expectLater(
        gateway.exchange(
          provider: auth.OAuthProvider.line,
          idToken: 't',
          nonce: 'n',
        ),
        throwsA(
          isA<ServerException>()
              .having(
                (e) => e.message,
                'message',
                zhDefaultErrorText(Code.unauthenticated),
              )
              .having(
                (e) => e.message,
                'no developer text',
                isNot(contains('DEV-ONLY')),
              ),
        ),
      );
    });

    test('沒收到回應：NetworkException', () async {
      final gateway = gatewayWith(
        exchange: (_) =>
            throw ConnectException.from(const SocketException('offline')),
      );

      await expectLater(
        gateway.exchange(
          provider: auth.OAuthProvider.google,
          idToken: 't',
          nonce: 'n',
        ),
        throwsA(isA<NetworkException>()),
      );
    });
  });
}
