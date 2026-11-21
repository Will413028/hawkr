import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:hawkr_api/gen/google/rpc/error_details.pb.dart'
    show LocalizedMessage;
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/policy_category.pbenum.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/report.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/report.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/report.pb.dart' as pb;
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_buyer/config/flavor.dart';
import 'package:hawkr_buyer/core/networking/connect_provider.dart';
import 'package:hawkr_buyer/core/session/session_model.dart';
import 'package:hawkr_buyer/core/session/session_provider.dart';
import 'package:hawkr_buyer/core/theme/app_theme.dart';
import 'package:hawkr_buyer/features/moderation/data/store_report_repository.dart';
import 'package:hawkr_buyer/features/moderation/presentation/store_report_entry.dart';
import 'package:hawkr_buyer/features/moderation/presentation/store_report_screen.dart';
import 'package:hawkr_ui_text/hawkr_ui_text.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

class _Session extends Session {
  _Session(this.loggedIn);
  final bool loggedIn;
  @override
  Future<SessionModel?> build() async => loggedIn
      ? const SessionModel(userId: 'buyer-account', nickname: '測試買家')
      : null;
}

class _FakeReports {
  _FakeReports({this.supplemented = false});
  final bool supplemented;
  final List<pb.CreateReportRequest> requests = [];

  SellerReportAppServiceClient client() => SellerReportAppServiceClient(
    FakeTransportBuilder().unary(SellerReportAppService.createReport, (
      req,
      _,
    ) async {
      requests.add(req);
      return pb.CreateReportResponse(
        report: pb.StoreReport(id: 'r1'),
        supplemented: supplemented,
        notice: supplemented
            ? LocalizedMessage(message: '伺服器說明：已補充到今天的同一筆檢舉')
            : null,
      );
    }).build(),
  );
}

Future<void> _pump(
  WidgetTester tester, {
  required bool loggedIn,
  required _FakeReports reports,
  StoreReportArgs args = const StoreReportArgs(storeId: 'store-1'),
}) async {
  tester.view.devicePixelRatio = 1;
  tester.view.physicalSize = const Size(390, 844);
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);
  final container = ProviderContainer(
    overrides: [
      sellerReportAppClientProvider.overrideWithValue(reports.client()),
      sessionProvider.overrideWith(() => _Session(loggedIn)),
    ],
  );
  addTearDown(container.dispose);
  await container.read(sessionProvider.future);
  final router = GoRouter(
    routes: [
      GoRoute(
        path: '/',
        builder: (_, _) => Scaffold(
          body: Consumer(
            builder: (context, ref, _) => TextButton(
              onPressed: () => showStoreReportMenu(context, ref, args: args),
              child: const Text('更多'),
            ),
          ),
        ),
      ),
      GoRoute(
        path: storeReportRoute,
        redirect: (_, state) => storeReportRedirect(state.extra),
        builder: (_, state) =>
            StoreReportScreen(args: state.extra! as StoreReportArgs),
      ),
      GoRoute(
        path: '/login',
        builder: (_, _) => const Scaffold(body: Text('登入頁')),
      ),
    ],
  );
  addTearDown(router.dispose);
  await tester.pumpWidget(
    UncontrolledProviderScope(
      container: container,
      child: ScreenUtilInit(
        designSize: const Size(390, 844),
        enableScaleWH: () => false,
        enableScaleText: () => false,
        builder: (_, _) =>
            MaterialApp.router(theme: AppTheme.light, routerConfig: router),
      ),
    ),
  );
  await tester.pumpAndSettle();
}

Future<void> _openForm(WidgetTester tester) async {
  await tester.tap(find.text('更多'));
  await tester.pumpAndSettle();
  await tester.tap(find.byKey(const ValueKey('store-report-menu-report')));
  await tester.pumpAndSettle();
}

