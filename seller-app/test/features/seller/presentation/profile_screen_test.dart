import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_api/transport.dart';
import 'package:hawkr_seller/core/refresh/refresh_clock.dart';
import 'package:hawkr_seller/core/session/session_provider.dart';
import 'package:hawkr_seller/features/notification/data/notification_model.dart';
import 'package:hawkr_seller/features/notification/data/notification_repository.dart';
import 'package:hawkr_seller/features/seller/data/current_store.dart';
import 'package:hawkr_seller/features/seller/data/current_kyc.dart';
import 'package:hawkr_seller/features/seller/data/onboarding_tasks.dart';
import 'package:hawkr_seller/features/seller/data/my_store_repository.dart';
import 'package:hawkr_seller/features/seller/data/store_closing.dart';
import 'package:hawkr_seller/features/seller/data/store_editor.dart';
import 'package:hawkr_seller/features/seller/presentation/profile_form_widgets.dart';
import 'package:hawkr_seller/features/seller/presentation/profile_screen.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

import '../../../support/connect_errors.dart';
import '../../../support/fake_my_store.dart';
import '../../../support/my_store_fixtures.dart';
import '../../../support/default_error_text.dart';

class _FakeKyc extends AccountKyc {
  _FakeKyc(this.kyc);

  final MyKyc kyc;

  @override
  Future<MyKyc> build(String accountId) async => kyc;
}

class _LoggedIn extends Session {
  @override
  SessionModel build() =>
      const SessionModel(accountId: 'acc-1', isLoggedIn: true);
}

