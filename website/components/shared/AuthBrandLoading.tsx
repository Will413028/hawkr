import "@/styles/components/route-loading-skeleton.css";
import BrandLogoLoader from "@/components/shared/BrandLogoLoader";
import BrandWordmark from "@/components/shared/BrandWordmark";

export default function AuthBrandLoading() {
	return (
		<div
			className="auth-brand-loading"
			role="status"
			aria-live="polite"
			aria-label="Hawkr 載入中"
		>
			<div className="auth-brand-loading__mark" aria-hidden="true">
				<BrandLogoLoader />
			</div>
			<BrandWordmark className="auth-brand-loading__word" />
		</div>
	);
}
