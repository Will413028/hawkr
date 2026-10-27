import 'package:hawkr_buyer/core/codemeta/shipping_presentation.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../../core/exceptions/app_exception.dart';
import '../../../../core/theme/app_colors.dart';
import '../../../../core/theme/app_typography.dart';
import '../../../../core/theme/hk_tokens.dart';
import '../../../../shared/icons/app_icon_colors.dart';
import '../../../../shared/widgets/common_snackbar.dart';
import '../../../../shared/widgets/hk_icon.dart';
import '../../data/logistics_track.dart';
import '../../data/order_model.dart';
import '../../data/order_repository.dart';
import '../order_status_presentation.dart';
import 'package:hawkr_buyer/shared/widgets/hk_brand_loader.dart';

/// 一張訂單的**即時**物流狀態。
///
/// 🔴 為什麼要另外打一支 API，而不是直接用 `order.shippingStatus`：
/// 訂單上的那一欄是**快照**（由 MQ 事件回填），而
/// `GET /app/orders/{order_no}/logistics` 會讓後端當場向 provider 同步再回傳。
/// 兩者在「包裹剛剛動了但事件還沒到」的窗口裡會不一致，而那正是買家最常看的時點。
/// 🔴 **`retry: … => null` 不是可以省略的樣板**（2026-09-12 實測）。
///
/// Riverpod 3.x 的 provider 預設帶自動重試，而預設的重試述詞是
/// 「錯誤**不是** `Error` 就重試」——本 App 所有 repository 丟的
/// `AppException`（`ServerException`／`NetworkException`／`UnauthorizedException`）
/// 全部是 `Exception` 而非 `Error` ⇒ **全部命中預設重試**。
/// 後果是 `AsyncValue` 永遠停在 `loading`、`.when(error:)` 那一格**永遠畫不出來**，
/// 畫面上就是一個轉不完的「查詢物流狀態中…」，而背景還在無限重打那支 API。
///
/// 實測矩陣（`ProviderContainer` ＋ 2 秒 timeout）：
/// `StateError` ⇒ 2 秒內拿到 `StateError`；`Exception('b')`／`ServerException`
/// ⇒ `TimeoutException`（＝根本沒完成）。加上本行之後 ⇒ 立刻拿到 `ServerException`。
///
/// ⚠️ 這個坑**沒有任何靜態訊號**：編譯過、analyze 乾淨、成功路徑的測試全綠，
/// 只有錯誤路徑的測試會停在 loading —— 而如果沒有人寫錯誤路徑的測試，
/// 它會一路活到使用者手上。
final orderLogisticsProvider = FutureProvider.autoDispose
    .family<LogisticsTrack, String>((ref, orderNo) async {
      final repository = ref.read(orderRepositoryProvider);
      return repository.getOrderLogistics(orderNo);
    }, retry: (retryCount, error) => null);

/// 訂單詳情頁的「物流追蹤」卡片。
///
/// 🔴 **單點狀態不是時間軸**：後端只拿得到 provider 的當前狀態＋時間
/// （`OrderLogisticsResp` 的 doc comment 逐字寫了「PayUni 無多筆軌跡」）。
/// 所以這裡畫的是一個狀態 ＋ 一個時間，**不是**一串節點——
/// 畫成節點會讓使用者以為那些中間節點真的被記錄過。
class OrderLogisticsSection extends ConsumerWidget {
  const OrderLogisticsSection({super.key, required this.order});

  final Order order;

  /// 這張訂單現在有沒有物流單可查。
  ///
  /// 🔴 沒有就**不要打那支 API**：後端對沒有物流單的訂單回非零業務碼，
  /// 打了只會在畫面上生出一張「查詢失敗」的紅卡，而那不是失敗、
  /// 是「賣家還沒出貨」這個完全正常的狀態。
  static bool hasShipment(Order order) =>
      order.shipmentNo.trim().isNotEmpty ||
      order.shippingStatus.trim().isNotEmpty;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    if (!hasShipment(order)) return const SizedBox.shrink();
    final async = ref.watch(orderLogisticsProvider(order.orderSn));

    return Padding(
      key: const ValueKey('order-logistics-section'),
      padding: const EdgeInsets.only(top: HkSpace.lg),
      child: Container(
        decoration: BoxDecoration(
          color: AppColors.bgCard.withValues(alpha: 0.94),
          borderRadius: BorderRadius.circular(HkRadius.lg),
          border: Border.all(color: AppColors.cardBorder),
          boxShadow: HkShadows.card,
        ),
        padding: EdgeInsets.all(16.w),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                HkIcon(
                  HkIconName.truck,
                  size: 20.sp,
                  color: AppIconColors.defaultColor,
                ),
                const SizedBox(width: HkSpace.sm),
                Expanded(
                  child: Text('物流追蹤', style: AppTypography.sectionTitle),
                ),
                _RefreshButton(orderNo: order.orderSn),
              ],
            ),
            SizedBox(height: 12.h),
            async.when(
              loading: () => const _LoadingRow(),
              error: (error, _) => _ErrorRow(
                message: error is AppException ? error.message : '查詢物流狀態失敗',
                orderNo: order.orderSn,
              ),
              data: (track) => _TrackBody(track: track),
            ),
          ],
        ),
      ),
    );
  }
}

