import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/chat/v1alpha1/chat.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/chat/v1alpha1/chat.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/chat/v1alpha1/chat.pb.dart' as pb;
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_seller/features/chat/data/chat_repository.dart';

import 'support/chat_fakes.dart';

ChatRepository _repo(FakeTransportBuilder b) =>
    ChatRepository(ChatAppServiceClient(b.build()));

void main() {
  test('markRead sends the conversation id and upto_seq', () async {
    pb.MarkConversationReadRequest? seen;
    final repo = _repo(
      FakeTransportBuilder().unary(ChatAppService.markConversationRead, (
        req,
        _,
      ) async {
        seen = req;
        return pb.MarkConversationReadResponse();
      }),
    );
    await repo.markRead('c1', 42);
    expect(seen!.conversationId, 'c1');
    expect(seen!.uptoSeq, i64(42));
  });

  test('send carries the request id and text content', () async {
    pb.SendMessageRequest? seen;
    final repo = _repo(
      FakeTransportBuilder().unary(ChatAppService.sendMessage, (req, _) async {
        seen = req;
        return pb.SendMessageResponse(message: protoMessage(9, store: true));
      }),
    );
    final m = await repo.send('c1', requestId: 'rid', text: 'hi');
    expect(seen!.requestId, 'rid');
    expect(seen!.content.text, 'hi');
    expect(m.seq, 9);
    expect(m.fromStore, isTrue);
  });

  test('sendToBuyer carries the buyer id and an image staged token', () async {
    pb.SendMessageToBuyerRequest? seen;
    final repo = _repo(
      FakeTransportBuilder().unary(ChatAppService.sendMessageToBuyer, (
        req,
        _,
      ) async {
        seen = req;
        return pb.SendMessageToBuyerResponse(message: protoMessage(1));
      }),
    );
    await repo.sendToBuyer('b1', requestId: 'rid', imageStagedToken: 'tok');
    expect(seen!.buyerId, 'b1');
    expect(seen!.content.imageStagedToken, 'tok');
    expect(seen!.content.hasText(), isFalse);
  });

  test('listOlder passes before_seq and returns oldest first', () async {
    pb.ListMessagesRequest? seen;
    final repo = _repo(
      FakeTransportBuilder().unary(ChatAppService.listMessages, (req, _) async {
        seen = req;
        return pb.ListMessagesResponse(
          messages: [protoMessage(5), protoMessage(4)],
          hasMore: true,
          peerReadSeq: i64(3),
        );
      }),
    );
    final page = await repo.listOlder('c1', beforeSeq: 6);
    expect(seen!.beforeSeq, i64(6));
    expect(seen!.hasAfterSeq(), isFalse);
    expect(page.messages.map((m) => m.seq), [4, 5]);
    expect(page.hasMore, isTrue);
    expect(page.peerReadSeq, 3);
  });

  test('listAfter pages with after_seq until has_more is false', () async {
    final afters = <dynamic>[];
    final repo = _repo(
      FakeTransportBuilder().unary(ChatAppService.listMessages, (req, _) async {
        afters.add(req.afterSeq);
        return afters.length == 1
            ? pb.ListMessagesResponse(
                messages: [protoMessage(3), protoMessage(4)],
                hasMore: true,
              )
            : pb.ListMessagesResponse(messages: [protoMessage(5)]);
      }),
    );
    final page = await repo.listAfter('c1', 2);
    expect(afters, [i64(2), i64(4)]);
    expect(page.messages.map((m) => m.seq), [3, 4, 5]);
  });

  test('findConversation pages through the list until it matches', () async {
    var call = 0;
    final repo = _repo(
      FakeTransportBuilder().unary(ChatAppService.listStoreConversations, (
        req,
        _,
      ) async {
        call++;
        return call == 1
            ? pb.ListStoreConversationsResponse(
                conversations: [protoConversation('c1', 'b1')],
                nextPageToken: 't2',
              )
            : pb.ListStoreConversationsResponse(
                conversations: [protoConversation('c2', 'b2')],
              );
      }),
    );
    expect((await repo.findConversation(buyerId: 'b2'))?.id, 'c2');
    call = 0;
    expect(await repo.findConversation(buyerId: 'none'), isNull);
  });
}
