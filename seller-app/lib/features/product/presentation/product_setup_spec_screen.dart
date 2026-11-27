import '../../../features/seller/data/current_store.dart';
import 'package:hawkr_seller/shared/widgets/hk_button_label.dart';
import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:hawkr_seller/shared/widgets/hk_round_action_button.dart';
import 'dart:async';
import 'dart:typed_data';

import 'package:flutter/material.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../core/analytics/analytics_providers.dart';
import '../../../core/codemeta/code_meta.dart';
import '../../../shared/utils/floating_toast.dart';
import '../../../shared/widgets/app_toggle_switch.dart';
import '../../../shared/widgets/setup_bottom_bar.dart';
import '../../../shared/widgets/truck_loading_overlay.dart';
import '../data/product_repository.dart';
import '../data/product_upload_service.dart';
import 'product_setup_progress_bar.dart';
import 'product_setup_top_bar.dart';
import 'widgets/shipping_spec_card.dart';

/// 商品上架第 5 步：設定規格等級、價格、庫存、配送方式，完成後觸發後台上傳

/// 只留賣家已開通的運送方式；伺服器回哪些選項就列哪些，不依溫層過濾。
@visibleForTesting
List<Map<String, dynamic>> filterOnboardedShippingOptions(
  List<Map<String, dynamic>> options,
  Set<String> onboardedMethods,
) => options
    .where(
      (o) => onboardedMethods.contains((o['shippingMethod'] ?? '').toString()),
    )
    .toList();

