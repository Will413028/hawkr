// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'payment_flow.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(paymentFlow)
final paymentFlowProvider = PaymentFlowProvider._();

final class PaymentFlowProvider
    extends $FunctionalProvider<PaymentFlow, PaymentFlow, PaymentFlow>
    with $Provider<PaymentFlow> {
  PaymentFlowProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'paymentFlowProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$paymentFlowHash();

  @$internal
  @override
  $ProviderElement<PaymentFlow> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  PaymentFlow create(Ref ref) {
    return paymentFlow(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(PaymentFlow value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<PaymentFlow>(value),
    );
  }
}

String _$paymentFlowHash() => r'5906ecf7e1ca325971a4c159af2732e60a6e9257';
