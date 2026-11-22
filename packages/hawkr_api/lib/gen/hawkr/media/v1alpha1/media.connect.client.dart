//
//  Generated code. Do not modify.
//  source: hawkr/media/v1alpha1/media.proto
//

import "package:connectrpc/connect.dart" as connect;
import "media.pb.dart" as hawkrmediav1alpha1media;
import "media.connect.spec.dart" as specs;

/// MediaAppService 是登入的買家與賣家上傳公開圖片；管理員等非顧客帳號的身分不能呼叫（permission_denied）。
extension type MediaAppServiceClient (connect.Transport _transport) {
  /// BeginUpload 宣告要上傳的檔案，回傳預簽的 PUT 網址、client 必須原樣帶上的標頭與 staged token。
  /// 回 invalid_argument 且不發網址的情況：size 不大於零或超過伺服器的上限（訊息是在地化文字，說明上限）；content_type 不是 image/jpeg、
  /// image/png、image/webp、image/gif 之一（HEIC 等由 client 先轉換）；md5 不是 16 位元組的標準 base64。
  /// 每次呼叫建立新的一筆暫存；網址與 staged token 只在這個回應出現一次，沒有用到的暫存由伺服器回收。
  Future<hawkrmediav1alpha1media.BeginUploadResponse> beginUpload(
    hawkrmediav1alpha1media.BeginUploadRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.MediaAppService.beginUpload,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// CompleteUpload 在 client 把檔案 PUT 完之後呼叫，伺服器開始非同步處理。可重送：已經完成的重送一律成功。
  /// staged token 不存在或不是呼叫者的回 not_found（兩者不可區分）；暫存已過期回 failed_precondition。
  Future<hawkrmediav1alpha1media.CompleteUploadResponse> completeUpload(
    hawkrmediav1alpha1media.CompleteUploadRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.MediaAppService.completeUpload,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// GetUpload 讀暫存的處理狀態。staged token 是憑證，所以不標 NO_SIDE_EFFECTS（以 POST 呼叫、不放進網址；規範第三節）；client 輪詢，失敗下一輪再讀。
  /// staged token 不存在或不是呼叫者的回 not_found（兩者不可區分）。
  Future<hawkrmediav1alpha1media.GetUploadResponse> getUpload(
    hawkrmediav1alpha1media.GetUploadRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.MediaAppService.getUpload,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }
}
