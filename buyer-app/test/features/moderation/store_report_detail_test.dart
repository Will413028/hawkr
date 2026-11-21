import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/policy_category.pbenum.dart';
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/store_card.pb.dart';
import 'package:hawkr_api/gen/hawkr/moderation/v1alpha1/moderation.pb.dart'
    as mod;
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/report.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/report.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/report.pb.dart' as pb;
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/report.pbenum.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_buyer/config/flavor.dart';
import 'package:hawkr_buyer/core/networking/connect_provider.dart';
import 'package:hawkr_buyer/core/theme/app_theme.dart';
import 'package:hawkr_buyer/features/moderation/presentation/my_reports_screen.dart';
import 'package:hawkr_buyer/features/moderation/presentation/store_report_detail_screen.dart';
import 'package:hawkr_buyer/features/moderation/presentation/store_report_supplement_screen.dart';
import 'package:hawkr_buyer/features/seller/data/seller_repository.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../support/fake_seller_repository.dart';
import 'moderation_fakes.dart';

pb.StoreReport _store({
  String id = 'r1',
  ReportOutcome? outcome,
  String note = '',
  String orderNo = '',
}) => pb.StoreReport(
  id: id,
  storeId: 'store-1',
  category: PolicyCategory.POLICY_CATEGORY_SPAM,
  description: '店家寄來廣告',
  orderNo: orderNo,
  resolutionNote: note,
  outcome: outcome,
);

mod.MyReportItem _storeItem(pb.StoreReport r) => mod.MyReportItem(
  id: r.id,
  category: r.category,
  description: r.description,
  createTime: r.createTime,
  store: r,
);

pb.ReportSupplement _supp(String id, List<int> positions) =>
    pb.ReportSupplement(
      id: id,
      description: '補充$id',
      evidencePositions: positions,
    );

class _FakeSellerReports {
  _FakeSellerReports(this.detail);
  pb.GetMyReportResponse detail;
  int gets = 0;
  final List<pb.AddReportSupplementRequest> added = [];

  SellerReportAppServiceClient client() => SellerReportAppServiceClient(
    FakeTransportBuilder()
        .unary(SellerReportAppService.getMyReport, (req, _) async {
          gets++;
          return detail;
        })
        .unary(SellerReportAppService.addReportSupplement, (req, _) async {
          added.add(req);
          detail = pb.GetMyReportResponse(
            report: detail.report,
            supplements: [...detail.supplements, _supp('new', const [])],
          );
          return pb.AddReportSupplementResponse(report: detail.report);
        })
        .build(),
  );
}

Future<void> _pump(
  WidgetTester tester, {
  required FakeModeration moderation,
  required _FakeSellerReports reports,
  String initial = '/settings/reports',
}) async {
  tester.view.devicePixelRatio = 1;
  tester.view.physicalSize = const Size(390, 844);
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);
  final router = GoRouter(
    initialLocation: initial,
    routes: [
      GoRoute(
        path: '/settings/reports',
        builder: (_, _) => const MyReportsScreen(),
        routes: [
          GoRoute(
            path: ':reportId',
            builder: (_, s) => StoreReportDetailScreen(
              reportId: s.pathParameters['reportId']!,
            ),
            routes: [
              GoRoute(
                path: 'supplement',
                builder: (_, s) => StoreReportSupplementScreen(
                  reportId: s.pathParameters['reportId']!,
                ),
              ),
            ],
          ),
        ],
      ),
    ],
  );
  addTearDown(router.dispose);
  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        moderationAppClientProvider.overrideWithValue(moderation.client()),
        sellerReportAppClientProvider.overrideWithValue(reports.client()),
        sellerRepositoryProvider.overrideWithValue(
          fakeSellerRepository(
            cards: [StoreCard(id: 'store-1', name: '好店')],
          ),
        ),
      ],
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

final _button = find.byKey(const ValueKey('store-report-supplement-button'));

