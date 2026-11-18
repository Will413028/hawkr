import 'dart:async';

import '../../../core/session/viewer_scoped_notifier.dart';
import 'thread_tree.dart';

enum ThreadPhase {
  /// 第一次讀取中（或失敗後重試中）。
  loading,
  ready,

  /// 第一次讀取失敗，沒有任何可顯示的項目。
  failed,
}

/// 一頁讀取結果；[meta] 帶各自回應裡樹以外的欄位。
class ThreadPage<T> {
  const ThreadPage(this.items, this.nextPageToken, {this.meta});

  final List<T> items;
  final String nextPageToken;
  final Object? meta;
}

/// [ThreadNotifier] 對狀態的要求：階段與樹，並能只換這兩項。
abstract interface class ThreadStateView<S, T> {
  ThreadPhase get phase;
  ThreadTree<T> get tree;
  S withThread({ThreadPhase? phase, ThreadTree<T>? tree});
}

class _Read {
  _Read(this.seq);

  final int seq;
}

class _Op<T> {
  _Op(this.seq, this.apply);

  final int seq;
  final ThreadTree<T> Function(ThreadTree<T> tree) apply;
}

/// 本地刪除的還原柄；失敗時交給 [ThreadNotifier.rollbackDelete]。
class ThreadDeletion<T> {
  ThreadDeletion._(this._op, this._undo);

  final _Op<T> _op;
  final ThreadTree<T> Function(ThreadTree<T> current) _undo;
}

