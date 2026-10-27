import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../shared/utils/floating_toast.dart';
import '../../../shared/widgets/rich_text_editor_text_block.dart';
import '../../../shared/widgets/setup_bottom_bar.dart';
import '../data/hashtag_extractor.dart';
import '../data/product_repository.dart';
import 'ai_assistant_button.dart';
import 'product_listing_draft_controller.dart';
import 'product_setup_progress_bar.dart';
import 'product_setup_top_bar.dart';

/// 商品上架第 2 步：填寫商品名稱、分類、產地、富文本描述，必填欄位全填方可進入下一步
class ProductSetupInfoScreen extends HookConsumerWidget {
  const ProductSetupInfoScreen({
    super.key,
    this.skuId,
    this.initialProductName,
    this.initialOrigin,
    this.initialDescription,
    this.initialCategory,
    this.initialSubCategory,
    this.initialCatalogId,
    this.isCandidateCategory = false,
    // 完整分類資訊
    this.l1Id,
    this.l1Name,
    this.l2Id,
    this.l2Name,
    this.l3Name,
    this.isEditMode = false,
    this.existingProduct,
  });

  final Object? skuId;
  final String? initialProductName;
  final String? initialOrigin;
  final String? initialDescription;
  final String? initialCategory;
  final String? initialSubCategory;
  final int? initialCatalogId; // 分類 ID（可能是正式或暫時）
  final bool
  isCandidateCategory; // true=來自 category_candidate, false=來自 category
  // 完整分類資訊
  final int? l1Id;
  final String? l1Name;
  final int? l2Id;
  final String? l2Name;
  final String? l3Name;
  final bool isEditMode;
  final Map<String, dynamic>? existingProduct; // 編輯模式時的現有商品資料

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final nameController = useTextEditingController();
    final categoryController = useTextEditingController();
    final subCategoryController = useTextEditingController();
    final originController = useTextEditingController();
    final descriptionController = useTextEditingController();
    final descriptionBlocks = useState<List<Map<String, dynamic>>?>(null);
    final descriptionStagedTokens = useState<List<String>>([]);

    // 監聽必填欄位，讓 Widget 在文字變動時重建以更新按鈕狀態
    useListenable(nameController);
    useListenable(categoryController);
    useListenable(subCategoryController);
    useListenable(originController);

    final isFormValid =
        nameController.text.trim().isNotEmpty &&
        categoryController.text.trim().isNotEmpty &&
        subCategoryController.text.trim().isNotEmpty &&
        originController.text.trim().isNotEmpty;

    // 分類選擇相關
    final l2Categories = useState<List<String>>([]);
    final l3Categories = useState<List<String>>([]);
    final selectedL1 = useState<String?>(null);
    final l2ByL1 = useState<Map<String, List<String>>>({});
    final l3ByL2 = useState<Map<String, List<String>>>({});
    final l3NameToId = useState<Map<String, int>>({});

    // 下拉選單相關
    final categoryOverlay = useState<OverlayEntry?>(null);
    final subCategoryOverlay = useState<OverlayEntry?>(null);
    final categoryFieldKey = useMemoized(() => GlobalKey());
    final subCategoryFieldKey = useMemoized(() => GlobalKey());

    // 將 catalogId 反查 L1/L2/L3 並填入欄位
    void applyCatalogId(String catalogId) {
      // 分類樹尚未載入，靜默等待（useEffect 依賴 l3NameToId.value.length，載入後會重試）
      if (l3NameToId.value.isEmpty) return;

      void showError() {
        WidgetsBinding.instance.addPostFrameCallback((_) {
          if (context.mounted) FloatingToast.error(context, '分類對應失敗，請手動選擇');
        });
      }

      final candidateId = int.tryParse(catalogId);
      if (candidateId == null) {
        showError();
        return;
      }
      String? foundL3;
      for (final e in l3NameToId.value.entries) {
        if (e.value == candidateId) {
          foundL3 = e.key;
          break;
        }
      }
      if (foundL3 == null) {
        showError();
        return;
      }
      String? foundL2;
      for (final e in l3ByL2.value.entries) {
        if (e.value.contains(foundL3)) {
          foundL2 = e.key;
          break;
        }
      }
      if (foundL2 == null) {
        showError();
        return;
      }
      String? foundL1;
      for (final e in l2ByL1.value.entries) {
        if (e.value.contains(foundL2)) {
          foundL1 = e.key;
          break;
        }
      }
      if (foundL1 == null) {
        showError();
        return;
      }
      selectedL1.value = foundL1;
      l2Categories.value = l2ByL1.value[foundL1] ?? [];
      categoryController.text = foundL2;
      l3Categories.value = l3ByL2.value[foundL2] ?? [];
      subCategoryController.text = foundL3;
    }

    // 對話式 AI 助手累積的 pending draft：進入本頁時把空欄位套進 local controller。
    // 只有 controller 為空時才套，避免覆蓋使用者手動輸入。
    final pendingDraft = ref.watch(productListingDraftControllerProvider);
    useEffect(() {
      if (pendingDraft.name != null &&
          pendingDraft.name!.isNotEmpty &&
          nameController.text.isEmpty) {
        nameController.text = pendingDraft.name!;
      }
      if (pendingDraft.origin != null &&
          pendingDraft.origin!.isNotEmpty &&
          originController.text.isEmpty) {
        originController.text = pendingDraft.origin!;
      }
      if (pendingDraft.description != null &&
          pendingDraft.description!.isNotEmpty &&
          descriptionController.text.isEmpty) {
        descriptionController.text = pendingDraft.description!;
      }
      if (pendingDraft.categoryId != null &&
          pendingDraft.categoryId!.isNotEmpty &&
          categoryController.text.isEmpty) {
        applyCatalogId(pendingDraft.categoryId!);
      }
      return null;
    }, [pendingDraft.patchVersion, l3NameToId.value.length]);

