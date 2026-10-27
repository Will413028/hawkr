// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'measures.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// [accountId] 這個帳號 [filter] 狀態的措施（帳號層級，不需要店家；UNSPECIFIED 是全部）。以帳號與狀態為鍵，換帳號不會看到
/// 上一個帳號的列表（讀取中或失敗時也不會）；沒有畫面在看時釋放。讀取失敗是 error 狀態，不當成沒有措施。

@ProviderFor(MyMeasures)
final myMeasuresProvider = MyMeasuresFamily._();

/// [accountId] 這個帳號 [filter] 狀態的措施（帳號層級，不需要店家；UNSPECIFIED 是全部）。以帳號與狀態為鍵，換帳號不會看到
/// 上一個帳號的列表（讀取中或失敗時也不會）；沒有畫面在看時釋放。讀取失敗是 error 狀態，不當成沒有措施。
final class MyMeasuresProvider
    extends $AsyncNotifierProvider<MyMeasures, List<Measure>> {
  /// [accountId] 這個帳號 [filter] 狀態的措施（帳號層級，不需要店家；UNSPECIFIED 是全部）。以帳號與狀態為鍵，換帳號不會看到
  /// 上一個帳號的列表（讀取中或失敗時也不會）；沒有畫面在看時釋放。讀取失敗是 error 狀態，不當成沒有措施。
  MyMeasuresProvider._({
    required MyMeasuresFamily super.from,
    required (String, MeasureState) super.argument,
  }) : super(
         retry: noRetry,
         name: r'myMeasuresProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$myMeasuresHash();

  @override
  String toString() {
    return r'myMeasuresProvider'
        ''
        '$argument';
  }

  @$internal
  @override
  MyMeasures create() => MyMeasures();

  @override
  bool operator ==(Object other) {
    return other is MyMeasuresProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$myMeasuresHash() => r'045775c2ca13c4b4f3ce978e0baa8d09acc39cc6';

/// [accountId] 這個帳號 [filter] 狀態的措施（帳號層級，不需要店家；UNSPECIFIED 是全部）。以帳號與狀態為鍵，換帳號不會看到
/// 上一個帳號的列表（讀取中或失敗時也不會）；沒有畫面在看時釋放。讀取失敗是 error 狀態，不當成沒有措施。

final class MyMeasuresFamily extends $Family
    with
        $ClassFamilyOverride<
          MyMeasures,
          AsyncValue<List<Measure>>,
          List<Measure>,
          FutureOr<List<Measure>>,
          (String, MeasureState)
        > {
  MyMeasuresFamily._()
    : super(
        retry: noRetry,
        name: r'myMeasuresProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// [accountId] 這個帳號 [filter] 狀態的措施（帳號層級，不需要店家；UNSPECIFIED 是全部）。以帳號與狀態為鍵，換帳號不會看到
  /// 上一個帳號的列表（讀取中或失敗時也不會）；沒有畫面在看時釋放。讀取失敗是 error 狀態，不當成沒有措施。

  MyMeasuresProvider call(String accountId, MeasureState filter) =>
      MyMeasuresProvider._(argument: (accountId, filter), from: this);

  @override
  String toString() => r'myMeasuresProvider';
}

/// [accountId] 這個帳號 [filter] 狀態的措施（帳號層級，不需要店家；UNSPECIFIED 是全部）。以帳號與狀態為鍵，換帳號不會看到
/// 上一個帳號的列表（讀取中或失敗時也不會）；沒有畫面在看時釋放。讀取失敗是 error 狀態，不當成沒有措施。

abstract class _$MyMeasures extends $AsyncNotifier<List<Measure>> {
  late final _$args = ref.$arg as (String, MeasureState);
  String get accountId => _$args.$1;
  MeasureState get filter => _$args.$2;

  FutureOr<List<Measure>> build(String accountId, MeasureState filter);
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<AsyncValue<List<Measure>>, List<Measure>>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<AsyncValue<List<Measure>>, List<Measure>>,
              AsyncValue<List<Measure>>,
              Object?,
              Object?
            >;
    element.handleCreate(ref, () => build(_$args.$1, _$args.$2));
  }
}

/// 目前登入帳號 [filter] 狀態的措施列表；沒有登入時是 null（和值是 null 的 `AsyncData` 不同）。

@ProviderFor(currentMyMeasures)
final currentMyMeasuresProvider = CurrentMyMeasuresFamily._();

/// 目前登入帳號 [filter] 狀態的措施列表；沒有登入時是 null（和值是 null 的 `AsyncData` 不同）。

final class CurrentMyMeasuresProvider
    extends
        $FunctionalProvider<
          AsyncValue<List<Measure>>?,
          AsyncValue<List<Measure>>?,
          AsyncValue<List<Measure>>?
        >
    with $Provider<AsyncValue<List<Measure>>?> {
  /// 目前登入帳號 [filter] 狀態的措施列表；沒有登入時是 null（和值是 null 的 `AsyncData` 不同）。
  CurrentMyMeasuresProvider._({
    required CurrentMyMeasuresFamily super.from,
    required MeasureState super.argument,
  }) : super(
         retry: null,
         name: r'currentMyMeasuresProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$currentMyMeasuresHash();

  @override
  String toString() {
    return r'currentMyMeasuresProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $ProviderElement<AsyncValue<List<Measure>>?> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  AsyncValue<List<Measure>>? create(Ref ref) {
    final argument = this.argument as MeasureState;
    return currentMyMeasures(ref, argument);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(AsyncValue<List<Measure>>? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<AsyncValue<List<Measure>>?>(value),
    );
  }

  @override
  bool operator ==(Object other) {
    return other is CurrentMyMeasuresProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$currentMyMeasuresHash() => r'5d4e75cc68da0d776c8198c44fccca78fa0c96cb';

/// 目前登入帳號 [filter] 狀態的措施列表；沒有登入時是 null（和值是 null 的 `AsyncData` 不同）。

final class CurrentMyMeasuresFamily extends $Family
    with $FunctionalFamilyOverride<AsyncValue<List<Measure>>?, MeasureState> {
  CurrentMyMeasuresFamily._()
    : super(
        retry: null,
        name: r'currentMyMeasuresProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// 目前登入帳號 [filter] 狀態的措施列表；沒有登入時是 null（和值是 null 的 `AsyncData` 不同）。

  CurrentMyMeasuresProvider call(MeasureState filter) =>
      CurrentMyMeasuresProvider._(argument: filter, from: this);

  @override
  String toString() => r'currentMyMeasuresProvider';
}

/// [accountId] 有沒有任何措施紀錄：只問一筆（`ListMyMeasures` 的 `page_size` 1），不讀完整個列表。
/// 讀取失敗是 error 狀態，由呼叫端決定怎麼處理。

@ProviderFor(accountHasMeasures)
final accountHasMeasuresProvider = AccountHasMeasuresFamily._();

/// [accountId] 有沒有任何措施紀錄：只問一筆（`ListMyMeasures` 的 `page_size` 1），不讀完整個列表。
/// 讀取失敗是 error 狀態，由呼叫端決定怎麼處理。

final class AccountHasMeasuresProvider
    extends $FunctionalProvider<AsyncValue<bool>, bool, FutureOr<bool>>
    with $FutureModifier<bool>, $FutureProvider<bool> {
  /// [accountId] 有沒有任何措施紀錄：只問一筆（`ListMyMeasures` 的 `page_size` 1），不讀完整個列表。
  /// 讀取失敗是 error 狀態，由呼叫端決定怎麼處理。
  AccountHasMeasuresProvider._({
    required AccountHasMeasuresFamily super.from,
    required String super.argument,
  }) : super(
         retry: noRetry,
         name: r'accountHasMeasuresProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$accountHasMeasuresHash();

  @override
  String toString() {
    return r'accountHasMeasuresProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $FutureProviderElement<bool> $createElement($ProviderPointer pointer) =>
      $FutureProviderElement(pointer);

  @override
  FutureOr<bool> create(Ref ref) {
    final argument = this.argument as String;
    return accountHasMeasures(ref, argument);
  }

  @override
  bool operator ==(Object other) {
    return other is AccountHasMeasuresProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$accountHasMeasuresHash() =>
    r'd6a9018b087dff8a4ff5d2ff4fdf67d8376e29a7';

/// [accountId] 有沒有任何措施紀錄：只問一筆（`ListMyMeasures` 的 `page_size` 1），不讀完整個列表。
/// 讀取失敗是 error 狀態，由呼叫端決定怎麼處理。

final class AccountHasMeasuresFamily extends $Family
    with $FunctionalFamilyOverride<FutureOr<bool>, String> {
  AccountHasMeasuresFamily._()
    : super(
        retry: noRetry,
        name: r'accountHasMeasuresProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// [accountId] 有沒有任何措施紀錄：只問一筆（`ListMyMeasures` 的 `page_size` 1），不讀完整個列表。
  /// 讀取失敗是 error 狀態，由呼叫端決定怎麼處理。

  AccountHasMeasuresProvider call(String accountId) =>
      AccountHasMeasuresProvider._(argument: accountId, from: this);

  @override
  String toString() => r'accountHasMeasuresProvider';
}

/// [accountId] 的 [measureId] 這一筆措施：已讀過全部列表而且列表裡有就用列表的，否則用 `GetMyMeasure` 讀（例如不經列表
/// 直接開啟）。只看列表是否已經存在，不會為了找一筆去觸發整個列表的讀取。

@ProviderFor(measureById)
final measureByIdProvider = MeasureByIdFamily._();

/// [accountId] 的 [measureId] 這一筆措施：已讀過全部列表而且列表裡有就用列表的，否則用 `GetMyMeasure` 讀（例如不經列表
/// 直接開啟）。只看列表是否已經存在，不會為了找一筆去觸發整個列表的讀取。

final class MeasureByIdProvider
    extends $FunctionalProvider<AsyncValue<Measure>, Measure, FutureOr<Measure>>
    with $FutureModifier<Measure>, $FutureProvider<Measure> {
  /// [accountId] 的 [measureId] 這一筆措施：已讀過全部列表而且列表裡有就用列表的，否則用 `GetMyMeasure` 讀（例如不經列表
  /// 直接開啟）。只看列表是否已經存在，不會為了找一筆去觸發整個列表的讀取。
  MeasureByIdProvider._({
    required MeasureByIdFamily super.from,
    required (String, String) super.argument,
  }) : super(
         retry: noRetry,
         name: r'measureByIdProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$measureByIdHash();

  @override
  String toString() {
    return r'measureByIdProvider'
        ''
        '$argument';
  }

  @$internal
  @override
  $FutureProviderElement<Measure> $createElement($ProviderPointer pointer) =>
      $FutureProviderElement(pointer);

  @override
  FutureOr<Measure> create(Ref ref) {
    final argument = this.argument as (String, String);
    return measureById(ref, argument.$1, argument.$2);
  }

  @override
  bool operator ==(Object other) {
    return other is MeasureByIdProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$measureByIdHash() => r'a3b49ee38e7740476f24060e138c6618343a29bf';

/// [accountId] 的 [measureId] 這一筆措施：已讀過全部列表而且列表裡有就用列表的，否則用 `GetMyMeasure` 讀（例如不經列表
/// 直接開啟）。只看列表是否已經存在，不會為了找一筆去觸發整個列表的讀取。

final class MeasureByIdFamily extends $Family
    with $FunctionalFamilyOverride<FutureOr<Measure>, (String, String)> {
  MeasureByIdFamily._()
    : super(
        retry: noRetry,
        name: r'measureByIdProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// [accountId] 的 [measureId] 這一筆措施：已讀過全部列表而且列表裡有就用列表的，否則用 `GetMyMeasure` 讀（例如不經列表
  /// 直接開啟）。只看列表是否已經存在，不會為了找一筆去觸發整個列表的讀取。

  MeasureByIdProvider call(String accountId, String measureId) =>
      MeasureByIdProvider._(argument: (accountId, measureId), from: this);

  @override
  String toString() => r'measureByIdProvider';
}
