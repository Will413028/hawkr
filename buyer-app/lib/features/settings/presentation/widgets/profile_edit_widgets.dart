import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/app_typography.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:hawkr_buyer/shared/widgets/app_dropdown.dart';
import 'package:hawkr_buyer/shared/widgets/hk_button_label.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:hawkr_buyer/shared/widgets/hk_brand_loader.dart';
import 'package:hawkr_buyer/shared/widgets/hk_placeholder.dart';

const _fieldBorderRadius = BorderRadius.all(Radius.circular(HkRadius.md));

/// 頭像預覽與明確的更換入口。
class ProfileAvatarEditor extends StatelessWidget {
  const ProfileAvatarEditor({
    super.key,
    required this.avatar,
    required this.onPressed,
    this.displayName = '',
    this.busy = false,
  });

  final ImageProvider? avatar;
  final VoidCallback? onPressed;
  final String displayName;
  final bool busy;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final resolvedName = displayName.trim().isEmpty ? 'Hawkr 會員' : displayName;

    final avatarPreview = Stack(
      clipBehavior: Clip.none,
      children: [
        DecoratedBox(
          key: const ValueKey('profile-avatar-preview'),
          decoration: BoxDecoration(
            shape: BoxShape.circle,
            color: scheme.surfaceContainer,
            border: Border.all(color: scheme.outlineVariant, width: 2),
            boxShadow: HkShadows.card,
            image: avatar == null
                ? null
                : DecorationImage(image: avatar!, fit: BoxFit.cover),
          ),
          child: SizedBox.square(
            dimension: 80,
            child: avatar == null
                ? const ClipOval(
                    child: HkPlaceholder(HkPlaceholderKind.buyerAvatar),
                  )
                : null,
          ),
        ),
        Positioned(
          right: -2,
          bottom: 2,
          child: DecoratedBox(
            decoration: BoxDecoration(
              color: scheme.primary,
              shape: BoxShape.circle,
              border: Border.all(color: scheme.surface, width: 3),
            ),
            child: SizedBox.square(
              dimension: 32,
              child: Center(
                child: busy
                    ? SizedBox.square(
                        dimension: 16,
                        child: HkBrandLoader(
                          tone: HkBrandLoaderTone.mono,
                          color: scheme.onPrimary,
                        ),
                      )
                    : HkIcon(
                        HkIconName.photoAdd,
                        size: 18,
                        color: scheme.onPrimary,
                      ),
              ),
            ),
          ),
        ),
      ],
    );

    final identity = Column(
      key: const ValueKey('profile-avatar-identity'),
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          resolvedName,
          maxLines: 2,
          overflow: TextOverflow.ellipsis,
          style: AppTypography.sectionTitle,
        ),
        const SizedBox(height: HkSpace.xs),
        Text('建議使用清晰、完整的正面照片', style: AppTypography.caption),
      ],
    );

    return LayoutBuilder(
      builder: (context, constraints) {
        final textScale = MediaQuery.textScalerOf(context).scale(1);
        final stack = constraints.maxWidth < 340 || textScale > 1.6;
        final content = stack
            ? Column(
                children: [
                  avatarPreview,
                  const SizedBox(height: HkSpace.md),
                  Center(child: identity),
                ],
              )
            : Row(
                children: [
                  avatarPreview,
                  const SizedBox(width: HkSpace.lg),
                  Expanded(child: identity),
                ],
              );

        return Semantics(
          button: true,
          image: true,
          enabled: onPressed != null,
          label: '更換個人頭像，目前使用 $resolvedName 的頭像',
          hint: onPressed != null ? '點兩下以選擇新照片' : '照片更新中',
          onTap: onPressed,
          child: ExcludeSemantics(
            child: Material(
              color: scheme.surface,
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(HkRadius.lg),
                side: BorderSide(color: scheme.outlineVariant),
              ),
              clipBehavior: Clip.antiAlias,
              child: InkWell(
                key: const ValueKey('profile-avatar-action'),
                onTap: onPressed,
                child: Padding(
                  padding: const EdgeInsets.all(HkSpace.lg),
                  child: content,
                ),
              ),
            ),
          ),
        );
      },
    );
  }
}

/// 有清楚標題與說明的表單區塊。
class ProfileSectionCard extends StatelessWidget {
  const ProfileSectionCard({
    super.key,
    required this.title,
    required this.description,
    required this.children,
    this.icon = HkIconName.info,
    this.badgeLabel,
  });

