import 'dart:typed_data';

import 'package:hawkr_api/dates.dart';
import 'package:hawkr_api/field_mask.dart';
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/postal_address.pb.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/gen/google/type/date.pb.dart';

import 'kyc_fields.dart';

/// 認證表單的值，全部是字串（日期是 `YYYY-MM-DD`）。伺服器版本是 baseline：只有和 baseline 不同的欄位才寫進
/// `update_mask`。身分證號與銀行帳號只能寫、讀不回明文，所以讀進來是空字串，它們的 baseline 是上一次送出的值。
/// 還缺什麼由伺服器的 `missing_fields` 決定，這裡不重寫完整性規則。
final class KycForm {
  const KycForm({
    this.realName = '',
    this.idCardNo = '',
    this.birthDate = '',
    this.idCardIssueDate = '',
    this.idCardIssueType = '',
    this.idCardIssueLocation = '',
    this.englishName = '',
    this.bankCode = '',
    this.bankName = '',
    this.bankAccount = '',
    this.productType = '',
    this.taxId = '',
    this.companyName = '',
    this.companyRegisterDate = '',
    this.companyTownCode = '',
    this.companyAddressDetail = '',
    this.companyPhone = '',
  });

  final String realName;
  final String idCardNo;
  final String birthDate;
  final String idCardIssueDate;
  final String idCardIssueType;
  final String idCardIssueLocation;
  final String englishName;
  final String bankCode;
  final String bankName;
  final String bankAccount;
  final String productType;
  final String taxId;
  final String companyName;
  final String companyRegisterDate;
  final String companyTownCode;
  final String companyAddressDetail;
  final String companyPhone;

  KycForm copyWith({
    String? realName,
    String? idCardNo,
    String? birthDate,
    String? idCardIssueDate,
    String? idCardIssueType,
    String? idCardIssueLocation,
    String? englishName,
    String? bankCode,
    String? bankName,
    String? bankAccount,
    String? productType,
    String? taxId,
    String? companyName,
    String? companyRegisterDate,
    String? companyTownCode,
    String? companyAddressDetail,
    String? companyPhone,
  }) => KycForm(
    realName: realName ?? this.realName,
    idCardNo: idCardNo ?? this.idCardNo,
    birthDate: birthDate ?? this.birthDate,
    idCardIssueDate: idCardIssueDate ?? this.idCardIssueDate,
    idCardIssueType: idCardIssueType ?? this.idCardIssueType,
    idCardIssueLocation: idCardIssueLocation ?? this.idCardIssueLocation,
    englishName: englishName ?? this.englishName,
    bankCode: bankCode ?? this.bankCode,
    bankName: bankName ?? this.bankName,
    bankAccount: bankAccount ?? this.bankAccount,
    productType: productType ?? this.productType,
    taxId: taxId ?? this.taxId,
    companyName: companyName ?? this.companyName,
    companyRegisterDate: companyRegisterDate ?? this.companyRegisterDate,
    companyTownCode: companyTownCode ?? this.companyTownCode,
    companyAddressDetail: companyAddressDetail ?? this.companyAddressDetail,
    companyPhone: companyPhone ?? this.companyPhone,
  );

  List<Object> get _fields => [
    realName,
    idCardNo,
    birthDate,
    idCardIssueDate,
    idCardIssueType,
    idCardIssueLocation,
    englishName,
    bankCode,
    bankName,
    bankAccount,
    productType,
    taxId,
    companyName,
    companyRegisterDate,
    companyTownCode,
    companyAddressDetail,
    companyPhone,
  ];

  @override
  bool operator ==(Object other) {
    if (other is! KycForm) return false;
    final mine = _fields;
    final theirs = other._fields;
    for (var i = 0; i < mine.length; i++) {
      if (mine[i] != theirs[i]) return false;
    }
    return true;
  }

  @override
  int get hashCode => Object.hashAll(_fields);
}

