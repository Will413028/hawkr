import 'package:flutter_test/flutter_test.dart';

import 'package:hawkr_seller/features/product/data/product_upload_service.dart';

/// 守住「編輯商品時 SKU 的身分不會消失」這個不變式。
///
/// 背景：`PUT /app/products/{id}` 只在 `skus[].id` 非空時走 UPDATE；沒有 id 一律 INSERT，
/// 並把不在保留清單裡的既有 SKU 實體刪除。App 曾經完全不送 id ⇒ 每次編輯都把整組 SKU
/// 換成新 TSID，樂觀鎖 CAS 從不執行，購物車與訂單裡的 sku_id 靜默指向不存在的列。
///
/// `expectedVersion` 只在 UPDATE 分支被讀取 ⇒ 只補 expectedVersion 不補 id，後端會**靜默
/// 丟棄**它（HTTP 200、無 log、無測試會紅）。兩者必須成對，這也是下面第三個 case 的理由。
void main() {
  SpecData spec({String? skuId, String? skuVersion}) => SpecData(
    skuId: skuId,
    skuVersion: skuVersion,
    grade: '大顆裝',
    price: 300,
    stock: 10,
    weight: 1.5,
    lengthCm: 20,
    widthCm: 15,
    heightCm: 10,
  );

  group('SpecData shipping methods', () {
    test('submitted shipping methods do not carry a temperature', () {
      final json = SpecData(
        grade: '大顆裝',
        price: 300,
        stock: 10,
        shippingMethods: [
          SpecData.shippingMethodJson(
            type: '3651',
            feePayer: '3101',
            freeThreshold: '0',
          ),
        ],
      ).toSkuJson();

      final methods = json['shippingMethods'] as List;
      expect(methods.single, {
        'type': '3651',
        'feePayer': '3101',
        'freeThreshold': '0',
      });
    });
  });

  group('SpecData.toSkuJson', () {
    test('既有 SKU：id 與 expectedVersion 成對送出', () {
      final json = spec(
        skuId: '7495464541774413824',
        skuVersion: '19',
      ).toSkuJson();

      expect(json['id'], '7495464541774413824');
      expect(json['expectedVersion'], '19');
    });

    test('新增規格（無 id）：兩個欄位都不送，維持 INSERT 語意', () {
      final json = spec().toSkuJson();

      // 送空字串 id 會讓後端的「帶 id 必須屬於本商品」檢查與 INSERT 分支語意都變糊，
      // 所以是「不送」而不是「送空字串」。
      expect(json.containsKey('id'), isFalse);
      expect(json.containsKey('expectedVersion'), isFalse);
    });

    test('只有 version 沒有 id：expectedVersion 不送（送了也會被後端丟棄）', () {
      final json = spec(skuVersion: '19').toSkuJson();

      expect(json.containsKey('id'), isFalse);
      expect(json.containsKey('expectedVersion'), isFalse);
    });

    test('有 id 但後端沒下發 version：仍送 id（不比對版本，非回歸）', () {
      final json = spec(skuId: '7495464541774413824').toSkuJson();

      expect(json['id'], '7495464541774413824');
      expect(json.containsKey('expectedVersion'), isFalse);
    });

    test('空白字串視同未提供', () {
      final json = spec(skuId: '  ', skuVersion: '  ').toSkuJson();

      expect(json.containsKey('id'), isFalse);
      expect(json.containsKey('expectedVersion'), isFalse);
    });

    test('既有欄位不受影響', () {
      final json = spec(skuId: '123', skuVersion: '4').toSkuJson();

      expect(json['skuName'], '大顆裝');
      expect(json['price'], '300');
      expect(json['stockQty'], '10');
      expect(json['weight'], '1.5');
      expect(json['lengthCm'], '20');
    });
  });

  group('ProductUploadTask.displayMessage', () {
    ProductUploadTask task(String? msg) => ProductUploadTask(
      id: '1',
      productName: 'p',
      createdAt: DateTime(2026),
      errorMessage: msg,
    );

    test('剝掉 Dart 的 Exception: 前綴（否則會原樣印在賣家畫面上）', () {
      expect(
        task('Exception: 資料已被更新，請重新載入後再試').displayMessage,
        '資料已被更新，請重新載入後再試',
      );
    });

    test('沒有前綴時原樣保留', () {
      expect(task('網路連線失敗，請稍後再試').displayMessage, '網路連線失敗，請稍後再試');
    });

    test('空值與空字串回 null，讓呼叫端用自己的預設文案', () {
      expect(task(null).displayMessage, isNull);
      expect(task('   ').displayMessage, isNull);
    });
  });

  group('版本衝突業務碼', () {
    test('與後端 CodeMeta 的 DELEGATION_RESOURCE_VERSION_CONFLICT 一致', () {
      // 值改了就代表後端 SSOT 改了，這支測試是提醒去對照 錯誤碼.md 的錨。
      expect(kVersionConflictBizCode, 9266);
    });
  });
}
