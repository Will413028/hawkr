import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_seller/core/networking/connect_provider.dart';
import 'package:hawkr_seller/core/session/session_provider.dart';
import 'package:hawkr_seller/features/seller/data/measure_display.dart';
import 'package:hawkr_seller/features/seller/data/measures.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../../support/measure_fixtures.dart';

class _LoggedIn extends Session {
  @override
  SessionModel build() =>
      const SessionModel(accountId: 'acc-1', isLoggedIn: true);
}

class _Switchable extends Session {
  @override
  SessionModel build() =>
      const SessionModel(accountId: 'acc-1', isLoggedIn: true);

  void loginAs(String accountId) =>
      state = SessionModel(accountId: accountId, isLoggedIn: true);
}

class _Server {
  List<Measure> measures = [];
  final filters = <MeasureState>[];
  Object? failure;
  int listCalls = 0;
  final pageSizes = <int>[];
  final gets = <String>[];

  SellerAppServiceClient get client => SellerAppServiceClient(
    FakeTransportBuilder()
        .unary(SellerAppService.listMyMeasures, (request, _) {
          listCalls++;
          pageSizes.add(request.pageSize);
          filters.add(request.state);
          final matching =
              request.state == MeasureState.MEASURE_STATE_UNSPECIFIED
              ? measures
              : [
                  for (final m in measures)
                    if (m.state == request.state) m,
                ];
          final error = failure;
          if (error != null) throw error;
          return ListMyMeasuresResponse(measures: matching);
        })
        .unary(SellerAppService.getMyMeasure, (request, _) {
          gets.add(request.measureId);
          return GetMyMeasureResponse(
            measure: measureOf(id: request.measureId, etag: 'remote'),
          );
        })
        .build(),
  );
}

ProviderContainer _container(_Server server, {bool switchable = false}) {
  final container = ProviderContainer(
    overrides: [
      sessionProvider.overrideWith(
        switchable ? _Switchable.new : _LoggedIn.new,
      ),
      sellerClientProvider.overrideWithValue(server.client),
    ],
  );
  addTearDown(container.dispose);
  return container;
}

