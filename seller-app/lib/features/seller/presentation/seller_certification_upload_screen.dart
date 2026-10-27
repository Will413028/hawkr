import 'dart:async';
import 'dart:typed_data';

import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_seller/shared/widgets/hk_button_label.dart';
import 'package:hawkr_ui_text/hawkr_ui_text.dart' show commonUnknown;
import 'package:flutter/material.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:image_picker/image_picker.dart';

import '../../../core/l10n/ui_locale.dart';
import '../../../core/theme/app_colors.dart';
import '../../../core/theme/hk_tokens.dart';
import '../../../shared/icons/app_icon_colors.dart';
import '../../../shared/icons/app_icons.dart';
import '../../../shared/utils/hk_log.dart';
import '../../../shared/utils/floating_toast.dart';
import '../../../shared/utils/platform_io.dart';
import '../../../shared/widgets/app_confirm_dialog.dart';
import '../../../shared/widgets/hk_async_state_panel.dart';
import '../../../shared/widgets/hk_frosted_card_surface.dart';
import '../../../shared/widgets/hk_glass_app_bar.dart';
import '../../../shared/widgets/truck_loading_overlay.dart';
import '../data/bank_book_ocr_service.dart';
import '../data/current_kyc.dart';
import "../data/current_store.dart";
import '../data/id_card_ocr_service.dart';
import '../data/kyc_fields.dart';
import '../data/kyc_flow.dart';
import '../data/kyc_form.dart';
import '../data/kyc_ocr.dart';
import '../data/kyc_wizard.dart';
import '../data/payuni.dart';
import '../data/seller_agreements_repository.dart';
import 'kyc_wizard_steps.dart';
import 'kyc_wizard_widgets.dart';
import 'store_address_edit_dialog.dart';
import 'widgets/payuni_status_body.dart';
import 'widgets/certification_journey_stepper.dart';

/// 呼叫端帶來的賣家型態提示（路由參數，可省略）；沒帶或不認得回 null。
KycSubject? kycSubjectOfRoute(String? sellerType) => switch (sellerType) {
  'individual' => KycSubject.individual,
  'business' => KycSubject.business,
  _ => null,
};

/// 商家認證：一步一步填寫，申請是店家的單例資源（`GetMyKYC`／`SaveKYCDraft`／`SubmitKYC`），沒有本機進度。
/// [sellerType] 只是提示：店家還沒有申請時用來建立草稿，已有申請時身分以申請為準，沒帶就讓賣家自己選。
class SellerCertificationUploadScreen extends HookConsumerWidget {
  const SellerCertificationUploadScreen({super.key, this.sellerType});

  /// 賣家類型提示: 'individual' 或 'business'
  final String? sellerType;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final requested = kycSubjectOfRoute(sellerType);
    final picker = useMemoized(() => ImagePicker());
    final kyc = ref.watch(currentKycProvider);
    final wizardState = ref.watch(kycWizardProvider);
    final wizard = ref.read(kycWizardProvider.notifier);
    final stepState = useState<KycStep?>(null);
    final privacyChecked = useState(false);
    final payuniChecked = useState(false);
    final isProcessingOcr = useState(false);

    final entered = useState(false);
    void resetScreen() {
      privacyChecked.value = false;
      payuniChecked.value = false;
      stepState.value = null;
    }

    Future<void> startDraft(KycSubject subject) async {
      await wizard.startDraft(subject);
      if (context.mounted) stepState.value = wizard.resumeStep;
    }

    // 進入時重讀申請（刷新共用快取），缺項與店家缺項才不會是過期的。重讀失敗時快取的 AsyncError 仍帶著先前的值，所以由重讀的
    // 結果決定：失敗就顯示重試，不用過期的申請載入。
    final rereadFailed = useState(false);
    Future<void> enter() async {
      final ok = await ref.read(kycReaderProvider.notifier).refresh();
      if (!context.mounted) return;
      rereadFailed.value = !ok;
      entered.value = true;
    }

    useEffect(() {
      scheduleMicrotask(enter);
      return null;
    }, const []);

