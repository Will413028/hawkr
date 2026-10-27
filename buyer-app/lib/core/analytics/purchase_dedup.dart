/// Tracks orderNos already sent as `purchase` this app session, so a rebuild
/// or back-navigation to the success view does not double-count. Mirrors web's
/// sessionStorage dedup.
final Set<String> _trackedPurchases = <String>{};

/// Returns true the first time [orderNo] is seen this session.
bool markPurchaseTracked(String orderNo) => _trackedPurchases.add(orderNo);

/// Resets the dedup set. Only for use in tests.
void resetPurchaseDedupForTest() => _trackedPurchases.clear();
