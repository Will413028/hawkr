//
//  Generated code. Do not modify.
//  source: hawkr/agent/v1alpha1/agent.proto
//

import "package:connectrpc/connect.dart" as connect;
import "agent.pb.dart" as hawkragentv1alpha1agent;
import "agent.connect.spec.dart" as specs;

/// VendorPortalPublicService 是持有邀請的供應商入口，不需要登入帳號。兩個方法都是 POST：憑證（邀請 secret）放在請求訊息，
/// 不進 URL（API 設計規範（Connect）§三），所以回應一律不可快取。兌換之後的方法在 VendorPortalInviteeService。
extension type VendorPortalPublicServiceClient (connect.Transport _transport) {
  /// PreviewInvite 回傳遮罩後的 email 與邀請的到期時間，讓供應商在兌換之前確認這是寄給自己的邀請；沒有副作用，也不發 session。
  /// 邀請不可用（不存在、secret 格式不對、已到期、已撤銷）一律回同一個 not_found，猜 secret 的人分不出原因。
  /// 與 ExchangeInvite 共用同一份額度：每個來源 5 分鐘最多 20 次，超過回 resource_exhausted（帶 RetryInfo）。
  Future<hawkragentv1alpha1agent.PreviewInviteResponse> previewInvite(
    hawkragentv1alpha1agent.PreviewInviteRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.VendorPortalPublicService.previewInvite,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// ExchangeInvite 以邀請 secret 取得這張邀請的 session，並在回應的 Set-Cookie 設上 session 的 cookie（名稱 __Host-hk_vp_<
  /// rfq_recipient_id>；HttpOnly、Secure、SameSite=Lax；Max-Age 是 session 到絕對期限的剩餘秒數，不超過邀請的到期時間）。
  /// 請求已帶這張邀請的有效 session 的 cookie 時沿用它：不發新的 session，回應是那個 session 目前的驗證狀態與 google_nonce；
  /// 沒有 cookie 或 cookie 無效時發新的 session（新的 Set-Cookie 蓋掉舊的）。邀請到期或撤銷之前可以重複兌換；同一張邀請同時
  /// 最多 5 個有效 session，超過時撤銷最舊的未驗證 session，全部都已驗證才撤銷最舊的，本次新發的不在撤銷之列。請求帶的邀請
  /// cookie 超過 20 個時，回應以 Max-Age=0 清掉多出的。錯誤與額度同 PreviewInvite。
  Future<hawkragentv1alpha1agent.ExchangeInviteResponse> exchangeInvite(
    hawkragentv1alpha1agent.ExchangeInviteRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.VendorPortalPublicService.exchangeInvite,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }
}
/// VendorPortalInviteeService 是已經兌換邀請的供應商（受邀供應商）：憑證是 ExchangeInvite 設的 cookie，沒有登入帳號。瀏覽器
/// 可以同時持有多張邀請的 cookie，每個方法以 rfq_recipient_id 指名要用哪一張（cookie 名稱含它）；沒有那張邀請的 cookie、
/// session 無效、已過期、已撤銷時回 unauthenticated。回應一律不可快取（private, no-store），讀取方法可以用 GET。
extension type VendorPortalInviteeServiceClient (connect.Transport _transport) {
  /// GetSession 回傳這張邀請的 session 目前的驗證狀態，以及要交給 Google 登入的 google_nonce。client 重新整理之後以它還原狀態。
  /// 每個來源每分鐘最多 300 次，超過回 resource_exhausted（帶 RetryInfo）。
  Future<hawkragentv1alpha1agent.GetSessionResponse> getSession(
    hawkragentv1alpha1agent.GetSessionRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.VendorPortalInviteeService.getSession,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// VerifySession 以 Google 登入證明供應商持有邀請的信箱（step-up）：google_id_token 由伺服器驗證，信箱以 Google 驗證過的
  /// 為準，不接受 client 自報；Google 帳號的信箱必須已經驗證，而且與邀請寄出的信箱相同（去前後空白、不分大小寫）。token 的
  /// nonce 必須是這個 session 的 google_nonce，iat 不得早於 10 分鐘前。驗證紀錄綁在 rfq_recipient_id 指名的 session。同一個
  /// session 以相同的信箱重送是冪等的成功。
  /// Google token 無效、nonce 不符、token 太舊、信箱未驗證、信箱不符、session 已用別的信箱驗證一律回同一個 permission_denied
  /// （同一段訊息），分不出原因。Google 的公鑰暫時問不到時回 unavailable，可以稍後重試。每個來源 10 分鐘最多 10 次，超過回
  /// resource_exhausted（帶 RetryInfo）。
  Future<hawkragentv1alpha1agent.VerifySessionResponse> verifySession(
    hawkragentv1alpha1agent.VerifySessionRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.VendorPortalInviteeService.verifySession,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }
}
