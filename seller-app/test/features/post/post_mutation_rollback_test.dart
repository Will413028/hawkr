import 'dart:async';

import 'package:hawkr_seller/core/codemeta/code_meta.dart';
import 'package:hawkr_seller/core/exceptions/app_exception.dart';
import 'package:hawkr_seller/features/post/data/post_model.dart';
import 'package:hawkr_seller/features/post/data/post_repository.dart';
import 'package:hawkr_seller/features/post/presentation/post_controller.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';

class _MockPostRepository extends Mock implements PostRepository {}

void main() {
  late _MockPostRepository repository;
  late ProviderContainer container;
  late PostListProvider provider;
  late ProviderSubscription<AsyncValue<List<Post>>> subscription;

  setUp(() async {
    repository = _MockPostRepository();
    when(() => repository.listMyPosts()).thenAnswer((_) async => [_post]);
    container = ProviderContainer(
      overrides: [postRepositoryProvider.overrideWithValue(repository)],
    );
    provider = postListProvider;
    subscription = container.listen(
      provider,
      (_, __) {},
      fireImmediately: true,
    );
    await container.read(provider.future);
  });

  tearDown(() {
    subscription.close();
    container.dispose();
  });

  test(
    'publish failure leaves the list unchanged and does not refetch',
    () async {
      when(
        () => repository.publishPostOrThrow(42),
      ).thenThrow(const ApiException(code: 9001, message: '目前無法發布'));
      final notifier = container.read(provider.notifier);

      await expectLater(
        notifier.setPublished(42, publish: true),
        throwsA(
          isA<ApiException>().having(
            (error) => error.message,
            'message',
            '目前無法發布',
          ),
        ),
      );

      expect(
        container.read(provider).requireValue.single.publishStatus,
        '4401',
      );
      verify(() => repository.listMyPosts()).called(1);
    },
  );

  test('timeout rolls delete state back', () async {
    final request = Completer<void>();
    when(() => repository.deletePost(42)).thenAnswer((_) => request.future);
    final notifier = container.read(provider.notifier);

    final operation = notifier.deletePost(42);
    await Future<void>.delayed(Duration.zero);
    expect(container.read(provider).requireValue, isEmpty);

    request.completeError(const TimeoutException(message: '連線逾時，請稍後再試'));
    await expectLater(operation, throwsA(isA<TimeoutException>()));

    expect(container.read(provider).requireValue.single.id, 42);
  });

  test(
    'publish success has no optimistic state and refetches the list',
    () async {
      final request = Completer<void>();
      when(
        () => repository.publishPostOrThrow(42),
      ).thenAnswer((_) => request.future);
      var listCalls = 0;
      when(() => repository.listMyPosts()).thenAnswer((_) async {
        listCalls++;
        return [
          listCalls == 1
              ? _post
              : _post.copyWith(
                  publishStatus: PostPublishStatus.published.value,
                ),
        ];
      });
      container.invalidate(provider);
      await container.read(provider.future);
      final notifier = container.read(provider.notifier);

      final operation = notifier.setPublished(42, publish: true);
      await Future<void>.delayed(Duration.zero);
      expect(
        container.read(provider).requireValue.single.publishStatus,
        '4401',
        reason: 'no optimistic update while the request is pending',
      );

      request.complete();
      await operation;
      expect(
        container.read(provider).requireValue.single.publishStatus,
        '4402',
      );
      verify(() => repository.publishPostOrThrow(42)).called(1);
    },
  );

  test('unpublish calls the unpublish endpoint', () async {
    when(() => repository.unpublishPostOrThrow(42)).thenAnswer((_) async {});
    await container.read(provider.notifier).setPublished(42, publish: false);
    verify(() => repository.unpublishPostOrThrow(42)).called(1);
    verifyNever(() => repository.publishPostOrThrow(any()));
  });
}

const _post = Post(id: 42, sellerId: 7, content: '測試貼文', publishStatus: '4401');
