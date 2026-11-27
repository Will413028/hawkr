import 'package:hawkr_api/gen/hawkr/procurement/v1alpha1/procurement.pbenum.dart';
import 'package:hawkr_seller/features/procurement/data/seller_quote_model.dart';
import 'package:hawkr_seller/features/procurement/presentation/seller_quote_validation.dart';
import 'package:flutter_test/flutter_test.dart';

/// 一份「除了要測的那一項之外都合法」的 payload。
SellerQuotePayload _completeQuote({
  SellerResponseType type =
      SellerResponseType.SELLER_RESPONSE_TYPE_MANUAL_QUOTE,
  String specNote = '外觀無壓傷，到貨 24 小時內驗收',
  String goods = '10000',
  String shipping = '500',
  String tax = '525',
  String total = '11025',
  String validUntil = '2026-12-31T15:00:00Z',
}) {
  return SellerQuotePayload(
    responseType: type,
    specNote: specNote,
    unitBasis: const QuoteUnitBasis(quantity: '1', unit: '公斤'),
    offeredQuantity: '500',
    offeredQuantityUnit: '公斤',
    conversionBasis: '1 箱 ＝ 10 公斤',
    goodsAmount: goods,
    shippingAmount: shipping,
    taxAmount: tax,
    taxInclusionBasis: TaxInclusionBasis.TAX_INCLUSION_BASIS_TAX_EXCLUDED,
    totalAmount: total,
    leadTimeDays: '3',
    promisedDeliveryAt: '2026-12-20T02:00:00Z',
    validUntil: validUntil,
    termsNote: '到貨後付款',
  );
}

final DateTime _now = DateTime.utc(2026, 9, 13, 8);

