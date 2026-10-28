import 'package:hawkr_seller/core/codemeta/code_meta.dart';
import 'package:hawkr_seller/features/procurement/data/procurement_error.dart';
import 'package:hawkr_seller/features/procurement/data/seller_quote_model.dart';
import 'package:hawkr_seller/features/procurement/data/seller_quote_repository.dart';
import 'package:hawkr_seller/features/procurement/presentation/seller_quote_controller.dart';
import 'package:hawkr_seller/features/procurement/presentation/seller_quote_form_screen.dart';
import 'package:hawkr_seller/features/procurement/presentation/seller_quote_labels.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
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

/// 需求內容的九欄。型別本身就是 deny-by-default：買家預算、可接受區間、
/// 候選排序連進到 model 的路都沒有（那一層在 procurement_error_test 驗）。
const _payload = SellerFacingPayload(
  item: '高麗菜',
  itemLabel: '梨山高麗菜',
  quantity: '200',
  unit: 'BOX',
  packaging: '紙箱',
  deliveryCity: '臺北市',
  deliveryDate: '2026-09-20',
  responseDeadline: '2026-09-18',
);

const _amountLabels = <String>['貨款', '運費', '稅額', '總金額'];

Future<void> _pumpScreen(
  WidgetTester tester,
  _MockRepo repo, {
  QuoteIdempotencyKeyBook? keys,
}) async {
  // 表單很長，把 viewport 拉高讓 ListView 一次把欄位都建出來。
  tester.view.physicalSize = const Size(420, 5200);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);

  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        sellerQuoteRepositoryProvider.overrideWithValue(repo),
        if (keys != null)
          quoteIdempotencyKeyBookProvider.overrideWithValue(keys),
      ],
      child: ScreenUtilInit(
        designSize: const Size(390, 844),
        builder: (_, _) => const MaterialApp(
          home: SellerQuoteFormScreen(publicationId: _publicationId),
        ),
      ),
    ),
  );
  await tester.pump();
  await tester.pump();
}

_MockRepo _repoWith({
  SellerResponseVersionResponse? latestResponse,
  SellerResponseDraftResponse? draft,
  SellerFacingPayload payload = _payload,
}) {
  final repo = _MockRepo();
  when(() => repo.getPublicationDetail(_publicationId)).thenAnswer(
    (_) async => SellerPublicationDetail(
      publication: _publication,
      sellerFacingPayload: payload,
      draft: draft,
      latestResponse: latestResponse,
    ),
  );
  return repo;
}

SellerQuotePayload _validQuote() => SellerQuotePayload(
  responseType: ProcurementSellerResponseType.manualQuote.code,
  offeredQuality: const OfferedQuality(grade: '特級', inspectionNotes: '無壓傷'),
  unitBasis: const UnitBasis(quantity: '1', unit: '公斤'),
  offeredQuantity: '500',
  offeredQuantityUnit: '公斤',
  goodsAmount: '10000',
  shippingAmount: '500',
  taxAmount: '525',
  taxInclusionBasis: ProcurementTaxInclusionBasis.taxExcluded.code,
  totalAmount: '11025',
  validUntil: '2099-12-31T15:00:00Z',
);

