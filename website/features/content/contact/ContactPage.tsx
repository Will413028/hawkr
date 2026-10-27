"use client";

import { useCallback, useMemo, useState } from "react";
import { Input, Select, Textarea } from "@/components/ui";
import Button from "@/components/ui/Button";
import { getEnv } from "@/lib/config/env";

const GOOGLE_FORM_URL =
	"https://docs.google.com/forms/d/e/1FAIpQLSefqVw5BrXD70RksYvw25ks0x4jD-JEb0zUenKk5JYQxlu3ow/formResponse";

const CONTACT_DETAILS: {
	/** "line" entries are rendered only when NEXT_PUBLIC_LINE_URL is configured. */
	kind?: "line";
	label: string;
	value: string;
	href?: string;
	icon: React.ReactNode;
}[] = [
	{
		label: "Email",
		value: "supports@example.com",
		href: "mailto:supports@example.com",
		icon: (
			<svg
				aria-hidden="true"
				width="20"
				height="20"
				viewBox="0 0 24 24"
				fill="none"
				stroke="currentColor"
				strokeWidth="1.8"
				strokeLinecap="round"
				strokeLinejoin="round"
			>
				<rect x="3" y="5" width="18" height="14" rx="2" />
				<path d="m3 7 9 6 9-6" />
			</svg>
		),
	},
	{
		kind: "line",
		label: "LINE",
		value: "加入官方帳號",
		icon: (
			<svg
				aria-hidden="true"
				width="20"
				height="20"
				viewBox="0 0 24 24"
				fill="none"
				stroke="currentColor"
				strokeWidth="1.8"
				strokeLinecap="round"
				strokeLinejoin="round"
			>
				<path d="M21 11.5a8.38 8.38 0 0 1-8.5 8.5 9.6 9.6 0 0 1-2.6-.35L4 21l1.35-4a8.38 8.38 0 0 1-1.35-4.5C4 7 7.8 3.5 12.5 3.5S21 7 21 11.5Z" />
			</svg>
		),
	},
	{
		label: "服務時間",
		value: "週一至週五 09:00–18:00",
		icon: (
			<svg
				aria-hidden="true"
				width="20"
				height="20"
				viewBox="0 0 24 24"
				fill="none"
				stroke="currentColor"
				strokeWidth="1.8"
				strokeLinecap="round"
				strokeLinejoin="round"
			>
				<circle cx="12" cy="12" r="9" />
				<path d="M12 7v5l3 2" />
			</svg>
		),
	},
];

const SUBJECT_OPTIONS = [
	"供應商問題",
	"企業採購問題",
	"商城問題",
	"交易與售後問題",
	"收帳問題",
	"投資人問題",
	"異業合作",
	"徵才",
	"其他",
];