  final String title;
  final String description;
  final List<Widget> children;
  final HkIconName icon;
  final String? badgeLabel;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Semantics(
      container: true,
      explicitChildNodes: true,
      child: DecoratedBox(
        key: ValueKey('profile-section-$title'),
        decoration: BoxDecoration(
          color: scheme.surface,
          borderRadius: BorderRadius.circular(HkRadius.lg),
          border: Border.all(color: scheme.outlineVariant),
          boxShadow: HkShadows.card,
        ),
        child: Padding(
          padding: const EdgeInsets.all(HkSpace.lg),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  DecoratedBox(
                    decoration: BoxDecoration(
                      color: scheme.primaryContainer,
                      borderRadius: BorderRadius.circular(HkRadius.md),
                    ),
                    child: SizedBox.square(
                      dimension: HkSpace.large,
                      child: Center(
                        child: HkIcon(
                          icon,
                          size: HkControl.icon,
                          color: scheme.onPrimaryContainer,
                        ),
                      ),
                    ),
                  ),
                  const SizedBox(width: HkSpace.md),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Semantics(
                          header: true,
                          child: Text(title, style: AppTypography.sectionTitle),
                        ),
                        const SizedBox(height: HkSpace.xs),
                        Text(description, style: AppTypography.metaMuted),
                      ],
                    ),
                  ),
                  if (badgeLabel != null) ...[
                    const SizedBox(width: HkSpace.sm),
                    DecoratedBox(
                      decoration: BoxDecoration(
                        color: badgeLabel == '必填'
                            ? scheme.primaryContainer
                            : scheme.surfaceContainer,
                        borderRadius: BorderRadius.circular(HkRadius.pill),
                      ),
                      child: Padding(
                        padding: const EdgeInsets.symmetric(
                          horizontal: HkSpace.sm,
                          vertical: HkSpace.xs,
                        ),
                        child: Text(
                          badgeLabel!,
                          style: AppTypography.caption.copyWith(
                            color: badgeLabel == '必填'
                                ? scheme.onPrimaryContainer
                                : scheme.onSurfaceVariant,
                            fontWeight: FontWeight.w700,
                          ),
                        ),
                      ),
                    ),
                  ],
                ],
              ),
              const SizedBox(height: HkSpace.lg),
              for (var index = 0; index < children.length; index++) ...[
                if (index != 0) const SizedBox(height: HkSpace.md),
                children[index],
              ],
            ],
          ),
        ),
      ),
    );
  }
}

/// 可直接編輯或明確唯讀的標準文字欄位。
class ProfileTextField extends StatelessWidget {
  const ProfileTextField({
    super.key,
    required this.label,
    required this.controller,
    this.focusNode,
    this.hintText,
    this.helperText,
    this.keyboardType,
    this.textInputAction,
    this.autofillHints,
    this.validator,
    this.onChanged,
    this.inputFormatters,
    this.icon,
    this.maxLines = 1,
    this.minLines,
    this.maxLength,
    this.required = false,
    this.readOnly = false,
    this.enabled = true,
  });

  final String label;
  final TextEditingController controller;
  final FocusNode? focusNode;
  final String? hintText;
  final String? helperText;
  final TextInputType? keyboardType;
  final TextInputAction? textInputAction;
  final Iterable<String>? autofillHints;
  final String? Function(String?)? validator;
  final ValueChanged<String>? onChanged;
  final List<TextInputFormatter>? inputFormatters;
  final HkIconName? icon;
  final int maxLines;
  final int? minLines;
  final int? maxLength;
  final bool required;
  final bool readOnly;
  final bool enabled;

