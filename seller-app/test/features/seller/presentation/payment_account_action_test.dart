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

/// 用 [usePaymentAccountActions] 的最小消費者：按下去執行 [action]。
class _Probe extends HookConsumerWidget {
  const _Probe(this.action);

  final OnboardingAction action;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final actions = usePaymentAccountActions(context, ref);
    return TextButton(
      key: const Key('go'),
      onPressed: actions.busy ? null : () => actions.run(action),
      child: const Text('go'),
    );
  }
}

typedef _Harness = ({
  ProviderContainer container,
  List<Code> toasts,
  List<Uri> launched,
  List<CreatePaymentOnboardingSessionRequest> sessions,
  int Function() refreshCalls,
  int Function() summaryReads,
});

/// 失敗沒有伺服器句子，所以每次要顯示 toast 都會呼叫一次 defaultErrorText：呼叫次數就是 toast 次數。
Future<_Harness> _pump(
  WidgetTester tester, {
  required OnboardingAction action,
  Code? refreshFailsWith,
  bool launchSucceeds = true,
}) async {
  final toasts = <Code>[];
  final launched = <Uri>[];
  final sessions = <CreatePaymentOnboardingSessionRequest>[];
  var refreshCalls = 0;
  var summaryReads = 0;
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
      externalBrowserLauncherProvider.overrideWithValue((url) async {
        launched.add(url);
        return launchSucceeds;
      }),
      sellerClientProvider.overrideWithValue(
        SellerAppServiceClient(
          FakeTransportBuilder()
              .unary(SellerAppService.getMyOnboarding, (_, _) {
                summaryReads++;
                return paymentPendingSummary();
              })
              .unary(SellerAppService.refreshMyPaymentAccount, (_, _) {
                refreshCalls++;
                final code = refreshFailsWith;
                if (code != null) throw reasonError(code, 'DOWN');
                return RefreshMyPaymentAccountResponse();
              })
              .unary(SellerAppService.createPaymentOnboardingSession, (
                request,
                _,
              ) {
                sessions.add(request);
                return CreatePaymentOnboardingSessionResponse(
                  surface: OnboardingSurface.ONBOARDING_SURFACE_HOSTED,
                  url:
                      'https://www.example.test/seller-center/me/certifications',
                );
              })
              .build(),
        ),
      ),
    ],
  );
  addTearDown(container.dispose);
  await tester.pumpWidget(
    UncontrolledProviderScope(
      container: container,
      child: MaterialApp(home: Scaffold(body: _Probe(action))),
    ),
  );
  await tester.pumpAndSettle();
  return (
    container: container,
    toasts: toasts,
    launched: launched,
    sessions: sessions,
    refreshCalls: () => refreshCalls,
    summaryReads: () => summaryReads,
  );
}

void main() {
  test('只有開始開通與聯絡客服有按鈕，其他動作沒有', () {
    expect(
      paymentAccountActionHasButton(
        OnboardingAction.ONBOARDING_ACTION_START_PAYMENT_ONBOARDING,
      ),
      isTrue,
    );
    expect(
      paymentAccountActionHasButton(
        OnboardingAction.ONBOARDING_ACTION_CONTACT_SUPPORT,
      ),
      isTrue,
    );
    for (final action in [
      OnboardingAction.ONBOARDING_ACTION_NONE,
      OnboardingAction.ONBOARDING_ACTION_UNSPECIFIED,
      OnboardingAction.ONBOARDING_ACTION_EDIT_PROFILE,
      OnboardingAction.ONBOARDING_ACTION_SUBMIT_CERTIFICATION,
      OnboardingAction.valueOf(99) ?? OnboardingAction.ONBOARDING_ACTION_NONE,
    ]) {
      expect(paymentAccountActionHasButton(action), isFalse, reason: '$action');
    }
  });

  testWidgets('開始開通：一律呼叫 session RPC（目的地是賣家 App）並開伺服器回的連結，不判斷介面', (
    tester,
  ) async {
    final h = await _pump(
      tester,
      action: OnboardingAction.ONBOARDING_ACTION_START_PAYMENT_ONBOARDING,
    );

    await tester.tap(find.byKey(const Key('go')));
    await tester.pumpAndSettle();

    expect(h.sessions, hasLength(1));
    expect(
      h.sessions.single.destination,
      OnboardingReturnDestination.ONBOARDING_RETURN_DESTINATION_SELLER_APP,
    );
    expect(h.launched, [
      Uri.parse('https://www.example.test/seller-center/me/certifications'),
    ]);
    expect(h.toasts, isEmpty);
  });

  testWidgets('開始開通：瀏覽器開不起來顯示一般的錯誤', (tester) async {
    final h = await _pump(
      tester,
      action: OnboardingAction.ONBOARDING_ACTION_START_PAYMENT_ONBOARDING,
      launchSucceeds: false,
    );

    await tester.tap(find.byKey(const Key('go')));
    await tester.pumpAndSettle(const Duration(seconds: 5));

    expect(h.toasts, [Code.internal]);
  });

  testWidgets('沒有按鈕的動作：按下去什麼都不做', (tester) async {
    final h = await _pump(
      tester,
      action: OnboardingAction.ONBOARDING_ACTION_NONE,
    );

    await tester.tap(find.byKey(const Key('go')));
    await tester.pumpAndSettle();

    expect(h.sessions, isEmpty);
    expect(h.launched, isEmpty);
  });

  for (final code in [
    Code.unavailable,
    Code.resourceExhausted,
    Code.permissionDenied,
  ]) {
    testWidgets('自動重讀失敗（$code）：不顯示，照常讀開通摘要', (tester) async {
      final h = await _pump(
        tester,
        action: OnboardingAction.ONBOARDING_ACTION_NONE,
        refreshFailsWith: code,
      );
      final notifier = h.container.read(paymentOnboardingProvider.notifier);
      await notifier.start();
      final before = h.summaryReads();

      final read = await notifier.onResumed();
      await tester.pumpAndSettle(const Duration(seconds: 5));

      expect(h.refreshCalls(), 1);
      expect(h.summaryReads(), greaterThan(before));
      expect(read, isTrue);
      expect(h.toasts, isEmpty);
    });
  }

  testWidgets('不是從瀏覽器回來的回前景與進入畫面：重讀失敗同樣不顯示', (tester) async {
    final h = await _pump(
      tester,
      action: OnboardingAction.ONBOARDING_ACTION_NONE,
      refreshFailsWith: Code.resourceExhausted,
    );
    final notifier = h.container.read(paymentOnboardingProvider.notifier);

    final entered = await notifier.autoRefresh();
    final resumed = await notifier.onResumed();
    await tester.pumpAndSettle(const Duration(seconds: 5));

    expect(entered, isTrue);
    expect(resumed, isTrue);
    expect(h.refreshCalls(), 2);
    expect(h.toasts, isEmpty);
  });
}
