import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/payment/v1alpha1/payment.pbenum.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_api/transport.dart' show Code;
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:hawkr_seller/core/l10n/ui_locale.dart';
import 'package:hawkr_seller/core/networking/connect_provider.dart';
import 'package:hawkr_seller/core/session/session_provider.dart';
import 'package:hawkr_seller/features/seller/data/my_store_repository.dart';
import 'package:hawkr_seller/features/seller/data/payment_onboarding.dart';
import 'package:hawkr_seller/features/seller/presentation/payment_account_action.dart';

import '../../../support/connect_errors.dart';
import '../../../support/fake_my_store.dart';
import '../../../support/onboarding_fixtures.dart';

class _LoggedIn extends Session {
  @override
  SessionModel build() =>
      const SessionModel(accountId: 'acc-1', isLoggedIn: true);
}

/// 用 [usePaymentAccountActions] 的最小消費者。
class _Probe extends HookConsumerWidget {
  const _Probe(this.id);

  final int id;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final actions = usePaymentAccountActions(context, ref);
    return TextButton(
      key: Key('go-$id'),
      onPressed: actions.refresh,
      child: Text('go-$id'),
    );
  }
}

/// 失敗沒有伺服器句子，所以每次要顯示 toast 都會呼叫一次 defaultErrorText：呼叫次數就是 toast 次數。
Future<
  ({
    ProviderContainer container,
    ValueNotifier<int> probes,
    List<Code> toasts,
    void Function({required bool fails}) setRefreshFails,
  })
>
_pump(WidgetTester tester, {int probes = 2}) async {
  final toasts = <Code>[];
  var refreshFails = true;
  final container = ProviderContainer(
    overrides: [
      sessionProvider.overrideWith(_LoggedIn.new),
      myStoreRepositoryProvider.overrideWithValue(
        FakeMyStoreServer().repository,
      ),
      defaultErrorTextProvider.overrideWithValue((code) {
        toasts.add(code);
        return '錯誤';
      }),
      allowInsecureOnboardingUrlProvider.overrideWithValue(false),
      externalBrowserLauncherProvider.overrideWithValue((_) async => true),
      sellerClientProvider.overrideWithValue(
        SellerAppServiceClient(
          FakeTransportBuilder()
              .unary(SellerAppService.getMyOnboarding, (_, _) {
                return reviewPendingSummary();
              })
              .unary(SellerAppService.refreshMyPaymentAccount, (_, _) {
                if (refreshFails) throw reasonError(Code.unavailable, 'DOWN');
                return RefreshMyPaymentAccountResponse();
              })
              .unary(SellerAppService.createPaymentOnboardingSession, (_, _) {
                return CreatePaymentOnboardingSessionResponse(
                  surface: OnboardingSurface.ONBOARDING_SURFACE_HOSTED,
                  url: 'https://connect.example.test/setup/abc',
                );
              })
              .build(),
        ),
      ),
    ],
  );
  addTearDown(container.dispose);
  final count = ValueNotifier<int>(probes);
  addTearDown(count.dispose);
  await tester.pumpWidget(
    UncontrolledProviderScope(
      container: container,
      child: MaterialApp(
        home: Scaffold(
          body: ValueListenableBuilder<int>(
            valueListenable: count,
            builder: (_, n, _) =>
                Column(children: [for (var i = 0; i < n; i++) _Probe(i)]),
          ),
        ),
      ),
    ),
  );
  await tester.pumpAndSettle();
  return (
    container: container,
    probes: count,
    toasts: toasts,
    setRefreshFails: ({required fails}) => refreshFails = fails,
  );
}

