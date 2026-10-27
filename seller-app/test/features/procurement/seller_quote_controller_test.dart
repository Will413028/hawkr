import 'package:hawkr_seller/core/codemeta/code_meta.dart';
import 'package:hawkr_seller/features/procurement/data/procurement_error.dart';
import 'package:hawkr_seller/features/procurement/data/seller_quote_model.dart';
import 'package:hawkr_seller/features/procurement/data/seller_quote_repository.dart';
import 'package:hawkr_seller/features/procurement/presentation/seller_quote_controller.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';

class _MockRepo extends Mock implements SellerQuoteRepository {}

const _publicationId = 'p-1';

const _publication = SellerFacingPublication(
  publicationId: _publicationId,
  publicationVersion: '4',
  inviteId: 'inv-1',
  lane: 'PLATFORM',
  sellerFacingPayloadHash: 'hash-4',
  deliveryState: 'SENT',
  publishedAt: '2026-09-13T02:00:00Z',
);

const _payload = SellerFacingPayload(item: '高麗菜', quantity: '200', unit: 'BOX');

SellerQuotePayload _validQuote({String total = '11025'}) => SellerQuotePayload(
  responseType: ProcurementSellerResponseType.manualQuote.code,
  offeredQuality: const OfferedQuality(grade: '特級', inspectionNotes: '外觀無壓傷'),
  unitBasis: const UnitBasis(quantity: '1', unit: '公斤'),
  offeredQuantity: '500',
  offeredQuantityUnit: '公斤',
  goodsAmount: '10000',
  shippingAmount: '500',
  taxAmount: '525',
  taxInclusionBasis: ProcurementTaxInclusionBasis.taxExcluded.code,
  totalAmount: total,
  validUntil: '2099-12-31T15:00:00Z',
);

/// 每次要一把新鍵就 `key-1`、`key-2`…，讓「同意圖同鍵、換意圖換鍵」可以被斷言。
class _CountingKeys extends QuoteIdempotencyKeyBook {
  _CountingKeys() : super(mint: _next);

  static int _counter = 0;
  static String _next() => 'key-${++_counter}';
  static void reset() => _counter = 0;
}

ProviderContainer _container(_MockRepo repo, QuoteIdempotencyKeyBook keys) {
  final container = ProviderContainer(
    overrides: [
      sellerQuoteRepositoryProvider.overrideWithValue(repo),
      quoteIdempotencyKeyBookProvider.overrideWithValue(keys),
    ],
  );
  addTearDown(container.dispose);
  return container;
}

