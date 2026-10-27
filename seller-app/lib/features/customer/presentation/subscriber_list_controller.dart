import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/exceptions/app_exception.dart';
import '../data/customer_model.dart';
import '../data/subscriber_model.dart';
import '../data/subscriber_repository.dart';

part 'subscriber_list_controller.g.dart';

@riverpod
class SubscriberListController extends _$SubscriberListController {
  /// Bumped by [refresh]; a [loadMore] started under an older generation must
  /// not write its page over the refreshed list.
  var _generation = 0;

  @override
  Future<SubscriberListViewData> build() => _load();

  Future<SubscriberListViewData> _load() async {
    final page = await ref.read(subscriberRepositoryProvider).loadSubscribers();
    return SubscriberListViewData(
      page.subscribers,
      nextPageToken: page.nextPageToken,
    );
  }

  Future<void> loadMore() async {
    final current = state.value;
    if (current == null ||
        current.nextPageToken.isEmpty ||
        current.loadingMore) {
      return;
    }
    final generation = _generation;
    bool isStale() => !ref.mounted || generation != _generation;
    state = AsyncData(
      SubscriberListViewData(
        current.subscribers,
        nextPageToken: current.nextPageToken,
        loadingMore: true,
      ),
    );
    try {
      final page = await ref
          .read(subscriberRepositoryProvider)
          .loadSubscribers(pageToken: current.nextPageToken);
      if (isStale()) return;
      state = AsyncData(
        SubscriberListViewData([
          ...current.subscribers,
          ...page.subscribers,
        ], nextPageToken: page.nextPageToken),
      );
    } catch (_) {
      if (isStale()) return;
      state = AsyncData(current);
      rethrow;
    }
  }

  Future<void> refresh() async {
    _generation++;
    state = const AsyncLoading();
    state = await AsyncValue.guard(_load);
  }

  /// Navigation remains a presentation concern; resolving its typed payload is
  /// a controller command so the view never reaches into a repository.
  Future<Customer?> resolveCustomer(String accountId) {
    return ref.read(subscriberRepositoryProvider).resolveCustomer(accountId);
  }
}

String subscriberListErrorMessage(Object error) {
  if (error case AppException(:final message)) return message;
  return '無法載入訂閱者，請稍後再試';
}
