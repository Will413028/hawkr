import 'package:hawkr_buyer/shared/widgets/hk_temperature_tag.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

import '../../../../core/codemeta/shipping_presentation.dart';
import '../../../../core/theme/app_colors.dart';
import '../../../../core/theme/app_spacing.dart';
import '../../../../core/theme/app_typography.dart';
import '../../../../shared/widgets/hk_icon.dart';
import '../../data/after_sale_model.dart';
import '../../data/order_status.dart';
import 'after_sale_vertical_timeline.dart';

/// 黑貓退貨退款狀態摘要卡。
///
/// 用在買家訂單展開與售後詳情頁，對齊設計稿：
/// 上排狀態 + 右側處理時間，下排提醒文案。只使用售後詳情已回傳資料，
/// 沒有時間時不顯示右側文字。
class BlackcatAfterSaleStatusCard extends StatelessWidget {
  const BlackcatAfterSaleStatusCard({super.key, required this.caseData});

  final AfterSaleCase caseData;

  static bool shouldShowFor(AfterSaleCase c) =>
      BlackcatReturnLogisticsCard.shouldShowFor(c);

  @override
  Widget build(BuildContext context) {
    final time = _statusTime(caseData);
    final message = _statusMessage(caseData);

    return Container(
      width: double.infinity,
      padding: EdgeInsets.symmetric(
        horizontal: AppSpacing.lg,
        vertical: AppSpacing.md,
      ),
      decoration: BoxDecoration(
        color: AppColors.white,
        borderRadius: BorderRadius.circular(12.r),
        border: Border.all(color: AppColors.cardBorder),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Expanded(
                child: Text(
                  AfterSaleStatus.label(caseData.status),
                  style: AppTypography.bodyBold.copyWith(
                    color: AppColors.gray800,
                  ),
                ),
              ),
              if (time.isNotEmpty) ...[
                SizedBox(width: AppSpacing.sm),
                Text(
                  time,
                  style: AppTypography.body.copyWith(color: AppColors.gray500),
                  textAlign: TextAlign.right,
                ),
              ],
            ],
          ),
          if (message.isNotEmpty) ...[
            SizedBox(height: AppSpacing.sm),
            Text(
              message,
              style: AppTypography.body.copyWith(color: AppColors.gray600),
            ),
          ],
        ],
      ),
    );
  }

  static String _statusTime(AfterSaleCase c) {
    final decisionTime = _decisionTime(c);
    if (decisionTime.isNotEmpty) return _formatDateTime(decisionTime);
    if (c.createdAt.isNotEmpty) return _formatDateTime(c.createdAt);
    return '';
  }

  static String _decisionTime(AfterSaleCase c) {
    for (final r in c.resolutions) {
      if (r.resolutionType == ResolutionType.approved &&
          r.createdAt.isNotEmpty) {
        return r.createdAt;
      }
    }
    for (final r in c.resolutions) {
      if (r.createdAt.isNotEmpty) return r.createdAt;
    }
    return '';
  }

  static String _statusMessage(AfterSaleCase c) {
    if (c.providerMessage.trim().isNotEmpty) return c.providerMessage.trim();
    return switch (c.status) {
      AfterSaleStatus.awaitingReturn => '賣家已預約派車到府取件，請注意來電',
      AfterSaleStatus.returnInTransit => '黑貓已收件，退貨商品運送中',
      AfterSaleStatus.returnReceived => '賣家已收到退貨，商品驗貨中',
      AfterSaleStatus.refundProcessing => '平台正在處理退款',
      AfterSaleStatus.closed => '售後案件已完結',
      _ => '',
    };
  }

  static String _formatDateTime(String raw) {
    final dt = DateTime.tryParse(raw);
    if (dt == null) return raw;
    final local = dt.toLocal();
    return '${local.year}/${local.month.toString().padLeft(2, '0')}/${local.day.toString().padLeft(2, '0')} '
        '${local.hour.toString().padLeft(2, '0')}:${local.minute.toString().padLeft(2, '0')}';
  }
}

/// 黑貓退貨退款物流卡，對齊設計稿的淺綠資訊面板。
///
/// OpenAPI 售後詳情沒有預計取件日，因此不渲染該欄位；地址、電話、
/// 溫層由訂單詳情帶入，若沒有值就不顯示。
class BlackcatReturnLogisticsCard extends StatelessWidget {
  const BlackcatReturnLogisticsCard({
    super.key,
    required this.caseData,
    this.pickupAddress = '',
    this.contactPhone = '',
    this.temperatureCode = '',
  });

