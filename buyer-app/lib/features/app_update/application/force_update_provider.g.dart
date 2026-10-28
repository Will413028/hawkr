// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'force_update_provider.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// 版本門檻的結果，供 GoRouter redirect 讀取（`packages/hawkr_app_update/docs/spec.md`）；攔截一律由 router redirect 執行。
///
/// 檢查的規則（只在 iOS、3 秒逾時、失敗一律放行、回到前景的節流）在兩支 App 共用的
/// `hawkr_app_update`，這裡只把結果放進 state。CheckAppVersion 是公開的 service，不帶身分：token 過期時
/// 不會觸發 refresh。

@ProviderFor(ForceUpdate)
final forceUpdateProvider = ForceUpdateProvider._();

/// 版本門檻的結果，供 GoRouter redirect 讀取（`packages/hawkr_app_update/docs/spec.md`）；攔截一律由 router redirect 執行。
///
/// 檢查的規則（只在 iOS、3 秒逾時、失敗一律放行、回到前景的節流）在兩支 App 共用的
/// `hawkr_app_update`，這裡只把結果放進 state。CheckAppVersion 是公開的 service，不帶身分：token 過期時
/// 不會觸發 refresh。
final class ForceUpdateProvider
    extends $NotifierProvider<ForceUpdate, CheckAppVersionResponse> {
  /// 版本門檻的結果，供 GoRouter redirect 讀取（`packages/hawkr_app_update/docs/spec.md`）；攔截一律由 router redirect 執行。
  ///
  /// 檢查的規則（只在 iOS、3 秒逾時、失敗一律放行、回到前景的節流）在兩支 App 共用的
  /// `hawkr_app_update`，這裡只把結果放進 state。CheckAppVersion 是公開的 service，不帶身分：token 過期時
  /// 不會觸發 refresh。
  ForceUpdateProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'forceUpdateProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$forceUpdateHash();

  @$internal
  @override
  ForceUpdate create() => ForceUpdate();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(CheckAppVersionResponse value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<CheckAppVersionResponse>(value),
    );
  }
}

String _$forceUpdateHash() => r'a0d4cc1736bcd1f796a98ccf7087dd43f60bc0b2';

/// 版本門檻的結果，供 GoRouter redirect 讀取（`packages/hawkr_app_update/docs/spec.md`）；攔截一律由 router redirect 執行。
///
/// 檢查的規則（只在 iOS、3 秒逾時、失敗一律放行、回到前景的節流）在兩支 App 共用的
/// `hawkr_app_update`，這裡只把結果放進 state。CheckAppVersion 是公開的 service，不帶身分：token 過期時
/// 不會觸發 refresh。

abstract class _$ForceUpdate extends $Notifier<CheckAppVersionResponse> {
  CheckAppVersionResponse build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref =
        this.ref as $Ref<CheckAppVersionResponse, CheckAppVersionResponse>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<CheckAppVersionResponse, CheckAppVersionResponse>,
              CheckAppVersionResponse,
              Object?,
              Object?
            >;
    element.handleCreate(ref, build);
  }
}
