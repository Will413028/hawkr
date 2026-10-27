import 'dart:async';

import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/gen/google/rpc/error_details.pb.dart'
    show LocalizedMessage;
import 'package:hawkr_api/testing.dart';
import 'package:protobuf/well_known_types/google/protobuf/timestamp.pb.dart';
import 'package:hawkr_api/transport.dart';
import 'package:hawkr_seller/config/flavor.dart';
import 'package:hawkr_seller/core/networking/connect_provider.dart';
import 'package:hawkr_seller/core/router/app_router.dart';
import 'package:hawkr_seller/core/router/pending_wallet_focus.dart';
import 'package:hawkr_seller/features/seller/data/seller_status_refresh.dart';
import 'package:hawkr_seller/features/seller/presentation/widgets/payuni_status_body.dart'
    show payuniReasonLocale;
import 'package:hawkr_seller/core/refresh/refresh_clock.dart';
import 'package:hawkr_seller/core/session/session_provider.dart';
import 'package:hawkr_seller/core/theme/app_theme.dart';
import 'package:hawkr_seller/features/coupon/data/coupon_model.dart';
import 'package:hawkr_seller/features/coupon/presentation/coupon_controller.dart';
import 'package:hawkr_seller/features/seller/data/my_store_repository.dart';
import 'package:hawkr_seller/features/wallet/presentation/wallet_screen.dart';
import 'package:hawkr_seller/shared/widgets/truck_loading_overlay.dart';
import 'package:flutter/foundation.dart' show ValueListenable;
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_riverpod/misc.dart' show Override;
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

import '../../support/connect_errors.dart';
import '../../support/fake_my_store.dart';
import '../../support/kyc_fixtures.dart';
import '../../support/my_store_fixtures.dart';

const _approvedFlow = KYCFlowStatus.KYC_FLOW_STATUS_APPROVED;
const _payuniReview = KYCFlowStatus.KYC_FLOW_STATUS_UNDER_REVIEW;

SellerPayUni _payuni({
  PayUniState state = PayUniState.PAY_UNI_STATE_REVIEWING,
  String explanation = '',
  String reason = '',
  List<StoreField> storeFields = const [],
  List<KYCField> fields = const [],
}) => SellerPayUni(
  state: state,
  explanation: explanation.isEmpty
      ? null
      : LocalizedMessage(locale: 'zh-TW', message: explanation),
  reason: reason,
  fixStoreFields: storeFields,
  fixFields: fields,
);

SellerPayUni _payuni_(PayUniState state) => _payuni(
  state: state,
  explanation: '伺服器說明-${state.name.replaceFirst('PAY_UNI_STATE_', '')}',
);

SellerPayUni get _payuniApproved =>
    _payuni(state: PayUniState.PAY_UNI_STATE_APPROVED);

class _Server {
  _Server({
    KYCFlowStatus status = _approvedFlow,
    bool canResubmit = false,
    SellerPayUni? payuni,
    this.certified = true,
    this.marketing = true,
    this.foodRegistrationNumber,
    this.foodRequest,
    this.revokedFood,
  }) : _status = status,
       _canResubmit = canResubmit,
       _payuni =
           payuni ??
           (status == _payuniReview
               ? _payuni_(PayUniState.PAY_UNI_STATE_REVIEWING)
               : null) {
    kyc = _kycResponse();
    store = FakeMyStoreServer(response: _storeResponse());
  }

  late Object kyc;
  final KYCFlowStatus _status;
  final bool _canResubmit;
  SellerPayUni? _payuni;
  SellerPayUni? get payuni => _payuni;
  set payuni(SellerPayUni? value) {
    _payuni = value;
    kyc = _kycResponse();
  }

  GetMyKYCResponse _kycResponse() => GetMyKYCResponse(
    application: individualApplication(
      status: _status,
      canResubmit: _canResubmit,
      payuni: _payuni,
    ),
  );
  bool certified;
  bool marketing;
  String? foodRegistrationNumber;
  FoodRegistration? foodRequest;
  FoodRegistration? revokedFood;
  late final FakeMyStoreServer store;
  Object? updateFailure;
  Completer<void>? kycGate;
  var now = DateTime(2026, 10, 9, 12);
  void advance(Duration d) => now = now.add(d);
  int kycCalls = 0;
  final updates = <UpdateMyStoreRequest>[];
  final submits = <String>[];
  final submittedAgreements = <List<AgreementAcceptance>>[];
  Object? submitFailure;
  var cancels = 0;
  var clears = 0;
  var _etag = 1;

  GetMyStoreResponse _storeResponse() => myStoreResponse(
    etag: 'etag-$_etag',
    foodRegistrationNumber: foodRegistrationNumber,
    // 假伺服器的對應：待審放 pending_food_registration，被退回放 rejected_food_registration。
    pendingFoodRegistration:
        foodRequest?.state ==
            FoodRegistrationState.FOOD_REGISTRATION_STATE_PENDING
        ? foodRequest
        : null,
    rejectedFoodRegistration:
        foodRequest?.state ==
            FoodRegistrationState.FOOD_REGISTRATION_STATE_REJECTED
        ? foodRequest
        : null,
    revokedFoodRegistration: revokedFood,
    capabilities: marketing
        ? null
        : [SellerCapability.SELLER_CAPABILITY_STOREFRONT_WRITE],
    publicListing: publicListingMissing([
      if (!certified) SellerRequirement.SELLER_REQUIREMENT_KYC_APPROVED,
    ]),
  );

  void reloadStore() => store.response = _storeResponse();

  var agreementsCalls = 0;
  var noticeVersion = 'n1';

  SellerPublicServiceClient get publicClient => SellerPublicServiceClient(
    FakeTransportBuilder().unary(SellerPublicService.getSellerAgreements, (
      _,
      _,
    ) {
      agreementsCalls++;
      return GetSellerAgreementsResponse(
        agreements: [
          SellerAgreement(
            kind: AgreementKind.AGREEMENT_KIND_FOOD_REGISTRATION_NOTICE,
            version: noticeVersion,
            url:
                'https://www.example.test/privacy#seller-food-registration-notice',
          ),
        ],
      );
    }).build(),
  );

