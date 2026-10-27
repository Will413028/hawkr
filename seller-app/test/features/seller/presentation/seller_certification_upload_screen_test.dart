import 'dart:async';

import 'package:hawkr_api/gen/hawkr/common/v1alpha1/postal_address.pb.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/gen/google/rpc/error_details.pb.dart'
    show LocalizedMessage;
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
import 'package:hawkr_seller/features/seller/presentation/profile_form_widgets.dart';
import 'package:hawkr_seller/features/seller/presentation/seller_certification_upload_screen.dart';
import 'package:hawkr_seller/features/seller/presentation/widgets/payuni_status_body.dart'
    show payuniReasonLocale;
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

import '../../../support/connect_errors.dart';
import '../../../support/fake_my_store.dart';
import '../../../support/kyc_fixtures.dart';
import '../../../support/my_store_fixtures.dart';

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
      SellerAgreement(
        kind: AgreementKind.AGREEMENT_KIND_PAYUNI_TERMS,
        version: 'u$reads',
        url: 'https://www.example.test/payuni',
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

  SellerAppServiceClient get client => SellerAppServiceClient(
    FakeTransportBuilder()
        .unary(SellerAppService.getMyKYC, (_, _) {
          final failure = kycFailure;
          if (failure != null) throw failure;
          final app = application;
          if (app == null) throw ConnectException(Code.notFound, 'none');
          return GetMyKYCResponse(application: app);
        })
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
                status: KYCFlowStatus.KYC_FLOW_STATUS_SUBMITTED,
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
      expect(find.text('資料步驟 1 / 9'), findsOneWidget);
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
        application: individualApplication(
          status: KYCFlowStatus.KYC_FLOW_STATUS_REJECTED,
          kycStatus: KYCStatus.KYC_STATUS_REJECTED,
          missing: [KYCField.KYC_FIELD_BANK_ACCOUNT],
          rejection: KYCRejection(reason: '證件模糊'),
          outstanding: [
            KYCIssue(
              field_1: KYCField.KYC_FIELD_ID_CARD_FRONT,
              code: KYCIssueCode.KYC_ISSUE_CODE_UNREADABLE,
              message: '反光看不清楚',
            ),
            KYCIssue(
              field_1: KYCField.KYC_FIELD_BANK_ACCOUNT,
              code: KYCIssueCode.KYC_ISSUE_CODE_MISMATCH,
              message: '與存摺不符',
            ),
          ],
        ),
      );

      expect(find.text('認證未通過'), findsOneWidget);
      expect(find.text('證件模糊'), findsOneWidget);
      expect(find.text('身分證正面照：反光看不清楚'), findsOneWidget);
      expect(find.text('銀行帳號：與存摺不符'), findsOneWidget);
      expect(find.text('收款帳戶'), findsOneWidget);
    });

    testWidgets('退回指出的證件問題顯示在那張證件旁，說明來自伺服器', (tester) async {
      await pumpScreen(
        tester,
        application: individualApplication(
          status: KYCFlowStatus.KYC_FLOW_STATUS_REJECTED,
          kycStatus: KYCStatus.KYC_STATUS_REJECTED,
          missing: [KYCField.KYC_FIELD_ID_CARD_FRONT],
          rejection: KYCRejection(reason: '證件模糊'),
          outstanding: [
            KYCIssue(
              field_1: KYCField.KYC_FIELD_ID_CARD_FRONT,
              code: KYCIssueCode.KYC_ISSUE_CODE_UNREADABLE,
              message: '正面反光看不清楚',
            ),
          ],
        ),
      );

      expect(
        find.descendant(
          of: find.byKey(const Key('kyc_id_front')),
          matching: find.text('正面反光看不清楚'),
        ),
        findsOneWidget,
      );
      expect(find.text('身分證正面照：正面反光看不清楚'), findsOneWidget);
    });

    testWidgets('審核中的申請是只讀的狀態頁，沒有任何欄位', (tester) async {
      await pumpScreen(
        tester,
        application: individualApplication(
          status: KYCFlowStatus.KYC_FLOW_STATUS_SUBMITTED,
          kycStatus: KYCStatus.KYC_STATUS_SUBMITTED,
        ),
      );

      expect(find.byKey(const Key('kyc_locked')), findsOneWidget);
      expect(find.text('平台審核中'), findsOneWidget);
      expect(find.byType(TextField), findsNothing);
    });

    testWidgets('綁定失敗、賣家能修正（can_resubmit）：表單可編輯，有 PayUni 問題框，沒有鎖住畫面', (
      tester,
    ) async {
      await pumpScreen(
        tester,
        application: individualApplication(
          status: KYCFlowStatus.KYC_FLOW_STATUS_BIND_FAILED,
          kycStatus: KYCStatus.KYC_STATUS_APPROVED,
          canResubmit: true,
          missing: [KYCField.KYC_FIELD_ID_CARD_FRONT],
          payuni: SellerPayUni(
            state: PayUniState.PAY_UNI_STATE_FIXABLE,
            explanation: LocalizedMessage(
              locale: 'zh-TW',
              message: '伺服器說明-綁定失敗',
            ),
            fixFields: [KYCField.KYC_FIELD_REAL_NAME],
          ),
        ),
      );

      expect(find.byKey(const Key('kyc_locked')), findsNothing);
      expect(find.byKey(const Key('kyc_payuni_problem')), findsOneWidget);
      expect(find.byKey(const Key('kyc_id_front')), findsOneWidget);
    });

    testWidgets('PayUni 拒絕、可修正：表單上方有 PayUni 問題框，沒有退回框', (tester) async {
      await pumpScreen(
        tester,
        application: individualApplication(
          status: KYCFlowStatus.KYC_FLOW_STATUS_PAYUNI_REJECTED,
          kycStatus: KYCStatus.KYC_STATUS_DRAFT,
          canResubmit: true,
          payuni: SellerPayUni(
            state: PayUniState.PAY_UNI_STATE_FIXABLE,
            explanation: LocalizedMessage(
              locale: 'zh-TW',
              message: '伺服器說明-FIXABLE',
            ),
            reason: '資料不符',
            fixFields: [KYCField.KYC_FIELD_REAL_NAME],
          ),
        ),
      );

      final box = find.byKey(const Key('kyc_payuni_problem'));
      expect(box, findsOneWidget);
      expect(
        find.descendant(of: box, matching: find.text('PayUni 需要補件')),
        findsOneWidget,
      );
      expect(
        find.descendant(of: box, matching: find.text('伺服器說明-FIXABLE')),
        findsOneWidget,
      );
      expect(tester.widget<Text>(find.text('資料不符')).locale, payuniReasonLocale);
      expect(find.text('認證未通過'), findsNothing);
    });

    testWidgets('草稿且有 payuni（賣家正在修正）：問題框仍在', (tester) async {
      await pumpScreen(
        tester,
        application: individualApplication(
          status: KYCFlowStatus.KYC_FLOW_STATUS_DRAFT,
          kycStatus: KYCStatus.KYC_STATUS_DRAFT,
          canResubmit: true,
          payuni: SellerPayUni(state: PayUniState.PAY_UNI_STATE_FIXABLE),
        ),
      );
      expect(find.byKey(const Key('kyc_payuni_problem')), findsOneWidget);
    });

    testWidgets('被退回：只有退回框，沒有 PayUni 問題框', (tester) async {
      await pumpScreen(
        tester,
        application: individualApplication(
          status: KYCFlowStatus.KYC_FLOW_STATUS_REJECTED,
          kycStatus: KYCStatus.KYC_STATUS_REJECTED,
          rejection: KYCRejection(reason: '證件模糊'),
        ),
      );
      expect(find.text('證件模糊'), findsOneWidget);
      expect(find.byKey(const Key('kyc_payuni_problem')), findsNothing);
    });

    testWidgets('鎖住畫面：有 PayUni 狀態時顯示 PayUni 標題、說明與審核意見；送審中是流程橫幅', (
      tester,
    ) async {
      await pumpScreen(
        tester,
        application: individualApplication(
          status: KYCFlowStatus.KYC_FLOW_STATUS_PAYUNI_REJECTED,
          kycStatus: KYCStatus.KYC_STATUS_APPROVED,
          payuni: SellerPayUni(
            state: PayUniState.PAY_UNI_STATE_CONTACT_SUPPORT,
            explanation: LocalizedMessage(
              locale: 'zh-TW',
              message: '伺服器說明-CONTACT',
            ),
            reason: '會員已停權',
          ),
        ),
      );
      expect(find.byKey(const Key('kyc_locked')), findsOneWidget);
      expect(find.text('PayUni 申請未通過'), findsOneWidget);
      expect(find.text('伺服器說明-CONTACT'), findsOneWidget);
      expect(
        tester.widget<Text>(find.text('會員已停權')).locale,
        payuniReasonLocale,
      );
      expect(find.byType(TextField), findsNothing);
    });

    testWidgets('證號只有遮蔽值：顯示遮蔽值，沒有重新輸入就不送', (tester) async {
      await pumpScreen(
        tester,
        application: individualApplication(
          realName: '王小明',
          idMasked: 'A12****789',
          missing: [KYCField.KYC_FIELD_REAL_NAME],
        ),
      );
      expect(find.text('已填寫（A12****789），如需修改請重新輸入'), findsOneWidget);
      expect(fieldText(tester, 'kyc_id_card_no'), isEmpty);
      expect(server.saves, isEmpty);
    });
  });

  group('存檔', () {
    testWidgets('下一步存進伺服器（只送改過的欄位、帶 etag），再前進', (tester) async {
      await pumpScreen(
        tester,
        application: individualApplication(
          etag: 'v1',
          missing: [KYCField.KYC_FIELD_BANK_ACCOUNT],
        ),
      );
      server.onSave = (req) =>
          individualApplication(etag: 'v2')
            ..bankName = req.application.bankName;

      await tester.enterText(field('kyc_bank_name'), '台北富邦銀行');
      await tapText(tester, '下一步');

      expect(server.saves.single.application.etag, 'v1');
      expect(server.saves.single.updateMask.paths, ['bank_name']);
      expect(find.text('商家營運資訊'), findsOneWidget);
    });

    testWidgets('版本衝突：停在原地、顯示更新提示，輸入保留', (tester) async {
      await pumpScreen(
        tester,
        application: individualApplication(
          etag: 'v1',
          missing: [KYCField.KYC_FIELD_BANK_ACCOUNT],
        ),
      );
      server.onSave = (_) =>
          throw reasonError(Code.aborted, 'VERSION_CONFLICT');
      server.application = individualApplication(etag: 'v9');

      await tester.enterText(field('kyc_bank_name'), '台北富邦銀行');
      await tapText(tester, '下一步');

      expect(find.text(versionConflictNotice('申請資料')), findsOneWidget);
      expect(fieldText(tester, 'kyc_bank_name'), '台北富邦銀行');
      expect(find.text('收款帳戶'), findsOneWidget);
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

    Future<void> checkBoth(WidgetTester tester) async {
      await tapKey(tester, 'kyc_privacy_checkbox');
      await tapKey(tester, 'kyc_payuni_checkbox');
    }

    testWidgets('沒有缺項時停在確認；兩個勾選都勾了才能送出，送出帶 etag 與兩份現行版本', (tester) async {
      await pumpScreen(tester, application: complete());

      expect(find.text('確認送出'), findsWidgets);
      KycPrimaryButton submitButton() => tester.widget<KycPrimaryButton>(
        find.widgetWithText(KycPrimaryButton, '送出認證'),
      );
      expect(submitButton().onTap, isNull);

      await checkBoth(tester);
      await tapText(tester, '送出認證');

      final request = server.submits.single;
      expect(request.etag, 'v1');
      expect(request.acceptedAgreements.map((a) => a.kind), [
        AgreementKind.AGREEMENT_KIND_KYC_PRIVACY_NOTICE,
        AgreementKind.AGREEMENT_KIND_PAYUNI_TERMS,
      ]);
      expect(request.acceptedAgreements.map((a) => a.version), ['p1', 'u1']);
      expect(find.text('已送出申請'), findsOneWidget);
      expect(find.text('送出後 PayUni 預設密碼為你的身分證字號'), findsOneWidget);
      await tester.tap(find.text('我知道了'));
      await tester.pumpAndSettle();
      expect(find.text('WALLET'), findsOneWidget);
    });

    testWidgets('AGREEMENT_OUTDATED：重讀條款、取消勾選，提示重新勾選', (tester) async {
      await pumpScreen(tester, application: complete());
      server.onSubmit = (_) =>
          throw reasonError(Code.failedPrecondition, 'AGREEMENT_OUTDATED');

      await checkBoth(tester);
      await tapText(tester, '送出認證');

      expect(find.text(agreementOutdatedMessage), findsOneWidget);
      expect(agreements.reads, 2);
      for (final key in const ['kyc_privacy_checkbox', 'kyc_payuni_checkbox']) {
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

      await checkBoth(tester);
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

      await checkBoth(tester);
      await tapText(tester, '送出認證');

      expect(find.text('下列欄位尚未填寫：店家地址、聯絡人姓名'), findsWidgets);
      expect(find.byKey(const Key('kyc_missing_list')), findsOneWidget);
    });

    testWidgets('欄位違規：顯示欄位名稱', (tester) async {
      await pumpScreen(tester, application: complete());
      server.onSubmit = (_) => throw fieldError('english_name', '請填寫商店英文名稱');

      await checkBoth(tester);
      await tapText(tester, '送出認證');

      expect(find.text('下列欄位尚未填寫：英文姓名'), findsOneWidget);
      expect(server.submits, hasLength(1));
    });
  });

  group('捨棄草稿', () {
    testWidgets('確認後 CancelKYC（帶 etag），回到選擇商家類型，可換身分重新開始', (tester) async {
      await pumpScreen(
        tester,
        application: individualApplication(
          etag: 'v4',
          missing: [KYCField.KYC_FIELD_ID_CARD_FRONT],
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
          missing: [KYCField.KYC_FIELD_ID_CARD_FRONT],
        ),
      );

      await tapKey(tester, 'kyc_discard');
      await tester.tap(find.text('取消'));
      await tester.pumpAndSettle();

      expect(server.cancels, isEmpty);
    });

    testWidgets('身分已被 PayUni 鎖定時沒有捨棄草稿', (tester) async {
      await pumpScreen(
        tester,
        application: individualApplication(
          status: KYCFlowStatus.KYC_FLOW_STATUS_REJECTED,
          kycStatus: KYCStatus.KYC_STATUS_REJECTED,
          identityLocked: true,
          missing: [KYCField.KYC_FIELD_ID_CARD_FRONT],
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

    testWidgets('發證地點是縣市選單（選項同 website 的縣市清單），可選', (tester) async {
      await pumpScreen(
        tester,
        application: individualApplication(
          missing: [KYCField.KYC_FIELD_ID_CARD_ISSUE_LOCATION],
        ),
      );
      expect(find.byKey(const Key('kyc_issue_location')), findsOneWidget);
      expect(field('kyc_issue_location'), findsNothing);

      await tapKey(tester, 'kyc_issue_location');
      expect(find.text('台北市'), findsOneWidget);
      expect(find.text('新北市'), findsOneWidget);
      await tester.tap(find.text('台北市'));
      await tester.pumpAndSettle();
      await tapText(tester, '下一步');

      expect(server.saves.single.application.idCardIssueLocation, '台北市');
    });
  });

  group('送出前提醒與離開', () {
    testWidgets('確認頁在「PayUni 預設密碼」旁顯示鎖定提醒；身分欄位已鎖住後不顯示', (tester) async {
      await pumpScreen(tester, application: individualApplication(etag: 'v1'));
      expect(find.text(kPayuniLockOnSubmitNotice), findsOneWidget);
      expect(kPayuniLockOnSubmitNotice, '送出後商店名稱與商店介紹將無法修改，請先確認無誤');
      expect(find.text('送出後 PayUni 預設密碼為你的身分證字號'), findsOneWidget);
    });

    testWidgets('identity_locked 時不顯示鎖定提醒', (tester) async {
      await pumpScreen(
        tester,
        application: individualApplication(
          etag: 'v1',
          status: KYCFlowStatus.KYC_FLOW_STATUS_REJECTED,
          kycStatus: KYCStatus.KYC_STATUS_REJECTED,
          identityLocked: true,
        ),
      );

      expect(find.text('送出後 PayUni 預設密碼為你的身分證字號'), findsOneWidget);
      expect(find.text(kPayuniLockOnSubmitNotice), findsNothing);
    });

    testWidgets('送出進行中不能返回；完成後才離開', (tester) async {
      await pumpScreen(tester, application: individualApplication(etag: 'v1'));
      await tapKey(tester, 'kyc_privacy_checkbox');
      await tapKey(tester, 'kyc_payuni_checkbox');
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
          missing: [KYCField.KYC_FIELD_BANK_ACCOUNT],
        ),
        application: individualApplication(etag: 'fresh'),
        rereadFailure: ConnectException(Code.unavailable, 'down'),
      );

      expect(find.text('無法載入認證資料'), findsOneWidget);
      expect(find.text('收款帳戶'), findsNothing);
      expect(find.text('確認送出'), findsNothing);

      server.kycFailure = null;
      await tapText(tester, '重試');

      expect(find.text('無法載入認證資料'), findsNothing);
      expect(find.text('確認送出'), findsWidgets);
    });

    testWidgets('送出時衝突發現申請已取消：回到選擇身分，重建後從第一步開始、勾選取消', (tester) async {
      await pumpScreen(tester, application: individualApplication(etag: 'v1'));
      await tapKey(tester, 'kyc_privacy_checkbox');
      await tapKey(tester, 'kyc_payuni_checkbox');
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

      expect(find.text('上傳身分證'), findsOneWidget);
      expect(find.text('確認送出'), findsNothing);
    });

    testWidgets('取消之後再回到確認頁，兩個勾選是取消的', (tester) async {
      await pumpScreen(tester, application: individualApplication(etag: 'v1'));
      await tapKey(tester, 'kyc_privacy_checkbox');
      await tapKey(tester, 'kyc_payuni_checkbox');
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
      for (final key in const ['kyc_privacy_checkbox', 'kyc_payuni_checkbox']) {
        expect(tester.widget<Checkbox>(find.byKey(Key(key))).value, isFalse);
      }
    });
  });

  group('日期', () {
    test('民國年與 ISO 日期互轉；不是有效日期回空字串', () {
      expect(rocDateOfIso('2026-06-29'), '115/06/29');
      expect(isoOfRocDate('077/06/10'), '1988-06-10');
      expect(isoOfRocDate('115/02/30'), isEmpty);
      expect(isoOfRocDate('not-a-date'), isEmpty);
      expect(rocDateOfIso(''), isEmpty);
    });
  });
}
