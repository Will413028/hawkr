import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:flutter/material.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../../shared/widgets/setup_bottom_bar.dart';
import '../../data/product_repository.dart';
import '../ai_assistant_button.dart';
import '../product_listing_draft_controller.dart';
import '../product_setup_progress_bar.dart';
import '../product_setup_top_bar.dart';
import 'picker_category_search_row.dart';
import 'picker_l3_content_panel.dart';
import 'picker_new_category_dialog.dart';
import 'picker_new_subcat_dialog.dart';
import 'picker_shared_widgets.dart';
import 'picker_subcat_drawer.dart';

/// 商品上架第 1 步：L1→L2→L3 三層分類選擇，支持新增自定義分類
class ProductSetupPickerScreen extends HookConsumerWidget {
  const ProductSetupPickerScreen({
    super.key,
    this.initialType,
    this.isEditMode = false,
    this.existingProduct,
  });

  final String? initialType;

  /// 是否為編輯模式
  final bool isEditMode;

  /// 編輯模式時帶入的現有商品資料
  final Map<String, dynamic>? existingProduct;

  // ── L1 靜態資料 ────────────────────────────────────────────────

  static const List<String> _defaultTopTabs = [
    'Vegetables',
    'Fruits',
    'Dairy',
    'Meat',
    'Seafood',
    'Grains',
    'Spices',
    'Processed',
    'Flowers',
  ];

  static final Map<String, String> _topTabLabels = {
    'Vegetables': '蔬菜',
    'Fruits': '水果',
    'Dairy': '蛋奶',
    'Meat': '肉類',
    'Seafood': '海鮮',
    'Grains': '穀物',
    'Spices': '香料',
    'Processed': '加工食品',
    'Flowers': '花卉',
  };

  static final Map<int, String> _categoryIdMap = {
    1: 'Vegetables',
    2: 'Fruits',
    3: 'Dairy',
    4: 'Meat',
    5: 'Seafood',
    6: 'Grains',
    7: 'Spices',
    8: 'Processed',
    9: 'Flowers',
  };

  static const Set<String> _allowedTopKeys = {
    'Vegetables',
    'Fruits',
    'Dairy',
    'Meat',
    'Seafood',
    'Grains',
    'Spices',
    'Processed',
    'Flowers',
  };

  // ── Build ──────────────────────────────────────────────────────

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final loading = useState(true);
    final loadFinished = useState(false);
    final searchCtrl = useTextEditingController();
    final leftListController = useScrollController();
    final rightListController = useScrollController();
    final draft = ref.watch(productListingDraftControllerProvider);

    final majors = useState<List<String>>(List<String>.from(_defaultTopTabs));
    final subByMajor = useState<Map<String, List<String>>>({});
    final l3ByL2 = useState<Map<String, List<String>>>({});
    final catIdToL2 = useState<Map<int, String>>({});
    final catIdToL3Name = useState<Map<int, String>>({});
    final l2NameToId = useState<Map<String, int>>({});
    final l3NameToId = useState<Map<String, int>>({});
    final l1KeyToId = useState<Map<String, int>>({});

    // 左側欄寬（可拖拉，Spring 動畫）
    final sidebarAnim = useAnimationController(
      lowerBound: 90.0.w,
      upperBound: 160.0.w,
      initialValue: 90.0.w,
    );
    final sidebarWidth = useAnimation(sidebarAnim);

    final editModeL2 = useState<String?>(null);
    final editModeL3 = useState<String?>(null);

    final activeTop = useState(
      _computeInitialActiveTop(
        isEditMode: isEditMode,
        existingProduct: existingProduct,
        initialType: initialType,
      ),
    );
    final subcats = useState<List<String>>([]);
    final activeSubcat = useState<String?>(null);
    final selectedItem = useState<Map<String, dynamic>?>(null);

