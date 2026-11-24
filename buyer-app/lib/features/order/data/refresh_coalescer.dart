import 'dart:async';

/// 狀態重讀訊號的合併窗：窗內來的多次要求（同一事件的串流訊號與前景推播提示、一次推播帶多個提示）
/// 只在窗結束時送出一次。斷線與背景時只有其中一條路徑送訊號，同樣會送出，不會漏。
class RefreshCoalescer {
  RefreshCoalescer(this._emit, {this.window = defaultWindow});

  final void Function() _emit;
  static const defaultWindow = Duration(milliseconds: 300);

  Duration window;
  Timer? _pending;

  void request() {
    if (_pending != null) return;
    _pending = Timer(window, () {
      _pending = null;
      _emit();
    });
  }

  void dispose() {
    _pending?.cancel();
    _pending = null;
  }
}
