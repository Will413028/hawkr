import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:hawkr_seller/features/chat/data/chat_conversations_notifier.dart';
import 'package:hawkr_seller/features/chat/data/chat_model.dart';
import 'package:hawkr_seller/features/chat/data/chat_repository.dart';
import 'package:hawkr_seller/features/chat/data/chat_session.dart';

import 'support/chat_fakes.dart';

ProviderContainer _container(FakeChatRepository repo) => ProviderContainer(
  overrides: [
    chatRepositoryProvider.overrideWithValue(repo),
    chatSessionProvider.overrideWithValue(idleSession(repo)),
  ],
);

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test('no conversations is an empty list, not an error', () async {
    final c = _container(FakeChatRepository());
    addTearDown(c.dispose);
    final s = await c.read(chatConversationsProvider.future);
    expect(s.items, isEmpty);
    expect(c.read(chatConversationsProvider).hasError, isFalse);
  });

  test('a read failure is an error state, distinct from empty', () async {
    final repo = FakeChatRepository()
      ..listConversationsError = ConnectException(Code.unavailable, 'down');
    final c = _container(repo);
    addTearDown(c.dispose);
    c.listen(chatConversationsProvider, (_, _) {});
    await expectLater(
      c.read(chatConversationsProvider.future),
      throwsA(isA<ConnectException>()),
    );
    expect(c.read(chatConversationsProvider).hasError, isTrue);
  });

  test('a failed background refresh keeps the rows already shown', () async {
    final repo = FakeChatRepository()
      ..conversations = [
        ChatConversation.fromProto(protoConversation('c1', 'b1', name: 'A')),
      ];
    final c = _container(repo);
    addTearDown(c.dispose);
    c.listen(chatConversationsProvider, (_, _) {});
    await c.read(chatConversationsProvider.future);
    repo.listConversationsError = ConnectException(Code.unavailable, 'down');
    await c.read(chatConversationsProvider.notifier).refresh();
    expect(c.read(chatConversationsProvider).value!.items, hasLength(1));
  });
}
