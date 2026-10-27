// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'current_store.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// 目前登入帳號的店家（`GetMyStore`）：伺服器端的狀態獨立於 session（session 只管登入的身分）。
///
/// 值是 `null` 表示沒有登入；帳號改變時自動重讀，舊帳號的讀取結果與較舊的讀取都不會寫進來。
/// 重讀失敗時保留同一帳號上一次成功的快照。

@ProviderFor(CurrentStore)
final currentStoreProvider = CurrentStoreProvider._();

/// 目前登入帳號的店家（`GetMyStore`）：伺服器端的狀態獨立於 session（session 只管登入的身分）。
///
/// 值是 `null` 表示沒有登入；帳號改變時自動重讀，舊帳號的讀取結果與較舊的讀取都不會寫進來。
/// 重讀失敗時保留同一帳號上一次成功的快照。
final class CurrentStoreProvider
    extends $AsyncNotifierProvider<CurrentStore, MyStoreResult?> {
  /// 目前登入帳號的店家（`GetMyStore`）：伺服器端的狀態獨立於 session（session 只管登入的身分）。
  ///
  /// 值是 `null` 表示沒有登入；帳號改變時自動重讀，舊帳號的讀取結果與較舊的讀取都不會寫進來。
  /// 重讀失敗時保留同一帳號上一次成功的快照。
  CurrentStoreProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'currentStoreProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$currentStoreHash();

  @$internal
  @override
  CurrentStore create() => CurrentStore();
}

String _$currentStoreHash() => r'a9059255bc1546015ea5b8dc08cf02b4f51ed842';

/// 目前登入帳號的店家（`GetMyStore`）：伺服器端的狀態獨立於 session（session 只管登入的身分）。
///
/// 值是 `null` 表示沒有登入；帳號改變時自動重讀，舊帳號的讀取結果與較舊的讀取都不會寫進來。
/// 重讀失敗時保留同一帳號上一次成功的快照。

abstract class _$CurrentStore extends $AsyncNotifier<MyStoreResult?> {
  FutureOr<MyStoreResult?> build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<AsyncValue<MyStoreResult?>, MyStoreResult?>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<AsyncValue<MyStoreResult?>, MyStoreResult?>,
              AsyncValue<MyStoreResult?>,
              Object?,
              Object?
            >;
    element.handleCreate(ref, build);
  }
}

/// 最近一次成功的店家快照；還沒讀到、讀取失敗過而沒有快照、沒有登入時是 null（不代表沒有店家）。

@ProviderFor(myStoreSnapshot)
final myStoreSnapshotProvider = MyStoreSnapshotProvider._();

/// 最近一次成功的店家快照；還沒讀到、讀取失敗過而沒有快照、沒有登入時是 null（不代表沒有店家）。

final class MyStoreSnapshotProvider
    extends $FunctionalProvider<MyStore?, MyStore?, MyStore?>
    with $Provider<MyStore?> {
  /// 最近一次成功的店家快照；還沒讀到、讀取失敗過而沒有快照、沒有登入時是 null（不代表沒有店家）。
  MyStoreSnapshotProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'myStoreSnapshotProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$myStoreSnapshotHash();

  @$internal
  @override
  $ProviderElement<MyStore?> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  MyStore? create(Ref ref) {
    return myStoreSnapshot(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(MyStore? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<MyStore?>(value),
    );
  }
}

String _$myStoreSnapshotHash() => r'cceff01426da9330a97d261bcb1315be424b5d36';

/// 店家 ID；沒有店家或還沒讀到時是 null。

@ProviderFor(sellerStoreId)
final sellerStoreIdProvider = SellerStoreIdProvider._();

/// 店家 ID；沒有店家或還沒讀到時是 null。

final class SellerStoreIdProvider
    extends $FunctionalProvider<String?, String?, String?>
    with $Provider<String?> {
  /// 店家 ID；沒有店家或還沒讀到時是 null。
  SellerStoreIdProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'sellerStoreIdProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$sellerStoreIdHash();

  @$internal
  @override
  $ProviderElement<String?> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  String? create(Ref ref) {
    return sellerStoreId(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(String? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<String?>(value),
    );
  }
}

String _$sellerStoreIdHash() => r'9bd9484f4f3e945791bbd529b277aa459f02df1c';

/// 需要完成賣家 onboarding（沒有店家，或店家沒有完成新手引導）。`null` 表示還不知道：還在讀，或讀取失敗而沒有快照。

@ProviderFor(needsOnboarding)
final needsOnboardingProvider = NeedsOnboardingProvider._();

/// 需要完成賣家 onboarding（沒有店家，或店家沒有完成新手引導）。`null` 表示還不知道：還在讀，或讀取失敗而沒有快照。

final class NeedsOnboardingProvider
    extends $FunctionalProvider<bool?, bool?, bool?>
    with $Provider<bool?> {
  /// 需要完成賣家 onboarding（沒有店家，或店家沒有完成新手引導）。`null` 表示還不知道：還在讀，或讀取失敗而沒有快照。
  NeedsOnboardingProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'needsOnboardingProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$needsOnboardingHash();

  @$internal
  @override
  $ProviderElement<bool?> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  bool? create(Ref ref) {
    return needsOnboarding(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(bool? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<bool?>(value),
    );
  }
}

String _$needsOnboardingHash() => r'9e66c9d57e4d7bee21957547576dd4a26aa31a39';

/// 所有分享入口與「店家公開後…」提示共用的唯一判斷。

@ProviderFor(storeShare)
final storeShareProvider = StoreShareProvider._();

/// 所有分享入口與「店家公開後…」提示共用的唯一判斷。

final class StoreShareProvider
    extends $FunctionalProvider<StoreShare, StoreShare, StoreShare>
    with $Provider<StoreShare> {
  /// 所有分享入口與「店家公開後…」提示共用的唯一判斷。
  StoreShareProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'storeShareProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$storeShareHash();

  @$internal
  @override
  $ProviderElement<StoreShare> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  StoreShare create(Ref ref) {
    return storeShare(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(StoreShare value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<StoreShare>(value),
    );
  }
}

String _$storeShareHash() => r'28fbeca69f49fd1d78d93d34df356be793d648f2';

/// 讀不到店家、也沒有上一次的快照：不知道有沒有店家，router 顯示重試而不是主畫面。

@ProviderFor(storeUnavailable)
final storeUnavailableProvider = StoreUnavailableProvider._();

/// 讀不到店家、也沒有上一次的快照：不知道有沒有店家，router 顯示重試而不是主畫面。

final class StoreUnavailableProvider
    extends $FunctionalProvider<bool, bool, bool>
    with $Provider<bool> {
  /// 讀不到店家、也沒有上一次的快照：不知道有沒有店家，router 顯示重試而不是主畫面。
  StoreUnavailableProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'storeUnavailableProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$storeUnavailableHash();

  @$internal
  @override
  $ProviderElement<bool> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  bool create(Ref ref) {
    return storeUnavailable(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(bool value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<bool>(value),
    );
  }
}

String _$storeUnavailableHash() => r'089d6a0bbafbbc96657d5293061c07c0576b1a5a';
