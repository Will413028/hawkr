import 'dart:async';

import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.pbenum.dart';
import 'package:hawkr_api/transport.dart';
import '../../support/connect_errors.dart';
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
    subscription = container.listen(provider, (_, _) {}, fireImmediately: true);
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
        () => repository.publishPostOrThrow(42, etag: any(named: 'etag')),
      ).thenThrow(
        reasonError(
          Code.failedPrecondition,
          'POST_NOT_PUBLISHABLE',
          message: '目前無法發布',
        ),
      );
      final notifier = container.read(provider.notifier);

      await expectLater(
        notifier.setPublished(42, publish: true),
        throwsA(
          isA<ConnectException>().having(
            (error) => error.code,
            'code',
            Code.failedPrecondition,
          ),
        ),
      );

      expect(
        container.read(provider).requireValue.single.status,
        PostStatus.POST_STATUS_DRAFT,
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

    request.completeError(ConnectException(Code.deadlineExceeded, 'timeout'));
    await expectLater(operation, throwsA(isA<ConnectException>()));

    expect(container.read(provider).requireValue.single.id, 42);
  });

  test(
    'publish success has no optimistic state and refetches the list',
    () async {
      final request = Completer<void>();
      when(
        () => repository.publishPostOrThrow(42, etag: any(named: 'etag')),
      ).thenAnswer((_) => request.future);
      var listCalls = 0;
      when(() => repository.listMyPosts()).thenAnswer((_) async {
        listCalls++;
        return [
          listCalls == 1
              ? _post
              : _post.copyWith(status: PostStatus.POST_STATUS_PUBLISHED),
        ];
      });
      container.invalidate(provider);
      await container.read(provider.future);
      final notifier = container.read(provider.notifier);

      final operation = notifier.setPublished(42, publish: true);
      await Future<void>.delayed(Duration.zero);
      expect(
        container.read(provider).requireValue.single.status,
        PostStatus.POST_STATUS_DRAFT,
        reason: 'no optimistic update while the request is pending',
      );

      request.complete();
      await operation;
      expect(
        container.read(provider).requireValue.single.status,
        PostStatus.POST_STATUS_PUBLISHED,
      );
      verify(
        () => repository.publishPostOrThrow(42, etag: any(named: 'etag')),
      ).called(1);
    },
  );

  test('unpublish calls the unpublish endpoint', () async {
    when(
      () => repository.unpublishPostOrThrow(42, etag: any(named: 'etag')),
    ).thenAnswer((_) async {});
    await container.read(provider.notifier).setPublished(42, publish: false);
    verify(
      () => repository.unpublishPostOrThrow(42, etag: any(named: 'etag')),
    ).called(1);
    verifyNever(
      () => repository.publishPostOrThrow(any(), etag: any(named: 'etag')),
    );
  });
}

const _post = Post(
  id: 42,
  content: '測試貼文',
  status: PostStatus.POST_STATUS_DRAFT,
);
