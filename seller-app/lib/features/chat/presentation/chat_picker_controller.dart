import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/exceptions/app_exception.dart';
import '../data/chat_picker_model.dart';
import '../data/chat_picker_repository.dart';

part 'chat_picker_controller.g.dart';

enum ChatPickerKind { order, product }

sealed class ChatPickerState {
  const ChatPickerState();
}

final class ChatPickerIdle extends ChatPickerState {
  const ChatPickerIdle();
}

final class ChatPickerLoading extends ChatPickerState {
  const ChatPickerLoading(this.kind);

  final ChatPickerKind kind;
}

final class ChatPickerReady extends ChatPickerState {
  const ChatPickerReady(this.kind, this.itemCount);

  final ChatPickerKind kind;
  final int itemCount;
}

final class ChatPickerFailure extends ChatPickerState {
  const ChatPickerFailure(this.kind, this.message);

  final ChatPickerKind kind;
  final String message;
}

@Riverpod(keepAlive: true)
class ChatPickerController extends _$ChatPickerController {
  var _generation = 0;

  @override
  ChatPickerState build() => const ChatPickerIdle();

  Future<List<ChatOrderPickerOption>> loadOrders(String buyerAccountId) async {
    final generation = ++_generation;
    state = const ChatPickerLoading(ChatPickerKind.order);
    try {
      final items = await ref
          .read(chatPickerRepositoryProvider)
          .loadOrders(buyerAccountId);
      if (generation == _generation) {
        state = ChatPickerReady(ChatPickerKind.order, items.length);
      }
      return items;
    } catch (error) {
      if (generation == _generation) {
        state = ChatPickerFailure(
          ChatPickerKind.order,
          chatPickerErrorMessage(error),
        );
      }
      rethrow;
    }
  }

  Future<List<ChatProductPickerOption>> loadProducts() async {
    final generation = ++_generation;
    state = const ChatPickerLoading(ChatPickerKind.product);
    try {
      final items = await ref.read(chatPickerRepositoryProvider).loadProducts();
      if (generation == _generation) {
        state = ChatPickerReady(ChatPickerKind.product, items.length);
      }
      return items;
    } catch (error) {
      if (generation == _generation) {
        state = ChatPickerFailure(
          ChatPickerKind.product,
          chatPickerErrorMessage(error),
        );
      }
      rethrow;
    }
  }

  void reset() {
    _generation++;
    state = const ChatPickerIdle();
  }
}

String chatPickerErrorMessage(Object error) {
  if (error case AppException(:final message)) return message;
  return '無法載入選擇清單，請稍後再試';
}
