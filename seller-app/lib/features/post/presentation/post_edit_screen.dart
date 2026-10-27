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

import 'post_controller.dart';
import 'widgets/post_hashtag_text_form_field.dart';
import '../../product/data/product_repository.dart';
import '../data/post_repository.dart';
import '../data/post_model.dart';
import '../data/post_hashtag_extractor.dart';
import '../../photo_editor/presentation/photo_editor_args.dart';
import '../../../config/image_config.dart';
import '../../../shared/utils/floating_toast.dart';
import '../../../shared/utils/image_format_detector.dart';
import '../../../shared/utils/price_display.dart';
import '../../../shared/widgets/optimized_image.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

class PostEditScreen extends HookConsumerWidget {
  final Post post;

  const PostEditScreen({super.key, required this.post});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final formKey = useMemoized(() => GlobalKey<FormState>());
    final contentCtrl = useTextEditingController(text: post.content);
    final submitting = useState(false);

    // 圖片狀態：新圖（bytes）；保留/清除由 post.imageKeys 衍生
    final newImageBytes = useState<Uint8List?>(null);
    final newStagedToken = useState<String?>(null);

    final loadingProducts = useState(true);
    final products = useState<List<Map<String, dynamic>>>([]);
    final selectedProductIndex = useState<int?>(null);

    final repo = ref.read(postRepositoryProvider);

