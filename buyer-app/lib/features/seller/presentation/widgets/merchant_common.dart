import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

import '../../../../config/image_config.dart';
import '../../../../config/media_image.dart';
import '../../../../shared/widgets/app_cached_image.dart';
import '../../../../shared/widgets/oss_image.dart';

/// 脈動骨架屏佔位框。
class MerchantSkeletonBox extends StatefulWidget {
  const MerchantSkeletonBox({
    super.key,
    required this.width,
    required this.height,
  });

  final double width;
  final double height;

  @override
  State<MerchantSkeletonBox> createState() => _MerchantSkeletonBoxState();
}

class _MerchantSkeletonBoxState extends State<MerchantSkeletonBox>
    with SingleTickerProviderStateMixin {
  late final AnimationController _ctrl;
  late final Animation<double> _animation;

  @override
  void initState() {
    super.initState();
    _ctrl = AnimationController(
      duration: const Duration(milliseconds: 1200),
      vsync: this,
    )..repeat(reverse: true);
    _animation = Tween<double>(
      begin: 0.3,
      end: 0.6,
    ).animate(CurvedAnimation(parent: _ctrl, curve: Curves.easeInOut));
  }

  @override
  void dispose() {
    _ctrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return AnimatedBuilder(
      animation: _animation,
      builder: (_, __) => Container(
        width: widget.width,
        height: widget.height,
        decoration: BoxDecoration(
          color: AppColors.textSecondary.withValues(alpha: _animation.value),
          borderRadius: BorderRadius.circular(6.r),
        ),
      ),
    );
  }
}

// ---------------------------------------------------------------------------

/// 商家頁圖片渲染共用邏輯（base64 / OSS URL / 預設佔位）。
class MerchantImageHelper {
  MerchantImageHelper._();

  /// The API's shared seller cover is a placeholder, not merchant-authored
  /// branding. Treat it as empty so the app can render the canonical
  /// storefront fallback instead of the legacy olive artwork.
  static bool isCustomBanner(String? source) {
    final value = (source ?? '').trim().toLowerCase();
    if (value.isEmpty) return false;
    return !value.contains('/public/defaults/seller_cover.');
  }

  /// 渲染頭像（大）。[payload] 可為 objectKey、完整 URL 或 `data:` inline 圖。
  static Widget buildAvatarLarge(String? payload, {Widget? fallback}) {
    return _buildOssOrBase64(
      payload,
      scene: ImageScene.avatarLarge,
      fallback: fallback ?? _defaultLogo(),
    );
  }

  /// 渲染頭像（小）。[payload] 可為 objectKey、完整 URL 或 `data:` inline 圖。
  static Widget buildAvatarSmall(String? payload, {Widget? fallback}) {
    return _buildOssOrBase64(
      payload,
      scene: ImageScene.avatarSmall,
      fallback: fallback ?? _defaultLogo(),
    );
  }

  /// 渲染商家 Banner。[url] 可為 objectKey、完整 URL 或 `data:` inline 圖。
  ///
  /// 不走 OSS resize 流程：直接抓原圖 + Flutter 本地 cacheWidth 降採樣，
  /// 跟 seller App 自己看時的處理一致（page_header.dart Image.network +
  /// cacheWidth:1200）。OSS server q=85 lossy 重壓會讓青苔/泡泡等高頻紋理
  /// 明顯模糊，原圖 + Flutter 高品質 decoder 看起來銳利。
  static Widget buildBanner(String? url, {Widget? fallback}) {
    final defaultBanner = fallback ?? _defaultBanner();
    final s = (url ?? '').trim();
    if (!isCustomBanner(s)) return defaultBanner;

    if (isInlineImageData(s)) {
      try {
        final b64 = s.contains(',') ? s.split(',').last : s;
        return Image.memory(
          base64Decode(b64),
          fit: BoxFit.cover,
          alignment: Alignment.center,
          gaplessPlayback: true,
          width: double.infinity,
          height: double.infinity,
        );
      } catch (_) {
        return defaultBanner;
      }
    }

    // Banner 走 buildImageUrl（banner preset）；外部 / 動態端點 URL 原圖 passthrough
    // （維持原設計：原圖 + Flutter 本地降採樣，避免高頻紋理被伺服端重壓模糊）。
    return AppCachedImage(
      imageUrl: buildImageUrl(s, ImagePreset.banner),
      fit: BoxFit.cover,
      width: double.infinity,
      height: double.infinity,
      memCacheWidth: 1200,
      errorWidget: defaultBanner,
    );
  }

