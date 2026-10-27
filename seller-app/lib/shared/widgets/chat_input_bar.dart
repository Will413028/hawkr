import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/core/theme/app_typography.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

import 'hk_frosted_icon_button.dart';
import 'hk_tap_target.dart';

/// 共用聊天輸入列：透明外層、圓角輸入框與圓形霧面玻璃送出按鈕。
///
/// 自帶 home indicator 安全區，不需額外包 `SafeArea(top: false)`。
/// `topSlot` 可放回覆預覽或通知；附件與送出和文字欄位共用點擊區，
/// 操作時維持輸入焦點。空白內容與傳送中狀態不可送出。
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

  /// 額外放在輸入列上方的內容（例如回覆預覽）。
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

  void _sendIfReady() {
    if (!sending && controller.text.trim().isNotEmpty) onSend();
  }

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
            constraints: const BoxConstraints(minHeight: 48),
            padding: EdgeInsets.symmetric(horizontal: 8.w, vertical: 4.h),
            decoration: BoxDecoration(
              color: _pillBg,
              border: Border.all(color: _border, width: 1),
              borderRadius: BorderRadius.circular(999.r),
            ),
            child: TextField(
              controller: controller,
              focusNode: focusNode,
              readOnly: sending,
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
              onSubmitted: (_) => _sendIfReady(),
            ),
          ),
        ),
        SizedBox(width: 8.w),
        ValueListenableBuilder<TextEditingValue>(
          valueListenable: controller,
          builder: (context, value, _) => HkFrostedIconButton(
            label: '發送訊息',
            icon: AppIcons.send,
            onPressed: value.text.trim().isNotEmpty ? _sendIfReady : null,
            loading: sending,
            loadingLabel: '訊息傳送中',
          ),
        ),
      ],
    );
  }
}
