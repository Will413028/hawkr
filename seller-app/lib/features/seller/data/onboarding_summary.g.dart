// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'onboarding_summary.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// [accountId] 這個帳號的開通摘要（`GetMyOnboarding`）：必要任務的狀態、下一步與文字都由伺服器推導，這裡不組合其他讀取。
/// 失敗是 error 狀態，不當成沒有任務。以帳號為鍵，換帳號不會看到上一個帳號的結果。

@ProviderFor(accountOnboarding)
final accountOnboardingProvider = AccountOnboardingFamily._();

/// [accountId] 這個帳號的開通摘要（`GetMyOnboarding`）：必要任務的狀態、下一步與文字都由伺服器推導，這裡不組合其他讀取。
/// 失敗是 error 狀態，不當成沒有任務。以帳號為鍵，換帳號不會看到上一個帳號的結果。

final class AccountOnboardingProvider
    extends
        $FunctionalProvider<
          AsyncValue<GetMyOnboardingResponse>,
          GetMyOnboardingResponse,
          FutureOr<GetMyOnboardingResponse>
        >
    with
        $FutureModifier<GetMyOnboardingResponse>,
        $FutureProvider<GetMyOnboardingResponse> {
  /// [accountId] 這個帳號的開通摘要（`GetMyOnboarding`）：必要任務的狀態、下一步與文字都由伺服器推導，這裡不組合其他讀取。
  /// 失敗是 error 狀態，不當成沒有任務。以帳號為鍵，換帳號不會看到上一個帳號的結果。
  AccountOnboardingProvider._({
    required AccountOnboardingFamily super.from,
    required String super.argument,
  }) : super(
         retry: noRetry,
         name: r'accountOnboardingProvider',
         isAutoDispose: false,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$accountOnboardingHash();

  @override
  String toString() {
    return r'accountOnboardingProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $FutureProviderElement<GetMyOnboardingResponse> $createElement(
    $ProviderPointer pointer,
  ) => $FutureProviderElement(pointer);

  @override
  FutureOr<GetMyOnboardingResponse> create(Ref ref) {
    final argument = this.argument as String;
    return accountOnboarding(ref, argument);
  }

  @override
  bool operator ==(Object other) {
    return other is AccountOnboardingProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$accountOnboardingHash() => r'f85a8010c289efe80b7fd320b945921b23706320';

/// [accountId] 這個帳號的開通摘要（`GetMyOnboarding`）：必要任務的狀態、下一步與文字都由伺服器推導，這裡不組合其他讀取。
/// 失敗是 error 狀態，不當成沒有任務。以帳號為鍵，換帳號不會看到上一個帳號的結果。

final class AccountOnboardingFamily extends $Family
    with $FunctionalFamilyOverride<FutureOr<GetMyOnboardingResponse>, String> {
  AccountOnboardingFamily._()
    : super(
        retry: noRetry,
        name: r'accountOnboardingProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: false,
      );

  /// [accountId] 這個帳號的開通摘要（`GetMyOnboarding`）：必要任務的狀態、下一步與文字都由伺服器推導，這裡不組合其他讀取。
  /// 失敗是 error 狀態，不當成沒有任務。以帳號為鍵，換帳號不會看到上一個帳號的結果。

  AccountOnboardingProvider call(String accountId) =>
      AccountOnboardingProvider._(argument: accountId, from: this);

  @override
  String toString() => r'accountOnboardingProvider';
}

/// 目前登入帳號的開通摘要；沒有登入時是 null（外層）。

@ProviderFor(currentOnboarding)
final currentOnboardingProvider = CurrentOnboardingProvider._();

/// 目前登入帳號的開通摘要；沒有登入時是 null（外層）。

final class CurrentOnboardingProvider
    extends
        $FunctionalProvider<
          AsyncValue<GetMyOnboardingResponse>?,
          AsyncValue<GetMyOnboardingResponse>?,
          AsyncValue<GetMyOnboardingResponse>?
        >
    with $Provider<AsyncValue<GetMyOnboardingResponse>?> {
  /// 目前登入帳號的開通摘要；沒有登入時是 null（外層）。
  CurrentOnboardingProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'currentOnboardingProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$currentOnboardingHash();

  @$internal
  @override
  $ProviderElement<AsyncValue<GetMyOnboardingResponse>?> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  AsyncValue<GetMyOnboardingResponse>? create(Ref ref) {
    return currentOnboarding(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(AsyncValue<GetMyOnboardingResponse>? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride:
          $SyncValueProvider<AsyncValue<GetMyOnboardingResponse>?>(value),
    );
  }
}

String _$currentOnboardingHash() => r'f6fcb65c34ef7a344506e3669a99eccbb5c18d99';

/// 重讀目前帳號的開通摘要（`GetMyOnboarding`）。

@ProviderFor(OnboardingSummaryReader)
final onboardingSummaryReaderProvider = OnboardingSummaryReaderProvider._();

/// 重讀目前帳號的開通摘要（`GetMyOnboarding`）。
final class OnboardingSummaryReaderProvider
    extends $NotifierProvider<OnboardingSummaryReader, void> {
  /// 重讀目前帳號的開通摘要（`GetMyOnboarding`）。
  OnboardingSummaryReaderProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'onboardingSummaryReaderProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$onboardingSummaryReaderHash();

  @$internal
  @override
  OnboardingSummaryReader create() => OnboardingSummaryReader();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(void value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<void>(value),
    );
  }
}

String _$onboardingSummaryReaderHash() =>
    r'3487ebf238efbff5b7d0d60b534931eb4a66c6e5';

/// 重讀目前帳號的開通摘要（`GetMyOnboarding`）。

abstract class _$OnboardingSummaryReader extends $Notifier<void> {
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
