import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../../core/theme/app_colors.dart';
import '../../core/theme/app_typography.dart';
import '../../core/theme/hk_tokens.dart';
import 'common_top_bar.dart';
import 'hk_icon.dart';

/// 共享的分類頁頂部列（logo/搜尋/購物車/聊天）
class CategoryTopBar extends StatelessWidget {
  const CategoryTopBar({
    super.key,
    this.leading,
    required this.middle,
    this.actions = const [],
    this.horizontalPadding,
    this.contentPadding,
    this.spacing,
    this.leadingSpacing,
    this.wrapWithCommon = true,
    this.backgroundColor = AppColors.topBarBackground,
    this.systemOverlayStyle = AppColors.topBarOverlay,
    this.borderRadius,
    this.boxShadow,
    this.showBottomBorder = true,
    this.frostedGlass,
  });

  final Widget? leading;
  final Widget middle;
  final List<Widget> actions;

  /// 外層左右 padding。預設 12 logical px（網頁 mobile gutter）。
  final EdgeInsetsGeometry? horizontalPadding;

  /// 內層 Row 的 padding。預設 zero（外層已處理）。
  final EdgeInsetsGeometry? contentPadding;

  /// actions 彼此之間 / middle↔actions 間距。預設 8 logical px。
  final double? spacing;

  /// leading → middle 間距。預設 8 logical px。
  final double? leadingSpacing;

  final bool wrapWithCommon;
  final Color backgroundColor;
  final SystemUiOverlayStyle systemOverlayStyle;
  final BorderRadiusGeometry? borderRadius;
  final List<BoxShadow>? boxShadow;
  final bool showBottomBorder;

  /// null 時淺色 chrome 預設使用玻璃；明確品牌深色則維持實色與正確對比。
  final bool? frostedGlass;

  @override
  Widget build(BuildContext context) {
    final effectiveSpacing = spacing ?? HkSpace.sm;
    final effectiveLeadingSpacing = leadingSpacing ?? HkSpace.sm;
    final effectiveHorizontalPadding =
        horizontalPadding ?? const EdgeInsets.symmetric(horizontal: HkSpace.md);
    final effectiveContentPadding = contentPadding ?? EdgeInsets.zero;

    final children = <Widget>[];
    if (leading != null) {
      children.add(leading!);
      children.add(SizedBox(width: effectiveLeadingSpacing));
    }
    children.add(Expanded(child: middle));
    if (actions.isNotEmpty) {
      children.add(SizedBox(width: effectiveSpacing));
      for (var i = 0; i < actions.length; i++) {
        children.add(actions[i]);
        if (i != actions.length - 1) {
          children.add(SizedBox(width: effectiveSpacing));
        }
      }
    }

    final row = Padding(
      padding: effectiveContentPadding,
      child: Row(children: children),
    );

    if (wrapWithCommon) {
      return commonTopBar(
        context: context,
        backgroundColor: backgroundColor,
        horizontalPadding: effectiveHorizontalPadding,
        systemOverlayStyle: systemOverlayStyle,
        borderRadius: borderRadius,
        boxShadow: boxShadow,
        showBottomBorder: showBottomBorder,
        frostedGlass:
            frostedGlass ?? backgroundColor != AppColors.actionPrimary,
        child: row,
      );
    }

    return Container(
      color: backgroundColor,
      child: SizedBox(
        height: kTopBarHeight,
        child: Padding(padding: effectiveHorizontalPadding, child: row),
      ),
    );
  }
}

class CategorySearchField extends StatelessWidget {
  const CategorySearchField({
    super.key,
    this.controller,
    this.onChanged,
    this.onSubmitted,
    this.onTap,
    this.hintText = '搜尋商品',
    this.semanticLabel,
    this.semanticHint,
    this.readOnly = false,
    this.suffixIcon,
    this.height = HkControl.searchFieldHeight,
    this.horizontalPadding = HkSpace.lg,
    this.autofocus = false,
  });

  final TextEditingController? controller;
  final ValueChanged<String>? onChanged;
  final ValueChanged<String>? onSubmitted;
  final VoidCallback? onTap;
  final String hintText;
  final String? semanticLabel;
  final String? semanticHint;
  final bool readOnly;
  final Widget? suffixIcon;
  final double height;
  final double horizontalPadding;
  final bool autofocus;

