"use client";

import {
	type PointerEvent as ReactPointerEvent,
	useCallback,
	useEffect,
	useRef,
	useState,
} from "react";
import AppImage from "@/components/shared/AppImage";

interface ShowcaseSlide {
	id: string;
	tag: string;
	title: string;
	desc: string;
	phoneSrc: string;
	phoneAlt: string;
	desktop: {
		url: string;
		title: string;
		hint: string;
		imageSrc: string;
		imageAlt: string;
	};
}

interface DragState {
	pointerId: number;
	startX: number;
	startY: number;
	currentX: number;
	currentY: number;
	startedAt: number;
}

const AUTOPLAY_DELAY = 5200;

const SLIDES: ShowcaseSlide[] = [
	{
		id: "dashboard",
		tag: "#經營儀表板",
		title: "一眼看完\n今天要忙什麼",
		desc: "銷售數據、訂單狀態與店鋪指標集中在首頁，打開 App 就知道下一步要處理什麼。",
		phoneSrc: "/placeholders/app-screen.svg",
		phoneAlt: "賣家 App 首頁銷售數據畫面",
		desktop: {
			url: "example.com/seller/dashboard",
			title: "經營儀表板",
			hint: "近 7 日營收、待出貨與熱賣商品一頁掌握",
			imageSrc: "/placeholders/web-screen.svg",
			imageAlt: "賣家中心網頁版儀表板預覽",
		},
	},
	{
		id: "orders",
		tag: "#訂單管理",
		title: "訂單來了\n不漏接、不出錯",
		// 第三桶叫「已結束」不是「已完成」——它含已取消（80）與退款（70）。名稱的唯一
		// 來源是 `features/seller-center/orders/orderTabs.ts` 的 `SELLER_ORDER_TAB_LABEL`；
		// 這句是招募頁對它的描述，字不一致等於在賣家還沒進來之前就先教錯一次。
		desc: "待出貨、配送中、已結束清楚分流，出貨與取件動作在列表上就能完成。",
		phoneSrc: "/placeholders/app-screen.svg",
		phoneAlt: "賣家 App 訂單列表畫面",
		desktop: {
			url: "example.com/seller/orders",
			title: "訂單管理",
			hint: "新單即時進線，狀態一鍵更新",
			imageSrc: "/placeholders/web-screen.svg",
			imageAlt: "賣家中心網頁版訂單管理預覽",
		},
	},
	{
		id: "products",
		tag: "#商品管理",
		title: "商品管理\n上架下架都直覺",
		desc: "商品、價格、評價與上下架狀態直接在店鋪中管理，買家看到的內容同步更新。",
		phoneSrc: "/placeholders/app-screen.svg",
		phoneAlt: "賣家 App 店鋪商品管理畫面",
		desktop: {
			url: "example.com/seller/products",
			title: "商品管理",
			hint: "上架、定價、庫存與分類同步更新",
			imageSrc: "/placeholders/web-screen.svg",
			imageAlt: "賣家中心網頁版商品管理預覽",
		},
	},
	{
		id: "notifications",
		tag: "#通知中心",
		title: "重要提醒\n主動推到你眼前",
		desc: "新訂單、商品瀏覽與買家互動即時提醒，讓賣家不用反覆刷新也能掌握機會。",
		phoneSrc: "/placeholders/app-screen.svg",
		phoneAlt: "賣家 App 通知提醒畫面",
		desktop: {
			url: "example.com/seller/notifications",
			title: "通知中心",
			hint: "新訂單、留言與商品瀏覽集中提醒",
			imageSrc: "/placeholders/web-screen.svg",
			imageAlt: "賣家中心網頁版通知列表預覽",
		},
	},
];