class ProductSetupSpecScreen extends HookConsumerWidget {
  const ProductSetupSpecScreen({
    super.key,
    required this.images,
    this.skuId,
    required this.productName,
    required this.category,
    required this.subCategory,
    this.catalogId,
    this.isCandidateCategory = false,
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

  final List<Uint8List> images;
  final Object? skuId;
  final String productName;
  final String category;
  final String subCategory;
  final int? catalogId;
  final bool
  isCandidateCategory;
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
  final Map<String, dynamic>? existingProduct;

  static Widget _buildRequiredLabel(String label) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Text(
          '*',
          style: TextStyle(
            fontSize: 14.sp,
            fontWeight: FontWeight.w700,
            color: AppColors.dangerStrong,
          ),
        ),
        SizedBox(width: 4.w),
        Flexible(
          child: Text(
            label,
            style: TextStyle(
              fontSize: 14.sp,
              fontWeight: FontWeight.w700,
              color: AppColors.textEmphasis,
            ),
          ),
        ),
      ],
    );
  }

  static Widget _buildTextField({
    required TextEditingController controller,
    required String hintText,
    TextInputType keyboardType = TextInputType.text,
    required VoidCallback onChanged,
  }) {
    return Container(
      height: 44.h,
      decoration: BoxDecoration(
        border: Border.all(color: AppColors.dividerLight, width: 1),
        borderRadius: BorderRadius.circular(8.r),
      ),
      alignment: Alignment.center,
      child: TextField(
        controller: controller,
        keyboardType: keyboardType,
        textAlignVertical: TextAlignVertical.center,
        onChanged: (_) => onChanged(),
        decoration: InputDecoration(
          hintText: hintText,
          hintStyle: TextStyle(color: AppColors.gray300, fontSize: 14.sp),
          border: InputBorder.none,
          contentPadding: EdgeInsets.symmetric(horizontal: 12.w),
          isDense: true,
        ),
        style: TextStyle(fontSize: 14.sp, color: AppColors.textEmphasis),
      ),
    );
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final isLoadingExisting = useState(isEditMode);

    final specs = useState<List<_SpecData>>([_SpecData()]);

    final selectedShippingMethods = useState<Set<String>>({});
    final specFeePayers = useState<List<Map<String, String>>>([{}]);
    final specFreeThresholds = useState<List<Map<String, String>>>([{}]);
    final shippingOptions = useState<List<Map<String, dynamic>>>([]);
    final shippingLoading = useState(false);
    final shippingError = useState<String?>(null);

    final onboardedMethodKeys = useState<Set<String>>({});
    final onboardedFetched = useState(false);

    useEffect(() {
      Future<void> fetchOnboardedMethods() async {
        try {
          final methods = await ref
              .read(productRepositoryProvider)
              .getSellerShippingMethods();
          if (!context.mounted) return;
          final keys = <String>{};
          for (final m in methods) {
            if (m['available'] != true) continue;
            final method = (m['shippingMethod'] ?? '').toString();
            if (method.isEmpty) continue;
            keys.add(method);
          }
          onboardedMethodKeys.value = keys;
        } catch (_) {
        } finally {
          if (context.mounted) onboardedFetched.value = true;
        }
      }

      fetchOnboardedMethods();
      return null;
    }, const []);

    final debounceTimer = useRef<Timer?>(null);

    Future<void> fetchShippingQuotes(_SpecData spec) async {
      if (!context.mounted) return;

      final length = spec.lengthController.text.trim();
      final width = spec.widthController.text.trim();
      final height = spec.heightController.text.trim();
      final weight = spec.weightController.text.trim();

      if (length.isEmpty || width.isEmpty || height.isEmpty || weight.isEmpty) {
        if (!context.mounted) return;
        shippingOptions.value = [];
        shippingError.value = null;
        return;
      }

      if (!context.mounted) return;
      shippingLoading.value = true;
      shippingError.value = null;
      try {
        final resp = await ref
            .read(productRepositoryProvider)
            .getShippingQuotes(
              length: length,
              width: width,
              height: height,
              weight: weight,
              weightUnit: spec.unitCode,
            )
            .timeout(const Duration(seconds: 10));

        if (!context.mounted) return;

        if (resp['code'] == 4301) {
          shippingError.value = '所有配送方式都超過尺寸或重量限制';
          shippingOptions.value = [];
        } else if (resp['code'] == 0) {
          final data = resp['data'] as Map<String, dynamic>? ?? {};
          final options =
              (data['shippingOptions'] as List?)
                  ?.whereType<Map<String, dynamic>>()
                  .toList() ??
              [];
          shippingOptions.value = options;
        } else {
          shippingError.value = resp['message']?.toString() ?? '查詢失敗';
          shippingOptions.value = [];
        }
      } on TimeoutException {
        if (!context.mounted) return;
        shippingError.value = '載入逾時，請重試';
        shippingOptions.value = [];
      } catch (_) {
        if (!context.mounted) return;
        shippingError.value = '載入失敗';
        shippingOptions.value = [];
      }
      if (!context.mounted) return;
      shippingLoading.value = false;
    }

    void onDimensionOrWeightChanged(_SpecData spec) {
      debounceTimer.value?.cancel();
      debounceTimer.value = Timer(const Duration(milliseconds: 600), () {
        fetchShippingQuotes(spec);
      });
    }

    useEffect(() {
      return () => debounceTimer.value?.cancel();
    }, const []);

    final rebuild = useState(0);

    final pageController = usePageController();
    final currentSpecIndex = useState(0);

    void applyExistingProduct(Map<String, dynamic> p) {
      String mapUnit(String? unitStr) {
        return WeightUnit.fromValue(unitStr ?? '')?.value ??
            WeightUnit.kg.value;
      }

      final skusList = p['skus'];
      if (skusList is List && skusList.isNotEmpty) {
        final newSpecs = skusList.whereType<Map>().map((sku) {
          final spec = _SpecData();
          final rawSkuId = (sku['id'] ?? '').toString().trim();
          spec.skuId = rawSkuId.isEmpty ? null : rawSkuId;
          final rawSkuVersion = (sku['version'] ?? '').toString().trim();
          spec.skuVersion = rawSkuVersion.isEmpty ? null : rawSkuVersion;
          spec.gradeController.text = (sku['skuName'] ?? '').toString();
          spec.priceController.text = (sku['price'] ?? '').toString();
          final rawSalePrice = sku['salePrice'] ?? sku['sale_price'];
          final salePriceStr = (rawSalePrice ?? '').toString().trim();
          final salePriceNum = int.tryParse(salePriceStr) ?? 0;
          if (salePriceStr.isNotEmpty && salePriceNum > 0) {
            spec.salePriceController.text = salePriceNum.toString();
            spec.saleEnabled = true;
          }
          spec.stockController.text = (sku['stockQty'] ?? '').toString();
          spec.weightController.text = (sku['weight'] ?? '').toString();
          spec.lengthController.text = (sku['lengthCm'] ?? '').toString();
          spec.widthController.text = (sku['widthCm'] ?? '').toString();
          spec.heightController.text = (sku['heightCm'] ?? '').toString();
          spec.unitCode = mapUnit(sku['unit']?.toString());
          return spec;
        }).toList();
        if (newSpecs.isEmpty) newSpecs.add(_SpecData());
        specs.value = newSpecs;
      }

      final firstSpec = specs.value.first;
      if (firstSpec.lengthController.text.isNotEmpty &&
          firstSpec.widthController.text.isNotEmpty &&
          firstSpec.heightController.text.isNotEmpty &&
          firstSpec.weightController.text.isNotEmpty) {
        fetchShippingQuotes(firstSpec);
      }

      final existingShipping = p['shippingMethods'];
      if (existingShipping is List && existingShipping.isNotEmpty) {
        final selectedKeys = <String>{};
        final feePayerMap = <String, String>{};
        final thresholdMap = <String, String>{};
        for (final sm in existingShipping) {
          if (sm is Map) {
            final type = (sm['type'] ?? '').toString();
            if (type.isNotEmpty) {
              final key = type;
              selectedKeys.add(key);
              feePayerMap[key] =
                  (sm['feePayer'] ?? ShippingFeePayer.buyer.value).toString();
              thresholdMap[key] = (sm['freeThreshold'] ?? '0').toString();
            }
          }
        }
        selectedShippingMethods.value = selectedKeys;
        final count = specs.value.length.clamp(1, 99);
        specFreeThresholds.value = List.generate(
          count,
          (_) => Map<String, String>.from(thresholdMap),
        );
        specFeePayers.value = List.generate(
          count,
          (_) => Map<String, String>.from(feePayerMap),
        );
      }
    }

    Future<bool> reloadDetailFromServer() async {
      final p = existingProduct;
      final productIdVal =
          p?['id'] ?? p?['skuId'] ?? p?['sku_id'] ?? p?['_productId'] ?? skuId;
      if (productIdVal == null) return false;
      debounceTimer.value?.cancel();
      try {
        final detail = await ref
            .read(productRepositoryProvider)
            .getProductDetail(productIdVal);
        final data =
            detail['data'] as Map<String, dynamic>? ?? <String, dynamic>{};
        if (data.isEmpty) return false;
        applyExistingProduct(data);
        rebuild.value = rebuild.value + 1;
        return true;
      } catch (e) {
        debugPrint('[ProductSetupSpec] load detail failed: $e');
        return false;
      }
    }

    useEffect(() {
      if (!isEditMode || existingProduct == null) {
        isLoadingExisting.value = false;
        return null;
      }
      final p = existingProduct!;

      applyExistingProduct(p);

      final rawSkus = p['skus'];
      final hasSkus =
          rawSkus is List &&
          rawSkus.isNotEmpty &&
          rawSkus.every(
            (s) => s is Map && (s['id']?.toString().trim().isNotEmpty ?? false),
          );
      final hasShipping =
          (p['shippingMethods'] is List) &&
          (p['shippingMethods'] as List).isNotEmpty;
      if (!hasSkus || !hasShipping) {
        Future.microtask(() async {
          try {
            await reloadDetailFromServer();
          } finally {
            if (context.mounted) isLoadingExisting.value = false;
          }
        });
      } else {
        isLoadingExisting.value = false;
      }
      return null;
    }, const []);

    bool isFormValid() {
      final hasValidSpec = specs.value.any((spec) => spec.isValid);
      return hasValidSpec && selectedShippingMethods.value.isNotEmpty;
    }

    void addSpec() {
      final newIdx = specs.value.length;
      specs.value = [...specs.value, _SpecData()];
      specFreeThresholds.value = [...specFreeThresholds.value, {}];
      specFeePayers.value = [...specFeePayers.value, {}];
      WidgetsBinding.instance.addPostFrameCallback((_) {
        pageController.animateToPage(
          newIdx,
          duration: const Duration(milliseconds: 300),
          curve: Curves.easeInOut,
        );
      });
    }

    void showShippingInfoDialog() {
      showDialog(
        context: context,
        builder: (context) => Dialog(
          backgroundColor: Colors.transparent,
          child: Container(
            width: 340.w,
            constraints: BoxConstraints(
              maxHeight: MediaQuery.of(context).size.height * 0.8,
            ),
            child: SingleChildScrollView(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Container(
                    width: double.infinity,
                    decoration: BoxDecoration(
                      color: AppColors.bgCard,
                      borderRadius: BorderRadius.circular(16.r),
                    ),
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Container(
                          width: double.infinity,
                          padding: EdgeInsets.symmetric(
                            horizontal: 16.w,
                            vertical: 12.h,
                          ),
                          decoration: BoxDecoration(
                            color: AppColors.brandPrimaryStrong,
                            borderRadius: BorderRadius.only(
                              topLeft: Radius.circular(16.r),
                              topRight: Radius.circular(16.r),
                            ),
                          ),
                          child: Row(
                            children: [
                              const _CarrierMark('7-ELEVEN'),
                              SizedBox(width: 12.w),
                              Column(
                                crossAxisAlignment: CrossAxisAlignment.start,
                                children: [
                                  Text(
                                    '7-11 店到店',
                                    style: TextStyle(
                                      fontSize: 16.sp,
                                      fontWeight: FontWeight.w700,
                                      color: AppColors.bgCard,
                                    ),
                                  ),
                                  Text(
                                    '便利商店取貨',
                                    style: TextStyle(
                                      fontSize: 12.sp,
                                      color: Colors.white70,
                                    ),
                                  ),
                                ],
                              ),
                            ],
                          ),
                        ),
                        Padding(
                          padding: EdgeInsets.fromLTRB(16.w, 16.h, 16.w, 8.h),
                          child: Column(
                            children: [
                              const ShippingSpecCard(
                                title: '常溫配送',
                                dotColor: AppColors.brandSecondary,
                                groups: [
                                  ShippingSpecGroup(
                                    icon: AppIcons.straightenOutlined,
                                    lines: [
                                      ShippingSpecLine('最長邊', '≤ 45 cm'),
                                      ShippingSpecLine('長 + 寬 + 高', '≤ 105 cm'),
                                    ],
                                  ),
                                  ShippingSpecGroup(
                                    icon: AppIcons.scaleOutlined,
                                    lines: [ShippingSpecLine('總重量', '≤ 5 kg')],
                                  ),
                                ],
                              ),
                              SizedBox(height: 12.h),
                              const ShippingSpecCard(
                                title: '冷凍配送',
                                dotColor: AppColors.info,
                                groups: [
                                  ShippingSpecGroup(
                                    icon: AppIcons.straightenOutlined,
                                    heading: '冷凍交貨便專用箱',
                                    options: ['60 cm', '80 cm', '90 cm'],
                                    note: '可依需求選購',
                                  ),
                                  ShippingSpecGroup(
                                    icon: AppIcons.scaleOutlined,
                                    lines: [ShippingSpecLine('總重量', '≤ 10 kg')],
                                  ),
                                ],
                              ),
                            ],
                          ),
                        ),
                      ],
                    ),
                  ),

                  SizedBox(height: 12.h),

                  Container(
                    width: double.infinity,
                    decoration: BoxDecoration(
                      color: AppColors.bgCard,
                      borderRadius: BorderRadius.circular(16.r),
                    ),
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Container(
                          width: double.infinity,
                          padding: EdgeInsets.symmetric(
                            horizontal: 16.w,
                            vertical: 12.h,
                          ),
                          decoration: BoxDecoration(
                            color: AppColors.brandPrimaryStrong,
                            borderRadius: BorderRadius.only(
                              topLeft: Radius.circular(16.r),
                              topRight: Radius.circular(16.r),
                            ),
                          ),
                          child: Row(
                            children: [
                              const _CarrierMark('宅急便'),
                              SizedBox(width: 12.w),
                              Column(
                                crossAxisAlignment: CrossAxisAlignment.start,
                                children: [
                                  Text(
                                    '黑貓宅急便',
                                    style: TextStyle(
                                      fontSize: 16.sp,
                                      fontWeight: FontWeight.w700,
                                      color: AppColors.bgCard,
                                    ),
                                  ),
                                  Text(
                                    '宅配到府',
                                    style: TextStyle(
                                      fontSize: 12.sp,
                                      color: Colors.white70,
                                    ),
                                  ),
                                ],
                              ),
                            ],
                          ),
                        ),
                        Padding(
                          padding: EdgeInsets.fromLTRB(16.w, 16.h, 16.w, 8.h),
                          child: Column(
                            children: [
                              const ShippingSpecCard(
                                title: '常溫配送',
                                dotColor: AppColors.brandSecondary,
                                groups: [
                                  ShippingSpecGroup(
                                    icon: AppIcons.straightenOutlined,
                                    lines: [
                                      ShippingSpecLine('長 + 寬 + 高', '≤ 150 cm'),
                                    ],
                                  ),
                                  ShippingSpecGroup(
                                    icon: AppIcons.scaleOutlined,
                                    lines: [ShippingSpecLine('總重量', '≤ 20 kg')],
                                  ),
                                ],
                              ),
                              SizedBox(height: 12.h),
                              const ShippingSpecCard(
                                title: '冷藏/冷凍配送',
                                dotColor: AppColors.info,
                                groups: [
                                  ShippingSpecGroup(
                                    icon: AppIcons.straightenOutlined,
                                    lines: [
                                      ShippingSpecLine('長 + 寬 + 高', '≤ 120 cm'),
                                    ],
                                  ),
                                  ShippingSpecGroup(
                                    icon: AppIcons.scaleOutlined,
                                    lines: [ShippingSpecLine('總重量', '≤ 20 kg')],
                                  ),
                                ],
                              ),
                            ],
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
      );
    }

    Future<void> onComplete() async {
      if (!isFormValid()) {
        FloatingToast.error(context, '請填寫所有必填欄位');
        return;
      }

      for (var i = 0; i < specs.value.length; i++) {
        final s = specs.value[i];
        if (!s.hasData) continue;
        final err = s.saleValidationError;
        if (err != null) {
          FloatingToast.error(
            context,
            specs.value.length > 1 ? '規格 ${i + 1}：$err' : err,
          );
          return;
        }
      }

      final sellerId = ref.read(sellerStoreIdProvider);
      if (sellerId == null) {
        FloatingToast.error(context, '缺少商家資訊');
        return;
      }

      final specEntries = specs.value
          .asMap()
          .entries
          .where((e) => e.value.isValid)
          .toList();
      if (specEntries.isEmpty) {
        FloatingToast.error(context, '沒有有效的規格資料');
        return;
      }

      final uploadData = ProductUploadData(
        images: images,
        skuId: skuId,
        productName: productName,
        category: category,
        subCategory: subCategory,
        catalogId: catalogId,
        isCandidateCategory: isCandidateCategory,
        l1Id: l1Id,
        l1Name: l1Name,
        l2Id: l2Id,
        l2Name: l2Name,
        l3Name: l3Name,
        origin: origin,
        description: description,
        descriptionBlocks: descriptionBlocks,
        descriptionStagedTokens: descriptionStagedTokens,
        isEditMode: isEditMode,
        existingProduct: existingProduct,
        sellerId: sellerId,
        specs: specEntries.map((entry) {
          final idx = entry.key;
          final s = entry.value;
          final thresholds = idx < specFreeThresholds.value.length
              ? specFreeThresholds.value[idx]
              : <String, String>{};
          final feePayers = idx < specFeePayers.value.length
              ? specFeePayers.value[idx]
              : <String, String>{};
          final specShippingMethods = selectedShippingMethods.value
              .map<Map<String, String>>(
                (key) => SpecData.shippingMethodJson(
                  type: key,
                  feePayer: feePayers[key] ?? ShippingFeePayer.buyer.value,
                  freeThreshold: thresholds[key] ?? '0',
                ),
              )
              .toList();
          return SpecData(
            skuId: s.skuId,
            skuVersion: s.skuVersion,
            grade: s.gradeController.text.trim(),
            price: int.tryParse(s.priceController.text.trim()) ?? 0,
            salePrice: s.parsedSalePrice,
            stock: int.tryParse(s.stockController.text.trim()) ?? 0,
            weight: double.tryParse(s.weightController.text.trim()),
            unit: s.unitCode,
            lengthCm: double.tryParse(s.lengthController.text.trim()),
            widthCm: double.tryParse(s.widthController.text.trim()),
            heightCm: double.tryParse(s.heightController.text.trim()),
            shippingMethods: specShippingMethods,
          );
        }).toList(),
      );

      showDialog<void>(
        context: context,
        barrierDismissible: false,
        builder: (_) => const Center(child: CircularProgressIndicator()),
      );

      ProductUploadTask task;
      try {
        task = await ref
            .read(productUploadServiceProvider)
            .submitUpload(
              uploadData,
              analytics: ref.read(analyticsEventsProvider),
            );
      } finally {
        if (context.mounted) {
          Navigator.of(context, rootNavigator: true).pop();
        }
      }

      if (!context.mounted) return;

      if (task.status == UploadStatus.failed &&
          task.errorCode == kVersionConflictBizCode) {
        final reload = await showDialog<bool>(
          context: context,
          barrierDismissible: false,
          builder: (dialogContext) => AlertDialog(
            title: const Text('商品已被更新'),
            content: Text(
              '${task.displayMessage ?? '這個商品在你編輯期間已被更新。'}'
              '\n\n重新載入會以伺服器上的最新內容覆蓋目前的編輯。',
            ),
            actions: [
              TextButton(
                onPressed: () => Navigator.of(dialogContext).pop(false),
                child: const HkButtonLabel('留在此頁'),
              ),
              TextButton(
                onPressed: () => Navigator.of(dialogContext).pop(true),
                child: const HkButtonLabel('重新載入'),
              ),
            ],
          ),
        );
        if (!context.mounted) return;
        if (reload == true) {
          isLoadingExisting.value = true;
          final ok = await reloadDetailFromServer();
          if (!context.mounted) return;
          isLoadingExisting.value = false;
          if (!ok) {
            FloatingToast.error(context, '重新載入失敗，請返回列表後再試');
          }
        }
        return;
      }

      if (task.status == UploadStatus.failed) {
        FloatingToast.error(
          context,
          task.displayMessage ?? (isEditMode ? '商品更新失敗' : '商品上架失敗'),
        );
        return;
      }

      FloatingToast.success(context, isEditMode ? '商品已更新' : '商品已上架');
      context.pop(true);
    }

    Widget buildSpecCard(_SpecData spec, int index) {
      return Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          if (specs.value.length > 1)
            Padding(
              padding: EdgeInsets.only(bottom: 12.h),
              child: Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  Text(
                    '規格 ${index + 1}',
                    style: TextStyle(
                      fontSize: 16.sp,
                      fontWeight: FontWeight.w700,
                      color: AppColors.primary,
                    ),
                  ),
                  GestureDetector(
                    onTap: () {
                      final newSpecs = [...specs.value];
                      newSpecs[index].dispose();
                      newSpecs.removeAt(index);
                      final newThresholds = [...specFreeThresholds.value]
                        ..removeAt(index);
                      final newFeePayers = [...specFeePayers.value]
                        ..removeAt(index);
                      if (newSpecs.isEmpty) {
                        newSpecs.add(_SpecData());
                        specFreeThresholds.value = [{}];
                        specFeePayers.value = [{}];
                      } else {
                        specFreeThresholds.value = newThresholds;
                        specFeePayers.value = newFeePayers;
                      }
                      specs.value = newSpecs;
                      final targetPage = (index - 1).clamp(
                        0,
                        newSpecs.length - 1,
                      );
                      WidgetsBinding.instance.addPostFrameCallback((_) {
                        if (pageController.hasClients) {
                          pageController.animateToPage(
                            targetPage,
                            duration: const Duration(milliseconds: 300),
                            curve: Curves.easeInOut,
                          );
                        }
                      });
                    },
                    child: Icon(
                      AppIcons.deleteOutline,
                      color: AppIconColors.danger,
                      size: 22.sp,
                    ),
                  ),
                ],
              ),
            ),
          Row(
            crossAxisAlignment: CrossAxisAlignment.end,
            children: [
              Expanded(
                flex: 5,
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    _buildRequiredLabel('規格等級'),
                    SizedBox(height: 6.h),
                    _buildTextField(
                      controller: spec.gradeController,
                      hintText: '填寫規格 ex:S',
                      onChanged: () => rebuild.value++,
                    ),
                  ],
                ),
              ),
              SizedBox(width: 8.w),
              Expanded(
                flex: 4,
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    _buildRequiredLabel('商品重量'),
                    SizedBox(height: 6.h),
                    _buildTextField(
                      controller: spec.weightController,
                      hintText: '填寫重量',
                      keyboardType: TextInputType.number,
                      onChanged: () {
                        rebuild.value++;
                        onDimensionOrWeightChanged(spec);
                      },
                    ),
                  ],
                ),
              ),
              SizedBox(width: 8.w),
              Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  _buildRequiredLabel('單位'),
                  SizedBox(height: 6.h),
                  _UnitDropdown(
                    value: spec.unitCode,
                    options: weightUnitOptions,
                    onChanged: (v) {
                      spec.unitCode = v;
                      rebuild.value++;
                      onDimensionOrWeightChanged(spec);
                    },
                  ),
                ],
              ),
            ],
          ),
          SizedBox(height: 12.h),
          Row(
            children: [
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    _buildRequiredLabel('價格 (NT\$)'),
                    SizedBox(height: 6.h),
                    _buildTextField(
                      controller: spec.priceController,
                      hintText: '填寫價格',
                      keyboardType: TextInputType.number,
                      onChanged: () => rebuild.value++,
                    ),
                  ],
                ),
              ),
              SizedBox(width: 12.w),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    _buildRequiredLabel('庫存數量'),
                    SizedBox(height: 6.h),
                    _buildTextField(
                      controller: spec.stockController,
                      hintText: '填寫庫存數量',
                      keyboardType: TextInputType.number,
                      onChanged: () => rebuild.value++,
                    ),
                  ],
                ),
              ),
            ],
          ),
          SizedBox(height: 12.h),
          _SalePriceSection(spec: spec, onChanged: () => rebuild.value++),
          SizedBox(height: 12.h),
          Row(
            children: [
              _buildRequiredLabel('包裹尺寸'),
              SizedBox(width: 8.w),
              Text(
                '材積計算',
                style: TextStyle(
                  fontSize: 14.sp,
                  fontWeight: FontWeight.w700,
                  color: AppColors.textEmphasis,
                ),
              ),
              SizedBox(width: 8.w),
              AnimatedOpacity(
                opacity: spec.volumeDisplay.isNotEmpty ? 1.0 : 0.0,
                duration: const Duration(milliseconds: 200),
                child: Container(
                  padding: EdgeInsets.symmetric(
                    horizontal: 12.w,
                    vertical: 4.h,
                  ),
                  decoration: BoxDecoration(
                    color: AppColors.warningSoft,
                    borderRadius: BorderRadius.circular(24.r),
                  ),
                  child: Text(
                    spec.volumeDisplay.isEmpty ? ' ' : spec.volumeDisplay,
                    style: TextStyle(
                      fontSize: 14.sp,
                      fontWeight: FontWeight.w700,
                      color: AppColors.warning,
                    ),
                  ),
                ),
              ),
            ],
          ),
          SizedBox(height: 6.h),
          Row(
            children: [
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      '長 (cm)',
                      style: TextStyle(
                        fontSize: 12.sp,
                        color: AppColors.textSecondary,
                      ),
                    ),
                    SizedBox(height: 4.h),
                    _buildTextField(
                      controller: spec.lengthController,
                      hintText: '',
                      keyboardType: TextInputType.number,
                      onChanged: () {
                        rebuild.value++;
                        onDimensionOrWeightChanged(spec);
                      },
                    ),
                  ],
                ),
              ),
              SizedBox(width: 12.w),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      '寬 (cm)',
                      style: TextStyle(
                        fontSize: 12.sp,
                        color: AppColors.textSecondary,
                      ),
                    ),
                    SizedBox(height: 4.h),
                    _buildTextField(
                      controller: spec.widthController,
                      hintText: '',
                      keyboardType: TextInputType.number,
                      onChanged: () {
                        rebuild.value++;
                        onDimensionOrWeightChanged(spec);
                      },
                    ),
                  ],
                ),
              ),
              SizedBox(width: 12.w),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      '高 (cm)',
                      style: TextStyle(
                        fontSize: 12.sp,
                        color: AppColors.textSecondary,
                      ),
                    ),
                    SizedBox(height: 4.h),
                    _buildTextField(
                      controller: spec.heightController,
                      hintText: '',
                      keyboardType: TextInputType.number,
                      onChanged: () {
                        rebuild.value++;
                        onDimensionOrWeightChanged(spec);
                      },
                    ),
                  ],
                ),
              ),
            ],
          ),
          SizedBox(height: 8.h),
          Row(
            children: [
              Text(
                '材積計算',
                style: TextStyle(
                  fontSize: 14.sp,
                  fontWeight: FontWeight.w700,
                  color: AppColors.textEmphasis,
                ),
              ),
              SizedBox(width: 8.w),
              AnimatedOpacity(
                opacity: spec.volumeDisplay.isNotEmpty ? 1.0 : 0.0,
                duration: const Duration(milliseconds: 200),
                child: Container(
                  width: 74.w,
                  height: 32.h,
                  decoration: BoxDecoration(
                    color: AppColors.primarySoft,
                    borderRadius: BorderRadius.circular(24.r),
                  ),
                  alignment: Alignment.center,
                  child: Text(
                    spec.volumeDisplay.isEmpty ? ' ' : spec.volumeDisplay,
                    style: TextStyle(
                      fontSize: 14.sp,
                      fontWeight: FontWeight.w700,
                      color: AppColors.textAccentOnTint,
                    ),
                  ),
                ),
              ),
            ],
          ),
        ],
      );
    }

    final formValid = isFormValid();
    final keyboardVisible = MediaQuery.viewInsetsOf(context).bottom > 0;

    return Scaffold(
      backgroundColor: AppColors.bgPrimary,
      body: Stack(
        children: [
          Column(
            children: [
              ProductSetupTopBar(
                title: isEditMode ? '編輯商品' : '新增商品',
                subtitle: '規格、價格與配送',
              ),

              if (!keyboardVisible)
                const ProductSetupProgressBar(currentStep: 4),

              Expanded(
                child: Column(
                  children: [
                    if (specs.value.length > 1)
                      Padding(
                        padding: EdgeInsets.only(top: 8.h, bottom: 4.h),
                        child: Row(
                          mainAxisAlignment: MainAxisAlignment.center,
                          children: List.generate(specs.value.length, (i) {
                            final isActive = i == currentSpecIndex.value;
                            return AnimatedContainer(
                              duration: const Duration(milliseconds: 200),
                              width: isActive ? 20.w : 8.w,
                              height: 8.h,
                              margin: EdgeInsets.symmetric(horizontal: 4.w),
                              decoration: BoxDecoration(
                                color: isActive
                                    ? AppColors.primary
                                    : AppColors.gray300,
                                borderRadius: BorderRadius.circular(6.r),
                              ),
                            );
                          }),
                        ),
                      ),
                    Expanded(
                      child: PageView.builder(
                        controller: pageController,
                        onPageChanged: (i) => currentSpecIndex.value = i,
                        itemCount: specs.value.length,
                        itemBuilder: (context, idx) {
                          final spec = specs.value[idx];
                          return Padding(
                            padding: EdgeInsets.fromLTRB(
                              16.w,
                              keyboardVisible ? 0 : 12.h,
                              16.w,
                              keyboardVisible ? 0 : 16.h,
                            ),
                            child: Container(
                              width: double.infinity,
                              decoration: BoxDecoration(
                                color: AppColors.bgCard,
                                borderRadius: BorderRadius.circular(16.r),
                                border: Border.all(
                                  color: AppColors.dividerLight,
                                  width: 1,
                                ),
                                boxShadow: [
                                  BoxShadow(
                                    color: AppColors.cardShadow,
                                    blurRadius: 8,
                                    offset: Offset(0, 4.h),
                                  ),
                                ],
                              ),
                              child: SingleChildScrollView(
                                child: Column(
                                  crossAxisAlignment: CrossAxisAlignment.start,
                                  children: [
                                    if (keyboardVisible)
                                      const ProductSetupProgressBar(
                                        currentStep: 4,
                                      ),
                                    Padding(
                                      padding: EdgeInsets.all(16.w),
                                      child: buildSpecCard(spec, idx),
                                    ),
                                    Container(
                                      height: 1,
                                      color: AppColors.gray100,
                                    ),
                                    Padding(
                                      padding: EdgeInsets.fromLTRB(
                                        16.w,
                                        16.h,
                                        16.w,
                                        16.h,
                                      ),
                                      child: Column(
                                        crossAxisAlignment:
                                            CrossAxisAlignment.start,
                                        children: [
                                          Row(
                                            children: [
                                              _buildRequiredLabel('運送方式'),
                                              const Spacer(),
                                              GestureDetector(
                                                onTap: showShippingInfoDialog,
                                                child: Icon(
                                                  AppIcons.infoOutline,
                                                  size: 22.sp,
                                                  color: const Color(
                                                    0xFFE0C44C,
                                                  ),
                                                ),
                                              ),
                                            ],
                                          ),
                                          SizedBox(height: 10.h),
                                          Builder(
                                            builder: (_) {
                                              if (shippingLoading.value) {
                                                return Padding(
                                                  padding: EdgeInsets.symmetric(
                                                    vertical: 16.h,
                                                  ),
                                                  child: Center(
                                                    child: SizedBox(
                                                      width: 20.w,
                                                      height: 20.w,
                                                      child:
                                                          const CircularProgressIndicator(
                                                            strokeWidth: 2,
                                                            color: AppColors
                                                                .primary,
                                                          ),
                                                    ),
                                                  ),
                                                );
                                              }
                                              if (shippingError.value != null) {
                                                return GestureDetector(
                                                  onTap: () =>
                                                      fetchShippingQuotes(spec),
                                                  child: Padding(
                                                    padding:
                                                        EdgeInsets.symmetric(
                                                          vertical: 8.h,
                                                        ),
                                                    child: Row(
                                                      children: [
                                                        Icon(
                                                          AppIcons.errorOutline,
                                                          size: 16.sp,
                                                          color: const Color(
                                                            0xFFE0C44C,
                                                          ),
                                                        ),
                                                        SizedBox(width: 4.w),
                                                        Flexible(
                                                          child: Text(
                                                            shippingError
                                                                .value!,
                                                            maxLines: 1,
                                                            overflow:
                                                                TextOverflow
                                                                    .ellipsis,
                                                            style: TextStyle(
                                                              fontSize: 14.sp,
                                                              color:
                                                                  const Color(
                                                                    0xFFE0C44C,
                                                                  ),
                                                            ),
                                                          ),
                                                        ),
                                                        SizedBox(width: 8.w),
                                                        Text(
                                                          '點擊重試',
                                                          style: TextStyle(
                                                            fontSize: 14.sp,
                                                            color: AppColors
                                                                .primary,
                                                            fontWeight:
                                                                FontWeight.w700,
                                                          ),
                                                        ),
                                                      ],
                                                    ),
                                                  ),
                                                );
                                              }
                                              final filteredOptions =
                                                  filterOnboardedShippingOptions(
                                                    shippingOptions.value,
                                                    onboardedMethodKeys.value,
                                                  );

                                              if (onboardedFetched.value &&
                                                  onboardedMethodKeys
                                                      .value
                                                      .isEmpty) {
                                                return Padding(
                                                  padding: EdgeInsets.symmetric(
                                                    vertical: 8.h,
                                                  ),
                                                  child: Text(
                                                    '尚未開通任何配送方式，請先至「物流設定」完成至少一種配送建店後再上架商品。',
                                                    style: TextStyle(
                                                      fontSize: 14.sp,
                                                      color: const Color(
                                                        0xFFE0C44C,
                                                      ),
                                                    ),
                                                  ),
                                                );
                                              }
                                              if (shippingOptions
                                                  .value
                                                  .isEmpty) {
                                                return Text(
                                                  '請先填寫包裝尺寸與重量',
                                                  style: TextStyle(
                                                    fontSize: 14.sp,
                                                    color: const Color(
                                                      0xFFC8C8C8,
                                                    ),
                                                  ),
                                                );
                                              }
                                              if (filteredOptions.isEmpty) {
                                                return Text(
                                                  '此尺寸下，您尚未開通對應的配送方式。可至「物流設定」開通對應的配送方式。',
                                                  style: TextStyle(
                                                    fontSize: 14.sp,
                                                    color: const Color(
                                                      0xFFE0C44C,
                                                    ),
                                                  ),
                                                );
                                              }
                                              return Column(
                                                crossAxisAlignment:
                                                    CrossAxisAlignment.start,
                                                children: [
                                                  Row(
                                                    children: [
                                                      Icon(
                                                        AppIcons.check,
                                                        size: 16.sp,
                                                        color: AppColors
                                                            .brandPrimaryDeep,
                                                      ),
                                                      SizedBox(width: 4.w),
                                                      Text(
                                                        '可使用的運送方式 (${filteredOptions.length})',
                                                        style: TextStyle(
                                                          fontSize: 14.sp,
                                                          color: AppColors
                                                              .brandPrimaryDeep,
                                                        ),
                                                      ),
                                                    ],
                                                  ),
                                                  SizedBox(height: 8.h),
                                                  ...filteredOptions.map((
                                                    option,
                                                  ) {
                                                    final method =
                                                        (option['shippingMethod'] ??
                                                                '')
                                                            .toString();
                                                    final key = method;
                                                    final name =
                                                        (option['label'] ??
                                                                _shippingTypeName(
                                                                  method,
                                                                ))
                                                            .toString();
                                                    final price =
                                                        option['fee'] ?? 0;
                                                    final days =
                                                        option['estimatedDays'] ??
                                                        2;
                                                    final isSelected =
                                                        selectedShippingMethods
                                                            .value
                                                            .contains(key);
                                                    return _ShippingMethodCard(
                                                      key: ValueKey(
                                                        '$idx|$key',
                                                      ),
                                                      name: name,
                                                      price: price,
                                                      days: days,
                                                      isSelected: isSelected,
                                                      onTap: () {
                                                        if (isSelected) {
                                                          selectedShippingMethods
                                                              .value = {
                                                            ...selectedShippingMethods
                                                                .value,
                                                          }..remove(key);
                                                        } else {
                                                          selectedShippingMethods
                                                              .value = {
                                                            ...selectedShippingMethods
                                                                .value,
                                                            key,
                                                          };
                                                        }
                                                      },
                                                      initialFreeThreshold:
                                                          idx <
                                                              specFreeThresholds
                                                                  .value
                                                                  .length
                                                          ? (specFreeThresholds
                                                                    .value[idx][key] ??
                                                                '')
                                                          : '',
                                                      isSellerPaying:
                                                          idx <
                                                              specFeePayers
                                                                  .value
                                                                  .length
                                                          ? (specFeePayers
                                                                    .value[idx][key] ==
                                                                ShippingFeePayer
                                                                    .seller
                                                                    .value)
                                                          : false,
                                                      onFreeThresholdChanged: (v) {
                                                        if (idx >=
                                                            specFreeThresholds
                                                                .value
                                                                .length) {
                                                          return;
                                                        }
                                                        final updated =
                                                            List<
                                                              Map<
                                                                String,
                                                                String
                                                              >
                                                            >.from(
                                                              specFreeThresholds
                                                                  .value,
                                                            );
                                                        updated[idx] = {
                                                          ...updated[idx],
                                                          key: v,
                                                        };
                                                        specFreeThresholds
                                                                .value =
                                                            updated;
                                                      },
                                                      onFeePayerChanged: (v) {
                                                        if (idx >=
                                                            specFeePayers
                                                                .value
                                                                .length) {
                                                          return;
                                                        }
                                                        final updated =
                                                            List<
                                                              Map<
                                                                String,
                                                                String
                                                              >
                                                            >.from(
                                                              specFeePayers
                                                                  .value,
                                                            );
                                                        updated[idx] = {
                                                          ...updated[idx],
                                                          key: v
                                                              ? ShippingFeePayer
                                                                    .seller
                                                                    .value
                                                              : ShippingFeePayer
                                                                    .buyer
                                                                    .value,
                                                        };
                                                        specFeePayers.value =
                                                            updated;
                                                      },
                                                    );
                                                  }),
                                                ],
                                              );
                                            },
                                          ),
                                        ],
                                      ),
                                    ),
                                    Container(
                                      height: 1,
                                      color: AppColors.gray100,
                                    ),
                                    Padding(
                                      padding: EdgeInsets.fromLTRB(
                                        16.w,
                                        12.h,
                                        12.w,
                                        12.h,
                                      ),
                                      child: Row(
                                        children: [
                                          Icon(
                                            AppIcons.sentimentSatisfiedAlt,
                                            size: 18.sp,
                                            color: AppIconColors.warning,
                                          ),
                                          SizedBox(width: 6.w),
                                          Expanded(
                                            child: Text(
                                              specs.value.length > 1
                                                  ? '可左右滑動切換規格'
                                                  : '可按右下角+新增規格',
                                              style: TextStyle(
                                                fontSize: 14.sp,
                                                color: AppColors.warning,
                                              ),
                                            ),
                                          ),
                                          SizedBox(width: 8.w),
                                          HkRoundActionButton(
                                            semanticLabel: '新增規格',
                                            tooltip: '新增規格',
                                            symbol: HkRoundActionSymbol.plus,
                                            onPressed: addSpec,
                                          ),
                                        ],
                                      ),
                                    ),
                                  ],
                                ),
                              ),
                            ),
                          );
                        },
                      ),
                    ),
                  ],
                ),
              ),

              SetupBottomBar(
                onBack: () => context.pop(),
                onNext: formValid ? onComplete : null,
                nextLabel: '完成且預覽',
              ),
            ],
          ),
          if (isLoadingExisting.value)
            const Positioned.fill(
              child: TruckLoadingOverlay(message: '載入商品資料中...'),
            ),
        ],
      ),
    );
  }
}