  OutlineInputBorder _border(Color color, {double width = 1}) {
    return OutlineInputBorder(
      borderRadius: _fieldBorderRadius,
      borderSide: BorderSide(color: color, width: width),
    );
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final fieldLabel = required ? '$label *' : label;
    return Semantics(
      container: true,
      label: fieldLabel,
      explicitChildNodes: true,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          ExcludeSemantics(
            child: Text.rich(
              TextSpan(
                text: label,
                children: [
                  if (required)
                    TextSpan(
                      text: ' *',
                      style: AppTypography.meta.copyWith(color: scheme.error),
                    ),
                ],
              ),
              style: AppTypography.meta.copyWith(fontWeight: FontWeight.w700),
            ),
          ),
          const SizedBox(height: HkSpace.sm),
          TextFormField(
            key: ValueKey('profile-field-$label'),
            controller: controller,
            focusNode: focusNode,
            enabled: enabled,
            readOnly: readOnly,
            keyboardType: keyboardType,
            textInputAction: textInputAction,
            autofillHints: autofillHints,
            validator: validator,
            autovalidateMode: AutovalidateMode.onUserInteraction,
            onChanged: onChanged,
            inputFormatters: inputFormatters,
            maxLines: maxLines,
            minLines: minLines,
            maxLength: maxLength,
            onTapOutside: (_) => FocusManager.instance.primaryFocus?.unfocus(),
            style: AppTypography.body.copyWith(color: scheme.onSurface),
            decoration: InputDecoration(
              hintText: hintText,
              hintStyle: AppTypography.body.copyWith(color: AppColors.textHint),
              errorStyle: AppTypography.caption.copyWith(color: scheme.error),
              counterText: maxLength == null ? null : '',
              filled: true,
              fillColor: readOnly
                  ? scheme.surfaceContainer
                  : scheme.surfaceContainerLowest,
              suffixIcon: readOnly
                  ? const Center(
                      widthFactor: 1,
                      child: Icon(
                        AppIcons.lockOutline,
                        color: AppIconColors.muted,
                        size: 20,
                      ),
                    )
                  : icon == null
                  ? null
                  : Center(
                      widthFactor: 1,
                      child: HkIcon(icon!, size: 20, color: scheme.primary),
                    ),
              suffixIconConstraints: readOnly || icon != null
                  ? const BoxConstraints(
                      minWidth: HkControl.minTap,
                      minHeight: HkControl.minTap,
                    )
                  : null,
              contentPadding: const EdgeInsets.symmetric(
                horizontal: HkSpace.lg,
                vertical: HkSpace.md,
              ),
              enabledBorder: _border(AppColors.borderInput),
              disabledBorder: _border(AppColors.borderSubtle),
              focusedBorder: _border(scheme.primary, width: 2),
              errorBorder: _border(scheme.error),
              focusedErrorBorder: _border(scheme.error, width: 2),
            ),
          ),
          if (helperText != null) ...[
            const SizedBox(height: HkSpace.xs),
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: HkSpace.xs),
              child: Text(helperText!, style: AppTypography.caption),
            ),
          ],
        ],
      ),
    );
  }
}

/// 固定於頁面底部的主要儲存操作，避免長表單只能回到頂部儲存。
class ProfileSaveBar extends StatelessWidget {
  const ProfileSaveBar({
    super.key,
    required this.enabled,
    required this.busy,
    required this.onPressed,
  });