    useEffect(() {
      final loaded = kyc?.value;
      if (!entered.value ||
          rereadFailed.value ||
          loaded == null ||
          wizardState.hydrated) {
        return null;
      }
      scheduleMicrotask(() async {
        if (!context.mounted || wizard.hydrated) return;
        wizard.hydrate(requested: requested);
        stepState.value = wizard.resumeStep;
        if (loaded.application == null && requested != null) {
          await startDraft(requested);
        }
      });
      return null;
    }, [kyc?.value, entered.value, rereadFailed.value]);

    final subject = wizardState.subject;
    final steps = subject?.steps ?? const <KycStep>[];
    final current = steps.isEmpty
        ? null
        : (steps.contains(stepState.value) ? stepState.value! : steps.first);

    final application = wizard.application;
    final locked = application != null && !wizard.editable;
    final busy = wizard.busy;

    // 別處取消了申請（衝突時發現）：和捨棄草稿一樣回到第一步、取消兩個勾選。
    final hasApplication = application != null;
    useEffect(() {
      if (wizardState.hydrated && !hasApplication) {
        scheduleMicrotask(() {
          if (context.mounted) resetScreen();
        });
      }
      return null;
    }, [wizardState.hydrated, hasApplication]);

    Future<void> exit() async {
      await wizard.saveBeforeExit();
      if (context.mounted) context.pop();
    }

    Future<void> handleBack() async {
      if (busy) return;
      if (locked || application == null) {
        context.pop();
        return;
      }
      if (current != null && steps.indexOf(current) > 0) {
        stepState.value = steps[steps.indexOf(current) - 1];
        return;
      }
      await exit();
    }

