import 'package:hawkr_seller/core/codemeta/code_meta.dart';
import 'package:hawkr_seller/core/exceptions/app_exception.dart';
import 'package:hawkr_seller/features/procurement/data/procurement_error.dart';
import 'package:hawkr_seller/features/procurement/data/seller_quote_model.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';

/// brief 列出的十個新碼。
const List<String> _newCodes = <String>[
  'STALE_RFQ_SNAPSHOT',
  'STALE_PUBLICATION',
  'STALE_SELLER_RESPONSE',
  'RFQ_RESPONSE_ALREADY_SUBMITTED',
  'RFQ_RESPONSE_EXPIRED',
  'RFQ_RECIPIENT_CLOSED',
  'RFQ_CHECKOUT_ALREADY_ACTIVE',
  'RFQ_ALREADY_AWARDED',
  'QUOTE_AMOUNT_INCONSISTENT',
  'QUOTE_QUALITY_REQUIRED',
];

/// 沿用既有的四個碼。
const List<String> _reusedCodes = <String>[
  'IDEMPOTENCY_KEY_CONFLICT',
  'STALE_REVISION',
  'RESOURCE_NOT_FOUND',
  'CAPABILITY_UNAVAILABLE',
];

DioException _errorWith(int statusCode, Map<String, dynamic> body) {
  final options = RequestOptions(path: '/x');
  return DioException(
    requestOptions: options,
    type: DioExceptionType.badResponse,
    response: Response<dynamic>(
      requestOptions: options,
      statusCode: statusCode,
      data: body,
    ),
  );
}

