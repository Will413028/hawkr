import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/procurement/v1alpha1/procurement.pbenum.dart';
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:hawkr_seller/core/l10n/ui_locale.dart';
import 'package:hawkr_seller/features/procurement/data/seller_quote_model.dart';
import 'package:hawkr_seller/features/procurement/data/seller_quote_repository.dart';
import 'package:hawkr_seller/features/procurement/presentation/seller_quote_controller.dart';
import 'package:mocktail/mocktail.dart';

import '../../support/connect_errors.dart';
import '../../support/default_error_text.dart';

class _MockRepo extends Mock implements SellerQuoteRepository {}

const _publicationId = 'p-1';

const _publication = SellerFacingPublication(
  publicationId: _publicationId,
  publicationVersion: '4',
  sellerFacingPayloadHash: 'hash-4',
);

SellerQuotePayload _validQuote({String total = '11025'}) => SellerQuotePayload(
  responseType: SellerResponseType.SELLER_RESPONSE_TYPE_MANUAL_QUOTE,
  specNote: '外觀無壓傷',
  unitBasis: const QuoteUnitBasis(quantity: '1', unit: '公斤'),
  offeredQuantity: '500',
  offeredQuantityUnit: '公斤',
  goodsAmount: '10000',
  shippingAmount: '500',
  taxAmount: '525',
  taxInclusionBasis: TaxInclusionBasis.TAX_INCLUSION_BASIS_TAX_EXCLUDED,
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
      defaultErrorTextProvider.overrideWithValue(zhDefaultErrorText),
    ],
  );
  addTearDown(container.dispose);
  return container;
}

_MockRepo _repoWithDetail({
  SellerResponseDraftResponse? draft,
  SellerResponseVersionResponse? latestResponse,
}) {
  final repo = _MockRepo();
  when(() => repo.getPublicationDetail(_publicationId)).thenAnswer(
    (_) async => SellerPublicationDetail(
      publication: _publication,
      draft: draft,
      latestResponse: latestResponse,
    ),
  );
  return repo;
}

Future<SellerResponseVersionResponse> _submit(_MockRepo repo) =>
    repo.submitResponse(
      publicationId: any(named: 'publicationId'),
      quote: any(named: 'quote'),
      expectedPublicationVersion: any(named: 'expectedPublicationVersion'),
      expectedSellerFacingPayloadHash: any(
        named: 'expectedSellerFacingPayloadHash',
      ),
      expectedSellerResponseVersion: any(
        named: 'expectedSellerResponseVersion',
      ),
      clientRequestId: any(named: 'clientRequestId'),
    );

const _activeResponse = SellerResponseVersionResponse(
  sellerResponseId: 'r-1',
  sellerResponseVersion: '3',
  status: SellerResponseStatus.SELLER_RESPONSE_STATUS_ACTIVE,
);

