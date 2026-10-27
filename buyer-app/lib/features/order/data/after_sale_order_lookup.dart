import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:hawkr_buyer/shared/utils/hk_log.dart';

import '../../../core/session/session_provider.dart';
import 'after_sale_model.dart';
import 'after_sale_repository.dart';

part 'after_sale_order_lookup.g.dart';

/// 售後案件快照（給 order card 顯示用）
class AfterSaleCaseSnapshot {
  const AfterSaleCaseSnapshot({
    required this.caseNo,
    required this.status,
    this.appealState = '',
    this.needRefundPage = false,
  });

  final String caseNo;
  final String status;

  /// 申訴狀態（'0'=未申訴；'10'/'20' 進入平台仲裁）。order card 需據此把
  /// 狀態顯示折疊成「申訴仲裁中」，否則只看 status 會顯示已拒絕 / 待處理。
  final String appealState;

  /// ATM 訂單退款時，買家須前往 PayUni 退款頁填寫銀行帳戶（B-104）
  /// true → order card 顯示「填寫退款資訊」取代「查看售後」
  final bool needRefundPage;

  /// 案件是否在平台仲裁中（申訴未裁決）。
  bool get isUnderAppeal => AfterSaleStatus.isUnderAppeal(appealState);

  String get statusLabel => AfterSaleStatus.displayLabel(status, appealState);
}

/// 快取「哪些 orderNo 有活躍售後案件」+ 該案件狀態，供 order card 查詢。
/// keepAlive 避免每次 rebuild 都重撈。
@Riverpod(keepAlive: true)
class AfterSaleOrderLookup extends _$AfterSaleOrderLookup {
  // 已取消/已結案的 status 不算「有售後案件」（71=確認收貨取消，#1225）
  static const _inactiveStatuses = {'50', '70', '71'};

  @override
  Future<Map<String, AfterSaleCaseSnapshot>> build() async {
    // Apple App Review 合規：訪客（無 token）打 /app/after_sales 必 401，回空不打。
    final session = ref.watch(sessionProvider).value;
    if (!hasUsableSession(session)) {
      return {};
    }
    try {
      final result = await ref
          .read(afterSaleRepositoryProvider)
          .getAfterSaleList(pageSize: 100);
      HKLog.logD(
        '[AfterSaleOrderLookup] build: ${result.records.length} records',
      );
      return {
        for (final c in result.records)
          if (!_inactiveStatuses.contains(c.status))
            c.orderNo: AfterSaleCaseSnapshot(
              caseNo: c.caseNo,
              status: c.status,
              appealState: c.appealState,
              needRefundPage: c.needRefundPage,
            ),
      };
    } catch (_) {
      return {};
    }
  }

  /// 提交售後後呼叫（不等 API 重撈，卡片馬上切換）
  void addOrderNo(String orderNo, {String caseNo = '', String status = '0'}) {
    final current = state.value ?? {};
    state = AsyncData({
      ...current,
      orderNo: AfterSaleCaseSnapshot(caseNo: caseNo, status: status),
    });
  }

  /// 取消售後後呼叫
  void removeOrderNo(String orderNo) {
    final current = state.value ?? {};
    state = AsyncData({...current}..remove(orderNo));
  }

  /// 背景重撈（保留舊資料，避免 UI 在 API 回來前短暫看到 null）
  ///
  /// 由 [OrderListScreen._refreshCurrentTab] 統一呼叫（下拉 / tab 切換 /
  /// FCM / lifecycle resume）。API 成功後覆蓋資料；失敗則保留舊資料。
  Future<void> refresh() async {
    try {
      final fresh = await build();
      state = AsyncData(fresh);
    } catch (e, s) {
      state = AsyncError(e, s);
    }
  }
}