/// 一張證件圖片：已在申請上的（[assetId]，[downloadUri] 是要授權的下載路徑）或剛上傳還沒存檔的（[stagedToken]，
/// [preview] 是本機的圖片位元組）。
final class KycDoc {
  const KycDoc({
    this.assetId = '',
    this.stagedToken = '',
    this.downloadUri = '',
    this.preview,
  });

  final String assetId;
  final String stagedToken;
  final String downloadUri;
  final Uint8List? preview;

  KycDoc withPreview(Uint8List? bytes) => KycDoc(
    assetId: assetId,
    stagedToken: stagedToken,
    downloadUri: downloadUri,
    preview: bytes,
  );

  KYCDocument toDocument() => stagedToken.isNotEmpty
      ? KYCDocument(stagedToken: stagedToken)
      : KYCDocument(assetId: assetId);
}

/// 申請上的證件圖片的三個單張位置與企業文件清單（最多 [maxCompanyDocuments] 張）。
final class KycDocs {
  const KycDocs({
    this.idCardFront,
    this.idCardBack,
    this.bankBook,
    this.companyDocuments = const [],
  });

  static const maxCompanyDocuments = 10;

  final KycDoc? idCardFront;
  final KycDoc? idCardBack;
  final KycDoc? bankBook;
  final List<KycDoc> companyDocuments;

  KycDocs withSlot(KycDocSlot slot, KycDoc? doc) => switch (slot) {
    KycDocSlot.idCardFront => KycDocs(
      idCardFront: doc,
      idCardBack: idCardBack,
      bankBook: bankBook,
      companyDocuments: companyDocuments,
    ),
    KycDocSlot.idCardBack => KycDocs(
      idCardFront: idCardFront,
      idCardBack: doc,
      bankBook: bankBook,
      companyDocuments: companyDocuments,
    ),
    KycDocSlot.bankBook => KycDocs(
      idCardFront: idCardFront,
      idCardBack: idCardBack,
      bankBook: doc,
      companyDocuments: companyDocuments,
    ),
  };

  KycDocs withCompanyDocuments(List<KycDoc> docs) => KycDocs(
    idCardFront: idCardFront,
    idCardBack: idCardBack,
    bankBook: bankBook,
    companyDocuments: List.unmodifiable(docs),
  );

  KycDoc? slot(KycDocSlot slot) => switch (slot) {
    KycDocSlot.idCardFront => idCardFront,
    KycDocSlot.idCardBack => idCardBack,
    KycDocSlot.bankBook => bankBook,
  };
}

/// 申請上單張證件的位置。
enum KycDocSlot {
  idCardFront('id_card_front'),
  idCardBack('id_card_back'),
  bankBook('bank_book');

  const KycDocSlot(this.path);

  /// `KYCApplication` 的欄位名（update_mask 的路徑）。
  final String path;
}

/// 申請上填的是哪一種身分；還沒有申請（`subject` 沒設）時是 null。
KycSubject? kycSubjectOf(KYCApplication? app) {
  if (app == null) return null;
  return switch (app.whichSubject()) {
    KYCApplication_Subject.individual => KycSubject.individual,
    KYCApplication_Subject.business => KycSubject.business,
    KYCApplication_Subject.notSet => null,
  };
}

/// 申請上的值轉成表單的值；身分證號與銀行帳號讀不回明文，是空字串。
KycForm kycFormOf(KYCApplication? app) {
  if (app == null) return const KycForm();
  final business = app.hasBusiness() ? app.business : null;
  return KycForm(
    realName: app.realName,
    birthDate: formatDate(app.hasBirthDate() ? app.birthDate : null),
    idCardIssueDate: formatDate(
      app.hasIdCardIssueDate() ? app.idCardIssueDate : null,
    ),
    idCardIssueType: app.idCardIssueType,
    idCardIssueLocation: app.idCardIssueLocation,
    englishName: app.englishName,
    bankCode: app.bankCode,
    bankName: app.bankName,
    productType: app.productType,
    taxId: business?.taxId ?? '',
    companyName: business?.companyName ?? '',
    companyRegisterDate: formatDate(
      business != null && business.hasCompanyRegisterDate()
          ? business.companyRegisterDate
          : null,
    ),
    companyTownCode: business != null && business.hasCompanyAddress()
        ? business.companyAddress.townCode
        : '',
    companyAddressDetail: business != null && business.hasCompanyAddress()
        ? business.companyAddress.addressDetail
        : '',
    companyPhone: business?.companyPhone ?? '',
  );
}

