import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/core/theme/app_typography.dart';
import 'package:hawkr_seller/shared/widgets/hk_button_label.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:hawkr_seller/shared/widgets/hk_icon.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../../config/image_config.dart';
import '../../../../shared/widgets/hk_empty_state.dart';
import '../../../../shared/widgets/optimized_image.dart';
import '../../../../shared/widgets/truck_loading_overlay.dart';
import '../../data/subscriber_model.dart';
import '../subscriber_list_controller.dart';

class SubscriberListContent extends StatelessWidget {
  const SubscriberListContent({
    super.key,
    required this.state,
    required this.onSubscriberTap,
    this.onLoadMore,
  });

  final AsyncValue<SubscriberListViewData> state;
  final ValueChanged<Subscriber> onSubscriberTap;
  final VoidCallback? onLoadMore;

  @override
  Widget build(BuildContext context) {
    return switch (state) {
      AsyncLoading<SubscriberListViewData>() => const TruckLoadingOverlay(),
      AsyncError<SubscriberListViewData>(:final error) => Center(
        child: Text(subscriberListErrorMessage(error)),
      ),
      AsyncData<SubscriberListViewData>(:final value) =>
        value.subscribers.isEmpty
            ? const HkEmptyState(
                key: ValueKey('seller-subscribers-empty-state'),
                iconKey: ValueKey('seller-subscribers-empty-state-icon'),
                icon: HkIconName.users,
                message: '暫無訂閱者',
                description: '買家訂閱商店後會自動出現在這裡。',
                topPadding: 48,
              )
            : ListView.builder(
                padding: EdgeInsets.symmetric(horizontal: 24.w, vertical: 12.h),
                itemCount:
                    value.subscribers.length +
                    (value.nextPageToken.isNotEmpty && onLoadMore != null
                        ? 1
                        : 0),
                itemBuilder: (context, index) {
                  if (index == value.subscribers.length) {
                    return Padding(
                      padding: EdgeInsets.only(top: 8.h, bottom: 20.h),
                      child: OutlinedButton(
                        onPressed: value.loadingMore ? null : onLoadMore,
                        style: OutlinedButton.styleFrom(
                          foregroundColor: AppColors.gray1_80,
                          side: BorderSide(color: AppColors.gray1_20),
                          shape: RoundedRectangleBorder(
                            borderRadius: BorderRadius.circular(999.r),
                          ),
                          padding: EdgeInsets.symmetric(vertical: 10.h),
                          textStyle: AppTypography.meta,
                        ),
                        child: HkButtonLabel(
                          value.loadingMore ? '載入中…' : '載入更多',
                        ),
                      ),
                    );
                  }
                  final subscriber = value.subscribers[index];
                  return Padding(
                    padding: EdgeInsets.only(bottom: 12.h),
                    child: SubscriberCard(
                      subscriber: subscriber,
                      onTap: () => onSubscriberTap(subscriber),
                    ),
                  );
                },
              ),
    };
  }
}

class SubscriberCard extends StatelessWidget {
  const SubscriberCard({super.key, required this.subscriber, this.onTap});

  final Subscriber subscriber;
  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        padding: EdgeInsets.symmetric(horizontal: 20.w, vertical: 12.h),
        decoration: BoxDecoration(
          color: AppColors.bgCard,
          borderRadius: BorderRadius.circular(8.r),
          border: Border.all(color: AppColors.cardBorder),
          boxShadow: [
            BoxShadow(
              color: AppColors.cardShadow,
              blurRadius: 2.r,
              offset: Offset(0, 1.h),
            ),
          ],
        ),
        child: Row(
          children: [
            Container(
              width: 48.w,
              height: 48.h,
              decoration: BoxDecoration(
                shape: BoxShape.circle,
                border: Border.all(color: AppColors.textSecondary, width: 1),
                color: AppColors.gray100,
              ),
              child: ClipOval(
                child:
                    subscriber.avatarUrl.isNotEmpty &&
                        subscriber.avatarUrl.startsWith('http')
                    ? OptimizedNetworkImage(
                        imageUrl: subscriber.avatarUrl,
                        scene: ImageScene.avatarMedium,
                        fit: BoxFit.cover,
                        width: 48.w,
                        height: 48.h,
                        placeholder: const SizedBox.shrink(),
                        errorWidget: Icon(
                          AppIcons.person,
                          size: 26.sp,
                          color: AppIconColors.muted,
                        ),
                      )
                    : Icon(
                        AppIcons.person,
                        size: 26.sp,
                        color: AppIconColors.muted,
                      ),
              ),
            ),
            SizedBox(width: 16.w),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                mainAxisAlignment: MainAxisAlignment.center,
                children: [
                  Text(
                    subscriber.displayName,
                    style: TextStyle(
                      fontSize: 16.sp,
                      fontWeight: FontWeight.w700,
                      color: AppColors.textPrimary,
                    ),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  ),
                  SizedBox(height: 4.h),
                  Text(
                    '訂閱者',
                    style: TextStyle(
                      fontSize: 14.sp,
                      fontWeight: FontWeight.w400,
                      color: AppColors.textSecondary,
                    ),
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}
