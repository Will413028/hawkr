import 'dart:async';
import 'dart:typed_data';

import 'package:hawkr_api/gen/hawkr/common/v1alpha1/postal_address.pb.dart';
import 'package:hawkr_api/gen/hawkr/identity/v1alpha1/identity.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/identity/v1alpha1/identity.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/identity/v1alpha1/identity.pb.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/gen/google/type/date.pb.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_api/transport.dart';
import 'package:hawkr_seller/core/networking/connect_provider.dart';
import 'package:hawkr_seller/core/session/session_provider.dart';
import 'package:hawkr_seller/features/media/data/private_document_uploader.dart';
import 'package:hawkr_seller/features/seller/data/contact_email.dart';
import 'package:hawkr_seller/features/seller/data/current_kyc.dart';
import 'package:hawkr_seller/features/seller/data/current_store.dart';
import 'package:hawkr_seller/features/seller/data/my_store_repository.dart';
import 'package:hawkr_seller/features/seller/data/seller_agreements_repository.dart';
import 'package:hawkr_seller/features/seller/data/kyc_fields.dart';
import 'package:hawkr_seller/features/seller/data/kyc_form.dart';
import 'package:hawkr_seller/features/seller/data/kyc_wizard.dart';
import 'package:hawkr_seller/features/seller/data/seller_onboarding.dart';
import 'package:hawkr_seller/features/seller/data/version_conflict.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:hawkr_ui_text/hawkr_ui_text.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../../support/connect_errors.dart';
import '../../../support/fake_my_store.dart';
import '../../../support/kyc_fixtures.dart';
import '../../../support/my_store_fixtures.dart';

final _acceptances = [
  AgreementAcceptance(
    kind: AgreementKind.AGREEMENT_KIND_KYC_PRIVACY_NOTICE,
    version: 'p1',
  ),
];

/// 一個假的伺服器：記下每個呼叫的順序與請求，回應由測試設定。
class KycFakeServer {
  final calls = <String>[];
  final saves = <SaveKYCDraftRequest>[];
  final submits = <SubmitKYCRequest>[];
  final cancels = <CancelKYCRequest>[];
  final lookups = <LookupBusinessRegistrationRequest>[];
  final storeUpdates = <UpdateMyStoreRequest>[];

  late KYCApplication Function(SaveKYCDraftRequest) onSave = echo;

  /// 伺服器寫入的結果：請求的欄位併進申請，staged token 換成 asset ID，etag 加一。
  KYCApplication echo(SaveKYCDraftRequest request) {
    final merged = request.application.hasBusiness()
        ? businessApplication()
        : individualApplication();
    merged.mergeFromMessage(request.application);
    merged.etag = 'v${saves.length + 1}';
    KYCDocument confirmed(KYCDocument doc) => KYCDocument(
      assetId: doc.stagedToken.isNotEmpty
          ? 'asset-${doc.stagedToken}'
          : doc.assetId,
      downloadUri: '/dl/${doc.stagedToken}${doc.assetId}',
    );
    if (merged.hasBusiness()) {
      final docs = [
        for (final d in merged.business.companyDocuments) confirmed(d),
      ];
      merged.business.companyDocuments
        ..clear()
        ..addAll(docs);
    }
    return merged;
  }

  KYCApplication Function(SubmitKYCRequest) onSubmit = (_) =>
      individualApplication(
        etag: 'v3',
        kycStatus: KYCStatus.KYC_STATUS_SUBMITTED,
      );
  LookupBusinessRegistrationResponse Function(LookupBusinessRegistrationRequest)
  onLookup = (_) => LookupBusinessRegistrationResponse();

  /// GetMyKYC 的回應（null 是 not_found）；也是版本衝突之後重讀拿到的最新申請。
  KYCApplication? application;
  MyKyc? get latest => application == null ? null : myKycOf(application);
  set latest(MyKyc? value) => application = value?.application;
  Object? reloadFailure;
  final storeServer = FakeMyStoreServer(
    response: myStoreResponse(contactPhone: '02-1234-5678'),
  );
  set store(Store value) => storeServer.response = GetMyStoreResponse(
    store: value,
    capabilities: storeServer.response.capabilities,
  );
  Object? storeFailure;
  bool storeWrittenAtSubmit = false;
  Object? cancelFailure;

  /// 帳號的聯絡信箱（identity 的 GetContactEmail）：預設是 provider 驗證過的信箱；null 是沒有任何信箱。
  ContactEmail? contact = ContactEmail(
    email: 'login@example.com',
    source: ContactEmailSource.CONTACT_EMAIL_SOURCE_VERIFIED,
  );
  Object? setContactFailure;
  Object? contactReadFailure;
  final contactSets = <String>[];
  Object? uploadFailure;
  final agreements = _Agreements();
  final uploads = <String>[];
  Completer<void>? submitGate;
  Completer<void>? saveGate;
  late ProviderContainer container;

