import '../../../shared/widgets/hk_temperature_tag.dart';
import 'package:hawkr_seller/shared/widgets/hk_button_label.dart';
import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:hawkr_seller/shared/widgets/hk_glass_app_bar.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import '../data/payuni_logistics_service.dart';
import '../../../shared/utils/floating_toast.dart';
import '../../../shared/widgets/hk_empty_state.dart';
import '../../../shared/widgets/fruit_refresh_indicator.dart';
import '../../../shared/widgets/truck_loading_overlay.dart';
import '../../../shared/widgets/hk_icon.dart';

/// 物流追蹤畫面 - 可視化 PAYUNi 物流狀態
class LogisticsTrackingScreen extends ConsumerStatefulWidget {
  /// 🔴 `lgsType` 是必填：原本預設 `'HOME'`，於是沒帶 lgsType 的呼叫端會把
  /// 超商單當宅配去查 PayUni（S7）。呼叫端拿不到 lgsType 時應帶空字串，
  /// 畫面會走「未知物流類型」的 fail-safe 呈現，而不是被冒名成宅配。
  const LogisticsTrackingScreen({
    super.key,
    this.shipTradeNo,
    required this.lgsType,
    this.tradeType,
    this.returnOdno,
    this.orderSn,
  });

  final String? shipTradeNo;
  final String lgsType; // B2C, C2C, HOME, C2B
  final int? tradeType; // 1=正物流, 2=逆物流
  final String? returnOdno; // C2B 退貨便編號
  final String? orderSn; // 訂單編號

  static Route<T> route<T extends Object?>({
    RouteSettings? settings,
    String? shipTradeNo,
    required String lgsType,
    int? tradeType,
    String? returnOdno,
    String? orderSn,
  }) {
    return MaterialPageRoute<T>(
      builder: (_) => LogisticsTrackingScreen(
        shipTradeNo: shipTradeNo,
        lgsType: lgsType,
        tradeType: tradeType,
        returnOdno: returnOdno,
        orderSn: orderSn,
      ),
      settings: settings,
    );
  }

  @override
  ConsumerState<LogisticsTrackingScreen> createState() =>
      _LogisticsTrackingScreenState();
}

