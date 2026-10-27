import 'dart:async';

import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/gen/google/rpc/error_details.pb.dart'
    show LocalizedMessage;
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_api/transport.dart';
import 'package:hawkr_seller/core/networking/connect_provider.dart';
import 'package:hawkr_seller/core/router/app_router.dart';
import 'package:hawkr_seller/core/session/session_provider.dart';
import 'package:hawkr_seller/features/seller/presentation/widgets/option_row.dart';
import 'package:hawkr_seller/features/seller/data/current_store.dart';
import 'package:hawkr_seller/features/seller/data/my_store_repository.dart';
import 'package:hawkr_seller/features/seller/data/seller_agreements_repository.dart';
import 'package:hawkr_seller/features/seller/data/seller_onboarding.dart';
import 'package:hawkr_seller/features/seller/presentation/seller_info_screen.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:hawkr_seller/core/analytics/analytics_events.dart';
import 'package:hawkr_seller/core/analytics/analytics_providers.dart';

import '../../../support/connect_errors.dart';
import '../../../support/fake_my_store.dart';
import '../../../support/my_store_fixtures.dart';
import '../../../support/measure_fixtures.dart';
import '../../../support/default_error_text.dart';

class _LoggedIn extends Session {
  @override
  SessionModel build() =>
      const SessionModel(accountId: 'acc-1', isLoggedIn: true);
}

class _Events implements AnalyticsEvents {
  int completed = 0;