  SellerAppServiceClient get client => SellerAppServiceClient(
    FakeTransportBuilder()
        .unary(SellerAppService.getMyKYC, (_, _) async {
          kycCalls++;
          final gate = kycGate;
          if (gate != null) await gate.future;
          final answer = kyc;
          if (answer is ConnectException) throw answer;
          return answer as GetMyKYCResponse;
        })
        .unary(SellerAppService.updateMyStore, (request, _) {
          updates.add(request);
          final failure = updateFailure;
          if (failure != null) throw failure;
          _etag++;
          reloadStore();
          return UpdateMyStoreResponse(store: store.response.store);
        })
        .unary(SellerAppService.submitFoodRegistration, (request, _) {
          submits.add(request.registrationNumber);
          submittedAgreements.add(request.acceptedAgreements);
          final failure = submitFailure ?? updateFailure;
          if (failure != null) throw failure;
          _etag++;
          revokedFood = null;
          foodRequest = FoodRegistration(
            id: 'fr-2',
            state: FoodRegistrationState.FOOD_REGISTRATION_STATE_PENDING,
            registrationNumber: request.registrationNumber,
            submitTime: Timestamp.fromDateTime(DateTime.utc(2026, 10, 9, 4)),
          );
          reloadStore();
          return SubmitFoodRegistrationResponse(store: store.response.store);
        })
        .unary(SellerAppService.withdrawFoodRegistration, (_, _) {
          cancels++;
          _etag++;
          foodRequest = null;
          reloadStore();
          return WithdrawFoodRegistrationResponse(store: store.response.store);
        })
        .unary(SellerAppService.clearFoodRegistration, (_, _) {
          clears++;
          _etag++;
          foodRegistrationNumber = null;
          foodRequest = null;
          reloadStore();
          return ClearFoodRegistrationResponse(store: store.response.store);
        })
        .build(),
  );
}

class _LoggedIn extends Session {
  @override
  SessionModel build() =>
      const SessionModel(accountId: 'acc-1', isLoggedIn: true);
}

class _CouponFixture extends CouponListController {
  @override
  FutureOr<CouponListState> build() => const CouponListState();
}

class _PresetFocus extends PendingWalletFocus {
  @override
  String? build() => WalletSections.foodRegistration;
}

List<Override> _overrides(_Server server, {bool focusFood = false}) => [
  refreshClockProvider.overrideWithValue(() => server.now),
  if (focusFood) pendingWalletFocusProvider.overrideWith(_PresetFocus.new),
  sessionProvider.overrideWith(_LoggedIn.new),
  sellerClientProvider.overrideWithValue(server.client),
  sellerPublicClientProvider.overrideWithValue(server.publicClient),
  myStoreRepositoryProvider.overrideWithValue(server.store.repository),
  couponListControllerProvider.overrideWith(_CouponFixture.new),
];

Future<GoRouter> _pump(
  WidgetTester tester,
  _Server server, {
  bool focusFood = false,
  ProviderContainer? container,
  ValueListenable<bool>? visible,
  ValueListenable<bool>? tickers,
}) async {
  addTearDown(tester.view.reset);
  tester.view
    ..devicePixelRatio = 1
    ..physicalSize = const Size(402, 1600);
  final router = GoRouter(
    routes: [
      GoRoute(
        path: '/',
        builder: (_, _) {
          const wallet = WalletScreen();
          Widget page = wallet;
          if (tickers != null) {
            page = ValueListenableBuilder<bool>(
              valueListenable: tickers,
              builder: (_, on, _) => TickerMode(enabled: on, child: wallet),
            );
          }
          if (visible == null) return page;
          return ValueListenableBuilder<bool>(
            valueListenable: visible,
            builder: (_, show, _) => show ? page : const SizedBox(),
          );
        },
      ),
      GoRoute(
        path: SellerRoutes.certificationUpload,
        builder: (_, state) => Scaffold(body: Text('wizard:${state.extra}')),
      ),
      GoRoute(
        path: MainRoutes.settings,
        builder: (_, _) => const Scaffold(body: Text('settings')),
      ),
    ],
  );
  final app = ScreenUtilInit(
    designSize: const Size(414, 896),
    enableScaleWH: () => false,
    enableScaleText: () => false,
    builder: (_, _) => MaterialApp.router(
      theme: AppTheme.light,
      routerConfig: router,
      builder: (context, child) => MediaQuery(
        data: MediaQuery.of(context).copyWith(disableAnimations: true),
        child: child!,
      ),
    ),
  );
  await tester.pumpWidget(
    container != null
        ? UncontrolledProviderScope(container: container, child: app)
        : ProviderScope(
            overrides: _overrides(server, focusFood: focusFood),
            child: app,
          ),
  );
  await _settle(tester);
  return router;
}

String _fieldText(WidgetTester tester) =>
    tester.widget<TextField>(find.byType(TextField)).controller!.text;

Future<void> _settle(WidgetTester tester) async {
  for (var i = 0; i < 12; i++) {
    await tester.pump(const Duration(milliseconds: 50));
  }
}

Future<void> _readNotice(WidgetTester tester) =>
    _tap(tester, find.byKey(const Key('food_notice_checkbox')));

Future<void> _tap(WidgetTester tester, Finder finder) async {
  await tester.ensureVisible(finder);
  await tester.tap(finder);
  await _settle(tester);
}

final launches = <Map<Object?, Object?>>[];
bool launchResult = true;

