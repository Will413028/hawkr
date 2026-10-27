// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'wallet_remote_service.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(walletRemoteService)
final walletRemoteServiceProvider = WalletRemoteServiceProvider._();

final class WalletRemoteServiceProvider
    extends
        $FunctionalProvider<
          WalletRemoteService,
          WalletRemoteService,
          WalletRemoteService
        >
    with $Provider<WalletRemoteService> {
  WalletRemoteServiceProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'walletRemoteServiceProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$walletRemoteServiceHash();

  @$internal
  @override
  $ProviderElement<WalletRemoteService> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  WalletRemoteService create(Ref ref) {
    return walletRemoteService(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(WalletRemoteService value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<WalletRemoteService>(value),
    );
  }
}

String _$walletRemoteServiceHash() =>
    r'e07ce0b639cc9b15c40965508c0f4151a1d88883';
