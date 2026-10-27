// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'seller_quote_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(quoteIdempotencyKeyBook)
final quoteIdempotencyKeyBookProvider = QuoteIdempotencyKeyBookProvider._();

final class QuoteIdempotencyKeyBookProvider
    extends
        $FunctionalProvider<
          QuoteIdempotencyKeyBook,
          QuoteIdempotencyKeyBook,
          QuoteIdempotencyKeyBook
        >
    with $Provider<QuoteIdempotencyKeyBook> {
  QuoteIdempotencyKeyBookProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'quoteIdempotencyKeyBookProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$quoteIdempotencyKeyBookHash();

  @$internal
  @override
  $ProviderElement<QuoteIdempotencyKeyBook> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  QuoteIdempotencyKeyBook create(Ref ref) {
    return quoteIdempotencyKeyBook(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(QuoteIdempotencyKeyBook value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<QuoteIdempotencyKeyBook>(value),
    );
  }
}

String _$quoteIdempotencyKeyBookHash() =>
    r'a5b3647765d0a96047fba12ff34cd70da6cb895e';

/// 平台賣家回價表單的狀態機。
///
/// 一個 publication 一份狀態（family key ＝ `publicationId`）。

@ProviderFor(SellerQuoteForm)
final sellerQuoteFormProvider = SellerQuoteFormFamily._();

/// 平台賣家回價表單的狀態機。
///
/// 一個 publication 一份狀態（family key ＝ `publicationId`）。
final class SellerQuoteFormProvider
    extends $AsyncNotifierProvider<SellerQuoteForm, SellerQuoteFormState> {
  /// 平台賣家回價表單的狀態機。
  ///
  /// 一個 publication 一份狀態（family key ＝ `publicationId`）。
  SellerQuoteFormProvider._({
    required SellerQuoteFormFamily super.from,
    required String super.argument,
  }) : super(
         retry: _noAutoRetry,
         name: r'sellerQuoteFormProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$sellerQuoteFormHash();

  @override
  String toString() {
    return r'sellerQuoteFormProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  SellerQuoteForm create() => SellerQuoteForm();

  @override
  bool operator ==(Object other) {
    return other is SellerQuoteFormProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$sellerQuoteFormHash() => r'422fa7bc17050cf1c40e76c74f30d32554812169';

/// 平台賣家回價表單的狀態機。
///
/// 一個 publication 一份狀態（family key ＝ `publicationId`）。

final class SellerQuoteFormFamily extends $Family
    with
        $ClassFamilyOverride<
          SellerQuoteForm,
          AsyncValue<SellerQuoteFormState>,
          SellerQuoteFormState,
          FutureOr<SellerQuoteFormState>,
          String
        > {
  SellerQuoteFormFamily._()
    : super(
        retry: _noAutoRetry,
        name: r'sellerQuoteFormProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// 平台賣家回價表單的狀態機。
  ///
  /// 一個 publication 一份狀態（family key ＝ `publicationId`）。

  SellerQuoteFormProvider call(String publicationId) =>
      SellerQuoteFormProvider._(argument: publicationId, from: this);

  @override
  String toString() => r'sellerQuoteFormProvider';
}

/// 平台賣家回價表單的狀態機。
///
/// 一個 publication 一份狀態（family key ＝ `publicationId`）。

abstract class _$SellerQuoteForm extends $AsyncNotifier<SellerQuoteFormState> {
  late final _$args = ref.$arg as String;
  String get publicationId => _$args;

  FutureOr<SellerQuoteFormState> build(String publicationId);
  @$mustCallSuper
  @override
  void runBuild() {
    final ref =
        this.ref
            as $Ref<AsyncValue<SellerQuoteFormState>, SellerQuoteFormState>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<
                AsyncValue<SellerQuoteFormState>,
                SellerQuoteFormState
              >,
              AsyncValue<SellerQuoteFormState>,
              Object?,
              Object?
            >;
    element.handleCreate(ref, () => build(_$args));
  }
}
