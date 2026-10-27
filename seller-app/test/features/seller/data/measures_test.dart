import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_api/transport.dart';
import 'package:hawkr_seller/features/seller/data/version_conflict.dart';
import 'package:hawkr_seller/features/seller/data/measures.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../../support/connect_errors.dart';
import '../../../support/measure_fixtures.dart';
import '../../../support/default_error_text.dart';

/// 可控的 SellerAppService：列表分頁、單筆讀取、申覆。
class _Server {
  final pages = <ListMyMeasuresResponse>[];
  final listRequests = <ListMyMeasuresRequest>[];
  final submits = <SubmitAppealRequest>[];
  final remediations = <RequestRemediationRequest>[];
  final gets = <GetMyMeasureRequest>[];
  Object? submitFailure;
  Object? getAnswer;
  Measure submitted = measureOf(
    canAppeal: false,
    appeal: MeasureRequest(
      state: MeasureRequestState.MEASURE_REQUEST_STATE_PENDING,
    ),
  );

  SellerAppServiceClient get client => SellerAppServiceClient(
    FakeTransportBuilder()
        .unary(SellerAppService.listMyMeasures, (request, _) {
          listRequests.add(request);
          return pages[listRequests.length - 1];
        })
        .unary(SellerAppService.submitAppeal, (request, _) {
          submits.add(request);
          final error = submitFailure;
          if (error != null) throw error;
          return SubmitAppealResponse(measure: submitted);
        })
        .unary(SellerAppService.requestRemediation, (request, _) {
          remediations.add(request);
          final error = submitFailure;
          if (error != null) throw error;
          return RequestRemediationResponse(measure: submitted);
        })
        .unary(SellerAppService.getMyMeasure, (request, _) {
          gets.add(request);
          final answer = getAnswer;
          if (answer is Object && answer is! Measure) throw answer;
          return GetMyMeasureResponse(measure: answer as Measure);
        })
        .build(),
  );
}

Future<MeasureRequestResult> _submit(_Server server, {Measure? measure}) =>
    submitMeasureRequest(
      server.client,
      kind: MeasureRequestAction.appeal,
      measure: measure ?? measureOf(),
      reason: '  判定有誤  ',
      evidenceTokens: const ['t1', 't2'],
      defaults: zhDefaultErrorText,
    );