  final AfterSaleCase caseData;
  final String pickupAddress;
  final String contactPhone;
  final String temperatureCode;

  static bool shouldShowFor(AfterSaleCase c) {
    if (c.returnShippingMethod != ReturnShippingMethod.kuroneko) return false;
    final status = int.tryParse(c.status) ?? 0;
    return status >= int.parse(AfterSaleStatus.awaitingReturn) &&
        status <= int.parse(AfterSaleStatus.closed);
  }

  @override
  Widget build(BuildContext context) {
    final tempLabel = DeliveryTempLabel.labelFor(temperatureCode);
    final logisticsNo = caseData.shipTradeNo.isNotEmpty
        ? caseData.shipTradeNo
        : caseData.returnShipmentNo;

    final infoTiles = <Widget>[
      _InfoTile(label: '預約出車', value: '賣家已預約', valueColor: AppColors.primary),
      if (tempLabel.isNotEmpty)
        _InfoTile(
          label: '溫層',
          value: tempLabel,
          valueWidget: HkTemperatureTag(value: temperatureCode),
        ),
      if (contactPhone.trim().isNotEmpty)
        _InfoTile(label: '聯繫電話', value: contactPhone.trim()),
    ];

    return Container(
      width: double.infinity,
      padding: EdgeInsets.all(AppSpacing.md),
      decoration: BoxDecoration(
        color: AppColors.primarySoft,
        borderRadius: BorderRadius.circular(12.r),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          if (pickupAddress.trim().isNotEmpty) ...[
            _FullInfoTile(label: '取件地址', value: pickupAddress.trim()),
            SizedBox(height: AppSpacing.sm),
          ],
          if (infoTiles.isNotEmpty) ...[
            _InfoTileGrid(children: infoTiles),
            SizedBox(height: AppSpacing.sm),
          ],
          if (logisticsNo.trim().isNotEmpty) ...[
            _FullInfoTile(label: '退貨物流單', value: logisticsNo.trim()),
            SizedBox(height: AppSpacing.md),
          ],
          Text(
            '收件流程',
            style: AppTypography.bodyBold.copyWith(color: AppColors.gray500),
          ),
          SizedBox(height: AppSpacing.sm),
          _PickupTimeline(caseData: caseData),
          SizedBox(height: AppSpacing.xs),
          Text(
            '* 請保持電話暢通並備妥商品',
            style: AppTypography.meta.copyWith(color: AppColors.gray500),
          ),
        ],
      ),
    );
  }
}

class _FullInfoTile extends StatelessWidget {
  const _FullInfoTile({required this.label, required this.value});

  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: double.infinity,
      padding: EdgeInsets.symmetric(
        horizontal: AppSpacing.md,
        vertical: AppSpacing.sm,
      ),
      decoration: BoxDecoration(
        color: AppColors.white,
        borderRadius: BorderRadius.circular(8.r),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            label,
            style: AppTypography.meta.copyWith(color: AppColors.gray500),
          ),
          SizedBox(height: 2.h),
          Text(
            value,
            style: AppTypography.bodyBold.copyWith(color: AppColors.gray700),
          ),
        ],
      ),
    );
  }
}

class _InfoTileGrid extends StatelessWidget {
  const _InfoTileGrid({required this.children});

  final List<Widget> children;

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(
      builder: (context, constraints) {
        final itemWidth = children.length == 1
            ? constraints.maxWidth
            : (constraints.maxWidth - AppSpacing.sm) / 2;
        return Wrap(
          spacing: AppSpacing.sm,
          runSpacing: AppSpacing.sm,
          children: [
            for (final child in children)
              SizedBox(width: itemWidth, child: child),
          ],
        );
      },
    );
  }
}

class _InfoTile extends StatelessWidget {
  const _InfoTile({
    required this.label,
    required this.value,
    this.valueColor,
    this.valueWidget,
  });

  final String label;
  final String value;
  final Color? valueColor;
  final Widget? valueWidget;

