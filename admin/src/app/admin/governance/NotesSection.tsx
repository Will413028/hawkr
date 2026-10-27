"use client";

import { timestampDate } from "@bufbuild/protobuf/wkt";
import type { GovernanceNote } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";
import { useState } from "react";
import { noteError } from "@/app/admin/measures/measure-actions";
import { GovernanceNoticeBanner } from "@/app/admin/merchants/[id]/GovernanceNotice";
import { Button } from "@/components/UI";
import { connectErrorText } from "@/lib/error-text";
import { formatDateTimeShort } from "@/lib/format";
import {
	currentPage,
	currentToken,
	firstPage,
	nextPage,
	previousPage,
} from "@/lib/keyset-pages";
import { severityChangeText } from "./notes";

/** 一頁註記（舊到新）的讀取結果；措施與案件各自的 ListXxxNotes 都符合這個形狀。 */
export interface NotesQuery {
	data: { notes: GovernanceNote[]; nextPageToken: string } | undefined;
	isLoading: boolean;
	isError: boolean;
	isFetching: boolean;
	error: unknown;
	refetch: () => unknown;
}

export interface NotesCreate {
	/** 新增一則註記；失敗時 reject，成功後呼叫端負責讓清單重讀。 */
	create: (body: string) => Promise<unknown>;
	isPending: boolean;
}

export interface NotesSectionProps {
	/** 區塊標題的 id；同一頁上每個註記區各自一個。 */
	titleId: string;
	/** 輸入框的 id。 */
	bodyId: string;
	/** 註記掛在哪個資源上（措施或案件的 id），原樣傳給下面兩個 hook。 */
	scopeId: string;
	/** 讀一頁註記的 hook：每次渲染以同樣的順序呼叫。 */
	useNotes: (scopeId: string, pageToken: string) => NotesQuery;
	/** 新增註記的 hook。 */
	useCreateNote: (scopeId: string) => NotesCreate;
}

/**
 * 內部註記：只新增的時間軸（舊到新），只有後台看得到。寫註記不經 etag、不改它掛的資源的版本，所以與其他操作互不影響。
 * 措施與案件共用這個區塊，各自給自己的讀取與新增；改嚴重度時寫的註記在標頭列出改前與改後。
 */
export default function NotesSection({
	titleId,
	bodyId,
	scopeId,
	useNotes,
	useCreateNote,
}: NotesSectionProps) {
	const [pageTokens, setPageTokens] = useState(firstPage);
	const [body, setBody] = useState("");
	const [error, setError] = useState<string | null>(null);
	const list = useNotes(scopeId, currentToken(pageTokens));
	const { create, isPending } = useCreateNote(scopeId);
	const notes = list.data?.notes ?? [];
	const nextPageToken = list.data?.nextPageToken ?? "";
	const page = currentPage(pageTokens);

	const submit = async () => {
		if (isPending) return;
		const invalid = noteError(body);
		if (invalid) {
			setError(invalid);
			return;
		}
		setError(null);
		try {
			await create(body);
			setBody("");
		} catch (caught) {
			setError(
				connectErrorText(
					caught,
					"新增註記失敗，註記沒有送出；請確認網路狀態後再試一次。",
				),
			);
		}
	};

	return (
		<section className="detail-card" aria-labelledby={titleId}>
			<div className="section-title">
				<MaterialSymbol name="sticky_note_2" />
				<h2 id={titleId}>內部註記</h2>
			</div>
			<div className="card-body">
				<p className="adm-operation-field-note">
					只有後台看得到。送出後不能修改或刪除，要更正請再寫一則。不要寫通報人的聯絡方式。
				</p>
				{list.isLoading ? (
					<p role="status">載入中...</p>
				) : list.isError ? (
					<div className="tab-empty-state" role="alert">
						<MaterialSymbol name="error" />
						<span>{connectErrorText(list.error, "註記讀取失敗")}</span>
						<Button variant="outline" onClick={() => void list.refetch()}>
							重試
						</Button>
					</div>
				) : notes.length === 0 ? (
					<p>還沒有註記</p>
				) : (
					<ol style={{ listStyle: "none", padding: 0, margin: 0 }}>
						{notes.map((note) => (
							<li key={note.id} style={{ marginBottom: 12 }}>
								<div className="adm-operation-field-note">
									{note.createdByAdminId || "-"}・
									{note.createTime
										? formatDateTimeShort(timestampDate(note.createTime))
										: "-"}
								</div>
								{note.severityChange && (
									<div style={{ fontWeight: 700 }}>
										{severityChangeText(note.severityChange)}
									</div>
								)}
								<div style={{ whiteSpace: "pre-wrap" }}>{note.body}</div>
							</li>
						))}
					</ol>
				)}
				{(page > 1 || nextPageToken) && (
					<div style={{ display: "flex", gap: 8, margin: "8px 0" }}>
						<Button
							variant="outline"
							disabled={page <= 1 || list.isFetching}
							onClick={() => setPageTokens(previousPage(pageTokens))}
						>
							上一頁
						</Button>
						<Button
							variant="outline"
							disabled={!nextPageToken || list.isFetching}
							onClick={() => setPageTokens(nextPage(pageTokens, nextPageToken))}
						>
							下一頁
						</Button>
					</div>
				)}
				<textarea
					id={bodyId}
					aria-label="內部註記"
					className="form-textarea"
					rows={3}
					placeholder="寫下調查經過或判斷依據（最多 2000 字）"
					value={body}
					disabled={isPending}
					onChange={(e) => {
						setBody(e.target.value);
						setError(null);
					}}
				/>
				<GovernanceNoticeBanner
					notice={error ? { tone: "error", text: error } : null}
				/>
				<Button
					variant="primary"
					onClick={() => void submit()}
					disabled={isPending}
				>
					新增註記
				</Button>
			</div>
		</section>
	);
}
