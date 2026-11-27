import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:hawkr_ui_text/hawkr_ui_text.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:hawkr_api/errors.dart' show errorText;
import '../../../core/l10n/ui_locale.dart';
import '../../../core/networking/suspension_signal.dart';
import '../../../shared/widgets/common_snackbar.dart';
import '../../../shared/widgets/common_top_bar.dart';
import '../../../shared/widgets/hk_button_label.dart';
import '../../order/presentation/order_list_controller.dart';
import 'review_controller.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:hawkr_buyer/shared/widgets/hk_brand_loader.dart';

class RateScreen extends HookConsumerWidget {
  const RateScreen({super.key, required this.orderItemId, this.orderNo = ''});
  final String orderItemId;
  final String orderNo;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final rating = useState(0);
    final submitting = useState(false);
    final contentCtrl = useTextEditingController();
    final controllerState = ref.watch(addReviewControllerProvider);

    return Scaffold(
      backgroundColor: AppColors.bgPrimary,
      appBar: buildCommonTitleAppBar(context, title: '寫評論'),
      body: SingleChildScrollView(
        padding: EdgeInsets.all(20.w),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Container(
              padding: EdgeInsets.all(20.w),
              decoration: BoxDecoration(
                color: AppColors.bgCard,
                borderRadius: BorderRadius.circular(12.r),
                boxShadow: [
                  BoxShadow(
                    color: AppColors.cardShadow,
                    blurRadius: 8,
                    offset: const Offset(0, 2),
                  ),
                ],
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    '商品評分',
                    style: TextStyle(
                      fontSize: 16,
                      fontWeight: FontWeight.w700,
                      color: AppColors.textEmphasis,
                    ),
                  ),
                  SizedBox(height: 12.h),
                  Row(
                    mainAxisAlignment: MainAxisAlignment.center,
                    children: List.generate(5, (i) {
                      return GestureDetector(
                        onTap: () => rating.value = i + 1,
                        child: Padding(
                          padding: EdgeInsets.symmetric(horizontal: 8.w),
                          child: Icon(
                            i < rating.value
                                ? AppIcons.star
                                : AppIcons.starBorder,
                            color: AppIconColors.gold,
                            size: 40.sp,
                          ),
                        ),
                      );
                    }),
                  ),
                  SizedBox(height: 8.h),
                  Center(
                    child: Text(
                      _ratingLabel(rating.value, ref.watch(uiLocaleProvider)),
                      style: TextStyle(
                        fontSize: 14,
                        color: AppColors.textSecondary,
                      ),
                    ),
                  ),
                ],
              ),
            ),
            SizedBox(height: 16.h),
            Container(
              padding: EdgeInsets.all(20.w),
              decoration: BoxDecoration(
                color: AppColors.bgCard,
                borderRadius: BorderRadius.circular(12.r),
                boxShadow: [
                  BoxShadow(
                    color: AppColors.cardShadow,
                    blurRadius: 8,
                    offset: const Offset(0, 2),
                  ),
                ],
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    '評論內容',
                    style: TextStyle(
                      fontSize: 16,
                      fontWeight: FontWeight.w700,
                      color: AppColors.textEmphasis,
                    ),
                  ),
                  SizedBox(height: 12.h),
                  TextField(
                    controller: contentCtrl,
                    maxLines: 5,
                    maxLength: 500,
                    decoration: InputDecoration(
                      hintText: '分享您的使用心得...',
                      hintStyle: const TextStyle(color: AppColors.gray300),
                      filled: true,
                      fillColor: AppColors.bgPrimary,
                      border: OutlineInputBorder(
                        borderRadius: BorderRadius.circular(8.r),
                        borderSide: BorderSide.none,
                      ),
                      contentPadding: EdgeInsets.all(12.w),
                    ),
                  ),
                ],
              ),
            ),
            SizedBox(height: 32.h),
            SizedBox(
              width: double.infinity,
              child: ElevatedButton(
                style: ElevatedButton.styleFrom(
                  backgroundColor: AppColors.primary,
                  foregroundColor: AppColors.bgCard,
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(12.r),
                  ),
                  minimumSize: const Size(0, HkControl.minTap),
                  padding: const EdgeInsets.symmetric(horizontal: HkSpace.lg),
                  disabledBackgroundColor: AppColors.gray300,
                ),
                onPressed: (controllerState.isLoading || submitting.value)
                    ? null
                    : () async {
                        if (rating.value == 0) {
                          showFloatingToast(context, '請選擇評分', isSuccess: false);
                          return;
                        }
                        if (contentCtrl.text.trim().isEmpty) {
                          showFloatingToast(
                            context,
                            '請輸入評論內容',
                            isSuccess: false,
                          );
                          return;
                        }
                        submitting.value = true;
                        final result = await ref
                            .read(addReviewControllerProvider.notifier)
                            .submit(
                              orderNo: orderNo,
                              orderItemId: orderItemId,
                              rating: rating.value,
                              content: contentCtrl.text.trim(),
                            );
                        if (result != null && context.mounted) {
                          ref
                              .read(orderListControllerProvider.notifier)
                              .refresh();
                          showFloatingToast(context, '評論提交成功');
                          if (Navigator.of(context).canPop()) {
                            context.pop(true);
                          }
                        } else if (context.mounted) {
                          submitting.value = false;
                          final err = ref.read(addReviewControllerProvider);
                          err.whenOrNull(
                            error: (e, _) {
                              if (isAccountSuspendedError(e)) return;
                              showFloatingToast(
                                context,
                                errorText(
                                  e,
                                  defaults: ref.read(defaultErrorTextProvider),
                                ),
                                isSuccess: false,
                              );
                            },
                          );
                        }
                      },
                child: controllerState.isLoading
                    ? SizedBox(
                        height: 20.h,
                        width: 20.w,
                        child: HkBrandLoader(
                          tone: HkBrandLoaderTone.mono,
                          color: AppColors.bgCard,
                        ),
                      )
                    : HkButtonLabel(
                        '提交評論',
                        style: TextStyle(
                          fontSize: 16,
                          fontWeight: FontWeight.w700,
                        ),
                      ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

String _ratingLabel(int r, UiLocale l) => switch (r) {
  1 => reviewReviewRatingLabel1(l),
  2 => reviewReviewRatingLabel2(l),
  3 => reviewReviewRatingLabel3(l),
  4 => reviewReviewRatingLabel4(l),
  5 => reviewReviewRatingLabel5(l),
  _ => '點擊評分',
};
