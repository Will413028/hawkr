import 'package:flutter/widgets.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../shared/widgets/seller_avatar_widget.dart';
import '../data/current_store.dart';

/// 目前登入店家的頭像：讀 `GetMyStore` 的 `logo`。
class SellerStoreAvatar extends ConsumerWidget {
  const SellerStoreAvatar({super.key, this.size = 40});

  final double size;

  @override
  Widget build(BuildContext context, WidgetRef ref) => SellerAvatarWidget(
    size: size,
    imageUrl: ref.watch(myStoreSnapshotProvider)?.logoUrl,
  );
}
