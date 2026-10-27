// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'receipt_photo_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(ReceiptPhotoController)
final receiptPhotoControllerProvider = ReceiptPhotoControllerFamily._();

final class ReceiptPhotoControllerProvider
    extends $AsyncNotifierProvider<ReceiptPhotoController, ReceiptPhotoState> {
  ReceiptPhotoControllerProvider._({
    required ReceiptPhotoControllerFamily super.from,
    required ({String orderSn, List<String> backendUrls}) super.argument,
  }) : super(
         retry: null,
         name: r'receiptPhotoControllerProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$receiptPhotoControllerHash();

  @override
  String toString() {
    return r'receiptPhotoControllerProvider'
        ''
        '$argument';
  }

  @$internal
  @override
  ReceiptPhotoController create() => ReceiptPhotoController();

  @override
  bool operator ==(Object other) {
    return other is ReceiptPhotoControllerProvider &&
        other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$receiptPhotoControllerHash() =>
    r'bb6e7ae225a04faccfd6bf5690c55f5121bca315';

final class ReceiptPhotoControllerFamily extends $Family
    with
        $ClassFamilyOverride<
          ReceiptPhotoController,
          AsyncValue<ReceiptPhotoState>,
          ReceiptPhotoState,
          FutureOr<ReceiptPhotoState>,
          ({String orderSn, List<String> backendUrls})
        > {
  ReceiptPhotoControllerFamily._()
    : super(
        retry: null,
        name: r'receiptPhotoControllerProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  ReceiptPhotoControllerProvider call({
    required String orderSn,
    required List<String> backendUrls,
  }) => ReceiptPhotoControllerProvider._(
    argument: (orderSn: orderSn, backendUrls: backendUrls),
    from: this,
  );

  @override
  String toString() => r'receiptPhotoControllerProvider';
}

abstract class _$ReceiptPhotoController
    extends $AsyncNotifier<ReceiptPhotoState> {
  late final _$args = ref.$arg as ({String orderSn, List<String> backendUrls});
  String get orderSn => _$args.orderSn;
  List<String> get backendUrls => _$args.backendUrls;

  FutureOr<ReceiptPhotoState> build({
    required String orderSn,
    required List<String> backendUrls,
  });
  @$mustCallSuper
  @override
  void runBuild() {
    final ref =
        this.ref as $Ref<AsyncValue<ReceiptPhotoState>, ReceiptPhotoState>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<AsyncValue<ReceiptPhotoState>, ReceiptPhotoState>,
              AsyncValue<ReceiptPhotoState>,
              Object?,
              Object?
            >;
    element.handleCreate(
      ref,
      () => build(orderSn: _$args.orderSn, backendUrls: _$args.backendUrls),
    );
  }
}