/// 留言與討論區共用的討論串讀取與本地寫入：第一頁、分頁、回覆展開，以及讀取與本地寫入的交錯。
///
/// 換觀看者整份重建（世代）；重新整理另有序號，讓整理之前發出的分頁與展開不蓋掉新的第一頁。
/// 讀取發出之後才發生的本地新增、刪除、編輯，在讀取結果寫入時重新套用一次，
/// 避免較早發出的讀取把刪掉的項目帶回來或蓋掉剛新增的。進行中旗標以執行個體辨識，
/// 重新整理失敗、被丟棄都不會讓旗標卡住。
mixin ThreadNotifier<S extends ThreadStateView<S, T>, T> on ViewerScoped<S, S> {
  int _epoch = 0;
  int _opSeq = 0;
  final List<_Op<T>> _ops = <_Op<T>>[];
  final List<_Read> _activeReads = <_Read>[];
  Object? _moreRun;
  final Map<String, Object> _replyRuns = <String, Object>{};

  Future<ThreadPage<T>> fetchPage({String? parentId, String pageToken = ''});

  /// 第一頁讀成功之後的整份狀態；[current] 供保留寫入中旗標之類的欄位。
  S firstPageState(S current, ThreadTree<T> tree, ThreadPage<T> page);

  /// 頂層下一頁的 token；沒有分頁就回空字串。
  String rootsPageToken(S current) => '';

  S withRootsPageToken(S current, String token) => current;

  /// 第一頁讀完之後是否自動展開有回覆的頂層項目。
  bool get autoLoadReplies => false;

  /// `build` 內在 [bindViewer] 之後呼叫：重置並排程第一次讀取。
  void startThread() {
    _epoch++;
    _ops.clear();
    _activeReads.clear();
    _moreRun = null;
    _replyRuns.clear();
    final scope = beginScope();
    scheduleMicrotask(() {
      if (scope.isCurrent) unawaited(loadFirstPage(scope));
    });
  }

  _Read _beginRead() {
    final read = _Read(_opSeq);
    _activeReads.add(read);
    return read;
  }

  void _endRead(_Read read) {
    _activeReads.remove(read);
    if (_activeReads.isEmpty) _ops.clear();
  }

  ThreadTree<T> _reconcile(ThreadTree<T> tree, _Read read) {
    var next = tree;
    for (final op in _ops) {
      if (op.seq > read.seq) next = op.apply(next);
    }
    return next;
  }

  /// 讀第一頁並取代現有內容。回傳是否成功；失敗時保留現有內容（從沒讀成功過則進入 failed）。
  Future<bool> loadFirstPage(ViewerScope scope) async {
    final epoch = ++_epoch;
    if (state.phase == ThreadPhase.failed) {
      state = state.withThread(phase: ThreadPhase.loading);
    }
    final seq = _beginRead();
    final ThreadPage<T> page;
    try {
      page = await scope.run(fetchPage());
    } on StaleScopeException {
      _endRead(seq);
      return true;
    } catch (_) {
      _endRead(seq);
      if (epoch != _epoch) return true;
      if (state.phase != ThreadPhase.ready) {
        state = state.withThread(phase: ThreadPhase.failed);
      }
      return false;
    }
    if (epoch != _epoch) {
      _endRead(seq);
      return true;
    }
    final tree = _reconcile(
      ThreadTree<T>(state.tree.codec).withRoots(page.items),
      seq,
    );
    _endRead(seq);
    _moreRun = null;
    _replyRuns.clear();
    state = firstPageState(state, tree, page);
    if (autoLoadReplies) {
      for (final item in page.items) {
        if (tree.codec.replyCount(item) > 0) {
          unawaited(loadReplies(tree.codec.id(item), skipEmpty: true));
        }
      }
    }
    return true;
  }

  Future<bool> loadMoreRoots() async {
    final token = rootsPageToken(state);
    if (token.isEmpty || _moreRun != null) return true;
    final scope = beginScope();
    final epoch = _epoch;
    final run = Object();
    _moreRun = run;
    final seq = _beginRead();
    try {
      final page = await scope.run(fetchPage(pageToken: token));
      if (epoch != _epoch) return true;
      final tree = _reconcile(state.tree.appendRoots(page.items), seq);
      state = withRootsPageToken(
        state.withThread(tree: tree),
        page.nextPageToken,
      );
      return true;
    } on StaleScopeException {
      return true;
    } catch (_) {
      return epoch != _epoch;
    } finally {
      _endRead(seq);
      if (identical(_moreRun, run)) _moreRun = null;
    }
  }

  /// 展開一個頂層項目的全部回覆；[skipEmpty] 時讀到空清單不動現有內容。
  Future<bool> loadReplies(String parentId, {bool skipEmpty = false}) async {
    if (_replyRuns.containsKey(parentId)) return true;
    final scope = beginScope();
    final epoch = _epoch;
    final run = Object();
    _replyRuns[parentId] = run;
    final seq = _beginRead();
    try {
      final all = <T>[];
      var token = '';
      do {
        final page = await scope.run(
          fetchPage(parentId: parentId, pageToken: token),
        );
        all.addAll(page.items);
        token = page.nextPageToken;
      } while (token.isNotEmpty);
      if (epoch != _epoch) return true;
      if (skipEmpty && all.isEmpty) return true;
      state = state.withThread(
        tree: _reconcile(state.tree.setReplies(parentId, all), seq),
      );
      return true;
    } on StaleScopeException {
      return true;
    } catch (_) {
      return epoch != _epoch;
    } finally {
      _endRead(seq);
      if (identical(_replyRuns[parentId], run)) _replyRuns.remove(parentId);
    }
  }

  void _log(ThreadTree<T> Function(ThreadTree<T>) apply) {
    final op = _Op<T>(++_opSeq, apply);
    if (_activeReads.isNotEmpty) _ops.add(op);
  }

  /// 伺服器已建立的新項目加進樹。
  void applyCreated(T item) {
    final parentId = state.tree.codec.parentId(item);
    final parent = parentId == null ? null : state.tree.byId[parentId];
    final floor =
        parent == null || state.tree.byId.containsKey(state.tree.codec.id(item))
        ? null
        : state.tree.codec.replyCount(parent) + 1;
    _log((t) => t.added(item, parentCountFloor: floor));
    state = state.withThread(tree: state.tree.added(item));
  }

  /// 伺服器回傳的完整項目取代樹裡同 id 的那一則。
  void applyReplaced(T item) {
    _log((t) => t.replaced(item));
    state = state.withThread(tree: state.tree.replaced(item));
  }

  /// 本地先刪；找不到 [id] 回 null。
  ThreadDeletion<T>? applyDelete(String id) {
    final removal = state.tree.removed(id);
    if (removal == null) return null;
    final op = _Op<T>(++_opSeq, (t) => t.removed(id)?.tree ?? t);
    if (_activeReads.isNotEmpty) _ops.add(op);
    state = state.withThread(tree: removal.tree);
    return ThreadDeletion<T>._(op, removal.undo);
  }

  void rollbackDelete(ThreadDeletion<T> deletion) {
    _ops.remove(deletion._op);
    state = state.withThread(tree: deletion._undo(state.tree));
  }
}
