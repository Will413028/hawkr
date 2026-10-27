import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'dart:io';
import 'dart:typed_data';
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:palette_generator_plus/palette_generator_plus.dart';

import '../../../shared/utils/floating_toast.dart';
import '../../../shared/utils/product_image_resolver.dart';
import '../../../shared/widgets/hk_tap_target.dart';
import '../../../shared/widgets/setup_bottom_bar.dart';
import '../../photo_editor/presentation/photo_editor_args.dart';
import '../../media/presentation/protected_media_controller.dart';
import '../data/product_repository.dart';
import 'ai_assistant_button.dart';
import 'product_setup_progress_bar.dart';
import 'product_setup_top_bar.dart';

/// 商品上架第 3 步：上傳最多 10 張商品圖片，編輯模式下自動加載現有圖片
class ProductSetupImageScreen extends HookConsumerWidget {
  const ProductSetupImageScreen({
    super.key,
    this.skuId,
    required this.productName,
    required this.category,
    required this.subCategory,
    this.catalogId,
    this.isCandidateCategory = false,
    // 完整分類資訊
    this.l1Id,
    this.l1Name,
    this.l2Id,
    this.l2Name,
    this.l3Name,
    this.origin,
    this.description,
    this.descriptionBlocks,
    this.descriptionStagedTokens = const [],
    this.isEditMode = false,
    this.existingProduct,
  });

  final Object? skuId;
  final String productName;
  final String category;
  final String subCategory;
  final int? catalogId; // 分類 ID（可能是正式或暫時）
  final bool
  isCandidateCategory; // true=來自 category_candidate, false=來自 category
  // 完整分類資訊
  final int? l1Id;
  final String? l1Name;
  final int? l2Id;
  final String? l2Name;
  final String? l3Name;
  final String? origin;
  final String? description;
  final List<Map<String, dynamic>>? descriptionBlocks;
  final List<String> descriptionStagedTokens;
  final bool isEditMode;
  final Map<String, dynamic>? existingProduct; // 編輯模式時的現有商品資料

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final images = useState<List<Uint8List>>([]);
    final selectedIndex = useState(0);
    final isLoading = useState(false);
    // 每張圖片的主色（用於 4:3 背景填充）
    final dominantColors = useState<Map<int, Color>>({});

    Future<Color> getDominantColor(Uint8List bytes) async {
      try {
        final palette = await PaletteGenerator.fromImageProvider(
          MemoryImage(bytes),
          size: const Size(50, 50),
          maximumColorCount: 4,
          quantizer: const LegacyQuantizer(),
        );
        return palette.dominantColor?.color ?? AppColors.bgPrimary;
      } catch (_) {
        return AppColors.bgPrimary;
      }
    }

    Future<void> updateDominantColor(int index) async {
      if (index < 0 || index >= images.value.length) return;
      if (dominantColors.value.containsKey(index)) return;
      final color = await getDominantColor(images.value[index]);
      dominantColors.value = {...dominantColors.value, index: color};
    }

    // 將原始圖片渲染為 4:3 + 主色背景的合成圖
    Future<Uint8List> renderImage43(Uint8List bytes, Color bgColor) async {
      const int outWidth = 1200;
      const int outHeight = 900; // 4:3

      // 解碼原始圖片
      final codec = await ui.instantiateImageCodec(bytes);
      final frame = await codec.getNextFrame();
      final srcImage = frame.image;

      // 建立 4:3 畫布
      final recorder = ui.PictureRecorder();
      final canvas = Canvas(
        recorder,
        Rect.fromLTWH(0, 0, outWidth.toDouble(), outHeight.toDouble()),
      );

      // 填滿主色背景（模擬 alpha 0.45 疊在 page 背景上）
      final baseBg = AppColors.bgPrimary;
      final blended = Color.lerp(baseBg, bgColor, 0.45) ?? bgColor;
      canvas.drawRect(
        Rect.fromLTWH(0, 0, outWidth.toDouble(), outHeight.toDouble()),
        Paint()..color = blended,
      );

      // 計算 BoxFit.contain 位置
      final double srcW = srcImage.width.toDouble();
      final double srcH = srcImage.height.toDouble();
      final double scale = (outWidth / srcW)
          .clamp(0, outHeight / srcH)
          .toDouble();
      final double dstW = srcW * scale;
      final double dstH = srcH * scale;
      final double dx = (outWidth - dstW) / 2;
      final double dy = (outHeight - dstH) / 2;

      canvas.drawImageRect(
        srcImage,
        Rect.fromLTWH(0, 0, srcW, srcH),
        Rect.fromLTWH(dx, dy, dstW, dstH),
        Paint()..filterQuality = FilterQuality.high,
      );

      final picture = recorder.endRecording();
      final img = await picture.toImage(outWidth, outHeight);
      final byteData = await img.toByteData(format: ui.ImageByteFormat.png);
      srcImage.dispose();
      img.dispose();
      return byteData!.buffer.asUint8List();
    }

