// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'payment_account.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(paymentAccountRepository)
final paymentAccountRepositoryProvider = PaymentAccountRepositoryProvider._();

final class PaymentAccountRepositoryProvider
    extends
        $FunctionalProvider<
          PaymentAccountRepository,
          PaymentAccountRepository,
          PaymentAccountRepository
        >
    with $Provider<PaymentAccountRepository> {
  PaymentAccountRepositoryProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'paymentAccountRepositoryProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$paymentAccountRepositoryHash();

  @$internal
  @override
  $ProviderElement<PaymentAccountRepository> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  PaymentAccountRepository create(Ref ref) {
    return paymentAccountRepository(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(PaymentAccountRepository value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<PaymentAccountRepository>(value),
    );
  }
}

String _$paymentAccountRepositoryHash() =>
    r'cdbb59b630fdb0b289bcebf05cdf7ccdf4d5f975';

/// [accountId] 這個帳號的收款帳戶（`GetMyPaymentAccount`）。還沒有帳戶是 null；其他失敗是 error 狀態，不當成沒有帳戶。
/// 以帳號為鍵，換帳號不會看到上一個帳號的結果。

@ProviderFor(accountPaymentAccount)
final accountPaymentAccountProvider = AccountPaymentAccountFamily._();

/// [accountId] 這個帳號的收款帳戶（`GetMyPaymentAccount`）。還沒有帳戶是 null；其他失敗是 error 狀態，不當成沒有帳戶。
/// 以帳號為鍵，換帳號不會看到上一個帳號的結果。

final class AccountPaymentAccountProvider
    extends
        $FunctionalProvider<
          AsyncValue<PaymentAccount?>,
          PaymentAccount?,
          FutureOr<PaymentAccount?>
        >
    with $FutureModifier<PaymentAccount?>, $FutureProvider<PaymentAccount?> {
  /// [accountId] 這個帳號的收款帳戶（`GetMyPaymentAccount`）。還沒有帳戶是 null；其他失敗是 error 狀態，不當成沒有帳戶。
  /// 以帳號為鍵，換帳號不會看到上一個帳號的結果。
  AccountPaymentAccountProvider._({
    required AccountPaymentAccountFamily super.from,
    required String super.argument,
  }) : super(
         retry: noRetry,
         name: r'accountPaymentAccountProvider',
         isAutoDispose: false,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$accountPaymentAccountHash();

  @override
  String toString() {
    return r'accountPaymentAccountProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $FutureProviderElement<PaymentAccount?> $createElement(
    $ProviderPointer pointer,
  ) => $FutureProviderElement(pointer);

  @override
  FutureOr<PaymentAccount?> create(Ref ref) {
    final argument = this.argument as String;
    return accountPaymentAccount(ref, argument);
  }

  @override
  bool operator ==(Object other) {
    return other is AccountPaymentAccountProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$accountPaymentAccountHash() =>
    r'c46af1a6826617047bd8a681e07f4662393bf3fe';

/// [accountId] 這個帳號的收款帳戶（`GetMyPaymentAccount`）。還沒有帳戶是 null；其他失敗是 error 狀態，不當成沒有帳戶。
/// 以帳號為鍵，換帳號不會看到上一個帳號的結果。

final class AccountPaymentAccountFamily extends $Family
    with $FunctionalFamilyOverride<FutureOr<PaymentAccount?>, String> {
  AccountPaymentAccountFamily._()
    : super(
        retry: noRetry,
        name: r'accountPaymentAccountProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: false,
      );

  /// [accountId] 這個帳號的收款帳戶（`GetMyPaymentAccount`）。還沒有帳戶是 null；其他失敗是 error 狀態，不當成沒有帳戶。
  /// 以帳號為鍵，換帳號不會看到上一個帳號的結果。

  AccountPaymentAccountProvider call(String accountId) =>
      AccountPaymentAccountProvider._(argument: accountId, from: this);

  @override
  String toString() => r'accountPaymentAccountProvider';
}

/// 目前登入帳號的收款帳戶；沒有登入時是 null（外層）。

@ProviderFor(currentPaymentAccount)
final currentPaymentAccountProvider = CurrentPaymentAccountProvider._();

/// 目前登入帳號的收款帳戶；沒有登入時是 null（外層）。

final class CurrentPaymentAccountProvider
    extends
        $FunctionalProvider<
          AsyncValue<PaymentAccount?>?,
          AsyncValue<PaymentAccount?>?,
          AsyncValue<PaymentAccount?>?
        >
    with $Provider<AsyncValue<PaymentAccount?>?> {
  /// 目前登入帳號的收款帳戶；沒有登入時是 null（外層）。
  CurrentPaymentAccountProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'currentPaymentAccountProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$currentPaymentAccountHash();

  @$internal
  @override
  $ProviderElement<AsyncValue<PaymentAccount?>?> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  AsyncValue<PaymentAccount?>? create(Ref ref) {
    return currentPaymentAccount(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(AsyncValue<PaymentAccount?>? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<AsyncValue<PaymentAccount?>?>(value),
    );
  }
}

String _$currentPaymentAccountHash() =>
    r'14d55c2d66e99ff1cfcd8f7db091589ca4f669cd';

/// 重讀目前帳號的收款帳戶（`GetMyPaymentAccount`）。

@ProviderFor(PaymentAccountReader)
final paymentAccountReaderProvider = PaymentAccountReaderProvider._();

/// 重讀目前帳號的收款帳戶（`GetMyPaymentAccount`）。
final class PaymentAccountReaderProvider
    extends $NotifierProvider<PaymentAccountReader, void> {
  /// 重讀目前帳號的收款帳戶（`GetMyPaymentAccount`）。
  PaymentAccountReaderProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'paymentAccountReaderProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$paymentAccountReaderHash();

  @$internal
  @override
  PaymentAccountReader create() => PaymentAccountReader();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(void value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<void>(value),
    );
  }
}

String _$paymentAccountReaderHash() =>
    r'b8590d621a4fa44ed31b0cf40db41d6f5937dfa3';

/// 重讀目前帳號的收款帳戶（`GetMyPaymentAccount`）。

abstract class _$PaymentAccountReader extends $Notifier<void> {
  void build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<void, void>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<void, void>,
              void,
              Object?,
              Object?
            >;
    element.handleCreate(ref, build);
  }
}
