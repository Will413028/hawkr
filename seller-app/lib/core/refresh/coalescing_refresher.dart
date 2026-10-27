import 'dart:async';

import 'package:flutter/foundation.dart' show visibleForTesting;

/// 一類「重讀伺服器狀態」的合併規則：同一時間最多一輪、需要最新資料的呼叫排一輪接在後面、可帶保鮮期略過重讀。
///
/// 已有一輪進行中時，預設共用那一輪（回前景、切回分頁、下拉重整：沒有新的伺服器變更需要等）。`fresh` 為真表示
/// 呼叫端知道伺服器剛改變了狀態（推播、從別的流程回來）：進行中的那輪可能在變更之前就開始讀了，所以排一輪接在後面
/// 的重讀，進行期間所有 fresh 呼叫共用這一輪，結束時一定有一次在該呼叫之後才開始的讀取。後續那一輪在前一輪結束的
/// 同一刻開始，中間沒有空檔，不會有別的呼叫趁隙另開一輪。
///
/// `settledOnly` 是縮小讀取範圍的要求（只重讀已有結果的項目）。後續那一輪要滿足所有等它的呼叫，所以只有等它的 fresh
/// 呼叫全部都要求 `settledOnly` 時才縮小；只要有一個要完整重讀，後續那一輪就完整重讀。
///
/// `maxAge` 不是 null 時，沒有進行中的一輪、而且最近一次成功在這段時間內，就不讀（回傳 true）；進行中的一輪照樣共用。
/// 只給「因為成為可見而重讀」的呼叫用。
///
/// 一輪實際怎麼讀由 [round] 決定，並在讀成功時呼叫 [markSucceeded]（什麼算成功由讀的人決定，回傳值不一定等於成功）。
class CoalescingRefresher {
  CoalescingRefresher({
    required DateTime Function() clock,
    required Future<bool> Function(bool settledOnly) round,
  }) : _clock = clock,
       _round = round;

  final DateTime Function() _clock;
  final Future<bool> Function(bool settledOnly) _round;

  Future<bool>? _inFlight;
  Completer<bool>? _trailing;
  bool _trailingSettledOnly = true;
  DateTime? _lastSucceededAt;

  /// 現在有一輪在進行（包含 [adopt] 的那一輪）。正式程式碼靠 [refresh] 的合併行為，不查這個狀態；只有測試直接觀察。
  @visibleForTesting
  bool get isRunning => _inFlight != null;

  /// 最近一次成功已經超過 [maxAge]，或還沒有成功過。
  bool isStale(Duration maxAge) {
    final last = _lastSucceededAt;
    return last == null || _clock().difference(last) >= maxAge;
  }

  /// 記下最近一次成功的時間（現在）。
  void markSucceeded() => _lastSucceededAt = _clock();

  /// 收編一輪不是由 [refresh] 開始的讀取（例如 provider 第一次 build 的讀取），讓之後的呼叫共用它。
  void adopt(Future<bool> started) => _track(started);

  /// 一輪重讀，回傳 [round] 的結果。
  Future<bool> refresh({
    bool fresh = false,
    bool settledOnly = false,
    Duration? maxAge,
  }) {
    final running = _inFlight;
    if (running == null) {
      if (maxAge != null && !isStale(maxAge)) return Future.value(true);
      return _start(settledOnly);
    }
    if (!fresh) return running;
    var waiting = _trailing;
    if (waiting == null) {
      waiting = _trailing = Completer<bool>();
      _trailingSettledOnly = settledOnly;
    } else {
      _trailingSettledOnly = _trailingSettledOnly && settledOnly;
    }
    return waiting.future;
  }

  Future<bool> _start(bool settledOnly) {
    // 先登記這一輪再開始讀：[round] 同步執行的部分（例如觸發 provider 重建）就已經看得到「進行中」。
    final completer = Completer<bool>();
    final started = completer.future;
    _track(started);
    final Future<bool> running;
    try {
      running = _round(settledOnly);
    } catch (error, stackTrace) {
      // [round] 同步丟出時這一輪就此結束：失敗交給等這一輪的人，並清掉「進行中」，不留下永遠不會完成的 future。
      completer.completeError(error, stackTrace);
      return started;
    }
    running.then<void>(completer.complete, onError: completer.completeError);
    return started;
  }

  void _track(Future<bool> started) {
    _inFlight = started;
    started.then<void>(
      (_) => _finish(started),
      onError: (_) => _finish(started),
    );
  }

  void _finish(Future<bool> started) {
    if (!identical(_inFlight, started)) return;
    final waiting = _trailing;
    if (waiting == null) {
      _inFlight = null;
      return;
    }
    _trailing = null;
    waiting.complete(_start(_trailingSettledOnly));
  }
}
