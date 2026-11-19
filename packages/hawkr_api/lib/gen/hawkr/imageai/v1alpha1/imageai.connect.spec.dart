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
  /// 錯誤：額度用完回 resource_exhausted；圖片被安全性檢查擋下、圖片涉及著作權複誦、模型沒有產出圖片回
  /// failed_precondition（LocalizedMessage 說明原因與是否可以重試）；模型供應商無法呼叫回 unavailable。
  /// 呼叫失敗時額度退回。
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
