import 'package:riverpod_annotation/riverpod_annotation.dart';
import '../data/order_model.dart';
import '../data/order_repository.dart';

part 'order_detail_controller.g.dart';

@riverpod
class OrderDetailController extends _$OrderDetailController {
  @override
  Future<Order> build(String orderSn) async {
    return ref.watch(orderRepositoryProvider).getOrderDetail(orderSn);
  }

  Future<void> refresh() async {
    ref.invalidateSelf();
    await future;
  }
}
