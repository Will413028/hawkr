import 'package:riverpod_annotation/riverpod_annotation.dart';

import 'customer_model.dart';
import 'customer_repository.dart';
import 'subscriber_model.dart';
import 'subscriber_remote_service.dart';

part 'subscriber_repository.g.dart';

abstract interface class SubscriberRepository {
  Future<({List<Subscriber> subscribers, String nextPageToken})>
  loadSubscribers({String pageToken = '', int size = 20});

  Future<Customer?> resolveCustomer(String accountId);
}

/// Subscriber single source of truth: one `ListMyFollowers` request per page.
class RemoteSubscriberRepository implements SubscriberRepository {
  const RemoteSubscriberRepository({
    required SubscriberRemoteService remoteService,
    required CustomerRepository customerRepository,
  }) : _remoteService = remoteService,
       _customerRepository = customerRepository;

  final SubscriberRemoteService _remoteService;
  final CustomerRepository _customerRepository;

  @override
  Future<({List<Subscriber> subscribers, String nextPageToken})>
  loadSubscribers({String pageToken = '', int size = 20}) async {
    final response = await _remoteService.fetchFollowers(
      pageToken: pageToken,
      size: size,
    );
    return (
      subscribers: List<Subscriber>.unmodifiable(
        response.followers.map(
          (follower) => Subscriber(
            buyerId: follower.buyerId,
            subscribedAt: follower.subscribedAt,
            nickname: follower.displayName,
            avatarUrl: follower.avatarUrl,
            deleted: follower.deleted,
          ),
        ),
      ),
      nextPageToken: response.nextPageToken,
    );
  }

  @override
  Future<Customer?> resolveCustomer(String accountId) {
    return _customerRepository.getCustomerByAccountId(accountId);
  }
}

@riverpod
SubscriberRepository subscriberRepository(Ref ref) {
  return RemoteSubscriberRepository(
    remoteService: ref.watch(subscriberRemoteServiceProvider),
    customerRepository: ref.watch(customerRepositoryProvider),
  );
}
