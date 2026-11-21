import 'package:hawkr_api/gen/hawkr/moderation/v1alpha1/moderation.pb.dart'
    as pb;
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/session/viewer_scoped_notifier.dart';
import '../data/moderation_repository.dart';
import 'paged_section.dart';

part 'my_reports_notifier.g.dart';

/// 我的檢舉（`ListMyReports`，伺服器合併社群與店家兩種）：分頁讀取，項目原樣保留。換觀看者整份重建。
@riverpod
class MyReports extends _$MyReports with ViewerScoped {
  late SectionLoader<pb.MyReportItem> _loader;

  @override
  PagedSection<pb.MyReportItem> build() {
    bindViewer();
    _loader = SectionLoader<pb.MyReportItem>(
      read: () => state,
      write: (next) => state = next,
      fetch: (token) async {
        final r = await ref
            .read(moderationRepositoryProvider)
            .listMyReports(pageToken: token);
        return (items: r.reports, nextPageToken: r.nextPageToken);
      },
      scope: beginScope,
    );
    scheduleInitialLoad(beginScope(), reload);
    return const PagedSection<pb.MyReportItem>();
  }

  Future<bool> reload() => _loader.loadFirst();

  Future<bool> loadMore() => _loader.loadMore();
}
