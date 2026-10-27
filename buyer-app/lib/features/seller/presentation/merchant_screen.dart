import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../core/networking/dio_provider.dart';
import '../../../core/session/session_provider.dart';
import '../../../core/theme/app_colors.dart';
import '../../../shared/utils/hk_log.dart';
import '../../../shared/utils/require_login.dart';
import '../../chat/presentation/chat_focus_policy.dart';
import '../../follow/data/follow_repository.dart';
import '../data/seller_repository.dart';
import 'widgets/merchant_chat_tab.dart';
import 'widgets/merchant_common.dart';
import 'widgets/merchant_header.dart';
import 'widgets/merchant_home_tab.dart';
import 'widgets/merchant_shop_tab.dart';
import 'widgets/merchant_tab_bar.dart';

/// 商家頁（貼文 / 商城 / 留言 三個分頁）
class MerchantScreen extends ConsumerStatefulWidget {
  const MerchantScreen({super.key, this.sellerId, this.sellerIdStr});
  final int? sellerId;
  final String? sellerIdStr;

  @override
  ConsumerState<MerchantScreen> createState() => _MerchantScreenState();
}

class _MerchantScreenState extends ConsumerState<MerchantScreen>
    with SingleTickerProviderStateMixin {
  late final TabController _tabController = TabController(
    length: 3,
    vsync: this,
    initialIndex: 1,
  );

  // 商家信息狀態
  Map<String, dynamic>? _merchantInfo;
  bool _isLoading = true;
  bool _merchantNotFound =
      false; // getMerchantInfo 回 null（404 / 不存在）→ 顯示 empty state
  String _merchantName = '';
  String _merchantBannerUrl = '';
  String _merchantAvatarPayload = '';
  String _merchantAddress = '';
  String _merchantLocation = '';
  String _merchantDescription = '';
  String _merchantAward = '';
  double _merchantRating = 0.0;
  int _merchantRatingCount = 0;
  int _merchantFollowerCount = 0;
  bool _isFollowing = false;
  bool _isTogglingFollow = false;
  bool _isCertified = false;

  String? _resolvedMerchantId;

  // Prefetch cache for Shop tab
  List<Map<String, dynamic>>? _shopPrefetchedProducts;
  bool _shopPrefetchReady = false;
  bool _shopPrefetchStarted = false;

  bool _shopFetchInFlight = false;

  String? get _merchantId {
    final resolved = _resolvedMerchantId?.trim();
    if (resolved != null && resolved.isNotEmpty) return resolved;
    final fromStr = widget.sellerIdStr?.trim();
    if (fromStr != null && fromStr.isNotEmpty) return fromStr;
    final fromWidget = widget.sellerId?.toString();
    if (fromWidget != null && fromWidget.isNotEmpty) return fromWidget;
    if (_merchantInfo != null) {
      final extracted = _extractMerchantId(_merchantInfo!);
      if (extracted != null && extracted.isNotEmpty) return extracted;
    }
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
    WidgetsBinding.instance.addPostFrameCallback((_) {
      _skipRouteTransition();
    });
    final initialId = widget.sellerIdStr?.trim().isNotEmpty == true
        ? widget.sellerIdStr
        : widget.sellerId?.toString();
    if (initialId != null && initialId.isNotEmpty) {
      _resolvedMerchantId = initialId;
    }
    final hasId = _merchantId != null && _merchantId!.isNotEmpty;
    _loadMerchantInfo().whenComplete(() {
      if (!mounted) return;
    });
    if (hasId) {
      _checkFollowing();
    }
    WidgetsBinding.instance.addPostFrameCallback((_) {
      _startShopPrefetch();
    });
  }

  String? _extractMerchantId(Map<dynamic, dynamic> data) {
    // 路由 extra 帶進來的可能是本地合成的 merchantId，也可能是 API 原樣的
    // storeId；兩者都是 store surrogate（44 號）。
    const candKeys = ['merchantId', 'storeId', 'id'];
    for (final key in candKeys) {
      final value = data[key];
      if (value == null) continue;
      final s = value.toString().trim();
      if (s.isNotEmpty) return s;
    }
    return null;
  }

  Future<void> _loadMerchantInfo() async {
    final currentId = _merchantId;
    HKLog.logD('[MerchantScreen] _loadMerchantInfo merchantId=$currentId');
    if (currentId == null || currentId.isEmpty) {
      HKLog.logD('[MerchantScreen] merchantId is empty, skipping');
      setState(() {
        _isLoading = false;
      });
      return;
    }

    try {
      final response = await ref
          .read(sellerRepositoryProvider)
          .getMerchantInfo(currentId);
      HKLog.logD('[MerchantScreen] getMerchantInfo FULL response: $response');
      if (!mounted) return;
      if (response != null) {
        final dio = ref.read(dioProvider);

        // Rating 與 stats 都是公開資料；stats 已包含 followers，避免再透過
        // followRepository 重複呼叫同一個 /stats endpoint。
        double rating = 0.0;
        int ratingCount = 0;
        int followerCount = 0;
        int totalProducts = 0;

        final futures = await Future.wait([
          // [0] GET /public/sellers/{id}/rating
          dio
              .get<Map<String, dynamic>>('/public/sellers/$currentId/rating')
              .then((r) => r.data)
              .catchError((_) => null),
          // [1] GET /public/sellers/{id}/stats → { total_products, followers, ... }
          dio
              .get<Map<String, dynamic>>('/public/sellers/$currentId/stats')
              .then((r) => r.data)
              .catchError((_) => null),
        ]);

        // Parse rating
        final ratingResp = futures[0];
        final ratingData = ratingResp?['data'] as Map? ?? {};
        HKLog.logD('[MerchantScreen] rating response: $ratingData');
        if (ratingData['rating'] != null) {
          rating = double.tryParse(ratingData['rating'].toString()) ?? 0.0;
        }
        if (ratingData['reviewCount'] != null) {
          ratingCount = int.tryParse(ratingData['reviewCount'].toString()) ?? 0;
        }

        // Parse stats (handler.SellerStats: totalProducts, followers, totalOrders, totalSales)
        final statsResp = futures[1];
        final statsData = statsResp?['data'] as Map? ?? {};
        HKLog.logD('[MerchantScreen] stats response: $statsData');
        if (statsData['followers'] != null) {
          followerCount = int.tryParse(statsData['followers'].toString()) ?? 0;
        }
        if (statsData['totalProducts'] != null) {
          totalProducts =
              int.tryParse(statsData['totalProducts'].toString()) ?? 0;
        }

        HKLog.logD(
          '[MerchantScreen] Enriched: rating=$rating, ratingCount=$ratingCount, followers=$followerCount, totalProducts=$totalProducts',
        );

        final enrichedResponse = Map<String, dynamic>.from(response);
        enrichedResponse['_cachedRating'] = rating;
        enrichedResponse['_cachedRatingCount'] = ratingCount;
        enrichedResponse['_cachedFollowerCount'] = followerCount;
        enrichedResponse['_cachedTotalProducts'] = totalProducts;

        if (!mounted) return;
        _applyMerchantData(enrichedResponse);
      } else {
        setState(() {
          _isLoading = false;
          _merchantNotFound = true;
        });
      }
    } catch (e) {
      HKLog.logE('Failed to load merchant info: $e', e);
      if (!mounted) return;
      setState(() {
        _isLoading = false;
        _merchantNotFound = true;
      });
    }
  }

  void _applyMerchantData(Map<String, dynamic> response) {
    // ── OpenAPI: handler.SellerResponse { code, data: handler.SellerInfo, msg }
    final info = response['data'] is Map<String, dynamic>
        ? response['data'] as Map<String, dynamic>
        : response;

    HKLog.logD(
      '[MerchantScreen] _applyMerchantData info keys: ${info.keys.toList()}',
    );
    HKLog.logD(
      '[MerchantScreen] _applyMerchantData address fields: city=${info['city']}, district=${info['district']}, addressDetail=${info['addressDetail']}, address=${info['address']}, postalCode=${info['postalCode']}',
    );

    // ── handler.SellerInfo 嚴格取用 ──
    // 名稱
    final String storeName = (info['storeName'] ?? '').toString().trim();
    // 地址
    final String city = (info['city'] ?? '').toString().trim();
    final String district = (info['district'] ?? '').toString().trim();
    final String addressDetail = (info['addressDetail'] ?? '')
        .toString()
        .trim();
    final String addressFull = (info['address'] ?? '').toString().trim();
    final String postalCode = (info['postalCode'] ?? '').toString().trim();
    final String description = (info['description'] ?? '').toString().trim();
    // 圖片 — 49 §6.1 成對契約：裸 key 優先（coverKey / logoKey），url 退路；
    // 顯示端過 buildImageUrl dual-mode。
    final String coverUrl = (info['coverKey'] ?? info['coverUrl'] ?? '')
        .toString()
        .trim();
    final String avatar = (info['logoKey'] ?? info['logoUrl'] ?? '')
        .toString()
        .trim();
    final String logo = avatar;
    // 其他
    final String prize = (info['prize'] ?? '').toString().trim();
    // 44 號身分分離：賣家業務鍵是 storeId（store surrogate），不是 account_id。
    final dynamic rawAccountId = info['storeId'];
    final int accountId = (rawAccountId is num)
        ? rawAccountId.toInt()
        : (int.tryParse(rawAccountId?.toString() ?? '') ?? 0);
    final double infoRating = (info['rating'] is num)
        ? (info['rating'] as num).toDouble()
        : (double.tryParse(info['rating']?.toString() ?? '') ?? 0.0);
    final String certStatus = (info['overallCertificationStatus'] ?? '')
        .toString()
        .trim();

    // ── 組合顯示用欄位 ──
    // 名稱：只使用 store_name
    final String displayName = storeName;

    // 地址：依序嘗試 city+district, city, address_detail, address (combined), postal_code
    String displayAddress = '';
    if (city.isNotEmpty && district.isNotEmpty) {
      displayAddress = '$city$district';
      if (addressDetail.isNotEmpty) displayAddress += ' $addressDetail';
    } else if (city.isNotEmpty) {
      displayAddress = city;
      if (addressDetail.isNotEmpty) displayAddress += ' $addressDetail';
    } else if (addressDetail.isNotEmpty) {
      displayAddress = addressDetail;
    } else if (addressFull.isNotEmpty) {
      displayAddress = addressFull;
    }
    final String displaySubtitle = displayAddress.isNotEmpty
        ? displayAddress
        : (postalCode.isNotEmpty ? postalCode : '');
    final String displayLocation = city.isNotEmpty && district.isNotEmpty
        ? '$city$district'
        : (city.isNotEmpty ? city : district);
    HKLog.logD(
      '[MerchantScreen] displayAddress=$displayAddress, displaySubtitle=$displaySubtitle',
    );

    // 圖片：直接使用 API 回傳的 URL，不做轉換
    // handler.SellerInfo: avatar, logo, cover_url
    final String displayAvatar = avatar.isNotEmpty ? avatar : logo;
    final String displayBanner = coverUrl;

    // ── rating / stats 來自並行呼叫的 _cachedXxx ──
    double displayRating = infoRating;
    int ratingCount = 0;
    int followerCount = 0;
    if (response.containsKey('_cachedRating')) {
      displayRating =
          (response['_cachedRating'] as num?)?.toDouble() ?? infoRating;
      ratingCount = (response['_cachedRatingCount'] as num?)?.toInt() ?? 0;
      followerCount = (response['_cachedFollowerCount'] as num?)?.toInt() ?? 0;
    }

    // ── resolvedId ──
    final String resolvedIdStr = accountId > 0 ? accountId.toString() : '';
    final prevId = _merchantId;
    final bool idChanged =
        resolvedIdStr.isNotEmpty && resolvedIdStr != (prevId ?? '');

    setState(() {
      _merchantInfo = response;
      _merchantName = displayName;
      _merchantBannerUrl = displayBanner;
      _merchantAvatarPayload = displayAvatar;
      _merchantAddress = displayAddress;
      _merchantLocation = displayLocation;
      _merchantDescription = description;
      _merchantAward = prize;
      _merchantRating = displayRating;
      _merchantRatingCount = ratingCount;
      _merchantFollowerCount = followerCount;
      _isCertified = certStatus == 'approved';
      _isLoading = false;
      if (resolvedIdStr.isNotEmpty) {
        _resolvedMerchantId = resolvedIdStr;
      }
    });

    if (idChanged) {
      _startShopPrefetch(force: true);
    }
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
      final intId = int.tryParse(mid);
      if (intId == null) return;

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
          await ref.read(followRepositoryProvider).unfollow(intId);
        } else {
          await ref.read(followRepositoryProvider).follow(intId);
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
                '找不到此商家',
                style: TextStyle(
                  fontSize: 16,
                  fontWeight: FontWeight.w700,
                  color: AppColors.textPrimary,
                ),
              ),
              SizedBox(height: 8.h),
              Text(
                '商家可能已下架或暫停營業',
                style: TextStyle(
                  fontSize: 14,
                  color: AppColors.textPrimary.withValues(alpha: 0.6),
                ),
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
                isCertified: _isCertified,
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
