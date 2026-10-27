import '../../../features/seller/data/current_store.dart';
import 'package:hawkr_seller/shared/widgets/hk_button_label.dart';
import 'package:hawkr_seller/shared/widgets/hk_tap_target.dart';
import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:hawkr_seller/shared/widgets/hk_glass_app_bar.dart';
import 'dart:io';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:go_router/go_router.dart';
import 'package:image_picker/image_picker.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:palette_generator_plus/palette_generator_plus.dart';

import 'post_controller.dart';
import 'widgets/post_hashtag_text_form_field.dart';
import '../../product/data/product_repository.dart';
import '../data/post_repository.dart';
import '../data/post_hashtag_extractor.dart';
import '../data/post_publish.dart';
import '../../photo_editor/presentation/photo_editor_args.dart';
import '../../../config/image_config.dart';
// if needed
import '../../../shared/utils/floating_toast.dart';
import '../../../shared/utils/image_format_detector.dart';
import '../../../shared/utils/price_display.dart';
import '../../../shared/utils/product_image_resolver.dart';
import '../../../shared/widgets/optimized_image.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

class PostComposeScreen extends HookConsumerWidget {
  final Uint8List? imagePreview;
  final Uint8List? mediaBytes;
  final String contentType;

  const PostComposeScreen({
    super.key,
    this.imagePreview,
    this.mediaBytes,
    required this.contentType,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final formKey = useMemoized(() => GlobalKey<FormState>());
    final titleCtrl = useTextEditingController();

    final currentContentType = useState(contentType);
    final currentImagePreview = useState(imagePreview);
    // 主色背景（跟商品上架同一套邏輯：dominantColor blend 到 page 背景的 0.45）
    final dominantColor = useState<Color?>(null);
    useEffect(() {
      final bytes = currentImagePreview.value;
      if (bytes == null) {
        dominantColor.value = null;
        return null;
      }
      () async {
        try {
          final palette = await PaletteGenerator.fromImageProvider(
            MemoryImage(bytes),
            size: const Size(50, 50),
            maximumColorCount: 4,
            quantizer: const LegacyQuantizer(),
          );
          dominantColor.value =
              palette.dominantColor?.color ?? AppColors.bgPrimary;
        } catch (_) {
          dominantColor.value = AppColors.bgPrimary;
        }
      }();
      return null;
    }, [currentImagePreview.value]);
    final currentMediaBytes = useState(mediaBytes);
    final submitting = useState(false);

    final loadingProducts = useState(true);
    final products = useState<List<Map<String, dynamic>>>([]);
    final selectedProductIndex = useState<int?>(null);

    final repo = ref.read(postRepositoryProvider);

    useEffect(() {
      Future.microtask(() async {
        final mid = ref.read(sellerStoreIdProvider);
        if (mid == null) {
          loadingProducts.value = false;
          return;
        }
        try {
          final result = await ref
              .read(productRepositoryProvider)
              .getMyProducts();
          final data = result['data'];
          final records = data is Map ? data['records'] : data;
          final list = records is List
              ? records.whereType<Map>().map(Map<String, dynamic>.from).toList()
              : <Map<String, dynamic>>[];
          if (context.mounted) {
            products.value = list;
            loadingProducts.value = false;
          }
        } catch (e) {
          debugPrint('[PostCompose] loadSellerProducts error: $e');
          if (context.mounted) loadingProducts.value = false;
        }
      });
      return null;
    }, []);

    Future<void> pickNewImage() async {
      final picker = ImagePicker();
      // 不指定 imageQuality，避免額外要求相簿轉碼；選取後仍依檔頭判斷格式。
      // 靜態圖由共用上傳層沿用原本壓縮策略。
      final picked = await picker.pickImage(source: ImageSource.gallery);
      if (picked == null) return;
      final bytes = await picked.readAsBytes();
      if (!context.mounted) return;
      currentImagePreview.value = bytes;
      currentMediaBytes.value = bytes;
      currentContentType.value = 'image';
    }

    Future<void> openImageEditor() async {
      if (currentMediaBytes.value == null) {
        FloatingToast.error(context, '請先選擇圖片');
        return;
      }
      if (ImageFormatDetector.detect(currentMediaBytes.value!).$2 ==
          'image/gif') {
        FloatingToast.error(context, 'GIF 動圖暫不支援編輯，已保留動圖格式');
        return;
      }
      final tempDir = Directory.systemTemp;
      final tempFile = File(
        '${tempDir.path}/post_edit_${DateTime.now().millisecondsSinceEpoch}.jpg',
      );
      await tempFile.writeAsBytes(currentMediaBytes.value!);

      if (!context.mounted) return;
      final editedFile = await context.push<File?>(
        '/photo-editor',
        extra: PhotoEditorArgs(file: tempFile, category: 'post'),
      );

      if (editedFile != null && context.mounted) {
        final editedBytes = await editedFile.readAsBytes();
        currentImagePreview.value = editedBytes;
        currentMediaBytes.value = editedBytes;
      }

      try {
        if (await tempFile.exists()) await tempFile.delete();
      } catch (_) {}
    }

    Future<void> submit() async {
      if (currentContentType.value != 'image' ||
          currentMediaBytes.value == null) {
        FloatingToast.error(
          context,
          '$kPostImageRequiredTitle\n$kPostCreateImageRequiredHint',
        );
        return;
      }
      if (!formKey.currentState!.validate()) return;
      final sellerId = ref.read(sellerStoreIdProvider);
      if (sellerId == null) {
        FloatingToast.error(context, '商家未登入，無法發文');
        return;
      }
      submitting.value = true;
      try {
        String? imageUrl;

        if (currentContentType.value == 'image' &&
            currentMediaBytes.value != null) {
          final ts = DateTime.now().millisecondsSinceEpoch;
          final (ext, mime) = ImageFormatDetector.detect(
            currentMediaBytes.value!,
          );
          imageUrl = await repo.uploadMedia(
            sellerId: sellerId,
            bytes: currentMediaBytes.value!,
            filename: 'image_$ts.$ext',
            mimeType: mime,
          );
          if (imageUrl == null) {
            if (context.mounted) FloatingToast.error(context, '圖片上傳失敗');
            return;
          }
        }

        // 建立貼文只送 product_id：關聯商品的 name / price / image 全部由後端
        // 自 product 帶出（`PostProductData`），前端組出來的那份從未被送出，
        // 2026-08-05 已移除。
        final selectedIndex = selectedProductIndex.value;
        final dynamic linkedProductId =
            (selectedIndex != null && selectedIndex < products.value.length)
            ? products.value[selectedIndex]['id']
            : null;

        final post = await repo.createPost(
          sellerId: sellerId,
          title: titleCtrl.text.trim(),
          content: titleCtrl.text.trim(),
          contentType: currentContentType.value,
          imageUrl: imageUrl,
          linkedProductId: linkedProductId,
          tags: PostHashtagExtractor.extractFromText(titleCtrl.text),
        );

        if (!context.mounted) return;
        if (post != null) {
          ref.invalidate(postListProvider);
          context.pop(
            post.toJson(),
          ); // return Map just in case caller expects Map
        } else {
          FloatingToast.error(context, '發文失敗');
        }
      } finally {
        if (context.mounted) submitting.value = false;
      }
    }

    // UI Builders
    Widget buildEmptyProductsState() {
      return Column(
        children: [
          SizedBox(height: 16.h),
          Icon(
            AppIcons.shoppingBagOutlined,
            size: 48.sp,
            color: AppIconColors.muted,
          ),
          SizedBox(height: 12.h),
          Text(
            '暫無上架商品',
            style: TextStyle(
              fontSize: 16.sp,
              fontWeight: FontWeight.w400,
              color: AppColors.textSecondary,
              height: 1.50,
            ),
          ),
          SizedBox(height: 16.h),
        ],
      );
    }

    Widget buildProductItem(
      Map<String, dynamic> product,
      int index,
      bool isSelected,
    ) {
      final name =
          product['name'] ??
          product['skuName'] ??
          product['productName'] ??
          '未命名商品';
      // PayUni #758: 取 salePrice ?? price 作為主價，原價（hasSale）顯示刪除線
      final rawPrice = product['price'] ?? product['priceMin'];
      final rawSalePrice = product['salePrice'] ?? product['sale_price'];
      final eff = PriceDisplay.effectiveSkuPrice(
        price: rawPrice,
        salePrice: rawSalePrice,
      );
      final price = eff?.toInt() ?? rawPrice ?? 0;
      final hasSale = PriceDisplay.hasSkuSale(
        price: rawPrice,
        salePrice: rawSalePrice,
      );
      final original = PriceDisplay.parseMoney(rawPrice);
      final unit = product['unit'] ?? '組';
      // 商品圖欄位判定統一走 product_image_resolver（`image` / `pic` 不是後端會
      // 回的欄位，2026-08-05 實打確認）。
      final productImageUrl = extractPrimaryProductImageUrl(product);

      return GestureDetector(
        onTap: () {
          if (selectedProductIndex.value == index) {
            selectedProductIndex.value = null;
          } else {
            selectedProductIndex.value = index;
          }
        },
        child: Container(
          padding: EdgeInsets.symmetric(horizontal: 12.w, vertical: 12.h),
          decoration: BoxDecoration(
            color: isSelected ? AppColors.bgPrimary : AppColors.bgCard,
            borderRadius: BorderRadius.circular(12.r),
            border: Border.all(
              color: isSelected ? AppColors.primary : AppColors.gray200,
              width: isSelected ? 2 : 1,
            ),
          ),
          child: Row(
            children: [
              Container(
                width: 24.w,
                height: 24.h,
                decoration: BoxDecoration(
                  shape: BoxShape.circle,
                  color: isSelected ? AppColors.primary : AppColors.bgCard,
                  border: Border.all(
                    color: isSelected
                        ? AppColors.primary
                        : AppColors.textSecondary,
                    width: 2.w,
                  ),
                ),
                child: isSelected
                    ? Icon(
                        AppIcons.check,
                        size: 16.sp,
                        color: AppIconColors.onBrand,
                      )
                    : null,
              ),
              SizedBox(width: 12.w),
              ClipRRect(
                borderRadius: BorderRadius.circular(8.r),
                child: Container(
                  width: 56.w,
                  height: 56.h,
                  color: AppColors.gray200,
                  child: productImageUrl != null
                      ? OptimizedNetworkImage(
                          imageUrl: productImageUrl,
                          scene: ImageScene.productThumbnail,
                          width: 56.w,
                          height: 56.h,
                          fit: BoxFit.cover,
                          errorWidget: Icon(
                            AppIcons.image,
                            color: AppIconColors.muted,
                          ),
                        )
                      : Icon(AppIcons.image, color: AppIconColors.muted),
                ),
              ),
              SizedBox(width: 12.w),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      '$name ($unit)',
                      style: TextStyle(
                        fontSize: 16.sp,
                        fontWeight: FontWeight.w400,
                        color: AppColors.textPrimary,
                        height: 1.50,
                      ),
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                    ),
                    SizedBox(height: 4.h),
                    Wrap(
                      spacing: 6.w,
                      runSpacing: 2.h,
                      crossAxisAlignment: WrapCrossAlignment.end,
                      children: [
                        Text(
                          'NT\$ $price',
                          style: TextStyle(
                            fontSize: 16.sp,
                            fontWeight: FontWeight.w400,
                            color: AppColors.danger,
                            height: 1.50,
                          ),
                        ),
                        if (hasSale && original != null) ...[
                          Text(
                            'NT\$ ${original.toInt()}',
                            style: TextStyle(
                              fontSize: 12.sp,
                              color: AppColors.textSecondary,
                              decoration: TextDecoration.lineThrough,
                              decorationColor: AppColors.textSecondary,
                            ),
                          ),
                        ],
                      ],
                    ),
                  ],
                ),
              ),
            ],
          ),
        ),
      );
    }

    Widget buildProductsList() {
      return Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            '目前上架商品',
            style: TextStyle(
              fontSize: 20.sp,
              fontWeight: FontWeight.w700,
              color: AppColors.textEmphasis,
            ),
          ),
          SizedBox(height: 16.h),
          ListView.separated(
            shrinkWrap: true,
            physics: const NeverScrollableScrollPhysics(),
            itemCount: products.value.length,
            separatorBuilder: (_, __) => SizedBox(height: 8.h),
            itemBuilder: (context, index) {
              final product = products.value[index];
              return buildProductItem(
                product,
                index,
                selectedProductIndex.value == index,
              );
            },
          ),
        ],
      );
    }

    Widget buildProductsSection() {
      if (loadingProducts.value) {
        return Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Container(
              width: 100.w,
              height: 20.h,
              decoration: BoxDecoration(
                color: AppColors.gray200,
                borderRadius: BorderRadius.circular(6.r),
              ),
            ),
            SizedBox(height: 16.h),
            Container(
              height: 80.h,
              decoration: BoxDecoration(
                color: AppColors.gray100,
                borderRadius: BorderRadius.circular(12.r),
              ),
            ),
          ],
        );
      }
      if (products.value.isEmpty) return buildEmptyProductsState();
      return buildProductsList();
    }

    return Scaffold(
      backgroundColor: AppColors.bgPrimary,
      appBar: HkGlassAppBar(
        title: const Text('新增貼文'),
        actions: [
          TextButton(
            onPressed: submitting.value ? null : submit,
            child: submitting.value
                ? SizedBox(
                    height: 18.h,
                    width: 18.w,
                    child: const CircularProgressIndicator(
                      strokeWidth: 2,
                      color: AppColors.actionPrimary,
                    ),
                  )
                : const HkButtonLabel('儲存'),
          ),
        ],
      ),
      body: SafeArea(
        top: false,
        child: Form(
          key: formKey,
          child: Column(
            children: [
              Expanded(
                child: SingleChildScrollView(
                  child: Column(
                    children: [
                      SizedBox(height: 16.h),
                      Padding(
                        padding: EdgeInsets.symmetric(horizontal: 16.w),
                        child: Stack(
                          children: [
                            Container(
                              width: double.infinity,
                              decoration: BoxDecoration(
                                borderRadius: BorderRadius.circular(16.r),
                                color:
                                    Color.lerp(
                                      AppColors.bgPrimary,
                                      dominantColor.value ??
                                          AppColors.bgPrimary,
                                      0.45,
                                    ) ??
                                    AppColors.bgPrimary,
                              ),
                              clipBehavior: Clip.antiAlias,
                              child: AspectRatio(
                                aspectRatio: 4 / 3,
                                child: currentImagePreview.value != null
                                    ? Image.memory(
                                        currentImagePreview.value!,
                                        fit: BoxFit.contain,
                                      )
                                    : Center(
                                        child: Icon(
                                          AppIcons.image,
                                          size: 48.sp,
                                          color: AppIconColors.onBrand,
                                        ),
                                      ),
                              ),
                            ),
                            Positioned(
                              top: 12,
                              right: 12,
                              child: HkTapTarget(
                                semanticLabel: '編輯貼文圖片',
                                onTap: openImageEditor,
                                child: SizedBox(
                                  width: 36.w,
                                  height: 36.h,
                                  child: Icon(
                                    AppIcons.autoAwesomeOutlined,
                                    size: 24.w,
                                    color: AppIconColors.active,
                                  ),
                                ),
                              ),
                            ),
                            Positioned(
                              bottom: 12,
                              right: 12,
                              child: HkTapTarget(
                                semanticLabel: '更換貼文圖片',
                                onTap: pickNewImage,
                                child: SizedBox(
                                  width: 36.w,
                                  height: 36.h,
                                  child: Icon(
                                    AppIcons.photoLibraryOutlined,
                                    size: 24.w,
                                    color: AppIconColors.active,
                                  ),
                                ),
                              ),
                            ),
                          ],
                        ),
                      ),
                      SizedBox(height: 16.h),
                      Padding(
                        padding: EdgeInsets.symmetric(horizontal: 16.w),
                        child: Container(
                          width: double.infinity,
                          padding: EdgeInsets.all(20.w),
                          decoration: BoxDecoration(
                            color: AppColors.bgCard,
                            borderRadius: BorderRadius.circular(16.r),
                            boxShadow: [
                              BoxShadow(
                                color: AppColors.cardShadow,
                                blurRadius: 10,
                                offset: Offset(0, 2.h),
                              ),
                            ],
                          ),
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text(
                                '商品說明',
                                style: TextStyle(
                                  fontSize: 20.sp,
                                  fontWeight: FontWeight.w700,
                                  color: AppColors.textEmphasis,
                                ),
                              ),
                              SizedBox(height: 8.h),
                              Text(
                                '詳細描述商品特色、食用方法、注意事項等',
                                style: TextStyle(
                                  fontSize: 14.sp,
                                  color: AppColors.textSecondary,
                                ),
                              ),
                              SizedBox(height: 20.h),
                              Container(
                                decoration: BoxDecoration(
                                  color: AppColors.gray100,
                                  borderRadius: BorderRadius.circular(12.r),
                                ),
                                child: PostHashtagTextFormField(
                                  controller: titleCtrl,
                                  maxLines: 4,
                                  minLines: 4,
                                  decoration: InputDecoration(
                                    hintText: '開始撰寫商品說明...',
                                    hintStyle: TextStyle(
                                      color: AppColors.textSecondary,
                                      fontSize: 16.sp,
                                    ),
                                    border: InputBorder.none,
                                    contentPadding: EdgeInsets.all(16.w),
                                  ),
                                  style: TextStyle(
                                    fontSize: 16.sp,
                                    color: AppColors.textEmphasis,
                                    height: 1.5,
                                  ),
                                  validator: (v) =>
                                      (v == null || v.trim().isEmpty)
                                      ? '請輸入商品說明'
                                      : null,
                                ),
                              ),
                            ],
                          ),
                        ),
                      ),
                      SizedBox(height: 16.h),
                      Padding(
                        padding: EdgeInsets.symmetric(horizontal: 16.w),
                        child: Container(
                          width: double.infinity,
                          padding: EdgeInsets.all(20.w),
                          decoration: BoxDecoration(
                            color: AppColors.bgCard,
                            borderRadius: BorderRadius.circular(16.r),
                            boxShadow: [
                              BoxShadow(
                                color: AppColors.cardShadow,
                                blurRadius: 10,
                                offset: Offset(0, 2.h),
                              ),
                            ],
                          ),
                          child: buildProductsSection(),
                        ),
                      ),
                      SizedBox(height: 24.h),
                    ],
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