void main() {
  setUpAll(() => registerFallbackValue(const SellerQuotePayload()));

  testWidgets('需求內容的結構化欄位都顯示得出來', (tester) async {
    await _pumpScreen(tester, _repoWith());

    expect(find.text('買家需求'), findsOneWidget);
    for (final value in const [
      '高麗菜',
      '梨山高麗菜',
      '200',
      'BOX',
      '紙箱',
      '臺北市',
      '2026-09-20',
      '2026-09-18',
    ]) {
      expect(find.text(value), findsOneWidget, reason: value);
    }
    // 這張 fixture 沒有補充說明 ⇒ 整個區塊不得出現。空字串畫成一個標著標題的
    // 空框，賣家會以為載入失敗。
    expect(find.text('買家的補充說明'), findsNothing);
  });

  testWidgets('買家的補充說明原文顯示，且不混進上面那組結構化欄位', (tester) async {
    await _pumpScreen(
      tester,
      _repoWith(payload: _payload.copyWith(buyerNote: '中大果，不要有壓傷')),
    );

    expect(find.text('買家的補充說明'), findsOneWidget);
    expect(find.text('中大果，不要有壓傷'), findsOneWidget);
  });

  testWidgets('產品規則 3：畫面上沒有預算／區間／排序這類字眼', (tester) async {
    await _pumpScreen(tester, _repoWith());

    for (final word in const ['預算', '區間', '排序', '其他賣家', '候選']) {
      expect(find.textContaining(word), findsNothing, reason: word);
    }
  });

  testWidgets('沒填需求說明時說清楚，不是載入失敗', (tester) async {
    await _pumpScreen(tester, _repoWith(payload: const SellerFacingPayload()));

    expect(find.text('這則詢價沒有填寫需求說明。'), findsOneWidget);
    expect(find.text('送出正式回價'), findsOneWidget);
  });

  testWidgets('需求說明不顯示買家品相說明', (tester) async {
    await _pumpScreen(
      tester,
      _repoWith(payload: const SellerFacingPayload(item: '高麗菜')),
    );

    expect(find.textContaining('買家品相說明'), findsNothing);
  });

  testWidgets('產品規則 2：選了「接受非價格條款」也不會少問任何金額欄位', (tester) async {
    await _pumpScreen(tester, _repoWith());

    for (final label in _amountLabels) {
      expect(find.text(label), findsOneWidget, reason: label);
    }

    await tester.tap(
      find.text(
        ProcurementSellerResponseType.acceptNonPriceTermsWithQuote.formLabel,
      ),
    );
    await tester.pump();

    for (final label in _amountLabels) {
      expect(
        find.text(label),
        findsOneWidget,
        reason: '$label 在第二個 responseType 底下也必須still在',
      );
    }
    expect(find.text('報價有效期限'), findsOneWidget);
    expect(find.text('計價基準數量'), findsOneWidget);
    expect(find.text('實際品相與驗收（必填）'), findsOneWidget);
  });

  testWidgets('產品規則 1：品相沒填就擋在送出之前，畫面說清楚只能自己填', (tester) async {
    final repo = _repoWith();
    await _pumpScreen(tester, repo);

    expect(find.textContaining('平台不會替你帶入上一次報價的品相'), findsOneWidget);

    await tester.tap(find.text('送出正式回價'));
    await tester.pump();

    expect(find.textContaining('只能由你自己填'), findsWidgets);
    expect(find.text('請填寫這批貨的實際品相等級'), findsOneWidget);
    expect(find.text('請填寫驗收說明'), findsOneWidget);
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
  });

  testWidgets('金額恆等式即時提示，且不覆寫使用者填的總金額', (tester) async {
    await _pumpScreen(
      tester,
      _repoWith(
        draft: SellerResponseDraftResponse(
          draftVersion: '1',
          draft: _validQuote().copyWith(totalAmount: '11000'),
        ),
      ),
    );

    expect(find.textContaining('與你填的總金額不一致'), findsOneWidget);

    // 使用者輸入的總金額原封不動留在欄位裡。
    final totalField = tester.widget<TextField>(
      find.ancestor(of: find.text('總金額'), matching: find.byType(TextField)),
    );
    expect(totalField.controller!.text, '11000');

    // 改成一致的值後提示轉為一致，欄位仍是使用者輸入的字面值。
    await tester.enterText(
      find.ancestor(of: find.text('總金額'), matching: find.byType(TextField)),
      '11025',
    );
    await tester.pump();
    expect(find.textContaining('與總金額一致'), findsOneWidget);
  });

  testWidgets('STALE_SELLER_RESPONSE 明確告知並只給「重新載入」一條路', (tester) async {
    final repo = _repoWith(
      latestResponse: const SellerResponseVersionResponse(
        sellerResponseId: 'r-1',
        sellerResponseVersion: '3',
        status: 'ACTIVE',
      ),
      draft: SellerResponseDraftResponse(
        draftVersion: '1',
        draft: _validQuote(),
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
        message: procurementErrorPresentation('STALE_SELLER_RESPONSE')!.message,
        code: 'STALE_SELLER_RESPONSE',
        presentation: procurementErrorPresentation('STALE_SELLER_RESPONSE'),
      );
    });

    await _pumpScreen(tester, repo);
    expect(find.text('送出改版'), findsOneWidget);

    await tester.tap(find.text('送出改版'));
    await tester.pump();
    await tester.pump();

    expect(submitCalls, 1);
    expect(find.textContaining('你的報價已經變動'), findsOneWidget);
    expect(find.text('重新載入最新版本'), findsOneWidget);
    // 送出鈕在重新載入之前不得再送。
    final submitButton = tester.widget<FilledButton>(
      find.ancestor(of: find.text('送出改版'), matching: find.byType(FilledButton)),
    );
    expect(submitButton.onPressed, isNull);

    await tester.tap(find.text('重新載入最新版本'));
    await tester.pump();
    await tester.pump();
    expect(find.textContaining('已載入最新版本'), findsOneWidget);
    expect(submitCalls, 1);
  });

  testWidgets('載入失敗時給重試，不會假裝有一張空表單', (tester) async {
    final repo = _MockRepo();
    when(
      () => repo.getPublicationDetail(_publicationId),
    ).thenThrow(const ProcurementException(message: '找不到這則詢價'));

    await _pumpScreen(tester, repo);
    expect(find.text('重新載入'), findsOneWidget);
    expect(find.text('送出正式回價'), findsNothing);
  });
}
