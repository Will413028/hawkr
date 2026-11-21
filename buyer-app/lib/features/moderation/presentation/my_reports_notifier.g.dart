// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'my_reports_notifier.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// 我的檢舉（`ListMyReports`，伺服器合併社群與店家兩種）：分頁讀取，項目原樣保留。換觀看者整份重建。

@ProviderFor(MyReports)
final myReportsProvider = MyReportsProvider._();

/// 我的檢舉（`ListMyReports`，伺服器合併社群與店家兩種）：分頁讀取，項目原樣保留。換觀看者整份重建。
final class MyReportsProvider
    extends $NotifierProvider<MyReports, PagedSection<pb.MyReportItem>> {
  /// 我的檢舉（`ListMyReports`，伺服器合併社群與店家兩種）：分頁讀取，項目原樣保留。換觀看者整份重建。
  MyReportsProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'myReportsProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$myReportsHash();

  @$internal
  @override
  MyReports create() => MyReports();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(PagedSection<pb.MyReportItem> value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<PagedSection<pb.MyReportItem>>(
        value,
      ),
    );
  }
}

String _$myReportsHash() => r'906959ed2e6b6ba50797ed8c3dd082adc611a02c';

/// 我的檢舉（`ListMyReports`，伺服器合併社群與店家兩種）：分頁讀取，項目原樣保留。換觀看者整份重建。

abstract class _$MyReports extends $Notifier<PagedSection<pb.MyReportItem>> {
  PagedSection<pb.MyReportItem> build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref =
        this.ref
            as $Ref<
              PagedSection<pb.MyReportItem>,
              PagedSection<pb.MyReportItem>
            >;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<
                PagedSection<pb.MyReportItem>,
                PagedSection<pb.MyReportItem>
              >,
              PagedSection<pb.MyReportItem>,
              Object?,
              Object?
            >;
    element.handleCreate(ref, build);
  }
}
