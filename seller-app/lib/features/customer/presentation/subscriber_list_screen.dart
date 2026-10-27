import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../core/theme/app_colors.dart';
import '../../../shared/utils/floating_toast.dart';
import '../../../shared/widgets/hk_glass_app_bar.dart';
import 'subscriber_list_controller.dart';
import 'widgets/subscriber_list_content.dart';

class SubscriberListScreen extends ConsumerWidget {
  const SubscriberListScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final state = ref.watch(subscriberListControllerProvider);

    return Scaffold(
      backgroundColor: AppColors.bgPrimary,
      appBar: const HkGlassAppBar(title: Text('店舖訂閱者')),
      body: SubscriberListContent(
        state: state,
        onLoadMore: () {
          ref
              .read(subscriberListControllerProvider.notifier)
              .loadMore()
              .catchError((Object error) {
                if (context.mounted) {
                  FloatingToast.error(
                    context,
                    subscriberListErrorMessage(error),
                  );
                }
              });
        },
        onSubscriberTap: (subscriber) async {
          final customer = await ref
              .read(subscriberListControllerProvider.notifier)
              .resolveCustomer(subscriber.buyerId);
          if (customer != null && context.mounted) {
            context.push('/customer/detail', extra: customer);
          }
        },
      ),
    );
  }
}
