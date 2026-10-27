import { createExecutionContext, waitOnExecutionContext } from "cloudflare:test";
import { env as untypedEnv } from "cloudflare:workers";
import { describe, expect, it } from "vitest";
import worker, { type Env } from "../src/index";
import { animatedGifBytes, corruptJpegBytes, pdfBytes, samplePngBytes } from "./fixtures";

const BASE = "https://test-media.example.com";

// wrangler 生成的 `Cloudflare.Env`（worker-configuration.d.ts）把兩個 binding 都標成可選；
// wrangler.toml 的頂層已宣告兩個 binding，實際跑起來一定有值，這裡轉型成本檔真正用的 Env 型別。
const env = untypedEnv as unknown as Env;

async function dispatch(request: Request): Promise<Response> {
  const ctx = createExecutionContext();
  const response = await worker.fetch(request, env, ctx);
  await waitOnExecutionContext(ctx);
  return response;
}

async function seed(key: string, body: Uint8Array, contentType: string): Promise<void> {
  await env.MEDIA_BUCKET.put(key, body, { httpMetadata: { contentType } });
}

describe("路由與白名單", () => {
  it("preset 不在白名單 → 400", async () => {
    const res = await dispatch(new Request(`${BASE}/i/not-a-preset/public/product/1/x.jpg`));
    expect(res.status).toBe(400);
  });

  it("非 /i/* 路徑 → 404", async () => {
    const res = await dispatch(new Request(`${BASE}/health`));
    expect(res.status).toBe(404);
  });

  it("POST 方法 → 404", async () => {
    const res = await dispatch(new Request(`${BASE}/i/card/public/product/1/x.jpg`, { method: "POST" }));
    expect(res.status).toBe(404);
  });
});

describe("objectKey 前綴白名單（fail-closed）", () => {
  it("secure/ 前綴 → 404（即使物件存在）", async () => {
    await seed("secure/kyc/1/doc.jpg", samplePngBytes(), "image/png");
    const res = await dispatch(new Request(`${BASE}/i/card/secure/kyc/1/doc.jpg`));
    expect(res.status).toBe(404);
  });

  it("tmp/ 前綴 → 404（即使物件存在）", async () => {
    await seed("tmp/upload/1/x.jpg", samplePngBytes(), "image/png");
    const res = await dispatch(new Request(`${BASE}/i/card/tmp/upload/1/x.jpg`));
    expect(res.status).toBe(404);
  });
});

describe("R2 物件存在性", () => {
  it("objectKey 不存在 → 404", async () => {
    const res = await dispatch(new Request(`${BASE}/i/card/public/product/1/does-not-exist.jpg`));
    expect(res.status).toBe(404);
  });
});

describe("preset=original 直出原檔", () => {
  it("原檔位元組與 Content-Type 原樣返回，不經 Images 轉換", async () => {
    const bytes = samplePngBytes();
    await seed("public/product/1/orig.png", bytes, "image/png");
    const res = await dispatch(new Request(`${BASE}/i/original/public/product/1/orig.png`));
    expect(res.status).toBe(200);
    expect(res.headers.get("Content-Type")).toBe("image/png");
    expect(new Uint8Array(await res.arrayBuffer())).toEqual(bytes);
  });
});

describe("非圖片檔（Content-Type 非 image/*）→ 跳過轉換直出", () => {
  it("preset=detail 但物件是 PDF → 原樣直出", async () => {
    const bytes = pdfBytes();
    await seed("public/aftersales/1/receipt.pdf", bytes, "application/pdf");
    const res = await dispatch(new Request(`${BASE}/i/detail/public/aftersales/1/receipt.pdf`));
    expect(res.status).toBe(200);
    expect(res.headers.get("Content-Type")).toBe("application/pdf");
    expect(new Uint8Array(await res.arrayBuffer())).toEqual(bytes);
  });
});