    Future<ImageSource?> chooseImageSource() =>
        showModalBottomSheet<ImageSource>(
          context: context,
          backgroundColor: AppColors.bgCard,
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.vertical(top: Radius.circular(24.r)),
          ),
          builder: (context) => SafeArea(
            child: Padding(
              padding: EdgeInsets.symmetric(vertical: 20.h),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  ListTile(
                    leading: const Icon(
                      AppIcons.cameraAlt,
                      color: AppIconColors.secondary,
                    ),
                    title: const Text('拍照'),
                    onTap: () => Navigator.pop(context, ImageSource.camera),
                  ),
                  ListTile(
                    leading: const Icon(
                      AppIcons.photoLibrary,
                      color: AppIconColors.secondary,
                    ),
                    title: const Text('從相簿選擇'),
                    onTap: () => Navigator.pop(context, ImageSource.gallery),
                  ),
                  SizedBox(height: 8.h),
                  ListTile(
                    leading: const Icon(
                      AppIcons.close,
                      color: AppIconColors.muted,
                    ),
                    title: const Text('取消'),
                    onTap: () => Navigator.pop(context),
                  ),
                ],
              ),
            ),
          ),
        );

    Future<({File file, Uint8List bytes})?> pickImage() async {
      final source = await chooseImageSource();
      if (source == null) return null;
      try {
        final image = await picker.pickImage(source: source, imageQuality: 80);
        if (image == null) return null;
        final file = File(image.path);
        return (file: file, bytes: await file.readAsBytes());
      } catch (error) {
        HKLog.logW('[CertUpload] pick image failed: $error');
        if (context.mounted) FloatingToast.error(context, '選取圖片失敗');
        return null;
      }
    }

    /// 裝置上的 OCR：選圖當下辨識，只預填空白欄位，不覆蓋使用者已填的值；辨識失敗不擋流程。
    Future<void> recognize(KycDocSlot slot, File file) async {
      isProcessingOcr.value = true;
      try {
        switch (slot) {
          case KycDocSlot.idCardFront:
            final result = await IdCardOcrService.recognizeFront(file);
            if (!context.mounted) return;
            wizard.updateForm(
              (f) => applyIdCardOcr(
                f,
                result,
                hasMaskedId: wizard.maskedId.isNotEmpty,
                identityLocked: wizard.identityLocked,
              ),
            );
          case KycDocSlot.idCardBack:
            final result = await IdCardOcrService.recognizeBack(file);
            if (!context.mounted) return;
            wizard.updateForm(
              (f) => applyIdCardOcr(
                f,
                result,
                hasMaskedId: wizard.maskedId.isNotEmpty,
                identityLocked: wizard.identityLocked,
              ),
            );
          case KycDocSlot.bankBook:
            final result = await BankBookOcrService.recognize(file);
            if (!context.mounted) return;
            wizard.updateForm(
              (f) => applyBankBookOcr(
                f,
                result,
                hasMaskedAccount: wizard.maskedBankAccount.isNotEmpty,
              ),
            );
        }
      } catch (error) {
        HKLog.logW('[CertUpload] OCR failed: $error');
        if (context.mounted) FloatingToast.error(context, '辨識失敗，請手動填寫');
      } finally {
        if (context.mounted) isProcessingOcr.value = false;
      }
    }

    Future<void> pickDocument(KycDocSlot slot) async {
      final picked = await pickImage();
      if (picked == null || !context.mounted) return;
      final stamp = DateTime.now().millisecondsSinceEpoch;
      await wizard.uploadDocument(
        slot,
        picked.bytes,
        docType: slot == KycDocSlot.bankBook ? 'bank_account' : 'id_card',
        filename: '${slot.path}_$stamp.jpg',
      );
      if (!context.mounted || wizard.docs.slot(slot) == null) return;
      await recognize(slot, picked.file);
    }

    Future<void> addCompanyDocument() async {
      final picked = await pickImage();
      if (picked == null || !context.mounted) return;
      await wizard.addCompanyDocument(
        picked.bytes,
        filename:
            'doc_${wizard.docs.companyDocuments.length}_${DateTime.now().millisecondsSinceEpoch}.jpg',
      );
    }

    Future<void> next() async {
      if (wizard.busy || current == null) return;
      if (current == KycStep.other && !await wizard.flushStore()) return;
      if (!await wizard.save() || !context.mounted) return;
      final index = steps.indexOf(current);
      if (index < steps.length - 1) stepState.value = steps[index + 1];
    }

    Future<void> editAddress({
      required String title,
      required String townCode,
      required String detail,
      required void Function(String townCode, String detail) apply,
    }) async {
      await showDialog<bool>(
        context: context,
        builder: (_) => StoreAddressEditDialog(
          title: title,
          initialTownCode: townCode,
          initialDetail: detail,
          onSave: (town, addressDetail) async {
            apply(town, addressDetail);
            return null;
          },
        ),
      );
    }

    Future<void> submit() async {
      final agreements = ref.read(kycAgreementsProvider).value;
      if (agreements == null) return;
      final outcome = await wizard.submit(agreements.acceptances);
      if (!context.mounted) return;
      switch (outcome) {
        case KycSubmitOutcome.submitted:
          await _showSubmittedDialog(
            context,
            business: wizard.subject == KycSubject.business,
          );
          if (context.mounted) context.pop(true);
        case KycSubmitOutcome.agreementOutdated:
          privacyChecked.value = false;
          payuniChecked.value = false;
        case KycSubmitOutcome.failed:
          break;
      }
    }

    Future<void> discard() async {
      final confirmed = await showAppConfirmDialog(
        context,
        title: '確定要捨棄目前的認證草稿？已上傳資料會清除。',
        confirmText: '捨棄草稿',
        confirmTextColor: AppColors.danger,
      );
      if (!confirmed || !context.mounted) return;
      if (await wizard.discard() && context.mounted) resetScreen();
    }

    final actions = KycStepActions(
      next: next,
      pickDocument: pickDocument,
      addCompanyDocument: addCompanyDocument,
      editStoreAddress: () => editAddress(
        title: '編輯店家地址',
        townCode: wizard.store.townCode,
        detail: wizard.store.addressDetail,
        apply: (town, detail) => wizard.updateStore(
          (s) => s.copyWith(townCode: town, addressDetail: detail),
        ),
      ),
      editCompanyAddress: () => editAddress(
        title: '編輯公司地址',
        townCode: wizard.form.companyTownCode,
        detail: wizard.form.companyAddressDetail,
        apply: (town, detail) => wizard.updateForm(
          (f) =>
              f.copyWith(companyTownCode: town, companyAddressDetail: detail),
        ),
      ),
      jumpTo: (section) {
        for (final step in steps) {
          if (step.section == section) {
            stepState.value = step;
            return;
          }
        }
      },
      submit: submit,
      retryAgreements: () => ref.invalidate(sellerAgreementsProvider),
    );

    final agreements = current == KycStep.confirm
        ? ref.watch(kycAgreementsProvider)
        : const AsyncLoading<KycAgreements>();

    // 輸入統編滿 8 碼時自動查登記資料。
    final taxId = wizardState.form.taxId;
    final autoLookup =
        wizardState.hydrated &&
        subject == KycSubject.business &&
        wizard.editable &&
        !wizard.identityLocked &&
        current == KycStep.company;
    final lookupIdle = wizardState.lookup is KycLookupIdle;
    useEffect(() {
      if (!autoLookup || !RegExp(r'^\d{8}$').hasMatch(taxId) || !lookupIdle) {
        return null;
      }
      final timer = Timer(
        const Duration(milliseconds: 500),
        () => unawaited(wizard.lookupTaxId(taxId)),
      );
      return timer.cancel;
    }, [autoLookup, taxId, lookupIdle]);

    Widget body() {
      if (kyc == null) return const SizedBox.shrink();
      if (!wizardState.hydrated) {
        if (entered.value && (kyc.hasError || rereadFailed.value)) {
          return Padding(
            padding: const EdgeInsets.all(HkSpace.lg),
            child: HkAsyncStatePanel(
              kind: HkAsyncStateKind.error,
              title: '無法載入認證資料',
              description: '請檢查網路後再試一次。',
              actionLabel: '重試',
              onAction: () {
                rereadFailed.value = false;
                unawaited(enter());
              },
            ),
          );
        }
        return const TruckLoadingOverlay(message: '載入進度中...');
      }
      if (wizardState.creating) {
        return const TruckLoadingOverlay(message: '建立認證草稿中...');
      }
      if (application != null && subject == null) {
        return Padding(
          padding: const EdgeInsets.all(HkSpace.lg),
          child: HkAsyncStatePanel(
            kind: HkAsyncStateKind.error,
            title: '無法載入認證資料',
            description: '請檢查網路後再試一次。',
            actionLabel: '重試',
            onAction: () => ref.read(kycReaderProvider.notifier).refresh(),
          ),
        );
      }
      if (locked) {
        return _LockedStatusView(
          application: application,
          certified: ref.watch(myStoreSnapshotProvider)?.isCertified ?? false,
        );
      }
      return Stack(
        children: [
          SingleChildScrollView(
            physics: const AlwaysScrollableScrollPhysics(),
            padding: const EdgeInsets.fromLTRB(
              HkSpace.lg,
              HkSpace.lg,
              HkSpace.lg,
              120,
            ),
            child: Center(
              child: ConstrainedBox(
                constraints: const BoxConstraints(maxWidth: 560),
                child: Column(
                  children: [
                    CertificationJourneyStepper(
                      currentStep: current?.journeyStep ?? 0,
                    ),
                    if (wizardState.error != null) ...[
                      const SizedBox(height: HkSpace.lg),
                      _ErrorBanner(
                        message: wizardState.error!,
                        onDismiss: wizard.dismissError,
                      ),
                    ],
                    if (application != null) ...[
                      _RejectionBanner(
                        application: application,
                        rejection: wizard.rejection,
                        issues: wizard.outstandingIssues,
                      ),
                      _PayuniProblemBanner(
                        payuni:
                            application.hasFlow() &&
                                application.flow.hasPayuni()
                            ? application.flow.payuni
                            : null,
                      ),
                    ],
                    const SizedBox(height: HkSpace.lg),
                    if (application == null || current == null)
                      _SubjectPicker(
                        selected: subject,
                        busy: busy,
                        onSelected: wizard.chooseSubject,
                        onStart: subject == null
                            ? null
                            : () => startDraft(subject),
                      )
                    else ...[
                      AnimatedSwitcher(
                        duration:
                            MediaQuery.maybeOf(context)?.disableAnimations ??
                                false
                            ? Duration.zero
                            : HkMotion.duration,
                        switchInCurve: HkMotion.ease,
                        switchOutCurve: Curves.easeIn,
                        child: KeyedSubtree(
                          key: ValueKey(current),
                          child: KycStepCard(
                            step: current,
                            controller: wizard,
                            actions: actions,
                            agreements: agreements,
                            privacyChecked: privacyChecked.value,
                            payuniChecked: payuniChecked.value,
                            onPrivacyChanged: (v) => privacyChecked.value = v,
                            onPayuniChanged: (v) => payuniChecked.value = v,
                          ),
                        ),
                      ),
                      if (wizard.canDiscard)
                        Padding(
                          padding: const EdgeInsets.only(top: HkSpace.md),
                          child: TextButton(
                            key: const Key('kyc_discard'),
                            onPressed: busy ? null : discard,
                            child: HkButtonLabel(
                              current == steps.first ? '更換商家類型 / 重新開始' : '捨棄草稿',
                            ),
                          ),
                        ),
                    ],
                  ],
                ),
              ),
            ),
          ),
          if (isProcessingOcr.value)
            Container(
              color: AppColors.textSecondary,
              child: const TruckLoadingOverlay(message: '正在辨識證件資料...'),
            ),
        ],
      );
    }

    return PopScope(
      canPop: false,
      onPopInvokedWithResult: (didPop, result) async {
        if (didPop || busy) return;
        if (locked || application == null) {
          context.pop();
          return;
        }
        await exit();
      },
      child: Scaffold(
        backgroundColor: AppColors.bgPrimary,
        appBar: HkGlassAppBar(
          title: const Text('商家認證'),
          subtitle: locked
              ? '申請狀態'
              : (current == null || application == null)
              ? null
              : '資料步驟 ${steps.indexOf(current) + 1} / ${steps.length}',
          onBackPressed: handleBack,
        ),
        body: DecoratedBox(
          decoration: BoxDecoration(
            gradient: LinearGradient(
              begin: Alignment.topLeft,
              end: Alignment.bottomRight,
              colors: [
                AppColors.brandPrimaryStrong.withValues(alpha: 0.10),
                AppColors.bgPrimary,
                AppColors.brandSecondary.withValues(alpha: 0.08),
              ],
            ),
          ),
          child: body(),
        ),
      ),
    );
  }
}

