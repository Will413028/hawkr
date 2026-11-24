/// 收件匣畫面在 App 內的位置（未知與空的深連結的目的地）。
const notificationInboxLocation = '/notifications';

/// 伺服器給的深連結是 website 的相對路徑，收件人語系是 en 時帶 `/en` 前綴（格式與路徑表見
/// `backend/docs/plans/2026-11-23-a8-notification/deep-links.md`）。這裡把買家受眾的路徑對應到 App 的畫面位置：
/// 外部連結、格式錯誤、空字串與不認得的路徑都回收件匣。
String resolveNotificationLocation(String rawLink) {
  final link = rawLink.trim();
  if (link.isEmpty) return notificationInboxLocation;
  if (!link.startsWith('/') || link.startsWith('//') || link.contains(r'\')) {
    return notificationInboxLocation;
  }
  final uri = Uri.tryParse(link);
  if (uri == null || uri.hasScheme || uri.hasAuthority) {
    return notificationInboxLocation;
  }

  final segments = [...uri.pathSegments.where((s) => s.isNotEmpty)];
  if (segments.isNotEmpty && segments.first == 'en') segments.removeAt(0);

  String path(List<String> parts) =>
      '/${parts.map(Uri.encodeComponent).join('/')}';

  switch (segments) {
    case ['account', 'orders', final no]:
      return path(['account', 'orders', no]);
    case ['account', 'after-sales', final no]:
      return path(['account', 'after-sales', no]);
    case ['account', 'messages', final targetId]:
      return path(['account', 'messages', targetId]);
    case ['account', 'reports', final id]:
      return path(['settings', 'reports', id]);
    case ['account', 'notifications']:
      return notificationInboxLocation;
    case ['products', final id]:
      return path(['products', id]);
    case ['sellers', final storeCode] ||
        ['sellers', final storeCode, 'posts'] ||
        ['sellers', final storeCode, 'reviews'] ||
        ['sellers', final storeCode, 'messages', _]:
      return path(['merchant', storeCode]);
    default:
      return notificationInboxLocation;
  }
}

/// 深連結指向的買家聊天室：`/account/messages/{targetId}`（可帶 `/en`）取最後一段，不是聊天路徑回 null。
String? chatTargetOfDeepLink(String rawLink) {
  final location = resolveNotificationLocation(rawLink);
  final segments = Uri.parse(location).pathSegments;
  if (segments.length == 3 &&
      segments[0] == 'account' &&
      segments[1] == 'messages') {
    return segments[2];
  }
  return null;
}
