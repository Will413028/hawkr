import type { ReactNode } from "react";

interface LegalSection {
	id: string;
	title: string;
	content: ReactNode;
}

interface LegalPageProps {
	title: string;
	enTitle: string;
	lastUpdated: string;
	description: string;
	sections: LegalSection[];
	/** Use the more opaque frosted-glass card surface (matches the FAQ page). */
	solidCards?: boolean;
}

export default function LegalPage({
	title,
	enTitle,
	lastUpdated,
	description,
	sections,
	solidCards = false,
}: LegalPageProps) {
	return (
		<section className="section relative overflow-hidden">
			<div className="content-container">
				<div className="flex flex-col items-center gap-3 mb-9 text-center">
					<span className="font-[var(--font-primary)] text-[length:var(--text-xl)] font-bold leading-7 text-[var(--color-primary)] uppercase">
						{enTitle}
					</span>
					<h1 className="font-[var(--font-chinese)] text-[length:var(--text-4xl)] font-bold leading-10 text-[var(--ink-body)] m-0">
						{title}
					</h1>
					<div className="w-20 h-1 bg-[var(--color-primary)] rounded-full" />
					<p className="font-[var(--font-chinese)] text-[length:var(--text-lg)] font-normal leading-7 text-[var(--ink-body)] max-w-[786px] m-0">
						{description}
					</p>
					<time
						className="block text-[length:var(--text-sm)] text-[color:var(--text-body)] mt-3"
						dateTime={lastUpdated}
					>
						最後更新：{lastUpdated}
					</time>
				</div>
				<div className="flex flex-col gap-[var(--space-6)] mt-[var(--space-8)] mb-[var(--space-16)]">
					{sections.map((section) => (
						<div
							key={section.id}
							id={section.id}
							className={`glass-card glass-card--sm p-[var(--space-8)] md:p-[var(--space-6)_var(--space-4)]${solidCards ? " glass-card--solid" : ""}`}
						>
							<h3 className="font-[var(--font-chinese)] text-[length:var(--text-xl)] font-bold text-[var(--color-primary)] mb-[var(--space-4)] pb-[var(--space-3)] border-b-2 border-[var(--color-primary)]">
								{section.title}
							</h3>
							<div className="font-[var(--font-chinese)] text-[length:var(--text-base)] text-[color:var(--text-body)] leading-relaxed [&_p]:mb-[var(--space-4)] [&_p:last-child]:mb-0 [&_ul]:pl-[var(--space-6)] [&_ul]:mb-[var(--space-4)] [&_ul]:list-disc [&_li]:mb-[var(--space-2)] [&_strong]:font-bold [&_strong]:text-[color:var(--text-heading)]">
								{section.content}
							</div>
						</div>
					))}
				</div>
			</div>
		</section>
	);
}
