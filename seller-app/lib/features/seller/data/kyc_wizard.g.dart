// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'kyc_wizard.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// 認證 wizard 的動作與讀取：草稿、證件、統編查詢，以及寫進店家的聯絡資料與地址。
///
/// 表單是整份字串狀態，伺服器版本是 baseline；每次存檔只送和 baseline 不同的欄位，帶申請目前的 etag。版本衝突
/// （`VERSION_CONFLICT`）時重讀最新版本、換上新的 etag，保留使用者的輸入。聯絡資料與地址在送出 `SubmitKYC` 之前先寫進店家。
/// 寫入（SaveKYCDraft、SubmitKYC、CancelKYC）回的申請直接寫進 [currentKycProvider] 的快取，進度卡與閘門立即看到新狀態。

@ProviderFor(KycWizard)
final kycWizardProvider = KycWizardProvider._();

/// 認證 wizard 的動作與讀取：草稿、證件、統編查詢，以及寫進店家的聯絡資料與地址。
///
/// 表單是整份字串狀態，伺服器版本是 baseline；每次存檔只送和 baseline 不同的欄位，帶申請目前的 etag。版本衝突
/// （`VERSION_CONFLICT`）時重讀最新版本、換上新的 etag，保留使用者的輸入。聯絡資料與地址在送出 `SubmitKYC` 之前先寫進店家。
/// 寫入（SaveKYCDraft、SubmitKYC、CancelKYC）回的申請直接寫進 [currentKycProvider] 的快取，進度卡與閘門立即看到新狀態。
final class KycWizardProvider
    extends $NotifierProvider<KycWizard, KycWizardState> {
  /// 認證 wizard 的動作與讀取：草稿、證件、統編查詢，以及寫進店家的聯絡資料與地址。
  ///
  /// 表單是整份字串狀態，伺服器版本是 baseline；每次存檔只送和 baseline 不同的欄位，帶申請目前的 etag。版本衝突
  /// （`VERSION_CONFLICT`）時重讀最新版本、換上新的 etag，保留使用者的輸入。聯絡資料與地址在送出 `SubmitKYC` 之前先寫進店家。
  /// 寫入（SaveKYCDraft、SubmitKYC、CancelKYC）回的申請直接寫進 [currentKycProvider] 的快取，進度卡與閘門立即看到新狀態。
  KycWizardProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'kycWizardProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$kycWizardHash();

  @$internal
  @override
  KycWizard create() => KycWizard();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(KycWizardState value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<KycWizardState>(value),
    );
  }
}

String _$kycWizardHash() => r'81271f8a33bd31fa6bd76bc8badcb5a8728a80cd';

/// 認證 wizard 的動作與讀取：草稿、證件、統編查詢，以及寫進店家的聯絡資料與地址。
///
/// 表單是整份字串狀態，伺服器版本是 baseline；每次存檔只送和 baseline 不同的欄位，帶申請目前的 etag。版本衝突
/// （`VERSION_CONFLICT`）時重讀最新版本、換上新的 etag，保留使用者的輸入。聯絡資料與地址在送出 `SubmitKYC` 之前先寫進店家。
/// 寫入（SaveKYCDraft、SubmitKYC、CancelKYC）回的申請直接寫進 [currentKycProvider] 的快取，進度卡與閘門立即看到新狀態。

abstract class _$KycWizard extends $Notifier<KycWizardState> {
  KycWizardState build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<KycWizardState, KycWizardState>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<KycWizardState, KycWizardState>,
              KycWizardState,
              Object?,
              Object?
            >;
    element.handleCreate(ref, build);
  }
}
