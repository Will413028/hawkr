import 'package:hawkr_seller/shared/widgets/hk_button_label.dart';
import 'package:flutter/material.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../../../core/theme/app_colors.dart';
import '../../../../../core/theme/app_typography.dart';
import '../../../../../core/theme/hk_tokens.dart';
import '../../../../../shared/icons/app_icon_colors.dart';
import '../../../../../shared/widgets/hk_content_card.dart';
import '../../../../../shared/widgets/hk_icon.dart';
import '../../../data/shipping_sender.dart';
import 'common_widgets.dart';

/// 7-11 店到店設定。
///
/// 以自然高度內容卡取代舊版 Stack 抽屜，窄螢幕與大字級皆可正常換行。
class CvsShippingSection extends HookConsumerWidget {
  const CvsShippingSection({
    super.key,
    required this.expanded,
    this.onToggleExpanded,
    this.enabled = false,
    this.senderLocked = false,
    required this.frozen,
    required this.ambient,
    required this.nameCtrl,
    required this.phoneCtrl,
    required this.selectedStoreName,
    required this.selectedStoreCode,
    this.onPickStore,
  });

  final ValueNotifier<bool> expanded;
  final VoidCallback? onToggleExpanded;
  final bool enabled;

  /// 寄件人與退貨門市已交給 PayUni（4302、4303），不可修改。
  final bool senderLocked;
  final ValueNotifier<bool> frozen;
  final ValueNotifier<bool> ambient;
  final TextEditingController nameCtrl;
  final TextEditingController phoneCtrl;
  final ValueNotifier<String?> selectedStoreName;
  final ValueNotifier<String?> selectedStoreCode;
  final Future<void> Function()? onPickStore;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final hasCvs = frozen.value || ambient.value;

