import 'dart:async';

import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/gen/google/rpc/error_details.pb.dart'
    show LocalizedMessage;
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_api/transport.dart';
import 'package:hawkr_seller/config/flavor.dart';
import 'package:hawkr_seller/core/networking/connect_provider.dart';
import 'package:hawkr_seller/core/router/app_router.dart';
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
  late final FakeMyStoreServer store;
  Completer<void>? kycGate;
  var now = DateTime(2026, 10, 9, 12);
  void advance(Duration d) => now = now.add(d);
  int kycCalls = 0;

  GetMyStoreResponse _storeResponse() => myStoreResponse(
    capabilities: marketing
        ? null
        : [SellerCapability.SELLER_CAPABILITY_STOREFRONT_WRITE],
    publicListing: publicListingMissing([
      if (!certified) SellerRequirement.SELLER_REQUIREMENT_KYC_APPROVED,
    ]),
  );

  void reloadStore() => store.response = _storeResponse();

  SellerAppServiceClient get client => SellerAppServiceClient(
    FakeTransportBuilder().unary(SellerAppService.getMyKYC, (_, _) async {
      kycCalls++;
      final gate = kycGate;
      if (gate != null) await gate.future;
      final answer = kyc;
      if (answer is ConnectException) throw answer;
      return answer as GetMyKYCResponse;
    }).build(),
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

List<Override> _overrides(_Server server) => [
  refreshClockProvider.overrideWithValue(() => server.now),
  sessionProvider.overrideWith(_LoggedIn.new),
  sellerClientProvider.overrideWithValue(server.client),
  myStoreRepositoryProvider.overrideWithValue(server.store.repository),
  couponListControllerProvider.overrideWith(_CouponFixture.new),
];

Future<GoRouter> _pump(
  WidgetTester tester,
  _Server server, {
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
        : ProviderScope(overrides: _overrides(server), child: app),
  );
  await _settle(tester);
  return router;
}

Future<void> _settle(WidgetTester tester) async {
  for (var i = 0; i < 12; i++) {
    await tester.pump(const Duration(milliseconds: 50));
  }
}

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
        application: individualApplication(
          status: _approvedFlow,
          payuni: _payuniApproved,
        ),
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

    testWidgets('GetMyStore 失敗而且沒有快照：同樣的錯誤面板與重試', (tester) async {
      final server = _Server(payuni: _payuniApproved);
      server.store.failure = ConnectException(Code.unavailable, 'later');
      await _pump(tester, server);
      expect(find.text('無法載入認證狀態'), findsOneWidget);
      expect(find.text('帳戶與認證'), findsNothing);

      server.store.failure = null;
      await _tap(tester, find.text('重試'));
      expect(find.text('無法載入認證狀態'), findsNothing);
      expect(find.text('帳戶與認證'), findsOneWidget);
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
      await _tap(tester, find.byKey(const ValueKey('wallet-cert-row')));
      expect(find.text('商家認證已通過'), findsOneWidget);
    });

    testWidgets('店家未認證：不顯示帳戶與認證', (tester) async {
      final server = _Server(status: _payuniReview, certified: false);
      await _pump(tester, server);
      expect(find.text('帳戶與認證'), findsNothing);
      expect(find.byKey(const ValueKey('wallet-cert-row')), findsNothing);
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
}
