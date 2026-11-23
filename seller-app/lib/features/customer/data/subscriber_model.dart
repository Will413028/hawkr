import 'package:hawkr_ui_text/hawkr_ui_text.dart';
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

  /// 已刪除的買家用目錄的 `buyer.Profile.deleted_name`。
  String displayName(UiLocale l) {
    if (deleted) return buyerProfileDeletedName(l);
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
