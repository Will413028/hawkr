import 'package:hawkr_api/money.dart';
import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:flutter/material.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../order/data/order_repository.dart';
import '../../../shared/widgets/common_top_bar.dart';
import '../../../shared/utils/amount_utils.dart';
import '../../../shared/utils/hk_log.dart';
import '../../../shared/widgets/truck_loading_overlay.dart';
import '../../../l10n/l10n.dart';
import '../../../shared/time/taipei_display.dart';

import 'package:flutter_screenutil/flutter_screenutil.dart';

/// 消費紀錄頁面
class SpendingHistoryScreen extends ConsumerStatefulWidget {
  const SpendingHistoryScreen({
    super.key,
    required this.merchantId,
    required this.merchantName,
  });

  final String merchantId;
  final String merchantName;

  @override
  ConsumerState<SpendingHistoryScreen> createState() =>
      _SpendingHistoryScreenState();
}

class _SpendingHistoryScreenState extends ConsumerState<SpendingHistoryScreen> {
  bool _loading = true;
  String? _error;
  List<Map<String, dynamic>> _records = [];
  double _totalSpent = 0;
  double _totalRefund = 0;

  @override
  void initState() {
    super.initState();
    HKLog.logD('=== SpendingHistoryScreen initState ===');
    HKLog.logD('merchantId: ${widget.merchantId}');
    HKLog.logD('merchantName: ${widget.merchantName}');
    _loadSpendingHistory();
  }

  Future<void> _loadSpendingHistory() async {
    setState(() {
      _loading = true;
      _error = null;
    });

    try {
      HKLog.logD('=== _loadSpendingHistory called ===');
      HKLog.logD(
        'Calling API orderListWithItem, then filter by merchantId: ${widget.merchantId}',
      );
      final allOrders = await ref
          .read(orderRepositoryProvider)
          .getOrdersRaw(page: 1);
      HKLog.logD('API returned ${allOrders.length} total orders');

      final merchantOrders = allOrders.where((order) {
        final m = Map<String, dynamic>.from(order);
        final ordMerchantId = _getStr(m, [
          'matchedMerchantId',
          'merchantId',
          'storeId',
          'shopId',
          'sellerId',
        ]);
        final status = (m['reservationStatus'] ?? '').toString();
        final isCurrentMerchant = ordMerchantId == widget.merchantId;
        final isCompleted = _isCompletedStatus(status);
        HKLog.logD(
          'Order merchantId: $ordMerchantId, status: $status, isMatch: $isCurrentMerchant, isCompleted: $isCompleted',
        );
        return isCurrentMerchant;
      }).toList();
      HKLog.logD(
        'Filtered ${merchantOrders.length} orders for merchant ${widget.merchantId}',
      );

      double totalSpent = 0;
      double totalRefund = 0;
      final List<Map<String, dynamic>> records = [];

      for (final order in merchantOrders) {
        final m = Map<String, dynamic>.from(order);

        final amount = _parseDouble(
          _getNum(m, [
            'totalAmount',
            'total_amount',
            'payAmount',
            'amount',
            'orderAmount',
          ]),
        );
        final refund = _parseDouble(
          _getNum(m, ['refundAmount', 'refund_amount', 'refund']),
        );
        final orderId = _getStr(m, [
          'orderSn',
          'orderNo',
          'sn',
          'order_id',
          'orderId',
          'id',
        ]);
        final createdAt = _getStr(m, [
          'createTime',
          'paymentTime',
          'orderTime',
          'gmtCreate',
          'created_at',
          'createdAt',
        ]);

        totalSpent += amount;
        totalRefund += refund;

        final date = _tryFormatDate(createdAt);
        records.add({
          'date': date ?? createdAt,
          'orderId': _truncateOrderId(orderId),
          'amount': amount,
          'refund': refund,
        });
      }

      records.sort((a, b) => b['date'].compareTo(a['date']));

      if (!mounted) return;
      setState(() {
        _records = records;
        _totalSpent = totalSpent;
        _totalRefund = totalRefund;
        _loading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _loading = false;
        _error = '載入消費紀錄失敗';
      });
    }
  }

  double _parseDouble(dynamic value) {
    if (value == null) return 0;
    if (value is num) return value.toDouble();
    return double.tryParse(value.toString()) ?? 0;
  }

  String _getStr(Map<String, dynamic>? m, List<String> keys) {
    if (m == null) return '';
    for (final k in keys) {
      final v = m[k];
      if (v == null) continue;
      final s = v.toString().trim();
      if (s.isNotEmpty) return s;
    }
    return '';
  }

  num _getNum(Map<String, dynamic>? m, List<String> keys) {
    if (m == null) return 0;
    for (final k in keys) {
      final v = m[k];
      if (v is num) return v;
      if (v is String) {
        final parsed = num.tryParse(v.replaceAll(RegExp(r'[^0-9.-]'), ''));
        if (parsed != null) return parsed;
      }
    }
    return 0;
  }

