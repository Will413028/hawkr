// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'earnings.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// 訂單詳情的入帳區：[orderNo] 這張訂單的入帳列；null 是沒有入帳資料（見 [fetchMyOrderEarnings]）。

@ProviderFor(myOrderEarnings)
final myOrderEarningsProvider = MyOrderEarningsFamily._();

/// 訂單詳情的入帳區：[orderNo] 這張訂單的入帳列；null 是沒有入帳資料（見 [fetchMyOrderEarnings]）。

final class MyOrderEarningsProvider
    extends
        $FunctionalProvider<
          AsyncValue<GetMyOrderEarningsResponse?>,
          GetMyOrderEarningsResponse?,
          FutureOr<GetMyOrderEarningsResponse?>
        >
    with
        $FutureModifier<GetMyOrderEarningsResponse?>,
        $FutureProvider<GetMyOrderEarningsResponse?> {
  /// 訂單詳情的入帳區：[orderNo] 這張訂單的入帳列；null 是沒有入帳資料（見 [fetchMyOrderEarnings]）。
  MyOrderEarningsProvider._({
    required MyOrderEarningsFamily super.from,
    required String super.argument,
  }) : super(
         retry: noRetry,
         name: r'myOrderEarningsProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$myOrderEarningsHash();

  @override
  String toString() {
    return r'myOrderEarningsProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $FutureProviderElement<GetMyOrderEarningsResponse?> $createElement(
    $ProviderPointer pointer,
  ) => $FutureProviderElement(pointer);

  @override
  FutureOr<GetMyOrderEarningsResponse?> create(Ref ref) {
    final argument = this.argument as String;
    return myOrderEarnings(ref, argument);
  }

  @override
  bool operator ==(Object other) {
    return other is MyOrderEarningsProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$myOrderEarningsHash() => r'3fbb0aecc4eddeff6aa3d3408fc8bf58d7dd1d24';

/// 訂單詳情的入帳區：[orderNo] 這張訂單的入帳列；null 是沒有入帳資料（見 [fetchMyOrderEarnings]）。

final class MyOrderEarningsFamily extends $Family
    with
        $FunctionalFamilyOverride<
          FutureOr<GetMyOrderEarningsResponse?>,
          String
        > {
  MyOrderEarningsFamily._()
    : super(
        retry: noRetry,
        name: r'myOrderEarningsProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// 訂單詳情的入帳區：[orderNo] 這張訂單的入帳列；null 是沒有入帳資料（見 [fetchMyOrderEarnings]）。

  MyOrderEarningsProvider call(String orderNo) =>
      MyOrderEarningsProvider._(argument: orderNo, from: this);

  @override
  String toString() => r'myOrderEarningsProvider';
}

/// [accountId] 這個帳號的入帳紀錄（`ListMyEarnings`，新到舊）：先讀第一頁，[loadMore] 讀下一頁並接在後面。
/// 以帳號為鍵，換帳號不會看到上一個帳號的紀錄；店家由伺服器依登入身分決定，請求不帶店家。

@ProviderFor(MyEarnings)
final myEarningsProvider = MyEarningsFamily._();

/// [accountId] 這個帳號的入帳紀錄（`ListMyEarnings`，新到舊）：先讀第一頁，[loadMore] 讀下一頁並接在後面。
/// 以帳號為鍵，換帳號不會看到上一個帳號的紀錄；店家由伺服器依登入身分決定，請求不帶店家。
final class MyEarningsProvider
    extends $AsyncNotifierProvider<MyEarnings, EarningsLedger> {
  /// [accountId] 這個帳號的入帳紀錄（`ListMyEarnings`，新到舊）：先讀第一頁，[loadMore] 讀下一頁並接在後面。
  /// 以帳號為鍵，換帳號不會看到上一個帳號的紀錄；店家由伺服器依登入身分決定，請求不帶店家。
  MyEarningsProvider._({
    required MyEarningsFamily super.from,
    required String super.argument,
  }) : super(
         retry: noRetry,
         name: r'myEarningsProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$myEarningsHash();

  @override
  String toString() {
    return r'myEarningsProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  MyEarnings create() => MyEarnings();

  @override
  bool operator ==(Object other) {
    return other is MyEarningsProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$myEarningsHash() => r'1c049b9013a63c2dffba5b62d87f79aa5d35d54e';

/// [accountId] 這個帳號的入帳紀錄（`ListMyEarnings`，新到舊）：先讀第一頁，[loadMore] 讀下一頁並接在後面。
/// 以帳號為鍵，換帳號不會看到上一個帳號的紀錄；店家由伺服器依登入身分決定，請求不帶店家。

final class MyEarningsFamily extends $Family
    with
        $ClassFamilyOverride<
          MyEarnings,
          AsyncValue<EarningsLedger>,
          EarningsLedger,
          FutureOr<EarningsLedger>,
          String
        > {
  MyEarningsFamily._()
    : super(
        retry: noRetry,
        name: r'myEarningsProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// [accountId] 這個帳號的入帳紀錄（`ListMyEarnings`，新到舊）：先讀第一頁，[loadMore] 讀下一頁並接在後面。
  /// 以帳號為鍵，換帳號不會看到上一個帳號的紀錄；店家由伺服器依登入身分決定，請求不帶店家。

  MyEarningsProvider call(String accountId) =>
      MyEarningsProvider._(argument: accountId, from: this);

  @override
  String toString() => r'myEarningsProvider';
}

/// [accountId] 這個帳號的入帳紀錄（`ListMyEarnings`，新到舊）：先讀第一頁，[loadMore] 讀下一頁並接在後面。
/// 以帳號為鍵，換帳號不會看到上一個帳號的紀錄；店家由伺服器依登入身分決定，請求不帶店家。

abstract class _$MyEarnings extends $AsyncNotifier<EarningsLedger> {
  late final _$args = ref.$arg as String;
  String get accountId => _$args;

  FutureOr<EarningsLedger> build(String accountId);
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<AsyncValue<EarningsLedger>, EarningsLedger>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<AsyncValue<EarningsLedger>, EarningsLedger>,
              AsyncValue<EarningsLedger>,
              Object?,
              Object?
            >;
    element.handleCreate(ref, () => build(_$args));
  }
}
