import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/store_card.pb.dart';
import 'package:hawkr_api/gen/hawkr/chat/v1alpha1/chat.pb.dart' as pb;
import 'chat_fakes.dart';
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

  group('activeChatCovers', () {
    test('對話 ID 相符', () async {
      final repo = FakeChatRepository();
      expect(
        await activeChatCovers(
          () => const ActiveChat(conversationId: 'c1'),
          repo,
          'c1',
        ),
        isTrue,
      );
    });

    test('從店面進來還沒有對話 ID：同一家店的對話也算', () async {
      final repo = FakeChatRepository()..storeCards['c1'] = StoreCard(id: '7');
      const gate = ActiveChat(storeId: '7');
      expect(await activeChatCovers(() => gate, repo, 'c1'), isTrue);
      repo.storeCards['c2'] = StoreCard(id: '8');
      expect(await activeChatCovers(() => gate, repo, 'c2'), isFalse);
    });

    test('店家資料還沒有時讀一次對話列表再比對', () async {
      final repo = _FillingRepository();
      const gate = ActiveChat(storeId: '7');
      expect(await activeChatCovers(() => gate, repo, 'c1'), isTrue);
    });

    test('開著別的對話或什麼都沒開：不算', () async {
      final repo = FakeChatRepository()..storeCards['c1'] = StoreCard(id: '7');
      expect(
        await activeChatCovers(
          () => const ActiveChat(storeId: '7', conversationId: 'c9'),
          repo,
          'c1',
        ),
        isFalse,
      );
      expect(
        await activeChatCovers(() => const ActiveChat.empty(), repo, 'c1'),
        isFalse,
      );
    });
  });
}

class _FillingRepository extends FakeChatRepository {
  @override
  Future<List<pb.Conversation>> listAllConversations() async {
    storeCards['c1'] = StoreCard(id: '7');
    return [];
  }
}