  static Widget _buildOssOrBase64(
    String? source, {
    required ImageScene scene,
    required Widget fallback,
    BoxFit fit = BoxFit.cover,
    Alignment alignment = Alignment.center,
    bool gaplessPlayback = true,
  }) {
    final s = (source ?? '').trim();
    if (s.isEmpty) return fallback;

    if (isInlineImageData(s)) {
      try {
        final b64 = s.contains(',') ? s.split(',').last : s;
        return Image.memory(
          base64Decode(b64),
          fit: fit,
          alignment: alignment,
          gaplessPlayback: gaplessPlayback,
          width: double.infinity,
          height: double.infinity,
        );
      } catch (_) {
        return fallback;
      }
    }

    return OssImage(
      imageSource: s,
      scene: scene,
      fit: fit,
      alignment: alignment,
      gaplessPlayback: gaplessPlayback,
      width: double.infinity,
      height: double.infinity,
      errorWidget: fallback,
    );
  }

  static Widget _defaultLogo() => Image.asset(
    'assets/hawkrlogo.png',
    fit: BoxFit.cover,
    width: double.infinity,
    height: double.infinity,
  );

  static Widget _defaultBanner() => DecoratedBox(
    decoration: const BoxDecoration(
      gradient: LinearGradient(
        begin: Alignment.topLeft,
        end: Alignment.bottomRight,
        colors: [AppColors.storefrontHeroStart, AppColors.storefrontHeroEnd],
      ),
    ),
    child: Stack(
      fit: StackFit.expand,
      children: [
        const DecoratedBox(
          decoration: BoxDecoration(
            gradient: RadialGradient(
              center: Alignment(0.72, -0.62),
              radius: 0.84,
              colors: [AppColors.storefrontHeroGlow, Color(0x00F2A541)],
            ),
          ),
        ),
      ],
    ),
  );
}

// ---------------------------------------------------------------------------

/// 後端 API list 響應解析共用邏輯。
class MerchantApiHelper {
  MerchantApiHelper._();

  /// 從 `body['data']` 中提取 list，相容 List / Map（records/items/list/rows）。
  static List<Map<String, dynamic>> extractList(Map<String, dynamic> body) {
    final dataField = body['data'];
    List rawList;
    if (dataField is List) {
      rawList = dataField;
    } else if (dataField is Map) {
      rawList =
          (dataField['records'] as List?) ??
          (dataField['items'] as List?) ??
          (dataField['list'] as List?) ??
          (dataField['rows'] as List?) ??
          [];
    } else {
      rawList = [];
    }
    return [
      for (final item in rawList)
        if (item is Map) Map<String, dynamic>.from(item),
    ];
  }
}

// ---------------------------------------------------------------------------

/// 從 Map 中按候選 key 順序提取第一個非空字串值。
class MerchantFieldHelper {
  MerchantFieldHelper._();

  static String str(Map m, List<String> keys, [String def = '']) {
    for (final k in keys) {
      final v = m[k];
      if (v != null && v.toString().trim().isNotEmpty) return v.toString();
    }
    return def;
  }

  static num num_(Map m, List<String> keys, [num def = 0]) {
    final s = str(m, keys, '');
    return s.isEmpty ? def : (num.tryParse(s) ?? def);
  }
}
