import '../../../core/codemeta/code_meta.dart';
import 'order_model.dart';

// status 10=已付款, 20=待出貨 → 都進待出貨 tab
const pendingOrderStatuses = {10, 20};
const inTransitOrderStatuses = {30, 32, 35, 40}; // 40=已送達，待買家確認收貨
// 第三桶：50=已完成、60=部分退款、70=全額退款、80=已取消。
// ⚠️ 含 80/70，所以這一桶**不是成交數**，語意是「賣家無需再動作」→ 顯示名見 sellerOrderTabLabels。
const completedOrderStatuses = {50, 60, 70, 80};

/// 賣家訂單三個 tab 的顯示名（`OrderListScreen` 的 tab bar 與儀表板「訂單進度」共用）。
///
/// ⚠️ 第三個叫「已結束」而**不是**「已完成」：它含 80 已取消、70 全額退款
/// （見 [completedOrderStatuses]），叫「已完成」會讓賣家把它讀成成交數 ——
/// dev 實測同一家店 81 筆時這張卡顯示 75，其中只有 19 筆真的成交、56 筆是已取消。
/// 「已完成」是 CodeMeta `OrderStatus` 50 單一狀態的 Label（見 code_meta.dart），
/// 只在按狀態逐一計數的場景才成立。後端 SSOT = 03 §2.1；web 側對應
/// `features/seller-center/orders/orderTabs.ts` 的 `SELLER_ORDER_TAB_LABEL`。
///
/// 刻意與 [completedOrderStatuses] 放同一個檔案：桶的成員與桶的名字要一起被看到，
/// 否則下一個人只看到集合就會自行編一個名字（這正是本次 bug 的來源）。
const sellerOrderTabLabels = ['待出貨', '配送中', '已結束'];

/// 根據 tab index 回傳後端 `/app/orders?status=` 的 CSV 篩選字串。
///
/// 後端 API 支援逗號分隔多值。把 status 直接丟給後端可以避免 client-side
/// 篩選 + 分頁導致的舊訂單被「埋」在後面頁數看不到的問題。
///
/// Tab 名稱見 [sellerOrderTabLabels]（tab 2 = 「已結束」）。
///
/// Tab 1 (配送中) 特別包含 status=20 是為了配合 `tabForOrder` 的 tcat 覆寫：
/// 呼叫黑貓取件後 shipmentStatus='210' 但後端主狀態還沒同步到 30 的短暫期間，
/// 這些訂單 client-side 會被分到配送中，所以後端也要把它們撈回來。
/// 一般 status=20 的訂單會被 client-side tabForOrder 過濾掉不會重複顯示。
///
/// 回傳 null 代表不篩選（全抓）。
String? statusFilterForTab(int tabIndex) {
  final statuses = switch (tabIndex) {
    0 => pendingOrderStatuses,
    1 => {...inTransitOrderStatuses, 20},
    2 => completedOrderStatuses,
    _ => null,
  };
  return statuses?.join(',');
}

/// 回傳 seller 訂單列表要落在哪個 tab。
///
/// 0 = 待出貨
/// 1 = 配送中
/// 2 = 已結束（含已取消／已退款，非成交數；見 [sellerOrderTabLabels]）
/// -1 = 不顯示（例如待付款）
int tabForOrder(OrderSummary order) {
  final code = order.statusCode ?? int.tryParse(order.subtitle) ?? -1;

  if (completedOrderStatuses.contains(code)) return 2;
  if (inTransitOrderStatuses.contains(code)) return 1;

  // 宅配：呼叫黑貓後 shipmentStatus='210' 時，後端主狀態可能尚未同步到 30，
  // 這段期間先暫時歸到「配送中」避免賣家看不到訂單。
  if (order.shipmentStatus == ShippingStatus.tcatCalled.value &&
      order.shippingMethod == ShippingMethod.taqbin.value) {
    return 1;
  }

  if (pendingOrderStatuses.contains(code)) return 0;
  return -1;
}
