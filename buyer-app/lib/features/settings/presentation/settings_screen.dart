import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/app_typography.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/core/layout/app_text_size.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:hawkr_buyer/shared/widgets/hk_button_label.dart';
import 'package:hawkr_buyer/shared/widgets/hk_tap_target.dart';
import 'package:hawkr_buyer/shared/widgets/text_size_settings_sheet.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:url_launcher/url_launcher.dart';
import '../../../shared/utils/app_version_utils.dart';
import '../../../core/session/session_model.dart';
import '../../../core/session/session_provider.dart';
import '../../../config/image_config.dart';
import '../../../shared/widgets/common_dialog.dart';
import '../../../shared/widgets/common_snackbar.dart';
import '../../../shared/widgets/common_top_bar.dart';
import '../../../shared/widgets/main_top_bar.dart';
import '../../../shared/widgets/oss_image.dart';
import '../../../shared/widgets/top_bar_provider.dart';
import '../../auth/data/auth_repository.dart';
import 'package:hawkr_buyer/shared/widgets/hk_placeholder.dart';

class SettingsScreen extends ConsumerStatefulWidget {
  const SettingsScreen({
    super.key,
    this.showBottomNav = true,
    this.showTopBar = true,
  });
  final bool showBottomNav;
  final bool showTopBar;
  @override
  ConsumerState<SettingsScreen> createState() => _SettingsScreenState();
}