/// WeightUnit 後端 value → 中文顯示名稱
final weightUnitOptions = {
  WeightUnit.kg.value: WeightUnit.kg.label,
  WeightUnit.catty.value: WeightUnit.catty.label,
  WeightUnit.gram.value: WeightUnit.gram.label,
};

String _shippingTypeName(String type) {
  return ShippingMethod.fromValue(type)?.label ?? type;
}

/// 單一規格的資料結構（包含控制器）
class _SpecData {
  /// 既有 SKU 的後端 id；新增的規格為 null。
  ///
  /// 回填它才會讓存檔走後端的 UPDATE 分支。空著＝後端刪光舊 SKU 再插一批新 TSID，
  /// 每次編輯換一次 id（詳見 [SpecData.skuId] 的說明）。
  String? skuId;

  /// 讀取當下的 SKU 樂觀鎖版本，存檔時當 expectedVersion 送回。
  String? skuVersion;

  final TextEditingController gradeController;
  final TextEditingController weightController;
  final TextEditingController priceController;
  final TextEditingController salePriceController;
  final TextEditingController stockController;
  final TextEditingController lengthController;
  final TextEditingController widthController;
  final TextEditingController heightController;
  String unitCode;
  bool saleEnabled;

  _SpecData()
    : gradeController = TextEditingController(),
      weightController = TextEditingController(),
      priceController = TextEditingController(),
      salePriceController = TextEditingController(),
      stockController = TextEditingController(),
      lengthController = TextEditingController(),
      widthController = TextEditingController(),
      heightController = TextEditingController(),
      unitCode = WeightUnit.kg.value,
      saleEnabled = false;

