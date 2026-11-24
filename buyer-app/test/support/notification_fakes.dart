import 'package:hawkr_api/gen/hawkr/notification/v1alpha1/notification.pb.dart'
    as pb;

/// 伺服器回的收件匣一則：文字已組好，client 只顯示。
pb.InboxItem inboxItem(
  String id, {
  String title = '訂單提醒',
  String body = '商品已準備出貨',
  String presentation = 'INBOX_PRESENTATION_SYSTEM',
  String deepLink = '',
  bool read = false,
  String createTime = '2020-07-26T00:00:00Z',
}) => pb.InboxItem()
  ..mergeFromProto3Json({
    'id': id,
    'title': title,
    'body': body,
    'presentation': presentation,
    'deepLink': deepLink,
    'read': read,
    'createTime': createTime,
  });
