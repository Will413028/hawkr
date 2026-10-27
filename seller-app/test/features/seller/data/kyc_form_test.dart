import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/gen/google/type/date.pb.dart';
import 'package:hawkr_seller/features/seller/data/kyc_fields.dart';
import 'package:hawkr_seller/features/seller/data/kyc_form.dart';
import 'package:hawkr_ui_text/hawkr_ui_text.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../../support/kyc_fixtures.dart';

SaveKYCDraftRequest? _build({
  KycSubject subject = KycSubject.individual,
  KycForm form = const KycForm(),
  KycDocs docs = const KycDocs(),
  KycForm baseline = const KycForm(),
  KycDocs baselineDocs = const KycDocs(),
  String? etag = 'v1',
}) => buildSaveKycDraftRequest(
  subject: subject,
  form: form,
  docs: docs,
  baseline: baseline,
  baselineDocs: baselineDocs,
  etag: etag,
);

void main() {
  group('buildSaveKycDraftRequest', () {
    test('沒有改動就不送', () {
      expect(_build(), isNull);
    });

    test('建立：沒有 etag，subject 一定設定，所有非零值都列進 update_mask', () {
      final request = _build(
        etag: null,
        form: const KycForm(realName: '王小明', birthDate: '1988-06-10'),
      )!;

      expect(request.application.etag, isEmpty);
      expect(request.application.hasIndividual(), isTrue);
      expect(request.updateMask.paths, ['real_name', 'birth_date']);
    });

    test('建立：沒有任何值也要建（subject 設定）', () {
      final request = _build(etag: null, subject: KycSubject.business)!;

      expect(request.application.hasBusiness(), isTrue);
      expect(request.updateMask.paths, isEmpty);
    });

    test('更新只列改過的欄位；沒碰到那一支專屬欄位就不帶 subject', () {
      final request = _build(
        form: const KycForm(realName: '王', englishName: 'Wang'),
        baseline: const KycForm(realName: '王'),
      )!;

      expect(request.updateMask.paths, ['english_name']);
      expect(request.application.hasIndividual(), isFalse);
      expect(request.application.hasBusiness(), isFalse);
    });

    test('日期是 google.type.Date；清空日期是列進 mask 但不帶值', () {
      final set = _build(
        form: const KycForm(birthDate: '1988-06-10'),
        baseline: const KycForm(),
      )!;
      expect(set.application.birthDate, Date(year: 1988, month: 6, day: 10));

      final cleared = _build(
        form: const KycForm(),
        baseline: const KycForm(birthDate: '1988-06-10'),
      )!;
      expect(cleared.updateMask.paths, ['birth_date']);
      expect(cleared.application.hasBirthDate(), isFalse);
    });

    test('無效的日期不送', () {
      expect(_build(form: const KycForm(birthDate: '1988-02-30')), isNull);
    });

    test('格式不合的身分證號與銀行帳號不送；合法的送', () {
      expect(
        _build(
          form: const KycForm(idCardNo: 'A12', bankAccount: '12'),
        ),
        isNull,
      );

      final request = _build(
        form: const KycForm(idCardNo: 'A123456789', bankAccount: '12345678'),
      )!;
      expect(request.updateMask.paths, [
        'bank_account',
        'individual.id_card_no',
      ]);
      expect(request.application.individual.idCardNo, 'A123456789');
      expect(request.application.bankAccount, '12345678');
    });

    test('企業：統編、公司名稱、電話、設立日期、地址在 business 底下；證號是負責人身分證號', () {
      final request = _build(
        subject: KycSubject.business,
        form: const KycForm(
          idCardNo: 'A123456789',
          taxId: '22223223',
          companyName: '範例果園',
          companyPhone: '02-1234',
          companyRegisterDate: '2020-01-02',
          companyTownCode: '63000010',
          companyAddressDetail: '路一號',
        ),
      )!;

      expect(request.updateMask.paths, [
        'business.rep_id_card_no',
        'business.company_name',
        'business.tax_id',
        'business.company_phone',
        'business.company_register_date',
        'business.company_address',
      ]);
      final business = request.application.business;
      expect(business.repIdCardNo, 'A123456789');
      expect(business.companyAddress.townCode, '63000010');
      expect(business.companyAddress.addressDetail, '路一號');
      expect(business.companyRegisterDate, Date(year: 2020, month: 1, day: 2));
    });

    test('個人不會帶企業的欄位', () {
      final request = _build(
        form: const KycForm(taxId: '22223223', companyName: 'x'),
      );
      expect(request, isNull);
    });

    test('證件：新上傳用 staged token，保留的用 asset ID，移除的列進 mask 不帶值', () {
      const front = KycDoc(assetId: 'a1', downloadUri: '/dl/a1');
      final request = _build(
        docs: const KycDocs(
          idCardFront: front,
          idCardBack: KycDoc(stagedToken: 't2'),
        ),
        baselineDocs: const KycDocs(
          idCardFront: front,
          bankBook: KycDoc(assetId: 'b1'),
        ),
      )!;

      expect(request.updateMask.paths, ['id_card_back', 'bank_book']);
      expect(request.application.idCardBack.stagedToken, 't2');
      expect(request.application.hasBankBook(), isFalse);
    });

    test('企業文件整個換成請求的清單，順序就是位置', () {
      final request = _build(
        subject: KycSubject.business,
        docs: const KycDocs(
          companyDocuments: [
            KycDoc(assetId: 'a'),
            KycDoc(stagedToken: 't'),
          ],
        ),
        baselineDocs: const KycDocs(companyDocuments: [KycDoc(assetId: 'a')]),
      )!;

      expect(request.updateMask.paths, ['business.company_documents']);
      final docs = request.application.business.companyDocuments;
      expect(docs[0].assetId, 'a');
      expect(docs[1].stagedToken, 't');
    });

    test('個人不送企業文件', () {
      final request = _build(
        docs: const KycDocs(companyDocuments: [KycDoc(stagedToken: 't')]),
      );
      expect(request, isNull);
    });
  });

  group('申請 → 表單', () {
    test('讀出的證號與帳號是空字串，遮蔽值另外取', () {
      final app = individualApplication(realName: '王', idMasked: 'A12****789')
        ..bankAccountMasked = '****5678';
      final form = kycFormOf(app);

      expect(form.idCardNo, isEmpty);
      expect(form.bankAccount, isEmpty);
      expect(maskedIdOf(app), 'A12****789');
    });

    test('身分以 subject 為準；沒有申請或 subject 沒設是 null', () {
      expect(kycSubjectOf(individualApplication()), KycSubject.individual);
      expect(kycSubjectOf(businessApplication()), KycSubject.business);
      expect(kycSubjectOf(KYCApplication()), isNull);
      expect(kycSubjectOf(null), isNull);
    });

    test('證件只認有 asset ID 的，帶下載路徑', () {
      final docs = kycDocsOf(individualApplication(frontAssetId: 'a1'));
      expect(docs.idCardFront?.downloadUri, contains('a1'));
      expect(docs.idCardBack, isNull);
    });
  });

  group('版本衝突的合併', () {
    test('使用者改過的欄位保留，沒改的換成最新；證號與帳號一律保留輸入', () {
      const old = KycForm(realName: '舊', englishName: '');
      final merged = mergeLatestForm(
        const KycForm(
          realName: '我改的',
          idCardNo: 'A123456789',
          bankAccount: '12345678',
        ),
        old,
        const KycForm(realName: '別處改的', englishName: 'Latest'),
      );

      expect(merged.realName, '我改的');
      expect(merged.englishName, 'Latest');
      expect(merged.idCardNo, 'A123456789');
      expect(merged.bankAccount, '12345678');
    });

    test('證件：換過、移除過的位置保留，其餘換成最新', () {
      const mine = KycDoc(stagedToken: 't');
      final merged = mergeLatestDocs(
        const KycDocs(idCardFront: mine),
        const KycDocs(),
        const KycDocs(
          idCardFront: KycDoc(assetId: 'x'),
          bankBook: KycDoc(assetId: 'b'),
        ),
      );

      expect(merged.idCardFront, mine);
      expect(merged.bankBook?.assetId, 'b');
    });
  });

  group('baselineAfterSave（以送出的值為基準，與伺服器的正規化無關）', () {
    test('送出的欄位的 baseline 是送出的值：正規化後不會被當成又改了而重送', () {
      const sent = KycForm(
        englishName: 'Wang Farm ',
        companyPhone: '02-12345678',
        companyRegisterDate: '2020-01-02',
        companyTownCode: '63000010',
        companyAddressDetail: '路一號',
      );

      final baseline = baselineAfterSave(const KycForm(), sent, [
        'english_name',
        'business.company_phone',
        'business.company_register_date',
        'business.company_address',
      ]);

      expect(baseline, sent);
      expect(
        _build(subject: KycSubject.business, form: sent, baseline: baseline),
        isNull,
      );
    });

    test('沒送出的欄位維持原本的 baseline，仍與輸入不同（不算已存）', () {
      const sent = KycForm(englishName: 'Wang ', birthDate: '1988-02-3');

      final baseline = baselineAfterSave(const KycForm(), sent, [
        'english_name',
      ]);

      expect(baseline.englishName, 'Wang ');
      expect(baseline.birthDate, isEmpty);
      expect(baseline.birthDate, isNot(sent.birthDate));
    });

    test('存檔期間又被改過的欄位仍與 baseline 不同，下次存檔會送', () {
      const sent = KycForm(englishName: 'Wang ');
      const current = KycForm(englishName: 'Wang Farm');

      final baseline = baselineAfterSave(const KycForm(), sent, [
        'english_name',
      ]);
      final request = _build(form: current, baseline: baseline);

      expect(request?.updateMask.paths, ['english_name']);
    });

    test('證號與帳號：只有真的送出（在 mask）才成為 baseline', () {
      const sent = KycForm(idCardNo: 'A123456789', bankAccount: '123');

      final baseline = baselineAfterSave(const KycForm(), sent, [
        'individual.id_card_no',
      ]);

      expect(baseline.idCardNo, 'A123456789');
      expect(baseline.bankAccount, isEmpty);
    });
  });

  group('逐欄位重建不丟欄位', () {
    const full = KycForm(
      realName: 'a',
      idCardNo: 'b',
      birthDate: '1988-06-10',
      idCardIssueDate: '2010-01-02',
      idCardIssueType: 'c',
      idCardIssueLocation: 'd',
      englishName: 'e',
      bankCode: 'f',
      bankName: 'g',
      bankAccount: 'h',
      productType: 'i',
      taxId: 'k',
      companyName: 'l',
      companyRegisterDate: '2020-01-02',
      companyTownCode: 'm',
      companyAddressDetail: 'n',
      companyPhone: 'o',
    );

    test('版本衝突合併：使用者改過的每個欄位都在', () {
      expect(mergeLatestForm(full, const KycForm(), const KycForm()), full);
    });

    test('copyWith 不帶參數等於原值，逐欄位帶參數只改那一欄', () {
      expect(full.copyWith(), full);
      expect(full.copyWith(taxId: 'z').taxId, 'z');
      expect(full.copyWith(taxId: 'z').copyWith(taxId: 'k'), full);
    });

    test('證件：換位置、換清單都不動其他位置', () {
      const docs = KycDocs(
        idCardFront: KycDoc(assetId: 'f'),
        idCardBack: KycDoc(assetId: 'b'),
        bankBook: KycDoc(assetId: 'k'),
        companyDocuments: [KycDoc(assetId: 'c')],
      );
      final swapped = docs.withSlot(KycDocSlot.bankBook, null);
      expect(swapped.bankBook, isNull);
      expect(swapped.idCardFront, docs.idCardFront);
      expect(swapped.idCardBack, docs.idCardBack);
      expect(swapped.companyDocuments, docs.companyDocuments);
      final relisted = docs.withCompanyDocuments(const []);
      expect(relisted.idCardFront, docs.idCardFront);
      expect(relisted.bankBook, docs.bankBook);
      expect(relisted.companyDocuments, isEmpty);
    });
  });

  group('續填、退回、可編輯', () {
    test('缺項的欄位名稱用 KYCField 為鍵；每個有效的 KYCField 都有名稱與所在的一步', () {
      for (final field in KYCField.values) {
        if (field == KYCField.KYC_FIELD_UNSPECIFIED) {
          expect(kycFieldLabel(field, UiLocale.zhTW), isNull);
          continue;
        }
        expect(
          kycFieldLabel(field, UiLocale.zhTW),
          isNotNull,
          reason: field.name,
        );
        expect(kycStepOfField(field), isNotNull, reason: field.name);
      }
    });

    test('缺項依表單控制項去重：銀行代碼與銀行名稱是兩個輸入框，各列一次', () {
      final items = missingItemsOf([
        KYCField.KYC_FIELD_BANK_CODE,
        KYCField.KYC_FIELD_BANK_NAME,
        KYCField.KYC_FIELD_BANK_ACCOUNT,
        KYCField.KYC_FIELD_UNSPECIFIED,
      ], UiLocale.zhTW);
      expect(items.map((i) => i.label), ['銀行代碼', '銀行名稱', '銀行帳號']);
    });

    test('缺項依表單控制項去重：個人與負責人共用的身分證字號輸入框只列一次（欄位號小的名詞）', () {
      final items = missingItemsOf([
        KYCField.KYC_FIELD_BUSINESS_REP_ID_CARD_NO,
        KYCField.KYC_FIELD_INDIVIDUAL_ID_CARD_NO,
      ], UiLocale.zhTW);
      expect(items.map((i) => i.label), ['身分證字號']);
    });

    test('表單標籤與缺項清單同名（共用文案目錄的 KYCField 名詞）', () {
      expect(
        kycFieldLabel(KYCField.KYC_FIELD_REAL_NAME, UiLocale.zhTW),
        '真實姓名',
      );
      expect(
        kycFieldLabel(KYCField.KYC_FIELD_ID_CARD_FRONT, UiLocale.zhTW),
        '身分證正面照',
      );
      expect(
        kycFieldLabel(KYCField.KYC_FIELD_PRODUCT_TYPE, UiLocale.zhTW),
        '主要商品類型',
      );
    });

    test('店家缺項的名稱', () {
      expect(
        storeMissingItemsOf(
          StoreField.values,
          UiLocale.zhTW,
        ).map((i) => i.label),
        ['店家地址', '聯絡人姓名', '聯絡人手機', '客服電子信箱', '商店電話'],
      );
      expect(
        submitPreconditionMessage([], '身分比對未通過', UiLocale.zhTW),
        '身分比對未通過',
      );
      expect(
        submitPreconditionMessage(
          [StoreField.STORE_FIELD_ADDRESS],
          'x',
          UiLocale.zhTW,
        ),
        '下列欄位尚未填寫：店家地址',
      );
    });

    test('BadRequest 的欄位路徑對應回 KYCField', () {
      expect(
        kycFieldOfPath('business.tax_id'),
        KYCField.KYC_FIELD_BUSINESS_TAX_ID,
      );
      expect(kycFieldOfPath('real_name'), KYCField.KYC_FIELD_REAL_NAME);
      expect(
        kycFieldOfPath('business.company_documents[2]'),
        KYCField.KYC_FIELD_BUSINESS_COMPANY_DOCUMENTS,
      );
      expect(kycFieldOfPath('unknown[1]'), isNull);
      expect(kycFieldOfPath('unknown'), isNull);
      expect(
        kycFieldOfPath('business.company_address.town_code'),
        KYCField.KYC_FIELD_BUSINESS_COMPANY_ADDRESS,
      );
      expect(
        kycFieldOfPath('business.company_address.address_detail'),
        KYCField.KYC_FIELD_BUSINESS_COMPANY_ADDRESS,
      );
    });

    test('續填位置依身分的步驟順序；個人沒有公司與文件步驟', () {
      expect(
        resumeStepOf(KycSubject.business, [
          KYCField.KYC_FIELD_BANK_ACCOUNT,
          KYCField.KYC_FIELD_BUSINESS_TAX_ID,
        ], []),
        KycStep.company,
      );
      expect(
        resumeStepOf(KycSubject.individual, [KYCField.KYC_FIELD_REAL_NAME], []),
        KycStep.idFront,
      );
      expect(
        resumeStepOf(KycSubject.individual, [], [
          StoreField.STORE_FIELD_ADDRESS,
        ]),
        KycStep.other,
      );
      expect(resumeStepOf(KycSubject.individual, [], []), isNull);
      expect(KycSubject.individual.steps, isNot(contains(KycStep.company)));
      expect(KycSubject.individual.steps, isNot(contains(KycStep.documents)));
      expect(KycSubject.business.steps.last, KycStep.confirm);
    });

    test('可編輯：沒有申請，或伺服器說能重送（can_resubmit）；與流程狀態的值無關', () {
      bool editable(KYCFlowStatus status, {bool canResubmit = false}) =>
          kycEditable(
            individualApplication(status: status, canResubmit: canResubmit),
          );

      expect(kycEditable(null), isTrue);
      expect(
        editable(KYCFlowStatus.KYC_FLOW_STATUS_DRAFT, canResubmit: true),
        isTrue,
      );
      expect(
        editable(KYCFlowStatus.KYC_FLOW_STATUS_REJECTED, canResubmit: true),
        isTrue,
      );
      // 綁定失敗而且賣家能自己修正（F17）：伺服器回 can_resubmit，表單要能編輯。
      expect(
        editable(KYCFlowStatus.KYC_FLOW_STATUS_BIND_FAILED, canResubmit: true),
        isTrue,
      );
      expect(editable(KYCFlowStatus.KYC_FLOW_STATUS_BIND_FAILED), isFalse);
      expect(
        editable(
          KYCFlowStatus.KYC_FLOW_STATUS_PAYUNI_REJECTED,
          canResubmit: true,
        ),
        isTrue,
      );
      expect(editable(KYCFlowStatus.KYC_FLOW_STATUS_PAYUNI_REJECTED), isFalse);
      for (final status in [
        KYCFlowStatus.KYC_FLOW_STATUS_SUBMITTED,
        KYCFlowStatus.KYC_FLOW_STATUS_BINDING,
        KYCFlowStatus.KYC_FLOW_STATUS_BIND_FAILED,
        KYCFlowStatus.KYC_FLOW_STATUS_FORWARDED,
        KYCFlowStatus.KYC_FLOW_STATUS_UNDER_REVIEW,
        KYCFlowStatus.KYC_FLOW_STATUS_PENDING_INFO,
        KYCFlowStatus.KYC_FLOW_STATUS_APPROVED,
        KYCFlowStatus.KYC_FLOW_STATUS_UNSPECIFIED,
      ]) {
        expect(editable(status), isFalse, reason: status.name);
      }
    });
  });

  group('輸入驗證與日期', () {
    test('身分證字號檢查碼', () {
      expect(validateTwId('A123456789'), isNull);
      expect(validateTwId('a123456789'), isNull);
      expect(validateTwId('A123456788'), isNotNull);
      expect(validateTwId('A12'), isNotNull);
      expect(validateTwId(''), isNotNull);
    });

    test('銀行帳號 8 到 16 位數字', () {
      expect(validateBankAccount('12345678'), isNull);
      expect(validateBankAccount('1234567'), isNotNull);
      expect(validateBankAccount('1234-5678'), isNotNull);
    });

    test('民國年與 ISO 日期互轉；不是有效日期回空字串', () {
      expect(rocDateOfIso('2026-06-29'), '115/06/29');
      expect(isoOfRocDate('077/06/10'), '1988-06-10');
      expect(isoOfRocDate('77/06/10'), '1988-06-10');
      expect(isoOfRocDate('115/02/30'), isEmpty);
      expect(isoOfRocDate('not-a-date'), isEmpty);
      expect(rocDateOfIso(''), isEmpty);
      expect(isoOfDateTime(DateTime(2026, 6, 9)), '2026-06-09');
    });
  });
}
