import 'dart:async';

import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_api/transport.dart';
import 'package:hawkr_seller/core/networking/connect_provider.dart';
import 'package:hawkr_seller/core/session/session_provider.dart';
import 'package:hawkr_seller/features/seller/data/current_kyc.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

class _LoggedIn extends Session {
  _LoggedIn(this.accountId);
  final String accountId;

  @override
  SessionModel build() => SessionModel(accountId: accountId, isLoggedIn: true);
}

class _Switchable extends Session {
  @override
  SessionModel build() =>
      const SessionModel(accountId: 'acc-1', isLoggedIn: true);

  void switchTo(String accountId) =>
      state = SessionModel(accountId: accountId, isLoggedIn: true);
}

class _LoggedOut extends Session {
  @override
  SessionModel build() => const SessionModel();
}

class _Server {
  Object? answer;
  int calls = 0;

  ProviderContainer container({Session Function()? session}) {
    final container = ProviderContainer(
      overrides: [
        sessionProvider.overrideWith(session ?? () => _LoggedIn('acc-1')),
        sellerClientProvider.overrideWithValue(
          SellerAppServiceClient(
            FakeTransportBuilder().unary(SellerAppService.getMyKYC, (_, _) {
              calls++;
              final result = answer;
              if (result is ConnectException) throw result;
              return result as GetMyKYCResponse;
            }).build(),
          ),
        ),
      ],
    );
    addTearDown(container.dispose);
    return container;
  }
}

GetMyKYCResponse _flow(KYCFlowStatus status, {bool identityLocked = false}) =>
    GetMyKYCResponse(
      application: KYCApplication(
        flow: KYCFlow(status: status, identityLocked: identityLocked),
      ),
    );

