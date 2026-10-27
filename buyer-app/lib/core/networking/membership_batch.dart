/// 批次成員檢查 RPC（`BatchCheckFavorites`、`BatchCheckFollows`）一次最多帶的 ID 數，
/// 見 `api/hawkr/buyer/v1alpha1/buyer.proto`。
const membershipBatchLimit = 100;

/// 把畫面上的 [ids] 去重後，以每批最多 [membershipBatchLimit] 個呼叫 [check]，
/// 合併每批回傳「有成員關係」的 ID。空清單不發請求。
///
/// 所有批次成員檢查共用這一份分批邏輯，不在各 repository 各寫一份。
Future<Set<T>> checkMembershipInBatches<T>(
  Iterable<T> ids,
  Future<Iterable<T>> Function(List<T> batch) check,
) async {
  final unique = ids.toSet().toList(growable: false);
  final matched = <T>{};
  for (var start = 0; start < unique.length; start += membershipBatchLimit) {
    final end = start + membershipBatchLimit < unique.length
        ? start + membershipBatchLimit
        : unique.length;
    matched.addAll(await check(unique.sublist(start, end)));
  }
  return matched;
}
