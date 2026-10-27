import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/widgets/hk_placeholder.dart';
import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';

import '../../config/image_config.dart';

class OptimizedNetworkImage extends StatelessWidget {
  const OptimizedNetworkImage({
    super.key,
    required this.imageUrl,
    this.scene,
    this.width,
    this.height,
    this.fit = BoxFit.cover,
    this.alignment = Alignment.center,
    this.borderRadius,
    this.errorWidget,
    this.placeholder,
    this.maxPixels = 1200,
    this.gaplessPlayback = true,
  });

  final String? imageUrl;
  final ImageScene? scene;
  final double? width;
  final double? height;
  final BoxFit fit;
  final Alignment alignment;
  final BorderRadius? borderRadius;
  final Widget? errorWidget;
  final Widget? placeholder;
  final int maxPixels;
  final bool gaplessPlayback;

  @override
  Widget build(BuildContext context) {
    final raw = imageUrl?.trim() ?? '';
    if (raw.isEmpty) return _error();

    final dpr = MediaQuery.of(context).devicePixelRatio;
    final resolvedUrl = scene == null
        ? ImageConfig.urlForDisplay(
            raw,
            width: width,
            height: height,
            devicePixelRatio: dpr,
            maxPixels: maxPixels,
          )
        : ImageConfig.urlFor(raw, scene!);

    if (resolvedUrl.isEmpty) return _error();

    Widget image = CachedNetworkImage(
      imageUrl: resolvedUrl,
      width: width,
      height: height,
      fit: fit,
      alignment: alignment,
      memCacheWidth: ImageConfig.cacheWidthFor(
        scene,
        width: width,
        devicePixelRatio: dpr,
        maxPixels: maxPixels,
      ),
      memCacheHeight: ImageConfig.cacheHeightFor(
        scene,
        height: height,
        devicePixelRatio: dpr,
        maxPixels: maxPixels,
      ),
      useOldImageOnUrlChange: gaplessPlayback,
      placeholder: (_, __) => placeholder ?? _placeholder(),
      errorWidget: (_, __, ___) => _error(),
    );

    if (borderRadius != null) {
      image = ClipRRect(borderRadius: borderRadius!, child: image);
    }
    return image;
  }

  Widget _placeholder() =>
      Container(width: width, height: height, color: AppColors.gray200);

  Widget _error() =>
      errorWidget ??
      SizedBox(
        width: width,
        height: height,
        child: const HkPlaceholder(HkPlaceholderKind.photo),
      );
}

class OptimizedAssetImage extends StatelessWidget {
  const OptimizedAssetImage({
    super.key,
    required this.assetName,
    this.width,
    this.height,
    this.fit = BoxFit.contain,
    this.alignment = Alignment.center,
    this.errorBuilder,
    this.maxPixels = 1200,
  });

  final String assetName;
  final double? width;
  final double? height;
  final BoxFit fit;
  final Alignment alignment;
  final ImageErrorWidgetBuilder? errorBuilder;
  final int maxPixels;

  @override
  Widget build(BuildContext context) {
    final dpr = MediaQuery.of(context).devicePixelRatio;
    return Image.asset(
      assetName,
      width: width,
      height: height,
      fit: fit,
      alignment: alignment,
      cacheWidth: width == null
          ? null
          : (width! * dpr).round().clamp(1, maxPixels).toInt(),
      cacheHeight: height == null
          ? null
          : (height! * dpr).round().clamp(1, maxPixels).toInt(),
      errorBuilder: errorBuilder,
    );
  }
}
