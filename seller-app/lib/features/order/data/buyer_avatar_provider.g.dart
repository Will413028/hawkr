// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'buyer_avatar_provider.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// 根據 buyerId 取得買家頭像 URL（帶快取）。

@ProviderFor(buyerAvatar)
final buyerAvatarProvider = BuyerAvatarFamily._();

/// 根據 buyerId 取得買家頭像 URL（帶快取）。

final class BuyerAvatarProvider
    extends $FunctionalProvider<AsyncValue<String?>, String?, FutureOr<String?>>
    with $FutureModifier<String?>, $FutureProvider<String?> {
  /// 根據 buyerId 取得買家頭像 URL（帶快取）。
  BuyerAvatarProvider._({
    required BuyerAvatarFamily super.from,
    required int super.argument,
  }) : super(
         retry: null,
         name: r'buyerAvatarProvider',
         isAutoDispose: false,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$buyerAvatarHash();

  @override
  String toString() {
    return r'buyerAvatarProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $FutureProviderElement<String?> $createElement($ProviderPointer pointer) =>
      $FutureProviderElement(pointer);

  @override
  FutureOr<String?> create(Ref ref) {
    final argument = this.argument as int;
    return buyerAvatar(ref, argument);
  }

  @override
  bool operator ==(Object other) {
    return other is BuyerAvatarProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$buyerAvatarHash() => r'3a3927568e9b31cb89dfed2f7cba7129634bd933';

/// 根據 buyerId 取得買家頭像 URL（帶快取）。

final class BuyerAvatarFamily extends $Family
    with $FunctionalFamilyOverride<FutureOr<String?>, int> {
  BuyerAvatarFamily._()
    : super(
        retry: null,
        name: r'buyerAvatarProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: false,
      );

  /// 根據 buyerId 取得買家頭像 URL（帶快取）。

  BuyerAvatarProvider call(int buyerId) =>
      BuyerAvatarProvider._(argument: buyerId, from: this);

  @override
  String toString() => r'buyerAvatarProvider';
}

/// 根據 orderNo 取得買家頭像 URL。
///
/// 流程：orderNo → order detail API (取 buyerId) → customer API (取 avatar)
/// keepAlive 確保同一訂單只查一次。

@ProviderFor(buyerAvatarByOrder)
final buyerAvatarByOrderProvider = BuyerAvatarByOrderFamily._();

/// 根據 orderNo 取得買家頭像 URL。
///
/// 流程：orderNo → order detail API (取 buyerId) → customer API (取 avatar)
/// keepAlive 確保同一訂單只查一次。

final class BuyerAvatarByOrderProvider
    extends $FunctionalProvider<AsyncValue<String?>, String?, FutureOr<String?>>
    with $FutureModifier<String?>, $FutureProvider<String?> {
  /// 根據 orderNo 取得買家頭像 URL。
  ///
  /// 流程：orderNo → order detail API (取 buyerId) → customer API (取 avatar)
  /// keepAlive 確保同一訂單只查一次。
  BuyerAvatarByOrderProvider._({
    required BuyerAvatarByOrderFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'buyerAvatarByOrderProvider',
         isAutoDispose: false,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$buyerAvatarByOrderHash();

  @override
  String toString() {
    return r'buyerAvatarByOrderProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $FutureProviderElement<String?> $createElement($ProviderPointer pointer) =>
      $FutureProviderElement(pointer);

  @override
  FutureOr<String?> create(Ref ref) {
    final argument = this.argument as String;
    return buyerAvatarByOrder(ref, argument);
  }

  @override
  bool operator ==(Object other) {
    return other is BuyerAvatarByOrderProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$buyerAvatarByOrderHash() =>
    r'1d72f4958fe33fc06b5d62f8bb8cbfeda6764fc9';

/// 根據 orderNo 取得買家頭像 URL。
///
/// 流程：orderNo → order detail API (取 buyerId) → customer API (取 avatar)
/// keepAlive 確保同一訂單只查一次。

final class BuyerAvatarByOrderFamily extends $Family
    with $FunctionalFamilyOverride<FutureOr<String?>, String> {
  BuyerAvatarByOrderFamily._()
    : super(
        retry: null,
        name: r'buyerAvatarByOrderProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: false,
      );

  /// 根據 orderNo 取得買家頭像 URL。
  ///
  /// 流程：orderNo → order detail API (取 buyerId) → customer API (取 avatar)
  /// keepAlive 確保同一訂單只查一次。

  BuyerAvatarByOrderProvider call(String orderNo) =>
      BuyerAvatarByOrderProvider._(argument: orderNo, from: this);

  @override
  String toString() => r'buyerAvatarByOrderProvider';
}
