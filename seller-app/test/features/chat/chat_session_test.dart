import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/chat/v1alpha1/chat.pb.dart' as pb;
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:hawkr_seller/features/chat/data/chat_repository.dart';
import 'package:hawkr_seller/features/chat/data/chat_session.dart';
import 'package:hawkr_seller/features/chat/data/chat_stream_client.dart';
import 'package:hawkr_seller/features/dashboard/data/dashboard_chat_unread_provider.dart';
import 'package:hawkr_seller/features/seller/data/current_store.dart';
import 'package:hawkr_ui_text/hawkr_ui_text.dart';

import 'support/chat_fakes.dart';

class _StoreId extends Notifier<String?> {
  @override
  String? build() => null;
  void set(String? v) => state = v;
}

final _storeId = NotifierProvider<_StoreId, String?>(_StoreId.new);

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test('a store appearing restarts a stream stopped for noStore', () async {
    final repo = FakeChatRepository();
    final streams = <StreamController<pb.ChatEvent>>[];
    final session = ChatSession(
      repo,
      () => UiLocale.zhTW,
      client: ChatStreamClient(
        open: (_) {
          final c = StreamController<pb.ChatEvent>();
          streams.add(c);
          return c.stream;
        },
      ),
    );
    final c = ProviderContainer(
      overrides: [
        chatRepositoryProvider.overrideWithValue(repo),
        chatSessionProvider.overrideWithValue(session),
        sellerStoreIdProvider.overrideWith((ref) => ref.watch(_storeId)),
      ],
    );
    addTearDown(c.dispose);
    addTearDown(session.dispose);
    // 建立 provider（它在建立時掛上店家變化的監聽）。
    c.listen(sellerStoreIdProvider, (_, _) {});
    c.read(chatSessionProvider);
    session.start();
    await Future<void>.delayed(Duration.zero);
    streams.last.addError(ConnectException(Code.notFound, 'no store'));
    await Future<void>.delayed(Duration.zero);
    expect(
      (session.status as ChatStreamStopped).reason,
      ChatStreamStopReason.noStore,
    );
    expect(streams, hasLength(1));

    session.onStoreChanged();
    await Future<void>.delayed(Duration.zero);
    expect(streams, hasLength(2));
    expect(session.status, isA<ChatStreamConnecting>());
  });

  test(
    'onStoreChanged leaves other stop reasons and a logged-out session',
    () async {
      final repo = FakeChatRepository();
      var opens = 0;
      final session = ChatSession(
        repo,
        () => UiLocale.zhTW,
        client: ChatStreamClient(
          open: (_) {
            opens++;
            return Stream<pb.ChatEvent>.error(
              ConnectException(Code.unauthenticated, 'x'),
            );
          },
        ),
      );
      addTearDown(session.dispose);
      session.onStoreChanged();
      expect(opens, 0);
      session.start();
      await Future<void>.delayed(Duration.zero);
      expect(opens, 1);
      session.onStoreChanged();
      await Future<void>.delayed(Duration.zero);
      expect(opens, 1);
    },
  );

  test(
    'the app wiring restarts the session when the store id appears',
    () async {
      var opens = 0;
      final repo = FakeChatRepository()
        ..onStream = (_) {
          opens++;
          return Stream<pb.ChatEvent>.error(
            ConnectException(Code.notFound, 'x'),
          );
        };
      final c = ProviderContainer(
        overrides: [
          chatRepositoryProvider.overrideWithValue(repo),
          sellerStoreIdProvider.overrideWith((ref) => ref.watch(_storeId)),
        ],
      );
      addTearDown(c.dispose);
      final session = c.read(chatSessionProvider);
      c.listen(
        sellerStoreIdProvider,
        (prev, next) => onChatStoreIdChanged(session, prev, next),
      );
      session.start();
      await Future<void>.delayed(Duration.zero);
      expect(session.client.isRunning, isFalse);
      expect(opens, 1);
      c.read(_storeId.notifier).set('s1');
      await Future<void>.delayed(Duration.zero);
      expect(opens, 2);
    },
  );

  test(
    'dashboard unread re-reads when the stream reports a new total',
    () async {
      final repo = FakeChatRepository()..unread = 1;
      final controller = StreamController<pb.ChatEvent>();
      final session = ChatSession(
        repo,
        () => UiLocale.zhTW,
        client: ChatStreamClient(open: (_) => controller.stream),
      );
      final c = ProviderContainer(
        overrides: [
          chatRepositoryProvider.overrideWithValue(repo),
          chatSessionProvider.overrideWithValue(session),
        ],
      );
      addTearDown(c.dispose);
      addTearDown(session.dispose);
      c.listen(dashboardChatUnreadProvider, (_, _) {});
      expect(await c.read(dashboardChatUnreadProvider.future), 1);
      session.start();
      await Future<void>.delayed(Duration.zero);
      repo.unread = 4;
      controller.add(
        pb.ChatEvent(unread: pb.UnreadChanged(unreadCount: i64(4))),
      );
      await Future<void>.delayed(Duration.zero);
      expect(await c.read(dashboardChatUnreadProvider.future), 4);
    },
  );
}