KycDoc? _docOf(KYCDocument? doc) {
  if (doc == null || doc.assetId.isEmpty) return null;
  return KycDoc(assetId: doc.assetId, downloadUri: doc.downloadUri);
}

/// 申請上的證件。
KycDocs kycDocsOf(KYCApplication? app) {
  if (app == null) return const KycDocs();
  final business = app.hasBusiness() ? app.business : null;
  return KycDocs(
    idCardFront: _docOf(app.hasIdCardFront() ? app.idCardFront : null),
    idCardBack: _docOf(app.hasIdCardBack() ? app.idCardBack : null),
    bankBook: _docOf(app.hasBankBook() ? app.bankBook : null),
    companyDocuments: List.unmodifiable([
      for (final doc in business?.companyDocuments ?? const <KYCDocument>[])
        if (_docOf(doc) case final converted?) converted,
    ]),
  );
}

/// 讀出的遮蔽身分證號（個人的身分證號或企業負責人的）；沒有時是空字串。
String maskedIdOf(KYCApplication? app) {
  if (app == null) return '';
  if (app.hasIndividual()) return app.individual.idCardNoMasked;
  if (app.hasBusiness()) return app.business.repIdCardNoMasked;
  return '';
}

/// 存檔成功之後的 baseline：這次請求真的送出的欄位（[sentPaths]，取自請求的 update_mask）的 baseline 變成送出的值，沒送出的欄位
/// 維持原本的 baseline。「存檔之後有沒有改動」因此只看目前的輸入和送出的值，與伺服器怎麼正規化（去前後空白、電話去橫線）無關：
/// 不會重送，也不需要把伺服器的值寫回輸入中的表單；格式不合而沒送出的欄位仍與 baseline 不同，輸入原樣保留。
KycForm baselineAfterSave(
  KycForm previous,
  KycForm sent,
  Iterable<String> sentPaths,
) {
  final paths = sentPaths.toSet();
  T pick<T>(T was, T now, List<String> any) =>
      any.any(paths.contains) ? now : was;
  return KycForm(
    realName: pick(previous.realName, sent.realName, ['real_name']),
    idCardNo: pick(previous.idCardNo, sent.idCardNo, [
      'individual.id_card_no',
      'business.rep_id_card_no',
    ]),
    birthDate: pick(previous.birthDate, sent.birthDate, ['birth_date']),
    idCardIssueDate: pick(previous.idCardIssueDate, sent.idCardIssueDate, [
      'id_card_issue_date',
    ]),
    idCardIssueType: pick(previous.idCardIssueType, sent.idCardIssueType, [
      'id_card_issue_type',
    ]),
    idCardIssueLocation: pick(
      previous.idCardIssueLocation,
      sent.idCardIssueLocation,
      ['id_card_issue_location'],
    ),
    englishName: pick(previous.englishName, sent.englishName, ['english_name']),
    bankCode: pick(previous.bankCode, sent.bankCode, ['bank_code']),
    bankName: pick(previous.bankName, sent.bankName, ['bank_name']),
    bankAccount: pick(previous.bankAccount, sent.bankAccount, ['bank_account']),
    productType: pick(previous.productType, sent.productType, ['product_type']),
    taxId: pick(previous.taxId, sent.taxId, ['business.tax_id']),
    companyName: pick(previous.companyName, sent.companyName, [
      'business.company_name',
    ]),
    companyRegisterDate: pick(
      previous.companyRegisterDate,
      sent.companyRegisterDate,
      ['business.company_register_date'],
    ),
    companyTownCode: pick(previous.companyTownCode, sent.companyTownCode, [
      'business.company_address',
    ]),
    companyAddressDetail: pick(
      previous.companyAddressDetail,
      sent.companyAddressDetail,
      ['business.company_address'],
    ),
    companyPhone: pick(previous.companyPhone, sent.companyPhone, [
      'business.company_phone',
    ]),
  );
}