  @override
  Widget build(BuildContext context) {
    final effectiveHeight = height < HkControl.minTap
        ? HkControl.minTap
        : height;
    final effectiveSemanticHint =
        semanticHint ?? (readOnly ? '開啟搜尋' : '輸入關鍵字後搜尋');
    final effectiveSemanticLabel = semanticLabel ?? hintText;
    final prominent = effectiveHeight >= HkControl.prominentSearchHeight;
    final borderRadius = BorderRadius.circular(effectiveHeight / 2);
    final decoration = BoxDecoration(
      color: AppColors.bgCard,
      borderRadius: borderRadius,
      border: Border.all(color: AppColors.borderInput),
      boxShadow: prominent ? HkShadows.xs : null,
    );
    final iconSize = prominent ? 22.0 : 20.0;
    final gap = prominent ? HkSpace.md : HkSpace.sm;

    if (readOnly) {
      return Semantics(
        button: onTap != null,
        enabled: onTap != null,
        label: effectiveSemanticLabel,
        hint: effectiveSemanticHint,
        onTap: onTap,
        excludeSemantics: true,
        child: GestureDetector(
          onTap: onTap,
          behavior: HitTestBehavior.opaque,
          child: DecoratedBox(
            decoration: decoration,
            child: SizedBox(
              height: effectiveHeight,
              child: Padding(
                padding: EdgeInsets.symmetric(horizontal: horizontalPadding),
                child: Row(
                  children: [
                    HkIcon(
                      HkIconName.search,
                      size: iconSize,
                      color: AppIconColors.muted,
                    ),
                    SizedBox(width: gap),
                    Expanded(
                      child: Text(
                        hintText,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: AppTypography.body.copyWith(
                          color: AppColors.textHint,
                          height: 1.25,
                        ),
                      ),
                    ),
                    if (suffixIcon != null) suffixIcon!,
                  ],
                ),
              ),
            ),
          ),
        ),
      );
    }
    // 可編輯模式維持 TextField
    return Semantics(
      label: effectiveSemanticLabel,
      hint: effectiveSemanticHint,
      child: DecoratedBox(
        decoration: decoration,
        child: SizedBox(
          height: effectiveHeight,
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: HkSpace.xs),
            child: TextField(
              controller: controller,
              onChanged: onChanged,
              onSubmitted: onSubmitted,
              onTap: onTap,
              readOnly: readOnly,
              autofocus: autofocus,
              textInputAction: TextInputAction.search,
              textAlignVertical: TextAlignVertical.center,
              decoration: InputDecoration(
                hint: ExcludeSemantics(
                  child: Text(
                    hintText,
                    maxLines: 1,
                    softWrap: false,
                    overflow: TextOverflow.ellipsis,
                    style: AppTypography.body.copyWith(
                      color: AppColors.textHint,
                    ),
                  ),
                ),
                hintMaxLines: 1,
                hintStyle: AppTypography.body.copyWith(
                  color: AppColors.textHint,
                ),
                prefixIcon: HkIcon(
                  HkIconName.search,
                  size: iconSize,
                  color: AppIconColors.muted,
                ),
                prefixIconConstraints: BoxConstraints.tightFor(
                  width: HkControl.searchFieldHeight,
                  height: effectiveHeight,
                ),
                suffixIconConstraints: const BoxConstraints(
                  minWidth: HkControl.minTap,
                  minHeight: HkControl.minTap,
                ),
                suffixIcon: suffixIcon,
                filled: false,
                fillColor: Colors.transparent,
                border: InputBorder.none,
                enabledBorder: InputBorder.none,
                focusedBorder: InputBorder.none,
                disabledBorder: InputBorder.none,
                errorBorder: InputBorder.none,
                focusedErrorBorder: InputBorder.none,
                isDense: true,
                contentPadding: EdgeInsets.zero,
              ),
              style: AppTypography.body.copyWith(color: AppColors.textEmphasis),
            ),
          ),
        ),
      ),
    );
  }
}