  @override
  Widget build(BuildContext context) {
    return Container(
      constraints: BoxConstraints(minHeight: 68.h),
      padding: EdgeInsets.symmetric(
        horizontal: AppSpacing.md,
        vertical: AppSpacing.sm,
      ),
      decoration: BoxDecoration(
        color: AppColors.white,
        borderRadius: BorderRadius.circular(8.r),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Text(
            label,
            style: AppTypography.meta.copyWith(color: AppColors.gray500),
          ),
          SizedBox(height: 2.h),
          valueWidget ??
              Text(
                value,
                style: AppTypography.bodyBold.copyWith(
                  color: valueColor ?? AppColors.gray700,
                ),
                maxLines: 2,
                overflow: TextOverflow.ellipsis,
              ),
        ],
      ),
    );
  }
}

class _PickupTimeline extends StatelessWidget {
  const _PickupTimeline({required this.caseData});

  final AfterSaleCase caseData;

  @override
  Widget build(BuildContext context) {
    final status = int.tryParse(caseData.status) ?? 0;
    final decisionTime = BlackcatAfterSaleStatusCard._decisionTime(caseData);
    final firstTime = decisionTime.isNotEmpty
        ? _formatShortDateTime(decisionTime)
        : '';

    final steps = [
      _PickupStepData(
        label: '賣家預約出車',
        active: status >= int.parse(AfterSaleStatus.awaitingReturn),
        timestamp: firstTime,
      ),
      _PickupStepData(
        label: '司機到府收件',
        active: status >= int.parse(AfterSaleStatus.returnInTransit),
      ),
      _PickupStepData(
        label: '送達回賣家',
        active: status >= int.parse(AfterSaleStatus.returnReceived),
      ),
    ];

    return Column(
      children: [
        for (var i = 0; i < steps.length; i++)
          _PickupTimelineStep(step: steps[i], isLast: i == steps.length - 1),
      ],
    );
  }

  String _formatShortDateTime(String raw) {
    final dt = DateTime.tryParse(raw);
    if (dt == null) return raw;
    final local = dt.toLocal();
    return '${local.month.toString().padLeft(2, '0')}/${local.day.toString().padLeft(2, '0')} '
        '${local.hour.toString().padLeft(2, '0')}:${local.minute.toString().padLeft(2, '0')}';
  }
}

class _PickupStepData {
  const _PickupStepData({
    required this.label,
    required this.active,
    this.timestamp = '',
  });

  final String label;
  final bool active;
  final String timestamp;
}

class _PickupTimelineStep extends StatelessWidget {
  const _PickupTimelineStep({required this.step, required this.isLast});

  final _PickupStepData step;
  final bool isLast;

  @override
  Widget build(BuildContext context) {
    final dotColor = step.active ? AppColors.primary : AppColors.gray300;
    final textColor = step.active ? AppColors.gray700 : AppColors.gray400;

    return SizedBox(
      height: isLast ? 24.h : 32.h,
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 14.w,
            child: Column(
              children: [
                Container(
                  width: 7.w,
                  height: 7.w,
                  margin: EdgeInsets.only(top: 8.h),
                  decoration: BoxDecoration(
                    color: dotColor,
                    shape: BoxShape.circle,
                  ),
                ),
                if (!isLast)
                  Expanded(
                    child: Container(width: 1.5.w, color: AppColors.gray300),
                  ),
              ],
            ),
          ),
          SizedBox(width: AppSpacing.sm),
          Expanded(
            child: Text(
              step.label,
              style: (step.active ? AppTypography.bodyBold : AppTypography.body)
                  .copyWith(color: textColor),
            ),
          ),
          if (step.timestamp.isNotEmpty) ...[
            SizedBox(width: AppSpacing.sm),
            Text(
              step.timestamp,
              style: AppTypography.meta.copyWith(color: AppColors.primary),
              textAlign: TextAlign.right,
            ),
          ],
        ],
      ),
    );
  }
}

/// 退貨物流 timeline — 依 `returnShippingMethod` **明確分派**
/// 黑貓 4 步（預約 → 派車 → 到府收件 → 賣家收貨）、
/// 7-11 3 步（至門市 ibon → 列印寄件單 → 交門市人員），
/// 未知 / 本版未支援者走 provider-neutral 3 步。
///
/// 🔴 禁止改回 `_isCvs ? 超商 : 黑貓`：那會把 3003/3005 與任何後端新碼值
/// 靜默標成「黑貓宅配收件流程」並畫出黑貓 4 步。
///
/// 步驟完成狀態由 `case.status` 推算：
/// - `20 待退貨` → 黑貓 step 1 done / 711 全部可操作
/// - `25 退貨運送中` → 黑貓 step 2 done / 711 已交件
/// - `30 已收退貨` 或 `40 退款處理中` 或 `50 已結案` → 全部 done
class ReturnLogisticsTimeline extends StatelessWidget {
  const ReturnLogisticsTimeline({super.key, required this.caseData});

