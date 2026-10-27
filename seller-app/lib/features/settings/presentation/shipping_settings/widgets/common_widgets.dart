import 'package:flutter/material.dart';

import '../../../../../core/theme/app_colors.dart';
import '../../../../../core/theme/app_typography.dart';
import '../../../../../core/theme/hk_tokens.dart';
import '../../../../../shared/icons/app_icon_colors.dart';
import '../../../../../shared/widgets/hk_button_label.dart';
import '../../../../../shared/widgets/hk_icon.dart';
import '../../../../../shared/widgets/hk_tap_target.dart';
import '../../../../../shared/widgets/temperature_tag.dart';

/// 物流方式與溫層共用的勾選控制。
class CustomCheckbox extends StatelessWidget {
  const CustomCheckbox({
    super.key,
    required this.checked,
    required this.onTap,
    this.enabled = true,
    this.semanticLabel = '物流方式',
  });

  final bool checked;
  final VoidCallback onTap;
  final bool enabled;
  final String semanticLabel;

  @override
  Widget build(BuildContext context) {
    return HkTapTarget(
      semanticLabel: '${checked ? '取消' : '選擇'}$semanticLabel',
      onTap: enabled ? onTap : null,
      child: AnimatedContainer(
        duration: HkMotion.duration,
        curve: HkMotion.ease,
        width: 24,
        height: 24,
        decoration: BoxDecoration(
          color: checked ? AppColors.actionPrimary : AppColors.surfaceCard,
          borderRadius: BorderRadius.circular(HkRadius.xs),
          border: Border.all(
            color: checked ? AppColors.actionPrimary : AppColors.cardBorder,
            width: 1.5,
          ),
          boxShadow: HkShadows.xs,
        ),
        child: checked
            ? const Center(
                child: HkIcon(
                  HkIconName.check,
                  size: 16,
                  color: AppIconColors.onBrand,
                ),
              )
            : null,
      ),
    );
  }
}

/// 物流品牌圖示的小型表面。
class IconBox extends StatelessWidget {
  const IconBox({super.key, required this.icon});

  final HkIconName icon;

  @override
  Widget build(BuildContext context) {
    return DecoratedBox(
      decoration: BoxDecoration(
        color: AppColors.neutralWhite.withValues(alpha: 0.78),
        borderRadius: BorderRadius.circular(HkRadius.sm),
        border: Border.all(color: AppColors.cardBorder),
      ),
      child: SizedBox.square(
        dimension: 36,
        child: Padding(
          padding: const EdgeInsets.all(HkSpace.xs),
          child: Center(
            child: HkIcon(icon, size: 20, color: AppIconColors.defaultColor),
          ),
        ),
      ),
    );
  }
}

/// 可點選的配送溫層標籤，視覺維持精簡但觸控區至少 48pt。
class TemperaturePill extends StatelessWidget {
  const TemperaturePill({
    super.key,
    required this.label,
    required this.code,
    required this.active,
    required this.onTap,
    this.enabled = true,
  });

  final String label;
  final String code;
  final bool active;
  final VoidCallback onTap;
  final bool enabled;

  @override
  Widget build(BuildContext context) {
    return AnimatedOpacity(
      duration: HkMotion.duration,
      opacity: enabled ? 1 : 0.36,
      child: TemperatureTag.selectable(
        code: code,
        label: label,
        isSelected: active,
        onTap: enabled ? onTap : null,
      ),
    );
  }
}

/// 7-11 單一溫層選項列。
class CvsRow extends StatelessWidget {
  const CvsRow({
    super.key,
    required this.tempLabel,
    required this.code,
    required this.checked,
    required this.onTap,
  });

