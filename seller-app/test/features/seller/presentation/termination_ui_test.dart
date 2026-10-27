import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_seller/core/router/app_router.dart';
import 'package:hawkr_seller/features/dashboard/presentation/termination_notice_banner.dart';
import 'package:hawkr_seller/features/seller/data/current_store.dart';
import 'package:hawkr_seller/features/seller/data/measures.dart';
import 'package:hawkr_seller/features/seller/presentation/seller_appeal_screen.dart'
    show MeasureCard;
import 'package:hawkr_seller/features/seller/presentation/suspension_notice_screen.dart';
import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/widgets/hk_glass_app_bar.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_riverpod/misc.dart' show Override;
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

import '../../../support/measure_fixtures.dart';
import '../../../support/my_store_fixtures.dart';

/// 公告頁標題（App bar）裡的文字；措施卡的狀態標示也叫「合作已終止」，所以標題要限定在 App bar。
Finder _appBarText(String text) =>
    find.descendant(of: find.byType(HkGlassAppBar), matching: find.text(text));

const _terminatedSubtitle =
    '店家已暫停營業，已成立的訂單仍需完成；訂單完成後店家會關閉。終止生效後 6 個月內申覆成立，可恢復同一家店。';

Future<void> _pumpApp(
  WidgetTester tester,
  Widget home, {
  List<Override> overrides = const [],
  GoRouter? router,
  bool fullScreen = false,
}) async {
  tester.view.physicalSize = const Size(800, 2400);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.reset);
  await tester.pumpWidget(
    ProviderScope(
      overrides: overrides,
      child: ScreenUtilInit(
        designSize: const Size(393, 852),
        builder: (_, _) => router == null
            ? MaterialApp(
                home: fullScreen
                    ? home
                    : Scaffold(body: SingleChildScrollView(child: home)),
              )
            : MaterialApp.router(routerConfig: router),
      ),
    ),
  );
  await tester.pump();
  expect(tester.takeException(), isNull);
}