    return HkContentCard(
      key: const ValueKey('shipping-cvs-card'),
      padding: EdgeInsets.zero,
      radius: HkRadius.xl,
      elevated: true,
      child: Column(
        children: [
          Material(
            color: Colors.transparent,
            child: InkWell(
              onTap: onToggleExpanded ?? () => expanded.value = !expanded.value,
              child: ConstrainedBox(
                constraints: const BoxConstraints(minHeight: 76),
                child: Padding(
                  padding: const EdgeInsets.symmetric(
                    horizontal: HkSpace.lg,
                    vertical: HkSpace.md,
                  ),
                  child: Row(
                    children: [
                      CustomCheckbox(
                        checked: hasCvs,
                        onTap: () {
                          final next = !hasCvs;
                          frozen.value = next;
                          ambient.value = next;
                        },
                        semanticLabel: '7-11 店到店',
                      ),
                      const SizedBox(width: HkSpace.sm),
                      const IconBox(icon: HkIconName.store),
                      const SizedBox(width: HkSpace.md),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text('7-11 店到店', style: AppTypography.sectionTitle),
                            const SizedBox(height: HkSpace.xs),
                            Text(
                              enabled ? '已開通，可調整配送溫層' : '超商取貨與退貨門市',
                              style: AppTypography.metaMuted,
                            ),
                          ],
                        ),
                      ),
                      AnimatedRotation(
                        duration: HkMotion.duration,
                        turns: expanded.value ? 0.5 : 0,
                        child: const HkIcon(
                          HkIconName.chevronDown,
                          size: 24,
                          color: AppIconColors.defaultColor,
                        ),
                      ),
                    ],
                  ),
                ),
              ),
            ),
          ),
          AnimatedCrossFade(
            duration: HkMotion.duration,
            crossFadeState: expanded.value
                ? CrossFadeState.showSecond
                : CrossFadeState.showFirst,
            firstChild: const SizedBox(width: double.infinity),
            secondChild: Column(
              children: [
                const Divider(height: 1, color: AppColors.cardBorder),
                Padding(
                  padding: const EdgeInsets.all(HkSpace.lg),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text('配送溫層', style: _labelStyle),
                      const SizedBox(height: HkSpace.sm),
                      DecoratedBox(
                        decoration: BoxDecoration(
                          color: AppColors.surfaceMuted,
                          borderRadius: BorderRadius.circular(HkRadius.lg),
                        ),
                        child: Padding(
                          padding: const EdgeInsets.symmetric(
                            horizontal: HkSpace.md,
                            vertical: HkSpace.xs,
                          ),
                          child: Column(
                            children: [
                              CvsRow(
                                tempLabel: '冷凍',
                                code: 'FROZEN',
                                checked: frozen.value,
                                onTap: () => frozen.value = !frozen.value,
                              ),
                              const Divider(
                                height: 1,
                                color: AppColors.cardBorder,
                              ),
                              CvsRow(
                                tempLabel: '常溫',
                                code: 'AMBIENT',
                                checked: ambient.value,
                                onTap: () => ambient.value = !ambient.value,
                              ),
                            ],
                          ),
                        ),
                      ),
                      const SizedBox(height: HkSpace.xl),
                      Row(
                        children: [
                          const HkIcon(
                            HkIconName.store,
                            size: 22,
                            color: AppIconColors.active,
                          ),
                          const SizedBox(width: HkSpace.sm),
                          Text('退貨門市', style: AppTypography.sectionTitle),
                        ],
                      ),
                      const SizedBox(height: HkSpace.md),
                      _CvsAdaptivePair(
                        first: _CvsLabeledField(
                          label: '退貨人姓名',
                          child: ValueListenableBuilder<TextEditingValue>(
                            valueListenable: nameCtrl,
                            builder: (context, value, _) => CustomInput(
                              controller: nameCtrl,
                              hint: '例如：王小明',
                              enabled: !senderLocked,
                              errorText: senderNameLengthError(
                                value.text,
                                maxDisplayLength: cvsSenderNameMaxDisplayLength,
                                fieldLabel: '退貨人姓名',
                              ),
                              textInputAction: TextInputAction.next,
                            ),
                          ),
                        ),
                        second: _CvsLabeledField(
                          label: '手機號碼',
                          child: CustomInput(
                            controller: phoneCtrl,
                            hint: '09xxxxxxxx',
                            enabled: !senderLocked,
                            keyboardType: TextInputType.phone,
                            textInputAction: TextInputAction.done,
                          ),
                        ),
                      ),
                      const SizedBox(height: HkSpace.md),
                      Text('門市', style: _labelStyle),
                      const SizedBox(height: HkSpace.sm),
                      SizedBox(
                        width: double.infinity,
                        height: HkControl.minTap,
                        child: OutlinedButton.icon(
                          onPressed: senderLocked ? null : onPickStore,
                          icon: const HkIcon(HkIconName.location, size: 20),
                          label: Align(
                            alignment: Alignment.centerLeft,
                            child: HkButtonLabel(
                              selectedStoreName.value ?? '選擇 7-11 退貨門市',
                            ),
                          ),
                        ),
                      ),
                      if (selectedStoreCode.value case final storeCode?) ...[
                        const SizedBox(height: HkSpace.xs),
                        Text('門市代碼 $storeCode', style: AppTypography.metaMuted),
                      ],
                    ],
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

TextStyle get _labelStyle =>
    AppTypography.meta.copyWith(fontWeight: FontWeight.w700);

class _CvsAdaptivePair extends StatelessWidget {
  const _CvsAdaptivePair({required this.first, required this.second});

  final Widget first;
  final Widget second;

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(
      builder: (context, constraints) {
        if (constraints.maxWidth < 420) {
          return Column(
            children: [
              first,
              const SizedBox(height: HkSpace.md),
              second,
            ],
          );
        }
        return Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Expanded(child: first),
            const SizedBox(width: HkSpace.md),
            Expanded(child: second),
          ],
        );
      },
    );
  }
}

class _CvsLabeledField extends StatelessWidget {
  const _CvsLabeledField({required this.label, required this.child});

  final String label;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(label, style: _labelStyle),
        const SizedBox(height: HkSpace.sm),
        child,
      ],
    );
  }
}