/* ===== 輸入驗證（只決定「這個值能不能送」，缺不缺由伺服器決定） ===== */

const _idLetterValues = {
  'A': 10,
  'B': 11,
  'C': 12,
  'D': 13,
  'E': 14,
  'F': 15,
  'G': 16,
  'H': 17,
  'I': 34,
  'J': 18,
  'K': 19,
  'L': 20,
  'M': 21,
  'N': 22,
  'O': 35,
  'P': 23,
  'Q': 24,
  'R': 25,
  'S': 26,
  'T': 27,
  'U': 28,
  'V': 29,
  'W': 32,
  'X': 30,
  'Y': 31,
  'Z': 33,
};

/// 身分證字號的錯誤文字；通過是 null。
String? validateTwId(String value) {
  final v = value.trim().toUpperCase();
  if (v.isEmpty) return '請輸入身分證字號';
  if (!RegExp(r'^[A-Z][12]\d{8}$').hasMatch(v)) return '格式錯誤（A123456789）';
  final letterValue = _idLetterValues[v[0]];
  if (letterValue == null) return '無效的英文字母';
  var sum = letterValue ~/ 10 + (letterValue % 10) * 9;
  for (var i = 1; i <= 8; i++) {
    sum += int.parse(v[i]) * (9 - i);
  }
  sum += int.parse(v[9]);
  if (sum % 10 != 0) return '身分證字號檢查碼錯誤，請確認再輸入';
  return null;
}

/// 銀行帳號的錯誤文字；通過是 null。
String? validateBankAccount(String value) {
  final v = value.trim();
  if (v.isEmpty) return '請輸入銀行帳號';
  if (!RegExp(r'^\d{8,16}$').hasMatch(v)) return '銀行帳號需為 8–16 位數字';
  return null;
}

bool isValidIdCardNo(String value) =>
    value.isNotEmpty && validateTwId(value) == null;

bool isValidBankAccount(String value) =>
    value.isNotEmpty && validateBankAccount(value) == null;

/// 已輸入的身分證號與銀行帳號的格式錯誤（沒輸入不報）。
({String? idError, String? accountError}) kycInputErrors(KycForm form) => (
  idError: form.idCardNo.isEmpty ? null : validateTwId(form.idCardNo),
  accountError: form.bankAccount.isEmpty
      ? null
      : validateBankAccount(form.bankAccount),
);

/* ===== 表單 → SaveKYCDraft ===== */

bool _docChanged(KycDoc? next, KycDoc? base) {
  if (next == null || base == null) return next != base;
  return next.stagedToken.isNotEmpty || next.assetId != base.assetId;
}

bool _docListChanged(List<KycDoc> next, List<KycDoc> base) {
  if (next.length != base.length) return true;
  for (var i = 0; i < next.length; i++) {
    if (_docChanged(next[i], base[i])) return true;
  }
  return false;
}