void main() {
  group('fetchMyMeasures：讀完所有分頁', () {
    test('一頁 100 筆，用 next_page_token 讀到沒有為止，順序照伺服器', () async {
      final server = _Server()
        ..pages.addAll([
          ListMyMeasuresResponse(
            measures: [
              measureOf(id: 'a'),
              measureOf(id: 'b'),
            ],
            nextPageToken: 'p2',
          ),
          ListMyMeasuresResponse(
            measures: [measureOf(id: 'c')],
            nextPageToken: 'p3',
          ),
          ListMyMeasuresResponse(measures: [measureOf(id: 'd')]),
        ]);

      final all = await fetchMyMeasures(server.client);

      expect([for (final v in all) v.id], ['a', 'b', 'c', 'd']);
      expect(
        [for (final r in server.listRequests) r.pageToken],
        ['', 'p2', 'p3'],
      );
      expect({for (final r in server.listRequests) r.pageSize}, {100});
    });

    test('state 篩選：每一頁都帶同一個 state；不給就是不篩', () async {
      final server = _Server()
        ..pages.addAll([
          ListMyMeasuresResponse(
            measures: [measureOf(id: 'a')],
            nextPageToken: 'p2',
          ),
          ListMyMeasuresResponse(measures: [measureOf(id: 'b')]),
        ]);

      await fetchMyMeasures(
        server.client,
        state: MeasureState.MEASURE_STATE_ACTIVE,
      );

      expect(
        {for (final r in server.listRequests) r.state},
        {MeasureState.MEASURE_STATE_ACTIVE},
      );
      expect(server.listRequests, hasLength(2));

      final unfiltered = _Server()..pages.add(ListMyMeasuresResponse());
      await fetchMyMeasures(unfiltered.client);
      expect(
        unfiltered.listRequests.single.state,
        MeasureState.MEASURE_STATE_UNSPECIFIED,
      );
    });

    test('沒有措施：一次請求，空列表', () async {
      final server = _Server()..pages.add(ListMyMeasuresResponse());
      expect(await fetchMyMeasures(server.client), isEmpty);
      expect(server.listRequests, hasLength(1));
    });

    test('next_page_token 沒有前進：丟錯，不無限迴圈', () async {
      final server = _Server()
        ..pages.addAll([
          ListMyMeasuresResponse(
            measures: [measureOf(id: 'a')],
            nextPageToken: 'same',
          ),
          ListMyMeasuresResponse(
            measures: [measureOf(id: 'b')],
            nextPageToken: 'same',
          ),
        ]);

      await expectLater(
        fetchMyMeasures(server.client),
        throwsA(isA<StateError>()),
      );
      expect(server.listRequests, hasLength(2));
    });
  });

  group('fetchMyMeasures：page token 循環', () {
    test('A→B→A：任何曾經出現過的 token 再出現都丟錯', () async {
      final server = _Server()
        ..pages.addAll([
          ListMyMeasuresResponse(
            measures: [measureOf(id: 'a')],
            nextPageToken: 'A',
          ),
          ListMyMeasuresResponse(
            measures: [measureOf(id: 'b')],
            nextPageToken: 'B',
          ),
          ListMyMeasuresResponse(
            measures: [measureOf(id: 'c')],
            nextPageToken: 'A',
          ),
          ListMyMeasuresResponse(measures: [measureOf(id: 'd')]),
        ]);

      await expectLater(
        fetchMyMeasures(server.client),
        throwsA(isA<StateError>()),
      );
      expect(server.listRequests, hasLength(3));
    });
  });

  group('submitMeasureRequest：送出的理由', () {
    test('送出的理由先以 Go 的 TrimSpace 修剪', () async {
      final server = _Server();
      await submitMeasureRequest(
        server.client,
        kind: MeasureRequestAction.appeal,
        measure: measureOf(),
        reason: '\uFEFF 判定有誤\u3000',
        evidenceTokens: const [],
        defaults: zhDefaultErrorText,
      );
      expect(server.submits.single.reason, '\uFEFF 判定有誤');
    });
  });

  group('submitMeasureRequest：請求解除', () {
    Future<MeasureRequestResult> remediate(
      _Server server, {
      Measure? measure,
    }) => submitMeasureRequest(
      server.client,
      kind: MeasureRequestAction.remediation,
      measure: measure ?? riskSuspension(canRequestRemediation: true),
      reason: '  已改善  ',
      evidenceTokens: const ['t1'],
      defaults: zhDefaultErrorText,
    );

    test(
      '成功：呼叫 RequestRemediation（不是 SubmitAppeal），帶 trim 後的理由、token 與 etag',
      () async {
        final server = _Server();

        final result = await remediate(
          server,
          measure: riskSuspension(canRequestRemediation: true).deepCopy()
            ..etag = 'e-7',
        );

        expect(result, isA<MeasureRequestSubmitted>());
        expect(server.submits, isEmpty);
        final request = server.remediations.single;
        expect(request.measureId, 'r1');
        expect(request.reason, '已改善');
        expect(request.evidenceStagedTokens, ['t1']);
        expect(request.etag, 'e-7');
      },
    );

    test(
      'VERSION_CONFLICT：重讀後 can_request_remediation 仍為真 → 保留輸入、換新 etag',
      () async {
        final latest = riskSuspension(canRequestRemediation: true).deepCopy()
          ..etag = 'etag-2';
        final server = _Server()
          ..submitFailure = reasonError(Code.aborted, 'VERSION_CONFLICT')
          ..getAnswer = latest;

        final result = await remediate(server);

        expect(result, isA<MeasureRequestRefreshed>());
        expect((result as MeasureRequestRefreshed).latest.etag, 'etag-2');
        expect(result.notice, remediationConflictNotice);
      },
    );

    test('VERSION_CONFLICT：重讀後不能再請求（只有 can_appeal 為真也一樣）→ 關閉', () async {
      final server = _Server()
        ..submitFailure = reasonError(Code.aborted, 'VERSION_CONFLICT')
        ..getAnswer = riskSuspension(canAppeal: true);

      final result = await remediate(server);

      expect(result, isA<MeasureRequestClosed>());
      expect(
        (result as MeasureRequestClosed).notice,
        remediationUnavailableNotice,
      );
    });
  });

  group('submitMeasureRequest：請求解除失敗後的原因取自重讀的措施', () {
    Future<MeasureRequestResult> remediate(_Server server) =>
        submitMeasureRequest(
          server.client,
          kind: MeasureRequestAction.remediation,
          measure: riskSuspension(canRequestRemediation: true),
          reason: '已改善',
          evidenceTokens: const [],
          defaults: zhDefaultErrorText,
        );

    Future<String> closedNotice(Measure latest) async {
      final server = _Server()
        ..submitFailure = reasonError(Code.failedPrecondition, 'NOT_ALLOWED')
        ..getAnswer = latest;
      final result = await remediate(server);
      expect(server.remediations, hasLength(1));
      return (result as MeasureRequestClosed).notice;
    }

    test('重送已送達的請求（重讀後自己的請求待處理）→ 伺服器的句子，不是通用說明', () async {
      expect(
        await closedNotice(
          riskSuspension(
            remediationRequests: [
              MeasureRequest(
                state: MeasureRequestState.MEASURE_REQUEST_STATE_PENDING,
              ),
            ],
            remediationUnavailableReasons: [reasonRemediationPending],
          ),
        ),
        remediationPendingSentence,
      );
    });

    test('申覆審核中 → 伺服器的句子', () async {
      expect(
        await closedNotice(
          riskSuspension(
            appeal: MeasureRequest(
              state: MeasureRequestState.MEASURE_REQUEST_STATE_PENDING,
            ),
            nextRemediationTime: DateTime.utc(2026, 11, 1, 6, 5),
            remediationUnavailableReasons: [
              reasonAppealPending,
              reasonCooldown,
            ],
          ),
        ),
        remediationAppealPendingSentence,
      );
    });

    test('網址直接開啟時仍在冷卻期 → 伺服器組好的冷卻結束時間句', () async {
      expect(
        await closedNotice(
          riskSuspension(
            nextRemediationTime: DateTime.utc(2026, 11, 1, 6, 5),
            remediationUnavailableReasons: [reasonCooldown],
          ),
        ),
        remediationCooldownSentence,
      );
    });

    test('沒有原因或原因沒有句子才用通用說明；NOT_ACTIVE 有句子就顯示句子', () async {
      expect(
        await closedNotice(
          riskSuspension(
            state: MeasureState.MEASURE_STATE_ENDED,
            remediationUnavailableReasons: [reasonNotActive],
          ),
        ),
        remediationNotActiveSentence,
      );
      expect(
        await closedNotice(
          withoutUnavailableMessages(
            riskSuspension(
              state: MeasureState.MEASURE_STATE_ENDED,
              remediationUnavailableReasons: [reasonNotActive],
            ),
          ),
        ),
        remediationUnavailableNotice,
      );
      expect(
        await closedNotice(riskSuspension()),
        remediationUnavailableNotice,
      );
    });
  });

  group('submitAppeal', () {
    test('成功：帶 trim 後的理由、staged token 與讀到的 etag', () async {
      final server = _Server();

      final result = await _submit(server, measure: measureOf(etag: 'e-9'));

      expect(result, isA<MeasureRequestSubmitted>());
      final request = server.submits.single;
      expect(request.measureId, 'm1');
      expect(request.reason, '判定有誤');
      expect(request.evidenceStagedTokens, ['t1', 't2']);
      expect(request.etag, 'e-9');
      expect(server.gets, isEmpty);
    });

    test('VERSION_CONFLICT：重讀，已有申覆 → 關閉並提示伺服器的句子', () async {
      final server = _Server()
        ..submitFailure = reasonError(Code.aborted, 'VERSION_CONFLICT')
        ..getAnswer = measureOf(
          canAppeal: false,
          appeal: MeasureRequest(
            state: MeasureRequestState.MEASURE_REQUEST_STATE_PENDING,
          ),
        );

      final result = await _submit(server);

      expect(result, isA<MeasureRequestClosed>());
      expect(
        (result as MeasureRequestClosed).notice,
        appealAlreadyAppealedSentence,
      );
      expect(server.gets.single.measureId, 'm1');
    });

    test('VERSION_CONFLICT：重讀，已撤銷 → 關閉並提示伺服器的句子', () async {
      final server = _Server()
        ..submitFailure = reasonError(Code.aborted, 'VERSION_CONFLICT')
        ..getAnswer = measureOf(
          canAppeal: false,
          state: MeasureState.MEASURE_STATE_REVOKED,
        );

      final result = await _submit(server);

      expect((result as MeasureRequestClosed).notice, appealRevokedSentence);
    });

    test('已申覆又已撤銷（接受申覆）：先說已經提出申覆', () async {
      final server = _Server()
        ..submitFailure = reasonError(Code.aborted, 'VERSION_CONFLICT')
        ..getAnswer = measureOf(
          canAppeal: false,
          state: MeasureState.MEASURE_STATE_REVOKED,
          appeal: MeasureRequest(
            state: MeasureRequestState.MEASURE_REQUEST_STATE_ACCEPTED,
          ),
        );

      expect(
        ((await _submit(server)) as MeasureRequestClosed).notice,
        appealAlreadyAppealedSentence,
      );
    });

    test('VERSION_CONFLICT：重讀後仍可申覆 → 保留輸入、換新 etag、CONFLICT_NOTICE', () async {
      final latest = measureOf(etag: 'etag-2', statement: '更新後的說明');
      final server = _Server()
        ..submitFailure = reasonError(Code.aborted, 'VERSION_CONFLICT')
        ..getAnswer = latest;

      final result = await _submit(server);

      expect(result, isA<MeasureRequestRefreshed>());
      final refreshed = result as MeasureRequestRefreshed;
      expect(refreshed.latest.etag, 'etag-2');
      expect(refreshed.notice, '這項措施已更新，已載入最新內容，請確認後再送出申覆');
    });

    test('failed_precondition 與 VERSION_CONFLICT 走同一條重讀路徑', () async {
      final server = _Server()
        ..submitFailure = ConnectException(
          Code.failedPrecondition,
          '這項措施已申覆過或已撤銷',
        )
        ..getAnswer = measureOf(
          canAppeal: false,
          state: MeasureState.MEASURE_STATE_REVOKED,
        );

      final result = await _submit(server);

      expect((result as MeasureRequestClosed).notice, appealRevokedSentence);
      expect(server.gets, hasLength(1));
    });

    test(
      '重送同一個申覆（伺服器回 failed_precondition）：重讀後 can_appeal 為 false，關閉並說已經申覆',
      () async {
        final server = _Server()
          ..submitFailure = ConnectException(Code.failedPrecondition, '已申覆')
          ..getAnswer = measureOf(
            canAppeal: false,
            appeal: MeasureRequest(
              state: MeasureRequestState.MEASURE_REQUEST_STATE_PENDING,
            ),
          );

        final result = await _submit(server);

        expect(result, isA<MeasureRequestClosed>());
        expect(
          (result as MeasureRequestClosed).notice,
          appealAlreadyAppealedSentence,
        );
        expect(result.latest.requests, isNotEmpty);
        expect(server.submits, hasLength(1));
        expect(server.gets.single.measureId, 'm1');
      },
    );

    test('failed_precondition 但重讀仍可申覆：也是保留輸入', () async {
      final server = _Server()
        ..submitFailure = ConnectException(Code.failedPrecondition, 'x')
        ..getAnswer = measureOf(etag: 'etag-3');

      expect(await _submit(server), isA<MeasureRequestRefreshed>());
    });

    test('不能申覆、伺服器給的原因這個版本不認得又沒有句子：關閉並用通用提示', () async {
      final server = _Server()
        ..submitFailure = reasonError(Code.aborted, 'VERSION_CONFLICT')
        ..getAnswer = withUnrecognizedAppealReason(measureOf());

      expect(
        ((await _submit(server)) as MeasureRequestClosed).notice,
        '這項措施目前不能提出申覆',
      );
    });

    test('不能申覆、原因沒有句子：關閉並用通用提示（client 不依 reason 組字）', () async {
      final server = _Server()
        ..submitFailure = reasonError(Code.aborted, 'VERSION_CONFLICT')
        ..getAnswer = withoutUnavailableMessages(
          measureOf(
            canAppeal: false,
            state: MeasureState.MEASURE_STATE_REVOKED,
          ),
        );

      expect(
        ((await _submit(server)) as MeasureRequestClosed).notice,
        appealUnavailableNotice,
      );
    });

    test('衝突後重讀失敗：顯示讀不到最新版本，不當成已解決', () async {
      final server = _Server()
        ..submitFailure = reasonError(Code.aborted, 'VERSION_CONFLICT')
        ..getAnswer = ConnectException(Code.notFound, 'gone');

      final result = await _submit(server);

      expect(result, isA<MeasureRequestFailed>());
      expect(
        (result as MeasureRequestFailed).message,
        versionReloadFailedNotice,
      );
    });

    test('理由欄位的錯誤：顯示在欄位上', () async {
      final server = _Server()
        ..submitFailure = fieldError('reason', '申覆理由不能超過 2000 字');

      final result = await _submit(server);

      expect(result, isA<MeasureRequestReasonInvalid>());
      expect(
        (result as MeasureRequestReasonInvalid).message,
        '申覆理由不能超過 2000 字',
      );
      expect(server.gets, isEmpty);
    });

    test('佐證欄位沒有索引的錯誤（張數上限）：整個欄位的訊息，不指向哪一張', () async {
      final server = _Server()
        ..submitFailure = fieldError('evidence_staged_tokens', '佐證最多 5 張');

      final result = await _submit(server);

      expect(result, isA<MeasureRequestEvidenceInvalid>());
      final invalid = result as MeasureRequestEvidenceInvalid;
      expect(invalid.byIndex, isEmpty);
      expect(invalid.evidenceMessage, '佐證最多 5 張');
      expect(invalid.reasonMessage, isNull);
    });

    test('其他欄位的錯誤：顯示伺服器的訊息', () async {
      final server = _Server()..submitFailure = fieldError('etag', '必須提供 etag');

      final result = await _submit(server);

      expect(result, isA<MeasureRequestFailed>());
      expect((result as MeasureRequestFailed).message, '必須提供 etag');
    });

    test('佐證 token 的欄位錯誤：依位置回報是第幾張，同時帶出理由欄位的錯誤', () async {
      final server = _Server()
        ..submitFailure = fieldErrors({
          'evidence_staged_tokens[1]': '佐證圖片已過期',
          'evidence_staged_tokens[3]': '佐證圖片未通過檢查',
          'reason': '申覆理由不能超過 2000 字',
        });

      final result = await _submit(server);

      expect(result, isA<MeasureRequestEvidenceInvalid>());
      final invalid = result as MeasureRequestEvidenceInvalid;
      expect(invalid.byIndex, {1: '佐證圖片已過期', 3: '佐證圖片未通過檢查'});
      expect(invalid.reasonMessage, '申覆理由不能超過 2000 字');
      expect(server.gets, isEmpty);
    });

    test('其他失敗：顯示伺服器的文字', () async {
      final server = _Server()
        ..submitFailure = ConnectException(Code.permissionDenied, 'no');

      final result = await _submit(server);

      expect(result, isA<MeasureRequestFailed>());
      expect((result as MeasureRequestFailed).message, isNotEmpty);
      expect(server.gets, isEmpty);
    });
  });
}
