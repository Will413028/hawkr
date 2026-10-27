import 'package:flutter/material.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../shared/icons/app_icons.dart';
import '../../../shared/utils/floating_toast.dart';
import '../../../shared/widgets/chat_order_picker_sheet.dart';
import '../../../shared/widgets/chat_product_picker_sheet.dart';
import '../../ai_assistant/presentation/ai_function_menu.dart';
import '../data/chat_thread_notifier.dart';
import 'chat_picker_controller.dart';
import 'chat_preview.dart';

/// Composer attachment menu extracted from the chat thread screen.
void showChatAttachmentMenu({
  required BuildContext context,
  required VoidCallback onPickPhoto,
  required VoidCallback onPickOrder,
  required VoidCallback onPickProduct,
}) {
  showAiFunctionMenu(
    context: context,
    items: [
      AiFunctionMenuItem(
        icon: AppIcons.assignmentOutlined,
        label: '選擇訂單',
        onTap: onPickOrder,
      ),
      AiFunctionMenuItem(
        icon: AppIcons.localFloristOutlined,
        label: '選擇上架商品',
        onTap: onPickProduct,
      ),
      AiFunctionMenuItem(
        icon: AppIcons.imageOutlined,
        label: '上傳照片',
        onTap: onPickPhoto,
      ),
    ],
  );
}

Future<void> openChatOrderPicker({
  required BuildContext context,
  required WidgetRef ref,
  required String? participantId,
  required ChatThreadArg arg,
  required ValueNotifier<bool> sending,
}) async {
  if (participantId == null || participantId.isEmpty) {
    FloatingToast.show(context, '無法取得買家資訊');
    return;
  }

  final pickerController = ref.read(chatPickerControllerProvider.notifier);
  FloatingToast.showCarLoading(context, '載入訂單中...');
  try {
    final options = await pickerController.loadOrders(participantId);
    FloatingToast.dismiss();
    if (!context.mounted) return;

    final selected = await showOrderPickerSheet(
      context: context,
      orders: options
          .map((option) => option.pickerData)
          .toList(growable: false),
    );
    if (selected == null || !context.mounted) return;

    final option = options.firstWhere(
      (candidate) => identical(candidate.pickerData, selected),
      orElse: () => options.firstWhere(
        (candidate) =>
            candidate.orderNo == (selected['orderNo'] ?? '').toString(),
      ),
    );
    sending.value = true;
    try {
      await ref
          .read(chatThreadProvider(arg).notifier)
          .send(
            content: option.toMessageContent(),
            messageType: 'order_prev',
            extra: {'preview': cardLabelForType('order_prev')!},
          );
    } catch (_) {
      if (context.mounted) {
        FloatingToast.error(context, '發送訂單卡片失敗');
      }
    } finally {
      sending.value = false;
    }
  } catch (error) {
    FloatingToast.dismiss();
    if (context.mounted) {
      FloatingToast.error(context, '載入訂單失敗：${chatPickerErrorMessage(error)}');
    }
  } finally {
    pickerController.reset();
  }
}

Future<void> openChatProductPicker({
  required BuildContext context,
  required WidgetRef ref,
  required String? participantId,
  required ChatThreadArg arg,
  required ValueNotifier<bool> sending,
}) async {
  if (participantId == null || participantId.isEmpty) {
    FloatingToast.show(context, '無法取得對象資訊');
    return;
  }

  final pickerController = ref.read(chatPickerControllerProvider.notifier);
  FloatingToast.showCarLoading(context, '載入商品中...');
  try {
    final options = await pickerController.loadProducts();
    FloatingToast.dismiss();
    if (!context.mounted) return;

    final selected = await showProductPickerSheet(
      context: context,
      products: options
          .map((option) => option.pickerData)
          .toList(growable: false),
    );
    if (selected == null || !context.mounted) return;

    final option = options.firstWhere(
      (candidate) => identical(candidate.pickerData, selected),
      orElse: () => options.firstWhere(
        (candidate) =>
            (candidate.pickerData['id'] ??
                    candidate.pickerData['productId'] ??
                    candidate.pickerData['skuId'])
                ?.toString() ==
            (selected['id'] ?? selected['productId'] ?? selected['skuId'])
                ?.toString(),
      ),
    );
    sending.value = true;
    try {
      await ref
          .read(chatThreadProvider(arg).notifier)
          .send(
            content: option.toMessageContent(),
            messageType: 'product',
            extra: {'preview': cardLabelForType('product')!},
          );
    } catch (_) {
      if (context.mounted) {
        FloatingToast.error(context, '發送商品卡片失敗');
      }
    } finally {
      sending.value = false;
    }
  } catch (error) {
    FloatingToast.dismiss();
    if (context.mounted) {
      FloatingToast.error(context, '載入商品失敗：${chatPickerErrorMessage(error)}');
    }
  } finally {
    pickerController.reset();
  }
}