void main() {
  setUpAll(() {
    if (!F.isInitialized) F.init(Flavor.dev);
  });
  setUp(() {
    launches.clear();
    launchResult = true;
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(
          const MethodChannel('plugins.flutter.io/url_launcher'),
          (call) async {
            if (call.method == 'launch') {
              launches.add(call.arguments as Map<Object?, Object?>);
              return launchResult;
            }
            return null;
          },
        );
  });

  group('讀取', () {
    testWidgets('讀取中顯示載入樣式，完成後換成內容', (tester) async {
      final server = _Server()..kycGate = Completer<void>();
      await _pump(tester, server);
      expect(find.byType(TruckLoadingOverlay), findsOneWidget);
      server.kycGate!.complete();
      await _settle(tester);
      expect(find.byType(TruckLoadingOverlay), findsNothing);
      expect(find.text('營運錢包'), findsOneWidget);
    });

    testWidgets('GetMyKYC 失敗顯示重試，不當成未開始認證', (tester) async {
      final server = _Server()
        ..kyc = ConnectException(Code.unavailable, 'later');
      await _pump(tester, server);
      expect(find.text('無法載入認證狀態'), findsOneWidget);
      expect(find.text('個人賣家'), findsNothing);
      expect(find.text('重試'), findsOneWidget);

      server.kyc = GetMyKYCResponse(
        application: individualApplication(status: _approvedFlow),
      );
      await _tap(tester, find.text('重試'));
      expect(find.text('無法載入認證狀態'), findsNothing);
      expect(find.text('帳戶與認證'), findsOneWidget);
    });

    testWidgets('切回錢包重讀認證流程（含 PayUni 狀態），重讀時繼續顯示上一次的內容', (tester) async {
      final server = _Server(payuni: _payuniApproved);
      final container = ProviderContainer(overrides: _overrides(server));
      addTearDown(container.dispose);
      final tickers = ValueNotifier(true);
      addTearDown(tickers.dispose);
      await _pump(tester, server, container: container, tickers: tickers);
      expect(find.text('前往提領'), findsOneWidget);

      tickers.value = false;
      await _settle(tester);
      final kycBefore = server.kycCalls;
      server.kycGate = Completer<void>();
      server.advance(statusStaleTime + const Duration(seconds: 1));
      tickers.value = true;
      await _settle(tester);

      expect(server.kycCalls, kycBefore + 1);
      expect(find.byType(TruckLoadingOverlay), findsNothing);
      expect(find.text('前往提領'), findsOneWidget);
      server.kycGate!.complete();
      await _settle(tester);
      expect(find.text('前往提領'), findsOneWidget);
    });

    testWidgets('流程狀態是 UNSPECIFIED（無法確認）：顯示重試，不當成任何有效狀態', (tester) async {
      final server = _Server()
        ..kyc = GetMyKYCResponse(
          application: individualApplication(
            status: KYCFlowStatus.KYC_FLOW_STATUS_UNSPECIFIED,
          ),
        );
      await _pump(tester, server);
      expect(find.text('無法載入認證狀態'), findsOneWidget);
      expect(find.text('個人賣家'), findsNothing);
      expect(find.text('帳戶與認證'), findsNothing);
    });

    testWidgets('App 回到前景時重讀認證流程（含 PayUni 狀態）', (tester) async {
      final server = _Server();
      await _pump(tester, server);
      final kycBefore = server.kycCalls;
      tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
      await _settle(tester);
      expect(server.kycCalls, kycBefore + 1);
    });
  });

  group('重讀時機與等待', () {
    testWidgets('第一次開啟讀一次；切分頁回來再讀一次', (tester) async {
      final server = _Server(payuni: _payuniApproved);
      final container = ProviderContainer(overrides: _overrides(server));
      addTearDown(container.dispose);
      final tickers = ValueNotifier(true);
      addTearDown(tickers.dispose);
      await _pump(tester, server, container: container, tickers: tickers);
      expect(server.kycCalls, 1);

      tickers.value = false;
      await _settle(tester);
      server.advance(statusStaleTime + const Duration(seconds: 1));
      tickers.value = true;
      await _settle(tester);
      expect(server.kycCalls, 2);
    });

    testWidgets('重讀失敗但還留著上一次成功的值：繼續顯示錢包，不出現錯誤面板', (tester) async {
      final server = _Server(payuni: _payuniApproved);
      await _pump(tester, server);
      expect(find.text('前往提領'), findsOneWidget);

      server.kyc = ConnectException(Code.unavailable, 'later');
      server.store.failure = ConnectException(Code.unavailable, 'later');
      tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
      await _settle(tester);

      expect(find.text('無法載入認證狀態'), findsNothing);
      expect(find.text('帳戶與認證'), findsOneWidget);
      expect(find.text('前往提領'), findsOneWidget);
    });

    testWidgets('成為可見的重讀：30 秒內讀過不讀，超過 30 秒讀一次（假時鐘）', (tester) async {
      final server = _Server(payuni: _payuniApproved);
      final container = ProviderContainer(overrides: _overrides(server));
      addTearDown(container.dispose);
      final tickers = ValueNotifier(true);
      addTearDown(tickers.dispose);
      await _pump(tester, server, container: container, tickers: tickers);
      tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
      await _settle(tester);
      final before = server.kycCalls;

      tickers.value = false;
      await _settle(tester);
      server.advance(const Duration(seconds: 29));
      tickers.value = true;
      await _settle(tester);
      expect(server.kycCalls, before, reason: '29 秒前才讀過');

      tickers.value = false;
      await _settle(tester);
      server.advance(const Duration(seconds: 2));
      tickers.value = true;
      await _settle(tester);
      expect(server.kycCalls, before + 1, reason: '超過 30 秒');
    });

    testWidgets('前往補件回來後重讀認證流程', (tester) async {
      final server = _Server(
        status: KYCFlowStatus.KYC_FLOW_STATUS_PAYUNI_REJECTED,
        canResubmit: true,
        certified: false,
        payuni: _payuni(state: PayUniState.PAY_UNI_STATE_FIXABLE),
      );
      final router = await _pump(tester, server);
      final kycBefore = server.kycCalls;
      await _tap(tester, find.text('前往補件'));
      expect(find.text('wizard:null'), findsOneWidget);
      router.pop(true);
      await _settle(tester);
      expect(server.kycCalls, greaterThan(kycBefore));
    });

    testWidgets('錢包成為目前的分頁時重讀；回前景只在錢包是目前的分頁時重讀', (tester) async {
      final server = _Server();
      final container = ProviderContainer(overrides: _overrides(server));
      addTearDown(container.dispose);
      final tickers = ValueNotifier(true);
      addTearDown(tickers.dispose);
      await _pump(tester, server, container: container, tickers: tickers);

      tickers.value = false;
      await _settle(tester);
      var before = server.kycCalls;
      tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
      await _settle(tester);
      expect(server.kycCalls, before, reason: '錢包不是目前的分頁，回前景不重讀');

      server.advance(statusStaleTime + const Duration(seconds: 1));
      tickers.value = true;
      await _settle(tester);
      expect(server.kycCalls, before + 1, reason: '切回錢包就重讀');

      before = server.kycCalls;
      tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
      await _settle(tester);
      expect(server.kycCalls, before + 1);
    });

    testWidgets('被認證流程蓋住時（TickerMode 關閉）回前景不重讀，回來才重讀', (tester) async {
      final server = _Server(payuni: _payuniApproved);
      final router = await _pump(tester, server);
      unawaited(router.push(SellerRoutes.certificationUpload));
      await _settle(tester);
      expect(find.text('wizard:null'), findsOneWidget);

      var before = server.kycCalls;
      tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
      await _settle(tester);
      expect(server.kycCalls, before, reason: '錢包被蓋住，不重讀');

      server.advance(statusStaleTime + const Duration(seconds: 1));
      router.pop();
      await _settle(tester);
      expect(server.kycCalls, before + 1, reason: '回到錢包就重讀');

      before = server.kycCalls;
      tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
      await _settle(tester);
      expect(server.kycCalls, before + 1);
    });

    testWidgets('GetMyStore 失敗而且沒有快照：同樣的錯誤面板與重試；重試成功後不套用已丟掉的展開意圖', (
      tester,
    ) async {
      final server = _Server(payuni: _payuniApproved);
      server.store.failure = ConnectException(Code.unavailable, 'later');
      await _pump(tester, server, focusFood: true);
      expect(find.text('無法載入認證狀態'), findsOneWidget);
      expect(find.text('帳戶與認證'), findsNothing);

      server.store.failure = null;
      await _tap(tester, find.text('重試'));
      expect(find.text('無法載入認證狀態'), findsNothing);
      expect(find.text('帳戶與認證'), findsOneWidget);
      expect(find.byKey(const ValueKey('wallet-food-form')), findsNothing);
    });

    testWidgets('第一次顯示也等第一次 GetMyStore，帳戶與認證、提領、優惠券不會後跳出來', (tester) async {
      final server = _Server(payuni: _payuniApproved);
      final storeGate = Completer<void>();
      server.store.gate = storeGate.future;
      await _pump(tester, server);
      expect(find.byType(TruckLoadingOverlay), findsOneWidget);
      expect(find.text('帳戶與認證'), findsNothing);
      storeGate.complete();
      await _settle(tester);
      expect(find.byType(TruckLoadingOverlay), findsNothing);
      expect(find.text('帳戶與認證'), findsOneWidget);
      expect(find.text('前往提領'), findsOneWidget);
      expect(find.text('優惠券'), findsOneWidget);
    });

    testWidgets('第一次顯示等 GetMyKYC，不會先閃出閘門；PayUni 狀態隨同一份回應到達', (tester) async {
      final server = _Server(status: _payuniReview, certified: false)
        ..kycGate = Completer<void>();
      await _pump(tester, server);
      expect(find.byType(TruckLoadingOverlay), findsOneWidget);
      expect(find.text('尚未通過商家認證'), findsNothing);
      server.kycGate!.complete();
      await _settle(tester);
      expect(find.byType(TruckLoadingOverlay), findsNothing);
      expect(find.text('PayUni 審核中'), findsOneWidget);
      expect(find.text('伺服器說明-REVIEWING'), findsOneWidget);
      expect(find.text('尚未通過商家認證'), findsNothing);
    });
  });

  group('還沒進 PayUni 階段', () {
    for (final status in [
      KYCFlowStatus.KYC_FLOW_STATUS_NOT_STARTED,
      KYCFlowStatus.KYC_FLOW_STATUS_DRAFT,
    ]) {
      testWidgets('$status：閘門的開始認證進認證流程，不帶型態，回來後重讀', (tester) async {
        final server = _Server(status: status, certified: false);
        final router = await _pump(tester, server);
        expect(find.text('尚未通過商家認證'), findsOneWidget);
        expect(find.text('請先完成商家認證，才能啟用錢包與收款。'), findsOneWidget);
        expect(find.text('個人賣家'), findsNothing);

        final kycBefore = server.kycCalls;
        await _tap(tester, find.text('開始認證'));
        expect(find.text('wizard:null'), findsOneWidget);
        router.pop(true);
        await _settle(tester);
        expect(server.kycCalls, greaterThan(kycBefore));
      });
    }

    testWidgets('送審中：閘門文案取自狀態表，按鈕進認證流程', (tester) async {
      final server = _Server(
        status: KYCFlowStatus.KYC_FLOW_STATUS_SUBMITTED,
        certified: false,
      );
      await _pump(tester, server);
      expect(find.text('商家認證審核中'), findsOneWidget);
      expect(
        find.text('你的商家認證平台審核中，通過後將自動送出至 PayUni 進行金流審核，完成後就能啟用錢包與收款。'),
        findsOneWidget,
      );
      expect(find.text('PayUni 審核中'), findsNothing);
      expect(find.byKey(const ValueKey('wallet-cert-row')), findsNothing);
      await _tap(tester, find.text('查看進度'));
      expect(find.text('wizard:null'), findsOneWidget);
    });

    testWidgets('未通過：補件閘門', (tester) async {
      final server = _Server(
        status: KYCFlowStatus.KYC_FLOW_STATUS_REJECTED,
        certified: false,
      );
      await _pump(tester, server);
      expect(find.text('商家認證需要補件'), findsOneWidget);
      expect(find.text('認證未通過，請修改資料後重新送審。'), findsOneWidget);
      expect(find.text('前往補件'), findsOneWidget);
    });

    testWidgets('已有 PayUni 註冊時即使流程狀態還在送審中也顯示 PayUni 面板', (tester) async {
      final server = _Server(
        status: KYCFlowStatus.KYC_FLOW_STATUS_SUBMITTED,
        certified: false,
        payuni: _payuni(),
      );
      await _pump(tester, server);
      expect(find.text('PayUni 審核中'), findsOneWidget);
      expect(find.text('商家認證審核中'), findsNothing);
    });
  });

  group('PayUni 狀態面板', () {
    testWidgets('已進入 PayUni 階段但還沒有註冊：用審核中文案', (tester) async {
      final server = _Server(status: _payuniReview, certified: false);
      await _pump(tester, server);
      expect(find.text('PayUni 審核中'), findsOneWidget);
      expect(find.text('前往補件'), findsNothing);
      expect(find.text('前往店家設定'), findsNothing);
    });

    testWidgets('可收款：沒有面板', (tester) async {
      final server = _Server(payuni: _payuniApproved);
      await _pump(tester, server);
      expect(find.byKey(const ValueKey('wallet-payuni-panel')), findsNothing);
    });

    testWidgets('賣家可以修正：顯示伺服器說明、審核意見、欄位與前往補件', (tester) async {
      final server = _Server(
        status: KYCFlowStatus.KYC_FLOW_STATUS_PAYUNI_REJECTED,
        canResubmit: true,
        certified: false,
        payuni: _payuni(
          state: PayUniState.PAY_UNI_STATE_FIXABLE,
          explanation: '伺服器說明-FIXABLE',
          reason: '資料不符',
          fields: [KYCField.KYC_FIELD_BUSINESS_COMPANY_ADDRESS],
        ),
      );
      await _pump(tester, server);
      expect(find.text('PayUni 需要補件'), findsOneWidget);
      expect(find.text('伺服器說明-FIXABLE'), findsOneWidget);
      expect(find.text('PayUni 的審核意見'), findsOneWidget);
      final reason = tester.widget<Text>(find.text('資料不符'));
      expect(reason.locale, payuniReasonLocale);
      expect(find.text('請修正以下資料'), findsOneWidget);
      expect(find.text('・公司地址'), findsOneWidget);
      await _tap(tester, find.text('前往補件'));
      expect(find.text('wizard:null'), findsOneWidget);
    });

    testWidgets('前往補件只看 canResubmit：FIXABLE 但不能重送時不出現', (tester) async {
      final server = _Server(
        status: KYCFlowStatus.KYC_FLOW_STATUS_PAYUNI_REJECTED,
        canResubmit: false,
        certified: false,
        payuni: _payuni(state: PayUniState.PAY_UNI_STATE_FIXABLE),
      );
      await _pump(tester, server);
      expect(find.text('PayUni 需要補件'), findsOneWidget);
      expect(find.text('前往補件'), findsNothing);
    });

    testWidgets('問題指向店家欄位：前往店家設定，只看 fixStoreFields', (tester) async {
      final server = _Server(
        status: KYCFlowStatus.KYC_FLOW_STATUS_PAYUNI_REJECTED,
        certified: false,
        payuni: _payuni(
          state: PayUniState.PAY_UNI_STATE_FIX_VIA_SUPPORT,
          storeFields: [StoreField.STORE_FIELD_CONTACT_MOBILE],
        ),
      );
      await _pump(tester, server);
      expect(find.text('・聯絡人手機'), findsOneWidget);
      expect(find.text('前往補件'), findsNothing);
      await _tap(tester, find.text('前往店家設定'));
      expect(find.text('settings'), findsOneWidget);
    });

    testWidgets('沒有審核意見與欄位時不出現小標題與入口', (tester) async {
      final server = _Server(
        status: KYCFlowStatus.KYC_FLOW_STATUS_PAYUNI_REJECTED,
        certified: false,
        payuni: _payuni(
          state: PayUniState.PAY_UNI_STATE_CONTACT_SUPPORT,
          explanation: '伺服器說明-CONTACT_SUPPORT',
        ),
      );
      await _pump(tester, server);
      expect(find.text('PayUni 申請未通過'), findsOneWidget);
      expect(find.text('伺服器說明-CONTACT_SUPPORT'), findsOneWidget);
      expect(find.text('PayUni 的審核意見'), findsNothing);
      expect(find.text('請修正以下資料'), findsNothing);
      expect(find.text('前往補件'), findsNothing);
      expect(find.text('前往店家設定'), findsNothing);
    });

    testWidgets('各狀態的標題來自目錄名詞', (tester) async {
      final cases = {
        PayUniState.PAY_UNI_STATE_RETRY_LATER: 'PayUni 暫時無法處理',
        PayUniState.PAY_UNI_STATE_TERMINATED: 'PayUni 收款帳戶已終止',
        PayUniState.PAY_UNI_STATE_PENDING_INFO: 'PayUni 需要補件',
      };
      for (final entry in cases.entries) {
        final server = _Server(
          certified: false,
          payuni: _payuni(state: entry.key),
        );
        await _pump(tester, server);
        expect(find.text(entry.value), findsOneWidget, reason: '${entry.key}');
        await tester.pumpWidget(const SizedBox.shrink());
      }
    });

    testWidgets('不認得的狀態：顯示「未知」與說明，入口照常讀 canResubmit 與清單', (tester) async {
      final server = _Server(
        canResubmit: true,
        certified: false,
        payuni: _payuni(
          state: PayUniState.PAY_UNI_STATE_UNSPECIFIED,
          explanation: '伺服器說明-未知',
        ),
      );
      await _pump(tester, server);
      expect(find.text('未知'), findsOneWidget);
      expect(find.text('伺服器說明-未知'), findsOneWidget);
      expect(find.text('前往補件'), findsOneWidget);
    });
  });

  group('核准後', () {
    testWidgets('店家已認證：顯示帳戶與認證，商家認證列可展開', (tester) async {
      final server = _Server(payuni: _payuniApproved);
      await _pump(tester, server);
      expect(find.text('帳戶與認證'), findsOneWidget);
      expect(find.text('商家認證'), findsOneWidget);
      expect(find.text('販售加工食品'), findsOneWidget);
      expect(find.text('未登錄'), findsOneWidget);
      await _tap(tester, find.byKey(const ValueKey('wallet-cert-row')));
      expect(find.text('商家認證已通過'), findsOneWidget);
    });

    testWidgets('店家未認證：不顯示商家認證列，食品業者登錄列照樣顯示', (tester) async {
      final server = _Server(status: _payuniReview, certified: false);
      await _pump(tester, server);
      expect(find.text('帳戶與認證'), findsOneWidget);
      expect(find.byKey(const ValueKey('wallet-cert-row')), findsNothing);
      expect(find.byKey(const ValueKey('wallet-food-row')), findsOneWidget);
    });

    testWidgets('已有登錄字號顯示已登錄', (tester) async {
      final server = _Server(
        payuni: _payuniApproved,
        foodRegistrationNumber: 'A-123456789-00000-1',
      );
      await _pump(tester, server);
      expect(find.text('已登錄'), findsOneWidget);
    });
  });

  group('前往提領', () {
    testWidgets('店家已認證而且 PayUni 可收款才顯示，用系統內建瀏覽器開 PayUni 登入頁', (tester) async {
      final server = _Server(payuni: _payuniApproved);
      await _pump(tester, server);
      expect(find.text('前往提領'), findsOneWidget);
      await _tap(tester, find.text('前往提領'));
      expect(launches.single['url'], 'https://sandbox.payuni.com.tw/login');
      expect(launches.single['useSafariVC'], isTrue);
    });

    testWidgets('開啟失敗：顯示提示', (tester) async {
      launchResult = false;
      final server = _Server(payuni: _payuniApproved);
      await _pump(tester, server);
      await _tap(tester, find.text('前往提領'));
      expect(find.text('無法開啟 PayUni，請稍後再試'), findsOneWidget);
      await tester.pump(const Duration(seconds: 3));
    });

    testWidgets('店家已認證但 PayUni 還在審核：不顯示', (tester) async {
      final server = _Server(payuni: _payuni());
      await _pump(tester, server);
      expect(find.text('前往提領'), findsNothing);
    });

    testWidgets('PayUni 可收款但店家未認證：不顯示', (tester) async {
      final server = _Server(payuni: _payuniApproved, certified: false);
      await _pump(tester, server);
      expect(find.text('前往提領'), findsNothing);
    });
  });

  group('優惠券依 MARKETING 能力，與認證狀態無關', () {
    for (final status in [
      KYCFlowStatus.KYC_FLOW_STATUS_NOT_STARTED,
      KYCFlowStatus.KYC_FLOW_STATUS_SUBMITTED,
      _payuniReview,
      _approvedFlow,
    ]) {
      testWidgets('$status 有 MARKETING 顯示、沒有則不顯示', (tester) async {
        final withMarketing = _Server(status: status, certified: false);
        await _pump(tester, withMarketing);
        expect(find.text('優惠券'), findsOneWidget);
        await tester.pumpWidget(const SizedBox.shrink());

        final without = _Server(
          status: status,
          certified: false,
          marketing: false,
        );
        await _pump(tester, without);
        expect(find.text('優惠券'), findsNothing);
      });
    }

    testWidgets('讀取失敗頁也依 MARKETING 顯示', (tester) async {
      final server = _Server()
        ..kyc = ConnectException(Code.unavailable, 'later');
      await _pump(tester, server);
      expect(find.text('無法載入認證狀態'), findsOneWidget);
      expect(find.text('優惠券'), findsOneWidget);
    });
  });

  group('食品業者登錄補件', () {
    Future<void> openFood(WidgetTester tester) =>
        _tap(tester, find.byKey(const ValueKey('wallet-food-row')));

    testWidgets('?section=food-registration 直接展開', (tester) async {
      final server = _Server(payuni: _payuniApproved);
      await _pump(tester, server, focusFood: true);
      expect(find.text('食品業者登錄字號'), findsWidgets);
      expect(find.byKey(const ValueKey('wallet-food-form')), findsOneWidget);
    });

    testWidgets('等待中的展開意圖在第一次穩定的畫面就丟掉：錯誤面板之後成功進入也不套用', (tester) async {
      final server = _Server(payuni: _payuniApproved)
        ..kyc = ConnectException(Code.unavailable, 'later');
      final container = ProviderContainer(overrides: _overrides(server));
      addTearDown(container.dispose);
      container
          .read(pendingWalletFocusProvider.notifier)
          .request(WalletSections.foodRegistration);
      await _pump(tester, server, container: container);
      expect(find.text('無法載入認證狀態'), findsOneWidget);
      expect(container.read(pendingWalletFocusProvider), isNull);

      server.kyc = GetMyKYCResponse(
        application: individualApplication(status: _approvedFlow),
      );
      await _tap(tester, find.text('重試'));
      expect(find.text('帳戶與認證'), findsOneWidget);
      expect(find.byKey(const ValueKey('wallet-food-form')), findsNothing);
    });

    testWidgets('閘門畫面也套用展開意圖：食品業者登錄不必等認證', (tester) async {
      final server = _Server(
        status: KYCFlowStatus.KYC_FLOW_STATUS_NOT_STARTED,
        certified: false,
      );
      final container = ProviderContainer(overrides: _overrides(server));
      addTearDown(container.dispose);
      container
          .read(pendingWalletFocusProvider.notifier)
          .request(WalletSections.foodRegistration);
      await _pump(tester, server, container: container);
      expect(find.text('尚未通過商家認證'), findsOneWidget);
      expect(find.byKey(const ValueKey('wallet-food-form')), findsOneWidget);
      expect(container.read(pendingWalletFocusProvider), isNull);
    });

    testWidgets('進度卡的意圖：每次導覽套用一次並清掉；重建不重播，再次要求才再展開', (tester) async {
      final server = _Server(payuni: _payuniApproved);
      final container = ProviderContainer(overrides: _overrides(server));
      addTearDown(container.dispose);
      final router = await _pump(tester, server, container: container);
      expect(find.byKey(const ValueKey('wallet-food-form')), findsNothing);

      container
          .read(pendingWalletFocusProvider.notifier)
          .request(WalletSections.foodRegistration);
      await _settle(tester);
      expect(find.byKey(const ValueKey('wallet-food-form')), findsOneWidget);
      expect(container.read(pendingWalletFocusProvider), isNull);

      await _tap(tester, find.byKey(const ValueKey('wallet-food-row')));
      expect(find.byKey(const ValueKey('wallet-food-form')), findsNothing);
      tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
      await _settle(tester);
      expect(
        find.byKey(const ValueKey('wallet-food-form')),
        findsNothing,
        reason: '重建、重讀都不重播',
      );
      router.go('/');
      await _settle(tester);
      expect(
        find.byKey(const ValueKey('wallet-food-form')),
        findsNothing,
        reason: '還原分頁的網址不重播',
      );

      container
          .read(pendingWalletFocusProvider.notifier)
          .request(WalletSections.foodRegistration);
      await _settle(tester);
      expect(find.byKey(const ValueKey('wallet-food-form')), findsOneWidget);
    });

    testWidgets('未登錄：輸入自動轉大寫，沒有字號不能送出，送出審核打 SubmitFoodRegistration', (
      tester,
    ) async {
      final server = _Server(payuni: _payuniApproved);
      await _pump(tester, server);
      await openFood(tester);

      expect(find.byType(TextField), findsOneWidget);
      expect(
        find.textContaining('上架加工食品前須完成食品業者登錄。送出後平台會核對食藥署的登錄資料'),
        findsOneWidget,
      );
      expect(find.textContaining('資料儲存後會立即套用'), findsNothing);
      expect(
        tester
            .widget<InkWell>(
              find
                  .ancestor(
                    of: find.text('送出審核'),
                    matching: find.byType(InkWell),
                  )
                  .first,
            )
            .onTap,
        isNull,
      );

      await tester.enterText(find.byType(TextField), 'a-123456789-00000-1');
      await tester.pump();
      expect(_fieldText(tester), 'A-123456789-00000-1');
      expect(
        tester
            .widget<InkWell>(
              find
                  .ancestor(
                    of: find.text('送出審核'),
                    matching: find.byType(InkWell),
                  )
                  .first,
            )
            .onTap,
        isNull,
        reason: '沒勾告知事項不能送出',
      );
      expect(
        find.textContaining('我已閱讀〈食品業者登錄個人資料蒐集告知事項〉', findRichText: true),
        findsOneWidget,
      );
      await _readNotice(tester);
      await _tap(tester, find.text('送出審核'));

      expect(server.submits, ['A-123456789-00000-1']);
      expect(
        server.submittedAgreements.single.map((a) => (a.kind, a.version)),
        [(AgreementKind.AGREEMENT_KIND_FOOD_REGISTRATION_NOTICE, 'n1')],
      );
      expect(server.updates, isEmpty);
      expect(
        find.textContaining('審核中：A-123456789-00000-1（2026/10/09 送出）'),
        findsOneWidget,
      );
      expect(find.text('撤回'), findsOneWidget);
    });

    testWidgets('審核中：顯示字號與日期，商家認證未核准時加一句，撤回只動待審', (tester) async {
      final server = _Server(
        status: KYCFlowStatus.KYC_FLOW_STATUS_SUBMITTED,
        certified: false,
        foodRegistrationNumber: 'A-1',
        foodRequest: FoodRegistration(
          id: 'fr-2',
          state: FoodRegistrationState.FOOD_REGISTRATION_STATE_PENDING,
          registrationNumber: 'B-2',
          submitTime: Timestamp.fromDateTime(DateTime.utc(2026, 10, 9, 4)),
        ),
      );
      await _pump(tester, server);
      await openFood(tester);
      expect(find.text('審核中：B-2（2026/10/09 送出）'), findsOneWidget);
      expect(find.text('平台會在商家認證通過後進行審核。'), findsOneWidget);
      expect(find.text('已登錄：A-1'), findsOneWidget);
      expect(find.text('送出審核'), findsNothing);
      expect(find.text('更換字號'), findsOneWidget);
      expect(find.text('清除'), findsOneWidget);

      await _tap(tester, find.text('撤回'));
      expect(server.cancels, 1);
      expect(find.textContaining('審核中：'), findsNothing);
      expect(find.text('已登錄：A-1'), findsOneWidget);
    });

    testWidgets('審核中的送出日期是臺北的日曆日：UTC 16:30 是臺北隔天', (tester) async {
      final server = _Server(
        status: KYCFlowStatus.KYC_FLOW_STATUS_SUBMITTED,
        certified: false,
        foodRequest: FoodRegistration(
          id: 'fr-2',
          state: FoodRegistrationState.FOOD_REGISTRATION_STATE_PENDING,
          registrationNumber: 'B-2',
          submitTime: Timestamp.fromDateTime(DateTime.utc(2026, 10, 9, 16, 30)),
        ),
      );
      await _pump(tester, server);
      await openFood(tester);
      expect(find.text('審核中：B-2（2026/10/10 送出）'), findsOneWidget);
    });

    testWidgets('審核中但伺服器沒給送出時間：日期留空，不顯示 1970', (tester) async {
      final server = _Server(
        status: KYCFlowStatus.KYC_FLOW_STATUS_SUBMITTED,
        certified: false,
        foodRequest: FoodRegistration(
          id: 'fr-2',
          state: FoodRegistrationState.FOOD_REGISTRATION_STATE_PENDING,
          registrationNumber: 'B-2',
        ),
      );
      await _pump(tester, server);
      await openFood(tester);
      expect(find.textContaining('審核中：B-2'), findsOneWidget);
      expect(find.textContaining('1970'), findsNothing);
    });

    testWidgets('商家認證已核准、PayUni 還沒通過：審核中不顯示商家認證的提示', (tester) async {
      for (final status in [
        KYCFlowStatus.KYC_FLOW_STATUS_BINDING,
        _payuniReview,
        KYCFlowStatus.KYC_FLOW_STATUS_PENDING_INFO,
        KYCFlowStatus.KYC_FLOW_STATUS_PAYUNI_REJECTED,
      ]) {
        final server = _Server(
          status: status,
          certified: false,
          payuni: _payuni(),
          foodRequest: FoodRegistration(
            id: 'fr-2',
            state: FoodRegistrationState.FOOD_REGISTRATION_STATE_PENDING,
            registrationNumber: 'B-2',
            submitTime: Timestamp.fromDateTime(DateTime.utc(2026, 10, 9, 4)),
          ),
        );
        await _pump(tester, server);
        await openFood(tester);
        expect(find.text('審核中：B-2（2026/10/09 送出）'), findsOneWidget);
        expect(find.text('平台會在商家認證通過後進行審核。'), findsNothing, reason: '$status');
      }
    });

    testWidgets('認證通過後審核中不再顯示商家認證的提示', (tester) async {
      final server = _Server(
        payuni: _payuniApproved,
        foodRequest: FoodRegistration(
          id: 'fr-2',
          state: FoodRegistrationState.FOOD_REGISTRATION_STATE_PENDING,
          registrationNumber: 'B-2',
          submitTime: Timestamp.fromDateTime(DateTime.utc(2026, 10, 9, 4)),
        ),
      );
      await _pump(tester, server);
      await openFood(tester);
      expect(find.text('平台會在商家認證通過後進行審核。'), findsNothing);
    });

    testWidgets('未通過：顯示理由，可以重新送出', (tester) async {
      final server = _Server(
        payuni: _payuniApproved,
        foodRequest: FoodRegistration(
          id: 'fr-2',
          state: FoodRegistrationState.FOOD_REGISTRATION_STATE_REJECTED,
          registrationNumber: 'B-2',
          submitTime: Timestamp.fromDateTime(DateTime.utc(2026, 10, 9, 4)),
          rejectReason: '查無此字號',
        ),
      );
      await _pump(tester, server);
      await openFood(tester);
      expect(find.text('未通過：查無此字號。請確認字號與食藥署登錄資料一致後重新送出。'), findsOneWidget);
      await tester.enterText(find.byType(TextField), 'c-3');
      await tester.pump();
      await _readNotice(tester);
      await _tap(tester, find.text('送出審核'));
      expect(server.submits, ['C-3']);
    });

    testWidgets('已登錄：更換字號才出現輸入框，送出後核准的照常顯示', (tester) async {
      final server = _Server(
        payuni: _payuniApproved,
        foodRegistrationNumber: 'A-1',
      );
      await _pump(tester, server);
      await openFood(tester);
      expect(find.text('已登錄：A-1'), findsOneWidget);
      expect(find.byType(TextField), findsNothing);
      expect(find.text('清除'), findsOneWidget);

      await _tap(tester, find.text('更換字號'));
      expect(find.byType(TextField), findsOneWidget);
      await tester.enterText(find.byType(TextField), 'b-9');
      await tester.pump();
      await _readNotice(tester);
      await _tap(tester, find.text('送出審核'));
      expect(server.submits, ['B-9']);
      expect(find.text('已登錄：A-1'), findsOneWidget);
      expect(find.textContaining('審核中：B-9'), findsOneWidget);
    });

    testWidgets('更換字號：可以取消回到已登錄的畫面，不送出', (tester) async {
      final server = _Server(
        payuni: _payuniApproved,
        foodRegistrationNumber: 'A-1',
      );
      await _pump(tester, server);
      await openFood(tester);
      await _tap(tester, find.text('更換字號'));
      expect(find.byType(TextField), findsOneWidget);
      await tester.enterText(find.byType(TextField), 'b-9');
      await tester.pump();
      await _tap(tester, find.text('取消'));
      expect(find.byType(TextField), findsNothing);
      expect(find.text('已登錄：A-1'), findsOneWidget);
      expect(server.submits, isEmpty);
      await _tap(tester, find.text('更換字號'));
      expect(_fieldText(tester), isEmpty);
    });

    testWidgets('清除：先確認，確認後打 ClearFoodRegistration；取消不送', (tester) async {
      final server = _Server(
        payuni: _payuniApproved,
        foodRegistrationNumber: 'A-1',
      );
      await _pump(tester, server);
      await openFood(tester);
      await _tap(tester, find.text('清除'));
      expect(find.text('清除後，店面與商品頁不再顯示食品業者登錄字號，也無法上架加工食品類商品。'), findsOneWidget);
      await _tap(tester, find.text('取消'));
      expect(server.clears, 0);

      await _tap(tester, find.text('清除'));
      await _tap(
        tester,
        find.descendant(of: find.byType(Dialog), matching: find.text('清除')),
      );
      expect(server.clears, 1);
      expect(find.text('已登錄：A-1'), findsNothing);
      expect(find.text('未登錄'), findsOneWidget);
    });

    testWidgets('其他失敗顯示錯誤，輸入保留', (tester) async {
      final server = _Server(payuni: _payuniApproved)
        ..updateFailure = ConnectException(Code.unavailable, 'later');
      await _pump(tester, server);
      await openFood(tester);
      await tester.enterText(find.byType(TextField), 'B-9');
      await tester.pump();
      await _readNotice(tester);
      await _tap(tester, find.text('送出審核'));

      expect(server.submits, ['B-9']);
      expect(find.textContaining('審核中'), findsNothing);
      expect(_fieldText(tester), 'B-9');
    });

    testWidgets('被平台撤銷：顯示字號與理由，按鈕是重新送出，送出後撤銷的說明消失', (tester) async {
      final server = _Server(
        payuni: _payuniApproved,
        revokedFood: FoodRegistration(
          id: 'fr-1',
          state: FoodRegistrationState.FOOD_REGISTRATION_STATE_ENDED,
          registrationNumber: 'A-9',
          endReason: '字號與食藥署資料不符',
        ),
      );
      await _pump(tester, server);
      await openFood(tester);
      expect(
        find.text('已被平台撤銷：A-9。理由：字號與食藥署資料不符。請確認字號與食藥署登錄資料一致後重新送出審核。'),
        findsOneWidget,
      );
      expect(find.text('送出審核'), findsNothing);
      await tester.enterText(find.byType(TextField), 'a-10');
      await tester.pump();
      await _readNotice(tester);
      await _tap(tester, find.text('重新送出'));
      expect(server.submits, ['A-10']);
      expect(find.textContaining('已被平台撤銷'), findsNothing);
      expect(find.textContaining('審核中：A-10'), findsOneWidget);
    });

    // 撤銷的說明由伺服器決定要不要帶：撤銷時就已存在的同字號待審件，兩者伺服器都帶，client 原樣顯示。
    testWidgets('伺服器同時帶撤銷與同字號的待審：兩個都顯示', (tester) async {
      final server = _Server(
        payuni: _payuniApproved,
        revokedFood: FoodRegistration(
          id: 'fr-1',
          state: FoodRegistrationState.FOOD_REGISTRATION_STATE_ENDED,
          registrationNumber: 'A-9',
          endReason: '不符',
        ),
        foodRequest: FoodRegistration(
          id: 'fr-2',
          state: FoodRegistrationState.FOOD_REGISTRATION_STATE_PENDING,
          registrationNumber: 'A-9',
          submitTime: Timestamp.fromDateTime(DateTime.utc(2026, 10, 9, 4)),
        ),
      );
      await _pump(tester, server);
      await openFood(tester);
      expect(find.textContaining('已被平台撤銷：A-9'), findsOneWidget);
      expect(find.textContaining('審核中：A-9'), findsOneWidget);
    });

    testWidgets('伺服器同時帶撤銷與被退回的請求：兩個都顯示，按鈕是重新送出', (tester) async {
      final server = _Server(
        payuni: _payuniApproved,
        revokedFood: FoodRegistration(
          id: 'fr-1',
          state: FoodRegistrationState.FOOD_REGISTRATION_STATE_ENDED,
          registrationNumber: 'A-9',
          endReason: '不符',
        ),
        foodRequest: FoodRegistration(
          id: 'fr-2',
          state: FoodRegistrationState.FOOD_REGISTRATION_STATE_REJECTED,
          registrationNumber: 'A-10',
          rejectReason: '查無此字號',
          submitTime: Timestamp.fromDateTime(DateTime.utc(2026, 10, 9, 4)),
        ),
      );
      await _pump(tester, server);
      await openFood(tester);
      expect(find.textContaining('已被平台撤銷：A-9'), findsOneWidget);
      expect(find.textContaining('未通過：查無此字號'), findsOneWidget);
      expect(find.text('重新送出'), findsOneWidget);
    });

    testWidgets('伺服器沒有帶撤銷：不顯示撤銷的說明，按鈕是送出審核', (tester) async {
      final server = _Server(payuni: _payuniApproved);
      await _pump(tester, server);
      await openFood(tester);
      expect(find.textContaining('已被平台撤銷'), findsNothing);
      expect(find.text('送出審核'), findsOneWidget);
    });

    testWidgets('字號欄位違規：顯示欄位訊息', (tester) async {
      final server = _Server(payuni: _payuniApproved)
        ..submitFailure = fieldError('registration_number', '請輸入食品業者登錄字號');
      await _pump(tester, server);
      await openFood(tester);
      await tester.enterText(find.byType(TextField), 'B-9');
      await tester.pump();
      await _readNotice(tester);
      await _tap(tester, find.text('送出審核'));
      expect(find.text('請輸入食品業者登錄字號'), findsOneWidget);
    });

    testWidgets('告知過期：取消勾選、重讀文件、提示重新勾選', (tester) async {
      final server = _Server(payuni: _payuniApproved)
        ..submitFailure = reasonError(
          Code.failedPrecondition,
          'AGREEMENT_OUTDATED',
        );
      await _pump(tester, server);
      await openFood(tester);
      await tester.enterText(find.byType(TextField), 'B-9');
      await tester.pump();
      await _readNotice(tester);
      final before = server.agreementsCalls;
      server.noticeVersion = 'n2';
      await _tap(tester, find.text('送出審核'));
      expect(find.text('條款已更新，請重新閱讀後再勾選。'), findsOneWidget);
      expect(
        tester
            .widget<Checkbox>(find.byKey(const Key('food_notice_checkbox')))
            .value,
        isFalse,
      );
      expect(server.agreementsCalls, greaterThan(before));
      server.submitFailure = null;
      await _readNotice(tester);
      await _tap(tester, find.text('送出審核'));
      expect(server.submittedAgreements.last.single.version, 'n2');
    });
  });
}
