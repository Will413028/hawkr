import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:hawkr_seller/shared/widgets/hk_button_label.dart';
import 'package:hawkr_seller/shared/widgets/hk_glass_app_bar.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';
import 'package:url_launcher/url_launcher.dart';

/// 客服 LINE 官方帳號：`--dart-define=HELP_CENTER_LINE_ID=...` 與
/// `--dart-define=HELP_CENTER_LINE_URL=...`；沒有提供時幫助中心不顯示 LINE 入口。
const helpCenterLineId = String.fromEnvironment('HELP_CENTER_LINE_ID');
const helpCenterLineUrl = String.fromEnvironment('HELP_CENTER_LINE_URL');
const helpCenterEmail = 'support@example.com';

class HelpCenterScreen extends StatefulWidget {
  const HelpCenterScreen({
    super.key,
    this.lineId = helpCenterLineId,
    this.lineUrl = helpCenterLineUrl,
  });

  /// 客服 LINE ID；空字串時不顯示 LINE ID 列。
  final String lineId;

  /// 客服 LINE 連結；空字串時不顯示 LINE 入口。
  final String lineUrl;

  @override
  State<HelpCenterScreen> createState() => _HelpCenterScreenState();
}

class _HelpCenterScreenState extends State<HelpCenterScreen> {
  // FAQ 展開狀態
  final Map<int, bool> _expandedItems = {};