  SellerAppServiceClient get client => SellerAppServiceClient(
    FakeTransportBuilder()
        .unary(SellerAppService.saveKYCDraft, (req, _) async {
          await saveGate?.future;
          calls.add('SaveKYCDraft');
          saves.add(req);
          return SaveKYCDraftResponse(application: onSave(req));
        })
        .unary(SellerAppService.getMyKYC, (_, _) {
          calls.add('GetMyKYC');
          final failure = reloadFailure;
          if (failure != null) throw failure;
          final app = application;
          if (app == null) throw ConnectException(Code.notFound, 'none');
          return GetMyKYCResponse(application: app);
        })
        .unary(SellerAppService.submitKYC, (req, _) async {
          await submitGate?.future;
          calls.add('SubmitKYC');
          storeWrittenAtSubmit = storeUpdates.isNotEmpty;
          submits.add(req);
          return SubmitKYCResponse(application: onSubmit(req));
        })
        .unary(SellerAppService.cancelKYC, (req, _) {
          calls.add('CancelKYC');
          cancels.add(req);
          final failure = cancelFailure;
          if (failure != null) throw failure;
          return CancelKYCResponse(application: KYCApplication());
        })
        .unary(SellerAppService.lookupBusinessRegistration, (req, _) {
          calls.add('LookupBusinessRegistration');
          lookups.add(req);
          return onLookup(req);
        })
        .unary(SellerAppService.updateMyStore, (req, _) {
          calls.add('UpdateMyStore');
          final failure = storeFailure;
          if (failure != null) throw failure;
          storeUpdates.add(req);
          final written = storeServer.response.store.deepCopy();
          for (final path in req.updateMask.paths) {
            switch (path) {
              case 'contact_name':
                written.contactName = req.store.contactName;
              case 'contact_mobile':
                written.contactMobile = req.store.contactMobile;
              case 'support_email':
                written.supportEmail = req.store.supportEmail;
              case 'address':
                written.address = req.store.address;
            }
          }
          written.etag = 'store-${storeUpdates.length + 1}';
          store = written;
          return UpdateMyStoreResponse(store: written);
        })
        .build(),
  );

  IdentityAppServiceClient get identity => IdentityAppServiceClient(
    FakeTransportBuilder()
        .unary(IdentityAppService.getContactEmail, (_, _) {
          calls.add('GetContactEmail');
          final failure = contactReadFailure;
          if (failure != null) throw failure;
          final current = contact;
          return GetContactEmailResponse(contactEmail: current);
        })
        .unary(IdentityAppService.setContactEmail, (req, _) {
          calls.add('SetContactEmail');
          final failure = setContactFailure;
          if (failure != null) throw failure;
          contactSets.add(req.email);
          contact = ContactEmail(
            email: req.email,
            source: ContactEmailSource.CONTACT_EMAIL_SOURCE_SELF_ENTERED,
          );
          return SetContactEmailResponse(contactEmail: contact);
        })
        .build(),
  );

  ProviderContainer build() {
    container = ProviderContainer(
      overrides: [
        sessionProvider.overrideWith(_LoggedIn.new),
        sellerClientProvider.overrideWithValue(client),
        identityClientProvider.overrideWithValue(identity),
        myStoreRepositoryProvider.overrideWithValue(storeServer.repository),
        sellerAgreementsRepositoryProvider.overrideWithValue(agreements),
        privateDocumentUploaderProvider.overrideWithValue((
          bytes,
          docType,
          filename,
        ) async {
          final failure = uploadFailure;
          if (failure != null) throw failure;
          uploads.add('$docType:$filename');
          return 'token-${uploads.length}';
        }),
      ],
    );
    return container;
  }

  /// 讀進共用的申請（[kyc] 是伺服器上的狀態），再載入 wizard。
  Future<void> boot({required MyKyc kyc, KycSubject? requested}) async {
    application = kyc.application;
    container.invalidate(accountKycProvider('acc-1'));
    container.invalidate(currentStoreProvider);
    await container.read(accountKycProvider('acc-1').future);
    await container.read(currentStoreProvider.future);
    container.invalidate(accountContactEmailProvider('acc-1'));
    await container
        .read(accountContactEmailProvider('acc-1').future)
        .then((_) {}, onError: (_) {});
    container.read(kycWizardProvider.notifier).hydrate(requested: requested);
    calls.clear();
  }
}

class _LoggedIn extends Session {
  @override
  SessionModel build() =>
      const SessionModel(accountId: 'acc-1', isLoggedIn: true);
}

class _Agreements implements SellerAgreementsRepository {
  int reads = 0;
  bool failNext = false;

  @override
  Future<SellerAgreementSet> load() async {
    reads++;
    if (failNext) {
      failNext = false;
      throw StateError('agreements unavailable');
    }
    return const SellerAgreementSet([]);
  }
}

late KycFakeServer server;

KycWizard get wizard => server.container.read(kycWizardProvider.notifier);

