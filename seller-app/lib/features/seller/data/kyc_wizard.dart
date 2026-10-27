import 'dart:async';

import 'package:hawkr_api/dates.dart';
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/error_reason.pbenum.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/submit.dart';
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:hawkr_ui_text/hawkr_ui_text.dart' show UiLocale;
import 'package:flutter/foundation.dart';
import 'package:freezed_annotation/freezed_annotation.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/l10n/ui_locale.dart';
import '../../../core/networking/connect_provider.dart';
import '../../../shared/utils/hk_log.dart';
import '../../media/data/private_document_uploader.dart';
import 'current_kyc.dart';
import 'current_store.dart';
import 'kyc_fields.dart';
import 'kyc_form.dart';
import 'kyc_store_contact.dart';
import 'seller_agreements_repository.dart';
import 'seller_onboarding.dart' show agreementOutdatedMessage;
import 'store_editor.dart';
import 'version_conflict.dart';

part 'kyc_wizard.freezed.dart';
part 'kyc_wizard.g.dart';

const kycCancelledNotice = '認證申請已取消，請重新開始。';
const kycDraftSaveIncompleteNotice = '認證資料尚未儲存完成，請稍後再試。';
const kycContactFormatNotice = '聯絡資料格式不正確，請檢查後再試。';
const kycTaxIdNotFoundNotice = 'Hawkr 系統查無此統編資料，請手動填入公司資訊。';
const kycCreateDraftFailedNotice = '建立認證草稿失敗';
const kycUploadFailedNotice = '上傳失敗，請稍後再試';

/// 送出認證的結果。
enum KycSubmitOutcome {
  /// 申請已送出。
  submitted,

  /// 同意的版本已不是現行版本：已重讀條款，請賣家重新勾選。
  agreementOutdated,

  /// 沒有送出，原因在 [KycWizard.error]。
  failed,
}

/// 統編查詢的結果（顯示在統編欄位下面）。
sealed class KycLookup {
  const KycLookup();
}

final class KycLookupIdle extends KycLookup {
  const KycLookupIdle();
}

final class KycLookupRunning extends KycLookup {
  const KycLookupRunning();
}

/// 查到登記資料，已帶入公司名稱、設立日期與地址。
final class KycLookupFound extends KycLookup {
  const KycLookupFound({required this.name, required this.registerDate});

  final String name;

  /// `YYYY-MM-DD`；來源缺漏時是空字串。
  final String registerDate;
}

/// 查無這個統編（不是錯誤）。
final class KycLookupNotFound extends KycLookup {
  const KycLookupNotFound();
}

/// 查詢失敗（資料來源暫時無法使用、查詢太頻繁等）：退回手動填寫。
final class KycLookupFailed extends KycLookup {
  const KycLookupFailed(this.message);

  final String message;
}

/// 伺服器對某個欄位的錯誤；欄位的值改過（和 [value] 不同）就不再顯示。
final class KycFieldError {
  const KycFieldError({required this.message, required this.value});

  final String message;
  final String value;
}

/// wizard 的輸入狀態。申請本身（整份 `KYCApplication`、etag、缺項、退回）不在這裡：它的唯一來源是 [currentKycProvider]。
@freezed
abstract class KycWizardState with _$KycWizardState {
  const factory KycWizardState({
    /// 已經從申請（或沒有申請）載入。
    @Default(false) bool hydrated,

    /// 申請的身分；還沒有申請也還沒選時是 null（顯示選擇畫面）。
    KycSubject? subject,
    @Default(KycForm()) KycForm form,
    @Default(KycForm()) KycForm baseline,
    @Default(KycDocs()) KycDocs docs,
    @Default(KycDocs()) KycDocs baselineDocs,
    @Default(StoreContactDraft()) StoreContactDraft store,
    @Default(StoreContactDraft()) StoreContactDraft storeBaseline,
    @Default(StoreContactErrors()) StoreContactErrors storeServerErrors,
    StoreContactDraft? storeServerErrorsFor,

    /// 最近一次動作失敗或提醒的文字。
    String? error,

    /// 載入後要停在哪一步；沒有申請或沒有缺項時是 null（從頭開始）。
    KycStep? resumeStep,
    @Default(KycLookupIdle()) KycLookup lookup,
    @Default({}) Map<KYCField, KycFieldError> fieldErrors,
    @Default({}) Set<String> uploading,
    @Default(false) bool saving,
    @Default(false) bool storeSaving,
    @Default(false) bool submitting,
    @Default(false) bool discarding,
    @Default(false) bool creating,
  }) = _KycWizardState;
}

