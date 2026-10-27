import 'package:riverpod_annotation/riverpod_annotation.dart';

import 'after_sale_model.dart';
import 'after_sale_repository.dart';

part 'seller_after_sale_lookup.g.dart';

/// 快取「orderNo → 售後案件」的 Map，供賣家訂單卡片查詢與展開區顯示。
///
/// 使用 Map 而非 Set：卡片的 `OrderFlagBanner` / `OrderBottomRow` 只需判斷
/// `containsKey(orderNo)`，`OrderCard` 展開區則能直接取得 [AfterSaleCase]
/// 當作摘要資料，避免每張卡片再發一次 detail 請求。
///
/// 排除 60 已拒絕 / 70 已取消 / 71 確認收貨取消（沒走完整退貨退款流程，展開區渲染會誤導）。
/// 50 已結案保留 — 已完成 tab 的訂單需要展開看完整售後 timeline。
@Riverpod(keepAlive: true)
class SellerAfterSaleLookup extends _$SellerAfterSaleLookup {
  static const _excludedStatuses = {'60', '70', '71'};

  @override
  Future<Map<String, AfterSaleCase>> build() async {
    try {
      final result = await ref
          .read(afterSaleRepositoryProvider)
          .getAfterSaleList(pageSize: 100);
      final map = <String, AfterSaleCase>{};
      for (final c in result.records) {
        // 仲裁中的案件即使 raw status 為 60 已拒絕，也已重新進入平台審理，
        // 必須保留讓賣家在訂單卡看到「申訴仲裁中」，不可當已拒絕排除。
        if (_excludedStatuses.contains(c.status) &&
            !AfterSaleStatus.isUnderAppeal(c.appealState)) {
          continue;
        }
        if (c.orderNo.isEmpty) continue;
        // 同一訂單可能有多筆 case，保留 createdAt 最新的那筆
        final existing = map[c.orderNo];
        if (existing == null || c.createdAt.compareTo(existing.createdAt) > 0) {
          map[c.orderNo] = c;
        }
      }
      return map;
    } catch (_) {
      return {};
    }
  }

  Future<void> refresh() async {
    state = const AsyncLoading();
    state = await AsyncValue.guard(() => build());
  }
}
