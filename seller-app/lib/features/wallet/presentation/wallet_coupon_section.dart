import 'package:flutter/material.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:hawkr_seller/shared/widgets/hk_icon.dart';
import 'package:hawkr_seller/shared/widgets/hk_tap_target.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../core/router/app_router.dart';
import '../../../core/theme/app_colors.dart';
import '../../../shared/utils/hk_log.dart';
import '../../../shared/utils/floating_toast.dart';
import '../../../shared/widgets/hk_choice_chip.dart';
import '../../../shared/widgets/hk_empty_state.dart';
import '../../../shared/widgets/truck_loading_overlay.dart';
import '../../coupon/presentation/coupon_card_widget.dart';
import '../../coupon/presentation/coupon_controller.dart';

/// 錢包頁的優惠券區塊：進行中與已過期的篩選、新增、批量刪除。
class WalletCouponSection extends HookConsumerWidget {
  const WalletCouponSection({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final couponFilter = useState('active');
    final isDeleteMode = useState(false);
    final selectedCoupons = useState<Set<String>>({});
    final couponState = ref.watch(couponListControllerProvider);

    Widget buildStatusBar() {
      return Row(
        children: [
          // 兩個篩選放不下時換行（窄螢幕加大字級），右側的按鈕維持靠右。
          Expanded(
            child: Wrap(
              spacing: 12.w,
              runSpacing: 8.h,
              children: [
                HkChoiceChip(
                  label: '進行中',
                  selected: couponFilter.value == 'active',
                  icon: AppIcons.chartOutlined,
                  selectedIcon: AppIcons.chart,
                  onTap: () {
                    couponFilter.value = 'active';
                    // 切換篩選時退出刪除模式
                    isDeleteMode.value = false;
                    selectedCoupons.value = {};
                  },
                ),
                HkChoiceChip(
                  label: '已過期',
                  selected: couponFilter.value == 'expired',
                  icon: AppIcons.checkCircleOutline,
                  selectedIcon: AppIcons.checkCircle,
                  onTap: () {
                    couponFilter.value = 'expired';
                    // 切換篩選時退出刪除模式
                    isDeleteMode.value = false;
                    selectedCoupons.value = {};
                  },
                ),
              ],
            ),
          ),
          SizedBox(width: 8.w),
          HkTapTarget(
            semanticLabel: '新增優惠券',
            onTap: () {
              HKLog.logD('[Wallet] Add coupon tapped');
              context.push('/coupon/editor');
            },
            child: const _CircleGlyph(AppIcons.add),
          ),
          SizedBox(width: 8.w),
          HkTapTarget(
            semanticLabel: '刪除優惠券',
            onTap: () {
              HKLog.logD('[Wallet] Delete coupon tapped');
              // 只有在進行中模式下才能進入刪除模式
              if (couponFilter.value == 'active') {
                isDeleteMode.value = !isDeleteMode.value;
                // 退出刪除模式時清空選擇
                if (!isDeleteMode.value) {
                  selectedCoupons.value = {};
                }
              } else {
                FloatingToast.show(context, '請先切換到進行中標籤');
              }
            },
            child: const _CircleGlyph(AppIcons.deleteOutline),
          ),
        ],
      );
    }

    /// 批量刪除選中的優惠券
    Future<void> deleteSelectedCoupons() async {
      final selectedIds = selectedCoupons.value.toList();
      HKLog.logD('[Wallet] deleteSelectedCoupons: ids=$selectedIds');
      int successCount = 0;
      int failCount = 0;

      for (final templateId in selectedIds) {
        try {
          final ok = await ref
              .read(couponListControllerProvider.notifier)
              .delete(templateId);
          if (ok) {
            successCount++;
          } else {
            failCount++;
          }
        } catch (e) {
          failCount++;
          HKLog.logD('[Wallet] Delete coupon $templateId error: $e');
        }
      }

      if (!context.mounted) return;

      // 清空選擇並退出刪除模式
      selectedCoupons.value = {};
      isDeleteMode.value = false;

      // 顯示結果
      if (failCount == 0) {
        FloatingToast.success(context, '已刪除 $successCount 張優惠券');
      } else {
        FloatingToast.error(context, '刪除完成：成功 $successCount，失敗 $failCount');
      }
    }

    /// 顯示刪除確認彈窗
    Future<void> showDeleteConfirmDialog() async {
      final selectedCount = selectedCoupons.value.length;

      if (selectedCount == 0) {
        FloatingToast.show(context, '請先選擇要刪除的優惠券');
        return;
      }

      final confirmed = await showDialog<bool>(
        context: context,
        barrierDismissible: false,
        builder: (ctx) => Dialog(
          backgroundColor: AppColors.bgCard,
          insetPadding: EdgeInsets.symmetric(horizontal: 40.w),
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(16.r),
          ),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              // 標題
              Padding(
                padding: EdgeInsets.only(top: 20.h, bottom: 4.h),
                child: Text(
                  '刪除確認',
                  style: TextStyle(
                    fontSize: 18.sp,
                    fontWeight: FontWeight.w700,
                    color: AppColors.textEmphasis,
                  ),
                ),
              ),
              // 內容
              Padding(
                padding: EdgeInsets.fromLTRB(16.w, 12.h, 16.w, 20.h),
                child: Text(
                  '確定要刪除 $selectedCount 張優惠券嗎？\n此操作無法撤銷。',
                  textAlign: TextAlign.center,
                  style: TextStyle(
                    fontSize: 16.sp,
                    height: 1.5,
                    color: AppColors.textPrimary,
                  ),
                ),
              ),
              // 分隔線
              const Divider(
                height: 0,
                thickness: 0.5,
                color: AppColors.gray200,
              ),
              // 取消 / 確認刪除 按鈕
              SizedBox(
                height: 44.h,
                child: Row(
                  children: [
                    // 取消
                    Expanded(
                      child: GestureDetector(
                        behavior: HitTestBehavior.opaque,
                        onTap: () => Navigator.of(ctx).pop(false),
                        child: Center(
                          child: Text(
                            '取消',
                            style: TextStyle(
                              fontSize: 18.sp,
                              color: AppColors.info,
                            ),
                          ),
                        ),
                      ),
                    ),
                    // 中間分隔線
                    Container(
                      width: 0.5,
                      height: 44.h,
                      color: AppColors.gray200,
                    ),
                    // 確認刪除
                    Expanded(
                      child: GestureDetector(
                        behavior: HitTestBehavior.opaque,
                        onTap: () => Navigator.of(ctx).pop(true),
                        child: Center(
                          child: Text(
                            '確認刪除',
                            style: TextStyle(
                              fontSize: 18.sp,
                              fontWeight: FontWeight.w700,
                              color: AppColors.danger,
                            ),
                          ),
                        ),
                      ),
                    ),
                  ],
                ),
              ),
            ],
          ),
        ),
      );