/// 認證 wizard 的動作與讀取：草稿、證件、統編查詢，以及寫進店家的聯絡資料與地址。
///
/// 表單是整份字串狀態，伺服器版本是 baseline；每次存檔只送和 baseline 不同的欄位，帶申請目前的 etag。版本衝突
/// （`VERSION_CONFLICT`）時重讀最新版本、換上新的 etag，保留使用者的輸入。聯絡資料與地址在送出 `SubmitKYC` 之前先寫進店家。
/// 寫入（SaveKYCDraft、SubmitKYC、CancelKYC）回的申請直接寫進 [currentKycProvider] 的快取，進度卡與閘門立即看到新狀態。
@riverpod
class KycWizard extends _$KycWizard {
  int _lookupSeq = 0;
  Future<void> _saveChain = Future.value();

  @override
  KycWizardState build() => const KycWizardState();

  void _set(KycWizardState Function(KycWizardState s) change) {
    if (ref.mounted) state = change(state);
  }

  /* ===== 讀取 ===== */

  bool get hydrated => state.hydrated;

  /// 申請的唯一來源：共用的 [currentKycProvider]。
  KYCApplication? get application =>
      ref.read(currentKycProvider)?.value?.application;

  KycSubject? get subject => state.subject;
  KycForm get form => state.form;
  KycDocs get docs => state.docs;
  StoreContactDraft get store => state.store;
  String? get error => state.error;
  KycStep? get resumeStep => state.resumeStep;
  KycLookup get lookup => state.lookup;
  bool get saving => state.saving;
  bool get storeSaving => state.storeSaving;
  bool get submitting => state.submitting;
  bool get discarding => state.discarding;
  bool get creating => state.creating;
  bool isUploading(String key) => state.uploading.contains(key);

  String get _etag => application?.etag ?? '';

  /// 有任何一個會改動資料的動作正在進行。
  bool get busy =>
      state.saving ||
      state.storeSaving ||
      state.submitting ||
      state.discarding ||
      state.creating ||
      state.uploading.isNotEmpty;

  bool get editable => kycEditable(application);
  bool get identityLocked {
    final app = application;
    return app != null && app.hasFlow() && app.flow.identityLocked;
  }

  bool get canDiscard => kycCanDiscard(application);

  /// 伺服器算的、賣家還沒處理的退回問題（欄位的值改過、指到的文件移除或替換就算處理，client 不推導）。
  List<KYCIssue> get outstandingIssues =>
      application?.outstandingIssues ?? const <KYCIssue>[];

  /// 退回的總理由：伺服器只在最近一次審核紀錄是退回、賣家還沒重送（核准、取消也算）時回 `last_rejection`，有就顯示，
  /// 處理完每一項、還沒重送時仍在；逐項標記看 [outstandingIssues]。
  KYCRejection? get rejection {
    final app = application;
    return app != null && app.hasLastRejection() ? app.lastRejection : null;
  }

  String get maskedId => maskedIdOf(application);
  String get maskedBankAccount => application?.bankAccountMasked ?? '';

  /// 伺服器的缺項（申請欄位加店家欄位）。
  List<KycMissingItem> get missingItems => [
    ...missingItemsOf(application?.missingFields ?? const [], _l),
    ...storeMissingItemsOf(application?.missingStoreFields ?? const [], _l),
  ];

  UiLocale get _l => ref.read(uiLocaleProvider);

  ({String? idError, String? accountError}) get inputErrors =>
      kycInputErrors(state.form);

  StoreContactErrors get contactErrors {
    final local = storeContactErrors(state.store);
    final server = state.storeServerErrorsFor == state.store
        ? state.storeServerErrors
        : const StoreContactErrors();
    return StoreContactErrors(
      name: local.name ?? server.name,
      mobile: local.mobile ?? server.mobile,
      email: local.email ?? server.email,
    );
  }

  String? get addressInputError => storeAddressInputError(state.store);

  /// 伺服器對 [field] 的欄位錯誤；欄位的值改過就不再顯示。
  /// 沒有（或已改過）伺服器的欄位錯誤時，顯示待處理的退回問題對這個欄位的說明。
  String? fieldError(KYCField field) {
    final entry = state.fieldErrors[_canonical(field)];
    if (entry != null && _valueIn(state.form, field) == entry.value) {
      return entry.message;
    }
    return _joined([
      for (final issue in outstandingIssues)
        if (_canonical(issue.field_1) == _canonical(field)) issue.message,
    ]);
  }

