import "@/styles/features/auth-login.css";
import type * as React from "react";

interface LoginShellProps {
	eyebrow?: string;
	title: React.ReactNode;
	subtitle?: React.ReactNode;
	children: React.ReactNode;
	footer?: React.ReactNode;
	/**
	 * Optional decorative slot rendered above the eyebrow — typically an
	 * icon that gives the auth surface visual identity. Sits
	 * inside the panel so the panel chrome (rounded corners, shadow)
	 * frames it together with the heading.
	 */
	hero?: React.ReactNode;
}

export default function LoginShell({
	eyebrow,
	title,
	subtitle,
	children,
	footer,
	hero,
}: LoginShellProps) {
	return (
		<main className="hkui-login">
			<div className="hkui-login__panel">
				{hero && <div className="hkui-login__hero">{hero}</div>}
				{eyebrow && <span className="hkui-login__eyebrow">{eyebrow}</span>}
				<h1 className="hkui-login__title">{title}</h1>
				{subtitle && <p className="hkui-login__subtitle">{subtitle}</p>}

				<div className="hkui-login__body">{children}</div>

				{footer && <div className="hkui-login__footer">{footer}</div>}
			</div>
		</main>
	);
}