    // AI 填入分類時自動啟用「下一步」按鈕
    useEffect(() {
      final catIdStr = draft.categoryId;
      if (catIdStr == null || catIdStr.isEmpty) return null;
      if (selectedItem.value != null) return null;
      if (catIdToL3Name.value.isEmpty) return null;
      final parsedId = int.tryParse(catIdStr);
      if (parsedId == null) return null;
      final l3Name = catIdToL3Name.value[parsedId];
      final l2Name = catIdToL2.value[parsedId] ?? '';
      String l1Key = activeTop.value;
      for (final entry in subByMajor.value.entries) {
        if (entry.value.contains(l2Name)) {
          l1Key = entry.key;
          break;
        }
      }
      selectedItem.value = {
        'id': parsedId,
        '_selectedL3': l3Name,
        '_selectedL2': l2Name.isNotEmpty ? l2Name : null,
        '_selectedL1': l1Key,
        'isCandidateCategory': false,
      };
      if (l2Name.isNotEmpty) {
        activeTop.value = l1Key;
        activeSubcat.value = l2Name;
      }
      return null;
    }, [draft.patchVersion, catIdToL3Name.value.length]);

    void rebuildSubcats({bool preserveSelection = true}) {
      final list = List<String>.from(
        subByMajor.value[activeTop.value] ?? const [],
      )..sort();
      final prev = activeSubcat.value;
      subcats.value = list;

      if (isEditMode) {
        debugPrint(
          '[ProductPicker] rebuildSubcats editModeL2="${editModeL2.value}" editModeL3="${editModeL3.value}" activeTop=${activeTop.value} listLen=${list.length} match=${editModeL2.value != null && list.contains(editModeL2.value)}',
        );
      }

      if (isEditMode &&
          editModeL2.value != null &&
          list.contains(editModeL2.value)) {
        activeSubcat.value = editModeL2.value;
        // 編輯模式一律預先 populate selectedItem 讓「下一步」可點。
        // 後端 schema 沒有 Lv4，所以 editModeL3 常常是 null — 這時從 tree 幫使用者自動挑一個：
        //   1. 優先選跟 L2 同名的 L3（常見「預設子分類」慣例）
        //   2. 否則挑第一個 L3
        //   3. 樹裡根本沒 L3 就留空（selectedItem 仍然 populate，proceed 照樣可點）
        final top = activeTop.value;
        final sub = activeSubcat.value!;
        final l3Candidates = l3ByL2.value[sub] ?? const <String>[];
        String l3;
        if (editModeL3.value != null &&
            l3Candidates.contains(editModeL3.value)) {
          l3 = editModeL3.value!;
        } else if (l3Candidates.contains(sub)) {
          l3 = sub; // 同名預設子分類
        } else if (l3Candidates.isNotEmpty) {
          l3 = l3Candidates.first;
        } else {
          l3 = '';
        }
        // 回寫給 editModeL3，避免下一次 rebuild 又退回 null
        editModeL3.value = l3.isNotEmpty ? l3 : null;

        final existingCatalogId = existingProduct?['catalogId'];
        selectedItem.value = {
          '_selectedL3': l3,
          '_selectedL2': sub,
          '_selectedL1': top,
          'category': '${_topTabLabels[top] ?? top}_$sub',
          'subCategory': l3,
          // 優先用 L3 name 對應的 id，否則 fallback 到 product 的 catalogId (backend Lv3Id)
          if (l3.isNotEmpty && l3NameToId.value.containsKey(l3))
            'id': l3NameToId.value[l3]
          else if (l3.isEmpty && l2NameToId.value.containsKey(sub))
            'id': l2NameToId.value[sub]
          else if (existingCatalogId is int)
            'id': existingCatalogId,
          'isCandidateCategory': false,
        };
      } else if (preserveSelection && prev != null && list.contains(prev)) {
        activeSubcat.value = prev;
      } else {
        activeSubcat.value = list.isNotEmpty ? list.first : null;
      }
    }