  void dispose() {
    gradeController.dispose();
    weightController.dispose();
    priceController.dispose();
    salePriceController.dispose();
    stockController.dispose();
    lengthController.dispose();
    widthController.dispose();
    heightController.dispose();
  }

  bool get hasData {
    return gradeController.text.trim().isNotEmpty ||
        weightController.text.trim().isNotEmpty ||
        priceController.text.trim().isNotEmpty ||
        stockController.text.trim().isNotEmpty;
  }

  bool get isValid {
    if (gradeController.text.trim().isEmpty ||
        weightController.text.trim().isEmpty ||
        priceController.text.trim().isEmpty ||
        stockController.text.trim().isEmpty) {
      return false;
    }
    return saleValidationError == null;
  }

  /// 解析後的特價（無 toggle / 空白回 null）。
  int? get parsedSalePrice {
    if (!saleEnabled) return null;
    final s = salePriceController.text.trim();
    if (s.isEmpty) return null;
    return int.tryParse(s);
  }

  /// 特價驗證錯誤訊息（合法時為 null）。
  String? get saleValidationError {
    if (!saleEnabled) return null;
    final s = salePriceController.text.trim();
    if (s.isEmpty) return null;
    final sp = int.tryParse(s);
    if (sp == null || sp <= 0) return '特價需大於 0';
    final p = int.tryParse(priceController.text.trim());
    if (p == null) return null;
    if (sp >= p) return '特價需低於原價';
    return null;
  }

