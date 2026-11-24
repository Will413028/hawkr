import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/procurement/v1alpha1/procurement.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/procurement/v1alpha1/procurement.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/procurement/v1alpha1/procurement.pb.dart'
    as pb;
import 'package:hawkr_api/gen/hawkr/procurement/v1alpha1/procurement.pbenum.dart';
import 'package:hawkr_api/taipei_time.dart' show Timestamp;
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_seller/features/procurement/data/seller_quote_model.dart';
import 'package:hawkr_seller/features/procurement/data/seller_quote_repository.dart';

/// 記下每個 RPC 收到的請求，回固定的回應。
class _Server {
  final submits = <pb.SubmitSellerResponseRequest>[];
  final drafts = <pb.UpsertSellerResponseDraftRequest>[];
  final withdraws = <pb.WithdrawSellerResponseRequest>[];
  final declines = <pb.DeclineRfqPublicationRequest>[];
  final listTokens = <String>[];

  SellerQuoteRepository get repository => SellerQuoteRepository(
    SellerProcurementAppServiceClient(
      FakeTransportBuilder()
          .unary(SellerProcurementAppService.listSellerPublications, (r, _) {
            listTokens.add(r.pageToken);
            final first = r.pageToken.isEmpty;
            return pb.ListSellerPublicationsResponse(
              publications: [
                pb.SellerPublicationListItem(
                  publication: pb.SellerPublicationSummary(
                    publicationId: first ? 'p-1' : 'p-2',
                    publicationVersion: '3',
                    sellerFacingPayloadHash: 'h',
                  ),
                  sellerFacingPayload: pb.SellerFacingRfq(
                    item: '高麗菜',
                    quantity: 200,
                  ),
                  latestResponseStatus: first
                      ? SellerResponseStatus.SELLER_RESPONSE_STATUS_ACTIVE
                      : null,
                  hasDraft: !first,
                ),
              ],
              nextPageToken: first ? 'next' : '',
            );
          })
          .unary(SellerProcurementAppService.getSellerPublication, (r, _) {
            return pb.GetSellerPublicationResponse(
              publication: pb.SellerPublicationSummary(
                publicationId: r.publicationId,
                publicationVersion: '4',
                sellerFacingPayloadHash: 'hash-4',
              ),
              sellerFacingPayload: pb.SellerFacingRfq(
                item: '高麗菜',
                buyerNote: '請小心搬運',
              ),
              draft: pb.SellerResponseDraft(
                draftVersion: 'd1',
                draft: pb.SellerQuote(specNote: '草稿說明'),
              ),
              latestResponse: pb.SellerResponseVersion(
                sellerResponseId: 'sr-1',
                sellerResponseVersion: '2',
                status: SellerResponseStatus.SELLER_RESPONSE_STATUS_ACTIVE,
                quote: _wireQuote(),
              ),
            );
          })
          .unary(SellerProcurementAppService.upsertSellerResponseDraft, (r, _) {
            drafts.add(r);
            return pb.UpsertSellerResponseDraftResponse(
              draft: pb.SellerResponseDraft(draftVersion: 'd2', draft: r.draft),
            );
          })
          .unary(SellerProcurementAppService.submitSellerResponse, (r, _) {
            submits.add(r);
            return pb.SubmitSellerResponseResponse(
              sellerResponse: pb.SellerResponseVersion(
                sellerResponseId: 'sr-1',
                sellerResponseVersion: '3',
                quote: r.quote,
              ),
            );
          })
          .unary(SellerProcurementAppService.withdrawSellerResponse, (r, _) {
            withdraws.add(r);
            return pb.WithdrawSellerResponseResponse(
              closure: pb.SellerResponseClosure(
                sellerResponseId: r.sellerResponseId,
                withdrawnVersion: r.expectedSellerResponseVersion,
              ),
            );
          })
          .unary(SellerProcurementAppService.declineRfqPublication, (r, _) {
            declines.add(r);
            return pb.DeclineRfqPublicationResponse(
              closure: pb.SellerResponseClosure(),
            );
          })
          .build(),
    ),
  );
}

pb.SellerQuote _wireQuote() =>
    pb.SellerQuote(
        responseType: SellerResponseType.SELLER_RESPONSE_TYPE_MANUAL_QUOTE,
        specNote: '外觀無壓傷',
        unitBasis: pb.UnitBasis(quantity: '1', unit: '公斤'),
        offeredQuantity: '500',
        offeredQuantityUnit: '公斤',
        taxInclusionBasis: TaxInclusionBasis.TAX_INCLUSION_BASIS_TAX_EXCLUDED,
        validUntilTime: Timestamp.fromDateTime(DateTime.utc(2099, 12, 31, 15)),
      )
      ..goodsAmount = pb.SellerQuote().goodsAmount + 10000
      ..shippingAmount = pb.SellerQuote().goodsAmount + 500
      ..taxAmount = pb.SellerQuote().goodsAmount + 525
      ..totalAmount = pb.SellerQuote().goodsAmount + 11025;

