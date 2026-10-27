import 'dart:async';

import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_api/transport.dart';
import 'package:hawkr_seller/core/networking/connect_provider.dart';
import 'package:hawkr_seller/core/session/session_provider.dart';
import 'package:hawkr_seller/features/seller/data/current_kyc.dart';
import 'package:hawkr_seller/features/seller/data/current_store.dart';
import 'package:hawkr_seller/features/seller/data/my_store_repository.dart';
import 'package:hawkr_seller/features/seller/data/seller_status_refresh.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../../support/fake_my_store.dart';

class _LoggedIn extends Session {
  @override
  SessionModel build() =>
      const SessionModel(accountId: 'acc-1', isLoggedIn: true);
}

class _Counters {
  int kyc = 0;
  final store = FakeMyStoreServer();

  ProviderContainer container() {
    final container = ProviderContainer(
      overrides: [
        sessionProvider.overrideWith(_LoggedIn.new),
        myStoreRepositoryProvider.overrideWithValue(store.repository),
        sellerClientProvider.overrideWithValue(
          SellerAppServiceClient(
            FakeTransportBuilder().unary(SellerAppService.getMyKYC, (_, _) {
              kyc++;
              throw ConnectException(Code.notFound, 'none');
            }).build(),
          ),
        ),
      ],
    );
    addTearDown(container.dispose);
    final storeSub = container.listen(currentStoreProvider, (_, _) {});
    final kycSub = container.listen(currentKycProvider, (_, _) {});
    addTearDown(storeSub.close);
    addTearDown(kycSub.close);
    return container;
  }
}

class _KycServer {
  final gates = <Completer<void>>[];
  var calls = 0;
  var active = 0;
  var maxConcurrent = 0;

  GetMyKYCResponse answer(int call) => GetMyKYCResponse(
    application: KYCApplication(
      flow: KYCFlow(
        status: call == 1
            ? KYCFlowStatus.KYC_FLOW_STATUS_DRAFT
            : KYCFlowStatus.KYC_FLOW_STATUS_SUBMITTED,
      ),
    ),
  );

  ProviderContainer container() {
    final store = FakeMyStoreServer();
    final container = ProviderContainer(
      overrides: [
        sessionProvider.overrideWith(_LoggedIn.new),
        myStoreRepositoryProvider.overrideWithValue(store.repository),
        sellerClientProvider.overrideWithValue(
          SellerAppServiceClient(
            FakeTransportBuilder().unary(SellerAppService.getMyKYC, (
              _,
              _,
            ) async {
              final mine = ++calls;
              active++;
              if (active > maxConcurrent) maxConcurrent = active;
              if (gates.length >= mine) await gates[mine - 1].future;
              active--;
              return answer(mine);
            }).build(),
          ),
        ),
      ],
    );
    addTearDown(container.dispose);
    final kycSub = container.listen(currentKycProvider, (_, _) {});
    final storeSub = container.listen(currentStoreProvider, (_, _) {});
    addTearDown(kycSub.close);
    addTearDown(storeSub.close);
    return container;
  }
}