void main() {
  group('MyMeasures.adopt', () {
    test('以 id 取代那一列、其餘順序不變；分組由內容重新算（不再可申覆就離開需要處理）', () async {
      final server = _Server()
        ..measures = [
          measureOf(id: 'a'),
          measureOf(id: 'b'),
          measureOf(
            id: 'c',
            canAppeal: false,
            state: MeasureState.MEASURE_STATE_ENDED,
          ),
        ];
      final container = _container(server);
      final sub = container.listen(
        myMeasuresProvider('acc-1', allMeasureStates),
        (_, _) {},
      );
      addTearDown(sub.close);
      await container.read(
        myMeasuresProvider('acc-1', allMeasureStates).future,
      );

      container
          .read(myMeasuresProvider('acc-1', allMeasureStates).notifier)
          .adopt(
            measureOf(
              id: 'a',
              state: MeasureState.MEASURE_STATE_ENDED,
              canAppeal: false,
              etag: 'etag-2',
              appeal: MeasureRequest(
                state: MeasureRequestState.MEASURE_REQUEST_STATE_PENDING,
              ),
            ),
          );

      final list = container
          .read(myMeasuresProvider('acc-1', allMeasureStates))
          .requireValue;
      expect(list.map((m) => m.id), ['a', 'b', 'c']);
      expect(list.first.etag, 'etag-2');
      final groups = groupMeasures(list);
      expect(groups.attention.map((m) => m.id), ['b']);
      expect(groups.history.map((m) => m.id), ['a', 'c']);
      expect(server.listCalls, 1);
    });

    test('列表裡沒有這一筆時不動；還沒讀過列表時不動', () async {
      final server = _Server()..measures = [measureOf(id: 'a')];
      final container = _container(server);
      final sub = container.listen(
        myMeasuresProvider('acc-1', allMeasureStates),
        (_, _) {},
      );
      addTearDown(sub.close);
      container
          .read(myMeasuresProvider('acc-1', allMeasureStates).notifier)
          .adopt(measureOf(id: 'z'));
      expect(
        container.read(myMeasuresProvider('acc-1', allMeasureStates)).hasValue,
        isFalse,
      );

      await container.read(
        myMeasuresProvider('acc-1', allMeasureStates).future,
      );
      container
          .read(myMeasuresProvider('acc-1', allMeasureStates).notifier)
          .adopt(measureOf(id: 'z'));
      expect(
        container
            .read(myMeasuresProvider('acc-1', allMeasureStates))
            .requireValue
            .map((m) => m.id),
        ['a'],
      );
    });
  });

  group('MyMeasures 以帳號為鍵', () {
    test('換帳號且新帳號的讀取失敗：看不到上一個帳號的列表，讀取中與失敗都沒有值', () async {
      final server = _Server()..measures = [measureOf(id: 'a')];
      final container = _container(server, switchable: true);
      final sub = container.listen(
        currentMyMeasuresProvider(allMeasureStates),
        (_, _) {},
      );
      addTearDown(sub.close);
      await container.read(
        myMeasuresProvider('acc-1', allMeasureStates).future,
      );
      expect(
        container
            .read(currentMyMeasuresProvider(allMeasureStates))!
            .value!
            .map((m) => m.id),
        ['a'],
      );

      server.failure = StateError('boom');
      (container.read(sessionProvider.notifier) as _Switchable).loginAs(
        'acc-2',
      );
      final loading = container.read(
        currentMyMeasuresProvider(allMeasureStates),
      )!;
      expect(loading.isLoading, isTrue);
      expect(loading.value, isNull);

      await expectLater(
        container.read(myMeasuresProvider('acc-2', allMeasureStates).future),
        throwsA(isA<StateError>()),
      );
      final failed = container.read(
        currentMyMeasuresProvider(allMeasureStates),
      )!;
      expect(failed.hasError, isTrue);
      expect(failed.value, isNull);
    });
  });

  group('MyMeasures 的 state 篩選', () {
    final activeKey = MeasureState.MEASURE_STATE_ACTIVE;

    test('篩選的列表把 state 帶給伺服器，只拿到那個狀態；不同篩選各自快取', () async {
      final server = _Server()
        ..measures = [
          measureOf(id: 'a'),
          measureOf(id: 'b', state: MeasureState.MEASURE_STATE_ENDED),
        ];
      final container = _container(server);

      final active = await container.read(
        myMeasuresProvider('acc-1', activeKey).future,
      );
      final all = await container.read(
        myMeasuresProvider('acc-1', allMeasureStates).future,
      );

      expect(active.map((m) => m.id), ['a']);
      expect(all.map((m) => m.id), ['a', 'b']);
      expect(server.filters, [activeKey, allMeasureStates]);
    });

    test('adopt：最新一筆已經不是這份列表的狀態時，從列表拿掉；全部列表裡只是取代', () async {
      final server = _Server()
        ..measures = [measureOf(id: 'a'), measureOf(id: 'b')];
      final container = _container(server);
      final activeProvider = myMeasuresProvider('acc-1', activeKey);
      final allProvider = myMeasuresProvider('acc-1', allMeasureStates);
      final subs = [
        container.listen(activeProvider, (_, _) {}),
        container.listen(allProvider, (_, _) {}),
      ];
      addTearDown(() {
        for (final sub in subs) {
          sub.close();
        }
      });
      await container.read(activeProvider.future);
      await container.read(allProvider.future);

      final ended = measureOf(
        id: 'a',
        state: MeasureState.MEASURE_STATE_ENDED,
        canAppeal: false,
      );
      container.read(activeProvider.notifier).adopt(ended);
      container.read(allProvider.notifier).adopt(ended);

      expect(container.read(activeProvider).requireValue.map((m) => m.id), [
        'b',
      ]);
      final all = container.read(allProvider).requireValue;
      expect(all.map((m) => m.id), ['a', 'b']);
      expect(all.first.state, MeasureState.MEASURE_STATE_ENDED);
    });

    test('adopt：仍是這個狀態（申覆後 can_appeal 變 false）就留在生效中列表', () async {
      final server = _Server()..measures = [measureOf(id: 'a')];
      final container = _container(server);
      final activeProvider = myMeasuresProvider('acc-1', activeKey);
      final sub = container.listen(activeProvider, (_, _) {});
      addTearDown(sub.close);
      await container.read(activeProvider.future);

      container
          .read(activeProvider.notifier)
          .adopt(measureOf(id: 'a', canAppeal: false, etag: 'etag-2'));

      final list = container.read(activeProvider).requireValue;
      expect(list.single.etag, 'etag-2');
      expect(list.single.canAppeal, isFalse);
    });
  });

  group('accountHasMeasures', () {
    test('只問一筆，有就是 true，不讀完整個列表', () async {
      final server = _Server()
        ..measures = [measureOf(id: 'a'), measureOf(id: 'b')];
      final container = _container(server);

      expect(
        await container.read(accountHasMeasuresProvider('acc-1').future),
        isTrue,
      );
      expect(server.pageSizes, [1]);
    });

    test('沒有措施是 false；讀取失敗是 error，不當成沒有', () async {
      final server = _Server();
      final container = _container(server);
      expect(
        await container.read(accountHasMeasuresProvider('acc-1').future),
        isFalse,
      );

      server.failure = StateError('boom');
      container.invalidate(accountHasMeasuresProvider('acc-1'));
      await expectLater(
        container.read(accountHasMeasuresProvider('acc-1').future),
        throwsA(isA<StateError>()),
      );
    });
  });

  group('measureById', () {
    test('列表已讀過而且有這一筆：用快取，不打 GetMyMeasure', () async {
      final server = _Server()..measures = [measureOf(id: 'a', etag: 'cached')];
      final container = _container(server);
      final sub = container.listen(
        myMeasuresProvider('acc-1', allMeasureStates),
        (_, _) {},
      );
      addTearDown(sub.close);
      await container.read(
        myMeasuresProvider('acc-1', allMeasureStates).future,
      );

      final found = await container.read(
        measureByIdProvider('acc-1', 'a').future,
      );
      expect(found.etag, 'cached');
      expect(server.gets, isEmpty);
    });

    test('列表沒讀過：直接 GetMyMeasure，不去觸發整個列表', () async {
      final server = _Server();
      final container = _container(server);

      final found = await container.read(
        measureByIdProvider('acc-1', 'a').future,
      );
      expect(found.etag, 'remote');
      expect(server.gets, ['a']);
      expect(server.listCalls, 0);
    });
  });

  group('appealClosedNotice 只讀伺服器的 appeal_unavailabilities 第一筆的句子', () {
    test('各個原因的句子逐字取自伺服器，不依 reason 組字', () {
      expect(
        appealClosedNotice(
          measureOf(
            canAppeal: false,
            appealUnavailableReasons: [
              AppealUnavailableReason
                  .APPEAL_UNAVAILABLE_REASON_ALREADY_APPEALED,
            ],
          ),
        ),
        appealAlreadyAppealedSentence,
      );
      expect(
        appealClosedNotice(
          measureOf(
            canAppeal: false,
            appealUnavailableReasons: [
              AppealUnavailableReason.APPEAL_UNAVAILABLE_REASON_REVOKED,
            ],
          ),
        ),
        appealRevokedSentence,
      );
      expect(
        appealClosedNotice(
          terminationOf(
            canAppeal: false,
            appealUnavailableReasons: [
              AppealUnavailableReason
                  .APPEAL_UNAVAILABLE_REASON_APPEAL_WINDOW_ENDED,
              AppealUnavailableReason.APPEAL_UNAVAILABLE_REASON_REQUEST_PENDING,
            ],
          ),
        ),
        appealWindowEndedSentence,
      );
    });

    test('不從 requests 或 state 推導：原因說審核中，就算有申覆紀錄、狀態已撤銷也照原因', () {
      final m = measureOf(
        state: MeasureState.MEASURE_STATE_REVOKED,
        canAppeal: false,
        appeal: MeasureRequest(
          state: MeasureRequestState.MEASURE_REQUEST_STATE_PENDING,
        ),
        appealUnavailableReasons: [
          AppealUnavailableReason.APPEAL_UNAVAILABLE_REASON_REQUEST_PENDING,
        ],
      );
      expect(appealClosedNotice(m), appealRequestPendingSentence);
    });

    test('沒有可用的原因（空、不認得或沒有句子）時用通用說明', () {
      expect(appealClosedNotice(measureOf()), '這項措施目前不能提出申覆');
      expect(
        appealClosedNotice(withUnrecognizedAppealReason(measureOf())),
        '這項措施目前不能提出申覆',
      );
      expect(
        appealClosedNotice(
          withoutUnavailableMessages(measureOf(canAppeal: false)),
        ),
        '這項措施目前不能提出申覆',
      );
    });
  });
}