    Future<void> loadExistingImages() async {
      debugPrint(
        '[ProductSetupImage] loadExistingImages called, isEditMode=$isEditMode',
      );
      debugPrint(
        '[ProductSetupImage] existingProduct keys=${existingProduct?.keys.toList()}',
      );
      debugPrint('[ProductSetupImage] skuId=$skuId');

      // 優先用已載入的 _loadedImages（來自商品卡片）
      final preloadedUrls =
          (existingProduct?['_loadedImages'] as List?)
              ?.whereType<String>()
              .toList() ??
          const <String>[];
      debugPrint(
        '[ProductSetupImage] _loadedImages count=${preloadedUrls.length}',
      );

      final productIdVal =
          existingProduct?['id'] ??
          existingProduct?['skuId'] ??
          existingProduct?['sku_id'] ??
          skuId;
      debugPrint('[ProductSetupImage] productIdVal=$productIdVal');
      if (productIdVal == null && preloadedUrls.isEmpty) return;

      isLoading.value = true;

      try {
        List<String> urls = preloadedUrls;
        if (urls.isEmpty && productIdVal != null) {
          final detail = await ref
              .read(productRepositoryProvider)
              .getProductDetail(productIdVal);
          final data = detail['data'] as Map<String, dynamic>? ?? {};
          debugPrint(
            '[ProductSetupImage] product detail keys=${data.keys.toList()}',
          );
          urls = extractProductImageUrls(data);
          debugPrint(
            '[ProductSetupImage] extracted urls from detail=${urls.length}',
          );
        }

        if (urls.isEmpty) {
          debugPrint('[ProductSetupImage] No URLs found, bailing out');
          if (context.mounted) isLoading.value = false;
          return;
        }
        debugPrint(
          '[ProductSetupImage] downloading ${urls.length} images in parallel...',
        );

        // 平行下載所有圖片（保留原始順序）— Future.wait 比 for-await 快 N 倍
        final results = await Future.wait(
          urls.map((url) async {
            try {
              return await ref
                  .read(protectedMediaControllerProvider.notifier)
                  .download(url);
            } catch (e) {
              debugPrint('Failed to download image: $url, error: $e');
            }
            return null;
          }),
        );
        final downloadedImages = results.whereType<Uint8List>().toList(
          growable: false,
        );

        if (context.mounted) {
          images.value = [...images.value, ...downloadedImages];
          isLoading.value = false;
        }
      } catch (e) {
        debugPrint('Failed to load existing images: $e');
        if (context.mounted) {
          isLoading.value = false;
        }
      }
    }

    useEffect(() {
      if (isEditMode) {
        loadExistingImages();
      }
      return null;
    }, const []);

    Future<void> editCurrentImage() async {
      if (images.value.isEmpty) return;

      final currentBytes = images.value[selectedIndex.value];
      final tempFile = File(
        '${Directory.systemTemp.path}/edit_${DateTime.now().millisecondsSinceEpoch}.jpg',
      );
      await tempFile.writeAsBytes(currentBytes);

      if (!context.mounted) return;

      final editedFile = await context.push<File?>(
        '/photo-editor',
        extra: PhotoEditorArgs(file: tempFile, category: 'product'),
      );

      if (!context.mounted || editedFile == null) return;

      final editedBytes = await editedFile.readAsBytes();
      final newList = [...images.value];
      newList[selectedIndex.value] = editedBytes;
      images.value = newList;
    }

