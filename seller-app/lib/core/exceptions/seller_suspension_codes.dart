/// 賣家停權相關業務碼（46 號設計書「賣家能力權限層」§5/§7）。
///
/// 對應 Gateway envelope 的 `code` 欄位（業務碼塌縮進 HTTP 409 body，
/// 不是 HTTP status；見 [ApiException.code] / [AppException.fromApiError]）。
///
/// 9240 由 `dio_provider.dart` 的攔截器偵測，轉成 `onSellerSuspendedSignal`
/// （重讀 `GetMyStore`，由 router 依能力導向停權公告頁）。
abstract final class SellerSuspensionCode {
  /// 賣家操作被拒 — 賣家帳號處於停權狀態（能力層 deny）
  static const int sellerSuspendedOperationBlocked = 9240;

  /// 商品不可購買 — 賣家停權導致商品暫停販售（買家結帳校驗）
  static const int sellerSuspendedProductUnavailable = 9241;
}
