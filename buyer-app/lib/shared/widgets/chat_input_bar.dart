import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/app_typography.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

import 'hk_tap_target.dart';
import 'hk_frosted_icon_button.dart';

/// Chat composer with a rounded input and a circular frosted send action.
///
/// Handles the home-indicator inset itself; do not wrap in another bottom
/// SafeArea. [topSlot] supports a reply preview or other conversation context.
class ChatInputBar extends StatelessWidget {
  const ChatInputBar({
    super.key,
    required this.controller,
    required this.onSend,
    required this.onPlusPressed,
    this.focusNode,
    this.sending = false,
    this.hintText = '輸入訊息...',
    this.topSlot,
  });

  final TextEditingController controller;
  final FocusNode? focusNode;
  final VoidCallback onSend;
  final VoidCallback onPlusPressed;
  final bool sending;
  final String hintText;

  /// 額外塞在輸入列上方的內容（例如回覆預覽）
  final Widget? topSlot;

  @override
  Widget build(BuildContext context) {
    final bottomInset = MediaQuery.of(context).padding.bottom;
    return Padding(
      padding: EdgeInsets.fromLTRB(16.w, 12.h, 16.w, 12.h + bottomInset),
      child: TextFieldTapRegion(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            if (topSlot != null) topSlot!,
            _InputRow(
              controller: controller,
              focusNode: focusNode,
              sending: sending,
              hintText: hintText,
              onSend: onSend,
              onPlusPressed: onPlusPressed,
            ),
          ],
        ),
      ),
    );
  }
}

class _InputRow extends StatelessWidget {
  const _InputRow({
    required this.controller,
    required this.focusNode,
    required this.sending,
    required this.hintText,
    required this.onSend,
    required this.onPlusPressed,
  });

  final TextEditingController controller;
  final FocusNode? focusNode;
  final bool sending;
  final String hintText;
  final VoidCallback onSend;
  final VoidCallback onPlusPressed;

  static const _brandGreen = AppColors.primary;
  static const _pillBg = AppColors.gray100;
  static const _border = AppColors.cardBorder;
  static const _hint = AppColors.textSecondary;
  static const _text = AppColors.textPrimary;

  @override
  Widget build(BuildContext context) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.end,
      children: [
        HkTapTarget(
          semanticLabel: '新增附件',
          onTap: onPlusPressed,
          child: Container(
            width: 36.w,
            height: 36.w,
            decoration: const BoxDecoration(
              color: _brandGreen,
              shape: BoxShape.circle,
            ),
            child: Icon(
              AppIcons.add,
              color: AppIconColors.onBrand,
              size: 22.sp,
            ),
          ),
        ),
        SizedBox(width: 12.w),
        Expanded(
          child: Container(
            constraints: const BoxConstraints(minHeight: HkControl.minTap),
            padding: EdgeInsets.symmetric(horizontal: 8.w, vertical: 4.h),
            decoration: BoxDecoration(
              color: _pillBg,
              border: Border.all(color: _border, width: 1),
              borderRadius: BorderRadius.circular(999.r),
            ),
            child: TextField(
              controller: controller,
              focusNode: focusNode,
              decoration: InputDecoration(
                hintText: hintText,
                hintStyle: AppTypography.body.copyWith(color: _hint),
                filled: false,
                fillColor: Colors.transparent,
                border: InputBorder.none,
                enabledBorder: InputBorder.none,
                focusedBorder: InputBorder.none,
                disabledBorder: InputBorder.none,
                errorBorder: InputBorder.none,
                focusedErrorBorder: InputBorder.none,
                isDense: true,
                contentPadding: EdgeInsets.symmetric(
                  horizontal: 12.w,
                  vertical: 8.h,
                ),
              ),
              style: AppTypography.body.copyWith(color: _text),
              minLines: 1,
              maxLines: 4,
              textInputAction: TextInputAction.send,
              onSubmitted: (_) {
                if (!sending && controller.text.trim().isNotEmpty) {
                  onSend();
                }
              },
            ),
          ),
        ),
        const SizedBox(width: HkSpace.sm),
        ValueListenableBuilder<TextEditingValue>(
          valueListenable: controller,
          builder: (context, value, _) {
            final hasText = value.text.trim().isNotEmpty;
            final enabled = hasText && !sending;
            return HkFrostedIconButton(
              label: '發送訊息',
              icon: AppIcons.send,
              onPressed: enabled ? onSend : null,
              loading: sending,
              loadingLabel: '訊息傳送中',
            );
          },
        ),
      ],
    );
  }
}
