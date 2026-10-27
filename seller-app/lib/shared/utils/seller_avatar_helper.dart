import 'package:hawkr_api/images.dart';

/// 店家圖片（`Store.logo`、`Store.cover_image`）的顯示網址：只把 `Image.uri_template` 的 `{preset}` 換成 preset 名稱。
abstract final class SellerAvatarHelper {
  /// logo 的頭像網址；沒有 logo 或樣板是空的時是 null。
  static String? avatarUrl(Image? logo) => _uri(logo, ImagePreset.avatar);

  /// 封面的橫幅網址；沒有封面或樣板是空的時是 null。
  static String? coverUrl(Image? cover) => _uri(cover, ImagePreset.banner);

  static String? _uri(Image? image, ImagePreset preset) {
    if (image == null || image.uriTemplate.isEmpty) return null;
    return image.uri(preset);
  }
}
