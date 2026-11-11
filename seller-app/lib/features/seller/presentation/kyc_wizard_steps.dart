import 'dart:async';

import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pbenum.dart';
import 'package:hawkr_twaddr/hawkr_twaddr.dart' as twaddr;
import 'package:hawkr_ui_text/hawkr_ui_text.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../core/l10n/ui_locale.dart';
import '../../../core/theme/app_colors.dart';
import '../../../shared/icons/app_icon_colors.dart';
import '../../../shared/widgets/hk_icon.dart';
import '../../../shared/widgets/hk_button_label.dart';
import '../data/contact_email.dart';
import '../data/kyc_fields.dart';
import '../data/kyc_form.dart';
import '../data/kyc_store_contact.dart';
import '../data/kyc_wizard.dart';
import '../data/seller_agreements_repository.dart';
import 'kyc_wizard_widgets.dart';
import 'widgets/agreement_checkbox.dart';
import 'widgets/certification_glass_step_card.dart';

/// 步驟裡會觸發的動作，由畫面實作（選圖、開地址對話框、開條款）。
class KycStepActions {
  const KycStepActions({
    required this.next,
    required this.addCompanyDocument,
    required this.editStoreAddress,
    required this.editCompanyAddress,
    required this.jumpTo,
    required this.submit,
    required this.retryAgreements,
  });

  /// 存檔後前進一步。
  final Future<void> Function() next;
  final Future<void> Function() addCompanyDocument;
  final Future<void> Function() editStoreAddress;
  final Future<void> Function() editCompanyAddress;
  final void Function(KycSection section) jumpTo;
  final Future<void> Function() submit;
  final VoidCallback retryAgreements;
}

/// 表單控制項的標籤：與缺項清單同一組共用文案目錄的欄位名詞。
String _fieldLabel(BuildContext context, KYCField field) =>
    field.uiText(context.uiLocale) ?? commonUnknown(context.uiLocale);

Widget _card({
  required String title,
  required String subtitle,
  required HkIconName icon,
  required List<Widget> children,
}) => SizedBox(
  width: double.infinity,
  child: CertificationGlassStepCard(
    title: title,
    subtitle: subtitle,
    icon: icon,
    child: Column(children: children),
  ),
);

Widget _gap([double h = 16]) => SizedBox(height: h.h);

/// 目前這一步的卡片。
class KycStepCard extends StatelessWidget {
  const KycStepCard({
    super.key,
    required this.step,
    required this.controller,
    required this.actions,
    required this.agreements,
    required this.privacyChecked,
    required this.onPrivacyChanged,
  });

  final KycStep step;
  final KycWizard controller;
  final KycStepActions actions;
  final AsyncValue<KycAgreements> agreements;
  final bool privacyChecked;
  final ValueChanged<bool> onPrivacyChanged;

  @override
  Widget build(BuildContext context) {
    return switch (step) {
      KycStep.company => _CompanyStep(controller, actions),
      KycStep.other => _OtherStep(controller, actions),
      KycStep.documents => _DocumentsStep(controller, actions),
      KycStep.confirm => _ConfirmStep(
        controller: controller,
        actions: actions,
        agreements: agreements,
        privacyChecked: privacyChecked,
        onPrivacyChanged: onPrivacyChanged,
      ),
    };
  }
}

KycButtonBar _nextBar(
  KycWizard c,
  KycStepActions actions, {
  bool enabled = true,
}) => KycButtonBar(
  label: '下一步',
  busy: c.saving || c.storeSaving,
  onTap: enabled && !c.busy ? actions.next : null,
);

/// 步驟底部的主按鈕（含上方的間距）。
class KycButtonBar extends StatelessWidget {
  const KycButtonBar({
    super.key,
    required this.label,
    required this.onTap,
    this.busy = false,
  });

  final String label;
  final Future<void> Function()? onTap;
  final bool busy;

  @override
  Widget build(BuildContext context) => Padding(
    padding: EdgeInsets.only(top: 28.h),
    child: KycPrimaryButton(
      label: label,
      busy: busy,
      onTap: onTap == null ? null : () => onTap!(),
    ),
  );
}

