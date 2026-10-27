import 'dart:collection';

/// Immutable subscriber data rendered by the presentation layer.
final class Subscriber {
  const Subscriber({
    required this.buyerId,
    required this.subscribedAt,
    this.nickname = '',
    this.avatarUrl = '',
    this.deleted = false,
  });

  final String buyerId;
  final String subscribedAt;
  final String nickname;
  final String avatarUrl;

  /// The buyer deleted their account (`BuyerCard.deleted`).
  final bool deleted;

  /// 已刪除的買家顯示墓碑字串，與舊 buyer-service 及單體 legacy adapter 的字面值相同。
  String get displayName {
    if (deleted) return '已刪除帳號';
    return nickname.isNotEmpty ? nickname : '用戶 $buyerId';
  }
}

/// Immutable snapshot consumed by [SubscriberListScreen].
final class SubscriberListViewData {
  SubscriberListViewData(
    Iterable<Subscriber> subscribers, {
    this.nextPageToken = '',
    this.loadingMore = false,
  }) : subscribers = UnmodifiableListView<Subscriber>(
         List<Subscriber>.of(subscribers),
       );

  final UnmodifiableListView<Subscriber> subscribers;
  final String nextPageToken;
  final bool loadingMore;
}
