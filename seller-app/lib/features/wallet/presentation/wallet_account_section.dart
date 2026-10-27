import 'package:flutter/material.dart';
import 'package:flutter_hooks/flutter_hooks.dart';

import '../../../core/theme/app_colors.dart';
import '../../../core/theme/app_typography.dart';
import '../../../core/theme/hk_tokens.dart';
import '../../../shared/widgets/hk_content_card.dart';
import '../../../shared/widgets/hk_icon.dart';
import '../../../shared/widgets/hk_status_badge.dart';

/// 錢包頁的「帳戶與認證」：商家認證通過後顯示的商家認證列，點開看認證說明。
/// 內容和 website 的 `AccountCertSection` 同一套語意與文字。
class WalletAccountSection extends HookWidget {
  const WalletAccountSection({super.key});

  @override
  Widget build(BuildContext context) {
    final certOpen = useState(false);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('帳戶與認證', style: AppTypography.sectionTitle),
        const SizedBox(height: HkSpace.md),
        _AccountRow(
          key: const ValueKey('wallet-cert-row'),
          title: '商家認證',
          subtitle: '身分、銀行帳戶、PayUni 資料',
          trailing: const HkStatusBadge(
            label: '已通過',
            tone: HkStatusTone.success,
            icon: HkIconName.shield,
          ),
          open: certOpen.value,
          onTap: () => certOpen.value = !certOpen.value,
        ),
        if (certOpen.value)
          _Expansion(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('商家認證已通過', style: AppTypography.bodyBold),
                const SizedBox(height: HkSpace.xs),
                const Text(
                  '身分、銀行帳戶與 PayUni 收款皆已完成驗證，可正常收款與出貨。撥款週期為訂單完成 + 7 天。',
                ),
              ],
            ),
          ),
      ],
    );
  }
}

class _AccountRow extends StatelessWidget {
  const _AccountRow({
    super.key,
    required this.title,
    required this.subtitle,
    required this.trailing,
    required this.open,
    required this.onTap,
  });

  final String title;
  final String subtitle;
  final Widget trailing;
  final bool open;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Semantics(
      container: true,
      expanded: open,
      child: HkContentCard(
        onTap: onTap,
        semanticLabel: title,
        child: Row(
          children: [
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(title, style: AppTypography.bodyBold),
                  const SizedBox(height: HkSpace.xs),
                  Text(
                    subtitle,
                    style: AppTypography.caption.copyWith(
                      color: AppColors.textSecondary,
                    ),
                  ),
                ],
              ),
            ),
            const SizedBox(width: HkSpace.sm),
            trailing,
            const SizedBox(width: HkSpace.sm),
            AnimatedRotation(
              turns: open ? 0.25 : 0,
              duration: Duration.zero,
              child: const HkIcon(HkIconName.chevronRight, size: 18),
            ),
          ],
        ),
      ),
    );
  }
}

class _Expansion extends StatelessWidget {
  const _Expansion({required this.child});

  final Widget child;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(top: HkSpace.sm),
      child: HkContentCard(
        backgroundColor: AppColors.surfaceMuted,
        child: child,
      ),
    );
  }
}
