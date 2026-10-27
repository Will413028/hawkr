import 'dart:async';
import 'dart:typed_data';

import 'package:hawkr_api/gen/hawkr/common/v1alpha1/policy_category.pbenum.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_api/transport.dart';
import 'package:hawkr_seller/core/networking/connect_provider.dart';
import 'package:hawkr_seller/core/router/app_router.dart';
import 'package:hawkr_seller/core/session/session_provider.dart';
import 'package:hawkr_seller/features/media/data/private_document_uploader.dart';
import 'package:hawkr_seller/features/seller/data/measures.dart';
import 'package:hawkr_seller/features/seller/presentation/seller_measure_request_form_screen.dart';
import 'package:hawkr_seller/features/seller/presentation/seller_appeal_screen.dart';
import 'package:hawkr_seller/shared/widgets/truck_loading_overlay.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

import '../../../support/connect_errors.dart';
import '../../../support/measure_fixtures.dart';

/// 可以按的申覆入口（`can_appeal` 為真）；反灰的入口（不能申覆，附原因）不算。
final _enabledAppealButton = find.byWidgetPredicate(
  (w) =>
      w is OutlinedButton &&
      w.key == const ValueKey('measure-appeal-button') &&
      w.onPressed != null,
);

class _LoggedIn extends Session {
  @override
  SessionModel build() =>
      const SessionModel(accountId: 'acc-1', isLoggedIn: true);
}

class _Server {
  List<Measure> measures = [];
  Object? listFailure;
  Object? submitFailure;
  Object? getAnswer;
  int listCalls = 0;
  final submits = <SubmitAppealRequest>[];
  final remediations = <RequestRemediationRequest>[];
  final gets = <GetMyMeasureRequest>[];
  Completer<void>? submitGate;
  Completer<void>? listGate;
  Completer<void>? getGate;

  SellerAppServiceClient get client => SellerAppServiceClient(
    FakeTransportBuilder()
        .unary(SellerAppService.listMyMeasures, (_, _) async {
          listCalls++;
          await listGate?.future;
          final error = listFailure;
          if (error != null) throw error;
          return ListMyMeasuresResponse(measures: measures);
        })
        .unary(SellerAppService.submitAppeal, (request, _) async {
          submits.add(request);
          await submitGate?.future;
          final error = submitFailure;
          if (error != null) throw error;
          return SubmitAppealResponse(
            measure: measureOf(
              canAppeal: false,
              state: MeasureState.MEASURE_STATE_ENDED,
              appeal: MeasureRequest(
                state: MeasureRequestState.MEASURE_REQUEST_STATE_PENDING,
              ),
            ),
          );
        })
        .unary(SellerAppService.requestRemediation, (request, _) async {
          remediations.add(request);
          final error = submitFailure;
          if (error != null) throw error;
          return RequestRemediationResponse(
            measure: riskSuspension(
              remediationRequests: [
                MeasureRequest(
                  state: MeasureRequestState.MEASURE_REQUEST_STATE_PENDING,
                ),
              ],
            ),
          );
        })
        .unary(SellerAppService.getMyMeasure, (request, _) async {
          gets.add(request);
          await getGate?.future;
          final answer = getAnswer;
          if (answer is! Measure) throw answer!;
          return GetMyMeasureResponse(measure: answer);
        })
        .build(),
  );
}

class _Uploads {
  final uploaded = <String>[];
  final gates = <Completer<void>>[];
  Object? failure;

  PrivateDocumentUploader get uploader => (bytes, docType, filename) async {
    uploaded.add('$docType:$filename');
    final index = uploaded.length - 1;
    if (gates.length > index) await gates[index].future;
    final error = failure;
    if (error != null) throw error;
    return 'token-${uploaded.length}';
  };
}

Future<GoRouter> _pump(
  WidgetTester tester,
  _Server server, {
  _Uploads? uploads,
  String initial = SellerRoutes.measures,
  EvidencePicker? picker,
}) async {
  tester.view.physicalSize = const Size(800, 2400);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.reset);
  final router = GoRouter(
    initialLocation: initial,
    routes: [
      GoRoute(
        path: '/',
        builder: (_, _) => const Scaffold(body: Text('ROOT')),
      ),
      GoRoute(
        path: MainRoutes.settings,
        builder: (_, _) => const Scaffold(body: Text('SETTINGS')),
      ),
      GoRoute(
        path: SellerRoutes.measures,
        builder: (_, _) => const SellerAppealScreen(),
      ),
      GoRoute(
        path: SellerRoutes.measureRequest,
        redirect: (_, state) => measureRequestRedirect(state.uri),
        builder: (_, state) => SellerMeasureRequestFormScreen(
          measureId: state.pathParameters['measureId']!,
          kind: measureFormKindOf(state.uri)!,
          pickEvidence: picker,
        ),
      ),
    ],
  );
  addTearDown(router.dispose);
  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        sessionProvider.overrideWith(_LoggedIn.new),
        sellerClientProvider.overrideWithValue(server.client),
        if (uploads != null)
          privateDocumentUploaderProvider.overrideWithValue(uploads.uploader),
      ],
      child: ScreenUtilInit(
        designSize: const Size(390, 844),
        builder: (_, _) => MaterialApp.router(routerConfig: router),
      ),
    ),
  );
  await tester.pumpAndSettle();
  return router;
}

/// 成功提示（FloatingToast）有 2 秒的計時器，測試結束前要走完。
Future<void> _drainToast(WidgetTester tester) async {
  await tester.pump(const Duration(seconds: 3));
  await tester.pumpAndSettle();
}

Finder get _reasonField => find.byKey(const ValueKey('request-reason'));

FilledButton _submitButton(WidgetTester tester) =>
    tester.widget<FilledButton>(find.widgetWithText(FilledButton, '送出申覆'));

Future<void> _openForm(WidgetTester tester) async {
  await tester.tap(find.text('提出申覆'));
  await tester.pumpAndSettle();
}

Finder get _remediationButton =>
    find.byKey(const ValueKey('measure-remediation-button'));

