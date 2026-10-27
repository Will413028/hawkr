//
//  Generated code. Do not modify.
//  source: hawkr/identity/v1alpha1/identity.proto
//

import "package:connectrpc/connect.dart" as connect;
import "identity.pb.dart" as hawkridentityv1alpha1identity;

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

  /// DeleteAccount 刪除呼叫者的帳號（App Store 審核準則 5.1.1(v)）。沒有未完成的訂單時立即刪除；有的話進入刪除流程，
  /// 最後一筆訂單完成後自動刪除，期間不能新增收藏與追蹤；流程中再呼叫回目前的狀態。帳號還有沒關閉的店家時回
  /// failed_precondition，reason 是 ACCOUNT_HAS_OPEN_STORE：要先關閉店家；其他 failed_precondition 不帶這個 reason。保存的 Apple 授權由伺服器在最後刪除時撤銷，client 不需要任何前置步驟。
  /// 刪除之後 token 不再有效，所以 client 不自動重送：沒收到回應時以 GetSession 確認，unauthenticated 表示已刪除。
  static const deleteAccount = connect.Spec(
    '/$name/DeleteAccount',
    connect.StreamType.unary,
    hawkridentityv1alpha1identity.DeleteAccountRequest.new,
    hawkridentityv1alpha1identity.DeleteAccountResponse.new,
  );
}