    useEffect(() {
      if (isEditMode && existingProduct != null) {
        final p = existingProduct!;
        final l2 = p['categoryL2']?.toString();
        final l3 = p['categoryL3']?.toString();
        if (l2 != null && l2.isNotEmpty) editModeL2.value = l2;
        if (l3 != null && l3.isNotEmpty) editModeL3.value = l3;

        if (editModeL2.value == null || editModeL3.value == null) {
          final categoryPath = p['categoryPath']?.toString() ?? '';
          if (categoryPath.contains('_')) {
            final parts = categoryPath.split('_');
            if (parts.length >= 3) {
              editModeL2.value ??= parts[1];
              editModeL3.value ??= parts[2];
            } else if (parts.length >= 2) {
              editModeL2.value ??= parts[1];
            }
          }
        }
        editModeL2.value ??=
            (p['_sub'] ?? p['subCategory'] ?? p['categoryName'])?.toString();
        editModeL3.value ??=
            (p['subProductCategory'] ?? p['_selectedL3'] ?? p['_var'])
                ?.toString();
      }

      () async {
        // 效能優化：分類樹 (_load) 跟商品詳情 (_loadCategoryAsync) 互相獨立，
        // 平行發起兩個網路請求，把總耗時從 tree+detail 降到 max(tree, detail)。
        // _loadCategoryAsync 需要 _load 的結果才能合併 subcats，所以先 await tree 完，
        // 但 product detail 的 Future 在 tree 開始時就 fire，等 tree 完就能立刻用。
        final needLoadDetail =
            isEditMode &&
            existingProduct != null &&
            existingProduct!['_needLoadCategory'] == true &&
            existingProduct!['_productId'] != null;
        final Future<Map<String, dynamic>>? productDetailFuture = needLoadDetail
            ? ref
                  .read(productRepositoryProvider)
                  .getProductDetail(existingProduct!['_productId'])
            : null;

        await _load(
          ref: ref,
          context: context,
          isEditMode: isEditMode,
          existingProduct: existingProduct,
          editModeL2: editModeL2,
          editModeL3: editModeL3,
          activeTop: activeTop,
          majors: majors,
          subByMajor: subByMajor,
          l3ByL2: l3ByL2,
          catIdToL2: catIdToL2,
          catIdToL3Name: catIdToL3Name,
          l2NameToId: l2NameToId,
          l3NameToId: l3NameToId,
          l1KeyToId: l1KeyToId,
          loading: loading,
          loadFinished: loadFinished,
          rebuildSubcats: rebuildSubcats,
        );

        if (productDetailFuture != null) {
          await _applyCategoryFromPreloadedDetail(
            productDetailFuture: productDetailFuture,
            activeTop: activeTop,
            editModeL2: editModeL2,
            editModeL3: editModeL3,
            subByMajor: subByMajor,
            l3ByL2: l3ByL2,
            l3NameToId: l3NameToId,
            rebuildSubcats: rebuildSubcats,
            existingProduct: existingProduct,
          );
        }
      }();

      return null;
    }, const []);

    // ── Event handlers ─────────────────────────────────────────

    void onSearch() {
      final key = searchCtrl.text.trim().toLowerCase();
      if (key.isEmpty) return;
      for (final entry in subByMajor.value.entries) {
        for (final sub in entry.value) {
          if (sub.toLowerCase().contains(key)) {
            activeTop.value = entry.key;
            rebuildSubcats();
            activeSubcat.value = sub;
            return;
          }
        }
      }
    }

    Future<void> proceedToNextStep() async {
      if (selectedItem.value == null) return;

      final l1Key = activeTop.value;
      final l1Label = _topTabLabels[l1Key] ?? l1Key;
      final l1Id = l1KeyToId.value[l1Key];
      final l2Name =
          selectedItem.value!['_selectedL2'] ?? activeSubcat.value ?? '';
      final l2Id = l2NameToId.value[l2Name];
      final l3Name = selectedItem.value!['_selectedL3'] ?? '';
      final detailCategoryName = l3Name.isNotEmpty ? l3Name : l2Name;
      final catalogId = selectedItem.value!['id'] is int
          ? selectedItem.value!['id'] as int
          : null;
      const isCandidateCat = false;

      String? initialProductName;
      if (isEditMode && existingProduct != null) {
        final ep = existingProduct!;
        initialProductName = (ep['productName'] ?? ep['name'] ?? ep['skuName'])
            ?.toString();
      }

      final result = await context.push(
        '/product/info',
        extra: {
          'skuId': isEditMode && existingProduct != null
              ? (existingProduct!['skuId'] ?? existingProduct!['id'])
              : null,
          'initialProductName': initialProductName,
          'initialOrigin': null,
          'initialDescription': null,
          'initialCategory': '${l1Label}_$l2Name',
          'initialSubCategory': detailCategoryName,
          'initialCatalogId': catalogId,
          'isCandidateCategory': isCandidateCat,
          'l1Id': l1Id,
          'l1Name': l1Label,
          'l2Id': l2Id,
          'l2Name': l2Name,
          'l3Name': l3Name.isNotEmpty ? l3Name : null,
          'isEditMode': isEditMode,
          'existingProduct': existingProduct,
        },
      );

      if (context.mounted && result != null) context.pop(result);
    }

