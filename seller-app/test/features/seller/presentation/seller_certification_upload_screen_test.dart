import 'dart:async';

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
import 'package:hawkr_seller/features/seller/data/current_kyc.dart';
import 'package:hawkr_seller/features/seller/data/current_store.dart';
import 'package:hawkr_seller/features/seller/data/kyc_form.dart';
import 'package:hawkr_seller/features/seller/data/my_store_repository.dart';
import 'package:hawkr_seller/features/seller/data/seller_agreements_repository.dart';
import 'package:hawkr_seller/features/seller/data/seller_onboarding.dart';
import 'package:hawkr_seller/features/seller/data/version_conflict.dart';
import 'package:hawkr_seller/features/seller/presentation/kyc_wizard_widgets.dart';
import 'package:hawkr_seller/features/seller/presentation/seller_certification_upload_screen.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

import '../../../support/connect_errors.dart';
import '../../../support/fake_my_store.dart';
import '../../../support/kyc_fixtures.dart';
import '../../../support/my_store_fixtures.dart';
import '../../../support/onboarding_fixtures.dart';

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
    return SellerAgreementSet([
      SellerAgreement(
        kind: AgreementKind.AGREEMENT_KIND_KYC_PRIVACY_NOTICE,
        version: 'p$reads',
        url: 'https://www.example.test/privacy',
      ),
    ]);
  }
}

/// 假的伺服器：[application] 是 GetMyKYC 回的申請（null 是 not_found）。
class _Server {
  KYCApplication? application;
  final saves = <SaveKYCDraftRequest>[];
  final submits = <SubmitKYCRequest>[];
  final cancels = <CancelKYCRequest>[];
  final lookups = <String>[];
  KYCApplication Function(SaveKYCDraftRequest)? onSave;
  KYCApplication Function(SubmitKYCRequest)? onSubmit;
  LookupBusinessRegistrationResponse Function(String)? onLookup;
  Completer<void>? submitGate;
  Object? kycFailure;

  /// GetMyPaymentAccount 的回應：[PaymentAccount]，或 null（not_found）。
  PaymentAccount? payment;

  /// GetContactEmail 的回應：預設是 provider 驗證過的信箱；null 是沒有任何信箱。
  ContactEmail? contact = ContactEmail(
    email: 'login@example.com',
    source: ContactEmailSource.CONTACT_EMAIL_SOURCE_VERIFIED,
  );
  final contactSets = <String>[];

  IdentityAppServiceClient get identity => IdentityAppServiceClient(
    FakeTransportBuilder()
        .unary(IdentityAppService.getContactEmail, (_, _) {
          return GetContactEmailResponse(contactEmail: contact);
        })
        .unary(IdentityAppService.setContactEmail, (req, _) {
          contactSets.add(req.email);
          contact = ContactEmail(
            email: req.email,
            source: ContactEmailSource.CONTACT_EMAIL_SOURCE_SELF_ENTERED,
          );
          return SetContactEmailResponse(contactEmail: contact);
        })
        .build(),
  );

  SellerAppServiceClient get client => SellerAppServiceClient(
    FakeTransportBuilder()
        .unary(SellerAppService.getMyKYC, (_, _) {
          final failure = kycFailure;
          if (failure != null) throw failure;
          final app = application;
          if (app == null) throw ConnectException(Code.notFound, 'none');
          return GetMyKYCResponse(application: app);
        })
        .unary(
          SellerAppService.getMyOnboarding,
          (_, _) => application?.status == KYCStatus.KYC_STATUS_APPROVED
              ? paymentPendingSummary()
              : reviewPendingSummary(),
        )
        .unary(SellerAppService.saveKYCDraft, (req, _) {
          saves.add(req);
          final result =
              onSave?.call(req) ??
              (req.application.hasBusiness()
                  ? businessApplication(etag: 'v${saves.length}')
                  : individualApplication(etag: 'v${saves.length}'));
          application = result;
          return SaveKYCDraftResponse(application: result);
        })
        .unary(SellerAppService.submitKYC, (req, _) async {
          await submitGate?.future;
          submits.add(req);
          final result =
              onSubmit?.call(req) ??
              individualApplication(
                etag: 'done',
                kycStatus: KYCStatus.KYC_STATUS_SUBMITTED,
              );
          application = result;
          return SubmitKYCResponse(application: result);
        })
        .unary(SellerAppService.cancelKYC, (req, _) {
          cancels.add(req);
          application = null;
          return CancelKYCResponse(application: KYCApplication());
        })
        .unary(SellerAppService.getMyPaymentAccount, (_, _) {
          final account = payment;
          if (account == null) throw ConnectException(Code.notFound, 'none');
          return GetMyPaymentAccountResponse(account: account);
        })
        .unary(SellerAppService.lookupBusinessRegistration, (req, _) {
          lookups.add(req.taxId);
          return onLookup?.call(req.taxId) ??
              LookupBusinessRegistrationResponse();
        })
        .build(),
  );
}

