//
//  Generated code. Do not modify.
//  source: hawkr/imageai/v1alpha1/imageai.proto
//

import "package:connectrpc/connect.dart" as connect;
import "imageai.pb.dart" as hawkrimageaiv1alpha1imageai;

/// ImageAiAppService 是登入的賣家的修圖方法。店家取自呼叫者目前的店家，帳號沒有賣家角色時回 permission_denied。
abstract final class ImageAiAppService {
  /// Fully-qualified name of the ImageAiAppService service.
  static const name = 'hawkr.imageai.v1alpha1.ImageAiAppService';

  /// TransformImage 以提示詞修一張圖，成功時扣一次本週額度。
  /// 錯誤：本週額度用完回 resource_exhausted，reason 是 IMAGE_QUOTA_EXCEEDED；invalid_argument 的 reason 是
  /// IMAGE_SAFETY_BLOCKED 與 IMAGE_RECITATION_BLOCKED（輸入要換：換圖或改提示詞，不要重試）；unavailable 的 reason 是
  /// IMAGE_NOT_PRODUCED（模型沒有產出圖片，重試可能成功），模型供應商無法呼叫也回 unavailable。
  /// 同時進行的修圖超過上限回 resource_exhausted，帶 RetryInfo，不扣額度。
  /// 原圖超過 2 MiB 或提示詞超過 500 字回 invalid_argument，BadRequest 帶出 field。
  /// 這個服務接受最大 3 MiB 的請求訊息，讓 2 MiB 原圖的 JSON／base64 編碼放得下。
  /// 呼叫失敗時額度退回；扣了額度之後伺服器程序中途終止的那一次不退回，是已知限制。
  static const transformImage = connect.Spec(
    '/$name/TransformImage',
    connect.StreamType.unary,
    hawkrimageaiv1alpha1imageai.TransformImageRequest.new,
    hawkrimageaiv1alpha1imageai.TransformImageResponse.new,
  );

  /// ListAiStyles 列出某個用途的啟用中風格，連同本週的額度使用量。
  static const listAiStyles = connect.Spec(
    '/$name/ListAiStyles',
    connect.StreamType.unary,
    hawkrimageaiv1alpha1imageai.ListAiStylesRequest.new,
    hawkrimageaiv1alpha1imageai.ListAiStylesResponse.new,
    idempotency: connect.Idempotency.noSideEffects,
  );
}
