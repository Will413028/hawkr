import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../core/session/session_provider.dart';
import '../data/store_pickup_model.dart';
import '../data/store_pickup_repository.dart';

final storePickupControllerProvider =
    AsyncNotifierProvider<StorePickupController, List<StorePickup>>(
      StorePickupController.new,
    );

class StorePickupController extends AsyncNotifier<List<StorePickup>> {
  @override
  Future<List<StorePickup>> build() async {
    final session = ref.watch(sessionProvider).value;
    if (!hasUsableSession(session)) return const [];
    return ref.watch(storePickupRepositoryProvider).getAll();
  }

  Future<void> reload() async {
    state = const AsyncLoading();
    state = await AsyncValue.guard(
      () => ref.read(storePickupRepositoryProvider).getAll(),
    );
  }

  Future<void> setDefault(StorePickup pickup) async {
    if (pickup.isDefault) return;
    await ref
        .read(storePickupRepositoryProvider)
        .update(pickup.copyWith(isDefault: true));
    await reload();
  }

  Future<void> save(StorePickup pickup) async {
    final repository = ref.read(storePickupRepositoryProvider);
    if (pickup.id.isEmpty) {
      await repository.create(pickup);
    } else {
      await repository.update(pickup);
    }
    await reload();
  }

  Future<String> createMapSession() =>
      ref.read(storePickupRepositoryProvider).createMapSession();

  Future<void> delete(String id) async {
    await ref.read(storePickupRepositoryProvider).delete(id);
    await reload();
  }
}
