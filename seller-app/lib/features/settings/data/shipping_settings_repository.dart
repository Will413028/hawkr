import 'dart:collection';

import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/exceptions/app_exception.dart';
import '../../product/data/product_repository.dart';
import 'shipping_settings_remote_service.dart';

part 'shipping_settings_repository.g.dart';

final class ShippingSettingsBootstrap {
  ShippingSettingsBootstrap({
    required Iterable<Map<String, dynamic>> onboardings,
  }) : onboardings = UnmodifiableListView<Map<String, dynamic>>(
         onboardings
             .map(
               (entry) => Map<String, dynamic>.unmodifiable(
                 Map<String, dynamic>.of(entry),
               ),
             )
             .toList(growable: false),
       );

  final UnmodifiableListView<Map<String, dynamic>> onboardings;
}

final class ShippingCommandResult {
  const ShippingCommandResult._({
    required this.isSuccess,
    required this.message,
  });

  const ShippingCommandResult.success() : this._(isSuccess: true, message: '');

  const ShippingCommandResult.failure(String message)
    : this._(isSuccess: false, message: message);

  final bool isSuccess;
  final String message;
}

abstract interface class ShippingSettingsRepository {
  Future<ShippingSettingsBootstrap> loadBootstrap();

  Future<Map<String, dynamic>?> loadStoreMapSession();

  Future<ShippingCommandResult> activateHomeDelivery(Map<String, dynamic> body);

  Future<Map<String, dynamic>> sendMutation(
    String path,
    Map<String, dynamic> body,
  );
}

class RemoteShippingSettingsRepository implements ShippingSettingsRepository {
  const RemoteShippingSettingsRepository(
    this._remoteService,
    this._productRepository,
  );

  final ShippingSettingsRemoteService _remoteService;
  final ProductRepository _productRepository;

  @override
  Future<ShippingSettingsBootstrap> loadBootstrap() async {
    List<Map<String, dynamic>> onboardings = const [];
    try {
      onboardings = await _remoteService.fetchOnboardings();
    } on AppException {
      // Preserve the existing partial/fail-soft bootstrap.
    }
    return ShippingSettingsBootstrap(onboardings: onboardings);
  }

  @override
  Future<Map<String, dynamic>?> loadStoreMapSession() async {
    final session = await _productRepository.getStoreMapSession();
    return session == null ? null : Map<String, dynamic>.unmodifiable(session);
  }

  @override
  Future<ShippingCommandResult> activateHomeDelivery(
    Map<String, dynamic> body,
  ) async {
    try {
      await _remoteService.sendMutation(
        '/api/v1/app/shipping_onboardings/home_delivery',
        body,
      );
      return const ShippingCommandResult.success();
    } on AppException catch (error) {
      return ShippingCommandResult.failure(error.message);
    }
  }

  @override
  Future<Map<String, dynamic>> sendMutation(
    String path,
    Map<String, dynamic> body,
  ) {
    return _remoteService.sendMutation(path, body);
  }
}

@riverpod
ShippingSettingsRepository shippingSettingsRepository(Ref ref) {
  return RemoteShippingSettingsRepository(
    ref.watch(shippingSettingsRemoteServiceProvider),
    ref.watch(productRepositoryProvider),
  );
}
