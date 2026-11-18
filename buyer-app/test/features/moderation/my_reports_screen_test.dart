import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/policy_category.pbenum.dart';
import 'package:hawkr_api/gen/hawkr/moderation/v1alpha1/moderation.pbenum.dart';
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:hawkr_buyer/core/networking/connect_provider.dart';
import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/app_typography.dart';
import 'package:hawkr_buyer/features/moderation/presentation/my_reports_screen.dart';
import 'package:hawkr_buyer/shared/widgets/hk_empty_state.dart';
import 'package:hawkr_buyer/shared/widgets/truck_loading_overlay.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import 'moderation_fakes.dart';

Future<void> _pump(
  WidgetTester tester,
  FakeModeration moderation, {
  bool settle = true,
}) async {
  tester.view.devicePixelRatio = 1;
  tester.view.physicalSize = const Size(390, 844);
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);
  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        moderationAppClientProvider.overrideWithValue(moderation.client()),
      ],
      child: ScreenUtilInit(
        designSize: const Size(390, 844),
        enableScaleWH: () => false,
        enableScaleText: () => false,
        builder: (_, _) => MaterialApp(
          theme: ThemeData(
            useMaterial3: true,
            colorScheme: ColorScheme.fromSeed(seedColor: AppColors.primary),
            textTheme: AppTypography.textTheme,
          ),
          home: const MyReportsScreen(),
        ),
      ),
    ),
  );
  if (settle) await tester.pumpAndSettle();
}

void main() {
  testWidgets('只讀 ListMyReports：對象種類、原因（共用目錄）、說明、結果', (tester) async {
    final moderation = FakeModeration(
      reportPages: [
        [
          reportRecord(
            '1',
            description: '廣告內容',
            outcome: ReportOutcome.REPORT_OUTCOME_VIOLATION,
          ),
          reportRecord(
            '2',
            kind: ReportTargetKind.REPORT_TARGET_KIND_COMMENT,
            category: PolicyCategory.POLICY_CATEGORY_HARASSMENT,
          ),
          reportRecord(
            '3',
            kind: ReportTargetKind.REPORT_TARGET_KIND_FORUM_ENTRY,
            category: PolicyCategory.POLICY_CATEGORY_HATE,
            outcome: ReportOutcome.REPORT_OUTCOME_NO_VIOLATION,
          ),
        ],
      ],
    );
    await _pump(tester, moderation);

    expect(find.text('我的檢舉'), findsOneWidget);
    expect(find.text('貼文'), findsOneWidget);
    expect(find.text('留言'), findsOneWidget);
    expect(find.text('討論區留言'), findsOneWidget);
    expect(find.text('原因：垃圾訊息'), findsOneWidget);
    expect(find.text('原因：騷擾或霸凌'), findsOneWidget);
    expect(find.text('原因：仇恨或歧視言論'), findsOneWidget);
    expect(find.text('廣告內容'), findsOneWidget);
    expect(find.text('已處理（違規）'), findsOneWidget);
    expect(find.text('審核中'), findsOneWidget);
    expect(find.text('已處理（不違規）'), findsOneWidget);
    expect(find.text('VIOLATION'), findsNothing);
    expect(moderation.listRequests.single.pageToken, '');
  });

  testWidgets('UNSPECIFIED 的結果與原因不顯示成有效值', (tester) async {
    final moderation = FakeModeration(
      reportPages: [
        [
          reportRecord(
            '1',
            category: PolicyCategory.POLICY_CATEGORY_UNSPECIFIED,
            outcome: ReportOutcome.REPORT_OUTCOME_UNSPECIFIED,
          ),
        ],
      ],
    );
    await _pump(tester, moderation);

    expect(find.text('貼文'), findsOneWidget);
    expect(find.textContaining('原因：'), findsNothing);
    expect(find.text('審核中'), findsNothing);
  });

  testWidgets('分頁：「查看更多」帶 next_page_token 並接在後面', (tester) async {
    final moderation = FakeModeration(
      reportPages: [
        [reportRecord('1', description: '第一頁')],
        [reportRecord('2', description: '第二頁')],
      ],
    );
    await _pump(tester, moderation);

    expect(find.text('第二頁'), findsNothing);
    await tester.tap(find.byKey(const ValueKey('my-reports-more')));
    await tester.pumpAndSettle();

    expect(moderation.listRequests.map((r) => r.pageToken), ['', 'page-1']);
    expect(find.text('第一頁'), findsOneWidget);
    expect(find.text('第二頁'), findsOneWidget);
    expect(find.byKey(const ValueKey('my-reports-more')), findsNothing);
  });

  testWidgets('無檢舉紀錄時顯示空狀態', (tester) async {
    await _pump(tester, FakeModeration());

    final empty = find.byType(HkEmptyState);
    expect(empty, findsOneWidget);
    expect(tester.widget<HkEmptyState>(empty).message, '尚無檢舉紀錄');
  });

  testWidgets('載入中顯示 TruckLoadingOverlay', (tester) async {
    final moderation = FakeModeration()..listGate = Completer<void>();
    await _pump(tester, moderation, settle: false);
    await tester.pump();

    expect(find.byType(TruckLoadingOverlay), findsOneWidget);
    moderation.listGate!.complete();
    await tester.pumpAndSettle();
  });

  testWidgets('載入失敗：錯誤狀態與空狀態分開，可重試', (tester) async {
    final moderation = FakeModeration(
      reportPages: [
        [reportRecord('1')],
      ],
      listError: ConnectException(Code.unavailable, 'down'),
    );
    await _pump(tester, moderation);

    expect(find.byType(HkEmptyState), findsNothing);
    expect(find.text('載入失敗，請稍後再試'), findsOneWidget);

    moderation.listError = null;
    await tester.tap(find.text('重試'));
    await tester.pumpAndSettle();

    expect(find.text('貼文'), findsOneWidget);
    expect(find.text('載入失敗，請稍後再試'), findsNothing);
  });
}
