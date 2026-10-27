// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'refresh_clock.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// 重讀伺服器狀態時用的現在時間（`CoalescingRefresher` 的保鮮期用它判斷）；測試換成可控的時鐘。

@ProviderFor(refreshClock)
final refreshClockProvider = RefreshClockProvider._();

/// 重讀伺服器狀態時用的現在時間（`CoalescingRefresher` 的保鮮期用它判斷）；測試換成可控的時鐘。

final class RefreshClockProvider
    extends
        $FunctionalProvider<
          DateTime Function(),
          DateTime Function(),
          DateTime Function()
        >
    with $Provider<DateTime Function()> {
  /// 重讀伺服器狀態時用的現在時間（`CoalescingRefresher` 的保鮮期用它判斷）；測試換成可控的時鐘。
  RefreshClockProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'refreshClockProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$refreshClockHash();

  @$internal
  @override
  $ProviderElement<DateTime Function()> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  DateTime Function() create(Ref ref) {
    return refreshClock(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(DateTime Function() value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<DateTime Function()>(value),
    );
  }
}

String _$refreshClockHash() => r'c0a86d54aa28dc5b5c4f92024bbdd4d28a74f57b';