Future<void> _showSubmittedDialog(
  BuildContext context, {
  required bool business,
}) {
  return showDialog<void>(
    context: context,
    barrierDismissible: false,
    builder: (context) => Dialog(
      backgroundColor: AppColors.bgCard,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(24.r)),
      child: Padding(
        padding: EdgeInsets.fromLTRB(24.w, 32.h, 24.w, 24.h),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(
              '已送出申請',
              style: TextStyle(
                fontSize: 18.sp,
                fontWeight: FontWeight.w700,
                color: AppColors.textEmphasis,
              ),
            ),
            SizedBox(height: 8.h),
            Text(
              '審核完成後會以通知告知你結果。',
              textAlign: TextAlign.center,
              style: TextStyle(
                fontSize: 16.sp,
                color: AppColors.textEmphasis,
                height: 1.6,
              ),
            ),
            SizedBox(height: 8.h),
            Text(
              '送出後 PayUni 預設密碼為你的${business ? '統一編號' : '身分證字號'}',
              textAlign: TextAlign.center,
              style: TextStyle(
                fontSize: 14.sp,
                fontWeight: FontWeight.w700,
                color: AppColors.warningText,
                height: 1.6,
              ),
            ),
            SizedBox(height: 24.h),
            KycPrimaryButton(
              label: '我知道了',
              onTap: () => Navigator.of(context).pop(),
            ),
          ],
        ),
      ),
    ),
  );
}

