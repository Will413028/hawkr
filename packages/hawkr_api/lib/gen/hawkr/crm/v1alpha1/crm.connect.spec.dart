//
//  Generated code. Do not modify.
//  source: hawkr/crm/v1alpha1/crm.proto
//

import "package:connectrpc/connect.dart" as connect;
import "crm.pb.dart" as hawkrcrmv1alpha1crm;

/// CrmAppService 是店家的客戶與標籤。
abstract final class CrmAppService {
  /// Fully-qualified name of the CrmAppService service.
  static const name = 'hawkr.crm.v1alpha1.CrmAppService';

  /// ListCustomers 列出店家的客戶。type 只列這個分類，tag_id 只列貼了這個標籤的客戶（單一標籤），都不設是全部；明確送出 type 卻是 CUSTOMER_TYPE_UNSPECIFIED 回 invalid_argument。
  /// sort 預設（CUSTOMER_SORT_UNSPECIFIED）是 CUSTOMER_SORT_FIRST_ORDER_TIME，descending 為真時大的在前（預設小的在前）；同值以買家 ID 排序，分頁穩定。
  /// 不回總數，結束只看 next_page_token；page_token 只能用在同一家店與同一組篩選與排序。tag_id 不是這家店的標籤時結果是空的一頁。
  static const listCustomers = connect.Spec(
    '/$name/ListCustomers',
    connect.StreamType.unary,
    hawkrcrmv1alpha1crm.ListCustomersRequest.new,
    hawkrcrmv1alpha1crm.ListCustomersResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// GetCustomer 讀一位客戶與他的標籤。客戶只在首筆完成訂單時建立；這家店沒有這位客戶（包含買家已刪除）回 not_found。
  static const getCustomer = connect.Spec(
    '/$name/GetCustomer',
    connect.StreamType.unary,
    hawkrcrmv1alpha1crm.GetCustomerRequest.new,
    hawkrcrmv1alpha1crm.GetCustomerResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// UpdateCustomer 修改客戶的分類（update_mask 只能列 type；值 NORMAL 或 VIP，UNSPECIFIED 回 invalid_argument）。update_mask 沒有列 type 回 invalid_argument。
  /// 這家店沒有這位客戶回 not_found。
  static const updateCustomer = connect.Spec(
    '/$name/UpdateCustomer',
    connect.StreamType.unary,
    hawkrcrmv1alpha1crm.UpdateCustomerRequest.new,
    hawkrcrmv1alpha1crm.UpdateCustomerResponse.new,
    idempotency: connect.Idempotency.idempotent,
  );

  /// CreateTag 建立標籤，名稱去掉前後空白後 1 到 50 個字元。同店已有同名標籤回 already_exists 與 CRM_TAG_NAME_TAKEN；每家店最多 200 個標籤，超過回 failed_precondition 與 CRM_TAG_LIMIT。
  static const createTag = connect.Spec(
    '/$name/CreateTag',
    connect.StreamType.unary,
    hawkrcrmv1alpha1crm.CreateTagRequest.new,
    hawkrcrmv1alpha1crm.CreateTagResponse.new,
  );

  /// ListTags 列出店家全部的標籤，名稱升序，不分頁（每家店最多 200 個）。
  static const listTags = connect.Spec(
    '/$name/ListTags',
    connect.StreamType.unary,
    hawkrcrmv1alpha1crm.ListTagsRequest.new,
    hawkrcrmv1alpha1crm.ListTagsResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );

  /// UpdateTag 改標籤名稱（update_mask 只能列 name，沒有列 name 回 invalid_argument）。撞名回 already_exists 與 CRM_TAG_NAME_TAKEN；標籤不存在回 not_found。
  static const updateTag = connect.Spec(
    '/$name/UpdateTag',
    connect.StreamType.unary,
    hawkrcrmv1alpha1crm.UpdateTagRequest.new,
    hawkrcrmv1alpha1crm.UpdateTagResponse.new,
    idempotency: connect.Idempotency.idempotent,
  );

  /// DeleteTag 刪標籤，它在客戶身上的關聯一併刪除。標籤不存在回 not_found。
  static const deleteTag = connect.Spec(
    '/$name/DeleteTag',
    connect.StreamType.unary,
    hawkrcrmv1alpha1crm.DeleteTagRequest.new,
    hawkrcrmv1alpha1crm.DeleteTagResponse.new,
    idempotency: connect.Idempotency.idempotent,
  );

  /// AddCustomerTags 替客戶貼上一批標籤，名稱不存在的先建立，已貼的不重複；一次 1 到 20 個名稱。整批原子：任何一步失敗都沒有寫入。每位客戶最多 20 個標籤、每家店最多 200 個標籤，
  /// 超過回 failed_precondition 與 CRM_TAG_LIMIT。這家店沒有這位客戶回 not_found。回傳貼完之後的客戶。
  static const addCustomerTags = connect.Spec(
    '/$name/AddCustomerTags',
    connect.StreamType.unary,
    hawkrcrmv1alpha1crm.AddCustomerTagsRequest.new,
    hawkrcrmv1alpha1crm.AddCustomerTagsResponse.new,
  );

  /// RemoveCustomerTag 移除客戶身上的一個標籤，客戶本來就沒有這個標籤時也成功。這家店沒有這位客戶回 not_found。回傳移除之後的客戶。
  static const removeCustomerTag = connect.Spec(
    '/$name/RemoveCustomerTag',
    connect.StreamType.unary,
    hawkrcrmv1alpha1crm.RemoveCustomerTagRequest.new,
    hawkrcrmv1alpha1crm.RemoveCustomerTagResponse.new,
    idempotency: connect.Idempotency.idempotent,
  );
}