  // FAQ 資料
  final List<Map<String, String>> _faqList = [
    {
      'question': '如何上架商品？',
      'answer': '進入「商品管理」頁面，點擊右上角的「+」按鈕，選擇商品類型後填寫商品資訊即可完成上架。',
    },
    {
      'question': '如何修改商品價格？',
      'answer': '在「商品管理」頁面找到要修改的商品，點擊進入編輯頁面，修改價格後點擊「保存」即可。',
    },
    {
      'question': '訂單如何發貨？',
      'answer': '進入「訂單管理」頁面，找到待發貨的訂單，點擊「發貨」按鈕，填寫物流資訊後確認發貨。',
    },
    {'question': '如何提現？', 'answer': '進入「錢包」頁面，點擊「提現」按鈕，輸入提現金額並選擇提現方式，確認後等待審核。'},
    {'question': '如何聯繫買家？', 'answer': '在訂單詳情頁面點擊「聯繫買家」按鈕，即可與買家進行即時通訊。'},
  ];

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: AppColors.bgPrimary,
      appBar: _buildAppBar(),
      body: SingleChildScrollView(
        padding: EdgeInsets.fromLTRB(20.w, 20.h, 20.w, 40.h),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // 常見問題標題
            Text(
              '常見問題',
              style: TextStyle(
                fontSize: 18.sp,
                fontWeight: FontWeight.w700,
                color: AppColors.textEmphasis,
              ),
            ),
            SizedBox(height: 12.h),
            // FAQ 卡片
            _buildFaqCard(),
            SizedBox(height: 24.h),
            // 聯絡我們標題
            Text(
              '聯絡我們',
              style: TextStyle(
                fontSize: 18.sp,
                fontWeight: FontWeight.w700,
                color: AppColors.textEmphasis,
              ),
            ),
            SizedBox(height: 12.h),
            // 聯絡資訊卡片
            _buildContactCard(),
          ],
        ),
      ),
    );
  }

  PreferredSizeWidget _buildAppBar() {
    return HkGlassAppBar(
      title: const Text('幫助中心'),
      onBackPressed: () => context.pop(),
    );
  }

  Widget _buildFaqCard() {
    return Container(
      decoration: BoxDecoration(
        color: AppColors.bgCard,
        borderRadius: BorderRadius.circular(12.r),
        border: Border.all(color: AppColors.cardBorder),
        boxShadow: [
          BoxShadow(
            color: AppColors.cardShadow,
            blurRadius: 12,
            offset: Offset(0, 4.h),
          ),
        ],
      ),
      child: Column(
        children: List.generate(_faqList.length, (index) {
          final isLast = index == _faqList.length - 1;
          return Column(
            children: [
              _buildFaqItem(
                index: index,
                question: _faqList[index]['question']!,
                answer: _faqList[index]['answer']!,
              ),
              if (!isLast) _buildDivider(),
            ],
          );
        }),
      ),
    );
  }

  Widget _buildFaqItem({
    required int index,
    required String question,
    required String answer,
  }) {
    final isExpanded = _expandedItems[index] ?? false;

    return Column(
      children: [
        // 問題列（可點擊展開）
        InkWell(
          onTap: () {
            setState(() {
              _expandedItems[index] = !isExpanded;
            });
          },
          borderRadius: index == 0
              ? BorderRadius.only(
                  topLeft: Radius.circular(12.r),
                  topRight: Radius.circular(12.r),
                )
              : index == _faqList.length - 1 && !isExpanded
              ? BorderRadius.only(
                  bottomLeft: Radius.circular(12.r),
                  bottomRight: Radius.circular(12.r),
                )
              : null,
          child: Padding(
            padding: EdgeInsets.symmetric(horizontal: 16.w, vertical: 16.h),
            child: Row(
              children: [
                Expanded(
                  child: Text(
                    question,
                    style: TextStyle(
                      fontSize: 16.sp,
                      fontWeight: FontWeight.w500,
                      color: AppColors.textEmphasis,
                    ),
                  ),
                ),
                Icon(
                  isExpanded
                      ? AppIcons.keyboardArrowUp
                      : AppIcons.keyboardArrowDown,
                  size: 24.sp,
                  color: AppIconColors.muted,
                ),
              ],
            ),
          ),
        ),
        // 答案區（展開時顯示）
        if (isExpanded)
          Container(
            width: double.infinity,
            padding: EdgeInsets.fromLTRB(16.w, 0, 16.w, 16.h),
            child: Text(
              answer,
              style: TextStyle(
                fontSize: 14.sp,
                fontWeight: FontWeight.normal,
                color: AppColors.textSecondary,
                height: 1.5,
              ),
            ),
          ),
      ],
    );
  }

  Widget _buildContactCard() {
    return Container(
      padding: EdgeInsets.all(16.w),
      decoration: BoxDecoration(
        color: AppColors.bgCard,
        borderRadius: BorderRadius.circular(12.r),
        border: Border.all(color: AppColors.cardBorder),
        boxShadow: [
          BoxShadow(
            color: AppColors.cardShadow,
            blurRadius: 12,
            offset: Offset(0, 4.h),
          ),
        ],
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            '如果有其他問題，歡迎透過以下方式聯繫客服',
            style: TextStyle(fontSize: 14.sp, color: AppColors.textSecondary),
          ),
          SizedBox(height: 16.h),
          // LINE
          if (widget.lineId.isNotEmpty && widget.lineUrl.isNotEmpty) ...[
            KeyedSubtree(
              key: const ValueKey('help-line-contact'),
              child: _buildCopyableRow(
                icon: AppIcons.chatBubbleOutline,
                label: 'ID：',
                value: widget.lineId,
                copyMessage: 'LINE ID 已複製',
              ),
            ),
            SizedBox(height: 12.h),
          ],
          // Email
          _buildCopyableRow(
            icon: AppIcons.emailOutlined,
            label: '',
            value: helpCenterEmail,
            copyMessage: 'Email 已複製',
          ),
          if (widget.lineUrl.isNotEmpty) ...[
            SizedBox(height: 16.h),
            // 線上客服按鈕
            SizedBox(
              key: const ValueKey('help-line-cta'),
              width: double.infinity,
              child: ElevatedButton(
                onPressed: _openLineCustomerService,
                style: ElevatedButton.styleFrom(
                  backgroundColor: AppColors.textPrimary,
                  foregroundColor: AppColors.bgCard,
                  padding: EdgeInsets.symmetric(vertical: 16.h),
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(8.r),
                  ),
                  elevation: 0,
                ),
                child: HkButtonLabel(
                  '線上客服',
                  style: TextStyle(
                    fontSize: 16.sp,
                    fontWeight: FontWeight.w700,
                  ),
                ),
              ),
            ),
          ],
        ],
      ),
    );
  }

  Future<void> _openLineCustomerService() async {
    final launched = await launchUrl(
      Uri.parse(widget.lineUrl),
      mode: LaunchMode.externalApplication,
    );

    if (!mounted || launched) return;
    _showCopyToast('無法開啟 LINE，請稍後再試');
  }

  void _showCopyToast(String text) {
    final overlay = Overlay.of(context);
    late OverlayEntry overlayEntry;
    overlayEntry = OverlayEntry(
      builder: (context) => Positioned(
        bottom: 120.h,
        left: 0,
        right: 0,
        child: Center(
          child: Material(
            color: Colors.transparent,
            child: TweenAnimationBuilder<double>(
              tween: Tween(begin: 0.0, end: 1.0),
              duration: const Duration(milliseconds: 300),
              curve: Curves.easeOut,
              builder: (context, value, child) {
                return Opacity(
                  opacity: value,
                  child: Transform.translate(
                    offset: Offset(0, 20.h * (1 - value)),
                    child: child,
                  ),
                );
              },
              child: Container(
                constraints: BoxConstraints(maxWidth: 280.w),
                padding: EdgeInsets.symmetric(horizontal: 32.w, vertical: 20.h),
                decoration: BoxDecoration(
                  color: AppColors.bgCard,
                  borderRadius: BorderRadius.circular(24.r),
                  boxShadow: [
                    BoxShadow(
                      color: AppColors.cardShadow,
                      blurRadius: 24,
                      offset: Offset(0, 8.h),
                    ),
                  ],
                ),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Icon(
                      AppIcons.contentCopy,
                      size: 24.sp,
                      color: AppIconColors.secondary,
                    ),
                    SizedBox(height: 12.h),
                    Text(
                      text,
                      textAlign: TextAlign.center,
                      style: TextStyle(
                        fontSize: 16.sp,
                        fontWeight: FontWeight.w500,
                        color: AppColors.textEmphasis,
                        height: 1.4,
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ),
        ),
      ),
    );

    overlay.insert(overlayEntry);

    Future.delayed(const Duration(seconds: 2), () {
      if (overlayEntry.mounted) {
        overlayEntry.remove();
      }
    });
  }

  Widget _buildCopyableRow({
    required IconData icon,
    required String label,
    required String value,
    required String copyMessage,
  }) {
    return GestureDetector(
      onTap: () {
        Clipboard.setData(ClipboardData(text: value));
        _showCopyToast(copyMessage);
      },
      child: Container(
        padding: EdgeInsets.symmetric(horizontal: 12.w, vertical: 12.h),
        decoration: BoxDecoration(
          color: AppColors.bgPrimary,
          borderRadius: BorderRadius.circular(8.r),
        ),
        child: Row(
          children: [
            SizedBox.square(
              dimension: 36.w,
              child: Icon(icon, size: 28.w, color: AppIconColors.defaultColor),
            ),
            SizedBox(width: 12.w),
            Expanded(
              child: Text(
                '$label$value',
                style: TextStyle(fontSize: 14.sp, color: AppColors.textPrimary),
              ),
            ),
            Icon(AppIcons.copy, size: 18.sp, color: AppIconColors.muted),
          ],
        ),
      ),
    );
  }

  Widget _buildDivider() {
    return Padding(
      padding: EdgeInsets.symmetric(horizontal: 16.w),
      child: Divider(height: 1, color: AppColors.gray100),
    );
  }
}
