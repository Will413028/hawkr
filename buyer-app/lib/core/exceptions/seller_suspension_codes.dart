/// 賣家停權相關業務碼（46 號設計書「賣家能力權限層」§5/§7）。
///
/// 對應 Gateway envelope 的 `code` 欄位（業務碼塌縮進 HTTP 409 body，
/// 不是 HTTP status）；本 app 慣例以 `data['code']?.toString()` 比對字串
/// （見 cart_repository.dart 的 '4015' / '6006' 用法），故此處採 String 型別。
///
/// 目前僅定義常數，尚無 caller — 分流 UI（結帳 9241 toast / 購物車不可結帳
/// 標示等）待使用者圈選範圍後才接線。
abstract final class SellerSuspensionCode {
  /// 賣家操作被拒 — 賣家帳號處於停權狀態（能力層 deny）
  static const String sellerSuspendedOperationBlocked = '9240';

  /// 商品不可購買 — 賣家停權導致商品暫停販售（買家結帳校驗）
  static const String sellerSuspendedProductUnavailable = '9241';
}
