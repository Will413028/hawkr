/// API 回的圖片（`Image.uri_template`）的網址（ADR backend/docs/adr/2026-09-25-api-image-uri-template.md D3）。
library;

import 'gen/hawkr/common/v1alpha1/image.pb.dart';
import 'image_presets.g.dart';

export 'gen/hawkr/common/v1alpha1/image.pb.dart' show Image;
export 'image_presets.g.dart';

extension ImageUri on Image {
  /// [preset] 的網址：只把 `{preset}` 換成 preset 名稱，不解析、不改寫其他部分；外部圖片的樣板沒有變數，
  /// 原樣回傳。
  String uri(ImagePreset preset) =>
      uriTemplate.replaceAll('{preset}', preset.name);
}
