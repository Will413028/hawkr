"use client";

import { useQuery } from "@tanstack/react-query";
import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import Link from "@/components/shared/Link";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import { Dropdown } from "@/components/ui";
import Button from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import DescriptionEditor from "@/features/seller/components/DescriptionEditor";
import ShelfStatusControl, {
	resolveShelfState,
} from "@/features/seller-center/components/ShelfStatusControl";
import { ApiClientError } from "@/lib/api/client";
import type { DelegatedMutationMeta } from "@/lib/api/delegated";
import {
	PRODUCT_STATUS_LABEL,
	SHIPPING_METHOD_LABEL,
} from "@/lib/codemeta/enums";
import { takePickedFiles } from "@/lib/media/pick-files";
import {
	type CategoryNode,
	categoryDropdownItems,
	fetchPublicCategoryTree,
	findCategoryPath,
	type ProductDescription,
} from "@/lib/products/seller-repo";
import { useSellerDataScope } from "@/lib/seller-data/scope";
import DelegatedAreaUnauthorized from "../components/DelegatedAreaUnauthorized";
import { shelfStatusChange, targetStoreLabel } from "../confirm-content";
import {
	classifyDelegatedLoadFailure,
	type DelegatedLoadFailureKind,
	delegatedProductLoadFailureCopy,
} from "../load-failure";
import { useDelegatedSession } from "../state/DelegatedSessionContext";
import {
	type DelegatedProductFormDraft,
	type DelegatedSkuDraft,
	draftFromDetail,
	keepMyEdits,
} from "./conflict";
import {
	buildProductChanges,
	buildProductPatch,
	type ImageDraft,
	imageDraftsOf,
	unitText,
} from "./edit-patch";
import {
	createDelegatedProductEditRepo,
	type DelegatedProductDetail,
} from "./repo";

/**
 * 代行工作區・編輯商品（mockup image6／§6.1.8）。
 *
 * ⚠️ **這一頁刻意不是 `features/seller-center/products/ProductFormPage.tsx` 的一個
 * 分支**：那支表單的資料形狀是賣家端的 `draft`（`useProductForm`），與代行的
 * detail／PATCH 契約在三個最危險的地方不同 —— `images` 是整份替換且原圖必須以
 * `{assetId}` 回送、`skus` 的刪除只認 `removedSkuIds`、`status` 出現在 PATCH body 裡
 * 就是 9260。半套映射的失敗形狀是**請求完全合法、回 `code: 0`，而賣家的商品圖片被
 * 清空**（L3774）。它還綁著賣家自己的認證閘記（`useCertGate`）與 AI 助手，兩者在
 * 代行下都沒有資料來源。
 *
 * **B5（本頁最大的陷阱）**：mockup 把「改庫存」與「上架」畫成同一個確認動作，實際是
 * **兩支端點、兩個 capability、兩筆稽核列、兩個 `auditLogId`**。所以這裡有兩顆各自
 * 開確認抽屜的按鈕，送出前預覽也是逐次一張；`/publish` 的變更恆為單一欄位，不畫
 * 三欄表（L3704）。
 *
 * **B20**：送出前的變更摘要由前端本地自算（比較已載入的 DTO 與當前表單），欄位名取
 * 表單自己的標籤，且**必須排除唯讀觀察欄位**（`status`／`priceMin`／`priceMax`／商品層
 * `stockQty`）—— 它們在 PATCH 一律被忽略且不報錯，列進預覽等於展示一筆不會發生的變更。
 * 算式本體在 `./edit-patch`（純函式，逐條可測）。
 *
 * **分類與認證是可編輯的**（§6.1.12 規則表（2）兩者皆標「可編輯 ✔」，mockup image6 畫的
 * 也是三層下拉＋帶 × 的 chip）。它們與其他欄位同屬 `products.write`（§6.1.1 端點總表：
 * `PATCH .../products/{id}` → `products.write`；九個 capability 是**封閉列舉**，要開第十
 * 個得走 §6.1.14 的十步檢查）。認證是法規宣稱，配套落在**送出前**：三欄表逐項列出、
 * 抽屜在動到認證時多說一句、通知賣家預設勾選（§4.1 對高風險操作的「diff＋二次確認＋
 * 通知」）。
 */

