import 'package:hawkr_api/gen/hawkr/notification/v1alpha1/notification.pbenum.dart';

// 暫時的占位：分類與渠道的顯示文字改由共用文案目錄 `hawkr_ui_text` 提供（另一個分支新增），
// 合併後把這兩個函式換成生成的名稱，這個檔案即刪除。App 不自己寫這些文字。
String notificationCategoryLabel(NotificationCategory category) =>
    category.name;

String notificationChannelLabel(NotificationChannel channel) => channel.name;