  /// 單張證件（身分證正反面、存摺封面）的待處理退回問題；單張的位置只有一張，所以不論問題有沒有指到那一張都算。
  String? documentIssue(KycDocSlot slot) {
    final field = switch (slot) {
      KycDocSlot.idCardFront => KYCField.KYC_FIELD_ID_CARD_FRONT,
      KycDocSlot.idCardBack => KYCField.KYC_FIELD_ID_CARD_BACK,
      KycDocSlot.bankBook => KYCField.KYC_FIELD_BANK_BOOK,
    };
    return _joined([
      for (final issue in outstandingIssues)
        if (issue.field_1 == field) issue.message,
    ]);
  }

  /// 指到某一張公司文件（以 asset ID）的待處理退回問題。
  String? companyDocumentIssue(KycDoc doc) {
    if (doc.assetId.isEmpty) return null;
    return _joined([
      for (final issue in outstandingIssues)
        if (issue.field_1 == KYCField.KYC_FIELD_BUSINESS_COMPANY_DOCUMENTS &&
            issue.documentAssetId == doc.assetId)
          issue.message,
    ]);
  }

  /// 公司文件沒有指到特定一張的待處理退回問題。
  String? get companyDocumentsIssue => _joined([
    for (final issue in outstandingIssues)
      if (issue.field_1 == KYCField.KYC_FIELD_BUSINESS_COMPANY_DOCUMENTS &&
          issue.documentAssetId.isEmpty)
        issue.message,
  ]);

  static String? _joined(List<String> messages) =>
      messages.isEmpty ? null : messages.join('；');

  static KYCField _canonical(KYCField field) =>
      field == KYCField.KYC_FIELD_BUSINESS_REP_ID_CARD_NO
      ? KYCField.KYC_FIELD_INDIVIDUAL_ID_CARD_NO
      : field;

  static String? _valueIn(KycForm form, KYCField field) => switch (_canonical(
    field,
  )) {
    KYCField.KYC_FIELD_REAL_NAME => form.realName,
    KYCField.KYC_FIELD_BIRTH_DATE => form.birthDate,
    KYCField.KYC_FIELD_ID_CARD_ISSUE_DATE => form.idCardIssueDate,
    KYCField.KYC_FIELD_ID_CARD_ISSUE_TYPE => form.idCardIssueType,
    KYCField.KYC_FIELD_ID_CARD_ISSUE_LOCATION => form.idCardIssueLocation,
    KYCField.KYC_FIELD_ENGLISH_NAME => form.englishName,
    KYCField.KYC_FIELD_BANK_NAME => form.bankName,
    KYCField.KYC_FIELD_BANK_CODE => form.bankCode,
    KYCField.KYC_FIELD_BANK_ACCOUNT => form.bankAccount,
    KYCField.KYC_FIELD_PRODUCT_TYPE => form.productType,
    KYCField.KYC_FIELD_INDIVIDUAL_ID_CARD_NO => form.idCardNo,
    KYCField.KYC_FIELD_BUSINESS_COMPANY_NAME => form.companyName,
    KYCField.KYC_FIELD_BUSINESS_TAX_ID => form.taxId,
    KYCField.KYC_FIELD_BUSINESS_COMPANY_REGISTER_DATE =>
      form.companyRegisterDate,
    KYCField.KYC_FIELD_BUSINESS_COMPANY_PHONE => form.companyPhone,
    KYCField.KYC_FIELD_BUSINESS_COMPANY_ADDRESS => form.companyAddressDetail,
    _ => null,
  };

  Store? get _currentStore => ref.read(myStoreSnapshotProvider)?.store;

  /* ===== 載入 ===== */

  /// 以共用快取裡的申請（沒有申請就是從頭開始）與店家載入表單。新申請才套用店家的預設值（公司電話取商店電話）；
  /// 既有的申請以申請為準。[requested] 是呼叫端帶來的身分提示，只在還沒有申請時生效。申請還沒讀到時不動。
  void hydrate({KycSubject? requested}) {
    final kyc = ref.read(currentKycProvider)?.value;
    if (kyc == null) return;
    final app = kyc.application;
    final subject = kycSubjectOf(app) ?? requested;
    final store = _currentStore;
    final draft = storeContactDraftOf(store);
    if (app == null) {
      state = KycWizardState(
        hydrated: true,
        subject: subject,
        form: _defaultsFrom(store),
        store: draft,
        storeBaseline: draft,
      );
      return;
    }
    final form = kycFormOf(app);
    final docs = kycDocsOf(app);
    state = KycWizardState(
      hydrated: true,
      subject: subject,
      form: form,
      baseline: form,
      docs: docs,
      baselineDocs: docs,
      store: draft,
      storeBaseline: draft,
      resumeStep: subject == null
          ? null
          : resumeStepOf(subject, app.missingFields, app.missingStoreFields) ??
                (kycEditable(app) ? KycStep.confirm : null),
    );
  }

