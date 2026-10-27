import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

import '../../config/image_config.dart';
import '../utils/hk_log.dart';
import 'optimized_image.dart';

/// 商家大頭貼元件：顯示 [imageUrl]（`SellerAvatarHelper.avatarUrl` 的結果），沒有網址或不是 http(s) 時顯示預設頭像。
class SellerAvatarWidget extends StatelessWidget {
  const SellerAvatarWidget({
    super.key,
    this.size = 40,
    this.borderRadius,
    this.isCircle = true,
    this.imageUrl,
  });

  final double size;
  final double? borderRadius;
  final bool isCircle;

  final String? imageUrl;

  @override
  Widget build(BuildContext context) => _renderUrl(imageUrl);

  Widget _renderUrl(String? url) {
    final defaultAvatar = Image.asset(
      'assets/hawkrlogo.png',
      fit: BoxFit.cover,
      width: size,
      height: size,
    );
    if (url == null || url.isEmpty || !url.startsWith('http')) {
      return _clip(defaultAvatar);
    }
    return _clip(
      OptimizedNetworkImage(
        // key=URL — URL 一變就強制重新 mount，避免 CachedNetworkImage 內部
        // State 黏住舊 stream（已知 cached_network_image 行為，prop 換不夠）。
        key: ValueKey(url),
        imageUrl: url,
        scene: ImageScene.avatarMedium,
        fit: BoxFit.cover,
        width: size,
        height: size,
        placeholder: defaultAvatar,
        errorWidget: _rawNetworkImage(url, defaultAvatar),
      ),
    );
  }

  Widget _rawNetworkImage(String url, Widget fallback) {
    return Image.network(
      url,
      key: ValueKey('raw-avatar-$url'),
      fit: BoxFit.cover,
      width: size,
      height: size,
      cacheWidth: (size * 3).toInt(),
      cacheHeight: (size * 3).toInt(),
      gaplessPlayback: true,
      errorBuilder: (_, error, ___) {
        HKLog.logW('[SellerAvatarWidget] Avatar load failed: $error');
        return fallback;
      },
    );
  }

  Widget _clip(Widget child) {
    if (isCircle) {
      return ClipOval(
        child: SizedBox(width: size, height: size, child: child),
      );
    }
    return ClipRRect(
      borderRadius: BorderRadius.circular((borderRadius ?? 8).r),
      child: SizedBox(width: size, height: size, child: child),
    );
  }
}
