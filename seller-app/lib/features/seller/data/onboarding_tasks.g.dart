// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'onboarding_tasks.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// 進度卡需要的物流與商品數；各自讀取，失敗的部分保守處理。

@ProviderFor(onboardingInputs)
final onboardingInputsProvider = OnboardingInputsProvider._();

/// 進度卡需要的物流與商品數；各自讀取，失敗的部分保守處理。

final class OnboardingInputsProvider
    extends
        $FunctionalProvider<
          AsyncValue<OnboardingInputs>,
          OnboardingInputs,
          FutureOr<OnboardingInputs>
        >
    with $FutureModifier<OnboardingInputs>, $FutureProvider<OnboardingInputs> {
  /// 進度卡需要的物流與商品數；各自讀取，失敗的部分保守處理。
  OnboardingInputsProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'onboardingInputsProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$onboardingInputsHash();

  @$internal
  @override
  $FutureProviderElement<OnboardingInputs> $createElement(
    $ProviderPointer pointer,
  ) => $FutureProviderElement(pointer);

  @override
  FutureOr<OnboardingInputs> create(Ref ref) {
    return onboardingInputs(ref);
  }
}

String _$onboardingInputsHash() => r'8f0536990b8cf38a25677f74be70c816efbf52c3';
