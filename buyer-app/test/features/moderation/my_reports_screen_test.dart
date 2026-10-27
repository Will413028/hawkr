import 'dart:async';

import 'package:hawkr_buyer/core/exceptions/app_exception.dart';
import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/app_typography.dart';
import 'package:hawkr_buyer/features/moderation/data/report_record_model.dart';
import 'package:hawkr_buyer/features/moderation/presentation/my_reports_screen.dart';
import 'package:hawkr_buyer/features/social/data/social_repository.dart';
import 'package:hawkr_buyer/shared/widgets/hk_empty_state.dart';
import 'package:hawkr_buyer/shared/widgets/truck_loading_overlay.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:mocktail/mocktail.dart';

/// 補齊 Task 4 review 指出的缺口：brief Step 9（手動導覽驗證）在無人值守環境
/// 無法用真實裝置登入操作，改以本測試檔案作為可自動執行、可重現的替代驗證——
/// 用 mock repository（依 `my_subscription_social_screen_test.dart` 既有慣例）
/// 實際 pump `MyReportsScreen` 並斷言 loading / 有資料 / empty state 三態渲染
/// 正確，以及下拉刷新確實重新呼叫 `getMyReports()`。
class _MockSocialRepository extends Mock implements SocialRepository {}

void main() {
  late _MockSocialRepository socialRepository;

  setUp(() {
    socialRepository = _MockSocialRepository();
  });

  testWidgets('載入中顯示 TruckLoadingOverlay', (tester) async {
    final completer = Completer<List<ReportRecord>>();
    when(
      () => socialRepository.getMyReports(),
    ).thenAnswer((_) => completer.future);

    await _pumpScreen(
      tester,
      socialRepository: socialRepository,
      settle: false,
    );
    await tester.pump();

    expect(find.byType(TruckLoadingOverlay), findsOneWidget);

    completer.complete(<ReportRecord>[]);
    await tester.pumpAndSettle();
  });

  testWidgets('有檢舉紀錄時渲染目標類型／原因／描述／時間', (tester) async {
    when(() => socialRepository.getMyReports()).thenAnswer(
      (_) async => [
        ReportRecord.fromJson(<String, dynamic>{
          'id': '1',
          'targetType': 'POST',
          'targetId': '111',
          'reasonCode': 'SPAM',
          'description': '廣告內容',
          'status': 'PENDING',
          'createTimeMs': 1721800000000,
        }),
        ReportRecord.fromJson(<String, dynamic>{
          'id': '2',
          'targetType': 'USER',
          'targetId': '222',
          'reasonCode': 'HARASSMENT',
          'status': 'RESOLVED_HIDDEN',
          'createTimeMs': 1721700000000,
        }),
      ],
    );

    await _pumpScreen(tester, socialRepository: socialRepository);

    expect(find.text('我的檢舉'), findsOneWidget);
    expect(find.byType(TruckLoadingOverlay), findsNothing);
    expect(find.byType(HkEmptyState), findsNothing);

    expect(find.text('貼文'), findsOneWidget);
    expect(find.text('原因：垃圾訊息 / 廣告'), findsOneWidget);
    expect(find.text('廣告內容'), findsOneWidget);
    expect(find.text('使用者'), findsOneWidget);
    expect(find.text('原因：騷擾 / 仇恨言論'), findsOneWidget);
    // 狀態徽章需轉譯成中文，不可直接顯示後端原始字串（PENDING/RESOLVED_HIDDEN）。
    expect(find.text('審核中'), findsOneWidget);
    expect(find.text('已處理（已隱藏）'), findsOneWidget);
    expect(find.text('PENDING'), findsNothing);
    expect(find.text('RESOLVED_HIDDEN'), findsNothing);
    expect(tester.takeException(), isNull);
  });

  testWidgets('未知狀態值 fallback 顯示原始字串', (tester) async {
    when(() => socialRepository.getMyReports()).thenAnswer(
      (_) async => [
        ReportRecord.fromJson(<String, dynamic>{
          'id': '3',
          'targetType': 'POST',
          'targetId': '333',
          'reasonCode': 'SPAM',
          'status': 'SOME_FUTURE_STATUS',
          'createTimeMs': 1721800000000,
        }),
      ],
    );

    await _pumpScreen(tester, socialRepository: socialRepository);

    expect(find.text('SOME_FUTURE_STATUS'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('無檢舉紀錄時顯示 empty state', (tester) async {
    when(() => socialRepository.getMyReports()).thenAnswer((_) async => []);

    await _pumpScreen(tester, socialRepository: socialRepository);

    final emptyState = find.byType(HkEmptyState);
    expect(emptyState, findsOneWidget);
    expect(tester.widget<HkEmptyState>(emptyState).message, '尚無檢舉紀錄');
    expect(tester.takeException(), isNull);
  });

  testWidgets('載入失敗時顯示錯誤狀態（與真的沒資料的 empty state 區分），並可重試', (tester) async {
    var callCount = 0;
    when(() => socialRepository.getMyReports()).thenAnswer((_) {
      callCount++;
      if (callCount == 1) throw const NetworkException();
      return Future.value(<ReportRecord>[]);
    });

    await _pumpScreen(tester, socialRepository: socialRepository);

    expect(find.byType(TruckLoadingOverlay), findsNothing);
    expect(
      find.byType(HkEmptyState),
      findsNothing,
      reason: '讀取失敗不應顯示「尚無檢舉紀錄」的空狀態，會誤導使用者以為真的沒有資料',
    );
    expect(find.text('網路連線失敗，請稍後再試'), findsOneWidget);
    expect(find.text('重試'), findsOneWidget);
    expect(tester.takeException(), isNull);

    await tester.tap(find.text('重試'));
    await tester.pumpAndSettle();

    expect(
      find.byType(HkEmptyState),
      findsOneWidget,
      reason: '重試成功且回傳空陣列，才進入真正的 empty state',
    );
    expect(find.text('網路連線失敗，請稍後再試'), findsNothing);
    expect(tester.takeException(), isNull);
  });

  testWidgets('下拉刷新會重新呼叫 getMyReports()', (tester) async {
    when(() => socialRepository.getMyReports()).thenAnswer(
      (_) async => [
        ReportRecord.fromJson(<String, dynamic>{
          'id': '1',
          'targetType': 'POST',
          'targetId': '111',
          'reasonCode': 'SPAM',
          'status': 'PENDING',
          'createTimeMs': 1721800000000,
        }),
      ],
    );

    await _pumpScreen(tester, socialRepository: socialRepository);
    verify(() => socialRepository.getMyReports()).called(1);

    await tester.fling(find.text('貼文'), const Offset(0, 300), 1000);
    await tester.pump();
    await tester.pump(const Duration(seconds: 1));

    // mocktail 的 verify() 會消耗已比對過的呼叫紀錄，故第二次 verify 只看
    // 「上次 verify 之後」新發生的呼叫數，此處應為下拉刷新觸發的 1 次。
    verify(() => socialRepository.getMyReports()).called(1);
    expect(tester.takeException(), isNull);
  });

  testWidgets('已有清單時下拉刷新失敗，保留清單並顯示錯誤 toast（不整頁蓋掉）', (tester) async {
    var callCount = 0;
    when(() => socialRepository.getMyReports()).thenAnswer((_) {
      callCount++;
      if (callCount == 1) {
        return Future.value([
          ReportRecord.fromJson(<String, dynamic>{
            'id': '1',
            'targetType': 'POST',
            'targetId': '111',
            'reasonCode': 'SPAM',
            'status': 'PENDING',
            'createTimeMs': 1721800000000,
          }),
        ]);
      }
      throw const ServerException('查詢檢舉紀錄失敗：伺服器忙碌中');
    });

    await _pumpScreen(tester, socialRepository: socialRepository);
    expect(find.text('貼文'), findsOneWidget);

    await tester.fling(find.text('貼文'), const Offset(0, 300), 1000);
    await tester.pump();
    await tester.pump(const Duration(seconds: 1));

    expect(
      find.text('貼文'),
      findsOneWidget,
      reason: '刷新失敗不該把已顯示的清單蓋掉，使用者原本看到的資料要保留',
    );
    expect(
      find.text('查詢檢舉紀錄失敗：伺服器忙碌中'),
      findsOneWidget,
      reason: '刷新失敗改用 toast 提示，不是整頁錯誤狀態',
    );
    expect(tester.takeException(), isNull);

    // showFloatingToast 用 Future.delayed(2s) 自動收起；測試結束前先讓它跑完，
    // 否則 flutter_test 會因「Timer 在 widget tree dispose 後仍 pending」判失敗。
    await tester.pump(const Duration(seconds: 3));
  });
}

Future<void> _pumpScreen(
  WidgetTester tester, {
  required SocialRepository socialRepository,
  bool settle = true,
}) async {
  tester.view.devicePixelRatio = 1;
  tester.view.physicalSize = const Size(390, 844);
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);

  await tester.pumpWidget(
    ProviderScope(
      overrides: [socialRepositoryProvider.overrideWithValue(socialRepository)],
      child: ScreenUtilInit(
        designSize: const Size(390, 844),
        enableScaleWH: () => false,
        enableScaleText: () => false,
        builder: (_, __) => MaterialApp(
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
  if (settle) {
    await tester.pumpAndSettle();
  }
}
