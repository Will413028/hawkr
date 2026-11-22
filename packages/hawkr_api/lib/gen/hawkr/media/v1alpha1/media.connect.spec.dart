//
//  Generated code. Do not modify.
//  source: hawkr/media/v1alpha1/media.proto
//

import "package:connectrpc/connect.dart" as connect;
import "media.pb.dart" as hawkrmediav1alpha1media;

/// MediaAppService 是登入的買家與賣家上傳公開圖片；管理員等非顧客帳號的身分不能呼叫（permission_denied）。
abstract final class MediaAppService {
  /// Fully-qualified name of the MediaAppService service.
  static const name = 'hawkr.media.v1alpha1.MediaAppService';

  /// BeginUpload 宣告要上傳的檔案，回傳預簽的 PUT 網址、client 必須原樣帶上的標頭與 staged token。
  /// 宣告的大小必須大於零且不超過伺服器的上限，型別不可為空；否則回 invalid_argument 且不發網址。
  /// 每次呼叫建立新的一筆暫存；網址與 staged token 只在這個回應出現一次，沒有用到的暫存由伺服器回收。
  static const beginUpload = connect.Spec(
    '/$name/BeginUpload',
    connect.StreamType.unary,
    hawkrmediav1alpha1media.BeginUploadRequest.new,
    hawkrmediav1alpha1media.BeginUploadResponse.new,
  );

  /// CompleteUpload 在 client 把檔案 PUT 完之後呼叫，伺服器開始非同步處理。可重送：已經完成的重送一律成功。
  /// staged token 不存在或不是呼叫者的回 not_found（兩者不可區分）；暫存已過期回 failed_precondition。
  static const completeUpload = connect.Spec(
    '/$name/CompleteUpload',
    connect.StreamType.unary,
    hawkrmediav1alpha1media.CompleteUploadRequest.new,
    hawkrmediav1alpha1media.CompleteUploadResponse.new,
    idempotency: connect.Idempotency.idempotent,
  );

  /// GetUpload 讀暫存的處理狀態。staged token 是憑證，所以不標 NO_SIDE_EFFECTS（以 POST 呼叫、不放進網址；規範第三節）；client 輪詢，失敗下一輪再讀。
  /// staged token 不存在或不是呼叫者的回 not_found（兩者不可區分）。
  static const getUpload = connect.Spec(
    '/$name/GetUpload',
    connect.StreamType.unary,
    hawkrmediav1alpha1media.GetUploadRequest.new,
    hawkrmediav1alpha1media.GetUploadResponse.new,
  );
}
