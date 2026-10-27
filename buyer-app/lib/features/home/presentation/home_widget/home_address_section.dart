import 'package:flutter/material.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../../core/theme/app_colors.dart';
import '../../../../core/theme/app_typography.dart';
import '../../../../core/theme/hk_tokens.dart';
import '../../../../shared/widgets/hk_icon.dart';
import '../../../address/data/address_model.dart';
import '../../../address/presentation/address_controller.dart';
import '../../../address/presentation/store_pickup_controller.dart';

/// 首頁物流設定列，摘要宅配地址與 7-11 取貨門市。
class HomeAddressSection extends ConsumerStatefulWidget {
  const HomeAddressSection({super.key, required this.onTap});

  final VoidCallback onTap;

  @override
  ConsumerState<HomeAddressSection> createState() => _HomeAddressSectionState();
}

class _HomeAddressSectionState extends ConsumerState<HomeAddressSection> {
  @override
  Widget build(BuildContext context) {
    final addresses = ref.watch(addressControllerProvider).value ?? [];
    final defaultAddr =
        addresses.where((a) => a.isDefault).firstOrNull ??
        addresses.firstOrNull;
    final displayAddress = defaultAddr?.fullAddress ?? '';
    final storePickups = ref.watch(storePickupControllerProvider).value ?? [];
    final defaultStorePickup =
        storePickups.where((pickup) => pickup.isDefault).firstOrNull ??
        storePickups.firstOrNull;
    final displayStore = defaultStorePickup == null
        ? ''
        : (defaultStorePickup.storeName.isNotEmpty
              ? defaultStorePickup.storeName
              : '店號 ${defaultStorePickup.storeCode}');
    final logisticsSummary = switch ((displayAddress, displayStore)) {
      ('', '') => '設定宅配地址或 7-11 取貨門市',
      (final address, '') => '宅配：$address',
      ('', final store) => '7-11：$store',
      _ => '宅配與 7-11 已設定',
    };

    return LayoutBuilder(
      builder: (context, constraints) {
        final gutter = HkBreakpoints.pageGutter(constraints.maxWidth);
        return Semantics(
          button: true,
          label: '物流設定，$logisticsSummary',
          onTap: widget.onTap,
          child: ExcludeSemantics(
            child: GestureDetector(
              behavior: HitTestBehavior.opaque,
              onTap: widget.onTap,
              child: ColoredBox(
                key: const ValueKey('home-address-section'),
                color: AppColors.bgPrimary,
                child: ConstrainedBox(
                  constraints: const BoxConstraints(minHeight: 56),
                  child: Padding(
                    padding: EdgeInsets.symmetric(
                      horizontal: gutter,
                      vertical: HkSpace.xs,
                    ),
                    child: Row(
                      children: [
                        SizedBox.square(
                          dimension: HkControl.minTap,
                          child: Center(
                            child: HkIcon(
                              HkIconName.truck,
                              key: const ValueKey('home-address-delivery-icon'),
                              size: 32,
                            ),
                          ),
                        ),
                        const SizedBox(width: HkSpace.xs),
                        Expanded(
                          child: Column(
                            mainAxisSize: MainAxisSize.min,
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text(
                                '物流設定',
                                maxLines: 1,
                                overflow: TextOverflow.ellipsis,
                                style: AppTypography.productName.copyWith(
                                  color: AppColors.textEmphasis,
                                ),
                              ),
                              Text(
                                logisticsSummary,
                                maxLines: 1,
                                overflow: TextOverflow.ellipsis,
                                style: AppTypography.meta.copyWith(
                                  color: AppColors.textPrimary,
                                ),
                              ),
                            ],
                          ),
                        ),
                        const SizedBox(width: HkSpace.sm),
                        Text(
                          '點擊設定',
                          maxLines: 1,
                          style: AppTypography.meta.copyWith(
                            fontWeight: FontWeight.w700,
                            color: AppColors.brandPrimaryStrong,
                          ),
                        ),
                      ],
                    ),
                  ),
                ),
              ),
            ),
          ),
        );
      },
    );
  }
}