  final AfterSaleCase caseData;

  ShippingPresentation get _presentation =>
      shippingPresentationOf(caseData.returnShippingMethod);

  @override
  Widget build(BuildContext context) {
    final (title, titleIcon, steps) = switch (_presentation) {
      ShippingPresentation.cvsStorePickup => (
        '7-11 退貨便流程',
        HkIconName.store,
        _buildCvsSteps(),
      ),
      ShippingPresentation.homeDelivery => (
        '黑貓宅配收件流程',
        HkIconName.truck,
        _buildHomeSteps(),
      ),
      ShippingPresentation.unsupported => (
        '退貨物流進度',
        HkIconName.truck,
        _buildGenericSteps(),
      ),
    };
    return AfterSaleVerticalTimeline(
      title: title,
      titleIcon: titleIcon,
      steps: steps,
      backgroundColor: AppColors.warningSoft,
      padding: EdgeInsets.all(12.w),
    );
  }

  List<AfterSaleTimelineStep> _buildHomeSteps() {
    final s = caseData.status;
    final step1Done = _reachedOrPast(s, '20');
    final step2Done = _reachedOrPast(s, '25');
    final step3Done = _reachedOrPast(s, '25');
    final step4Done =
        _reachedOrPast(s, '30') ||
        _reachedOrPast(s, '40') ||
        _reachedOrPast(s, '50');

    return [
      AfterSaleTimelineStep(
        label: '賣家預約出車',
        icon: HkIconName.calendar,
        done: step1Done,
        current: s == '20' && !step2Done,
        timestamp: step1Done ? '已預約' : '等待中',
      ),
      AfterSaleTimelineStep(
        label: '黑貓確認派車',
        icon: HkIconName.truck,
        done: step2Done,
        current: s == '25' && !step3Done,
        timestamp: step2Done ? '已派車' : '等待中',
      ),
      AfterSaleTimelineStep(
        label: '司機到府收件',
        icon: HkIconName.inventory,
        done: step3Done,
        current: s == '25' && !step4Done,
        timestamp: step3Done ? '已收件' : '等待中',
      ),
      AfterSaleTimelineStep(
        label: '賣家已收到退貨',
        icon: HkIconName.check,
        done: step4Done,
        timestamp: step4Done ? '已收貨' : '等待中',
      ),
    ];
  }

  List<AfterSaleTimelineStep> _buildCvsSteps() {
    final s = caseData.status;
    final hasCode = caseData.returnShipmentNo.isNotEmpty;
    // status >= 25 不代表買家已寄 — backend 在拋單成功時就會把 status 推到 25，
    // 但買家可能還沒去 7-11。改用 providerStatus / providerMessage 判斷物流商
    // 是否真的收件（hasShipped）。歷史 case 已關案/收到時無物流訊息，回退到
    // status 比對。
    final shipped =
        AfterSaleReturnProgress.hasShipped(caseData) ||
        _reachedOrPast(s, '30') ||
        _reachedOrPast(s, '40') ||
        _reachedOrPast(s, '50');
    final received =
        AfterSaleReturnProgress.isDelivered(caseData) ||
        _reachedOrPast(s, '30') ||
        _reachedOrPast(s, '40') ||
        _reachedOrPast(s, '50');

    return [
      AfterSaleTimelineStep(
        label: '至門市 ibon 機台選擇「退貨便」',
        icon: HkIconName.qrCode,
        done: hasCode,
        current: hasCode && !shipped,
        timestamp: hasCode ? '退貨代碼：${caseData.returnShipmentNo}' : '等待代碼',
      ),
      AfterSaleTimelineStep(
        label: '列印寄件單並貼於包裹',
        icon: HkIconName.document,
        done: shipped,
        current: hasCode && !shipped,
        timestamp: shipped ? '已列印' : '等待寄件',
      ),
      AfterSaleTimelineStep(
        label: '交付門市人員',
        icon: HkIconName.store,
        done: received,
        current: shipped && !received,
        timestamp: received
            ? '已收貨'
            : shipped
            ? '配送中'
            : '等待中',
      ),
    ];
  }