void main() {
  setUp(() {
    server = KycFakeServer();
    final container = server.build();
    // 讓 wizard 在整個測試期間存活（autoDispose）。
    final sub = container.listen(kycWizardProvider, (_, _) {});
    addTearDown(sub.close);
    addTearDown(container.dispose);
  });

  group('載入', () {
    test('not_found 是還沒開始：以錢包選的身分起草，新申請套用店家的預設值', () async {
      server.store = myStore(contactPhone: ' 02-1234-5678 ').store!;
      await server.boot(
        kyc: const MyKyc.notStarted(),
        requested: KycSubject.business,
      );

      expect(wizard.application, isNull);
      expect(wizard.subject, KycSubject.business);
      expect(wizard.form.companyPhone, '02-1234-5678');
      expect(wizard.resumeStep, isNull);
    });

    test('既有申請的身分以申請為準，不理錢包選的身分；欄位也以申請為準', () async {
      await server.boot(
        kyc: myKycOf(
          businessApplication(
            companyName: '範例果園',
            taxId: '22223223',
            missing: [KYCField.KYC_FIELD_BUSINESS_COMPANY_PHONE],
          ),
        ),
        requested: KycSubject.individual,
      );

      expect(wizard.subject, KycSubject.business);
      expect(wizard.form.companyName, '範例果園');
      expect(wizard.form.companyRegisterDate, '2020-01-02');
      expect(wizard.form.companyTownCode, '63000010');
      expect(wizard.form.companyPhone, isEmpty, reason: '新申請以外不套用店家預設值');
      expect(wizard.resumeStep, KycStep.company);
    });

    test('續填的位置是第一個還有缺項的一步；缺店家欄位停在其他資訊；沒有缺項停在確認', () async {
      await server.boot(
        kyc: myKycOf(
          businessApplication(
            missing: [
              KYCField.KYC_FIELD_BUSINESS_COMPANY_DOCUMENTS,
              KYCField.KYC_FIELD_BUSINESS_TAX_ID,
            ],
          ),
        ),
      );
      expect(wizard.resumeStep, KycStep.company);

      await server.boot(
        kyc: myKycOf(
          individualApplication(missingStore: [StoreField.STORE_FIELD_ADDRESS]),
        ),
      );
      expect(wizard.resumeStep, KycStep.other);

      await server.boot(kyc: myKycOf(individualApplication()));
      expect(wizard.resumeStep, KycStep.confirm);
    });

    test('被退回：退回的總理由與伺服器算的待處理問題（欄位用 KYCField）', () async {
      await server.boot(
        kyc: myKycOf(
          businessApplication(
            kycStatus: KYCStatus.KYC_STATUS_REJECTED,
            rejection: KYCRejection(reason: '資料不符'),
            outstanding: [
              KYCIssue(
                field_1: KYCField.KYC_FIELD_BUSINESS_COMPANY_DOCUMENTS,
                documentAssetId: 'asset-doc',
                code: KYCIssueCode.KYC_ISSUE_CODE_UNREADABLE,
                message: '反光看不清楚',
              ),
              KYCIssue(
                field_1: KYCField.KYC_FIELD_BUSINESS_TAX_ID,
                code: KYCIssueCode.KYC_ISSUE_CODE_MISMATCH,
                message: '與登記不符',
              ),
            ],
          ),
        ),
      );

      expect(wizard.editable, isTrue);
      expect(wizard.rejection?.reason, '資料不符');
      expect(
        [
          for (final i in wizard.outstandingIssues)
            '${kycFieldLabel(i.field_1, UiLocale.zhTW)}：${i.message}',
        ],
        ['補充文件：反光看不清楚', '統一編號：與登記不符'],
      );
    });

    test('退回的總理由看伺服器有沒有回 last_rejection：處理完每一項、還沒重送時仍在', () async {
      final rejection = KYCRejection(reason: '舊的退回');
      final issue = KYCIssue(
        field_1: KYCField.KYC_FIELD_BUSINESS_COMPANY_NAME,
        code: KYCIssueCode.KYC_ISSUE_CODE_OTHER,
        message: '名稱不符',
      );

      await server.boot(
        kyc: myKycOf(
          individualApplication(rejection: rejection, outstanding: [issue]),
        ),
      );
      expect(wizard.rejection?.reason, '舊的退回');

      // 逐項都處理完了、還沒重送：總理由仍在，沒有待處理的標記。
      await server.boot(
        kyc: myKycOf(individualApplication(rejection: rejection)),
      );
      expect(wizard.rejection?.reason, '舊的退回');
      expect(wizard.outstandingIssues, isEmpty);

      // 沒有被退回過：伺服器不回 last_rejection。
      await server.boot(kyc: myKycOf(individualApplication()));
      expect(wizard.rejection, isNull);
    });

    test('待處理的問題顯示在對應的欄位與文件旁；伺服器的欄位錯誤優先', () async {
      KYCIssue issue(KYCField field, String message, [String asset = '']) =>
          KYCIssue(
            field_1: field,
            documentAssetId: asset,
            code: KYCIssueCode.KYC_ISSUE_CODE_OTHER,
            message: message,
          );
      await server.boot(
        kyc: myKycOf(
          businessApplication()
            ..business.companyDocuments.addAll([
              KYCDocument(assetId: 'co-1'),
              KYCDocument(assetId: 'co-2'),
            ])
            ..outstandingIssues.addAll([
              issue(KYCField.KYC_FIELD_BUSINESS_TAX_ID, '與登記不符'),
              issue(KYCField.KYC_FIELD_BUSINESS_TAX_ID, '請重新確認'),
              issue(
                KYCField.KYC_FIELD_BUSINESS_COMPANY_DOCUMENTS,
                '已過期',
                'co-2',
              ),
              issue(KYCField.KYC_FIELD_BUSINESS_COMPANY_DOCUMENTS, '請補登記文件'),
            ]),
        ),
      );

      expect(
        wizard.fieldError(KYCField.KYC_FIELD_BUSINESS_TAX_ID),
        '與登記不符；請重新確認',
      );
      expect(
        wizard.fieldError(KYCField.KYC_FIELD_BUSINESS_COMPANY_NAME),
        isNull,
      );
      final docs = wizard.docs.companyDocuments;
      expect(wizard.companyDocumentIssue(docs[0]), isNull);
      expect(wizard.companyDocumentIssue(docs[1]), '已過期');
      expect(wizard.companyDocumentsIssue, '請補登記文件');
    });
  });

  group('建立與存檔', () {
    test('建立草稿是不帶 etag 的 SaveKYCDraft，subject 設定，店家預設值一併送', () async {
      server.store = myStore(contactPhone: '02-1234-5678').store!;
      await server.boot(
        kyc: const MyKyc.notStarted(),
        requested: KycSubject.business,
      );
      server.onSave = (_) => businessApplication(etag: 'v1');

      expect(await wizard.startDraft(KycSubject.business), isTrue);

      final request = server.saves.single;
      expect(request.application.etag, isEmpty);
      expect(request.application.hasBusiness(), isTrue);
      expect(request.updateMask.paths, ['business.company_phone']);
      expect(request.application.business.companyPhone, '02-1234-5678');
      expect(wizard.application?.etag, 'v1');
    });

    test('存檔只送改過的欄位，帶讀到的 etag', () async {
      await server.boot(kyc: myKycOf(individualApplication(etag: 'v1')));
      wizard.updateForm((f) => f.copyWith(productType: '水果'));

      expect(await wizard.save(), isTrue);

      final request = server.saves.single;
      expect(request.application.etag, 'v1');
      expect(request.updateMask.paths, ['product_type']);
      expect(request.application.productType, '水果');
      expect(request.application.hasIndividual(), isFalse);
    });

    test('存檔成功後 etag 換新、已存的欄位不再重送', () async {
      await server.boot(kyc: myKycOf(individualApplication(etag: 'v1')));
      wizard.updateForm((f) => f.copyWith(productType: '水果'));
      await wizard.save();
      server.onSave = (_) => individualApplication(etag: 'v3');
      wizard.updateForm((f) => f.copyWith(productType: '蔬菜'));
      await wizard.save();

      expect(server.saves.last.application.etag, 'v2');
      expect(server.saves.last.updateMask.paths, ['product_type']);
      expect(wizard.application?.etag, 'v3');
    });

    test('伺服器正規化：輸入原樣保留（尾端空白不被吃掉），存完不算未存、不重送', () async {
      await server.boot(kyc: myKycOf(businessApplication(etag: 'v1')));
      server.onSave = (request) {
        final saved = server.echo(request);
        saved.productType = saved.productType.trim();
        if (saved.hasBusiness()) {
          saved.business.companyPhone = saved.business.companyPhone.replaceAll(
            '-',
            '',
          );
        }
        return saved;
      };
      wizard.updateForm(
        (f) =>
            f.copyWith(companyPhone: '02-12345678', productType: 'Wang Farm '),
      );

      expect(await wizard.save(), isTrue);

      expect(wizard.form.companyPhone, '02-12345678');
      expect(wizard.form.productType, 'Wang Farm ', reason: '打字中的尾端空白不能被換掉');
      expect(wizard.hasUnsavedDraft, isFalse);
      final saves = server.saves.length;
      expect(await wizard.save(), isTrue);
      expect(server.saves.length, saves, reason: '正規化後的值不會被當成又改了而重送');
    });

    test('正規化之後送出不會被「未存」擋住', () async {
      await server.boot(kyc: myKycOf(individualApplication(etag: 'v1')));
      server.onSave = (request) {
        final saved = server.echo(request);
        saved.productType = saved.productType.trim();
        return saved;
      };
      wizard.updateForm((f) => f.copyWith(productType: 'Wang Farm '));

      expect(await wizard.submit(_acceptances), KycSubmitOutcome.submitted);
    });

    test('沒送出的欄位原樣保留且仍算未存：打到一半的日期，存別的欄位之後', () async {
      await server.boot(kyc: myKycOf(businessApplication(etag: 'v1')));
      wizard.updateForm(
        (f) =>
            f.copyWith(productType: 'Wang ', companyRegisterDate: '2020-01-3'),
      );

      expect(await wizard.save(), isTrue);

      expect(server.saves.last.updateMask.paths, ['product_type']);
      final state = server.container.read(kycWizardProvider);
      expect(state.form.companyRegisterDate, '2020-01-3');
      expect(
        state.baseline.companyRegisterDate,
        '2020-01-02',
        reason: '沒送出，baseline 不變，仍與輸入不同',
      );
    });

    test('存檔期間又被使用者改過的欄位保留輸入，之後的存檔會送出它', () async {
      await server.boot(kyc: myKycOf(businessApplication(etag: 'v1')));
      final gate = Completer<void>();
      server.saveGate = gate;
      wizard.updateForm(
        (f) => f.copyWith(companyName: '範例果園 ', productType: 'Wang '),
      );

      final saving = wizard.save();
      await Future<void>.delayed(Duration.zero);
      wizard.updateForm((f) => f.copyWith(productType: 'Wang Farm'));
      gate.complete();
      expect(await saving, isTrue);

      expect(wizard.form.productType, 'Wang Farm');
      expect(wizard.form.companyName, '範例果園 ');
      expect(wizard.hasUnsavedDraft, isTrue, reason: '期間改的欄位還沒存');
      server.saveGate = null;
      expect(await wizard.save(), isTrue);
      expect(server.saves.last.updateMask.paths, ['product_type']);
    });

    test('草稿的欄位違規在訊息欄位底下（company_address.town_code）也對到地址欄位', () async {
      await server.boot(kyc: myKycOf(businessApplication(etag: 'v1')));
      wizard.updateForm((f) => f.copyWith(companyAddressDetail: '路二號'));
      server.onSave = (_) => throw fieldError(
        'application.business.company_address.town_code',
        '鄉鎮市區代碼不存在',
      );

      expect(await wizard.save(), isFalse);

      expect(wizard.error, '下列欄位格式不正確：公司地址');
      expect(
        wizard.fieldError(KYCField.KYC_FIELD_BUSINESS_COMPANY_ADDRESS),
        '鄉鎮市區代碼不存在',
      );
    });

    test('寫入的回應直接成為共用的申請（進度卡與閘門不必再讀一次）', () async {
      await server.boot(kyc: myKycOf(individualApplication(etag: 'v1')));
      wizard.updateForm((f) => f.copyWith(productType: '水果'));

      expect(await wizard.save(), isTrue);

      expect(server.calls, ['SaveKYCDraft']);
      final shared = server.container.read(currentKycProvider)?.value;
      expect(shared?.application?.etag, 'v2');
      expect(shared?.application?.productType, '水果');
    });

    test('版本衝突：重讀最新版本、換上新的 etag，保留使用者的輸入', () async {
      await server.boot(
        kyc: myKycOf(businessApplication(etag: 'v1', companyName: '舊名')),
      );
      wizard.updateForm((f) => f.copyWith(companyName: '新名'));
      server.onSave = (_) =>
          throw reasonError(Code.aborted, 'VERSION_CONFLICT');
      final elsewhere = businessApplication(etag: 'v9', companyName: '舊名')
        ..productType = 'Fruit';
      server.latest = myKycOf(elsewhere);

      expect(await wizard.save(), isFalse);

      expect(wizard.form.companyName, '新名', reason: '使用者改過的欄位保留');
      expect(wizard.form.productType, 'Fruit', reason: '別處改過、使用者沒動的欄位換成最新');
      expect(wizard.error, versionConflictNotice('申請資料'));
      expect(wizard.application?.etag, 'v9');

      server.onSave = (_) => businessApplication(etag: 'v10');
      expect(await wizard.save(), isTrue);
      expect(server.saves.last.application.etag, 'v9');
      expect(server.saves.last.updateMask.paths, ['business.company_name']);
    });

    test('版本衝突後重讀失敗：顯示無法載入最新版本，輸入保留', () async {
      await server.boot(kyc: myKycOf(individualApplication()));
      wizard.updateForm((f) => f.copyWith(productType: '水果'));
      server.onSave = (_) =>
          throw reasonError(Code.aborted, 'VERSION_CONFLICT');
      server.reloadFailure = ConnectException(Code.unavailable, 'down');

      expect(await wizard.save(), isFalse);

      expect(wizard.error, versionReloadFailedNotice);
      expect(wizard.form.productType, '水果');
    });

    test('衝突時申請已被取消：回到沒有申請並重設表單與文件（和捨棄草稿相同），之後重建不會帶舊文件', () async {
      await server.boot(
        kyc: myKycOf(
          businessApplication(etag: 'v1')
            ..business.companyDocuments.add(KYCDocument(assetId: 'old')),
        ),
      );
      wizard.updateForm((f) => f.copyWith(productType: '水果'));
      server.onSave = (_) =>
          throw reasonError(Code.aborted, 'VERSION_CONFLICT');
      server.application = null;

      await wizard.save();

      expect(wizard.application, isNull);
      expect(wizard.subject, isNull);
      expect(wizard.error, kycCancelledNotice);
      expect(wizard.form.productType, isEmpty);
      expect(wizard.docs.companyDocuments, isEmpty);

      server.onSave = server.echo;
      expect(await wizard.startDraft(KycSubject.business), isTrue);
      wizard.updateForm((f) => f.copyWith(productType: 'Wang'));
      expect(await wizard.save(), isTrue);
      expect(
        server.saves.last.updateMask.paths,
        isNot(contains('business.company_documents')),
      );
      expect(server.saves.last.updateMask.paths, ['product_type']);
    });

    test('建立時別處已經建立（already_exists）：原樣採用那一份，店家預設值不蓋掉它，回到續填位置', () async {
      server.store = myStore(contactPhone: '02-1234-5678').store!;
      await server.boot(
        kyc: const MyKyc.notStarted(),
        requested: KycSubject.business,
      );
      expect(wizard.form.companyPhone, '02-1234-5678');
      server.onSave = (_) =>
          throw reasonError(Code.alreadyExists, 'VERSION_CONFLICT');
      server.latest = myKycOf(
        businessApplication(
          etag: 'v5',
          missing: [KYCField.KYC_FIELD_BUSINESS_TAX_ID],
        ),
      );

      expect(await wizard.startDraft(KycSubject.business), isFalse);

      expect(wizard.application?.etag, 'v5');
      expect(wizard.form.companyPhone, isEmpty, reason: '既有申請沒填電話，不被店家預設值蓋掉');
      expect(wizard.form.companyName, '');
      expect(wizard.resumeStep, KycStep.company);
      expect(wizard.hasUnsavedDraft, isFalse, reason: '店家預設值沒有被當成待存的修改');
    });

    test('欄位錯誤：列出欄位名稱，欄位的值改過就不再顯示錯誤', () async {
      await server.boot(kyc: myKycOf(individualApplication()));
      wizard.updateForm((f) => f.copyWith(productType: '99'));
      server.onSave = (_) =>
          throw fieldError('application.product_type', '商品類型格式不符');

      expect(await wizard.save(), isFalse);

      expect(wizard.error, '下列欄位格式不正確：主要商品類型');
      expect(wizard.fieldError(KYCField.KYC_FIELD_PRODUCT_TYPE), '商品類型格式不符');
      wizard.updateForm((f) => f.copyWith(productType: '水果'));
      expect(wizard.fieldError(KYCField.KYC_FIELD_PRODUCT_TYPE), isNull);
    });
  });

  group('企業文件', () {
    test('公司文件以 staged token 存進申請，最多 10 張', () async {
      await server.boot(kyc: myKycOf(businessApplication()));

      for (var i = 0; i < 12; i++) {
        await wizard.addCompanyDocument(
          Uint8List.fromList([i]),
          filename: 'd$i.jpg',
        );
      }
      expect(
        wizard.docs.companyDocuments.length,
        lessThanOrEqualTo(KycDocs.maxCompanyDocuments),
      );
      expect(server.uploads.where((u) => u.startsWith('document')).length, 10);
      expect(server.saves.first.updateMask.paths, [
        'business.company_documents',
      ]);
      expect(
        server
            .saves
            .first
            .application
            .business
            .companyDocuments
            .single
            .stagedToken,
        'token-1',
      );
    });

    test('伺服器拒絕文件（staged token 過期）：提示顯示伺服器的訊息與是哪一張，不說格式不正確', () async {
      await server.boot(kyc: myKycOf(businessApplication()));
      server.onSave = (_) => throw fieldErrors({
        'application.business.company_documents[1]': '圖片無法使用，請重新上傳',
        'application.business.tax_id': '統編不合法',
      });

      expect(
        await wizard.addCompanyDocument(
          Uint8List.fromList([1]),
          filename: 'f.jpg',
        ),
        isFalse,
      );

      expect(
        wizard.error,
        '下列欄位格式不正確：統一編號；'
        '補充文件（第 2 張）：圖片無法使用，請重新上傳',
      );
    });

    test('只有文件被拒絕時，提示只有伺服器的訊息', () async {
      await server.boot(kyc: myKycOf(businessApplication()));
      server.onSave = (_) => throw fieldError(
        'application.business.company_documents',
        '圖片無法使用，請重新上傳',
      );

      await wizard.addCompanyDocument(
        Uint8List.fromList([1]),
        filename: 'b.jpg',
      );

      expect(wizard.error, '補充文件：圖片無法使用，請重新上傳');
    });

    test('上傳失敗不存檔，錯誤保留在 error', () async {
      await server.boot(kyc: myKycOf(businessApplication()));
      server.uploadFailure = StateError('offline');

      expect(
        await wizard.addCompanyDocument(Uint8List(1), filename: 'b.jpg'),
        isFalse,
      );
      expect(wizard.error, kycUploadFailedNotice);
      expect(server.saves, isEmpty);
    });
  });

  group('統編查詢', () {
    setUp(() async {
      await server.boot(kyc: myKycOf(businessApplication()));
    });

    test('查到：帶入公司名稱、設立日期與地址，不帶組織型態', () async {
      server.onLookup = (_) => LookupBusinessRegistrationResponse(
        registration: BusinessRegistration(
          taxId: '22223223',
          name: '範例果園有限公司',
          address: '臺北市中正區路一號',
          setupDate: Date(year: 2019, month: 3, day: 4),
          organizationType: '有限公司',
          postalAddress: PostalAddress(
            townCode: '63000010',
            addressDetail: '路一號',
          ),
        ),
      );

      await wizard.lookupTaxId('22223223');

      expect(server.lookups.single.taxId, '22223223');
      expect(wizard.form.companyName, '範例果園有限公司');
      expect(wizard.form.companyRegisterDate, '2019-03-04');
      expect(wizard.form.companyTownCode, '63000010');
      expect(wizard.form.companyAddressDetail, '路一號');
      expect(wizard.lookup, isA<KycLookupFound>());
    });

    test('拆不出鄉鎮市區：原文進地址明細，不動縣市鄉鎮', () async {
      wizard.updateForm((f) => f.copyWith(companyTownCode: '63000020'));
      server.onLookup = (_) => LookupBusinessRegistrationResponse(
        registration: BusinessRegistration(name: '某公司', address: '某地某路一號'),
      );

      await wizard.lookupTaxId('22223223');

      expect(wizard.form.companyTownCode, '63000020');
      expect(wizard.form.companyAddressDetail, '某地某路一號');
    });

    test('查無不是錯誤：不改表單，標示查無', () async {
      final before = wizard.form;

      await wizard.lookupTaxId('22223223');

      expect(wizard.lookup, isA<KycLookupNotFound>());
      expect(wizard.form, before);
      expect(wizard.error, isNull);
    });

    test('資料來源暫時無法使用：退回手動填寫，不改表單', () async {
      final before = wizard.form;
      server.onLookup = (_) => throw ConnectException(Code.unavailable, 'down');

      await wizard.lookupTaxId('22223223');

      expect(wizard.lookup, isA<KycLookupFailed>());
      expect(wizard.form, before);
    });

    test('查詢太頻繁（resource_exhausted）也是失敗，不改表單', () async {
      server.onLookup = (_) =>
          throw ConnectException(Code.resourceExhausted, 'slow down');

      await wizard.lookupTaxId('22223223');

      expect(wizard.lookup, isA<KycLookupFailed>());
    });

    test('不是 8 碼數字就不查', () async {
      await wizard.lookupTaxId('1234');
      expect(server.lookups, isEmpty);
    });
  });

  group('送出', () {
    Future<KycWizard> ready({
      List<StoreField> missingStore = const [],
      List<KYCField> missing = const [],
    }) async {
      await server.boot(
        kyc: myKycOf(
          individualApplication(
            etag: 'v1',
            missing: missing,
            missingStore: missingStore,
          ),
        ),
      );
      return wizard;
    }

    test('先把店家的聯絡資料與地址寫進店家，再直接 SubmitKYC（帶 etag 與同意）', () async {
      await ready(missingStore: [StoreField.STORE_FIELD_CONTACT_NAME]);
      wizard.updateStore(
        (s) => s.copyWith(
          contactName: '王小明',
          contactMobile: '0912345678',
          supportEmail: 'shop@example.com',
          townCode: '63000010',
          addressDetail: '路一號',
        ),
      );
      server.latest = myKycOf(individualApplication(etag: 'v1'));

      expect(await wizard.submit(_acceptances), KycSubmitOutcome.submitted);

      expect(
        server.storeWrittenAtSubmit,
        isTrue,
        reason: '店家資料要在 SubmitKYC 之前寫入',
      );
      expect(
        server.calls.indexOf('UpdateMyStore'),
        lessThan(server.calls.indexOf('SubmitKYC')),
      );
      final update = server.storeUpdates.single;
      expect(update.updateMask.paths, [
        'contact_name',
        'contact_mobile',
        'support_email',
        'address',
      ]);
      expect(update.store.address.townCode, '63000010');
      final submit = server.submits.single;
      expect(submit.etag, 'v1');
      expect(submit.acceptedAgreements, _acceptances);
      expect(wizard.application?.status, KYCStatus.KYC_STATUS_SUBMITTED);
    });

    test('被退回的申請也直接送出，不必先存檔', () async {
      await server.boot(
        kyc: myKycOf(
          individualApplication(
            etag: 'v1',
            kycStatus: KYCStatus.KYC_STATUS_REJECTED,
          ),
        ),
      );

      expect(await wizard.submit(_acceptances), KycSubmitOutcome.submitted);

      expect(server.calls, ['SubmitKYC'], reason: '回應直接成為共用的申請，不再重讀');
      expect(
        server.container.read(currentKycProvider)?.value?.reviewStatus,
        KYCStatus.KYC_STATUS_SUBMITTED,
      );
    });

    test('已核准與審核中的申請不能編輯也不能送出，不呼叫伺服器', () async {
      for (final status in [
        KYCStatus.KYC_STATUS_APPROVED,
        KYCStatus.KYC_STATUS_SUBMITTED,
      ]) {
        await server.boot(
          kyc: myKycOf(individualApplication(kycStatus: status)),
        );

        expect(wizard.editable, isFalse, reason: '$status');
        expect(
          await wizard.submit(_acceptances),
          KycSubmitOutcome.failed,
          reason: '$status',
        );
        expect(server.submits, isEmpty, reason: '$status');
      }
    });

    test('伺服器回報還有缺項：不送出，列出缺項', () async {
      await ready(missing: [KYCField.KYC_FIELD_BUSINESS_COMPANY_DOCUMENTS]);

      expect(await wizard.submit(_acceptances), KycSubmitOutcome.failed);

      expect(wizard.error, '下列欄位尚未填寫：補充文件');
      expect(server.submits, isEmpty);
    });

    test('店家缺項的快取過期（店家資料其實已齊全）：不擋送出，由伺服器判斷', () async {
      await ready(missingStore: [StoreField.STORE_FIELD_ADDRESS]);

      expect(await wizard.submit(_acceptances), KycSubmitOutcome.submitted);

      expect(server.submits, hasLength(1));
    });

    test('店家真的缺項：伺服器回 failed_precondition，重讀後列出缺哪些', () async {
      await ready(missingStore: [StoreField.STORE_FIELD_ADDRESS]);
      server.onSubmit = (_) {
        server.application = individualApplication(
          etag: 'v1',
          missingStore: [StoreField.STORE_FIELD_ADDRESS],
        );
        throw ConnectException(Code.failedPrecondition, '請先到店家設定補齊');
      };

      expect(await wizard.submit(_acceptances), KycSubmitOutcome.failed);

      expect(wizard.error, '下列欄位尚未填寫：店家地址');
    });

    test('聯絡資料格式不合：不寫店家也不送出', () async {
      await ready();
      wizard.updateStore((s) => s.copyWith(contactMobile: '123'));

      expect(await wizard.submit(_acceptances), KycSubmitOutcome.failed);

      expect(wizard.error, kycContactFormatNotice);
      expect(server.calls, isEmpty);
    });

    test('地址只有明細沒有鄉鎮市區：不送出', () async {
      await ready();
      wizard.updateStore((s) => s.copyWith(addressDetail: '路一號'));

      expect(await wizard.submit(_acceptances), KycSubmitOutcome.failed);

      expect(wizard.error, '請選擇縣市與鄉鎮市區');
      expect(server.calls, isEmpty);
    });

    test('AGREEMENT_OUTDATED：重讀條款，提示重新勾選', () async {
      await ready();
      server.onSubmit = (_) =>
          throw reasonError(Code.failedPrecondition, 'AGREEMENT_OUTDATED');

      expect(
        await wizard.submit(_acceptances),
        KycSubmitOutcome.agreementOutdated,
      );

      expect(server.agreements.reads, 1);
      expect(wizard.error, agreementOutdatedMessage);
    });

    test('AGREEMENT_OUTDATED 後重讀條款失敗：無法載入最新版本', () async {
      await ready();
      server.agreements.failNext = true;
      server.onSubmit = (_) =>
          throw reasonError(Code.failedPrecondition, 'AGREEMENT_OUTDATED');

      await wizard.submit(_acceptances);

      expect(wizard.error, versionReloadFailedNotice);
    });

    test('failed_precondition：重讀申請，列出 missing_store_fields', () async {
      await ready();
      server.onSubmit = (_) =>
          throw ConnectException(Code.failedPrecondition, '請先到店家設定補齊');
      server.latest = myKycOf(
        individualApplication(
          etag: 'v1',
          missingStore: [
            StoreField.STORE_FIELD_ADDRESS,
            StoreField.STORE_FIELD_SUPPORT_EMAIL,
          ],
        ),
      );

      expect(await wizard.submit(_acceptances), KycSubmitOutcome.failed);

      expect(wizard.error, '下列欄位尚未填寫：店家地址、客服電子信箱');
      expect(wizard.missingItems.map((i) => i.label), ['店家地址', '客服電子信箱']);
    });

    test('failed_precondition 沒有缺項（例如身分比對）：原樣顯示伺服器的訊息', () async {
      await ready();
      server.onSubmit = (_) =>
          throw ConnectException(Code.failedPrecondition, '身分比對未通過');
      server.latest = myKycOf(individualApplication(etag: 'v1'));

      await wizard.submit(_acceptances);

      expect(wizard.error, isNotEmpty);
      expect(wizard.error, isNot(startsWith('下列欄位')));
    });

    test('帳號有驗證過的信箱：不呼叫 SetContactEmail，也不需要輸入', () async {
      await ready();
      expect(wizard.contactEmailBlocker, isNull);

      expect(await wizard.submit(_acceptances), KycSubmitOutcome.submitted);

      expect(server.contactSets, isEmpty);
      expect(server.calls, isNot(contains('SetContactEmail')));
    });

    test('帳號沒有任何信箱：沒填不送出；填了先 SetContactEmail（去空白、轉小寫）再 SubmitKYC', () async {
      server.contact = null;
      await ready();

      expect(wizard.contactEmailBlocker, '請輸入金流商聯絡信箱');
      expect(await wizard.submit(_acceptances), KycSubmitOutcome.failed);
      expect(wizard.error, '請輸入金流商聯絡信箱');
      expect(server.submits, isEmpty);

      wizard.updateContactEmail('  Shop@Example.COM ');
      expect(wizard.contactEmailBlocker, isNull);
      expect(await wizard.submit(_acceptances), KycSubmitOutcome.submitted);

      expect(server.contactSets, ['shop@example.com']);
      expect(
        server.calls.indexOf('SetContactEmail'),
        lessThan(server.calls.indexOf('SubmitKYC')),
        reason: '信箱要在送出申請之前寫進帳號',
      );
    });

    test('格式不合的信箱不送出、不呼叫 SetContactEmail', () async {
      server.contact = null;
      await ready();
      wizard.updateContactEmail('not-an-email');

      expect(wizard.contactEmailError, 'Email 格式不正確');
      expect(await wizard.submit(_acceptances), KycSubmitOutcome.failed);

      expect(wizard.error, 'Email 格式不正確');
      expect(server.contactSets, isEmpty);
      expect(server.submits, isEmpty);
    });

    test('自填的信箱沒有改：不重複寫入', () async {
      server.contact = ContactEmail(
        email: 'mine@example.com',
        source: ContactEmailSource.CONTACT_EMAIL_SOURCE_SELF_ENTERED,
      );
      await ready();
      expect(wizard.contactEmailValue, 'mine@example.com');

      expect(await wizard.submit(_acceptances), KycSubmitOutcome.submitted);

      expect(server.contactSets, isEmpty);
    });

    test('SetContactEmail 失敗：顯示伺服器的訊息，不送出申請', () async {
      server.contact = null;
      await ready();
      wizard.updateContactEmail('shop@example.com');
      server.setContactFailure = reasonError(
        Code.failedPrecondition,
        'CONTACT_EMAIL_NOT_SETTABLE',
        message: '帳號已有驗證過的信箱',
      );

      expect(await wizard.submit(_acceptances), KycSubmitOutcome.failed);

      expect(wizard.error, '帳號已有驗證過的信箱');
      expect(server.submits, isEmpty);
    });

    test('信箱還沒讀到（讀取失敗）：不能送出', () async {
      server.contactReadFailure = ConnectException(Code.unavailable, 'down');
      await ready();

      expect(wizard.contactEmailBlocker, '載入中');
      expect(await wizard.submit(_acceptances), KycSubmitOutcome.failed);
      expect(server.submits, isEmpty);
    });

    test('SubmitKYC 回 CONTACT_EMAIL_REQUIRED：重讀帳號的信箱，顯示伺服器的訊息', () async {
      await ready();
      server.onSubmit = (_) => throw reasonError(
        Code.failedPrecondition,
        'CONTACT_EMAIL_REQUIRED',
        message: '請先填寫聯絡信箱',
      );
      server.calls.clear();

      expect(await wizard.submit(_acceptances), KycSubmitOutcome.failed);

      expect(wizard.error, '請先填寫聯絡信箱');
      expect(server.calls, contains('GetContactEmail'));
    });

    test('欄位違規：欄位下顯示錯誤，並列出欄位名稱', () async {
      await ready();
      wizard.updateForm((f) => f.copyWith(productType: 'x'));
      server.onSave = (_) =>
          individualApplication(etag: 'v2')..productType = 'x';
      server.onSubmit = (_) => throw fieldError('product_type', '請填寫主要商品類型');

      expect(await wizard.submit(_acceptances), KycSubmitOutcome.failed);

      expect(wizard.error, '下列欄位尚未填寫：主要商品類型');
      expect(wizard.fieldError(KYCField.KYC_FIELD_PRODUCT_TYPE), '請填寫主要商品類型');
    });

    test('版本衝突：重讀最新版本，輸入保留，提示重新送出', () async {
      await ready();
      server.onSubmit = (_) =>
          throw reasonError(Code.aborted, 'VERSION_CONFLICT');
      server.latest = myKycOf(individualApplication(etag: 'v8'));

      expect(await wizard.submit(_acceptances), KycSubmitOutcome.failed);

      expect(wizard.application?.etag, 'v8');
      expect(wizard.error, versionConflictNotice('申請資料'));
    });

    test('店家寫入版本衝突：不送出，提示店家資料已更新，輸入保留', () async {
      await ready();
      wizard.updateStore((s) => s.copyWith(contactName: '王小明'));
      server.storeFailure = reasonError(Code.aborted, 'VERSION_CONFLICT');

      expect(await wizard.submit(_acceptances), KycSubmitOutcome.failed);

      expect(wizard.error, versionConflictNotice('店家資料'));
      expect(wizard.store.contactName, '王小明');
      expect(server.submits, isEmpty);
    });

    test('店家寫入的欄位錯誤：顯示在聯絡欄位', () async {
      await ready();
      wizard.updateStore((s) => s.copyWith(contactName: '王小明'));
      server.storeFailure = fieldError('store.contact_name', '姓名不可包含符號');

      expect(await wizard.submit(_acceptances), KycSubmitOutcome.failed);

      expect(wizard.contactErrors.name, '姓名不可包含符號');
      expect(server.submits, isEmpty);
    });
  });

  group('捨棄草稿', () {
    test('CancelKYC 帶 etag，成功後回到沒有申請、可重新選身分', () async {
      server.store = myStore(contactPhone: '02-1234-5678').store!;
      await server.boot(kyc: myKycOf(individualApplication(etag: 'v4')));
      wizard.updateForm((f) => f.copyWith(productType: '水果'));

      expect(await wizard.discard(), isTrue);

      expect(server.cancels.single.etag, 'v4');
      expect(wizard.application, isNull);
      expect(wizard.subject, isNull);
      expect(wizard.form.productType, isEmpty);
      expect(wizard.form.companyPhone, '02-1234-5678');
    });

    test('不是草稿就不能捨棄', () async {
      await server.boot(
        kyc: myKycOf(
          individualApplication(kycStatus: KYCStatus.KYC_STATUS_REJECTED),
        ),
      );
      expect(wizard.canDiscard, isFalse);

      await server.boot(kyc: myKycOf(individualApplication()));
      expect(wizard.canDiscard, isTrue);
    });

    test('捨棄時版本衝突：重讀最新版本，申請還在，可以再試', () async {
      await server.boot(kyc: myKycOf(individualApplication(etag: 'v4')));
      server.cancelFailure = reasonError(Code.aborted, 'VERSION_CONFLICT');
      server.latest = myKycOf(individualApplication(etag: 'v6'));

      expect(await wizard.discard(), isFalse);

      expect(wizard.application?.etag, 'v6');
      expect(wizard.error, versionConflictNotice('申請資料'));

      server.cancelFailure = null;
      expect(await wizard.discard(), isTrue);
      expect(server.cancels.last.etag, 'v6');
    });
  });
}