    void dismissCategoryOverlay() {
      categoryOverlay.value?.remove();
      categoryOverlay.value = null;
    }

    void dismissSubCategoryOverlay() {
      subCategoryOverlay.value?.remove();
      subCategoryOverlay.value = null;
    }

    void updateL3Categories(String l2Name) {
      if (l3ByL2.value.containsKey(l2Name)) {
        l3Categories.value = l3ByL2.value[l2Name]!;
      } else {
        l3Categories.value = [];
      }
    }

    /// 解析 description 欄位（可能是純文字 String 或結構化 Map）
    void parseDescription(Map<String, dynamic> p) {
      final desc = p['description'] ?? p['productDescription'];
      if (desc == null) return;

      if (desc is Map) {
        final blocks = desc['blocks'];
        if (blocks is List) {
          descriptionBlocks.value = blocks
              .whereType<Map<String, dynamic>>()
              .toList();
          descriptionController.text = descriptionBlocks.value!
              .where((b) => b['type'] == 'text')
              // runs 優先、legacy content 相容——只認 content 在 inline-run
              // 格式下摘要欄會顯示成空（issue #1246）
              .map(HashtagExtractor.blockPlainText)
              .where((s) => s.isNotEmpty)
              .join('\n');
        } else {
          // Map 但沒 blocks：可能是 {text: "..."} 或空 Map {}
          // 不能用 desc.toString()，空 Map 會變成字面字串 "{}"
          final textField = desc['text']?.toString() ?? '';
          if (textField.isNotEmpty) {
            descriptionController.text = textField;
          }
          // 空 Map 就保持 controller 為空字串，不要汙染
        }
      } else if (desc is String) {
        // 過濾後端/舊資料殘留的 empty JSON dump
        final trimmed = desc.trim();
        if (trimmed.isNotEmpty &&
            trimmed != '{}' &&
            trimmed != '[]' &&
            trimmed != 'null') {
          descriptionController.text = trimmed;
        }
      }
    }

    Future<void> loadCategories() async {
      try {
        final categories = await ref
            .read(productRepositoryProvider)
            .fetchCategories();
        if (!context.mounted) return;

        final Map<String, List<String>> newL2ByL1 = {};
        final Map<String, List<String>> newL3ByL2 = {};
        final Map<int, String> categoryIdToName = {};
        final Map<String, int> newL3NameToId = {};

        final l1Cats = categories.where((c) {
          final level = c['cat_level'] ?? c['catLevel'];
          if (level != null) return level == 1;
          final parentId = c['parentId'] ?? 0;
          return parentId == 0;
        }).toList();

        for (final l1 in l1Cats) {
          final l1NameStr = l1['name']?.toString() ?? '';
          final l1IdVal = (l1['catId'] ?? l1['cat_id'] ?? l1['id']);
          if (l1NameStr.isEmpty || l1IdVal == null) continue;
          final l1IdInt = l1IdVal is int
              ? l1IdVal
              : int.tryParse(l1IdVal.toString()) ?? 0;
          if (l1IdInt > 0) categoryIdToName[l1IdInt] = l1NameStr;

          final l2List = <String>[];

          final children = l1['children'] ?? l1['sub_category'];
          if (children is List && children.isNotEmpty) {
            for (final l2 in children) {
              final l2NameStr = l2['name']?.toString() ?? '';
              final l2IdVal = l2['catId'] ?? l2['cat_id'] ?? l2['id'];
              if (l2NameStr.isNotEmpty) {
                l2List.add(l2NameStr);
                final l2IdInt = l2IdVal is int
                    ? l2IdVal
                    : int.tryParse(l2IdVal.toString()) ?? 0;
                if (l2IdInt > 0) categoryIdToName[l2IdInt] = l2NameStr;

                final l3List = <String>[];
                final l3Children = l2['children'] ?? l2['sub_category'];
                if (l3Children is List && l3Children.isNotEmpty) {
                  for (final l3 in l3Children) {
                    final l3NameStr = l3['name']?.toString() ?? '';
                    final l3IdVal = l3['catId'] ?? l3['cat_id'] ?? l3['id'];
                    if (l3NameStr.isNotEmpty) {
                      l3List.add(l3NameStr);
                      if (l3IdVal != null) {
                        newL3NameToId[l3NameStr] = l3IdVal is int
                            ? l3IdVal
                            : int.tryParse(l3IdVal.toString()) ?? 0;
                      }
                    }
                  }
                } else if (l2IdVal != null) {
                  for (final c in categories) {
                    final parentId = c['parentId'];
                    final level = c['cat_level'] ?? c['catLevel'] ?? 0;
                    if (parentId == l2IdVal && level == 3) {
                      final cName = c['name']?.toString() ?? '';
                      final cId = c['catId'] ?? c['cat_id'] ?? c['id'];
                      if (cName.isNotEmpty) {
                        l3List.add(cName);
                        if (cId != null) {
                          newL3NameToId[cName] = cId is int
                              ? cId
                              : int.tryParse(cId.toString()) ?? 0;
                        }
                      }
                    }
                  }
                }
                if (l3List.isNotEmpty) {
                  newL3ByL2[l2NameStr] = l3List;
                }
              }
            }
          } else {
            for (final c in categories) {
              final parentId = c['parentId'];
              final level = c['cat_level'] ?? c['catLevel'] ?? 0;
              if (parentId == l1IdVal && level == 2) {
                final cName = c['name']?.toString() ?? '';
                final cId = c['catId'] ?? c['cat_id'] ?? c['id'];
                if (cName.isNotEmpty) {
                  l2List.add(cName);

                  if (cId != null) {
                    final l3List = <String>[];
                    for (final l3c in categories) {
                      final l3ParentId = l3c['parentId'];
                      final l3Level = l3c['cat_level'] ?? l3c['catLevel'] ?? 0;
                      if (l3ParentId == cId && l3Level == 3) {
                        final l3NameStr = l3c['name']?.toString() ?? '';
                        final l3IdVal =
                            l3c['catId'] ?? l3c['cat_id'] ?? l3c['id'];
                        if (l3NameStr.isNotEmpty) {
                          l3List.add(l3NameStr);
                          if (l3IdVal != null) {
                            newL3NameToId[l3NameStr] = l3IdVal is int
                                ? l3IdVal
                                : int.tryParse(l3IdVal.toString()) ?? 0;
                          }
                        }
                      }
                    }
                    if (l3List.isNotEmpty) {
                      newL3ByL2[cName] = l3List;
                    }
                  }
                }
              }
            }
          }

          if (l2List.isNotEmpty) {
            newL2ByL1[l1NameStr] = l2List;
          }
        }

        l2ByL1.value = newL2ByL1;
        l3ByL2.value = newL3ByL2;
        l3NameToId.value = newL3NameToId;

        if (selectedL1.value != null &&
            newL2ByL1.containsKey(selectedL1.value)) {
          l2Categories.value = newL2ByL1[selectedL1.value]!;
        } else if (newL2ByL1.isNotEmpty) {
          selectedL1.value = newL2ByL1.keys.first;
          l2Categories.value = newL2ByL1[selectedL1.value]!;
        }

        final sel2 = categoryController.text;
        if (sel2.isNotEmpty && newL3ByL2.containsKey(sel2)) {
          l3Categories.value = newL3ByL2[sel2]!;
        }
      } catch (e) {
        debugPrint('[ProductInfoSetup] Failed to load categories: $e');
      }
    }