class _RefreshButton extends ConsumerWidget {
  const _RefreshButton({required this.orderNo});
  final String orderNo;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return Semantics(
      button: true,
      label: '重新整理物流狀態',
      onTap: () => ref.invalidate(orderLogisticsProvider(orderNo)),
      child: ExcludeSemantics(
        child: Material(
          color: Colors.transparent,
          child: InkWell(
            borderRadius: BorderRadius.circular(HkRadius.sm),
            onTap: () => ref.invalidate(orderLogisticsProvider(orderNo)),
            child: Padding(
              padding: const EdgeInsets.all(HkSpace.sm),
              child: HkIcon(
                HkIconName.refresh,
                size: 18.sp,
                color: AppIconColors.active,
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class _LoadingRow extends StatelessWidget {
  const _LoadingRow();

  @override
  Widget build(BuildContext context) {
    return Row(
      key: const ValueKey('order-logistics-loading'),
      children: [
        SizedBox(
          width: 16.w,
          height: 16.h,
          child: HkBrandLoader(
            tone: HkBrandLoaderTone.mono,
            color: AppColors.primary,
          ),
        ),
        const SizedBox(width: HkSpace.sm),
        Text('查詢物流狀態中…', style: AppTypography.meta),
      ],
    );
  }
}

/// 查詢失敗。
///
/// 🔴 這一格與「沒有物流單」是**兩件事**：後者根本不渲染這張卡（見
/// [OrderLogisticsSection.hasShipment]）。把兩者混在一起，
/// 使用者會在一張剛下單的訂單上看到「查詢失敗」而誤以為出貨出了問題。
class _ErrorRow extends ConsumerWidget {
  const _ErrorRow({required this.message, required this.orderNo});
  final String message;
  final String orderNo;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return Container(
      key: const ValueKey('order-logistics-error'),
      padding: const EdgeInsets.all(HkSpace.md),
      decoration: BoxDecoration(
        color: AppColors.warningSoft,
        borderRadius: BorderRadius.circular(HkRadius.md),
        border: Border.all(color: AppColors.warning.withValues(alpha: 0.42)),
      ),
      child: Row(
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('暫時查不到物流狀態', style: AppTypography.productName),
                const SizedBox(height: HkSpace.xs),
                // 把後端／網路的原訊息帶出來，不換成通用字串。
                Text(message, style: AppTypography.caption),
              ],
            ),
          ),
          Semantics(
            button: true,
            label: '重試',
            onTap: () => ref.invalidate(orderLogisticsProvider(orderNo)),
            child: ExcludeSemantics(
              child: Material(
                color: Colors.transparent,
                child: InkWell(
                  borderRadius: BorderRadius.circular(HkRadius.sm),
                  onTap: () => ref.invalidate(orderLogisticsProvider(orderNo)),
                  child: Padding(
                    padding: const EdgeInsets.symmetric(
                      horizontal: HkSpace.sm,
                      vertical: HkSpace.xs,
                    ),
                    child: Text(
                      '重試',
                      style: AppTypography.meta.copyWith(
                        color: AppColors.primary,
                        fontWeight: FontWeight.w700,
                      ),
                    ),
                  ),
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _TrackBody extends StatelessWidget {
  const _TrackBody({required this.track});
  final LogisticsTrack track;

  /// 失敗／異常狀態沿用訂單卡那一份清單，**不另外寫一份**。
  ///
  /// 🔴 兩份「哪些狀態算異常」一旦分岔，症狀是同一張訂單在列表上是紅的、
  /// 在詳情頁是正常的——而沒有任何東西會報錯。
  bool get _isAbnormal =>
      DeliveryStatusPresentation.abnormalValues.contains(track.status);

  @override
  Widget build(BuildContext context) {
    final presentation = shippingPresentationOf(track.shippingMethod);
    final rows = <Widget>[
      if (track.shipTradeNo.isNotEmpty)
        _CopyRow(label: '物流單號', value: track.shipTradeNo),
      if (presentation == ShippingPresentation.cvsStorePickup) ...[
        if (track.storeName.isNotEmpty)
          _InfoRow(label: '取貨門市', value: track.storeName),
        if (track.cvsStoreCode.isNotEmpty)
          _CopyRow(label: '門市代號', value: track.cvsStoreCode),
      ],
      if (track.recipientName.isNotEmpty)
        _InfoRow(label: '收件人', value: track.recipientName),
      if (track.statusTime.isNotEmpty)
        _InfoRow(label: '狀態更新時間', value: _formatTime(track.statusTime)),
      if (track.createdAt.isNotEmpty)
        _InfoRow(label: '物流單建立時間', value: _formatTime(track.createdAt)),
    ];

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        _StatusBanner(track: track, isAbnormal: _isAbnormal),
        // provider 的原文只在「我們的碼表不認得這個狀態」或「有異常」時才補上。
        // 平時顯示它只是把同一件事講兩次。
        if (track.providerMessage.isNotEmpty &&
            (_isAbnormal || !track.isKnownStatus)) ...[
          SizedBox(height: 6.h),
          Text('物流商回報：${track.providerMessage}', style: AppTypography.caption),
        ],
        if (rows.isNotEmpty) ...[SizedBox(height: 12.h), ...rows],
      ],
    );
  }

  /// ISO 8601（UTC）→ 本地時間字串。
  ///
  /// ⚠️ 解析不出來時**原樣回傳**，不要回空字串：後端給了值卻顯示成空白，
  /// 會讓人以為那個欄位沒有資料。
  static String _formatTime(String iso) {
    final parsed = DateTime.tryParse(iso);
    if (parsed == null) return iso;
    final local = parsed.toLocal();
    String two(int n) => n.toString().padLeft(2, '0');
    return '${local.year}/${two(local.month)}/${two(local.day)} '
        '${two(local.hour)}:${two(local.minute)}';
  }
}

class _StatusBanner extends StatelessWidget {
  const _StatusBanner({required this.track, required this.isAbnormal});
  final LogisticsTrack track;
  final bool isAbnormal;

  @override
  Widget build(BuildContext context) {
    final color = isAbnormal ? AppColors.danger : AppColors.brandPrimaryStrong;
    final background = isAbnormal
        ? AppColors.warningSoft
        : AppColors.primarySubtle;
    return Container(
      key: ValueKey(
        isAbnormal
            ? 'order-logistics-status-abnormal'
            : 'order-logistics-status-normal',
      ),
      width: double.infinity,
      padding: const EdgeInsets.all(HkSpace.md),
      decoration: BoxDecoration(
        color: background,
        borderRadius: BorderRadius.circular(HkRadius.md),
        border: Border.all(color: color.withValues(alpha: 0.32)),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              _brandIcon(),
              const SizedBox(width: HkSpace.sm),
              Expanded(
                child: Text(
                  // 正規化後的狀態文案：後端優先、CodeMeta 回退、最後原樣顯示碼值。
                  track.normalizedStatusLabel,
                  style: AppTypography.productName.copyWith(color: color),
                ),
              ),
            ],
          ),
          if (isAbnormal) ...[
            const SizedBox(height: HkSpace.xs),
            Text('配送發生異常，請查看通知或聯繫客服。', style: AppTypography.caption),
          ],
        ],
      ),
    );
  }

  Widget _brandIcon() {
    return HkIcon(
      HkIconName.truck,
      size: 20.sp,
      color: AppIconColors.defaultColor,
    );
  }
}

class _InfoRow extends StatelessWidget {
  const _InfoRow({required this.label, required this.value});
  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: EdgeInsets.symmetric(vertical: 4.h),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 96.w,
            child: Text(label, style: AppTypography.metaMuted),
          ),
          Expanded(
            child: Text(
              value,
              style: AppTypography.meta,
              textAlign: TextAlign.right,
            ),
          ),
        ],
      ),
    );
  }
}

class _CopyRow extends StatelessWidget {
  const _CopyRow({required this.label, required this.value});
  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    void copy() {
      Clipboard.setData(ClipboardData(text: value));
      showCommonSnackBar(context, '已複製$label');
    }

    return Semantics(
      button: true,
      label: '複製$label $value',
      onTap: copy,
      child: ExcludeSemantics(
        child: Material(
          color: Colors.transparent,
          child: InkWell(
            borderRadius: BorderRadius.circular(HkRadius.sm),
            onTap: copy,
            child: ConstrainedBox(
              constraints: const BoxConstraints(minHeight: HkControl.minTap),
              child: Row(
                children: [
                  SizedBox(
                    width: 96.w,
                    child: Text(label, style: AppTypography.metaMuted),
                  ),
                  Expanded(
                    child: Text(
                      value,
                      style: AppTypography.meta,
                      textAlign: TextAlign.right,
                    ),
                  ),
                  const SizedBox(width: HkSpace.xs),
                  HkIcon(
                    HkIconName.copy,
                    size: 16.sp,
                    color: AppIconColors.muted,
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}
