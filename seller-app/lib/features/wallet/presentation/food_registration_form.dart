import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart'
    show AgreementAcceptance;
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pbenum.dart'
    show AgreementKind;
import 'package:hawkr_api/taipei_time.dart' show formatTaipeiDate;
import 'package:hawkr_ui_text/hawkr_ui_text.dart';
import 'package:flutter/material.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../core/l10n/ui_locale.dart';
import '../../../core/theme/app_colors.dart';
import '../../../core/theme/app_typography.dart';
import '../../../core/theme/hk_tokens.dart';
import '../../../shared/widgets/app_confirm_dialog.dart';
import '../../../shared/widgets/hk_button_label.dart';
import '../../seller/data/current_store.dart';
import '../../seller/data/seller_agreements_repository.dart';
import '../../seller/data/store_editor.dart';
import '../../seller/presentation/kyc_wizard_widgets.dart';
import '../../seller/presentation/widgets/agreement_checkbox.dart';

/// 食品業者登錄的送審：送出審核、撤回待審、更換字號、清除。內容和 website 的 `FoodRegSupplement` 同一套語意與文字。
class FoodRegistrationForm extends HookConsumerWidget {
  const FoodRegistrationForm({super.key, this.kycApproved = false});

  /// 商家認證（KYC）已核准；還沒核准時審核中的狀態多一句說明。
  final bool kycApproved;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l = context.uiLocale;
    final my = ref.watch(myStoreSnapshotProvider);
    final approved = my?.foodRegistrationNumber;
    // 狀態由欄位有沒有值表達（待審、被退回各一個欄位），不比較登錄的 state；
    // 帳號能不能操作由錢包頁的 STOREFRONT_WRITE 閘門擋。
    final pending = my?.pendingFoodRegistration;
    final rejected = my?.rejectedFoodRegistration;

    // 撤銷的說明由伺服器決定要不要帶（Store.revoked_food_registration），原樣顯示，不自己推導。
    final revoked = my?.revokedFoodRegistration;
    final notice = ref.watch(foodRegistrationNoticeProvider);
    final noticeRead = useState(false);
    final replacing = useState(false);
    final regNo = useState('');
    final busy = useState(false);
    final error = useState<String?>(null);

    final showInput = replacing.value || (approved == null && pending == null);
    final normalized = regNo.value.trim();

    Future<void> run(Future<StoreSaveOutcome> Function() action) async {
      error.value = null;
      busy.value = true;
      try {
        final outcome = await action();
        if (!context.mounted) return;
        switch (outcome) {
          case StoreSaved():
            regNo.value = '';
            noticeRead.value = false;
            replacing.value = false;
          case StoreSaveFailed(agreementOutdated: true):
            noticeRead.value = false;
            ref.invalidate(sellerAgreementsProvider);
            error.value = storeSaveMessage(outcome);
          case StoreSaveConflict() ||
              StoreSaveFieldErrors() ||
              StoreSaveFailed():
            error.value = storeSaveMessage(
              outcome,
              fieldPaths: const ['registration_number'],
            );
        }
      } finally {
        if (context.mounted) busy.value = false;
      }
    }