    useEffect(() {
      if (initialProductName != null) {
        nameController.text = initialProductName!;
      }
      if (initialOrigin != null && initialOrigin!.isNotEmpty) {
        originController.text = initialOrigin!;
      }
      if (initialDescription != null && initialDescription!.isNotEmpty) {
        descriptionController.text = initialDescription!;
      }
      if (initialCategory != null) {
        final parts = initialCategory!.split('_');
        if (parts.isNotEmpty) {
          selectedL1.value = parts[0];
          if (parts.length > 1) {
            categoryController.text = parts[1];
          }
        }
      }
      if (initialSubCategory != null) {
        subCategoryController.text = initialSubCategory!;
      }
      if (isEditMode && existingProduct != null) {
        final p = existingProduct!;
        if (nameController.text.isEmpty) {
          final productName =
              p['productName'] ?? p['name'] ?? p['skuName'] ?? '';
          if (productName.toString().isNotEmpty) {
            nameController.text = productName.toString();
          }
        }
        final l1 = p['categoryL1']?.toString();
        if (l1 != null && l1.isNotEmpty && selectedL1.value == null) {
          selectedL1.value = l1;
        }
        if (categoryController.text.isEmpty) {
          categoryController.text =
              (p['categoryL2'] ?? p['category'] ?? p['productCategory'] ?? '')
                  .toString();
        }
        if (subCategoryController.text.isEmpty) {
          subCategoryController.text =
              (p['categoryL3'] ??
                      p['subCategory'] ??
                      p['subProductCategory'] ??
                      '')
                  .toString();
        }
        originController.text = (p['origin'] ?? p['productOrigin'] ?? '')
            .toString();
        parseDescription(p);
      }
      loadCategories();
      return null;
    }, const []);

    void showCategoryDropdown() {
      dismissSubCategoryOverlay();

      if (categoryOverlay.value != null) {
        dismissCategoryOverlay();
        return;
      }

      final renderBox =
          categoryFieldKey.currentContext?.findRenderObject() as RenderBox?;
      if (renderBox == null) return;

      final position = renderBox.localToGlobal(Offset.zero);
      final size = renderBox.size;

      categoryOverlay.value = OverlayEntry(
        builder: (ctx) => _CategoryDropdownOverlay(
          position: position,
          fieldWidth: size.width,
          fieldHeight: size.height,
          categories: l2Categories.value,
          selectedCategory: categoryController.text,
          onSelect: (category) {
            categoryController.text = category;
            subCategoryController.text = '';
            updateL3Categories(category);
            dismissCategoryOverlay();
          },
          onDismiss: dismissCategoryOverlay,
          onAddNew: () {
            dismissCategoryOverlay();
            showAddCategoryDialog(
              context,
              categoryController,
              subCategoryController,
              l2Categories,
              l3Categories,
            );
          },
        ),
      );

      Overlay.of(context).insert(categoryOverlay.value!);
    }

    void showSubCategoryDropdown() {
      dismissCategoryOverlay();

      if (subCategoryOverlay.value != null) {
        dismissSubCategoryOverlay();
        return;
      }

      final renderBox =
          subCategoryFieldKey.currentContext?.findRenderObject() as RenderBox?;
      if (renderBox == null) return;

      final position = renderBox.localToGlobal(Offset.zero);
      final size = renderBox.size;

      subCategoryOverlay.value = OverlayEntry(
        builder: (ctx) => _CategoryDropdownOverlay(
          position: position,
          fieldWidth: size.width,
          fieldHeight: size.height,
          categories: l3Categories.value,
          selectedCategory: subCategoryController.text,
          onSelect: (category) {
            subCategoryController.text = category;
            dismissSubCategoryOverlay();
          },
          onDismiss: dismissSubCategoryOverlay,
          onAddNew: () {
            dismissSubCategoryOverlay();
            showAddSubCategoryDialog(
              context,
              subCategoryController,
              l3Categories,
            );
          },
        ),
      );

      Overlay.of(context).insert(subCategoryOverlay.value!);
    }

