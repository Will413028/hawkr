import 'dart:async';

import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.pb.dart' as pb;
import 'package:hawkr_buyer/config/flavor.dart';
import 'package:hawkr_buyer/core/networking/connect_provider.dart';
import 'package:hawkr_buyer/core/session/session_model.dart';
import 'package:hawkr_buyer/core/session/session_provider.dart';
import 'package:hawkr_buyer/features/social/presentation/post_feed_controller.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import 'social_fakes.dart';

class _GuestSession extends Session {
  @override
  Future<SessionModel?> build() async => null;
}

pb.Post _post(int id) => pb.Post(id: '$id', body: '貼文 $id');

void main() {
  if (!F.isInitialized) F.init(Flavor.dev);

  test('重整前發出的 loadMore 晚回來：丟棄，不接到新列表也不蓋掉新的 page token', () async {
    final staleGate = Completer<void>();
    final tokens = <String>[];
    var firstBuild = true;
    final public = FakeSocialPublic();
    public.postsResponder = (req) async {
      tokens.add(req.pageToken);
      if (req.pageToken.isEmpty) {
        final first = firstBuild;
        firstBuild = false;
        return pb.ListPostsResponse(
          posts: [_post(first ? 1 : 10)],
          nextPageToken: first ? 'old-2' : 'new-2',
        );
      }
      if (req.pageToken == 'old-2') {
        await staleGate.future;
        return pb.ListPostsResponse(
          posts: [_post(99)],
          nextPageToken: 'stale-3',
        );
      }
      return pb.ListPostsResponse(posts: [_post(11)]);
    };
    final container = ProviderContainer(
      overrides: [
        socialPublicClientProvider.overrideWithValue(public.client()),
        socialAppClientProvider.overrideWithValue(FakeSocialApp().client()),
        sessionProvider.overrideWith(_GuestSession.new),
      ],
    );
    addTearDown(container.dispose);
    final provider = postFeedControllerProvider(merchantId: 7);
    await container.read(provider.future);
    final notifier = container.read(provider.notifier);

    final stale = notifier.loadMore();
    await Future<void>.delayed(Duration.zero);
    container.invalidate(provider);
    expect((await container.read(provider.future)).ids, [10]);

    staleGate.complete();
    await stale;
    expect(container.read(provider).value!.ids, [10]);

    await notifier.loadMore();
    expect(tokens.last, 'new-2');
    expect(container.read(provider).value!.ids, [10, 11]);
  });

  test('舊一輪的 loadMore 結束時不解除新一輪 loadMore 的進行中旗標', () async {
    final staleGate = Completer<void>();
    final freshGate = Completer<void>();
    var firstBuild = true;
    var freshRequests = 0;
    final public = FakeSocialPublic();
    public.postsResponder = (req) async {
      if (req.pageToken.isEmpty) {
        final first = firstBuild;
        firstBuild = false;
        return pb.ListPostsResponse(
          posts: [_post(first ? 1 : 10)],
          nextPageToken: first ? 'old-2' : 'new-2',
        );
      }
      if (req.pageToken == 'old-2') {
        await staleGate.future;
        return pb.ListPostsResponse(posts: [_post(99)]);
      }
      freshRequests++;
      await freshGate.future;
      return pb.ListPostsResponse(posts: [_post(11)]);
    };
    final container = ProviderContainer(
      overrides: [
        socialPublicClientProvider.overrideWithValue(public.client()),
        socialAppClientProvider.overrideWithValue(FakeSocialApp().client()),
        sessionProvider.overrideWith(_GuestSession.new),
      ],
    );
    addTearDown(container.dispose);
    final provider = postFeedControllerProvider(merchantId: 7);
    await container.read(provider.future);
    final notifier = container.read(provider.notifier);

    final stale = notifier.loadMore();
    await Future<void>.delayed(Duration.zero);
    container.invalidate(provider);
    await container.read(provider.future);
    final fresh = notifier.loadMore();
    await Future<void>.delayed(Duration.zero);

    staleGate.complete();
    await stale;
    final duplicate = notifier.loadMore();
    await Future<void>.delayed(Duration.zero);

    expect(freshRequests, 1);
    freshGate.complete();
    await Future.wait([fresh, duplicate]);
  });

  test('登入狀態改變後重讀 feed（讚與留言權限屬於另一個觀看者）', () async {
    final public = FakeSocialPublic(posts: [_post(1)]);
    final container = ProviderContainer(
      overrides: [
        socialPublicClientProvider.overrideWithValue(public.client()),
        socialAppClientProvider.overrideWithValue(FakeSocialApp().client()),
        sessionProvider.overrideWith(SwitchableSession.new),
      ],
    );
    addTearDown(container.dispose);
    final provider = postFeedControllerProvider(merchantId: 7);
    await container.read(provider.future);
    await container.read(sessionProvider.future);
    expect(public.listPostsCalls, 1);

    (container.read(sessionProvider.notifier) as SwitchableSession).signIn();
    await container.read(provider.future);

    expect(public.listPostsCalls, 2);
  });
}
