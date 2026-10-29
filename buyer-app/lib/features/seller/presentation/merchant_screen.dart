import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart' as seller;
import 'package:hawkr_api/images.dart' show ImagePreset, ImageUri;
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../core/exceptions/app_exception.dart';
import '../../../core/networking/dio_provider.dart';
import '../../../core/session/session_provider.dart';
import '../../../core/theme/app_colors.dart';
import '../../../shared/utils/hk_log.dart';
import '../../../shared/utils/require_login.dart';
import '../../../shared/widgets/hk_button_label.dart';
import '../../chat/presentation/chat_focus_policy.dart';
import '../../follow/data/follow_repository.dart';
import '../data/store_card_display.dart';
import '../data/seller_repository.dart';
import 'widgets/merchant_chat_tab.dart';
import 'widgets/merchant_common.dart';
import 'widgets/merchant_header.dart';
import 'widgets/merchant_home_tab.dart';
import 'widgets/merchant_shop_tab.dart';
import 'widgets/merchant_tab_bar.dart';

/// 商家頁（貼文 / 商城 / 留言 三個分頁）。[storeCode] 是店家網址的路徑段，不是 store id。
class MerchantScreen extends ConsumerStatefulWidget {
  const MerchantScreen({super.key, required this.storeCode});
  final String storeCode;

  @override
  ConsumerState<MerchantScreen> createState() => _MerchantScreenState();
}