void main() {
  group('CodeMeta 值域', () {
    test('十四個碼都已經在生成物裡（code_meta.dart 不手改）', () {
      for (final code in [..._newCodes, ..._reusedCodes]) {
        expect(
          ProcurementErrorReason.fromCode(code),
          isNotNull,
          reason: '$code 不在 ProcurementErrorReason',
        );
      }
    });
  });

  group('錯誤碼 → 使用者可讀訊息', () {
    test('十四個碼都有對照，訊息不是空的也不是原始碼字串', () {
      for (final code in [..._newCodes, ..._reusedCodes]) {
        final presentation = procurementErrorPresentation(code);
        expect(presentation, isNotNull, reason: code);
        expect(presentation!.message.trim(), isNotEmpty, reason: code);
        expect(presentation.message, isNot(contains(code)), reason: code);
      }
    });

    test('未知碼與空碼回 null', () {
      expect(procurementErrorPresentation('NOT_A_REAL_CODE'), isNull);
      expect(procurementErrorPresentation(''), isNull);
      expect(procurementErrorPresentation(null), isNull);
    });

    test('STALE_* 系列要求重新載入，不得靜默重試', () {
      for (final code in const [
        'STALE_RFQ_SNAPSHOT',
        'STALE_PUBLICATION',
        'STALE_SELLER_RESPONSE',
        'STALE_REVISION',
      ]) {
        expect(
          procurementErrorPresentation(code)!.requiresReload,
          isTrue,
          reason: code,
        );
      }
    });

    test('STALE_SELLER_RESPONSE 的訊息明講報價已變動', () {
      final presentation = procurementErrorPresentation(
        'STALE_SELLER_RESPONSE',
      )!;
      expect(presentation.message, contains('報價已經變動'));
      expect(presentation.recovery, ProcurementRecovery.reloadRequired);
    });

    test('終局碼不給重送的出路', () {
      for (final code in const [
        'RFQ_RECIPIENT_CLOSED',
        'RFQ_CHECKOUT_ALREADY_ACTIVE',
        'RFQ_ALREADY_AWARDED',
        'RESOURCE_NOT_FOUND',
      ]) {
        expect(
          procurementErrorPresentation(code)!.recovery,
          ProcurementRecovery.terminal,
          reason: code,
        );
      }
    });

    test('IDEMPOTENCY_KEY_CONFLICT 要求換一把新鍵', () {
      expect(
        procurementErrorPresentation('IDEMPOTENCY_KEY_CONFLICT')!.recovery,
        ProcurementRecovery.newIdempotencyKey,
      );
    });
  });

  group('ProcurementException.from', () {
    test('已知碼帶對照與 statusCode', () {
      final error = ProcurementException.from(
        _errorWith(422, {'reason': 'QUOTE_AMOUNT_INCONSISTENT'}),
      );
      expect(error, isA<ProcurementException>());
      final procurement = error as ProcurementException;
      expect(procurement.code, 'QUOTE_AMOUNT_INCONSISTENT');
      expect(procurement.statusCode, 422);
      expect(procurement.requiresReload, isFalse);
      expect(procurement.requiresNewIdempotencyKey, isFalse);
      expect(procurement.toString(), contains('QUOTE_AMOUNT_INCONSISTENT'));
    });

    test('未知碼落回 server 原文', () {
      final error =
          ProcurementException.from(
                _errorWith(400, {
                  'reason': 'SOMETHING_NEW',
                  'messageZh': '缺少 Idempotency-Key',
                }),
              )
              as ProcurementException;
      expect(error.message, '缺少 Idempotency-Key');
      expect(error.presentation, isNull);
      expect(error.requiresReload, isFalse);
    });

    test('沒有碼也沒有訊息時交回通用 AppException', () {
      final error = ProcurementException.from(_errorWith(500, {'code': 0}));
      expect(error, isA<AppException>());
      expect(error, isNot(isA<ProcurementException>()));
    });

    test('body 不是 map（連線層錯誤）交回通用 AppException', () {
      final options = RequestOptions(path: '/x');
      final error = ProcurementException.from(
        DioException(
          requestOptions: options,
          type: DioExceptionType.connectionError,
        ),
      );
      expect(error, isA<NetworkException>());
    });
  });

  group('產品規則 3：賣家端不得出現買家預算、PII、他人報價與排序', () {
    test('需求內容的型別就是十欄，是 deny-by-default 的那道防線', () {
      const payload = SellerFacingPayload(
        qualityKnowledgeState: 'BUYER_SPECIFIED',
        item: '高麗菜',
        itemLabel: '梨山高麗菜',
        quantity: '200',
        unit: 'BOX',
        packaging: '紙箱',
        deliveryCity: '臺北市',
        deliveryDate: '2026-09-20',
        responseDeadline: '2026-09-18',
      );
      expect(payload.toJson().keys.toList(), <String>[
        'qualityKnowledgeState',
        'item',
        'itemLabel',
        'quantity',
        'unit',
        'packaging',
        'deliveryCity',
        'deliveryDate',
        'responseDeadline',
        'buyerNote',
      ]);
    });

    // ⛔ 只有這兩個鍵獲准帶上面那些片語，其餘一律視為漏擋。
    //
    // `qualityKnowledgeState` 是「買家知不知道品相」的宣告，不是品相本身。
    // `buyerNote` 是買家自己打字、明知會給供應商看的補充說明（backend #3410）
    // ——它是**唯一**一個帶「buyer」而仍然該出現在賣家視野裡的鍵，因為它的內容
    // 就是買家寫給賣家的。買家私有的 budgetPerUnit／buyerRequestedRange 仍然
    // 一律不得出現。
    const allowedDespiteFragment = <String>{
      'qualityKnowledgeState',
      'buyerNote',
    };

    test('十欄裡沒有買家預算、可接受區間、候選排序這類鍵', () {
      const forbidden = [
        'budget',
        'price',
        'range',
        'rank',
        'score',
        'buyer',
        'competitor',
        'seller',
      ];
      for (final key in const SellerFacingPayload().toJson().keys) {
        if (allowedDespiteFragment.contains(key)) continue;
        final normalized = key.toLowerCase();
        for (final fragment in forbidden) {
          expect(
            normalized.contains(fragment),
            isFalse,
            reason: '$key 看起來像敏感欄位，加欄位前先確認',
          );
        }
      }
    });

    test('豁免清單本身不得長大：每一個都要是刻意的決定', () {
      expect(allowedDespiteFragment, <String>{
        'qualityKnowledgeState',
        'buyerNote',
      }, reason: '要放行第三個鍵，先在這裡寫下它為什麼該給賣家看');
    });

    test('fromJson 只認那十個鍵，回應多出來的敏感欄位進不了 model', () {
      final payload = SellerFacingPayload.fromJson(<String, dynamic>{
        'item': '高麗菜',
        'quantity': '200',
        'budgetPerUnit': '38',
        'acceptableRange': '35-40',
        'candidateRank': '2',
        'buyerPhone': '0912345678',
      });
      expect(payload.item, '高麗菜');
      expect(payload.quantity, '200');
      final serialized = payload.toJson().values.map((v) => '$v').toList();
      for (final leaked in const ['38', '35-40', '2', '0912345678']) {
        expect(serialized, isNot(contains(leaked)), reason: leaked);
      }
      expect(payload.contextFields.map((f) => f.value), <String>['高麗菜', '200']);
    });

    test('空字串的欄位不顯示——那代表這張 RFQ 沒填這一項，不是載入失敗', () {
      const payload = SellerFacingPayload(item: '高麗菜', packaging: '   ');
      expect(payload.contextFields.map((f) => f.label), <String>['品項']);
      expect(const SellerFacingPayload().contextFields, isEmpty);
    });
  });

  group('qualityKnowledgeState', () {
    test('空字串＝未知，不補值也不推斷成「買家沒指定」', () {
      const payload = SellerFacingPayload();
      expect(payload.qualityKnowledgeState, '');
      expect(payload.hasQualityKnowledge, isFalse);
    });

    test('有值就是有值，原樣保留', () {
      const payload = SellerFacingPayload(
        qualityKnowledgeState: 'BUYER_SPECIFIED',
      );
      expect(payload.hasQualityKnowledge, isTrue);
      expect(payload.qualityKnowledgeState, 'BUYER_SPECIFIED');
    });

    test('它不混進需求欄位列表，畫面另外處理', () {
      const payload = SellerFacingPayload(
        qualityKnowledgeState: 'BUYER_SPECIFIED',
        item: '高麗菜',
      );
      expect(payload.contextFields.map((f) => f.value), <String>['高麗菜']);
    });
  });
}
