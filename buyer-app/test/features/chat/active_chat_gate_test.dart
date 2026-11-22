import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_buyer/features/chat/data/active_chat_gate.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

void main() {
  group('activeChatGateProvider', () {
    late ProviderContainer container;

    setUp(() {
      container = ProviderContainer();
    });
    tearDown(() => container.dispose());

    test('starts empty', () {
      final gate = container.read(activeChatGateProvider);
      expect(gate.isEmpty, isTrue);
      expect(gate.storeId, isNull);
      expect(gate.conversationId, isNull);
    });

    test('enter sets storeId and conversationId', () {
      container
          .read(activeChatGateProvider.notifier)
          .enter(storeId: '123456', conversationId: 'conv-abc');
      final gate = container.read(activeChatGateProvider);
      expect(gate.storeId, '123456');
      expect(gate.conversationId, 'conv-abc');
      expect(gate.isEmpty, isFalse);
    });

    test('enter without conversationId is valid', () {
      container.read(activeChatGateProvider.notifier).enter(storeId: '99');
      expect(container.read(activeChatGateProvider).storeId, '99');
      expect(container.read(activeChatGateProvider).conversationId, isNull);
    });

    test('clear resets to empty', () {
      container.read(activeChatGateProvider.notifier).enter(storeId: '1');
      container.read(activeChatGateProvider.notifier).clear();
      expect(container.read(activeChatGateProvider).isEmpty, isTrue);
    });

    test('two ActiveChat with same fields are equal', () {
      final notifier = container.read(activeChatGateProvider.notifier);
      notifier.enter(storeId: 'p', conversationId: 'c');
      final a = container.read(activeChatGateProvider);
      notifier.enter(storeId: 'p', conversationId: 'c');
      final b = container.read(activeChatGateProvider);
      expect(a, b);
      expect(a.hashCode, b.hashCode);
    });
  });
}