  final bool enabled;
  final bool busy;
  final VoidCallback onPressed;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Material(
      color: scheme.surface.withValues(alpha: 0.98),
      elevation: 0,
      child: DecoratedBox(
        decoration: BoxDecoration(
          border: Border(top: BorderSide(color: scheme.outlineVariant)),
          boxShadow: HkShadows.sm,
        ),
        child: SafeArea(
          top: false,
          child: Align(
            alignment: Alignment.topCenter,
            heightFactor: 1,
            child: ConstrainedBox(
              constraints: const BoxConstraints(
                maxWidth: HkControl.chromeMaxWidth,
              ),
              child: Padding(
                padding: const EdgeInsets.symmetric(
                  horizontal: HkSpace.lg,
                  vertical: HkSpace.sm,
                ),
                child: LayoutBuilder(
                  builder: (context, constraints) {
                    final textScale = MediaQuery.textScalerOf(context).scale(1);
                    final stack = constraints.maxWidth < 330 || textScale > 1.6;
                    final status = Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        HkIcon(
                          busy ? HkIconName.refresh : HkIconName.info,
                          size: 18,
                          color: scheme.onSurfaceVariant,
                        ),
                        const SizedBox(width: HkSpace.xs),
                        Flexible(
                          child: Text(
                            busy ? '正在安全儲存' : '資料將同步至你的帳號',
                            style: AppTypography.caption.copyWith(
                              color: scheme.onSurfaceVariant,
                            ),
                          ),
                        ),
                      ],
                    );
                    final button = Semantics(
                      button: true,
                      enabled: enabled && !busy,
                      label: busy ? '正在儲存個人資料' : '儲存個人資料變更',
                      onTap: enabled && !busy ? onPressed : null,
                      child: ExcludeSemantics(
                        child: ConstrainedBox(
                          constraints: const BoxConstraints(
                            minHeight: HkControl.minTap,
                          ),
                          child: FilledButton.icon(
                            key: const ValueKey('profile-save-primary'),
                            onPressed: enabled && !busy ? onPressed : null,
                            icon: busy
                                ? SizedBox.square(
                                    dimension: 18,
                                    child: HkBrandLoader(
                                      tone: HkBrandLoaderTone.mono,
                                      color: scheme.onPrimary,
                                    ),
                                  )
                                : const HkIcon(HkIconName.check, size: 20),
                            label: HkButtonLabel(busy ? '儲存中…' : '儲存變更'),
                            style: FilledButton.styleFrom(
                              padding: const EdgeInsets.symmetric(
                                horizontal: HkSpace.xl,
                              ),
                              textStyle: AppTypography.buttonLarge,
                            ),
                          ),
                        ),
                      ),
                    );

                    if (stack) {
                      return Column(
                        crossAxisAlignment: CrossAxisAlignment.stretch,
                        children: [
                          status,
                          const SizedBox(height: HkSpace.sm),
                          button,
                        ],
                      );
                    }
                    return Row(
                      children: [
                        Expanded(child: status),
                        const SizedBox(width: HkSpace.md),
                        button,
                      ],
                    );
                  },
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}

/// 個人資料頁底部的「刪除帳號」。[message] 是上一次刪除的結果說明（還在處理、被擋下或失敗）。
class ProfileDeleteAccountSection extends StatelessWidget {
  const ProfileDeleteAccountSection({
    super.key,
    required this.busy,
    required this.onPressed,
    this.message,
  });

  final bool busy;
  final VoidCallback? onPressed;
  final String? message;

  @override
  Widget build(BuildContext context) {
    return ProfileSectionCard(
      title: '刪除帳號',
      description: '刪除後帳號資料會被移除，無法復原。',
      icon: HkIconName.info,
      children: [
        if (message != null)
          Semantics(
            liveRegion: true,
            child: Text(
              message!,
              key: const ValueKey('profile-delete-account-message'),
              style: AppTypography.metaMuted.copyWith(color: AppColors.danger),
            ),
          ),
        ConstrainedBox(
          constraints: const BoxConstraints(minHeight: HkControl.minTap),
          child: OutlinedButton(
            key: const ValueKey('profile-delete-account'),
            onPressed: busy ? null : onPressed,
            style: OutlinedButton.styleFrom(
              foregroundColor: AppColors.danger,
              side: const BorderSide(color: AppColors.danger),
              textStyle: AppTypography.buttonLarge,
            ),
            child: HkButtonLabel(busy ? '刪除帳號中…' : '刪除帳號'),
          ),
        ),
      ],
    );
  }
}

/// 性別、生日、縣市等非文字輸入的整欄選擇控制。
class ProfileSelectionField extends StatelessWidget {
  const ProfileSelectionField({
    super.key,
    required this.label,
    required this.value,
    required this.placeholder,
    required this.onTap,
    this.icon = HkIconName.chevronDown,
    this.helperText,
    this.enabled = true,
  });

  final String label;
  final String value;
  final String placeholder;
  final VoidCallback? onTap;
  final HkIconName icon;
  final String? helperText;
  final bool enabled;

  @override
  Widget build(BuildContext context) {
    final hasValue = value.trim().isNotEmpty;
    final effectiveTap = enabled ? onTap : null;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          label,
          style: AppTypography.meta.copyWith(fontWeight: FontWeight.w700),
        ),
        const SizedBox(height: HkSpace.sm),
        Semantics(
          button: true,
          enabled: effectiveTap != null,
          label: '$label，${hasValue ? value : placeholder}',
          hint: effectiveTap != null ? '點兩下以選擇' : helperText,
          onTap: effectiveTap,
          child: ExcludeSemantics(
            child: _ProfileSelectionSurface(
              key: ValueKey('profile-selection-$label'),
              text: hasValue ? value : placeholder,
              hasValue: hasValue,
              icon: icon,
              enabled: enabled,
              onTap: effectiveTap,
            ),
          ),
        ),
        if (helperText != null) ...[
          const SizedBox(height: HkSpace.xs),
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: HkSpace.xs),
            child: Text(helperText!, style: AppTypography.caption),
          ),
        ],
      ],
    );
  }
}

