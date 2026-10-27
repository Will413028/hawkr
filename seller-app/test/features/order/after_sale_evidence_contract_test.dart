import 'dart:convert';

import 'package:hawkr_seller/features/order/data/after_sale_evidence.dart';
import 'package:hawkr_seller/features/order/data/after_sale_model.dart';
import 'package:flutter_test/flutter_test.dart';

/// 售後佐證圖與商品圖的欄位契約。
///
/// fixture 是 2026-08-05 對 dev 實打
/// `GET /api/v1/app/seller/after_sales/A724174691`（seller 2685914682）
/// 回應的逐字複製，未經整理——不是照設計書手打的理想形狀。
const _realEvidenceJson = '''
{"assetId":"866540497739859931",
 "imageKey":"public/aftersale/2623900520/20260719/ce35c925-7008-4dbb-90f0-d48e0d57b7c5.png",
 "imageUrl":"https://test-media.example.com/i/original/public/aftersale/2623900520/20260719/ce35c925-7008-4dbb-90f0-d48e0d57b7c5.png",
 "evidenceType":"8601",
 "createdAt":"2026-07-19T04:42:34.24775Z"}
''';

/// 同一次實打的 `items[0]`。
///
/// 注意它**沒有** unitPrice / listPrice / shareDiscount——
/// Gateway `AfterSaleCaseItemResponse`（`aftersales.go:133-143`）就只有這 8 個
/// 欄位，訂單那邊才有 unitPrice。所以 AfterSaleItem 上那三個欄位恆為空，
/// `effectiveUnitPrice` 永遠 fallback 到 originalPrice。
const _realItemJson = '''
{"orderItemId":"866538678507455741",
 "skuId":"308",
 "skuName":"預設規格",
 "quantity":"1",
 "originalPrice":"120",
 "actualPayAmount":"120",
 "imageKey":"public/product/2685914682/20260718/f8d18047-9b3a-4c1b-8a21-7b8f2fdabecc.png",
 "imageUrl":"https://test-media.example.com/i/original/public/product/2685914682/20260718/f8d18047-9b3a-4c1b-8a21-7b8f2fdabecc.png"}
''';

Map<String, dynamic> _decode(String raw) =>
    jsonDecode(raw) as Map<String, dynamic>;

