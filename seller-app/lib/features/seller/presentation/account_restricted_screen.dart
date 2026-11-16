import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/policy_category.pbenum.dart'
    show PolicyCategory;
import 'package:hawkr_api/taipei_time.dart';
import 'package:hawkr_ui_text/hawkr_ui_text.dart';

import '../../../core/l10n/ui_locale.dart';
import '../../../core/session/account_restriction.dart';
import '../../../core/theme/app_colors.dart';
import '../../../shared/icons/app_icon_colors.dart';
import '../../../shared/icons/app_icons.dart';
import '../../../shared/widgets/hk_button_label.dart';
import '../../../shared/widgets/hk_glass_app_bar.dart';
import 'profile_account_closure.dart';
import 'profile_other_options_card.dart' show ProfileLogoutButton;

const accountRestrictedTitle = '帳號已被停權';
const accountRestrictedBody = '你的帳號目前受平台限制，無法使用賣家功能。';
const accountRestrictedDeleteLabel = '刪除帳號';

/// 帳號被停權（`GetSession.suspension` 或前景請求的 `ACCOUNT_SUSPENDED`）時唯一能進的頁面：
/// 顯示停權資訊，只提供刪除帳號與登出。
class AccountRestrictedScreen extends ConsumerStatefulWidget {
  const AccountRestrictedScreen({super.key});

  @override
  ConsumerState<AccountRestrictedScreen> createState() =>
      _AccountRestrictedScreenState();
}

class _AccountRestrictedScreenState
    extends ConsumerState<AccountRestrictedScreen> {
  bool _busy = false;

  @override
  Widget build(BuildContext context) {
    final restriction = ref.watch(accountRestrictionStateProvider);
    final locale = context.uiLocale;
    final category = restriction?.category.uiText(locale);
    final reason = restriction?.reason ?? '';
    final suspendedAt = restriction?.suspendedAt;
    final rows = <String>[
      if (restriction != null &&
          restriction.category != PolicyCategory.POLICY_CATEGORY_UNSPECIFIED &&
          category != null)
        '原因：$category',
      if (suspendedAt != null) '停權時間：${formatTaipeiDate(suspendedAt)}',
    ];

    return Scaffold(
      backgroundColor: AppColors.bgApp,
      appBar: const HkGlassAppBar(
        title: Text(accountRestrictedTitle),
        showBackButton: false,
      ),
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
                accountRestrictedBody,
                textAlign: TextAlign.center,
                style: TextStyle(
                  fontSize: 14.sp,
                  color: AppColors.textSecondary,
                ),
              ),
              if (rows.isNotEmpty || reason.isNotEmpty) ...[
                SizedBox(height: 24.h),
                for (final line in [
                  ...rows,
                  if (reason.isNotEmpty) reason,
                ]) ...[
                  Text(
                    line,
                    textAlign: TextAlign.center,
                    style: TextStyle(
                      fontSize: 16.sp,
                      fontWeight: FontWeight.w700,
                      color: AppColors.dangerStrong,
                    ),
                  ),
                  SizedBox(height: 8.h),
                ],
              ],
              SizedBox(height: 32.h),
              OutlinedButton(
                key: const ValueKey('account-restricted-delete'),
                onPressed: _busy
                    ? null
                    : () => confirmAndDeleteAccount(
                        context,
                        ref,
                        onBusy: (busy) {
                          if (mounted) setState(() => _busy = busy);
                        },
                      ),
                child: const HkButtonLabel(accountRestrictedDeleteLabel),
              ),
              SizedBox(height: 12.h),
              const ProfileLogoutButton(),
            ],
          ),
        ),
      ),
    );
  }
}
