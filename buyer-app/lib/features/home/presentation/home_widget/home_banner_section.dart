import 'dart:async';

import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:hawkr_api/gen/hawkr/content/v1alpha1/content.pb.dart' as api;
import 'package:flutter/material.dart';

import '../../../../config/image_config.dart';
import '../../../../shared/widgets/app_cached_image.dart';
import '../../../../shared/widgets/hk_tap_target.dart';
import 'banner_popup_dialog.dart';

class HomeBannerSection extends StatefulWidget {
  const HomeBannerSection({super.key, required this.banners});

  /// ContentPublicService 的 ListLiveBanners，依顯示順序。
  final List<api.Banner> banners;

  @override
  State<HomeBannerSection> createState() => _HomeBannerSectionState();
}

class _HomeBannerSectionState extends State<HomeBannerSection> {
  late PageController _pager;
  double _vpFraction = 1.0;
  int _page = 0;
  Timer? _timer;

  @override
  void initState() {
    super.initState();
    _pager = PageController();
    _pager.addListener(_onPageChange);
    _startAutoPlay();
  }

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    final sw = MediaQuery.of(context).size.width;
    if (sw > 0) {
      const cardSpacing = HkSpace.sm;
      final horizontalPadding = HkBreakpoints.pageGutter(sw);
      final vp = ((sw - horizontalPadding * 2) + cardSpacing) / sw;
      if ((vp - _vpFraction).abs() > 0.001) {
        _pager.removeListener(_onPageChange);
        _pager.dispose();
        _pager = PageController(viewportFraction: vp);
        _pager.addListener(_onPageChange);
        _vpFraction = vp;
      }
    }
  }

  @override
  void didUpdateWidget(HomeBannerSection old) {
    super.didUpdateWidget(old);
    if (old.banners.length != widget.banners.length) {
      _startAutoPlay();
    }
  }

  @override
  void dispose() {
    _timer?.cancel();
    _pager.removeListener(_onPageChange);
    _pager.dispose();
    super.dispose();
  }

  void _onPageChange() {
    final p = _pager.page?.round() ?? 0;
    if (p != _page && mounted) setState(() => _page = p);
  }

  void _startAutoPlay() {
    _timer?.cancel();
    final count = widget.banners.length;
    _timer = Timer.periodic(const Duration(seconds: 5), (_) {
      if (mounted && _pager.hasClients && count > 1) {
        _pager.animateToPage(
          (_page + 1) % count,
          duration: const Duration(milliseconds: 400),
          curve: Curves.easeInOut,
        );
      }
    });
  }

  /// 有彈窗大圖才能點開。
  bool _hasPopup(api.Banner banner) =>
      banner.hasPopupImage() && banner.popupImage.uriTemplate.isNotEmpty;

  String _bannerLabel(api.Banner banner) {
    for (final value in [banner.title, banner.buttonText]) {
      if (value.trim().isNotEmpty) {
        return '活動橫幅：${value.trim()}';
      }
    }
    return '活動橫幅';
  }

  void _showBannerDetail(api.Banner banner) {
    if (!_hasPopup(banner)) return;
    showBannerPopupDialog(
      context,
      popupImage: banner.popupImage,
      action: BannerAction.fromProto(
        banner.action,
        buttonText: banner.buttonText,
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    if (widget.banners.isEmpty) return const SizedBox.shrink();

    return LayoutBuilder(
      builder: (context, constraints) {
        final width = constraints.maxWidth;
        final gutter = HkBreakpoints.pageGutter(width);
        final cardWidth = (width - gutter * 2)
            .clamp(0.0, double.infinity)
            .toDouble();
        // 卡片固定 16:9；高度隨卡片寬度換算，不再 clamp，
        // 避免高度被壓扁成扁條、圖片被 BoxFit.cover 裁掉上下。
        const bannerAspectRatio = 16 / 9;
        final bannerHeight = cardWidth / bannerAspectRatio;

        return Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            SizedBox(
              key: const ValueKey('home-banner-viewport'),
              height: bannerHeight,
              child: PageView.builder(
                controller: _pager,
                onPageChanged: (i) => setState(() => _page = i),
                itemCount: widget.banners.length,
                itemBuilder: (_, index) {
                  return Padding(
                    padding: const EdgeInsets.symmetric(horizontal: HkSpace.xs),
                    child: _buildBannerCard(widget.banners[index]),
                  );
                },
              ),
            ),
            const SizedBox(height: HkSpace.xs),
            Row(
              mainAxisAlignment: MainAxisAlignment.center,
              children: List.generate(widget.banners.length, (i) {
                final selected = i == _page;
                return Container(
                  width: selected ? HkSpace.md : HkSpace.xs,
                  height: HkSpace.xs,
                  margin: const EdgeInsets.symmetric(horizontal: HkSpace.xs),
                  decoration: BoxDecoration(
                    color: selected
                        ? AppColors.brandPrimaryStrong
                        : AppColors.textPrimary.withValues(alpha: 0.3),
                    borderRadius: BorderRadius.circular(HkRadius.pill),
                  ),
                );
              }),
            ),
          ],
        );
      },
    );
  }

  Widget _buildBannerCard(api.Banner banner) {
    final label = _bannerLabel(banner);
    final visual = ClipRRect(
      borderRadius: BorderRadius.circular(HkRadius.lg),
      child: AppCachedImage.api(
        image: banner.hasPreviewImage() ? banner.previewImage : null,
        scene: ImageScene.homeBanner,
        fit: BoxFit.cover,
        errorWidget: _buildImagePlaceholder(),
      ),
    );
    if (!_hasPopup(banner)) {
      return Semantics(
        image: true,
        label: label,
        child: ExcludeSemantics(child: visual),
      );
    }
    return HkTapTarget(
      semanticLabel: '查看$label',
      hint: '開啟活動內容',
      tooltip: label,
      onTap: () => _showBannerDetail(banner),
      child: visual,
    );
  }

  Widget _buildImagePlaceholder() {
    return const ColoredBox(
      color: AppColors.primarySoft,
      child: Center(
        child: Icon(
          AppIcons.imageOutlined,
          size: HkSpace.section,
          color: AppIconColors.active,
        ),
      ),
    );
  }
}
