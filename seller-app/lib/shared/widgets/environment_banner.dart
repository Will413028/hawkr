import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:flutter/material.dart';
import '../../config/flavor.dart';

/// 環境浮動標籤
/// 在非 Production 環境時顯示環境名稱標籤
class EnvironmentBanner extends StatelessWidget {
  final Widget child;

  const EnvironmentBanner({super.key, required this.child});

  @override
  Widget build(BuildContext context) {
    // Production 與 Dev（截圖用）環境不顯示標籤；僅 Stage 顯示
    if (F.appFlavor == Flavor.prod || F.appFlavor == Flavor.dev) return child;

    return Directionality(
      textDirection: TextDirection.ltr,
      child: Banner(
        message: _getLabel(),
        location: BannerLocation.topEnd,
        color: _getBannerColor(),
        child: child,
      ),
    );
  }

  String _getLabel() {
    switch (F.appFlavor) {
      case Flavor.dev:
        return 'DEV';
      case Flavor.stage:
        return 'STAGE';
      case Flavor.prod:
        return '';
    }
  }

  Color _getBannerColor() {
    switch (F.appFlavor) {
      case Flavor.dev:
        return AppColors.success;
      case Flavor.stage:
        return Colors.orange;
      case Flavor.prod:
        return Colors.transparent;
    }
  }
}