void main() {
  if (!F.isInitialized) F.init(Flavor.dev);

  testWidgets('清單：社群與店家兩種都顯示，店家卡片有店名與審核中', (tester) async {
    final moderation = FakeModeration(
      reportPages: [
        [reportRecord('c1', description: '社群內容'), _storeItem(_store())],
      ],
    );
    await _pump(
      tester,
      moderation: moderation,
      reports: _FakeSellerReports(pb.GetMyReportResponse(report: _store())),
    );
    expect(find.text('貼文'), findsOneWidget);
    expect(find.text('好店'), findsOneWidget);
    expect(find.text('店家寄來廣告'), findsOneWidget);
    expect(find.text('審核中'), findsWidgets);
    expect(find.byKey(const ValueKey('my-report-store-r1')), findsOneWidget);
    expect(find.byKey(const ValueKey('my-report-store-c1')), findsNothing);
  });

  testWidgets('清單：只有店家檢舉時不顯示空狀態，結果走目錄文字', (tester) async {
    final moderation = FakeModeration(
      reportPages: [
        [
          _storeItem(
            _store(outcome: ReportOutcome.REPORT_OUTCOME_NO_VIOLATION),
          ),
        ],
      ],
    );
    await _pump(
      tester,
      moderation: moderation,
      reports: _FakeSellerReports(pb.GetMyReportResponse(report: _store())),
    );
    expect(find.text('尚無檢舉紀錄'), findsNothing);
    expect(find.text('已處理（不違規）'), findsOneWidget);
    expect(find.text('審核中'), findsNothing);
  });

  testWidgets('詳情：處理中顯示各段落、佐證張數分開計、補充按鈕', (tester) async {
    final reports = _FakeSellerReports(
      pb.GetMyReportResponse(
        report: _store(orderNo: 'NO-9'),
        supplements: [
          _supp('a', [2]),
          _supp('b', [3, 4]),
        ],
        evidence: [1, 2, 3, 4].map((p) => pb.ReportEvidence(position: p)),
      ),
    );
    await _pump(
      tester,
      moderation: FakeModeration(),
      reports: reports,
      initial: '/settings/reports/r1',
    );
    expect(find.text('檢舉詳情'), findsOneWidget);
    expect(find.text('好店'), findsOneWidget);
    expect(find.text('審核中'), findsOneWidget);
    expect(find.text('訂單編號：NO-9'), findsOneWidget);
    expect(find.text('平台說明'), findsNothing);
    expect(find.text('補充說明（2/10）'), findsOneWidget);
    expect(find.text('已附 1 張佐證'), findsNWidgets(2));
    expect(find.text('已附 2 張佐證'), findsOneWidget);
    expect(_button, findsOneWidget);
  });

  testWidgets('詳情：有結果時隱藏補充按鈕並顯示平台說明', (tester) async {
    final reports = _FakeSellerReports(
      pb.GetMyReportResponse(
        report: _store(
          outcome: ReportOutcome.REPORT_OUTCOME_ACTION_TAKEN,
          note: '已下架商品',
        ),
      ),
    );
    await _pump(
      tester,
      moderation: FakeModeration(),
      reports: reports,
      initial: '/settings/reports/r1',
    );
    expect(find.text('平台說明'), findsOneWidget);
    expect(find.text('已下架商品'), findsOneWidget);
    expect(find.text('審核中'), findsNothing);
    expect(_button, findsNothing);
  });

  testWidgets('詳情：補充滿 10 則時隱藏按鈕', (tester) async {
    final reports = _FakeSellerReports(
      pb.GetMyReportResponse(
        report: _store(),
        supplements: [for (var i = 0; i < 10; i++) _supp('$i', const [])],
      ),
    );
    await _pump(
      tester,
      moderation: FakeModeration(),
      reports: reports,
      initial: '/settings/reports/r1',
    );
    expect(find.text('補充說明（10/10）'), findsOneWidget);
    expect(_button, findsNothing);
  });

  testWidgets('補充：說明必填，成功顯示提示並重讀詳情與清單', (tester) async {
    final moderation = FakeModeration(
      reportPages: [
        [_storeItem(_store())],
      ],
    );
    final reports = _FakeSellerReports(
      pb.GetMyReportResponse(report: _store()),
    );
    await _pump(tester, moderation: moderation, reports: reports);
    await tester.tap(find.byKey(const ValueKey('my-report-store-r1')));
    await tester.pumpAndSettle();
    expect(reports.gets, 1);
    await tester.tap(_button);
    await tester.pumpAndSettle();

    final submit = find.byKey(const ValueKey('store-report-supplement-submit'));
    expect(tester.widget<ElevatedButton>(submit).onPressed, isNull);
    await tester.enterText(
      find.byKey(const ValueKey('store-report-supplement-description')),
      '  更多經過  ',
    );
    await tester.pump();
    await tester.tap(submit);
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 400));

    expect(reports.added.single.reportId, 'r1');
    expect(reports.added.single.description, '更多經過');
    expect(find.text('已收到補充說明'), findsOneWidget);
    await tester.pump(const Duration(seconds: 3));
    await tester.pumpAndSettle();
    expect(reports.gets, 2);
    expect(moderation.listRequests.length, 2);
    expect(find.text('補充說明（1/10）'), findsOneWidget);
  });
}
