// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'seller_agreements_repository.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(sellerAgreementsRepository)
final sellerAgreementsRepositoryProvider =
    SellerAgreementsRepositoryProvider._();

final class SellerAgreementsRepositoryProvider
    extends
        $FunctionalProvider<
          SellerAgreementsRepository,
          SellerAgreementsRepository,
          SellerAgreementsRepository
        >
    with $Provider<SellerAgreementsRepository> {
  SellerAgreementsRepositoryProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'sellerAgreementsRepositoryProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$sellerAgreementsRepositoryHash();

  @$internal
  @override
  $ProviderElement<SellerAgreementsRepository> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  SellerAgreementsRepository create(Ref ref) {
    return sellerAgreementsRepository(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(SellerAgreementsRepository value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<SellerAgreementsRepository>(value),
    );
  }
}

String _$sellerAgreementsRepositoryHash() =>
    r'514b2acbf121c2ad8c025cef2f7295476371d972';

/// 入駐文件的現行版本（一次 `GetSellerAgreements`，開店與送出 KYC 共用）；條款過期後 invalidate 這個 provider 重讀。

@ProviderFor(sellerAgreements)
final sellerAgreementsProvider = SellerAgreementsProvider._();

/// 入駐文件的現行版本（一次 `GetSellerAgreements`，開店與送出 KYC 共用）；條款過期後 invalidate 這個 provider 重讀。

final class SellerAgreementsProvider
    extends
        $FunctionalProvider<
          AsyncValue<SellerAgreementSet>,
          SellerAgreementSet,
          FutureOr<SellerAgreementSet>
        >
    with
        $FutureModifier<SellerAgreementSet>,
        $FutureProvider<SellerAgreementSet> {
  /// 入駐文件的現行版本（一次 `GetSellerAgreements`，開店與送出 KYC 共用）；條款過期後 invalidate 這個 provider 重讀。
  SellerAgreementsProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'sellerAgreementsProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$sellerAgreementsHash();

  @$internal
  @override
  $FutureProviderElement<SellerAgreementSet> $createElement(
    $ProviderPointer pointer,
  ) => $FutureProviderElement(pointer);

  @override
  FutureOr<SellerAgreementSet> create(Ref ref) {
    return sellerAgreements(ref);
  }
}

String _$sellerAgreementsHash() => r'bb7708199c24dcabb61d5eee172697d03254929c';

/// 賣家契約（開店時同意）；取自 [sellerAgreementsProvider]。

@ProviderFor(sellerTerms)
final sellerTermsProvider = SellerTermsProvider._();

/// 賣家契約（開店時同意）；取自 [sellerAgreementsProvider]。

final class SellerTermsProvider
    extends
        $FunctionalProvider<
          AsyncValue<SellerAgreement>,
          SellerAgreement,
          FutureOr<SellerAgreement>
        >
    with $FutureModifier<SellerAgreement>, $FutureProvider<SellerAgreement> {
  /// 賣家契約（開店時同意）；取自 [sellerAgreementsProvider]。
  SellerTermsProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'sellerTermsProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$sellerTermsHash();

  @$internal
  @override
  $FutureProviderElement<SellerAgreement> $createElement(
    $ProviderPointer pointer,
  ) => $FutureProviderElement(pointer);

  @override
  FutureOr<SellerAgreement> create(Ref ref) {
    return sellerTerms(ref);
  }
}

String _$sellerTermsHash() => r'b2562fd66667fad709818a6b6ccc57c3a8e5451c';

/// 送出 KYC 要確認的兩份文件；取自 [sellerAgreementsProvider]。

@ProviderFor(kycAgreements)
final kycAgreementsProvider = KycAgreementsProvider._();

/// 送出 KYC 要確認的兩份文件；取自 [sellerAgreementsProvider]。

final class KycAgreementsProvider
    extends
        $FunctionalProvider<
          AsyncValue<KycAgreements>,
          KycAgreements,
          FutureOr<KycAgreements>
        >
    with $FutureModifier<KycAgreements>, $FutureProvider<KycAgreements> {
  /// 送出 KYC 要確認的兩份文件；取自 [sellerAgreementsProvider]。
  KycAgreementsProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'kycAgreementsProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$kycAgreementsHash();

  @$internal
  @override
  $FutureProviderElement<KycAgreements> $createElement(
    $ProviderPointer pointer,
  ) => $FutureProviderElement(pointer);

  @override
  FutureOr<KycAgreements> create(Ref ref) {
    return kycAgreements(ref);
  }
}

String _$kycAgreementsHash() => r'f44bb1aac07f072bc96e1adb58d63064d5241bd4';

/// 送出食品業者登錄要確認的告知；取自 [sellerAgreementsProvider]。

@ProviderFor(foodRegistrationNotice)
final foodRegistrationNoticeProvider = FoodRegistrationNoticeProvider._();

/// 送出食品業者登錄要確認的告知；取自 [sellerAgreementsProvider]。

final class FoodRegistrationNoticeProvider
    extends
        $FunctionalProvider<
          AsyncValue<SellerAgreement>,
          SellerAgreement,
          FutureOr<SellerAgreement>
        >
    with $FutureModifier<SellerAgreement>, $FutureProvider<SellerAgreement> {
  /// 送出食品業者登錄要確認的告知；取自 [sellerAgreementsProvider]。
  FoodRegistrationNoticeProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'foodRegistrationNoticeProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$foodRegistrationNoticeHash();

  @$internal
  @override
  $FutureProviderElement<SellerAgreement> $createElement(
    $ProviderPointer pointer,
  ) => $FutureProviderElement(pointer);

  @override
  FutureOr<SellerAgreement> create(Ref ref) {
    return foodRegistrationNotice(ref);
  }
}

String _$foodRegistrationNoticeHash() =>
    r'2fcb8263eb093c85beb1d2f232c7c8de617b958b';
