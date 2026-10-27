import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'dart:async';
import 'dart:io';

import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/features/ai_assistant/data/ai_message_model.dart';
import 'package:hawkr_buyer/features/ai_assistant/data/ai_assistant_repository.dart';
import 'package:hawkr_buyer/features/ai_assistant/presentation/ai_assistant_screen.dart';
import 'package:hawkr_buyer/features/ai_assistant/presentation/widgets/ai_assistant_avatar.dart';
import 'package:hawkr_buyer/shared/widgets/chat_conversation_chrome.dart';
import 'package:hawkr_buyer/shared/widgets/chat_input_bar.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

void main() {
  for (final width in <double>[320, 390]) {
    testWidgets('AI 客服在 ${width.toInt()}px 沿用外部聊天室介面且不溢位', (tester) async {
      tester.view.devicePixelRatio = 1;
      tester.view.physicalSize = Size(width, 844);
      addTearDown(tester.view.resetDevicePixelRatio);
      addTearDown(tester.view.resetPhysicalSize);
      final repository = _FakeAiAssistantRepository();
      addTearDown(repository.dispose);

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            aiAssistantRepositoryProvider.overrideWithValue(repository),
          ],
          child: ScreenUtilInit(
            designSize: const Size(390, 844),
            enableScaleWH: () => false,
            enableScaleText: () => false,
            builder: (_, __) => MaterialApp(
              builder: (context, child) => MediaQuery(
                data: MediaQuery.of(
                  context,
                ).copyWith(textScaler: const TextScaler.linear(1.4)),
                child: child!,
              ),
              home: const AiAssistantScreen(),
            ),
          ),
        ),
      );
      await tester.pump();
      await tester.pumpAndSettle();

      final scaffold = tester.widget<Scaffold>(
        find.byKey(const ValueKey('ai-assistant-chat-screen')),
      );
      expect(scaffold.backgroundColor, AppColors.bgPrimary);
      expect(find.byType(ChatConversationHeaderContent), findsOneWidget);
      expect(find.byType(ChatConversationDotPattern), findsOneWidget);
      expect(find.byType(ChatInputBar), findsOneWidget);
      expect(find.byType(AiAssistantAvatar), findsNWidgets(2));
      expect(
        find.byKey(const ValueKey('ai-assistant-avatar-icon')),
        findsNWidgets(2),
      );
      expect(find.text('買家小幫手 ✨'), findsOneWidget);
      expect(find.text('AI 服務中'), findsOneWidget);
      expect(find.textContaining('生鮮買家小幫手'), findsOneWidget);
      expect(find.bySemanticsLabel('新增附件'), findsOneWidget);
      expect(find.bySemanticsLabel('發送訊息'), findsOneWidget);
      expect(tester.takeException(), isNull);
    });
  }

  testWidgets('AI 頭像只在回覆串流進行中標示為回覆中', (tester) async {
    tester.view.devicePixelRatio = 1;
    tester.view.physicalSize = const Size(390, 844);
    addTearDown(tester.view.resetDevicePixelRatio);
    addTearDown(tester.view.resetPhysicalSize);
    final repository = _FakeAiAssistantRepository();
    addTearDown(repository.dispose);

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          aiAssistantRepositoryProvider.overrideWithValue(repository),
        ],
        child: ScreenUtilInit(
          designSize: const Size(390, 844),
          enableScaleWH: () => false,
          enableScaleText: () => false,
          builder: (_, __) => const MaterialApp(home: AiAssistantScreen()),
        ),
      ),
    );
    await tester.pump();
    await tester.pumpAndSettle();

    final headerAvatar = find.byKey(
      const ValueKey('ai-assistant-header-avatar'),
    );
    expect(tester.widget<AiAssistantAvatar>(headerAvatar).isReplying, isFalse);

    await tester.enterText(find.byType(TextField), '幫我查詢訂單');
    await tester.pump();
    await tester.tap(find.bySemanticsLabel('發送訊息'));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 120));

    expect(tester.widget<AiAssistantAvatar>(headerAvatar).isReplying, isTrue);
    final replyingAvatars = tester
        .widgetList<AiAssistantAvatar>(
          find.byType(AiAssistantAvatar, skipOffstage: false),
        )
        .where((avatar) => avatar.isReplying);
    expect(replyingAvatars, isNotEmpty);
    expect(find.bySemanticsLabel('AI 客服正在回覆'), findsWidgets);

    repository.finishReply();
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 500));

    expect(tester.widget<AiAssistantAvatar>(headerAvatar).isReplying, isFalse);
    expect(find.bySemanticsLabel('AI 客服正在回覆'), findsNothing);
    final avatarsAfterReply = tester.widgetList<AiAssistantAvatar>(
      find.byType(AiAssistantAvatar, skipOffstage: false),
    );
    expect(avatarsAfterReply.every((avatar) => !avatar.isReplying), isTrue);
    expect(tester.takeException(), isNull);
  });

  testWidgets('AI 頭像畫靜態 Material 圖示且保持框的尺寸', (tester) async {
    await tester.pumpWidget(
      const MaterialApp(
        home: Scaffold(
          body: Center(child: AiAssistantAvatar(size: 40, isReplying: true)),
        ),
      ),
    );

    final icon = tester.widget<Icon>(
      find.byKey(const ValueKey('ai-assistant-avatar-icon')),
    );
    expect(icon.icon, AppIcons.smartToyOutlined);
    expect(icon.icon!.fontFamily, 'MaterialIcons');
    expect(tester.getSize(find.byType(AiAssistantAvatar)), const Size(40, 40));
    expect(tester.takeException(), isNull);
  });

  testWidgets('AI快速送出和IME不重複請求，離頁後串流結束不寫入已卸載狀態', (tester) async {
    final repository = _FakeAiAssistantRepository();
    addTearDown(repository.dispose);
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          aiAssistantRepositoryProvider.overrideWithValue(repository),
        ],
        child: ScreenUtilInit(
          designSize: const Size(390, 844),
          enableScaleWH: () => false,
          enableScaleText: () => false,
          builder: (_, __) => const MaterialApp(home: AiAssistantScreen()),
        ),
      ),
    );
    await tester.pumpAndSettle();
    await tester.enterText(find.byType(TextField), '離頁仍在傳送');
    await tester.pump();
    await tester.tap(find.bySemanticsLabel('發送訊息'));
    await tester.testTextInput.receiveAction(TextInputAction.send);
    await tester.pump();
    expect(repository.requests, 1);
    await tester.pumpWidget(const SizedBox.shrink());
    repository.finishReply();
    await tester.pumpAndSettle();
    expect(tester.takeException(), isNull);
  });

  test('AI 與外部聊天室必須共用對話 chrome，禁止恢復分岔元件', () {
    final aiSource = File(
      'lib/features/ai_assistant/presentation/ai_assistant_screen.dart',
    ).readAsStringSync();
    final externalChatSource = File(
      'lib/features/chat/presentation/chat_thread_screen.dart',
    ).readAsStringSync();

    for (final sharedWidget in <String>[
      'ChatConversationHeaderContent(',
      'ChatConversationDotPattern()',
    ]) {
      expect(aiSource, contains(sharedWidget));
      expect(externalChatSource, contains(sharedWidget));
    }
    expect(aiSource, contains('ChatInputBar('));
    expect(aiSource, contains('NotificationCountBadge('));
    expect(aiSource, isNot(contains('_ChatBgPainter')));
    expect(aiSource, isNot(contains('_PlusButton')));
    expect(aiSource, isNot(contains('_SendButton')));
    expect(aiSource, isNot(contains('AppColors.surfacePanel')));
    expect(aiSource, isNot(contains('ic_ai_avatar.png')));
  });
}

class _FakeAiAssistantRepository extends AiAssistantRepository {
  _FakeAiAssistantRepository._(this.dio) : super(dio);

  factory _FakeAiAssistantRepository() => _FakeAiAssistantRepository._(Dio());

  final Dio dio;
  final Completer<void> _replyDone = Completer<void>();
  int requests = 0;

  @override
  Future<({String sessionId, List<AiMessage> messages})>
  loadLatestSession() async {
    return (sessionId: '', messages: <AiMessage>[]);
  }

  @override
  Stream<AiSseEvent> streamChat({
    BuyerAssistantEndpoint endpoint = BuyerAssistantEndpoint.chat,
    String? message,
    String? sessionId,
    String? imageUrl,
  }) async* {
    requests++;
    await _replyDone.future;
    yield const AiSseEvent(type: 'text', data: '已找到您的訂單。');
    yield const AiSseEvent(type: 'done', data: '{}', terminal: true);
  }

  void finishReply() {
    if (!_replyDone.isCompleted) _replyDone.complete();
  }

  Future<void> dispose() async {
    if (!_replyDone.isCompleted) _replyDone.complete();
    dio.close(force: true);
  }
}