/** `product.name` 的真正上限是 `VARCHAR(255)`；mockup 上的「/ 60 字」是假的（D4）。 */
const NAME_MAX = 255;
/** 商品圖片上限（L3049）。 */
const IMAGE_MAX = 10;

/** 表單上一個規格的可編輯欄位（型別與合流規則同住 `./conflict`）。 */
type SkuDraft = DelegatedSkuDraft;

export default function DelegatedProductEditPage({
	productId,
}: {
	productId: string;
}) {
	const scope = useSellerDataScope();
	const { summary, hasCapability } = useDelegatedSession();
	const storeId = scope.ownerStoreId ?? summary?.target.storeId ?? "";
	// 確認抽屜的提示卡要指名店家（§6.5）。情境列是畫面上唯一寫著店名的地方，而它
	// 不 sticky —— 捲到下半段再開抽屜時，這一句就是操作者確認「是不是這一家」的
	// 唯一憑據。
	const targetStore = targetStoreLabel(summary?.target.storeName, storeId);
	const repo = React.useMemo(
		() => createDelegatedProductEditRepo(storeId),
		[storeId],
	);
	const canWrite = hasCapability("products.write");
	const canPublish = hasCapability("products.publish");
	const productsHref = `/adm/${encodeURIComponent(storeId)}/products`;

	const [detail, setDetail] = React.useState<DelegatedProductDetail | null>(
		null,
	);
	const [loading, setLoading] = React.useState(true);
	const [error, setError] = React.useState<string | null>(null);
	/** 首次載入（或重試）失敗的分類。與 `error` 分開：那一份是給表單內的錯誤列用的。 */
	const [loadFailure, setLoadFailure] =
		React.useState<DelegatedLoadFailureKind | null>(null);
	const [conflict, setConflict] = React.useState(false);
	const [submitting, setSubmitting] = React.useState(false);
	const [uploading, setUploading] = React.useState(false);

	const [name, setName] = React.useState("");
	const [categoryId, setCategoryId] = React.useState("");
	const [origin, setOrigin] = React.useState("");
	const [description, setDescription] = React.useState<ProductDescription>({
		version: "1.0",
		blocks: [],
	});
	const [images, setImages] = React.useState<ImageDraft[]>([]);
	const [skuDrafts, setSkuDrafts] = React.useState<Record<string, SkuDraft>>(
		{},
	);
	const fileInputRef = React.useRef<HTMLInputElement>(null);

	/**
	 * 分類樹：匿名端點（代行的瀏覽器在 www 上沒有登入態，`/app/categories/tree` 恆
	 * 401 ⇒ 下拉恆空），快取鍵與 `/app` 那份分開。與建立頁共用同一格。
	 */
	const categoryQuery = useQuery({
		queryKey: ["categories", "tree", "public"],
		queryFn: fetchPublicCategoryTree,
		staleTime: 60 * 60 * 1000,
	});
	const tree: CategoryNode[] = React.useMemo(
		() => categoryQuery.data ?? [],
		[categoryQuery.data],
	);
	const [path, setPath] = React.useState<[string, string, string]>([
		"",
		"",
		"",
	]);

	/**
	 * 把一份 detail 放進表單。
	 *
	 * `draft` 給的是「表單要顯示什麼」：預設是這份 detail 自己（＝重新載入最新版本），
	 * 「複製我的內容」則傳一份合流過的 draft 進來。**圖片一律取 detail**——保留既有
	 * 圖的憑據 `assetId` 必須來自這一次 GET（見 `conflict.ts` 規則 3）。
	 */
	const hydrate = React.useCallback(
		(next: DelegatedProductDetail, draft?: DelegatedProductFormDraft) => {
			const view = draft ?? draftFromDetail(next);
			setDetail(next);
			setName(view.name);
			setCategoryId(view.categoryId);
			setOrigin(view.origin);
			setDescription(view.description);
			setImages(imageDraftsOf(next));
			setSkuDrafts(view.skus);
			setConflict(false);
		},
		[],
	);

	/**
	 * 三個下拉的選取路徑由 `categoryId` ＋ 分類樹回推。
	 *
	 * ⚠️ 這一段的相依是 `categoryId` 而不是 `detail`：分類樹通常比商品晚到，只在
	 * hydrate 當下算一次的話，樹回來時下拉仍是三個空框，而 `categoryId` 已經有值 ——
	 * 畫面看起來「沒有分類」，但送出時它並沒有改變，操作者無從察覺。
	 *
	 * ⚠️ 只在**回推得到 L1** 時才覆寫，否則會踩掉正在進行中的選取：選完第一層時
	 * `categoryId` 被清成空字串（還沒選到葉節點），此時無條件寫回等於把剛選的那一層
	 * 也一起清掉，下拉會在使用者眼前跳回空白。
	 */
	React.useEffect(() => {
		const found = findCategoryPath(tree, categoryId);
		if (found[0]) setPath(found);
	}, [tree, categoryId]);

	const l2Options =
		tree.find((node) => String(node.id) === path[0])?.children ?? [];
	const l3Options =
		l2Options.find((node) => String(node.id) === path[1])?.children ?? [];

	const load = React.useCallback(async () => {
		setLoading(true);
		setError(null);
		setLoadFailure(null);
		try {
			hydrate(await repo.detail(productId));
		} catch (e) {
			// 讀取失敗要留下的是**業務碼分類**，不是那句組好的字串：「找不到／不屬於
			// 本店」與「上游暫時掛掉」對操作者是兩個完全不同的下一步，而 message 把
			// 它們攤平成同一句話。
			setLoadFailure(classifyDelegatedLoadFailure(e));
			setError(e instanceof Error ? e.message : "載入商品失敗");
		} finally {
			setLoading(false);
		}
	}, [hydrate, productId, repo]);

	/**
	 * 版本衝突的出口②：**複製我的內容**（契約要求的兩個出口之一，L5346）。
	 *
	 * 重抓最新版本當基底（`expectedVersion` 因此是新的），但把操作者剛才打的字留著；
	 * 差異會出現在下一次「檢視變更並儲存」的三欄表裡，由他自己判斷要不要蓋過去。
	 * ⚠️ 它**不會**直接送出 —— 直接送就是「用我的內容覆寫賣家較新的修改」，正是這條
	 * 契約要防的事。
	 */
	const keepMine = React.useCallback(async () => {
		setError(null);
		try {
			const latest = await repo.detail(productId);
			hydrate(
				latest,
				keepMyEdits(latest, {
					name,
					categoryId,
					origin,
					description,
					skus: skuDrafts,
				}),
			);
		} catch (e) {
			setError(e instanceof Error ? e.message : "載入商品失敗");
		}
	}, [
		categoryId,
		description,
		hydrate,
		name,
		origin,
		productId,
		repo,
		skuDrafts,
	]);

	React.useEffect(() => {
		void load();
	}, [load]);

	const form = React.useMemo(
		() => ({
			name,
			categoryId,
			origin,
			description,
			images,
			skus: skuDrafts,
		}),
		[categoryId, description, images, name, origin, skuDrafts],
	);

	/** 只送真的改了的欄位：未送的欄位由 Gateway 以 before 快照回填（見 repo.ts）。 */
	const patch = React.useMemo(
		() => (detail ? buildProductPatch(detail, form) : {}),
		[detail, form],
	);

	const dirty = Object.keys(patch).length > 0;

	/** 分類 id → 可讀名稱。三欄表印的是名字，不是那串數字。 */
	const categoryNameOf = React.useCallback(
		(id: string) => {
			for (const l1 of tree) {
				for (const l2 of l1.children ?? []) {
					for (const l3 of l2.children ?? []) {
						if (String(l3.id) === id) return l3.name;
					}
				}
			}
			return "";
		},
		[tree],
	);

	const changes = React.useMemo(
		() =>
			detail ? buildProductChanges(detail, form, patch, categoryNameOf) : [],
		[categoryNameOf, detail, form, patch],
	);

	const handleFailure = React.useCallback((e: unknown) => {
		// 9266 是版本衝突：賣家（或另一個管理員）在這段期間改過同一個商品。它與一般
		// 欄位驗證要分開呈現，否則操作者會以為「再按一次」就好，而再按一次仍然是 9266。
		if (e instanceof ApiClientError && e.code === 9266) {
			setConflict(true);
			setError(null);
			return;
		}
		setError(e instanceof Error ? e.message : "操作失敗");
	}, []);

	const handleSave = async () => {
		if (!detail || !dirty) return;
		const decision = await scope.confirmMutation({
			title: "檢視變更並儲存",
			intent: `以 ADM 身分替「${targetStore}」編輯「${detail.name || "未命名商品"}」`,
			// 動到認證標章時多一句法規宣稱的提醒（§4.1 高風險確認要求的第二道）。
			hint: "請確認以下變更內容無誤後儲存。",
			reasonPlaceholder: "例：協助修正商品說明與售價",
			changes,
			expectedVersion: detail.version,
			confirmLabel: "確認儲存",
		});
		if (!decision.ok || !decision.meta) return;
		setSubmitting(true);
		setError(null);
		try {
			await repo.update(productId, patch, {
				...decision.meta,
				expectedVersion: detail.version,
			} satisfies DelegatedMutationMeta);
			// 版本在寫入後一定變了：不重抓就會讓下一次送出必然撞 9266。
			hydrate(await repo.detail(productId));
		} catch (e) {
			handleFailure(e);
		} finally {
			setSubmitting(false);
		}
	};

	const handleShelf = async (target: "4003" | "4004") => {
		if (!detail) return;
		const publish = target === "4004";
		const decision = await scope.confirmMutation({
			title: publish ? "檢視變更並上架" : "檢視變更並下架",
			intent: `以 ADM 身分替「${targetStore}」${publish ? "上架" : "下架"}「${
				detail.name || "未命名商品"
			}」`,
			// `/publish`／`/unpublish` 的 body 只有共通三欄，變更恆為 `status` 單一欄位，
			// 所以這張表只有一列 —— 但 §4.1 對 `ADMIN_PRODUCT_PUBLISH` 的確認要求是
			// 「diff＋二次確認＋通知」，一列不等於不畫。庫存不在這裡：那是 PATCH，另一支
			// 端點、另一個 capability、另一筆稽核列（B5）。
			changes: [shelfStatusChange(detail, target)],
			hint: `這是與內容修改分開的一次寫入，會另外產生一筆代行紀錄。`,
			reasonPlaceholder: publish
				? "例：協助完成新品上架"
				: "例：依賣家來信暫停販售",
			expectedVersion: detail.version,
			confirmLabel: publish ? "確認上架" : "確認下架",
		});
		if (!decision.ok || !decision.meta) return;
		setSubmitting(true);
		setError(null);
		try {
			await scope.repos.products.updateProductStatus(productId, target, {
				...decision.meta,
				expectedVersion: detail.version,
			} satisfies DelegatedMutationMeta);
			hydrate(await repo.detail(productId));
		} catch (e) {
			handleFailure(e);
		} finally {
			setSubmitting(false);
		}
	};

	const handlePickImages = async (files: File[]) => {
		if (files.length === 0) return;
		const room = IMAGE_MAX - images.length;
		if (room <= 0) {
			setError(`商品圖片最多 ${IMAGE_MAX} 張。`);
			return;
		}
		setUploading(true);
		setError(null);
		try {
			const picked = files.slice(0, room);
			for (const file of picked) {
				const asset = await scope.repos.media.uploadImageFile(file);
				setImages((prev) => [
					...prev,
					{
						key: asset.stagedToken,
						stagedToken: asset.stagedToken,
						previewUrl: asset.previewUrl,
					},
				]);
			}
		} catch (e) {
			setError(e instanceof Error ? e.message : "圖片上傳失敗");
		} finally {
			setUploading(false);
		}
	};

	if (loading) {
		return (
			<section className="adm-panel" aria-busy="true">
				<p className="adm-panel__hint" role="status">
					正在載入商品…
				</p>
			</section>
		);
	}

	if (!detail) {
		// capability 不足與「讀不到這筆」是兩件事：前者整個面都不該出現，後者只是這一
		// 筆。共用一個畫面會讓操作者以為店裡沒有這個商品。
		if (loadFailure === "denied") {
			return (
				<DelegatedAreaUnauthorized
					reason="本次工作階段沒有核發商品讀取權限，因此看不到這個商品的內容。需要處理請返回 ADM 重新建立工作階段。"
					backHref={productsHref}
					backLabel="返回商品管理"
				/>
			);
		}
		const failure = delegatedProductLoadFailureCopy(loadFailure ?? "temporary");
		return (
			<section className="adm-panel">
				<h1 className="adm-panel__title">{failure.title}</h1>
				{/* 這裡刻意**不印** `error`：那句話是 `代行請求失敗（HTTP 502／代碼
				    UPSTREAM_FAILURE）`，對操作者既不可行動、也不穩定（9280 正在從 502
				    改成 424）。要診斷的人看 log 的 traceId，不是看畫面。 */}
				<p className="adm-panel__hint">{failure.hint}</p>
				{failure.retryable ? (
					<button
						type="button"
						className="adm-blocked__link cursor-pointer border-0"
						onClick={() => void load()}
					>
						重新載入
					</button>
				) : null}
				<Link
					href={productsHref}
					// 上面那顆「重新載入」是 `<button>`，dark 的 `body a:not(…)` 不會打到它；
					// 這顆是 `<a>`，少了 `data-hkui-button` 就只有它的白字被換成 `--color-ink-200`，
					// 於是同一排兩顆同款鈕字色不同。
					data-hkui-button=""
					className="adm-blocked__link"
				>
					返回商品管理
				</Link>
			</section>
		);
	}

	const shelfState = resolveShelfState(detail.status);
	/**
	 * 上架中的商品**內容不可編輯**：product-service 對 4004 的 PATCH 一律回
	 * `3019 商品需先下架後再編輯`（本機實打坐實：`PATCH .../products/9/10011` →
	 * `{"code":3019}` / HTTP 409）。這是賣家自助端也有的規則（列表的編輯鈕在
	 * 上架中會變成鎖頭），代行沒有例外 —— 讓表單看起來可以改、按下去才收到一個
	 * 沒有 message 的 409，等於把後端規則藏起來。
	 */
	const contentLocked = shelfState === "active";
	const editable = canWrite && !contentLocked;
	const statusLabel = detail.statusLabel || PRODUCT_STATUS_LABEL[detail.status];

	return (
		<>
			<SellerPageShell
				className="adm-product-edit"
				title="編輯商品"
				subtitle={`商品管理 / ${detail.name || "未命名商品"}`}
				actions={
					<>
						{/* 買家視角預覽（56 號 §2.2 第 13 列）。
						    ⚠️ 刻意**不走 `OperationConfirmDrawer`**：那支關卡強制收
						    `operationReason`（每一次代行寫入的必填欄），而預覽是唯讀、不
						    產生稽核列 —— 為一次唯讀動作要求填寫修改原因，會讓那一欄在操作者
						    眼中從「這次改了什麼」退化成一道形式手續，代價落在真正的寫入上。
						    ⚠️ 也不新開分頁：預覽在 `/adm` 之下，同分頁返回才留得住工作階段
						    的上下文（右上角的情境列與剩餘時間）。 */}
						<Link
							href={`${productsHref}/${encodeURIComponent(productId)}/preview`}
							className="seller-orders__btn"
						>
							<AppIcon name="preview" size={16} />
							買家視角預覽
						</Link>
						<ShelfStatusControl
							status={detail.status}
							statusLabel={statusLabel}
						/>
					</>
				}
			>
				{conflict ? (
					<div className="adm-conflict" role="alert">
						<div className="adm-conflict__text">
							<strong>這個商品已被更新（版本衝突）</strong>
							<span>
								{
									"賣家本人或另一位管理員在這段期間改過它，系統不會覆寫較新的內容。你可以整份改用最新版本，或保留剛才填的內容、以最新版本為基底再檢視一次差異。"
								}
							</span>
						</div>
						{/* 契約明列的兩個出口（L5346）。只給「重新載入」等於要求操作者把剛
						    才打的字再打一次；只給「複製我的內容」則會變成覆寫賣家的修改 ——
						    兩顆都在，才是「不覆寫較新的內容」＋「不必白打一次」。 */}
						<div className="adm-conflict__actions">
							<Button
								type="button"
								variant="bordered"
								className="adm-quiet-button"
								onClick={() => void load()}
								disabled={submitting}
							>
								重新載入最新版本
							</Button>
							<Button
								type="button"
								color="primary"
								onClick={() => void keepMine()}
								disabled={submitting}
							>
								複製我的內容
							</Button>
						</div>
					</div>
				) : null}

				{contentLocked ? (
					<div className="adm-product-edit__locked" role="note">
						<AppIcon name="lock" size={16} />
						{/* 全形逗號後面原本還打了一個半形空白（源碼裡的字，不是換行折出來的，
						    所以 check-jsx-cjk-linebreak 看不到它）。順手拿掉，其餘空白照舊：
						    「規則 3019」與破折號兩側的空白都是刻意的。 */}
						<span>
							{
								"上架中的商品內容不可編輯（後端規則 3019）。請先按「檢視變更並下架」，修改後再上架 —— 上下架與內容修改本來就是分開的兩次寫入。"
							}
						</span>
					</div>
				) : null}

				{error ? (
					<p className="adm-product-edit__error" role="alert">
						{error}
					</p>
				) : null}

				<div className="adm-product-edit__grid">
					<div className="adm-product-edit__col">
						<section className="seller-card adm-product-edit__card">
							<header className="adm-product-edit__card-head">
								<h2>基本資訊</h2>
								<p>買家最先看到的商品名稱與描述。</p>
							</header>
							<Input
								variant="bordered"
								size="lg"
								label="商品名稱"
								isRequired
								value={name}
								onChange={(e) => setName(e.target.value.slice(0, NAME_MAX))}
								maxLength={NAME_MAX}
								disabled={!editable}
								description={`${name.length} / ${NAME_MAX} 字`}
							/>
							<div className="adm-product-edit__field">
								<span className="adm-product-edit__label">商品描述</span>
								<DescriptionEditor
									value={description}
									onChange={setDescription}
								/>
							</div>
						</section>

						<section className="seller-card adm-product-edit__card">
							<header className="adm-product-edit__card-head">
								<h2>商品圖片</h2>
								<p>
									最多 {IMAGE_MAX} 張，第一張是商品卡的封面。移除後儲存即生效。
								</p>
							</header>
							<ul className="adm-product-edit__images">
								{images.map((image) => (
									<li key={image.key}>
										{/* biome-ignore lint/performance/noImgElement: 代行的圖片來源是 CDN 上的動態 objectKey。 */}
										<img src={image.previewUrl} alt="" loading="lazy" />
										{editable ? (
											<button
												type="button"
												aria-label="移除這張圖片"
												onClick={() =>
													setImages((prev) =>
														prev.filter((item) => item.key !== image.key),
													)
												}
											>
												<AppIcon name="delete" size={16} />
											</button>
										) : null}
									</li>
								))}
								{editable && images.length < IMAGE_MAX ? (
									<li className="adm-product-edit__image-add">
										{/* 走共用 Button 的 loading 契約：原生 button 自己畫「上傳中…」是
									    `button-loading-contract` 明令的違規（那個文案的唯一合法出處是
									    共用元件的 `loadingLabel`）。 */}
										<Button
											type="button"
											variant="light"
											className="adm-quiet-button"
											onClick={() => fileInputRef.current?.click()}
											disabled={uploading}
											isLoading={uploading}
											loadingLabel="上傳圖片"
										>
											<AppIcon name="image" size={22} />
											<span>新增圖片</span>
										</Button>
									</li>
								) : null}
							</ul>
							<input
								ref={fileInputRef}
								type="file"
								accept="image/*"
								multiple
								className="hidden"
								onChange={(e) => {
									void handlePickImages(takePickedFiles(e.target));
								}}
							/>
						</section>

						<section className="seller-card adm-product-edit__card">
							<header className="adm-product-edit__card-head">
								<h2>分類與產地</h2>
								<p>
									分類需選到最後一層。代行工作階段只能選既有分類，不能新增分類。
								</p>
							</header>
							<div className="adm-product-create__categories">
								<Dropdown
									label="商品分類"
									placeholder="選擇分類"
									variant="bordered"
									size="lg"
									floatingLabel
									searchable
									noResultsLabel="找不到符合的分類"
									value={path[0] || undefined}
									onChange={(id: string) => {
										// 上層一改，下層的既有選擇就不再成立；`categoryId` 一併清空，
										// 避免「看起來已改分類、其實還送著舊的葉節點」。
										setCategoryId("");
										setPath([id, "", ""]);
									}}
									items={categoryDropdownItems(tree)}
									disabled={!editable}
								/>
								<Dropdown
									label="子分類"
									placeholder="選擇子分類"
									variant="bordered"
									size="lg"
									floatingLabel
									searchable
									noResultsLabel="找不到符合的子分類"
									value={path[1] || undefined}
									onChange={(id: string) => {
										setCategoryId("");
										setPath([path[0], id, ""]);
									}}
									items={categoryDropdownItems(l2Options)}
									disabled={!editable || !path[0] || l2Options.length === 0}
								/>
								<Dropdown
									label="細分類"
									placeholder="選擇細分類"
									variant="bordered"
									size="lg"
									floatingLabel
									searchable
									noResultsLabel="找不到符合的細分類"
									value={path[2] || undefined}
									onChange={(id: string) => {
										setCategoryId(id);
										setPath([path[0], path[1], id]);
									}}
									items={categoryDropdownItems(l3Options)}
									disabled={!editable || !path[1] || l3Options.length === 0}
								/>
							</div>
							{categoryQuery.isError ? (
								// 樹載不到時分類不可改，但要說清楚「目前是什麼、不動就不會被送出」——
								// 三個空下拉單獨出現，看起來像分類被清掉了。
								<p className="adm-product-edit__error" role="alert">
									分類暫時無法載入，請稍後再試。目前分類為「
									{detail.categoryName || detail.categoryId || "未設定"}
									」，未改動時不會送出。
								</p>
							) : null}
							{path[0] !== "" && categoryId === "" ? (
								// 選到一半的路徑不會送出任何 categoryId（空值是 9260）。不說的話
								// 畫面顯示新分類、送出後分類沒變，而三欄表也不會有這一列。
								<p className="adm-product-edit__note" role="status">
									分類尚未選到最後一層，本次不會變更商品分類。
								</p>
							) : null}
							<Input
								variant="bordered"
								size="lg"
								label="產地"
								value={origin}
								onChange={(e) => setOrigin(e.target.value)}
								maxLength={100}
								disabled={!editable}
								placeholder="例：台灣 梨山"
							/>
						</section>
					</div>

					<div className="adm-product-edit__col adm-product-edit__col--side">
						<section className="seller-card adm-product-edit__card">
							<header className="adm-product-edit__card-head">
								<h2>售價與庫存</h2>
								<p>庫存會影響買家可購買的數量。</p>
							</header>
							{detail.skus.map((sku) => {
								const draft = skuDrafts[sku.skuId];
								if (!draft) return null;
								const patchSku = (next: Partial<SkuDraft>) =>
									setSkuDrafts((prev) => ({
										...prev,
										[sku.skuId]: { ...prev[sku.skuId], ...next },
									}));
								return (
									<div key={sku.skuId} className="adm-product-edit__sku">
										<Input
											variant="bordered"
											size="lg"
											label="規格名稱"
											value={draft.spec}
											onChange={(e) => patchSku({ spec: e.target.value })}
											maxLength={100}
											disabled={!editable}
										/>
										<Input
											variant="bordered"
											size="lg"
											label="售價"
											inputMode="numeric"
											value={draft.price}
											onChange={(e) => patchSku({ price: e.target.value })}
											disabled={!editable}
											startContent={
												<span className="adm-product-edit__affix">NT$</span>
											}
										/>
										<Input
											variant="bordered"
											size="lg"
											label="特價"
											inputMode="numeric"
											value={draft.salePrice}
											onChange={(e) => patchSku({ salePrice: e.target.value })}
											disabled={!editable}
											description="留空代表沒有進行中的特價"
											startContent={
												<span className="adm-product-edit__affix">NT$</span>
											}
										/>
										<Input
											variant="bordered"
											size="lg"
											label="庫存數量"
											inputMode="numeric"
											value={draft.stockQty}
											onChange={(e) => patchSku({ stockQty: e.target.value })}
											disabled={!editable}
											endContent={
												// D1：單位是 SKU 層的 CodeMeta 值（`WeightUnit`），不是可以
												// 自由填寫的中文字。這裡唯讀顯示它的 Label。
												<span className="adm-product-edit__affix">
													單位：{unitText(sku.unit)}
												</span>
											}
										/>
									</div>
								);
							})}
						</section>

						<section className="seller-card adm-product-edit__card">
							<header className="adm-product-edit__card-head">
								<h2>規格與運送</h2>
								<p>淨重、尺寸與配送方式由賣家本人維護。</p>
							</header>
							{detail.skus.map((sku) => (
								<dl key={sku.skuId} className="adm-product-edit__facts">
									<div>
										<dt>{sku.spec || "規格"}</dt>
										<dd>
											淨重 {sku.weight || "—"}・{sku.lengthCm || "—"} ×{" "}
											{sku.widthCm || "—"} × {sku.heightCm || "—"} cm
										</dd>
									</div>
									<div>
										<dt>配送方式</dt>
										<dd>
											{sku.shippingMethods.length === 0
												? "—"
												: [
														...new Set(
															sku.shippingMethods.map(
																(method) =>
																	SHIPPING_METHOD_LABEL[method.type] ??
																	method.type,
															),
														),
													].join("、")}
										</dd>
									</div>
								</dl>
							))}
						</section>

						<section className="seller-card adm-product-edit__card">
							<header className="adm-product-edit__card-head">
								<h2>發布狀態</h2>
								<p>
									{shelfState === "active"
										? "此商品目前顯示在商店頁面。"
										: "此商品目前不會在商店頁面顯示。"}
								</p>
							</header>
							<div className="adm-product-edit__status">
								<ShelfStatusControl
									status={detail.status}
									statusLabel={statusLabel}
								/>
								{/* 動作按鈕在 capability 不足時**不渲染**（不是 disabled，L3857）。
								    上／下架是獨立的一次寫入，與上面的內容修改分開送（B5）。 */}
								{canPublish && shelfState !== "review" ? (
									<Button
										type="button"
										variant={shelfState === "active" ? "bordered" : undefined}
										disabled={submitting}
										onClick={() =>
											void handleShelf(
												shelfState === "active" ? "4003" : "4004",
											)
										}
									>
										{shelfState === "active"
											? "檢視變更並下架"
											: "檢視變更並上架"}
									</Button>
								) : null}
							</div>
							<p className="adm-product-edit__version">
								<span>目前版本</span>
								<strong>{detail.version}</strong>
							</p>
						</section>
					</div>
				</div>
			</SellerPageShell>

			<div className="adm-product-edit__bar" role="toolbar">
				<span className="adm-product-edit__bar-hint">
					<AppIcon name="shield" size={16} />
					操作行為仍為 ADM；所有寫入將綁定本次 Operation Session
				</span>
				<span className="adm-product-edit__bar-actions">
					<Button
						href={productsHref}
						variant="bordered"
						className="adm-quiet-button"
						disabled={submitting}
					>
						返回
					</Button>
					{canWrite ? (
						<Button
							type="button"
							disabled={!dirty || submitting || uploading}
							isLoading={submitting}
							loadingLabel="正在寫入代行紀錄"
							onClick={() => void handleSave()}
						>
							檢視變更並儲存
						</Button>
					) : null}
				</span>
			</div>
		</>
	);
}