  /// provider-neutral 退貨步驟 — 未知 / 本版未支援的退貨方式使用。
  ///
  /// 只讀 `AfterSaleCase.status`（本身與物流商無關），不出現任何物流商名稱、
  /// 門市／到府語意或 provider 專屬代碼欄位。
  List<AfterSaleTimelineStep> _buildGenericSteps() {
    final s = caseData.status;
    final created = _reachedOrPast(s, '20');
    final shipped = _reachedOrPast(s, '25');
    final received =
        _reachedOrPast(s, '30') ||
        _reachedOrPast(s, '40') ||
        _reachedOrPast(s, '50');

    return [
      AfterSaleTimelineStep(
        label: '賣家已建立退貨',
        icon: HkIconName.document,
        done: created,
        current: created && !shipped,
        timestamp: created ? '已建立' : '等待中',
      ),
      AfterSaleTimelineStep(
        label: '退貨運送中',
        icon: HkIconName.truck,
        done: shipped,
        current: shipped && !received,
        timestamp: shipped ? '運送中' : '等待中',
      ),
      AfterSaleTimelineStep(
        label: '賣家已收到退貨',
        icon: HkIconName.check,
        done: received,
        timestamp: received ? '已收貨' : '等待中',
      ),
    ];
  }

  /// 判斷狀態 [s] 是否已達到 [target]（數字大小比較）。
  bool _reachedOrPast(String s, String target) {
    final cur = int.tryParse(s) ?? -1;
    final t = int.tryParse(target) ?? -1;
    return cur != -1 && t != -1 && cur >= t;
  }
}

/// 退款進度 — 當 `case.status >= 30` 時顯示。
///
/// 視覺對齊賣家 `退貨物流進度` 的 inline step indicator + 摘要卡片：
/// 上方四步驟橘色圓圈 + 連接線，下方狀態卡片（標題 / 時間戳 / 副標）。
///
/// - `30 已收退貨` → 步驟 1 done，步驟 2 current（商品驗貨中）
/// - `40 退款處理中` → 步驟 2 done，步驟 3 current（退款處理中）
/// - `50 已結案` → 步驟 4 current（退款入帳）
class RefundProgressTimeline extends StatelessWidget {
  const RefundProgressTimeline({super.key, required this.caseData});

  final AfterSaleCase caseData;

  static const _accent = AppColors.warning;
  static const _bg = AppColors.warningSoft;
  static const _muted = AppColors.gray300;
  static const _textPrimary = AppColors.textPrimary;
  static const _textMuted = AppColors.textSecondary;

  @override
  Widget build(BuildContext context) {
    final s = caseData.status;
    final inspecting = s == '30';
    final processing = s == '40';
    final refunded = s == '50';

    final refundAmount = caseData.resolutions.isNotEmpty
        ? caseData.resolutions.first.refundAmount
        : caseData.refundableAmount;

    // 決定當前 step（0-3）與摘要卡內容
    int currentStep;
    String summaryTitle;
    String summarySub;
    String summaryTime;
    if (refunded) {
      currentStep = 3;
      summaryTitle = refundAmount.isNotEmpty && refundAmount != '0'
          ? '退款 NT\$ $refundAmount 已入帳'
          : '退款已入帳';
      summarySub = '退款已完成';
      summaryTime = caseData.bankDate.isNotEmpty
          ? caseData.bankDate
          : caseData.closedAt;
    } else if (processing) {
      currentStep = 2;
      summaryTitle = '退款處理中';
      summarySub = caseData.estimatedRefundDate.isNotEmpty
          ? '預計 ${caseData.estimatedRefundDate} 入帳'
          : '系統處理中，預計 1-3 個工作天入帳';
      summaryTime = '';
    } else if (inspecting) {
      currentStep = 1;
      summaryTitle = '商品驗貨中';
      summarySub = '賣家正在確認商品狀態';
      summaryTime = '';
    } else {
      currentStep = 0;
      summaryTitle = '賣家收到退貨';
      summarySub = '等待賣家確認';
      summaryTime = '';
    }

    return Container(
      width: double.infinity,
      padding: EdgeInsets.all(12.w),
      decoration: BoxDecoration(
        color: _bg,
        borderRadius: BorderRadius.circular(12.r),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              HkIcon(
                HkIconName.payment,
                size: 18.sp,
                color: AppIconColors.warningText,
              ),
              SizedBox(width: 6.w),
              Text(
                '退款進度',
                style: TextStyle(
                  fontSize: 14,
                  fontWeight: FontWeight.w700,
                  color: AppColors.warningText,
                ),
              ),
            ],
          ),
          SizedBox(height: 12.h),
          _RefundStepIndicator(
            currentStep: currentStep,
            activeColor: _accent,
            inactiveColor: _muted,
          ),
          SizedBox(height: 12.h),
          _RefundSummaryCard(
            title: summaryTitle,
            time: summaryTime,
            message: summarySub,
            primaryColor: _textPrimary,
            mutedColor: _textMuted,
          ),
        ],
      ),
    );
  }
}