void main() {
  group('已改善，請求解除（按鈕）', () {
    final cooldownEnd = DateTime.utc(2026, 11, 1, 6, 5); // 臺北 2026/11/01 14:05
    final pending = MeasureRequestState.MEASURE_REQUEST_STATE_PENDING;

    Future<void> pumpOne(WidgetTester tester, Measure measure) =>
        _pump(tester, _Server()..measures = [measure]);

    bool enabled(WidgetTester tester) =>
        tester.widget<OutlinedButton>(_remediationButton).onPressed != null;

    testWidgets('RISK 生效中且 can_request_remediation：啟用，沒有說明', (tester) async {
      await pumpOne(tester, riskSuspension(canRequestRemediation: true));
      expect(find.text('已改善，請求解除'), findsOneWidget);
      expect(enabled(tester), isTrue);
      expect(
        find.byKey(const ValueKey('measure-remediation-reason')),
        findsNothing,
      );
    });

    testWidgets('解除請求審核中：停用並顯示伺服器的句子（優先於申覆與冷卻）', (tester) async {
      await pumpOne(
        tester,
        riskSuspension(
          remediationRequests: [MeasureRequest(state: pending)],
          appeal: MeasureRequest(state: pending),
          nextRemediationTime: cooldownEnd,
          remediationUnavailableReasons: [
            reasonRemediationPending,
            reasonAppealPending,
            reasonCooldown,
          ],
        ),
      );
      expect(enabled(tester), isFalse);
      expect(
        tester
            .widget<Text>(
              find.byKey(const ValueKey('measure-remediation-reason')),
            )
            .data,
        remediationPendingSentence,
      );
    });

    testWidgets('申覆審核中：停用並說明', (tester) async {
      await pumpOne(
        tester,
        riskSuspension(
          appeal: MeasureRequest(state: pending),
          nextRemediationTime: cooldownEnd,
          remediationUnavailableReasons: [reasonAppealPending, reasonCooldown],
        ),
      );
      expect(enabled(tester), isFalse);
      expect(
        tester
            .widget<Text>(
              find.byKey(const ValueKey('measure-remediation-reason')),
            )
            .data,
        remediationAppealPendingSentence,
      );
    });

    testWidgets('冷卻期：停用並顯示臺北時間的可再提出時間', (tester) async {
      await pumpOne(
        tester,
        riskSuspension(
          nextRemediationTime: cooldownEnd,
          remediationUnavailableReasons: [reasonCooldown],
        ),
      );
      expect(enabled(tester), isFalse);
      expect(find.text(remediationCooldownSentence), findsOneWidget);
    });

    testWidgets('RISK 已結束與 VIOLATION：不渲染', (tester) async {
      await pumpOne(
        tester,
        riskSuspension(
          state: MeasureState.MEASURE_STATE_ENDED,
          remediationUnavailableReasons: [reasonNotActive],
        ),
      );
      expect(find.text('已改善，請求解除'), findsNothing);
      await pumpOne(
        tester,
        measureOf(remediationUnavailableReasons: [reasonNotRisk]),
      );
      expect(find.text('已改善，請求解除'), findsNothing);
    });

    testWidgets('啟用時點下去開請求解除的表單（標題「請求解除措施」）', (tester) async {
      await pumpOne(tester, riskSuspension(canRequestRemediation: true));
      await tester.tap(_remediationButton);
      await tester.pumpAndSettle();
      expect(find.text('請求解除措施'), findsOneWidget);
      expect(find.text('改善說明 *'), findsOneWidget);
    });

    testWidgets('卡片狀態：沒有待處理時顯示最後裁定的結果與理由', (tester) async {
      await pumpOne(
        tester,
        riskSuspension(
          canRequestRemediation: true,
          remediationRequests: [
            MeasureRequest(
              state: MeasureRequestState.MEASURE_REQUEST_STATE_REJECTED,
              decisionReason: '尚未改善',
              decideTime: ts(DateTime.utc(2026, 10, 9)),
            ),
          ],
        ),
      );
      expect(find.text('解除請求已駁回'), findsOneWidget);
      expect(find.text('平台回覆：尚未改善'), findsOneWidget);
    });
  });

  group('卡片的冷卻期顯示', () {
    testWidgets('冷卻期只出現在按鈕的反灰原因；卡片的說明行是最新裁定的理由，不帶冷卻時間', (tester) async {
      await _pump(
        tester,
        _Server()
          ..measures = [
            riskSuspension(
              remediationRequests: [
                MeasureRequest(
                  state: MeasureRequestState.MEASURE_REQUEST_STATE_REJECTED,
                  decisionReason: '尚未改善',
                  decideTime: ts(DateTime.utc(2026, 10, 9)),
                ),
              ],
              nextRemediationTime: DateTime.utc(2026, 11, 1, 6, 5, 30),
              remediationUnavailableReasons: [reasonCooldown],
            ),
          ],
      );
      expect(find.text('解除請求已駁回'), findsOneWidget);
      expect(find.text('平台回覆：尚未改善'), findsOneWidget);
      expect(find.textContaining('後可再提出'), findsOneWidget);
      expect(find.text(remediationCooldownSentence), findsOneWidget);
      expect(find.textContaining('平台回覆：2026'), findsNothing);
    });

    testWidgets('已結案（CLOSED）的請求：沒有 chip 也沒有說明', (tester) async {
      await _pump(
        tester,
        _Server()
          ..measures = [
            riskSuspension(
              state: MeasureState.MEASURE_STATE_REVOKED,
              remediationRequests: [
                MeasureRequest(
                  state: MeasureRequestState.MEASURE_REQUEST_STATE_CLOSED,
                  closeCause: MeasureRequestCloseCause
                      .MEASURE_REQUEST_CLOSE_CAUSE_REVOKED,
                ),
              ],
            ),
          ],
      );
      expect(find.textContaining('措施已撤銷'), findsNothing);
      expect(find.textContaining('解除請求'), findsNothing);
    });
  });

  group('措施與申覆列表', () {
    testWidgets('分成「需要處理」與「歷史」，每列顯示類別、措施時間、說明與目前的影響', (tester) async {
      final server = _Server()
        ..measures = [
          activeSuspension(id: 'a'),
          measureOf(
            id: 'b',
            canAppeal: false,
            findings: [finding(category: PolicyCategory.POLICY_CATEGORY_FRAUD)],
            statement: '重複下單詐欺',
            state: MeasureState.MEASURE_STATE_REVOKED,
            appeal: MeasureRequest(
              state: MeasureRequestState.MEASURE_REQUEST_STATE_ACCEPTED,
              decisionReason: '申覆成立',
            ),
          ),
          measureOf(
            id: 'c',
            canAppeal: false,
            kind: MeasureKind.MEASURE_KIND_WARNING,
            state: MeasureState.MEASURE_STATE_ISSUED,
            findings: [
              finding(category: PolicyCategory.POLICY_CATEGORY_PLATFORM_ABUSE),
            ],
            statement: '口頭警告',
          ),
        ];
      await _pump(tester, server);

      expect(find.text('需要處理'), findsOneWidget);
      expect(find.text('歷史'), findsOneWidget);
      // 需要處理在歷史之前。
      expect(
        tester.getTopLeft(find.text('需要處理')).dy,
        lessThan(tester.getTopLeft(find.text('歷史')).dy),
      );
      expect(find.text('食品安全'), findsOneWidget);
      expect(find.text('商品標示與實品不符'), findsOneWidget);
      expect(find.text('嚴重程度：高 · 措施：停權'), findsNWidgets(2));
      // create_time 2026-10-05 01:30 UTC ＝ 臺北 09:30。
      expect(find.text('措施時間：2026/10/05 09:30'), findsNWidgets(3));
      expect(find.text('停權中（至 2026/10/19 16:30）'), findsOneWidget);
      expect(find.text('已撤銷'), findsOneWidget);
      expect(find.text('已發出'), findsNWidgets(1));
      // 申覆狀態用 chip；沒有申覆的兩列沒有 chip。
      expect(find.text('申覆已通過'), findsOneWidget);
      expect(find.textContaining('申覆審核中'), findsNothing);
      expect(find.text('平台回覆：申覆成立'), findsOneWidget);
    });

    testWidgets('風險依據的開放式停權與依據認定被撤銷的措施', (tester) async {
      final server = _Server()
        ..measures = [
          riskSuspension(),
          measureOf(
            id: 'x',
            canAppeal: false,
            state: MeasureState.MEASURE_STATE_REVOKED,
            revokeReason: RevokeReason.REVOKE_REASON_FINDING_REVOKED,
          ),
        ];
      await _pump(tester, server);

      expect(find.text('風險：交易風險'), findsOneWidget);
      expect(find.text('停權中（未定結束時間，平台定期複審）'), findsOneWidget);
      expect(find.text('依據的認定已撤銷'), findsOneWidget);
      expect(find.textContaining('1970'), findsNothing);
    });

    testWidgets('沒有 create_time：不顯示「措施時間」，也不出現 1970', (tester) async {
      final server = _Server()..measures = [measureOf(withCreateTime: false)];
      await _pump(tester, server);

      expect(find.textContaining('措施時間'), findsNothing);
      expect(find.textContaining('1970'), findsNothing);
    });

    testWidgets('「提出申覆」按鈕一律顯示，只有 can_appeal 為真時可按', (tester) async {
      final server = _Server()
        ..measures = [
          activeSuspension(id: 'a', canAppeal: false),
          measureOf(id: 'b', canAppeal: true),
        ];
      await _pump(tester, server);

      expect(_enabledAppealButton, findsOneWidget);
      expect(find.widgetWithText(OutlinedButton, '提出申覆'), findsNWidgets(2));
    });

    testWidgets('停權生效中但不能申覆：在「需要處理」，申覆按鈕反灰', (tester) async {
      final server = _Server()..measures = [activeSuspension(canAppeal: false)];
      await _pump(tester, server);

      expect(find.text('需要處理'), findsOneWidget);
      expect(find.text('歷史'), findsNothing);
      expect(_enabledAppealButton, findsNothing);
    });

    testWidgets('沒有措施：空狀態文案同 website', (tester) async {
      await _pump(tester, _Server());

      expect(find.text('目前沒有措施紀錄'), findsOneWidget);
      expect(find.text('保持良好營運狀況，這裡會維持空空如也。'), findsOneWidget);
      expect(find.text('需要處理'), findsNothing);
    });

    testWidgets('讀取失敗：錯誤與重試，重試成功後顯示列表', (tester) async {
      final server = _Server()
        ..listFailure = ConnectException(Code.unavailable, 'down')
        ..measures = [measureOf()];
      await _pump(tester, server);

      expect(find.text('無法載入措施紀錄'), findsOneWidget);
      expect(find.text('目前沒有措施紀錄'), findsNothing);

      server.listFailure = null;
      await tester.tap(find.text('重試'));
      await tester.pumpAndSettle();

      expect(find.text('無法載入措施紀錄'), findsNothing);
      expect(find.text('食品安全'), findsOneWidget);
    });

    testWidgets('重試期間顯示讀取中，不停在錯誤面板；失敗時再回到錯誤', (tester) async {
      final server = _Server()
        ..listFailure = ConnectException(Code.unavailable, 'down');
      await _pump(tester, server);
      expect(find.text('無法載入措施紀錄'), findsOneWidget);

      final gate = server.listGate = Completer<void>();
      await tester.tap(find.text('重試'));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 50));
      expect(find.text('無法載入措施紀錄'), findsNothing);
      expect(find.byType(TruckLoadingOverlay), findsOneWidget);

      gate.complete();
      server.listGate = null;
      await tester.pumpAndSettle();
      expect(find.text('無法載入措施紀錄'), findsOneWidget);
      expect(find.byType(TruckLoadingOverlay), findsNothing);
    });

    testWidgets('點「提出申覆」開全螢幕表單（路由），返回後回到列表', (tester) async {
      final server = _Server()..measures = [measureOf()];
      final router = await _pump(tester, server);

      await _openForm(tester);

      expect(router.state.uri.toString(), SellerRoutes.measureRequestFor('m1'));
      expect(find.byType(Dialog), findsNothing);
      expect(find.byType(BottomSheet), findsNothing);
      expect(find.text('送出申覆'), findsOneWidget);

      await tester.tap(find.text('返回'));
      await tester.pumpAndSettle();
      expect(find.text('需要處理'), findsOneWidget);
    });
  });

  group('申覆表單的路由以措施 id 開啟', () {
    testWidgets('不經列表直接開啟：用 GetMyMeasure 讀，不為了找一筆去讀整個列表', (tester) async {
      final server = _Server()..getAnswer = measureOf(id: 'm9');
      await _pump(
        tester,
        server,
        initial: SellerRoutes.measureRequestFor('m9'),
      );

      expect(find.text('送出申覆'), findsOneWidget);
      expect(server.gets.single.measureId, 'm9');
      expect(server.listCalls, 0);

      await tester.enterText(_reasonField, '判定有誤');
      await tester.pump();
      await tester.tap(find.text('送出申覆'));
      await tester.pumpAndSettle();
      expect(server.submits.single.measureId, 'm9');
      // 沒有上一頁（網址直接開啟）：送出後改去措施與申覆列表，不是丟「nothing to pop」。
      expect(find.text('措施與申覆'), findsOneWidget);
      await _drainToast(tester);

      // 列表是唯一一頁：返回不能丟「nothing to pop」，改去設定分頁（常駐入口所在）。
      await tester.tap(find.byTooltip('返回'));
      await tester.pumpAndSettle();
      expect(tester.takeException(), isNull);
      expect(find.text('SETTINGS'), findsOneWidget);
    });

    testWidgets('從列表開啟：用列表快取的那一筆，不再打 GetMyMeasure；id 含特殊字元也完整帶到', (
      tester,
    ) async {
      final server = _Server()..measures = [measureOf(id: 'a/b c')];
      await _pump(tester, server);
      await _openForm(tester);

      expect(find.text('送出申覆'), findsOneWidget);
      expect(server.gets, isEmpty);
    });

    testWidgets('列表有但沒有這個 id：用 GetMyMeasure 讀', (tester) async {
      final server = _Server()
        ..measures = [measureOf(id: 'other')]
        ..getAnswer = measureOf(id: 'm2');
      final router = await _pump(tester, server);
      unawaited(router.push(SellerRoutes.measureRequestFor('m2')));
      await tester.pumpAndSettle();

      expect(find.text('送出申覆'), findsOneWidget);
      expect(server.gets.single.measureId, 'm2');
    });

    testWidgets('讀取失敗：錯誤與重試，重試成功後顯示表單', (tester) async {
      final server = _Server()
        ..getAnswer = ConnectException(Code.unavailable, 'x');
      await _pump(
        tester,
        server,
        initial: SellerRoutes.measureRequestFor('m1'),
      );

      expect(find.text('無法載入措施紀錄'), findsOneWidget);
      expect(find.text('送出申覆'), findsNothing);

      server.getAnswer = measureOf();
      await tester.tap(find.text('重試'));
      await tester.pumpAndSettle();
      expect(find.text('送出申覆'), findsOneWidget);
    });

    for (final code in [Code.notFound, Code.permissionDenied]) {
      testWidgets('讀取失敗是 $code：終態「找不到這項措施」，沒有重試，按鈕回措施與申覆', (tester) async {
        final server = _Server()
          ..measures = [measureOf(id: 'other')]
          ..getAnswer = ConnectException(code, 'x');
        await _pump(
          tester,
          server,
          initial: SellerRoutes.measureRequestFor('m1'),
        );

        expect(find.text('找不到這項措施'), findsOneWidget);
        expect(find.text('無法載入措施紀錄'), findsNothing);
        expect(find.text('重試'), findsNothing);
        expect(find.text('送出申覆'), findsNothing);

        await tester.tap(find.text('回到措施與申覆'));
        await tester.pumpAndSettle();
        expect(find.text('找不到這項措施'), findsNothing);
        expect(find.text('措施與申覆'), findsOneWidget);
      });
    }

    testWidgets('重試期間表單顯示讀取中，不停在錯誤面板', (tester) async {
      final server = _Server()
        ..getAnswer = ConnectException(Code.unavailable, 'x');
      await _pump(
        tester,
        server,
        initial: SellerRoutes.measureRequestFor('m1'),
      );
      expect(find.text('無法載入措施紀錄'), findsOneWidget);

      final gate = server.getGate = Completer<void>();
      await tester.tap(find.text('重試'));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 50));
      expect(find.text('無法載入措施紀錄'), findsNothing);
      expect(find.byType(TruckLoadingOverlay), findsOneWidget);

      server.getAnswer = measureOf();
      gate.complete();
      await tester.pumpAndSettle();
      expect(find.text('送出申覆'), findsOneWidget);
    });

    testWidgets('表單開著時重新讀取失敗：表單與已輸入的理由保留，不換成錯誤面板', (tester) async {
      final server = _Server()..getAnswer = measureOf(id: 'm1');
      await _pump(
        tester,
        server,
        initial: SellerRoutes.measureRequestFor('m1'),
      );
      await tester.enterText(_reasonField, '判定有誤');
      await tester.pump();

      server.getAnswer = ConnectException(Code.unavailable, 'x');
      ProviderScope.containerOf(
        tester.element(find.byType(SellerMeasureRequestFormScreen)),
      ).invalidate(measureByIdProvider('acc-1', 'm1'));
      await tester.pumpAndSettle();

      expect(server.gets.length, 2);
      expect(find.text('無法載入措施紀錄'), findsNothing);
      expect(find.text('送出申覆'), findsOneWidget);
      expect(find.text('判定有誤'), findsOneWidget);
    });

    testWidgets('讀到的措施已經不能申覆：說明原因，不顯示表單', (tester) async {
      final server = _Server()
        ..getAnswer = measureOf(
          canAppeal: false,
          appeal: MeasureRequest(
            state: MeasureRequestState.MEASURE_REQUEST_STATE_PENDING,
          ),
        );
      await _pump(
        tester,
        server,
        initial: SellerRoutes.measureRequestFor('m1'),
      );

      expect(find.text(appealAlreadyAppealedSentence), findsOneWidget);
      expect(find.text('請回到措施與申覆查看目前狀態。'), findsOneWidget);
      expect(find.text('送出申覆'), findsNothing);
    });
  });

  group('申覆表單', () {
    Future<_Server> openForm(
      WidgetTester tester, {
      _Uploads? uploads,
      EvidencePicker? picker,
      Measure? measure,
    }) async {
      final server = _Server()
        ..measures = [
          measure ?? measureOf(state: MeasureState.MEASURE_STATE_ENDED),
        ];
      await _pump(tester, server, uploads: uploads, picker: picker);
      await _openForm(tester);
      return server;
    }

    testWidgets('顯示平台的說明；理由空白不能送出；有內容後可以', (tester) async {
      await openForm(tester);

      expect(find.text('商品標示與實品不符'), findsOneWidget);
      expect(find.text('申覆理由 *'), findsOneWidget);
      expect(find.text('佐證圖片（選填）'), findsOneWidget);
      expect(find.text('0 / 2000'), findsOneWidget);
      expect(_submitButton(tester).onPressed, isNull);

      await tester.enterText(_reasonField, '   ');
      await tester.pump();
      expect(_submitButton(tester).onPressed, isNull);

      await tester.enterText(_reasonField, '判定有誤');
      await tester.pump();
      expect(_submitButton(tester).onPressed, isNotNull);
      expect(find.text('4 / 2000'), findsOneWidget);
    });

    testWidgets('字數以 trim 後的 rune 計：2000 個 emoji 剛好可以送，2001 個不行', (
      tester,
    ) async {
      await openForm(tester);

      await tester.enterText(_reasonField, '  ${'😀' * 2000}  ');
      await tester.pump();
      expect(find.text('2000 / 2000'), findsOneWidget);
      expect(_submitButton(tester).onPressed, isNotNull);
      expect(find.text('申覆理由最多 2000 字'), findsNothing);

      await tester.enterText(_reasonField, '😀' * 2001);
      await tester.pump();
      expect(find.text('2001 / 2000'), findsOneWidget);
      expect(find.text('申覆理由最多 2000 字'), findsOneWidget);
      expect(_submitButton(tester).onPressed, isNull);
    });

    testWidgets('送出：帶 trim 後的理由與讀到的 etag，成功後回到列表，回傳的那一筆直接取代原列、不重讀', (
      tester,
    ) async {
      final server = await openForm(tester);
      final before = server.listCalls;

      await tester.enterText(_reasonField, '  判定有誤  ');
      await tester.pump();
      await tester.tap(find.text('送出申覆'));
      await tester.pumpAndSettle();

      final request = server.submits.single;
      expect(request.measureId, 'm1');
      expect(request.reason, '判定有誤');
      expect(request.etag, 'etag-1');
      // 回應的那一筆（can_appeal 為假）取代原列：離開「需要處理」、沒有申覆按鈕，列表沒有再讀。
      expect(find.text('需要處理'), findsNothing);
      expect(find.text('歷史'), findsOneWidget);
      expect(_enabledAppealButton, findsNothing);
      expect(server.listCalls, before);
      await _drainToast(tester);
    });

    testWidgets('送出成功：顯示成功提示「申覆已送出，請等候審核」', (tester) async {
      await openForm(tester);

      await tester.enterText(_reasonField, '判定有誤');
      await tester.pump();
      await tester.tap(find.text('送出申覆'));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 500));

      expect(find.text('申覆已送出，請等候審核'), findsOneWidget);
      await _drainToast(tester);
    });

    testWidgets('送出成功後列表再讀失敗也不會停在過期的 can_appeal（取代的是快取，不靠重讀）', (tester) async {
      final server = await openForm(tester);
      server.listFailure = StateError('offline');

      await tester.enterText(_reasonField, '判定有誤');
      await tester.pump();
      await tester.tap(find.text('送出申覆'));
      await tester.pumpAndSettle();

      expect(_enabledAppealButton, findsNothing);
      expect(find.text('無法載入措施紀錄'), findsNothing);
      expect(find.text('歷史'), findsOneWidget);
      await _drainToast(tester);
    });

    testWidgets('送出成功後，停權公告頁讀的「生效中」列表也換成回傳的那一筆（不再是生效中就移除）', (tester) async {
      final server = await openForm(tester);
      final container = ProviderScope.containerOf(
        tester.element(find.byType(MaterialApp)),
      );
      final activeProvider = myMeasuresProvider(
        'acc-1',
        MeasureState.MEASURE_STATE_ACTIVE,
      );
      final sub = container.listen(activeProvider, (_, _) {});
      addTearDown(sub.close);
      expect((await container.read(activeProvider.future)).map((m) => m.id), [
        'm1',
      ]);

      await tester.enterText(_reasonField, '判定有誤');
      await tester.pump();
      await tester.tap(find.text('送出申覆'));
      await tester.pumpAndSettle();

      expect(server.submits, hasLength(1));
      expect(container.read(activeProvider).requireValue, isEmpty);
      await _drainToast(tester);
    });

    testWidgets('送出中：按鈕顯示「申覆送出中…」且不能重複送出', (tester) async {
      final server = await openForm(tester);
      server.submitGate = Completer<void>();

      await tester.enterText(_reasonField, '判定有誤');
      await tester.pump();
      await tester.tap(find.text('送出申覆'));
      await tester.pump();

      expect(find.text('申覆送出中…'), findsOneWidget);
      expect(
        tester.widget<FilledButton>(find.byType(FilledButton)).onPressed,
        isNull,
      );
      expect(server.submits, hasLength(1));

      server.submitGate!.complete();
      await tester.pumpAndSettle();
      expect(server.submits, hasLength(1));
      await _drainToast(tester);
    });

    testWidgets('衝突後仍可申覆：保留輸入、顯示 CONFLICT_NOTICE、下一次送出用新的 etag', (
      tester,
    ) async {
      final server = await openForm(tester);
      server
        ..submitFailure = reasonError(Code.aborted, 'VERSION_CONFLICT')
        ..getAnswer = measureOf(etag: 'etag-2', statement: '更新後的說明');

      await tester.enterText(_reasonField, '判定有誤');
      await tester.pump();
      await tester.tap(find.text('送出申覆'));
      await tester.pumpAndSettle();

      expect(find.text('這項措施已更新，已載入最新內容，請確認後再送出申覆'), findsOneWidget);
      expect(find.text('更新後的說明'), findsOneWidget);
      expect(tester.widget<TextField>(_reasonField).controller?.text, '判定有誤');
      expect(find.text('送出申覆'), findsOneWidget);

      server.submitFailure = null;
      await tester.tap(find.text('送出申覆'));
      await tester.pumpAndSettle();

      expect(server.submits.last.etag, 'etag-2');
      expect(find.text('歷史'), findsOneWidget);
      await _drainToast(tester);
    });

    testWidgets('衝突後已申覆：關閉表單回列表並提示', (tester) async {
      final server = await openForm(tester);
      server
        ..submitFailure = reasonError(Code.aborted, 'VERSION_CONFLICT')
        ..getAnswer = measureOf(
          canAppeal: false,
          appeal: MeasureRequest(
            state: MeasureRequestState.MEASURE_REQUEST_STATE_PENDING,
          ),
        );

      await tester.enterText(_reasonField, '判定有誤');
      await tester.pump();
      await tester.tap(find.text('送出申覆'));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 500));

      // 提示與卡片按鈕旁的說明是同一句伺服器的句子。
      expect(find.text(appealAlreadyAppealedSentence), findsNWidgets(2));
      await tester.pump(const Duration(seconds: 3));
      await tester.pumpAndSettle();
      expect(find.text('送出申覆'), findsNothing);
      // 伺服器的最新一筆（已申覆）直接寫進列表。
      expect(find.text('申覆審核中'), findsOneWidget);
      expect(_enabledAppealButton, findsNothing);
    });

    testWidgets('failed_precondition 且已撤銷：關閉表單並提示不需要申覆', (tester) async {
      final server = await openForm(tester);
      server
        ..submitFailure = ConnectException(Code.failedPrecondition, 'x')
        ..getAnswer = measureOf(
          canAppeal: false,
          state: MeasureState.MEASURE_STATE_REVOKED,
        );

      await tester.enterText(_reasonField, '判定有誤');
      await tester.pump();
      await tester.tap(find.text('送出申覆'));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 500));

      expect(find.text(appealRevokedSentence), findsNWidgets(2));
      await tester.pump(const Duration(seconds: 3));
      await tester.pumpAndSettle();
      expect(find.text('送出申覆'), findsNothing);
    });

    testWidgets('重送同一個申覆（failed_precondition，重讀已有申覆）：關閉表單並說明已經申覆，列表換成最新一筆', (
      tester,
    ) async {
      final server = await openForm(tester);
      server
        ..submitFailure = ConnectException(Code.failedPrecondition, '已申覆')
        ..getAnswer = measureOf(
          canAppeal: false,
          state: MeasureState.MEASURE_STATE_ENDED,
          appeal: MeasureRequest(
            state: MeasureRequestState.MEASURE_REQUEST_STATE_PENDING,
          ),
        );

      await tester.enterText(_reasonField, '判定有誤');
      await tester.pump();
      await tester.tap(find.text('送出申覆'));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 500));

      expect(find.text(appealAlreadyAppealedSentence), findsNWidgets(2));
      expect(server.gets.single.measureId, 'm1');
      await tester.pump(const Duration(seconds: 3));
      await tester.pumpAndSettle();
      expect(find.text('送出申覆'), findsNothing);
      expect(find.text('申覆審核中'), findsOneWidget);
      expect(_enabledAppealButton, findsNothing);
    });

    testWidgets('理由欄位的錯誤顯示在欄位上，輸入後清除', (tester) async {
      final server = await openForm(tester);
      server.submitFailure = fieldError('reason', '申覆理由不夠具體');

      await tester.enterText(_reasonField, '判定有誤');
      await tester.pump();
      await tester.tap(find.text('送出申覆'));
      await tester.pumpAndSettle();

      expect(find.text('申覆理由不夠具體'), findsOneWidget);
      expect(find.text('送出申覆'), findsOneWidget);

      await tester.enterText(_reasonField, '判定有誤，補充說明');
      await tester.pump();
      expect(find.text('申覆理由不夠具體'), findsNothing);
    });

    testWidgets('其他錯誤顯示伺服器的訊息並保留輸入', (tester) async {
      final server = await openForm(tester);
      server.submitFailure = fieldError('etag', '必須提供 etag');

      await tester.enterText(_reasonField, '判定有誤');
      await tester.pump();
      await tester.tap(find.text('送出申覆'));
      await tester.pumpAndSettle();

      expect(find.byKey(const ValueKey('request-notice')), findsOneWidget);
      expect(find.text('必須提供 etag'), findsOneWidget);
      expect(tester.widget<TextField>(_reasonField).controller?.text, '判定有誤');
    });

    testWidgets('佐證欄位沒有索引的錯誤：訊息顯示在佐證區，不標任何一張，輸入與照片保留且仍可再送', (tester) async {
      final uploads = _Uploads();
      final server = await openForm(
        tester,
        uploads: uploads,
        picker: (_) async => (bytes: _png, filename: 'a.jpg'),
      );
      await tester.enterText(_reasonField, '判定有誤');
      await tester.pump();
      await tester.tap(find.bySemanticsLabel('新增照片'));
      await tester.pumpAndSettle();
      server.submitFailure = fieldError('evidence_staged_tokens', '佐證最多 5 張');

      await tester.tap(find.text('送出申覆'));
      await tester.pumpAndSettle();

      expect(
        find.byKey(const ValueKey('request-evidence-error')),
        findsOneWidget,
      );
      expect(find.text('佐證最多 5 張'), findsOneWidget);
      expect(find.byKey(const ValueKey('request-notice')), findsNothing);
      expect(find.byType(Image), findsOneWidget);
      expect(_submitButton(tester).onPressed, isNotNull); // 沒有任何一張被標成失敗
      expect(tester.widget<TextField>(_reasonField).controller?.text, '判定有誤');
    });

    testWidgets('佐證欄位的錯誤在照片增減後清除（新增或移除任一張）', (tester) async {
      final uploads = _Uploads();
      final server = await openForm(
        tester,
        uploads: uploads,
        picker: (_) async => (bytes: _png, filename: 'a.jpg'),
      );
      await tester.enterText(_reasonField, '判定有誤');
      await tester.pump();
      await tester.tap(find.bySemanticsLabel('新增照片'));
      await tester.pumpAndSettle();
      server.submitFailure = fieldError('evidence_staged_tokens', '佐證最多 5 張');
      await tester.tap(find.text('送出申覆'));
      await tester.pumpAndSettle();
      expect(find.text('佐證最多 5 張'), findsOneWidget);

      await tester.tap(find.bySemanticsLabel('新增照片'));
      await tester.pumpAndSettle();
      expect(find.text('佐證最多 5 張'), findsNothing);

      await tester.tap(find.text('送出申覆'));
      await tester.pumpAndSettle();
      expect(find.text('佐證最多 5 張'), findsOneWidget);
      await tester.tap(find.bySemanticsLabel('移除照片').last);
      await tester.pump();
      expect(find.text('佐證最多 5 張'), findsNothing);
    });

    testWidgets('佐證 token 被伺服器拒絕：該張標成失敗並帶伺服器訊息、移除後可再送，其他張與輸入保留', (
      tester,
    ) async {
      final uploads = _Uploads();
      final server = await openForm(
        tester,
        uploads: uploads,
        picker: (_) async => (bytes: _png, filename: 'a.jpg'),
      );
      await tester.enterText(_reasonField, '判定有誤');
      await tester.pump();
      for (var i = 0; i < 2; i++) {
        await tester.tap(find.bySemanticsLabel('新增照片'));
        await tester.pumpAndSettle();
      }
      server.submitFailure = fieldError('evidence_staged_tokens[1]', '佐證圖片已過期');

      await tester.tap(find.text('送出申覆'));
      await tester.pumpAndSettle();

      expect(server.submits.single.evidenceStagedTokens, [
        'token-1',
        'token-2',
      ]);
      expect(find.text('佐證圖片已過期'), findsOneWidget); // 表單提示
      expect(find.bySemanticsLabel('佐證圖片已過期'), findsWidgets); // 失敗的縮圖
      expect(find.byType(Image), findsNWidgets(2));
      expect(_submitButton(tester).onPressed, isNull); // 壞圖還在，重送只會再失敗
      expect(tester.widget<TextField>(_reasonField).controller?.text, '判定有誤');

      // 移除第二張（失敗的那張）。
      await tester.tap(find.bySemanticsLabel('移除照片').last);
      await tester.pump();
      expect(find.byType(Image), findsOneWidget);
      expect(_submitButton(tester).onPressed, isNotNull);
      expect(find.text('佐證圖片已過期'), findsNothing); // 該張的提示跟著照片移除

      server.submitFailure = null;
      await tester.tap(find.text('送出申覆'));
      await tester.pumpAndSettle();
      expect(server.submits.last.evidenceStagedTokens, ['token-1']);
      await _drainToast(tester);
    });

    testWidgets(
      '佐證圖片：選好就上傳（docType appeal_evidence），上傳完才可送出，送出帶 staged token',
      (tester) async {
        final uploads = _Uploads()..gates.add(Completer<void>());
        final server = await openForm(
          tester,
          uploads: uploads,
          picker: (_) async => (bytes: _png, filename: 'a.jpg'),
        );

        await tester.enterText(_reasonField, '判定有誤');
        await tester.pump();
        await tester.tap(find.bySemanticsLabel('新增照片'));
        await tester.pump();

        expect(uploads.uploaded, ['appeal_evidence:a.jpg']);
        // 上傳中不能送出。
        expect(_submitButton(tester).onPressed, isNull);

        uploads.gates.first.complete();
        await tester.pump();
        await tester.pump();
        expect(_submitButton(tester).onPressed, isNotNull);

        await tester.tap(find.text('送出申覆'));
        await tester.pumpAndSettle();
        expect(server.submits.single.evidenceStagedTokens, ['token-1']);
        await _drainToast(tester);
      },
    );

    testWidgets('最多 5 張：滿了就沒有新增入口；可以移除', (tester) async {
      final uploads = _Uploads();
      await openForm(
        tester,
        uploads: uploads,
        picker: (_) async => (bytes: _png, filename: 'a.jpg'),
      );

      for (var i = 0; i < 5; i++) {
        await tester.tap(find.bySemanticsLabel('新增照片'));
        await tester.pumpAndSettle();
      }
      expect(uploads.uploaded, hasLength(5));
      expect(find.bySemanticsLabel('新增照片'), findsNothing);

      await tester.tap(find.bySemanticsLabel('移除照片').first);
      await tester.pump();
      expect(find.bySemanticsLabel('新增照片'), findsOneWidget);
      await _drainToast(tester);
    });

    testWidgets('選圖器開著時送出了：選好的照片不加進表單、不上傳', (tester) async {
      final uploads = _Uploads();
      final pick = Completer<PickedEvidence?>();
      final server = await openForm(
        tester,
        uploads: uploads,
        picker: (_) => pick.future,
      );
      server.submitGate = Completer<void>();

      await tester.enterText(_reasonField, '判定有誤');
      await tester.pump();
      await tester.tap(find.bySemanticsLabel('新增照片'));
      await tester.pump();
      await tester.tap(find.text('送出申覆'));
      await tester.pump();
      expect(find.text('申覆送出中…'), findsOneWidget);

      pick.complete((bytes: _png, filename: 'a.jpg'));
      await tester.pump();
      expect(find.byType(Image), findsNothing);
      expect(uploads.uploaded, isEmpty);

      server.submitGate!.complete();
      await tester.pumpAndSettle();
      expect(server.submits.single.evidenceStagedTokens, isEmpty);
      await _drainToast(tester);
    });

    testWidgets('上傳中移除照片：上傳成功的結果被丟掉，送出不帶這個 token', (tester) async {
      final uploads = _Uploads()..gates.add(Completer<void>());
      final server = await openForm(
        tester,
        uploads: uploads,
        picker: (_) async => (bytes: _png, filename: 'a.jpg'),
      );

      await tester.enterText(_reasonField, '判定有誤');
      await tester.pump();
      await tester.tap(find.bySemanticsLabel('新增照片'));
      await tester.pump();
      expect(find.byType(Image), findsOneWidget);
      expect(_submitButton(tester).onPressed, isNull); // 上傳中不能送出

      await tester.tap(find.bySemanticsLabel('移除照片'));
      await tester.pump();
      expect(find.byType(Image), findsNothing);

      uploads.gates.single.complete();
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 500));
      expect(find.byType(Image), findsNothing);

      await tester.tap(find.text('送出申覆'));
      await tester.pumpAndSettle();
      expect(server.submits.single.evidenceStagedTokens, isEmpty);
      await _drainToast(tester);
    });

    testWidgets('上傳中移除照片：上傳失敗也不提示', (tester) async {
      final uploads = _Uploads()
        ..gates.add(Completer<void>())
        ..failure = StateError('boom');
      await openForm(
        tester,
        uploads: uploads,
        picker: (_) async => (bytes: _png, filename: 'a.jpg'),
      );

      await tester.tap(find.bySemanticsLabel('新增照片'));
      await tester.pump();
      await tester.tap(find.bySemanticsLabel('移除照片'));
      await tester.pump();

      uploads.gates.single.complete();
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 500));
      expect(find.text('上傳失敗，請再試一次'), findsNothing);
      expect(find.byType(Image), findsNothing);
    });

    testWidgets('上傳失敗：不留下這張圖，提示並可繼續', (tester) async {
      final uploads = _Uploads()..failure = StateError('boom');
      await openForm(
        tester,
        uploads: uploads,
        picker: (_) async => (bytes: _png, filename: 'a.jpg'),
      );

      await tester.enterText(_reasonField, '判定有誤');
      await tester.pump();
      await tester.tap(find.bySemanticsLabel('新增照片'));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 500));

      expect(find.text('上傳失敗，請再試一次'), findsOneWidget);
      expect(find.byType(Image), findsNothing);
      expect(_submitButton(tester).onPressed, isNotNull);
      await tester.pump(const Duration(seconds: 3));
    });
  });
  group('表單網址的 mode', () {
    testWidgets('mode 不是 remediation：不開申覆表單，導回措施列表', (tester) async {
      final server = _Server()..measures = [activeSuspension(id: 'm1')];
      final router = await _pump(
        tester,
        server,
        initial: '${SellerRoutes.measureRequestFor('m1')}?mode=remedation',
      );
      expect(_reasonField, findsNothing);
      expect(find.text('請求解除措施'), findsNothing);
      expect(find.text('措施與申覆'), findsWidgets);
      expect(
        router.routeInformationProvider.value.uri.path,
        SellerRoutes.measures,
      );
    });
  });

  group('請求解除表單（mode=remediation）', () {
    Future<_Server> openRemediation(
      WidgetTester tester, {
      Measure? measure,
    }) async {
      final shown = measure ?? riskSuspension(canRequestRemediation: true);
      // 直接開網址：列表沒讀過，用 GetMyMeasure 讀這一筆。
      final server = _Server()..getAnswer = shown;
      await _pump(
        tester,
        server,
        initial: SellerRoutes.measureRemediationFor('r1'),
      );
      return server;
    }

    FilledButton submit(WidgetTester tester) =>
        tester.widget<FilledButton>(find.widgetWithText(FilledButton, '送出請求'));

    testWidgets('送出呼叫 RequestRemediation（不是 SubmitAppeal），帶 trim 後的理由與 etag', (
      tester,
    ) async {
      final server = await openRemediation(tester);
      expect(find.text('請求解除措施'), findsOneWidget);

      await tester.enterText(_reasonField, '  已改善  ');
      await tester.pump();
      await tester.tap(find.text('送出請求'));
      await tester.pumpAndSettle();

      expect(server.submits, isEmpty);
      final request = server.remediations.single;
      expect(request.measureId, 'r1');
      expect(request.reason, '已改善');
      expect(request.etag, 'etag-1');
      await _drainToast(tester);
    });

    testWidgets('理由限制同申覆：空白不能送、超過 2000 字不能送', (tester) async {
      await openRemediation(tester);
      expect(submit(tester).onPressed, isNull);
      await tester.enterText(_reasonField, '😀' * 2001);
      await tester.pump();
      expect(submit(tester).onPressed, isNull);
      await tester.enterText(_reasonField, '😀' * 2000);
      await tester.pump();
      expect(submit(tester).onPressed, isNotNull);
    });

    testWidgets('can_request_remediation 為假（只有 can_appeal 為真）：說明原因，不顯示表單', (
      tester,
    ) async {
      await openRemediation(tester, measure: riskSuspension(canAppeal: true));
      expect(find.text(remediationUnavailableNotice), findsOneWidget);
      expect(_reasonField, findsNothing);
    });

    testWidgets('網址直接開啟時仍在冷卻期：不顯示表單，顯示冷卻結束時間', (tester) async {
      await openRemediation(
        tester,
        measure: riskSuspension(
          nextRemediationTime: DateTime.utc(2026, 11, 1, 6, 5),
          remediationUnavailableReasons: [reasonCooldown],
        ),
      );
      expect(find.text(remediationCooldownSentence), findsOneWidget);
      expect(find.text(remediationUnavailableNotice), findsNothing);
      expect(_reasonField, findsNothing);
    });

    testWidgets('重送已送達的請求（failed_precondition，重讀後待處理）：顯示伺服器的句子', (
      tester,
    ) async {
      final server = await openRemediation(tester);
      server
        ..submitFailure = ConnectException(Code.failedPrecondition, 'x')
        ..getAnswer = riskSuspension(
          remediationRequests: [
            MeasureRequest(
              state: MeasureRequestState.MEASURE_REQUEST_STATE_PENDING,
            ),
          ],
          remediationUnavailableReasons: [reasonRemediationPending],
        );
      await tester.enterText(_reasonField, '已改善');
      await tester.pump();
      await tester.tap(find.text('送出請求'));
      await tester.pumpAndSettle();

      expect(server.remediations, hasLength(1));
      expect(find.text(remediationPendingSentence), findsOneWidget);
      expect(find.text(remediationUnavailableNotice), findsNothing);
      expect(_reasonField, findsNothing);
      await _drainToast(tester);
    });

    testWidgets('衝突後仍可請求：保留輸入、換新 etag', (tester) async {
      final server = await openRemediation(tester);
      server
        ..submitFailure = reasonError(Code.aborted, 'VERSION_CONFLICT')
        ..getAnswer = (riskSuspension(canRequestRemediation: true).deepCopy()
          ..etag = 'etag-2');
      await tester.enterText(_reasonField, '已改善');
      await tester.pump();
      await tester.tap(find.text('送出請求'));
      await tester.pumpAndSettle();

      expect(find.text(remediationConflictNotice), findsOneWidget);
      expect(find.text('已改善'), findsOneWidget);
      server.submitFailure = null;
      await tester.tap(find.text('送出請求'));
      await tester.pumpAndSettle();
      expect(server.remediations.last.etag, 'etag-2');
      await _drainToast(tester);
    });

    testWidgets('送出前警告：有 rejection_cooldown 才有，天數取整日', (tester) async {
      await openRemediation(
        tester,
        measure: riskSuspension(
          canRequestRemediation: true,
          rejectionCooldown: const Duration(days: 14),
        ),
      );
      expect(find.text('若本次經駁回，將於 14 日內不得再提出解除請求'), findsOneWidget);
    });

    testWidgets('沒有 rejection_cooldown：沒有警告', (tester) async {
      await openRemediation(tester);
      expect(
        find.byKey(const ValueKey('rejection-cooldown-warning')),
        findsNothing,
      );
    });

    testWidgets('申覆表單同樣依 rejection_cooldown 顯示警告', (tester) async {
      final server = _Server()
        ..getAnswer = riskSuspension(
          canAppeal: true,
          rejectionCooldown: const Duration(days: 7),
        );
      await _pump(
        tester,
        server,
        initial: SellerRoutes.measureRequestFor('r1'),
      );
      expect(find.text('提出申覆'), findsOneWidget);
      expect(find.text('若本次經駁回，將於 7 日內不得再提出解除請求'), findsOneWidget);
    });
  });
}

/// 1x1 的透明 PNG，給 `Image.memory`。
final _png = Uint8List.fromList(const [
  0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D, //
  0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
  0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4, 0x89, 0x00, 0x00, 0x00,
  0x0D, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9C, 0x63, 0xF8, 0xFF, 0xFF, 0x3F,
  0x00, 0x05, 0xFE, 0x02, 0xFE, 0xA7, 0x35, 0x81, 0x84, 0x00, 0x00, 0x00,
  0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82,
]);