void main() {
  group('evidence 佐證圖', () {
    test('讀得出實打回應裡的 imageKey', () {
      final keys = buyerIssueEvidenceImageKeys([_decode(_realEvidenceJson)]);
      expect(keys, [
        'public/aftersale/2623900520/20260719/'
            'ce35c925-7008-4dbb-90f0-d48e0d57b7c5.png',
      ]);
    });

    test('回的是 objectKey 而不是組好的顯示 URL', () {
      // SSOT 是 key；顯示 URL 交回 buildImageUrl 唯一組裝入口（49 §6.1）。
      final keys = buyerIssueEvidenceImageKeys([_decode(_realEvidenceJson)]);
      expect(keys.single.startsWith('http'), isFalse);
    });

    test('evidenceType 只認數字碼，8602 驗貨照片不算買家舉證', () {
      final inspection = _decode(_realEvidenceJson)
        ..['evidenceType'] = AfterSaleEvidenceType.inspectionPhoto
        ..['imageKey'] = 'public/aftersale/seller-inspection.png';
      expect(buyerIssueEvidenceImageKeys([inspection]), isEmpty);

      expect(
        afterSaleEvidenceImageKeys([
          inspection,
        ], includeType: (t) => t == AfterSaleEvidenceType.inspectionPhoto),
        ['public/aftersale/seller-inspection.png'],
      );
    });

    test('去重且維持原順序', () {
      final a = _decode(_realEvidenceJson);
      final b = _decode(_realEvidenceJson)
        ..['imageKey'] = 'public/aftersale/second.png';
      final dup = _decode(_realEvidenceJson);
      expect(buyerIssueEvidenceImageKeys([a, b, dup]), [
        'public/aftersale/2623900520/20260719/'
            'ce35c925-7008-4dbb-90f0-d48e0d57b7c5.png',
        'public/aftersale/second.png',
      ]);
    });

    // ── 防復發：已刪的候選鍵單獨給值，必須解不出任何圖 ──────────────
    //
    // 這是本組測試真正的鑑別力所在。任何一個 fallback 復活，
    // 對應那條就會從 isEmpty 變成有值而轉紅。
    test('已刪的 4 個圖片候選鍵單獨給值都解不出圖', () {
      for (final deadKey in const ['url', 'images', 'imageUrls', 'photoUrls']) {
        final evidence = <String, dynamic>{
          'evidenceType': AfterSaleEvidenceType.buyerPhoto,
          deadKey: 'public/aftersale/should-not-be-read.png',
        };
        expect(
          buyerIssueEvidenceImageKeys([evidence]),
          isEmpty,
          reason: '$deadKey 不是契約欄位，任何端點都不回，不該被讀',
        );
      }
    });

    test('imageUrl 不再被當成圖片來源（它是 key 的衍生物）', () {
      final evidence = <String, dynamic>{
        'evidenceType': AfterSaleEvidenceType.buyerPhoto,
        'imageUrl': 'https://test-media.example.com/i/original/public/x/y.png',
      };
      expect(buyerIssueEvidenceImageKeys([evidence]), isEmpty);
    });

    test('已刪的 2 個型別候選鍵單獨給值都不會被當成買家舉證', () {
      for (final deadTypeKey in const ['evidence_type', 'type']) {
        final evidence = <String, dynamic>{
          deadTypeKey: AfterSaleEvidenceType.buyerPhoto,
          'imageKey': 'public/aftersale/should-not-be-read.png',
        };
        expect(
          buyerIssueEvidenceImageKeys([evidence]),
          isEmpty,
          reason: '$deadTypeKey 不是契約欄位；缺 evidenceType 就不該通過型別過濾',
        );
      }
    });

    test('已刪的角色字面值 BUYER / BUYER_PROOF 不再被接受', () {
      for (final deadType in const ['BUYER', 'BUYER_PROOF']) {
        final evidence = <String, dynamic>{
          'evidenceType': deadType,
          'imageKey': 'public/aftersale/should-not-be-read.png',
        };
        expect(
          buyerIssueEvidenceImageKeys([evidence]),
          isEmpty,
          reason: 'evidenceType 是數字碼 8601/8602，不是角色字面值',
        );
      }
    });

    test('evidenceType 為空不再放行（實打回應一律帶值）', () {
      final evidence = <String, dynamic>{
        'imageKey': 'public/aftersale/should-not-be-read.png',
      };
      expect(buyerIssueEvidenceImageKeys([evidence]), isEmpty);
    });
  });

  group('AfterSaleItem 商品圖', () {
    test('讀得出實打回應裡的 imageKey', () {
      final item = AfterSaleItem.fromJson(_decode(_realItemJson));
      expect(
        item.imageKey,
        'public/product/2685914682/20260718/'
        'f8d18047-9b3a-4c1b-8a21-7b8f2fdabecc.png',
      );
      expect(item.imageKey.startsWith('http'), isFalse);
    });

    test('只給 imageUrl（舊讀法）時解不出圖', () {
      final json = _decode(_realItemJson)..remove('imageKey');
      expect(AfterSaleItem.fromJson(json).imageKey, isEmpty);
    });

    test('其餘欄位仍照實打形狀解析', () {
      final item = AfterSaleItem.fromJson(_decode(_realItemJson));
      expect(item.orderItemId, '866538678507455741');
      expect(item.skuId, '308');
      expect(item.skuName, '預設規格');
      expect(item.quantity, '1');
      expect(item.originalPrice, '120');
      expect(item.actualPayAmount, '120');
    });

    test('unitPrice / listPrice / shareDiscount 在售後端點恆為空', () {
      // 不是 bug，是契約差異：Gateway 的售後 item DTO 沒有這三個欄位
      // （訂單 item 才有）。寫成測試是為了讓「哪天後端補上了」這件事
      // 由這條轉紅來通知，而不是靠人回頭發現 fallback 一直沒被用到。
      final item = AfterSaleItem.fromJson(_decode(_realItemJson));
      expect(item.unitPrice, isEmpty);
      expect(item.listPrice, isEmpty);
      expect(item.shareDiscount, isEmpty);
      expect(item.effectiveUnitPrice, '120');
    });
  });
}
