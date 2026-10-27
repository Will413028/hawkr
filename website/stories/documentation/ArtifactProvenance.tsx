import { useEffect, useState } from "react";

type Artifact = {
	version: string;
	builtAt: string;
	artifactDigest: string;
	release?: {
		channel?: string;
		sourceBranch?: string;
		contractVersion?: string;
	};
	sources: {
		id: string;
		commit: string;
		lockfile: string;
		lockfileSha256: string;
	}[];
	checks: { id: string; status: string; completedAt: string }[];
};

function isArtifact(value: unknown): value is Artifact {
	if (!value || typeof value !== "object") return false;
	const artifact = value as Partial<Artifact>;
	return (
		typeof artifact.version === "string" &&
		typeof artifact.builtAt === "string" &&
		typeof artifact.artifactDigest === "string" &&
		Array.isArray(artifact.sources) &&
		artifact.sources.every(
			(source) =>
				source &&
				typeof source.id === "string" &&
				typeof source.commit === "string" &&
				typeof source.lockfile === "string" &&
				typeof source.lockfileSha256 === "string",
		) &&
		Array.isArray(artifact.checks) &&
		artifact.checks.every(
			(check) =>
				check &&
				typeof check.id === "string" &&
				typeof check.status === "string" &&
				typeof check.completedAt === "string",
		)
	);
}

/** Reads the packaged artifact beside iframe.html; local development has none. */
export function ArtifactProvenance() {
	const [artifact, setArtifact] = useState<Artifact | null>(null);
	const [loading, setLoading] = useState(true);

	useEffect(() => {
		const controller = new AbortController();
		fetch(new URL("./artifact-manifest.json", window.location.href), {
			signal: controller.signal,
			cache: "no-store",
		})
			.then(async (response) => {
				if (!response.ok) throw new Error("Artifact manifest unavailable");
				const data: unknown = await response.json();
				if (!isArtifact(data)) throw new Error("Invalid artifact manifest");
				if (!controller.signal.aborted) setArtifact(data);
			})
			.catch(() => {
				if (!controller.signal.aborted) setArtifact(null);
			})
			.finally(() => {
				if (!controller.signal.aborted) setLoading(false);
			});
		return () => controller.abort();
	}, []);

	if (loading) return <p role="status">正在讀取建置來源紀錄…</p>;
	if (!artifact)
		return (
			<p>
				此預覽沒有可讀取的 artifact-manifest.json。本機開發不包含發布產物紀錄；
				請在已封裝版本查看實際來源與檢查結果，勿把候選 metadata 當成發布證據。
			</p>
		);

	return (
		<section aria-label="建置產物來源">
			{artifact.release?.channel === "dev" && (
				<p>
					這是 <strong>dev 自動快照</strong>，來源分支為 dev；手動版本契約：
					<strong>{artifact.release.contractVersion ?? "未提供"}</strong>。
					快照通過建置檢查不代表手動版本已完成審查或正式發布。
				</p>
			)}
			<p>
				產物版本：<strong>{artifact.version}</strong>。建置時間：
				<time dateTime={artifact.builtAt}>{artifact.builtAt}</time>。
			</p>
			<p>
				Artifact SHA-256：
				<code className="break-all">{artifact.artifactDigest}</code>
			</p>
			<table>
				<thead>
					<tr>
						<th>來源</th>
						<th>Commit</th>
						<th>Lockfile SHA-256</th>
					</tr>
				</thead>
				<tbody>
					{artifact.sources.map((source) => (
						<tr key={source.id}>
							<td>{source.id}</td>
							<td>
								<code className="break-all">{source.commit}</code>
							</td>
							<td>
								{source.lockfile}
								<br />
								<code className="break-all">{source.lockfileSha256}</code>
							</td>
						</tr>
					))}
				</tbody>
			</table>
			<table>
				<thead>
					<tr>
						<th>建置檢查</th>
						<th>紀錄狀態</th>
						<th>完成時間</th>
					</tr>
				</thead>
				<tbody>
					{artifact.checks.map((check) => (
						<tr key={check.id}>
							<td>{check.id}</td>
							<td>{check.status}</td>
							<td>{check.completedAt}</td>
						</tr>
					))}
				</tbody>
			</table>
			<p>
				以上顯示此產物的原始檢查紀錄；實際啟用版本與時間以管理後台 registry
				為準。
			</p>
		</section>
	);
}
