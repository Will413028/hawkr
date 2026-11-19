import 'dart:async';

import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.pb.dart' as buyer;
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/session/viewer_scoped_notifier.dart';
import '../../social/presentation/write_result.dart';
import '../data/blocking_repository.dart';
import 'paged_section.dart';

part 'blocked_list_notifier.g.dart';

class BlockedListState {
  const BlockedListState({
    this.stores = const PagedSection<buyer.BlockedStore>(),
    this.buyers = const PagedSection<buyer.BlockedBuyer>(),
    this.unblocking = const <String>{},
  });

  final PagedSection<buyer.BlockedStore> stores;
  final PagedSection<buyer.BlockedBuyer> buyers;

  /// 解除封鎖進行中的項目（`store:<id>`、`buyer:<id>`）。
  final Set<String> unblocking;

  BlockedListState copyWith({
    PagedSection<buyer.BlockedStore>? stores,
    PagedSection<buyer.BlockedBuyer>? buyers,
    Set<String>? unblocking,
  }) => BlockedListState(
    stores: stores ?? this.stores,
    buyers: buyers ?? this.buyers,
    unblocking: unblocking ?? this.unblocking,
  );
}

String storeUnblockKey(String storeId) => 'store:$storeId';
String buyerUnblockKey(String buyerId) => 'buyer:$buyerId';

/// 封鎖名單：店家與買家兩段各自分頁。換觀看者整份重建；每個跨 await 的路徑回來先比對世代。
@riverpod
class BlockedList extends _$BlockedList with ViewerScoped {
  late SectionLoader<buyer.BlockedStore> _storeLoader;
  late SectionLoader<buyer.BlockedBuyer> _buyerLoader;

  @override
  BlockedListState build() {
    bindViewer();
    _storeLoader = SectionLoader<buyer.BlockedStore>(
      read: () => state.stores,
      write: (next) => state = state.copyWith(stores: next),
      fetch: (token) async {
        final r = await _repo.listBlockedStores(pageToken: token);
        return (items: r.blockedStores, nextPageToken: r.nextPageToken);
      },
      scope: beginScope,
    );
    _buyerLoader = SectionLoader<buyer.BlockedBuyer>(
      read: () => state.buyers,
      write: (next) => state = state.copyWith(buyers: next),
      fetch: (token) async {
        final r = await _repo.listBlockedBuyers(pageToken: token);
        return (items: r.blockedBuyers, nextPageToken: r.nextPageToken);
      },
      scope: beginScope,
    );
    scheduleInitialLoad(beginScope(), reload);
    return const BlockedListState();
  }

  BlockingRepository get _repo => ref.read(blockingRepositoryProvider);

  /// 兩段各自重讀；回傳兩段是否都成功。
  Future<bool> reload() async {
    final results = await Future.wait([
      _storeLoader.loadFirst(),
      _buyerLoader.loadFirst(),
    ]);
    return results.every((ok) => ok);
  }

  Future<bool> reloadStores() => _storeLoader.loadFirst();

  Future<bool> reloadBuyers() => _buyerLoader.loadFirst();

  Future<bool> loadMoreStores() => _storeLoader.loadMore();

  Future<bool> loadMoreBuyers() => _buyerLoader.loadMore();

  Future<WriteResult> unblockStore(String storeId) => _unblock(
    storeUnblockKey(storeId),
    () => _repo.unblockStore(storeId),
    (s) => s.copyWith(
      stores: s.stores.copyWith(
        items: [
          for (final b in s.stores.items)
            if (b.storeId != storeId) b,
        ],
      ),
    ),
  );

  Future<WriteResult> unblockBuyer(String buyerId) => _unblock(
    buyerUnblockKey(buyerId),
    () => _repo.unblockBuyer(buyerId),
    (s) => s.copyWith(
      buyers: s.buyers.copyWith(
        items: [
          for (final b in s.buyers.items)
            if (b.buyerId != buyerId) b,
        ],
      ),
    ),
  );

  Future<WriteResult> _unblock(
    String key,
    Future<void> Function() call,
    BlockedListState Function(BlockedListState) removeItem,
  ) async {
    if (state.unblocking.contains(key)) return const WriteResult.discarded();
    final scope = beginScope();
    state = state.copyWith(unblocking: {...state.unblocking, key});
    try {
      await scope.run(call());
    } on StaleScopeException {
      return const WriteResult.discarded();
    } catch (e) {
      state = state.copyWith(unblocking: {...state.unblocking}..remove(key));
      return WriteResult.failed(e);
    }
    state = removeItem(
      state.copyWith(unblocking: {...state.unblocking}..remove(key)),
    );
    return const WriteResult.applied();
  }
}
