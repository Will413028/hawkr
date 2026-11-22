import 'package:hawkr_api/gen/hawkr/chat/v1alpha1/chat.pb.dart' as pb;
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:hawkr_seller/features/chat/data/chat_model.dart';
import 'package:hawkr_seller/features/chat/data/chat_repository.dart';
import 'package:hawkr_seller/features/chat/data/chat_session.dart';
import 'package:hawkr_seller/features/dashboard/data/dashboard_chat_unread_provider.dart';
import 'package:hawkr_seller/features/chat/presentation/chat_empty_state.dart';
import 'package:hawkr_seller/features/chat/presentation/chat_list_screen.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import 'support/chat_fakes.dart';

Future<void> _pump(WidgetTester tester, FakeChatRepository repo) async {
  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        chatRepositoryProvider.overrideWithValue(repo),
        chatSessionProvider.overrideWithValue(idleSession(repo)),
        dashboardChatUnreadProvider.overrideWith((ref) async => 2),
      ],
      child: ScreenUtilInit(
        designSize: const Size(390, 844),
        minTextAdapt: true,
        builder: (_, _) => MaterialApp(
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
}

void main() {
  testWidgets(
    'empty inbox keeps search and customer chat hierarchy, AI entry hidden',
    (tester) async {
      await tester.binding.setSurfaceSize(const Size(320, 844));
      addTearDown(() => tester.binding.setSurfaceSize(null));

      await _pump(tester, FakeChatRepository());

      expect(find.text('訊息中心'), findsOneWidget);
      expect(find.text('搜尋顧客或訊息'), findsOneWidget);
      expect(find.text('AI 營運助手'), findsNothing);
      expect(find.text('快速查詢訂單、出貨與金流'), findsNothing);
      expect(find.text('顧客對話'), findsOneWidget);
      expect(find.text('還沒有顧客對話'), findsOneWidget);
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets('a read failure shows the error panel, not the empty state', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(320, 844));
    addTearDown(() => tester.binding.setSurfaceSize(null));
    final repo = FakeChatRepository()
      ..listConversationsError = ConnectException(Code.unavailable, 'down');
    await _pump(tester, repo);

    expect(find.text('目前無法載入對話'), findsOneWidget);
    expect(find.text('還沒有顧客對話'), findsNothing);
    expect(tester.takeException(), isNull);
  });

  testWidgets('rows show the buyer name and a preview from the catalog', (
    tester,
  ) async {
    await tester.binding.setSurfaceSize(const Size(390, 844));
    addTearDown(() => tester.binding.setSurfaceSize(null));
    final repo = FakeChatRepository()
      ..conversations = [
        ChatConversation.fromProto(
          protoConversation('c1', 'b1', name: '小明', unread: 2)
            ..preview = pb.Preview(
              kind: pb.MessageKind.MESSAGE_KIND_IMAGE,
              sender: pb.ChatParty.CHAT_PARTY_BUYER,
            ),
        ),
      ];
    await _pump(tester, repo);

    expect(find.text('小明'), findsOneWidget);
    expect(find.text('傳送了一張圖片'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('search empty state offers a clear recovery action', (
    tester,
  ) async {
    var cleared = false;
    await tester.pumpWidget(
      ScreenUtilInit(
        designSize: const Size(390, 844),
        builder: (_, _) => MaterialApp(
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
