//
//  Generated code. Do not modify.
//  source: hawkr/notification/v1alpha1/notification.proto
//

import "package:connectrpc/connect.dart" as connect;
import "notification.pb.dart" as hawkrnotificationv1alpha1notification;

/// NotificationAppService 是登入的帳號讀寫自己的通知。沒有登入回 unauthenticated；管理員（含代行）回 permission_denied。
abstract final class NotificationAppService {
  /// Fully-qualified name of the NotificationAppService service.
  static const name = 'hawkr.notification.v1alpha1.NotificationAppService';

  /// ListInboxItems 列出呼叫者在指定受眾的收件匣，新到舊（建立時間，再依 id 新到舊）。聚合過的通知（例如多人按讚）是一列，
  /// 時間是最後一次合併的時間。page_token 不透明，換了 audience 或 category 就不能沿用。
  /// total_size 是同一個篩選下的全部筆數，不是整個收件匣。
  static const listInboxItems = connect.Spec(
    '/$name/ListInboxItems',
    connect.StreamType.unary,
    hawkrnotificationv1alpha1notification.ListInboxItemsRequest.new,
    hawkrnotificationv1alpha1notification.ListInboxItemsResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// GetUnreadCount 回傳指定受眾的未讀數（不分分類）。
  static const getUnreadCount = connect.Spec(
    '/$name/GetUnreadCount',
    connect.StreamType.unary,
    hawkrnotificationv1alpha1notification.GetUnreadCountRequest.new,
    hawkrnotificationv1alpha1notification.GetUnreadCountResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// MarkInboxItemRead 把一則通知標成已讀；已經是已讀時成功，不改已讀時間。通知不存在或不是呼叫者的回 not_found，兩者的回應完全相同。
  static const markInboxItemRead = connect.Spec(
    '/$name/MarkInboxItemRead',
    connect.StreamType.unary,
    hawkrnotificationv1alpha1notification.MarkInboxItemReadRequest.new,
    hawkrnotificationv1alpha1notification.MarkInboxItemReadResponse.new,
    idempotency: connect.Idempotency.idempotent,
  );

  /// MarkAllInboxItemsRead 把指定受眾的全部未讀通知標成已讀，回傳這次改成已讀的筆數。
  static const markAllInboxItemsRead = connect.Spec(
    '/$name/MarkAllInboxItemsRead',
    connect.StreamType.unary,
    hawkrnotificationv1alpha1notification.MarkAllInboxItemsReadRequest.new,
    hawkrnotificationv1alpha1notification.MarkAllInboxItemsReadResponse.new,
    idempotency: connect.Idempotency.idempotent,
  );

  /// BatchDeleteInboxItems 刪除通知。ids 為空、重複或超過 100 個回 invalid_argument。ids 中任何一個不存在或不是呼叫者的，
  /// 整個請求回 not_found、一則都不刪（兩種情況的回應相同）。
  static const batchDeleteInboxItems = connect.Spec(
    '/$name/BatchDeleteInboxItems',
    connect.StreamType.unary,
    hawkrnotificationv1alpha1notification.BatchDeleteInboxItemsRequest.new,
    hawkrnotificationv1alpha1notification.BatchDeleteInboxItemsResponse.new,
    idempotency: connect.Idempotency.idempotent,
  );

  /// GetNotificationPreferences 列出指定受眾的開關。entries 只包含註冊表實際會在該 (受眾, 分類, 渠道) 送達的組合，
  /// 不是分類與渠道的笛卡兒積（例如沒有型別走 email 的分類就沒有 email 那一項）；沒有設定過的是預設開。
  static const getNotificationPreferences = connect.Spec(
    '/$name/GetNotificationPreferences',
    connect.StreamType.unary,
    hawkrnotificationv1alpha1notification.GetNotificationPreferencesRequest.new,
    hawkrnotificationv1alpha1notification.GetNotificationPreferencesResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// UpdateNotificationPreference 設定一個 (分類, 渠道) 的開關，回傳更新後的那一項。分類不屬於該受眾、該 (分類, 渠道) 不在
  /// GetNotificationPreferences 的 entries 內（註冊表不會在這個組合送達），或該組合不可關閉（PreferenceEntry.can_disable 為 false）
  /// 而 enabled 是 false，回 invalid_argument。重複設定同一個值成功。
  static const updateNotificationPreference = connect.Spec(
    '/$name/UpdateNotificationPreference',
    connect.StreamType.unary,
    hawkrnotificationv1alpha1notification.UpdateNotificationPreferenceRequest.new,
    hawkrnotificationv1alpha1notification.UpdateNotificationPreferenceResponse.new,
    idempotency: connect.Idempotency.idempotent,
  );

  /// RegisterDeviceToken 註冊 App 的推播 token。App 每次啟動都呼叫，伺服器更新最後註冊時間；同一個 token 換帳號登入時改綁給呼叫者。
  /// 一個 token 只綁一個帳號與一支 App。
  static const registerDeviceToken = connect.Spec(
    '/$name/RegisterDeviceToken',
    connect.StreamType.unary,
    hawkrnotificationv1alpha1notification.RegisterDeviceTokenRequest.new,
    hawkrnotificationv1alpha1notification.RegisterDeviceTokenResponse.new,
    idempotency: connect.Idempotency.idempotent,
  );

  /// DeleteDeviceToken 刪除呼叫者的一個 token（登出時呼叫）。token 不存在或屬於別的帳號也回成功，不洩漏它是否存在。
  static const deleteDeviceToken = connect.Spec(
    '/$name/DeleteDeviceToken',
    connect.StreamType.unary,
    hawkrnotificationv1alpha1notification.DeleteDeviceTokenRequest.new,
    hawkrnotificationv1alpha1notification.DeleteDeviceTokenResponse.new,
    idempotency: connect.Idempotency.idempotent,
  );

  /// WatchNotifications 是指定受眾的即時訊號串流（server streaming）。串流只傳「該重讀了」的訊號，不傳通知內容，也沒有續傳游標：
  /// 連線建立後先送一則 inbox 訊號（目前的未讀數與最新一則的 ID），之後每有變化再送；client 收到訊號就重新呼叫 ListInboxItems 與其他列表，
  /// 斷線重連後也先重讀一次。訊號可能合併或遺漏，資料以列表為準。
  /// STORE 受眾的串流另外承載賣家工作區事件（seller_workspace）。STORE 受眾但呼叫者沒有現任店家回 not_found；
  /// 每個帳號同時開著的串流有上限，超過回 resource_exhausted。
  static const watchNotifications = connect.Spec(
    '/$name/WatchNotifications',
    connect.StreamType.server,
    hawkrnotificationv1alpha1notification.WatchNotificationsRequest.new,
    hawkrnotificationv1alpha1notification.WatchNotificationsResponse.new,
  );
}
