import 'package:flutter/material.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../../../core/theme/app_colors.dart';
import '../../../../../core/theme/app_typography.dart';
import '../../../../../core/theme/hk_tokens.dart';
import '../../../../../shared/icons/app_icon_colors.dart';
import '../../../../../shared/utils/floating_toast.dart';
import '../../../../../shared/widgets/app_dropdown.dart';
import '../../../../../shared/widgets/hk_button_label.dart';
import '../../../../../shared/widgets/hk_content_card.dart';
import '../../../../../shared/widgets/hk_icon.dart';
import '../../../data/tgos_address_service.dart';
import '../../../data/shipping_sender.dart';
import '../data/taiwan_cities_data.dart';
import 'address_autocomplete_field.dart';
import 'common_widgets.dart';

/// 黑貓宅急便設定。
///
/// 使用單一內容卡與自然高度，避免舊版 Stack 抽屜在文字放大或窄螢幕時溢位。
class TaqbinShippingSection extends HookConsumerWidget {
  const TaqbinShippingSection({
    super.key,
    required this.expanded,
    this.onToggleExpanded,
    this.enabled = false,
    required this.frozen,
    required this.chilled,
    required this.ambient,
    required this.nameCtrl,
    required this.phoneCtrl,
    required this.city,
    required this.district,
    required this.addressCtrl,
    this.onCitySelected,
    this.onDistrictSelected,
    required this.addressVerified,
    required this.addressVerifying,
    this.onAddressSelected,
    this.onAddressManualEdit,
    this.onVerifyAddress,
  });

  final ValueNotifier<bool> expanded;
  final VoidCallback? onToggleExpanded;
  final bool enabled;
  final ValueNotifier<bool> frozen;
  final ValueNotifier<bool> chilled;
  final ValueNotifier<bool> ambient;
  final TextEditingController nameCtrl;
  final TextEditingController phoneCtrl;
  final ValueNotifier<String?> city;
  final ValueNotifier<String?> district;
  final TextEditingController addressCtrl;
  final ValueChanged<String>? onCitySelected;
  final ValueChanged<String>? onDistrictSelected;
  final ValueNotifier<bool> addressVerified;
  final ValueNotifier<bool> addressVerifying;
  final ValueChanged<AddressSuggestion>? onAddressSelected;
  final VoidCallback? onAddressManualEdit;
  final VoidCallback? onVerifyAddress;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    useListenable(addressCtrl);
    final hasTaqbin = frozen.value || chilled.value || ambient.value;
    final canVerify =
        !enabled &&
        !addressVerified.value &&
        !addressVerifying.value &&
        addressCtrl.text.trim().isNotEmpty &&
        city.value != null &&
        district.value != null;

