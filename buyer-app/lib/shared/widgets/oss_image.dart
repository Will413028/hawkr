import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import '../../config/image_config.dart';
import '../../config/media_image.dart';
import 'app_cached_image.dart';

/// OSS 圖片 Widget
///
/// 自動處理 OSS objectKey 和一般 URL 的圖片載入
/// - 如果是 OSS objectKey，會自動拼接 CDN base URL
/// - 如果是一般 URL，直接載入
/// - 支援快取、錯誤處理
class OssImage extends StatelessWidget {
  /// 圖片來源（可以是 objectKey 或完整 URL）
  final String imageSource;

  /// 圖片寬度
  final double? width;

  /// 圖片高度
  final double? height;

  /// 圖片填充模式
  final BoxFit fit;

  /// 佔位 Widget（載入中顯示）
  final Widget? placeholder;

  /// 錯誤時顯示的 Widget
  final Widget? errorWidget;

  /// 圖片對齊方式
  final Alignment alignment;

  /// 顏色濾鏡
  final Color? color;

  /// 顏色混合模式
  final BlendMode? colorBlendMode;

  /// 是否允許無縫播放（避免重新載入閃爍）
  final bool gaplessPlayback;

  /// 是否使用快取
  final bool useCache;

  /// 圖片場景（使用配置表策略，優先於 width/height 動態計算）
  final ImageScene? scene;

  const OssImage({
    super.key,
    required this.imageSource,
    this.width,
    this.height,
    this.fit = BoxFit.cover,
    this.placeholder,
    this.errorWidget,
    this.alignment = Alignment.center,
    this.color,
    this.colorBlendMode,
    this.gaplessPlayback = true,
    this.useCache = true,
    this.scene,
  });

  @override
  Widget build(BuildContext context) {
    final String url;

    if (scene != null) {
      // 配置表模式：使用集中管理的場景 preset
      url = ImageConfig.urlFor(imageSource, scene!);
    } else {
      // 動態模式：無場景時走 detail preset（長邊 scale-down、不裁切）；
      // 顯示尺寸交由下方 memCacheWidth 本地降採樣。
      url = buildImageUrl(imageSource, ImagePreset.detail);
    }

    if (url.isEmpty) {
      return _buildErrorWidget();
    }

    return AppCachedImage(
      imageUrl: url,
      width: width,
      height: height,
      fit: fit,
      // 只設 memCacheWidth、不設 memCacheHeight。
      // 同時指定兩者會讓底層 instantiateImageCodec(targetWidth, targetHeight)
      // 把點陣圖「強制」解碼成該寬高 → 長寬比被拉歪（首頁商品卡變形主因）。
      // 只給一邊，decoder 會自動依原圖比例算另一邊；最終裁切/縮放交給外層 BoxFit。
      memCacheWidth: _memCacheWidth(),
      memCacheHeight: null,
      errorWidget: _buildErrorWidget(),
    );
  }

  /// 解碼上限（單邊）。場景模式取 policy 寬高的較大邊，避免直式場景解析度不足；
  /// 動態模式用 widget 尺寸 ×DPR。皆不限制另一邊以保留原圖長寬比。
  int _memCacheWidth() {
    if (scene != null) {
      final p = ImageConfig.policyFor(scene!);
      final w = p.width ?? 0;
      final h = p.height ?? 0;
      final cap = w >= h ? w : h;
      return cap > 0 ? cap : 400;
    }
    final dim = width ?? height;
    return dim == null ? 400 : (dim * 3).round().clamp(1, 1200).toInt();
  }

  Widget _buildErrorWidget() {
    if (errorWidget != null) return errorWidget!;

    return Container(
      width: width,
      height: height,
      color: AppColors.dividerLight,
      child: Icon(
        AppIcons.brokenImageOutlined,
        color: AppIconColors.muted,
        size: (width ?? height ?? 48.sp) * 0.4,
      ),
    );
  }
}

/// 根據場景獲取 OSS 圖片的 ImageProvider
///
/// 用於需要 ImageProvider 的場景（如 CircleAvatar.backgroundImage、DecorationImage）
ImageProvider? getOssSceneImageProvider(String source, ImageScene scene) {
  final url = ImageConfig.urlFor(source, scene);
  if (url.isEmpty) return null;
  return NetworkImage(url);
}
