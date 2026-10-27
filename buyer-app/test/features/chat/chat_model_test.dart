import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_buyer/features/chat/data/chat_model.dart';

void main() {
  group('ChatMessage.fromJson', () {
    test('normalizes id/messageId alias', () {
      final m = ChatMessage.fromJson({
        'messageId': 'abc',
        'fromId': 'u1',
        'toId': 'u2',
        'content': 'hi',
        'timestamp': 1_700_000_000_000,
      });
      expect(m.id, 'abc');
      expect(m.senderId, 'u1');
      expect(m.receiverId, 'u2');
      expect(m.messageContent, 'hi');
    });

    test('normalizes from/to aliases', () {
      final m = ChatMessage.fromJson({
        'id': 'x',
        'from': 'a',
        'to': 'b',
        'content': 'hey',
        'timestamp': 0,
      });
      expect(m.senderId, 'a');
      expect(m.receiverId, 'b');
    });

    test('msgType int 2 → image', () {
      final m = ChatMessage.fromJson({
        'id': 'x',
        'fromId': 'u1',
        'toId': 'u2',
        'content': 'img.jpg',
        'msgType': 2,
        'timestamp': 0,
      });
      expect(m.messageType, 'image');
    });

    test('MESSAGE_TYPE_IMAGE string → image', () {
      final m = ChatMessage.fromJson({
        'id': 'x',
        'senderId': 'u1',
        'receiverId': 'u2',
        'content': '',
        'type': 'MESSAGE_TYPE_IMAGE',
        'timestamp': 0,
      });
      expect(m.messageType, 'image');
    });

    test('MESSAGE_TYPE_TEXT string → text', () {
      final m = ChatMessage.fromJson({
        'id': 'x',
        'senderId': 'u1',
        'receiverId': 'u2',
        'content': '',
        'type': 'MESSAGE_TYPE_TEXT',
        'timestamp': 0,
      });
      expect(m.messageType, 'text');
    });

    test('seqId tolerated as string', () {
      final m = ChatMessage.fromJson({
        'id': 'x',
        'fromId': 'u1',
        'toId': 'u2',
        'content': '',
        'timestamp': 0,
        'seqId': '42',
      });
      expect(m.seqId, 42);
    });

    test('int timestamp parsed correctly', () {
      final m = ChatMessage.fromJson({
        'id': 'x',
        'fromId': 'u1',
        'toId': 'u2',
        'content': '',
        'timestamp': 1_700_000_000_000,
      });
      expect(m.timestamp.millisecondsSinceEpoch, 1_700_000_000_000);
    });

    test('senderName resolved from multiple aliases', () {
      final m = ChatMessage.fromJson({
        'id': 'x',
        'fromId': 'u1',
        'toId': 'u2',
        'content': '',
        'timestamp': 0,
        'sender_name': 'Alice',
      });
      expect(m.senderName, 'Alice');
    });

    test('defaults: deleted=false, read=false, seqId=0', () {
      final m = ChatMessage.fromJson({
        'id': 'x',
        'fromId': 'u1',
        'toId': 'u2',
        'content': '',
        'timestamp': 0,
      });
      expect(m.deleted, false);
      expect(m.read, false);
      expect(m.seqId, 0);
    });
  });

  group('ChatMessage.isSentBy', () {
    test('returns true for matching senderId', () {
      final m = ChatMessage.fromJson({
        'id': 'x',
        'senderId': 'u1',
        'receiverId': 'u2',
        'content': '',
        'timestamp': 0,
      });
      expect(m.isSentBy('u1'), true);
      expect(m.isSentBy('u2'), false);
    });
  });
}