class _ErrorBanner extends StatelessWidget {
  const _ErrorBanner({required this.message, required this.onDismiss});

  final String message;
  final VoidCallback onDismiss;

  @override
  Widget build(BuildContext context) {
    return Container(
      key: const Key('kyc_error'),
      width: double.infinity,
      padding: EdgeInsets.fromLTRB(12.w, 8.h, 4.w, 8.h),
      decoration: BoxDecoration(
        color: AppColors.dangerSoft,
        borderRadius: BorderRadius.circular(HkRadius.md),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Expanded(
            child: Padding(
              padding: EdgeInsets.only(top: 6.h),
              child: Text(
                message,
                style: TextStyle(fontSize: 14.sp, color: AppColors.danger),
              ),
            ),
          ),
          IconButton(
            tooltip: '關閉',
            onPressed: onDismiss,
            icon: const Icon(
              AppIcons.close,
              size: 18,
              color: AppIconColors.muted,
            ),
          ),
        ],
      ),
    );
  }
}

/// 被平台退回的申請：狀態說明、退回原因與伺服器算的待處理問題（欄位名稱加說明）。只看 [rejection]；PayUni 的問題另由
/// [_PayuniProblemBanner] 呈現。
class _RejectionBanner extends StatelessWidget {
  const _RejectionBanner({
    required this.application,
    required this.rejection,
    required this.issues,
  });