  bool _isCompletedStatus(String statusText) {
    final s = statusText.trim();
    if (s.isEmpty) return false;
    final lower = s.toLowerCase();
    final compact = lower
        .replaceAll(' ', '')
        .replaceAll('_', '')
        .replaceAll('-', '');
    return s == '已送達' ||
        s == '已完成' ||
        s == '完成' ||
        s == '送達' ||
        s == '取消' ||
        s.contains('送達') ||
        s.contains('完成') ||
        s.contains('取消') ||
        compact.contains('delivered') ||
        compact.contains('completed') ||
        compact.contains('finished') ||
        compact.contains('canceled') ||
        compact.contains('cancelled');
  }

  /// 解析失敗回 null（呼叫端不為原樣字串附臺北時間標籤）。
  String? _tryFormatDate(String raw) {
    return TaipeiDisplay.plainIso(raw, TaipeiFormat.date);
  }

  String _truncateOrderId(String orderId) {
    if (orderId.length <= 10) return orderId;
    return '${orderId.substring(0, 8)}...';
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: AppColors.bgPrimary,
      appBar: buildCommonTitleAppBar(context, title: '消費紀錄'),
      body: _loading
          ? const TruckLoadingOverlay()
          : _error != null
          ? Center(
              child: Text(
                _error!,
                style: const TextStyle(color: AppColors.danger),
              ),
            )
          : SingleChildScrollView(
              padding: EdgeInsets.all(16.w),
              child: Column(
                children: [
                  _buildSummaryCard(),
                  SizedBox(height: 12.h),
                  Text(
                    '• 退回總額可以當成下次購物扣款使用',
                    style: TextStyle(
                      fontSize: 14,
                      color: AppColors.textPrimary,
                    ),
                  ),
                  SizedBox(height: 16.h),
                  SpendingHistoryTable(records: _records),
                ],
              ),
            ),
    );
  }

  Widget _buildSummaryCard() {
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
                  'NT\$${_totalSpent.toStringAsFixed(0)}',
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
                  'NT\$${_totalRefund.toStringAsFixed(0)}',
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

/// 消費紀錄表格；裝置偏移不是 +8 時在表格上方附一行台北時間說明。
class SpendingHistoryTable extends StatelessWidget {
  const SpendingHistoryTable({super.key, required this.records});

  final List<Map<String, dynamic>> records;

  @override
  Widget build(BuildContext context) {
    final showNote = context.taipeiDisplay.labelIfNeeded != null;
    return Column(
      children: [
        if (showNote)
          Padding(
            padding: EdgeInsets.only(bottom: 4.h),
            child: Text(
              context.l10n.taipeiTimeTableNote,
              style: TextStyle(fontSize: 12, color: AppColors.textPrimary),
            ),
          ),
        _buildTableHeader(context),
        ..._buildTableRows(context),
      ],
    );
  }

  Widget _buildTableHeader(BuildContext context) {
    final textStyle = TextStyle(
      color: AppColors.textPrimary,
      fontSize: 14,
      fontWeight: FontWeight.w700,
    );
    const dividerColor = AppColors.textPrimary;

    return Container(
      padding: EdgeInsets.symmetric(vertical: 12.h, horizontal: 12.w),
      decoration: BoxDecoration(
        color: const Color(0x80B8441F),
        borderRadius: BorderRadius.circular(999.r),
      ),
      child: IntrinsicHeight(
        child: Row(
          children: [
            Expanded(
              flex: 1,
              child: Text('日期', textAlign: TextAlign.center, style: textStyle),
            ),
            Container(width: 1, color: dividerColor),
            Expanded(
              flex: 1,
              child: Text(
                '訂單編號',
                textAlign: TextAlign.center,
                style: textStyle,
              ),
            ),
            Container(width: 1, color: dividerColor),
            Expanded(
              flex: 1,
              child: Text(
                '消費金額',
                textAlign: TextAlign.center,
                style: textStyle,
              ),
            ),
            Container(width: 1, color: dividerColor),
            Expanded(
              flex: 1,
              child: Text('退回', textAlign: TextAlign.center, style: textStyle),
            ),
          ],
        ),
      ),
    );
  }

  List<Widget> _buildTableRows(BuildContext context) {
    if (records.isEmpty) {
      return [
        Padding(
          padding: EdgeInsets.all(24.w),
          child: Center(
            child: Text(
              '尚無消費紀錄',
              style: TextStyle(color: AppColors.textPrimary, fontSize: 14),
            ),
          ),
        ),
      ];
    }

    final textStyle = TextStyle(color: AppColors.textPrimary, fontSize: 14);

    return records.map((record) {
      return Padding(
        padding: EdgeInsets.symmetric(vertical: 12.h, horizontal: 12.w),
        child: Row(
          children: [
            Expanded(
              flex: 1,
              child: Text(
                record['date'] ?? '',
                textAlign: TextAlign.center,
                style: textStyle,
              ),
            ),
            Expanded(
              flex: 1,
              child: Text(
                record['orderId'] ?? '',
                textAlign: TextAlign.center,
                style: textStyle,
              ),
            ),
            Expanded(
              flex: 1,
              child: Text(
                formatMoney(AmountUtils.parseToInt(record['amount'])),
                textAlign: TextAlign.center,
                style: textStyle,
              ),
            ),
            Expanded(
              flex: 1,
              child: Text(
                'NT\$${(record['refund'] as double).toStringAsFixed(0)}',
                textAlign: TextAlign.center,
                style: textStyle,
              ),
            ),
          ],
        ),
      );
    }).toList();
  }
}