  KycForm _defaultsFrom(Store? store) {
    if (store == null) return const KycForm();
    return KycForm(companyPhone: store.contactPhone.trim());
  }

  /* ===== 輸入 ===== */

  void updateForm(KycForm Function(KycForm current) change) {
    _set((s) => s.copyWith(form: change(s.form)));
  }

  void updateStore(
    StoreContactDraft Function(StoreContactDraft current) change,
  ) {
    _set((s) => s.copyWith(store: change(s.store)));
  }

  /// 選了身分（還沒有申請時）。
  void chooseSubject(KycSubject subject) {
    if (application != null) return;
    _set((s) => s.copyWith(subject: subject));
  }

  void dismissError() {
    if (state.error == null) return;
    _set((s) => s.copyWith(error: null));
  }

  /* ===== 存檔 ===== */

  SaveKYCDraftRequest? _draftRequest(
    KycForm form,
    KycDocs docs, {
    bool creating = false,
  }) {
    final subject = state.subject;
    if (subject == null) return null;
    return buildSaveKycDraftRequest(
      subject: subject,
      form: form,
      docs: docs,
      baseline: creating ? const KycForm() : state.baseline,
      baselineDocs: creating ? const KycDocs() : state.baselineDocs,
      etag: creating || _etag.isEmpty ? null : _etag,
    );
  }

  /// 目前的輸入還有沒存進伺服器的申請欄位。
  bool get hasUnsavedDraft =>
      application != null &&
      editable &&
      _draftRequest(state.form, state.docs) != null;

  /// 建立草稿（不帶 etag 的 `SaveKYCDraft`）。成功或版本衝突（別處已經建立）後 [application] 有值。
  Future<bool> startDraft(KycSubject subject) async {
    if (application != null || state.creating) return application != null;
    _set((s) => s.copyWith(subject: subject, creating: true, error: null));
    try {
      final sent = state.form;
      final request = _draftRequest(sent, const KycDocs(), creating: true);
      if (request == null) throw StateError('無法建立認證草稿');
      final response = await ref
          .read(sellerClientProvider)
          .saveKYCDraft(request);
      if (!ref.mounted) return false;
      _adoptSaved(
        sent,
        const KycDocs(),
        response.application,
        request.updateMask.paths,
      );
      return true;
    } catch (error) {
      HKLog.logW('[KycWizard] create draft failed: $error');
      if (!ref.mounted) return false;
      switch (submitError(
        error,
        defaults: ref.read(defaultErrorTextProvider),
      )) {
        case SubmitConflict():
          await _adoptExistingApplication();
        case SubmitFields():
          _set((s) => s.copyWith(error: kycCreateDraftFailedNotice));
        case SubmitMessage(:final message):
          _set((s) => s.copyWith(error: message));
      }
      return false;
    } finally {
      _set((s) => s.copyWith(creating: false));
    }
  }

  /// 寫入回的申請成為共用的申請。輸入中的表單不動（不把伺服器正規化後的值寫回，免得打字打到一半被改掉）；請求送出的欄位的
  /// baseline 變成送出的值，所以存完之後「有沒有改動」只看輸入和送出的值。
  void _adoptSaved(
    KycForm sent,
    KycDocs sentDocs,
    KYCApplication saved,
    Iterable<String> sentPaths,
  ) {
    ref.read(kycReaderProvider.notifier).adopt(saved);
    _set((s) {
      final baselineDocs = kycDocsOf(saved);
      return s.copyWith(
        baseline: baselineAfterSave(s.baseline, sent, sentPaths),
        docs: adoptSavedDocs(s.docs, sentDocs, baselineDocs),
        baselineDocs: baselineDocs,
        fieldErrors: const {},
      );
    });
  }

  /// 建立草稿時別處已經建立了申請：原樣採用那一份（店家預設值只屬於全新的申請，不能蓋掉它），回到它的續填位置。
  Future<void> _adoptExistingApplication() async {
    final latest = await _reload();
    if (!ref.mounted) return;
    if (latest == null) {
      _set((s) => s.copyWith(error: versionReloadFailedNotice));
      return;
    }
    final app = latest.application;
    if (app == null) {
      _resetToNoApplication(error: kycCancelledNotice);
      return;
    }
    final subject = kycSubjectOf(app);
    final form = kycFormOf(app);
    final docs = kycDocsOf(app);
    _set(
      (s) => s.copyWith(
        subject: subject ?? s.subject,
        form: form,
        baseline: form,
        docs: docs,
        baselineDocs: docs,
        fieldErrors: const {},
        resumeStep: subject == null
            ? null
            : resumeStepOf(
                    subject,
                    app.missingFields,
                    app.missingStoreFields,
                  ) ??
                  (kycEditable(app) ? KycStep.confirm : null),
      ),
    );
  }