void main() {
  testWidgets('按鈕觸發的 refresh 失敗：兩個消費者同時掛載也只有發起的那個顯示一次', (tester) async {
    final h = await _pump(tester);

    await tester.tap(find.byKey(const Key('go-0')));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));

    expect(h.toasts, [Code.unavailable]);
    await tester.pumpAndSettle(const Duration(seconds: 5));
  });

  testWidgets('失敗顯示過後，新掛載的消費者不再顯示舊錯誤，共享 state 也沒有殘留', (tester) async {
    final h = await _pump(tester);
    await tester.tap(find.byKey(const Key('go-0')));
    await tester.pumpAndSettle(const Duration(seconds: 5));
    expect(h.toasts, [Code.unavailable]);

    h.probes.value = 3;
    await tester.pumpAndSettle(const Duration(seconds: 5));
    expect(h.toasts, [Code.unavailable]);
    expect(h.container.read(paymentOnboardingProvider), isNull);

    final notifier = h.container.read(paymentOnboardingProvider.notifier);
    await notifier.start(OnboardingSurface.ONBOARDING_SURFACE_HOSTED);
    await notifier.onResumed();
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));
    expect(h.toasts, [Code.unavailable, Code.unavailable]);
    await tester.pumpAndSettle(const Duration(seconds: 5));
  });

  testWidgets('從瀏覽器回來的 refresh 失敗：沒有發起的 widget，兩個消費者只顯示一次，之後被消耗', (
    tester,
  ) async {
    final h = await _pump(tester);
    final notifier = h.container.read(paymentOnboardingProvider.notifier);
    await notifier.start(OnboardingSurface.ONBOARDING_SURFACE_HOSTED);

    await notifier.onResumed();
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));

    expect(h.toasts, [Code.unavailable]);
    expect(h.container.read(paymentOnboardingProvider), isNull);
    h.probes.value = 3;
    await tester.pumpAndSettle(const Duration(seconds: 5));
    expect(h.toasts, hasLength(1));
  });

  testWidgets('失敗時沒有消費者：之後成功的瀏覽器回來 refresh 清掉訊號，掛載的消費者不顯示舊錯誤', (tester) async {
    final h = await _pump(tester, probes: 0);
    final notifier = h.container.read(paymentOnboardingProvider.notifier);
    await notifier.start(OnboardingSurface.ONBOARDING_SURFACE_HOSTED);
    await notifier.onResumed();
    expect(h.container.read(paymentOnboardingProvider), isNotNull);

    h.setRefreshFails(fails: false);
    await notifier.start(OnboardingSurface.ONBOARDING_SURFACE_HOSTED);
    await notifier.onResumed();
    expect(h.container.read(paymentOnboardingProvider), isNull);

    h.probes.value = 2;
    await tester.pumpAndSettle(const Duration(seconds: 5));
    expect(h.toasts, isEmpty);
  });

  testWidgets('按鈕的成功 refresh 也清掉留著的訊號', (tester) async {
    final h = await _pump(tester, probes: 0);
    final notifier = h.container.read(paymentOnboardingProvider.notifier);
    await notifier.start(OnboardingSurface.ONBOARDING_SURFACE_HOSTED);
    await notifier.onResumed();
    expect(h.container.read(paymentOnboardingProvider), isNotNull);

    h.setRefreshFails(fails: false);
    final result = await notifier.confirmPaymentAccount();

    expect(result.refreshError, isNull);
    expect(h.container.read(paymentOnboardingProvider), isNull);
  });

  testWidgets('失敗時沒有消費者：之後掛載的消費者不顯示舊錯誤', (tester) async {
    final h = await _pump(tester, probes: 0);
    final notifier = h.container.read(paymentOnboardingProvider.notifier);
    await notifier.start(OnboardingSurface.ONBOARDING_SURFACE_HOSTED);
    await notifier.onResumed();

    h.probes.value = 2;
    await tester.pumpAndSettle(const Duration(seconds: 5));

    expect(h.toasts, isEmpty);
  });

  testWidgets('新的一次 refresh 一開始就清掉舊的失敗訊號', (tester) async {
    final h = await _pump(tester, probes: 0);
    final notifier = h.container.read(paymentOnboardingProvider.notifier);
    await notifier.start(OnboardingSurface.ONBOARDING_SURFACE_HOSTED);
    await notifier.onResumed();
    expect(h.container.read(paymentOnboardingProvider), isNotNull);

    final pending = notifier.confirmPaymentAccount();
    expect(h.container.read(paymentOnboardingProvider), isNull);
    await pending;
  });

  testWidgets('取走過的訊號不會再被取到', (tester) async {
    final h = await _pump(tester, probes: 0);
    final notifier = h.container.read(paymentOnboardingProvider.notifier);
    await notifier.start(OnboardingSurface.ONBOARDING_SURFACE_HOSTED);
    await notifier.onResumed();

    expect(notifier.takeRefreshFailure(), isNotNull);
    expect(notifier.takeRefreshFailure(), isNull);
  });
}