/// 表單相對於 baseline 的差異，組成 `SaveKYCDraft`。沒有申請（[etag] 為 null 或空）時是建立：subject 一定設，所有非零值都送；
/// 已經有申請時只列有改的欄位，etag 必帶。沒有改動回 null。格式不合的身分證號與銀行帳號不送——伺服器會因為任何一欄
/// 不合法而整筆不寫入，那會讓其他欄位的暫存一起失敗。
SaveKYCDraftRequest? buildSaveKycDraftRequest({
  required KycSubject subject,
  required KycForm form,
  required KycDocs docs,
  required KycForm baseline,
  required KycDocs baselineDocs,
  String? etag,
}) {
  final creating = etag == null || etag.isEmpty;
  final business = subject == KycSubject.business;
  final paths = <String>[];
  final app = KYCApplication();
  final individual = IndividualKYC();
  final biz = BusinessKYC();
  var touchesBranch = false;

  void text(
    String path,
    String value,
    String base,
    void Function(String) set, {
    bool allowed = true,
  }) {
    if (!allowed || value == base) return;
    paths.add(path);
    set(value);
  }

  void date(
    String path,
    String value,
    String base,
    void Function(Date) set, {
    bool onBranch = false,
  }) {
    if (value == base) return;
    final parsed = parseDate(value);
    if (value.isNotEmpty && parsed == null) return;
    paths.add(path);
    if (parsed != null) set(parsed);
    if (onBranch) touchesBranch = true;
  }

  text('real_name', form.realName, baseline.realName, (v) => app.realName = v);
  text(
    'english_name',
    form.englishName,
    baseline.englishName,
    (v) => app.englishName = v,
  );
  text(
    'id_card_issue_type',
    form.idCardIssueType,
    baseline.idCardIssueType,
    (v) => app.idCardIssueType = v,
  );
  text(
    'id_card_issue_location',
    form.idCardIssueLocation,
    baseline.idCardIssueLocation,
    (v) => app.idCardIssueLocation = v,
  );
  text('bank_code', form.bankCode, baseline.bankCode, (v) => app.bankCode = v);
  text('bank_name', form.bankName, baseline.bankName, (v) => app.bankName = v);
  text(
    'bank_account',
    form.bankAccount,
    baseline.bankAccount,
    (v) => app.bankAccount = v,
    allowed: isValidBankAccount(form.bankAccount),
  );
  text(
    'product_type',
    form.productType,
    baseline.productType,
    (v) => app.productType = v,
  );
  date(
    'birth_date',
    form.birthDate,
    baseline.birthDate,
    (v) => app.birthDate = v,
  );
  date(
    'id_card_issue_date',
    form.idCardIssueDate,
    baseline.idCardIssueDate,
    (v) => app.idCardIssueDate = v,
  );

  if (form.idCardNo != baseline.idCardNo && isValidIdCardNo(form.idCardNo)) {
    if (business) {
      paths.add('business.rep_id_card_no');
      biz.repIdCardNo = form.idCardNo;
    } else {
      paths.add('individual.id_card_no');
      individual.idCardNo = form.idCardNo;
    }
    touchesBranch = true;
  }

  if (business) {
    text('business.company_name', form.companyName, baseline.companyName, (v) {
      biz.companyName = v;
      touchesBranch = true;
    });
    text('business.tax_id', form.taxId, baseline.taxId, (v) {
      biz.taxId = v;
      touchesBranch = true;
    });
    text('business.company_phone', form.companyPhone, baseline.companyPhone, (
      v,
    ) {
      biz.companyPhone = v;
      touchesBranch = true;
    });
    date(
      'business.company_register_date',
      form.companyRegisterDate,
      baseline.companyRegisterDate,
      (v) => biz.companyRegisterDate = v,
      onBranch: true,
    );
    final addressChanged =
        form.companyTownCode != baseline.companyTownCode ||
        form.companyAddressDetail != baseline.companyAddressDetail;
    if (addressChanged) {
      if (form.companyTownCode.isNotEmpty) {
        paths.add('business.company_address');
        biz.companyAddress = PostalAddress(
          townCode: form.companyTownCode,
          addressDetail: form.companyAddressDetail,
        );
        touchesBranch = true;
      } else if (form.companyAddressDetail.isEmpty) {
        paths.add('business.company_address');
        touchesBranch = true;
      }
    }
  }

  for (final slot in KycDocSlot.values) {
    final next = docs.slot(slot);
    if (_docChanged(next, baselineDocs.slot(slot))) {
      paths.add(slot.path);
      if (next != null) {
        final document = next.toDocument();
        switch (slot) {
          case KycDocSlot.idCardFront:
            app.idCardFront = document;
          case KycDocSlot.idCardBack:
            app.idCardBack = document;
          case KycDocSlot.bankBook:
            app.bankBook = document;
        }
      }
    }
  }
  if (business &&
      _docListChanged(docs.companyDocuments, baselineDocs.companyDocuments)) {
    paths.add('business.company_documents');
    biz.companyDocuments.addAll([
      for (final doc in docs.companyDocuments) doc.toDocument(),
    ]);
    touchesBranch = true;
  }

  if (!creating && paths.isEmpty) return null;

  if (creating || touchesBranch) {
    if (business) {
      app.business = biz;
    } else {
      app.individual = individual;
    }
  }
  if (!creating) app.etag = etag;
  return SaveKYCDraftRequest(
    application: app,
    updateMask: fieldMask(KYCApplication(), paths),
  );
}

