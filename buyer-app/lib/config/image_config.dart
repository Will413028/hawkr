import 'media_image.dart';

/// 圖片使用場景
enum ImageScene {
  // ── 首頁 ──
  homeBanner,
  homeBannerPopup,
  homeProductCard,
  homeAvatar,

  // ── 商品 ──
  productDetail,
  productLandscapeCard,
  productThumbnail,
  productSearch,

  // ── 購物車 / 訂單 ──
  cartItem,
  orderItem,

  // ── 商家 ──
  merchantBanner,
  merchantAvatar,

  // ── 評論 / 社群 ──
  reviewPhoto,
  socialPost,
  postImage,

  // ── 分類 ──
  categoryBanner,

  // ── 搜尋 ──
  searchResult,

  // ── 分享 ──
  shareThumbnail,

  // ── 聊天 ──
  chatImage,
  chatAvatar,

  // ── 通用頭像 ──
  avatarSmall,
  avatarMedium,
  avatarLarge,

  // ── 全屏查看 ──
  fullscreen,
}

/// 場景本地降採樣策略。
///
/// [width] / [height] 為目標像素尺寸（已按 2x DPR 預算，調用方無需再乘），
/// 提供給 `memCacheWidth` / `memCacheHeight` 做本地解碼降採樣。
///
/// 圖片伺服端縮放已改由 Worker preset 決定（見 [_presets] / [buildImageUrl]），
/// 本策略不再參與 URL 組裝。
class ImagePolicy {
  final int? width;
  final int? height;

  const ImagePolicy({this.width, this.height});
}

/// 圖片配置中心
///
/// 集中管理各場景的顯示 preset（伺服端）與本地降採樣尺寸。
/// 使用方式：
/// ```dart
/// final url = ImageConfig.urlFor('products/xxx.jpg', ImageScene.cartItem);
/// ```
class ImageConfig {
  ImageConfig._();

  /// 場景 → Worker 顯示 preset（49 §4.4 白名單）。
  static const Map<ImageScene, ImagePreset> _presets = {
    // ── 首頁 ──
    ImageScene.homeBanner: ImagePreset.banner,
    ImageScene.homeBannerPopup: ImagePreset.banner,
    ImageScene.homeProductCard: ImagePreset.card,
    ImageScene.homeAvatar: ImagePreset.avatar,

    // ── 商品 ──
    ImageScene.productDetail: ImagePreset.detail,
    // 收藏／瀏覽 4:3 橫卡：card preset（720×540 cover）與畫面同比例，
    // 避免直圖被橫向卡片二次裁切放大。
    ImageScene.productLandscapeCard: ImagePreset.card,
    ImageScene.productThumbnail: ImagePreset.thumb,
    ImageScene.productSearch: ImagePreset.thumb,

    // ── 購物車 / 訂單 ──
    ImageScene.cartItem: ImagePreset.thumb,
    ImageScene.orderItem: ImagePreset.thumb,

    // ── 商家 ──
    ImageScene.merchantBanner: ImagePreset.banner,
    ImageScene.merchantAvatar: ImagePreset.avatar,

    // ── 評論 / 社群 ──
    ImageScene.reviewPhoto: ImagePreset.card,
    ImageScene.socialPost: ImagePreset.card,
    ImageScene.postImage: ImagePreset.card,

    // ── 分類 ──
    ImageScene.categoryBanner: ImagePreset.banner,

    // ── 搜尋 ──
    ImageScene.searchResult: ImagePreset.thumb,

    // ── 分享 ──
    ImageScene.shareThumbnail: ImagePreset.thumb,

    // ── 聊天 ──
    ImageScene.chatImage: ImagePreset.chat,
    ImageScene.chatAvatar: ImagePreset.avatar,

    // ── 通用頭像 ──
    ImageScene.avatarSmall: ImagePreset.avatar,
    ImageScene.avatarMedium: ImagePreset.avatar,
    ImageScene.avatarLarge: ImagePreset.avatar,

    // ── 全屏查看 ──
    ImageScene.fullscreen: ImagePreset.original,
  };

  /// 各場景本地降採樣尺寸（memCacheWidth / Height 用）。
  static const Map<ImageScene, ImagePolicy> _policies = {
    ImageScene.homeBanner: ImagePolicy(width: 1080),
    ImageScene.homeBannerPopup: ImagePolicy(width: 1080),
    ImageScene.homeProductCard: ImagePolicy(width: 320, height: 420),
    ImageScene.homeAvatar: ImagePolicy(width: 44, height: 44),
    ImageScene.productDetail: ImagePolicy(width: 660, height: 500),
    ImageScene.productLandscapeCard: ImagePolicy(width: 384, height: 288),
    ImageScene.productThumbnail: ImagePolicy(width: 144, height: 144),
    ImageScene.productSearch: ImagePolicy(width: 144, height: 144),
    ImageScene.cartItem: ImagePolicy(width: 112, height: 112),
    ImageScene.orderItem: ImagePolicy(width: 160, height: 160),
    ImageScene.merchantBanner: ImagePolicy(width: 1500),
    ImageScene.merchantAvatar: ImagePolicy(width: 210, height: 210),
    ImageScene.reviewPhoto: ImagePolicy(width: 400, height: 400),
    ImageScene.socialPost: ImagePolicy(width: 600, height: 600),
    ImageScene.postImage: ImagePolicy(width: 600, height: 600),
    ImageScene.categoryBanner: ImagePolicy(width: 750, height: 280),
    ImageScene.searchResult: ImagePolicy(width: 144, height: 144),
    ImageScene.shareThumbnail: ImagePolicy(width: 160, height: 160),
    ImageScene.chatImage: ImagePolicy(width: 400, height: 400),
    ImageScene.chatAvatar: ImagePolicy(width: 80, height: 80),
    ImageScene.avatarSmall: ImagePolicy(width: 72, height: 72),
    ImageScene.avatarMedium: ImagePolicy(width: 120, height: 120),
    ImageScene.avatarLarge: ImagePolicy(width: 210, height: 210),
    ImageScene.fullscreen: ImagePolicy(),
  };

  /// 取得指定場景的顯示 preset。
  static ImagePreset presetFor(ImageScene scene) =>
      _presets[scene] ?? ImagePreset.original;

  /// 取得指定場景的本地降採樣策略。
  static ImagePolicy policyFor(ImageScene scene) =>
      _policies[scene] ?? const ImagePolicy();

  /// 根據場景生成顯示 URL（走 [buildImageUrl] 唯一組裝入口）。
  static String urlFor(String source, ImageScene scene) =>
      buildImageUrl(source, presetFor(scene));
}
