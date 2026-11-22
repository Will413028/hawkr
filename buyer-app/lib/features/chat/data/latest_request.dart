/// 同一類請求只認最後發出的那一個：較早發出、較晚回來的結果被丟棄，不蓋掉較新的。
class LatestRequest {
  int _issued = 0;

  /// 執行 [task]；回來時若已有更新的請求發出，回傳 `(superseded: true, value: null)`，
  /// 失敗也一樣視為過時而吞掉（丟出 [Object] 之前先檢查）。
  Future<({bool superseded, T? value})> run<T>(
    Future<T> Function() task,
  ) async {
    final ticket = ++_issued;
    try {
      final value = await task();
      return (superseded: ticket != _issued, value: value);
    } catch (_) {
      if (ticket != _issued) return (superseded: true, value: null);
      rethrow;
    }
  }
}