      if (confirmed == true && context.mounted) {
        await deleteSelectedCoupons();
      }
    }

    Widget buildCouponSection() {
      return Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // 標題
          Text(
            '優惠券',
            style: TextStyle(
              fontSize: 20.sp,
              fontWeight: FontWeight.w700,
              color: AppColors.textEmphasis,
            ),
          ),
          SizedBox(height: 12.h),
          // 篩選與操作列（使用圖片資源）
          buildStatusBar(),
          SizedBox(height: 16.h),
          // 優惠券列表
          couponState.when(
            loading: () => const TruckLoadingOverlay(),
            error: (e, _) => Padding(
              padding: EdgeInsets.all(16.w),
              child: Text(
                '載入優惠券失敗',
                style: TextStyle(color: AppColors.textSecondary),
              ),
            ),
            data: (couponListState) {
              final filtered = couponListState.coupons.where((c) {
                // status 已在 _parseCoupon 中規範化為大寫 code
                // 進行中：UNUSED(未使用) / LOCKED(已鎖定)
                // 已過期：USED / EXPIRED / REVOKED / ANNUL
                final isActive = c.status == 'UNUSED' || c.status == 'LOCKED';
                return couponFilter.value == 'active' ? isActive : !isActive;
              }).toList();

              if (filtered.isEmpty) {
                return const HkEmptyState(
                  key: ValueKey('seller-coupons-empty-state'),
                  iconKey: ValueKey('seller-coupons-empty-state-icon'),
                  icon: HkIconName.coupon,
                  message: '暫無優惠券',
                  description: '建立優惠券後會顯示在這裡。',
                  topPadding: 24,
                );
              }

              return Column(
                children: [
                  // 優惠券列表
                  ...filtered.map(
                    (coupon) => Padding(
                      padding: EdgeInsets.only(bottom: 16.h),
                      child: CouponCardWidget(
                        coupon: coupon,
                        // 點整張卡：刪除模式切換勾選，否則開啟唯讀詳情頁。
                        // （優惠券模板無更新端點，故詳情頁為唯讀）
                        onTap: () {
                          final inDeleteMode =
                              isDeleteMode.value &&
                              couponFilter.value == 'active';
                          if (inDeleteMode) {
                            final current = Set<String>.from(
                              selectedCoupons.value,
                            );
                            if (current.contains(coupon.templateId)) {
                              current.remove(coupon.templateId);
                            } else {
                              current.add(coupon.templateId);
                            }
                            selectedCoupons.value = current;
                          } else {
                            HKLog.logD(
                              '[Wallet] Coupon tapped: ${coupon.templateId}',
                            );
                            context.push(CouponRoutes.detail, extra: coupon);
                          }
                        },
                        // 刪除模式下顯示 checkbox
                        showCheckbox:
                            isDeleteMode.value &&
                            couponFilter.value == 'active',
                        isSelected: selectedCoupons.value.contains(
                          coupon.templateId,
                        ),
                        onSelectionChanged: (selected) {
                          final current = Set<String>.from(
                            selectedCoupons.value,
                          );
                          if (selected) {
                            current.add(coupon.templateId);
                          } else {
                            current.remove(coupon.templateId);
                          }
                          selectedCoupons.value = current;
                        },
                      ),
                    ),
                  ),

                  // 刪除模式下的操作按鈕
                  if (isDeleteMode.value && couponFilter.value == 'active')
                    Padding(
                      padding: EdgeInsets.only(top: 8.h),
                      child: Row(
                        children: [
                          // 全選 / 取消全選
                          Expanded(
                            child: GestureDetector(
                              onTap: () {
                                if (selectedCoupons.value.length ==
                                    filtered.length) {
                                  // 取消全選
                                  selectedCoupons.value = {};
                                } else {
                                  // 全選
                                  selectedCoupons.value = filtered
                                      .map((c) => c.templateId)
                                      .toSet();
                                }
                              },
                              child: Container(
                                padding: EdgeInsets.symmetric(vertical: 12.h),
                                decoration: BoxDecoration(
                                  color: AppColors.bgCard,
                                  borderRadius: BorderRadius.circular(12.r),
                                  border: Border.all(color: AppColors.gray300),
                                ),
                                child: Center(
                                  child: Text(
                                    selectedCoupons.value.length ==
                                            filtered.length
                                        ? '取消全選'
                                        : '全選',
                                    style: TextStyle(
                                      fontSize: 14.sp,
                                      color: AppColors.textPrimary,
                                    ),
                                  ),
                                ),
                              ),
                            ),
                          ),
                          SizedBox(width: 12.w),
                          // 刪除按鈕
                          Expanded(
                            child: GestureDetector(
                              onTap: showDeleteConfirmDialog,
                              child: Container(
                                padding: EdgeInsets.symmetric(vertical: 12.h),
                                decoration: BoxDecoration(
                                  color: selectedCoupons.value.isEmpty
                                      ? AppColors.gray300
                                      : AppColors.danger,
                                  borderRadius: BorderRadius.circular(12.r),
                                ),
                                child: Center(
                                  child: Text(
                                    '刪除 (${selectedCoupons.value.length})',
                                    style: TextStyle(
                                      fontSize: 14.sp,
                                      fontWeight: FontWeight.w700,
                                      color: AppColors.bgCard,
                                    ),
                                  ),
                                ),
                              ),
                            ),
                          ),
                        ],
                      ),
                    ),
                ],
              );
            },
          ),
        ],
      );
    }

    return buildCouponSection();
  }
}

class _CircleGlyph extends StatelessWidget {
  const _CircleGlyph(this.icon);

  final IconData icon;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: 36.w,
      height: 36.w,
      decoration: BoxDecoration(
        shape: BoxShape.circle,
        border: Border.all(color: AppColors.borderSoft, width: 2),
      ),
      child: Icon(icon, size: 20.w, color: AppIconColors.muted),
    );
  }
}