  final KYCApplication application;
  final KYCRejection? rejection;
  final List<KYCIssue> issues;

  @override
  Widget build(BuildContext context) {
    final app = application;
    if (!app.hasFlow()) return const SizedBox.shrink();
    final rejection = this.rejection;
    if (rejection == null) return const SizedBox.shrink();
    final l = context.uiLocale;
    final copy = kycFlowCopy(KYCFlowStatus.KYC_FLOW_STATUS_REJECTED, l);
    return Padding(
      padding: const EdgeInsets.only(top: HkSpace.lg),
      child: SizedBox(
        width: double.infinity,
        child: HkFrostedCardSurface(
          radius: HkRadius.lg,
          shadows: HkShadows.sm,
          child: Padding(
            padding: const EdgeInsets.all(HkSpace.lg),
            child: Column(
              key: const Key('kyc_rejection'),
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  copy.bannerTitle ?? commonUnknown(l),
                  style: TextStyle(
                    fontSize: 14.sp,
                    fontWeight: FontWeight.w700,
                    color: AppColors.danger,
                  ),
                ),
                SizedBox(height: 4.h),
                Text(
                  copy.bannerDescription ?? commonUnknown(l),
                  style: TextStyle(fontSize: 14.sp, color: AppColors.gray1),
                ),
                SizedBox(height: 8.h),
                Text(
                  rejection.reason.isNotEmpty
                      ? rejection.reason
                      : '請重新確認並提交完整資料。',
                  style: TextStyle(fontSize: 14.sp, color: AppColors.gray1),
                ),
                for (final issue in issues) ...[
                  SizedBox(height: 8.h),
                  Text(
                    '${kycFieldLabel(issue.field_1, l) ?? '其他'}：${issue.message}',
                    style: TextStyle(fontSize: 14.sp, color: AppColors.gray1),
                  ),
                ],
              ],
            ),
          ),
        ),
      ),
    );
  }
}

/// 賣家正在修正 PayUni 拒絕或綁定失敗的申請（FIXABLE，含存成草稿之後）：PayUni 的標題、伺服器的說明、審核意見與要修正的欄位。
/// 沒有 PayUni 狀態時不顯示；樣式沿用 [_RejectionBanner]。
class _PayuniProblemBanner extends StatelessWidget {
  const _PayuniProblemBanner({required this.payuni});

  final SellerPayUni? payuni;

