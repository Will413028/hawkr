//
//  Generated code. Do not modify.
//  source: hawkr/imageai/v1alpha1/imageai.proto
//

import "package:connectrpc/connect.dart" as connect;
import "imageai.pb.dart" as hawkrimageaiv1alpha1imageai;
import "imageai.connect.spec.dart" as specs;

/// ImageAiAppService 是登入的賣家的修圖方法。店家取自呼叫者目前的店家，帳號沒有賣家角色時回 permission_denied。
extension type ImageAiAppServiceClient (connect.Transport _transport) {
  /// TransformImage 以提示詞修一張圖，成功時扣一次本週額度。
  /// 錯誤：本週額度用完回 resource_exhausted，reason 是 IMAGE_QUOTA_EXCEEDED；failed_precondition 的 reason 是
  /// IMAGE_SAFETY_BLOCKED 與 IMAGE_RECITATION_BLOCKED（換圖或改提示詞，不要重試）、IMAGE_NOT_PRODUCED（可以重試）；
  /// 模型供應商無法呼叫回 unavailable。
  /// 呼叫失敗時額度退回。
  Future<hawkrimageaiv1alpha1imageai.TransformImageResponse> transformImage(
    hawkrimageaiv1alpha1imageai.TransformImageRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.ImageAiAppService.transformImage,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// ListAiStyles 列出某個用途的啟用中風格，連同本週的額度使用量。
  Future<hawkrimageaiv1alpha1imageai.ListAiStylesResponse> listAiStyles(
    hawkrimageaiv1alpha1imageai.ListAiStylesRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.ImageAiAppService.listAiStyles,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }
}
