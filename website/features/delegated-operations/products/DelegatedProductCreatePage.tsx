"use client";

import { useQuery } from "@tanstack/react-query";
import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import { Dropdown } from "@/components/ui";
import Button from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import DescriptionEditor from "@/features/seller/components/DescriptionEditor";
import type { DelegatedMutationMeta } from "@/lib/api/delegated";
import { WEIGHT_UNIT_LABEL } from "@/lib/codemeta/enums";
import { takePickedFiles } from "@/lib/media/pick-files";
import { useObservedRouter } from "@/lib/navigation/use-observed-router";
import {
	type CategoryNode,
	categoryDropdownItems,
	fetchPublicCategoryTree,
	type ProductDescription,
	type ProductWriteBody,
	type SkuWriteInput,
} from "@/lib/products/seller-repo";
import {
	type SellerMutationChange,
	useSellerDataScope,
} from "@/lib/seller-data/scope";
import DelegatedAreaUnauthorized from "../components/DelegatedAreaUnauthorized";
import { targetStoreLabel } from "../confirm-content";
import { useDelegatedSession } from "../state/DelegatedSessionContext";
import CertificationEditor from "./CertificationEditor";
import { certificationsSummary } from "./certifications";
import type { DelegatedProductCertification } from "./repo";

/**
 * 代行工作區・上架新商品（mockup §4.4.2 的 `＋ 上架新商品`／§6.1.8 `POST .../products`）。
 *
 * **為什麼不是 `features/seller-center/products/ProductFormPage.tsx` 的一個分支**：
 * 與編輯頁同一個判準（見 `DelegatedProductEditPage` 頂註）。額外兩點只在建立這一面
 * 成立：
 * - 那支表單的分類選擇器 `CategorySelector` 綁 `/app/categories/tree`（**authed**）
 *   與「順手新建分類」（`POST /app/categories`）—— 代行的瀏覽器在 www 上沒有登入
 *   態，前者恆 401（分類下拉恆空 ⇒ 商品建不出來），後者更是代行不該有的能力
 * - 它綁賣家自己的認證閘記（`useCertGate`）與 AI 上架助手，兩者在代行下都沒有資料來源
 *
 * **§3.4 三件必備在這一頁的處置**：
 * - ① 修改原因、③ 通知賣家 —— 由共用確認抽屜承擔（`scope.confirmMutation`）
 * - ② `expectedVersion`：**建立沒有目前版本**（新資源，create 的 DTO 連這個欄位都
 *   沒有，帶了就是 9260）。抽屜因此改為明說「不適用」而不是留白 —— 留白會讓人以為
 *   是漏畫的，而這一頁本來就是三件必備裡唯一合法缺一件的情境。
 *   ⚠️ 接著上架時**又需要版本**：`/publish` 的 `expectedVersion` 取自建立回應的
 *   `version`，那是它唯一的來源（重抓一次 detail 也拿得到，但重抓與建立之間的任何
 *   寫入都會讓上架打在別人的版本上）。
 *
 * **兩條路徑分開命名**（A-5 的教訓：mockup 把「儲存」畫成實際會發布的按鈕）：
 * - `儲存，暫不上架` → 一支呼叫、一筆稽核列，商品落 4003 未上架，買家看不到
 * - `儲存並上架` → 兩支呼叫、**兩筆稽核列**，需要 `products.publish`
 *
 * **「暫不上架」是主要按鈕**：建立與上架是兩個 capability，把上架畫成預設會讓一次
 * 誤點就對買家可見。反過來，只留一顆按鈕（無論是哪一顆）就是把選擇藏成預設 ——
 * 操作者無從得知自己剛才做了哪一種。沒有 `products.publish` 時上架那顆**不渲染**
 * （L3857），但底下會補一行說明本次工作階段沒有這個授權，否則畫面退回「只有一顆
 * 按鈕」，看起來又像沒有選擇。
 */

