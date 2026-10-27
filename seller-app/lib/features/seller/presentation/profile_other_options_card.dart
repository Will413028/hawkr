import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../core/router/app_router.dart';
import '../../../core/session/my_store.dart';
import '../../../core/theme/app_colors.dart';
import '../../../core/theme/hk_tokens.dart';
import '../../../shared/icons/app_icon_colors.dart';
import '../../../shared/widgets/app_confirm_dialog.dart';
import '../../../shared/widgets/hk_button_label.dart';
import '../../../shared/widgets/hk_content_card.dart';
import '../../../shared/widgets/hk_icon.dart';
import '../../auth/presentation/auth_controller.dart';
import '../../settings/presentation/text_size_settings_sheet.dart';
import 'widgets/option_row.dart';

/// 設定首頁的營運入口。
///
/// 入口圖示使用語意 icon。功能路由維持不變，
/// 同時把通知收件匣與通知偏好分開，避免「看通知」與「關閉通知」混在一起。
class ProfileOtherOptionsCard extends StatelessWidget {
  const ProfileOtherOptionsCard({
    super.key,
    required this.onShippingTap,
    this.shippingGate = ShippingGate.unknown,
    this.notifUnread = 0,
  });

  final VoidCallback onShippingTap;
  final ShippingGate shippingGate;
  final int notifUnread;

  @override
  Widget build(BuildContext context) {
    final items = <OptionRow>[
      OptionRow(
        icon: HkIconName.truck,
        label: '物流與退貨',
        description: switch (shippingGate) {
          ShippingGate.certification => '通過商家認證後才可設定配送',
          _ => '管理宅配、超商與寄退件地址',
        },
        statusLabel: switch (shippingGate) {
          ShippingGate.open => null,
          ShippingGate.certification => '需認證',
          _ => '未營業',
        },
        onTap: onShippingTap,
      ),
      OptionRow(
        icon: HkIconName.document,
        label: '待回覆詢價',
        description: '查看買家發出的詢價，點進去回價',
        onTap: () => context.push(ProcurementRoutes.sellerPublications),
      ),
      OptionRow(
        icon: HkIconName.notification,
        label: '通知中心',
        description: notifUnread > 0 ? '有 $notifUnread 則尚未閱讀' : '查看訂單與系統事件',
        badgeCount: notifUnread,
        onTap: () => context.push('/notifications'),
      ),
      OptionRow(
        icon: HkIconName.settings,
        label: '通知偏好',
        description: '選擇要接收的營運提醒',
        onTap: () => context.push('/notifications/settings'),
      ),
      OptionRow(
        icon: HkIconName.textSize,
        label: '字體大小',
        description: '選擇小、中或大字，依閱讀習慣調整',
        onTap: () => showTextSizeSettingsSheet(context),
      ),
      OptionRow(
        icon: HkIconName.report,
        label: '我的檢舉',
        description: '查看已送出的檢舉紀錄與處理進度',
        onTap: () => context.push('/reports/me'),
      ),
      OptionRow(
        icon: HkIconName.shield,
        label: '措施與申覆',
        description: '查看平台的措施、期間與申覆進度',
        onTap: () => context.push(SellerRoutes.measures),
      ),
      OptionRow(
        icon: HkIconName.block,
        label: '封鎖名單',
        description: '管理已封鎖的使用者',
        onTap: () => context.push('/blocks'),
      ),
      OptionRow(
        icon: HkIconName.support,
        label: '幫助中心',
        description: '常見問題、客服與版本資訊',
        onTap: () => context.push('/settings/help'),
      ),
    ];

    return HkContentCard(
      padding: EdgeInsets.zero,
      child: Column(
        children: [
          for (var index = 0; index < items.length; index++) ...[
            items[index],
            if (index != items.length - 1)
              const Divider(
                height: 1,
                indent: HkSpace.lg,
                endIndent: HkSpace.lg,
                color: AppColors.dividerLight,
              ),
          ],
        ],
      ),
    );
  }
}

class ProfileLogoutButton extends ConsumerWidget {
  const ProfileLogoutButton({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return SizedBox(
      width: double.infinity,
      child: OutlinedButton.icon(
        onPressed: () => _handleLogout(context, ref),
        icon: const HkIcon(HkIconName.logout, color: AppIconColors.danger),
        label: const HkButtonLabel('登出'),
      ),
    );
  }

  Future<void> _handleLogout(BuildContext context, WidgetRef ref) async {
    final shouldLogout = await showAppConfirmDialog(
      context,
      title: '要登出賣家帳號嗎？',
      message: '未送出的表單內容可能不會保留。',
      confirmText: '登出',
      confirmTextColor: AppColors.danger,
    );
    if (!shouldLogout) return;

    await ref.read(sessionProvider.notifier).logout();
    if (!context.mounted) return;
    context.go('/auth');
  }
}
