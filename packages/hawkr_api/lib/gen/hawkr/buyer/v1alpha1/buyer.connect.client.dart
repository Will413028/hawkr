//
//  Generated code. Do not modify.
//  source: hawkr/buyer/v1alpha1/buyer.proto
//

import "package:connectrpc/connect.dart" as connect;
import "buyer.pb.dart" as hawkrbuyerv1alpha1buyer;
import "buyer.connect.spec.dart" as specs;

/// BuyerAppService 是登入的帳號的方法。以呼叫者的買家身分操作，帳號沒有買家角色時回 permission_denied。
extension type BuyerAppServiceClient (connect.Transport _transport) {
  /// GetProfile 回傳呼叫者的買家 profile。
  Future<hawkrbuyerv1alpha1buyer.GetProfileResponse> getProfile(
    hawkrbuyerv1alpha1buyer.GetProfileRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.BuyerAppService.getProfile,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// UpdateProfile 修改 profile 的欄位（update_mask），含頭像（avatar_staged_token）。etag 必填；換頭像與文字欄位在
  /// 同一個版本條件下一起寫入，任何一項失敗都不會寫入；沒有任何欄位改變時不寫入，etag 不變。
  Future<hawkrbuyerv1alpha1buyer.UpdateProfileResponse> updateProfile(
    hawkrbuyerv1alpha1buyer.UpdateProfileRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.BuyerAppService.updateProfile,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// ListAddresses 列出收貨地址，預設地址在最前，其餘依修改時間新到舊。每個買家最多 20 筆（見 CreateAddress），
  /// 預設頁大小等於上限，所以通常一頁就是完整清單；client 仍要照 next_page_token 讀到底，上限調整時不會被截斷。
  Future<hawkrbuyerv1alpha1buyer.ListAddressesResponse> listAddresses(
    hawkrbuyerv1alpha1buyer.ListAddressesRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.BuyerAppService.listAddresses,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// GetAddress 回傳一筆收貨地址。
  Future<hawkrbuyerv1alpha1buyer.GetAddressResponse> getAddress(
    hawkrbuyerv1alpha1buyer.GetAddressRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.BuyerAppService.getAddress,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// CreateAddress 新增收貨地址。第一筆地址自動成為預設地址。每個買家最多 20 筆：已有 20 筆時回
  /// failed_precondition（帶 LocalizedMessage），要先刪掉一筆。
  Future<hawkrbuyerv1alpha1buyer.CreateAddressResponse> createAddress(
    hawkrbuyerv1alpha1buyer.CreateAddressRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.BuyerAppService.createAddress,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// UpdateAddress 修改收貨地址。default_address 設為 true 時取代原本的預設地址；有地址時一定有一筆預設，
  /// 所以不能把預設地址改成 false，要改選另一筆。
  Future<hawkrbuyerv1alpha1buyer.UpdateAddressResponse> updateAddress(
    hawkrbuyerv1alpha1buyer.UpdateAddressRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.BuyerAppService.updateAddress,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// DeleteAddress 刪除收貨地址。刪掉的是預設地址時，最近修改的另一筆成為預設。
  Future<hawkrbuyerv1alpha1buyer.DeleteAddressResponse> deleteAddress(
    hawkrbuyerv1alpha1buyer.DeleteAddressRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.BuyerAppService.deleteAddress,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// ListFavorites 列出收藏的商品，新到舊。
  Future<hawkrbuyerv1alpha1buyer.ListFavoritesResponse> listFavorites(
    hawkrbuyerv1alpha1buyer.ListFavoritesRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.BuyerAppService.listFavorites,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// CheckFavorite 回傳商品是否已收藏。
  Future<hawkrbuyerv1alpha1buyer.CheckFavoriteResponse> checkFavorite(
    hawkrbuyerv1alpha1buyer.CheckFavoriteRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.BuyerAppService.checkFavorite,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// BatchCheckFavorites 回傳畫面上已收藏的商品 ID；一次最多 100 個。
  Future<hawkrbuyerv1alpha1buyer.BatchCheckFavoritesResponse> batchCheckFavorites(
    hawkrbuyerv1alpha1buyer.BatchCheckFavoritesRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.BuyerAppService.batchCheckFavorites,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// AddFavorite 收藏商品；已收藏時也成功。帳號在刪除流程中（等待訂單完成）時回 failed_precondition。
  Future<hawkrbuyerv1alpha1buyer.AddFavoriteResponse> addFavorite(
    hawkrbuyerv1alpha1buyer.AddFavoriteRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.BuyerAppService.addFavorite,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// RemoveFavorite 取消收藏；沒有收藏時也成功。
  Future<hawkrbuyerv1alpha1buyer.RemoveFavoriteResponse> removeFavorite(
    hawkrbuyerv1alpha1buyer.RemoveFavoriteRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.BuyerAppService.removeFavorite,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// ListFollows 列出追蹤的店家，新到舊。
  Future<hawkrbuyerv1alpha1buyer.ListFollowsResponse> listFollows(
    hawkrbuyerv1alpha1buyer.ListFollowsRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.BuyerAppService.listFollows,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// CheckFollow 回傳是否已追蹤店家。
  Future<hawkrbuyerv1alpha1buyer.CheckFollowResponse> checkFollow(
    hawkrbuyerv1alpha1buyer.CheckFollowRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.BuyerAppService.checkFollow,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// BatchCheckFollows 回傳畫面上已追蹤的店家 ID；一次最多 100 個。
  Future<hawkrbuyerv1alpha1buyer.BatchCheckFollowsResponse> batchCheckFollows(
    hawkrbuyerv1alpha1buyer.BatchCheckFollowsRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.BuyerAppService.batchCheckFollows,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// FollowStore 追蹤店家；已追蹤時也成功。店家不存在或不公開時回 not_found，追蹤自己的店回 invalid_argument，
  /// 帳號在刪除流程中時回 failed_precondition。
  Future<hawkrbuyerv1alpha1buyer.FollowStoreResponse> followStore(
    hawkrbuyerv1alpha1buyer.FollowStoreRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.BuyerAppService.followStore,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// UnfollowStore 取消追蹤；沒有追蹤時也成功。
  Future<hawkrbuyerv1alpha1buyer.UnfollowStoreResponse> unfollowStore(
    hawkrbuyerv1alpha1buyer.UnfollowStoreRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.BuyerAppService.unfollowStore,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// ListMemberCards 列出買家消費過的店家與累計消費額（會員卡），消費額高到低。
  Future<hawkrbuyerv1alpha1buyer.ListMemberCardsResponse> listMemberCards(
    hawkrbuyerv1alpha1buyer.ListMemberCardsRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.BuyerAppService.listMemberCards,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// ListSpendingRecords 列出買家在一個店家的消費紀錄，新到舊。沒有紀錄時回空的清單。
  Future<hawkrbuyerv1alpha1buyer.ListSpendingRecordsResponse> listSpendingRecords(
    hawkrbuyerv1alpha1buyer.ListSpendingRecordsRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.BuyerAppService.listSpendingRecords,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// ListVisits 列出瀏覽過的商品，依最後瀏覽時間新到舊。
  Future<hawkrbuyerv1alpha1buyer.ListVisitsResponse> listVisits(
    hawkrbuyerv1alpha1buyer.ListVisitsRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.BuyerAppService.listVisits,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// RecordVisit 記錄一次商品瀏覽；每次呼叫都算一次瀏覽，client 不自動重送。
  Future<hawkrbuyerv1alpha1buyer.RecordVisitResponse> recordVisit(
    hawkrbuyerv1alpha1buyer.RecordVisitRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.BuyerAppService.recordVisit,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// ClearVisits 清除全部瀏覽紀錄。
  Future<hawkrbuyerv1alpha1buyer.ClearVisitsResponse> clearVisits(
    hawkrbuyerv1alpha1buyer.ClearVisitsRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.BuyerAppService.clearVisits,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// ListRestockSubscriptions 列出等待補貨通知的商品，新到舊。通知送出或取消之後就不在清單上。
  Future<hawkrbuyerv1alpha1buyer.ListRestockSubscriptionsResponse> listRestockSubscriptions(
    hawkrbuyerv1alpha1buyer.ListRestockSubscriptionsRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.BuyerAppService.listRestockSubscriptions,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// SubscribeRestock 訂閱商品的補貨通知；已訂閱時也成功。
  Future<hawkrbuyerv1alpha1buyer.SubscribeRestockResponse> subscribeRestock(
    hawkrbuyerv1alpha1buyer.SubscribeRestockRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.BuyerAppService.subscribeRestock,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// UnsubscribeRestock 取消補貨通知；沒有訂閱時也成功。
  Future<hawkrbuyerv1alpha1buyer.UnsubscribeRestockResponse> unsubscribeRestock(
    hawkrbuyerv1alpha1buyer.UnsubscribeRestockRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.BuyerAppService.unsubscribeRestock,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }
}
