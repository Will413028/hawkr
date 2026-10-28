import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../data/product_listing_model.dart';

part 'product_listing_draft_controller.g.dart';

/// AI 對話助手累積的商品草稿。
///
/// Merge 策略：覆蓋單一欄位；tags / skus 收到新值時整體替換。
/// patchVersion 每次 merge 遞增，各 step screen 用它當 useEffect 的 dep。
@riverpod
class ProductListingDraftController extends _$ProductListingDraftController {
  @override
  PendingAiDraft build() => const PendingAiDraft();

  void mergePatch(Map<String, dynamic> patch) {
    final next = state;
    final filled = {...next.filledFields};

    String? nextName = next.name;
    if (patch['name'] is String && (patch['name'] as String).isNotEmpty) {
      nextName = patch['name'] as String;
      filled.add('name');
    }

    String? nextOrigin = next.origin;
    if (patch['origin'] is String && (patch['origin'] as String).isNotEmpty) {
      nextOrigin = patch['origin'] as String;
      filled.add('origin');
    }

    String? nextCategoryId = next.categoryId;
    if (patch['categoryId'] is String &&
        (patch['categoryId'] as String).isNotEmpty) {
      nextCategoryId = patch['categoryId'] as String;
      filled.add('category');
    }

    String? nextDescription = next.description;
    if (patch['description'] is String &&
        (patch['description'] as String).isNotEmpty) {
      nextDescription = patch['description'] as String;
      filled.add('description');
    }

    List<String> nextTags = next.tags;
    if (patch['tags'] is List) {
      nextTags = (patch['tags'] as List).whereType<String>().toList();
      filled.add('tags');
    }

    List<SpecDraftPending> nextSkus = next.skus;
    if (patch['skus'] is List) {
      nextSkus = (patch['skus'] as List)
          .whereType<Map<String, dynamic>>()
          .map(SpecDraftPending.fromJson)
          .toList();
      filled.add('skus');
    }

    state = next.copyWith(
      name: nextName,
      origin: nextOrigin,
      categoryId: nextCategoryId,
      description: nextDescription,
      tags: nextTags,
      skus: nextSkus,
      filledFields: filled,
      patchVersion: next.patchVersion + 1,
    );
  }

  void clear() {
    state = const PendingAiDraft();
  }
}
