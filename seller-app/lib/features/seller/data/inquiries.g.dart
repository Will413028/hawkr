// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'inquiries.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// [accountId] 這個帳號的詢問（帳號層級，店家關閉後也讀得到）。以帳號為鍵，換帳號不會看到上一個帳號的列表；
/// 讀取失敗是 error 狀態，不當成沒有詢問。

@ProviderFor(MyInquiries)
final myInquiriesProvider = MyInquiriesFamily._();

/// [accountId] 這個帳號的詢問（帳號層級，店家關閉後也讀得到）。以帳號為鍵，換帳號不會看到上一個帳號的列表；
/// 讀取失敗是 error 狀態，不當成沒有詢問。
final class MyInquiriesProvider
    extends $AsyncNotifierProvider<MyInquiries, List<Inquiry>> {
  /// [accountId] 這個帳號的詢問（帳號層級，店家關閉後也讀得到）。以帳號為鍵，換帳號不會看到上一個帳號的列表；
  /// 讀取失敗是 error 狀態，不當成沒有詢問。
  MyInquiriesProvider._({
    required MyInquiriesFamily super.from,
    required String super.argument,
  }) : super(
         retry: noRetry,
         name: r'myInquiriesProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$myInquiriesHash();

  @override
  String toString() {
    return r'myInquiriesProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  MyInquiries create() => MyInquiries();

  @override
  bool operator ==(Object other) {
    return other is MyInquiriesProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$myInquiriesHash() => r'aa15fb133419994392e7b173051d694191abfd9c';

/// [accountId] 這個帳號的詢問（帳號層級，店家關閉後也讀得到）。以帳號為鍵，換帳號不會看到上一個帳號的列表；
/// 讀取失敗是 error 狀態，不當成沒有詢問。

final class MyInquiriesFamily extends $Family
    with
        $ClassFamilyOverride<
          MyInquiries,
          AsyncValue<List<Inquiry>>,
          List<Inquiry>,
          FutureOr<List<Inquiry>>,
          String
        > {
  MyInquiriesFamily._()
    : super(
        retry: noRetry,
        name: r'myInquiriesProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// [accountId] 這個帳號的詢問（帳號層級，店家關閉後也讀得到）。以帳號為鍵，換帳號不會看到上一個帳號的列表；
  /// 讀取失敗是 error 狀態，不當成沒有詢問。

  MyInquiriesProvider call(String accountId) =>
      MyInquiriesProvider._(argument: accountId, from: this);

  @override
  String toString() => r'myInquiriesProvider';
}

/// [accountId] 這個帳號的詢問（帳號層級，店家關閉後也讀得到）。以帳號為鍵，換帳號不會看到上一個帳號的列表；
/// 讀取失敗是 error 狀態，不當成沒有詢問。

abstract class _$MyInquiries extends $AsyncNotifier<List<Inquiry>> {
  late final _$args = ref.$arg as String;
  String get accountId => _$args;

  FutureOr<List<Inquiry>> build(String accountId);
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<AsyncValue<List<Inquiry>>, List<Inquiry>>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<AsyncValue<List<Inquiry>>, List<Inquiry>>,
              AsyncValue<List<Inquiry>>,
              Object?,
              Object?
            >;
    element.handleCreate(ref, () => build(_$args));
  }
}

/// 目前登入帳號的詢問列表；沒有登入時是 null。

@ProviderFor(currentMyInquiries)
final currentMyInquiriesProvider = CurrentMyInquiriesProvider._();

/// 目前登入帳號的詢問列表；沒有登入時是 null。

final class CurrentMyInquiriesProvider
    extends
        $FunctionalProvider<
          AsyncValue<List<Inquiry>>?,
          AsyncValue<List<Inquiry>>?,
          AsyncValue<List<Inquiry>>?
        >
    with $Provider<AsyncValue<List<Inquiry>>?> {
  /// 目前登入帳號的詢問列表；沒有登入時是 null。
  CurrentMyInquiriesProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'currentMyInquiriesProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$currentMyInquiriesHash();

  @$internal
  @override
  $ProviderElement<AsyncValue<List<Inquiry>>?> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  AsyncValue<List<Inquiry>>? create(Ref ref) {
    return currentMyInquiries(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(AsyncValue<List<Inquiry>>? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<AsyncValue<List<Inquiry>>?>(value),
    );
  }
}

String _$currentMyInquiriesHash() =>
    r'b54483864c67a42522f3d5bc84736e9ed867492d';

/// [accountId] 待回覆（OPEN，逾期仍算）的詢問數，伺服器算好的 `pending_reply_count`；只問一筆，不讀完整個列表。

@ProviderFor(pendingInquiryCount)
final pendingInquiryCountProvider = PendingInquiryCountFamily._();

/// [accountId] 待回覆（OPEN，逾期仍算）的詢問數，伺服器算好的 `pending_reply_count`；只問一筆，不讀完整個列表。

final class PendingInquiryCountProvider
    extends $FunctionalProvider<AsyncValue<int>, int, FutureOr<int>>
    with $FutureModifier<int>, $FutureProvider<int> {
  /// [accountId] 待回覆（OPEN，逾期仍算）的詢問數，伺服器算好的 `pending_reply_count`；只問一筆，不讀完整個列表。
  PendingInquiryCountProvider._({
    required PendingInquiryCountFamily super.from,
    required String super.argument,
  }) : super(
         retry: noRetry,
         name: r'pendingInquiryCountProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$pendingInquiryCountHash();

  @override
  String toString() {
    return r'pendingInquiryCountProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $FutureProviderElement<int> $createElement($ProviderPointer pointer) =>
      $FutureProviderElement(pointer);

  @override
  FutureOr<int> create(Ref ref) {
    final argument = this.argument as String;
    return pendingInquiryCount(ref, argument);
  }

  @override
  bool operator ==(Object other) {
    return other is PendingInquiryCountProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$pendingInquiryCountHash() =>
    r'74bdceb40492c28d47060d9accfef188bb84dc49';

/// [accountId] 待回覆（OPEN，逾期仍算）的詢問數，伺服器算好的 `pending_reply_count`；只問一筆，不讀完整個列表。

final class PendingInquiryCountFamily extends $Family
    with $FunctionalFamilyOverride<FutureOr<int>, String> {
  PendingInquiryCountFamily._()
    : super(
        retry: noRetry,
        name: r'pendingInquiryCountProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// [accountId] 待回覆（OPEN，逾期仍算）的詢問數，伺服器算好的 `pending_reply_count`；只問一筆，不讀完整個列表。

  PendingInquiryCountProvider call(String accountId) =>
      PendingInquiryCountProvider._(argument: accountId, from: this);

  @override
  String toString() => r'pendingInquiryCountProvider';
}

/// 目前登入帳號的待回覆數；沒有登入、讀取中或失敗時是 0（徽章只是提示，失敗不阻擋入口）。

@ProviderFor(currentPendingInquiryCount)
final currentPendingInquiryCountProvider =
    CurrentPendingInquiryCountProvider._();

/// 目前登入帳號的待回覆數；沒有登入、讀取中或失敗時是 0（徽章只是提示，失敗不阻擋入口）。

final class CurrentPendingInquiryCountProvider
    extends $FunctionalProvider<int, int, int>
    with $Provider<int> {
  /// 目前登入帳號的待回覆數；沒有登入、讀取中或失敗時是 0（徽章只是提示，失敗不阻擋入口）。
  CurrentPendingInquiryCountProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'currentPendingInquiryCountProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$currentPendingInquiryCountHash();

  @$internal
  @override
  $ProviderElement<int> $createElement($ProviderPointer pointer) =>
      $ProviderElement(pointer);

  @override
  int create(Ref ref) {
    return currentPendingInquiryCount(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(int value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<int>(value),
    );
  }
}

String _$currentPendingInquiryCountHash() =>
    r'95e582423f367a0b662f98e4de7a789695e51c69';

/// [accountId] 的 [inquiryId] 這一則詢問：已讀過列表而且列表裡有就用列表的（監看列表，回覆後列表換成最新一筆，這裡跟著更新），否則 `GetMyInquiry`。

@ProviderFor(inquiryById)
final inquiryByIdProvider = InquiryByIdFamily._();

/// [accountId] 的 [inquiryId] 這一則詢問：已讀過列表而且列表裡有就用列表的（監看列表，回覆後列表換成最新一筆，這裡跟著更新），否則 `GetMyInquiry`。

final class InquiryByIdProvider
    extends $FunctionalProvider<AsyncValue<Inquiry>, Inquiry, FutureOr<Inquiry>>
    with $FutureModifier<Inquiry>, $FutureProvider<Inquiry> {
  /// [accountId] 的 [inquiryId] 這一則詢問：已讀過列表而且列表裡有就用列表的（監看列表，回覆後列表換成最新一筆，這裡跟著更新），否則 `GetMyInquiry`。
  InquiryByIdProvider._({
    required InquiryByIdFamily super.from,
    required (String, String) super.argument,
  }) : super(
         retry: noRetry,
         name: r'inquiryByIdProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$inquiryByIdHash();

  @override
  String toString() {
    return r'inquiryByIdProvider'
        ''
        '$argument';
  }

  @$internal
  @override
  $FutureProviderElement<Inquiry> $createElement($ProviderPointer pointer) =>
      $FutureProviderElement(pointer);

  @override
  FutureOr<Inquiry> create(Ref ref) {
    final argument = this.argument as (String, String);
    return inquiryById(ref, argument.$1, argument.$2);
  }

  @override
  bool operator ==(Object other) {
    return other is InquiryByIdProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$inquiryByIdHash() => r'0b8008c10aa985932b8d83f8a28b4e1a6edf7850';

/// [accountId] 的 [inquiryId] 這一則詢問：已讀過列表而且列表裡有就用列表的（監看列表，回覆後列表換成最新一筆，這裡跟著更新），否則 `GetMyInquiry`。

final class InquiryByIdFamily extends $Family
    with $FunctionalFamilyOverride<FutureOr<Inquiry>, (String, String)> {
  InquiryByIdFamily._()
    : super(
        retry: noRetry,
        name: r'inquiryByIdProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// [accountId] 的 [inquiryId] 這一則詢問：已讀過列表而且列表裡有就用列表的（監看列表，回覆後列表換成最新一筆，這裡跟著更新），否則 `GetMyInquiry`。

  InquiryByIdProvider call(String accountId, String inquiryId) =>
      InquiryByIdProvider._(argument: (accountId, inquiryId), from: this);

  @override
  String toString() => r'inquiryByIdProvider';
}
