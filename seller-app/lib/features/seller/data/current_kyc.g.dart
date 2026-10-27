// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'current_kyc.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// [accountId] 這個帳號的認證流程狀態（`GetMyKYC`）。not_found 是 [MyKyc.notStarted]；其他失敗是 error 狀態，
/// 不當成沒開始認證。以帳號為鍵，換帳號不會看到上一個帳號的結果。申請的唯一來源：寫入的回應用 [adopt] 直接寫進來。

@ProviderFor(AccountKyc)
final accountKycProvider = AccountKycFamily._();

/// [accountId] 這個帳號的認證流程狀態（`GetMyKYC`）。not_found 是 [MyKyc.notStarted]；其他失敗是 error 狀態，
/// 不當成沒開始認證。以帳號為鍵，換帳號不會看到上一個帳號的結果。申請的唯一來源：寫入的回應用 [adopt] 直接寫進來。
final class AccountKycProvider
    extends $AsyncNotifierProvider<AccountKyc, MyKyc> {
  /// [accountId] 這個帳號的認證流程狀態（`GetMyKYC`）。not_found 是 [MyKyc.notStarted]；其他失敗是 error 狀態，
  /// 不當成沒開始認證。以帳號為鍵，換帳號不會看到上一個帳號的結果。申請的唯一來源：寫入的回應用 [adopt] 直接寫進來。
  AccountKycProvider._({
    required AccountKycFamily super.from,
    required String super.argument,
  }) : super(
         retry: noRetry,
         name: r'accountKycProvider',
         isAutoDispose: false,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$accountKycHash();

  @override
  String toString() {
    return r'accountKycProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  AccountKyc create() => AccountKyc();

  @override
  bool operator ==(Object other) {
    return other is AccountKycProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$accountKycHash() => r'f253aa70597d9f975931f77aff467902cf7a419e';

/// [accountId] 這個帳號的認證流程狀態（`GetMyKYC`）。not_found 是 [MyKyc.notStarted]；其他失敗是 error 狀態，
/// 不當成沒開始認證。以帳號為鍵，換帳號不會看到上一個帳號的結果。申請的唯一來源：寫入的回應用 [adopt] 直接寫進來。

final class AccountKycFamily extends $Family
    with
        $ClassFamilyOverride<
          AccountKyc,
          AsyncValue<MyKyc>,
          MyKyc,
          FutureOr<MyKyc>,
          String
        > {
  AccountKycFamily._()
    : super(
        retry: noRetry,
        name: r'accountKycProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: false,
      );

  /// [accountId] 這個帳號的認證流程狀態（`GetMyKYC`）。not_found 是 [MyKyc.notStarted]；其他失敗是 error 狀態，
  /// 不當成沒開始認證。以帳號為鍵，換帳號不會看到上一個帳號的結果。申請的唯一來源：寫入的回應用 [adopt] 直接寫進來。

  AccountKycProvider call(String accountId) =>
      AccountKycProvider._(argument: accountId, from: this);

  @override
  String toString() => r'accountKycProvider';
}

/// [accountId] 這個帳號的認證流程狀態（`GetMyKYC`）。not_found 是 [MyKyc.notStarted]；其他失敗是 error 狀態，
/// 不當成沒開始認證。以帳號為鍵，換帳號不會看到上一個帳號的結果。申請的唯一來源：寫入的回應用 [adopt] 直接寫進來。

abstract class _$AccountKyc extends $AsyncNotifier<MyKyc> {
  late final _$args = ref.$arg as String;
  String get accountId => _$args;

  FutureOr<MyKyc> build(String accountId);
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<AsyncValue<MyKyc>, MyKyc>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<AsyncValue<MyKyc>, MyKyc>,
              AsyncValue<MyKyc>,
              Object?,
              Object?
            >;
    element.handleCreate(ref, () => build(_$args));
  }
}

/// 目前登入帳號的認證流程狀態；沒有登入時是 null。

@ProviderFor(currentKyc)
final currentKycProvider = CurrentKycProvider._();

/// 目前登入帳號的認證流程狀態；沒有登入時是 null。

final class CurrentKycProvider
    extends
        $FunctionalProvider<
          AsyncValue<MyKyc>?,
          AsyncValue<MyKyc>?,
          AsyncValue<MyKyc>?
        >
    with $Provider<AsyncValue<MyKyc>?> {
  /// 目前登入帳號的認證流程狀態；沒有登入時是 null。
  CurrentKycProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'currentKycProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$currentKycHash();

  @$internal
  @override
  $ProviderElement<AsyncValue<MyKyc>?> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  AsyncValue<MyKyc>? create(Ref ref) {
    return currentKyc(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(AsyncValue<MyKyc>? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<AsyncValue<MyKyc>?>(value),
    );
  }
}

String _$currentKycHash() => r'5736ddedccbf7484c3ba159cb1e89bd27f9b138e';

/// 重讀目前帳號的認證流程狀態（`GetMyKYC`）。

@ProviderFor(KycReader)
final kycReaderProvider = KycReaderProvider._();

/// 重讀目前帳號的認證流程狀態（`GetMyKYC`）。
final class KycReaderProvider extends $NotifierProvider<KycReader, void> {
  /// 重讀目前帳號的認證流程狀態（`GetMyKYC`）。
  KycReaderProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'kycReaderProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$kycReaderHash();

  @$internal
  @override
  KycReader create() => KycReader();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(void value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<void>(value),
    );
  }
}

String _$kycReaderHash() => r'c060b6696210a7f3a9ee1c8167f8de490fd1d371';

/// 重讀目前帳號的認證流程狀態（`GetMyKYC`）。

abstract class _$KycReader extends $Notifier<void> {
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