void main() {
  late _Server server;
  late _Agreements agreements;
  late GoRouter router;
  late ProviderContainer container;

  Future<void> pumpScreen(
    WidgetTester tester, {
    String? sellerType = 'individual',
    KYCApplication? application,
    KYCApplication? staleCache,
    Object? rereadFailure,
  }) async {
    server.application = staleCache ?? application;
    final store = FakeMyStoreServer(
      response: myStoreResponse(contactPhone: '02-1234-5678'),
    );
    container = ProviderContainer(
      overrides: [
        sessionProvider.overrideWith(_LoggedIn.new),
        sellerClientProvider.overrideWithValue(server.client),
        identityClientProvider.overrideWithValue(server.identity),
        myStoreRepositoryProvider.overrideWithValue(store.repository),
        sellerAgreementsRepositoryProvider.overrideWithValue(agreements),
      ],
    );
    addTearDown(container.dispose);
    final sub = container.listen(currentStoreProvider, (_, _) {});
    addTearDown(sub.close);
    await container.read(currentStoreProvider.future);
    if (staleCache != null) {
      await container.read(accountKycProvider('acc-1').future);
      server.application = application;
    }
    server.kycFailure = rereadFailure;

    router = GoRouter(
      routes: [
        GoRoute(
          path: '/',
          builder: (_, _) => const Scaffold(body: Text('WALLET')),
        ),
        GoRoute(
          path: '/kyc',
          builder: (_, _) =>
              SellerCertificationUploadScreen(sellerType: sellerType),
        ),
      ],
    );
    addTearDown(router.dispose);
    tester.view.physicalSize = const Size(800, 3000);
    tester.view.devicePixelRatio = 2;
    addTearDown(tester.view.reset);
    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: ScreenUtilInit(
          designSize: const Size(393, 852),
          builder: (_, _) => MaterialApp.router(routerConfig: router),
        ),
      ),
    );
    router.push('/kyc');
    await tester.pumpAndSettle();
  }

  Finder field(String key) => find.descendant(
    of: find.byKey(Key(key)),
    matching: find.byType(TextField),
  );

  String fieldText(WidgetTester tester, String key) =>
      tester.widget<TextField>(field(key)).controller!.text;

  Future<void> pumpMore(WidgetTester tester, [int ms = 600]) async {
    await tester.pump();
    await tester.pump(Duration(milliseconds: ms));
    await tester.pumpAndSettle();
  }

  Future<void> tapText(WidgetTester tester, String text) async {
    await tester.ensureVisible(find.text(text));
    await tester.tap(find.text(text));
    await tester.pumpAndSettle();
  }

  Future<void> tapKey(WidgetTester tester, String key) async {
    await tester.ensureVisible(find.byKey(Key(key)));
    await tester.tap(find.byKey(Key(key)));
    await tester.pumpAndSettle();
  }

  setUp(() {
    server = _Server();
    agreements = _Agreements();
  });

  group('載入', () {
    testWidgets('沒有申請：依錢包選的身分建立草稿（不帶 etag），停在第一步', (tester) async {
      await pumpScreen(tester, sellerType: 'business');

      expect(server.saves.single.application.etag, isEmpty);
      expect(server.saves.single.application.hasBusiness(), isTrue);
      expect(find.text('公司資訊'), findsWidgets);
      expect(find.text('資料步驟 1 / 4'), findsOneWidget);
    });

    testWidgets('已有申請：身分以申請為準，不理錢包選的身分，也不再建立', (tester) async {
      await pumpScreen(
        tester,
        sellerType: 'individual',
        application: businessApplication(
          missing: [KYCField.KYC_FIELD_BUSINESS_TAX_ID],
        ),
      );

      expect(server.saves, isEmpty);
      expect(find.text('公司資訊'), findsWidgets);
    });

    testWidgets('被退回：顯示退回原因與要修正的欄位，停在第一個缺項的一步', (tester) async {
      await pumpScreen(
        tester,
        application: businessApplication(
          kycStatus: KYCStatus.KYC_STATUS_REJECTED,
          missing: [KYCField.KYC_FIELD_BUSINESS_TAX_ID],
          rejection: KYCRejection(reason: '資料不符'),
          outstanding: [
            KYCIssue(
              field_1: KYCField.KYC_FIELD_BUSINESS_TAX_ID,
              code: KYCIssueCode.KYC_ISSUE_CODE_MISMATCH,
              message: '與登記不符',
            ),
          ],
        ),
      );

      expect(find.text('需修改'), findsOneWidget);
      expect(find.text('資料不符'), findsOneWidget);
      expect(find.text('統一編號：與登記不符'), findsOneWidget);
      expect(find.text('公司資訊'), findsWidgets);
    });

    testWidgets('退回指出的補充文件問題顯示在那張文件旁，說明來自伺服器', (tester) async {
      await pumpScreen(
        tester,
        application: businessApplication(
          kycStatus: KYCStatus.KYC_STATUS_REJECTED,
          missing: [KYCField.KYC_FIELD_BUSINESS_COMPANY_DOCUMENTS],
          rejection: KYCRejection(reason: '文件過期'),
          outstanding: [
            KYCIssue(
              field_1: KYCField.KYC_FIELD_BUSINESS_COMPANY_DOCUMENTS,
              documentAssetId: 'co-1',
              code: KYCIssueCode.KYC_ISSUE_CODE_OTHER,
              message: '已過期',
            ),
          ],
        )..business.companyDocuments.add(KYCDocument(assetId: 'co-1')),
      );

      expect(
        find.descendant(
          of: find.byKey(const Key('kyc_company_document_0')),
          matching: find.text('已過期'),
        ),
        findsOneWidget,
      );
      expect(find.text('補充文件：已過期'), findsOneWidget);
    });

    testWidgets('審核中的申請是只讀的兩區狀態頁：平台審核區顯示申請狀態，收款帳戶區依開通摘要還沒開放', (tester) async {
      await pumpScreen(
        tester,
        application: individualApplication(
          kycStatus: KYCStatus.KYC_STATUS_SUBMITTED,
        ),
      );

      expect(find.byKey(const Key('kyc_locked')), findsOneWidget);
      expect(
        find.descendant(
          of: find.byKey(const Key('kyc_platform_review')),
          matching: find.text('審核中'),
        ),
        findsOneWidget,
      );
      expect(
        find.descendant(
          of: find.byKey(const Key('kyc_payment_account')),
          matching: find.text('平台審核通過後開放設定'),
        ),
        findsOneWidget,
      );
      expect(find.byType(TextField), findsNothing);
    });

    testWidgets('已核准的申請唯讀：沒有表單欄位，也沒有送出入口', (tester) async {
      await pumpScreen(
        tester,
        application: individualApplication(
          kycStatus: KYCStatus.KYC_STATUS_APPROVED,
        ),
      );

      expect(find.byKey(const Key('kyc_locked')), findsOneWidget);
      expect(find.byType(TextField), findsNothing);
      expect(find.text('送出認證'), findsNothing);
      expect(find.text('下一步'), findsNothing);
      expect(find.byKey(const Key('kyc_discard')), findsNothing);
      expect(server.saves, isEmpty);
      expect(server.submits, isEmpty);
    });

    testWidgets('審核中沒有送出入口與捨棄草稿', (tester) async {
      await pumpScreen(
        tester,
        application: individualApplication(
          kycStatus: KYCStatus.KYC_STATUS_SUBMITTED,
        ),
      );

      expect(find.text('送出認證'), findsNothing);
      expect(find.byKey(const Key('kyc_discard')), findsNothing);
    });

    testWidgets('被退回：顯示退回框', (tester) async {
      await pumpScreen(
        tester,
        application: individualApplication(
          kycStatus: KYCStatus.KYC_STATUS_REJECTED,
          rejection: KYCRejection(reason: '資料不符'),
        ),
      );
      expect(find.text('資料不符'), findsOneWidget);
      expect(
        find.descendant(
          of: find.byKey(const Key('kyc_rejection')),
          matching: find.text('請查看退件原因並編輯後重新送出。'),
        ),
        findsOneWidget,
      );
      expect(find.byKey(const Key('kyc_locked')), findsNothing);
    });
  });

  group('存檔', () {
    testWidgets('下一步存進伺服器（只送改過的欄位、帶 etag），再前進', (tester) async {
      await pumpScreen(
        tester,
        application: businessApplication(
          etag: 'v1',
          missing: [KYCField.KYC_FIELD_BUSINESS_COMPANY_PHONE],
        ),
      );
      server.onSave = (req) => businessApplication(etag: 'v2');

      await tester.enterText(field('kyc_company_name'), '範例果園');
      await tapText(tester, '下一步');

      expect(server.saves.single.application.etag, 'v1');
      expect(server.saves.single.updateMask.paths, ['business.company_name']);
      expect(find.text('商家營運資訊'), findsOneWidget);
    });

    testWidgets('版本衝突：停在原地、顯示更新提示，輸入保留', (tester) async {
      await pumpScreen(
        tester,
        application: businessApplication(
          etag: 'v1',
          missing: [KYCField.KYC_FIELD_BUSINESS_COMPANY_PHONE],
        ),
      );
      server.onSave = (_) =>
          throw reasonError(Code.aborted, 'VERSION_CONFLICT');
      server.application = businessApplication(etag: 'v9');

      await tester.enterText(field('kyc_company_name'), '範例果園');
      await tapText(tester, '下一步');

      expect(find.text(versionConflictNotice('申請資料')), findsOneWidget);
      expect(fieldText(tester, 'kyc_company_name'), '範例果園');
      expect(find.text('公司資訊'), findsWidgets);
    });
  });

  group('統編查詢', () {
    testWidgets('輸入 8 碼自動查：帶入公司名稱，不帶組織型態', (tester) async {
      await pumpScreen(
        tester,
        application: businessApplication(
          missing: [KYCField.KYC_FIELD_BUSINESS_TAX_ID],
        ),
      );
      server.onLookup = (_) => LookupBusinessRegistrationResponse(
        registration: BusinessRegistration(
          name: '範例果園有限公司',
          organizationType: '有限公司',
          setupDate: Date(year: 2019, month: 3, day: 4),
          postalAddress: PostalAddress(
            townCode: '63000010',
            addressDetail: '路一號',
          ),
        ),
      );

      await tester.enterText(field('kyc_tax_id'), '22223223');
      await pumpMore(tester);

      expect(server.lookups, ['22223223']);
      expect(fieldText(tester, 'kyc_company_name'), '範例果園有限公司');
      expect(find.text('查得 範例果園有限公司，登記日 108/03/04'), findsOneWidget);
      expect(find.text('有限公司'), findsNothing);
    });

    testWidgets('查無：顯示查無提示，欄位留給手填', (tester) async {
      await pumpScreen(
        tester,
        application: businessApplication(
          missing: [KYCField.KYC_FIELD_BUSINESS_TAX_ID],
        ),
      );

      await tester.enterText(field('kyc_tax_id'), '22223223');
      await pumpMore(tester);

      expect(find.text('Hawkr 系統查無此統編資料，請手動填入公司資訊。'), findsOneWidget);
      expect(fieldText(tester, 'kyc_company_name'), isEmpty);
    });

    testWidgets('資料來源暫時無法使用：退回手動填寫，不擋流程', (tester) async {
      await pumpScreen(
        tester,
        application: businessApplication(
          missing: [KYCField.KYC_FIELD_BUSINESS_TAX_ID],
        ),
      );
      server.onLookup = (_) => throw ConnectException(Code.unavailable, 'down');

      await tester.enterText(field('kyc_tax_id'), '22223223');
      await pumpMore(tester);

      expect(find.byKey(const Key('kyc_tax_id_notice')), findsOneWidget);
      expect(find.byKey(const Key('kyc_tax_id_lookup')), findsOneWidget);
      await tester.enterText(field('kyc_company_name'), '手填公司');
      expect(fieldText(tester, 'kyc_company_name'), '手填公司');
    });
  });

  group('確認送出', () {
    KYCApplication complete() => individualApplication(etag: 'v1');

    Future<void> checkNotice(WidgetTester tester) async {
      await tapKey(tester, 'kyc_privacy_checkbox');
    }

    testWidgets('沒有缺項時停在確認；告知事項勾了才能送出，送出帶 etag 與現行版本，只帶個資告知', (
      tester,
    ) async {
      await pumpScreen(tester, application: complete());

      expect(find.text('確認送出'), findsWidgets);
      KycPrimaryButton submitButton() => tester.widget<KycPrimaryButton>(
        find.widgetWithText(KycPrimaryButton, '送出認證'),
      );
      expect(submitButton().onTap, isNull);

      await checkNotice(tester);
      await tapText(tester, '送出認證');

      final request = server.submits.single;
      expect(request.etag, 'v1');
      expect(request.acceptedAgreements.map((a) => a.kind), [
        AgreementKind.AGREEMENT_KIND_KYC_PRIVACY_NOTICE,
      ]);
      expect(request.acceptedAgreements.map((a) => a.version), ['p1']);
      expect(find.byKey(const Key('kyc_payuni_checkbox')), findsNothing);
      expect(find.text('已送出申請'), findsOneWidget);
      expect(find.textContaining('PayUni'), findsNothing);
      await tester.tap(find.text('我知道了'));
      await tester.pumpAndSettle();
      expect(find.text('WALLET'), findsOneWidget);
    });

    testWidgets('帳號有驗證過的信箱：確認頁顯示信箱，沒有輸入框，送出不呼叫 SetContactEmail', (
      tester,
    ) async {
      await pumpScreen(tester, application: complete());

      expect(
        find.byKey(const Key('kyc_contact_email_verified')),
        findsOneWidget,
      );
      expect(find.text('login@example.com'), findsOneWidget);
      expect(find.byKey(const Key('kyc_contact_email')), findsNothing);

      await checkNotice(tester);
      await tapText(tester, '送出認證');

      expect(server.contactSets, isEmpty);
      expect(server.submits, hasLength(1));
    });

    testWidgets('帳號沒有任何信箱：確認頁有輸入框，填了才能送出，送出前先寫進帳號', (tester) async {
      server.contact = null;
      await pumpScreen(tester, application: complete());

      expect(find.byKey(const Key('kyc_contact_email_verified')), findsNothing);
      expect(field('kyc_contact_email'), findsOneWidget);
      await checkNotice(tester);
      KycPrimaryButton submitButton() => tester.widget<KycPrimaryButton>(
        find.widgetWithText(KycPrimaryButton, '送出認證'),
      );
      expect(submitButton().onTap, isNull, reason: '信箱還沒填');

      await tester.enterText(field('kyc_contact_email'), 'Shop@Example.com');
      await tester.pumpAndSettle();
      expect(submitButton().onTap, isNotNull);
      await tapText(tester, '送出認證');

      expect(server.contactSets, ['shop@example.com']);
      expect(server.submits, hasLength(1));
    });

    testWidgets('AGREEMENT_OUTDATED：重讀條款、取消勾選，提示重新勾選', (tester) async {
      await pumpScreen(tester, application: complete());
      server.onSubmit = (_) =>
          throw reasonError(Code.failedPrecondition, 'AGREEMENT_OUTDATED');

      await checkNotice(tester);
      await tapText(tester, '送出認證');

      expect(find.text(agreementOutdatedMessage), findsOneWidget);
      expect(agreements.reads, 2);
      for (final key in const ['kyc_privacy_checkbox']) {
        expect(
          tester.widget<Checkbox>(find.byKey(Key(key))).value,
          isFalse,
          reason: key,
        );
      }
    });

    testWidgets('AGREEMENT_OUTDATED 後重讀條款失敗：顯示無法載入最新版本', (tester) async {
      await pumpScreen(tester, application: complete());
      server.onSubmit = (_) {
        agreements.failNext = true;
        throw reasonError(Code.failedPrecondition, 'AGREEMENT_OUTDATED');
      };

      await checkNotice(tester);
      await tapText(tester, '送出認證');

      expect(find.text(versionReloadFailedNotice), findsOneWidget);
    });

    testWidgets('failed_precondition：重讀申請，列出 missing_store_fields', (
      tester,
    ) async {
      await pumpScreen(tester, application: complete());
      server.onSubmit = (_) {
        server.application = individualApplication(
          etag: 'v1',
          missingStore: [
            StoreField.STORE_FIELD_ADDRESS,
            StoreField.STORE_FIELD_CONTACT_NAME,
          ],
        );
        throw ConnectException(Code.failedPrecondition, '請先到店家設定補齊');
      };

      await checkNotice(tester);
      await tapText(tester, '送出認證');

      expect(find.text('下列欄位尚未填寫：店家地址、聯絡人姓名'), findsWidgets);
      expect(find.byKey(const Key('kyc_missing_list')), findsOneWidget);
    });

    testWidgets('欄位違規：顯示欄位名稱', (tester) async {
      await pumpScreen(tester, application: complete());
      server.onSubmit = (_) => throw fieldError('product_type', '請填寫主要商品類型');

      await checkNotice(tester);
      await tapText(tester, '送出認證');

      expect(find.text('下列欄位尚未填寫：主要商品類型'), findsOneWidget);
      expect(server.submits, hasLength(1));
    });
  });

  group('捨棄草稿', () {
    testWidgets('確認後 CancelKYC（帶 etag），回到選擇商家類型，可換身分重新開始', (tester) async {
      await pumpScreen(
        tester,
        application: individualApplication(
          etag: 'v4',
          missingStore: [StoreField.STORE_FIELD_ADDRESS],
        ),
      );

      await tapKey(tester, 'kyc_discard');
      expect(find.text('確定要捨棄目前的認證草稿？已上傳資料會清除。'), findsOneWidget);
      await tester.tap(find.text('捨棄草稿').last);
      await tester.pumpAndSettle();

      expect(server.cancels.single.etag, 'v4');
      expect(find.text('選擇商家類型'), findsOneWidget);

      server.onSave = (req) => businessApplication(etag: 'new');
      await tapKey(tester, 'kyc_subject_business');
      await tapText(tester, '開始填寫');

      expect(server.saves.last.application.hasBusiness(), isTrue);
      expect(find.text('公司資訊'), findsWidgets);
    });

    testWidgets('取消確認就不送出', (tester) async {
      await pumpScreen(
        tester,
        application: individualApplication(
          missingStore: [StoreField.STORE_FIELD_ADDRESS],
        ),
      );

      await tapKey(tester, 'kyc_discard');
      await tester.tap(find.text('取消'));
      await tester.pumpAndSettle();

      expect(server.cancels, isEmpty);
    });

    testWidgets('被退回的申請沒有捨棄草稿', (tester) async {
      await pumpScreen(
        tester,
        application: individualApplication(
          kycStatus: KYCStatus.KYC_STATUS_REJECTED,
          missingStore: [StoreField.STORE_FIELD_ADDRESS],
        ),
      );

      expect(find.byKey(const Key('kyc_discard')), findsNothing);
    });
  });

  group('路由提示與欄位', () {
    testWidgets('沒帶 sellerType 且沒有申請：顯示選擇商家類型，不自動建立草稿', (tester) async {
      await pumpScreen(tester, sellerType: null);

      expect(find.text('選擇商家類型'), findsOneWidget);
      expect(server.saves, isEmpty);
    });

    testWidgets('沒帶 sellerType 但已有申請：以申請為準', (tester) async {
      await pumpScreen(
        tester,
        sellerType: null,
        application: businessApplication(
          missing: [KYCField.KYC_FIELD_BUSINESS_TAX_ID],
        ),
      );

      expect(find.text('公司資訊'), findsWidgets);
      expect(server.saves, isEmpty);
    });

    testWidgets('主要產品類型是自由文字，只在企業的公司資訊顯示', (tester) async {
      await pumpScreen(
        tester,
        application: businessApplication(
          missing: [KYCField.KYC_FIELD_BUSINESS_TAX_ID],
        ),
      );

      expect(field('kyc_product_type'), findsOneWidget);
      expect(find.text('例如：水果 / 蔬菜 / 海鮮 / 加工食品'), findsOneWidget);
    });

    testWidgets('個人沒有主要產品類型欄位', (tester) async {
      await pumpScreen(
        tester,
        application: individualApplication(
          missingStore: [StoreField.STORE_FIELD_ADDRESS],
        ),
      );

      expect(find.text('商家營運資訊'), findsOneWidget);
      expect(find.byKey(const Key('kyc_product_type')), findsNothing);
    });
  });

  group('送出前提醒與離開', () {
    testWidgets('確認頁顯示「身分與銀行由金流商收集」，沒有店名鎖定提醒', (tester) async {
      await pumpScreen(tester, application: individualApplication(etag: 'v1'));
      expect(find.text('身分與銀行帳號會在審核通過後，由金流商收集，平台不經手。'), findsOneWidget);
      expect(find.byKey(const Key('kyc_lock_on_submit_notice')), findsNothing);
      expect(find.textContaining('無法修改'), findsNothing);
      expect(find.textContaining('PayUni'), findsNothing);
    });

    testWidgets('送出進行中不能返回；完成後才離開', (tester) async {
      await pumpScreen(tester, application: individualApplication(etag: 'v1'));
      await tapKey(tester, 'kyc_privacy_checkbox');
      server.submitGate = Completer<void>();

      await tester.ensureVisible(find.text('送出認證'));
      await tester.tap(find.text('送出認證'));
      await tester.pump();
      await tester.binding.handlePopRoute();
      await tester.pump();

      expect(find.text('確認送出'), findsWidgets);
      expect(find.text('WALLET'), findsNothing);

      server.submitGate!.complete();
      await tester.pumpAndSettle();
      expect(find.text('已送出申請'), findsOneWidget);
      await tester.tap(find.text('我知道了'));
      await tester.pumpAndSettle();
      expect(find.text('WALLET'), findsOneWidget);
    });
  });

  group('進入與重設', () {
    testWidgets('進入時重讀申請：過期快取裡的店家缺項不再顯示', (tester) async {
      await pumpScreen(
        tester,
        staleCache: individualApplication(
          etag: 'v1',
          missingStore: [StoreField.STORE_FIELD_ADDRESS],
        ),
        application: individualApplication(etag: 'v1'),
      );

      expect(find.text('確認送出'), findsWidgets);
      expect(find.byKey(const Key('kyc_missing_list')), findsNothing);
    });

    testWidgets('進入時重讀失敗：顯示重試，不用快取裡過期的申請載入；重試成功才載入', (tester) async {
      await pumpScreen(
        tester,
        staleCache: individualApplication(
          etag: 'stale',
          missingStore: [StoreField.STORE_FIELD_ADDRESS],
        ),
        application: individualApplication(etag: 'fresh'),
        rereadFailure: ConnectException(Code.unavailable, 'down'),
      );

      expect(find.text('無法載入認證資料'), findsOneWidget);
      expect(find.text('商家營運資訊'), findsNothing);
      expect(find.text('確認送出'), findsNothing);

      server.kycFailure = null;
      await tapText(tester, '重試');

      expect(find.text('無法載入認證資料'), findsNothing);
      expect(find.text('確認送出'), findsWidgets);
    });

    testWidgets('送出時衝突發現申請已取消：回到選擇身分，重建後從第一步開始、勾選取消', (tester) async {
      await pumpScreen(tester, application: individualApplication(etag: 'v1'));
      await tapKey(tester, 'kyc_privacy_checkbox');
      server.onSubmit = (_) {
        server.application = null;
        throw reasonError(Code.aborted, 'VERSION_CONFLICT');
      };

      await tapText(tester, '送出認證');

      expect(find.text('選擇商家類型'), findsOneWidget);
      expect(find.text('認證申請已取消，請重新開始。'), findsOneWidget);

      server.onSave = (_) => individualApplication(etag: 'new');
      await tapKey(tester, 'kyc_subject_individual');
      await tapText(tester, '開始填寫');

      expect(find.text('商家營運資訊'), findsOneWidget);
      expect(find.text('確認送出'), findsNothing);
    });

    testWidgets('取消之後再回到確認頁，勾選是取消的', (tester) async {
      await pumpScreen(tester, application: individualApplication(etag: 'v1'));
      await tapKey(tester, 'kyc_privacy_checkbox');
      server.onSubmit = (_) {
        server.application = null;
        throw reasonError(Code.aborted, 'VERSION_CONFLICT');
      };
      await tapText(tester, '送出認證');

      server.onSave = (_) {
        server.application = individualApplication(etag: 'other');
        throw reasonError(Code.alreadyExists, 'VERSION_CONFLICT');
      };
      await tapKey(tester, 'kyc_subject_individual');
      await tapText(tester, '開始填寫');

      expect(find.text('確認送出'), findsWidgets);
      for (final key in const ['kyc_privacy_checkbox']) {
        expect(tester.widget<Checkbox>(find.byKey(Key(key))).value, isFalse);
      }
    });
  });

  group('日期', () {
    test('民國年顯示；不是有效日期回空字串', () {
      expect(rocDateOfIso('2026-06-29'), '115/06/29');
      expect(rocDateOfIso(''), isEmpty);
    });
  });
}