  @override
  Widget build(BuildContext context) {
    final payuni = this.payuni;
    if (!payuniHasState(payuni)) return const SizedBox.shrink();
    return Padding(
      padding: const EdgeInsets.only(top: HkSpace.lg),
      child: SizedBox(
        width: double.infinity,
        child: HkFrostedCardSurface(
          radius: HkRadius.lg,
          shadows: HkShadows.sm,
          child: Padding(
            padding: const EdgeInsets.all(HkSpace.lg),
            child: Column(
              key: const Key('kyc_payuni_problem'),
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  payuniStateTitle(payuni!.state, context.uiLocale),
                  style: TextStyle(
                    fontSize: 14.sp,
                    fontWeight: FontWeight.w700,
                    color: AppColors.danger,
                  ),
                ),
                SizedBox(height: 4.h),
                PayuniStatusBody(payuni: payuni),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

/// 還沒有申請（或剛捨棄草稿）：選個人賣家或企業用戶，按下開始才建立草稿。
class _SubjectPicker extends StatelessWidget {
  const _SubjectPicker({
    required this.selected,
    required this.busy,
    required this.onSelected,
    required this.onStart,
  });

  final KycSubject? selected;
  final bool busy;
  final ValueChanged<KycSubject> onSelected;
  final VoidCallback? onStart;

  @override
  Widget build(BuildContext context) {
    Widget option(KycSubject subject, String title, List<String> lines) {
      final active = selected == subject;
      return InkWell(
        key: Key('kyc_subject_${subject.name}'),
        onTap: () => onSelected(subject),
        borderRadius: BorderRadius.circular(HkRadius.lg),
        child: Container(
          width: double.infinity,
          padding: const EdgeInsets.all(HkSpace.lg),
          decoration: BoxDecoration(
            color: AppColors.bgCard.withValues(alpha: 0.8),
            borderRadius: BorderRadius.circular(HkRadius.lg),
            border: Border.all(
              color: active ? AppColors.primary : AppColors.dividerLight,
              width: active ? 2 : 1,
            ),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                title,
                style: TextStyle(
                  fontSize: 16.sp,
                  fontWeight: FontWeight.w700,
                  color: AppColors.textEmphasis,
                ),
              ),
              for (final line in lines)
                Text(
                  line,
                  style: TextStyle(
                    fontSize: 12.sp,
                    color: AppColors.textSecondary,
                  ),
                ),
            ],
          ),
        ),
      );
    }

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Text(
          '選擇商家類型',
          style: TextStyle(
            fontSize: 18.sp,
            fontWeight: FontWeight.w700,
            color: AppColors.textEmphasis,
          ),
        ),
        SizedBox(height: 4.h),
        Text(
          '選擇後將建立認證草稿，可隨時暫存進度。',
          style: TextStyle(fontSize: 14.sp, color: AppColors.textSecondary),
        ),
        SizedBox(height: 16.h),
        option(KycSubject.individual, '個人賣家', const [
          '以個人名義販售農產品 / 自家產品',
          '需身分證 + 存摺',
        ]),
        SizedBox(height: 12.h),
        option(KycSubject.business, '企業用戶', const [
          '以行號或公司名義販售（需統一編號）',
          '需公司資訊 + 負責人身分證 + 公司帳戶存摺',
        ]),
        SizedBox(height: 24.h),
        KycPrimaryButton(label: '開始填寫', busy: busy, onTap: onStart),
      ],
    );
  }
}

/// 已送出審核 / 已完成認證的唯讀狀態畫面（不含任何可編輯欄位或送出按鈕）。
class _LockedStatusView extends StatelessWidget {
  const _LockedStatusView({required this.application, required this.certified});

  final KYCApplication application;
  final bool certified;

  @override
  Widget build(BuildContext context) {
    final status = certified
        ? KYCFlowStatus.KYC_FLOW_STATUS_APPROVED
        : application.flow.status;
    final l = context.uiLocale;
    final copy = kycFlowCopy(status, l);
    final approved = status == KYCFlowStatus.KYC_FLOW_STATUS_APPROVED;
    final payuni =
        !certified && application.hasFlow() && application.flow.hasPayuni()
        ? application.flow.payuni
        : null;
    final showPayuni = payuniHasState(payuni);
    return Center(
      child: SingleChildScrollView(
        padding: const EdgeInsets.all(24),
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 520),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              const CertificationJourneyStepper(currentStep: 3),
              const SizedBox(height: 16),
              HkFrostedCardSurface(
                radius: HkRadius.xl,
                shadows: HkShadows.lg,
                child: Padding(
                  padding: const EdgeInsets.all(24),
                  child: Column(
                    key: const Key('kyc_locked'),
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Container(
                        width: 64,
                        height: 64,
                        decoration: BoxDecoration(
                          color: approved
                              ? AppColors.primarySoft
                              : AppColors.bgCard,
                          shape: BoxShape.circle,
                        ),
                        child: Icon(
                          approved
                              ? AppIcons.checkCircle
                              : AppIcons.hourglassTop,
                          size: 32,
                          color: approved
                              ? AppIconColors.success
                              : AppIconColors.active,
                        ),
                      ),
                      const SizedBox(height: 16),
                      Text(
                        showPayuni
                            ? payuniStateTitle(payuni!.state, l)
                            : copy.bannerTitle ?? commonUnknown(l),
                        textAlign: TextAlign.center,
                        style: const TextStyle(
                          fontSize: 18,
                          fontWeight: FontWeight.w700,
                          color: AppColors.textEmphasis,
                        ),
                      ),
                      const SizedBox(height: 8),
                      if (showPayuni)
                        PayuniStatusBody(
                          payuni: payuni!,
                          textAlign: TextAlign.center,
                        )
                      else
                        Text(
                          copy.bannerDescription ?? commonUnknown(l),
                          textAlign: TextAlign.center,
                          style: const TextStyle(
                            fontSize: 14,
                            color: AppColors.gray1,
                            height: 1.5,
                          ),
                        ),
                    ],
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