    void onAddCategory() {
      showPickerNewCategoryDialog(
        context: context,
        onSubmit: (name) async {
          final trimmed = name.trim();
          if (trimmed.isEmpty) return;

          final l1Key = activeTop.value;

          // 1. 即時加入本地 L2 列表，讓使用者馬上看到新分類
          final newSubByMajor = Map<String, List<String>>.from(
            subByMajor.value,
          );
          final list = List<String>.from(newSubByMajor[l1Key] ?? const []);
          if (!list.contains(trimmed)) {
            list.add(trimmed);
            newSubByMajor[l1Key] = list;
            subByMajor.value = newSubByMajor;
          }

          // 2. 選中新分類（rebuildSubcats 會 preserve activeSubcat）
          activeSubcat.value = trimmed;
          selectedItem.value = null;
          rebuildSubcats();

          // 3. 送出 L2 分類。後端會立即建立 active 分類；
          // 同 parent 重名時會回傳既有 active 分類 id。
          final l1Id = l1KeyToId.value[l1Key];
          if (l1Id == null) return;
          try {
            final result = await ref
                .read(productRepositoryProvider)
                .submitCategoryCandidate(name: trimmed, parentId: l1Id);
            if (result['code'] == 0) {
              final data = result['data'];
              final rawId = data is Map
                  ? (data['id'] ?? data['candidateId'])
                  : result['id'];
              final l2Id = rawId is int
                  ? rawId
                  : int.tryParse(rawId?.toString() ?? '');
              if (l2Id != null) {
                final newMap = Map<String, int>.from(l2NameToId.value);
                newMap[trimmed] = l2Id;
                l2NameToId.value = newMap;
                selectedItem.value = {
                  'id': l2Id,
                  '_selectedL3': '',
                  '_selectedL2': trimmed,
                  '_selectedL1': l1Key,
                  'category': '${_topTabLabels[l1Key] ?? l1Key}_$trimmed',
                  'subCategory': trimmed,
                  'isCandidateCategory': false,
                };
              }
            }
          } catch (e) {
            debugPrint('[ProductPicker] submit L2 category failed: $e');
          }
        },
      );
    }

    void onAddSubcategory() {
      final parentName = activeSubcat.value;
      if (parentName == null) return;
      final parentId = l2NameToId.value[parentName];
      if (parentId == null) {
        // FloatingToast 需要 context，但此處 context 仍有效
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(const SnackBar(content: Text('無法取得父分類 ID')));
        return;
      }
      final l1Key = activeTop.value;
      final l1Label = _topTabLabels[l1Key] ?? l1Key;
      final l1Id = l1KeyToId.value[l1Key] ?? 0;

      showPickerNewSubcatDialog(
        context: context,
        ref: ref,
        parentName: parentName,
        parentId: parentId,
        l1Key: l1Key,
        l1Label: l1Label,
        l1Id: l1Id,
        isEditMode: isEditMode,
        existingProduct: existingProduct,
        l3ByL2: l3ByL2,
      );
    }

    // ── UI ─────────────────────────────────────────────────────

