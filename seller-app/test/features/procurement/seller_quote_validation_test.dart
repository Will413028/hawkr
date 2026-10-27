import 'package:hawkr_seller/core/codemeta/code_meta.dart';
import 'package:hawkr_seller/features/procurement/data/seller_quote_model.dart';
import 'package:hawkr_seller/features/procurement/presentation/seller_quote_validation.dart';
import 'package:flutter_test/flutter_test.dart';

/// 一份「除了要測的那一項之外都合法」的 payload。
SellerQuotePayload _completeQuote({
  ProcurementSellerResponseType type =
      ProcurementSellerResponseType.manualQuote,
  String grade = '特級',
  String inspectionNotes = '外觀無壓傷，到貨 24 小時內驗收',
  String goods = '10000',
  String shipping = '500',
  String tax = '525',
  String total = '11025',
  String validUntil = '2026-12-31T15:00:00Z',
}) {
  return SellerQuotePayload(
    responseType: type.code,
    offeredQuality: OfferedQuality(
      grade: grade,
      inspectionNotes: inspectionNotes,
    ),
    unitBasis: const UnitBasis(quantity: '1', unit: '公斤'),
    offeredQuantity: '500',
    offeredQuantityUnit: '公斤',
    conversionBasis: '1 箱 ＝ 10 公斤',
    goodsAmount: goods,
    shippingAmount: shipping,
    taxAmount: tax,
    taxInclusionBasis: ProcurementTaxInclusionBasis.taxExcluded.code,
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

  group('產品規則 1：實際品相不得由平台代填', () {
    test('沒填品相等級就不能送出', () {
      final result = validateSellerQuote(_completeQuote(grade: ''), now: _now);
      expect(result.isSubmittable, isFalse);
      expect(result.errorFor(SellerQuoteField.qualityGrade), isNotNull);
    });

    test('沒填驗收說明就不能送出', () {
      final result = validateSellerQuote(
        _completeQuote(inspectionNotes: '   '),
        now: _now,
      );
      expect(result.isSubmittable, isFalse);
      expect(
        result.errorFor(SellerQuoteField.qualityInspectionNotes),
        isNotNull,
      );
    });

    test('空白表單的品相是空的——不會被上一次報價帶入', () {
      const blank = SellerQuotePayload();
      expect(blank.offeredQuality, const OfferedQuality());
      expect(blank.offeredQuality.grade, isEmpty);
      expect(blank.offeredQuality.inspectionNotes, isEmpty);
    });
  });

  group('產品規則 2：ACCEPT_NON_PRICE_TERMS_WITH_QUOTE 不是少填金額的捷徑', () {
    test('兩個 responseType 的必填清單完全相同', () {
      for (final type in ProcurementSellerResponseType.values) {
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
          reason: '${type.code} 少填金額也必須擋下',
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
          SellerQuoteField.qualityGrade,
          SellerQuoteField.qualityInspectionNotes,
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
      for (final type in ProcurementSellerResponseType.values) {
        expect(
          validateSellerQuote(
            _completeQuote(type: type),
            now: _now,
          ).isSubmittable,
          isTrue,
          reason: type.code,
        );
      }
    });
  });

  group('值域', () {
    test('v6.0 範例裡的 SELLER_QUOTE 不是合法值', () {
      expect(ProcurementSellerResponseType.fromCode('SELLER_QUOTE'), isNull);
      final result = validateSellerQuote(
        _completeQuote().copyWith(responseType: 'SELLER_QUOTE'),
        now: _now,
      );
      expect(result.errorFor(SellerQuoteField.responseType), isNotNull);
    });

    test('responseType 只有兩個值', () {
      expect(ProcurementSellerResponseType.values.map((e) => e.code), [
        'MANUAL_QUOTE',
        'ACCEPT_NON_PRICE_TERMS_WITH_QUOTE',
      ]);
    });

    test('taxInclusionBasis 只有兩個值，缺一不可送出', () {
      expect(ProcurementTaxInclusionBasis.values.map((e) => e.code), [
        'TAX_INCLUDED',
        'TAX_EXCLUDED',
      ]);
      final result = validateSellerQuote(
        _completeQuote().copyWith(taxInclusionBasis: ''),
        now: _now,
      );
      expect(result.errorFor(SellerQuoteField.taxInclusionBasis), isNotNull);
    });

    test('currency v1 恆為 TWD', () {
      expect(const SellerQuotePayload().currency, 'TWD');
      final result = validateSellerQuote(
        _completeQuote().copyWith(currency: 'USD'),
        now: _now,
      );
      expect(result.errorFor(SellerQuoteField.currency), isNotNull);
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
      final quote = _completeQuote(total: '11025.00');
      final result = validateSellerQuote(quote, now: _now);
      expect(result.amountIdentityHolds, isTrue);
      expect(result.isSubmittable, isTrue);
      // 使用者填的字面值原封不動。
      expect(quote.totalAmount, '11025.00');
      expect(result.computedTotal, '11025');
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