    Future<void> clear() async {
      final confirmed = await showAppConfirmDialog(
        context,
        title: '清除食品業者登錄',
        message: '清除後，店面與商品頁不再顯示食品業者登錄字號，也無法上架加工食品類商品。',
        confirmText: '清除',
        confirmTextColor: AppColors.danger,
      );
      if (!confirmed || !context.mounted) return;
      await run(ref.read(storeEditorProvider).clearFoodRegistration);
    }

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Text('食品業者登錄字號', style: AppTypography.bodyBold),
        const SizedBox(height: HkSpace.xs),
        Text(
          '上架加工食品前須完成食品業者登錄。送出後平台會核對食藥署的登錄資料，通過後才會顯示在店面與商品頁，並開放上架加工食品。審核期間沿用目前的設定。',
          style: AppTypography.caption.copyWith(color: AppColors.textSecondary),
        ),
        if (approved != null) ...[
          const SizedBox(height: HkSpace.md),
          Text(
            sellerFoodRegistrationStatusApproved(l, number: approved),
            key: const ValueKey('food-status-approved'),
            style: AppTypography.body.copyWith(color: AppColors.success),
          ),
        ],
        if (pending != null) ...[
          const SizedBox(height: HkSpace.md),
          Text(
            sellerFoodRegistrationStatusPending(
              l,
              number: pending.registrationNumber,
              // 伺服器沒給送出時間時不顯示日期（不把 epoch 當成 1970/01/01）
              date: pending.hasSubmitTime()
                  ? formatTaipeiDate(pending.submitTime.toDateTime())
                  : '',
            ),
            key: const ValueKey('food-status-pending'),
            style: AppTypography.body,
          ),
          if (!kycApproved)
            Text(
              sellerFoodRegistrationKycPendingNote(l),
              key: const ValueKey('food-status-kyc-note'),
              style: AppTypography.caption.copyWith(
                color: AppColors.textSecondary,
              ),
            ),
        ],
        if (revoked != null) ...[
          const SizedBox(height: HkSpace.md),
          Text(
            sellerFoodRegistrationStatusRevoked(
              l,
              number: revoked.registrationNumber,
              reason: revoked.endReason,
            ),
            key: const ValueKey('food-status-revoked'),
            style: AppTypography.body.copyWith(color: AppColors.danger),
          ),
        ],
        if (rejected != null) ...[
          const SizedBox(height: HkSpace.md),
          Text(
            sellerFoodRegistrationStatusRejected(
              l,
              reason: rejected.rejectReason,
            ),
            key: const ValueKey('food-status-rejected'),
            style: AppTypography.body.copyWith(color: AppColors.danger),
          ),
        ],
        if (showInput) ...[
          const SizedBox(height: HkSpace.md),
          KycTextField(
            label: '食品業者登錄字號',
            helper: '食藥署食品業者登錄字號（例：A-123456789-00000-1）',
            hint: 'A-123456789-00000-1',
            value: regNo.value,
            inputFormatters: [UpperCaseTextFormatter()],
            onChanged: (value) {
              regNo.value = value;
              error.value = null;
            },
          ),
          const SizedBox(height: HkSpace.sm),
          AgreementCheckbox(
            checkboxKey: const Key('food_notice_checkbox'),
            before: '我已閱讀〈',
            linkText: '食品業者登錄個人資料蒐集告知事項',
            after: '〉',
            url: notice.value?.url,
            loadFailed: notice.hasError,
            checked: noticeRead.value,
            enabled: !busy.value,
            onChanged: (value) {
              noticeRead.value = value;
              error.value = null;
            },
            onRetry: () => ref.invalidate(sellerAgreementsProvider),
          ),
        ],
        if (error.value case final message?) ...[
          const SizedBox(height: HkSpace.md),
          Text(
            message,
            style: AppTypography.caption.copyWith(color: AppColors.danger),
          ),
        ],
        const SizedBox(height: HkSpace.md),
        if (showInput)
          KycPrimaryButton(
            key: const ValueKey('food-submit'),
            label: busy.value ? '送出中…' : (revoked != null ? '重新送出' : '送出審核'),
            busy: busy.value,
            onTap:
                normalized.isEmpty || !noticeRead.value || notice.value == null
                ? null
                : () => run(
                    () => ref
                        .read(storeEditorProvider)
                        .submitFoodRegistration(normalized, [
                          AgreementAcceptance(
                            kind: AgreementKind
                                .AGREEMENT_KIND_FOOD_REGISTRATION_NOTICE,
                            version: notice.value!.version,
                          ),
                        ]),
                  ),
          ),
        if (showInput && replacing.value) ...[
          const SizedBox(height: HkSpace.sm),
          _SecondaryButton(
            key: const ValueKey('food-replace-cancel'),
            label: '取消',
            onTap: busy.value
                ? null
                : () {
                    replacing.value = false;
                    regNo.value = '';
                    noticeRead.value = false;
                    error.value = null;
                  },
          ),
        ],
        if (pending != null) ...[
          if (showInput) const SizedBox(height: HkSpace.sm),
          _SecondaryButton(
            key: const ValueKey('food-cancel'),
            label: '撤回',
            onTap: busy.value
                ? null
                : () => run(
                    ref.read(storeEditorProvider).withdrawFoodRegistration,
                  ),
          ),
        ],
        if (approved != null) ...[
          const SizedBox(height: HkSpace.sm),
          _SecondaryButton(
            key: const ValueKey('food-replace'),
            label: '更換字號',
            onTap: busy.value ? null : () => replacing.value = true,
          ),
          const SizedBox(height: HkSpace.sm),
          _SecondaryButton(
            key: const ValueKey('food-clear'),
            label: '清除',
            onTap: busy.value ? null : clear,
          ),
        ],
      ],
    );
  }
}

class _SecondaryButton extends StatelessWidget {
  const _SecondaryButton({super.key, required this.label, required this.onTap});

  final String label;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    return OutlinedButton(
      onPressed: onTap,
      style: OutlinedButton.styleFrom(
        foregroundColor: AppColors.textPrimary,
        minimumSize: const Size.fromHeight(HkSpace.control),
        shape: const StadiumBorder(),
      ),
      child: HkButtonLabel(label),
    );
  }
}
