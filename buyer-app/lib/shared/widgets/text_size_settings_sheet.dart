import 'package:flutter/material.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../core/layout/app_text_size.dart';
import '../../core/theme/app_colors.dart';
import '../../core/theme/app_typography.dart';
import '../../core/theme/hk_tokens.dart';
import 'common_snackbar.dart';
import 'hk_button_label.dart';
import 'hk_secondary_cta_tabs.dart';
import 'hk_sheet_drag_handle.dart';

Future<void> showTextSizeSettingsSheet(BuildContext context) {
  return showModalBottomSheet<void>(
    context: context,
    useSafeArea: true,
    isScrollControlled: true,
    showDragHandle: false,
    builder: (_) => const TextSizeSettingsSheet(),
  );
}

/// Shared Small / Standard / Large text-size chooser.
///
/// The preference remains owned by [appTextSizeProvider], so opening this from
/// Settings or a top-bar shortcut always updates and persists the same value.
class TextSizeSettingsSheet extends ConsumerWidget {
  const TextSizeSettingsSheet({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final selected = ref.watch(appTextSizeProvider);
    final systemBottomInset = MediaQuery.viewPaddingOf(context).bottom;
    final bottomPadding = systemBottomInset > HkSpace.xxl
        ? systemBottomInset
        : HkSpace.xxl;
    return Padding(
      // The page MediaQuery also reserves space for the floating navbar.
      // Only the hardware inset belongs inside a modal sheet.
      padding: EdgeInsets.only(bottom: bottomPadding),
      child: SingleChildScrollView(
        key: const ValueKey('text-size-settings-sheet'),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            const HkSheetDragHandle(
              key: ValueKey('text-size-sheet-drag-handle'),
            ),
            Padding(
              padding: const EdgeInsets.fromLTRB(
                HkSpace.xxl,
                HkSpace.sm,
                HkSpace.xxl,
                0,
              ),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  Text('字體大小', style: AppTypography.h1),
                  const SizedBox(height: HkSpace.xs),
                  Text(
                    '依閱讀習慣調整文字；按鈕與畫面比例維持穩定。',
                    style: AppTypography.metaMuted,
                  ),
                  const SizedBox(height: HkSpace.xl),
                  Semantics(
                    container: true,
                    label: '字體預覽',
                    child: Container(
                      padding: const EdgeInsets.all(HkSpace.lg),
                      decoration: BoxDecoration(
                        color: AppColors.surfacePanel,
                        borderRadius: BorderRadius.circular(HkRadius.lg),
                        border: Border.all(color: AppColors.cardBorder),
                      ),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            '每日新鮮，剛剛好的閱讀大小',
                            style: AppTypography.sectionTitle,
                          ),
                          const SizedBox(height: HkSpace.xs),
                          Text(
                            '商品資訊、價格與配送狀態都會依這個設定顯示。',
                            style: AppTypography.body,
                          ),
                        ],
                      ),
                    ),
                  ),
                  const SizedBox(height: HkSpace.lg),
                  HkSecondaryCtaTabs(
                    labels: AppTextSize.values
                        .map((preference) => preference.label)
                        .toList(growable: false),
                    selectedIndex: AppTextSize.values.indexOf(selected),
                    itemKeyPrefix: 'settings-text-size',
                    onSelected: (index) async {
                      final saved = await ref
                          .read(appTextSizeProvider.notifier)
                          .setTextSize(AppTextSize.values[index]);
                      if (!saved && context.mounted) {
                        showFloatingToast(
                          context,
                          '字體大小儲存失敗，請再試一次',
                          isSuccess: false,
                        );
                      }
                    },
                  ),
                  const SizedBox(height: HkSpace.xl),
                  FilledButton(
                    onPressed: () => Navigator.of(context).pop(),
                    child: const HkButtonLabel('完成'),
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}