void main() {
  group('decimal string 不經過 double', () {
    test('0.1 + 0.2 加起來等於 0.3，不是 0.30000000000000004', () {
      expect(QuoteDecimal.sumOf(['0.1', '0.2'])!.toString(), '0.3');
    });

    test('尾隨零視為同一個值：10 == 10.00', () {
      final a = QuoteDecimal.tryParse('10')!;
      final b = QuoteDecimal.tryParse('10.00')!;
      expect(a.sameValueAs(b), isTrue);
      expect(b.toString(), '10.00');
    });

    test('負號、科學記號、千分位都不是 wire 上的 decimal string', () {
      expect(QuoteDecimal.tryParse('-1'), isNull);
      expect(QuoteDecimal.tryParse('1e3'), isNull);
      expect(QuoteDecimal.tryParse('1,000'), isNull);
      expect(QuoteDecimal.tryParse(''), isNull);
    });

    test('小數位不同也能相加，取較長的位數', () {
      expect(QuoteDecimal.sumOf(['1.5', '2', '0.25'])!.toString(), '3.75');
    });
  });

  group('RFC3339', () {
    test('必須帶時區', () {
      expect(isRfc3339('2026-12-31T15:00:00Z'), isTrue);
      expect(isRfc3339('2026-12-31T23:00:00+08:00'), isTrue);
      expect(isRfc3339('2026-12-31T15:00:00'), isFalse);
      expect(isRfc3339('2026-12-31'), isFalse);
    });

    test('本地時間轉 UTC 後是 RFC3339 且不帶毫秒', () {
      final value = toRfc3339Utc(DateTime.utc(2026, 12, 31, 15, 4, 5));
      expect(value, '2026-12-31T15:04:05Z');
      expect(isRfc3339(value), isTrue);
    });
  });

  group('規格說明（spec_note）選填、不由平台代填', () {
    test('不填規格說明也能送出', () {
      final result = validateSellerQuote(
        _completeQuote(specNote: ''),
        now: _now,
      );
      expect(result.isSubmittable, isTrue);
    });

    test('空白表單的規格說明是空的——不會被上一次報價帶入', () {
      expect(const SellerQuotePayload().specNote, isEmpty);
    });
  });

  group('產品規則 2：ACCEPT_NON_PRICE_TERMS_WITH_QUOTE 不是少填金額的捷徑', () {
    test('兩個 responseType 的必填清單完全相同', () {
      for (final type in kSelectableResponseTypes) {
        final missingAmounts = validateSellerQuote(
          _completeQuote(
            type: type,
            goods: '',
            shipping: '',
            tax: '',
            total: '',
          ),
          now: _now,
        );
        expect(
          missingAmounts.isSubmittable,
          isFalse,
          reason: '${type.name} 少填金額也必須擋下',
        );
        for (final field in const [
          SellerQuoteField.goodsAmount,
          SellerQuoteField.shippingAmount,
          SellerQuoteField.taxAmount,
          SellerQuoteField.totalAmount,
        ]) {
          expect(missingAmounts.errorFor(field), isNotNull);
        }
      }
    });

    test('必填清單不看 responseType', () {
      expect(
        kRequiredQuoteFields,
        containsAll(const [
          SellerQuoteField.unitBasisQuantity,
          SellerQuoteField.unitBasisUnit,
          SellerQuoteField.goodsAmount,
          SellerQuoteField.shippingAmount,
          SellerQuoteField.taxAmount,
          SellerQuoteField.totalAmount,
          SellerQuoteField.validUntil,
        ]),
      );
    });

    test('兩個值都填齊就都能送出', () {
      for (final type in kSelectableResponseTypes) {
        expect(
          validateSellerQuote(
            _completeQuote(type: type),
            now: _now,
          ).isSubmittable,
          isTrue,
          reason: type.name,
        );
      }
    });
  });

  group('值域', () {
    test('UNSPECIFIED 的 responseType 不能送出', () {
      final result = validateSellerQuote(
        _completeQuote().copyWith(
          responseType: SellerResponseType.SELLER_RESPONSE_TYPE_UNSPECIFIED,
        ),
        now: _now,
      );
      expect(result.errorFor(SellerQuoteField.responseType), isNotNull);
    });

    test('可選的 responseType 與含稅基準各兩個值，含稅基準缺一不可送出', () {
      expect(kSelectableResponseTypes, hasLength(2));
      expect(kSelectableTaxBases, hasLength(2));
      final result = validateSellerQuote(
        _completeQuote().copyWith(
          taxInclusionBasis: TaxInclusionBasis.TAX_INCLUSION_BASIS_UNSPECIFIED,
        ),
        now: _now,
      );
      expect(result.errorFor(SellerQuoteField.taxInclusionBasis), isNotNull);
    });
  });

  group('金額恆等式', () {
    test('total ≠ goods + shipping + tax 時擋下並附上計算值', () {
      final result = validateSellerQuote(
        _completeQuote(total: '11000'),
        now: _now,
      );
      expect(result.amountIdentityHolds, isFalse);
      expect(result.computedTotal, '11025');
      expect(result.errorFor(SellerQuoteField.totalAmount), contains('11025'));
      expect(result.isSubmittable, isFalse);
    });

    test('恆等式成立時 computedTotal 只是提示，不改寫使用者輸入', () {
      final quote = _completeQuote(total: '11025');
      final result = validateSellerQuote(quote, now: _now);
      expect(result.amountIdentityHolds, isTrue);
      expect(result.isSubmittable, isTrue);
      expect(quote.totalAmount, '11025');
      expect(result.computedTotal, '11025');
    });

    test('金額是非負整數：小數與負號都不接受', () {
      for (final bad in ['1200.5', '-5', '1,200']) {
        final result = validateSellerQuote(
          _completeQuote(goods: bad),
          now: _now,
        );
        expect(
          result.errorFor(SellerQuoteField.goodsAmount),
          isNotNull,
          reason: bad,
        );
      }
    });

    test('金額格式不合時報格式錯誤', () {
      final result = validateSellerQuote(
        _completeQuote(goods: '一萬'),
        now: _now,
      );
      expect(result.errorFor(SellerQuoteField.goodsAmount), isNotNull);
      expect(result.computedTotal, isNull);
    });
  });

  group('有效期限', () {
    test('缺 validUntil 擋下', () {
      final result = validateSellerQuote(
        _completeQuote(validUntil: ''),
        now: _now,
      );
      expect(result.errorFor(SellerQuoteField.validUntil), isNotNull);
    });

    test('已過期的 validUntil 擋下', () {
      final result = validateSellerQuote(
        _completeQuote(validUntil: '2026-01-01T00:00:00Z'),
        now: _now,
      );
      expect(result.errorFor(SellerQuoteField.validUntil), contains('晚於現在'));
    });

    test('promisedDeliveryAt 選填，但填了就要是 RFC3339', () {
      expect(
        validateSellerQuote(
          _completeQuote().copyWith(promisedDeliveryAt: ''),
          now: _now,
        ).errorFor(SellerQuoteField.promisedDeliveryAt),
        isNull,
      );
      expect(
        validateSellerQuote(
          _completeQuote().copyWith(promisedDeliveryAt: '2026-12-20'),
          now: _now,
        ).errorFor(SellerQuoteField.promisedDeliveryAt),
        isNotNull,
      );
    });
  });

  test('deviationDisclosure 無偏離時留空也能送出', () {
    final result = validateSellerQuote(
      _completeQuote().copyWith(deviationDisclosure: ''),
      now: _now,
    );
    expect(result.isSubmittable, isTrue);
  });
}