    // 初始化：載入商品列表，並預選現有 linkedProduct
    useEffect(() {
      Future.microtask(() async {
        final sellerId = ref.read(sellerStoreIdProvider);
        if (sellerId == null) {
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
            // 預選現有關聯商品
            if (post.linkedProduct != null) {
              // `PostProductData` 只有 id / name / imageKey / imageUrl / price
              // （gateway/internal/handler/post.go）；skuId / productId 不存在。
              final existingId = post.linkedProduct!['id'];
              if (existingId != null) {
                final idx = list.indexWhere(
                  (p) => p['id']?.toString() == existingId.toString(),
                );
                if (idx != -1) selectedProductIndex.value = idx;
              }
            }
            loadingProducts.value = false;
          }
        } catch (e) {
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
      newImageBytes.value = bytes;
      newStagedToken.value = null; // 先清，submit 時上傳
    }

    Future<void> openImageEditor() async {
      final bytes = newImageBytes.value;
      if (bytes == null) {
        FloatingToast.error(context, '請先選擇圖片');
        return;
      }
      if (ImageFormatDetector.detect(bytes).$2 == 'image/gif') {
        FloatingToast.error(context, 'GIF 動圖暫不支援編輯，已保留動圖格式');
        return;
      }
      final tempDir = Directory.systemTemp;
      final tempFile = File(
        '${tempDir.path}/post_edit_${DateTime.now().millisecondsSinceEpoch}.jpg',
      );
      await tempFile.writeAsBytes(bytes);

      if (!context.mounted) return;
      final editedFile = await context.push<File?>(
        '/photo-editor',
        extra: PhotoEditorArgs(file: tempFile, category: 'post'),
      );

      if (editedFile != null && context.mounted) {
        newImageBytes.value = await editedFile.readAsBytes();
        newStagedToken.value = null;
      }
      try {
        if (await tempFile.exists()) await tempFile.delete();
      } catch (_) {}
    }

    Future<void> submit() async {
      if (!formKey.currentState!.validate()) return;
      final sellerId = ref.read(sellerStoreIdProvider);
      if (sellerId == null) {
        FloatingToast.error(context, '商家未登入，無法儲存');
        return;
      }
      submitting.value = true;
      try {
        String keepAssetId = '';
        String stagedToken = '';

        if (newImageBytes.value != null && newStagedToken.value == null) {
          // 使用者換了新圖，上傳取得 staged token
          final ts = DateTime.now().millisecondsSinceEpoch;
          final (ext, mime) = ImageFormatDetector.detect(newImageBytes.value!);
          final token = await repo.uploadMedia(
            sellerId: sellerId,
            bytes: newImageBytes.value!,
            filename: 'image_$ts.$ext',
            mimeType: mime,
          );
          if (token == null) {
            if (context.mounted) FloatingToast.error(context, '圖片上傳失敗');
            return;
          }
          stagedToken = token;
        } else if (newImageBytes.value == null && post.imageKeys.isNotEmpty) {
          // 沒有新圖且原本有圖 → 保留現有圖片。
          // 判斷來源必須是 [Post.imageKeys]（貼文圖的唯一契約欄位）：
          // 若改看某個單數顯示欄位，第二次編輯會誤判成「本來就沒圖」而讓
          // keepAssetId 與 newStagedToken 同時送空 → 後端刪圖（#430 A0）。
          keepAssetId = 'keep';
        }
        // newImageBytes == null && imageKeys 為空 → 兩者皆空 → 清除/無圖

        int? productId;
        if (selectedProductIndex.value != null &&
            selectedProductIndex.value! < products.value.length) {
          final p = products.value[selectedProductIndex.value!];
          productId = p['id'] as int?;
        }

        final updated = await ref
            .read(postListProvider.notifier)
            .updatePost(
              post.id,
              content: contentCtrl.text.trim(),
              keepAssetId: keepAssetId,
              newStagedToken: stagedToken,
              productId: productId,
              tags: PostHashtagExtractor.extractFromText(contentCtrl.text),
            );

        if (!context.mounted) return;
        if (updated != null) {
          FloatingToast.success(context, '貼文已更新');
          context.pop(true);
        } else {
          FloatingToast.error(context, '儲存失敗，請稍後再試');
        }
      } finally {
        if (context.mounted) submitting.value = false;
      }
    }

    Widget buildImageSection() {
      return Padding(
        padding: EdgeInsets.symmetric(horizontal: 24.w),
        child: Stack(
          children: [
            Container(
              width: double.infinity,
              height: 200.h,
              decoration: BoxDecoration(
                borderRadius: BorderRadius.circular(16.r),
                color: AppColors.gray100,
              ),
              clipBehavior: Clip.antiAlias,
              child: _buildImageContent(newImageBytes.value),
            ),
            // AI 編輯按鈕
            if (newImageBytes.value != null)
              Positioned(
                top: 16,
                right: 16,
                child: HkTapTarget(
                  semanticLabel: '編輯貼文圖片',
                  onTap: openImageEditor,
                  child: Container(
                    width: 36.w,
                    height: 36.h,
                    decoration: BoxDecoration(
                      color: AppColors.bgCard.withValues(alpha: 0.9),
                      shape: BoxShape.circle,
                    ),
                    child: Icon(
                      AppIcons.autoFixHigh,
                      size: 20.sp,
                      color: AppIconColors.secondary,
                    ),
                  ),
                ),
              ),
            // 更換圖片按鈕
            Positioned(
              bottom: 16,
              right: 16,
              child: HkTapTarget(
                semanticLabel: '更換貼文圖片',
                onTap: pickNewImage,
                child: Container(
                  width: 36.w,
                  height: 36.h,
                  decoration: BoxDecoration(
                    color: AppColors.bgCard.withValues(alpha: 0.9),
                    shape: BoxShape.circle,
                  ),
                  child: Icon(
                    AppIcons.permMediaOutlined,
                    size: 20.sp,
                    color: AppIconColors.defaultColor,
                  ),
                ),
              ),
            ),
          ],
        ),
      );
    }

    Widget buildContentSection() {
      return Padding(
        padding: EdgeInsets.symmetric(horizontal: 24.w),
        child: Container(
          width: double.infinity,
          decoration: BoxDecoration(
            color: AppColors.bgCard,
            borderRadius: BorderRadius.circular(16.r),
            border: Border.all(color: AppColors.cardBorder),
            boxShadow: [
              BoxShadow(
                color: AppColors.cardShadow,
                blurRadius: 2,
                offset: Offset(0, 1.h),
              ),
            ],
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              // Header
              Container(
                padding: EdgeInsets.fromLTRB(24.w, 16.h, 24.w, 16.h),
                decoration: const BoxDecoration(
                  border: Border(
                    bottom: BorderSide(color: AppColors.successSoft),
                  ),
                ),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      '貼文敘述',
                      style: TextStyle(
                        fontSize: 18.sp,
                        fontWeight: FontWeight.bold,
                        color: AppColors.textEmphasis,
                      ),
                    ),
                    SizedBox(height: 6.h),
                    Text(
                      '詳細描述商品特色、食用方法、注意事項等',
                      style: TextStyle(
                        fontSize: 14.sp,
                        color: AppColors.textSecondary,
                      ),
                    ),
                  ],
                ),
              ),
              // Text input
              Padding(
                padding: EdgeInsets.all(20.w),
                child: Container(
                  decoration: BoxDecoration(
                    color: AppColors.gray100,
                    borderRadius: BorderRadius.circular(12.r),
                  ),
                  child: PostHashtagTextFormField(
                    controller: contentCtrl,
                    maxLines: 4,
                    minLines: 3,
                    decoration: InputDecoration(
                      hintText: '開始撰寫商品說明...',
                      hintStyle: TextStyle(
                        color: AppColors.textPrimary.withValues(alpha: 0.5),
                        fontSize: 14.sp,
                      ),
                      border: InputBorder.none,
                      contentPadding: EdgeInsets.all(12.w),
                    ),
                    style: TextStyle(
                      fontSize: 14.sp,
                      color: AppColors.textPrimary,
                      height: 1.5,
                    ),
                    validator: (v) =>
                        (v == null || v.trim().isEmpty) ? '請輸入貼文內容' : null,
                  ),
                ),
              ),
            ],
          ),
        ),
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
      // PayUni #758: 主價 = salePrice ?? price；有特價就顯示原價刪除線
      final rawPrice = product['price'];
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
      // `SellerProductItem` 的圖片欄位只有 imageKey / imageUrl（49 §6.6.1）。
      final productImageUrl = product['imageUrl'];
      return GestureDetector(
        onTap: () {
          selectedProductIndex.value = selectedProductIndex.value == index
              ? null
              : index;
        },
        child: Container(
          constraints: BoxConstraints(minHeight: 74.h),
          padding: EdgeInsets.symmetric(horizontal: 16.w, vertical: 4.h),
          decoration: BoxDecoration(
            color: AppColors.bgCard,
            borderRadius: BorderRadius.circular(16.r),
            border: Border.all(
              color: isSelected ? AppColors.primary : AppColors.cardBorder,
              width: isSelected ? 2 : 1,
            ),
            boxShadow: [
              BoxShadow(
                color: AppColors.cardShadow,
                blurRadius: 3,
                offset: Offset(0, 1.h),
              ),
            ],
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
                  width: 50.w,
                  height: 50.h,
                  color: AppColors.gray100,
                  child:
                      productImageUrl != null &&
                          productImageUrl.toString().isNotEmpty
                      ? OptimizedNetworkImage(
                          imageUrl: productImageUrl.toString(),
                          scene: ImageScene.productThumbnail,
                          width: 50.w,
                          height: 50.h,
                          fit: BoxFit.cover,
                          errorWidget: const Icon(
                            AppIcons.image,
                            color: AppIconColors.muted,
                          ),
                        )
                      : const Icon(AppIcons.image, color: AppIconColors.muted),
                ),
              ),
              SizedBox(width: 12.w),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  mainAxisAlignment: MainAxisAlignment.center,
                  children: [
                    Text(
                      name,
                      style: TextStyle(
                        fontSize: 14.sp,
                        fontWeight: FontWeight.bold,
                        color: AppColors.textEmphasis,
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
                            fontWeight: FontWeight.bold,
                            color: AppColors.warning,
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
              height: 74.h,
              decoration: BoxDecoration(
                color: AppColors.gray100,
                borderRadius: BorderRadius.circular(16.r),
              ),
            ),
          ],
        );
      }
      if (products.value.isEmpty) {
        return Column(
          children: [
            Icon(
              AppIcons.shoppingBagOutlined,
              size: 36.sp,
              color: AppIconColors.muted,
            ),
            SizedBox(height: 6.h),
            Text(
              '暫無推薦商家',
              style: TextStyle(fontSize: 14.sp, color: AppColors.textSecondary),
            ),
          ],
        );
      }
      return Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            '目前上架商品',
            style: TextStyle(
              fontSize: 20.sp,
              fontWeight: FontWeight.bold,
              color: AppColors.textEmphasis,
            ),
          ),
          SizedBox(height: 16.h),
          ListView.separated(
            shrinkWrap: true,
            physics: const NeverScrollableScrollPhysics(),
            itemCount: products.value.length,
            separatorBuilder: (_, __) => SizedBox(height: 8.h),
            itemBuilder: (_, i) => buildProductItem(
              products.value[i],
              i,
              selectedProductIndex.value == i,
            ),
          ),
        ],
      );
    }

    return Scaffold(
      backgroundColor: AppColors.bgPrimary,
      appBar: HkGlassAppBar(
        title: const Text('編輯貼文'),
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
          child: SingleChildScrollView(
            child: Column(
              children: [
                SizedBox(height: 16.h),
                buildImageSection(),
                SizedBox(height: 16.h),
                buildContentSection(),
                SizedBox(height: 16.h),
                Padding(
                  padding: EdgeInsets.symmetric(horizontal: 24.w),
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
                SizedBox(height: 32.h),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildImageContent(Uint8List? newBytes) {
    if (newBytes != null) {
      return Image.memory(newBytes, fit: BoxFit.cover, width: double.infinity);
    }
    if (post.imageKeys.isNotEmpty) {
      return OptimizedNetworkImage(
        imageUrl: post.imageKeys.first,
        scene: ImageScene.postImage,
        fit: BoxFit.cover,
        width: double.infinity,
        gaplessPlayback: true,
        placeholder: const Center(
          child: CircularProgressIndicator(strokeWidth: 2),
        ),
        errorWidget: Center(
          child: Icon(AppIcons.image, color: AppIconColors.muted, size: 48.sp),
        ),
      );
    }
    return Center(
      child: Icon(AppIcons.image, color: AppIconColors.muted, size: 48.sp),
    );
  }
}
