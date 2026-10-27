import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../data/ai_edit_history_model.dart';
import '../data/ai_edit_history_repository.dart';

part 'ai_edit_history_controller.g.dart';

@riverpod
class AiEditHistoryController extends _$AiEditHistoryController {
  @override
  Future<List<AiEditHistoryEntry>> build() {
    return ref.read(aiEditHistoryRepositoryProvider).load();
  }

  Future<void> addEntry({
    required String action,
    required int count,
    DateTime? timestamp,
  }) async {
    final repository = ref.read(aiEditHistoryRepositoryProvider);
    final current = state.value ?? await repository.load();
    final next = await repository.prepend(
      current: current,
      entry: AiEditHistoryEntry(
        action: action,
        timestamp: timestamp ?? DateTime.now(),
        count: count,
      ),
    );
    state = AsyncData(next);
  }
}
