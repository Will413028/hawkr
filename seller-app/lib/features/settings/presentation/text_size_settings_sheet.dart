import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/layout/app_text_size.dart';
import '../../../core/theme/app_colors.dart';
import '../../../core/theme/app_typography.dart';
import '../../../core/theme/hk_tokens.dart';
import '../../../shared/utils/floating_toast.dart';
import '../../../shared/widgets/hk_button_label.dart';
import '../../../shared/widgets/hk_content_card.dart';
import '../../../shared/widgets/glass_tab_bar.dart';

Future<void> showTextSizeSettingsSheet(BuildContext context) {
  return showModalBottomSheet<void>(
    context: context,
    useSafeArea: true,
    isScrollControlled: true,
    showDragHandle: true,
    builder: (_) => const TextSizeSettingsSheet(),
  );
}

/// Device-level display preference shared by every seller surface.
class TextSizeSettingsSheet extends ConsumerWidget {
  const TextSizeSettingsSheet({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final selected = ref.watch(appTextSizeProvider);

    return SafeArea(
      top: false,
      child: SingleChildScrollView(
        padding: const EdgeInsets.fromLTRB(
          HkSpace.xxl,
          HkSpace.sm,
          HkSpace.xxl,
          HkSpace.xxl,
        ),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Text('字體大小', style: AppTypography.h1),
            const SizedBox(height: HkSpace.xs),
            Text('依閱讀習慣調整文字；版面、圖示與操作區域維持原本比例。', style: AppTypography.metaMuted),
            const SizedBox(height: HkSpace.xl),
            Semantics(
              container: true,
              label: '字體預覽',
              child: HkContentCard(
                backgroundColor: AppColors.surfaceMuted,
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text('清楚掌握今日營運狀況', style: AppTypography.sectionTitle),
                    const SizedBox(height: HkSpace.xs),
                    Text('商品、訂單與物流資訊都會依這個設定顯示。', style: AppTypography.body),
                  ],
                ),
              ),
            ),
            const SizedBox(height: HkSpace.lg),
            HkGlassTabBar(
              labels: AppTextSize.values
                  .map((preference) => preference.label)
                  .toList(growable: false),
              selectedIndex: AppTextSize.values.indexOf(selected),
              onTap: (index) async {
                final saved = await ref
                    .read(appTextSizeProvider.notifier)
                    .setTextSize(AppTextSize.values[index]);
                if (!saved && context.mounted) {
                  FloatingToast.error(context, '字體大小儲存失敗，請再試一次');
                }
              },
              labelKeyPrefix: 'settings-text-size-',
              mutuallyExclusiveSemantics: true,
            ),
            const SizedBox(height: HkSpace.xl),
            FilledButton(
              onPressed: () => Navigator.of(context).pop(),
              child: const HkButtonLabel('完成'),
            ),
          ],
        ),
      ),
    );
  }
}
