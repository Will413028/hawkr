import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../data/store_block_repository.dart';

part 'report_list_controller.g.dart';

// ============================================================
// 封鎖名單
// ============================================================

class BlockListState {
  const BlockListState({
    this.blocks = const [],
    this.nextPageToken = '',
    this.isLoadingMore = false,
  });

  final List<BlockedBuyerItem> blocks;
  final String nextPageToken;
  final bool isLoadingMore;

  bool get hasMore => nextPageToken.isNotEmpty;

  BlockListState copyWith({
    List<BlockedBuyerItem>? blocks,
    String? nextPageToken,
    bool? isLoadingMore,
  }) {
    return BlockListState(
      blocks: blocks ?? this.blocks,
      nextPageToken: nextPageToken ?? this.nextPageToken,
      isLoadingMore: isLoadingMore ?? this.isLoadingMore,
    );
  }
}

/// 店家封鎖的買家名單（`ListBlockedBuyers`，keyset 分頁）。
///
/// 每次從第一頁重載（build、refresh、silentRefresh）都換一個世代；跨過重載才回來的 loadMore 結果屬於舊世代，直接丟掉。
/// 進行中的解除封鎖以 [_pendingUnblock] 記錄，重載與分頁的結果都濾掉這些買家，直到那次 RPC 結束；
/// 成功的解除再記在 [_unblockedThisGeneration]，擋住重載前發出、之後才回來的分頁帶回舊資料。
@riverpod
class BlockListController extends _$BlockListController {
  static const int _pageSize = 20;
  int _generation = 0;
  final Set<String> _pendingUnblock = {};
  final Set<String> _unblockedThisGeneration = {};

  StoreBlockRepository get _repo => ref.read(storeBlockRepositoryProvider);

  @override
  Future<BlockListState> build() async => (await _fetchFirstPage()).$2;

  Future<(int, BlockListState)> _fetchFirstPage() async {
    final generation = ++_generation;
    final page = await _repo.list(pageSize: _pageSize);
    final blocks = _withoutPending(page.items);
    _unblockedThisGeneration.clear();
    return (
      generation,
      BlockListState(blocks: blocks, nextPageToken: page.nextPageToken),
    );
  }

  List<BlockedBuyerItem> _withoutPending(List<BlockedBuyerItem> items) => items
      .where(
        (b) =>
            !_pendingUnblock.contains(b.buyerId) &&
            !_unblockedThisGeneration.contains(b.buyerId),
      )
      .toList();

  Future<void> refresh() async {
    state = const AsyncValue.loading();
    final next = await AsyncValue.guard(_fetchFirstPage);
    if (next.hasValue && next.requireValue.$1 != _generation) return;
    state = next.whenData((r) => r.$2);
  }

  /// 下拉刷新失敗時保留既有清單，rethrow 交給呼叫端顯示 toast。
  Future<void> silentRefresh() async {
    final (generation, next) = await _fetchFirstPage();
    if (generation != _generation) return;
    state = AsyncValue.data(next);
  }

  /// 分頁載入失敗時保留已載入清單，只重置 loading 旗標並 rethrow。世代已換的結果（含失敗）直接丟掉。
  Future<void> loadMore() async {
    final current = state.value;
    if (current == null || current.isLoadingMore || !current.hasMore) return;
    final generation = _generation;
    state = AsyncValue.data(current.copyWith(isLoadingMore: true));
    try {
      final page = await _repo.list(
        pageToken: current.nextPageToken,
        pageSize: _pageSize,
      );
      if (generation != _generation) return;
      final latest = state.requireValue;
      final seen = latest.blocks.map((b) => b.buyerId).toSet();
      final fresh = _withoutPending(
        page.items,
      ).where((b) => seen.add(b.buyerId)).toList();
      state = AsyncValue.data(
        latest.copyWith(
          blocks: [...latest.blocks, ...fresh],
          nextPageToken: page.nextPageToken,
          isLoadingMore: false,
        ),
      );
    } on Exception {
      if (generation != _generation) return;
      state = AsyncValue.data(
        state.requireValue.copyWith(isLoadingMore: false),
      );
      rethrow;
    }
  }

  /// 樂觀移除；失敗時若清單沒被重載過，就把這一筆放回它的 keyset 位置，否則重載；兩種都 rethrow 讓呼叫端顯示錯誤。
  Future<void> unblock(String buyerId) async {
    final current = state.value;
    if (current == null) return;
    final removed = current.blocks.where((b) => b.buyerId == buyerId);
    if (removed.isEmpty || _pendingUnblock.contains(buyerId)) return;
    final item = removed.first;
    final generation = _generation;
    _pendingUnblock.add(buyerId);
    state = AsyncValue.data(
      current.copyWith(
        blocks: current.blocks.where((b) => b.buyerId != buyerId).toList(),
      ),
    );
    try {
      await _repo.unblock(buyerId);
      _unblockedThisGeneration.add(buyerId);
      _pendingUnblock.remove(buyerId);
    } on Exception {
      _pendingUnblock.remove(buyerId);
      if (generation == _generation) {
        final latest = state.requireValue;
        final blocks = [...latest.blocks];
        var at = blocks.indexWhere((b) => item.compareInList(b) < 0);
        if (at < 0) at = blocks.length;
        blocks.insert(at, item);
        state = AsyncValue.data(latest.copyWith(blocks: blocks));
      } else {
        try {
          await silentRefresh();
        } on Exception {
          // 重載失敗時沿用目前清單；原本的解除失敗會往上丟。
        }
      }
      rethrow;
    } catch (_) {
      _pendingUnblock.remove(buyerId);
      rethrow;
    }
  }
}
