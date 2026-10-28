import 'package:flutter/material.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:url_launcher/url_launcher.dart';

import '../../../core/theme/app_colors.dart';
import '../../../core/theme/hk_tokens.dart';
import '../../../shared/icons/app_icon_colors.dart';
import '../../../shared/icons/app_icons.dart';
import '../../../shared/utils/hk_log.dart';
import '../../../shared/widgets/common_top_bar.dart';
import '../../../shared/widgets/hk_button_label.dart';
import '../application/force_update_provider.dart';

/// App 強制更新畫面（`packages/hawkr_app_update/docs/spec.md`）。
///
/// 版本低於後端設定的 `minimum_version` 時，router redirect 一律導來此頁。
/// 這是**不可跳過**的攔截頁：沒有返回鍵、`PopScope(canPop: false)` 擋掉
/// 系統返回手勢，唯一出路是更新 App 後重啟。
///
/// `storeUrl` 為空時**隱藏「前往更新」按鈕**改顯示文字指引——不導向任何猜測
/// URL，使用者撞 404 比沒有按鈕更糟。
class ForceUpdateScreen extends ConsumerWidget {
  const ForceUpdateScreen({super.key});

  static const String _defaultTitle = '請更新至最新版本';
  static const String _defaultBody = '此版本已停止支援，請前往商店更新後繼續使用。';
  static const String _manualGuidance =
      '請至 App Store / Google Play 搜尋「Hawkr」，更新到最新版本後重新開啟。';

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final gate = ref.watch(forceUpdateProvider);
    final title = gate.noticeTitle.trim().isNotEmpty
        ? gate.noticeTitle.trim()
        : _defaultTitle;
    final body = gate.noticeBody.trim().isNotEmpty
        ? gate.noticeBody.trim()
        : _defaultBody;
    final storeUrl = gate.storeUrl.trim();

    return PopScope(
      canPop: false,
      child: Scaffold(
        backgroundColor: AppColors.bgPrimary,
        appBar: buildCommonTitleAppBar(context, title: '需要更新', showBack: false),
        body: SafeArea(
          child: SingleChildScrollView(
            padding: EdgeInsets.fromLTRB(
              HkSpace.xl.w,
              HkSpace.section.h,
              HkSpace.xl.w,
              HkSpace.section.h,
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Center(
                  child: Container(
                    width: 72.w,
                    height: 72.w,
                    decoration: const BoxDecoration(
                      color: AppColors.warningSoft,
                      shape: BoxShape.circle,
                    ),
                    child: Icon(
                      AppIcons.warningAmberRounded,
                      color: AppIconColors.warning,
                      size: 40,
                    ),
                  ),
                ),
                SizedBox(height: HkSpace.xl.h),
                Text(
                  title,
                  textAlign: TextAlign.center,
                  style: TextStyle(
                    fontSize: 20,
                    fontWeight: FontWeight.w700,
                    color: AppColors.textEmphasis,
                  ),
                ),
                SizedBox(height: HkSpace.sm.h),
                Text(
                  body,
                  textAlign: TextAlign.center,
                  style: TextStyle(
                    fontSize: 14,
                    color: AppColors.textSecondary,
                  ),
                ),
                SizedBox(height: HkSpace.xxl.h),
                _VersionInfoCard(minimumVersion: gate.minimumVersion),
                SizedBox(height: HkSpace.section.h),
                if (storeUrl.isEmpty)
                  Text(
                    _manualGuidance,
                    textAlign: TextAlign.center,
                    style: TextStyle(
                      fontSize: 14,
                      color: AppColors.textSecondary,
                    ),
                  )
                else
                  ElevatedButton(
                    onPressed: () => _openStore(context, storeUrl),
                    style: ElevatedButton.styleFrom(
                      minimumSize: const Size(0, HkControl.minTap),
                      padding: const EdgeInsets.symmetric(
                        horizontal: HkSpace.xl,
                      ),
                      shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(HkRadius.md.r),
                      ),
                    ),
                    child: HkButtonLabel(
                      '前往更新',
                      style: TextStyle(
                        fontSize: 16,
                        fontWeight: FontWeight.w700,
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

  Future<void> _openStore(BuildContext context, String storeUrl) async {
    final messenger = ScaffoldMessenger.maybeOf(context);
    bool opened = false;
    try {
      final uri = Uri.parse(storeUrl);
      opened = await launchUrl(uri, mode: LaunchMode.externalApplication);
    } catch (e) {
      HKLog.logW('[ForceUpdate] 開啟商店連結失敗：$e');
    }
    if (!opened) {
      messenger?.showSnackBar(const SnackBar(content: Text(_manualGuidance)));
    }
  }
}

/// 最低可用版本說明卡（純展示，後端未回傳版本時整張隱藏）。
class _VersionInfoCard extends StatelessWidget {
  const _VersionInfoCard({required this.minimumVersion});

  final String minimumVersion;

  @override
  Widget build(BuildContext context) {
    if (minimumVersion.trim().isEmpty) return const SizedBox.shrink();

    return Container(
      width: double.infinity,
      padding: EdgeInsets.all(HkSpace.lg.w),
      decoration: BoxDecoration(
        color: AppColors.bgCard,
        borderRadius: BorderRadius.circular(HkRadius.lg.r),
        border: Border.all(color: AppColors.cardBorder),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            '最低可用版本',
            style: TextStyle(fontSize: 14, color: AppColors.textSecondary),
          ),
          SizedBox(height: HkSpace.xs.h),
          Text(
            minimumVersion.trim(),
            style: TextStyle(
              fontSize: 16,
              fontWeight: FontWeight.w700,
              color: AppColors.textEmphasis,
            ),
          ),
        ],
      ),
    );
  }
}