class _LogisticsTrackingScreenState
    extends ConsumerState<LogisticsTrackingScreen> {
  bool _isLoading = true;
  String? _errorMessage;
  LogisticsInfo? _logisticsInfo;

  @override
  void initState() {
    super.initState();
    _fetchLogistics();
  }

  Future<void> _fetchLogistics() async {
    setState(() {
      _isLoading = true;
      _errorMessage = null;
    });

    final result = await ref
        .read(payuniLogisticsServiceProvider)
        .queryLogistics(
          shipTradeNo: widget.shipTradeNo,
          lgsType: widget.lgsType,
          tradeType: widget.tradeType,
          returnOdno: widget.returnOdno,
        );

    if (mounted) {
      setState(() {
        _isLoading = false;
        if (result.success && result.data != null) {
          _logisticsInfo = result.data;
        } else {
          _errorMessage = result.errorMessage ?? '查詢失敗';
        }
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: AppColors.bgPrimary,
      appBar: HkGlassAppBar(
        title: const Text('物流追蹤'),
        onBackPressed: () => context.pop(),
        actions: [
          IconButton(
            onPressed: _fetchLogistics,
            icon: const HkIcon(
              HkIconName.refresh,
              color: AppIconColors.defaultColor,
            ),
            tooltip: '重新整理',
          ),
        ],
      ),
      body: _buildBody(),
    );
  }

  Widget _buildBody() {
    // 僅首次載入（無資料時）顯示 TruckLoadingOverlay；
    // 下拉刷新時已有資料，跳過遮罩，由 FruitRefreshIndicator 提供視覺回饋
    if (_isLoading && _logisticsInfo == null) {
      return TruckLoadingOverlay(message: '查詢物流狀態中...');
    }

    if (_errorMessage != null) {
      return Center(
        child: Padding(
          padding: EdgeInsets.all(24.w),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              HkIcon(
                HkIconName.truck,
                size: 64.sp,
                color: AppIconColors.disabled,
              ),
              SizedBox(height: 16.h),
              Text(
                _errorMessage!,
                textAlign: TextAlign.center,
                style: TextStyle(
                  fontSize: 16.sp,
                  color: AppColors.textSecondary,
                ),
              ),
              SizedBox(height: 24.h),
              ElevatedButton.icon(
                onPressed: _fetchLogistics,
                icon: const HkIcon(HkIconName.refresh),
                label: const HkButtonLabel('重新查詢'),
                style: ElevatedButton.styleFrom(
                  backgroundColor: AppColors.primary,
                  foregroundColor: AppColors.bgCard,
                  padding: EdgeInsets.symmetric(
                    horizontal: 24.w,
                    vertical: 12.h,
                  ),
                ),
              ),
            ],
          ),
        ),
      );
    }

    if (_logisticsInfo == null) {
      return const HkEmptyState(
        key: ValueKey('seller-tracking-empty-state'),
        iconKey: ValueKey('seller-tracking-empty-state-icon'),
        icon: HkIconName.truck,
        message: '暫無物流資訊',
        description: '建立物流單後可在這裡查看配送進度。',
        topPadding: 64,
      );
    }

    return FruitRefreshIndicator(
      onRefresh: _fetchLogistics,
      child: ListView(
        padding: EdgeInsets.all(16.w),
        children: [
          // 1. 物流進度卡片
          _buildProgressCard(),
          SizedBox(height: 16.h),
          // 2. 物流狀態卡片
          _buildStatusCard(),
          SizedBox(height: 16.h),
          // 3. 配送資訊卡片
          _buildDeliveryInfoCard(),
          SizedBox(height: 16.h),
          // 4. 訂單資訊卡片
          _buildOrderInfoCard(),
          SizedBox(height: 24.h),
        ],
      ),
    );
  }

  /// 1. 物流進度卡片 - 時間軸進度
  Widget _buildProgressCard() {
    final info = _logisticsInfo!;
    // progress 為 null = 本 App 沒有這條流程的進度表（B2C／C2B／未知 lgsType）。
    final progress =
        info.progressPercent ??
        LogisticsStatusCode.getProgressPercent(info.shipStatus, info.lgsType);
    final isError = LogisticsStatusCode.isErrorStatus(info.shipStatus);
    final isFinal = LogisticsStatusCode.isFinalStatus(info.shipStatus);

    // 🔴 步驟表依 lgsType **明確分派**；原本是 `HOME ? 宅配 : 超商`，
    // 於是 B2C／C2B／空字串都被畫成超商 C2C 的五步。
    final steps = switch (info.lgsType) {
      PayuniLogisticsService.lgsTypeHOME => [
        _ProgressStep('已下單', HkIconName.receipt, (progress ?? -1) >= 0),
        _ProgressStep('已收件', HkIconName.inventory, (progress ?? -1) >= 30),
        _ProgressStep('配送中', HkIconName.truck, (progress ?? -1) >= 60),
        _ProgressStep(
          '已送達',
          HkIconName.check,
          (progress ?? -1) >= 100 && !isError,
        ),
      ],
      PayuniLogisticsService.lgsTypeC2C => [
        _ProgressStep('已取號', HkIconName.qrCode, (progress ?? -1) >= 10),
        _ProgressStep('已出貨', HkIconName.inventory, (progress ?? -1) >= 35),
        _ProgressStep('配送中', HkIconName.truck, (progress ?? -1) >= 50),
        _ProgressStep('已到店', HkIconName.store, (progress ?? -1) >= 80),
        _ProgressStep(
          '已取貨',
          HkIconName.check,
          (progress ?? -1) >= 100 && !isError,
        ),
      ],
      // 未知 / 本版未支援的物流類型：provider-neutral 兩步，不冒充任一條流程。
      _ => [
        _ProgressStep('已建立物流單', HkIconName.receipt, true),
        _ProgressStep('進度以物流商公告為準', HkIconName.truck, isFinal),
      ],
    };

    return Container(
      padding: EdgeInsets.all(16.w),
      decoration: BoxDecoration(
        color: AppColors.bgCard,
        borderRadius: BorderRadius.circular(12.r),
        boxShadow: [
          BoxShadow(
            color: AppColors.cardShadow,
            blurRadius: 4,
            offset: Offset(0, 2.h),
          ),
        ],
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // 標題
          Row(
            children: [
              HkIcon(
                switch (info.lgsType) {
                  PayuniLogisticsService.lgsTypeHOME => HkIconName.home,
                  PayuniLogisticsService.lgsTypeC2C => HkIconName.store,
                  _ => HkIconName.truck,
                },
                color: AppIconColors.secondary,
                size: 24.sp,
              ),
              SizedBox(width: 8.w),
              Flexible(
                child: Text(
                  info.shipTypeText,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(
                    fontSize: 16.sp,
                    fontWeight: FontWeight.w700,
                    color: AppColors.textEmphasis,
                  ),
                ),
              ),
              SizedBox(width: 8.w),
              HkTemperatureTag(value: info.goodsTypeText),
              const Spacer(),
              Container(
                padding: EdgeInsets.symmetric(horizontal: 12.w, vertical: 4.h),
                decoration: BoxDecoration(
                  color: isError
                      ? AppColors.dangerStrong
                      : isFinal
                      ? AppColors.primary
                      : AppColors.accentFill,
                  borderRadius: BorderRadius.circular(12.r),
                ),
                child: Text(
                  info.shipStatusLabel,
                  style: TextStyle(
                    fontSize: 12.sp,
                    fontWeight: FontWeight.w500,
                    color: isError || isFinal
                        ? AppColors.bgCard
                        : AppColors.brandInk,
                  ),
                ),
              ),
            ],
          ),
          SizedBox(height: 20.h),
          // 進度條
          ClipRRect(
            borderRadius: BorderRadius.circular(6.r),
            child: LinearProgressIndicator(
              // 進度未知時顯示 0（空進度條），不得借用別條流程的百分比。
              value: (progress ?? 0) / 100,
              backgroundColor: AppColors.dividerLight,
              valueColor: AlwaysStoppedAnimation<Color>(
                isError ? AppColors.dangerStrong : AppColors.primary,
              ),
              minHeight: 8.h,
            ),
          ),
          SizedBox(height: 20.h),
          // 步驟指示
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: steps.map((step) => _buildStepItem(step)).toList(),
          ),
        ],
      ),
    );
  }

  Widget _buildStepItem(_ProgressStep step) {
    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        Container(
          width: 36.w,
          height: 36.h,
          decoration: BoxDecoration(
            color: step.isActive ? AppColors.primary : AppColors.dividerLight,
            shape: BoxShape.circle,
          ),
          child: HkIcon(
            step.icon,
            color: step.isActive ? AppIconColors.onBrand : AppIconColors.muted,
            size: 18.sp,
          ),
        ),
        SizedBox(height: 4.h),
        Text(
          step.label,
          style: TextStyle(
            fontSize: 12.sp,
            color: step.isActive
                ? AppColors.textEmphasis
                : AppColors.textSecondary,
            fontWeight: step.isActive ? FontWeight.w700 : FontWeight.normal,
          ),
        ),
      ],
    );
  }

  /// 2. 物流狀態卡片
  Widget _buildStatusCard() {
    final info = _logisticsInfo!;

    return Container(
      padding: EdgeInsets.all(16.w),
      decoration: BoxDecoration(
        color: AppColors.bgCard,
        borderRadius: BorderRadius.circular(12.r),
        boxShadow: [
          BoxShadow(
            color: AppColors.cardShadow,
            blurRadius: 4,
            offset: Offset(0, 2.h),
          ),
        ],
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // 標題
          Row(
            children: [
              HkIcon(
                HkIconName.info,
                color: AppIconColors.defaultColor,
                size: 20.sp,
              ),
              SizedBox(width: 8.w),
              Text(
                '物流狀態',
                style: TextStyle(
                  fontSize: 14.sp,
                  fontWeight: FontWeight.w700,
                  color: AppColors.textPrimary,
                ),
              ),
            ],
          ),
          SizedBox(height: 12.h),
          const Divider(height: 1, color: AppColors.gray100),
          SizedBox(height: 12.h),
          // 狀態時間軸
          _buildStatusTimeline(info),
        ],
      ),
    );
  }

  Widget _buildStatusTimeline(LogisticsInfo info) {
    // 建立狀態歷史（模擬，實際應從 API 取得歷史記錄）
    final currentStatus = info.shipStatusLabel;
    final statusTime = info.shipStatusTime.isNotEmpty
        ? info.shipStatusTime
        : '-';

    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        // 時間軸指示
        Column(
          children: [
            Container(
              width: 12.w,
              height: 12.h,
              decoration: const BoxDecoration(
                color: AppColors.primary,
                shape: BoxShape.circle,
              ),
            ),
            Container(width: 2, height: 40.h, color: AppColors.dividerLight),
            Container(
              width: 8.w,
              height: 8.h,
              decoration: BoxDecoration(
                color: AppColors.dividerLight,
                shape: BoxShape.circle,
                border: Border.all(color: AppColors.gray300),
              ),
            ),
          ],
        ),
        SizedBox(width: 12.w),
        // 狀態內容
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              // 當前狀態
              Row(
                children: [
                  Expanded(
                    child: Text(
                      currentStatus,
                      style: TextStyle(
                        fontSize: 14.sp,
                        fontWeight: FontWeight.w700,
                        color: AppColors.textEmphasis,
                      ),
                    ),
                  ),
                  Text(
                    statusTime,
                    style: TextStyle(
                      fontSize: 12.sp,
                      color: AppColors.textSecondary,
                    ),
                  ),
                ],
              ),
              SizedBox(height: 4.h),
              Text(
                _getStatusDescription(info),
                style: TextStyle(
                  fontSize: 12.sp,
                  color: AppColors.textSecondary,
                ),
              ),
              SizedBox(height: 20.h),
              // 訂單建立
              Row(
                children: [
                  Expanded(
                    child: Text(
                      '訂單已建立',
                      style: TextStyle(
                        fontSize: 14.sp,
                        color: AppColors.textSecondary,
                      ),
                    ),
                  ),
                  Text(
                    '-',
                    style: TextStyle(
                      fontSize: 12.sp,
                      color: AppColors.textSecondary,
                    ),
                  ),
                ],
              ),
            ],
          ),
        ),
      ],
    );
  }

  String _getStatusDescription(LogisticsInfo info) {
    // 🔴 PayUni 原始 shipStatus 的碼空間是 provider-specific（10–14 超商 /
    // 30–35 黑貓），所以描述必須依 lgsType 明確分派，未知一律走中性文案。
    switch (info.lgsType) {
      case PayuniLogisticsService.lgsTypeHOME:
        return _homeStatusDescription(info.shipStatus);
      case PayuniLogisticsService.lgsTypeC2C:
        return _cvsStatusDescription(info.shipStatus);
      default:
        return info.shipStatusLabel.isNotEmpty
            ? info.shipStatusLabel
            : '請耐心等候配送';
    }
  }

  String _homeStatusDescription(int shipStatus) {
    switch (shipStatus) {
      case 30:
        return '包裹已由物流人員收件';
      case 31:
        return '包裹正在配送途中，請保持電話暢通';
      case 32:
        return '包裹已成功送達';
      case 33:
        return '無法投遞，請聯繫物流公司';
      case 34:
        return '包裹正在退回寄件方';
      case 35:
        return '包裹已退回寄件方';
      default:
        return '請耐心等候配送';
    }
  }

  String _cvsStatusDescription(int shipStatus) {
    switch (shipStatus) {
      case 1:
        return '已產生取貨編號';
      case 2:
        return '託運單已列印';
      case 3:
        return '包裹已交寄物流';
      case 10:
        return '包裹正在配送至門市';
      case 11:
        return '包裹已到達門市，請於期限內取貨';
      case 12:
        return '取貨完成，感謝您的購買';
      case 13:
        return '包裹正在退回';
      case 14:
        return '退貨已完成';
      case 20:
        return '包裹已逾期未取，將退回寄件方';
      case 21:
        return '包裹遺失，請聯繫客服';
      case 81:
        return '門市暫時關閉，包裹已轉送至鄰近門市';
      default:
        return '請耐心等候配送';
    }
  }

  /// 3. 配送資訊卡片
  Widget _buildDeliveryInfoCard() {
    final info = _logisticsInfo!;

    return Container(
      padding: EdgeInsets.all(16.w),
      decoration: BoxDecoration(
        color: AppColors.bgCard,
        borderRadius: BorderRadius.circular(12.r),
        boxShadow: [
          BoxShadow(
            color: AppColors.cardShadow,
            blurRadius: 4,
            offset: Offset(0, 2.h),
          ),
        ],
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // 標題
          Row(
            children: [
              HkIcon(
                HkIconName.truck,
                color: AppIconColors.defaultColor,
                size: 20.sp,
              ),
              SizedBox(width: 8.w),
              Text(
                '配送資訊',
                style: TextStyle(
                  fontSize: 14.sp,
                  fontWeight: FontWeight.w700,
                  color: AppColors.textPrimary,
                ),
              ),
            ],
          ),
          SizedBox(height: 12.h),
          const Divider(height: 1, color: AppColors.gray100),
          SizedBox(height: 12.h),
          // 物流類型
          _buildInfoRow('物流類型', info.lgsTypeText),
          SizedBox(height: 8.h),
          // 取件方式
          _buildInfoRow('取件方式', info.serviceTypeText),
          if (info.shipAmt > 0) ...[
            SizedBox(height: 8.h),
            _buildInfoRow('代收金額', 'NT\$ ${info.shipAmt}'),
          ],
          // 超商資訊
          if (info.storeID != null && info.storeID!.isNotEmpty) ...[
            SizedBox(height: 12.h),
            const Divider(height: 1, color: AppColors.gray100),
            SizedBox(height: 12.h),
            _buildInfoRow('取件門市', '${info.storeName ?? ''} (${info.storeID})'),
          ],
          // 驗證碼 (C2C)
          if (info.validationNo != null && info.validationNo!.isNotEmpty) ...[
            SizedBox(height: 8.h),
            _buildInfoRowWithCopy('驗證碼', info.validationNo!),
          ],
          // 宅配地址
          if (info.consigneeAddress != null &&
              info.consigneeAddress!.isNotEmpty) ...[
            SizedBox(height: 12.h),
            const Divider(height: 1, color: AppColors.gray100),
            SizedBox(height: 12.h),
            _buildInfoRow('配送地址', info.consigneeAddress!),
          ],
          // 收件人資訊
          SizedBox(height: 12.h),
          const Divider(height: 1, color: AppColors.gray100),
          SizedBox(height: 12.h),
          _buildInfoRow('收件人', info.consignee),
          SizedBox(height: 8.h),
          _buildInfoRow('聯絡電話', info.consigneeMobile),
        ],
      ),
    );
  }

  Widget _buildInfoRow(String label, String value) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        SizedBox(
          width: 80.w,
          child: Text(
            label,
            style: TextStyle(fontSize: 14.sp, color: AppColors.textSecondary),
          ),
        ),
        Expanded(
          child: Text(
            value.isNotEmpty ? value : '-',
            style: TextStyle(fontSize: 14.sp, color: AppColors.textEmphasis),
          ),
        ),
      ],
    );
  }

  Widget _buildInfoRowWithCopy(String label, String value) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        SizedBox(
          width: 80.w,
          child: Text(
            label,
            style: TextStyle(fontSize: 14.sp, color: AppColors.textSecondary),
          ),
        ),
        Expanded(
          child: Row(
            children: [
              Text(
                value,
                style: TextStyle(
                  fontSize: 14.sp,
                  fontWeight: FontWeight.w700,
                  color: AppColors.primary,
                  letterSpacing: 1,
                ),
              ),
              SizedBox(width: 8.w),
              GestureDetector(
                onTap: () {
                  Clipboard.setData(ClipboardData(text: value));
                  FloatingToast.show(
                    context,
                    '已複製驗證碼',
                    icon: AppIcons.contentCopy,
                  );
                },
                child: HkIcon(
                  HkIconName.copy,
                  size: 16.sp,
                  color: AppIconColors.secondary,
                ),
              ),
            ],
          ),
        ),
      ],
    );
  }

  /// 4. 訂單資訊卡片
  Widget _buildOrderInfoCard() {
    final info = _logisticsInfo!;

    return Container(
      padding: EdgeInsets.all(16.w),
      decoration: BoxDecoration(
        color: AppColors.bgCard,
        borderRadius: BorderRadius.circular(12.r),
        boxShadow: [
          BoxShadow(
            color: AppColors.cardShadow,
            blurRadius: 4,
            offset: Offset(0, 2.h),
          ),
        ],
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // 標題
          Row(
            children: [
              HkIcon(
                HkIconName.receipt,
                color: AppIconColors.defaultColor,
                size: 20.sp,
              ),
              SizedBox(width: 8.w),
              Text(
                '訂單資訊',
                style: TextStyle(
                  fontSize: 14.sp,
                  fontWeight: FontWeight.w700,
                  color: AppColors.textPrimary,
                ),
              ),
            ],
          ),
          SizedBox(height: 12.h),
          const Divider(height: 1, color: AppColors.gray100),
          SizedBox(height: 12.h),
          // 物流編號
          _buildInfoRowWithCopyLong('物流編號', info.shipTradeNo),
          SizedBox(height: 8.h),
          // 託運單號
          if (info.odno.isNotEmpty) ...[
            _buildInfoRowWithCopyLong('託運單號', info.odno),
            SizedBox(height: 8.h),
          ],
          // 商家訂單編號
          if (info.merTradeNo.isNotEmpty) ...[
            _buildInfoRowWithCopyLong('商家單號', info.merTradeNo),
            SizedBox(height: 8.h),
          ],
          // 平台訂單編號
          if (widget.orderSn != null && widget.orderSn!.isNotEmpty) ...[
            _buildInfoRowWithCopyLong('訂單編號', widget.orderSn!),
          ],
        ],
      ),
    );
  }

  Widget _buildInfoRowWithCopyLong(String label, String value) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          label,
          style: TextStyle(fontSize: 12.sp, color: AppColors.textSecondary),
        ),
        SizedBox(height: 4.h),
        Row(
          children: [
            Expanded(
              child: Text(
                value.isNotEmpty ? value : '-',
                style: TextStyle(
                  fontSize: 14.sp,
                  color: AppColors.textEmphasis,
                ),
              ),
            ),
            if (value.isNotEmpty)
              GestureDetector(
                onTap: () {
                  Clipboard.setData(ClipboardData(text: value));
                  FloatingToast.show(
                    context,
                    '已複製 $label',
                    icon: AppIcons.contentCopy,
                  );
                },
                child: HkIcon(
                  HkIconName.copy,
                  size: 16.sp,
                  color: AppIconColors.muted,
                ),
              ),
          ],
        ),
      ],
    );
  }
}

/// 進度步驟模型
class _ProgressStep {
  final String label;
  final HkIconName icon;
  final bool isActive;

  _ProgressStep(this.label, this.icon, this.isActive);
}