void main() {
  if (!F.isInitialized) F.init(Flavor.dev);

  test('類別清單不含 UNSPECIFIED 與 PLATFORM_ABUSE，其餘都有標籤', () {
    expect(
      storeReportCategories,
      isNot(contains(PolicyCategory.POLICY_CATEGORY_PLATFORM_ABUSE)),
    );
    expect(
      storeReportCategories,
      isNot(contains(PolicyCategory.POLICY_CATEGORY_UNSPECIFIED)),
    );
    expect(
      storeReportCategories,
      contains(PolicyCategory.POLICY_CATEGORY_FOOD_SAFETY),
    );
    for (final c in storeReportCategories) {
      expect(c.uiText(UiLocale.zhTW), isNotNull, reason: '$c');
    }
  });

  test('畫面選項只含有標籤的可檢舉類別', () {
    final options = storeReportCategoryOptions(UiLocale.zhTW);
    expect(options, isNotEmpty);
    final cats = options.map((o) => o.$1);
    expect(
      cats,
      isNot(contains(PolicyCategory.POLICY_CATEGORY_PLATFORM_ABUSE)),
    );
    expect(cats, isNot(contains(PolicyCategory.POLICY_CATEGORY_UNSPECIFIED)));
    for (final o in options) {
      expect(o.$2, isNotEmpty);
    }
  });

  test('成功提示：補充且 notice 有文字才用 notice，否則已收到檢舉', () {
    expect(storeReportSuccessText(pb.CreateReportResponse()), contains('我的檢舉'));
    expect(
      storeReportSuccessText(pb.CreateReportResponse(supplemented: true)),
      contains('我的檢舉'),
    );
    expect(
      storeReportSuccessText(
        pb.CreateReportResponse(
          supplemented: true,
          notice: LocalizedMessage(message: '  '),
        ),
      ),
      contains('我的檢舉'),
    );
    expect(
      storeReportSuccessText(
        pb.CreateReportResponse(
          supplemented: true,
          notice: LocalizedMessage(message: '已補充'),
        ),
      ),
      '已補充',
    );
  });

  test('沒有店家 id 的 /report/store 轉回首頁', () {
    expect(storeReportRedirect(null), '/');
    expect(storeReportRedirect(const StoreReportArgs(storeId: '')), '/');
    expect(storeReportRedirect(const StoreReportArgs(storeId: 's')), isNull);
  });

  testWidgets('訪客點檢舉走登入閘門，不顯示表單', (tester) async {
    final reports = _FakeReports();
    await _pump(tester, loggedIn: false, reports: reports);
    await _openForm(tester);
    expect(find.text('需要登入'), findsOneWidget);
    expect(find.byKey(const ValueKey('store-report-submit')), findsNothing);
    await tester.tap(find.text('前往登入'));
    await tester.pumpAndSettle();
    expect(find.text('登入頁'), findsOneWidget);
    expect(find.byKey(const ValueKey('store-report-submit')), findsNothing);
    expect(reports.requests, isEmpty);
  });

  testWidgets('登入的買家進表單，送出帶商品與類別', (tester) async {
    final reports = _FakeReports();
    await _pump(
      tester,
      loggedIn: true,
      reports: reports,
      args: const StoreReportArgs(
        storeId: 'store-1',
        productId: 'p-9',
        productName: '好吃的麵包',
      ),
    );
    await _openForm(tester);
    expect(find.text('檢舉商品'), findsOneWidget);
    expect(find.text('檢舉商品：好吃的麵包'), findsOneWidget);
    final submit = find.byKey(const ValueKey('store-report-submit'));
    expect(tester.widget<ElevatedButton>(submit).onPressed, isNull);

    final label = PolicyCategory.POLICY_CATEGORY_FOOD_SAFETY.uiText(
      UiLocale.zhTW,
    )!;
    await tester.ensureVisible(find.text(label));
    await tester.tap(find.text(label));
    await tester.enterText(
      find.byKey(const ValueKey('store-report-description')),
      '  麵包有異物  ',
    );
    await tester.pump();
    await tester.ensureVisible(submit);
    await tester.tap(submit);
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 400));

    expect(reports.requests, hasLength(1));
    final req = reports.requests.single;
    expect(req.storeId, 'store-1');
    expect(req.productId, 'p-9');
    expect(req.category, PolicyCategory.POLICY_CATEGORY_FOOD_SAFETY);
    expect(req.description, '麵包有異物');
    expect(find.textContaining('我的檢舉'), findsOneWidget);
    await tester.pump(const Duration(seconds: 3));
  });

  testWidgets('supplemented 時顯示伺服器的 notice', (tester) async {
    final reports = _FakeReports(supplemented: true);
    await _pump(tester, loggedIn: true, reports: reports);
    await _openForm(tester);
    expect(find.text('檢舉店家'), findsOneWidget);
    final spam = find.text(
      PolicyCategory.POLICY_CATEGORY_SPAM.uiText(UiLocale.zhTW)!,
    );
    await tester.ensureVisible(spam);
    await tester.tap(spam);
    await tester.enterText(
      find.byKey(const ValueKey('store-report-description')),
      '重複內容',
    );
    await tester.pump();
    final send = find.byKey(const ValueKey('store-report-submit'));
    await tester.ensureVisible(send);
    await tester.tap(send);
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 400));
    expect(find.text('伺服器說明：已補充到今天的同一筆檢舉'), findsOneWidget);
    expect(find.textContaining('我的檢舉'), findsNothing);
    await tester.pump(const Duration(seconds: 3));
  });
}
