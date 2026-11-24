import 'notification_repository.dart';

// 分類與渠道的顯示文字：共用文案目錄（hawkr_ui_text）加入後在這裡改成查目錄；這個檔是唯一的接點。
String notificationCategoryLabel(NotificationCategory category) =>
    category.name;

String notificationChannelLabel(NotificationChannel channel) => channel.name;