/** `product.name` 的真正上限是 `VARCHAR(255)`；mockup 上的「/ 60 字」是假的（D4）。 */
const NAME_MAX = 255;
/** 商品圖片上限（L3049）。 */
const IMAGE_MAX = 10;
/** 規格數上限（Gateway `validateProductSkus`）。 */
const SKU_MAX = 20;

type ImageDraft = { key: string; stagedToken: string; previewUrl: string };

interface SkuDraft {
	key: string;
	spec: string;
	price: string;
	salePrice: string;
	stockQty: string;
	unit: string;
	weight: string;
	lengthCm: string;
	widthCm: string;
	heightCm: string;
}

let skuSeq = 0;
function emptySku(): SkuDraft {
	skuSeq += 1;
	return {
		key: `sku-${skuSeq}`,
		spec: "",
		price: "",
		salePrice: "",
		stockQty: "0",
		// 預設公斤：`unit` 是 CodeMeta 的 WeightUnit **值**（3301／3302／3303），不是
		// 可自由填寫的中文字。髒值會一路過關到結帳鏈才由 `WeightUnit.fromValue()` 拋。
		unit: "3301",
		weight: "",
		lengthCm: "",
		widthCm: "",
		heightCm: "",
	};
}

const UNIT_ITEMS = Object.entries(WEIGHT_UNIT_LABEL).map(([value, label]) => ({
	value,
	label,
}));

function positive(value: string): boolean {
	const n = Number(value);
	return Number.isFinite(n) && n > 0;
}

/**
 * 建立前的前端驗證。
 *
 * 這裡不是「防呆」：product-service 逐條驗、**第一條失敗就 return**，所以少填四欄
 * 的表單會換來四次各只講一欄的 400（賣家自助端 #435 的原始症狀）。代行更糟一層 ——
 * 代行 BFF 不透傳 message，操作者只會看到「代行請求失敗（代碼 9260）」。
 */
function validate(
	name: string,
	categoryId: string,
	skus: SkuDraft[],
): string[] {
	const errors: string[] = [];
	if (name.trim() === "") errors.push("商品名稱必填");
	if (name.trim().length > NAME_MAX)
		errors.push(`商品名稱不得超過 ${NAME_MAX} 字`);
	if (categoryId === "") errors.push("請選擇商品分類（需選到最後一層）");
	if (skus.length === 0) errors.push("至少需要一個商品規格");
	skus.forEach((sku, index) => {
		const at = `規格 ${index + 1}`;
		if (sku.spec.trim() === "") errors.push(`${at}：規格名稱必填`);
		if (!positive(sku.price)) errors.push(`${at}：原價需大於 0`);
		if (sku.salePrice !== "" && !positive(sku.salePrice)) {
			errors.push(`${at}：售價需大於 0`);
		}
		if (
			sku.salePrice !== "" &&
			positive(sku.salePrice) &&
			positive(sku.price) &&
			Number(sku.salePrice) >= Number(sku.price)
		) {
			errors.push(`${at}：售價需小於原價`);
		}
		const stock = Number(sku.stockQty);
		if (!Number.isInteger(stock) || stock < 0) {
			errors.push(`${at}：庫存需為 0 或正整數`);
		}
		if (!positive(sku.weight)) errors.push(`${at}：重量需大於 0`);
		if (!positive(sku.lengthCm)) errors.push(`${at}：包裝長度需大於 0`);
		if (!positive(sku.widthCm)) errors.push(`${at}：包裝寬度需大於 0`);
		if (!positive(sku.heightCm)) errors.push(`${at}：包裝高度需大於 0`);
	});
	return errors;
}