function DesktopPreview({
	slide,
	active,
	onActivate,
}: {
	slide: ShowcaseSlide;
	active: boolean;
	onActivate: () => void;
}) {
	return (
		<button
			type="button"
			className="showcase-desktop"
			onClick={onActivate}
			aria-label={`切換到 Web 預覽：${slide.desktop.title}`}
			aria-pressed={active}
		>
			<div className="showcase-desktop__bar">
				<span className="dot" />
				<span className="dot" />
				<span className="dot" />
				<span className="showcase-desktop__url">{slide.desktop.url}</span>
			</div>
			<div className="showcase-desktop__body">
				<AppImage
					src={slide.desktop.imageSrc}
					alt={slide.desktop.imageAlt}
					width={1040}
					height={650}
					className="showcase-desktop__img"
					sizes="(max-width: 1100px) 72vw, 760px"
					loading="eager"
					draggable={false}
				/>
			</div>
		</button>
	);
}

function PhonePreview({
	slide,
	active,
	onActivate,
}: {
	slide: ShowcaseSlide;
	active: boolean;
	onActivate: () => void;
}) {
	return (
		<button
			type="button"
			className="showcase-phone"
			onClick={onActivate}
			aria-label={`切換到 App 預覽：${slide.title.replace(/\n/g, "")}`}
			aria-pressed={active}
		>
			<div className="showcase-phone__screen">
				<AppImage
					src={slide.phoneSrc}
					alt={slide.phoneAlt}
					width={720}
					height={1564}
					className="showcase-phone__img"
					sizes="(max-width: 760px) 230px, (max-width: 1100px) 280px, 320px"
					loading="eager"
					draggable={false}
				/>
			</div>
		</button>
	);
}

