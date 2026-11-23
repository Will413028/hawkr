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

  /// BeginSecureUpload 開一個機敏檔案的分段上傳會話（B1-D17）。回傳 staged token、段大小與段總數；client 依段大小把檔案切段，
  /// 從第 0 段起依序以 `PUT /api/files/secure_uploads/{staged_token}/chunks/{index}`（原始位元組 body，不經 Connect；規範第十三節）送出。
  /// size 小於一或超過伺服器的上限回 invalid_argument 指向 size（訊息是在地化文字）；伺服器沒有設定加密金鑰回 unavailable。
  /// 每次呼叫建立新的會話；沒有送完的由伺服器回收。
  Future<hawkrmediav1alpha1media.BeginSecureUploadResponse> beginSecureUpload(
    hawkrmediav1alpha1media.BeginSecureUploadRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.MediaAppService.beginSecureUpload,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }
}
/// MediaDelegatedService 是代行營運中的平台管理員為代行店家上傳機敏檔：憑證是 X-HK-Operation-Session 標頭裡的代行 session（由 admin 的 BFF 轉送，不放在請求訊息），
/// 不讀顧客或管理員的身分；擁有者是 session 建立當下綁定的那一家店，請求不帶 store_id。
/// 需要能力 media.upload（B1-D16）：憑證缺少或無效回 unauthenticated；session 已結束、到期或撤銷回 failed_precondition；session 沒有 media.upload 回 permission_denied；
/// 店家此刻不可代行回 failed_precondition，只拒絕、不撤銷 session。回應一律不可快取（private, no-store）。
/// 稽核：上傳只產生會過期的暫存，不改賣家看得到的資料，所以成功的 BeginSecureUpload 與段上傳都不寫代行稽核；稽核留在擁有者確認時的寫入。被拒絕的請求另寫一列 FAILED。
/// 會話的上傳者是管理員本人（與 Java 一致），並綁定被代行的那一家店：段上傳與暫存預覽（`GET /api/files/delegated/secure_uploads/{staged_token}`）都只以 X-HK-Operation-Session 授權（沒有管理員 Bearer），session 此刻有效且帶 media.upload，上傳者是 session 的管理員；擁有者確認時 token 也只對同一家店成立。不保留原始檔名。
extension type MediaDelegatedServiceClient (connect.Transport _transport) {
  /// BeginSecureUpload 為代行店家開一個機敏檔案的分段上傳會話，回傳與 MediaAppService.BeginSecureUpload 相同的 staged token、段大小與段總數。
  /// 段以 `PUT /api/files/delegated/secure_uploads/{staged_token}/chunks/{index}` 送出（原始位元組 body，不經 Connect；規範第十三節），只有綁定的那家店的有效代行 session（請求帶 X-HK-Operation-Session）能續傳。
  /// size 小於一或超過伺服器的上限回 invalid_argument 指向 size；伺服器沒有設定加密金鑰回 unavailable。需要能力 media.upload。
  Future<hawkrmediav1alpha1media.MediaDelegatedServiceBeginSecureUploadResponse> beginSecureUpload(
    hawkrmediav1alpha1media.MediaDelegatedServiceBeginSecureUploadRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.MediaDelegatedService.beginSecureUpload,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }
}
