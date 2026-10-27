import 'dart:collection';
import 'dart:convert';

/// Immutable order option used by the chat attachment picker.
final class ChatOrderPickerOption {
  ChatOrderPickerOption(Map<String, dynamic> source)
    : pickerData = UnmodifiableMapView<String, dynamic>(
        Map<String, dynamic>.of(source),
      );

  final UnmodifiableMapView<String, dynamic> pickerData;

  String get orderNo => (pickerData['orderNo'] ?? '').toString();

  String toMessageContent() {
    return jsonEncode(<String, dynamic>{
      'messageType': 'order_prev',
      'orderNo': orderNo,
      ...pickerData,
    });
  }
}

/// Immutable product option used by the chat attachment picker.
final class ChatProductPickerOption {
  ChatProductPickerOption(Map<String, dynamic> source)
    : pickerData = UnmodifiableMapView<String, dynamic>(
        Map<String, dynamic>.of(source),
      );

  final UnmodifiableMapView<String, dynamic> pickerData;

  String toMessageContent() {
    return jsonEncode(<String, dynamic>{
      'messageType': 'product',
      'productName':
          pickerData['productName'] ??
          pickerData['name'] ??
          pickerData['skuName'] ??
          '',
      'price':
          pickerData['price'] ??
          pickerData['unitPrice'] ??
          pickerData['priceMin'] ??
          '',
      'imageUrl': pickerData['imageUrl'] ?? '',
      'skuId': pickerData['skuId'] ?? pickerData['id'] ?? '',
      'productId': pickerData['productId'] ?? pickerData['id'] ?? '',
    });
  }
}
