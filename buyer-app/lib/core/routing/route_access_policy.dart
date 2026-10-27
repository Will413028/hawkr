/// Central route-access policy for guest browsing.
///
/// Product discovery, seller storefronts and the marketplace are public on
/// the website, so the App must only gate account-owned or mutating surfaces.
bool requiresLoginForRoute(String location) {
  const exact = {
    '/cart',
    '/chat',
    '/chat/ai-assistant',
    '/notifications',
    '/member-cards',
  };
  if (exact.contains(location)) return true;

  const prefixes = [
    '/orders',
    '/settings',
    '/wallet',
    '/chat/',
    '/payment',
    '/refund',
    '/review',
    '/member-cards/',
  ];
  return prefixes.any(location.startsWith);
}
