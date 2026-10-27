import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/features/report/data/block_model.dart';
import 'package:hawkr_seller/features/report/data/block_repository.dart';
import 'package:hawkr_seller/features/report/presentation/block_list_screen.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:mocktail/mocktail.dart';

class _MockBlockRepository extends Mock implements BlockRepository {}

/// `GET /app/blocks` 對 dev 的實打回應（2026-08-06，賣家 2685914682 封鎖買家
/// 2623900520 後查詢，逐字貼上；驗完已解封）。
const Map<String, dynamic> _liveBlock = {
  'id': '7490888018291833798',
  'blocked_id': '2623900520',
  'create_time_ms': 1785966877535,
};

void main() {
  late _MockBlockRepository repository;

  setUp(() {
    repository = _MockBlockRepository();
    when(
      () => repository.listBlockedUsers(page: 1, pageSize: 20),
    ).thenAnswer((_) async => [BlockedUser.fromRaw(_liveBlock)]);
    when(() => repository.unblockUser(any())).thenAnswer((_) async {});
  });

  testWidgets('列表標籤取自 blocked_id 尾碼', (tester) async {
    await _pumpScreen(tester, repository);

    expect(find.text('使用者 900520'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  // 這一條釘住整條 id 傳遞：舊版 fromRaw 的候選鏈沒有 blocked_id 卻有 id，
  // 解除封鎖會打成封鎖紀錄自己的 TSID，後端 delete-by-pair 找不到列而變 no-op。
  testWidgets('解除封鎖送出的是 blocked_id，不是封鎖紀錄的 TSID', (tester) async {
    await _pumpScreen(tester, repository);

    await tester.tap(find.text('解除封鎖'));
    await tester.pumpAndSettle();

    // 確認彈窗的標題帶的是同一個 id 尾碼。
    expect(find.textContaining('900520'), findsWidgets);

    // 彈窗的確認鍵與列表按鈕同字樣，取最後一個（彈窗在上層）。
    await tester.tap(find.text('解除封鎖').last);
    await tester.pumpAndSettle();

    verify(() => repository.unblockUser('2623900520')).called(1);
    verifyNever(() => repository.unblockUser('7490888018291833798'));
  });
}

Future<void> _pumpScreen(
  WidgetTester tester,
  BlockRepository repository,
) async {
  tester.view.devicePixelRatio = 1;
  tester.view.physicalSize = const Size(390, 844);
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);

  await tester.pumpWidget(
    ProviderScope(
      overrides: [blockRepositoryProvider.overrideWithValue(repository)],
      child: ScreenUtilInit(
        designSize: const Size(390, 844),
        enableScaleWH: () => false,
        enableScaleText: () => false,
        builder: (_, __) => MaterialApp(
          theme: ThemeData(
            useMaterial3: true,
            colorScheme: ColorScheme.fromSeed(seedColor: AppColors.primary),
          ),
          home: const BlockListScreen(),
        ),
      ),
    ),
  );
  await tester.pumpAndSettle();
}
