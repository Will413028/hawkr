import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:flutter/material.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import '../../order/data/order_repository.dart';
import '../../../shared/widgets/common_top_bar.dart';
import '../../../shared/utils/amount_utils.dart';
import '../../../shared/utils/hk_log.dart';
import '../../../shared/widgets/truck_loading_overlay.dart';
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
      // 使用和訂單頁相同的 API 撈取訂單列表
      HKLog.logD('=== _loadSpendingHistory called ===');
      HKLog.logD(
        'Calling API orderListWithItem, then filter by merchantId: ${widget.merchantId}',
      );
      final allOrders = await ref
          .read(orderRepositoryProvider)
          .getOrdersRaw(page: 1);
      HKLog.logD('API returned ${allOrders.length} total orders');

      // 篩選當前商家的訂單
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

      // 計算累積消費總額和退回總額
      double totalSpent = 0;
      double totalRefund = 0;
      final List<Map<String, dynamic>> records = [];

      for (final order in merchantOrders) {
        final m = Map<String, dynamic>.from(order);

        // 使用和訂單頁相同的欄位名稱
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

        records.add({
          'date': _formatDate(createdAt),
          'orderId': _truncateOrderId(orderId),
          'amount': amount,
          'refund': refund,
        });
      }

      // 按日期排序（最新的在前）
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

  // 從 Map 中取得字串值（支援多個 key）
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

  // 從 Map 中取得數值（支援多個 key）
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

  // 判斷訂單是否為「完成訂單」狀態
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

  String _formatDate(String raw) {
    if (raw.isEmpty) return '';
    try {
      DateTime? dt;
      final digits = RegExp(r'^\d+$');
      if (digits.hasMatch(raw)) {
        if (raw.length >= 13) {
          final ms = int.tryParse(raw.substring(0, 13));
          if (ms != null) {
            dt = DateTime.fromMillisecondsSinceEpoch(ms).toLocal();
          }
        } else if (raw.length >= 10) {
          final sec = int.tryParse(raw.substring(0, 10));
          if (sec != null) {
            dt = DateTime.fromMillisecondsSinceEpoch(sec * 1000).toLocal();
          }
        }
      }
      dt ??= DateTime.tryParse(
        raw.replaceFirst(' ', 'T').replaceAll('/', '-'),
      )?.toLocal();
      if (dt == null) return raw;
      String two(int n) => n.toString().padLeft(2, '0');
      return '${dt.year}/${two(dt.month)}/${two(dt.day)}';
    } catch (_) {
      return raw;
    }
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
                  // 頂部統計卡片
                  _buildSummaryCard(),
                  SizedBox(height: 12.h),
                  // 提示文字
                  Text(
                    '• 退回總額可以當成下次購物扣款使用',
                    style: TextStyle(
                      fontSize: 14,
                      color: AppColors.textPrimary,
                    ),
                  ),
                  SizedBox(height: 16.h),
                  // 表格標題
                  _buildTableHeader(),
                  // 表格內容
                  ..._buildTableRows(),
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
          // 累積消費總額
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
          // 分隔線
          Container(width: 1, height: 50.h, color: AppColors.gray300),
          // 退回總額
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

  Widget _buildTableHeader() {
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

  List<Widget> _buildTableRows() {
    if (_records.isEmpty) {
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

    return _records.map((record) {
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
                // B-109：後端 amount 一律字串，使用 AmountUtils 做容錯解析
                AmountUtils.formatNTD(record['amount']),
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
