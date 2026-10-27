import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/shared/widgets/hk_placeholder.dart';
import 'package:cached_network_image/cached_network_image.dart';
import 'package:hawkr_api/images.dart' as api;
import 'package:flutter/material.dart';
import 'package:shimmer/shimmer.dart';
import '../../config/image_config.dart';

enum AppImageLoadingStyle {
  /// 移動光帶。網頁只在「整頁骨架屏」用掃光，單張圖片不用，因此這裡僅保留給
  /// 少數需要明顯載入回饋的場景，不是預設值。
  shimmer,

  /// 預設：固定的淡色佔位塊，圖片到位後短暫淡入，與網頁的圖片載入一致。
  quiet,
}

class AppCachedImage extends StatelessWidget {
  const AppCachedImage({
    super.key,
    required this.imageUrl,
    this.width,
    this.height,
    this.fit = BoxFit.cover,
    this.borderRadius,
    this.memCacheWidth = 400,
    this.memCacheHeight,
    this.errorWidget,
    this.scene,
    this.loadingStyle = AppImageLoadingStyle.quiet,
  }) : _fromApi = false;

  /// API 回的圖片（`Image.uri_template`，ADR 2026-09-25-api-image-uri-template）：只把 `{preset}` 換成
  /// [scene] 的 preset，不經過 `buildImageUrl`；網址由伺服器組，App 不持有媒體網域。還沒搬進單體的模組
  /// 回的是 key 或網址，仍用預設的建構子。
  AppCachedImage.api({
    super.key,
    required api.Image? image,
    required ImageScene this.scene,
    this.width,
    this.height,
    this.fit = BoxFit.cover,
    this.borderRadius,
    this.errorWidget,
    this.loadingStyle = AppImageLoadingStyle.quiet,
  }) : imageUrl = image?.uri(ImageConfig.presetFor(scene)),
       memCacheWidth = 400,
       memCacheHeight = null,
       _fromApi = true;

  final String? imageUrl;
  final double? width;
  final double? height;
  final BoxFit fit;
  final BorderRadius? borderRadius;
  final int memCacheWidth;
  final int? memCacheHeight;
  final Widget? errorWidget;
  final AppImageLoadingStyle loadingStyle;

  /// 圖片場景（提供時自動套用 OSS 壓縮策略 + 推導 memCacheWidth）
  final ImageScene? scene;

  /// [imageUrl] 已經是 API 圖片的完整網址，不再經過 [ImageConfig.urlFor]。
  final bool _fromApi;

  @override
  Widget build(BuildContext context) {
    var url = imageUrl?.trim() ?? '';

    if (url.isEmpty) {
      return _buildError();
    }

    // 當有 scene 時，URL 走 ImageConfig 處理管線
    if (scene != null && !_fromApi) {
      url = ImageConfig.urlFor(url, scene!);
    }

    final effectivePolicy = scene == null
        ? null
        : ImageConfig.policyFor(scene!);
    final effectiveMemCacheWidth = effectivePolicy == null
        ? memCacheWidth
        : _aspectSafeDecodeWidth(effectivePolicy);
    final effectiveMemCacheHeight = effectivePolicy == null
        ? memCacheHeight
        : null;

    Widget image = CachedNetworkImage(
      imageUrl: url,
      width: width,
      height: height,
      fit: fit,
      memCacheWidth: effectiveMemCacheWidth,
      memCacheHeight: effectiveMemCacheHeight,
      placeholder: (context, url) => _buildPlaceholder(),
      errorWidget: (context, url, error) => _buildError(),
      placeholderFadeInDuration: Duration.zero,
      // quiet 不淡出佔位塊：1 秒的淡出在捲動列表裡會拖出殘影，圖片換到下一張
      // 時尤其明顯。淡入取 180ms，與網頁圖片到位的節奏相同。
      fadeOutDuration: loadingStyle == AppImageLoadingStyle.quiet
          ? Duration.zero
          : const Duration(milliseconds: 1000),
      fadeInDuration: loadingStyle == AppImageLoadingStyle.quiet
          ? const Duration(milliseconds: 180)
          : const Duration(milliseconds: 500),
      fadeInCurve: Curves.easeOut,
    );

    if (borderRadius != null) {
      image = ClipRRect(borderRadius: borderRadius!, child: image);
    }

    return image;
  }

  /// Scene policy 的寬高是畫面解析度預算，不是可強制套用的圖片比例。
  ///
  /// 同時把 width/height 傳給 image codec 會先將點陣圖解碼成固定比例，
  /// 後續即使使用 BoxFit.contain 也無法還原。只限制單邊，讓 codec 依原圖
  /// 比例推導另一邊；實際的裁切或留白再交給 BoxFit。
  int _aspectSafeDecodeWidth(ImagePolicy policy) {
    final policyWidth = policy.width ?? 0;
    final policyHeight = policy.height ?? 0;
    final longestSide = policyWidth >= policyHeight
        ? policyWidth
        : policyHeight;
    return longestSide > 0 ? longestSide : memCacheWidth;
  }

  Widget _buildPlaceholder() {
    if (loadingStyle == AppImageLoadingStyle.quiet) {
      return const ColoredBox(
        key: ValueKey('app-cached-image-quiet-placeholder'),
        color: AppColors.surfaceMuted,
      );
    }
    return Shimmer.fromColors(
      baseColor: AppColors.dividerLight,
      highlightColor: AppColors.bgPrimary,
      child: Container(
        width: width,
        height: height,
        color: AppColors.dividerLight,
      ),
    );
  }

  Widget _buildError() {
    return errorWidget ??
        SizedBox(
          width: width,
          height: height,
          child: const HkPlaceholder(HkPlaceholderKind.photo),
        );
  }
}
