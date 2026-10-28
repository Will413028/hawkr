import 'package:hawkr_seller/shared/widgets/hk_button_label.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:url_launcher/url_launcher.dart';

import '../../../core/theme/app_colors.dart';
import '../../../shared/icons/app_icon_colors.dart';
import '../../../shared/icons/app_icons.dart';
import '../../../shared/utils/hk_log.dart';
import '../../../shared/widgets/hk_glass_app_bar.dart';
import '../application/force_update_provider.dart';

/// App 強制更新攔截頁（`packages/hawkr_app_update/docs/spec.md`）。
///
/// 後端判定本機版本低於最低可用版本時，router gating（見
/// `core/router/redirect_logic.dart`）一律導來此頁。骨架比照
/// `suspension_notice_screen.dart`，差別在多一層 `PopScope(canPop: false)`：
/// 停權頁還留著白名單頁可去，強更沒有任何可去之處，語意更強。
///
/// `storeUrl` 為空時**隱藏**「前往更新」按鈕改顯示文字指引——不導向任何猜測
/// 的商店 URL，使用者撞 404 比沒有按鈕更糟。
class ForceUpdateScreen extends ConsumerWidget {
  const ForceUpdateScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final gate = ref.watch(forceUpdateGateProvider);
    final title = gate.noticeTitle.isNotEmpty ? gate.noticeTitle : '請更新至最新版本';
    final body = gate.noticeBody.isNotEmpty
        ? gate.noticeBody
        : '此版本已停止支援，請前往商店更新後繼續使用。';
    final storeUrl = gate.storeUrl.trim();

    return PopScope(
      canPop: false,
      child: Scaffold(
        backgroundColor: AppColors.bgApp,
        appBar: const HkGlassAppBar(title: Text('需要更新'), showBackButton: false),
        body: SafeArea(
          child: SingleChildScrollView(
            padding: EdgeInsets.fromLTRB(20.w, 32.h, 20.w, 32.h),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Center(
                  child: Container(
                    width: 72.w,
                    height: 72.w,
                    decoration: BoxDecoration(
                      color: AppColors.dangerStrong.withValues(alpha: 0.12),
                      shape: BoxShape.circle,
                    ),
                    child: Icon(
                      AppIcons.warningAmberRounded,
                      color: AppIconColors.warning,
                      size: 40.sp,
                    ),
                  ),
                ),
                SizedBox(height: 20.h),
                Text(
                  title,
                  textAlign: TextAlign.center,
                  style: TextStyle(
                    fontSize: 20.sp,
                    fontWeight: FontWeight.w700,
                    color: AppColors.gray1,
                  ),
                ),
                SizedBox(height: 8.h),
                Text(
                  body,
                  textAlign: TextAlign.center,
                  style: TextStyle(
                    fontSize: 14.sp,
                    color: AppColors.textSecondary,
                  ),
                ),
                SizedBox(height: 24.h),
                _VersionInfoCard(minimumVersion: gate.minimumVersion),
                SizedBox(height: 32.h),
                if (storeUrl.isNotEmpty)
                  ElevatedButton(
                    onPressed: () => _openStore(storeUrl),
                    style: ElevatedButton.styleFrom(
                      backgroundColor: AppColors.buttonPrimary,
                      foregroundColor: Colors.white,
                      padding: EdgeInsets.symmetric(vertical: 14.h),
                      shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(8.r),
                      ),
                      elevation: 0,
                    ),
                    child: HkButtonLabel(
                      '前往更新',
                      style: TextStyle(
                        fontSize: 16.sp,
                        fontWeight: FontWeight.w700,
                      ),
                    ),
                  )
                else
                  const _ManualUpdateGuidance(),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Future<void> _openStore(String storeUrl) async {
    final uri = Uri.tryParse(storeUrl);
    if (uri == null) {
      HKLog.logD('[ForceUpdate] invalid storeUrl: $storeUrl');
      return;
    }
    try {
      await launchUrl(uri, mode: LaunchMode.externalApplication);
    } catch (e) {
      HKLog.logD('[ForceUpdate] launch store failed: $e');
    }
  }
}

/// 顯示後端要求的最低版本；後端沒給就整張卡不顯示。
class _VersionInfoCard extends StatelessWidget {
  const _VersionInfoCard({required this.minimumVersion});

  final String minimumVersion;

  @override
  Widget build(BuildContext context) {
    if (minimumVersion.isEmpty) return const SizedBox.shrink();

    return Container(
      width: double.infinity,
      padding: EdgeInsets.all(16.w),
      decoration: BoxDecoration(
        color: AppColors.dangerStrong.withValues(alpha: 0.08),
        borderRadius: BorderRadius.circular(12.r),
        border: Border.all(
          color: AppColors.dangerStrong.withValues(alpha: 0.3),
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            '最低可用版本',
            style: TextStyle(fontSize: 14.sp, color: AppColors.textSecondary),
          ),
          SizedBox(height: 4.h),
          Text(
            minimumVersion,
            style: TextStyle(
              fontSize: 16.sp,
              fontWeight: FontWeight.w700,
              color: AppColors.dangerStrong,
            ),
          ),
        ],
      ),
    );
  }
}

/// `storeUrl` 未設定時的文字指引。
class _ManualUpdateGuidance extends StatelessWidget {
  const _ManualUpdateGuidance();

  @override
  Widget build(BuildContext context) {
    return Container(
      width: double.infinity,
      padding: EdgeInsets.all(16.w),
      decoration: BoxDecoration(
        color: Colors.white,
        borderRadius: BorderRadius.circular(12.r),
        border: Border.all(color: AppColors.border),
        boxShadow: [
          BoxShadow(
            color: AppColors.cardShadow,
            blurRadius: 12,
            offset: Offset(0, 4.h),
          ),
        ],
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            '如何更新：',
            style: TextStyle(
              fontSize: 14.sp,
              fontWeight: FontWeight.w700,
              color: AppColors.gray1,
            ),
          ),
          SizedBox(height: 8.h),
          Text(
            '請開啟手機的 App Store 或 Google Play，搜尋「Hawkr 賣家」並更新至最新版本後再重新開啟。',
            style: TextStyle(fontSize: 14.sp, color: AppColors.textSecondary),
          ),
        ],
      ),
    );
  }
}
