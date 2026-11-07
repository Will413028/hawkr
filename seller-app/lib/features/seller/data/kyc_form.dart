import 'dart:typed_data';

import 'package:hawkr_api/dates.dart';
import 'package:hawkr_api/field_mask.dart';
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/postal_address.pb.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/gen/google/type/date.pb.dart';

import 'kyc_fields.dart';

/// 認證表單的值，全部是字串（日期是 `YYYY-MM-DD`）。伺服器版本是 baseline：只有和 baseline 不同的欄位才寫進
/// `update_mask`。平台只收市集登記資料（D22）：身分、銀行與證件由金流商收集，不在表單裡。
/// 還缺什麼由伺服器的 `missing_fields` 決定，這裡不重寫完整性規則。
final class KycForm {
  const KycForm({
    this.productType = '',
    this.taxId = '',
    this.companyName = '',
    this.companyRegisterDate = '',
    this.companyTownCode = '',
    this.companyAddressDetail = '',
    this.companyPhone = '',
  });

  final String productType;
  final String taxId;
  final String companyName;
  final String companyRegisterDate;
  final String companyTownCode;
  final String companyAddressDetail;
  final String companyPhone;

  KycForm copyWith({
    String? productType,
    String? taxId,
    String? companyName,
    String? companyRegisterDate,
    String? companyTownCode,
    String? companyAddressDetail,
    String? companyPhone,
  }) => KycForm(
    productType: productType ?? this.productType,
    taxId: taxId ?? this.taxId,
    companyName: companyName ?? this.companyName,
    companyRegisterDate: companyRegisterDate ?? this.companyRegisterDate,
    companyTownCode: companyTownCode ?? this.companyTownCode,
    companyAddressDetail: companyAddressDetail ?? this.companyAddressDetail,
    companyPhone: companyPhone ?? this.companyPhone,
  );

  List<Object> get _fields => [
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

/// 申請上的企業補充文件清單（最多 [maxCompanyDocuments] 張；市集審核用的公司證明，不是身分證件）。
final class KycDocs {
  const KycDocs({this.companyDocuments = const []});

  static const maxCompanyDocuments = 10;

  final List<KycDoc> companyDocuments;

  KycDocs withCompanyDocuments(List<KycDoc> docs) =>
      KycDocs(companyDocuments: List.unmodifiable(docs));
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

/// 申請上的值轉成表單的值。
KycForm kycFormOf(KYCApplication? app) {
  if (app == null) return const KycForm();
  final business = app.hasBusiness() ? app.business : null;
  return KycForm(
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

/// 申請上的企業補充文件。
KycDocs kycDocsOf(KYCApplication? app) {
  if (app == null) return const KycDocs();
  final business = app.hasBusiness() ? app.business : null;
  return KycDocs(
    companyDocuments: List.unmodifiable([
      for (final doc in business?.companyDocuments ?? const <KYCDocument>[])
        if (_docOf(doc) case final converted?) converted,
    ]),
  );
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
/// 已經有申請時只列有改的欄位，etag 必帶。沒有改動回 null。
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

  void text(String path, String value, String base, void Function(String) set) {
    if (value == base) return;
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

  text(
    'product_type',
    form.productType,
    baseline.productType,
    (v) => app.productType = v,
  );

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

/// 衝突之後的表單：使用者改過的欄位（與舊 baseline 不同）保留輸入，沒改的欄位換成最新版本。
KycForm mergeLatestForm(KycForm current, KycForm oldBaseline, KycForm latest) =>
    KycForm(
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

/// 企業文件的同一規則：使用者換過、移除過或新傳的清單保留，否則換成最新版本。
KycDocs mergeLatestDocs(KycDocs current, KycDocs oldBaseline, KycDocs latest) =>
    KycDocs(
      companyDocuments:
          _docListChanged(
            current.companyDocuments,
            oldBaseline.companyDocuments,
          )
          ? current.companyDocuments
          : latest.companyDocuments,
    );

/// 存檔回應之後的證件：剛傳的（staged）換成申請上的圖片，預覽沿用本機的（不必重新載入），使用者在存檔期間又動過的清單不碰。
KycDocs adoptSavedDocs(KycDocs current, KycDocs sent, KycDocs saved) {
  KycDoc? adopt(KycDoc? cur, KycDoc? now) =>
      cur != null && now != null ? now.withPreview(cur.preview) : now;
  return KycDocs(
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

/* ===== 日期的顯示（表單存 `YYYY-MM-DD`，畫面是民國年） ===== */

/// `YYYY-MM-DD` 轉成民國年 `YYY/MM/DD`；不是有效日期回空字串。
String rocDateOfIso(String iso) {
  final date = parseDate(iso);
  if (date == null) return '';
  final year = (date.year - 1911).toString().padLeft(3, '0');
  return '$year/${date.month.toString().padLeft(2, '0')}/${date.day.toString().padLeft(2, '0')}';
}

/// 日期選擇器選到的日子轉成 `YYYY-MM-DD`。
String isoOfDateTime(DateTime date) =>
    '${date.year.toString().padLeft(4, '0')}-${date.month.toString().padLeft(2, '0')}-${date.day.toString().padLeft(2, '0')}';