    void onDescriptionTap() {
      context
          .push<RichTextEditorResult>(
            '/rich-text-editor',
            extra: {
              'initialContent': descriptionBlocks.value != null
                  ? null
                  : descriptionController.text,
              'initialBlocks': descriptionBlocks.value,
              'maxLength': 3000,
            },
          )
          .then((result) {
            if (result != null && context.mounted) {
              descriptionController.text = result.content;
              descriptionBlocks.value = result.blocks;
              descriptionStagedTokens.value = result.stagedTokens;
            }
          });
    }

    Future<void> onNext() async {
      if (nameController.text.trim().isEmpty) {
        FloatingToast.error(context, '請輸入商品名稱');
        return;
      }
      if (categoryController.text.trim().isEmpty) {
        FloatingToast.error(context, '請輸入商品分類');
        return;
      }
      if (subCategoryController.text.trim().isEmpty) {
        FloatingToast.error(context, '請輸入詳細分類');
        return;
      }
      if (originController.text.trim().isEmpty) {
        FloatingToast.error(context, '請輸入產地');
        return;
      }

      final subCategoryName = subCategoryController.text.trim();
      int? catalogId = initialCatalogId ?? l3NameToId.value[subCategoryName];

      final bool isFallbackCategory = catalogId != null && catalogId < 0;
      const bool isCandidateCat = false;

      if (isFallbackCategory) {
        debugPrint(
          '[ProductInfoSetup] Using fallback category: $subCategoryName (id: $catalogId)',
        );
        catalogId = null;
      }

      final result = await context.push(
        '/product/images',
        extra: {
          'skuId': skuId,
          'productName': nameController.text.trim(),
          'category': categoryController.text.trim(),
          'subCategory': subCategoryName,
          'catalogId': catalogId,
          'isCandidateCategory': isCandidateCat,
          'l1Id': l1Id,
          'l1Name': l1Name ?? categoryController.text.trim(),
          'l2Id': l2Id,
          'l2Name': l2Name ?? subCategoryName,
          'l3Name': l3Name,
          'origin': originController.text.trim().isEmpty
              ? null
              : originController.text.trim(),
          'description': descriptionController.text.trim().isEmpty
              ? null
              : descriptionController.text.trim(),
          'descriptionBlocks': descriptionBlocks.value,
          'descriptionStagedTokens': descriptionStagedTokens.value,
          'isEditMode': isEditMode,
          'existingProduct': existingProduct,
        },
      );
      if (result == true && context.mounted) {
        context.pop(true);
      }
    }

