import 'dart:async';

import 'package:hawkr_seller/features/ai_assistant/data/ai_assistant_repository.dart';
import 'package:hawkr_seller/features/ai_assistant/data/ai_message_model.dart';
import 'package:hawkr_seller/features/ai_assistant/presentation/ai_assistant_controller.dart';
import 'package:hawkr_seller/features/product/data/product_listing_chat_repository.dart';
import 'package:hawkr_seller/features/product/data/product_listing_model.dart';
import 'package:hawkr_seller/features/product/presentation/product_listing_chat/product_listing_chat_controller.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

class _AssistantRepo extends AiAssistantRepository {
  _AssistantRepo() : super(Dio());
  final events = StreamController<AiSseEvent>();
  final history = Completer<({String sessionId, List<AiMessage> messages})>();
  var calls = 0;
  @override
  Future<({String sessionId, List<AiMessage> messages})> loadLatestSession() =>
      history.future;
  @override
  Stream<AiSseEvent> streamChat({
    SellerAssistantEndpoint endpoint = SellerAssistantEndpoint.chat,
    String? message,
    String? sessionId,
    String? imageUrl,
  }) {
    calls++;
    return events.stream;
  }
}

class _ListingRepo implements ProductListingChatRepository {
  final events = StreamController<SseEvent>();
  var calls = 0;
  @override
  Stream<SseEvent> stream({
    required String message,
    String? sessionId,
    String? imageUrl,
  }) {
    calls++;
    return events.stream;
  }
}

void main() {
  test('late history cannot replace a newly started conversation', () async {
    final repo = _AssistantRepo();
    final container = ProviderContainer(
      overrides: [aiAssistantRepositoryProvider.overrideWithValue(repo)],
    );
    addTearDown(container.dispose);
    final listener = container.listen(aiAssistantControllerProvider, (_, _) {});
    addTearDown(listener.close);
    final notifier = container.read(aiAssistantControllerProvider.notifier);
    final history = notifier.loadLatestSession();
    final send = notifier.sendMessage('新訊息');
    repo.history.complete((
      sessionId: 'old-session',
      messages: [
        AiMessage(role: 'assistant', content: '舊紀錄', timestamp: DateTime(2026)),
      ],
    ));
    await history;
    expect(container.read(aiAssistantControllerProvider).first.content, '新訊息');
    expect(notifier.sessionId, isNull);
    repo.events.add(const AiSseEvent(type: 'text', data: '新回覆'));
    repo.events.add(
      const AiSseEvent(
        type: 'done',
        data: '{"sessionId":"new-session"}',
        terminal: true,
      ),
    );
    await send;
    expect(container.read(aiAssistantControllerProvider).last.content, '新回覆');
    expect(notifier.sessionId, 'new-session');
    await repo.events.close();
  });

  test(
    'assistant rejects duplicate send and cancels active stream on dispose',
    () async {
      final repo = _AssistantRepo();
      final container = ProviderContainer(
        overrides: [aiAssistantRepositoryProvider.overrideWithValue(repo)],
      );
      final listener = container.listen(
        aiAssistantControllerProvider,
        (_, _) {},
      );
      final notifier = container.read(aiAssistantControllerProvider.notifier);
      final pending = notifier.sendMessage('商品詢問');
      await notifier.sendMessage('第二次點擊');
      expect(repo.calls, 1);
      expect(
        container
            .read(aiAssistantControllerProvider)
            .where((m) => m.role == 'user'),
        hasLength(1),
      );
      listener.close();
      container.dispose();
      await pending;
      expect(repo.events.hasListener, isFalse);
      await repo.events.close();
    },
  );
  test('assistant ignores late history after disposal', () async {
    final repo = _AssistantRepo();
    final container = ProviderContainer(
      overrides: [aiAssistantRepositoryProvider.overrideWithValue(repo)],
    );
    final pending = container
        .read(aiAssistantControllerProvider.notifier)
        .loadLatestSession();
    container.dispose();
    repo.history.complete((
      sessionId: 'old',
      messages: [
        AiMessage(role: 'assistant', content: '舊紀錄', timestamp: DateTime(2026)),
      ],
    ));
    await pending;
    unawaited(repo.events.close());
  });
  test(
    'listing rejects duplicate send and cancels active stream on dispose',
    () async {
      final repo = _ListingRepo();
      final container = ProviderContainer(
        overrides: [
          productListingChatRepositoryProvider.overrideWithValue(repo),
        ],
      );
      final listener = container.listen(
        productListingChatControllerProvider,
        (_, _) {},
      );
      final notifier = container.read(
        productListingChatControllerProvider.notifier,
      );
      final pending = notifier.sendMessage('上架高麗菜');
      await notifier.sendMessage('第二次點擊');
      expect(repo.calls, 1);
      expect(
        container
            .read(productListingChatControllerProvider)
            .messages
            .where((m) => m.role == 'user'),
        hasLength(1),
      );
      listener.close();
      container.dispose();
      await pending;
      expect(repo.events.hasListener, isFalse);
      await repo.events.close();
    },
  );
}