    Future<void> pickImage() async {
      if (images.value.length >= 10) {
        FloatingToast.error(context, '最多只能上傳 10 張照片');
        return;
      }

      final result = await context.push<Map<String, dynamic>>(
        '/product/media-picker',
      );

      if (!context.mounted || result == null) return;

      // 多選模式：接收 List<Uint8List>
      final pickedImages = result['images'] as List<Uint8List>?;
      if (pickedImages != null && pickedImages.isNotEmpty) {
        final remaining = 10 - images.value.length;
        final toAdd = pickedImages.take(remaining).toList();
        images.value = [...images.value, ...toAdd];
        selectedIndex.value = images.value.length - 1;
        // 清除主色快取（新圖片需要重新取色）
        dominantColors.value = {};
        if (pickedImages.length > remaining) {
          FloatingToast.show(context, '最多只能上傳 10 張，已新增 $remaining 張');
        }
      }
    }

    void removeImage(int index) {
      if (index < 0 || index >= images.value.length) return;
      final newList = [...images.value];
      newList.removeAt(index);
      images.value = newList;
      if (selectedIndex.value >= images.value.length) {
        selectedIndex.value = images.value.isEmpty
            ? 0
            : images.value.length - 1;
      }
    }

    Future<void> showDeleteConfirmDialog() async {
      final confirmed = await showDialog<bool>(
        context: context,
        barrierDismissible: false,
        builder: (context) => Dialog(
          backgroundColor: Colors.transparent,
          insetPadding: EdgeInsets.symmetric(horizontal: 40.w),
          child: Container(
            decoration: BoxDecoration(
              color: AppColors.dividerLight,
              borderRadius: BorderRadius.circular(24.r),
            ),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Container(
                  width: double.infinity,
                  padding: EdgeInsets.symmetric(
                    vertical: 24.h,
                    horizontal: 16.w,
                  ),
                  decoration: BoxDecoration(
                    color: AppColors.bgCard,
                    borderRadius: BorderRadius.vertical(
                      top: Radius.circular(24.r),
                    ),
                  ),
                  child: Text(
                    '請問是否要刪除此照片',
                    textAlign: TextAlign.center,
                    style: TextStyle(
                      fontSize: 18.sp,
                      color: AppColors.textEmphasis,
                      height: 1.6,
                    ),
                  ),
                ),
                const Divider(
                  height: 0,
                  thickness: 1,
                  color: AppColors.gray300,
                ),
                Material(
                  color: AppColors.bgCard,
                  borderRadius: BorderRadius.vertical(
                    bottom: Radius.circular(24.r),
                  ),
                  child: Row(
                    children: [
                      Expanded(
                        child: InkWell(
                          onTap: () => Navigator.of(context).pop(false),
                          child: SizedBox(
                            height: 54.h,
                            child: Center(
                              child: Text(
                                '取消',
                                style: TextStyle(
                                  fontSize: 20.sp,
                                  fontWeight: FontWeight.w700,
                                  color: AppColors.textEmphasis,
                                ),
                              ),
                            ),
                          ),
                        ),
                      ),
                      Container(
                        width: 1,
                        height: 54.h,
                        color: AppColors.gray300,
                      ),
                      Expanded(
                        child: InkWell(
                          borderRadius: BorderRadius.only(
                            bottomRight: Radius.circular(24.r),
                          ),
                          onTap: () => Navigator.of(context).pop(true),
                          child: SizedBox(
                            height: 54.h,
                            child: Center(
                              child: Text(
                                '刪除',
                                style: TextStyle(
                                  fontSize: 20.sp,
                                  fontWeight: FontWeight.w700,
                                  color: AppColors.textEmphasis,
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
            ),
          ),
        ),
      );

      if (confirmed == true && context.mounted) {
        removeImage(selectedIndex.value);
      }
    }

    Future<void> onNext() async {
      if (images.value.isEmpty) {
        FloatingToast.error(context, '請至少上傳一張商品照片');
        return;
      }

      // 渲染所有圖片為 4:3 + 主色背景的合成圖
      FloatingToast.show(context, '圖片處理中...');
      final List<Uint8List> renderedImages = [];
      for (int i = 0; i < images.value.length; i++) {
        final color =
            dominantColors.value[i] ?? await getDominantColor(images.value[i]);
        final rendered = await renderImage43(images.value[i], color);
        renderedImages.add(rendered);
      }
      if (!context.mounted) return;

      final result = await context.push(
        '/product/tags',
        extra: {
          'images': renderedImages,
          'skuId': skuId,
          'productName': productName,
          'category': category,
          'subCategory': subCategory,
          'catalogId': catalogId,
          'isCandidateCategory': isCandidateCategory,
          'l1Id': l1Id,
          'l1Name': l1Name,
          'l2Id': l2Id,
          'l2Name': l2Name,
          'l3Name': l3Name,
          'origin': origin,
          'description': description,
          'descriptionBlocks': descriptionBlocks,
          'descriptionStagedTokens': descriptionStagedTokens,
          'isEditMode': isEditMode,
          'existingProduct': existingProduct,
        },
      );
      if (result == true && context.mounted) {
        context.pop(true);
      }
    }

    Widget buildEmptyState() {
      return InkWell(
        onTap: pickImage,
        child: Container(
          width: 370.w,
          height: 278.h,
          decoration: BoxDecoration(
            color: AppColors.bgCard,
            border: Border.all(color: AppColors.cardBorder),
            borderRadius: BorderRadius.circular(16.r),
          ),
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              Container(
                width: 50.w,
                height: 50.w,
                decoration: BoxDecoration(
                  shape: BoxShape.circle,
                  border: Border.all(color: AppColors.primary, width: 2),
                ),
                child: Icon(
                  AppIcons.add,
                  size: 28.sp,
                  color: AppIconColors.secondary,
                ),
              ),
              SizedBox(height: 18.h),
              Text(
                '最少需提供一張照片，最多顯示10張照片',
                style: TextStyle(fontSize: 16.sp, color: AppColors.textPrimary),
              ),
            ],
          ),
        ),
      );
    }

    Widget buildMainImage() {
      final idx = selectedIndex.value;
      final bgColor = dominantColors.value[idx] ?? AppColors.bgPrimary;
      final blended = Color.lerp(AppColors.bgPrimary, bgColor, 0.45) ?? bgColor;

      // 載入選中圖片的主色
      updateDominantColor(idx);

      return AspectRatio(
        aspectRatio: 4 / 3,
        child: Container(
          decoration: BoxDecoration(
            color: blended,
            borderRadius: BorderRadius.circular(16.r),
          ),
          child: Stack(
            children: [
              // 4:3 圖片完整顯示（同最終上傳效果）
              ClipRRect(
                borderRadius: BorderRadius.circular(16.r),
                child: SizedBox.expand(
                  child: Image.memory(images.value[idx], fit: BoxFit.contain),
                ),
              ),
              // AI 編輯按鈕（右上）
              Positioned(
                right: 16,
                top: 16,
                child: HkTapTarget(
                  semanticLabel: '編輯圖片',
                  onTap: editCurrentImage,
                  child: const _OverlayGlyph(AppIcons.autoAwesomeOutlined),
                ),
              ),
              // 刪除按鈕（右下）
              Positioned(
                right: 16,
                bottom: 16,
                child: HkTapTarget(
                  semanticLabel: '刪除圖片',
                  onTap: showDeleteConfirmDialog,
                  child: const _OverlayGlyph(AppIcons.deleteOutline),
                ),
              ),
            ],
          ),
        ),
      );
    }

    Widget buildThumbnailItem(int index, {double? size}) {
      final isSelected = index == selectedIndex.value;
      final double sz = size ?? 60.w;
      return GestureDetector(
        onTap: () => selectedIndex.value = index,
        child: Container(
          width: sz,
          height: sz,
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(16.r),
            border: Border.all(
              color: isSelected ? AppColors.primary : AppColors.textPrimary,
              width: isSelected ? 3 : 1,
            ),
          ),
          child: Stack(
            children: [
              ClipRRect(
                borderRadius: BorderRadius.circular(isSelected ? 12.r : 14.r),
                child: Image.memory(
                  images.value[index],
                  width: double.infinity,
                  height: double.infinity,
                  fit: BoxFit.cover,
                ),
              ),
              if (index == 0)
                Positioned(
                  left: 5,
                  top: 5,
                  child: Container(
                    width: 36.w,
                    height: 18.h,
                    decoration: BoxDecoration(
                      color: AppColors.textPrimary,
                      borderRadius: BorderRadius.circular(999.r),
                      boxShadow: const [
                        BoxShadow(
                          color: AppColors.cardShadow,
                          blurRadius: 3,
                          offset: Offset(0, 1),
                        ),
                      ],
                    ),
                    child: Center(
                      child: Text(
                        '主圖',
                        style: TextStyle(
                          color: AppColors.bgCard,
                          fontSize: 12.sp,
                          fontWeight: FontWeight.w700,
                          letterSpacing: 1,
                        ),
                      ),
                    ),
                  ),
                ),
            ],
          ),
        ),
      );
    }

    Widget buildThumbnailWrap() {
      return LayoutBuilder(
        builder: (context, constraints) {
          const int crossCount = 5;
          final double gap = 8.w;
          final double itemSize =
              (constraints.maxWidth - gap * (crossCount - 1)) / crossCount;

          final addButton = GestureDetector(
            onTap: pickImage,
            child: Container(
              width: itemSize,
              height: itemSize,
              decoration: BoxDecoration(
                color: AppColors.bgCard,
                borderRadius: BorderRadius.circular(16.r),
                border: Border.all(color: AppColors.primary),
              ),
              child: Column(
                mainAxisAlignment: MainAxisAlignment.center,
                children: [
                  Icon(
                    AppIcons.add,
                    size: 24.sp,
                    color: AppIconColors.secondary,
                  ),
                  SizedBox(height: 4.h),
                  Text(
                    '新增圖片',
                    style: TextStyle(
                      fontSize: 12.sp,
                      fontWeight: FontWeight.w700,
                      color: AppColors.primary,
                      letterSpacing: 1,
                    ),
                  ),
                ],
              ),
            ),
          );

          return Wrap(
            spacing: gap,
            runSpacing: gap,
            children: [
              ...List.generate(
                images.value.length,
                (i) => buildThumbnailItem(i, size: itemSize),
              ),
              if (images.value.length < 10) addButton,
            ],
          );
        },
      );
    }

    Widget buildLoadingSkeleton() {
      return Column(
        children: [
          _ImageShimmer(
            child: Container(
              width: double.infinity,
              height: 200.h,
              decoration: BoxDecoration(
                color: AppColors.gray200,
                borderRadius: BorderRadius.circular(16.r),
              ),
            ),
          ),
          SizedBox(height: 16.h),
          LayoutBuilder(
            builder: (context, constraints) {
              const int crossCount = 5;
              final double gap = 8.w;
              final double itemSize =
                  (constraints.maxWidth - gap * (crossCount - 1)) / crossCount;
              return _ImageShimmer(
                child: Wrap(
                  spacing: gap,
                  runSpacing: gap,
                  children: List.generate(
                    crossCount,
                    (_) => Container(
                      width: itemSize,
                      height: itemSize,
                      decoration: BoxDecoration(
                        color: AppColors.gray200,
                        borderRadius: BorderRadius.circular(8.r),
                      ),
                    ),
                  ),
                ),
              );
            },
          ),
        ],
      );
    }

    return Scaffold(
      backgroundColor: AppColors.bgPrimary,
      body: Column(
        children: [
          ProductSetupTopBar(
            title: isEditMode ? '編輯商品' : '新增商品',
            subtitle: '商品圖片設定',
            trailing: kAiProductListingAssistantEnabled
                ? const AiAssistantButton()
                : null,
          ),

          const ProductSetupProgressBar(currentStep: 3),

          Expanded(
            child: SingleChildScrollView(
              child: Padding(
                padding: EdgeInsets.fromLTRB(16.w, 8.h, 16.w, 16.h),
                child: Column(
                  children: [
                    isLoading.value
                        ? buildLoadingSkeleton()
                        : images.value.isEmpty
                        ? buildEmptyState()
                        : buildMainImage(),
                    SizedBox(height: 16.h),
                    Container(
                      width: double.infinity,
                      padding: EdgeInsets.all(16.w),
                      decoration: BoxDecoration(
                        color: AppColors.bgCard,
                        borderRadius: BorderRadius.circular(16.r),
                        border: Border.all(color: AppColors.gray300, width: 1),
                        boxShadow: [
                          BoxShadow(
                            color: AppColors.cardShadow,
                            blurRadius: 8,
                            offset: Offset(0, 4.h),
                          ),
                        ],
                      ),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          if (images.value.isNotEmpty) ...[
                            buildThumbnailWrap(),
                            SizedBox(height: 16.h),
                          ],
                          // 小技巧提示
                          Row(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Icon(
                                AppIcons.sentimentVerySatisfied,
                                size: 24.sp,
                                color: AppIconColors.warning,
                              ),
                              SizedBox(width: 6.w),
                              Text(
                                '小技巧：',
                                style: TextStyle(
                                  fontSize: 18.sp,
                                  fontWeight: FontWeight.w700,
                                  color: AppColors.warning,
                                ),
                              ),
                            ],
                          ),
                          SizedBox(height: 6.h),
                          Text(
                            '點擊圖片右上角ICON或者圖片可進入 AI 編輯器，使用預設風格或自訂提示詞來美化你的商品照片！',
                            style: TextStyle(
                              fontSize: 16.sp,
                              height: 1.5,
                              color: AppColors.textPrimary,
                            ),
                          ),
                        ],
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ),

          SetupBottomBar(
            onBack: () => context.pop(),
            onNext: images.value.isEmpty ? null : onNext,
          ),
        ],
      ),
    );
  }
}

/// 閃爍動畫效果
class _ImageShimmer extends StatefulWidget {
  const _ImageShimmer({required this.child});
  final Widget child;

  @override
  State<_ImageShimmer> createState() => _ImageShimmerState();
}

class _ImageShimmerState extends State<_ImageShimmer>
    with SingleTickerProviderStateMixin {
  late final AnimationController _controller;

  @override
  void initState() {
    super.initState();
    _controller = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 1100),
    )..repeat();
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(
      builder: (context, constraints) {
        final double width = constraints.maxWidth.isFinite
            ? constraints.maxWidth
            : MediaQuery.of(context).size.width;
        final double shimmerWidth = width * 0.45;
        final double start = -shimmerWidth;
        final double end = width;

        return AnimatedBuilder(
          animation: _controller,
          builder: (_, __) {
            final double dx = start + (end - start) * _controller.value;

            return Stack(
              children: [
                widget.child,
                Positioned(
                  left: dx,
                  top: 0,
                  bottom: 0,
                  child: IgnorePointer(
                    child: Container(
                      width: shimmerWidth,
                      decoration: const BoxDecoration(
                        gradient: LinearGradient(
                          begin: Alignment.centerLeft,
                          end: Alignment.centerRight,
                          colors: [
                            Color(0x00FFFFFF),
                            Color(0xCCFFFFFF),
                            Color(0x00FFFFFF),
                          ],
                        ),
                      ),
                    ),
                  ),
                ),
              ],
            );
          },
        );
      },
    );
  }
}

class _OverlayGlyph extends StatelessWidget {
  const _OverlayGlyph(this.icon);

  final IconData icon;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: 36.w,
      height: 36.w,
      decoration: const BoxDecoration(
        shape: BoxShape.circle,
        color: AppColors.gray100,
      ),
      child: Icon(icon, size: 22.w, color: AppIconColors.active),
    );
  }
}