  final String tempLabel;
  final String code;
  final bool checked;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Semantics(
      button: true,
      selected: checked,
      label: '7-11 店到店 $tempLabel',
      child: Material(
        color: Colors.transparent,
        child: InkWell(
          onTap: onTap,
          borderRadius: BorderRadius.circular(HkRadius.md),
          child: ConstrainedBox(
            constraints: const BoxConstraints(minHeight: HkControl.minTap),
            child: Row(
              children: [
                CustomCheckbox(
                  checked: checked,
                  onTap: onTap,
                  semanticLabel: '7-11 $tempLabel',
                ),
                const SizedBox(width: HkSpace.sm),
                const IconBox(icon: HkIconName.store),
                const SizedBox(width: HkSpace.sm),
                Expanded(
                  child: Text('7-11 店到店', style: AppTypography.bodyBold),
                ),
                TemperatureTag(code: code),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

/// 物流表單輸入框。欄位名稱由外層顯示，placeholder 只提供範例。
class CustomInput extends StatelessWidget {
  const CustomInput({
    super.key,
    required this.controller,
    required this.hint,
    this.width,
    this.maxLength,
    this.keyboardType,
    this.textInputAction,
    this.enabled = true,
    this.errorText,
  });

  final TextEditingController controller;
  final String hint;
  final double? width;
  final int? maxLength;
  final TextInputType? keyboardType;
  final TextInputAction? textInputAction;
  final bool enabled;
  final String? errorText;

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      width: width,
      child: ConstrainedBox(
        constraints: const BoxConstraints(minHeight: HkControl.minTap),
        child: TextField(
          controller: controller,
          enabled: enabled,
          maxLength: maxLength,
          keyboardType: keyboardType,
          textInputAction: textInputAction,
          style: AppTypography.body,
          decoration: InputDecoration(
            hintText: hint,
            errorText: errorText,
            errorMaxLines: 2,
            counterText: '',
            isDense: true,
            filled: true,
            fillColor: enabled
                ? AppColors.neutralWhite.withValues(alpha: 0.72)
                : AppColors.gray100,
            contentPadding: const EdgeInsets.symmetric(
              horizontal: HkSpace.md,
              vertical: HkSpace.md,
            ),
            border: OutlineInputBorder(
              borderRadius: BorderRadius.circular(HkRadius.md),
              borderSide: const BorderSide(color: AppColors.cardBorder),
            ),
            enabledBorder: OutlineInputBorder(
              borderRadius: BorderRadius.circular(HkRadius.md),
              borderSide: const BorderSide(color: AppColors.cardBorder),
            ),
            focusedBorder: OutlineInputBorder(
              borderRadius: BorderRadius.circular(HkRadius.md),
              borderSide: const BorderSide(
                color: AppColors.actionPrimary,
                width: 1.5,
              ),
            ),
          ),
        ),
      ),
    );
  }
}

/// 自訂下拉觸發器，實際選單由呼叫端控制。
class CustomDropdown extends StatelessWidget {
  const CustomDropdown({
    super.key,
    required this.text,
    required this.hasValue,
    required this.onTap,
  });

  final String text;
  final bool hasValue;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Semantics(
      button: true,
      label: text,
      child: Material(
        color: Colors.transparent,
        child: InkWell(
          onTap: onTap,
          borderRadius: BorderRadius.circular(HkRadius.md),
          child: Container(
            height: HkControl.minTap,
            padding: const EdgeInsets.symmetric(horizontal: HkSpace.md),
            decoration: BoxDecoration(
              color: AppColors.neutralWhite.withValues(alpha: 0.72),
              borderRadius: BorderRadius.circular(HkRadius.md),
              border: Border.all(color: AppColors.cardBorder),
            ),
            child: Row(
              children: [
                Expanded(
                  child: Text(
                    text,
                    style: AppTypography.body.copyWith(
                      color: hasValue
                          ? AppColors.textPrimary
                          : AppColors.textSecondary,
                    ),
                  ),
                ),
                const HkIcon(
                  HkIconName.chevronDown,
                  size: 20,
                  color: AppIconColors.defaultColor,
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

/// 物流設定的主操作按鈕。
class SaveButton extends StatelessWidget {
  const SaveButton({
    super.key,
    required this.canSave,
    required this.saving,
    this.onSave,
    this.buttonText = '儲存物流設定',
  });

  final bool canSave;
  final bool saving;
  final VoidCallback? onSave;
  final String buttonText;

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      width: double.infinity,
      height: 52,
      child: FilledButton(
        onPressed: canSave && !saving ? onSave : null,
        child: saving
            ? const SizedBox.square(
                dimension: 20,
                child: CircularProgressIndicator(
                  strokeWidth: 2,
                  color: AppIconColors.onBrand,
                ),
              )
            : HkButtonLabel(buttonText),
      ),
    );
  }
}
