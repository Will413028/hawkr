import 'dart:io';
import 'dart:typed_data';

import 'package:hawkr_seller/core/exceptions/app_exception.dart';
import 'package:hawkr_seller/core/networking/dio_provider.dart';
import 'package:hawkr_seller/features/media/data/protected_media_repository.dart';
import 'package:hawkr_seller/features/media/presentation/protected_media_controller.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

void main() {
  test(
    'protected media command uses the injected central Dio client',
    () async {
      final adapter = _BytesAdapter(statusCode: 200, bytes: [1, 2, 3, 4]);
      final dio = Dio(BaseOptions(baseUrl: 'https://example.test'))
        ..httpClientAdapter = adapter;
      final container = ProviderContainer(
        overrides: [dioProvider.overrideWithValue(dio)],
      );
      addTearDown(container.dispose);

      final bytes = await container
          .read(protectedMediaControllerProvider.notifier)
          .download('/secure/preview');

      expect(bytes, Uint8List.fromList([1, 2, 3, 4]));
      expect(adapter.request?.path, '/secure/preview');
    },
  );

  test('protected media repository keeps non-2xx failure typed', () async {
    final adapter = _BytesAdapter(statusCode: 404, bytes: const []);
    final dio = Dio(BaseOptions(baseUrl: 'https://example.test'))
      ..httpClientAdapter = adapter;
    final container = ProviderContainer(
      overrides: [dioProvider.overrideWithValue(dio)],
    );
    addTearDown(container.dispose);
    final repository = container.read(protectedMediaRepositoryProvider);

    expect(
      () => repository.download('/missing'),
      throwsA(isA<ClientException>()),
    );
  });

  test('Batch A and B views no longer own Dio and repository is in data', () {
    final certification = [
      'lib/features/seller/presentation/seller_certification_upload_screen.dart',
      'lib/features/seller/presentation/kyc_wizard_steps.dart',
      'lib/features/seller/presentation/kyc_wizard_widgets.dart',
    ].map((path) => File(path).readAsStringSync()).join('\n');
    final productImage = File(
      'lib/features/product/presentation/product_setup_image_screen.dart',
    ).readAsStringSync();

    expect(certification, isNot(contains('package:dio/dio.dart')));
    expect(certification, isNot(contains('dioProvider')));
    expect(productImage, isNot(contains('package:dio/dio.dart')));
    expect(productImage, isNot(contains('dioProvider')));
    expect(
      File(
        'lib/features/product/data/product_listing_chat_repository.dart',
      ).existsSync(),
      isTrue,
    );
    expect(
      File(
        'lib/features/product/presentation/product_listing_chat/'
        'product_listing_chat_repository.dart',
      ).existsSync(),
      isFalse,
    );
  });
}

class _BytesAdapter implements HttpClientAdapter {
  _BytesAdapter({required this.statusCode, required this.bytes});

  final int statusCode;
  final List<int> bytes;
  RequestOptions? request;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    request = options;
    return ResponseBody(
      Stream<Uint8List>.value(Uint8List.fromList(bytes)),
      statusCode,
      headers: {
        Headers.contentTypeHeader: ['application/octet-stream'],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}