/* ===== 版本衝突：保留輸入、顯示最新 ===== */

T _keep<T>(T current, T oldBaseline, T latest) =>
    current != oldBaseline ? current : latest;

/// 衝突之後的表單：使用者改過的欄位（與舊 baseline 不同）保留輸入，沒改的欄位換成最新版本。身分證號與銀行帳號伺服器
/// 讀不回明文，一律留著目前輸入的值。
KycForm mergeLatestForm(KycForm current, KycForm oldBaseline, KycForm latest) =>
    KycForm(
      realName: _keep(current.realName, oldBaseline.realName, latest.realName),
      idCardNo: current.idCardNo,
      birthDate: _keep(
        current.birthDate,
        oldBaseline.birthDate,
        latest.birthDate,
      ),
      idCardIssueDate: _keep(
        current.idCardIssueDate,
        oldBaseline.idCardIssueDate,
        latest.idCardIssueDate,
      ),
      idCardIssueType: _keep(
        current.idCardIssueType,
        oldBaseline.idCardIssueType,
        latest.idCardIssueType,
      ),
      idCardIssueLocation: _keep(
        current.idCardIssueLocation,
        oldBaseline.idCardIssueLocation,
        latest.idCardIssueLocation,
      ),
      englishName: _keep(
        current.englishName,
        oldBaseline.englishName,
        latest.englishName,
      ),
      bankCode: _keep(current.bankCode, oldBaseline.bankCode, latest.bankCode),
      bankName: _keep(current.bankName, oldBaseline.bankName, latest.bankName),
      bankAccount: current.bankAccount,
      productType: _keep(
        current.productType,
        oldBaseline.productType,
        latest.productType,
      ),
      taxId: _keep(current.taxId, oldBaseline.taxId, latest.taxId),
      companyName: _keep(
        current.companyName,
        oldBaseline.companyName,
        latest.companyName,
      ),
      companyRegisterDate: _keep(
        current.companyRegisterDate,
        oldBaseline.companyRegisterDate,
        latest.companyRegisterDate,
      ),
      companyTownCode: _keep(
        current.companyTownCode,
        oldBaseline.companyTownCode,
        latest.companyTownCode,
      ),
      companyAddressDetail: _keep(
        current.companyAddressDetail,
        oldBaseline.companyAddressDetail,
        latest.companyAddressDetail,
      ),
      companyPhone: _keep(
        current.companyPhone,
        oldBaseline.companyPhone,
        latest.companyPhone,
      ),
    );