/* ===== 公司資訊（企業） ===== */

class _CompanyStep extends StatelessWidget {
  const _CompanyStep(this.c, this.actions);

  final KycWizard c;
  final KycStepActions actions;

  @override
  Widget build(BuildContext context) {
    final f = c.form;
    final lookup = c.lookup;
    return _card(
      title: '公司資訊',
      subtitle: '輸入統編後可自動帶入公司名稱、地址與設立日期',
      icon: HkIconName.store,
      children: [
        KycTextField(
          key: const Key('kyc_tax_id'),
          label: _fieldLabel(context, KYCField.KYC_FIELD_BUSINESS_TAX_ID),
          value: f.taxId,
          hint: '請輸入統一編號（8碼）',
          keyboardType: TextInputType.number,
          inputFormatters: [
            FilteringTextInputFormatter.digitsOnly,
            LengthLimitingTextInputFormatter(8),
          ],
          error: c.fieldError(KYCField.KYC_FIELD_BUSINESS_TAX_ID),
          helper: '輸入 8 位數字會自動查詢',
          suffix: lookup is KycLookupRunning
              ? Padding(
                  padding: EdgeInsets.only(right: 12.w),
                  child: SizedBox(
                    width: 16.w,
                    height: 16.w,
                    child: const CircularProgressIndicator(strokeWidth: 1.8),
                  ),
                )
              : null,
          onChanged: (v) {
            c.updateForm((f) => f.copyWith(taxId: v));
            c.resetLookup();
          },
        ),
        _LookupNotice(lookup),
        if (lookup is KycLookupNotFound || lookup is KycLookupFailed)
          Align(
            alignment: Alignment.centerRight,
            child: TextButton(
              key: const Key('kyc_tax_id_lookup'),
              onPressed: () => c.lookupTaxId(f.taxId),
              child: const HkButtonLabel('重新查詢'),
            ),
          ),
        _gap(),
        KycTextField(
          key: const Key('kyc_company_name'),
          label: _fieldLabel(context, KYCField.KYC_FIELD_BUSINESS_COMPANY_NAME),
          value: f.companyName,
          hint: '請輸入公司名稱',
          error: c.fieldError(KYCField.KYC_FIELD_BUSINESS_COMPANY_NAME),
          onChanged: (v) => c.updateForm((f) => f.copyWith(companyName: v)),
        ),
        _gap(),
        KycDateField(
          key: const Key('kyc_company_register_date'),
          label: _fieldLabel(
            context,
            KYCField.KYC_FIELD_BUSINESS_COMPANY_REGISTER_DATE,
          ),
          value: f.companyRegisterDate,
          error: c.fieldError(
            KYCField.KYC_FIELD_BUSINESS_COMPANY_REGISTER_DATE,
          ),
          onChanged: (v) =>
              c.updateForm((f) => f.copyWith(companyRegisterDate: v)),
        ),
        _gap(),
        KycAddressField(
          key: const Key('kyc_company_address'),
          label: _fieldLabel(
            context,
            KYCField.KYC_FIELD_BUSINESS_COMPANY_ADDRESS,
          ),
          townCode: f.companyTownCode,
          detail: f.companyAddressDetail,
          error: c.fieldError(KYCField.KYC_FIELD_BUSINESS_COMPANY_ADDRESS),
          onTap: actions.editCompanyAddress,
        ),
        _gap(),
        KycTextField(
          key: const Key('kyc_company_phone'),
          label: _fieldLabel(
            context,
            KYCField.KYC_FIELD_BUSINESS_COMPANY_PHONE,
          ),
          value: f.companyPhone,
          hint: '例如 02-12345678',
          keyboardType: TextInputType.phone,
          inputFormatters: [
            FilteringTextInputFormatter.allow(RegExp(r'[0-9\-#]')),
          ],
          error: c.fieldError(KYCField.KYC_FIELD_BUSINESS_COMPANY_PHONE),
          onChanged: (v) => c.updateForm((f) => f.copyWith(companyPhone: v)),
        ),
        _gap(),
        KycTextField(
          key: const Key('kyc_product_type'),
          label: _fieldLabel(context, KYCField.KYC_FIELD_PRODUCT_TYPE),
          value: f.productType,
          hint: '例如：水果 / 蔬菜 / 海鮮 / 加工食品',
          error: c.fieldError(KYCField.KYC_FIELD_PRODUCT_TYPE),
          onChanged: (v) => c.updateForm((f) => f.copyWith(productType: v)),
        ),
        _nextBar(c, actions),
      ],
    );
  }
}

