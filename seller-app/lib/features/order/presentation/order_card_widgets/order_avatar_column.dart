import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

import '../../../../shared/widgets/buyer_avatar_widget.dart';
import '../../data/buyer_avatar_provider.dart';
import '../../data/order_model.dart';

class OrderAvatarColumn extends ConsumerWidget {
  const OrderAvatarColumn({super.key, required this.order});

  final OrderSummary order;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final name = order.recipientName.isNotEmpty
        ? order.recipientName
        : order.buyerName.isNotEmpty
        ? order.buyerName
        : '買家';

    String? avatarUrl = order.buyerAvatar.isNotEmpty ? order.buyerAvatar : null;
    if (avatarUrl == null && order.buyerId != null) {
      final avatarAsync = ref.watch(buyerAvatarProvider(order.buyerId!));
      avatarUrl = avatarAsync.value;
    }

    return SizedBox(
      width: 48.w,
      child: BuyerAvatarWidget(size: 48, imageUrl: avatarUrl, name: name),
    );
  }
}
