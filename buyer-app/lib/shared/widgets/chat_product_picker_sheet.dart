import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:hawkr_buyer/shared/widgets/hk_empty_state.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

import '../../config/image_config.dart';
import 'app_cached_image.dart';
import '../utils/price_display.dart';
import 'package:hawkr_buyer/shared/widgets/hk_brand_loader.dart';

/// 聊天室「選擇上架商品」半屏 BottomSheet
///
/// 列出賣家的商品清單，讓買/賣家選一筆送出成商品卡片。
Future<Map<String, dynamic>?> showProductPickerSheet({
  required BuildContext context,
  required List<Map<String, dynamic>> products,
  bool loading = false,
}) {
  return showModalBottomSheet<Map<String, dynamic>>(
    context: context,
    isScrollControlled: true,
    useSafeArea: true,
    showDragHandle: false,
    backgroundColor: AppColors.bgCard,
    shape: RoundedRectangleBorder(
      borderRadius: BorderRadius.vertical(top: Radius.circular(16.r)),
    ),
    builder: (ctx) => _ProductPickerBody(products: products, loading: loading),
  );
}

class _ProductPickerBody extends StatelessWidget {
  const _ProductPickerBody({required this.products, this.loading = false});
  final List<Map<String, dynamic>> products;
  final bool loading;

  @override
  Widget build(BuildContext context) {
    return DraggableScrollableSheet(
      initialChildSize: 0.55,
      minChildSize: 0.35,
      maxChildSize: 0.85,
      expand: false,
      builder: (ctx, scrollCtrl) => Column(
        children: [
          Padding(
            padding: EdgeInsets.only(top: 12.h),
            child: Container(
              width: 40.w,
              height: 4.h,
              decoration: BoxDecoration(
                color: AppColors.dividerLight,
                borderRadius: BorderRadius.circular(2.r),
              ),
            ),
          ),
          Padding(
            padding: EdgeInsets.symmetric(vertical: 12.h),
            child: Text(
              '選擇商品',
              style: TextStyle(
                fontSize: 18,
                fontWeight: FontWeight.w700,
                color: AppColors.textEmphasis,
              ),
            ),
          ),
          const Divider(height: 1),
          Expanded(
            child: loading
                ? const Center(child: HkBrandLoader(size: HkBrandLoader.page))
                : products.isEmpty
                ? const HkEmptyState(
                    key: ValueKey('chat-product-picker-empty-state'),
                    iconKey: ValueKey('chat-product-picker-empty-state-icon'),
                    icon: HkIconName.emptyProducts,
                    message: '暫無上架商品',
                  )
                : ListView.separated(
                    controller: scrollCtrl,
                    padding: EdgeInsets.symmetric(
                      horizontal: 16.w,
                      vertical: 12.h,
                    ),
                    itemCount: products.length,
                    separatorBuilder: (_, __) => SizedBox(height: 10.h),
                    itemBuilder: (_, i) => _ProductPickerCard(
                      product: products[i],
                      onTap: () => Navigator.pop(ctx, products[i]),
                    ),
                  ),
          ),
        ],
      ),
    );
  }
}

class _ProductPickerCard extends StatelessWidget {
  const _ProductPickerCard({required this.product, required this.onTap});
  final Map<String, dynamic> product;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final name =
        (product['productName'] ?? product['name'] ?? product['skuName'] ?? '')
            .toString();
    // 判準走 PriceDisplay：`"0"` = 無價格，不能印成 `NT$ 0`。
    final price = PriceDisplay.formatPrice(
      product['price'] ?? product['unitPrice'] ?? product['priceMin'],
      prefix: 'NT\$ ',
    );
    final imageUrl =
        (product['imageKey'] ?? product['imageUrl'] ?? product['image'] ?? '')
            .toString();

    return GestureDetector(
      onTap: onTap,
      child: Container(
        padding: EdgeInsets.symmetric(horizontal: 16.w, vertical: 12.h),
        decoration: BoxDecoration(
          color: AppColors.bgCard,
          borderRadius: BorderRadius.circular(16.r),
          border: Border.all(color: AppColors.cardBorder),
          boxShadow: const [
            BoxShadow(
              color: AppColors.cardShadow,
              blurRadius: 2,
              offset: Offset(0, 1),
            ),
          ],
        ),
        child: Row(
          children: [
            // 50×50 image
            Container(
              width: 50.w,
              height: 50.w,
              decoration: BoxDecoration(
                color: AppColors.gray100,
                borderRadius: BorderRadius.circular(8.r),
              ),
              clipBehavior: Clip.antiAlias,
              child: imageUrl.isNotEmpty
                  ? AppCachedImage(
                      imageUrl: imageUrl,
                      scene: ImageScene.productThumbnail,
                      fit: BoxFit.cover,
                      width: 50.w,
                      height: 50.w,
                      errorWidget: Icon(
                        AppIcons.imageOutlined,
                        size: 24.sp,
                        color: AppIconColors.disabled,
                      ),
                    )
                  : Icon(
                      AppIcons.imageOutlined,
                      size: 24.sp,
                      color: AppIconColors.disabled,
                    ),
            ),
            SizedBox(width: 12.w),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    name,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: TextStyle(
                      fontSize: 14,
                      fontWeight: FontWeight.w700,
                      color: AppColors.textPrimary,
                    ),
                  ),
                  SizedBox(height: 4.h),
                  if (price.isNotEmpty)
                    Text(
                      price,
                      style: TextStyle(
                        fontSize: 16,
                        fontWeight: FontWeight.w700,
                        color: AppColors.warning,
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
