import 'package:riverpod_annotation/riverpod_annotation.dart';

import 'ai_edit_history_local_service.dart';
import 'ai_edit_history_model.dart';

part 'ai_edit_history_repository.g.dart';

abstract interface class AiEditHistoryRepository {
  Future<List<AiEditHistoryEntry>> load();

  Future<List<AiEditHistoryEntry>> prepend({
    required List<AiEditHistoryEntry> current,
    required AiEditHistoryEntry entry,
  });
}

class LocalAiEditHistoryRepository implements AiEditHistoryRepository {
  const LocalAiEditHistoryRepository(this._localService);

  final AiEditHistoryLocalService _localService;

  @override
  Future<List<AiEditHistoryEntry>> load() async {
    try {
      return await _localService.load();
    } catch (_) {
      return const [];
    }
  }

  @override
  Future<List<AiEditHistoryEntry>> prepend({
    required List<AiEditHistoryEntry> current,
    required AiEditHistoryEntry entry,
  }) async {
    final next = List<AiEditHistoryEntry>.unmodifiable([
      entry,
      ...current.take(49),
    ]);
    await _localService.save(next);
    return next;
  }
}

@riverpod
AiEditHistoryRepository aiEditHistoryRepository(Ref ref) {
  return LocalAiEditHistoryRepository(
    ref.watch(aiEditHistoryLocalServiceProvider),
  );
}
