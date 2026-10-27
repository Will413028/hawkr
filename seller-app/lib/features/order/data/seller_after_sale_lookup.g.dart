// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'seller_after_sale_lookup.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// 快取「orderNo → 售後案件」的 Map，供賣家訂單卡片查詢與展開區顯示。
///
/// 使用 Map 而非 Set：卡片的 `OrderFlagBanner` / `OrderBottomRow` 只需判斷
/// `containsKey(orderNo)`，`OrderCard` 展開區則能直接取得 [AfterSaleCase]
/// 當作摘要資料，避免每張卡片再發一次 detail 請求。
///
/// 排除 60 已拒絕 / 70 已取消 / 71 確認收貨取消（沒走完整退貨退款流程，展開區渲染會誤導）。
/// 50 已結案保留 — 已完成 tab 的訂單需要展開看完整售後 timeline。

@ProviderFor(SellerAfterSaleLookup)
final sellerAfterSaleLookupProvider = SellerAfterSaleLookupProvider._();

/// 快取「orderNo → 售後案件」的 Map，供賣家訂單卡片查詢與展開區顯示。
///
/// 使用 Map 而非 Set：卡片的 `OrderFlagBanner` / `OrderBottomRow` 只需判斷
/// `containsKey(orderNo)`，`OrderCard` 展開區則能直接取得 [AfterSaleCase]
/// 當作摘要資料，避免每張卡片再發一次 detail 請求。
///
/// 排除 60 已拒絕 / 70 已取消 / 71 確認收貨取消（沒走完整退貨退款流程，展開區渲染會誤導）。
/// 50 已結案保留 — 已完成 tab 的訂單需要展開看完整售後 timeline。
final class SellerAfterSaleLookupProvider
    extends
        $AsyncNotifierProvider<
          SellerAfterSaleLookup,
          Map<String, AfterSaleCase>
        > {
  /// 快取「orderNo → 售後案件」的 Map，供賣家訂單卡片查詢與展開區顯示。
  ///
  /// 使用 Map 而非 Set：卡片的 `OrderFlagBanner` / `OrderBottomRow` 只需判斷
  /// `containsKey(orderNo)`，`OrderCard` 展開區則能直接取得 [AfterSaleCase]
  /// 當作摘要資料，避免每張卡片再發一次 detail 請求。
  ///
  /// 排除 60 已拒絕 / 70 已取消 / 71 確認收貨取消（沒走完整退貨退款流程，展開區渲染會誤導）。
  /// 50 已結案保留 — 已完成 tab 的訂單需要展開看完整售後 timeline。
  SellerAfterSaleLookupProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'sellerAfterSaleLookupProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$sellerAfterSaleLookupHash();

  @$internal
  @override
  SellerAfterSaleLookup create() => SellerAfterSaleLookup();
}

String _$sellerAfterSaleLookupHash() =>
    r'626c29c1c4be96156fe77c6a031ad69f2a09c110';

/// 快取「orderNo → 售後案件」的 Map，供賣家訂單卡片查詢與展開區顯示。
///
/// 使用 Map 而非 Set：卡片的 `OrderFlagBanner` / `OrderBottomRow` 只需判斷
/// `containsKey(orderNo)`，`OrderCard` 展開區則能直接取得 [AfterSaleCase]
/// 當作摘要資料，避免每張卡片再發一次 detail 請求。
///
/// 排除 60 已拒絕 / 70 已取消 / 71 確認收貨取消（沒走完整退貨退款流程，展開區渲染會誤導）。
/// 50 已結案保留 — 已完成 tab 的訂單需要展開看完整售後 timeline。

abstract class _$SellerAfterSaleLookup
    extends $AsyncNotifier<Map<String, AfterSaleCase>> {
  FutureOr<Map<String, AfterSaleCase>> build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref =
        this.ref
            as $Ref<
              AsyncValue<Map<String, AfterSaleCase>>,
              Map<String, AfterSaleCase>
            >;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<
                AsyncValue<Map<String, AfterSaleCase>>,
                Map<String, AfterSaleCase>
              >,
              AsyncValue<Map<String, AfterSaleCase>>,
              Object?,
              Object?
            >;
    element.handleCreate(ref, build);
  }
}