describe("preset 轉換（真跑 Images binding）", () => {
  it("preset=avatar 對合法圖片轉換成功，回應為圖片", async () => {
    await seed("public/avatar/1/a.png", samplePngBytes(), "image/png");
    const res = await dispatch(new Request(`${BASE}/i/avatar/public/avatar/1/a.png`));
    expect(res.status).toBe(200);
    expect(res.headers.get("Content-Type")).toMatch(/^image\//);
  });

  it("preset=card 對合法圖片轉換成功", async () => {
    await seed("public/product/1/card.png", samplePngBytes(), "image/png");
    const res = await dispatch(new Request(`${BASE}/i/card/public/product/1/card.png`));
    expect(res.status).toBe(200);
    expect(res.headers.get("Content-Type")).toMatch(/^image\//);
  });
});

describe("format=auto（Accept header 協商，取代 binding 不支援的 'auto' 字面值）", () => {
  it("Accept 含 image/avif → 輸出 image/avif", async () => {
    await seed("public/product/1/fmt-avif.png", samplePngBytes(), "image/png");
    const res = await dispatch(
      new Request(`${BASE}/i/thumb/public/product/1/fmt-avif.png`, {
        headers: { Accept: "image/avif,image/webp,*/*" },
      }),
    );
    expect(res.status).toBe(200);
    expect(res.headers.get("Content-Type")).toBe("image/avif");
  });

  it("Accept 只含 image/webp → 輸出 image/webp", async () => {
    await seed("public/product/1/fmt-webp.png", samplePngBytes(), "image/png");
    const res = await dispatch(
      new Request(`${BASE}/i/thumb/public/product/1/fmt-webp.png`, {
        headers: { Accept: "image/webp,*/*" },
      }),
    );
    expect(res.status).toBe(200);
    expect(res.headers.get("Content-Type")).toBe("image/webp");
  });

  it("Accept 不含 avif/webp → 退回 image/jpeg", async () => {
    await seed("public/product/1/fmt-jpeg.png", samplePngBytes(), "image/png");
    const res = await dispatch(
      new Request(`${BASE}/i/thumb/public/product/1/fmt-jpeg.png`, {
        headers: { Accept: "text/html" },
      }),
    );
    expect(res.status).toBe(200);
    expect(res.headers.get("Content-Type")).toBe("image/jpeg");
  });
});

describe("transform 失敗 → degrade 直出原檔（不破圖）", () => {
  it("Content-Type 宣稱 image/jpeg 但 bytes 非法圖片 → Images binding 轉換失敗，fallback 回原始 bytes", async () => {
    const bytes = corruptJpegBytes();
    await seed("public/product/1/corrupt.jpg", bytes, "image/jpeg");
    const res = await dispatch(new Request(`${BASE}/i/card/public/product/1/corrupt.jpg`));
    expect(res.status).toBe(200);
    expect(res.headers.get("Content-Type")).toBe("image/jpeg");
    expect(new Uint8Array(await res.arrayBuffer())).toEqual(bytes);
  });
});

describe("CORS（公開圖資產，前端 fetch 需要）", () => {
  it("OPTIONS preflight → 204 + ACAO * + Allow-Methods GET, HEAD + Max-Age", async () => {
    const res = await dispatch(new Request(`${BASE}/i/card/public/product/1/x.jpg`, { method: "OPTIONS" }));
    expect(res.status).toBe(204);
    expect(res.headers.get("Access-Control-Allow-Origin")).toBe("*");
    expect(res.headers.get("Access-Control-Allow-Methods")).toBe("GET, HEAD");
    expect(res.headers.get("Access-Control-Max-Age")).toBeTruthy();
  });

  it("成功 GET → 帶 ACAO *", async () => {
    await seed("public/product/1/cors-ok.png", samplePngBytes(), "image/png");
    const res = await dispatch(new Request(`${BASE}/i/original/public/product/1/cors-ok.png`));
    expect(res.status).toBe(200);
    expect(res.headers.get("Access-Control-Allow-Origin")).toBe("*");
  });

  it("HEAD → 帶 ACAO *、無 body", async () => {
    await seed("public/product/1/cors-head.png", samplePngBytes(), "image/png");
    const res = await dispatch(new Request(`${BASE}/i/original/public/product/1/cors-head.png`, { method: "HEAD" }));
    expect(res.status).toBe(200);
    expect(res.headers.get("Access-Control-Allow-Origin")).toBe("*");
    expect((await res.arrayBuffer()).byteLength).toBe(0);
  });

  it("404（objectKey 不存在）→ 仍帶 ACAO *（前端讀得到狀態）", async () => {
    const res = await dispatch(new Request(`${BASE}/i/card/public/product/1/nope-cors.jpg`));
    expect(res.status).toBe(404);
    expect(res.headers.get("Access-Control-Allow-Origin")).toBe("*");
  });

  it("400（未知 preset）→ 仍帶 ACAO *", async () => {
    const res = await dispatch(new Request(`${BASE}/i/not-a-preset/public/product/1/x.jpg`));
    expect(res.status).toBe(400);
    expect(res.headers.get("Access-Control-Allow-Origin")).toBe("*");
  });

  it("secure/ 前綴 → 仍 404（CORS 放寬不改變 fail-closed 白名單），且帶 ACAO *", async () => {
    await seed("secure/kyc/2/doc.jpg", samplePngBytes(), "image/png");
    const res = await dispatch(new Request(`${BASE}/i/card/secure/kyc/2/doc.jpg`));
    expect(res.status).toBe(404);
    expect(res.headers.get("Access-Control-Allow-Origin")).toBe("*");
  });
});

describe("Cache-Control", () => {
  it("成功回應一律 immutable 一年快取", async () => {
    await seed("public/product/1/cache.png", samplePngBytes(), "image/png");
    const res = await dispatch(new Request(`${BASE}/i/original/public/product/1/cache.png`));
    expect(res.headers.get("Cache-Control")).toBe("public, max-age=31536000, immutable");
  });

  it("同一 URL 第二次請求命中 edge cache（不再依賴 R2 是否仍有該 key）", async () => {
    const key = "public/product/1/cache-hit.png";
    await seed(key, samplePngBytes(), "image/png");
    const url = new Request(`${BASE}/i/original/${key}`);

    const first = await dispatch(url);
    expect(first.status).toBe(200);

    // 快取寫入發生在 ctx.waitUntil 內，dispatch() 已 waitOnExecutionContext 等它完成。
    // 之後把 R2 物件刪掉：若第二次請求仍是 200，代表是從 edge cache 命中而非重新讀 R2。
    await env.MEDIA_BUCKET.delete(key);
    const second = await dispatch(new Request(`${BASE}/i/original/${key}`));
    expect(second.status).toBe(200);
    expect(new Uint8Array(await second.arrayBuffer())).toEqual(samplePngBytes());
  });
});

describe("motion preset（動圖：有界尺寸 + 保留動畫）", () => {
  it("Accept 含 webp → 輸出 image/webp（不是 avif，動圖不賭 animated AVIF）", async () => {
    await seed("public/post/1/anim-webp.gif", animatedGifBytes(), "image/gif");
    const res = await dispatch(
      new Request(`${BASE}/i/motion/public/post/1/anim-webp.gif`, {
        headers: { Accept: "image/avif,image/webp,*/*" },
      }),
    );
    expect(res.status).toBe(200);
    expect(res.headers.get("Content-Type")).toBe("image/webp");
  });

  it("Accept 不含 webp → 退回 image/gif，絕不落到 jpeg", async () => {
    await seed("public/post/1/anim-gif.gif", animatedGifBytes(), "image/gif");
    const res = await dispatch(
      new Request(`${BASE}/i/motion/public/post/1/anim-gif.gif`, {
        headers: { Accept: "text/html" },
      }),
    );
    expect(res.status).toBe(200);
    // jpeg 會把動圖壓成單張——這正是這個 preset 要防的事，所以斷言的是「不是 jpeg」
    // 而不只是「是 gif」：將來若多一個靜態退路，這條仍然會擋下來。
    expect(res.headers.get("Content-Type")).not.toBe("image/jpeg");
    expect(res.headers.get("Content-Type")).toBe("image/gif");
  });

  // ⚠️ 「輸出真的會動」在這裡驗不到，所以這裡不放那條斷言 —— 一支跑得過卻沒有
  // 鑑別力的測試比沒有測試更糟，它會讓人以為這件事有人守著。
  //
  // 本機 Miniflare Images 模擬器：`image/gif` 直接回
  // `IMAGES_TRANSFORM_ERROR 9520: GIF output is not supported in local mode`
  // （於是走 degrade 直出原檔，看起來「是動的」其實根本沒轉換過）；`image/webp`
  // 會成功但**靜默吐單幀**（無 ANMF chunk）。兩條路都證明不了真 binding 的行為。
  //
  // 動畫本身由 `scripts/verify-motion-preset.mjs` 對**已部署**的 worker 實測
  // （位元組數 + 幀數），見該檔與 49 號設計書 §4.4。這裡守的是我方這一側的兩個
  // 前提：格式沒落到靜態格式（上面兩條）、`anim` 有真的送出去（outputOptionsFor
  // 的單元測試）。
});
