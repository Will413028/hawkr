import 'package:flutter/widgets.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import 'session_provider.dart';

/// 目前的觀看者；登入、登出與換帳號時才變，session 的其他變化不會通知。
final viewerIdProvider = Provider<String?>(
  (ref) => ref.watch(sessionProvider.select((s) => s.value?.userId)),
);

/// 依觀看者載入資料的 widget 共用機制。
///
/// - [beginLoad] 取得載入序號；任何一次非同步載入（首讀、重讀、分頁、回覆、貼文權限）回來後用
///   [isCurrentLoad] 檢查，序號對不上就丟棄。
/// - 觀看者換了：先把序號推進（所有進行中的載入作廢），再呼叫 [onViewerChanged]，
///   由 widget 同步清掉前一位觀看者的權限與狀態並重讀。
mixin ViewerScopedLoads<T extends StatefulWidget> on State<T> {
  /// 取得 Riverpod 的 ref；沒有時不訂閱觀看者。
  WidgetRef? get viewerRef;

  /// 觀看者已換：同步清掉屬於前一位觀看者的狀態，再重讀。
  void onViewerChanged();

  int _loadSeq = 0;
  ProviderSubscription<String?>? _viewerSub;

  int get currentLoad => _loadSeq;

  int beginLoad() => ++_loadSeq;

  bool isCurrentLoad(int seq) => mounted && seq == _loadSeq;

  void watchViewer() {
    _viewerSub = viewerRef?.listenManual(viewerIdProvider, (_, _) {
      if (!mounted) return;
      beginLoad();
      onViewerChanged();
    });
  }

  @override
  void dispose() {
    _viewerSub?.close();
    super.dispose();
  }
}
