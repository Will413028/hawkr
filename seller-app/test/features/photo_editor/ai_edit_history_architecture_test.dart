import 'dart:io';

import 'package:hawkr_seller/features/photo_editor/data/ai_edit_history_local_service.dart';
import 'package:hawkr_seller/features/photo_editor/data/ai_edit_history_model.dart';
import 'package:hawkr_seller/features/photo_editor/data/ai_edit_history_repository.dart';
import 'package:hawkr_seller/features/photo_editor/presentation/ai_edit_history_controller.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  test('history repository persists an immutable fifty-entry window', () async {
    SharedPreferences.setMockInitialValues(const {});
    final repository = LocalAiEditHistoryRepository(
      const AiEditHistoryLocalService(),
    );
    List<AiEditHistoryEntry> entries = const [];

    for (var index = 0; index <= 50; index++) {
      entries = await repository.prepend(
        current: entries,
        entry: AiEditHistoryEntry(
          action: '操作 $index',
          timestamp: DateTime.utc(2026, 7, 27, 0, index),
          count: index,
        ),
      );
    }

    final restored = await repository.load();
    expect(restored, hasLength(50));
    expect(restored.first.action, '操作 50');
    expect(restored.last.action, '操作 1');
    expect(
      () => restored.add(
        AiEditHistoryEntry(
          action: '不可變更',
          timestamp: DateTime.utc(2026, 7, 27),
          count: 1,
        ),
      ),
      throwsUnsupportedError,
    );
  });

  test('history controller exposes typed persisted entries', () async {
    SharedPreferences.setMockInitialValues(const {});
    final container = ProviderContainer();
    addTearDown(container.dispose);
    final provider = aiEditHistoryControllerProvider;
    final subscription = container.listen(provider, (_, __) {});
    addTearDown(subscription.close);
    await container.read(provider.future);

    await container
        .read(provider.notifier)
        .addEntry(
          action: '背景去除',
          count: 2,
          timestamp: DateTime.utc(2026, 7, 27),
        );

    final state = container.read(provider).requireValue;
    expect(state.single.action, '背景去除');
    expect(state.single.count, 2);
  });

  test('hawkr_seller#115: clear() wipes persisted history so the next '
      'account on this device starts empty', () async {
    SharedPreferences.setMockInitialValues(const {});
    const service = AiEditHistoryLocalService();
    await service.save([
      AiEditHistoryEntry(
        action: '背景去除',
        timestamp: DateTime.utc(2026, 7, 27),
        count: 1,
      ),
    ]);
    expect(await service.load(), hasLength(1));

    await service.clear();

    expect(await service.load(), isEmpty);
  });

  test('image editor delegates history persistence to its controller', () {
    final source = File(
      'lib/features/photo_editor/presentation/image_editor_screen.dart',
    ).readAsStringSync();

    expect(source, isNot(contains('SharedPreferences')));
    expect(source, isNot(contains('jsonEncode(')));
    expect(source, contains('aiEditHistoryControllerProvider'));
  });
}