  /// 把目前的輸入存進伺服器（排在前一個存檔之後）。沒有要存的欄位也算成功；失敗（含版本衝突）回 false，原因在 [error]，輸入保留。
  Future<bool> save() {
    final next = _saveChain.then((_) => _saveNow());
    _saveChain = next.then((_) {}, onError: (_) {});
    return next;
  }

  Future<bool> _saveNow() async {
    if (!ref.mounted || application == null || !editable) return true;
    final sent = state.form;
    final sentDocs = state.docs;
    final request = _draftRequest(sent, sentDocs);
    if (request == null) return true;
    _set((s) => s.copyWith(saving: true));
    try {
      final response = await ref
          .read(sellerClientProvider)
          .saveKYCDraft(request);
      if (!ref.mounted) return false;
      _adoptSaved(
        sent,
        sentDocs,
        response.application,
        request.updateMask.paths,
      );
      _set((s) => s.copyWith(error: null));
      return true;
    } catch (error) {
      HKLog.logW('[KycWizard] SaveKYCDraft failed: $error');
      if (!ref.mounted) return false;
      switch (submitError(
        error,
        defaults: ref.read(defaultErrorTextProvider),
      )) {
        case SubmitConflict():
          await _resolveConflict();
        case SubmitFields(:final fields, :final message):
          _recordDraftFieldErrors(fields, sent, message);
        case SubmitMessage(:final message):
          _set((s) => s.copyWith(error: message));
      }
      return false;
    } finally {
      _set((s) => s.copyWith(saving: false));
    }
  }

  void _recordDraftFieldErrors(
    Map<String, String> fields,
    KycForm sent,
    String? serverMessage,
  ) {
    final errors = <KYCField, KycFieldError>{...state.fieldErrors};
    final labels = <String>[];
    final documentNotices = <String>[];
    for (final MapEntry(key: path, value: message) in fields.entries) {
      final applicationPath = path.replaceFirst('application.', '');
      final field = kycFieldOfPath(applicationPath);
      if (field == null) continue;
      // 證件沒有行內的提示位置：把伺服器對那一張的訊息放進提示，賣家刪掉那一張重新上傳。
      if (kycDocumentNotice(applicationPath, field, message, _l)
          case final notice?) {
        if (!documentNotices.contains(notice)) documentNotices.add(notice);
        continue;
      }
      final value = _valueIn(sent, field);
      if (value != null) {
        errors[_canonical(field)] = KycFieldError(
          message: message,
          value: value,
        );
      }
      final label = kycFieldLabel(field, _l);
      if (label != null && !labels.contains(label)) labels.add(label);
    }
    _set(
      (s) => s.copyWith(
        fieldErrors: errors,
        error: labels.isEmpty && documentNotices.isEmpty
            ? (serverMessage ?? '有欄位格式不正確，請檢查後再試。')
            : [
                if (labels.isNotEmpty) '下列欄位格式不正確：${labels.join('、')}',
                ...documentNotices,
              ].join('；'),
      ),
    );
  }

  /// 重讀申請（共用快取一併更新）；失敗回 null。
  Future<MyKyc?> _reload() async {
    try {
      return await ref.read(kycReaderProvider.notifier).reload();
    } catch (error) {
      HKLog.logW('[KycWizard] reload failed: $error');
      return null;
    }
  }

  /// 版本衝突：重讀最新版本、換上新的 etag，保留使用者改過的欄位。重讀失敗時只顯示失敗。申請已被取消時回到沒有申請的狀態，
  /// 表單與證件都重設（和捨棄草稿相同）。
  Future<void> _resolveConflict() async {
    final latest = await _reload();
    if (!ref.mounted) return;
    if (latest == null) {
      _set((s) => s.copyWith(error: versionReloadFailedNotice));
      return;
    }
    final app = latest.application;
    if (app == null) {
      _resetToNoApplication(error: kycCancelledNotice);
      return;
    }
    final latestForm = kycFormOf(app);
    _set(
      (s) => s.copyWith(
        subject: kycSubjectOf(app) ?? s.subject,
        baseline: latestForm.copyWith(
          idCardNo: s.baseline.idCardNo,
          bankAccount: s.baseline.bankAccount,
        ),
        baselineDocs: kycDocsOf(app),
        form: mergeLatestForm(s.form, s.baseline, latestForm),
        docs: mergeLatestDocs(s.docs, s.baselineDocs, kycDocsOf(app)),
        error: versionConflictNotice('申請資料'),
      ),
    );
  }

