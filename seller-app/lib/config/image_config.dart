import 'media_image.dart';

/// Centralized image scenes for seller app.
///
/// 各場景對應 Worker 顯示 preset（伺服端縮放）與本地降採樣尺寸。
enum ImageScene {
  avatarSmall,
  avatarMedium,
  avatarLarge,
  productThumbnail,
  productDetail,
  orderItem,
  reviewPhoto,
  postImage,
  descriptionImage,
  shareThumbnail,
  chatImage,
  chatAvatar,
  merchantBanner,
  couponImage,
  fullscreen,
}

/// 場景本地降採樣策略（memCacheWidth / Height 用）。
///
/// [width] / [height] 為目標像素尺寸（已按 2x-3x DPR 預算）。圖片伺服端縮放已改
/// 由 Worker preset 決定（見 [ImageConfig._presets] / [buildImageUrl]），本策略
/// 不再參與 URL 組裝。
class ImagePolicy {
  const ImagePolicy({this.width, this.height});

  final int? width;
  final int? height;
}

class ImageConfig {
  ImageConfig._();

  /// 場景 → Worker 顯示 preset（49 §4.4 白名單）。
  static const Map<ImageScene, ImagePreset> _presets = {
    ImageScene.avatarSmall: ImagePreset.avatar,
    ImageScene.avatarMedium: ImagePreset.avatar,
    ImageScene.avatarLarge: ImagePreset.avatar,
    ImageScene.productThumbnail: ImagePreset.thumb,
    ImageScene.productDetail: ImagePreset.detail,
    ImageScene.orderItem: ImagePreset.thumb,
    ImageScene.reviewPhoto: ImagePreset.card,
    // 全螢幕貼文保留完整構圖；店舖縮圖另由呼叫端明確使用 card。
    ImageScene.postImage: ImagePreset.detail,
    // 商品圖文描述畫質敏感（含內嵌文字）→ 不轉換直出原圖。
    ImageScene.descriptionImage: ImagePreset.original,
    ImageScene.shareThumbnail: ImagePreset.thumb,
    ImageScene.chatImage: ImagePreset.chat,
    ImageScene.chatAvatar: ImagePreset.avatar,
    ImageScene.merchantBanner: ImagePreset.banner,
    ImageScene.couponImage: ImagePreset.card,
    ImageScene.fullscreen: ImagePreset.original,
  };

  /// 各場景本地降採樣尺寸。
  static const Map<ImageScene, ImagePolicy> _policies = {
    ImageScene.avatarSmall: ImagePolicy(width: 72, height: 72),
    ImageScene.avatarMedium: ImagePolicy(width: 120, height: 120),
    ImageScene.avatarLarge: ImagePolicy(width: 210, height: 210),
    ImageScene.productThumbnail: ImagePolicy(width: 240, height: 240),
    ImageScene.productDetail: ImagePolicy(width: 1200),
    ImageScene.orderItem: ImagePolicy(width: 240, height: 240),
    ImageScene.reviewPhoto: ImagePolicy(width: 400, height: 400),
    ImageScene.postImage: ImagePolicy(width: 900),
    ImageScene.descriptionImage: ImagePolicy(width: 1600),
    ImageScene.shareThumbnail: ImagePolicy(width: 240, height: 240),
    ImageScene.chatImage: ImagePolicy(width: 720),
    ImageScene.chatAvatar: ImagePolicy(width: 96, height: 96),
    ImageScene.merchantBanner: ImagePolicy(width: 1500),
    ImageScene.couponImage: ImagePolicy(width: 900),
    ImageScene.fullscreen: ImagePolicy(),
  };

  /// 取得指定場景的顯示 preset。
  static ImagePreset presetFor(ImageScene scene) =>
      _presets[scene] ?? ImagePreset.original;

  static ImagePolicy policyFor(ImageScene scene) => _policies[scene]!;

  /// 根據場景生成顯示 URL（走 [buildImageUrl] 唯一組裝入口）。
  static String urlFor(String source, ImageScene scene) {
    final preset = presetFor(scene);
    if (scene == ImageScene.postImage) {
      return buildPostImageUrl(source, staticPreset: preset);
    }
    return buildImageUrl(source, preset);
  }

  /// 無固定場景時的顯示 URL：走 detail preset（長邊 scale-down、不裁切），
  /// 顯示尺寸交由 memCacheWidth / Height 本地降採樣。
  static String urlForDisplay(
    String source, {
    double? width,
    double? height,
    double devicePixelRatio = 3,
    int maxPixels = 1200,
  }) => buildImageUrl(source, ImagePreset.detail);

  static int? cacheWidthFor(
    ImageScene? scene, {
    double? width,
    double devicePixelRatio = 3,
    int maxPixels = 1200,
  }) {
    if (scene != null) return policyFor(scene).width;
    if (width == null) return null;
    return _cap((width * devicePixelRatio).round(), maxPixels);
  }

  static int? cacheHeightFor(
    ImageScene? scene, {
    double? height,
    double devicePixelRatio = 3,
    int maxPixels = 1200,
  }) {
    if (scene != null) return policyFor(scene).height;
    if (height == null) return null;
    return _cap((height * devicePixelRatio).round(), maxPixels);
  }

  static int _cap(int value, int maxPixels) =>
      value > maxPixels ? maxPixels : value;
}
