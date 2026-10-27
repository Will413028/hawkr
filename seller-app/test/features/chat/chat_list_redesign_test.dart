import 'package:hawkr_seller/features/chat/data/chat_model.dart';
import 'package:hawkr_seller/features/chat/data/chat_repository.dart';
import 'package:hawkr_seller/features/chat/presentation/chat_empty_state.dart';
import 'package:hawkr_seller/features/chat/presentation/chat_list_screen.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:mocktail/mocktail.dart';

class _MockChatRepository extends Mock implements ChatRepository {}

void main() {
  testWidgets(
    'empty inbox keeps search and customer chat hierarchy, AI entry hidden',
    (tester) async {
      await tester.binding.setSurfaceSize(const Size(320, 844));
      addTearDown(() => tester.binding.setSurfaceSize(null));

      final repository = _MockChatRepository();
      when(
        () => repository.getConversations(pageSize: 50),
      ).thenAnswer((_) async => const []);
      when(() => repository.getUnreadCount()).thenAnswer((_) async => 0);
      when(
        () => repository.onMessage,
      ).thenAnswer((_) => const Stream<ChatMessage>.empty());
      when(
        () => repository.onUnreadCountChange,
      ).thenAnswer((_) => const Stream<int>.empty());

      await tester.pumpWidget(
        ProviderScope(
          overrides: [chatRepositoryProvider.overrideWithValue(repository)],
          child: ScreenUtilInit(
            designSize: const Size(390, 844),
            minTextAdapt: true,
            builder: (_, __) => MaterialApp(
              builder: (context, child) => MediaQuery(
                data: MediaQuery.of(context).copyWith(
                  textScaler: const TextScaler.linear(1.3),
                  disableAnimations: true,
                ),
                child: child!,
              ),
              home: const ChatListScreen(),
            ),
          ),
        ),
      );
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 100));

      expect(find.text('訊息中心'), findsOneWidget);
      expect(find.text('與顧客保持聯繫'), findsOneWidget);
      expect(find.text('搜尋顧客或訊息'), findsOneWidget);
      expect(find.text('AI 營運助手'), findsNothing);
      expect(find.text('快速查詢訂單、出貨與金流'), findsNothing);
      expect(find.text('顧客對話'), findsOneWidget);
      expect(find.text('還沒有顧客對話'), findsOneWidget);
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets('search empty state offers a clear recovery action', (
    tester,
  ) async {
    var cleared = false;
    await tester.pumpWidget(
      ScreenUtilInit(
        designSize: const Size(390, 844),
        builder: (_, __) => MaterialApp(
          home: Scaffold(
            body: ChatEmptyState(
              isSearching: true,
              onClearSearch: () => cleared = true,
            ),
          ),
        ),
      ),
    );
    await tester.pump(const Duration(milliseconds: 100));

    expect(find.text('找不到相關對話'), findsOneWidget);
    expect(find.text('清除搜尋'), findsOneWidget);

    await tester.tap(find.text('清除搜尋'));
    await tester.pump();
    expect(cleared, isTrue);
    expect(tester.takeException(), isNull);
  });
}
