import 'package:hooks_riverpod/hooks_riverpod.dart';

import 'viewer_scope.dart';

/// 一次非同步操作開始時擷取的範圍；回來時範圍已不屬於同一代，結果必須丟棄。
class StaleScopeException implements Exception {
  const StaleScopeException();

  @override
  String toString() => 'StaleScopeException';
}

/// [ViewerScoped.beginScope] 擷取的世代。
class ViewerScope {
  ViewerScope._(this._owner, this._generation);

  final ViewerScoped<Object?, Object?> _owner;
  final int _generation;

  /// 擁有者沒有被銷毀，也沒有在擷取之後重建（觀看者換了或重新整理）。
  bool get isCurrent => _owner._mounted && _owner._generation == _generation;

  /// 等 [future]；完成或失敗之後範圍已過期就丟 [StaleScopeException]，不讓舊結果或舊錯誤被當成現況。
  Future<T> run<T>(Future<T> future) async {
    try {
      final value = await future;
      if (!isCurrent) throw const StaleScopeException();
      return value;
    } catch (_) {
      if (!isCurrent) throw const StaleScopeException();
      rethrow;
    }
  }
}

/// 以觀看者為範圍的 notifier 共用機制。
///
/// `build` 開頭呼叫 [bindViewer]：訂閱 [viewerIdProvider]（觀看者換了就重建）並推進世代。
/// 每個跨 await 的路徑（讀取、分頁、mutation、樂觀回滾）都用 [beginScope] 擷取範圍、
/// 以 [ViewerScope.run] 等待；世代對不上就丟棄。`ref.mounted` 在重建後仍為 true，不能拿來判斷過期。
mixin ViewerScoped<StateT, ValueT> on AnyNotifier<StateT, ValueT> {
  int _generation = 0;

  bool get _mounted => ref.mounted;

  /// 目前的觀看者；訪客是 null。
  String? bindViewer() {
    _generation++;
    return ref.watch(viewerIdProvider);
  }

  ViewerScope beginScope() => ViewerScope._(this, _generation);
}