  void _resetToNoApplication({String? error}) {
    _set(
      (s) => KycWizardState(
        hydrated: true,
        form: _defaultsFrom(_currentStore),
        store: s.store,
        storeBaseline: s.storeBaseline,
        error: error,
      ),
    );
  }

  /* ===== 證件 ===== */

  /// 上傳一張單張位置的證件，上傳完成後存進申請。
  Future<bool> uploadDocument(
    KycDocSlot slot,
    Uint8List bytes, {
    required String docType,
    required String filename,
  }) async {
    final key = slot.path;
    _set((s) => s.copyWith(uploading: {...s.uploading, key}, error: null));
    try {
      final token = await ref.read(privateDocumentUploaderProvider)(
        bytes,
        docType,
        filename,
      );
      _set(
        (s) => s.copyWith(
          docs: s.docs.withSlot(
            slot,
            KycDoc(stagedToken: token, preview: bytes),
          ),
        ),
      );
    } catch (error) {
      HKLog.logW('[KycWizard] upload $key failed: $error');
      _set((s) => s.copyWith(error: kycUploadFailedNotice));
      return false;
    } finally {
      _set((s) => s.copyWith(uploading: {...s.uploading}..remove(key)));
    }
    return save();
  }

  void clearDocument(KycDocSlot slot) {
    _set((s) => s.copyWith(docs: s.docs.withSlot(slot, null)));
  }

  /// 新增一張企業文件（最多 [KycDocs.maxCompanyDocuments] 張），上傳完成後存進申請。
  Future<bool> addCompanyDocument(
    Uint8List bytes, {
    required String filename,
  }) async {
    if (state.docs.companyDocuments.length >= KycDocs.maxCompanyDocuments) {
      return false;
    }
    const key = 'company_document';
    _set((s) => s.copyWith(uploading: {...s.uploading, key}, error: null));
    try {
      final token = await ref.read(privateDocumentUploaderProvider)(
        bytes,
        'document',
        filename,
      );
      _set(
        (s) => s.copyWith(
          docs: s.docs.withCompanyDocuments([
            ...s.docs.companyDocuments,
            KycDoc(stagedToken: token, preview: bytes),
          ]),
        ),
      );
    } catch (error) {
      HKLog.logW('[KycWizard] upload company document failed: $error');
      _set((s) => s.copyWith(error: kycUploadFailedNotice));
      return false;
    } finally {
      _set((s) => s.copyWith(uploading: {...s.uploading}..remove(key)));
    }
    return save();
  }

  void removeCompanyDocument(int index) {
    final docs = state.docs.companyDocuments;
    if (index < 0 || index >= docs.length) return;
    _set(
      (s) => s.copyWith(
        docs: s.docs.withCompanyDocuments([
          for (var i = 0; i < docs.length; i++)
            if (i != index) docs[i],
        ]),
      ),
    );
  }

  /* ===== 統編查詢 ===== */

  /// 以 8 碼統編查登記資料並帶入公司名稱、設立日期與地址（不帶組織型態）。查無是 [KycLookupNotFound]，不是錯誤；查詢失敗退回手動填寫。
  Future<void> lookupTaxId(String taxId) async {
    final id = taxId.trim();
    if (!RegExp(r'^\d{8}$').hasMatch(id)) return;
    final seq = ++_lookupSeq;
    _set((s) => s.copyWith(lookup: const KycLookupRunning()));
    try {
      final response = await ref
          .read(sellerClientProvider)
          .lookupBusinessRegistration(
            LookupBusinessRegistrationRequest(taxId: id),
          );
      if (seq != _lookupSeq || !ref.mounted) return;
      if (!response.hasRegistration()) {
        _set((s) => s.copyWith(lookup: const KycLookupNotFound()));
        return;
      }
      final registration = response.registration;
      final registerDate = formatDate(
        registration.hasSetupDate() ? registration.setupDate : null,
      );
      final postal = registration.hasPostalAddress()
          ? registration.postalAddress
          : null;
      final hasTown = postal != null && postal.townCode.isNotEmpty;
      final detail = hasTown ? postal.addressDetail : registration.address;
      _set(
        (s) => s.copyWith(
          form: s.form.copyWith(
            companyName: registration.name.isNotEmpty
                ? registration.name
                : null,
            companyRegisterDate: registerDate.isNotEmpty ? registerDate : null,
            companyTownCode: hasTown ? postal.townCode : null,
            companyAddressDetail: detail.isNotEmpty ? detail : null,
          ),
          lookup: KycLookupFound(
            name: registration.name,
            registerDate: registerDate,
          ),
        ),
      );
    } catch (error) {
      if (seq != _lookupSeq || !ref.mounted) return;
      HKLog.logW('[KycWizard] lookup failed: $error');
      _set(
        (s) => s.copyWith(
          lookup: switch (submitError(
            error,
            defaults: ref.read(defaultErrorTextProvider),
          )) {
            SubmitMessage(:final message) => KycLookupFailed(message),
            SubmitFields(:final message) => KycLookupFailed(
              message ?? '統編查詢失敗',
            ),
            SubmitConflict() => const KycLookupFailed('統編查詢失敗'),
          },
        ),
      );
    }
  }

