import assert from "node:assert/strict";
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { takePickedFiles } from "@/lib/media/pick-files";

/**
 * `<input type="file">` 的選檔順序：**先具體化，再清空**。
 *
 * `input.files` 是 live FileList，`input.value = ""` 會就地清空它。代行工作區的商品
 * 建立／編輯兩頁曾先清空再把那個參考傳給上傳函式，於是接收端的
 * `if (files.length === 0) return;` 每次都早退 —— 圖片數不變、錯誤訊息為空、儲存鈕
 * 仍 disabled、**零筆網路請求**。整條媒體上傳 100% 失效，而畫面上什麼訊號都沒有。
 *
 * 這種靜默失效沒有型別、編譯或既有測試擋得住（兩行都合法、順序才是錯的），所以這支
 * 檔案守兩件事：
 * ① `takePickedFiles` 的順序本身（行為測試，用一個忠實模擬 live 語意的假 input）
 * ② 全 repo 不得再出現「把 live FileList 存進變數，然後在用它之前清空 `value`」
 */

const ROOT = fileURLToPath(new URL("..", import.meta.url));

/** 把 live FileList 存進變數（沒有 `Array.from` / `?.[0]` 這類具體化）。 */
const KEEPS_LIVE_LIST = /(?:const|let|var)\s+\w+\s*=\s*[\w.]*\.files\s*;/g;
/** 同一段裡把 `value` 清成空字串。 */
const CLEARS_VALUE = /\.value\s*=\s*(""|'')/;
/** 兩者相隔多遠仍算同一個 handler。 */
const WINDOW = 240;

function walk(dir: string): string[] {
	const out: string[] = [];
	let entries: string[];
	try {
		entries = readdirSync(join(ROOT, dir));
	} catch {
		return out;
	}
	for (const entry of entries) {
		if (entry === "node_modules" || entry === ".next" || entry === ".git") {
			continue;
		}
		const rel = relative(ROOT, join(ROOT, dir, entry));
		if (statSync(join(ROOT, rel)).isDirectory()) out.push(...walk(rel));
		else if (/\.tsx?$/.test(rel)) out.push(rel);
	}
	return out;
}

function sourceFiles(): string[] {
	return ["app", "components", "features", "lib"]
		.flatMap(walk)
		.filter((path) => !/\.(test|spec|unit\.test)\.tsx?$/.test(path));
}

/** 一段原始碼裡「留著 live FileList 又清空 value」的每一處（回行號）。 */
function clearBeforeUseHits(src: string): number[] {
	const hits: number[] = [];
	KEEPS_LIVE_LIST.lastIndex = 0;
	let match = KEEPS_LIVE_LIST.exec(src);
	while (match) {
		const after = src.slice(match.index, match.index + WINDOW);
		if (CLEARS_VALUE.test(after)) {
			hits.push(src.slice(0, match.index).split("\n").length);
		}
		match = KEEPS_LIVE_LIST.exec(src);
	}
	return hits;
}

/**
 * 忠實模擬 live FileList：`value = ""` 就地清空清單。
 *
 * 這正是瀏覽器的行為，也是整個 bug 的機制 —— 假 input 若不模擬它，下面的行為測試
 * 就會對真正的錯誤形狀失明。
 */
function liveFileInput(files: File[]): HTMLInputElement {
	// ⚠️ 清空必須**就地**進行（`length = 0`），不能換一個新陣列：真實 FileList 是
	// 同一個物件被清空，所以先取走的那個「參考」也會跟著變空 —— 那正是 bug 的來源。
	const list: File[] = files;
	return {
		get files() {
			return list;
		},
		get value() {
			return list.length > 0 ? `C:\\fakepath\\${list[0].name}` : "";
		},
		set value(next: string) {
			if (next === "") list.length = 0;
		},
	} as unknown as HTMLInputElement;
}

function twoFiles(): File[] {
	return [
		new File(["a"], "a.jpg", { type: "image/jpeg" }),
		new File(["b"], "b.jpg", { type: "image/jpeg" }),
	];
}

test("選檔之後拿到的檔案數 > 0（清空 value 不得把選到的檔案一起帶走）", () => {
	const input = liveFileInput(twoFiles());

	const picked = takePickedFiles(input);

	assert.equal(
		picked.length,
		2,
		"清空 value 前必須先把 live FileList 具體化成 File[]，否則上傳鏈整條靜默不執行",
	);
	assert.deepEqual(
		picked.map((f) => f.name),
		["a.jpg", "b.jpg"],
	);
});

test("選完檔仍會清空 value（同一個檔案要能再選一次）", () => {
	const input = liveFileInput(twoFiles());

	takePickedFiles(input);

	assert.equal(
		input.value,
		"",
		"不清空 value，連續選同一個檔案不會再觸發 change",
	);
	assert.equal(input.files?.length, 0);
});

test("假 input 真的會被錯誤順序打到（鑑別力自檢）", () => {
	// 這是修復前的寫法。假 input 若沒有模擬 live 語意，這裡會拿到 2，上面兩支測試
	// 就變成「怎麼寫都會綠」。
	const input = liveFileInput(twoFiles());
	const live = input.files;
	input.value = "";
	const broken = Array.from(live ?? []);

	assert.equal(
		broken.length,
		0,
		"假 input 沒有模擬 live FileList，本檔的斷言全部失效",
	);
});

test("全 repo 不得把 live FileList 存進變數後才清空 value", () => {
	const offenders = sourceFiles().flatMap((file) =>
		clearBeforeUseHits(readFileSync(join(ROOT, file), "utf8")).map(
			(line) => `${file}:${line}`,
		),
	);

	assert.deepEqual(
		offenders,
		[],
		"先清空 value 再使用 `e.target.files` ⇒ 拿到空清單，接收端早退，畫面零訊號。改用 lib/media/pick-files.ts 的 takePickedFiles(e.target)。",
	);
});

test("掃描器抓得到錯誤形狀，且掃得到東西（鑑別力自檢）", () => {
	const broken = `onChange={(e) => {
		const files = e.target.files;
		e.target.value = "";
		void handlePickImages(files);
	}}`;
	assert.deepEqual(
		clearBeforeUseHits(broken),
		[2],
		"述詞抓不到修復前的原始寫法 ⇒ 上面那條「零命中」沒有意義",
	);

	const safe = `const picked = Array.from(input.files ?? []);
	input.value = "";`;
	assert.deepEqual(
		clearBeforeUseHits(safe),
		[],
		"具體化之後才清空是正確寫法，不得誤報",
	);

	const files = sourceFiles();
	assert.ok(
		files.length > 300,
		`只掃到 ${files.length} 支檔案，述詞或路徑有誤`,
	);
});
