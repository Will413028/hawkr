import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/core/theme/hk_tokens.dart';
import 'package:hawkr_seller/shared/widgets/hk_glass_chrome_surface.dart';
import 'package:hawkr_seller/shared/widgets/hk_icon.dart';
import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import 'product_setup_picker/picker_exit_dialog.dart';

/// 商品上架 Wizard 共用頂部導覽列，統一 Step 1–5 的 AppBar 實現
///
/// 按下左側 X 按鈕時會先顯示二次確認 Dialog，確認後關閉整個商品上架流程，
/// 返回商品列表（/main/category）。
class ProductSetupTopBar extends StatelessWidget {
  const ProductSetupTopBar({
    super.key,
    required this.title,
    this.subtitle,
    this.trailing,
  });

  /// 主標題（由呼叫方傳入已解析字串，例如 '新增商品' 或 '編輯商品'）
  final String title;

  /// 副標題，null = 不顯示
  final String? subtitle;

  /// 右側自訂 widget（null = 空白占位）
  final Widget? trailing;

  @override
  Widget build(BuildContext context) {
    return HkGlassChromeSurface(
      child: SafeArea(
        bottom: false,
        child: ConstrainedBox(
          constraints: const BoxConstraints(minHeight: 64),
          child: Stack(
            alignment: Alignment.center,
            children: [
              Align(
                alignment: Alignment.centerLeft,
                child: Padding(
                  padding: const EdgeInsets.only(left: HkSpace.xs),
                  child: IconButton(
                    tooltip: '離開商品編輯',
                    onPressed: () => showProductWizardExitDialog(
                      context,
                      onConfirm: () => context.go('/main/category'),
                    ),
                    icon: const HkIcon(HkIconName.close),
                  ),
                ),
              ),
              Padding(
                padding: const EdgeInsets.symmetric(
                  horizontal: HkSpace.sectionLarge,
                  vertical: HkSpace.sm,
                ),
                child: Column(
                  mainAxisAlignment: MainAxisAlignment.center,
                  children: [
                    Text(
                      title,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(
                        fontSize: 18,
                        fontWeight: FontWeight.w700,
                        color: AppColors.textEmphasis,
                      ),
                    ),
                    if (subtitle?.trim().isNotEmpty ?? false) ...[
                      const SizedBox(height: 2),
                      Text(
                        subtitle!,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: const TextStyle(
                          fontSize: 12,
                          fontWeight: FontWeight.w500,
                          color: AppColors.textMuted,
                        ),
                      ),
                    ],
                  ],
                ),
              ),
              Align(
                alignment: Alignment.centerRight,
                child: Padding(
                  padding: const EdgeInsets.only(right: HkSpace.xs),
                  child: ConstrainedBox(
                    constraints: const BoxConstraints(
                      minWidth: 48,
                      minHeight: 48,
                    ),
                    child: trailing ?? const SizedBox(width: 48, height: 48),
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
