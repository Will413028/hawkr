import type { Page } from "@playwright/test";

declare global {
	interface Window {
		/** Test-only: the arguments of every `confirmPayment` call on the stubbed Stripe.js. */
		__stripeConfirmCalls?: Array<{ return_url?: string; redirect?: string }>;
	}
}

/**
 * 以最小的替身取代 https://js.stripe.com/*：`Stripe(key)` 回傳只有 Payment Element 掛載與 `confirmPayment`
 * 的物件，`confirmPayment` 回 `{}`（沒有即時拒絕）。頁面送出後的結果仍由 GetPayment 決定，這裡不模擬 Stripe 的任何結果。
 * 在載入頁面前呼叫。
 */
export async function installStripeStub(page: Page): Promise<void> {
	await page.route("https://js.stripe.com/**", (route) =>
		route.fulfill({
			status: 200,
			contentType: "application/javascript",
			body: `
(function () {
  window.__stripeConfirmCalls = [];
  function element() {
    var handlers = {};
    return {
      mount: function (node) {
        var host = typeof node === "string" ? document.querySelector(node) : node;
        if (host) host.setAttribute("data-testid", "stub-payment-element");
        (handlers.ready || []).forEach(function (h) { h(); });
      },
      on: function (name, h) { (handlers[name] = handlers[name] || []).push(h); },
      update: function () {},
      unmount: function () {},
      destroy: function () {},
      collapse: function () {}
    };
  }
  window.Stripe = function () {
    return {
      registerAppInfo: function () {},
      _registerWrapper: function () {},
      createToken: function () { return Promise.resolve({}); },
      createPaymentMethod: function () { return Promise.resolve({}); },
      confirmCardPayment: function () { return Promise.resolve({}); },
      elements: function () {
        return {
          create: element,
          getElement: function () { return null; },
          update: function () {},
          fetchUpdates: function () { return Promise.resolve({}); },
          submit: function () { return Promise.resolve({}); }
        };
      },
      confirmPayment: function (args) {
        window.__stripeConfirmCalls.push(args.confirmParams);
        return Promise.resolve({});
      }
    };
  };
})();
`,
		}),
	);
}
