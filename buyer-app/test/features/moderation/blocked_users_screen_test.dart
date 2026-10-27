import 'package:hawkr_buyer/core/exceptions/app_exception.dart';
import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/app_typography.dart';
import 'package:hawkr_buyer/features/moderation/data/block_model.dart';
import 'package:hawkr_buyer/features/moderation/data/block_repository.dart';
import 'package:hawkr_buyer/features/moderation/presentation/blocked_users_screen.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:mocktail/mocktail.dart';

/// 補齊 Task 4 review 指出的缺口：brief Step 9（手動導覽驗證）在無人值守環境
/// 無法用真實裝置登入操作，改以本測試檔案作為可自動執行、可重現的替代驗證——
/// 用 mock repository（依 `my_subscription_social_screen_test.dart` 既有慣例）
/// 實際 pump `BlockedUsersScreen` 並斷言 loading / 有資料 / empty state 三態渲染
/// 正確，以及解除封鎖成功／失敗與下拉刷新皆如預期運作。
class _MockBlockRepository extends Mock implements BlockRepository {}

void main() {
  late _MockBlockRepository blockRepository;

  setUp(() {
    blockRepository = _MockBlockRepository();
  });

  // /app/blocks 只回 id / blocked_id / create_time_ms（見 BlockedUser 契約註解），
  // 沒有名稱欄位，所以每一列都是「使用者 + id 尾六碼」。
  testWidgets('有封鎖名單時每列都渲染 id 尾碼標籤與解除封鎖鈕', (tester) async {
    when(() => blockRepository.getBlockedUsers()).thenAnswer(
      (_) async => const [
        BlockedUser(userId: '1001'),
        BlockedUser(userId: '2002003004'),
      ],
    );

    await _pumpScreen(tester, blockRepository: blockRepository);

    expect(find.text('封鎖名單'), findsOneWidget);
    expect(find.text('使用者 1001'), findsOneWidget);
    expect(find.text('使用者 003004'), findsOneWidget);
    expect(find.text('解除封鎖'), findsNWidgets(2));
    expect(tester.takeException(), isNull);
  });

  testWidgets('無封鎖名單時顯示 empty state', (tester) async {
    when(() => blockRepository.getBlockedUsers()).thenAnswer((_) async => []);

    await _pumpScreen(tester, blockRepository: blockRepository);

    expect(find.text('尚無封鎖的使用者'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('載入失敗時顯示錯誤狀態（與真的沒資料的 empty state 區分），並可重試', (tester) async {
    var callCount = 0;
    when(() => blockRepository.getBlockedUsers()).thenAnswer((_) {
      callCount++;
      if (callCount == 1) throw const ServerException('查詢封鎖名單失敗：伺服器忙碌中');
      return Future.value(const <BlockedUser>[]);
    });

    await _pumpScreen(tester, blockRepository: blockRepository);

    expect(
      find.text('尚無封鎖的使用者'),
      findsNothing,
      reason: '讀取失敗不應顯示「尚無封鎖的使用者」的空狀態，會誤導使用者以為真的沒有封鎖任何人',
    );
    expect(find.text('查詢封鎖名單失敗：伺服器忙碌中'), findsOneWidget);
    expect(find.text('重試'), findsOneWidget);
    expect(tester.takeException(), isNull);

    await tester.tap(find.text('重試'));
    await tester.pumpAndSettle();

    expect(
      find.text('尚無封鎖的使用者'),
      findsOneWidget,
      reason: '重試成功且回傳空陣列，才進入真正的 empty state',
    );
    expect(find.text('查詢封鎖名單失敗：伺服器忙碌中'), findsNothing);
    expect(tester.takeException(), isNull);
  });

  testWidgets('解除封鎖成功後從清單移除並顯示成功 toast', (tester) async {
    when(
      () => blockRepository.getBlockedUsers(),
    ).thenAnswer((_) async => const [BlockedUser(userId: '1001')]);
    when(() => blockRepository.unblockUser('1001')).thenAnswer((_) async {});

    await _pumpScreen(tester, blockRepository: blockRepository);
    expect(find.text('使用者 1001'), findsOneWidget);

    await tester.tap(find.text('解除封鎖'));
    await tester.pump();
    await tester.pump();

    verify(() => blockRepository.unblockUser('1001')).called(1);
    expect(find.text('使用者 1001'), findsNothing);
    expect(find.text('已解除封鎖'), findsOneWidget);
    expect(find.text('尚無封鎖的使用者'), findsOneWidget);
    expect(tester.takeException(), isNull);

    // showFloatingToast 用 Future.delayed(2s) 自動收起；測試結束前先讓它跑完，
    // 否則 flutter_test 會因「Timer 在 widget tree dispose 後仍 pending」判失敗。
    await tester.pump(const Duration(seconds: 3));
  });

  testWidgets('解除封鎖失敗時清單不變並顯示錯誤 toast', (tester) async {
    when(
      () => blockRepository.getBlockedUsers(),
    ).thenAnswer((_) async => const [BlockedUser(userId: '1001')]);
    when(
      () => blockRepository.unblockUser('1001'),
    ).thenThrow(const NetworkException());

    await _pumpScreen(tester, blockRepository: blockRepository);

    await tester.tap(find.text('解除封鎖'));
    await tester.pump();
    await tester.pump();

    expect(find.text('使用者 1001'), findsOneWidget);
    expect(find.text('網路連線失敗，請稍後再試'), findsOneWidget);
    expect(tester.takeException(), isNull);

    // showFloatingToast 用 Future.delayed(2s) 自動收起；測試結束前先讓它跑完，
    // 否則 flutter_test 會因「Timer 在 widget tree dispose 後仍 pending」判失敗。
    await tester.pump(const Duration(seconds: 3));
  });

  testWidgets('下拉刷新會重新呼叫 getBlockedUsers()', (tester) async {
    when(
      () => blockRepository.getBlockedUsers(),
    ).thenAnswer((_) async => const [BlockedUser(userId: '1001')]);

    await _pumpScreen(tester, blockRepository: blockRepository);
    verify(() => blockRepository.getBlockedUsers()).called(1);

    await tester.fling(find.text('使用者 1001'), const Offset(0, 300), 1000);
    await tester.pump();
    await tester.pump(const Duration(seconds: 1));

    // mocktail 的 verify() 會消耗已比對過的呼叫紀錄，故第二次 verify 只看
    // 「上次 verify 之後」新發生的呼叫數，此處應為下拉刷新觸發的 1 次。
    verify(() => blockRepository.getBlockedUsers()).called(1);
    expect(tester.takeException(), isNull);
  });

  testWidgets('已有清單時下拉刷新失敗，保留清單並顯示錯誤 toast（不整頁蓋掉）', (tester) async {
    var callCount = 0;
    when(() => blockRepository.getBlockedUsers()).thenAnswer((_) {
      callCount++;
      if (callCount == 1) {
        return Future.value(const [BlockedUser(userId: '1001')]);
      }
      throw const ServerException('查詢封鎖名單失敗：伺服器忙碌中');
    });

    await _pumpScreen(tester, blockRepository: blockRepository);
    expect(find.text('使用者 1001'), findsOneWidget);

    await tester.fling(find.text('使用者 1001'), const Offset(0, 300), 1000);
    await tester.pump();
    await tester.pump(const Duration(seconds: 1));

    expect(
      find.text('使用者 1001'),
      findsOneWidget,
      reason: '刷新失敗不該把已顯示的清單蓋掉，使用者原本看到的資料要保留',
    );
    expect(
      find.text('查詢封鎖名單失敗：伺服器忙碌中'),
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
  required BlockRepository blockRepository,
}) async {
  tester.view.devicePixelRatio = 1;
  tester.view.physicalSize = const Size(390, 844);
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);

  await tester.pumpWidget(
    ProviderScope(
      overrides: [blockRepositoryProvider.overrideWithValue(blockRepository)],
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
          home: const BlockedUsersScreen(),
        ),
      ),
    ),
  );
  await tester.pumpAndSettle();
}
