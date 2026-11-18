import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../data/store_block_repository.dart';
import '../data/report_model.dart';
import '../data/report_repository.dart';

part 'report_list_controller.g.dart';

// ============================================================
// 我的檢舉列表
// ============================================================

class MyReportsState {
  const MyReportsState({
    this.reports = const [],
    this.page = 1,
    this.hasMore = true,
    this.isLoadingMore = false,
  });

  final List<ReportRecord> reports;
  final int page;
  final bool hasMore;
  final bool isLoadingMore;

  MyReportsState copyWith({
    List<ReportRecord>? reports,
    int? page,
    bool? hasMore,
    bool? isLoadingMore,
  }) {
    return MyReportsState(
      reports: reports ?? this.reports,
      page: page ?? this.page,
      hasMore: hasMore ?? this.hasMore,
      isLoadingMore: isLoadingMore ?? this.isLoadingMore,
    );
  }
}

/// 我的檢舉列表 AsyncNotifier（backend-payuni#645）。
@riverpod
class MyReportsController extends _$MyReportsController {
  static const int _pageSize = 20;

  ReportRepository get _repo => ref.read(reportRepositoryProvider);

  @override
  Future<MyReportsState> build() async {
    final records = await _repo.listMyReports(page: 1, pageSize: _pageSize);
    return MyReportsState(
      reports: records,
      page: 1,
      hasMore: records.length == _pageSize,
    );
  }

  Future<void> refresh() async {
    state = const AsyncValue.loading();
    state = await AsyncValue.guard(() => build());
  }

  Future<void> silentRefresh() async {
    try {
      final next = await build();
      state = AsyncValue.data(next);
    } catch (e) {
      // 下拉刷新失敗：保留既有清單，不用 AsyncValue.guard 把整頁蓋成
      // error（review fix，backend-payuni#645）。交由呼叫端 catch 這個
      // rethrow 顯示 toast，畫面維持在原本的清單上。
      rethrow;
    }
  }

  Future<void> loadMore() async {
    final current = state.value;
    if (current == null || current.isLoadingMore || !current.hasMore) return;
    state = AsyncValue.data(current.copyWith(isLoadingMore: true));
    try {
      final nextPage = current.page + 1;
      final records = await _repo.listMyReports(
        page: nextPage,
        pageSize: _pageSize,
      );
      state = AsyncValue.data(
        current.copyWith(
          reports: [...current.reports, ...records],
          page: nextPage,
          hasMore: records.length == _pageSize,
          isLoadingMore: false,
        ),
      );
    } catch (e) {
      // 分頁載入失敗：保留已載入清單，只重置 loading 旗標，避免整頁被
      // AsyncValue.error 蓋掉（review fix，backend-payuni#645）。
      // 交由呼叫端（畫面）catch 這個 rethrow 顯示 toast。
      state = AsyncValue.data(current.copyWith(isLoadingMore: false));
      rethrow;
    }
  }
}

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
@riverpod
class BlockListController extends _$BlockListController {
  static const int _pageSize = 20;

  StoreBlockRepository get _repo => ref.read(storeBlockRepositoryProvider);

  @override
  Future<BlockListState> build() async {
    final page = await _repo.list(pageSize: _pageSize);
    return BlockListState(
      blocks: page.items,
      nextPageToken: page.nextPageToken,
    );
  }

  Future<void> refresh() async {
    state = const AsyncValue.loading();
    state = await AsyncValue.guard(() => build());
  }

  /// 下拉刷新失敗時保留既有清單，rethrow 交給呼叫端顯示 toast。
  Future<void> silentRefresh() async {
    final next = await build();
    state = AsyncValue.data(next);
  }

  /// 分頁載入失敗時保留已載入清單，只重置 loading 旗標並 rethrow。
  Future<void> loadMore() async {
    final current = state.value;
    if (current == null || current.isLoadingMore || !current.hasMore) return;
    state = AsyncValue.data(current.copyWith(isLoadingMore: true));
    try {
      final page = await _repo.list(
        pageToken: current.nextPageToken,
        pageSize: _pageSize,
      );
      state = AsyncValue.data(
        current.copyWith(
          blocks: [...current.blocks, ...page.items],
          nextPageToken: page.nextPageToken,
          isLoadingMore: false,
        ),
      );
    } catch (e) {
      state = AsyncValue.data(current.copyWith(isLoadingMore: false));
      rethrow;
    }
  }

  /// 樂觀移除；失敗時回滾並 rethrow，讓呼叫端顯示錯誤。
  Future<void> unblock(String buyerId) async {
    final current = state.value;
    if (current == null) return;
    final previous = current.blocks;
    state = AsyncValue.data(
      current.copyWith(
        blocks: current.blocks.where((b) => b.buyerId != buyerId).toList(),
      ),
    );
    try {
      await _repo.unblock(buyerId);
    } catch (e) {
      state = AsyncValue.data(current.copyWith(blocks: previous));
      rethrow;
    }
  }
}