export default function ContactPage() {
	const { lineUrl } = getEnv();
	const contactDetails = CONTACT_DETAILS.flatMap((item) => {
		if (item.kind !== "line") return [item];
		return lineUrl ? [{ ...item, href: lineUrl }] : [];
	});
	const [contact, setContact] = useState({
		name: "",
		email: "",
		phone: "",
		subject: "",
		message: "",
	});
	const [submitted, setSubmitted] = useState(false);
	const [submitting, setSubmitting] = useState(false);

	const canSubmit = useMemo(() => {
		return (
			contact.name.trim() !== "" &&
			contact.email.trim() !== "" &&
			contact.phone.trim() !== "" &&
			contact.subject !== "" &&
			contact.message.trim() !== ""
		);
	}, [contact]);

	const handleChange = useCallback(
		(
			e: React.ChangeEvent<
				HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement
			>,
		) => {
			const { name, value } = e.target;
			setContact((prev) => ({ ...prev, [name]: value }));
		},
		[],
	);

	const handleSubmit = useCallback(
		async (e: React.FormEvent<HTMLFormElement>) => {
			e.preventDefault();
			// submitting guard: the Google Form roundtrip can take seconds on a
			// slow network — without it repeat clicks POST duplicate entries.
			if (!canSubmit || submitting) return;
			setSubmitting(true);

			const formData = new FormData();
			formData.append("entry.70596064", contact.name);
			formData.append("entry.2093562995", contact.email);
			formData.append("entry.80969214", contact.phone);
			formData.append("entry.939655911", contact.subject);
			formData.append("entry.409767", contact.message);
			formData.append("fvv", "1");
			formData.append("pageHistory", "0");

			try {
				await fetch(GOOGLE_FORM_URL, {
					method: "POST",
					body: formData,
					mode: "no-cors",
				});
			} catch {
				// Google Forms CORS will cause an error but the submission still goes through
			}

			setSubmitted(true);
			setContact({ name: "", email: "", phone: "", subject: "", message: "" });
			setSubmitting(false);
		},
		[canSubmit, contact, submitting],
	);

	const closeDialog = useCallback(() => {
		setSubmitted(false);
	}, []);

	return (
		<>
			<section className="section relative overflow-hidden">
				<div className="content-container relative z-10">
					<h1 className="sr-only">聯絡我們</h1>

					<div className="mx-auto grid w-full max-w-[1040px] grid-cols-1 items-start gap-[var(--hkui-space-10)] px-[var(--hkui-space-5)] py-[var(--hkui-space-4)] sm:px-[var(--hkui-space-6)] sm:py-[var(--hkui-space-6)] lg:grid-cols-[minmax(0,0.85fr)_minmax(0,1.15fr)] lg:gap-[var(--hkui-space-8)] lg:px-[var(--hkui-space-8)] lg:py-0">
						{/* Intro / contact details */}
						<div className="flex flex-col gap-[var(--hkui-space-6)] lg:pt-[var(--hkui-space-4)]">
							<div className="flex flex-col gap-[var(--hkui-space-3)]">
								<span className="text-[length:var(--text-sm)] font-bold uppercase tracking-[4px] text-[var(--color-primary)]">
									Contact Us
								</span>
								<h2 className="m-0 text-[length:var(--text-4xl)] font-bold leading-tight text-[color:var(--text-heading)]">
									聯絡我們
								</h2>
								<p className="m-0 max-w-[40ch] text-[length:var(--text-base)] leading-[1.8] text-[color:var(--text-body)]">
									無論是訂單與售後、賣家入駐，或是通路與品牌合作，留下你的訊息，我們會盡快回覆，陪你把從產地到餐桌的事做得更好。
								</p>
							</div>

							<ul className="flex flex-col gap-[var(--hkui-space-5)] lg:gap-[var(--hkui-space-4)]">
								{contactDetails.map((item) => (
									<li
										key={item.label}
										className="flex items-start gap-[var(--hkui-space-3)]"
									>
										<span className="mt-[2px] inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-[rgb(var(--color-brand-primary-strong-rgb)/0.12)] text-[var(--color-primary)]">
											{item.icon}
										</span>
										<span className="flex flex-col gap-[2px]">
											<span className="text-[length:var(--text-xs)] font-medium uppercase tracking-[1px] text-[var(--color-gray-mid)]">
												{item.label}
											</span>
											{item.href ? (
												<a
													className="text-[length:var(--text-base)] font-semibold text-[color:var(--text-heading)] transition-colors hover:text-[var(--color-primary)]"
													href={item.href}
													target={
														item.href.startsWith("http") ? "_blank" : undefined
													}
													rel={
														item.href.startsWith("http")
															? "noopener noreferrer"
															: undefined
													}
												>
													{item.value}
												</a>
											) : (
												<span className="text-[length:var(--text-base)] font-semibold text-[color:var(--text-heading)]">
													{item.value}
												</span>
											)}
										</span>
									</li>
								))}
							</ul>
						</div>

						{/* Form */}
						<form
							id="contact-form"
							className="flex w-full flex-col gap-[var(--hkui-space-5)] rounded-[var(--hkui-radius-2xl)] border border-[var(--hkui-divider)] bg-[var(--hkui-content1)] px-[var(--hkui-space-6)] py-[var(--hkui-space-8)] shadow-[var(--hkui-shadow-md)]"
							onSubmit={handleSubmit}
						>
							<Input
								floatingLabel
								variant="bordered"
								size="lg"
								label="姓名"
								name="name"
								value={contact.name}
								onChange={handleChange}
								type="text"
								isRequired
							/>
							<Input
								floatingLabel
								variant="bordered"
								size="lg"
								label="E-mail"
								name="email"
								value={contact.email}
								onChange={handleChange}
								type="email"
								isRequired
							/>
							<Input
								floatingLabel
								variant="bordered"
								size="lg"
								label="電話"
								name="phone"
								value={contact.phone}
								onChange={handleChange}
								type="tel"
								placeholder="0912-345-678"
								isRequired
							/>
							<Select
								variant="bordered"
								size="lg"
								label="主旨"
								name="subject"
								value={contact.subject}
								onChange={handleChange}
								isRequired
								placeholder="請選擇主旨"
							>
								{SUBJECT_OPTIONS.map((opt) => (
									<option key={opt} value={opt}>
										{opt}
									</option>
								))}
							</Select>
							<Textarea
								variant="bordered"
								size="lg"
								label="訊息內容"
								name="message"
								value={contact.message}
								onChange={handleChange}
								minRows={5}
								isRequired
								placeholder="請輸入想詢問的內容"
							/>
							<div className="flex justify-end mt-[var(--hkui-space-2)]">
								<Button
									type="submit"
									size="lg"
									disabled={!canSubmit}
									isLoading={submitting}
									loadingLabel="送出聯絡資料中…"
								>
									送出
								</Button>
							</div>
						</form>
					</div>
				</div>
			</section>

			{/* Success Dialog */}
			{submitted && (
				<div
					className="fixed inset-0 z-[var(--z-tooltip)] flex items-center justify-center p-6"
					role="dialog"
					aria-modal="true"
					aria-labelledby="contact-success-title"
				>
					<button
						type="button"
						className="absolute inset-0 cursor-pointer border-0 bg-[rgb(var(--color-ink-950-rgb)/0.42)] backdrop-blur-[8px]"
						onClick={closeDialog}
						aria-label="關閉"
					/>
					<div className="relative z-[1] w-[min(92vw,420px)] rounded-[var(--radius-lg)] border border-[rgba(86,86,86,0.12)] bg-[var(--color-neutral-cream)] px-7 py-8 text-center shadow-[0_24px_72px_rgb(var(--color-ink-950-rgb)/0.24)]">
						<div className="mb-[18px] inline-flex h-[72px] w-[72px] items-center justify-center rounded-full bg-[rgb(var(--color-brand-primary-strong-rgb)/0.12)]">
							<svg
								aria-hidden="true"
								width="48"
								height="48"
								viewBox="0 0 24 24"
								fill="none"
								stroke="var(--action-primary)"
								strokeWidth="2"
								strokeLinecap="round"
								strokeLinejoin="round"
							>
								<path d="M22 11.08V12a10 10 0 11-5.93-9.14" />
								<polyline points="22 4 12 14.01 9 11.01" />
							</svg>
						</div>
						<h3
							className="m-0 text-[length:var(--text-xl)] font-bold text-[var(--ink-heading)]"
							id="contact-success-title"
						>
							感謝你的來信
						</h3>
						<p className="my-[10px] mb-6 leading-[1.7] text-[var(--ink-body)]">
							我們已收到你的訊息，將盡快回覆你。
						</p>
						<button
							type="button"
							className="hk-cta-primary inline-flex h-11 min-w-[120px] cursor-pointer items-center justify-center rounded-full border-0 bg-hk-primary font-bold text-hk-inverse hover:bg-hk-primary"
							onClick={closeDialog}
						>
							確定
						</button>
					</div>
				</div>
			)}
		</>
	);
}