void main() {
  setUpAll(() {
    registerFallbackValue(const SellerQuotePayload());
  });
  setUp(_CountingKeys.reset);

  SellerQuoteFormState read(ProviderContainer c) =>
      c.read(sellerQuoteFormProvider(_publicationId)).value!;

  group('載入', () {
    test('沒有草稿也沒有正式回價時，表單是空白的', () async {
      final container = _container(_repoWithDetail(), _CountingKeys());
      final state = await container.read(
        sellerQuoteFormProvider(_publicationId).future,
      );
      expect(state.quote, const SellerQuotePayload());
      expect(state.draftVersion, '');
      expect(state.isRevision, isFalse);
    });

    test('有草稿時讀回草稿，CAS 帶 draftVersion', () async {
      final container = _container(
        _repoWithDetail(
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
      expect(state.quote.specNote, '外觀無壓傷');
    });

    test('已有正式回價時，CAS 帶目前版本且按鈕語意是改版', () async {
      final container = _container(
        _repoWithDetail(latestResponse: _activeResponse),
        _CountingKeys(),
      );
      final state = await container.read(
        sellerQuoteFormProvider(_publicationId).future,
      );
      expect(state.expectedSellerResponseVersion, '3');
      expect(state.isRevision, isTrue);
    });
  });

  group('送出', () {
    test('驗證不過不呼叫伺服器', () async {
      final repo = _repoWithDetail();
      final container = _container(repo, _CountingKeys());
      final notifier = container.read(
        sellerQuoteFormProvider(_publicationId).notifier,
      );
      await container.read(sellerQuoteFormProvider(_publicationId).future);
      notifier.updateQuote((_) => _validQuote(total: '1'));

      expect(await notifier.submit(), isNull);
      verifyNever(() => _submit(repo));
      expect(read(container).notice!.message, contains('總金額必須等於'));
    });

    test('失敗後原封不動重送用同一把鍵，改了內容才換新鍵', () async {
      final repo = _repoWithDetail();
      final keys = <String>[];
      when(() => _submit(repo)).thenAnswer((invocation) async {
        keys.add(invocation.namedArguments[#clientRequestId] as String);
        throw ConnectException(Code.unavailable, 'down');
      });
      final container = _container(repo, _CountingKeys());
      final notifier = container.read(
        sellerQuoteFormProvider(_publicationId).notifier,
      );
      await container.read(sellerQuoteFormProvider(_publicationId).future);
      notifier.updateQuote((_) => _validQuote());

      await notifier.submit();
      await notifier.submit();
      notifier.updateQuote(
        (q) => q.copyWith(goodsAmount: '9000', totalAmount: '10025'),
      );
      await notifier.submit();

      expect(keys, ['key-1', 'key-1', 'key-2']);
    });

    test('client_request_id 的欄位錯誤（同鍵不同內容）：換新鍵', () async {
      final repo = _repoWithDetail();
      final keys = <String>[];
      when(() => _submit(repo)).thenAnswer((invocation) async {
        keys.add(invocation.namedArguments[#clientRequestId] as String);
        throw fieldError('client_request_id', '這次送出與先前不同');
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

    test('VERSION_CONFLICT：要求重新載入，不會拿最新版偷偷重送', () async {
      final repo = _repoWithDetail(latestResponse: _activeResponse);
      var submitCalls = 0;
      when(() => _submit(repo)).thenAnswer((_) async {
        submitCalls++;
        throw reasonError(
          Code.aborted,
          'VERSION_CONFLICT',
          message: '你的報價已經變動，請重新載入',
        );
      });
      final container = _container(repo, _CountingKeys());
      final notifier = container.read(
        sellerQuoteFormProvider(_publicationId).notifier,
      );
      await container.read(sellerQuoteFormProvider(_publicationId).future);
      notifier.updateQuote((_) => _validQuote());

      expect(await notifier.submit(), isNull);
      var state = read(container);
      expect(state.reloadRequired, isTrue);
      expect(state.notice!.code, 'VERSION_CONFLICT');
      expect(state.notice!.message, '你的報價已經變動，請重新載入');
      expect(submitCalls, 1);

      expect(await notifier.submit(), isNull);
      expect(submitCalls, 1);

      notifier.updateQuote((q) => q.copyWith(goodsAmount: '9000'));
      expect(read(container).reloadRequired, isTrue);

      await notifier.reload();
      state = read(container);
      expect(state.reloadRequired, isFalse);
      expect(state.notice!.message, contains('已載入最新版本'));
    });

    test('成功後帶回新版本與回應的報價', () async {
      final repo = _repoWithDetail();
      when(() => _submit(repo)).thenAnswer(
        (_) async => SellerResponseVersionResponse(
          sellerResponseId: 'r-1',
          sellerResponseVersion: '1',
          quote: _validQuote(),
          outsideBuyerRequestedRange: true,
        ),
      );
      final container = _container(repo, _CountingKeys());
      final notifier = container.read(
        sellerQuoteFormProvider(_publicationId).notifier,
      );
      await container.read(sellerQuoteFormProvider(_publicationId).future);
      notifier.updateQuote((_) => _validQuote());

      expect(await notifier.submit(), isNotNull);
      final state = read(container);
      expect(state.expectedSellerResponseVersion, '1');
      expect(state.quote.totalAmount, '11025', reason: '金額不得被篡改');
      expect(state.notice!.message, contains('買家原本說明的範圍之外'));
    });
  });

  group('撤回與拒絕', () {
    test('撤回只帶目標版本，成功後重讀詳情', () async {
      final repo = _repoWithDetail(latestResponse: _activeResponse);
      when(
        () => repo.withdrawResponse(
          publicationId: any(named: 'publicationId'),
          sellerResponseId: any(named: 'sellerResponseId'),
          expectedSellerResponseVersion: any(
            named: 'expectedSellerResponseVersion',
          ),
        ),
      ).thenAnswer((_) async => const SellerResponseClosure());
      final container = _container(repo, _CountingKeys());
      final notifier = container.read(
        sellerQuoteFormProvider(_publicationId).notifier,
      );
      await container.read(sellerQuoteFormProvider(_publicationId).future);

      expect(await notifier.withdraw(), isTrue);

      verify(
        () => repo.withdrawResponse(
          publicationId: _publicationId,
          sellerResponseId: 'r-1',
          expectedSellerResponseVersion: '3',
        ),
      ).called(1);
      verify(() => repo.getPublicationDetail(_publicationId)).called(2);
      expect(read(container).notice!.message, '已撤回這一版報價。');
    });

    test('婉拒把原因代碼交給 repository', () async {
      final repo = _repoWithDetail();
      when(
        () => repo.declinePublication(
          publicationId: any(named: 'publicationId'),
          reasonCode: any(named: 'reasonCode'),
        ),
      ).thenAnswer((_) async => const SellerResponseClosure());
      final container = _container(repo, _CountingKeys());
      final notifier = container.read(
        sellerQuoteFormProvider(_publicationId).notifier,
      );
      await container.read(sellerQuoteFormProvider(_publicationId).future);

      expect(await notifier.decline(reasonCode: 'OUT_OF_STOCK'), isTrue);

      verify(
        () => repo.declinePublication(
          publicationId: _publicationId,
          reasonCode: 'OUT_OF_STOCK',
        ),
      ).called(1);
    });
  });
}
