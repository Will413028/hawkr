import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_seller/features/chat/data/active_chat_gate.dart';

void main() {
  group('ActiveChatGate', () {
    test('initial state has null peerId and conversationId', () {
      final container = ProviderContainer();
      addTearDown(container.dispose);
      final state = container.read(activeChatGateProvider);
      expect(state.peerId, isNull);
      expect(state.conversationId, isNull);
    });

    test('enter sets peerId and conversationId', () {
      final container = ProviderContainer();
      addTearDown(container.dispose);
      container
          .read(activeChatGateProvider.notifier)
          .enter(peerId: 'buyer-99', conversationId: 'conv-abc');
      final state = container.read(activeChatGateProvider);
      expect(state.peerId, 'buyer-99');
      expect(state.conversationId, 'conv-abc');
    });

    test('enter with null conversationId preserves peerId only', () {
      final container = ProviderContainer();
      addTearDown(container.dispose);
      container.read(activeChatGateProvider.notifier).enter(peerId: 'p1');
      final state = container.read(activeChatGateProvider);
      expect(state.peerId, 'p1');
      expect(state.conversationId, isNull);
    });

    test('clear resets state to empty', () {
      final container = ProviderContainer();
      addTearDown(container.dispose);
      container
          .read(activeChatGateProvider.notifier)
          .enter(peerId: 'p1', conversationId: 'c1');
      container.read(activeChatGateProvider.notifier).clear();
      final state = container.read(activeChatGateProvider);
      expect(state.peerId, isNull);
      expect(state.conversationId, isNull);
    });

    test(
      'enter with conversationId only (null peerId) sets conversationId',
      () {
        final container = ProviderContainer();
        addTearDown(container.dispose);
        container
            .read(activeChatGateProvider.notifier)
            .enter(conversationId: 'conv-only');
        final state = container.read(activeChatGateProvider);
        expect(state.peerId, isNull);
        expect(state.conversationId, 'conv-only');
      },
    );

    test('updateConversationId patches only conversationId', () {
      final container = ProviderContainer();
      addTearDown(container.dispose);
      container.read(activeChatGateProvider.notifier).enter(peerId: 'p1');
      container
          .read(activeChatGateProvider.notifier)
          .updateConversationId('conv-late');
      final state = container.read(activeChatGateProvider);
      expect(state.peerId, 'p1');
      expect(state.conversationId, 'conv-late');
    });
  });
}
