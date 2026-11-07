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
        form: const KycForm(productType: '水果'),
      )!;

      expect(request.application.etag, isEmpty);
      expect(request.application.hasIndividual(), isTrue);
      expect(request.updateMask.paths, ['product_type']);
    });

    test('建立：沒有任何值也要建（subject 設定）', () {
      final request = _build(etag: null, subject: KycSubject.business)!;

      expect(request.application.hasBusiness(), isTrue);
      expect(request.updateMask.paths, isEmpty);
    });

    test('更新只列改過的欄位；沒碰到那一支專屬欄位就不帶 subject', () {
      final request = _build(
        form: const KycForm(productType: '水果'),
        baseline: const KycForm(productType: '蔬菜'),
      )!;

      expect(request.updateMask.paths, ['product_type']);
      expect(request.application.hasIndividual(), isFalse);
      expect(request.application.hasBusiness(), isFalse);
    });

    test('日期是 google.type.Date；清空日期是列進 mask 但不帶值', () {
      final set = _build(
        subject: KycSubject.business,
        form: const KycForm(companyRegisterDate: '2020-01-02'),
      )!;
      expect(
        set.application.business.companyRegisterDate,
        Date(year: 2020, month: 1, day: 2),
      );

      final cleared = _build(
        subject: KycSubject.business,
        form: const KycForm(),
        baseline: const KycForm(companyRegisterDate: '2020-01-02'),
      )!;
      expect(cleared.updateMask.paths, ['business.company_register_date']);
      expect(cleared.application.business.hasCompanyRegisterDate(), isFalse);
    });

    test('無效的日期不送', () {
      expect(
        _build(
          subject: KycSubject.business,
          form: const KycForm(companyRegisterDate: '2020-02-30'),
        ),
        isNull,
      );
    });

    test('企業：統編、公司名稱、電話、設立日期、地址在 business 底下', () {
      final request = _build(
        subject: KycSubject.business,
        form: const KycForm(
          taxId: '22223223',
          companyName: '範例果園',
          companyPhone: '02-1234',
          companyRegisterDate: '2020-01-02',
          companyTownCode: '63000010',
          companyAddressDetail: '路一號',
        ),
      )!;

      expect(request.updateMask.paths, [
        'business.company_name',
        'business.tax_id',
        'business.company_phone',
        'business.company_register_date',
        'business.company_address',
      ]);
      final business = request.application.business;
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

    test('平台不收身分、銀行與證件：請求的 update_mask 不會出現這些路徑', () {
      final request = _build(
        subject: KycSubject.business,
        etag: null,
        form: const KycForm(
          productType: 'a',
          taxId: 'b',
          companyName: 'c',
          companyPhone: 'd',
          companyRegisterDate: '2020-01-02',
          companyTownCode: 'e',
          companyAddressDetail: 'f',
        ),
        docs: const KycDocs(companyDocuments: [KycDoc(stagedToken: 't')]),
      )!;
      for (final path in request.updateMask.paths) {
        expect(
          path,
          isNot(
            anyOf(
              contains('real_name'),
              contains('english_name'),
              contains('id_card'),
              contains('bank'),
              contains('birth_date'),
            ),
          ),
        );
      }
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
    test('身分以 subject 為準；沒有申請或 subject 沒設是 null', () {
      expect(kycSubjectOf(individualApplication()), KycSubject.individual);
      expect(kycSubjectOf(businessApplication()), KycSubject.business);
      expect(kycSubjectOf(KYCApplication()), isNull);
      expect(kycSubjectOf(null), isNull);
    });

    test('企業文件只認有 asset ID 的，帶下載路徑', () {
      final app = businessApplication()
        ..business.companyDocuments.addAll([
          KYCDocument(assetId: 'a1', downloadUri: '/dl/a1'),
          KYCDocument(),
        ]);
      final docs = kycDocsOf(app);
      expect(docs.companyDocuments, hasLength(1));
      expect(docs.companyDocuments.single.downloadUri, contains('a1'));
    });
  });

  group('版本衝突的合併', () {
    test('使用者改過的欄位保留，沒改的換成最新', () {
      const old = KycForm(companyName: '舊', productType: '');
      final merged = mergeLatestForm(
        const KycForm(companyName: '我改的'),
        old,
        const KycForm(companyName: '別處改的', productType: 'Latest'),
      );

      expect(merged.companyName, '我改的');
      expect(merged.productType, 'Latest');
    });

    test('企業文件：換過的清單保留，沒換的換成最新', () {
      const mine = KycDoc(stagedToken: 't');
      final kept = mergeLatestDocs(
        const KycDocs(companyDocuments: [mine]),
        const KycDocs(),
        const KycDocs(companyDocuments: [KycDoc(assetId: 'x')]),
      );
      expect(kept.companyDocuments, [mine]);

      final latest = mergeLatestDocs(
        const KycDocs(),
        const KycDocs(),
        const KycDocs(companyDocuments: [KycDoc(assetId: 'x')]),
      );
      expect(latest.companyDocuments.single.assetId, 'x');
    });
  });

  group('baselineAfterSave（以送出的值為基準，與伺服器的正規化無關）', () {
    test('送出的欄位的 baseline 是送出的值：正規化後不會被當成又改了而重送', () {
      const sent = KycForm(
        productType: 'Fruit ',
        companyPhone: '02-12345678',
        companyRegisterDate: '2020-01-02',
        companyTownCode: '63000010',
        companyAddressDetail: '路一號',
      );

      final baseline = baselineAfterSave(const KycForm(), sent, [
        'product_type',
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
      const sent = KycForm(
        productType: 'Fruit ',
        companyRegisterDate: '2020-01-3',
      );

      final baseline = baselineAfterSave(const KycForm(), sent, [
        'product_type',
      ]);

      expect(baseline.productType, 'Fruit ');
      expect(baseline.companyRegisterDate, isEmpty);
      expect(baseline.companyRegisterDate, isNot(sent.companyRegisterDate));
    });

    test('存檔期間又被改過的欄位仍與 baseline 不同，下次存檔會送', () {
      const sent = KycForm(productType: 'Fruit ');
      const current = KycForm(productType: 'Fruit Farm');

      final baseline = baselineAfterSave(const KycForm(), sent, [
        'product_type',
      ]);
      final request = _build(form: current, baseline: baseline);

      expect(request?.updateMask.paths, ['product_type']);
    });
  });

  group('逐欄位重建不丟欄位', () {
    const full = KycForm(
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
  });

  group('續填、退回、可編輯', () {
    // 平台只收市集登記資料（D22）：身分、銀行與證件欄位不在表單裡，沒有所在的一步。
    const supported = {
      KYCField.KYC_FIELD_PRODUCT_TYPE,
      KYCField.KYC_FIELD_WEBSITE_URL,
      KYCField.KYC_FIELD_BUSINESS_COMPANY_NAME,
      KYCField.KYC_FIELD_BUSINESS_TAX_ID,
      KYCField.KYC_FIELD_BUSINESS_COMPANY_REGISTER_DATE,
      KYCField.KYC_FIELD_BUSINESS_COMPANY_PHONE,
      KYCField.KYC_FIELD_BUSINESS_COMPANY_DOCUMENTS,
      KYCField.KYC_FIELD_BUSINESS_COMPANY_ADDRESS,
    };

    test('表單上的欄位有名稱與所在的一步；身分、銀行、證件欄位沒有，不認得的值回 null', () {
      for (final field in KYCField.values) {
        if (supported.contains(field)) {
          expect(
            kycFieldLabel(field, UiLocale.zhTW),
            isNotNull,
            reason: field.name,
          );
          expect(kycStepOfField(field), isNotNull, reason: field.name);
        } else {
          expect(
            kycFieldLabel(field, UiLocale.zhTW),
            isNull,
            reason: field.name,
          );
          expect(kycStepOfField(field), isNull, reason: field.name);
        }
      }
    });

    test('缺項略過不在表單上的欄位（不認得的值不列）', () {
      final items = missingItemsOf([
        KYCField.KYC_FIELD_BANK_CODE,
        KYCField.KYC_FIELD_REAL_NAME,
        KYCField.KYC_FIELD_BUSINESS_TAX_ID,
        KYCField.KYC_FIELD_UNSPECIFIED,
      ], UiLocale.zhTW);
      expect(items.map((i) => i.label), ['統一編號']);
    });

    test('表單標籤與缺項清單同名（共用文案目錄的 KYCField 名詞）', () {
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

    test('步驟：個人兩步（營運資訊、確認），企業四步；最後一步一定是確認', () {
      expect(KycSubject.individual.steps, [KycStep.other, KycStep.confirm]);
      expect(KycSubject.business.steps, [
        KycStep.company,
        KycStep.other,
        KycStep.documents,
        KycStep.confirm,
      ]);
    });

    test('續填位置依身分的步驟順序；個人沒有公司與文件步驟', () {
      expect(
        resumeStepOf(KycSubject.business, [
          KYCField.KYC_FIELD_BUSINESS_COMPANY_DOCUMENTS,
          KYCField.KYC_FIELD_BUSINESS_TAX_ID,
        ], []),
        KycStep.company,
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
      expect(editable(KYCFlowStatus.KYC_FLOW_STATUS_SUBMITTED), isFalse);
      expect(editable(KYCFlowStatus.KYC_FLOW_STATUS_APPROVED), isFalse);
      expect(editable(KYCFlowStatus.KYC_FLOW_STATUS_UNSPECIFIED), isFalse);
    });
  });

  group('日期', () {
    test('民國年顯示；不是有效日期回空字串', () {
      expect(rocDateOfIso('2026-06-29'), '115/06/29');
      expect(rocDateOfIso(''), isEmpty);
      expect(isoOfDateTime(DateTime(2026, 6, 9)), '2026-06-09');
    });
  });
}
