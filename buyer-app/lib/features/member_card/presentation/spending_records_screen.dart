import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import '../../../core/theme/app_colors.dart';
import '../../../shared/widgets/common_top_bar.dart';
import '../../../shared/widgets/hk_button_label.dart';
import '../../../shared/widgets/common_snackbar.dart';
import '../../../shared/widgets/truck_loading_overlay.dart';
import 'member_card_controller.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

class SpendingRecordsScreen extends ConsumerWidget {
  const SpendingRecordsScreen({
    super.key,
    required this.sellerId,
    required this.sellerName,
  });

  final String sellerId;
  final String sellerName;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final recordsAsync = ref.watch(spendingRecordsControllerProvider(sellerId));

    return Scaffold(
      backgroundColor: AppColors.bgPrimary,
      appBar: buildCommonTitleAppBar(
        context,
        title: sellerName.isNotEmpty ? sellerName : '消費紀錄',
        showBack: true,
      ),
      body: recordsAsync.when(
        loading: () => const TruckLoadingOverlay(),
        error: (err, _) => Center(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(
                AppIcons.errorOutline,
                color: AppIconColors.danger,
                size: 48.sp,
              ),
              SizedBox(height: 12.h),
              const Text(
                '載入失敗',
                style: TextStyle(color: AppColors.textPrimary),
              ),
              SizedBox(height: 12.h),
              TextButton(
                onPressed: () =>
                    ref.invalidate(spendingRecordsControllerProvider(sellerId)),
                child: const HkButtonLabel('重試'),
              ),
            ],
          ),
        ),
        data: (data) => SingleChildScrollView(
          padding: EdgeInsets.all(16.w),
          child: Column(
            children: [
              _SummaryCard(
                totalSpending: data.totalSpending,
                totalRefund: data.totalRefund,
              ),
              SizedBox(height: 12.h),
              Text(
                '• 退回總額可以當成下次購物扣款使用',
                style: TextStyle(fontSize: 14, color: AppColors.textPrimary),
              ),
              SizedBox(height: 16.h),
              _TableHeader(),
              if (data.records.isEmpty)
                Padding(
                  padding: EdgeInsets.all(24.w),
                  child: Center(
                    child: Text(
                      '尚無消費紀錄',
                      style: TextStyle(
                        color: AppColors.textPrimary,
                        fontSize: 14,
                      ),
                    ),
                  ),
                )
              else
                ...data.records.map((record) => _TableRow(record: record)),
              if (data.hasMore)
                TextButton(
                  onPressed: () => ref
                      .read(
                        spendingRecordsControllerProvider(sellerId).notifier,
                      )
                      .loadMore()
                      .catchError((Object _) {
                        if (context.mounted) {
                          showCommonSnackBar(context, '載入失敗，請稍後再試');
                        }
                      }),
                  child: const HkButtonLabel('載入更多'),
                ),
            ],
          ),
        ),
      ),
    );
  }
}

class _SummaryCard extends StatelessWidget {
  const _SummaryCard({required this.totalSpending, required this.totalRefund});