export default function SellerShowcaseCarousel() {
	const [active, setActive] = useState(0);
	const [activeDevice, setActiveDevice] = useState<"phone" | "web">("phone");
	const [rotationEnabled, setRotationEnabled] = useState(false);
	const [isVisible, setIsVisible] = useState(false);
	const [isDocumentVisible, setIsDocumentVisible] = useState(true);
	const [announcement, setAnnouncement] = useState("");
	const viewportRef = useRef<HTMLDivElement>(null);
	const trackRef = useRef<HTMLDivElement>(null);
	const dragRef = useRef<DragState | null>(null);
	const suppressClickRef = useRef(false);
	const total = SLIDES.length;

	const stopRotation = useCallback(() => {
		setRotationEnabled(false);
	}, []);

	const announceSlide = useCallback((index: number) => {
		const normalized = (index + SLIDES.length) % SLIDES.length;
		const title = SLIDES[normalized]?.title.replace(/\n/g, "，") ?? "";
		setAnnouncement(
			`已切換到第 ${normalized + 1} 張，共 ${SLIDES.length} 張：${title}`,
		);
	}, []);

	const goTo = useCallback(
		(index: number, userInitiated = true) => {
			const normalized = (index + SLIDES.length) % SLIDES.length;
			setActive(normalized);
			if (userInitiated) {
				stopRotation();
				announceSlide(normalized);
			}
		},
		[announceSlide, stopRotation],
	);

	const next = useCallback(() => goTo(active + 1), [active, goTo]);
	const prev = useCallback(() => goTo(active - 1), [active, goTo]);

	useEffect(() => {
		const motionQuery = window.matchMedia("(prefers-reduced-motion: reduce)");
		if (!motionQuery.matches) setRotationEnabled(true);

		const handleMotionChange = (event: MediaQueryListEvent) => {
			if (event.matches) setRotationEnabled(false);
		};
		motionQuery.addEventListener("change", handleMotionChange);
		return () => motionQuery.removeEventListener("change", handleMotionChange);
	}, []);

	useEffect(() => {
		const viewport = viewportRef.current;
		if (!viewport) return;

		const observer = new IntersectionObserver(
			(entries) => {
				setIsVisible(entries.some((entry) => entry.isIntersecting));
			},
			{ threshold: 0.22 },
		);
		observer.observe(viewport);
		return () => observer.disconnect();
	}, []);

	useEffect(() => {
		const handleVisibility = () => {
			setIsDocumentVisible(document.visibilityState === "visible");
		};
		document.addEventListener("visibilitychange", handleVisibility);
		return () =>
			document.removeEventListener("visibilitychange", handleVisibility);
	}, []);

	useEffect(() => {
		if (!rotationEnabled || !isVisible || !isDocumentVisible) return;

		const timer = window.setTimeout(() => {
			setActive((active + 1) % SLIDES.length);
		}, AUTOPLAY_DELAY);
		return () => window.clearTimeout(timer);
	}, [active, isDocumentVisible, isVisible, rotationEnabled]);

	const resetDrag = useCallback(() => {
		const track = trackRef.current;
		if (track) {
			track.classList.remove("is-dragging");
			track.style.transform = `translateX(-${active * 100}%)`;
		}
		dragRef.current = null;
	}, [active]);

	const handlePointerDown = (event: ReactPointerEvent<HTMLDivElement>) => {
		if (
			!event.isPrimary ||
			(event.pointerType !== "touch" && event.pointerType !== "pen")
		) {
			return;
		}

		stopRotation();
		dragRef.current = {
			pointerId: event.pointerId,
			startX: event.clientX,
			startY: event.clientY,
			currentX: event.clientX,
			currentY: event.clientY,
			startedAt: performance.now(),
		};
		event.currentTarget.setPointerCapture(event.pointerId);
		trackRef.current?.classList.add("is-dragging");
	};

	const handlePointerMove = (event: ReactPointerEvent<HTMLDivElement>) => {
		const drag = dragRef.current;
		const track = trackRef.current;
		if (!drag || !track || drag.pointerId !== event.pointerId) return;

		drag.currentX = event.clientX;
		drag.currentY = event.clientY;
		const deltaX = drag.currentX - drag.startX;
		const deltaY = drag.currentY - drag.startY;
		if (Math.abs(deltaY) > Math.abs(deltaX)) return;

		if (Math.abs(deltaX) > 8) suppressClickRef.current = true;
		track.style.transform = `translate3d(calc(-${active * 100}% + ${deltaX}px), 0, 0)`;
	};

	const handlePointerUp = (event: ReactPointerEvent<HTMLDivElement>) => {
		const drag = dragRef.current;
		const viewport = viewportRef.current;
		const track = trackRef.current;
		if (!drag || !viewport || !track || drag.pointerId !== event.pointerId) {
			return;
		}

		const deltaX = drag.currentX - drag.startX;
		const deltaY = drag.currentY - drag.startY;
		const elapsed = Math.max(performance.now() - drag.startedAt, 1);
		const velocity = Math.abs(deltaX) / elapsed;
		const distanceThreshold = Math.min(
			84,
			Math.max(42, viewport.clientWidth * 0.12),
		);
		const shouldMove =
			Math.abs(deltaX) > Math.abs(deltaY) &&
			(Math.abs(deltaX) >= distanceThreshold ||
				(velocity > 0.45 && Math.abs(deltaX) > 24));

		track.classList.remove("is-dragging");
		dragRef.current = null;
		if (event.currentTarget.hasPointerCapture(event.pointerId)) {
			event.currentTarget.releasePointerCapture(event.pointerId);
		}

		if (shouldMove) {
			goTo(deltaX < 0 ? active + 1 : active - 1);
		} else {
			track.style.transform = `translateX(-${active * 100}%)`;
		}
		window.setTimeout(() => {
			suppressClickRef.current = false;
		}, 320);
	};

	const handleClickCapture = (event: React.MouseEvent<HTMLDivElement>) => {
		if (!suppressClickRef.current) return;
		event.preventDefault();
		event.stopPropagation();
		suppressClickRef.current = false;
	};

	const toggleRotation = () => {
		setRotationEnabled((enabled) => {
			const nextValue = !enabled;
			setAnnouncement(nextValue ? "已開始自動輪播" : "已停止自動輪播");
			return nextValue;
		});
	};

	return (
		<section
			className="seller-showcase seller-snap-section"
			id="showcase"
			aria-labelledby="seller-showcase-title"
			aria-roledescription="輪播"
			data-seller-snap
		>
			<h2 className="sr-only" id="seller-showcase-title">
				手機與電腦都能管理店舖
			</h2>
			{/* biome-ignore lint/a11y/noStaticElementInteractions: Pointer Events provide optional touch swiping; all keyboard actions remain available through real buttons. */}
			<div
				className="carousel-viewport"
				ref={viewportRef}
				onPointerDown={handlePointerDown}
				onPointerMove={handlePointerMove}
				onPointerUp={handlePointerUp}
				onPointerCancel={resetDrag}
				onClickCapture={handleClickCapture}
				onMouseEnter={stopRotation}
				onFocusCapture={stopRotation}
			>
				<div
					className="carousel-track"
					id="showcaseTrack"
					ref={trackRef}
					style={{ transform: `translateX(-${active * 100}%)` }}
					aria-live="off"
				>
					{SLIDES.map((slide, index) => (
						<article
							className="showcase-row"
							key={slide.id}
							aria-roledescription="投影片"
							aria-label={`${index + 1} / ${total}：${slide.title.replace(/\n/g, "，")}`}
							aria-hidden={index !== active}
							inert={index !== active}
						>
							<div
								className="showcase-phone-wrap"
								data-active-device={activeDevice}
							>
								<DesktopPreview
									slide={slide}
									active={activeDevice === "web"}
									onActivate={() => {
										stopRotation();
										setActiveDevice("web");
									}}
								/>
								<PhonePreview
									slide={slide}
									active={activeDevice === "phone"}
									onActivate={() => {
										stopRotation();
										setActiveDevice("phone");
									}}
								/>
							</div>
							<div className="showcase-copy">
								<span className="showcase-eyebrow">{slide.tag}</span>
								<h3 className="showcase-title">{slide.title}</h3>
								<p className="showcase-desc">{slide.desc}</p>
							</div>
						</article>
					))}
				</div>
				<div className="carousel-progress" id="showcaseProgress">
					<button
						type="button"
						className="carousel-toggle"
						onClick={toggleRotation}
						aria-controls="showcaseTrack"
						aria-label={rotationEnabled ? "停止自動輪播" : "開始自動輪播"}
					>
						<span aria-hidden="true">{rotationEnabled ? "Ⅱ" : "▶"}</span>
					</button>
					<span className="cp-current" id="showcaseCurrent">
						{String(active + 1).padStart(2, "0")}
					</span>
					<div className="cp-track">
						{SLIDES.map((slide, index) => (
							<button
								type="button"
								key={slide.id}
								className={`cp-seg${index <= active ? " is-filled" : ""}`}
								onClick={() => goTo(index)}
								aria-label={`第 ${index + 1} 張，共 ${total} 張：${slide.title.replace(/\n/g, "，")}`}
								aria-current={index === active ? "step" : undefined}
								aria-controls="showcaseTrack"
							/>
						))}
					</div>
					<span className="cp-total">/ {String(total).padStart(2, "0")}</span>
				</div>
				<button
					type="button"
					className="carousel-nav carousel-nav--prev"
					id="showcasePrev"
					onClick={prev}
					aria-label="上一張"
					aria-controls="showcaseTrack"
				>
					‹
				</button>
				<button
					type="button"
					className="carousel-nav carousel-nav--next"
					id="showcaseNext"
					onClick={next}
					aria-label="下一張"
					aria-controls="showcaseTrack"
				>
					›
				</button>
				<p className="sr-only" aria-live="polite" aria-atomic="true">
					{announcement}
				</p>
			</div>
		</section>
	);
}