  String get volumeDisplay {
    final l = double.tryParse(lengthController.text) ?? 0;
    final w = double.tryParse(widthController.text) ?? 0;
    final h = double.tryParse(heightController.text) ?? 0;
    if (l > 0 && w > 0 && h > 0) {
      final volume = (l * w * h / 6000).toStringAsFixed(2);
      return '$volume 才積';
    }
    return '';
  }
}

/// 單位下拉選單 — 按照 Figma 設計，彈出客製化選單
class _UnitDropdown extends StatefulWidget {
  const _UnitDropdown({
    required this.value,
    required this.options,
    required this.onChanged,
  });

  final String value;
  final Map<String, String> options;
  final ValueChanged<String> onChanged;

  @override
  State<_UnitDropdown> createState() => _UnitDropdownState();
}

class _UnitDropdownState extends State<_UnitDropdown> {
  final LayerLink _layerLink = LayerLink();
  OverlayEntry? _overlay;

  void _showDropdown() {
    _overlay = OverlayEntry(
      builder: (_) => Stack(
        children: [
          Positioned.fill(
            child: GestureDetector(
              behavior: HitTestBehavior.translucent,
              onTap: _hideDropdown,
            ),
          ),
          CompositedTransformFollower(
            link: _layerLink,
            targetAnchor: Alignment.bottomLeft,
            followerAnchor: Alignment.topLeft,
            offset: Offset(0, 4.h),
            child: Material(color: Colors.transparent, child: _buildMenu()),
          ),
        ],
      ),
    );
    Overlay.of(context).insert(_overlay!);
  }