void main() {
  group('公告頁：合作已終止', () {
    Future<void> pumpNotice(WidgetTester tester, List<Measure> active) =>
        _pumpApp(
          tester,
          const SuspensionNoticeScreen(),
          fullScreen: true,
          overrides: [
            myStoreSnapshotProvider.overrideWithValue(suspendedStore()),
            currentMyMeasuresProvider(
              MeasureState.MEASURE_STATE_ACTIVE,
            ).overrideWithValue(AsyncData(active)),
          ],
        );

    testWidgets('生效中的措施有終止合作：標題與副標同 website，疑義入口到措施與申覆', (tester) async {
      await pumpNotice(tester, [
        terminationOf(state: MeasureState.MEASURE_STATE_ACTIVE),
      ]);

      expect(_appBarText('合作已終止'), findsOneWidget);
      expect(find.text(_terminatedSubtitle), findsOneWidget);
      expect(find.text('商店已暫停營業'), findsNothing);
      expect(find.text('停權期間仍可使用'), findsNothing);
      expect(find.text('措施與申覆'), findsOneWidget);
    });

    testWidgets('生效中的終止合作：措施卡的狀態標示是「合作已終止」', (tester) async {
      await pumpNotice(tester, [
        terminationOf(state: MeasureState.MEASURE_STATE_ACTIVE),
      ]);

      expect(
        find.descendant(
          of: find.byType(MeasureCard),
          matching: find.text('合作已終止'),
        ),
        findsOneWidget,
      );
      expect(
        find.descendant(
          of: find.byType(MeasureCard),
          matching: find.text('停權中'),
        ),
        findsNothing,
      );
    });

    testWidgets('生效中的措施只有停權：仍是「商店已暫停營業」', (tester) async {
      await pumpNotice(tester, [activeSuspension()]);

      expect(find.text('商店已暫停營業'), findsOneWidget);
      expect(find.text('合作已終止'), findsNothing);
      expect(find.text(_terminatedSubtitle), findsNothing);
    });

    testWidgets('終止與停權並存：照終止顯示', (tester) async {
      await pumpNotice(tester, [
        activeSuspension(),
        terminationOf(state: MeasureState.MEASURE_STATE_ACTIVE),
      ]);

      expect(_appBarText('合作已終止'), findsOneWidget);
    });

    testWidgets('措施還在讀取或讀不到：不知道是終止，用通用的停權公告', (tester) async {
      await pumpNotice(tester, const []);

      expect(find.text('商店已暫停營業'), findsOneWidget);
      expect(find.text('合作已終止'), findsNothing);
    });

    testWidgets('終止的措施卡：終止生效時間與申覆期限，申覆入口反灰並附原因', (tester) async {
      await pumpNotice(tester, [
        terminationOf(
          state: MeasureState.MEASURE_STATE_ACTIVE,
          canAppeal: false,
          appealDeadlineTime: DateTime.utc(2027, 5, 22, 4, 30),
          appealUnavailableReasons: [
            AppealUnavailableReason.APPEAL_UNAVAILABLE_REASON_ALREADY_APPEALED,
          ],
        ),
      ]);

      expect(find.text('終止生效：2026/11/20 12:30'), findsOneWidget);
      expect(
        find.text('$appealAlreadyAppealedSentence · 申覆期限：2027/05/22 12:30'),
        findsOneWidget,
      );
    });
  });

  group('MeasureCard 的申覆入口與預定生效', () {
    Widget card(Measure m) =>
        MeasureCard(measure: m, onAppeal: () {}, onRemediate: () {});

    Finder button() => find.byKey(const ValueKey('measure-appeal-button'));

    testWidgets('預定生效的終止：chip、終止生效時間、申覆期限；可以申覆時按鈕啟用、沒有原因', (tester) async {
      await _pumpApp(
        tester,
        card(
          terminationOf(
            createTime: DateTime.utc(2026, 10, 15, 2),
            appealDeadlineTime: DateTime.utc(2027, 5, 22, 4, 30),
          ),
        ),
      );

      expect(find.text('預定生效'), findsOneWidget);
      expect(find.text('終止生效：2026/11/20 12:30'), findsOneWidget);
      expect(find.text('申覆期限：2027/05/22 12:30'), findsOneWidget);
      expect(tester.widget<OutlinedButton>(button()).onPressed, isNotNull);
      expect(
        tester
            .widget<Text>(find.byKey(const ValueKey('measure-appeal-note')))
            .data,
        '申覆期限：2027/05/22 12:30',
      );
    });

    testWidgets('生效中的終止加上待處理的申覆：狀態 chip 與請求 chip 並列在標題列，不互相取代', (tester) async {
      await _pumpApp(
        tester,
        card(
          terminationOf(
            state: MeasureState.MEASURE_STATE_ACTIVE,
            requests: [
              MeasureRequest(
                kind: MeasureRequestKind.MEASURE_REQUEST_KIND_APPEAL,
                state: MeasureRequestState.MEASURE_REQUEST_STATE_PENDING,
              ),
            ],
          ),
        ),
      );

      final state = find.text('合作已終止');
      final request = find.text('申覆審核中');
      expect(state, findsOneWidget);
      expect(request, findsOneWidget);
      expect(
        tester.getCenter(state).dy,
        closeTo(tester.getCenter(request).dy, 1),
      );
      expect(
        tester.getCenter(state).dx,
        lessThan(tester.getCenter(request).dx),
      );
    });

    testWidgets('申覆期限已過：按鈕反灰，原因是伺服器給的第一項', (tester) async {
      await _pumpApp(
        tester,
        card(
          terminationOf(
            state: MeasureState.MEASURE_STATE_ACTIVE,
            canAppeal: false,
            appealDeadlineTime: DateTime.utc(2027, 5, 22, 4, 30),
            appealUnavailableReasons: [
              AppealUnavailableReason
                  .APPEAL_UNAVAILABLE_REASON_APPEAL_WINDOW_ENDED,
              AppealUnavailableReason.APPEAL_UNAVAILABLE_REASON_REQUEST_PENDING,
            ],
          ),
        ),
      );

      expect(tester.widget<OutlinedButton>(button()).onPressed, isNull);
      expect(
        tester
            .widget<Text>(find.byKey(const ValueKey('measure-appeal-note')))
            .data,
        '$appealWindowEndedSentence · 申覆期限：2027/05/22 12:30',
      );
      // 原因與期限同一行，位在按鈕上方。
      expect(
        tester
            .getBottomLeft(find.byKey(const ValueKey('measure-appeal-note')))
            .dy,
        lessThanOrEqualTo(tester.getTopLeft(button()).dy),
      );
      expect(find.text('預定生效'), findsNothing);
    });

    testWidgets('不能申覆又沒有有效原因：反灰、不顯示原因（不把 UNSPECIFIED 當文字）', (tester) async {
      await _pumpApp(
        tester,
        card(withUnrecognizedAppealReason(activeSuspension())),
      );

      expect(tester.widget<OutlinedButton>(button()).onPressed, isNull);
      expect(find.byKey(const ValueKey('measure-appeal-note')), findsNothing);
    });

    testWidgets('非終止的措施沒有申覆期限', (tester) async {
      await _pumpApp(tester, card(activeSuspension()));

      expect(find.textContaining('申覆期限：'), findsNothing);
    });
  });

  group('首頁終止預告提示卡', () {
    List<Override> scheduled(AsyncValue<List<Measure>>? value) => [
      currentMyMeasuresProvider(
        MeasureState.MEASURE_STATE_SCHEDULED,
      ).overrideWithValue(value),
    ];

    testWidgets('有預定生效的終止：顯示 period.start_time（不是建立時間加 30 天）', (tester) async {
      await _pumpApp(
        tester,
        const TerminationNoticeBanner(),
        overrides: scheduled(
          AsyncData([
            terminationOf(
              start: DateTime.utc(2026, 11, 20, 4, 30),
              createTime: DateTime.utc(2026, 10, 15, 2),
            ),
          ]),
        ),
      );

      expect(
        find.text('平台已通知將於 2026/11/20 12:30 終止合作。在此之前店家照常營業；如有異議，可提出申覆。'),
        findsOneWidget,
      );
    });

    testWidgets('多筆預定生效：每筆各一張卡，順序照伺服器', (tester) async {
      await _pumpApp(
        tester,
        const TerminationNoticeBanner(),
        overrides: scheduled(
          AsyncData([
            terminationOf(id: 'late', start: DateTime.utc(2026, 12, 1, 4, 30)),
            terminationOf(
              id: 'early',
              start: DateTime.utc(2026, 11, 20, 4, 30),
            ),
          ]),
        ),
      );

      final texts = tester
          .widgetList<Text>(
            find.byKey(const ValueKey('termination-notice-text')),
          )
          .map((t) => t.data)
          .toList();
      // 順序照伺服器給的，不依生效時間重排。
      expect(texts, [
        '平台已通知將於 2026/12/01 12:30 終止合作。在此之前店家照常營業；如有異議，可提出申覆。',
        '平台已通知將於 2026/11/20 12:30 終止合作。在此之前店家照常營業；如有異議，可提出申覆。',
      ]);
    });

    testWidgets('色調是 warning，不是 error', (tester) async {
      await _pumpApp(
        tester,
        const TerminationNoticeBanner(),
        overrides: scheduled(AsyncData([terminationOf()])),
      );

      final box = tester.widget<Container>(
        find
            .ancestor(
              of: find.byKey(const ValueKey('termination-notice-text')),
              matching: find.byType(Container),
            )
            .first,
      );
      final border = (box.decoration! as BoxDecoration).border! as Border;
      expect(border.top.color, AppColors.warning);
      expect(border.top.color, isNot(AppColors.error));
      final icon = tester.widget<Icon>(find.byType(Icon).first);
      expect(icon.color, AppIconColors.warning);
    });

    for (final (name, value) in <(String, AsyncValue<List<Measure>>?)>[
      ('沒有預定生效的措施', const AsyncData(<Measure>[])),
      ('讀取中', const AsyncLoading()),
      ('讀取失敗', AsyncError(StateError('down'), StackTrace.empty)),
      ('沒有登入帳號', null),
    ]) {
      testWidgets('$name：不顯示', (tester) async {
        await _pumpApp(
          tester,
          const TerminationNoticeBanner(),
          overrides: scheduled(value),
        );

        expect(
          find.byKey(const ValueKey('termination-notice-text')),
          findsNothing,
        );
      });
    }

    testWidgets('點提示卡開措施與申覆', (tester) async {
      final router = GoRouter(
        routes: [
          GoRoute(
            path: '/',
            builder: (_, _) => const Scaffold(body: TerminationNoticeBanner()),
          ),
          GoRoute(
            path: SellerRoutes.measures,
            builder: (_, _) => const Scaffold(body: Text('MEASURES')),
          ),
        ],
      );
      addTearDown(router.dispose);
      await _pumpApp(
        tester,
        const SizedBox(),
        router: router,
        overrides: scheduled(AsyncData([terminationOf()])),
      );

      await tester.tap(find.byKey(const ValueKey('termination-notice-text')));
      await tester.pumpAndSettle();

      expect(find.text('MEASURES'), findsOneWidget);
    });
  });
}
