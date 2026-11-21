// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'store_report_detail_provider.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// 我的一筆店家檢舉的詳情（`GetMyReport`）；補充成功後 `ref.invalidate` 重讀。

@ProviderFor(storeReportDetail)
final storeReportDetailProvider = StoreReportDetailFamily._();

/// 我的一筆店家檢舉的詳情（`GetMyReport`）；補充成功後 `ref.invalidate` 重讀。

final class StoreReportDetailProvider
    extends
        $FunctionalProvider<
          AsyncValue<pb.GetMyReportResponse>,
          pb.GetMyReportResponse,
          FutureOr<pb.GetMyReportResponse>
        >
    with
        $FutureModifier<pb.GetMyReportResponse>,
        $FutureProvider<pb.GetMyReportResponse> {
  /// 我的一筆店家檢舉的詳情（`GetMyReport`）；補充成功後 `ref.invalidate` 重讀。
  StoreReportDetailProvider._({
    required StoreReportDetailFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'storeReportDetailProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$storeReportDetailHash();

  @override
  String toString() {
    return r'storeReportDetailProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $FutureProviderElement<pb.GetMyReportResponse> $createElement(
    $ProviderPointer pointer,
  ) => $FutureProviderElement(pointer);

  @override
  FutureOr<pb.GetMyReportResponse> create(Ref ref) {
    final argument = this.argument as String;
    return storeReportDetail(ref, argument);
  }

  @override
  bool operator ==(Object other) {
    return other is StoreReportDetailProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$storeReportDetailHash() => r'a330b93d106fe9692f0e336085a496f5fc7ebf51';

/// 我的一筆店家檢舉的詳情（`GetMyReport`）；補充成功後 `ref.invalidate` 重讀。

final class StoreReportDetailFamily extends $Family
    with $FunctionalFamilyOverride<FutureOr<pb.GetMyReportResponse>, String> {
  StoreReportDetailFamily._()
    : super(
        retry: null,
        name: r'storeReportDetailProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// 我的一筆店家檢舉的詳情（`GetMyReport`）；補充成功後 `ref.invalidate` 重讀。

  StoreReportDetailProvider call(String reportId) =>
      StoreReportDetailProvider._(argument: reportId, from: this);

  @override
  String toString() => r'storeReportDetailProvider';
}
