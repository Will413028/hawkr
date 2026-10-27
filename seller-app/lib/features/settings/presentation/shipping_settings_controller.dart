import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../data/shipping_settings_repository.dart';

part 'shipping_settings_controller.g.dart';

@Riverpod(keepAlive: true)
class ShippingSettingsController extends _$ShippingSettingsController {
  @override
  void build() {}

  Future<ShippingSettingsBootstrap> loadBootstrap() {
    return ref.read(shippingSettingsRepositoryProvider).loadBootstrap();
  }

  Future<Map<String, dynamic>?> loadStoreMapSession() {
    return ref.read(shippingSettingsRepositoryProvider).loadStoreMapSession();
  }

  Future<ShippingCommandResult> activateHomeDelivery(
    Map<String, dynamic> body,
  ) {
    return ref
        .read(shippingSettingsRepositoryProvider)
        .activateHomeDelivery(body);
  }

  Future<Map<String, dynamic>> sendCvsMutation(
    String path,
    Map<String, dynamic> body,
  ) {
    return ref
        .read(shippingSettingsRepositoryProvider)
        .sendMutation(path, body);
  }
}