  /// 統編被改掉：之前的查詢結果不再適用。
  void resetLookup() {
    if (state.lookup is KycLookupIdle) return;
    _lookupSeq++;
    _set((s) => s.copyWith(lookup: const KycLookupIdle()));
  }

  /* ===== 店家資料 ===== */

  /// 把聯絡資料與地址寫進店家（`UpdateMyStore`）。沒有要寫的欄位、已寫完回 true；格式不合、地址不完整、版本衝突或失敗回 false，
  /// 原因在 [error]，輸入保留。成功後重讀申請，缺項以伺服器的清單為準。
  Future<bool> flushStore() async {
    if (storeContactErrors(state.store).any) {
      _set((s) => s.copyWith(error: kycContactFormatNotice));
      return false;
    }
    final addressError = storeAddressInputError(state.store);
    if (addressError != null) {
      _set((s) => s.copyWith(error: addressError));
      return false;
    }
    final draft = state.store;
    final changes = pendingStoreChanges(draft, state.storeBaseline);
    if (changes == null) return true;
    _set((s) => s.copyWith(storeSaving: true));
    try {
      final outcome = await ref.read(storeEditorProvider).save(changes);
      if (!ref.mounted) return false;
      switch (outcome) {
        case StoreSaved():
          final written = _currentStore;
          _set(
            (s) => s.copyWith(
              storeBaseline: written == null
                  ? draft
                  : storeContactDraftOf(written),
              storeServerErrorsFor: null,
            ),
          );
          await _reload();
          return true;
        case StoreSaveConflict(:final reloaded):
          if (reloaded) {
            final latest = storeContactDraftOf(_currentStore);
            _set(
              (s) => s.copyWith(
                store: rebaseStoreContact(
                  draft: s.store,
                  was: s.storeBaseline,
                  latest: latest,
                ),
                storeBaseline: latest,
                error: versionConflictNotice('店家資料'),
              ),
            );
          } else {
            _set((s) => s.copyWith(error: versionReloadFailedNotice));
          }
          return false;
        case StoreSaveFieldErrors(:final fields, :final message):
          _set(
            (s) => s.copyWith(
              storeServerErrors: storeContactServerErrors(fields),
              storeServerErrorsFor: draft,
              error: message ?? fields.values.first,
            ),
          );
          return false;
        case StoreSaveFailed(:final message):
          _set((s) => s.copyWith(error: message));
          return false;
      }
    } finally {
      _set((s) => s.copyWith(storeSaving: false));
    }
  }

  /* ===== 送出與捨棄 ===== */

  /// 送出認證：先把店家資料寫進店家、存好草稿，再以 [acceptances]（個資告知與 PayUni 條款的現行版本）直接送出 `SubmitKYC`。
  Future<KycSubmitOutcome> submit(List<AgreementAcceptance> acceptances) async {
    if (application == null || !editable || state.submitting) {
      return KycSubmitOutcome.failed;
    }
    _set((s) => s.copyWith(submitting: true, error: null));
    try {
      if (!await flushStore()) {
        _set((s) => s.copyWith(error: s.error ?? kycDraftSaveIncompleteNotice));
        return KycSubmitOutcome.failed;
      }
      if (!ref.mounted) return KycSubmitOutcome.failed;
      if (!await save() ||
          !ref.mounted ||
          _draftRequest(state.form, state.docs) != null) {
        if (ref.mounted) {
          _set(
            (s) => s.copyWith(error: s.error ?? kycDraftSaveIncompleteNotice),
          );
        }
        return KycSubmitOutcome.failed;
      }
      // 店家欄位的缺項以伺服器為準（快取可能過期）：送出後回 failed_precondition 再重讀並列出缺哪些。
      final stillMissing = missingItemsOf(
        application?.missingFields ?? const [],
        _l,
      );
      if (stillMissing.isNotEmpty) {
        _set((s) => s.copyWith(error: missingItemsMessage(stillMissing)));
        return KycSubmitOutcome.failed;
      }
      try {
        final response = await ref
            .read(sellerClientProvider)
            .submitKYC(
              SubmitKYCRequest(etag: _etag, acceptedAgreements: acceptances),
            );
        if (!ref.mounted) return KycSubmitOutcome.submitted;
        ref.read(kycReaderProvider.notifier).adopt(response.application);
        return KycSubmitOutcome.submitted;
      } catch (error) {
        if (!ref.mounted) return KycSubmitOutcome.failed;
        return await _submitFailure(error);
      }
    } finally {
      _set((s) => s.copyWith(submitting: false));
    }
  }