class _Notifications implements NotificationRepository {
  @override
  Future<UnreadCount> getUnreadCount() async => const UnreadCount();

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

void main() {
  late FakeMyStoreServer server;
  late List<UpdateMyStoreRequest> updates;
  late int closes;
  Object? updateFailure;

  Future<ProviderContainer> pump(
    WidgetTester tester, {
    bool identityLocked = false,
    KYCFlowStatus kycStatus = KYCFlowStatus.KYC_FLOW_STATUS_NOT_STARTED,
    GetMyStoreResponse? response,
    GoRouter? router,
    ValueGetter<int>? onInputsBuilt,
    ValueGetter<DateTime>? clock,
  }) async {
    updates = [];
    closes = 0;
    server = FakeMyStoreServer(response: response);
    final container = ProviderContainer(
      overrides: [
        if (clock != null) refreshClockProvider.overrideWithValue(clock),
        sessionProvider.overrideWith(_LoggedIn.new),
        myStoreRepositoryProvider.overrideWithValue(server.repository),
        accountKycProvider('acc-1').overrideWith(
          () => _FakeKyc(
            MyKyc(status: kycStatus, identityLocked: identityLocked),
          ),
        ),
        onboardingInputsProvider.overrideWith((ref) async {
          onInputsBuilt?.call();
          return const OnboardingInputs();
        }),
        notificationRepositoryProvider.overrideWithValue(_Notifications()),
        storeClosingProvider.overrideWith(
          (ref) => StoreClosing(
            client: SellerAppServiceClient(
              FakeTransportBuilder().unary(SellerAppService.closeMyStore, (
                _,
                _,
              ) {
                closes++;
                return CloseMyStoreResponse(
                  store: Store(status: StoreStatus.STORE_STATUS_CLOSED),
                );
              }).build(),
            ),
            adoptClosed: () {},
            reload: () async => true,
            defaults: zhDefaultErrorText,
          ),
        ),
        storeEditorProvider.overrideWith(
          (ref) => StoreEditor(
            client: SellerAppServiceClient(
              FakeTransportBuilder().unary(SellerAppService.updateMyStore, (
                req,
                _,
              ) {
                updates.add(req);
                final error = updateFailure;
                if (error != null) throw error;
                return UpdateMyStoreResponse(
                  store: Store(
                    id: 'store-1',
                    storeName: '阿明果園',
                    contactPhone: req.store.contactPhone,
                    etag: 'v2',
                  ),
                );
              }).build(),
            ),
            current: () => ref.read(myStoreSnapshotProvider),
            reload: () => ref.read(currentStoreProvider.notifier).refresh(),
            adopt: (written) =>
                ref.read(currentStoreProvider.notifier).adoptWritten(written),
            defaults: zhDefaultErrorText,
          ),
        ),
      ],
    );
    addTearDown(container.dispose);
    final sub = container.listen(currentStoreProvider, (_, _) {});
    addTearDown(sub.close);
    await container.read(currentStoreProvider.future);

    tester.view.physicalSize = const Size(900, 3000);
    tester.view.devicePixelRatio = 2;
    addTearDown(tester.view.reset);
    await tester.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: ScreenUtilInit(
          designSize: const Size(393, 852),
          builder: (_, _) => router == null
              ? const MaterialApp(home: ProfileScreen())
              : MaterialApp.router(routerConfig: router),
        ),
      ),
    );
    await tester.pumpAndSettle();
    return container;
  }

  setUp(() => updateFailure = null);

  testWidgets('開店進度卡取代店舖資料完整度與舊的認證卡', (tester) async {
    await pump(
      tester,
      response: myStoreResponse(
        publicListing: publicListingMissing([
          SellerRequirement.SELLER_REQUIREMENT_LOGO,
          SellerRequirement.SELLER_REQUIREMENT_KYC_APPROVED,
        ]),
        shippingSetup: shippingSetupMissing([
          SellerRequirement.SELLER_REQUIREMENT_KYC_APPROVED,
        ]),
      ),
    );

    expect(find.text('賣家開店任務'), findsOneWidget);
    expect(find.text('缺少：logo'), findsOneWidget);
    expect(find.text('待認證'), findsOneWidget);
    expect(find.text('店舖資料完整度'), findsNothing);
    expect(find.text('實名與收款認證'), findsNothing);
    expect(find.text('通過商家認證後才可設定配送'), findsOneWidget);
  });

  testWidgets('物流設定入口：去了回來由進度卡的可見性重讀一次，離開時不重讀', (tester) async {
    var inputsBuilds = 0;
    var now = DateTime(2026, 10, 9, 12);
    final router = GoRouter(
      routes: [
        GoRoute(path: '/', builder: (_, _) => const ProfileScreen()),
        GoRoute(
          path: '/settings/shipping',
          builder: (_, _) => const Scaffold(body: Text('物流設定頁')),
        ),
      ],
    );
    addTearDown(router.dispose);
    await pump(
      tester,
      kycStatus: KYCFlowStatus.KYC_FLOW_STATUS_APPROVED,
      response: myStoreResponse(publicListing: publicListingMissing()),
      router: router,
      onInputsBuilt: () => inputsBuilds++,
      clock: () => now,
    );

    await tester.tap(find.text('管理宅配、超商與寄退件地址'));
    await tester.pumpAndSettle();
    expect(find.text('物流設定頁'), findsOneWidget);
    final inputsAtDeparture = inputsBuilds;
    final storeAtDeparture = server.calls;

    now = now.add(const Duration(seconds: 31));
    router.pop();
    await tester.pumpAndSettle();
    expect(inputsBuilds, inputsAtDeparture + 1, reason: '回來重讀進度輸入一次');
    expect(server.calls, storeAtDeparture + 1, reason: '回來重讀店家一次');
  });

  testWidgets('SHIPPING_SETUP 啟用才開放營運設定的物流入口', (tester) async {
    await pump(
      tester,
      kycStatus: KYCFlowStatus.KYC_FLOW_STATUS_APPROVED,
      response: myStoreResponse(publicListing: publicListingMissing()),
    );

    expect(find.text('管理宅配、超商與寄退件地址'), findsOneWidget);
    expect(find.text('通過商家認證後才可設定配送'), findsNothing);
    expect(find.text('任務完成'), findsOneWidget);
  });

  testWidgets('identity_locked：店名與商店介紹顯示鎖定說明且點不進編輯，其他欄位可編輯', (tester) async {
    await pump(tester, identityLocked: true);

    expect(find.text(kPayuniLockedNotice), findsNWidgets(2));
    await tester.tap(find.text('商店名稱'));
    await tester.pumpAndSettle();
    expect(find.text('編輯商店名稱'), findsNothing);

    await tester.tap(find.text('得獎紀錄 / 店鋪亮點'));
    await tester.pumpAndSettle();
    expect(find.text('編輯得獎紀錄 / 店鋪亮點'), findsOneWidget);
  });

  testWidgets('沒有 identity_locked：店名可編輯，沒有鎖定說明', (tester) async {
    await pump(tester);

    expect(find.text(kPayuniLockedNotice), findsNothing);
    await tester.tap(find.text('商店名稱'));
    await tester.pumpAndSettle();
    expect(find.text('編輯商店名稱'), findsOneWidget);
  });

  testWidgets('欄位與標籤：商店電話、聯絡人姓名、聯絡人手機、客服電子信箱、店家地址；沒有信箱列', (tester) async {
    await pump(tester);

    for (final label in [
      '商店名稱',
      '商店介紹',
      '店家地址',
      '官方網站',
      '得獎紀錄 / 店鋪亮點',
      '商店電話',
      '聯絡人姓名',
      '聯絡人手機',
      '客服電子信箱',
    ]) {
      expect(
        find.descendant(
          of: find.byType(ProfileInfoSection),
          matching: find.text(label),
        ),
        findsOneWidget,
        reason: label,
      );
    }
    expect(find.text('信箱'), findsNothing);
    expect(find.text('聯絡電話'), findsNothing);
    expect(find.text('公司地址'), findsNothing);
    expect(find.text('已認證'), findsNothing);
  });

  testWidgets('沒有 STOREFRONT_WRITE：唯讀，說明「商店目前無法編輯」，點不進編輯', (tester) async {
    await pump(
      tester,
      response: myStoreResponse(capabilities: suspendedCapabilities),
    );

    expect(find.text('商店目前無法編輯，這裡只能檢視'), findsOneWidget);
    await tester.tap(find.text('商店名稱'));
    await tester.pumpAndSettle();
    expect(find.text('編輯商店名稱'), findsNothing);
  });

  testWidgets('儲存：只寫那個欄位、帶 etag，成功顯示「商店資訊已儲存」', (tester) async {
    await pump(tester);

    await tester.tap(find.text('商店電話'));
    await tester.pumpAndSettle();
    await tester.enterText(find.byType(TextField), '0212345678');
    await tester.tap(find.text('保存'));
    await tester.pumpAndSettle();

    expect(updates.single.updateMask.paths, ['contact_phone']);
    expect(updates.single.store.contactPhone, '0212345678');
    expect(updates.single.store.etag, 'etag-1');
    expect(find.text('商店資訊已儲存'), findsOneWidget);
    await tester.pump(const Duration(seconds: 3));
  });

  testWidgets('有 ACCOUNT_CLOSE（含停權中）：顯示「關閉商店」，確認後呼叫 CloseMyStore', (
    tester,
  ) async {
    await pump(
      tester,
      response: myStoreResponse(capabilities: suspendedCapabilities),
    );

    await tester.ensureVisible(find.widgetWithText(TextButton, '關閉商店'));
    await tester.pumpAndSettle();
    await tester.tap(find.widgetWithText(TextButton, '關閉商店'));
    await tester.pumpAndSettle();
    expect(closes, 0);
    await tester.tap(find.text('確認關閉'));
    await tester.pumpAndSettle();

    expect(closes, 1);
  });

  testWidgets('等待刪除沒有 ACCOUNT_CLOSE：不顯示「關閉商店」，仍有「刪除帳號」', (tester) async {
    await pump(
      tester,
      response: myStoreResponse(
        status: StoreStatus.STORE_STATUS_PENDING_DELETION,
      ),
    );
    await tester.scrollUntilVisible(
      find.widgetWithText(TextButton, '刪除帳號'),
      200,
      scrollable: find.byType(Scrollable).first,
    );

    expect(find.text('關閉商店'), findsNothing);
    expect(find.widgetWithText(TextButton, '刪除帳號'), findsOneWidget);
  });

  testWidgets('值沒改：不送出也不顯示已儲存', (tester) async {
    await pump(tester);

    await tester.tap(find.text('商店名稱'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('保存'));
    await tester.pumpAndSettle();

    expect(updates, isEmpty);
    expect(find.text('商店資訊已儲存'), findsNothing);
  });

  testWidgets('版本衝突：對話框留著、輸入保留、顯示最新版本說明並重讀店家', (tester) async {
    await pump(tester);
    updateFailure = reasonError(Code.aborted, 'VERSION_CONFLICT');
    final readsBefore = server.calls;

    await tester.tap(find.text('商店電話'));
    await tester.pumpAndSettle();
    await tester.enterText(find.byType(TextField), '0212345678');
    await tester.tap(find.text('保存'));
    await tester.pumpAndSettle();

    expect(find.text('編輯商店電話'), findsOneWidget);
    expect(find.text('0212345678'), findsOneWidget);
    expect(
      find.text('商店資料已在其他地方更新\n已載入最新版本，你輸入的內容仍保留，請確認後再儲存。'),
      findsOneWidget,
    );
    expect(server.calls, readsBefore + 1);
  });

  testWidgets('欄位錯誤顯示在對應欄位的對話框，格式錯誤不送出', (tester) async {
    await pump(tester);

    await tester.tap(find.text('聯絡人手機'));
    await tester.pumpAndSettle();
    await tester.enterText(find.byType(TextField), '0912');
    await tester.tap(find.text('保存'));
    await tester.pumpAndSettle();
    expect(find.text('請輸入 09 開頭的 10 碼手機號碼'), findsOneWidget);
    expect(updates, isEmpty);

    updateFailure = fieldError('store.contact_mobile', '手機格式不符');
    await tester.enterText(find.byType(TextField), '0912345678');
    await tester.tap(find.text('保存'));
    await tester.pumpAndSettle();
    expect(find.text('手機格式不符'), findsOneWidget);
    expect(updates.single.store.contactMobile, '0912345678');
  });
}
