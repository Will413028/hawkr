import { describe, expect, it } from "vitest";
import { negotiateFormat, outputOptionsFor } from "../src/index";
import {
  ORIGINAL_PRESET_NAME,
  PRESETS,
  isAnimatedPreset,
  isKnownPreset,
  isTransformPreset,
} from "../src/presets";

// 對照 49 號設計書 §4.4 preset 白名單表格，逐值核對——本檔是全平台唯一定義點，
// 任何數值 drift 都代表與設計書失去同步。
describe("PRESETS 對照 49 號設計書 §4.4", () => {
  it("avatar", () => {
    expect(PRESETS.avatar).toEqual({ transform: { width: 160, height: 160, fit: "cover" }, quality: 60 });
  });

  it("thumb", () => {
    expect(PRESETS.thumb).toEqual({ transform: { width: 240, height: 240, fit: "cover" }, quality: 70 });
  });

  it("card", () => {
    expect(PRESETS.card).toEqual({ transform: { width: 720, height: 540, fit: "cover" }, quality: 75 });
  });

  it("detail", () => {
    expect(PRESETS.detail).toEqual({ transform: { width: 1280, fit: "scale-down" }, quality: 80 });
  });

  it("banner", () => {
    expect(PRESETS.banner).toEqual({ transform: { width: 1600, fit: "scale-down" }, quality: 85 });
  });

  it("chat", () => {
    expect(PRESETS.chat).toEqual({ transform: { width: 720, fit: "scale-down" }, quality: 75 });
  });

  it("motion", () => {
    expect(PRESETS.motion).toEqual({
      transform: { width: 720, fit: "scale-down" },
      quality: 70,
      animated: true,
    });
  });

  it("恰好 7 個轉換 preset（不含 original）", () => {
    expect(Object.keys(PRESETS).sort()).toEqual([
      "avatar",
      "banner",
      "card",
      "chat",
      "detail",
      "motion",
      "thumb",
    ]);
  });
});

describe("preset 白名單判核", () => {
  it("7 個轉換 preset 皆為已知且為 transform preset", () => {
    for (const name of Object.keys(PRESETS)) {
      expect(isKnownPreset(name)).toBe(true);
      expect(isTransformPreset(name)).toBe(true);
    }
  });

  it("original 是已知 preset 但非 transform preset", () => {
    expect(isKnownPreset(ORIGINAL_PRESET_NAME)).toBe(true);
    expect(isTransformPreset(ORIGINAL_PRESET_NAME)).toBe(false);
  });

  it("未知 preset 名一律回 false（deny-by-default）", () => {
    for (const bogus of ["", "avatar2", "AVATAR", "card ", "../avatar", "webp"]) {
      expect(isKnownPreset(bogus)).toBe(false);
    }
  });
});

describe("animated 旗標", () => {
  it("只有 motion 保留動畫幀", () => {
    for (const name of Object.keys(PRESETS)) {
      expect(isAnimatedPreset(name as keyof typeof PRESETS)).toBe(name === "motion");
    }
  });
});

describe("動圖 preset 的兩個前提（本機唯一驗得到的部分）", () => {
  it("motion 的格式協商絕不落到靜態格式", () => {
    for (const accept of [
      "image/avif,image/webp,*/*",
      "image/webp,*/*",
      "text/html",
      "",
      null,
    ]) {
      const format = negotiateFormat(accept, true);
      // 反面斷言才是保證本身：jpeg/avif 會把動圖壓成單張。
      expect(format).not.toBe("image/jpeg");
      expect(format).not.toBe("image/avif");
      expect(["image/webp", "image/gif"]).toContain(format);
    }
  });

  it("非動圖 preset 的協商維持原行為（avif → webp → jpeg）", () => {
    expect(negotiateFormat("image/avif,image/webp,*/*")).toBe("image/avif");
    expect(negotiateFormat("image/webp,*/*")).toBe("image/webp");
    expect(negotiateFormat("text/html")).toBe("image/jpeg");
  });

  it("motion 一定把 anim: true 送進 .output()", () => {
    expect(outputOptionsFor("motion", "image/webp")).toEqual({
      format: "image/webp",
      quality: 70,
      anim: true,
    });
  });

  it("其餘 preset 一律不帶 anim（不改動既有行為）", () => {
    for (const name of Object.keys(PRESETS)) {
      if (name === "motion") continue;
      expect(outputOptionsFor(name as keyof typeof PRESETS, "image/webp")).not.toHaveProperty("anim");
    }
  });
});
