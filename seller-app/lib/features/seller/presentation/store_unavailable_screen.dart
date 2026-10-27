import 'package:hawkr_seller/shared/widgets/hk_button_label.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

import '../../../core/theme/app_colors.dart';
import '../../../shared/icons/app_icon_colors.dart';
import '../../../shared/icons/app_icons.dart';
import '../data/current_store.dart';
import '../data/version_conflict.dart';
import 'profile_other_options_card.dart' show ProfileLogoutButton;

/// 讀不到店家狀態（`GetMyStore` 失敗，也沒有上一次的快照）時 router 導來的重試頁：不知道有沒有店家，
/// 不能放行進主畫面。版型與重試按鈕沿用其他清單頁的錯誤狀態（`block_list_screen.dart`）。
class StoreUnavailableScreen extends ConsumerWidget {
  const StoreUnavailableScreen({super.key});

  static const message = storeStateUnavailableNotice;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return Scaffold(
      backgroundColor: AppColors.bgApp,
      body: SafeArea(
        child: Center(
          child: Padding(
            padding: EdgeInsets.symmetric(horizontal: 20.w),
            child: Column(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                Icon(
                  AppIcons.errorOutline,
                  size: 64.sp,
                  color: AppIconColors.muted,
                ),
                SizedBox(height: 16.h),
                Text(
                  message,
                  style: TextStyle(
                    fontSize: 14.sp,
                    color: AppColors.textSecondary,
                  ),
                  textAlign: TextAlign.center,
                ),
                SizedBox(height: 16.h),
                ElevatedButton(
                  onPressed: () =>
                      ref.read(currentStoreProvider.notifier).refresh(),
                  style: ElevatedButton.styleFrom(
                    backgroundColor: AppColors.primary,
                  ),
                  child: const HkButtonLabel(
                    '重試',
                    style: TextStyle(color: AppColors.bgCard),
                  ),
                ),
                SizedBox(height: 12.h),
                const ProfileLogoutButton(),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