    return Scaffold(
      backgroundColor: AppColors.bgPrimary,
      body: Column(
        children: [
          ProductSetupTopBar(
            title: isEditMode ? '編輯商品' : '新增商品',
            subtitle: '商品資訊設定',
            trailing: kAiProductListingAssistantEnabled
                ? const AiAssistantButton()
                : null,
          ),

          // 主要內容
          Expanded(
            child: SingleChildScrollView(
              child: Column(
                children: [
                  const ProductSetupProgressBar(currentStep: 2),
                  Padding(
                    padding: EdgeInsets.fromLTRB(16.w, 8.h, 16.w, 16.h),
                    child: Column(
                      children: [
                        // 商品資訊卡片
                        Container(
                          width: double.infinity,
                          decoration: BoxDecoration(
                            color: AppColors.bgCard,
                            borderRadius: BorderRadius.circular(16.r),
                            border: Border.all(
                              color: AppColors.gray300,
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
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Padding(
                                padding: EdgeInsets.fromLTRB(
                                  16.w,
                                  16.h,
                                  16.w,
                                  0,
                                ),
                                child: Row(
                                  crossAxisAlignment: CrossAxisAlignment.center,
                                  children: [
                                    Expanded(
                                      child: Column(
                                        crossAxisAlignment:
                                            CrossAxisAlignment.start,
                                        children: [
                                          Text(
                                            '商品資訊',
                                            style: TextStyle(
                                              fontSize: 18.sp,
                                              fontWeight: FontWeight.w700,
                                              color: AppColors.textEmphasis,
                                            ),
                                          ),
                                          SizedBox(height: 4.h),
                                          Text(
                                            '填寫商品的基本資料',
                                            style: TextStyle(
                                              fontSize: 12.sp,
                                              color: AppColors.textPrimary,
                                            ),
                                          ),
                                        ],
                                      ),
                                    ),
                                  ],
                                ),
                              ),
                              SizedBox(height: 8.h),
                              Container(
                                width: double.infinity,
                                height: 1,
                                color: AppColors.dividerLight,
                              ),
                              SizedBox(height: 16.h),
                              Padding(
                                padding: EdgeInsets.fromLTRB(
                                  16.w,
                                  0,
                                  16.w,
                                  16.h,
                                ),
                                child: Column(
                                  crossAxisAlignment: CrossAxisAlignment.start,
                                  children: [
                                    // 商品名稱
                                    _buildRequiredLabel('商品名稱'),
                                    SizedBox(height: 8.h),
                                    _buildTextField(
                                      controller: nameController,
                                      hintText: '例：紐西蘭奇異果',
                                    ),
                                    SizedBox(height: 16.h),
                                    // 商品分類 和 詳細分類
                                    Row(
                                      children: [
                                        Expanded(
                                          child: Column(
                                            crossAxisAlignment:
                                                CrossAxisAlignment.start,
                                            children: [
                                              _buildRequiredLabel('商品分類'),
                                              SizedBox(height: 8.h),
                                              _buildCategoryField(
                                                key: categoryFieldKey,
                                                controller: categoryController,
                                                hintText: '選擇分類',
                                                onTap: showCategoryDropdown,
                                              ),
                                            ],
                                          ),
                                        ),
                                        SizedBox(width: 12.w),
                                        Expanded(
                                          child: Column(
                                            crossAxisAlignment:
                                                CrossAxisAlignment.start,
                                            children: [
                                              _buildRequiredLabel('詳細分類'),
                                              SizedBox(height: 8.h),
                                              _buildCategoryField(
                                                key: subCategoryFieldKey,
                                                controller:
                                                    subCategoryController,
                                                hintText: '選擇分類',
                                                onTap: showSubCategoryDropdown,
                                              ),
                                            ],
                                          ),
                                        ),
                                      ],
                                    ),
                                    SizedBox(height: 16.h),
                                    // 產地
                                    _buildRequiredLabel('產地'),
                                    SizedBox(height: 8.h),
                                    _buildTextField(
                                      controller: originController,
                                      hintText: '例：台灣 / 紐西蘭',
                                    ),
                                  ],
                                ),
                              ),
                            ],
                          ),
                        ),
                        SizedBox(height: 16.h),
                        // 商品說明卡片（可點擊跳轉）
                        GestureDetector(
                          onTap: onDescriptionTap,
                          child: Container(
                            width: double.infinity,
                            padding: EdgeInsets.all(16.w),
                            decoration: BoxDecoration(
                              color: AppColors.bgCard,
                              borderRadius: BorderRadius.circular(16.r),
                              border: Border.all(
                                color: AppColors.gray300,
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
                            child: Row(
                              children: [
                                Expanded(
                                  child: Column(
                                    crossAxisAlignment:
                                        CrossAxisAlignment.start,
                                    children: [
                                      Text(
                                        '商品說明',
                                        style: TextStyle(
                                          fontSize: 18.sp,
                                          fontWeight: FontWeight.w700,
                                          color: AppColors.textEmphasis,
                                        ),
                                      ),
                                      SizedBox(height: 4.h),
                                      Text(
                                        '點擊進入編輯頁',
                                        style: TextStyle(
                                          fontSize: 12.sp,
                                          color: AppColors.textSecondary,
                                        ),
                                      ),
                                    ],
                                  ),
                                ),
                                SizedBox(width: 8.w),
                                Icon(
                                  AppIcons.chevronRight,
                                  color: AppIconColors.defaultColor,
                                  size: 24.sp,
                                ),
                              ],
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

          SetupBottomBar(
            onBack: () => context.pop(),
            onNext: isFormValid ? onNext : null,
          ),
        ],
      ),
    );
  }

  static Widget _buildRequiredLabel(String label) {
    return Row(
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
        Text(
          label,
          style: TextStyle(
            fontSize: 14.sp,
            fontWeight: FontWeight.w700,
            color: AppColors.textEmphasis,
          ),
        ),
      ],
    );
  }

  static Widget _buildTextField({
    required TextEditingController controller,
    required String hintText,
    bool readOnly = false,
    ValueChanged<String>? onChanged,
  }) {
    return Container(
      height: 44.h,
      decoration: BoxDecoration(
        color: readOnly ? AppColors.gray100 : null,
        border: Border.all(color: AppColors.dividerLight, width: 1),
        borderRadius: BorderRadius.circular(8.r),
      ),
      alignment: Alignment.center,
      child: TextField(
        controller: controller,
        readOnly: readOnly,
        onChanged: onChanged,
        textAlignVertical: TextAlignVertical.center,
        decoration: InputDecoration(
          hintText: hintText,
          hintStyle: TextStyle(color: AppColors.gray300, fontSize: 14.sp),
          border: InputBorder.none,
          contentPadding: EdgeInsets.symmetric(horizontal: 12.w),
          isDense: true,
        ),
        style: TextStyle(
          fontSize: 14.sp,
          color: readOnly ? AppColors.textSecondary : AppColors.textEmphasis,
        ),
      ),
    );
  }

  /// 分類選擇欄位（可點擊顯示下拉選單）
  static Widget _buildCategoryField({
    required GlobalKey key,
    required TextEditingController controller,
    required String hintText,
    required VoidCallback onTap,
  }) {
    return GestureDetector(
      key: key,
      onTap: onTap,
      child: Container(
        height: 44.h,
        decoration: BoxDecoration(
          color: AppColors.bgCard,
          border: Border.all(color: AppColors.dividerLight, width: 1),
          borderRadius: BorderRadius.circular(8.r),
        ),
        padding: EdgeInsets.symmetric(horizontal: 12.w),
        child: Row(
          children: [
            Expanded(
              child: Text(
                controller.text.isEmpty ? hintText : controller.text,
                style: TextStyle(
                  fontSize: 14.sp,
                  color: controller.text.isEmpty
                      ? AppColors.gray300
                      : AppColors.textEmphasis,
                ),
              ),
            ),
            Icon(
              AppIcons.keyboardArrowDown,
              color: AppIconColors.defaultColor,
              size: 20.sp,
            ),
          ],
        ),
      ),
    );
  }

  static void showAddCategoryDialog(
    BuildContext context,
    TextEditingController categoryController,
    TextEditingController subCategoryController,
    ValueNotifier<List<String>> l2Categories,
    ValueNotifier<List<String>> l3Categories,
  ) {
    final controller = TextEditingController();
    showDialog(
      context: context,
      barrierDismissible: true,
      barrierColor: AppColors.textPrimary,
      builder: (ctx) => StatefulBuilder(
        builder: (context, setDialogState) {
          final hasText = controller.text.trim().isNotEmpty;
          return Dialog(
            shape: RoundedRectangleBorder(
              borderRadius: BorderRadius.circular(12.r),
            ),
            insetPadding: EdgeInsets.symmetric(horizontal: 40.w),
            child: Container(
              width: 280.w,
              constraints: const BoxConstraints(maxWidth: 280),
              decoration: BoxDecoration(
                color: AppColors.bgCard,
                borderRadius: BorderRadius.circular(12.r),
              ),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Padding(
                    padding: EdgeInsets.fromLTRB(16.w, 20.h, 16.w, 12.h),
                    child: Text(
                      '新增商品分類',
                      textAlign: TextAlign.center,
                      style: TextStyle(
                        color: AppColors.textPrimary,
                        fontSize: 16.sp,
                        fontWeight: FontWeight.w700,
                      ),
                    ),
                  ),
                  Padding(
                    padding: EdgeInsets.symmetric(horizontal: 16.w),
                    child: TextField(
                      controller: controller,
                      onChanged: (_) => setDialogState(() {}),
                      decoration: InputDecoration(
                        hintText: '輸入分類名稱',
                        hintStyle: TextStyle(
                          color: AppColors.textSecondary,
                          fontSize: 14.sp,
                        ),
                        contentPadding: EdgeInsets.symmetric(
                          horizontal: 12.w,
                          vertical: 12.h,
                        ),
                        border: OutlineInputBorder(
                          borderRadius: BorderRadius.circular(8.r),
                          borderSide: const BorderSide(
                            color: AppColors.cardBorder,
                          ),
                        ),
                        enabledBorder: OutlineInputBorder(
                          borderRadius: BorderRadius.circular(8.r),
                          borderSide: const BorderSide(
                            color: AppColors.cardBorder,
                          ),
                        ),
                        focusedBorder: OutlineInputBorder(
                          borderRadius: BorderRadius.circular(8.r),
                          borderSide: BorderSide(
                            color: AppColors.primary,
                            width: 1.5.w,
                          ),
                        ),
                      ),
                    ),
                  ),
                  SizedBox(height: 16.h),
                  Container(
                    width: double.infinity,
                    height: 1,
                    color: AppColors.textSecondary,
                  ),
                  SizedBox(
                    height: 44.h,
                    child: Row(
                      children: [
                        Expanded(
                          child: GestureDetector(
                            onTap: () => Navigator.pop(ctx),
                            behavior: HitTestBehavior.opaque,
                            child: Center(
                              child: Text(
                                '取消',
                                textAlign: TextAlign.center,
                                style: TextStyle(
                                  color: AppColors.textPrimary,
                                  fontSize: 16.sp,
                                  fontWeight: FontWeight.w700,
                                ),
                              ),
                            ),
                          ),
                        ),
                        Container(
                          width: 1,
                          height: 44.h,
                          color: AppColors.textSecondary,
                        ),
                        Expanded(
                          child: GestureDetector(
                            onTap: hasText
                                ? () {
                                    final name = controller.text.trim();
                                    Navigator.pop(ctx);
                                    categoryController.text = name;
                                    subCategoryController.text = '';
                                    if (!l2Categories.value.contains(name)) {
                                      l2Categories.value = [
                                        ...l2Categories.value,
                                        name,
                                      ];
                                    }
                                    l3Categories.value = [];
                                  }
                                : null,
                            behavior: HitTestBehavior.opaque,
                            child: Center(
                              child: Text(
                                '確定',
                                textAlign: TextAlign.center,
                                style: TextStyle(
                                  color: hasText
                                      ? AppColors.primary
                                      : AppColors.textSecondary,
                                  fontSize: 16.sp,
                                  fontWeight: FontWeight.w700,
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
        },
      ),
    );
  }

  static void showAddSubCategoryDialog(
    BuildContext context,
    TextEditingController subCategoryController,
    ValueNotifier<List<String>> l3Categories,
  ) {
    final controller = TextEditingController();
    showDialog(
      context: context,
      barrierDismissible: true,
      barrierColor: AppColors.textPrimary,
      builder: (ctx) => StatefulBuilder(
        builder: (context, setDialogState) {
          final hasText = controller.text.trim().isNotEmpty;
          return Dialog(
            shape: RoundedRectangleBorder(
              borderRadius: BorderRadius.circular(12.r),
            ),
            insetPadding: EdgeInsets.symmetric(horizontal: 40.w),
            child: Container(
              width: 280.w,
              constraints: const BoxConstraints(maxWidth: 280),
              decoration: BoxDecoration(
                color: AppColors.bgCard,
                borderRadius: BorderRadius.circular(12.r),
              ),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Padding(
                    padding: EdgeInsets.fromLTRB(16.w, 20.h, 16.w, 12.h),
                    child: Text(
                      '新增詳細分類',
                      textAlign: TextAlign.center,
                      style: TextStyle(
                        color: AppColors.textPrimary,
                        fontSize: 16.sp,
                        fontWeight: FontWeight.w700,
                      ),
                    ),
                  ),
                  Padding(
                    padding: EdgeInsets.symmetric(horizontal: 16.w),
                    child: TextField(
                      controller: controller,
                      onChanged: (_) => setDialogState(() {}),
                      decoration: InputDecoration(
                        hintText: '輸入分類名稱',
                        hintStyle: TextStyle(
                          color: AppColors.textSecondary,
                          fontSize: 14.sp,
                        ),
                        contentPadding: EdgeInsets.symmetric(
                          horizontal: 12.w,
                          vertical: 12.h,
                        ),
                        border: OutlineInputBorder(
                          borderRadius: BorderRadius.circular(8.r),
                          borderSide: const BorderSide(
                            color: AppColors.cardBorder,
                          ),
                        ),
                        enabledBorder: OutlineInputBorder(
                          borderRadius: BorderRadius.circular(8.r),
                          borderSide: const BorderSide(
                            color: AppColors.cardBorder,
                          ),
                        ),
                        focusedBorder: OutlineInputBorder(
                          borderRadius: BorderRadius.circular(8.r),
                          borderSide: BorderSide(
                            color: AppColors.primary,
                            width: 1.5.w,
                          ),
                        ),
                      ),
                    ),
                  ),
                  SizedBox(height: 16.h),
                  Container(
                    width: double.infinity,
                    height: 1,
                    color: AppColors.textSecondary,
                  ),
                  SizedBox(
                    height: 44.h,
                    child: Row(
                      children: [
                        Expanded(
                          child: GestureDetector(
                            onTap: () => Navigator.pop(ctx),
                            behavior: HitTestBehavior.opaque,
                            child: Center(
                              child: Text(
                                '取消',
                                textAlign: TextAlign.center,
                                style: TextStyle(
                                  color: AppColors.textPrimary,
                                  fontSize: 16.sp,
                                  fontWeight: FontWeight.w700,
                                ),
                              ),
                            ),
                          ),
                        ),
                        Container(
                          width: 1,
                          height: 44.h,
                          color: AppColors.textSecondary,
                        ),
                        Expanded(
                          child: GestureDetector(
                            onTap: hasText
                                ? () {
                                    final name = controller.text.trim();
                                    Navigator.pop(ctx);
                                    subCategoryController.text = name;
                                    if (!l3Categories.value.contains(name)) {
                                      l3Categories.value = [
                                        ...l3Categories.value,
                                        name,
                                      ];
                                    }
                                  }
                                : null,
                            behavior: HitTestBehavior.opaque,
                            child: Center(
                              child: Text(
                                '確定',
                                textAlign: TextAlign.center,
                                style: TextStyle(
                                  color: hasText
                                      ? AppColors.primary
                                      : AppColors.textSecondary,
                                  fontSize: 16.sp,
                                  fontWeight: FontWeight.w700,
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
        },
      ),
    );
  }
}

/// 分類下拉選單 Overlay（支援滑動選擇）
class _CategoryDropdownOverlay extends StatefulWidget {
  const _CategoryDropdownOverlay({
    required this.position,
    required this.fieldWidth,
    required this.fieldHeight,
    required this.categories,
    required this.selectedCategory,
    required this.onSelect,
    required this.onDismiss,
    required this.onAddNew,
  });

  final Offset position;
  final double fieldWidth;
  final double fieldHeight;
  final List<String> categories;
  final String selectedCategory;
  final ValueChanged<String> onSelect;
  final VoidCallback onDismiss;
  final VoidCallback onAddNew;

  @override
  State<_CategoryDropdownOverlay> createState() =>
      _CategoryDropdownOverlayState();
}

class _CategoryDropdownOverlayState extends State<_CategoryDropdownOverlay> {
  String? _hoveredCategory;
  bool _isLongPressActive = false; // 長按模式是否啟動
  final TextEditingController _searchController = TextEditingController();
  List<String> _filteredCategories = [];
  final GlobalKey _listKey = GlobalKey();
  final Map<int, GlobalKey> _itemKeys = {};
  final ScrollController _scrollController = ScrollController();

  @override
  void initState() {
    super.initState();
    _filteredCategories = List.from(widget.categories);
    _searchController.addListener(_onSearchChanged);
  }

  @override
  void dispose() {
    _searchController.dispose();
    _scrollController.dispose();
    super.dispose();
  }

  void _onSearchChanged() {
    final query = _searchController.text.toLowerCase();
    setState(() {
      if (query.isEmpty) {
        _filteredCategories = List.from(widget.categories);
      } else {
        _filteredCategories = widget.categories
            .where((c) => c.toLowerCase().contains(query))
            .toList();
      }
      _itemKeys.clear();
    });
  }

  /// 根據全局座標找到對應的分類
  String? _findCategoryAtPosition(Offset globalPosition) {
    for (int i = 0; i < _filteredCategories.length; i++) {
      final key = _itemKeys[i];
      if (key?.currentContext != null) {
        final renderBox = key!.currentContext!.findRenderObject() as RenderBox?;
        if (renderBox != null) {
          final itemPosition = renderBox.localToGlobal(Offset.zero);
          final itemSize = renderBox.size;
          final itemRect = Rect.fromLTWH(
            itemPosition.dx,
            itemPosition.dy,
            itemSize.width,
            itemSize.height,
          );
          if (itemRect.contains(globalPosition)) {
            return _filteredCategories[i];
          }
        }
      }
    }
    return null;
  }

  /// 長按啟動拖曳選擇模式
  void _onLongPressStart(LongPressStartDetails details) {
    setState(() {
      _isLongPressActive = true;
      _hoveredCategory = _findCategoryAtPosition(details.globalPosition);
    });
  }

  /// 長按拖曳中，更新高亮項目
  void _onLongPressMoveUpdate(LongPressMoveUpdateDetails details) {
    if (_isLongPressActive) {
      setState(() {
        _hoveredCategory = _findCategoryAtPosition(details.globalPosition);
      });
    }
  }

  /// 長按結束，選擇當前項目
  void _onLongPressEnd(LongPressEndDetails details) {
    if (_isLongPressActive && _hoveredCategory != null) {
      widget.onSelect(_hoveredCategory!);
    } else if (_isLongPressActive &&
        _isOnAddNewButton(details.globalPosition)) {
      widget.onAddNew();
    }
    // 不關閉選單，只重置狀態（如果沒選到任何東西）
    setState(() {
      _isLongPressActive = false;
      _hoveredCategory = null;
    });
  }

  @override
  Widget build(BuildContext context) {
    final screenHeight = MediaQuery.of(context).size.height;
    final maxHeight =
        (screenHeight - widget.position.dy - widget.fieldHeight - 20).clamp(
          150.0,
          350.0,
        );

    return GestureDetector(
      onTap: widget.onDismiss, // 點擊背景關閉
      onLongPressStart: _onLongPressStart,
      onLongPressMoveUpdate: _onLongPressMoveUpdate,
      onLongPressEnd: _onLongPressEnd,
      behavior: HitTestBehavior.opaque,
      child: Stack(
        children: [
          // 背景
          Positioned.fill(child: Container(color: Colors.transparent)),
          // 下拉選單
          Positioned(
            left: widget.position.dx,
            top: widget.position.dy + widget.fieldHeight + 4,
            child: Material(
              elevation: 0,
              color: Colors.transparent,
              child: Container(
                width: widget.fieldWidth + 20,
                constraints: BoxConstraints(maxHeight: maxHeight),
                decoration: BoxDecoration(
                  color: AppColors.bgPrimary,
                  borderRadius: BorderRadius.circular(12.r),
                  border: Border.all(color: AppColors.cardBorder),
                  boxShadow: [
                    BoxShadow(
                      color: AppColors.cardShadow,
                      blurRadius: 4,
                      offset: Offset(0, 4.h),
                    ),
                  ],
                ),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    // 搜尋欄
                    Padding(
                      padding: EdgeInsets.all(8.w),
                      child: Container(
                        height: 30.h,
                        decoration: BoxDecoration(
                          color: AppColors.bgCard,
                          borderRadius: BorderRadius.circular(999.r),
                          border: Border.all(color: AppColors.cardBorder),
                        ),
                        padding: EdgeInsets.symmetric(horizontal: 12.w),
                        child: Row(
                          children: [
                            Icon(
                              AppIcons.search,
                              color: AppIconColors.defaultColor,
                              size: 16.sp,
                            ),
                            SizedBox(width: 8.w),
                            Expanded(
                              child: TextField(
                                controller: _searchController,
                                style: TextStyle(
                                  fontSize: 14.sp,
                                  color: AppColors.textPrimary,
                                ),
                                decoration: InputDecoration(
                                  hintText: '搜尋',
                                  hintStyle: TextStyle(
                                    fontSize: 14.sp,
                                    color: AppColors.textSecondary,
                                  ),
                                  border: InputBorder.none,
                                  isDense: true,
                                  contentPadding: EdgeInsets.zero,
                                ),
                              ),
                            ),
                          ],
                        ),
                      ),
                    ),
                    // 分類列表
                    Flexible(
                      child: ListView.builder(
                        key: _listKey,
                        controller: _scrollController,
                        shrinkWrap: true,
                        padding: EdgeInsets.symmetric(
                          horizontal: 8.w,
                          vertical: 4.h,
                        ),
                        itemCount:
                            _filteredCategories.length +
                            1, // +1 for add new button
                        itemBuilder: (context, index) {
                          if (index == _filteredCategories.length) {
                            // 新增分類按鈕
                            _itemKeys[-1] ??= GlobalKey();
                            return GestureDetector(
                              onTap: widget.onAddNew,
                              child: Container(
                                key: _itemKeys[-1],
                                padding: EdgeInsets.symmetric(
                                  vertical: 8.h,
                                  horizontal: 8.w,
                                ),
                                child: Row(
                                  children: [
                                    Container(
                                      width: 20.w,
                                      height: 20.h,
                                      decoration: BoxDecoration(
                                        color: AppColors.primary,
                                        borderRadius: BorderRadius.circular(
                                          10.r,
                                        ),
                                      ),
                                      child: Icon(
                                        AppIcons.add,
                                        color: AppIconColors.onBrand,
                                        size: 14.sp,
                                      ),
                                    ),
                                    SizedBox(width: 8.w),
                                    Text(
                                      '新增分類',
                                      style: TextStyle(
                                        fontSize: 14.sp,
                                        color: AppColors.textPrimary,
                                      ),
                                    ),
                                  ],
                                ),
                              ),
                            );
                          }

                          // 為每個項目建立唯一的 key
                          _itemKeys[index] ??= GlobalKey();
                          final category = _filteredCategories[index];
                          final isHovered = _hoveredCategory == category;
                          final isSelected =
                              widget.selectedCategory == category;

                          return GestureDetector(
                            onTap: () => widget.onSelect(category),
                            child: Container(
                              key: _itemKeys[index],
                              padding: EdgeInsets.symmetric(
                                vertical: 8.h,
                                horizontal: 8.w,
                              ),
                              decoration: BoxDecoration(
                                color: isHovered
                                    ? const Color(0xB38F3315)
                                    : (isSelected
                                          ? const Color(0x338F3315)
                                          : Colors.transparent),
                                borderRadius: BorderRadius.circular(999.r),
                              ),
                              child: Text(
                                category,
                                style: TextStyle(
                                  fontSize: 16.sp,
                                  color: isHovered
                                      ? AppColors.bgCard
                                      : AppColors.textPrimary,
                                  letterSpacing: 2,
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
            ),
          ),
        ],
      ),
    );
  }

  /// 檢查是否點在「新增分類」按鈕上
  bool _isOnAddNewButton(Offset globalPosition) {
    final key = _itemKeys[-1];
    if (key?.currentContext != null) {
      final renderBox = key!.currentContext!.findRenderObject() as RenderBox?;
      if (renderBox != null) {
        final itemPosition = renderBox.localToGlobal(Offset.zero);
        final itemSize = renderBox.size;
        final itemRect = Rect.fromLTWH(
          itemPosition.dx,
          itemPosition.dy,
          itemSize.width,
          itemSize.height,
        );
        return itemRect.contains(globalPosition);
      }
    }
    return false;
  }
}