  @override
  Future<void> onboardingComplete() async => completed++;

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _Agreements implements SellerAgreementsRepository {
  _Agreements(this.versions);

  final List<String> versions;
  int calls = 0;

  /// 不是 null 時，下一次讀取等它完成才回答。
  Completer<void>? gate;

  /// 為真時下一次讀取失敗（失敗一次後恢復）。
  bool failNext = false;

  @override
  Future<SellerAgreementSet> load() async {
    final wait = gate;
    if (wait != null) await wait.future;
    if (failNext) {
      failNext = false;
      calls++;
      throw StateError('agreements unavailable');
    }
    final index = calls < versions.length ? calls : versions.length - 1;
    calls++;
    return SellerAgreementSet([
      SellerAgreement(
        kind: AgreementKind.AGREEMENT_KIND_SELLER_TERMS,
        version: versions[index],
        url: 'https://www.example.test/seller-terms',
      ),
    ]);
  }
}

void main() {
  late List<ApplyStoreRequest> applies;
  late List<UpdateMyStoreRequest> updates;
  late List<CompleteMyStoreOnboardingRequest> completes;
  Object? applyFailure;
  late _Events events;
  var openingReads = 0;
  late GoRouter router;

  Future<ProviderContainer> pump(
    WidgetTester tester, {
    required FakeMyStoreServer server,
    required _Agreements agreements,
    FakeMyStoreServer? onboardingServer,
    List<Measure> measures = const [],
    Object? measuresFailure,
    List<int>? measurePageSizes,
    List<StoreOpeningUnavailableReason> openingReasons = const [],
    Object? openingFailure,
    bool? openingCanApply,
    Completer<void>? openingGate,
    bool openingWithMessages = true,
  }) async {
    events = _Events();
    openingReads = 0;
    applies = [];
    updates = [];
    completes = [];
    final onboarding = SellerOnboarding(
      client: SellerAppServiceClient(
        FakeTransportBuilder()
            .unary(SellerAppService.applyStore, (req, _) {
              applies.add(req);
              final error = applyFailure;
              if (error != null) throw error;
              return ApplyStoreResponse(store: Store(etag: 'applied'));
            })
            .unary(SellerAppService.updateMyStore, (req, _) {
              updates.add(req);
              return UpdateMyStoreResponse(store: Store(etag: 'updated'));
            })
            .unary(SellerAppService.completeMyStoreOnboarding, (req, _) {
              completes.add(req);
              return CompleteMyStoreOnboardingResponse();
            })
            .build(),
      ),
      stores: (onboardingServer ?? server).repository,
      reload: () async => true,
      defaults: zhDefaultErrorText,
    );
    final container = ProviderContainer(
      overrides: [
        sessionProvider.overrideWith(_LoggedIn.new),
        myStoreRepositoryProvider.overrideWithValue(server.repository),
        sellerAgreementsRepositoryProvider.overrideWithValue(agreements),
        sellerOnboardingProvider.overrideWithValue(onboarding),
        // 措施提示卡讀 ListMyMeasures（帳號層級）；預設沒有措施。
        sellerClientProvider.overrideWithValue(
          SellerAppServiceClient(
            FakeTransportBuilder()
                .unary(SellerAppService.listMyMeasures, (request, _) {
                  measurePageSizes?.add(request.pageSize);
                  final error = measuresFailure;
                  if (error != null) throw error;
                  return ListMyMeasuresResponse(measures: measures);
                })
                .unary(SellerAppService.getMyStoreOpening, (_, _) async {
                  openingReads++;
                  await openingGate?.future;
                  final error = openingFailure;
                  if (error != null) throw error;
                  return GetMyStoreOpeningResponse(
                    canApplyStore: openingCanApply ?? openingReasons.isEmpty,
                    unavailabilities: [
                      for (final r in openingReasons)
                        StoreOpeningUnavailability(
                          reason: r,
                          message: openingWithMessages
                              ? LocalizedMessage(
                                  locale: 'zh-TW',
                                  message: _openingSentence(r),
                                )
                              : null,
                        ),
                    ],
                  );
                })
                .build(),
          ),
        ),
        analyticsEventsProvider.overrideWithValue(events),
      ],
    );
    addTearDown(container.dispose);
    final sub = container.listen(currentStoreProvider, (_, _) {});
    addTearDown(sub.close);
    await container.read(currentStoreProvider.future);

    router = GoRouter(
      routes: [
        GoRoute(path: '/', builder: (_, _) => const SellerInformationScreen()),
        GoRoute(
          path: '/main/dashboard',
          builder: (_, _) => const Scaffold(body: Text('DASHBOARD')),
        ),
        GoRoute(
          path: '/profile',
          builder: (_, _) => const Scaffold(body: Text('PROFILE')),
        ),
        GoRoute(
          path: SellerRoutes.measures,
          builder: (_, _) => const Scaffold(body: Text('VIOLATIONS')),
        ),
      ],
    );
    addTearDown(router.dispose);
    // 有措施提示卡時內容多一張卡，加高避免表單被擠出畫面。
    tester.view.physicalSize = Size(
      800,
      measures.isEmpty && openingReasons.isEmpty ? 1600 : 2000,
    );
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
    await tester.pumpAndSettle();
    return container;
  }

  Future<void> fillForm(WidgetTester tester) async {
    await tester.enterText(find.widgetWithText(TextField, '店舖名稱'), '阿明果園');
    await tester.tap(find.text('選擇縣市'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('臺北市').last);
    await tester.pumpAndSettle();
    await tester.tap(find.text('選擇行政區'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('松山區').last);
    await tester.pumpAndSettle();
    await tester.enterText(find.widgetWithText(TextField, '詳細地址'), '路一號');
    await tester.pumpAndSettle();
  }

  FilledButton submitButton(WidgetTester tester) =>
      tester.widget<FilledButton>(find.byType(FilledButton));

  setUp(() => applyFailure = null);

  group('措施提示卡', () {
    testWidgets('有措施紀錄：整張卡可點，位在介紹卡之下、表單之上；點下去開措施與申覆列表', (tester) async {
      await pump(
        tester,
        server: FakeMyStoreServer.absent(),
        agreements: _Agreements(['2026-10-01']),
        measures: [measureOf()],
      );

      final notice = find.text('你有措施紀錄，查看與申覆');
      expect(notice, findsOneWidget);
      expect(
        tester.getTopLeft(find.text('準備好開始營運了嗎？')).dy,
        lessThan(tester.getTopLeft(notice).dy),
      );
      expect(
        tester.getTopLeft(notice).dy,
        lessThan(tester.getTopLeft(find.text('品牌身份')).dy),
      );
      // 共用的入口列（button semantics），整張卡都是點擊範圍。
      final row = find.ancestor(of: notice, matching: find.byType(OptionRow));
      expect(row, findsOneWidget);
      expect(tester.getSize(row).width, greaterThan(300));

      await tester.tap(notice);
      await tester.pumpAndSettle();
      expect(find.text('VIOLATIONS'), findsOneWidget);
    });

    testWidgets('只問「有沒有措施」：一次 page_size 1 的請求，不讀完整個列表', (tester) async {
      final pageSizes = <int>[];
      await pump(
        tester,
        server: FakeMyStoreServer.absent(),
        agreements: _Agreements(['2026-10-01']),
        measures: [
          measureOf(id: 'a'),
          measureOf(id: 'b'),
        ],
        measurePageSizes: pageSizes,
      );

      expect(find.text('你有措施紀錄，查看與申覆'), findsOneWidget);
      expect(pageSizes, [1]);
    });

    testWidgets('沒有措施紀錄：不顯示', (tester) async {
      await pump(
        tester,
        server: FakeMyStoreServer.absent(),
        agreements: _Agreements(['2026-10-01']),
      );

      expect(find.text('你有措施紀錄，查看與申覆'), findsNothing);
    });

    testWidgets('讀取失敗：不顯示，也不擋開店', (tester) async {
      await pump(
        tester,
        server: FakeMyStoreServer.absent(),
        agreements: _Agreements(['2026-10-01']),
        measuresFailure: ConnectException(Code.unavailable, 'down'),
      );
      await fillForm(tester);
      await tester.tap(find.byKey(const Key('seller_terms_checkbox')));
      await tester.pumpAndSettle();

      expect(find.text('你有措施紀錄，查看與申覆'), findsNothing);
      expect(submitButton(tester).onPressed, isNotNull);
    });

    testWidgets('有措施紀錄也不擋開店：照常填表、勾選後可送出', (tester) async {
      await pump(
        tester,
        server: FakeMyStoreServer.absent(),
        agreements: _Agreements(['2026-10-01']),
        measures: [measureOf()],
      );
      await fillForm(tester);
      await tester.tap(find.byKey(const Key('seller_terms_checkbox')));
      await tester.pumpAndSettle();

      expect(submitButton(tester).onPressed, isNotNull);
    });
  });

  group('開店可用性（GetMyStoreOpening）', () {
    // 伺服器對各個原因給的句子（設計 §7(a)）；畫面必須原樣顯示它，client 的程式碼沒有這張表。
    const reason = StoreOpeningUnavailableReason.values;
    final terminated = reason.firstWhere((r) => r.name.endsWith('_TERMINATED'));
    final suspended = reason.firstWhere((r) => r.name.endsWith('_SUSPENDED'));
    final deleting = reason.firstWhere(
      (r) => r.name.endsWith('_ACCOUNT_DELETING'),
    );
    final storeExists = reason.firstWhere(
      (r) => r.name.endsWith('_STORE_EXISTS'),
    );

    Future<void> ready(WidgetTester tester) async {
      await fillForm(tester);
      await tester.tap(find.byKey(const Key('seller_terms_checkbox')));
      await tester.pumpAndSettle();
    }

    for (final c in [
      (
        '終止合作',
        [terminated, suspended],
        '此帳號因終止合作（第 21.3 條）不能重新開店。如有異議，可對該終止提出申覆。',
        true,
      ),
      ('停權', [suspended], '帳號停權中，解除停權前不能開店', true),
      ('帳號刪除', [deleting], '帳號正在刪除，不能申請開店', false),
      ('已經是賣家', [storeExists], '你已經是賣家，不需要重複申請', false),
    ]) {
      testWidgets('${c.$1}：表單填完也不能送出，只顯示第一個原因；沒有送出 ApplyStore', (tester) async {
        await pump(
          tester,
          server: FakeMyStoreServer.absent(),
          agreements: _Agreements(['2026-10-01']),
          openingReasons: c.$2,
        );
        await ready(tester);

        expect(submitButton(tester).onPressed, isNull);
        expect(find.text(c.$3), findsOneWidget);
        if (c.$2.length > 1) {
          expect(find.text('帳號停權中，解除停權前不能開店'), findsNothing);
        }
        final entry = find.descendant(
          of: find.byKey(const Key('store_opening_refusal')),
          matching: find.text('措施與申覆'),
        );
        expect(entry, c.$4 ? findsOneWidget : findsNothing);
        if (c.$4) {
          await tester.tap(entry);
          await tester.pumpAndSettle();
          expect(find.text('VIOLATIONS'), findsOneWidget);
        }
        expect(applies, isEmpty);
      });
    }

    for (final c in [
      (
        'UNSPECIFIED',
        [
          StoreOpeningUnavailableReason.values.firstWhere(
            (r) => r.name.endsWith('_UNSPECIFIED'),
          ),
        ],
      ),
      ('空原因', <StoreOpeningUnavailableReason>[]),
    ]) {
      testWidgets('can_apply_store 為 false 而原因是${c.$1}且沒有句子：仍停用送出，不顯示說明文字', (
        tester,
      ) async {
        await pump(
          tester,
          server: FakeMyStoreServer.absent(),
          agreements: _Agreements(['2026-10-01']),
          openingReasons: c.$2,
          openingCanApply: false,
          openingWithMessages: false,
        );
        await ready(tester);

        expect(openingReads, 1);
        expect(submitButton(tester).onPressed, isNull);
        expect(find.byKey(const Key('store_opening_refusal')), findsNothing);
        expect(applies, isEmpty);
      });
    }

    testWidgets('已知原因但伺服器沒有句子：仍停用送出，不顯示說明文字（client 不自己組字）', (tester) async {
      await pump(
        tester,
        server: FakeMyStoreServer.absent(),
        agreements: _Agreements(['2026-10-01']),
        openingReasons: [suspended],
        openingWithMessages: false,
      );
      await ready(tester);

      expect(submitButton(tester).onPressed, isNull);
      expect(find.byKey(const Key('store_opening_refusal')), findsNothing);
      expect(find.text('帳號停權中，解除停權前不能開店'), findsNothing);
      expect(applies, isEmpty);
    });

    testWidgets('開店可用性讀取中：送出停用，答案回來（可以開店）後才可送出', (tester) async {
      final gate = Completer<void>();
      await pump(
        tester,
        server: FakeMyStoreServer.absent(),
        agreements: _Agreements(['2026-10-01']),
        openingGate: gate,
      );
      await ready(tester);
      expect(submitButton(tester).onPressed, isNull);

      gate.complete();
      await tester.pumpAndSettle();
      expect(submitButton(tester).onPressed, isNotNull);
    });

    testWidgets('可以開店：不顯示說明，填完勾選後可送出', (tester) async {
      await pump(
        tester,
        server: FakeMyStoreServer.absent(),
        agreements: _Agreements(['2026-10-01']),
      );
      await ready(tester);

      expect(openingReads, 1);
      expect(find.byKey(const Key('store_opening_refusal')), findsNothing);
      expect(submitButton(tester).onPressed, isNotNull);
    });

    testWidgets('讀取失敗：不擋，送出時由 ApplyStore 的 ErrorReason 後備', (tester) async {
      applyFailure = reasonError(
        Code.failedPrecondition,
        'STORE_OPENING_SUSPENDED',
        message: '帳號停權中，解除停權前不能開店',
      );
      await pump(
        tester,
        server: FakeMyStoreServer.absent(),
        agreements: _Agreements(['2026-10-01']),
        openingFailure: ConnectException(Code.unavailable, 'down'),
      );
      await ready(tester);
      expect(submitButton(tester).onPressed, isNotNull);

      await tester.tap(find.byType(FilledButton));
      await tester.pumpAndSettle();

      expect(applies, hasLength(1));
      expect(find.text('帳號停權中，解除停權前不能開店'), findsOneWidget);
    });

    testWidgets('已有店家：不問能不能開店', (tester) async {
      await pump(
        tester,
        server: FakeMyStoreServer(response: myStoreResponse()),
        agreements: _Agreements(['2026-10-01']),
        openingReasons: [suspended],
      );

      expect(openingReads, 0);
      expect(find.byKey(const Key('store_opening_refusal')), findsNothing);
    });

    testWidgets('已有店家：不問能不能開店，送出走更新流程不呼叫 ApplyStore', (tester) async {
      await pump(
        tester,
        server: FakeMyStoreServer(response: myStoreResponse()),
        agreements: _Agreements(['2026-10-01']),
        openingReasons: [storeExists],
      );
      await fillForm(tester);

      expect(openingReads, 0);
      expect(find.byKey(const Key('store_opening_refusal')), findsNothing);
      await tester.tap(find.byType(FilledButton));
      await tester.pumpAndSettle();
      expect(applies, isEmpty);
      expect(updates, hasLength(1));
    });
  });

  testWidgets('沒有店家：顯示賣家契約勾選，沒勾不能送出；沒有「稍後設定」與聯絡方式', (tester) async {
    await pump(
      tester,
      server: FakeMyStoreServer.absent(),
      agreements: _Agreements(['2026-10-01']),
    );
    await fillForm(tester);

    expect(find.byKey(const Key('seller_terms_checkbox')), findsOneWidget);
    expect(
      find.text('我已閱讀並同意〈Hawkr 賣家契約〉', findRichText: true),
      findsOneWidget,
    );
    expect(find.text('稍後設定'), findsNothing);
    expect(find.text('聯絡方式'), findsNothing);
    expect(find.text('儲存並開始使用'), findsOneWidget);
    expect(find.text('儲存後前往完整設定'), findsOneWidget);
    expect(submitButton(tester).onPressed, isNull);

    await tester.tap(find.byKey(const Key('seller_terms_checkbox')));
    await tester.pumpAndSettle();
    expect(submitButton(tester).onPressed, isNotNull);
  });

  testWidgets('勾選後送出：ApplyStore 帶 SELLER_TERMS 與取得的版本、townCode', (
    tester,
  ) async {
    await pump(
      tester,
      server: FakeMyStoreServer.absent(),
      agreements: _Agreements(['2026-10-01']),
    );
    await fillForm(tester);
    await tester.tap(find.byKey(const Key('seller_terms_checkbox')));
    await tester.pumpAndSettle();

    await tester.tap(find.byType(FilledButton));
    await tester.pump();
    await tester.pump();

    final apply = applies.single;
    expect(apply.acceptedAgreements.single.version, '2026-10-01');
    expect(apply.address.townCode, '63000010');
    expect(apply.address.addressDetail, '路一號');
    expect(apply.storeName, '阿明果園');
    await tester.pumpAndSettle();
    expect(find.text('DASHBOARD'), findsOneWidget);
    expect(events.completed, 1);
  });

  testWidgets('AGREEMENT_OUTDATED：重讀條款、取消勾選並顯示過期文字', (tester) async {
    final agreements = _Agreements(['2026-10-01', '2026-11-01']);
    applyFailure = reasonError(Code.failedPrecondition, 'AGREEMENT_OUTDATED');
    await pump(
      tester,
      server: FakeMyStoreServer.absent(),
      agreements: agreements,
    );
    await fillForm(tester);
    await tester.tap(find.byKey(const Key('seller_terms_checkbox')));
    await tester.pumpAndSettle();

    agreements.gate = Completer<void>();
    await tester.tap(find.byType(FilledButton));
    await tester.pump();
    await tester.pump();

    // 條款重讀還沒回來：勾選框停用、不能送出，不會拿舊版本去送。
    expect(find.text('條款已更新，請重新閱讀後再勾選。'), findsOneWidget);
    expect(
      tester
          .widget<Checkbox>(find.byKey(const Key('seller_terms_checkbox')))
          .onChanged,
      isNull,
    );
    expect(submitButton(tester).onPressed, isNull);

    agreements.gate!.complete();
    agreements.gate = null;
    await tester.pumpAndSettle();
    expect(
      tester
          .widget<Checkbox>(find.byKey(const Key('seller_terms_checkbox')))
          .onChanged,
      isNotNull,
    );
    expect(
      tester
          .widget<Checkbox>(find.byKey(const Key('seller_terms_checkbox')))
          .value,
      isFalse,
    );
    expect(agreements.calls, 2);
    expect(submitButton(tester).onPressed, isNull);

    applyFailure = null;
    await tester.tap(find.byKey(const Key('seller_terms_checkbox')));
    await tester.pumpAndSettle();
    await tester.tap(find.byType(FilledButton));
    await tester.pump();
    await tester.pump();
    expect(applies.last.acceptedAgreements.single.version, '2026-11-01');
    await tester.pump(const Duration(seconds: 3));
  });

  testWidgets('已有店家：沒有勾選框；送出走 UpdateMyStore 加完成引導', (tester) async {
    await pump(
      tester,
      server: FakeMyStoreServer(
        response: myStoreResponse(onboardingCompleted: false),
      ),
      agreements: _Agreements(['2026-10-01']),
    );
    await fillForm(tester);

    expect(find.byKey(const Key('seller_terms_checkbox')), findsNothing);
    expect(submitButton(tester).onPressed, isNotNull);

    await tester.tap(find.byType(FilledButton));
    await tester.pump();
    await tester.pump();

    expect(applies, isEmpty);
    expect(updates.single.updateMask.paths, ['store_name', 'address']);
    expect(completes.single.etag, 'updated');
    await tester.pumpAndSettle();
    expect(find.text('DASHBOARD'), findsOneWidget);
  });

  testWidgets('條款讀取失敗：顯示「重試」，勾選框停用；重試成功後可勾', (tester) async {
    final agreements = _Agreements(['2026-10-01'])..failNext = true;
    await pump(
      tester,
      server: FakeMyStoreServer.absent(),
      agreements: agreements,
    );

    expect(find.text('重試'), findsOneWidget);
    expect(
      tester
          .widget<Checkbox>(find.byKey(const Key('seller_terms_checkbox')))
          .onChanged,
      isNull,
    );

    await tester.tap(find.text('重試'));
    await tester.pumpAndSettle();

    expect(find.text('重試'), findsNothing);
    expect(
      tester
          .widget<Checkbox>(find.byKey(const Key('seller_terms_checkbox')))
          .onChanged,
      isNotNull,
    );
  });

  for (final c in [
    ('STORE_OPENING_TERMINATED', '此帳號因終止合作（第 21.3 條）不能重新開店。如有異議，可對該終止提出申覆。'),
    ('STORE_OPENING_SUSPENDED', '帳號停權中，解除停權前不能開店'),
  ]) {
    testWidgets('${c.$1}：頁內顯示說明與「措施與申覆」入口，點下去開列表；輸入保留', (tester) async {
      applyFailure = reasonError(Code.failedPrecondition, c.$1, message: c.$2);
      await pump(
        tester,
        server: FakeMyStoreServer.absent(),
        agreements: _Agreements(['2026-10-01']),
      );
      await fillForm(tester);
      await tester.tap(find.byKey(const Key('seller_terms_checkbox')));
      await tester.pumpAndSettle();
      expect(find.byKey(const Key('store_opening_refusal')), findsNothing);

      await tester.tap(find.byType(FilledButton));
      await tester.pumpAndSettle();

      expect(find.text(c.$2), findsOneWidget);
      expect(find.text('阿明果園'), findsOneWidget);
      final entry = find.descendant(
        of: find.byKey(const Key('store_opening_refusal')),
        matching: find.text('措施與申覆'),
      );
      await tester.tap(entry);
      await tester.pumpAndSettle();
      expect(find.text('VIOLATIONS'), findsOneWidget);
    });
  }

  testWidgets('ALREADY_EXISTS：顯示建立衝突說明，畫面上的輸入保留', (tester) async {
    applyFailure = ConnectException(Code.alreadyExists, 'exists');
    await pump(
      tester,
      server: FakeMyStoreServer.absent(),
      agreements: _Agreements(['2026-10-01']),
    );
    await fillForm(tester);
    await tester.tap(find.byKey(const Key('seller_terms_checkbox')));
    await tester.pumpAndSettle();

    await tester.tap(find.byType(FilledButton));
    await tester.pump();
    await tester.pump();

    expect(
      find.text('店家資料已在其他地方建立，已載入最新版本，你輸入的內容仍保留，請確認後再送出。'),
      findsOneWidget,
    );
    expect(find.text('阿明果園'), findsOneWidget);
    expect(find.text('路一號'), findsOneWidget);
    expect(find.text('松山區'), findsOneWidget);
    await tester.pump(const Duration(seconds: 3));
  });

  testWidgets('快照說有店家、送出時伺服器說沒有：重讀快照讓勾選框出現，不顯示條款過期', (tester) async {
    final server = FakeMyStoreServer(
      response: myStoreResponse(onboardingCompleted: false),
    );
    await pump(tester, server: server, agreements: _Agreements(['2026-10-01']));
    await fillForm(tester);
    expect(find.byKey(const Key('seller_terms_checkbox')), findsNothing);

    server.failure = ConnectException(Code.notFound, 'gone');
    await tester.tap(find.byType(FilledButton));
    await tester.pump();
    await tester.pumpAndSettle();

    expect(applies, isEmpty);
    expect(find.byKey(const Key('seller_terms_checkbox')), findsOneWidget);
    expect(find.text('條款已更新，請重新閱讀後再勾選。'), findsNothing);
    expect(submitButton(tester).onPressed, isNull);
  });

  testWidgets('快照重讀失敗：顯示無法確認商店狀態，不顯示條款過期', (tester) async {
    final server = FakeMyStoreServer(
      response: myStoreResponse(onboardingCompleted: false),
    );
    await pump(
      tester,
      server: server,
      onboardingServer: FakeMyStoreServer.absent(),
      agreements: _Agreements(['2026-10-01']),
    );
    await fillForm(tester);

    server.failure = ConnectException(Code.unavailable, 'later');
    await tester.tap(find.byType(FilledButton));
    await tester.pump();
    await tester.pump();

    expect(find.text('目前無法確認商店狀態，請稍後再試一次。'), findsOneWidget);
    expect(find.text('條款已更新，請重新閱讀後再勾選。'), findsNothing);
    await tester.pump(const Duration(seconds: 3));
  });
}

/// 伺服器對各個「不能開店」原因給的句子（設計 §7(a)，與 seller 模組 `internal/text/zh-TW.json` 相同）。
String _openingSentence(
  StoreOpeningUnavailableReason reason,
) => switch (reason) {
  StoreOpeningUnavailableReason.STORE_OPENING_UNAVAILABLE_REASON_TERMINATED =>
    '此帳號因終止合作（第 21.3 條）不能重新開店。如有異議，可對該終止提出申覆。',
  StoreOpeningUnavailableReason.STORE_OPENING_UNAVAILABLE_REASON_SUSPENDED =>
    '帳號停權中，解除停權前不能開店',
  StoreOpeningUnavailableReason
      .STORE_OPENING_UNAVAILABLE_REASON_ACCOUNT_DELETING =>
    '帳號正在刪除，不能申請開店',
  StoreOpeningUnavailableReason.STORE_OPENING_UNAVAILABLE_REASON_STORE_EXISTS =>
    '你已經是賣家，不需要重複申請',
  _ => '伺服器的句子',
};