void main() {
  test('讀取途中又完整重讀：結束時有一次在這個呼叫之後才開始的讀取', () async {
    final server = _KycServer();
    final container = server.container();
    await container.read(accountKycProvider('acc-1').future);
    expect(server.calls, 1);
    final gate = Completer<void>();
    server.gates
      ..add(Completer<void>()..complete())
      ..add(gate);

    final first = container
        .read(sellerStatusRefreshProvider.notifier)
        .refresh();
    await Future<void>.delayed(Duration.zero);
    expect(server.calls, 2, reason: '第一次重讀的 KYC 讀取進行中');
    final second = container
        .read(sellerStatusRefreshProvider.notifier)
        .refresh(fresh: true);
    gate.complete();
    await first;
    await second;

    expect(server.calls, 3);
    final cached = container.read(accountKycProvider('acc-1')).requireValue;
    expect(cached.status, KYCFlowStatus.KYC_FLOW_STATUS_SUBMITTED);
  });

  test('前一輪結束與後續一輪開始之間，非 fresh 呼叫不會另開一輪（同時最多一輪）', () async {
    final server = _KycServer();
    final container = server.container();
    await container.read(accountKycProvider('acc-1').future);
    final gate = Completer<void>();
    server.gates
      ..add(Completer<void>()..complete())
      ..add(gate)
      ..add(Completer<void>()..complete());
    server.calls = 1;

    final notifier = container.read(sellerStatusRefreshProvider.notifier);
    final first = notifier.refresh();
    await Future<void>.delayed(Duration.zero);
    final trailing = notifier.refresh(fresh: true);
    late Future<bool> landed;
    unawaited(first.then((_) => landed = notifier.refresh()));
    gate.complete();
    await Future.wait([first, trailing]);
    await landed;

    expect(server.maxConcurrent, 1);
    expect(server.calls, 3, reason: '初始 1 + 第一輪 + 共用的後續一輪；落在中間的呼叫共用後續那輪');
  });

  test('進行中收到兩次推播：只多讀一次', () async {
    final server = _KycServer();
    final container = server.container();
    await container.read(accountKycProvider('acc-1').future);
    final gate = Completer<void>();
    server.gates
      ..add(Completer<void>()..complete())
      ..add(gate);

    final notifier = container.read(sellerStatusRefreshProvider.notifier);
    final first = notifier.refresh();
    await Future<void>.delayed(Duration.zero);
    final second = notifier.refresh(fresh: true);
    final third = notifier.refresh(fresh: true);
    gate.complete();
    await Future.wait([first, second, third]);

    expect(server.calls, 3, reason: '1 次初始 + 1 次進行中 + 1 次共用的後續');
  });

  test('回前景、切回分頁等非變更觸發的重讀：進行中時共用同一輪，三個呼叫只讀一輪', () async {
    final storeGate = Completer<void>();
    final kycGate = Completer<void>();
    final store = FakeMyStoreServer();
    var kycCalls = 0;
    final container = ProviderContainer(
      overrides: [
        sessionProvider.overrideWith(_LoggedIn.new),
        myStoreRepositoryProvider.overrideWithValue(store.repository),
        sellerClientProvider.overrideWithValue(
          SellerAppServiceClient(
            FakeTransportBuilder().unary(SellerAppService.getMyKYC, (
              _,
              _,
            ) async {
              kycCalls++;
              await kycGate.future;
              throw ConnectException(Code.notFound, 'none');
            }).build(),
          ),
        ),
      ],
    );
    addTearDown(container.dispose);
    final subs = [
      container.listen(currentStoreProvider, (_, _) {}),
      container.listen(currentKycProvider, (_, _) {}),
    ];
    addTearDown(() {
      for (final sub in subs) {
        sub.close();
      }
    });
    kycGate.complete();
    await container.read(currentStoreProvider.future);
    await container.read(accountKycProvider('acc-1').future);
    final storeBefore = store.calls;
    final kycBefore = kycCalls;

    store.gate = storeGate.future;
    final notifier = container.read(sellerStatusRefreshProvider.notifier);
    final calls = [
      notifier.refresh(),
      notifier.refresh(settledOnly: true),
      notifier.refresh(),
    ];
    await Future<void>.delayed(Duration.zero);
    storeGate.complete();
    await Future.wait(calls);
    await Future<void>.delayed(Duration.zero);

    expect(store.calls, storeBefore + 1);
    expect(kycCalls, kycBefore + 1);
  });

  test('認證流程的寫入結果（adopt）不會被之後才完成的舊讀取蓋掉', () async {
    final stale = Completer<void>();
    var call = 0;
    final container = ProviderContainer(
      overrides: [
        sessionProvider.overrideWith(_LoggedIn.new),
        sellerClientProvider.overrideWithValue(
          SellerAppServiceClient(
            FakeTransportBuilder().unary(SellerAppService.getMyKYC, (
              _,
              _,
            ) async {
              if (++call == 1) await stale.future;
              return GetMyKYCResponse(
                application: KYCApplication(
                  etag: 'old',
                  flow: KYCFlow(status: KYCFlowStatus.KYC_FLOW_STATUS_DRAFT),
                ),
              );
            }).build(),
          ),
        ),
      ],
    );
    addTearDown(container.dispose);
    final sub = container.listen(currentKycProvider, (_, _) {});
    addTearDown(sub.close);
    await Future<void>.delayed(Duration.zero);

    container
        .read(kycReaderProvider.notifier)
        .adopt(
          KYCApplication(
            etag: 'new',
            flow: KYCFlow(status: KYCFlowStatus.KYC_FLOW_STATUS_SUBMITTED),
          ),
        );
    stale.complete();
    await Future<void>.delayed(Duration.zero);
    await Future<void>.delayed(Duration.zero);

    final kyc = container.read(accountKycProvider('acc-1')).requireValue;
    expect(kyc.application?.etag, 'new');
    expect(kyc.status, KYCFlowStatus.KYC_FLOW_STATUS_SUBMITTED);
  });

  test('讀取回來的流程狀態無效（FormatException）時，仍採用讀取期間 adopt 的結果', () async {
    final stale = Completer<void>();
    final container = ProviderContainer(
      overrides: [
        sessionProvider.overrideWith(_LoggedIn.new),
        sellerClientProvider.overrideWithValue(
          SellerAppServiceClient(
            FakeTransportBuilder().unary(SellerAppService.getMyKYC, (
              _,
              _,
            ) async {
              await stale.future;
              return GetMyKYCResponse(application: KYCApplication(etag: 'bad'));
            }).build(),
          ),
        ),
      ],
    );
    addTearDown(container.dispose);
    final sub = container.listen(currentKycProvider, (_, _) {});
    addTearDown(sub.close);
    await Future<void>.delayed(Duration.zero);

    container
        .read(kycReaderProvider.notifier)
        .adopt(
          KYCApplication(
            etag: 'new',
            flow: KYCFlow(status: KYCFlowStatus.KYC_FLOW_STATUS_SUBMITTED),
          ),
        );
    stale.complete();
    await Future<void>.delayed(Duration.zero);
    await Future<void>.delayed(Duration.zero);

    final kyc = container.read(accountKycProvider('acc-1'));
    expect(kyc.hasError, isFalse);
    expect(kyc.requireValue.application?.etag, 'new');
  });

  test('settledOnly 跳過進行中的第一次店家讀取時，回傳那次讀取的結果', () async {
    final gate = Completer<void>();
    final store = FakeMyStoreServer(
      failure: ConnectException(Code.unavailable, 'later'),
      gate: gate.future,
    );
    final container = ProviderContainer(
      overrides: [
        sessionProvider.overrideWith(_LoggedIn.new),
        myStoreRepositoryProvider.overrideWithValue(store.repository),
        sellerClientProvider.overrideWithValue(
          SellerAppServiceClient(
            FakeTransportBuilder().unary(SellerAppService.getMyKYC, (_, _) {
              throw ConnectException(Code.notFound, 'none');
            }).build(),
          ),
        ),
      ],
    );
    addTearDown(container.dispose);
    final storeSub = container.listen(currentStoreProvider, (_, _) {});
    addTearDown(storeSub.close);
    await Future<void>.delayed(Duration.zero);

    final result = container
        .read(sellerStatusRefreshProvider.notifier)
        .refresh(settledOnly: true);
    gate.complete();

    expect(await result, isFalse, reason: '店家讀取失敗，不能回報成功');
    expect(store.calls, 1);
  });

  test('一次重讀店家與認證流程（PayUni 狀態隨認證流程一起讀）', () async {
    final counters = _Counters();
    final container = counters.container();
    await container.read(currentStoreProvider.future);
    await container.read(accountKycProvider('acc-1').future);
    final storeBefore = counters.store.calls;
    final kycBefore = counters.kyc;

    final ok = await container
        .read(sellerStatusRefreshProvider.notifier)
        .refresh();

    expect(ok, isTrue);
    expect(counters.store.calls, storeBefore + 1);
    expect(counters.kyc, kycBefore + 1);
  });

  test('重讀進行中的第二次呼叫排一次後續重讀，完成後再呼叫才又重讀', () async {
    final storeGate = Completer<void>();
    final kycGate = Completer<void>();
    final store = FakeMyStoreServer();
    var kycCalls = 0;
    final container = ProviderContainer(
      overrides: [
        sessionProvider.overrideWith(_LoggedIn.new),
        myStoreRepositoryProvider.overrideWithValue(store.repository),
        sellerClientProvider.overrideWithValue(
          SellerAppServiceClient(
            FakeTransportBuilder().unary(SellerAppService.getMyKYC, (
              _,
              _,
            ) async {
              kycCalls++;
              await kycGate.future;
              throw ConnectException(Code.notFound, 'none');
            }).build(),
          ),
        ),
      ],
    );
    addTearDown(container.dispose);
    final storeSub = container.listen(currentStoreProvider, (_, _) {});
    final kycSub = container.listen(currentKycProvider, (_, _) {});
    addTearDown(storeSub.close);
    addTearDown(kycSub.close);
    await container.read(currentStoreProvider.future);
    kycGate.complete();
    await container.read(accountKycProvider('acc-1').future);

    final storeBefore = store.calls;
    final kycBefore = kycCalls;
    final refresh = container.read(sellerStatusRefreshProvider.notifier);
    store.gate = storeGate.future;

    final first = refresh.refresh();
    final second = refresh.refresh(fresh: true);
    await Future<void>.delayed(Duration.zero);
    storeGate.complete();
    await Future.wait([first, second]);

    expect(store.calls, storeBefore + 2);
    expect(kycCalls, kycBefore + 2);

    await refresh.refresh();
    expect(store.calls, storeBefore + 3);
    expect(kycCalls, kycBefore + 3);
  });
}
