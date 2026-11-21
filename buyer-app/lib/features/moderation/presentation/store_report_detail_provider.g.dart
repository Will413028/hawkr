// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'store_report_detail_provider.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// 我的一筆店家檢舉的詳情（`GetMyReport`）；補充成功後以回應的完整詳情取代狀態，不重讀。

@ProviderFor(StoreReportDetail)
final storeReportDetailProvider = StoreReportDetailFamily._();

/// 我的一筆店家檢舉的詳情（`GetMyReport`）；補充成功後以回應的完整詳情取代狀態，不重讀。
final class StoreReportDetailProvider
    extends $AsyncNotifierProvider<StoreReportDetail, pb.GetMyReportResponse> {
  /// 我的一筆店家檢舉的詳情（`GetMyReport`）；補充成功後以回應的完整詳情取代狀態，不重讀。
  StoreReportDetailProvider._({
    required StoreReportDetailFamily super.from,
    required String super.argument,
  }) : super(
         retry: _noAutomaticRetry,
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
  StoreReportDetail create() => StoreReportDetail();

  @override
  bool operator ==(Object other) {
    return other is StoreReportDetailProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$storeReportDetailHash() => r'18ecc7d622a6d0d24cb72aad93365153bb43606f';

/// 我的一筆店家檢舉的詳情（`GetMyReport`）；補充成功後以回應的完整詳情取代狀態，不重讀。

final class StoreReportDetailFamily extends $Family
    with
        $ClassFamilyOverride<
          StoreReportDetail,
          AsyncValue<pb.GetMyReportResponse>,
          pb.GetMyReportResponse,
          FutureOr<pb.GetMyReportResponse>,
          String
        > {
  StoreReportDetailFamily._()
    : super(
        retry: _noAutomaticRetry,
        name: r'storeReportDetailProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// 我的一筆店家檢舉的詳情（`GetMyReport`）；補充成功後以回應的完整詳情取代狀態，不重讀。

  StoreReportDetailProvider call(String reportId) =>
      StoreReportDetailProvider._(argument: reportId, from: this);

  @override
  String toString() => r'storeReportDetailProvider';
}

/// 我的一筆店家檢舉的詳情（`GetMyReport`）；補充成功後以回應的完整詳情取代狀態，不重讀。

abstract class _$StoreReportDetail
    extends $AsyncNotifier<pb.GetMyReportResponse> {
  late final _$args = ref.$arg as String;
  String get reportId => _$args;

  FutureOr<pb.GetMyReportResponse> build(String reportId);
  @$mustCallSuper
  @override
  void runBuild() {
    final ref =
        this.ref
            as $Ref<AsyncValue<pb.GetMyReportResponse>, pb.GetMyReportResponse>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<
                AsyncValue<pb.GetMyReportResponse>,
                pb.GetMyReportResponse
              >,
              AsyncValue<pb.GetMyReportResponse>,
              Object?,
              Object?
            >;
    element.handleCreate(ref, () => build(_$args));
  }
}