class _SettingsScreenState extends ConsumerState<SettingsScreen>
    with TopBarProvider {
  late final Future<String> _versionFuture;

  @override
  void initState() {
    super.initState();
    _versionFuture = _fetchAppVersion();
  }

  Future<String> _fetchAppVersion() => fetchAppVersion();

  @override
  Widget build(BuildContext context) {
    final sessionAsync = ref.watch(sessionProvider);
    final session = sessionAsync.value;
    final textSize = ref.watch(appTextSizeProvider);

    return Scaffold(
      backgroundColor: AppColors.bgPrimary,
      appBar: widget.showTopBar ? buildMainTopBar(context) : null,
      body: SafeArea(
        top: !widget.showTopBar,
        child: ListView(
          padding: const EdgeInsets.only(top: 16, bottom: 16),
          children: [
            // 用戶頭像和名稱區塊
            _buildUserSection(session),
            SizedBox(height: 16.h),
            // 三個標籤頁 (追蹤中、收藏、瀏覽過)
            _buildTabsSection(),
            SizedBox(height: 16.h),
            // 選單項目列表
            _buildMenuList(textSize),
            SizedBox(height: 24.h),
            // 登出 / 登入按鈕（依登入狀態切換）
            _buildLogoutButton(session),
            SizedBox(height: 10.h),
            // 版本號
            _buildVersionNumber(),
          ],
        ),
      ),
    );
  }

  Widget _buildUserSection(dynamic session) {
    final userName = session?.nickname ?? 'User';

    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 24),
      child: Column(
        children: [
          // 用戶頭像 + 編輯按鈕
          SizedBox(
            key: const ValueKey('settings-avatar-stage'),
            width: 100 + HkSpace.sm,
            height: 100 + HkSpace.sm,
            child: Stack(
              children: [
                Align(
                  alignment: Alignment.topCenter,
                  child: KeyedSubtree(
                    key: const ValueKey('settings-avatar-image'),
                    child: _buildAvatarWidget(session),
                  ),
                ),
                // 編輯按鈕（筆圖標）
                Positioned(
                  right: 0,
                  bottom: 0,
                  child: HkTapTarget(
                    key: const ValueKey('settings-avatar-edit-target'),
                    semanticLabel: '編輯個人資料',
                    onTap: () => context.push('/settings/profile'),
                    child: Container(
                      key: const ValueKey('settings-avatar-edit-badge'),
                      width: HkSpace.section,
                      height: HkSpace.section,
                      decoration: BoxDecoration(
                        color: AppColors.primary,
                        shape: BoxShape.circle,
                        border: Border.all(color: AppColors.bgCard, width: 2),
                      ),
                      child: const HkIcon(
                        HkIconName.edit,
                        size: HkSpace.lg,
                        color: AppIconColors.onBrand,
                      ),
                    ),
                  ),
                ),
              ],
            ),
          ),
          SizedBox(height: 8.h),
          // 用戶名稱
          Text(
            userName,
            style: TextStyle(
              fontSize: 18,
              fontWeight: FontWeight.w700,
              color: AppColors.textPrimary,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildDefaultAvatar() {
    return Container(
      width: 100,
      height: 100,
      decoration: BoxDecoration(
        shape: BoxShape.circle,
        border: Border.all(color: AppColors.dividerLight, width: 2),
      ),
      clipBehavior: Clip.antiAlias,
      child: const HkPlaceholder(HkPlaceholderKind.buyerAvatar),
    );
  }

  Widget _buildAvatarWidget(dynamic session) {
    final sessionModel = session is SessionModel ? session : null;
    if (!hasUsableSession(sessionModel)) return _buildDefaultAvatar();
    final userId = sessionModel!.userId;
    // 同一使用者只取一次；切帳號後重取
    if (_avatarFuture == null || _avatarFutureUserId != userId) {
      _avatarFutureUserId = userId;
      _avatarFuture = _loadAvatarUrl(userId);
    }
    return FutureBuilder<String?>(
      future: _avatarFuture,
      builder: (ctx, snap) {
        final s = (snap.data ?? '').trim();
        // [_loadAvatarUrl] 只會回 `GET /app/buyers/me` 的 `avatarUrl`，那是
        // Gateway 用 MEDIA_BASE_URL 組好的顯示 URL（外部 Google 頭像
        // passthrough，本身也是 http），沒有 key 時整個欄位是空字串而不是裸
        // objectKey。所以這裡只需要處理 http(s) 一種形態。
        //
        // 原本還有兩條分支：前綴白名單（`/` `member` `avatars/` `public/`）
        // 走 normalizeAvatarUrl，以及 else 把字串當 base64 解。兩條都不可達
        // ——而 base64 那條特別危險，它是「什麼都不是就當圖片資料解」，
        // 任何非預期字串都會走進去然後被 catch 吞掉，變成靜默的預設頭像。
        if (!s.startsWith('http://') && !s.startsWith('https://')) {
          return _buildDefaultAvatar();
        }
        try {
          final ImageProvider img =
              getOssSceneImageProvider(s, ImageScene.avatarLarge) ??
              NetworkImage(s);
          return Container(
            width: 100,
            height: 100,
            decoration: BoxDecoration(
              shape: BoxShape.circle,
              border: Border.all(color: AppColors.dividerLight, width: 2),
              image: DecorationImage(image: img, fit: BoxFit.cover),
            ),
          );
        } catch (_) {
          return _buildDefaultAvatar();
        }
      },
    );
  }

  Future<String?>? _avatarFuture;
  String? _avatarFutureUserId;

  Future<String?> _loadAvatarUrl(String userId) async {
    final info = await ref.read(authRepositoryProvider).tryGetProfile();
    if (info == null) return null;
    final v = info.avatarUrl.trim();
    return v.isEmpty ? null : v;
  }

  Widget _buildTabsSection() {
    return Container(
      margin: const EdgeInsets.symmetric(horizontal: 24),
      padding: const EdgeInsets.symmetric(vertical: 16, horizontal: 20),
      decoration: BoxDecoration(
        color: AppColors.bgCard,
        borderRadius: BorderRadius.circular(16),
        border: Border.all(color: AppColors.cardBorder),
        boxShadow: [
          BoxShadow(
            color: AppColors.cardShadow,
            blurRadius: 8,
            offset: const Offset(0, 4),
          ),
        ],
      ),
      child: Row(
        children: [
          _buildStatsAction(
            icon: HkIconName.settingsFollowing,
            label: '追蹤中',
            onTap: () => context.push(
              '/settings/subscription',
              extra: {'initialTab': 0},
            ),
          ),
          const SizedBox(
            height: 40,
            child: VerticalDivider(
              width: 1,
              thickness: 1,
              color: AppColors.dividerLight,
            ),
          ),
          _buildStatsAction(
            icon: HkIconName.settingsFavorite,
            label: '收藏',
            onTap: () => context.push(
              '/settings/subscription',
              extra: {'initialTab': 1},
            ),
          ),
          const SizedBox(
            height: 40,
            child: VerticalDivider(
              width: 1,
              thickness: 1,
              color: AppColors.dividerLight,
            ),
          ),
          _buildStatsAction(
            icon: HkIconName.settingsViewed,
            label: '瀏覽過',
            onTap: () => context.push(
              '/settings/subscription',
              extra: {'initialTab': 2},
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildStatsAction({
    required HkIconName icon,
    required String label,
    required VoidCallback onTap,
  }) {
    return Expanded(
      child: Semantics(
        button: true,
        label: label,
        child: ExcludeSemantics(
          child: InkWell(
            borderRadius: BorderRadius.circular(12),
            onTap: onTap,
            child: Padding(
              padding: const EdgeInsets.symmetric(vertical: 4),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  SizedBox.square(
                    dimension: 30,
                    child: Stack(
                      clipBehavior: Clip.none,
                      children: [
                        Positioned.fill(
                          child: HkIcon(
                            icon,
                            size: 30,
                            color: AppIconColors.secondary,
                          ),
                        ),
                        if (icon == HkIconName.settingsFollowing)
                          const Positioned(
                            top: -1,
                            right: -1,
                            child: Icon(
                              AppIcons.add,
                              size: 10,
                              color: AppIconColors.secondary,
                            ),
                          ),
                      ],
                    ),
                  ),
                  const SizedBox(height: 6),
                  Text(
                    label,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(
                      fontSize: 16,
                      fontWeight: FontWeight.w500,
                      color: AppColors.textPrimary,
                    ),
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildMenuList(AppTextSize textSize) {
    final menuItems =
        <({HkIconName icon, String label, String? value, VoidCallback onTap})>[
          (
            icon: HkIconName.user,
            label: '個人資料設定',
            value: null,
            onTap: _onProfileTap,
          ),
          (
            icon: HkIconName.location,
            label: '收貨地址管理',
            value: null,
            onTap: _onAddressTap,
          ),
          (
            icon: HkIconName.textSize,
            label: '字體大小',
            value: textSize.label,
            onTap: () => showTextSizeSettingsSheet(context),
          ),
          (
            icon: HkIconName.afterSales,
            label: '售後服務',
            value: null,
            onTap: _onAfterSalesTap,
          ),
          (
            icon: HkIconName.notification,
            label: '通知設定',
            value: null,
            onTap: _onNotificationTap,
          ),
          (
            icon: HkIconName.warning,
            label: '我的檢舉',
            value: null,
            onTap: _onMyReportsTap,
          ),
          (
            icon: HkIconName.users,
            label: '封鎖名單',
            value: null,
            onTap: _onBlockedUsersTap,
          ),
          (
            icon: HkIconName.help,
            label: '幫助中心',
            value: null,
            onTap: _onHelpTap,
          ),
        ];

    return Container(
      margin: const EdgeInsets.symmetric(horizontal: 24),
      decoration: BoxDecoration(
        color: AppColors.bgCard,
        borderRadius: BorderRadius.circular(16),
        border: Border.all(color: AppColors.cardBorder),
        boxShadow: [
          BoxShadow(
            color: AppColors.cardShadow,
            blurRadius: 12,
            offset: const Offset(0, 4),
          ),
        ],
      ),
      child: Column(
        children: List.generate(menuItems.length, (index) {
          final item = menuItems[index];
          final isLast = index == menuItems.length - 1;
          return _buildMenuItem(
            icon: item.icon,
            label: item.label,
            value: item.value,
            onTap: item.onTap,
            showDivider: !isLast,
          );
        }),
      ),
    );
  }

  Widget _buildMenuItem({
    required HkIconName icon,
    required String label,
    required VoidCallback onTap,
    String? value,
    bool showDivider = true,
    int badge = 0,
  }) {
    return Column(
      children: [
        InkWell(
          onTap: onTap,
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 16),
            child: Row(
              children: [
                HkIcon(icon, size: 24, color: AppIconColors.secondary),
                SizedBox(width: 13.w),
                Expanded(
                  child: Text(
                    label,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: AppTypography.body.copyWith(
                      fontWeight: FontWeight.w500,
                    ),
                  ),
                ),
                if (value != null) ...[
                  const SizedBox(width: HkSpace.sm),
                  Text(
                    value,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: AppTypography.metaMuted,
                  ),
                ],
                if (badge > 0) ...[
                  Container(
                    padding: EdgeInsets.symmetric(
                      horizontal: 8.w,
                      vertical: 4.h,
                    ),
                    decoration: BoxDecoration(
                      color: AppColors.danger,
                      borderRadius: BorderRadius.circular(12.r),
                    ),
                    child: Text(
                      '$badge',
                      style: TextStyle(
                        fontSize: 12,
                        fontWeight: FontWeight.w700,
                        color: AppColors.bgCard,
                      ),
                    ),
                  ),
                  SizedBox(width: 8.w),
                ],
                const HkIcon(
                  HkIconName.chevronRight,
                  size: 20,
                  color: AppIconColors.muted,
                ),
              ],
            ),
          ),
        ),
        if (showDivider)
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: 16),
            child: Divider(height: 1, thickness: 1, color: AppColors.gray200),
          ),
      ],
    );
  }

  Widget _buildLogoutButton(dynamic session) {
    final isLoggedIn = hasUsableSession(session);
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 24),
      child: isLoggedIn
          ? OutlinedButton(
              onPressed: _confirmAndLogout,
              style: OutlinedButton.styleFrom(
                minimumSize: const Size(double.infinity, HkControl.minTap),
                foregroundColor: AppColors.danger,
                side: const BorderSide(color: AppColors.danger),
              ),
              child: const HkButtonLabel('登出'),
            )
          : FilledButton(
              onPressed: () {
                final router = GoRouter.of(context);
                final loc = router.routerDelegate.currentConfiguration.uri
                    .toString();
                router.push('/login?continue=${Uri.encodeComponent(loc)}');
              },
              style: FilledButton.styleFrom(
                minimumSize: const Size(double.infinity, HkControl.minTap),
              ),
              child: const HkButtonLabel('登入'),
            ),
    );
  }

  Widget _buildVersionNumber() {
    return FutureBuilder<String>(
      future: _versionFuture,
      builder: (context, snapshot) {
        final version = snapshot.data?.trim() ?? '';
        if (version.isEmpty) return const SizedBox.shrink();
        return Padding(
          padding: const EdgeInsets.symmetric(horizontal: 24),
          child: Align(
            alignment: Alignment.centerRight,
            child: Text(
              'v.$version',
              style: TextStyle(color: AppColors.textSecondary, fontSize: 12),
            ),
          ),
        );
      },
    );
  }

  // 選單點擊事件
  void _onProfileTap() => context.push('/settings/profile');

  void _onAddressTap() => context.push('/settings/address');

  void _onAfterSalesTap() {
    context.push('/settings/after-sale');
  }

  void _onNotificationTap() {
    context.push('/settings/notifications');
  }

  void _onHelpTap() {
    context.push('/settings/help');
  }

  void _onMyReportsTap() {
    context.push('/settings/reports');
  }

  void _onBlockedUsersTap() {
    context.push('/settings/blocks');
  }

  Future<void> _confirmAndLogout() async {
    final confirmed = await showCommonDialog(
      context: context,
      title: '確認登出',
      content: '確定要登出當前帳號嗎？',
      confirmText: '登出',
      cancelText: '取消',
      confirmColor: AppColors.danger,
      barrierDismissible: true,
    );
    if (confirmed != true) return;

    await ref.read(sessionProvider.notifier).logout();
    if (!mounted) return;
    context.go('/');
  }

  @override
  PreferredSizeWidget buildTopBar(BuildContext context) {
    return buildMainTopBar(context);
  }
}

/// 客服 LINE 官方帳號：`--dart-define=HELP_CENTER_LINE_ID=...` 與
/// `--dart-define=HELP_CENTER_LINE_URL=...`；沒有提供時幫助中心不顯示 LINE 入口。
const helpCenterLineId = String.fromEnvironment('HELP_CENTER_LINE_ID');
const helpCenterLineUrl = String.fromEnvironment('HELP_CENTER_LINE_URL');
const helpCenterEmail = 'support@example.com';

// 幫助中心獨立頁面
class HelpCenterScreen extends StatefulWidget {
  const HelpCenterScreen({
    super.key,
    this.lineId = helpCenterLineId,
    this.lineUrl = helpCenterLineUrl,
  });

  /// LINE 官方帳號顯示用 ID；空字串時不顯示 LINE 帳號列。
  final String lineId;

  /// LINE 官方帳號連結；空字串時不顯示 LINE 帳號列與聯絡按鈕。
  final String lineUrl;

  @override
  State<HelpCenterScreen> createState() => _HelpCenterScreenState();
}

class _HelpCenterScreenState extends State<HelpCenterScreen> {
  static const List<Map<String, String>> _faqList = [
    {'question': '如何取消訂單？', 'answer': '訂單成立後請直接與賣家聯繫協助取消，買家無法自行取消訂單。'},
    {
      'question': '退款需要多久時間？',
      'answer': '退款通常會在3-7個工作日內退回您的付款帳戶，具體時間視您的付款方式而定。',
    },
    {'question': '運費如何計算？', 'answer': '運費根據您的收貨地址和訂單金額計算。滿額免運，詳情請參考結帳頁面。'},
  ];
  late final List<bool> _expandedList = List.filled(_faqList.length, false);

  void _copyValue(String value, String message) {
    Clipboard.setData(ClipboardData(text: value));
    showFloatingToast(context, message, icon: AppIcons.contentCopy);
  }

  Future<void> _openExternal(Uri uri, String errorMessage) async {
    try {
      final opened = await launchUrl(uri, mode: LaunchMode.externalApplication);
      if (!opened && mounted) {
        showFloatingToast(context, errorMessage, isSuccess: false);
      }
    } catch (_) {
      if (mounted) {
        showFloatingToast(context, errorMessage, isSuccess: false);
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: AppColors.bgPrimary,
      appBar: buildCommonTitleAppBar(
        context,
        title: '幫助中心',
        onBack: () => Navigator.maybePop(context),
      ),
      body: LayoutBuilder(
        builder: (context, constraints) {
          final pageGutter = HkBreakpoints.pageGutter(constraints.maxWidth);
          return SingleChildScrollView(
            padding: EdgeInsets.fromLTRB(
              pageGutter,
              HkSpace.xxl,
              pageGutter,
              HkSpace.large,
            ),
            child: Center(
              child: ConstrainedBox(
                constraints: const BoxConstraints(
                  maxWidth: HkControl.chromeMaxWidth,
                ),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text('常見問題', style: AppTypography.sectionTitle),
                    const SizedBox(height: HkSpace.md),
                    _buildSurfaceCard(
                      child: Column(
                        children: List.generate(_faqList.length, (index) {
                          final faq = _faqList[index];
                          return _buildFaqItem(
                            question: faq['question']!,
                            answer: faq['answer']!,
                            isExpanded: _expandedList[index],
                            showDivider: index != _faqList.length - 1,
                            onTap: () {
                              setState(() {
                                _expandedList[index] = !_expandedList[index];
                              });
                            },
                          );
                        }),
                      ),
                    ),
                    const SizedBox(height: HkSpace.section),
                    Text('聯絡我們', style: AppTypography.sectionTitle),
                    const SizedBox(height: HkSpace.md),
                    _buildContactCard(),
                  ],
                ),
              ),
            ),
          );
        },
      ),
    );
  }

  Widget _buildSurfaceCard({required Widget child}) {
    return Container(
      decoration: BoxDecoration(
        color: AppColors.bgCard,
        borderRadius: BorderRadius.circular(HkRadius.lg),
        border: Border.all(color: AppColors.cardBorder),
        boxShadow: HkShadows.card,
      ),
      child: ClipRRect(
        borderRadius: BorderRadius.circular(HkRadius.lg),
        child: Material(color: Colors.transparent, child: child),
      ),
    );
  }

  Widget _buildContactCard() {
    return _buildSurfaceCard(
      child: Padding(
        padding: const EdgeInsets.all(HkSpace.lg),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('如果有其他問題，歡迎透過以下方式聯繫客服。', style: AppTypography.metaMuted),
            const SizedBox(height: HkSpace.xs),
            Text('服務時間：週一至週五 09:00–18:00', style: AppTypography.caption),
            const SizedBox(height: HkSpace.lg),
            if (widget.lineId.isNotEmpty && widget.lineUrl.isNotEmpty) ...[
              _buildContactRow(
                key: const ValueKey('help-line-contact'),
                label: 'LINE 官方帳號',
                value: widget.lineId,
                leading: const Icon(
                  AppIcons.chatBubbleOutline,
                  size: HkControl.icon,
                  color: AppIconColors.active,
                ),
                onTap: () =>
                    _openExternal(Uri.parse(widget.lineUrl), '無法開啟 LINE，請稍後再試'),
                onCopy: () => _copyValue(widget.lineId, 'LINE ID 已複製'),
                copyTooltip: '複製 LINE ID',
              ),
              const SizedBox(height: HkSpace.sm),
            ],
            _buildContactRow(
              key: const ValueKey('help-email-contact'),
              label: '客服信箱',
              value: helpCenterEmail,
              leading: const Icon(
                AppIcons.emailOutlined,
                size: HkControl.icon,
                color: AppIconColors.active,
              ),
              onTap: () => _openExternal(
                Uri(scheme: 'mailto', path: helpCenterEmail),
                '無法開啟郵件 App，請稍後再試',
              ),
              onCopy: () => _copyValue(helpCenterEmail, '客服信箱已複製'),
              copyTooltip: '複製客服信箱',
            ),
            if (widget.lineUrl.isNotEmpty) ...[
              const SizedBox(height: HkSpace.lg),
              SizedBox(
                width: double.infinity,
                height: HkControl.minTap,
                child: ElevatedButton.icon(
                  key: const ValueKey('help-line-cta'),
                  onPressed: () => _openExternal(
                    Uri.parse(widget.lineUrl),
                    '無法開啟 LINE，請稍後再試',
                  ),
                  style: ElevatedButton.styleFrom(
                    elevation: 0,
                    backgroundColor: AppColors.actionPrimary,
                    foregroundColor: AppColors.textInverse,
                    shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(HkRadius.md),
                    ),
                  ),
                  icon: const HkIcon(
                    HkIconName.externalLink,
                    size: 20,
                    color: AppIconColors.onBrand,
                  ),
                  label: HkButtonLabel(
                    '透過 LINE 聯絡客服',
                    style: AppTypography.buttonLarge,
                  ),
                ),
              ),
            ],
          ],
        ),
      ),
    );
  }

  Widget _buildContactRow({
    required Key key,
    required String label,
    required String value,
    required Widget leading,
    required VoidCallback onTap,
    required VoidCallback onCopy,
    required String copyTooltip,
  }) {
    return Material(
      key: key,
      color: AppColors.surfaceMuted,
      borderRadius: BorderRadius.circular(HkRadius.md),
      child: ConstrainedBox(
        constraints: const BoxConstraints(
          minHeight: HkControl.minTap + HkSpace.xxl,
        ),
        child: Row(
          children: [
            Expanded(
              child: InkWell(
                onTap: onTap,
                borderRadius: BorderRadius.circular(HkRadius.md),
                child: Padding(
                  padding: const EdgeInsets.fromLTRB(
                    HkSpace.md,
                    HkSpace.md,
                    HkSpace.sm,
                    HkSpace.md,
                  ),
                  child: Row(
                    children: [
                      Container(
                        width: HkSpace.large,
                        height: HkSpace.large,
                        alignment: Alignment.center,
                        decoration: const BoxDecoration(
                          color: AppColors.primarySoft,
                          shape: BoxShape.circle,
                        ),
                        child: leading,
                      ),
                      const SizedBox(width: HkSpace.md),
                      Expanded(
                        child: Column(
                          mainAxisSize: MainAxisSize.min,
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(label, style: AppTypography.caption),
                            const SizedBox(height: HkSpace.xs),
                            Text(
                              value,
                              key: ValueKey('help-contact-value-$value'),
                              maxLines: 1,
                              softWrap: false,
                              overflow: TextOverflow.ellipsis,
                              style: AppTypography.bodyBold,
                            ),
                          ],
                        ),
                      ),
                    ],
                  ),
                ),
              ),
            ),
            Tooltip(
              message: copyTooltip,
              child: IconButton(
                onPressed: onCopy,
                icon: const Icon(
                  AppIcons.copyOutlined,
                  size: HkSpace.xl,
                  color: AppIconColors.muted,
                ),
              ),
            ),
            const SizedBox(width: HkSpace.xs),
          ],
        ),
      ),
    );
  }

  Widget _buildFaqItem({
    required String question,
    required String answer,
    required bool isExpanded,
    required bool showDivider,
    required VoidCallback onTap,
  }) {
    return Column(
      children: [
        Semantics(
          button: true,
          expanded: isExpanded,
          child: InkWell(
            onTap: onTap,
            child: ConstrainedBox(
              constraints: const BoxConstraints(minHeight: 64),
              child: Padding(
                padding: const EdgeInsets.symmetric(horizontal: HkSpace.lg),
                child: Row(
                  children: [
                    Expanded(
                      child: Text(question, style: AppTypography.bodyBold),
                    ),
                    const SizedBox(width: HkSpace.sm),
                    AnimatedRotation(
                      turns: isExpanded ? 0.5 : 0,
                      duration: HkMotion.duration,
                      curve: HkMotion.ease,
                      child: const HkIcon(
                        HkIconName.chevronDown,
                        size: HkControl.icon,
                        color: AppIconColors.muted,
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ),
        ),
        AnimatedSize(
          duration: HkMotion.duration,
          curve: HkMotion.ease,
          child: isExpanded
              ? Container(
                  width: double.infinity,
                  padding: const EdgeInsets.fromLTRB(
                    HkSpace.lg,
                    0,
                    HkSpace.lg,
                    HkSpace.lg,
                  ),
                  child: Text(answer, style: AppTypography.metaMuted),
                )
              : const SizedBox.shrink(),
        ),
        if (showDivider)
          const Padding(
            padding: EdgeInsets.symmetric(horizontal: HkSpace.lg),
            child: Divider(
              height: 1,
              thickness: 1,
              color: AppColors.dividerLight,
            ),
          ),
      ],
    );
  }
}
