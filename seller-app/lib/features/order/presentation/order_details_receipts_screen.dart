import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/widgets/hk_adaptive_page.dart';
import 'package:hawkr_seller/shared/widgets/hk_async_state_panel.dart';
import 'package:hawkr_seller/shared/widgets/hk_glass_app_bar.dart';
import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

/// 舊版收據深連結的相容頁。
///
/// 舊路由沒有攜帶訂單資料，因此不顯示任何範例發票；引導使用者從真實訂單進入文件中心。
class OrderDetailsReceiptsScreen extends StatelessWidget {
  const OrderDetailsReceiptsScreen({super.key});

  static Route<T> route<T extends Object?>({RouteSettings? settings}) {
    return MaterialPageRoute<T>(
      builder: (_) => const OrderDetailsReceiptsScreen(),
      settings: settings,
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: AppColors.surfacePage,
      appBar: const HkGlassAppBar(title: Text('訂單文件')),
      body: HkAdaptivePage(
        maxWidth: 720,
        padding: const EdgeInsets.fromLTRB(16, 24, 16, 32),
        child: Align(
          alignment: Alignment.topCenter,
          child: HkAsyncStatePanel(
            kind: HkAsyncStateKind.restricted,
            title: '需要從訂單開啟',
            description: '這個舊連結沒有訂單資料。請先選擇訂單，再查看對應的收據與文件。',
            actionLabel: '前往訂單',
            onAction: () => context.go('/main/orders'),
          ),
        ),
      ),
    );
  }
}
