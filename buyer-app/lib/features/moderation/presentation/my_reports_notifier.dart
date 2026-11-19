import 'package:hawkr_api/gen/hawkr/moderation/v1alpha1/moderation.pb.dart'
    as pb;
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/session/viewer_scoped_notifier.dart';
import '../data/moderation_repository.dart';
import 'paged_section.dart';

part 'my_reports_notifier.g.dart';

/// 我的檢舉（`ListMyReports`，只含社群對象）：分頁讀取。換觀看者整份重建。
@riverpod
class MyReports extends _$MyReports with ViewerScoped {
  late SectionLoader<pb.ReportRecord> _loader;

  @override
  PagedSection<pb.ReportRecord> build() {
    bindViewer();
    _loader = SectionLoader<pb.ReportRecord>(
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
    return const PagedSection<pb.ReportRecord>();
  }

  Future<bool> reload() => _loader.loadFirst();

  Future<bool> loadMore() => _loader.loadMore();
}
