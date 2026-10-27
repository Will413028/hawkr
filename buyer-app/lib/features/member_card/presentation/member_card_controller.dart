import 'package:riverpod_annotation/riverpod_annotation.dart';
import '../../../core/session/session_provider.dart';
import '../data/member_card_model.dart';
import '../data/member_card_repository.dart';

part 'member_card_controller.g.dart';

@riverpod
class MemberCardsController extends _$MemberCardsController {
  bool _loadingMore = false;
  String _nextPageToken = '';

  /// Bumped by every [build] (refresh, session change); a [loadMore] started
  /// under an older generation must not write its page over the new list.
  var _generation = 0;

  @override
  Future<({List<MemberCard> records, bool hasMore})> build() async {
    final generation = ++_generation;
    _loadingMore = false;
    _nextPageToken = '';
    // Apple App Review 合規：訪客（無 token）打 member-cards API 必 401，回空不打。
    final session = ref.watch(sessionProvider).value;
    if (!hasUsableSession(session)) {
      return (records: const <MemberCard>[], hasMore: false);
    }
    final page = await ref.watch(memberCardRepositoryProvider).getMemberCards();
    if (generation == _generation) _nextPageToken = page.nextPageToken;
    return (records: page.records, hasMore: page.nextPageToken.isNotEmpty);
  }

  Future<void> refresh() async {
    ref.invalidateSelf();
    await future;
  }

  Future<void> loadMore() async {
    final current = state.value;
    if (current == null || _nextPageToken.isEmpty || _loadingMore) {
      return;
    }
    final generation = _generation;
    bool isStale() => !ref.mounted || generation != _generation;
    _loadingMore = true;
    try {
      final page = await ref
          .read(memberCardRepositoryProvider)
          .getMemberCards(pageToken: _nextPageToken);
      if (isStale()) return;
      _nextPageToken = page.nextPageToken;
      state = AsyncData((
        records: [...current.records, ...page.records],
        hasMore: _nextPageToken.isNotEmpty,
      ));
    } finally {
      if (!isStale()) _loadingMore = false;
    }
  }
}

@riverpod
class SpendingRecordsController extends _$SpendingRecordsController {
  bool _loadingMore = false;
  String _nextPageToken = '';

  /// Bumped by every [build]; see [MemberCardsController].
  var _generation = 0;

  @override
  Future<
    ({
      int totalSpending,
      int totalRefund,
      List<SpendingRecord> records,
      bool hasMore,
    })
  >
  build(String sellerId) async {
    final generation = ++_generation;
    _loadingMore = false;
    _nextPageToken = '';
    final page = await ref
        .watch(memberCardRepositoryProvider)
        .getSpendingRecords(sellerId: sellerId);
    if (generation == _generation) _nextPageToken = page.nextPageToken;
    return (
      totalSpending: page.totalSpending,
      totalRefund: page.totalRefund,
      records: page.records,
      hasMore: page.nextPageToken.isNotEmpty,
    );
  }

  Future<void> loadMore() async {
    final current = state.value;
    if (current == null || _nextPageToken.isEmpty || _loadingMore) {
      return;
    }
    final generation = _generation;
    bool isStale() => !ref.mounted || generation != _generation;
    _loadingMore = true;
    try {
      final page = await ref
          .read(memberCardRepositoryProvider)
          .getSpendingRecords(sellerId: sellerId, pageToken: _nextPageToken);
      if (isStale()) return;
      _nextPageToken = page.nextPageToken;
      state = AsyncData((
        totalSpending: page.totalSpending,
        totalRefund: page.totalRefund,
        records: [...current.records, ...page.records],
        hasMore: _nextPageToken.isNotEmpty,
      ));
    } finally {
      if (!isStale()) _loadingMore = false;
    }
  }
}
