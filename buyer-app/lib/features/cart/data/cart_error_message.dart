import '../../../core/exceptions/app_exception.dart';

const cannotAddOwnProductToCartMessage = '不可以加入自己的商品';

String addToCartErrorMessage(
  Object error, {
  String fallback = '加入購物車失敗，請稍後再試',
}) {
  if (error is AppException && error.message.isNotEmpty) {
    return error.message;
  }
  return fallback;
}