  final int totalSpending;
  final int totalRefund;

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: EdgeInsets.symmetric(vertical: 20.h, horizontal: 16.w),
      decoration: BoxDecoration(
        color: AppColors.bgCard,
        borderRadius: BorderRadius.circular(16.r),
        border: Border.all(color: AppColors.cardBorder, width: 2),
        boxShadow: const [
          BoxShadow(
            color: AppColors.cardShadow,
            blurRadius: 8,
            offset: Offset(0, 4),
          ),
        ],
      ),
      child: Row(
        children: [
          Expanded(
            child: Column(
              children: [
                Text(
                  '累積消費總額',
                  style: TextStyle(
                    fontSize: 14,
                    color: AppColors.textPrimary,
                    fontWeight: FontWeight.w500,
                  ),
                ),
                SizedBox(height: 8.h),
                Text(
                  'NT\$$totalSpending',
                  style: TextStyle(
                    fontSize: 24,
                    color: AppColors.primary,
                    fontWeight: FontWeight.w700,
                  ),
                ),
              ],
            ),
          ),
          Container(width: 1, height: 50.h, color: AppColors.gray300),
          Expanded(
            child: Column(
              children: [
                Text(
                  '退回總額',
                  style: TextStyle(
                    fontSize: 14,
                    color: AppColors.textPrimary,
                    fontWeight: FontWeight.w500,
                  ),
                ),
                SizedBox(height: 8.h),
                Text(
                  'NT\$$totalRefund',
                  style: TextStyle(
                    fontSize: 24,
                    color: AppColors.primary,
                    fontWeight: FontWeight.w700,
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

/// 四欄的寬度配比，表頭與資料列**必須共用**——分開寫過一次就會錯開。
///
/// 原本是 1:2:1:1，日期只拿到全寬的 1/5。`enableScaleWH: () => false`（main.dart）
/// 讓 `.w` 就是邏輯像素，所以在 390pt 機身上那一份只有 ~64px，而「2026/07/19」在
/// fontSize 14 下約 70px —— 於是被折成「2026/07/1」+「9」。**欄位本身太窄**，
/// 不是文字設定的問題，所以修法是重新配寬度而不是把字縮小
/// （design_policy_test「text is never silently scaled down to fit」明令禁止
/// 用 FittedBox.scaleDown 硬塞，要求走 responsive layout + ellipsis）。
const _kFlexDate = 4;
const _kFlexOrderSn = 4;
const _kFlexAmount = 3;
const _kFlexRefund = 3;

/// 表格左右內距。原為 12，收成 8 是為了把省下來的 8px 還給日期欄
/// （小機身 320pt 上這 8px 正好是日期能不能單行的差別）。仍在 4px ladder 上。
final _kRowPadding = EdgeInsets.symmetric(vertical: 12.h, horizontal: 8.w);

/// 表格內文一律單行 + ellipsis，不折行、不縮字。
TextStyle get _cellTextStyle =>
    TextStyle(color: AppColors.textPrimary, fontSize: 14);

Widget _oneLine(String text, TextStyle style) => Text(
  text,
  textAlign: TextAlign.center,
  style: style,
  maxLines: 1,
  softWrap: false,
  overflow: TextOverflow.ellipsis,
);

class _TableHeader extends StatelessWidget {
  @override
  Widget build(BuildContext context) {
    final textStyle = TextStyle(
      color: AppColors.textPrimary,
      fontSize: 14,
      fontWeight: FontWeight.w700,
    );
    const dividerColor = AppColors.textPrimary;

    Widget cell(int flex, String label) =>
        Expanded(flex: flex, child: _oneLine(label, textStyle));

    return Container(
      padding: _kRowPadding,
      decoration: BoxDecoration(
        color: const Color(0x80B8441F),
        borderRadius: BorderRadius.circular(999.r),
      ),
      child: IntrinsicHeight(
        child: Row(
          children: [
            cell(_kFlexDate, '日期'),
            Container(width: 1, color: dividerColor),
            cell(_kFlexOrderSn, '訂單編號'),
            Container(width: 1, color: dividerColor),
            cell(_kFlexAmount, '消費金額'),
            Container(width: 1, color: dividerColor),
            cell(_kFlexRefund, '退回'),
          ],
        ),
      ),
    );
  }
}

class _TableRow extends StatelessWidget {
  const _TableRow({required this.record});

  final dynamic record;

  /// 退款額由後端以整數字串給（`orders.refund_total_amount`，元）。
  /// 解析失敗一律當 0 —— 這一格寧可少顯示，也不能顯示一個看不出來源的數字。
  int get _refund => int.tryParse('${record.refund}') ?? 0;

  @override
  Widget build(BuildContext context) {
    final textStyle = _cellTextStyle;

    return Padding(
      padding: _kRowPadding,
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.center,
        children: [
          Expanded(
            flex: _kFlexDate,
            child: _oneLine(record.orderDate, textStyle),
          ),
          // 優惠券標示改掛在訂單編號底下：它描述的是「這筆訂單怎麼付的」，
          // 與退款金額是兩件事。之前兩者共用第 4 格，導致「退回」欄從頭到尾
          // 沒顯示過 record.refund（表頭寫「退回」、格子畫的是優惠券 badge），
          // 而且 badge 在那個窄欄裡還會被折成「優惠券折」+「抵」。
          Expanded(
            flex: _kFlexOrderSn,
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                _oneLine(record.orderSn, textStyle),
                if (record.hasCoupon) ...[
                  SizedBox(height: 4.h),
                  Container(
                    padding: EdgeInsets.symmetric(
                      horizontal: 4.w,
                      vertical: 4.h,
                    ),
                    decoration: BoxDecoration(
                      color: AppColors.warning,
                      borderRadius: BorderRadius.circular(999.r),
                    ),
                    child: Text(
                      '優惠券折抵',
                      style: TextStyle(
                        color: AppColors.onWarning,
                        fontSize: 12,
                        fontWeight: FontWeight.w700,
                      ),
                      textAlign: TextAlign.center,
                      maxLines: 1,
                      softWrap: false,
                      overflow: TextOverflow.ellipsis,
                    ),
                  ),
                ],
              ],
            ),
          ),
          Expanded(
            flex: _kFlexAmount,
            child: _oneLine('NT\$${record.amount}', textStyle),
          ),
          // 「退回」欄現在真的畫退款金額。0 維持原本的灰色「-」，
          // 非 0 用 danger 色 + 粗體，讓它在整片綠字裡一眼看得出來。
          Expanded(
            flex: _kFlexRefund,
            child: _refund > 0
                ? _oneLine(
                    'NT\$$_refund',
                    TextStyle(
                      color: AppColors.danger,
                      fontSize: 14,
                      fontWeight: FontWeight.w700,
                    ),
                  )
                : _oneLine(
                    '-',
                    TextStyle(color: AppColors.gray300, fontSize: 14),
                  ),
          ),
        ],
      ),
    );
  }
}
