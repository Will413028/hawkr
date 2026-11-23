//
//  Generated code. Do not modify.
//  source: hawkr/crm/v1alpha1/crm.proto
//

import "package:connectrpc/connect.dart" as connect;
import "crm.pb.dart" as hawkrcrmv1alpha1crm;
import "crm.connect.spec.dart" as specs;

/// CrmAppService 是店家的客戶與標籤。
extension type CrmAppServiceClient (connect.Transport _transport) {
  /// ListCustomers 列出店家的客戶。type 只列這個分類，tag_id 只列貼了這個標籤的客戶（單一標籤），都不設是全部；明確送出 type 卻是 CUSTOMER_TYPE_UNSPECIFIED 回 invalid_argument。
  /// sort 預設（CUSTOMER_SORT_UNSPECIFIED）是 CUSTOMER_SORT_FIRST_ORDER_TIME，descending 為真時大的在前（預設小的在前）；同值以買家 ID 排序，分頁穩定。
  /// 不回總數，結束只看 next_page_token；page_token 只能用在同一家店與同一組篩選與排序。tag_id 不是這家店的標籤時結果是空的一頁。
  Future<hawkrcrmv1alpha1crm.ListCustomersResponse> listCustomers(
    hawkrcrmv1alpha1crm.ListCustomersRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.CrmAppService.listCustomers,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// GetCustomer 讀一位客戶與他的標籤。客戶只在首筆完成訂單時建立；這家店沒有這位客戶（包含買家已刪除）回 not_found。
  Future<hawkrcrmv1alpha1crm.GetCustomerResponse> getCustomer(
    hawkrcrmv1alpha1crm.GetCustomerRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.CrmAppService.getCustomer,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// UpdateCustomer 修改客戶的分類（update_mask 只能列 type；值 NORMAL 或 VIP，UNSPECIFIED 回 invalid_argument）。update_mask 沒有列 type 回 invalid_argument。
  /// 這家店沒有這位客戶回 not_found。
  Future<hawkrcrmv1alpha1crm.UpdateCustomerResponse> updateCustomer(
    hawkrcrmv1alpha1crm.UpdateCustomerRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.CrmAppService.updateCustomer,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// CreateTag 建立標籤，名稱去掉前後空白後 1 到 50 個字元。同店已有同名標籤回 already_exists 與 CRM_TAG_NAME_TAKEN；每家店最多 200 個標籤，超過回 failed_precondition 與 CRM_TAG_LIMIT。
  Future<hawkrcrmv1alpha1crm.CreateTagResponse> createTag(
    hawkrcrmv1alpha1crm.CreateTagRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.CrmAppService.createTag,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// ListTags 列出店家全部的標籤，名稱升序，不分頁（每家店最多 200 個）。
  Future<hawkrcrmv1alpha1crm.ListTagsResponse> listTags(
    hawkrcrmv1alpha1crm.ListTagsRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.CrmAppService.listTags,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// UpdateTag 改標籤名稱（update_mask 只能列 name，沒有列 name 回 invalid_argument）。撞名回 already_exists 與 CRM_TAG_NAME_TAKEN；標籤不存在回 not_found。
  Future<hawkrcrmv1alpha1crm.UpdateTagResponse> updateTag(
    hawkrcrmv1alpha1crm.UpdateTagRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.CrmAppService.updateTag,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// DeleteTag 刪標籤，它在客戶身上的關聯一併刪除。標籤不存在回 not_found。
  Future<hawkrcrmv1alpha1crm.DeleteTagResponse> deleteTag(
    hawkrcrmv1alpha1crm.DeleteTagRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.CrmAppService.deleteTag,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// AddCustomerTags 替客戶貼上一批標籤，名稱不存在的先建立，已貼的不重複；一次 1 到 20 個名稱。整批原子：任何一步失敗都沒有寫入。每位客戶最多 20 個標籤、每家店最多 200 個標籤，
  /// 超過回 failed_precondition 與 CRM_TAG_LIMIT。這家店沒有這位客戶回 not_found。回傳貼完之後的客戶。
  Future<hawkrcrmv1alpha1crm.AddCustomerTagsResponse> addCustomerTags(
    hawkrcrmv1alpha1crm.AddCustomerTagsRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.CrmAppService.addCustomerTags,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// RemoveCustomerTag 移除客戶身上的一個標籤，客戶本來就沒有這個標籤時也成功。這家店沒有這位客戶回 not_found。回傳移除之後的客戶。
  Future<hawkrcrmv1alpha1crm.RemoveCustomerTagResponse> removeCustomerTag(
    hawkrcrmv1alpha1crm.RemoveCustomerTagRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.CrmAppService.removeCustomerTag,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }
}