void main() {
  setUpAll(() {
    registerFallbackValue(const SellerQuotePayload());
  });

  setUp(_CountingKeys.reset);

  _MockRepo repoWithDetail({
    SellerResponseDraftResponse? draft,
    SellerResponseVersionResponse? latestResponse,
  }) {
    final repo = _MockRepo();
    when(() => repo.getPublicationDetail(_publicationId)).thenAnswer(
      (_) async => SellerPublicationDetail(
        publication: _publication,
        sellerFacingPayload: _payload,
        draft: draft,
        latestResponse: latestResponse,
      ),
    );
    return repo;
  }

  group('載入', () {
    test('沒有草稿也沒有正式回價時，表單是空白的（不帶入上次品相）', () async {
      final container = _container(repoWithDetail(), _CountingKeys());
      final state = await container.read(
        sellerQuoteFormProvider(_publicationId).future,
      );
      expect(state.quote, const SellerQuotePayload());
      expect(state.quote.offeredQuality.grade, isEmpty);
      expect(state.draftVersion, '');
      expect(state.isRevision, isFalse);
      expect(state.sellerFacingPayload, _payload);
    });

    test('有草稿時讀回草稿，CAS 帶 draftVersion', () async {
      final container = _container(
        repoWithDetail(
          draft: SellerResponseDraftResponse(
            draftVersion: '2',
            draft: _validQuote(),
          ),
        ),
        _CountingKeys(),
      );
      final state = await container.read(
        sellerQuoteFormProvider(_publicationId).future,
      );
      expect(state.draftVersion, '2');
      expect(state.quote.offeredQuality.grade, '特級');
    });

    test('已有正式回價時，CAS 帶目前版本且按鈕語意是改版', () async {
      final container = _container(
        repoWithDetail(
          latestResponse: const SellerResponseVersionResponse(
            sellerResponseId: 'r-1',
            sellerResponseVersion: '3',
            status: 'ACTIVE',
          ),
        ),
        _CountingKeys(),
      );
      final state = await container.read(
        sellerQuoteFormProvider(_publicationId).future,
      );
      expect(state.expectedSellerResponseVersion, '3');
      expect(state.isRevision, isTrue);
    });
  });

  group('草稿', () {
    test('存草稿不驗金額恆等式，也不需要冪等鍵', () async {
      final repo = repoWithDetail();
      when(
        () => repo.saveDraft(
          publicationId: any(named: 'publicationId'),
          draft: any(named: 'draft'),
          expectedDraftVersion: any(named: 'expectedDraftVersion'),
        ),
      ).thenAnswer(
        (_) async => const SellerResponseDraftResponse(draftVersion: '1'),
      );

      final container = _container(repo, _CountingKeys());
      final notifier = container.read(
        sellerQuoteFormProvider(_publicationId).notifier,
      );
      await container.read(sellerQuoteFormProvider(_publicationId).future);

      // 完全空白、金額對不起來的表單也存得起來。
      notifier.updateQuote(
        (q) => q.copyWith(goodsAmount: '1', totalAmount: '9'),
      );
      expect(await notifier.saveDraft(), isTrue);

      final captured = verify(
        () => repo.saveDraft(
          publicationId: _publicationId,
          draft: captureAny(named: 'draft'),
          expectedDraftVersion: captureAny(named: 'expectedDraftVersion'),
        ),
      ).captured;
      expect((captured[0] as SellerQuotePayload).totalAmount, '9');
      expect(captured[1], '');
      expect(
        container
            .read(sellerQuoteFormProvider(_publicationId))
            .value!
            .draftVersion,
        '1',
      );
    });
  });

  group('正式提交', () {
    test('品相沒填就擋在送出之前，不打 API', () async {
      final repo = repoWithDetail();
      final container = _container(repo, _CountingKeys());
      final notifier = container.read(
        sellerQuoteFormProvider(_publicationId).notifier,
      );
      await container.read(sellerQuoteFormProvider(_publicationId).future);

      notifier.updateQuote(
        (_) => _validQuote().copyWith(offeredQuality: const OfferedQuality()),
      );
      expect(await notifier.submit(), isNull);

      verifyNever(
        () => repo.submitResponse(
          publicationId: any(named: 'publicationId'),
          quote: any(named: 'quote'),
          expectedPublicationVersion: any(named: 'expectedPublicationVersion'),
          expectedSellerFacingPayloadHash: any(
            named: 'expectedSellerFacingPayloadHash',
          ),
          expectedSellerResponseVersion: any(
            named: 'expectedSellerResponseVersion',
          ),
          idempotencyKey: any(named: 'idempotencyKey'),
        ),
      );
      final notice = container
          .read(sellerQuoteFormProvider(_publicationId))
          .value!
          .notice!;
      expect(notice.message, contains('品相'));
    });

    test('金額恆等式不成立也擋下，不浪費一把冪等鍵', () async {
      final repo = repoWithDetail();
      final container = _container(repo, _CountingKeys());
      final notifier = container.read(
        sellerQuoteFormProvider(_publicationId).notifier,
      );
      await container.read(sellerQuoteFormProvider(_publicationId).future);

      notifier.updateQuote((_) => _validQuote(total: '11000'));
      expect(await notifier.submit(), isNull);
      final notice = container
          .read(sellerQuoteFormProvider(_publicationId))
          .value!
          .notice!;
      expect(notice.message, contains('11025'));
    });

    test('送出帶三個 CAS 與冪等鍵；成功後回填版本', () async {
      final repo = repoWithDetail();
      when(
        () => repo.submitResponse(
          publicationId: any(named: 'publicationId'),
          quote: any(named: 'quote'),
          expectedPublicationVersion: any(named: 'expectedPublicationVersion'),
          expectedSellerFacingPayloadHash: any(
            named: 'expectedSellerFacingPayloadHash',
          ),
          expectedSellerResponseVersion: any(
            named: 'expectedSellerResponseVersion',
          ),
          idempotencyKey: any(named: 'idempotencyKey'),
        ),
      ).thenAnswer(
        (invocation) async => SellerResponseVersionResponse(
          sellerResponseId: 'r-1',
          sellerResponseVersion: '1',
          status: 'ACTIVE',
          quote: invocation.namedArguments[#quote] as SellerQuotePayload,
        ),
      );

      final container = _container(repo, _CountingKeys());
      final notifier = container.read(
        sellerQuoteFormProvider(_publicationId).notifier,
      );
      await container.read(sellerQuoteFormProvider(_publicationId).future);
      notifier.updateQuote((_) => _validQuote());

      final response = await notifier.submit();
      expect(response, isNotNull);

      final captured = verify(
        () => repo.submitResponse(
          publicationId: _publicationId,
          quote: any(named: 'quote'),
          expectedPublicationVersion: captureAny(
            named: 'expectedPublicationVersion',
          ),
          expectedSellerFacingPayloadHash: captureAny(
            named: 'expectedSellerFacingPayloadHash',
          ),
          expectedSellerResponseVersion: captureAny(
            named: 'expectedSellerResponseVersion',
          ),
          idempotencyKey: captureAny(named: 'idempotencyKey'),
        ),
      ).captured;
      expect(captured[0], '4');
      expect(captured[1], 'hash-4');
      expect(captured[2], '', reason: '第一次送出的 CAS 是空字串');
      expect(captured[3], 'key-1');

      final state = container
          .read(sellerQuoteFormProvider(_publicationId))
          .value!;
      expect(state.isRevision, isTrue);
      expect(state.expectedSellerResponseVersion, '1');
    });

    test('outsideBuyerRequestedRange 只標示，不阻擋也不改金額', () async {
      final repo = repoWithDetail();
      when(
        () => repo.submitResponse(
          publicationId: any(named: 'publicationId'),
          quote: any(named: 'quote'),
          expectedPublicationVersion: any(named: 'expectedPublicationVersion'),
          expectedSellerFacingPayloadHash: any(
            named: 'expectedSellerFacingPayloadHash',
          ),
          expectedSellerResponseVersion: any(
            named: 'expectedSellerResponseVersion',
          ),
          idempotencyKey: any(named: 'idempotencyKey'),
        ),
      ).thenAnswer(
        (invocation) async => SellerResponseVersionResponse(
          sellerResponseId: 'r-1',
          sellerResponseVersion: '1',
          status: 'ACTIVE',
          outsideBuyerRequestedRange: true,
          quote: invocation.namedArguments[#quote] as SellerQuotePayload,
        ),
      );

      final container = _container(repo, _CountingKeys());
      final notifier = container.read(
        sellerQuoteFormProvider(_publicationId).notifier,
      );
      await container.read(sellerQuoteFormProvider(_publicationId).future);
      notifier.updateQuote((_) => _validQuote());

      final response = await notifier.submit();
      expect(response, isNotNull);
      final state = container
          .read(sellerQuoteFormProvider(_publicationId))
          .value!;
      expect(state.quote.totalAmount, '11025', reason: '金額不得被篡改');
      expect(state.notice!.tone, SellerQuoteNoticeTone.success);
      expect(state.notice!.message, contains('範圍之外'));
    });
  });

  group('冪等鍵是 stable client request id', () {
    Future<
      ({
        ProviderContainer container,
        SellerQuoteForm notifier,
        List<String> keys,
      })
    >
    harness() async {
      final repo = repoWithDetail();
      final keys = <String>[];
      when(
        () => repo.submitResponse(
          publicationId: any(named: 'publicationId'),
          quote: any(named: 'quote'),
          expectedPublicationVersion: any(named: 'expectedPublicationVersion'),
          expectedSellerFacingPayloadHash: any(
            named: 'expectedSellerFacingPayloadHash',
          ),
          expectedSellerResponseVersion: any(
            named: 'expectedSellerResponseVersion',
          ),
          idempotencyKey: any(named: 'idempotencyKey'),
        ),
      ).thenAnswer((invocation) async {
        keys.add(invocation.namedArguments[#idempotencyKey] as String);
        throw ProcurementException(
          message: '伺服器忙碌',
          code: ProcurementErrorReason.capabilityUnavailable.code,
          presentation: procurementErrorPresentation(
            ProcurementErrorReason.capabilityUnavailable.code,
          ),
        );
      });

      final container = _container(repo, _CountingKeys());
      final notifier = container.read(
        sellerQuoteFormProvider(_publicationId).notifier,
      );
      await container.read(sellerQuoteFormProvider(_publicationId).future);
      return (container: container, notifier: notifier, keys: keys);
    }

    test('同一次意圖重送用同一把鍵', () async {
      final h = await harness();
      h.notifier.updateQuote((_) => _validQuote());
      await h.notifier.submit();
      await h.notifier.submit();
      expect(h.keys, ['key-1', 'key-1']);
    });

    test('改了內容再送就是新意圖，換一把鍵', () async {
      final h = await harness();
      h.notifier.updateQuote((_) => _validQuote());
      await h.notifier.submit();
      h.notifier.updateQuote(
        (q) => q.copyWith(goodsAmount: '10001', totalAmount: '11026'),
      );
      await h.notifier.submit();
      expect(h.keys, ['key-1', 'key-2']);
    });

    test('IDEMPOTENCY_KEY_CONFLICT 之後換新鍵，不再拿同一把撞牆', () async {
      final repo = repoWithDetail();
      final keys = <String>[];
      when(
        () => repo.submitResponse(
          publicationId: any(named: 'publicationId'),
          quote: any(named: 'quote'),
          expectedPublicationVersion: any(named: 'expectedPublicationVersion'),
          expectedSellerFacingPayloadHash: any(
            named: 'expectedSellerFacingPayloadHash',
          ),
          expectedSellerResponseVersion: any(
            named: 'expectedSellerResponseVersion',
          ),
          idempotencyKey: any(named: 'idempotencyKey'),
        ),
      ).thenAnswer((invocation) async {
        keys.add(invocation.namedArguments[#idempotencyKey] as String);
        throw ProcurementException(
          message: '衝突',
          code: ProcurementErrorReason.idempotencyKeyConflict.code,
          presentation: procurementErrorPresentation(
            ProcurementErrorReason.idempotencyKeyConflict.code,
          ),
        );
      });

      final container = _container(repo, _CountingKeys());
      final notifier = container.read(
        sellerQuoteFormProvider(_publicationId).notifier,
      );
      await container.read(sellerQuoteFormProvider(_publicationId).future);
      notifier.updateQuote((_) => _validQuote());
      await notifier.submit();
      await notifier.submit();
      expect(keys, ['key-1', 'key-2']);
    });
  });

  group('STALE_SELLER_RESPONSE：必須明確告知，不得靜默送最新版', () {
    test('橫幅要求重新載入，再按送出不會再打一次 API', () async {
      final repo = repoWithDetail(
        latestResponse: const SellerResponseVersionResponse(
          sellerResponseId: 'r-1',
          sellerResponseVersion: '3',
          status: 'ACTIVE',
        ),
      );
      var submitCalls = 0;
      when(
        () => repo.submitResponse(
          publicationId: any(named: 'publicationId'),
          quote: any(named: 'quote'),
          expectedPublicationVersion: any(named: 'expectedPublicationVersion'),
          expectedSellerFacingPayloadHash: any(
            named: 'expectedSellerFacingPayloadHash',
          ),
          expectedSellerResponseVersion: any(
            named: 'expectedSellerResponseVersion',
          ),
          idempotencyKey: any(named: 'idempotencyKey'),
        ),
      ).thenAnswer((_) async {
        submitCalls++;
        throw ProcurementException(
          message: procurementErrorPresentation(
            'STALE_SELLER_RESPONSE',
          )!.message,
          code: 'STALE_SELLER_RESPONSE',
          presentation: procurementErrorPresentation('STALE_SELLER_RESPONSE'),
        );
      });

      final container = _container(repo, _CountingKeys());
      final notifier = container.read(
        sellerQuoteFormProvider(_publicationId).notifier,
      );
      await container.read(sellerQuoteFormProvider(_publicationId).future);
      notifier.updateQuote((_) => _validQuote());

      expect(await notifier.submit(), isNull);
      var state = container
          .read(sellerQuoteFormProvider(_publicationId))
          .value!;
      expect(state.reloadRequired, isTrue);
      expect(state.notice!.code, 'STALE_SELLER_RESPONSE');
      expect(state.notice!.message, contains('報價已經變動'));
      expect(submitCalls, 1);

      // 再按一次送出：擋住，不會拿最新版偷偷重送。
      expect(await notifier.submit(), isNull);
      expect(submitCalls, 1);

      // 改欄位也清不掉這條橫幅——只有明確重新載入才行。
      notifier.updateQuote((q) => q.copyWith(goodsAmount: '9000'));
      state = container.read(sellerQuoteFormProvider(_publicationId)).value!;
      expect(state.reloadRequired, isTrue);

      await notifier.reload();
      state = container.read(sellerQuoteFormProvider(_publicationId)).value!;
      expect(state.reloadRequired, isFalse);
      expect(state.notice!.message, contains('已載入最新版本'));
    });
  });

  group('撤回與拒絕', () {
    test('撤回帶目前版本與冪等鍵', () async {
      final repo = repoWithDetail(
        latestResponse: const SellerResponseVersionResponse(
          sellerResponseId: 'r-1',
          sellerResponseVersion: '3',
          status: 'ACTIVE',
        ),
      );
      when(
        () => repo.withdrawResponse(
          publicationId: any(named: 'publicationId'),
          sellerResponseId: any(named: 'sellerResponseId'),
          expectedSellerResponseVersion: any(
            named: 'expectedSellerResponseVersion',
          ),
          idempotencyKey: any(named: 'idempotencyKey'),
        ),
      ).thenAnswer(
        (_) async => const SellerResponseVersionResponse(
          sellerResponseId: 'r-1',
          sellerResponseVersion: '3',
          status: 'WITHDRAWN',
        ),
      );

      final container = _container(repo, _CountingKeys());
      final notifier = container.read(
        sellerQuoteFormProvider(_publicationId).notifier,
      );
      await container.read(sellerQuoteFormProvider(_publicationId).future);
      expect(await notifier.withdraw(), isTrue);

      final captured = verify(
        () => repo.withdrawResponse(
          publicationId: _publicationId,
          sellerResponseId: captureAny(named: 'sellerResponseId'),
          expectedSellerResponseVersion: captureAny(
            named: 'expectedSellerResponseVersion',
          ),
          idempotencyKey: captureAny(named: 'idempotencyKey'),
        ),
      ).captured;
      expect(captured, ['r-1', '3', 'key-1']);
      expect(
        container
            .read(sellerQuoteFormProvider(_publicationId))
            .value!
            .latestResponse!
            .statusValue,
        ProcurementSellerResponseStatus.withdrawn,
      );
    });

    test('沒有正式回價時撤回是 no-op', () async {
      final repo = repoWithDetail();
      final container = _container(repo, _CountingKeys());
      final notifier = container.read(
        sellerQuoteFormProvider(_publicationId).notifier,
      );
      await container.read(sellerQuoteFormProvider(_publicationId).future);
      expect(await notifier.withdraw(), isFalse);
      verifyNever(
        () => repo.withdrawResponse(
          publicationId: any(named: 'publicationId'),
          sellerResponseId: any(named: 'sellerResponseId'),
          expectedSellerResponseVersion: any(
            named: 'expectedSellerResponseVersion',
          ),
          idempotencyKey: any(named: 'idempotencyKey'),
        ),
      );
    });

    test('拒絕帶冪等鍵；同一次拒絕重送用同一把鍵', () async {
      final repo = repoWithDetail();
      final keys = <String>[];
      when(
        () => repo.declinePublication(
          publicationId: any(named: 'publicationId'),
          idempotencyKey: any(named: 'idempotencyKey'),
          reason: any(named: 'reason'),
        ),
      ).thenAnswer((invocation) async {
        keys.add(invocation.namedArguments[#idempotencyKey] as String);
      });

      final container = _container(repo, _CountingKeys());
      final notifier = container.read(
        sellerQuoteFormProvider(_publicationId).notifier,
      );
      await container.read(sellerQuoteFormProvider(_publicationId).future);
      expect(await notifier.decline(reason: '調不到貨'), isTrue);
      expect(await notifier.decline(reason: '調不到貨'), isTrue);
      expect(keys, ['key-1', 'key-1']);
    });
  });
}