  Future<KycSubmitOutcome> _submitFailure(Object error) async {
    HKLog.logW('[KycWizard] SubmitKYC failed: $error');
    if (hasErrorReason(error, ErrorReason.AGREEMENT_OUTDATED)) {
      final reloaded = await _refreshAgreements();
      _set(
        (s) => s.copyWith(
          error: reloaded
              ? agreementOutdatedMessage
              : versionReloadFailedNotice,
        ),
      );
      return KycSubmitOutcome.agreementOutdated;
    }
    final failure = submitError(
      error,
      defaults: ref.read(defaultErrorTextProvider),
    );
    switch (failure) {
      case SubmitConflict():
        await _resolveConflict();
      case SubmitFields(:final fields, :final message):
        final errors = <KYCField, KycFieldError>{...state.fieldErrors};
        final labels = <String>[];
        for (final MapEntry(key: path, value: text) in fields.entries) {
          final field = kycFieldOfPath(path);
          if (field == null) continue;
          final value = _valueIn(state.form, field);
          if (value != null) {
            errors[_canonical(field)] = KycFieldError(
              message: text,
              value: value,
            );
          }
          final label = kycFieldLabel(field, _l);
          if (label != null && !labels.contains(label)) labels.add(label);
        }
        _set(
          (s) => s.copyWith(
            fieldErrors: errors,
            error: labels.isNotEmpty
                ? '下列欄位尚未填寫：${labels.join('、')}'
                : (message ?? '送出認證失敗'),
          ),
        );
        await _reload();
      case SubmitMessage(:final message):
        if (ConnectException.from(error).code == Code.failedPrecondition) {
          await _reload();
          _set(
            (s) => s.copyWith(
              error: submitPreconditionMessage(
                application?.missingStoreFields ?? const [],
                message,
                _l,
              ),
            ),
          );
        } else {
          _set((s) => s.copyWith(error: message));
        }
    }
    return KycSubmitOutcome.failed;
  }

  /// 條款過期後重讀入駐文件的現行版本；回傳有沒有讀成功。
  Future<bool> _refreshAgreements() async {
    ref.invalidate(sellerAgreementsProvider);
    try {
      await ref.read(sellerAgreementsProvider.future);
      return true;
    } catch (error) {
      HKLog.logW('[KycWizard] agreements reload failed: $error');
      return false;
    }
  }

  /// 捨棄草稿（`CancelKYC`），換身分重新開始。成功後沒有申請也沒有身分，表單回到店家的預設值。
  Future<bool> discard() async {
    if (application == null || state.discarding) return false;
    _set((s) => s.copyWith(discarding: true, error: null));
    try {
      await ref
          .read(sellerClientProvider)
          .cancelKYC(CancelKYCRequest(etag: _etag));
      if (!ref.mounted) return false;
      ref.read(kycReaderProvider.notifier).adopt(null);
      _resetToNoApplication();
      _lookupSeq++;
      return true;
    } catch (error) {
      HKLog.logW('[KycWizard] CancelKYC failed: $error');
      if (!ref.mounted) return false;
      switch (submitError(
        error,
        defaults: ref.read(defaultErrorTextProvider),
      )) {
        case SubmitConflict():
          await _resolveConflict();
        case SubmitFields():
          _set((s) => s.copyWith(error: '捨棄認證草稿失敗'));
        case SubmitMessage(:final message):
          _set((s) => s.copyWith(error: message));
      }
      return false;
    } finally {
      _set((s) => s.copyWith(discarding: false));
    }
  }

  /// 離開之前把還沒存的輸入存起來（草稿與店家資料各盡力一次）；失敗不阻擋離開。有動作進行中（送出、捨棄……）時不做。
  Future<void> saveBeforeExit() async {
    if (application == null || busy) return;
    await save();
    await flushStore();
  }
}