/// 退款進度 horizontal step indicator（4 步：收件 → 驗貨 → 退款處理 → 入帳）。
/// 視覺對齊賣家 `_InlineStepProgress`：圓圈 34×34、icon 18sp、連接線 3h 圓角。
class _RefundStepIndicator extends StatelessWidget {
  const _RefundStepIndicator({
    required this.currentStep,
    required this.activeColor,
    required this.inactiveColor,
  });

  final int currentStep;
  final Color activeColor;
  final Color inactiveColor;

  static const _steps = <(String, HkIconName)>[
    ('賣家收到退貨', HkIconName.inventory),
    ('商品驗貨中', HkIconName.search),
    ('退款處理中', HkIconName.payment),
    ('退款入帳', HkIconName.check),
  ];

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(
      builder: (context, constraints) {
        final lineWidth =
            ((constraints.maxWidth - _steps.length * 54.w) /
                    (_steps.length - 1))
                .clamp(8.0, double.infinity);
        return Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            for (var i = 0; i < _steps.length; i++) ...[
              SizedBox(
                width: 54.w,
                child: Column(
                  children: [
                    Container(
                      width: 34.w,
                      height: 34.w,
                      decoration: BoxDecoration(
                        color: i <= currentStep ? activeColor : inactiveColor,
                        shape: BoxShape.circle,
                      ),
                      alignment: Alignment.center,
                      child: HkIcon(
                        _steps[i].$2,
                        size: 18.sp,
                        color: i <= currentStep
                            ? AppIconColors.strong
                            : AppIconColors.onBrand,
                      ),
                    ),
                    SizedBox(height: 5.h),
                    Text(
                      _steps[i].$1,
                      textAlign: TextAlign.center,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(
                        fontSize: 12,
                        color: i <= currentStep
                            ? AppColors.textPrimary
                            : AppColors.textSecondary,
                      ),
                    ),
                  ],
                ),
              ),
              if (i < _steps.length - 1)
                Container(
                  width: lineWidth,
                  height: 3.h,
                  margin: EdgeInsets.only(top: 16.h),
                  decoration: BoxDecoration(
                    color: i < currentStep ? activeColor : inactiveColor,
                    borderRadius: BorderRadius.circular(999),
                  ),
                ),
            ],
          ],
        );
      },
    );
  }
}

/// 退款進度摘要卡 — 對齊賣家 `_processCard`：白底、標題 + 右側時間、副標。
class _RefundSummaryCard extends StatelessWidget {
  const _RefundSummaryCard({
    required this.title,
    required this.time,
    required this.message,
    required this.primaryColor,
    required this.mutedColor,
  });

  final String title;
  final String time;
  final String message;
  final Color primaryColor;
  final Color mutedColor;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: double.infinity,
      padding: EdgeInsets.symmetric(horizontal: 12.w, vertical: 12.h),
      decoration: BoxDecoration(
        color: AppColors.bgCard,
        borderRadius: BorderRadius.circular(12.r),
        border: Border.all(color: AppColors.cardBorder),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Expanded(
                child: Text(
                  title,
                  style: TextStyle(
                    fontSize: 14,
                    fontWeight: FontWeight.w700,
                    color: primaryColor,
                  ),
                ),
              ),
              if (time.isNotEmpty)
                Text(time, style: TextStyle(fontSize: 12, color: mutedColor)),
            ],
          ),
          if (message.isNotEmpty) ...[
            SizedBox(height: 6.h),
            Text(message, style: TextStyle(fontSize: 12, color: mutedColor)),
          ],
        ],
      ),
    );
  }
}
