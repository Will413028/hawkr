import 'package:hawkr_buyer/features/cart/presentation/widgets/shipping_section.dart';
import 'package:flutter/material.dart';
import 'package:hawkr_buyer/features/address/data/address_model.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../support/production_layout_test_harness.dart';

/// **本版 App 沒有對應 UI 的配送方式不得出現在購物車的寄送方式清單**
/// （`ShippingSection._groupOptions` 的第一道防線）。
///
/// ## 為什麼需要這一支
///
/// 本波（物流 Shared Foundation）在 CodeMeta 新增了 `3004`（7-11 大宗寄倉）與
/// `3005`（商家自行配送），兩者在 App 端都是 `ShippingPresentation.unsupported`
/// ——它們的流程與既有 3002 C2C 不同，借用同一個畫面會顯示錯誤的取件方式與代碼欄位。
///
/// `3003`（Uber 即時配送）已刪除（ADR 2026-10-27-general-ecommerce-scope-cut D4），
/// 值域仍在 CodeMeta，同樣是 unsupported。
///
/// 🔴 **這條路徑先前零覆蓋，而且無法用 dev 資料補**：dev 的
/// `chk_shipping_type CHECK (shipping_type IN ('3651','3652'))` 讓那些配送方式
/// 根本建不出來 ⇒ 截圖驗不到、E2E 也驗不到。widget test 是唯一驗得到的層。
///
/// ## 這一支不證明什麼（誠實標註）
///
/// 它驗的是「清單有沒有濾掉」，不驗 `checkout_payment_sheet` 的另外兩道
/// （`_loadPreview` 的 unsupported 錯誤、`_handleCheckout` 的 dialog）——
/// 那兩道要 mock preview API，另案處理。
void main() {
  testWidgets('unsupported 配送方式被 _groupOptions 濾掉，supported 的仍在', (
    tester,
  ) async {
    await _pump(
      tester,
      // 🔴 四個選項的 label 刻意互不相同，且 supported 與 unsupported 各有其一：
      //    過濾若整個失效（全顯示）或反向（只顯示 unsupported），兩種都會被抓到。
      //    若四者共用同一個 label，這支測試對「濾錯邊」零鑑別力。
      const [
        {'shippingMethod': '3001', 'label': '黑貓宅配', 'fee': 100},
        {'shippingMethod': '3003', 'label': 'UberInstant', 'fee': 150},
        {'shippingMethod': '3004', 'label': 'BulkWarehouse', 'fee': 60},
        {'shippingMethod': '3005', 'label': 'MerchantSelf', 'fee': 0},
      ],
    );

    // supported：3001 必須出現
    expect(find.textContaining('黑貓宅配'), findsWidgets);

    // unsupported：三個都不得出現。分開寫而不是合成一條，
    // 是為了讓失敗訊息直接指出「哪一個碼值漏濾了」。
    expect(find.textContaining('UberInstant'), findsNothing);
    expect(find.textContaining('BulkWarehouse'), findsNothing);
    expect(find.textContaining('MerchantSelf'), findsNothing);
  });

  testWidgets('整份清單都是 unsupported 時不會渲染出任何寄送方式群組', (tester) async {
    await _pump(tester, const [
      {'shippingMethod': '3003', 'label': 'UberInstant', 'fee': 150},
      {'shippingMethod': '3005', 'label': 'MerchantSelf', 'fee': 0},
    ]);

    expect(find.textContaining('UberInstant'), findsNothing);
    expect(find.textContaining('MerchantSelf'), findsNothing);
    // ⚠️ 這一條是上一個 case 的反面：確認「全部濾掉」走的是空清單分支，
    //    而不是把最後一個 unsupported 當成 fallback 顯示出來。
    expect(tester.takeException(), isNull);
  });

  testWidgets('空字串與 null 的 shippingMethod 同樣被濾掉（fail-safe，不是崩潰）', (
    tester,
  ) async {
    await _pump(tester, const [
      {'shippingMethod': '3001', 'label': '黑貓宅配', 'fee': 100},
      {'shippingMethod': '', 'label': 'EmptyMethod', 'fee': 0},
      {'label': 'MissingMethod', 'fee': 0},
    ]);

    expect(find.textContaining('黑貓宅配'), findsWidgets);
    expect(find.textContaining('EmptyMethod'), findsNothing);
    expect(find.textContaining('MissingMethod'), findsNothing);
    expect(tester.takeException(), isNull);
  });

  testWidgets('validate 判定整車超限時顯示「無法配送」錯誤卡', (tester) async {
    await _pump(tester, const [
      {'shippingMethod': '3001', 'label': '黑貓宅配', 'fee': 100},
    ], validateValid: false);

    // ⚠️ 斷言的是面板標題而不是 `errorMessage`：`_buildValidateErrorPanel`
    //    今天用的是固定文案、不讀後端的 errorMessage（那是另一個缺口，
    //    不在本次範圍）。拿 errorMessage 當錨會驗到一個不存在的行為。
    expect(find.text('無法配送'), findsOne);
    expect(tester.takeException(), isNull);
  });
}

Future<void> _pump(
  WidgetTester tester,
  List<Map<String, dynamic>> options, {
  bool validateValid = true,
}) async {
  await pumpProductionLayoutTestApp(
    tester,
    size: const Size(390, 1000),
    home: Scaffold(
      body: SingleChildScrollView(
        child: ShippingSection(
          shippingOptions: options,
          selectedShippingIdx: 0,
          validateResult: validateValid
              ? const {'valid': true, 'errors': []}
              : const {
                  'valid': false,
                  'errors': [
                    {
                      'errorCode': 'WEIGHT_EXCEEDED',
                      'errorMessage': '超過單件重量上限',
                    },
                  ],
                },
          validateLoading: false,
          selectedAddress: const Address(
            etag: 'v1',
            id: 1,
            townCode: '63000030',
            addressDetail: '測試路1號',
            receiverName: '測試收件人',
            receiverPhone: '0912345678',
          ),
          userAddresses: const [
            Address(
              id: 1,
              townCode: '63000030',
              etag: 'v1',
              receiverName: '測試收件人',
              receiverPhone: '0912345678',
            ),
          ],
          storePickups: const [],
          selectedStorePickupId: null,
          checkedSkuIds: const {101},
          onShippingIdxChanged: (_) {},
          onStorePickupSelected: (_) {},
          onExpandedGroupsChanged: (_) {},
          onShowAddressPicker: () {},
          onOpenStorePickupForm: (_) {},
          onAddAddress: () {},
          onRetryValidate: () {},
        ),
      ),
    ),
  );
}