    return Scaffold(
      backgroundColor: AppColors.bgPrimary,
      body: Column(
        children: [
          ProductSetupTopBar(
            title: '新增商品',
            trailing: kAiProductListingAssistantEnabled
                ? const AiAssistantButton()
                : null,
          ),
          const ProductSetupProgressBar(currentStep: 1),
          SizedBox(height: 12.h),
          PickerCategorySearchRow(
            majors: majors.value,
            topTabLabels: _topTabLabels,
            activeTop: activeTop.value,
            onTopChanged: (key) {
              activeTop.value = key;
              rebuildSubcats();
            },
            searchCtrl: searchCtrl,
            onSearch: onSearch,
          ),
          SizedBox(height: 8.h),
          const Divider(height: 1, thickness: 1, color: AppColors.cardBorder),
          Expanded(
            child: Stack(
              children: [
                // 右側 L3 清單（固定起始位置，不隨抽屜寬度變動）
                Positioned(
                  left: 90.0.w + kPickerHandleWidth,
                  top: 0,
                  right: 0,
                  bottom: 0,
                  child: PickerL3ContentPanel(
                    l3List: l3ByL2.value[activeSubcat.value] ?? [],
                    activeSubcat: activeSubcat.value,
                    selectedL3: selectedItem.value?['_selectedL3'] as String?,
                    loading: loading.value,
                    loadFinished: loadFinished.value,
                    scrollController: rightListController,
                    onL3Selected: (l3) {
                      selectedItem.value = {
                        '_selectedL3': l3,
                        '_selectedL2': activeSubcat.value,
                        '_selectedL1': activeTop.value,
                        'category':
                            '${_topTabLabels[activeTop.value] ?? activeTop.value}_${activeSubcat.value}',
                        'subCategory': l3,
                        if (l3NameToId.value.containsKey(l3))
                          'id': l3NameToId.value[l3],
                        'isCandidateCategory': false,
                      };
                    },
                    onAddSubcategory: onAddSubcategory,
                  ),
                ),
                // 左側抽屜（含遮罩與把手）
                Positioned.fill(
                  child: PickerSubcatDrawer(
                    sidebarAnim: sidebarAnim,
                    sidebarWidth: sidebarWidth,
                    loading: loading.value,
                    loadFinished: loadFinished.value,
                    subcats: subcats.value,
                    activeSubcat: activeSubcat.value,
                    leftListController: leftListController,
                    onSubcatSelected: (s) {
                      activeSubcat.value = s;
                      final l3List = l3ByL2.value[s] ?? const <String>[];
                      if (l3List.isEmpty && l2NameToId.value.containsKey(s)) {
                        selectedItem.value = {
                          'id': l2NameToId.value[s],
                          '_selectedL3': '',
                          '_selectedL2': s,
                          '_selectedL1': activeTop.value,
                          'category':
                              '${_topTabLabels[activeTop.value] ?? activeTop.value}_$s',
                          'subCategory': s,
                          'isCandidateCategory': false,
                        };
                      } else {
                        selectedItem.value = null;
                      }
                    },
                    onAddCategory: onAddCategory,
                  ),
                ),
              ],
            ),
          ),
          SetupBottomBar(
            onBack: null,
            onNext: selectedItem.value != null ? proceedToNextStep : null,
          ),
        ],
      ),
    );
  }

  // ── Static helpers ──────────────────────────────────────────

  static String _computeInitialActiveTop({
    required bool isEditMode,
    required Map<String, dynamic>? existingProduct,
    required String? initialType,
  }) {
    if (isEditMode && existingProduct != null) {
      final p = existingProduct;
      final l1 = p['categoryL1']?.toString();
      if (l1 != null && l1.isNotEmpty) return _formatTopLabel(l1);
      final categoryPath = p['categoryPath']?.toString() ?? '';
      if (categoryPath.contains('_')) {
        final parts = categoryPath.split('_');
        if (parts.isNotEmpty) return _formatTopLabel(parts[0]);
      }
      final type =
          p['type'] ??
          p['productType'] ??
          p['category'] ??
          p['_top'] ??
          p['catalogType'];
      if (type != null && type.toString().trim().isNotEmpty) {
        return _formatTopLabel(type.toString());
      }
      return 'Fruits';
    } else if (initialType != null && initialType.trim().isNotEmpty) {
      return _formatTopLabel(initialType);
    }
    return 'Fruits';
  }

  static num? _toNum(dynamic v) {
    if (v == null) return null;
    if (v is num) return v;
    return num.tryParse(v.toString());
  }

  static String _formatTopLabel(String raw) {
    final trimmed = raw.trim();
    if (trimmed.isEmpty) return 'Other';
    if (_allowedTopKeys.contains(trimmed)) return trimmed;
    for (final entry in _topTabLabels.entries) {
      if (entry.value == trimmed) return entry.key;
    }
    for (final key in _allowedTopKeys) {
      if (key.toLowerCase() == trimmed.toLowerCase()) return key;
    }
    // 不在白名單但有具名 label → 直接保留原值，不要 fallback 到 Other。
    // 過去 fallback 會把 "李"、"番茄" 這種正常 L1 都吞掉，editMode 進來時
    // activeTop 變成 Other → L2/L3 對不上 → 右邊空清單。
    return trimmed;
  }

  // ── Async loaders (static) ──────────────────────────────────

  /// 接收「已經發起」的 product detail Future，等其完成後把資料套進 picker state。
  /// 跟 _load (category tree) 平行跑，把兩個獨立網路 call 的總耗時從
  /// tree+detail 降到 max(tree, detail)。
  static Future<void> _applyCategoryFromPreloadedDetail({
    required Future<Map<String, dynamic>> productDetailFuture,
    required ValueNotifier<String> activeTop,
    required ValueNotifier<String?> editModeL2,
    required ValueNotifier<String?> editModeL3,
    required ValueNotifier<Map<String, List<String>>> subByMajor,
    required ValueNotifier<Map<String, List<String>>> l3ByL2,
    required ValueNotifier<Map<String, int>> l3NameToId,
    required VoidCallback rebuildSubcats,
    required Map<String, dynamic>? existingProduct,
  }) async {
    try {
      final productResp = await productDetailFuture;
      final productData = productResp['data'] as Map<String, dynamic>?;
      if (productData == null) return;

      // 後端 schema 層級對應前端 picker：
      //   - 後端 Lv1 (水果 / 蔬菜 ...) ↔ 前端 L1 top tab
      //   - 後端 Lv2 (李 / 番茄 ...)   ↔ 前端 L2 left drawer
      //   - 後端 Lv3 (黃肉李 / 小番茄) ↔ 前端 L3 right drawer
      // 歷史備註：曾以為 backend Lv1 是 hidden root 所以讀偏一層；
      // 後端 ProductDetail 已穩定回三層名稱，這個假設過時了。
      // 同時保留 categoryL1/2/3 跟 categoryName 作為更舊資料 fallback。
      final l1 = (productData['categoryLv1Name'] ?? productData['categoryL1'])
          ?.toString()
          .trim();
      final l2 = (productData['categoryLv2Name'] ?? productData['categoryL2'])
          ?.toString()
          .trim();
      final l3 =
          (productData['categoryLv3Name'] ??
                  productData['categoryL3'] ??
                  productData['categoryName'])
              ?.toString()
              .trim();
      debugPrint(
        '[ProductPicker] applyCategoryFromDetail l1="$l1" l2="$l2" l3="$l3" catLv2Id=${productData['categoryLv2Id']} catLv3Id=${productData['categoryLv3Id']}',
      );
      final rawCatId =
          productData['categoryLv3Id'] ?? productData['categoryId'];
      final catalogId = rawCatId is int
          ? rawCatId
          : (rawCatId != null ? int.tryParse(rawCatId.toString()) : null);

      if (l1 != null && l1.isNotEmpty) activeTop.value = _formatTopLabel(l1);
      if (l2 != null && l2.isNotEmpty) {
        editModeL2.value = l2;
        final topKey = activeTop.value;
        final newSubByMajor = Map<String, List<String>>.from(subByMajor.value);
        final topList = List<String>.from(newSubByMajor[topKey] ?? []);
        if (!topList.contains(l2)) topList.add(l2);
        newSubByMajor[topKey] = topList;
        subByMajor.value = newSubByMajor;
      }
      if (l3 != null && l3.isNotEmpty) {
        editModeL3.value = l3;
        if (editModeL2.value != null) {
          final newL3ByL2 = Map<String, List<String>>.from(l3ByL2.value);
          final l3List = List<String>.from(newL3ByL2[editModeL2.value] ?? []);
          if (!l3List.contains(l3)) l3List.add(l3);
          newL3ByL2[editModeL2.value!] = l3List;
          l3ByL2.value = newL3ByL2;
        }
      }

      if (existingProduct != null) {
        existingProduct.addAll(productData);
        existingProduct['catalogId'] = catalogId;
        existingProduct['categoryL1'] = l1;
        existingProduct['categoryL2'] = l2;
        existingProduct['categoryL3'] = l3;
      }

      rebuildSubcats();
    } catch (e) {
      debugPrint('[ProductPicker] Failed to load product detail: $e');
    }
  }

  static Future<void> _load({
    required WidgetRef ref,
    required BuildContext context,
    required bool isEditMode,
    required Map<String, dynamic>? existingProduct,
    required ValueNotifier<String?> editModeL2,
    required ValueNotifier<String?> editModeL3,
    required ValueNotifier<String> activeTop,
    required ValueNotifier<List<String>> majors,
    required ValueNotifier<Map<String, List<String>>> subByMajor,
    required ValueNotifier<Map<String, List<String>>> l3ByL2,
    required ValueNotifier<Map<int, String>> catIdToL2,
    required ValueNotifier<Map<int, String>> catIdToL3Name,
    required ValueNotifier<Map<String, int>> l2NameToId,
    required ValueNotifier<Map<String, int>> l3NameToId,
    required ValueNotifier<Map<String, int>> l1KeyToId,
    required ValueNotifier<bool> loading,
    required ValueNotifier<bool> loadFinished,
    required VoidCallback rebuildSubcats,
  }) async {
    _topTabLabels.removeWhere((key, _) => !_allowedTopKeys.contains(key));
    majors.value = List<String>.from(_defaultTopTabs);
    subByMajor.value = {};
    loading.value = true;
    loadFinished.value = false;

    try {
      final List<Map<String, dynamic>> treeData = isEditMode
          ? await ref.read(productRepositoryProvider).fetchSellerCategoryTree()
          : await ref.read(productRepositoryProvider).fetchCategories();

      if (treeData.isNotEmpty) {
        final newMajors = <String>[];
        final newSubByMajor = <String, List<String>>{};
        final newL3ByL2 = <String, List<String>>{};
        final newCatIdToL2 = <int, String>{};
        final newCatIdToL3Name = <int, String>{};
        final newL2NameToId = <String, int>{};
        final newL3NameToId = <String, int>{};
        final newL1KeyToId = <String, int>{};
        final newIdMap = <int, String>{};

        int? getId(Map<String, dynamic>? c) =>
            c == null ? null : _toNum(c['id'])?.toInt();

        for (final root in treeData) {
          final rootId = getId(root);
          final rootName = root['name']?.toString().trim() ?? '';
          if (rootId == null || rootName.isEmpty) continue;

          final rootKey = _categoryIdMap[rootId] ?? rootName;
          if (!newMajors.contains(rootKey)) newMajors.add(rootKey);
          _topTabLabels[rootKey] = rootName;
          newL1KeyToId[rootKey] = rootId;
          newIdMap[rootId] = rootKey;

          final l2nodes = (root['children'] as List? ?? [])
              .whereType<Map>()
              .map((e) => e.cast<String, dynamic>())
              .toList();

          final l2NameList = <String>[];
          for (final l2 in l2nodes) {
            final l2Id = getId(l2);
            final l2Name = l2['name']?.toString().trim() ?? '';
            if (l2Id == null || l2Name.isEmpty) continue;

            newIdMap[l2Id] = rootKey;
            newCatIdToL2[l2Id] = l2Name;
            newL2NameToId[l2Name] = l2Id;
            l2NameList.add(l2Name);

            final l3nodes = (l2['children'] as List? ?? [])
                .whereType<Map>()
                .map((e) => e.cast<String, dynamic>())
                .toList();

            final l3NameList = <String>[];
            for (final l3 in l3nodes) {
              final l3Id = getId(l3);
              final l3Name = l3['name']?.toString().trim() ?? '';
              if (l3Id != null && l3Name.isNotEmpty) {
                newIdMap[l3Id] = rootKey;
                newCatIdToL2[l3Id] = l2Name;
                l3NameList.add(l3Name);
                newCatIdToL3Name[l3Id] = l3Name;
                newL3NameToId[l3Name] = l3Id;
              }
            }
            if (l3NameList.isNotEmpty) newL3ByL2[l2Name] = l3NameList;
          }
          if (l2NameList.isNotEmpty) newSubByMajor[rootKey] = l2NameList;
        }

        majors.value = newMajors.isNotEmpty
            ? newMajors
            : List<String>.from(_defaultTopTabs);
        subByMajor.value = newSubByMajor;
        l3ByL2.value = newL3ByL2;
        catIdToL2.value = newCatIdToL2;
        catIdToL3Name.value = newCatIdToL3Name;
        l2NameToId.value = newL2NameToId;
        l3NameToId.value = newL3NameToId;
        l1KeyToId.value = newL1KeyToId;
        _categoryIdMap.addAll(newIdMap);
      }
    } catch (e) {
      debugPrint('[ProductPicker] Fetch categories error: $e');
    }

    if (isEditMode &&
        existingProduct != null &&
        (editModeL2.value == null || editModeL3.value == null)) {
      final catId = _toNum(
        existingProduct['category_id'] ??
            existingProduct['categoryId'] ??
            existingProduct['catalogId'],
      )?.toInt();
      if (catId != null && catId > 0) {
        final topKey = _categoryIdMap[catId];
        if (topKey != null) activeTop.value = topKey;
        editModeL2.value ??= catIdToL2.value[catId];
        editModeL3.value ??= catIdToL3Name.value[catId];
      }
    }

    if (context.mounted) {
      rebuildSubcats();
      loading.value = false;
      loadFinished.value = true;
    }
  }
}