  void _hideDropdown() {
    _overlay?.remove();
    _overlay = null;
  }

  Widget _buildMenu() {
    final entries = widget.options.entries.toList();
    return Container(
      width: 78.w,
      decoration: BoxDecoration(
        color: AppColors.bgPrimary,
        border: Border.all(color: AppColors.cardBorder, width: 1),
        borderRadius: BorderRadius.circular(12.r),
        boxShadow: const [
          BoxShadow(
            color: AppColors.cardShadow,
            blurRadius: 2,
            offset: Offset(0, 1),
          ),
        ],
      ),
      padding: EdgeInsets.all(4.w),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: List.generate(entries.length, (i) {
          final e = entries[i];
          final isSelected = e.key == widget.value;
          final isLast = i == entries.length - 1;
          return Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              GestureDetector(
                onTap: () {
                  widget.onChanged(e.key);
                  _hideDropdown();
                },
                child: Container(
                  width: double.infinity,
                  height: 28.h,
                  decoration: BoxDecoration(
                    color: isSelected ? AppColors.primary : Colors.transparent,
                    borderRadius: BorderRadius.circular(999.r),
                  ),
                  alignment: Alignment.center,
                  child: Text(
                    e.value,
                    style: TextStyle(
                      fontSize: 14.sp,
                      color: isSelected
                          ? AppColors.bgCard
                          : AppColors.textPrimary,
                    ),
                  ),
                ),
              ),
              if (!isLast)
                Divider(height: 1, thickness: 0.5, color: AppColors.cardBorder),
            ],
          );
        }),
      ),
    );
  }

  @override
  void dispose() {
    _hideDropdown();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final label = widget.options[widget.value] ?? widget.value;
    return CompositedTransformTarget(
      link: _layerLink,
      child: GestureDetector(
        onTap: _showDropdown,
        child: Container(
          width: 78.w,
          height: 44.h,
          padding: EdgeInsets.symmetric(horizontal: 8.w),
          decoration: BoxDecoration(
            border: Border.all(color: AppColors.dividerLight),
            borderRadius: BorderRadius.circular(8.r),
          ),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              Text(
                label,
                style: TextStyle(
                  fontSize: 14.sp,
                  color: AppColors.textEmphasis,
                ),
              ),
              SizedBox(width: 2.w),
              Icon(
                AppIcons.arrowDropDown,
                size: 18.sp,
                color: AppIconColors.muted,
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _ShippingMethodCard extends StatefulWidget {
  const _ShippingMethodCard({
    super.key,
    required this.name,
    required this.price,
    required this.days,
    required this.isSelected,
    required this.onTap,
    required this.initialFreeThreshold,
    required this.isSellerPaying,
    required this.onFreeThresholdChanged,
    required this.onFeePayerChanged,
  });

  final String name;
  final dynamic price;
  final dynamic days;
  final bool isSelected;
  final VoidCallback onTap;
  final String initialFreeThreshold;
  final bool isSellerPaying;
  final ValueChanged<String> onFreeThresholdChanged;
  final ValueChanged<bool> onFeePayerChanged;

  @override
  State<_ShippingMethodCard> createState() => _ShippingMethodCardState();
}

class _ShippingMethodCardState extends State<_ShippingMethodCard> {
  late final TextEditingController _thresholdCtrl;

  @override
  void initState() {
    super.initState();
    _thresholdCtrl = TextEditingController(text: widget.initialFreeThreshold);
  }

  @override
  void dispose() {
    _thresholdCtrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: widget.onTap,
      child: Container(
        width: double.infinity,
        margin: EdgeInsets.only(bottom: 8.h),
        padding: EdgeInsets.symmetric(horizontal: 16.w, vertical: 12.h),
        decoration: BoxDecoration(
          color: widget.isSelected ? AppColors.primarySoft : AppColors.bgCard,
          border: Border.all(
            color: widget.isSelected
                ? AppColors.primarySoft
                : AppColors.cardBorder,
          ),
          borderRadius: BorderRadius.circular(12.r),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Container(
                  width: 16.w,
                  height: 16.w,
                  decoration: BoxDecoration(
                    color: widget.isSelected
                        ? AppColors.brandPrimaryStrong
                        : AppColors.bgCard,
                    border: Border.all(
                      color: AppColors.brandPrimaryStrong,
                      width: 1.4,
                    ),
                    borderRadius: BorderRadius.circular(6.r),
                  ),
                  child: widget.isSelected
                      ? Icon(
                          AppIcons.check,
                          size: 11.sp,
                          color: AppIconColors.onBrand,
                        )
                      : null,
                ),
                SizedBox(width: 8.w),
                Flexible(
                  child: Text(
                    widget.name,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: TextStyle(
                      fontSize: 14.sp,
                      fontWeight: FontWeight.w700,
                      color: AppColors.textPrimary,
                    ),
                  ),
                ),
                const Spacer(),
                Text(
                  'NT\$ ${widget.price}',
                  style: TextStyle(
                    fontSize: 14.sp,
                    fontWeight: FontWeight.w700,
                    color: widget.isSelected
                        ? AppColors.brandPrimaryStrong
                        : AppColors.textPrimary,
                  ),
                ),
              ],
            ),
            SizedBox(height: 6.h),
            Padding(
              padding: EdgeInsets.only(left: 24.w),
              child: Row(
                children: [
                  Icon(
                    AppIcons.calendarTodayOutlined,
                    size: 13.sp,
                    color: AppIconColors.defaultColor,
                  ),
                  SizedBox(width: 4.w),
                  Text(
                    '預計到貨天數：${widget.days}天',
                    style: TextStyle(
                      fontSize: 14.sp,
                      color: AppColors.textPrimary,
                    ),
                  ),
                ],
              ),
            ),
            SizedBox(height: 6.h),
            Padding(
              padding: EdgeInsets.only(left: 24.w),
              child: Row(
                children: [
                  Text(
                    '免運門檻',
                    style: TextStyle(
                      fontSize: 14.sp,
                      color: AppColors.textPrimary,
                    ),
                  ),
                  SizedBox(width: 6.w),
                  Container(
                    padding: EdgeInsets.symmetric(
                      horizontal: 8.w,
                      vertical: 4.h,
                    ),
                    decoration: BoxDecoration(
                      color: AppColors.bgCard,
                      borderRadius: BorderRadius.circular(6.r),
                    ),
                    child: SizedBox(
                      width: 58.w,
                      height: 20.h,
                      child: TextField(
                        controller: _thresholdCtrl,
                        keyboardType: TextInputType.number,
                        style: TextStyle(
                          fontSize: 14.sp,
                          fontWeight: FontWeight.w700,
                          color: AppColors.textPrimary,
                        ),
                        decoration: InputDecoration(
                          prefixText: 'NT\$',
                          prefixStyle: TextStyle(
                            fontSize: 14.sp,
                            fontWeight: FontWeight.w700,
                            color: AppColors.textPrimary,
                          ),
                          isDense: true,
                          border: InputBorder.none,
                          contentPadding: EdgeInsets.zero,
                        ),
                        onChanged: widget.onFreeThresholdChanged,
                      ),
                    ),
                  ),
                  SizedBox(width: 8.w),
                  Text(
                    '我會負擔運費',
                    style: TextStyle(
                      fontSize: 14.sp,
                      color: AppColors.textPrimary,
                    ),
                  ),
                  const Spacer(),
                  AppToggleSwitch(
                    value: widget.isSellerPaying,
                    onChanged: widget.onFeePayerChanged,
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

class _SalePriceSection extends StatefulWidget {
  const _SalePriceSection({required this.spec, required this.onChanged});

  final _SpecData spec;
  final VoidCallback onChanged;

  @override
  State<_SalePriceSection> createState() => _SalePriceSectionState();
}

class _SalePriceSectionState extends State<_SalePriceSection> {
  static const _accent = AppColors.warning;
  static const _accentBg = AppColors.warningSoft;
  static const _border = AppColors.dividerLight;
  static const _hint = AppColors.gray300;
  static const _text = AppColors.textEmphasis;
  static const _muted = AppColors.textSecondary;
  static const _green = AppColors.primary;

  Widget _priceField({
    required TextEditingController controller,
    required String hint,
    Color? textColor,
  }) {
    return Container(
      height: 44.h,
      decoration: BoxDecoration(
        color: AppColors.bgCard,
        border: Border.all(color: _border),
        borderRadius: BorderRadius.circular(8.r),
      ),
      alignment: Alignment.center,
      child: TextField(
        controller: controller,
        keyboardType: TextInputType.number,
        textAlignVertical: TextAlignVertical.center,
        onChanged: (_) {
          setState(() {});
          widget.onChanged();
        },
        decoration: InputDecoration(
          hintText: hint,
          hintStyle: TextStyle(fontSize: 14.sp, color: _hint),
          prefixText: '\$ ',
          prefixStyle: TextStyle(fontSize: 14.sp, color: textColor ?? _text),
          border: InputBorder.none,
          contentPadding: EdgeInsets.symmetric(horizontal: 12.w),
          isDense: true,
        ),
        style: TextStyle(fontSize: 14.sp, color: textColor ?? _text),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final spec = widget.spec;
    final price = int.tryParse(spec.priceController.text.trim()) ?? 0;
    final sp = int.tryParse(spec.salePriceController.text.trim()) ?? 0;
    final hasValidSale = spec.saleEnabled && price > 0 && sp > 0 && sp < price;
    final discountPct = hasValidSale ? ((price - sp) * 100 / price).round() : 0;
    final savings = hasValidSale ? (price - sp) : 0;
    final err = spec.saleValidationError;

    return Container(
      padding: EdgeInsets.fromLTRB(12.w, 12.h, 12.w, 12.h),
      decoration: BoxDecoration(
        color: spec.saleEnabled ? _accentBg : AppColors.bgElevated,
        border: Border.all(
          color: spec.saleEnabled ? _accent : _border,
          width: spec.saleEnabled ? 1.2 : 1,
        ),
        borderRadius: BorderRadius.circular(12.r),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      '啟用特價',
                      style: TextStyle(
                        fontSize: 14.sp,
                        fontWeight: FontWeight.w700,
                        color: _text,
                      ),
                    ),
                    SizedBox(height: 2.h),
                    Text(
                      '選填，留空即無特價',
                      style: TextStyle(fontSize: 12.sp, color: _muted),
                    ),
                  ],
                ),
              ),
              AppToggleSwitch(
                value: spec.saleEnabled,
                onChanged: (v) {
                  setState(() {
                    spec.saleEnabled = v;
                    if (!v) {
                      spec.salePriceController.clear();
                    }
                  });
                  widget.onChanged();
                },
              ),
            ],
          ),
          if (spec.saleEnabled) ...[
            SizedBox(height: 12.h),
            Row(
              children: [
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        '原價',
                        style: TextStyle(fontSize: 12.sp, color: _muted),
                      ),
                      SizedBox(height: 4.h),
                      _priceField(
                        controller: spec.priceController,
                        hint: '填寫原價',
                      ),
                    ],
                  ),
                ),
                SizedBox(width: 12.w),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        '特價',
                        style: TextStyle(
                          fontSize: 12.sp,
                          color: _accent,
                          fontWeight: FontWeight.w700,
                        ),
                      ),
                      SizedBox(height: 4.h),
                      _priceField(
                        controller: spec.salePriceController,
                        hint: '填寫特價',
                        textColor: _accent,
                      ),
                    ],
                  ),
                ),
              ],
            ),
            SizedBox(height: 10.h),
            if (hasValidSale)
              Row(
                children: [
                  Container(
                    padding: EdgeInsets.symmetric(
                      horizontal: 12.w,
                      vertical: 4.h,
                    ),
                    decoration: BoxDecoration(
                      color: AppColors.dangerSoft,
                      borderRadius: BorderRadius.circular(24.r),
                    ),
                    child: Text(
                      '- $discountPct %',
                      style: TextStyle(
                        fontSize: 14.sp,
                        fontWeight: FontWeight.w700,
                        color: _accent,
                      ),
                    ),
                  ),
                  SizedBox(width: 8.w),
                  Text(
                    '每件省下 NT\$ $savings',
                    style: TextStyle(
                      fontSize: 14.sp,
                      fontWeight: FontWeight.w700,
                      color: _green,
                    ),
                  ),
                ],
              )
            else if (err != null)
              Padding(
                padding: EdgeInsets.only(top: 4.h),
                child: Text(
                  err,
                  style: TextStyle(
                    fontSize: 12.sp,
                    color: AppColors.dangerStrong,
                  ),
                ),
              ),
          ],
        ],
      ),
    );
  }
}

class _CarrierMark extends StatelessWidget {
  const _CarrierMark(this.name);

  final String name;

  @override
  Widget build(BuildContext context) {
    return Container(
      constraints: BoxConstraints(minWidth: 45.w, minHeight: 45.w),
      padding: EdgeInsets.symmetric(horizontal: 8.w, vertical: 4.h),
      alignment: Alignment.center,
      decoration: BoxDecoration(
        color: AppColors.bgCard,
        borderRadius: BorderRadius.circular(12.r),
      ),
      child: Text(
        name,
        maxLines: 1,
        style: TextStyle(
          fontSize: 12.sp,
          fontWeight: FontWeight.w700,
          color: AppColors.brandPrimaryStrong,
        ),
      ),
    );
  }
}
