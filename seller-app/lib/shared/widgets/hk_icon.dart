import 'package:flutter/material.dart';

import '../icons/app_icon_colors.dart';
import '../icons/app_icons.dart';

/// Semantic icon names backed by the app-wide [AppIcons] registry.
enum HkIconName {
  home,
  market,
  wallet,
  box,
  settings,
  textSize,
  chart,
  goods,
  edit,
  users,
  search,
  back,
  close,
  filter,
  cart,
  chat,
  notification,
  clock,
  warning,
  check,
  document,
  post,
  truck,
  info,
  copy,
  cloudUpload,
  download,
  refresh,
  star,
  award,
  user,
  chevronDown,
  chevronRight,
  payment,
  receipt,
  returnOrder,
  store,
  location,
  qrCode,
  inventory,
  coupon,
  tag,
  add,
  remove,
  phone,
  support,
  history,
  externalLink,
  photoAdd,
  calendar,
  swap,
  more,
  shield,
  report,
  block,
  logout,
}

/// Semantic Material icon that inherits [IconTheme] size and color.
///
/// [selected] draws the filled glyph for names that have one, so selection is
/// readable from shape and not only from color.
class HkIcon extends StatelessWidget {
  const HkIcon(
    this.name, {
    super.key,
    this.size,
    this.color,
    this.selected = false,
    this.semanticLabel,
  });

  final HkIconName name;
  final double? size;
  final Color? color;
  final bool selected;
  final String? semanticLabel;

  @override
  Widget build(BuildContext context) {
    final icon = Icon(
      selected ? _selectedIconData(name) : _iconData(name),
      size: size ?? IconTheme.of(context).size ?? 24,
      color: color ?? IconTheme.of(context).color ?? AppIconColors.defaultColor,
      semanticLabel: semanticLabel,
      textDirection: Directionality.maybeOf(context),
    );
    if (semanticLabel?.trim().isNotEmpty ?? false) return icon;
    return ExcludeSemantics(child: icon);
  }
}

IconData _selectedIconData(HkIconName name) => switch (name) {
  HkIconName.home || HkIconName.store => AppIcons.store,
  HkIconName.market => AppIcons.category,
  HkIconName.wallet => AppIcons.accountBalanceWalletRounded,
  HkIconName.box => AppIcons.shoppingBag,
  HkIconName.user => AppIcons.person,
  HkIconName.settings => AppIcons.settings,
  HkIconName.chat => AppIcons.chatBubbleRounded,
  HkIconName.notification => AppIcons.notificationsActive,
  HkIconName.star => AppIcons.star,
  HkIconName.users => AppIcons.people,
  _ => _iconData(name),
};

IconData _iconData(HkIconName name) => switch (name) {
  HkIconName.home => AppIcons.storefrontOutlined,
  HkIconName.market => AppIcons.categoryOutlined,
  HkIconName.wallet => AppIcons.wallet,
  HkIconName.box => AppIcons.shoppingBagOutlined,
  HkIconName.settings => AppIcons.setting,
  HkIconName.textSize => AppIcons.textFields,
  HkIconName.chart => AppIcons.chartOutlined,
  HkIconName.goods => AppIcons.inventory2Outlined,
  HkIconName.edit => AppIcons.editSquareOutlined,
  HkIconName.users => AppIcons.peopleOutline,
  HkIconName.search => AppIcons.search,
  HkIconName.back => AppIcons.arrowBack,
  HkIconName.close => AppIcons.close,
  HkIconName.filter => AppIcons.tune,
  HkIconName.cart => AppIcons.shoppingCartOutlined,
  HkIconName.chat => AppIcons.chatBubbleOutline,
  HkIconName.notification => AppIcons.notificationsOutlined,
  HkIconName.clock => AppIcons.accessTimeOutlined,
  HkIconName.warning => AppIcons.warningAmberOutlined,
  HkIconName.check => AppIcons.checkCircleOutline,
  HkIconName.document => AppIcons.descriptionOutlined,
  HkIconName.post => AppIcons.articleOutlined,
  HkIconName.truck => AppIcons.localShippingOutlined,
  HkIconName.info => AppIcons.infoOutline,
  HkIconName.copy => AppIcons.contentCopy,
  HkIconName.cloudUpload => AppIcons.cloudUpload,
  HkIconName.download => AppIcons.download,
  HkIconName.refresh => AppIcons.refresh,
  HkIconName.star => AppIcons.starOutline,
  HkIconName.award => AppIcons.workspacePremiumOutlined,
  HkIconName.user => AppIcons.personOutline,
  HkIconName.chevronDown => AppIcons.keyboardArrowDown,
  HkIconName.chevronRight => AppIcons.chevronRight,
  HkIconName.payment => AppIcons.paymentOutlined,
  HkIconName.receipt => AppIcons.receiptOutlined,
  HkIconName.returnOrder => AppIcons.assignmentReturnOutlined,
  HkIconName.store => AppIcons.storefrontOutlined,
  HkIconName.location => AppIcons.location,
  HkIconName.qrCode => AppIcons.qrCode,
  HkIconName.inventory => AppIcons.inventory2Outlined,
  HkIconName.coupon => AppIcons.couponOutlined,
  HkIconName.tag => AppIcons.localOfferOutlined,
  HkIconName.add => AppIcons.add,
  HkIconName.remove => AppIcons.remove,
  HkIconName.phone => AppIcons.phoneOutlined,
  HkIconName.support => AppIcons.headsetMicOutlined,
  HkIconName.history => AppIcons.historyToggleOffOutlined,
  HkIconName.externalLink => AppIcons.openInNewOutlined,
  HkIconName.photoAdd => AppIcons.addPhotoAlternateOutlined,
  HkIconName.calendar => AppIcons.calendarTodayOutlined,
  HkIconName.swap => AppIcons.sync,
  HkIconName.more => AppIcons.moreHoriz,
  HkIconName.shield => AppIcons.shieldOutlined,
  HkIconName.report => AppIcons.reportOutlined,
  HkIconName.block => AppIcons.blockOutlined,
  HkIconName.logout => AppIcons.logout,
};
