import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import '../../../shared/widgets/hk_glass_app_bar.dart';
import '../../../shared/widgets/hk_icon.dart';
import '../data/order_model.dart';

class OrderPaymentDetailsScreen extends StatelessWidget {
  const OrderPaymentDetailsScreen({super.key, this.order});

  final OrderSummary? order;

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: AppColors.bgPrimary,
      appBar: const HkGlassAppBar(title: Text('付款詳情')),
      body: ListView(
        padding: EdgeInsets.symmetric(horizontal: 16.w, vertical: 12.h),
        children: [
          _pageTitle(),
          SizedBox(height: 16.h),
          _orderSummaryCard(),
          SizedBox(height: 16.h),
          _paymentMethodCard(),
          SizedBox(height: 16.h),
          _promotionsCard(),
          SizedBox(height: 80.h),
        ],
      ),
    );
  }

  Widget _pageTitle() {
    return Container(
      padding: EdgeInsets.all(16.w),
      decoration: BoxDecoration(
        color: AppColors.bgCard,
        borderRadius: BorderRadius.circular(12.r),
        border: Border.all(color: AppColors.gray300),
      ),
      child: Text(
        '付款明細',
        style: TextStyle(fontWeight: FontWeight.w700, fontSize: 18.sp),
      ),
    );
  }

  Widget _orderSummaryCard() {
    final priceRaw = order?.price ?? '';
    final total = priceRaw.isNotEmpty
        ? (priceRaw.startsWith('NT') ? priceRaw : 'NT\$$priceRaw')
        : 'NT\$0';
    return Container(
      padding: EdgeInsets.all(16.w),
      decoration: BoxDecoration(
        color: AppColors.bgCard,
        borderRadius: BorderRadius.circular(12.r),
        border: Border.all(color: AppColors.gray300),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            '訂單金額',
            style: TextStyle(fontWeight: FontWeight.w700, fontSize: 16.sp),
          ),
          SizedBox(height: 12.h),
          // 🔴 這四列原本是硬編碼的 'NT$0'，與真實訂單無關。OrderSummary 上
          // shippingFee／discountAmount／totalAmount 三個欄位一直都在，只是沒接。
          _summaryRow('商品小計', total),
          _summaryRow('運費', _money(order?.shippingFee)),
          _summaryRow(
            '折扣',
            '-${_money(order?.discountAmount)}',
            color: AppColors.danger,
          ),
          const Divider(height: 24),
          // 合計取 totalAmount 而不是沿用商品小計：有運費或折扣時兩者不相等。
          _summaryRow('合計', _money(order?.totalAmount), isTotal: true),
        ],
      ),
    );
  }

  /// _money 把 OrderSummary 的數值欄位格式化成 'NT$X'。
  ///
  /// 🔴 **這張卡刻意沒有「稅額」那一列**（原本有，值是硬編碼的 'NT$0'）：
  /// 台灣定價一律含稅（營業稅法 §32 II），`orders` 與 `order_items` 的金額欄位
  /// **沒有任何未稅欄位**，稅是在開發票時才由 invoice-service 從含稅總額拆出來的
  /// （B2B 才拆，B2C 二聯式依法不分列）。在訂單層顯示「稅額 NT$0」是概念錯誤
  /// ——它會讓賣家以為這筆訂單免稅，或以為稅要另外加。
  /// 要看稅額拆分請到發票詳情，那裡的數字來自 invoice_documents。
  String _money(num? value) {
    final v = value ?? 0;
    return 'NT\$${v.toStringAsFixed(0)}';
  }

  Widget _summaryRow(
    String label,
    String amount, {
    Color? color,
    bool isTotal = false,
  }) {
    return Padding(
      padding: EdgeInsets.symmetric(vertical: 4.h),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          Text(
            label,
            style: TextStyle(
              fontWeight: isTotal ? FontWeight.w700 : FontWeight.normal,
              fontSize: isTotal ? 16.sp : 14.sp,
            ),
          ),
          Text(
            amount,
            style: TextStyle(
              fontWeight: isTotal ? FontWeight.w700 : FontWeight.normal,
              fontSize: isTotal ? 16.sp : 14.sp,
              color: color,
            ),
          ),
        ],
      ),
    );
  }

  Widget _paymentMethodCard() {
    final priceRaw = order?.price ?? '';
    final total = priceRaw.isNotEmpty
        ? (priceRaw.startsWith('NT') ? priceRaw : 'NT\$$priceRaw')
        : 'NT\$0';
    return Container(
      padding: EdgeInsets.all(16.w),
      decoration: BoxDecoration(
        color: AppColors.bgCard,
        borderRadius: BorderRadius.circular(12.r),
        border: Border.all(color: AppColors.gray300),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            '付款方式',
            style: TextStyle(fontWeight: FontWeight.w700, fontSize: 16.sp),
          ),
          SizedBox(height: 12.h),
          Row(
            children: [
              Container(
                width: 40,
                height: 40,
                decoration: BoxDecoration(
                  color: AppColors.primarySoft,
                  borderRadius: BorderRadius.circular(12),
                ),
                child: const Center(
                  child: HkIcon(
                    HkIconName.payment,
                    color: AppIconColors.active,
                  ),
                ),
              ),
              SizedBox(width: 12.w),
              const Expanded(
                child: Text(
                  '付款資訊以金流紀錄為準',
                  style: TextStyle(color: AppColors.textPrimary),
                ),
              ),
              Text(
                total,
                style: const TextStyle(
                  color: AppColors.textEmphasis,
                  fontWeight: FontWeight.w700,
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }

  Widget _promotionsCard() {
    return Container(
      padding: EdgeInsets.all(16.w),
      decoration: BoxDecoration(
        color: AppColors.bgCard,
        borderRadius: BorderRadius.circular(12.r),
        border: Border.all(color: AppColors.gray300),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            '優惠內容',
            style: TextStyle(fontWeight: FontWeight.w700, fontSize: 16.sp),
          ),
          SizedBox(height: 12.h),
          Container(
            padding: EdgeInsets.all(12.w),
            decoration: BoxDecoration(
              color: AppColors.primarySoft,
              borderRadius: BorderRadius.circular(12.r),
              border: Border.all(color: AppColors.borderSubtle),
            ),
            child: const Row(
              children: [
                HkIcon(HkIconName.tag, color: AppIconColors.active, size: 20),
                SizedBox(width: 10),
                Expanded(
                  child: Text(
                    '優惠金額已反映在訂單合計中；實際項目以訂單交易資料為準。',
                    style: TextStyle(
                      color: AppColors.textPrimary,
                      height: 1.45,
                    ),
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
