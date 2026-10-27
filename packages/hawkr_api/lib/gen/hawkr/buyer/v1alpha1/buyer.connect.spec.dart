//
//  Generated code. Do not modify.
//  source: hawkr/buyer/v1alpha1/buyer.proto
//

import "package:connectrpc/connect.dart" as connect;
import "buyer.pb.dart" as hawkrbuyerv1alpha1buyer;

/// BuyerAppService 是登入的帳號的方法。以呼叫者的買家身分操作，帳號沒有買家角色時回 permission_denied。
abstract final class BuyerAppService {
  /// Fully-qualified name of the BuyerAppService service.
  static const name = 'hawkr.buyer.v1alpha1.BuyerAppService';

  /// GetProfile 回傳呼叫者的買家 profile。
  static const getProfile = connect.Spec(
    '/$name/GetProfile',
    connect.StreamType.unary,
    hawkrbuyerv1alpha1buyer.GetProfileRequest.new,
    hawkrbuyerv1alpha1buyer.GetProfileResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// UpdateProfile 修改 profile 的欄位（update_mask），含頭像（avatar_staged_token）。etag 必填；換頭像與文字欄位在
  /// 同一個版本條件下一起寫入，任何一項失敗都不會寫入；沒有任何欄位改變時不寫入，etag 不變。
  static const updateProfile = connect.Spec(
    '/$name/UpdateProfile',
    connect.StreamType.unary,
    hawkrbuyerv1alpha1buyer.UpdateProfileRequest.new,
    hawkrbuyerv1alpha1buyer.UpdateProfileResponse.new,
  );

  /// ListAddresses 列出收貨地址，預設地址在最前，其餘依修改時間新到舊。每個買家最多 20 筆（見 CreateAddress），
  /// 預設頁大小等於上限，所以通常一頁就是完整清單；client 仍要照 next_page_token 讀到底，上限調整時不會被截斷。
  static const listAddresses = connect.Spec(
    '/$name/ListAddresses',
    connect.StreamType.unary,
    hawkrbuyerv1alpha1buyer.ListAddressesRequest.new,
    hawkrbuyerv1alpha1buyer.ListAddressesResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// GetAddress 回傳一筆收貨地址。
  static const getAddress = connect.Spec(
    '/$name/GetAddress',
    connect.StreamType.unary,
    hawkrbuyerv1alpha1buyer.GetAddressRequest.new,
    hawkrbuyerv1alpha1buyer.GetAddressResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// CreateAddress 新增收貨地址。第一筆地址自動成為預設地址。每個買家最多 20 筆：已有 20 筆時回
  /// failed_precondition（帶 LocalizedMessage），要先刪掉一筆。
  static const createAddress = connect.Spec(
    '/$name/CreateAddress',
    connect.StreamType.unary,
    hawkrbuyerv1alpha1buyer.CreateAddressRequest.new,
    hawkrbuyerv1alpha1buyer.CreateAddressResponse.new,
  );

  /// UpdateAddress 修改收貨地址。default_address 設為 true 時取代原本的預設地址；有地址時一定有一筆預設，
  /// 所以不能把預設地址改成 false，要改選另一筆。
  static const updateAddress = connect.Spec(
    '/$name/UpdateAddress',
    connect.StreamType.unary,
    hawkrbuyerv1alpha1buyer.UpdateAddressRequest.new,
    hawkrbuyerv1alpha1buyer.UpdateAddressResponse.new,
  );

  /// DeleteAddress 刪除收貨地址。刪掉的是預設地址時，最近修改的另一筆成為預設。
  static const deleteAddress = connect.Spec(
    '/$name/DeleteAddress',
    connect.StreamType.unary,
    hawkrbuyerv1alpha1buyer.DeleteAddressRequest.new,
    hawkrbuyerv1alpha1buyer.DeleteAddressResponse.new,
  );

  /// ListFavorites 列出收藏的商品，新到舊。
  static const listFavorites = connect.Spec(
    '/$name/ListFavorites',
    connect.StreamType.unary,
    hawkrbuyerv1alpha1buyer.ListFavoritesRequest.new,
    hawkrbuyerv1alpha1buyer.ListFavoritesResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// CheckFavorite 回傳商品是否已收藏。
  static const checkFavorite = connect.Spec(
    '/$name/CheckFavorite',
    connect.StreamType.unary,
    hawkrbuyerv1alpha1buyer.CheckFavoriteRequest.new,
    hawkrbuyerv1alpha1buyer.CheckFavoriteResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// BatchCheckFavorites 回傳畫面上已收藏的商品 ID；一次最多 100 個。
  static const batchCheckFavorites = connect.Spec(
    '/$name/BatchCheckFavorites',
    connect.StreamType.unary,
    hawkrbuyerv1alpha1buyer.BatchCheckFavoritesRequest.new,
    hawkrbuyerv1alpha1buyer.BatchCheckFavoritesResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// AddFavorite 收藏商品；已收藏時也成功。帳號在刪除流程中（等待訂單完成）時回 failed_precondition。
  static const addFavorite = connect.Spec(
    '/$name/AddFavorite',
    connect.StreamType.unary,
    hawkrbuyerv1alpha1buyer.AddFavoriteRequest.new,
    hawkrbuyerv1alpha1buyer.AddFavoriteResponse.new,
    idempotency: connect.Idempotency.idempotent,
  );

  /// RemoveFavorite 取消收藏；沒有收藏時也成功。
  static const removeFavorite = connect.Spec(
    '/$name/RemoveFavorite',
    connect.StreamType.unary,
    hawkrbuyerv1alpha1buyer.RemoveFavoriteRequest.new,
    hawkrbuyerv1alpha1buyer.RemoveFavoriteResponse.new,
    idempotency: connect.Idempotency.idempotent,
  );

  /// ListFollows 列出追蹤的店家，新到舊。
  static const listFollows = connect.Spec(
    '/$name/ListFollows',
    connect.StreamType.unary,
    hawkrbuyerv1alpha1buyer.ListFollowsRequest.new,
    hawkrbuyerv1alpha1buyer.ListFollowsResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// CheckFollow 回傳是否已追蹤店家。
  static const checkFollow = connect.Spec(
    '/$name/CheckFollow',
    connect.StreamType.unary,
    hawkrbuyerv1alpha1buyer.CheckFollowRequest.new,
    hawkrbuyerv1alpha1buyer.CheckFollowResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// BatchCheckFollows 回傳畫面上已追蹤的店家 ID；一次最多 100 個。
  static const batchCheckFollows = connect.Spec(
    '/$name/BatchCheckFollows',
    connect.StreamType.unary,
    hawkrbuyerv1alpha1buyer.BatchCheckFollowsRequest.new,
    hawkrbuyerv1alpha1buyer.BatchCheckFollowsResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// FollowStore 追蹤店家；已追蹤時也成功。店家不存在或不公開時回 not_found，追蹤自己的店回 invalid_argument，
  /// 帳號在刪除流程中時回 failed_precondition。
  static const followStore = connect.Spec(
    '/$name/FollowStore',
    connect.StreamType.unary,
    hawkrbuyerv1alpha1buyer.FollowStoreRequest.new,
    hawkrbuyerv1alpha1buyer.FollowStoreResponse.new,
    idempotency: connect.Idempotency.idempotent,
  );

  /// UnfollowStore 取消追蹤；沒有追蹤時也成功。
  static const unfollowStore = connect.Spec(
    '/$name/UnfollowStore',
    connect.StreamType.unary,
    hawkrbuyerv1alpha1buyer.UnfollowStoreRequest.new,
    hawkrbuyerv1alpha1buyer.UnfollowStoreResponse.new,
    idempotency: connect.Idempotency.idempotent,
  );

  /// ListMemberCards 列出買家消費過的店家與累計消費額（會員卡），消費額高到低。
  static const listMemberCards = connect.Spec(
    '/$name/ListMemberCards',
    connect.StreamType.unary,
    hawkrbuyerv1alpha1buyer.ListMemberCardsRequest.new,
    hawkrbuyerv1alpha1buyer.ListMemberCardsResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// ListSpendingRecords 列出買家在一個店家的消費紀錄，新到舊。沒有紀錄時回空的清單。
  static const listSpendingRecords = connect.Spec(
    '/$name/ListSpendingRecords',
    connect.StreamType.unary,
    hawkrbuyerv1alpha1buyer.ListSpendingRecordsRequest.new,
    hawkrbuyerv1alpha1buyer.ListSpendingRecordsResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// ListVisits 列出瀏覽過的商品，依最後瀏覽時間新到舊。
  static const listVisits = connect.Spec(
    '/$name/ListVisits',
    connect.StreamType.unary,
    hawkrbuyerv1alpha1buyer.ListVisitsRequest.new,
    hawkrbuyerv1alpha1buyer.ListVisitsResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// RecordVisit 記錄一次商品瀏覽；每次呼叫都算一次瀏覽，client 不自動重送。
  static const recordVisit = connect.Spec(
    '/$name/RecordVisit',
    connect.StreamType.unary,
    hawkrbuyerv1alpha1buyer.RecordVisitRequest.new,
    hawkrbuyerv1alpha1buyer.RecordVisitResponse.new,
  );

  /// ClearVisits 清除全部瀏覽紀錄。
  static const clearVisits = connect.Spec(
    '/$name/ClearVisits',
    connect.StreamType.unary,
    hawkrbuyerv1alpha1buyer.ClearVisitsRequest.new,
    hawkrbuyerv1alpha1buyer.ClearVisitsResponse.new,
    idempotency: connect.Idempotency.idempotent,
  );

  /// ListRestockSubscriptions 列出等待補貨通知的商品，新到舊。通知送出或取消之後就不在清單上。
  static const listRestockSubscriptions = connect.Spec(
    '/$name/ListRestockSubscriptions',
    connect.StreamType.unary,
    hawkrbuyerv1alpha1buyer.ListRestockSubscriptionsRequest.new,
    hawkrbuyerv1alpha1buyer.ListRestockSubscriptionsResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// SubscribeRestock 訂閱商品的補貨通知；已訂閱時也成功。
  static const subscribeRestock = connect.Spec(
    '/$name/SubscribeRestock',
    connect.StreamType.unary,
    hawkrbuyerv1alpha1buyer.SubscribeRestockRequest.new,
    hawkrbuyerv1alpha1buyer.SubscribeRestockResponse.new,
    idempotency: connect.Idempotency.idempotent,
  );

  /// UnsubscribeRestock 取消補貨通知；沒有訂閱時也成功。
  static const unsubscribeRestock = connect.Spec(
    '/$name/UnsubscribeRestock',
    connect.StreamType.unary,
    hawkrbuyerv1alpha1buyer.UnsubscribeRestockRequest.new,
    hawkrbuyerv1alpha1buyer.UnsubscribeRestockResponse.new,
    idempotency: connect.Idempotency.idempotent,
  );
}
