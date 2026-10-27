// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'wallet_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(CouponController)
final couponControllerProvider = CouponControllerFamily._();

final class CouponControllerProvider
    extends
        $AsyncNotifierProvider<CouponController, List<WalletCouponCardState>> {
  CouponControllerProvider._({
    required CouponControllerFamily super.from,
    required int super.argument,
  }) : super(
         retry: _disableCouponAutomaticRetry,
         name: r'couponControllerProvider',
         isAutoDispose: false,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$couponControllerHash();

  @override
  String toString() {
    return r'couponControllerProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  CouponController create() => CouponController();

  @override
  bool operator ==(Object other) {
    return other is CouponControllerProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$couponControllerHash() => r'77e687ad94afb6f1101ebce2f9e0f993ce9efb32';

final class CouponControllerFamily extends $Family
    with
        $ClassFamilyOverride<
          CouponController,
          AsyncValue<List<WalletCouponCardState>>,
          List<WalletCouponCardState>,
          FutureOr<List<WalletCouponCardState>>,
          int
        > {
  CouponControllerFamily._()
    : super(
        retry: _disableCouponAutomaticRetry,
        name: r'couponControllerProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: false,
      );

  CouponControllerProvider call({int status = 0}) =>
      CouponControllerProvider._(argument: status, from: this);

  @override
  String toString() => r'couponControllerProvider';
}

abstract class _$CouponController
    extends $AsyncNotifier<List<WalletCouponCardState>> {
  late final _$args = ref.$arg as int;
  int get status => _$args;

  FutureOr<List<WalletCouponCardState>> build({int status = 0});
  @$mustCallSuper
  @override
  void runBuild() {
    final ref =
        this.ref
            as $Ref<
              AsyncValue<List<WalletCouponCardState>>,
              List<WalletCouponCardState>
            >;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<
                AsyncValue<List<WalletCouponCardState>>,
                List<WalletCouponCardState>
              >,
              AsyncValue<List<WalletCouponCardState>>,
              Object?,
              Object?
            >;
    element.handleCreate(ref, () => build(status: _$args));
  }
}