class _LookupNotice extends StatelessWidget {
  const _LookupNotice(this.lookup);

  final KycLookup lookup;

  @override
  Widget build(BuildContext context) {
    final (text, color) = switch (lookup) {
      KycLookupRunning() => ('查詢營業登記中...', AppColors.textSecondary),
      KycLookupFound(:final name, :final registerDate) => (
        '查得 $name，登記日 ${rocDateOfIso(registerDate).isEmpty ? '—' : rocDateOfIso(registerDate)}',
        AppColors.textSecondary,
      ),
      KycLookupNotFound() => (kycTaxIdNotFoundNotice, AppColors.dangerStrong),
      KycLookupFailed(:final message) => (message, AppColors.dangerStrong),
      KycLookupIdle() => ('', AppColors.textSecondary),
    };
    if (text.isEmpty) return const SizedBox.shrink();
    return Padding(
      padding: EdgeInsets.only(top: 8.h),
      child: Text(
        text,
        key: const Key('kyc_tax_id_notice'),
        style: TextStyle(fontSize: 12.sp, color: color),
      ),
    );
  }
}

/* ===== 商家營運資訊（聯絡人、店家地址） ===== */

class _OtherStep extends StatelessWidget {
  const _OtherStep(this.c, this.actions);

  final KycWizard c;
  final KycStepActions actions;

  @override
  Widget build(BuildContext context) {
    final s = c.store;
    final errors = c.contactErrors;
    return _card(
      title: '商家營運資訊',
      subtitle: '店家地址與聯絡資料會同步到你的商店設定',
      icon: HkIconName.info,
      children: [
        KycTextField(
          key: const Key('kyc_contact_name'),
          label:
              StoreField.STORE_FIELD_CONTACT_NAME.uiText(context.uiLocale) ??
              commonUnknown(context.uiLocale),
          value: s.contactName,
          hint: '最多 50 字，會寫入商店設定',
          error: errors.name,
          onChanged: (v) => c.updateStore((s) => s.copyWith(contactName: v)),
        ),
        _gap(),
        KycTextField(
          key: const Key('kyc_contact_mobile'),
          label: '聯絡人手機',
          value: s.contactMobile,
          hint: '09 開頭的 10 碼手機號碼',
          keyboardType: TextInputType.phone,
          inputFormatters: [
            FilteringTextInputFormatter.digitsOnly,
            LengthLimitingTextInputFormatter(10),
          ],
          error: errors.mobile,
          onChanged: (v) => c.updateStore((s) => s.copyWith(contactMobile: v)),
        ),
        _gap(),
        KycTextField(
          key: const Key('kyc_support_email'),
          label: '客服電子信箱',
          value: s.supportEmail,
          hint: '請輸入客服電子信箱',
          keyboardType: TextInputType.emailAddress,
          inputFormatters: [
            FilteringTextInputFormatter.allow(RegExp(r'[a-zA-Z0-9@._\-+]')),
          ],
          error: errors.email,
          onChanged: (v) => c.updateStore((s) => s.copyWith(supportEmail: v)),
        ),
        _gap(),
        KycAddressField(
          key: const Key('kyc_store_address'),
          label: '店家地址',
          townCode: s.townCode,
          detail: s.addressDetail,
          error: c.addressInputError,
          onTap: actions.editStoreAddress,
        ),
        _nextBar(c, actions),
      ],
    );
  }
}

/* ===== 公司文件（企業） ===== */

class _DocumentsStep extends StatelessWidget {
  const _DocumentsStep(this.c, this.actions);

  final KycWizard c;
  final KycStepActions actions;

