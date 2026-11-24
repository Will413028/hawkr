//
//  Generated code. Do not modify.
//  source: hawkr/coupon/v1alpha1/coupon.proto
//

import "package:connectrpc/connect.dart" as connect;
import "coupon.pb.dart" as hawkrcouponv1alpha1coupon;

/// CouponAppService 是登入帳號的優惠券。
abstract final class CouponAppService {
  /// Fully-qualified name of the CouponAppService service.
  static const name = 'hawkr.coupon.v1alpha1.CouponAppService';

  /// ListMyVouchers 列出呼叫者（買家）持有的券，新到舊。statuses 為空是全部狀態；到期的券仍是 AVAILABLE，以 expired 呈現。
  /// 帶 checkout 時是結帳情境：statuses 必須為空或只含 AVAILABLE，只回這筆訂單現在用得上的券（與鎖券同一組檢查：狀態、時間窗、店家停權、資格、店家範圍、門檻、折抵大於 0），
  /// 每張附 estimated_discount。page_token 只能用在同一組 statuses 與 checkout。
  static const listMyVouchers = connect.Spec(
    '/$name/ListMyVouchers',
    connect.StreamType.unary,
    hawkrcouponv1alpha1coupon.ListMyVouchersRequest.new,
    hawkrcouponv1alpha1coupon.ListMyVouchersResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// ListClaimableCampaigns 列出呼叫者現在領得到的活動，新到舊：進行中、在時間窗內、還有名額、呼叫者符合資格、而且還沒有這個活動的券（與 ClaimCoupon 同一個資格判定）。
  /// store_id 有值時只列該店與全平台的活動。page_token 只能用在同一個 store_id。
  static const listClaimableCampaigns = connect.Spec(
    '/$name/ListClaimableCampaigns',
    connect.StreamType.unary,
    hawkrcouponv1alpha1coupon.ListClaimableCampaignsRequest.new,
    hawkrcouponv1alpha1coupon.ListClaimableCampaignsResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// ClaimCoupon 領取活動的券，回傳領到的券。已經領過回既有的那張（冪等，不再計數）。活動不存在回 not_found；不是進行中、不在時間窗、發行店家停權、
  /// 不符合資格回 failed_precondition（reason 見 ErrorReason 的 COUPON_*）；領完了回 failed_precondition 與 COUPON_ISSUE_LIMIT_REACHED；
  /// 曾經領過但券已被作廢回 failed_precondition。
  static const claimCoupon = connect.Spec(
    '/$name/ClaimCoupon',
    connect.StreamType.unary,
    hawkrcouponv1alpha1coupon.ClaimCouponRequest.new,
    hawkrcouponv1alpha1coupon.ClaimCouponResponse.new,
    idempotency: connect.Idempotency.idempotent,
  );

  /// CreateCampaign 以呼叫者此刻的店建立活動，直接生效（ACTIVE）。需要賣家能力 CAP_MARKETING。每家店同時進行的活動數有上限（預設 5），超過回
  /// failed_precondition 與 COUPON_QUOTA_EXCEEDED。DESIGNATED 的名單不得為空、不得含呼叫者自己的買家身分。欄位不合法回 invalid_argument 並指出欄位。
  static const createCampaign = connect.Spec(
    '/$name/CreateCampaign',
    connect.StreamType.unary,
    hawkrcouponv1alpha1coupon.CreateCampaignRequest.new,
    hawkrcouponv1alpha1coupon.CreateCampaignResponse.new,
  );

  /// ListCampaigns 列出呼叫者此刻的店自己發的活動，新到舊（不含系統建立的補償活動）。需要賣家能力 CAP_MARKETING（唯讀也要）。
  /// 回應附店家的活動配額與同時進行中的活動數。page_token 只能用在同一個 status。
  static const listCampaigns = connect.Spec(
    '/$name/ListCampaigns',
    connect.StreamType.unary,
    hawkrcouponv1alpha1coupon.ListCampaignsRequest.new,
    hawkrcouponv1alpha1coupon.ListCampaignsResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// GetCampaign 讀店家自己的活動：名單（DESIGNATED）與各狀態的券數。需要賣家能力 CAP_MARKETING。不存在與不是這家店發的一律 not_found。
  static const getCampaign = connect.Spec(
    '/$name/GetCampaign',
    connect.StreamType.unary,
    hawkrcouponv1alpha1coupon.GetCampaignRequest.new,
    hawkrcouponv1alpha1coupon.GetCampaignResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// EndCampaign 提前結束店家自己的活動：擋掉新的領取與鎖定，作廢還沒被使用的券（AVAILABLE），已被訂單鎖定的券照常核銷或釋放。收縮型動作，
  /// 需要賣家能力 CAP_CONTENT_CONTRACT（停權的店家仍可結束自己的活動）。已經結束的重送成功。不存在與不是這家店發的一律 not_found。
  static const endCampaign = connect.Spec(
    '/$name/EndCampaign',
    connect.StreamType.unary,
    hawkrcouponv1alpha1coupon.EndCampaignRequest.new,
    hawkrcouponv1alpha1coupon.EndCampaignResponse.new,
    idempotency: connect.Idempotency.idempotent,
  );
}
