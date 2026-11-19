import 'dart:async';

import '../../../core/session/viewer_scoped_notifier.dart';

enum SectionPhase {
  /// 第一次讀取中（或失敗後重試中）。
  loading,
  ready,

  /// 第一次讀取失敗，沒有任何可顯示的項目。
  failed,
}

/// 一個分頁清單的狀態：已載入的項目、下一頁的 token（空字串表示沒有下一頁）、階段與載入更多的旗標。
class PagedSection<T> {
  const PagedSection({
    this.phase = SectionPhase.loading,
    this.items = const [],
    this.nextPageToken = '',
    this.loadingMore = false,
    this.error,
  });

  final SectionPhase phase;
  final List<T> items;
  final String nextPageToken;
  final bool loadingMore;

  /// 最近一次讀取失敗的原因；下一次成功就清掉。
  final Object? error;

  bool get hasMore => nextPageToken.isNotEmpty;

  PagedSection<T> copyWith({
    SectionPhase? phase,
    List<T>? items,
    String? nextPageToken,
    bool? loadingMore,
    Object? error,
    bool clearError = false,
  }) => PagedSection<T>(
    phase: phase ?? this.phase,
    items: items ?? this.items,
    nextPageToken: nextPageToken ?? this.nextPageToken,
    loadingMore: loadingMore ?? this.loadingMore,
    error: clearError ? null : (error ?? this.error),
  );
}

typedef PageFetch<T> =
    Future<({List<T> items, String nextPageToken})> Function(String pageToken);

/// 一個 [PagedSection] 的讀取：第一頁與載入更多。重新整理有序號，讓整理之前發出的「載入更多」不蓋掉新的第一頁；
/// 觀看者換了由 [ViewerScope] 丟棄。狀態怎麼存回 notifier 由 [read]、[write] 決定。
class SectionLoader<T> {
  SectionLoader({
    required this.read,
    required this.write,
    required this.fetch,
    required this.scope,
  });

  final PagedSection<T> Function() read;
  final void Function(PagedSection<T> next) write;
  final PageFetch<T> fetch;
  final ViewerScope Function() scope;

  int _epoch = 0;

  /// 讀第一頁並取代現有內容。回傳是否成功；失敗時保留現有內容（從沒讀成功過則進入 failed）。
  Future<bool> loadFirst() async {
    final epoch = ++_epoch;
    final s = scope();
    if (read().phase == SectionPhase.failed) {
      write(read().copyWith(phase: SectionPhase.loading));
    }
    try {
      final page = await s.run(fetch(''));
      if (epoch != _epoch) return true;
      write(
        PagedSection<T>(
          phase: SectionPhase.ready,
          items: page.items,
          nextPageToken: page.nextPageToken,
        ),
      );
      return true;
    } on StaleScopeException {
      return true;
    } catch (e) {
      if (epoch != _epoch) return true;
      if (read().phase != SectionPhase.ready) {
        write(read().copyWith(phase: SectionPhase.failed, error: e));
      } else {
        write(read().copyWith(error: e));
      }
      return false;
    }
  }

  /// 讀下一頁接在後面。回傳是否成功（沒有下一頁或已在讀也算成功）。
  Future<bool> loadMore() async {
    final current = read();
    if (!current.hasMore || current.loadingMore) return true;
    final epoch = _epoch;
    final s = scope();
    write(current.copyWith(loadingMore: true));
    try {
      final page = await s.run(fetch(current.nextPageToken));
      if (epoch != _epoch) return true;
      write(
        read().copyWith(
          items: [...read().items, ...page.items],
          nextPageToken: page.nextPageToken,
          loadingMore: false,
          clearError: true,
        ),
      );
      return true;
    } on StaleScopeException {
      return true;
    } catch (e) {
      if (epoch != _epoch) return true;
      write(read().copyWith(loadingMore: false, error: e));
      return false;
    }
  }
}

/// 第一次讀取排在下一個 microtask，讓 `build` 先回傳初始狀態。
void scheduleInitialLoad(ViewerScope scope, Future<void> Function() load) {
  scheduleMicrotask(() {
    if (scope.isCurrent) unawaited(load());
  });
}