void main() {
  test('讀到流程狀態、identity_locked', () async {
    final server = _Server()
      ..answer = _flow(
        KYCFlowStatus.KYC_FLOW_STATUS_FORWARDED,
        identityLocked: true,
      );
    final kyc = await server.container().read(
      accountKycProvider('acc-1').future,
    );

    expect(kyc.status, KYCFlowStatus.KYC_FLOW_STATUS_FORWARDED);
    expect(kyc.identityLocked, isTrue);
    expect(kyc.status, isNot(KYCFlowStatus.KYC_FLOW_STATUS_APPROVED));
  });

  test('讀到整份申請（etag、欄位、缺項），沒有申請時 application 是 null', () async {
    final server = _Server()
      ..answer = GetMyKYCResponse(
        application: KYCApplication(
          etag: 'v7',
          realName: '王小明',
          missingFields: [KYCField.KYC_FIELD_BANK_ACCOUNT],
          flow: KYCFlow(status: KYCFlowStatus.KYC_FLOW_STATUS_DRAFT),
        ),
      );
    final kyc = await server.container().read(
      accountKycProvider('acc-1').future,
    );

    expect(kyc.application?.etag, 'v7');
    expect(kyc.application?.realName, '王小明');
    expect(kyc.application?.missingFields, [KYCField.KYC_FIELD_BANK_ACCOUNT]);

    final none = _Server()..answer = ConnectException(Code.notFound, 'none');
    final notStarted = await none.container().read(
      accountKycProvider('acc-1').future,
    );
    expect(notStarted.application, isNull);
  });

  test('adopt：寫入回的申請直接成為目前的狀態，不再呼叫 GetMyKYC；null 是申請已取消', () async {
    final server = _Server()
      ..answer = _flow(KYCFlowStatus.KYC_FLOW_STATUS_REJECTED);
    final container = server.container();
    final sub = container.listen(currentKycProvider, (_, _) {});
    addTearDown(sub.close);
    await container.read(accountKycProvider('acc-1').future);
    expect(server.calls, 1);

    container
        .read(kycReaderProvider.notifier)
        .adopt(
          KYCApplication(
            etag: 'v2',
            flow: KYCFlow(status: KYCFlowStatus.KYC_FLOW_STATUS_DRAFT),
          ),
        );

    final adopted = container.read(currentKycProvider)!.value!;
    expect(adopted.status, KYCFlowStatus.KYC_FLOW_STATUS_DRAFT);
    expect(adopted.application?.etag, 'v2');
    expect(server.calls, 1);

    container.read(kycReaderProvider.notifier).adopt(null);
    expect(
      container.read(currentKycProvider)!.value!.status,
      KYCFlowStatus.KYC_FLOW_STATUS_NOT_STARTED,
    );
    expect(server.calls, 1);
  });

  test('adopt 收到沒有有效流程狀態的回應：退回重讀，不寫入壞資料', () async {
    final server = _Server()
      ..answer = _flow(KYCFlowStatus.KYC_FLOW_STATUS_DRAFT);
    final container = server.container();
    final sub = container.listen(currentKycProvider, (_, _) {});
    addTearDown(sub.close);
    await container.read(accountKycProvider('acc-1').future);

    container.read(kycReaderProvider.notifier).adopt(KYCApplication());
    await container.read(accountKycProvider('acc-1').future);

    expect(server.calls, 2);
    expect(
      container.read(currentKycProvider)!.value!.status,
      KYCFlowStatus.KYC_FLOW_STATUS_DRAFT,
    );
  });

  test('只有 APPROVED 算已通過', () async {
    final server = _Server()
      ..answer = _flow(KYCFlowStatus.KYC_FLOW_STATUS_APPROVED);
    final kyc = await server.container().read(
      accountKycProvider('acc-1').future,
    );

    expect(kyc.status, KYCFlowStatus.KYC_FLOW_STATUS_APPROVED);
  });

  test('not_found（沒有申請）是 NOT_STARTED，不是錯誤', () async {
    final server = _Server()..answer = ConnectException(Code.notFound, 'none');
    final kyc = await server.container().read(
      accountKycProvider('acc-1').future,
    );

    expect(kyc.status, KYCFlowStatus.KYC_FLOW_STATUS_NOT_STARTED);
    expect(kyc.identityLocked, isFalse);
    expect(kyc.status, isNot(KYCFlowStatus.KYC_FLOW_STATUS_APPROVED));
  });

  test('暫時失敗是 error 狀態，不當成 NOT_STARTED', () async {
    final server = _Server()
      ..answer = ConnectException(Code.unavailable, 'later');
    final container = server.container();
    final sub = container.listen(currentKycProvider, (_, _) {});
    addTearDown(sub.close);

    await expectLater(
      container.read(accountKycProvider('acc-1').future),
      throwsA(isA<ConnectException>()),
    );
    final state = container.read(currentKycProvider)!;
    expect(state.hasError, isTrue);
    expect(state.value, isNull);
  });

  test('UNSPECIFIED 與沒有流程的回應是 error 狀態', () async {
    final unspecified = _Server()
      ..answer = _flow(KYCFlowStatus.KYC_FLOW_STATUS_UNSPECIFIED);
    await expectLater(
      unspecified.container().read(accountKycProvider('acc-1').future),
      throwsFormatException,
    );

    final noFlow = _Server()
      ..answer = GetMyKYCResponse(application: KYCApplication());
    await expectLater(
      noFlow.container().read(accountKycProvider('acc-1').future),
      throwsFormatException,
    );
  });

  test('currentKyc 跟著登入的帳號；沒有登入是 null', () async {
    final server = _Server()
      ..answer = _flow(KYCFlowStatus.KYC_FLOW_STATUS_SUBMITTED);
    final container = server.container();
    final sub = container.listen(currentKycProvider, (_, _) {});
    addTearDown(sub.close);
    await container.read(accountKycProvider('acc-1').future);

    expect(
      container.read(currentKycProvider)?.value?.status,
      KYCFlowStatus.KYC_FLOW_STATUS_SUBMITTED,
    );

    final loggedOut = _Server().container(session: _LoggedOut.new);
    expect(loggedOut.read(currentKycProvider), isNull);
    expect(await loggedOut.read(kycReaderProvider.notifier).reload(), isNull);
  });

  test('reload 重讀並回傳最新狀態；失敗時丟出錯誤，refresh 回 false', () async {
    final server = _Server()
      ..answer = _flow(KYCFlowStatus.KYC_FLOW_STATUS_SUBMITTED);
    final container = server.container();
    final sub = container.listen(currentKycProvider, (_, _) {});
    addTearDown(sub.close);
    await container.read(accountKycProvider('acc-1').future);
    final reader = container.read(kycReaderProvider.notifier);

    server.answer = _flow(KYCFlowStatus.KYC_FLOW_STATUS_APPROVED);
    final reloaded = await reader.reload();
    expect(reloaded?.status, KYCFlowStatus.KYC_FLOW_STATUS_APPROVED);
    expect(server.calls, 2);
    expect(
      container.read(currentKycProvider)?.value?.status,
      KYCFlowStatus.KYC_FLOW_STATUS_APPROVED,
    );

    server.answer = ConnectException(Code.unavailable, 'later');
    await expectLater(reader.reload(), throwsA(isA<ConnectException>()));
    expect(await reader.refresh(), isFalse);
  });

  test('換帳號：acc-1 的結果（含遲到的回應）不會出現在 acc-2', () async {
    final acc1Gate = Completer<void>();
    var calls = 0;
    final container = ProviderContainer(
      overrides: [
        sessionProvider.overrideWith(_Switchable.new),
        sellerClientProvider.overrideWithValue(
          SellerAppServiceClient(
            FakeTransportBuilder().unary(SellerAppService.getMyKYC, (
              _,
              _,
            ) async {
              final call = ++calls;
              if (call == 1) {
                await acc1Gate.future;
                return _flow(
                  KYCFlowStatus.KYC_FLOW_STATUS_APPROVED,
                  identityLocked: true,
                );
              }
              return _flow(KYCFlowStatus.KYC_FLOW_STATUS_DRAFT);
            }).build(),
          ),
        ),
      ],
    );
    addTearDown(container.dispose);
    final sub = container.listen(currentKycProvider, (_, _) {});
    addTearDown(sub.close);

    expect(container.read(currentKycProvider)?.isLoading, isTrue);

    (container.read(sessionProvider.notifier) as _Switchable).switchTo('acc-2');
    final acc2 = await container.read(accountKycProvider('acc-2').future);
    expect(acc2.status, KYCFlowStatus.KYC_FLOW_STATUS_DRAFT);
    expect(
      container.read(currentKycProvider)?.value?.status,
      KYCFlowStatus.KYC_FLOW_STATUS_DRAFT,
    );

    acc1Gate.complete();
    final acc1 = await container.read(accountKycProvider('acc-1').future);
    expect(acc1.status, KYCFlowStatus.KYC_FLOW_STATUS_APPROVED);
    expect(
      container.read(currentKycProvider)?.value?.status,
      KYCFlowStatus.KYC_FLOW_STATUS_DRAFT,
    );
    expect(container.read(currentKycProvider)?.value?.identityLocked, isFalse);
  });
}
