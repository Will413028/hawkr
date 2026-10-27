import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../data/block_model.dart';
import '../data/block_repository.dart';
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
    this.page = 1,
    this.hasMore = true,
    this.isLoadingMore = false,
  });

  final List<BlockedUser> blocks;
  final int page;
  final bool hasMore;
  final bool isLoadingMore;

  BlockListState copyWith({
    List<BlockedUser>? blocks,
    int? page,
    bool? hasMore,
    bool? isLoadingMore,
  }) {
    return BlockListState(
      blocks: blocks ?? this.blocks,
      page: page ?? this.page,
      hasMore: hasMore ?? this.hasMore,
      isLoadingMore: isLoadingMore ?? this.isLoadingMore,
    );
  }
}

/// 封鎖名單 AsyncNotifier（backend-payuni#645）。
@riverpod
class BlockListController extends _$BlockListController {
  static const int _pageSize = 20;

  BlockRepository get _repo => ref.read(blockRepositoryProvider);

  @override
  Future<BlockListState> build() async {
    final records = await _repo.listBlockedUsers(page: 1, pageSize: _pageSize);
    return BlockListState(
      blocks: records,
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
      final records = await _repo.listBlockedUsers(
        page: nextPage,
        pageSize: _pageSize,
      );
      state = AsyncValue.data(
        current.copyWith(
          blocks: [...current.blocks, ...records],
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

  /// Optimistic 移除；API 失敗時回滾並 rethrow，讓呼叫端顯示錯誤
  /// （review fix，backend-payuni#645：先前吞例外導致靜默失敗）。
  Future<void> unblock(String userId) async {
    final current = state.value;
    if (current == null) return;
    final previous = current.blocks;
    state = AsyncValue.data(
      current.copyWith(
        blocks: current.blocks.where((b) => b.userId != userId).toList(),
      ),
    );
    try {
      await _repo.unblockUser(userId);
    } catch (e) {
      state = AsyncValue.data(current.copyWith(blocks: previous));
      rethrow;
    }
  }
}
