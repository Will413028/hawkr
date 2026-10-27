import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

import '../../../shared/widgets/chat_bubble_icon.dart';
import 'product_listing_chat/product_listing_chat_sheet.dart';

/// 對話式 AI 上架助手完成度不足，暫時對 user 隱藏入口；功能補完後改回 true。
const bool kAiProductListingAssistantEnabled = false;

/// Wizard top bar 右上角的「AI 助手」按鈕，5 個 step 共用。
///
/// 樣式對齊既有 `_AiAutofillButton`（綠色膠囊 + auto_awesome icon），
/// 但改成 chat_bubble icon + 文字「AI 助手」，以區分兩者。
class AiAssistantButton extends StatelessWidget {
  const AiAssistantButton({super.key});

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: () => ProductListingChatSheet.show(context),
      child: Container(
        margin: EdgeInsets.only(right: 8.w),
        padding: EdgeInsets.symmetric(horizontal: 12.w, vertical: 8.h),
        decoration: BoxDecoration(
          color: AppColors.primarySoft,
          borderRadius: BorderRadius.circular(24),
          border: Border.all(color: AppColors.primary),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            ChatBubbleIcon(size: 14.sp, color: AppIconColors.secondary),
            SizedBox(width: 4.w),
            Text(
              'AI 助手',
              style: TextStyle(
                fontSize: 14.sp,
                color: AppColors.textAccentOnTint,
                fontWeight: FontWeight.w500,
              ),
            ),
          ],
        ),
      ),
    );
  }
}
