//
//  Generated code. Do not modify.
//  source: hawkr/identity/v1alpha1/identity.proto
//

import "package:connectrpc/connect.dart" as connect;
import "identity.pb.dart" as hawkridentityv1alpha1identity;

/// IdentityPublicService 是登入之前的方法：App 把原生 SDK 取得的 ID token 換成 session。兩個方法都是 POST，回應一律不可快取。
/// 每個來源 IP 有額度，超過回 resource_exhausted（帶 RetryInfo）。
abstract final class IdentityPublicService {
  /// Fully-qualified name of the IdentityPublicService service.
  static const name = 'hawkr.identity.v1alpha1.IdentityPublicService';

  /// IssueAppLoginNonce 發一個單次使用、5 分鐘內有效的 nonce：App 把它交給原生 SDK（Google 的 nonce、LINE 的 nonce），
  /// SDK 簽發的 ID token 的 nonce claim 必須等於它，再交給 ExchangeIdToken 或 LinkIdentity。
  static const issueAppLoginNonce = connect.Spec(
    '/$name/IssueAppLoginNonce',
    connect.StreamType.unary,
    hawkridentityv1alpha1identity.IssueAppLoginNonceRequest.new,
    hawkridentityv1alpha1identity.IssueAppLoginNonceResponse.new,
  );

  /// ExchangeIdToken 以原生 SDK 取得的 ID token 登入並換發 session（App 種類，閒置 14 天、絕對 90 天）：驗證簽章、issuer、audience、
  /// nonce（必須是 IssueAppLoginNonce 發的、尚未用過）；Google 另外要求 azp 在允許的 App client 清單。已有這個身分就登入它的帳號，
  /// 否則依規則自動連結或建立帳號（ADR 同上 D4）。請求已帶 Authorization: Bearer 的舊 session 時一併撤銷。
  /// ID token 或 nonce 無效回 unauthenticated；身分屬於已刪除的帳號也回 unauthenticated；這個 provider 沒有設定回 unavailable。
  static const exchangeIdToken = connect.Spec(
    '/$name/ExchangeIdToken',
    connect.StreamType.unary,
    hawkridentityv1alpha1identity.ExchangeIdTokenRequest.new,
    hawkridentityv1alpha1identity.ExchangeIdTokenResponse.new,
  );
}
/// IdentityAppService 是登入的買家、賣家的方法。
abstract final class IdentityAppService {
  /// Fully-qualified name of the IdentityAppService service.
  static const name = 'hawkr.identity.v1alpha1.IdentityAppService';

  /// GetSession 回傳目前登入的帳號；web 以它判斷是否仍在登入。
  static const getSession = connect.Spec(
    '/$name/GetSession',
    connect.StreamType.unary,
    hawkridentityv1alpha1identity.GetSessionRequest.new,
    hawkridentityv1alpha1identity.GetSessionResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// LinkAppleCredential 交出 Sign in with Apple 的憑證（App Store 審核準則 5.1.1(v)）：App 每次以 Apple 原生登入成功後呼叫。
  /// 伺服器驗證 identity token（簽章、issuer、audience），之後在背景以 authorization code 換成 refresh token；Apple 換 token
  /// 簽發的 ID token 必須與這個 identity token 是同一個身分才加密保存（取代舊的），成功只表示已經排入。帳號最終刪除時
  /// 伺服器自動撤銷保存的 token。可以安全重送。identity token 無效時回 invalid_argument；Apple 沒有設定（本機開發）時回
  /// unavailable。
  static const linkAppleCredential = connect.Spec(
    '/$name/LinkAppleCredential',
    connect.StreamType.unary,
    hawkridentityv1alpha1identity.LinkAppleCredentialRequest.new,
    hawkridentityv1alpha1identity.LinkAppleCredentialResponse.new,
    idempotency: connect.Idempotency.idempotent,
  );

  /// SignOut 撤銷目前的 session。web 的回應另以 Set-Cookie 清掉 `__Host-hk_session`。session 已經不存在時也成功。停權的帳號也能呼叫。
  static const signOut = connect.Spec(
    '/$name/SignOut',
    connect.StreamType.unary,
    hawkridentityv1alpha1identity.SignOutRequest.new,
    hawkridentityv1alpha1identity.SignOutResponse.new,
  );

  /// LinkIdentity 把 App 以原生 SDK 取得的 Google 或 LINE 身分連結到呼叫者的帳號（web 走 GET /api/auth/{provider}/start?link=1）。
  /// ID token 與 nonce 的驗證同 IdentityPublicService.ExchangeIdToken，不換發 session。這個身分已連結到別的帳號（含已刪除帳號
  /// 留下的身分）回 already_exists 與 ErrorReason IDENTITY_ALREADY_LINKED，什麼都不改；已連結到呼叫者自己時成功。
  static const linkIdentity = connect.Spec(
    '/$name/LinkIdentity',
    connect.StreamType.unary,
    hawkridentityv1alpha1identity.LinkIdentityRequest.new,
    hawkridentityv1alpha1identity.LinkIdentityResponse.new,
  );

  /// ListSessions 列出呼叫者的有效 session（新到舊），標出目前這一個。停權的帳號也能呼叫。
  static const listSessions = connect.Spec(
    '/$name/ListSessions',
    connect.StreamType.unary,
    hawkridentityv1alpha1identity.ListSessionsRequest.new,
    hawkridentityv1alpha1identity.ListSessionsResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// ListLinkedIdentities 列出呼叫者的帳號已連結的登入身分。同一 provider 可能有多筆；client 依 provider 判斷是否已連結。
  /// 不回 provider 的使用者識別與 email。停權的帳號也能呼叫。
  static const listLinkedIdentities = connect.Spec(
    '/$name/ListLinkedIdentities',
    connect.StreamType.unary,
    hawkridentityv1alpha1identity.ListLinkedIdentitiesRequest.new,
    hawkridentityv1alpha1identity.ListLinkedIdentitiesResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// RevokeSession 終止呼叫者自己的一個 session（包含目前這個）。目前的 session 必須在 15 分鐘內以 provider 驗證過
  /// （重新走一次登入），否則回 failed_precondition 與 ErrorReason REAUTHENTICATION_REQUIRED，client 引導重新登入後再送。
  /// 沒有這個 session 或屬於別的帳號回 not_found。停權的帳號也能呼叫。
  static const revokeSession = connect.Spec(
    '/$name/RevokeSession',
    connect.StreamType.unary,
    hawkridentityv1alpha1identity.RevokeSessionRequest.new,
    hawkridentityv1alpha1identity.RevokeSessionResponse.new,
  );

  /// DeleteAccount 刪除呼叫者的帳號（App Store 審核準則 5.1.1(v)）。沒有未完成的訂單時立即刪除；有的話進入刪除流程，
  /// 最後一筆訂單完成後自動刪除，期間不能新增收藏與追蹤；流程中再呼叫回目前的狀態。帳號有店家時店家是
  /// 參與者：同一個 transaction 關閉買家、店家並刪除帳號，店家有未結訂單時與買家一起等待到兩側都結清。保存的 Apple 授權由伺服器在最後刪除時撤銷，client 不需要任何前置步驟。
  /// 刪除之後 token 不再有效，所以 client 不自動重送：沒收到回應時以 GetSession 確認，unauthenticated 表示已刪除。
  static const deleteAccount = connect.Spec(
    '/$name/DeleteAccount',
    connect.StreamType.unary,
    hawkridentityv1alpha1identity.DeleteAccountRequest.new,
    hawkridentityv1alpha1identity.DeleteAccountResponse.new,
  );
}
