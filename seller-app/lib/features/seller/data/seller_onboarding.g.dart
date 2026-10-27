// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'seller_onboarding.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// [accountId] 這個帳號在開店表單送出之前先問伺服器能不能開店（帳號層級；以帳號為鍵，換帳號不會看到上一個帳號的答案）。讀取失敗是 error 狀態，畫面不擋，送出時由 ApplyStore 的
/// ErrorReason 後備。沒有畫面在看時釋放，回到表單時重讀。

@ProviderFor(storeOpeningBlockFor)
final storeOpeningBlockForProvider = StoreOpeningBlockForFamily._();

/// [accountId] 這個帳號在開店表單送出之前先問伺服器能不能開店（帳號層級；以帳號為鍵，換帳號不會看到上一個帳號的答案）。讀取失敗是 error 狀態，畫面不擋，送出時由 ApplyStore 的
/// ErrorReason 後備。沒有畫面在看時釋放，回到表單時重讀。

final class StoreOpeningBlockForProvider
    extends
        $FunctionalProvider<
          AsyncValue<StoreOpeningBlock?>,
          StoreOpeningBlock?,
          FutureOr<StoreOpeningBlock?>
        >
    with
        $FutureModifier<StoreOpeningBlock?>,
        $FutureProvider<StoreOpeningBlock?> {
  /// [accountId] 這個帳號在開店表單送出之前先問伺服器能不能開店（帳號層級；以帳號為鍵，換帳號不會看到上一個帳號的答案）。讀取失敗是 error 狀態，畫面不擋，送出時由 ApplyStore 的
  /// ErrorReason 後備。沒有畫面在看時釋放，回到表單時重讀。
  StoreOpeningBlockForProvider._({
    required StoreOpeningBlockForFamily super.from,
    required String super.argument,
  }) : super(
         retry: noRetry,
         name: r'storeOpeningBlockForProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$storeOpeningBlockForHash();

  @override
  String toString() {
    return r'storeOpeningBlockForProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $FutureProviderElement<StoreOpeningBlock?> $createElement(
    $ProviderPointer pointer,
  ) => $FutureProviderElement(pointer);

  @override
  FutureOr<StoreOpeningBlock?> create(Ref ref) {
    final argument = this.argument as String;
    return storeOpeningBlockFor(ref, argument);
  }

  @override
  bool operator ==(Object other) {
    return other is StoreOpeningBlockForProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$storeOpeningBlockForHash() =>
    r'ea60f1348f2ccb238dbdaeb5fbafdc46c89a9267';

/// [accountId] 這個帳號在開店表單送出之前先問伺服器能不能開店（帳號層級；以帳號為鍵，換帳號不會看到上一個帳號的答案）。讀取失敗是 error 狀態，畫面不擋，送出時由 ApplyStore 的
/// ErrorReason 後備。沒有畫面在看時釋放，回到表單時重讀。

final class StoreOpeningBlockForFamily extends $Family
    with $FunctionalFamilyOverride<FutureOr<StoreOpeningBlock?>, String> {
  StoreOpeningBlockForFamily._()
    : super(
        retry: noRetry,
        name: r'storeOpeningBlockForProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// [accountId] 這個帳號在開店表單送出之前先問伺服器能不能開店（帳號層級；以帳號為鍵，換帳號不會看到上一個帳號的答案）。讀取失敗是 error 狀態，畫面不擋，送出時由 ApplyStore 的
  /// ErrorReason 後備。沒有畫面在看時釋放，回到表單時重讀。

  StoreOpeningBlockForProvider call(String accountId) =>
      StoreOpeningBlockForProvider._(argument: accountId, from: this);

  @override
  String toString() => r'storeOpeningBlockForProvider';
}

@ProviderFor(sellerOnboarding)
final sellerOnboardingProvider = SellerOnboardingProvider._();

final class SellerOnboardingProvider
    extends
        $FunctionalProvider<
          SellerOnboarding,
          SellerOnboarding,
          SellerOnboarding
        >
    with $Provider<SellerOnboarding> {
  SellerOnboardingProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'sellerOnboardingProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$sellerOnboardingHash();

  @$internal
  @override
  $ProviderElement<SellerOnboarding> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  SellerOnboarding create(Ref ref) {
    return sellerOnboarding(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(SellerOnboarding value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<SellerOnboarding>(value),
    );
  }
}

String _$sellerOnboardingHash() => r'b5debbbf559a2e3fce2af40b579c438c1992d819';
