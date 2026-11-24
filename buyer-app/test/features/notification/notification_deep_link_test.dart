import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_buyer/features/chat/data/active_chat_gate.dart';
import 'package:hawkr_buyer/features/notification/data/push/deep_link.dart';

void main() {
  // deep-links.md 的 BUYER 受眾路徑。
  const table = <String, String>{
    '/account/orders/O123': '/account/orders/O123',
    '/account/after-sales/A9': '/account/after-sales/A9',
    '/account/messages/store-1': '/account/messages/store-1',
    '/sellers/s1/messages/t9': '/merchant/s1',
    '/sellers/s1/posts': '/merchant/s1',
    '/sellers/s1': '/merchant/s1',
    '/products/p77': '/products/p77',
    '/sellers/s1/reviews': '/merchant/s1',
    '/account/reports/r5': '/settings/reports/r5',
    '/account/notifications': notificationInboxLocation,
  };

  test('路徑表每一列對應到 App 位置', () {
    table.forEach((link, location) {
      expect(resolveNotificationLocation(link), location, reason: link);
    });
  });

  test('en 前綴的版本對應到同一個位置', () {
    table.forEach((link, location) {
      expect(resolveNotificationLocation('/en$link'), location, reason: link);
    });
    expect(resolveNotificationLocation('/en'), notificationInboxLocation);
  });

  test('query 與結尾斜線不影響；ID 的特殊字元保留編碼', () {
    expect(
      resolveNotificationLocation('/account/orders/O123?from=push'),
      '/account/orders/O123',
    );
    expect(
      resolveNotificationLocation('/account/orders/O%2F1/'),
      '/account/orders/O%2F1',
    );
  });

  test('空、不認得、賣家端與缺 ID 的路徑開收件匣', () {
    for (final link in [
      '',
      '  ',
      '/',
      '/unknown/path',
      '/seller-center/orders/O1',
      '/seller-center/notifications',
      '/account/orders',
      '/account/orders/O1/extra',
      '/en/seller/measures/m1',
    ]) {
      expect(
        resolveNotificationLocation(link),
        notificationInboxLocation,
        reason: link,
      );
    }
  });

  test('外部連結、// 與非絕對路徑拒絕', () {
    for (final link in [
      'https://example.com/account/orders/O1',
      '//example.com/x',
      'account/orders/O1',
      r'/\evil.com',
      'javascript:alert(1)',
    ]) {
      expect(resolveNotificationLocation(link), isNull, reason: link);
    }
  });

  test('聊天路徑取最後一段，其他路徑不是聊天', () {
    expect(chatTargetOfDeepLink('/account/messages/store-1'), 'store-1');
    expect(chatTargetOfDeepLink('/en/account/messages/store-1'), 'store-1');
    expect(chatTargetOfDeepLink('/account/orders/O1'), isNull);
    expect(chatTargetOfDeepLink('/seller-center/messages/u1'), isNull);
    expect(chatTargetOfDeepLink(''), isNull);
  });

  test('正開著的聊天室是深連結指向的店家才抑制 banner', () {
    String? storeOf(String c) => {'conv-1': 'store-1'}[c];
    expect(
      activeChatIsStore(
        const ActiveChat(storeId: 'store-1'),
        'store-1',
        storeOf,
      ),
      isTrue,
    );
    expect(
      activeChatIsStore(
        const ActiveChat(conversationId: 'conv-1'),
        'store-1',
        storeOf,
      ),
      isTrue,
    );
    expect(
      activeChatIsStore(
        const ActiveChat(conversationId: 'conv-1'),
        'store-2',
        storeOf,
      ),
      isFalse,
    );
    expect(
      activeChatIsStore(const ActiveChat.empty(), 'store-1', storeOf),
      isFalse,
    );
  });
}
