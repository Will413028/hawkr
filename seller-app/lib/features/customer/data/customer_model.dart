import 'package:hawkr_ui_text/hawkr_ui_text.dart';
import 'package:hawkr_api/gen/hawkr/crm/v1alpha1/crm.pb.dart' as pb;
import 'package:hawkr_api/gen/hawkr/crm/v1alpha1/crm.pbenum.dart';
import 'package:hawkr_api/images.dart';

/// 店家的標籤（`crm.Tag`）；[id] 是不透明字串。
class CustomerTag {
  const CustomerTag({required this.id, required this.name});

  factory CustomerTag.fromProto(pb.Tag t) =>
      CustomerTag(id: t.id, name: t.name);

  final String id;
  final String name;

  @override
  bool operator ==(Object other) =>
      other is CustomerTag && other.id == id && other.name == name;

  @override
  int get hashCode => Object.hash(id, name);
}

/// 店家的客戶（`crm.Customer`）：只有契約給的欄位，沒有電話、Email、地址。
class Customer {
  const Customer({
    required this.id,
    this.name = '',
    this.avatarUrl,
    this.deleted = false,
    this.type = CustomerType.CUSTOMER_TYPE_NORMAL,
    this.totalSpent = 0,
    this.orderCount = 0,
    this.firstOrderTime,
    this.tags = const [],
  });

  factory Customer.fromProto(pb.Customer c) {
    final display = c.display;
    return Customer(
      id: c.buyerId,
      name: display.hasNickname() ? display.nickname.trim() : '',
      avatarUrl: display.hasAvatar() && display.avatar.uriTemplate.isNotEmpty
          ? display.avatar.uri(ImagePreset.avatar)
          : null,
      deleted: display.deleted,
      type: c.type,
      totalSpent: c.totalSpent.toInt(),
      orderCount: c.orderCount,
      firstOrderTime: c.hasFirstOrderTime()
          ? c.firstOrderTime.toDateTime()
          : null,
      tags: c.tags.map(CustomerTag.fromProto).toList(),
    );
  }

  /// 買家 ID（`buyer_id`）。
  final String id;

  /// 暱稱；沒有設定或已刪除時是空字串，顯示用 [displayName]。
  final String name;
  final String? avatarUrl;
  final bool deleted;
  final CustomerType type;
  final int totalSpent;
  final int orderCount;
  final DateTime? firstOrderTime;
  final List<CustomerTag> tags;

  bool get isVip => type == CustomerType.CUSTOMER_TYPE_VIP;

  /// 顯示名稱：已刪除的買家用目錄的 `buyer.Profile.deleted_name`，沒有暱稱時用「買家」。
  String displayName(UiLocale l) => deleted
      ? buyerProfileDeletedName(l)
      : name.isNotEmpty
      ? name
      : '買家';

  Customer copyWith({CustomerType? type, List<CustomerTag>? tags}) => Customer(
    id: id,
    name: name,
    avatarUrl: avatarUrl,
    deleted: deleted,
    type: type ?? this.type,
    totalSpent: totalSpent,
    orderCount: orderCount,
    firstOrderTime: firstOrderTime,
    tags: tags ?? this.tags,
  );
}

enum OrderStatus { pendingPayment, delivering, completed, cancelled, issue }

/// 客戶詳情的訂單列（取自店家訂單列表，以 `buyer_id` 篩選）。
class OrderRecord {
  const OrderRecord({
    required this.orderId,
    required this.date,
    required this.items,
    required this.amount,
    this.status = OrderStatus.completed,
  });

  final String orderId;
  final String date;
  final String items;
  final int amount;
  final OrderStatus status;
}