function toWriteBody(
	name: string,
	categoryId: string,
	origin: string,
	description: ProductDescription,
	certifications: DelegatedProductCertification[],
	images: ImageDraft[],
	skus: SkuDraft[],
): ProductWriteBody {
	const skuInputs: SkuWriteInput[] = skus.map((sku) => ({
		skuName: sku.spec.trim(),
		price: sku.price.trim(),
		salePrice: sku.salePrice.trim(),
		stockQty: sku.stockQty.trim(),
		unit: sku.unit,
		weight: sku.weight.trim(),
		lengthCm: sku.lengthCm.trim(),
		widthCm: sku.widthCm.trim(),
		heightCm: sku.heightCm.trim(),
	}));
	return {
		name: name.trim(),
		categoryId,
		origin: origin.trim(),
		// 空文件也送：`blocks: []` 是合法的「沒有描述」，而不送它與送空是同一個結果。
		description,
		// `name` 是 CertificationType 碼值，不是中文名（見 `./certifications`）。
		certifications: certifications.map((cert) => ({
			name: cert.name,
			certNo: cert.certNo,
		})),
		images: images.map((image) => ({ stagedToken: image.stagedToken })),
		skus: skuInputs,
	};
}

export default function DelegatedProductCreatePage() {
	const router = useObservedRouter("delegated_product_create");
	const scope = useSellerDataScope();
	const { summary, hasCapability } = useDelegatedSession();
	const storeId = scope.ownerStoreId ?? summary?.target.storeId ?? "";
	// 確認抽屜的提示卡要指名店家（§6.5）。情境列是畫面上唯一寫著店名的地方，而它
	// 不 sticky —— 捲到下半段再開抽屜時，這一句就是操作者確認「是不是這一家」的
	// 唯一憑據。
	const targetStore = targetStoreLabel(summary?.target.storeName, storeId);
	const productsHref = `/adm/${encodeURIComponent(storeId)}/products`;
	const canWrite = hasCapability("products.write");
	const canPublish = hasCapability("products.publish");

	const [name, setName] = React.useState("");
	const [categoryId, setCategoryId] = React.useState("");
	const [origin, setOrigin] = React.useState("");
	const [description, setDescription] = React.useState<ProductDescription>({
		version: "1.0",
		blocks: [],
	});
	const [certifications, setCertifications] = React.useState<
		DelegatedProductCertification[]
	>([]);
	const [images, setImages] = React.useState<ImageDraft[]>([]);
	const [skus, setSkus] = React.useState<SkuDraft[]>(() => [emptySku()]);
	const [errors, setErrors] = React.useState<string[]>([]);
	const [error, setError] = React.useState<string | null>(null);
	/**
	 * 送出中的是哪一條路徑。用意圖而不是布林：兩顆按鈕各自帶 loading，布林會讓
	 * 「儲存並上架」的等待轉圈畫在「暫不上架」那顆上，操作者會以為自己按錯了。
	 */
	const [submittingIntent, setSubmittingIntent] = React.useState<
		null | "draft" | "publish"
	>(null);
	const submitting = submittingIntent !== null;
	const [uploading, setUploading] = React.useState(false);
	const fileInputRef = React.useRef<HTMLInputElement>(null);

	/** 分類樹：匿名端點（代行瀏覽器沒有登入態），快取鍵與 `/app` 那份分開。 */
	const categoryQuery = useQuery({
		queryKey: ["categories", "tree", "public"],
		queryFn: fetchPublicCategoryTree,
		staleTime: 60 * 60 * 1000,
	});
	const tree: CategoryNode[] = categoryQuery.data ?? [];
	const [path, setPath] = React.useState<[string, string, string]>([
		"",
		"",
		"",
	]);
	const l2Options =
		tree.find((node) => String(node.id) === path[0])?.children ?? [];
	const l3Options =
		l2Options.find((node) => String(node.id) === path[1])?.children ?? [];

	const patchSku = (key: string, next: Partial<SkuDraft>) =>
		setSkus((prev) =>
			prev.map((sku) => (sku.key === key ? { ...sku, ...next } : sku)),
		);

	/** 送出前的三欄表（B20）。建立沒有 before，`before` 一律是「（新增）」。 */
	const changes = React.useMemo<SellerMutationChange[]>(() => {
		const rows: SellerMutationChange[] = [
			{ field: "商品名稱", before: "（新增）", after: name.trim() },
			{
				field: "商品分類",
				before: "（新增）",
				after:
					l3Options.find((node) => String(node.id) === path[2])?.name ??
					categoryId,
			},
			{ field: "商品規格", before: "（新增）", after: `${skus.length} 個` },
			{ field: "商品圖片", before: "（新增）", after: `${images.length} 張` },
		];
		if (origin.trim() !== "") {
			rows.push({ field: "產地", before: "（新增）", after: origin.trim() });
		}
		if (description.blocks.length > 0) {
			rows.push({
				field: "商品描述",
				before: "（新增）",
				after: `${description.blocks.length} 個區塊`,
			});
		}
		// 認證是法規宣稱：只要掛了就逐項列出（不摺成「N 個」），操作者要看到的是
		// 自己替賣家宣稱了什麼、編號是多少。
		if (certifications.length > 0) {
			rows.push({
				field: "認證標章",
				before: "（新增）",
				after: certificationsSummary(certifications),
			});
		}
		return rows;
	}, [
		categoryId,
		certifications,
		description.blocks.length,
		images.length,
		l3Options,
		name,
		origin,
		path,
		skus.length,
	]);

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
			for (const file of files.slice(0, room)) {
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

	const submit = async (publish: boolean) => {
		const found = validate(name, categoryId, skus);
		setErrors(found);
		if (found.length > 0) return;

		const decision = await scope.confirmMutation({
			title: publish ? "檢視內容並建立後上架" : "檢視內容並建立（暫不上架）",
			intent: `以 ADM 身分替「${targetStore}」建立商品「${name.trim() || "未命名商品"}」`,
			hint: `${
				publish
					? "建立與上架是兩次寫入：先建立未上架的商品，再以建立回應的版本執行上架。"
					: "建立後為未上架，買家看不到；賣家可自行上架，或由代行者另外執行一次上架。"
			}${
				certifications.length > 0
					? "本次會掛上認證標章 —— 那是賣家對買家的法規宣稱，錯誤由賣家承擔責任，請再次核對類型與證書編號。"
					: ""
			}`,
			reasonPlaceholder: "例：協助賣家補上季節新品",
			changes,
			// ② 的處置：新資源沒有版本，明說而不是留白。
			versionNote: "不適用（新建商品尚未有版本）",
			auditNote: publish
				? "將產生兩筆不可竄改的代行紀錄（建立、上架）"
				: "將產生不可竄改的代行紀錄",
			confirmLabel: publish ? "確認建立並上架" : "確認建立，暫不上架",
		});
		if (!decision.ok || !decision.meta) return;

		setSubmittingIntent(publish ? "publish" : "draft");
		setError(null);
		try {
			const created = await scope.repos.products.createProduct(
				toWriteBody(
					name,
					categoryId,
					origin,
					description,
					certifications,
					images,
					skus,
				),
				decision.meta,
			);
			const productId = created.id == null ? "" : String(created.id);
			if (publish) {
				if (productId === "" || !created.version) {
					// 建立成功但拿不到 id／version：不猜、不重抓後盲目上架 —— 那會讓上架
					// 打在一個我們沒有讀過的版本上。停在這裡，商品已是草稿、資料沒壞。
					setError(
						"商品已建立（未上架），但建立回應沒有版本資訊，無法接著上架。請到商品列表開啟它再執行上架。",
					);
					setSubmittingIntent(null);
					return;
				}
				await scope.repos.products.updateProductStatus(productId, "4004", {
					...decision.meta,
					expectedVersion: created.version,
				} satisfies DelegatedMutationMeta);
			}
			router.push(productsHref);
		} catch (e) {
			setError(e instanceof Error ? e.message : "建立商品失敗");
			setSubmittingIntent(null);
		}
	};

	if (!canWrite) {
		// 入口在 capability 不足時就不渲染，走到這裡只可能是直接輸入網址。
		return (
			<DelegatedAreaUnauthorized
				reason="本次代行工作階段沒有「商品編輯」能力，無法建立商品。"
				backHref={productsHref}
				backLabel="返回商品管理"
			/>
		);
	}

	return (
		<>
			<SellerPageShell
				className="adm-product-edit"
				title="上架新商品"
				subtitle="商品管理 / 新增商品"
			>
				{errors.length > 0 ? (
					<ul className="adm-product-edit__errors" role="alert">
						{errors.map((message) => (
							<li key={message}>{message}</li>
						))}
					</ul>
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
								<p>最多 {IMAGE_MAX} 張，第一張是商品卡的封面。</p>
							</header>
							<ul className="adm-product-edit__images">
								{images.map((image) => (
									<li key={image.key}>
										{/* biome-ignore lint/performance/noImgElement: 代行的圖片來源是 CDN 上的動態 objectKey。 */}
										<img src={image.previewUrl} alt="" loading="lazy" />
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
									</li>
								))}
								{images.length < IMAGE_MAX ? (
									<li className="adm-product-edit__image-add">
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
								<h2>分類與認證</h2>
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
										setCategoryId("");
										setPath([id, "", ""]);
									}}
									items={categoryDropdownItems(tree)}
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
									disabled={!path[0] || l2Options.length === 0}
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
									disabled={!path[1] || l3Options.length === 0}
								/>
							</div>
							{categoryQuery.isError ? (
								<p className="adm-product-edit__error" role="alert">
									分類暫時無法載入，請稍後再試。
								</p>
							) : null}
							<div className="adm-product-edit__field">
								<span className="adm-product-edit__label">認證標章</span>
								<CertificationEditor
									value={certifications}
									onChange={setCertifications}
								/>
							</div>
							<Input
								variant="bordered"
								size="lg"
								label="產地"
								value={origin}
								onChange={(e) => setOrigin(e.target.value)}
								maxLength={100}
								placeholder="例：台灣 梨山"
							/>
						</section>
					</div>

					<div className="adm-product-edit__col adm-product-edit__col--side">
						<section className="seller-card adm-product-edit__card">
							<header className="adm-product-edit__card-head">
								<h2>規格、售價與運送</h2>
								<p>重量與長寬高必須大於 0，缺一都建不起來。</p>
							</header>
							{skus.map((sku, index) => (
								<div key={sku.key} className="adm-product-edit__sku">
									<div className="adm-product-create__sku-head">
										<strong>規格 {index + 1}</strong>
										{skus.length > 1 ? (
											<Button
												type="button"
												variant="light"
												size="sm"
												className="adm-quiet-button"
												onClick={() =>
													setSkus((prev) =>
														prev.filter((item) => item.key !== sku.key),
													)
												}
											>
												移除
											</Button>
										) : null}
									</div>
									<Input
										variant="bordered"
										size="lg"
										label="規格名稱"
										isRequired
										value={sku.spec}
										onChange={(e) =>
											patchSku(sku.key, { spec: e.target.value })
										}
										maxLength={100}
									/>
									<Input
										variant="bordered"
										size="lg"
										label="原價"
										isRequired
										inputMode="numeric"
										value={sku.price}
										onChange={(e) =>
											patchSku(sku.key, { price: e.target.value })
										}
										startContent={
											<span className="adm-product-edit__affix">NT$</span>
										}
									/>
									<Input
										variant="bordered"
										size="lg"
										label="售價"
										inputMode="numeric"
										value={sku.salePrice}
										onChange={(e) =>
											patchSku(sku.key, { salePrice: e.target.value })
										}
										description="留空代表沒有進行中的特價"
										startContent={
											<span className="adm-product-edit__affix">NT$</span>
										}
									/>
									<Input
										variant="bordered"
										size="lg"
										label="庫存數量"
										isRequired
										inputMode="numeric"
										value={sku.stockQty}
										onChange={(e) =>
											patchSku(sku.key, { stockQty: e.target.value })
										}
									/>
									<Dropdown
										label="單位"
										variant="bordered"
										size="lg"
										floatingLabel
										value={sku.unit}
										onChange={(value: string) =>
											patchSku(sku.key, { unit: value })
										}
										items={UNIT_ITEMS}
									/>
									<Input
										variant="bordered"
										size="lg"
										label="淨重"
										isRequired
										inputMode="numeric"
										value={sku.weight}
										onChange={(e) =>
											patchSku(sku.key, { weight: e.target.value })
										}
									/>
									<Input
										variant="bordered"
										size="lg"
										label="包裝長度（cm）"
										isRequired
										inputMode="numeric"
										value={sku.lengthCm}
										onChange={(e) =>
											patchSku(sku.key, { lengthCm: e.target.value })
										}
									/>
									<Input
										variant="bordered"
										size="lg"
										label="包裝寬度（cm）"
										isRequired
										inputMode="numeric"
										value={sku.widthCm}
										onChange={(e) =>
											patchSku(sku.key, { widthCm: e.target.value })
										}
									/>
									<Input
										variant="bordered"
										size="lg"
										label="包裝高度（cm）"
										isRequired
										inputMode="numeric"
										value={sku.heightCm}
										onChange={(e) =>
											patchSku(sku.key, { heightCm: e.target.value })
										}
									/>
								</div>
							))}
							{skus.length < SKU_MAX ? (
								<Button
									type="button"
									variant="bordered"
									className="adm-quiet-button"
									onClick={() => setSkus((prev) => [...prev, emptySku()])}
								>
									新增規格
								</Button>
							) : null}
						</section>

						<section className="seller-card adm-product-edit__card">
							<header className="adm-product-edit__card-head">
								<h2>發布狀態</h2>
								<p>
									由底部的兩顆按鈕決定：
									<strong>儲存，暫不上架</strong>
									建立後買家看不到，賣家可以自己上架；
									<strong>儲存並上架</strong>
									會在建立之後再執行一次上架，兩次寫入各留一筆代行紀錄。
								</p>
							</header>
							<p className="adm-product-edit__version">
								<span>目前版本</span>
								<strong>不適用（新建商品）</strong>
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
					{/* 上架能力不足時，那顆按鈕**不渲染**（不是 disabled，L3857）：這個工作
					    階段從頭到尾都不能上架，一顆停用的按鈕仍在宣稱「這件事本來做得到」。
					    但只是拿掉它會讓畫面退回「只有一顆按鈕」＝看起來沒有選擇，於是這裡
					    補一句說明：不是操作者沒看到，是這次沒有這個授權。 */}
					{canPublish ? null : (
						<span className="adm-product-edit__bar-note">
							本次工作階段沒有「商品上架」授權，只能建立未上架的商品；要讓買家看到，請由賣家自行上架。
						</span>
					)}
					<Button
						href={productsHref}
						variant="bordered"
						className="adm-quiet-button"
						disabled={submitting}
					>
						返回
					</Button>
					{/* 主要按鈕是「暫不上架」：建立與上架是兩個 capability，把上架畫成預設
					    會讓一次誤點就對買家可見。 */}
					<Button
						type="button"
						disabled={submitting || uploading}
						isLoading={submittingIntent === "draft"}
						loadingLabel="正在寫入代行紀錄"
						onClick={() => void submit(false)}
					>
						儲存，暫不上架
					</Button>
					{canPublish ? (
						<Button
							type="button"
							variant="bordered"
							className="adm-quiet-button"
							disabled={submitting || uploading}
							isLoading={submittingIntent === "publish"}
							loadingLabel="正在寫入代行紀錄"
							onClick={() => void submit(true)}
						>
							儲存並上架
						</Button>
					) : null}
				</span>
			</div>
		</>
	);
}