/// 使用全站 [AppDropdown] 的個人資料下拉欄位。
///
/// [value] 是領域值（沒有值是 null），[items] 與 [itemLabel] 決定選項與文字。
/// 欄位顯示 `value == null ? placeholder : itemLabel(value)`，沒有值時用
/// placeholder 樣式；選單裡與 [value] 相等的那一項標成已選，所以值不在 [items]
/// 裡時仍顯示它的文字、只是選單沒有已選項。
class ProfileDropdownField<T> extends StatelessWidget {
  const ProfileDropdownField({
    super.key,
    required this.label,
    required this.value,
    required this.placeholder,
    required this.items,
    required this.itemLabel,
    required this.onSelected,
    this.icon = HkIconName.chevronDown,
    this.helperText,
    this.showSearch = false,
    this.enabled = true,
  });

  final String label;
  final T? value;
  final String placeholder;
  final List<T> items;
  final String Function(T item) itemLabel;
  final ValueChanged<T> onSelected;
  final HkIconName icon;
  final String? helperText;
  final bool showSearch;
  final bool enabled;

  @override
  Widget build(BuildContext context) {
    final current = value;
    final hasValue = current != null;
    final text = current == null ? placeholder : itemLabel(current);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          label,
          style: AppTypography.meta.copyWith(fontWeight: FontWeight.w700),
        ),
        const SizedBox(height: HkSpace.sm),
        AppDropdown<T>(
          items: items,
          itemLabel: itemLabel,
          selectedItem: value,
          placeholder: placeholder,
          showSearch: showSearch,
          enabled: enabled,
          semanticLabel: '$label，$text',
          onSelected: onSelected,
          child: _ProfileSelectionSurface(
            key: ValueKey('profile-dropdown-$label'),
            text: text,
            hasValue: hasValue,
            icon: icon,
            enabled: enabled,
          ),
        ),
        if (helperText != null) ...[
          const SizedBox(height: HkSpace.xs),
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: HkSpace.xs),
            child: Text(helperText!, style: AppTypography.caption),
          ),
        ],
      ],
    );
  }
}

class _ProfileSelectionSurface extends StatelessWidget {
  const _ProfileSelectionSurface({
    super.key,
    required this.text,
    required this.hasValue,
    required this.icon,
    required this.enabled,
    this.onTap,
  });

  final String text;
  final bool hasValue;
  final HkIconName icon;
  final bool enabled;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    return Material(
      color: enabled ? AppColors.bgElevated : AppColors.surfaceMuted,
      shape: RoundedRectangleBorder(
        borderRadius: _fieldBorderRadius,
        side: const BorderSide(color: AppColors.borderInput),
      ),
      clipBehavior: Clip.antiAlias,
      child: InkWell(
        onTap: onTap,
        child: ConstrainedBox(
          constraints: const BoxConstraints(minHeight: HkControl.minTap),
          child: Padding(
            padding: const EdgeInsets.only(
              left: HkSpace.lg,
              top: HkSpace.md,
              bottom: HkSpace.md,
            ),
            child: Row(
              children: [
                Expanded(
                  child: Text(
                    text,
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                    style: AppTypography.body.copyWith(
                      color: hasValue
                          ? AppColors.textEmphasis
                          : AppColors.textHint,
                    ),
                  ),
                ),
                SizedBox(
                  width: HkControl.minTap,
                  child: Center(
                    child: HkIcon(
                      icon,
                      size: 20,
                      color: enabled
                          ? AppIconColors.defaultColor
                          : AppIconColors.muted,
                    ),
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

/// 空間足夠時並排，窄螢幕與大字級時自然堆疊。
class ProfileResponsivePair extends StatelessWidget {
  const ProfileResponsivePair({
    super.key,
    required this.first,
    required this.second,
    this.breakpoint = 520,
  });

  final Widget first;
  final Widget second;
  final double breakpoint;

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(
      builder: (context, constraints) {
        final textScale = MediaQuery.textScalerOf(context).scale(1);
        final stack = constraints.maxWidth < breakpoint || textScale > 1.6;
        if (stack) {
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
            const SizedBox(width: HkSpace.lg),
            Expanded(child: second),
          ],
        );
      },
    );
  }
}
