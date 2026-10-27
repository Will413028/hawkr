import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:hawkr_buyer/features/chat/data/active_chat_gate.dart';

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
      expect(gate.peerId, '');
      expect(gate.conversationId, isNull);
    });

    test('enter sets peerId and conversationId', () {
      container
          .read(activeChatGateProvider.notifier)
          .enter(peerId: '123456', conversationId: 'conv-abc');
      final gate = container.read(activeChatGateProvider);
      expect(gate.peerId, '123456');
      expect(gate.conversationId, 'conv-abc');
      expect(gate.isEmpty, isFalse);
    });

    test('enter without conversationId is valid', () {
      container.read(activeChatGateProvider.notifier).enter(peerId: '99');
      expect(container.read(activeChatGateProvider).peerId, '99');
      expect(container.read(activeChatGateProvider).conversationId, isNull);
    });

    test('clear resets to empty', () {
      container.read(activeChatGateProvider.notifier).enter(peerId: '1');
      container.read(activeChatGateProvider.notifier).clear();
      expect(container.read(activeChatGateProvider).isEmpty, isTrue);
      expect(container.read(activeChatGateProvider).conversationId, isNull);
    });

    test('two ActiveChat with same fields are equal', () {
      container
          .read(activeChatGateProvider.notifier)
          .enter(peerId: 'p', conversationId: 'c');
      final a = container.read(activeChatGateProvider);
      container
          .read(activeChatGateProvider.notifier)
          .enter(peerId: 'p', conversationId: 'c');
      final b = container.read(activeChatGateProvider);
      expect(a, b);
      expect(a.hashCode, b.hashCode);
    });

    test('enter twice overwrites previous state', () {
      container
          .read(activeChatGateProvider.notifier)
          .enter(peerId: 'old', conversationId: 'conv1');
      container
          .read(activeChatGateProvider.notifier)
          .enter(peerId: 'new', conversationId: 'conv2');
      final gate = container.read(activeChatGateProvider);
      expect(gate.peerId, 'new');
      expect(gate.conversationId, 'conv2');
    });
  });
}
