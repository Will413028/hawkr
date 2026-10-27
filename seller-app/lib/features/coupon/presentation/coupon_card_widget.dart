import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

import '../../../core/codemeta/code_meta.dart';
import '../../../core/codemeta/code_meta_display.dart';
import '../data/coupon_model.dart';
import 'widgets/coupon_type_badge.dart';
import 'widgets/date_formatter.dart';

/// Coupon card widget — white card + top-right badge + press scale animation.
class CouponCardWidget extends StatefulWidget {
  final Coupon coupon;
  final VoidCallback? onTap;

  /// 是否显示 checkbox（用于删除模式）
  final bool showCheckbox;

  /// checkbox 是否被选中
  final bool isSelected;

  /// checkbox 状态改变回调
  final ValueChanged<bool>? onSelectionChanged;

  const CouponCardWidget({
    super.key,
    required this.coupon,
    this.onTap,
    this.showCheckbox = false,
    this.isSelected = false,
    this.onSelectionChanged,
  });

  @override
  State<CouponCardWidget> createState() => _CouponCardWidgetState();
}

class _CouponCardWidgetState extends State<CouponCardWidget>
    with SingleTickerProviderStateMixin {
  late final AnimationController _pressCtrl = AnimationController(
    vsync: this,
    duration: const Duration(milliseconds: 100),
  );
  late final Animation<double> _scale = Tween<double>(
    begin: 1.0,
    end: 0.97,
  ).animate(CurvedAnimation(parent: _pressCtrl, curve: Curves.easeInOut));

  @override
  void dispose() {
    _pressCtrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final coupon = widget.coupon;
    final badge = _badgeConfig(coupon);

    return GestureDetector(
      onTap: widget.onTap,
      onTapDown: (_) => _pressCtrl.forward(),
      onTapUp: (_) => _pressCtrl.reverse(),
      onTapCancel: () => _pressCtrl.reverse(),
      child: ScaleTransition(
        scale: _scale,
        child: Stack(
          clipBehavior: Clip.none,
          children: [
            // ── White card with selection border ──
            Container(
              width: double.infinity,
              decoration: BoxDecoration(
                color: AppColors.bgCard,
                borderRadius: BorderRadius.circular(16.r),
                border: Border.all(
                  width: widget.isSelected ? 2 : 1,
                  color: widget.isSelected
                      ? AppColors.primary
                      : AppColors.cardBorder,
                ),
                boxShadow: [
                  BoxShadow(
                    color: widget.isSelected
                        ? AppColors.primary.withValues(alpha: 0.2)
                        : AppColors.cardShadow,
                    blurRadius: widget.isSelected ? 6 : 2,
                    offset: Offset(0, widget.isSelected ? 3 : 1),
                  ),
                ],
              ),
              child: Row(
                children: [
                  // ── Checkbox on far left center（删除模式时显示）──
                  if (widget.showCheckbox)
                    GestureDetector(
                      onTap: () {
                        widget.onSelectionChanged?.call(!widget.isSelected);
                      },
                      child: Container(
                        width: 50.w,
                        alignment: Alignment.center,
                        child: Container(
                          width: 24.w,
                          height: 24.w,
                          decoration: BoxDecoration(
                            color: widget.isSelected
                                ? AppColors.primary
                                : AppColors.bgCard,
                            borderRadius: BorderRadius.circular(6.r),
                            border: Border.all(
                              color: widget.isSelected
                                  ? AppColors.primary
                                  : AppColors.gray300,
                              width: 2,
                            ),
                          ),
                          child: widget.isSelected
                              ? Icon(
                                  AppIcons.check,
                                  size: 16.sp,
                                  color: AppIconColors.onBrand,
                                )
                              : null,
                        ),
                      ),
                    ),

                  // 右側內容區
                  Expanded(
                    child: Padding(
                      padding: EdgeInsets.only(
                        top: 12.h,
                        left: widget.showCheckbox ? 8.w : 16.w,
                        right: 64.w,
                        bottom: 12.h,
                      ),
                      child: Column(
                        mainAxisSize: MainAxisSize.min,
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          // Row 1: avatar + title
                          Row(
                            children: [
                              _buildAvatar(),
                              SizedBox(width: 9.w),
                              Expanded(
                                child: Text(
                                  coupon.title,
                                  style: TextStyle(
                                    fontFamily: 'Roboto',
                                    fontWeight: FontWeight.w700,
                                    fontSize: 18.sp,
                                    height: 1.23,
                                    color: AppColors.textPrimary,
                                  ),
                                  maxLines: 1,
                                  overflow: TextOverflow.ellipsis,
                                ),
                              ),
                            ],
                          ),
                          SizedBox(height: 6.h),

                          // Description
                          Text(
                            _description(coupon),
                            style: TextStyle(
                              fontFamily: 'Roboto',
                              fontWeight: FontWeight.w400,
                              fontSize: 16.sp,
                              height: 1.50,
                              color: AppColors.textPrimary,
                            ),
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                          ),
                          SizedBox(height: 6.h),

                          // Threshold
                          if (coupon.thresholdAmount != null &&
                              coupon.thresholdAmount!.isNotEmpty &&
                              coupon.thresholdAmount != '0') ...[
                            Text(
                              '• 消費滿 \$${coupon.thresholdAmount} 可用',
                              style: TextStyle(
                                fontFamily: 'Roboto',
                                fontWeight: FontWeight.w400,
                                fontSize: 16.sp,
                                height: 1.50,
                                color: AppColors.textPrimary,
                              ),
                              maxLines: 1,
                              overflow: TextOverflow.ellipsis,
                            ),
                            SizedBox(height: 6.h),
                          ],

                          // Expiry（使用 DateFormatter 格式化日期）
                          if (coupon.endAt != null && coupon.endAt!.isNotEmpty)
                            Row(
                              children: [
                                Icon(
                                  AppIcons.accessTimeOutlined,
                                  size: 24.sp,
                                  color: AppIconColors.defaultColor,
                                ),
                                SizedBox(width: 5.w),
                                Expanded(
                                  child: Text(
                                    '有效期至 ${DateFormatter.formatToYYYYMMDD(coupon.endAt)}',
                                    style: TextStyle(
                                      fontFamily: 'Roboto',
                                      fontWeight: FontWeight.w400,
                                      fontSize: 16.sp,
                                      height: 1.50,
                                      color: AppColors.textPrimary,
                                    ),
                                  ),
                                ),
                              ],
                            ),
                        ],
                      ),
                    ),
                  ),
                ],
              ),
            ),

            // ── Top-right badge ──
            Positioned(left: 306.w, top: -5.h, child: _buildBadge(badge)),
          ],
        ),
      ),
    );
  }

  Widget _buildAvatar() {
    return Container(
      width: 45.w,
      height: 45.w,
      decoration: const BoxDecoration(
        color: AppColors.bgPrimary,
        shape: BoxShape.circle,
      ),
      child: Center(
        child: Icon(
          AppIcons.localOffer,
          size: 22.sp,
          color: AppIconColors.danger,
        ),
      ),
    );
  }

  Widget _buildBadge(_BadgeConfig badge) {
    // 使用通用的 CouponTypeBadge 组件
    return CouponTypeBadge(type: badge.type, label: badge.label);
  }

  static String _description(Coupon coupon) {
    return switch (CodeMetaDisplay.couponTypeMeta(coupon.type)) {
      CouponType.cash =>
        '消費滿\$${coupon.thresholdAmount ?? '0'}可折抵\$${coupon.discountAmount ?? '0'}',
      CouponType.percent => '${coupon.discountPercent ?? '0'}%折優惠',
      null => CodeMetaDisplay.couponType(coupon.type),
    };
  }

  static _BadgeConfig _badgeConfig(Coupon coupon) {
    return switch (CodeMetaDisplay.couponTypeMeta(coupon.type)) {
      CouponType.percent => _BadgeConfig(
        type: CodeMetaDisplay.couponBadgeVariant(coupon.type),
        label: '${coupon.discountPercent ?? '0'}%',
      ),
      CouponType.cash || null => _BadgeConfig(
        type: CodeMetaDisplay.couponBadgeVariant(coupon.type),
        label: '\$${coupon.discountAmount ?? '0'}',
      ),
    };
  }
}

class _BadgeConfig {
  final String type;
  final String label;
  _BadgeConfig({required this.type, required this.label});
}