    return HkContentCard(
      key: const ValueKey('shipping-taqbin-card'),
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
                        checked: hasTaqbin,
                        enabled: !enabled,
                        onTap: () {
                          final next = !hasTaqbin;
                          frozen.value = next;
                          chilled.value = next;
                          ambient.value = next;
                        },
                        semanticLabel: '黑貓宅急便',
                      ),
                      const SizedBox(width: HkSpace.sm),
                      const IconBox(icon: HkIconName.truck),
                      const SizedBox(width: HkSpace.md),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text('黑貓宅急便', style: AppTypography.sectionTitle),
                            const SizedBox(height: HkSpace.xs),
                            Text(
                              enabled ? '已開通，資料由 PayUni 管理' : '宅配到府與退貨地址',
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
                AbsorbPointer(
                  absorbing: enabled,
                  child: Opacity(
                    opacity: enabled ? 0.76 : 1,
                    child: Padding(
                      padding: const EdgeInsets.all(HkSpace.lg),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          const _FieldLabel('配送溫層'),
                          const SizedBox(height: HkSpace.sm),
                          Row(
                            children: [
                              Expanded(
                                child: TemperaturePill(
                                  label: '冷凍',
                                  code: 'FROZEN',
                                  active: frozen.value,
                                  enabled: !enabled,
                                  onTap: () => frozen.value = !frozen.value,
                                ),
                              ),
                              const SizedBox(width: HkSpace.sm),
                              Expanded(
                                child: TemperaturePill(
                                  label: '冷藏',
                                  code: 'CHILLED',
                                  active: chilled.value,
                                  enabled: !enabled,
                                  onTap: () => chilled.value = !chilled.value,
                                ),
                              ),
                              const SizedBox(width: HkSpace.sm),
                              Expanded(
                                child: TemperaturePill(
                                  label: '常溫',
                                  code: 'AMBIENT',
                                  active: ambient.value,
                                  enabled: !enabled,
                                  onTap: () => ambient.value = !ambient.value,
                                ),
                              ),
                            ],
                          ),
                          const SizedBox(height: HkSpace.xl),
                          Row(
                            children: [
                              const HkIcon(
                                HkIconName.location,
                                size: 22,
                                color: AppIconColors.active,
                              ),
                              const SizedBox(width: HkSpace.sm),
                              Text(
                                '收貨／退貨地址',
                                style: AppTypography.sectionTitle,
                              ),
                            ],
                          ),
                          const SizedBox(height: HkSpace.md),
                          _AdaptivePair(
                            first: _LabeledField(
                              label: '聯絡人姓名',
                              child: ValueListenableBuilder<TextEditingValue>(
                                valueListenable: nameCtrl,
                                builder: (context, value, _) => CustomInput(
                                  controller: nameCtrl,
                                  hint: '例如：王小明',
                                  enabled: !enabled,
                                  errorText: senderNameLengthError(
                                    value.text,
                                    maxDisplayLength:
                                        taqbinSenderNameMaxDisplayLength,
                                    fieldLabel: '聯絡人姓名',
                                  ),
                                  textInputAction: TextInputAction.next,
                                ),
                              ),
                            ),
                            second: _LabeledField(
                              label: '手機號碼',
                              child: CustomInput(
                                controller: phoneCtrl,
                                hint: '09xxxxxxxx',
                                enabled: !enabled,
                                keyboardType: TextInputType.phone,
                                textInputAction: TextInputAction.next,
                              ),
                            ),
                          ),
                          const SizedBox(height: HkSpace.md),
                          _AdaptivePair(
                            first: _LabeledField(
                              label: '縣市',
                              child: AppDropdown(
                                items: taiwanCities.keys.toList(),
                                selectedItem: city.value,
                                placeholder: '選擇縣市',
                                height: HkControl.minTap,
                                onSelected: (value) {
                                  onCitySelected?.call(value);
                                },
                              ),
                            ),
                            second: _LabeledField(
                              label: '行政區',
                              child: AppDropdown(
                                items: taiwanCities[city.value] ?? const [],
                                selectedItem: district.value,
                                placeholder: '選擇行政區',
                                height: HkControl.minTap,
                                onSelected: (value) {
                                  if (city.value == null) {
                                    FloatingToast.show(context, '請先選擇縣市');
                                    return;
                                  }
                                  onDistrictSelected?.call(value);
                                },
                              ),
                            ),
                          ),
                          const SizedBox(height: HkSpace.md),
                          const _FieldLabel('路名與門牌'),
                          const SizedBox(height: HkSpace.sm),
                          LayoutBuilder(
                            builder: (context, constraints) {
                              final address = AddressAutocompleteField(
                                controller: addressCtrl,
                                county: city.value,
                                district: district.value,
                                enabled: !enabled,
                                verified: addressVerified.value,
                                onSuggestionSelected: onAddressSelected,
                                onManualEdit: (_) =>
                                    onAddressManualEdit?.call(),
                              );
                              final button = SizedBox(
                                height: HkControl.minTap,
                                child: FilledButton.tonalIcon(
                                  onPressed: canVerify ? onVerifyAddress : null,
                                  icon: addressVerifying.value
                                      ? const SizedBox.square(
                                          dimension: 18,
                                          child: CircularProgressIndicator(
                                            strokeWidth: 2,
                                          ),
                                        )
                                      : HkIcon(
                                          addressVerified.value
                                              ? HkIconName.check
                                              : HkIconName.location,
                                          size: 20,
                                        ),
                                  label: HkButtonLabel(
                                    addressVerified.value ? '已驗證' : '驗證地址',
                                  ),
                                ),
                              );
                              if (constraints.maxWidth < 420) {
                                return Column(
                                  crossAxisAlignment:
                                      CrossAxisAlignment.stretch,
                                  children: [
                                    address,
                                    const SizedBox(height: HkSpace.sm),
                                    button,
                                  ],
                                );
                              }
                              return Row(
                                crossAxisAlignment: CrossAxisAlignment.start,
                                children: [
                                  Expanded(child: address),
                                  const SizedBox(width: HkSpace.sm),
                                  button,
                                ],
                              );
                            },
                          ),
                        ],
                      ),
                    ),
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

class _AdaptivePair extends StatelessWidget {
  const _AdaptivePair({required this.first, required this.second});

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

class _LabeledField extends StatelessWidget {
  const _LabeledField({required this.label, required this.child});

  final String label;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        _FieldLabel(label),
        const SizedBox(height: HkSpace.sm),
        child,
      ],
    );
  }
}

class _FieldLabel extends StatelessWidget {
  const _FieldLabel(this.label);

  final String label;

  @override
  Widget build(BuildContext context) {
    return Text(
      label,
      style: AppTypography.meta.copyWith(fontWeight: FontWeight.w700),
    );
  }
}
