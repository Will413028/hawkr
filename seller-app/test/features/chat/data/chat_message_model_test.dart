import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_seller/features/chat/data/chat_model.dart';

void main() {
  group('ChatMessage.fromJson', () {
    test('parses senderName from senderName key', () {
      final m = ChatMessage.fromJson({
        'id': '1',
        'from': '10',
        'to': '20',
        'content': 'hi',
        'type': 'text',
        'timestamp': 1700000000000,
        'senderName': 'Alice',
      });
      expect(m.senderName, 'Alice');
    });

    test('parses senderName from fromName fallback', () {
      final m = ChatMessage.fromJson({
        'id': '1',
        'from': '10',
        'to': '20',
        'content': 'hi',
        'type': 'text',
        'timestamp': 1700000000000,
        'fromName': 'Bob',
      });
      expect(m.senderName, 'Bob');
    });

    test('parses senderAvatar from the WS avatarKey field', () {
      final m = ChatMessage.fromJson({
        'id': '1',
        'from': '10',
        'to': '20',
        'content': 'hi',
        'type': 'text',
        'timestamp': 1700000000000,
        'avatarKey': 'public/user_avatar/10/a.png',
      });
      expect(m.senderAvatar, 'public/user_avatar/10/a.png');
    });

    test('retired avatar names are no longer read', () {
      final m = ChatMessage.fromJson({
        'id': '1',
        'from': '10',
        'to': '20',
        'content': 'hi',
        'type': 'text',
        'timestamp': 1700000000000,
        'senderAvatar': 'https://cdn/a.jpg',
        'sender_avatar': 'https://cdn/a.jpg',
        'senderAvatarUrl': 'https://cdn/a.jpg',
        'avatarUrl': 'https://cdn/a.jpg',
      });
      expect(m.senderAvatar, isNull);
    });

    test('senderName and senderAvatar default to null', () {
      final m = ChatMessage.fromJson({
        'id': '1',
        'from': '10',
        'to': '20',
        'content': 'hi',
        'type': 'text',
        'timestamp': 1700000000000,
      });
      expect(m.senderName, isNull);
      expect(m.senderAvatar, isNull);
    });

    test('isSentBy returns true when senderId matches', () {
      final m = ChatMessage.fromJson({
        'id': '1',
        'from': '42',
        'to': '99',
        'content': 'hi',
        'type': 'text',
        'timestamp': 1700000000000,
      });
      expect(m.isSentBy('42'), isTrue);
      expect(m.isSentBy('99'), isFalse);
    });
  });
}