class _MerchantScreenState extends ConsumerState<MerchantScreen>
    with SingleTickerProviderStateMixin {
  late final TabController _tabController;

  // 商家信息狀態
  bool _isLoading = true;
  bool _merchantNotFound =
      false; // GetStore 回 not_found（不存在或不公開）→ 顯示 empty state
  String? _loadError; // 其他失敗（網路、伺服器）→ 顯示可重試的錯誤，不當成店家不存在
  String _merchantName = '';
  String _merchantBannerUrl = '';
  String _merchantAvatarPayload = '';
  String _merchantAddress = '';
  String _merchantLocation = '';
  String _merchantDescription = '';
  String _merchantAward = '';
  double? _merchantRating; // 沒有評分（讀不到或還沒有評價）是 null，不當成 0
  int? _merchantRatingCount;
  int _merchantFollowerCount = 0;
  bool _isFollowing = false;
  bool _isTogglingFollow = false;

  // GetStore 回的 store id；商品、貼文、討論區與追蹤都以它為鍵。
  String? _resolvedMerchantId;

  // Prefetch cache for Shop tab
  List<Map<String, dynamic>>? _shopPrefetchedProducts;
  bool _shopPrefetchReady = false;
  bool _shopPrefetchStarted = false;

  bool _shopFetchInFlight = false;

  String? get _merchantId {
    final resolved = _resolvedMerchantId?.trim();
    if (resolved != null && resolved.isNotEmpty) return resolved;
    return null;
  }

  String get _merchantIdOrEmpty => _merchantId ?? '';

  @override
  void dispose() {
    _tabController.dispose();
    super.dispose();
  }

  @override
  void initState() {
    super.initState();
    _tabController = TabController(length: 3, vsync: this, initialIndex: 1);
    WidgetsBinding.instance.addPostFrameCallback((_) {
      _skipRouteTransition();
    });
    _loadMerchantInfo();
  }

  Future<void> _loadMerchantInfo() async {
    if (_loadError != null) {
      setState(() {
        _loadError = null;
        _isLoading = true;
      });
    }
    try {
      final response = await ref
          .read(sellerRepositoryProvider)
          .getStore(widget.storeCode);
      if (!mounted) return;
      _applyStore(response);
      _checkFollowing();
      _startShopPrefetch(force: true);
    } catch (e) {
      HKLog.logE('Failed to load merchant info: $e', e);
      if (!mounted) return;
      setState(() {
        _isLoading = false;
        if (e is NotFoundException) {
          _merchantNotFound = true;
        } else {
          _loadError = e is AppException ? e.message : '$e';
        }
      });
    }
  }

  void _applyStore(seller.GetStoreResponse response) {
    final store = response.store;
    final stats = response.stats;
    final location = townLocation(store.townCode);
    setState(() {
      _merchantName = store.storeName.trim();
      _merchantBannerUrl = store.hasCoverImage()
          ? store.coverImage.uri(ImagePreset.banner)
          : '';
      _merchantAvatarPayload = store.hasLogo()
          ? store.logo.uri(ImagePreset.avatar)
          : '';
      _merchantAddress = location;
      _merchantLocation = location;
      _merchantDescription = store.description.trim();
      _merchantAward = store.prize.trim();
      _merchantRating = stats.hasRatingAverage() ? stats.ratingAverage : null;
      _merchantRatingCount = stats.hasReviewCount() ? stats.reviewCount : null;
      _merchantFollowerCount = stats.hasFollowerCount()
          ? stats.followerCount
          : 0;
      _resolvedMerchantId = store.id;
      _isLoading = false;
    });
  }

  Future<void> _checkFollowing() async {
    try {
      final session = await ref.read(sessionProvider.future);
      if (!hasUsableSession(session)) return;
      final mid = _merchantIdOrEmpty;
      if (mid.isEmpty) return;
      final followed = await ref
          .read(followRepositoryProvider)
          .checkFollow(mid);
      if (!mounted) return;
      setState(() {
        _isFollowing = followed;
      });
    } catch (_) {}
  }

  Future<void> _toggleFollow() async {
    if (_isTogglingFollow) return;
    _isTogglingFollow = true;
    try {
      if (!await requireLogin(context, ref)) return;
      if (!mounted) return;
      final mid = _merchantIdOrEmpty;
      if (mid.isEmpty) return;

      // 樂觀更新：先切換 UI，再呼叫 API
      final wasFollowing = _isFollowing;
      final prevCount = _merchantFollowerCount;

      setState(() {
        _isFollowing = !wasFollowing;
        _merchantFollowerCount = wasFollowing
            ? (prevCount - 1).clamp(0, double.maxFinite.toInt())
            : prevCount + 1;
      });

      try {
        if (wasFollowing) {
          await ref.read(followRepositoryProvider).unfollow(mid);
        } else {
          await ref.read(followRepositoryProvider).follow(mid);
        }
      } catch (e) {
        // API 失敗 → 回滾
        HKLog.logW('[MerchantScreen] toggleFollow failed, rolling back', e);
        if (!mounted) return;
        setState(() {
          _isFollowing = wasFollowing;
          _merchantFollowerCount = prevCount;
        });
      }
    } finally {
      _isTogglingFollow = false;
    }
  }

  Future<void> _startShopPrefetch({bool force = false}) async {
    if (_shopFetchInFlight) return;
    final id = _merchantIdOrEmpty;
    if (id.isEmpty) {
      if (!_shopPrefetchReady) {
        setState(() {
          _shopPrefetchReady = true;
        });
      }
      return;
    }
    if (!force && _shopPrefetchStarted && _shopPrefetchReady) {
      return;
    }
    _shopPrefetchStarted = true;
    _shopFetchInFlight = true;
    try {
      final dio = ref.read(dioProvider);
      HKLog.logD(
        '[MerchantScreen] _startShopPrefetch fetching /public/sellers/$id/products',
      );
      final resp = await dio.get<Map<String, dynamic>>(
        '/public/sellers/$id/products',
        queryParameters: {'page': 1, 'page_size': 50, 'status': 1},
      );
      final body = resp.data ?? {};
      HKLog.logD('[MerchantScreen] _startShopPrefetch FULL response: $body');
      final list = MerchantApiHelper.extractList(body);
      HKLog.logD(
        '[MerchantScreen] _startShopPrefetch got ${list.length} products',
      );
      if (!mounted) return;
      setState(() {
        _shopPrefetchedProducts = list;
        _shopPrefetchReady = true;
      });
    } catch (e) {
      HKLog.logE('[MerchantScreen] _startShopPrefetch error: $e', e);
      if (!mounted) return;
      setState(() {
        _shopPrefetchReady = true;
      });
    } finally {
      _shopFetchInFlight = false;
    }
  }

  void _skipRouteTransition() {
    final route = ModalRoute.of(context);
    if (route == null) return;
    void jumpToEnd(Animation<double>? anim) {
      if (anim == null) return;
      final parent = anim is ProxyAnimation ? anim.parent : anim;
      if (parent is AnimationController) {
        parent.value = 1;
        parent.stop();
      }
    }

    jumpToEnd(route.animation);
    jumpToEnd(route.secondaryAnimation);
  }

  @override
  Widget build(BuildContext context) {
    // 讀取失敗（非 not_found）→ 可重試的錯誤，不要讓人以為店家不存在
    final loadError = _loadError;
    if (!_isLoading && loadError != null) {
      return Scaffold(
        backgroundColor: AppColors.bgPrimary,
        appBar: AppBar(
          backgroundColor: Colors.transparent,
          elevation: 0,
          leading: IconButton(
            icon: const Icon(
              AppIcons.arrowBack,
              color: AppIconColors.defaultColor,
            ),
            onPressed: () => Navigator.of(context).maybePop(),
          ),
        ),
        body: Center(
          child: Padding(
            padding: EdgeInsets.symmetric(horizontal: 24.w),
            child: Column(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                Text(
                  loadError,
                  textAlign: TextAlign.center,
                  style: TextStyle(
                    fontSize: 14,
                    color: AppColors.textPrimary.withValues(alpha: 0.6),
                  ),
                ),
                SizedBox(height: 16.h),
                ElevatedButton(
                  onPressed: _loadMerchantInfo,
                  child: const HkButtonLabel('重試'),
                ),
              ],
            ),
          ),
        ),
      );
    }

    // 商家不存在 / API 404 → 顯示 empty state，不要 render 整個空白商家頁
    if (!_isLoading && _merchantNotFound) {
      return Scaffold(
        backgroundColor: AppColors.bgPrimary,
        appBar: AppBar(
          backgroundColor: Colors.transparent,
          elevation: 0,
          leading: IconButton(
            icon: const Icon(
              AppIcons.arrowBack,
              color: AppIconColors.defaultColor,
            ),
            onPressed: () => Navigator.of(context).maybePop(),
          ),
        ),
        body: Center(
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              const HkIcon(HkIconName.emptyMerchantSearch, size: 64),
              SizedBox(height: 16.h),
              Text(
                '找不到這家店',
                style: TextStyle(
                  fontSize: 16,
                  fontWeight: FontWeight.w700,
                  color: AppColors.textPrimary,
                ),
              ),
              SizedBox(height: 8.h),
              Text(
                '這家店可能暫停營業、已經關閉，或網址有誤。',
                style: TextStyle(
                  fontSize: 14,
                  color: AppColors.textPrimary.withValues(alpha: 0.6),
                ),
              ),
              SizedBox(height: 16.h),
              OutlinedButton(
                onPressed: () => Navigator.of(context).maybePop(),
                child: const HkButtonLabel('返回'),
              ),
            ],
          ),
        ),
      );
    }

    final bool hasBanner = MerchantImageHelper.isCustomBanner(
      _merchantBannerUrl,
    );
    final statusBarStyle = hasBanner
        ? SystemUiOverlayStyle.light.copyWith(
            statusBarColor: Colors.transparent,
          )
        : SystemUiOverlayStyle.dark.copyWith(
            statusBarColor: Colors.transparent,
          );

    VoidCallback chatCallback() => () async {
      if (!await requireLogin(context, ref)) return;
      if (!context.mounted) return;
      ChatFocusPolicy.dismissKeyboard();
      context.push(
        '/chat/room',
        extra: {
          'merchantId': _merchantIdOrEmpty,
          'merchantName': _merchantName,
        },
      );
    };

    VoidCallback cartCallback() => () async {
      if (!await requireLogin(context, ref)) return;
      if (!context.mounted) return;
      context.push('/cart');
    };

    return AnnotatedRegion<SystemUiOverlayStyle>(
      value: statusBarStyle,
      child: Scaffold(
        backgroundColor: AppColors.bgPrimary,
        body: NestedScrollView(
          headerSliverBuilder: (context, innerBoxIsScrolled) => [
            SliverToBoxAdapter(
              child: MerchantHeader(
                bannerUrl: _merchantBannerUrl,
                avatarPayload: _merchantAvatarPayload,
                merchantName: _merchantName,
                merchantAddress: _merchantAddress,
                merchantLocation: _merchantLocation,
                merchantDescription: _merchantDescription,
                merchantAward: _merchantAward,
                rating: _merchantRating,
                ratingCount: _merchantRatingCount,
                followerCount: _merchantFollowerCount,
                isFollowing: _isFollowing,
                isLoading: _isLoading,
                onToggleFollow: _toggleFollow,
                onChat: chatCallback(),
                onCart: cartCallback(),
              ),
            ),
            SliverOverlapAbsorber(
              handle: NestedScrollView.sliverOverlapAbsorberHandleFor(context),
              sliver: SliverPersistentHeader(
                pinned: true,
                delegate: MerchantTabBarSliverDelegate(
                  tabController: _tabController,
                ),
              ),
            ),
          ],
          body: Builder(
            builder: (innerCtx) {
              final handle = NestedScrollView.sliverOverlapAbsorberHandleFor(
                innerCtx,
              );
              return TabBarView(
                controller: _tabController,
                physics: const BouncingScrollPhysics(),
                children: [
                  MerchantHomeTab(
                    ref: ref,
                    merchantId: _merchantId,
                    merchantName: _merchantName,
                    merchantAvatar: _merchantAvatarPayload,
                    outerHandle: handle,
                  ),
                  MerchantShopTab(
                    ref: ref,
                    merchantId: _merchantId,
                    merchantName: _merchantName,
                    merchantAvatar: _merchantAvatarPayload,
                    prefetchedProducts: _shopPrefetchedProducts,
                    prefetchedReady: _shopPrefetchReady,
                    outerHandle: handle,
                  ),
                  MerchantChatTab(
                    ref: ref,
                    merchantId: _merchantId,
                    merchantName: _merchantName,
                    outerHandle: handle,
                  ),
                ],
              );
            },
          ),
        ),
      ),
    );
  }
}