SellerQuotePayload _formQuote({String leadTimeDays = ''}) => SellerQuotePayload(
  responseType: SellerResponseType.SELLER_RESPONSE_TYPE_MANUAL_QUOTE,
  specNote: '外觀無壓傷',
  unitBasis: const QuoteUnitBasis(quantity: '1', unit: '公斤'),
  offeredQuantity: '500',
  offeredQuantityUnit: '公斤',
  goodsAmount: '10000',
  shippingAmount: '500',
  taxAmount: '525',
  taxInclusionBasis: TaxInclusionBasis.TAX_INCLUSION_BASIS_TAX_EXCLUDED,
  totalAmount: '11025',
  leadTimeDays: leadTimeDays,
  validUntil: '2099-12-31T15:00:00Z',
);

void main() {
  test('列表逐頁讀完，狀態與草稿旗標對應', () async {
    final server = _Server();
    final items = await server.repository.listPublications();

    expect(server.listTokens, ['', 'next']);
    expect(items.map((i) => i.publication.publicationId), ['p-1', 'p-2']);
    expect(
      items.first.latestResponseStatus,
      SellerResponseStatus.SELLER_RESPONSE_STATUS_ACTIVE,
    );
    expect(items.first.sellerFacingPayload.quantity, '200');
    expect(items.last.latestResponseStatus, isNull);
    expect(items.last.hasDraft, isTrue);
  });

  test('詳情帶出草稿、正式回價與賣家看到的內容', () async {
    final detail = await _Server().repository.getPublicationDetail('p-9');

    expect(detail.publication.publicationVersion, '4');
    expect(detail.sellerFacingPayload.buyerNote, '請小心搬運');
    expect(detail.expectedDraftVersion, 'd1');
    expect(detail.draft!.draft.specNote, '草稿說明');
    expect(detail.expectedSellerResponseVersion, '2');
    final quote = detail.latestResponse!.quote;
    expect(quote.goodsAmount, '10000');
    expect(quote.totalAmount, '11025');
    expect(quote.validUntil, '2099-12-31T15:00:00Z');
    expect(quote.leadTimeDays, '', reason: '沒填交期就不承諾，不顯示 0');
  });

  test('送出：金額轉 int64、spec_note、request id 與三個 expected 欄位', () async {
    final server = _Server();
    await server.repository.submitResponse(
      publicationId: 'p-1',
      quote: _formQuote(),
      expectedPublicationVersion: '4',
      expectedSellerFacingPayloadHash: 'hash-4',
      expectedSellerResponseVersion: '',
      clientRequestId: 'req-1',
    );

    final sent = server.submits.single;
    expect(sent.clientRequestId, 'req-1');
    expect(sent.expectedPublicationVersion, '4');
    expect(sent.expectedSellerFacingPayloadHash, 'hash-4');
    expect(sent.expectedSellerResponseVersion, '');
    expect(sent.quote.specNote, '外觀無壓傷');
    expect(sent.quote.totalAmount.toString(), '11025');
    expect(sent.quote.hasLeadTimeDays(), isFalse);
    expect(sent.quote.hasValidUntilTime(), isTrue);
  });

  test('備貨天數有填才送，0 代表當天出貨', () async {
    final server = _Server();
    await server.repository.saveDraft(
      publicationId: 'p-1',
      draft: _formQuote(leadTimeDays: '0'),
      expectedDraftVersion: '',
    );

    expect(server.drafts.single.draft.hasLeadTimeDays(), isTrue);
    expect(server.drafts.single.draft.leadTimeDays, 0);
  });

  test('撤回不帶 request id，只帶目標版本', () async {
    final server = _Server();
    final closure = await server.repository.withdrawResponse(
      publicationId: 'p-1',
      sellerResponseId: 'sr-1',
      expectedSellerResponseVersion: '2',
    );

    final sent = server.withdraws.single;
    expect(sent.expectedSellerResponseVersion, '2');
    expect(sent.reasonCode, '');
    expect(closure.withdrawnVersion, '2');
  });

  test('婉拒把原因代碼送到 reason_code', () async {
    final server = _Server();
    await server.repository.declinePublication(
      publicationId: 'p-1',
      reasonCode: 'OUT_OF_STOCK',
    );

    expect(server.declines.single.reasonCode, 'OUT_OF_STOCK');
  });
}
