// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'force_update_provider.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// App 強制更新 gate 的單一狀態來源（設計書 54 §4.1）；攔截一律由 GoRouter 頂層 redirect 執行（見
/// `core/router/redirect_logic.dart`）。
///
/// 檢查的規則（只在 iOS、3 秒逾時、失敗一律放行、回到前景的節流）在兩支 App 共用的
/// `hawkr_app_update`，這裡只把結果放進 state。state 是 freeze 過的 CheckAppVersionResponse，有值相等，
/// 結果沒變就不會白白通知 router 重判。CheckAppVersion 是公開的 service，不帶身分。

@ProviderFor(ForceUpdateGate)
final forceUpdateGateProvider = ForceUpdateGateProvider._();

/// App 強制更新 gate 的單一狀態來源（設計書 54 §4.1）；攔截一律由 GoRouter 頂層 redirect 執行（見
/// `core/router/redirect_logic.dart`）。
///
/// 檢查的規則（只在 iOS、3 秒逾時、失敗一律放行、回到前景的節流）在兩支 App 共用的
/// `hawkr_app_update`，這裡只把結果放進 state。state 是 freeze 過的 CheckAppVersionResponse，有值相等，
/// 結果沒變就不會白白通知 router 重判。CheckAppVersion 是公開的 service，不帶身分。
final class ForceUpdateGateProvider
    extends $NotifierProvider<ForceUpdateGate, CheckAppVersionResponse> {
  /// App 強制更新 gate 的單一狀態來源（設計書 54 §4.1）；攔截一律由 GoRouter 頂層 redirect 執行（見
  /// `core/router/redirect_logic.dart`）。
  ///
  /// 檢查的規則（只在 iOS、3 秒逾時、失敗一律放行、回到前景的節流）在兩支 App 共用的
  /// `hawkr_app_update`，這裡只把結果放進 state。state 是 freeze 過的 CheckAppVersionResponse，有值相等，
  /// 結果沒變就不會白白通知 router 重判。CheckAppVersion 是公開的 service，不帶身分。
  ForceUpdateGateProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'forceUpdateGateProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$forceUpdateGateHash();

  @$internal
  @override
  ForceUpdateGate create() => ForceUpdateGate();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(CheckAppVersionResponse value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<CheckAppVersionResponse>(value),
    );
  }
}

String _$forceUpdateGateHash() => r'83ec7b32788c204a3817cd2a13d21fc705cb387b';

/// App 強制更新 gate 的單一狀態來源（設計書 54 §4.1）；攔截一律由 GoRouter 頂層 redirect 執行（見
/// `core/router/redirect_logic.dart`）。
///
/// 檢查的規則（只在 iOS、3 秒逾時、失敗一律放行、回到前景的節流）在兩支 App 共用的
/// `hawkr_app_update`，這裡只把結果放進 state。state 是 freeze 過的 CheckAppVersionResponse，有值相等，
/// 結果沒變就不會白白通知 router 重判。CheckAppVersion 是公開的 service，不帶身分。

abstract class _$ForceUpdateGate extends $Notifier<CheckAppVersionResponse> {
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
