/// 買家已儲存的 7-11 取貨門市。
///
/// 後端 `/app/store_pickups` 的欄名一律 camelCase（storeCode / storeName /
/// storeAddress / receiverName / receiverPhone / isDefault）。
class StorePickup {
  const StorePickup({
    required this.id,
    required this.receiverName,
    required this.receiverPhone,
    required this.storeCode,
    required this.storeName,
    required this.storeAddress,
    required this.isDefault,
  });

  final String id;
  final String receiverName;
  final String receiverPhone;
  final String storeCode;
  final String storeName;
  final String storeAddress;
  final bool isDefault;

  factory StorePickup.fromJson(Map<String, dynamic> json) {
    String value(String key) => (json[key] ?? '').toString().trim();

    final rawDefault = json['isDefault'];
    final isDefault =
        rawDefault == true ||
        rawDefault == 1 ||
        rawDefault?.toString().toLowerCase() == 'true' ||
        rawDefault?.toString() == '1';

    return StorePickup(
      id: (json['id'] ?? '').toString(),
      receiverName: value('receiverName'),
      receiverPhone: value('receiverPhone'),
      storeCode: value('storeCode'),
      storeName: value('storeName'),
      storeAddress: value('storeAddress'),
      isDefault: isDefault,
    );
  }

  StorePickup copyWith({bool? isDefault}) => StorePickup(
    id: id,
    receiverName: receiverName,
    receiverPhone: receiverPhone,
    storeCode: storeCode,
    storeName: storeName,
    storeAddress: storeAddress,
    isDefault: isDefault ?? this.isDefault,
  );

  /// `StorePickupFormSheet` 與門市 API 共用的 camelCase payload。
  Map<String, dynamic> toJson() => {
    'id': id,
    'receiverName': receiverName,
    'receiverPhone': receiverPhone,
    'storeCode': storeCode,
    'storeName': storeName,
    'storeAddress': storeAddress,
    'isDefault': isDefault,
  };
}