  @override
  Widget build(BuildContext context) {
    final docs = c.docs.companyDocuments;
    final canAdd = docs.length < KycDocs.maxCompanyDocuments;
    return _card(
      title: '上傳公司文件',
      subtitle: '可提供工廠登記證、產品責任保險證明、食品檢驗報告、授權書或代理證明（選填）',
      icon: HkIconName.document,
      children: [
        if (c.companyDocumentsIssue case final message?) ...[
          Text(
            message,
            key: const Key('kyc_company_documents_issue'),
            style: TextStyle(fontSize: 12.sp, color: AppColors.dangerStrong),
          ),
          SizedBox(height: 8.h),
        ],
        Wrap(
          spacing: 10.w,
          runSpacing: 8.h,
          children: [
            for (var i = 0; i < docs.length; i++)
              SizedBox(
                width: 140.w,
                child: KycUploadBox(
                  key: Key('kyc_company_document_$i'),
                  label: '文件 ${i + 1}',
                  doc: docs[i],
                  error: c.companyDocumentIssue(docs[i]),
                  height: 80.h,
                  onTap: () {},
                  onRemove: () => c.removeCompanyDocument(i),
                ),
              ),
            if (canAdd)
              SizedBox(
                width: 140.w,
                child: KycUploadBox(
                  key: const Key('kyc_company_document_add'),
                  label: '新增文件',
                  doc: null,
                  height: 80.h,
                  uploading: c.isUploading('company_document'),
                  onTap: actions.addCompanyDocument,
                ),
              ),
          ],
        ),
        KycButtonBar(
          label: '下一步',
          busy: c.saving,
          onTap: c.busy ? null : actions.next,
        ),
      ],
    );
  }
}

/* ===== 確認送出 ===== */

/// 收款聯絡信箱欄位（D23 修訂）：金流商用這個信箱聯絡賣家。帳號有 provider 驗證過的信箱時唯讀、不給輸入；沒有任何信箱或
/// 只有自填的信箱時才可以輸入（寫回帳號）。帳號的信箱還沒讀到時不顯示；讀取失敗顯示重試。
class KycContactEmailField extends StatelessWidget {
  const KycContactEmailField({
    super.key,
    required this.contact,
    required this.value,
    required this.error,
    required this.onChanged,
    required this.onRetry,
  });

  final AsyncValue<AccountContactEmail?>? contact;
  final String value;
  final String? error;
  final ValueChanged<String> onChanged;
  final VoidCallback onRetry;

  static const label = '金流商聯絡信箱';
  static const hint = '金流商會用這個信箱聯絡你收款事宜';

  @override
  Widget build(BuildContext context) {
    final read = contact;
    if (read == null || (read.isLoading && !read.hasValue)) {
      return const SizedBox.shrink();
    }
    if (!read.hasValue) {
      return Row(
        key: const Key('kyc_contact_email_error'),
        children: [
          const Expanded(child: Text(label)),
          TextButton(onPressed: onRetry, child: const HkButtonLabel('重新載入')),
        ],
      );
    }
    final account = read.requireValue;
    if (!contactEmailEditable(account)) {
      return Column(
        key: const Key('kyc_contact_email_verified'),
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          KycInfoRow(label, account!.email),
          Text(
            hint,
            style: TextStyle(fontSize: 12.sp, color: AppColors.textMuted),
          ),
        ],
      );
    }
    return KycTextField(
      key: const Key('kyc_contact_email'),
      label: label,
      value: value,
      hint: '例如 name@example.com',
      helper: hint,
      keyboardType: TextInputType.emailAddress,
      error: error,
      onChanged: onChanged,
    );
  }
}

class _ConfirmStep extends StatelessWidget {
  const _ConfirmStep({
    required this.controller,
    required this.actions,
    required this.agreements,
    required this.privacyChecked,
    required this.onPrivacyChanged,
  });

  final KycWizard controller;
  final KycStepActions actions;
  final AsyncValue<KycAgreements> agreements;
  final bool privacyChecked;
  final ValueChanged<bool> onPrivacyChanged;

