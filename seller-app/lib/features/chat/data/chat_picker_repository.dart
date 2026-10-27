import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/codemeta/code_meta.dart';
import '../../../core/exceptions/app_exception.dart';
import '../../product/data/product_repository.dart';
import 'chat_picker_model.dart';
import 'chat_picker_remote_service.dart';

part 'chat_picker_repository.g.dart';

abstract interface class ChatPickerRepository {
  Future<List<ChatOrderPickerOption>> loadOrders(String buyerAccountId);

  Future<List<ChatProductPickerOption>> loadProducts();
}

class RemoteChatPickerRepository implements ChatPickerRepository {
  const RemoteChatPickerRepository(this._remoteService, this._products);

  final ChatPickerRemoteService _remoteService;
  final ProductRepository _products;

  @override
  Future<List<ChatOrderPickerOption>> loadOrders(String buyerAccountId) async {
    final rows = await _remoteService.fetchBuyerOrders(buyerAccountId);
    return List<ChatOrderPickerOption>.unmodifiable(
      rows.map(ChatOrderPickerOption.new),
    );
  }

  @override
  Future<List<ChatProductPickerOption>> loadProducts() async {
    final result = await _products.getMyProducts(
      status: ProductStatus.active.value,
    );
    final code = result['code'];
    if (code != null && code.toString() != '0') {
      throw AppException.fromApiError(
        int.tryParse(code.toString()) ?? -1,
        (result['messageZh'] ?? result['message'] ?? '載入失敗').toString(),
      );
    }
    final data = result['data'];
    final rows = data is Map && data['records'] is List
        ? data['records'] as List<dynamic>
        : data is List
        ? data
        : const <dynamic>[];
    return List<ChatProductPickerOption>.unmodifiable(
      rows.whereType<Map>().map(
        (row) => ChatProductPickerOption(Map<String, dynamic>.from(row)),
      ),
    );
  }
}

@riverpod
ChatPickerRepository chatPickerRepository(Ref ref) {
  return RemoteChatPickerRepository(
    ref.watch(chatPickerRemoteServiceProvider),
    ref.watch(productRepositoryProvider),
  );
}