/// 證件圖片的同一規則：使用者換過、移除過或新傳的位置保留，其餘換成最新版本。
KycDocs mergeLatestDocs(KycDocs current, KycDocs oldBaseline, KycDocs latest) {
  KycDoc? slot(KycDocSlot s) =>
      _docChanged(current.slot(s), oldBaseline.slot(s))
      ? current.slot(s)
      : latest.slot(s);
  return KycDocs(
    idCardFront: slot(KycDocSlot.idCardFront),
    idCardBack: slot(KycDocSlot.idCardBack),
    bankBook: slot(KycDocSlot.bankBook),
    companyDocuments:
        _docListChanged(current.companyDocuments, oldBaseline.companyDocuments)
        ? current.companyDocuments
        : latest.companyDocuments,
  );
}

/// 存檔回應之後的證件：剛傳的（staged）換成申請上的圖片，預覽沿用本機的（不必重新載入），使用者在存檔期間又動過的位置
/// 不碰。
KycDocs adoptSavedDocs(KycDocs current, KycDocs sent, KycDocs saved) {
  KycDoc? adopt(KycDoc? cur, KycDoc? now) =>
      cur != null && now != null ? now.withPreview(cur.preview) : now;
  KycDoc? slot(KycDocSlot s) => identical(current.slot(s), sent.slot(s))
      ? adopt(current.slot(s), saved.slot(s))
      : current.slot(s);
  return KycDocs(
    idCardFront: slot(KycDocSlot.idCardFront),
    idCardBack: slot(KycDocSlot.idCardBack),
    bankBook: slot(KycDocSlot.bankBook),
    companyDocuments: identical(current.companyDocuments, sent.companyDocuments)
        ? List.unmodifiable([
            for (var i = 0; i < sent.companyDocuments.length; i++)
              if (adopt(
                    sent.companyDocuments[i],
                    i < saved.companyDocuments.length
                        ? saved.companyDocuments[i]
                        : null,
                  )
                  case final doc?)
                doc,
          ])
        : current.companyDocuments,
  );
}

/* ===== 可編輯 ===== */

/// 表單可以編輯：沒有申請（要建立），或伺服器說此刻能儲存草稿與重送（`flow.can_resubmit`：草稿、被退回、PayUni 拒絕或綁定失敗而且賣家能自己修正）。
bool kycEditable(KYCApplication? app) =>
    app == null || (app.hasFlow() && app.flow.canResubmit);

/// 可以捨棄草稿（`CancelKYC`）：還是草稿，而且身分欄位沒有被 PayUni 鎖住。
bool kycCanDiscard(KYCApplication? app) =>
    app != null &&
    app.status == KYCStatus.KYC_STATUS_DRAFT &&
    !(app.hasFlow() && app.flow.identityLocked);

/* ===== 日期的顯示與輸入（表單存 `YYYY-MM-DD`，畫面與證件上是民國年） ===== */

/// `YYYY-MM-DD` 轉成民國年 `YYY/MM/DD`；不是有效日期回空字串。
String rocDateOfIso(String iso) {
  final date = parseDate(iso);
  if (date == null) return '';
  final year = (date.year - 1911).toString().padLeft(3, '0');
  return '$year/${date.month.toString().padLeft(2, '0')}/${date.day.toString().padLeft(2, '0')}';
}

/// 民國年 `YYY/MM/DD`（OCR 辨識的證件日期）轉成 `YYYY-MM-DD`；不是有效日期回空字串。
String isoOfRocDate(String roc) {
  final match = RegExp(r'^(\d{2,3})/(\d{2})/(\d{2})$').firstMatch(roc.trim());
  if (match == null) return '';
  final year = int.parse(match[1]!) + 1911;
  final iso = '${year.toString().padLeft(4, '0')}-${match[2]}-${match[3]}';
  return parseDate(iso) == null ? '' : iso;
}

/// 日期選擇器選到的日子轉成 `YYYY-MM-DD`。
String isoOfDateTime(DateTime date) =>
    '${date.year.toString().padLeft(4, '0')}-${date.month.toString().padLeft(2, '0')}-${date.day.toString().padLeft(2, '0')}';