  @override
  Widget build(BuildContext context) {
    final c = controller;
    final f = c.form;
    final s = c.store;
    final business = c.subject == KycSubject.business;
    final missing = c.missingItems;
    final loaded = agreements.value;
    final ready =
        privacyChecked &&
        loaded != null &&
        missingItemsOf(
          c.application?.missingFields ?? const [],
          context.uiLocale,
        ).isEmpty &&
        c.contactEmailBlocker == null &&
        !c.hasUnsavedDraft &&
        c.addressInputError == null &&
        !c.busy;
    return _card(
      title: '確認送出',
      subtitle: '最後確認資料正確；送出後進入平台審核',
      icon: HkIconName.check,
      children: [
        if (business) ...[
          KycInfoRow(
            _fieldLabel(context, KYCField.KYC_FIELD_BUSINESS_COMPANY_NAME),
            f.companyName,
          ),
          KycInfoRow(
            _fieldLabel(context, KYCField.KYC_FIELD_BUSINESS_TAX_ID),
            f.taxId,
          ),
          KycInfoRow(
            _fieldLabel(context, KYCField.KYC_FIELD_PRODUCT_TYPE),
            f.productType,
          ),
          _gap(8),
          const Divider(color: AppColors.dividerLight),
        ],
        KycInfoRow(
          StoreField.STORE_FIELD_CONTACT_NAME.uiText(context.uiLocale) ??
              commonUnknown(context.uiLocale),
          s.contactName,
        ),
        KycInfoRow('店家地址', _storeAddressText(s)),
        _gap(8),
        KycContactEmailField(
          contact: c.contactEmailRead,
          value: c.contactEmailValue,
          error: c.contactEmailError,
          onChanged: c.updateContactEmail,
          onRetry: () => unawaited(c.reloadContactEmail()),
        ),
        if (missing.isNotEmpty) ...[
          _gap(8),
          _MissingList(missing: missing, onJump: actions.jumpTo),
        ],
        _gap(20),
        AgreementCheckbox(
          checkboxKey: const Key('kyc_privacy_checkbox'),
          before: '我已閱讀',
          linkText: '賣家身分認證個人資料蒐集告知事項',
          after: '',
          url: loaded?.privacyNotice.url,
          loadFailed: agreements.hasError,
          checked: privacyChecked,
          enabled: !c.submitting,
          onChanged: onPrivacyChanged,
          onRetry: actions.retryAgreements,
        ),
        _gap(12),
        Align(
          alignment: Alignment.centerLeft,
          child: Text(
            '身分與銀行帳號會在審核通過後，由金流商收集，平台不經手。',
            key: const Key('kyc_provider_collects_notice'),
            style: TextStyle(fontSize: 12.sp, color: AppColors.textMuted),
          ),
        ),
        KycButtonBar(
          label: '送出認證',
          busy: c.submitting,
          onTap: ready ? actions.submit : null,
        ),
      ],
    );
  }
}

String _storeAddressText(StoreContactDraft s) =>
    twaddr.formatAddress(s.townCode, s.addressDetail.trim());

class _MissingList extends StatelessWidget {
  const _MissingList({required this.missing, required this.onJump});

  final List<KycMissingItem> missing;
  final void Function(KycSection section) onJump;

  @override
  Widget build(BuildContext context) {
    return Container(
      key: const Key('kyc_missing_list'),
      padding: EdgeInsets.all(12.w),
      decoration: BoxDecoration(
        color: AppColors.dangerSoft,
        borderRadius: BorderRadius.circular(12.r),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            '尚需補齊 ${missing.length} 項',
            style: TextStyle(
              fontSize: 14.sp,
              fontWeight: FontWeight.w700,
              color: AppColors.danger,
            ),
          ),
          for (final item in missing)
            InkWell(
              onTap: () => onJump(item.section),
              child: Padding(
                padding: EdgeInsets.symmetric(vertical: 6.h),
                child: Row(
                  children: [
                    HkIcon(
                      HkIconName.warning,
                      size: 14,
                      color: AppIconColors.danger,
                    ),
                    SizedBox(width: 6.w),
                    Expanded(
                      child: Text(
                        '${item.section.label}：${item.label}',
                        style: TextStyle(
                          fontSize: 14.sp,
                          color: AppColors.textEmphasis,
                        ),
                      ),
                    ),
                  ],
                ),
              ),
            ),
        ],
      ),
    );
  }
}
