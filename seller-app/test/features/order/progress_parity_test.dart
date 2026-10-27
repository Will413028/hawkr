import 'package:hawkr_seller/features/order/data/payuni_logistics_service.dart';
import 'package:flutter_test/flutter_test.dart';

/// 出貨進度百分比的**跨 repo 對拍**。
///
/// 🔴 為什麼需要它：同一張 (PayUni 貨態碼 → 進度百分比) 對照表在本 repo 與
/// `hawkr-website` 各有一份逐值相同的實作（App 的
/// `LogisticsStatusCode.getProgressPercent`／website 的 `progressPercentFor`），
/// 而**沒有任何東西保證它們同步**。
///
/// 兩份不會同時錯，只會**先後**錯：有人改了一邊，另一邊照舊，
/// 於是同一筆訂單在 App 與網頁上顯示不同的進度——而兩邊的測試都是綠的。
///
/// ⚠️ 本 gate 擋不住什麼（誠實標明，別把它當成牆）：
/// digest 在兩個 repo 各硬寫一份，**它不會自動比對另一個 repo**。
/// 它能做到的是：改了邏輯而沒改 digest ⇒ 這一邊轉紅 ⇒ 那個人會看到這段註解，
/// 知道另一邊也要改。把「悄悄漂移」變成「明著改兩個 digest」。
///
/// 🔴 **改動這張表時，兩個 repo 的 digest 必須一起更新。**
/// 對應檔案：`hawkr-website/lib/seller/shipment-progress-parity.unit.test.ts`
///
/// 判定紀錄（2026-09-10）：這兩份吃的是 `providerStatus`（PayUni 原生碼），
/// **不吃** normalized status ⇒ 物流履約 Foundation 新增的 7 個 normalized 值
/// 不會讓它們漂移 ⇒ 不是 Foundation correctness risk。本 gate 防的是未來的人為漂移。
void main() {
  /// 涵蓋兩張表的每一個 case ＋ default 區間。
  final codes = <int>[...List<int>.generate(41, (i) => i), 99];

  /// FNV-1a 32-bit，與 website 側逐字相同的演算法。
  String fnv1a32(String input) {
    var h = 0x811c9dc5;
    for (final unit in input.codeUnits) {
      h ^= unit;
      h = (h * 0x01000193) & 0xFFFFFFFF;
    }
    return h.toRadixString(16).padLeft(8, '0');
  }

  String canonicalise() {
    final parts = <String>[];
    for (final entry in <List<String>>[
      ['HOME', PayuniLogisticsService.lgsTypeHOME],
      ['C2C', PayuniLogisticsService.lgsTypeC2C],
    ]) {
      for (final code in codes) {
        parts.add(
          '${entry[0]}:$code=${LogisticsStatusCode.getProgressPercent(code, entry[1])}',
        );
      }
    }
    return parts.join(';');
  }

  test('digest 與 website 的同一張表一致', () {
    expect(
      fnv1a32(canonicalise()),
      '0480fbc6',
      reason:
          '進度對照表改動了。🔴 hawkr-website 的 '
          'lib/seller/shipment-progress-parity.unit.test.ts 必須同批更新，'
          '否則同一筆訂單在 App 與網頁上會顯示不同的進度，而兩邊的測試都是綠的。',
    );
  });

  test('未知配送軸回 null，不借用任何一張表的百分比', () {
    for (final code in codes) {
      expect(LogisticsStatusCode.getProgressPercent(code, 'B2C'), isNull);
      expect(LogisticsStatusCode.getProgressPercent(code, ''), isNull);
    }
  });

  /// 測試自身的鑑別力：兩張表若產出完全相同的結果，本 gate 對「軸接錯」是盲的。
  test('兩張表必須真的不同（否則本 gate 對軸接錯完全無感）', () {
    final home = codes
        .map(
          (c) => LogisticsStatusCode.getProgressPercent(
            c,
            PayuniLogisticsService.lgsTypeHOME,
          ),
        )
        .join(',');
    final c2c = codes
        .map(
          (c) => LogisticsStatusCode.getProgressPercent(
            c,
            PayuniLogisticsService.lgsTypeC2C,
          ),
        )
        .join(',');
    expect(home, isNot(equals(c2c)));
  });
}
