"use client";

import {
	createConnectQueryKey,
	useMutation,
	useQuery,
} from "@connectrpc/connect-query";
import { SellerGovernanceAdminService } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import { useQueryClient } from "@tanstack/react-query";
import NotesSection from "../../governance/NotesSection";
import { buildCreateNote } from "../measure-actions";

const NOTES_PAGE_SIZE = 100;

function useMeasureNotes(measureId: string, pageToken: string) {
	return useQuery(SellerGovernanceAdminService.method.listMeasureNotes, {
		measureId,
		pageSize: NOTES_PAGE_SIZE,
		pageToken,
	});
}

function useCreateMeasureNote(measureId: string) {
	const queryClient = useQueryClient();
	const mutation = useMutation(
		SellerGovernanceAdminService.method.createMeasureNote,
		{
			onSuccess: () =>
				queryClient.invalidateQueries({
					queryKey: createConnectQueryKey({
						schema: SellerGovernanceAdminService.method.listMeasureNotes,
						cardinality: undefined,
					}),
				}),
		},
	);
	return {
		create: (body: string) =>
			mutation.mutateAsync(buildCreateNote(measureId, body)),
		isPending: mutation.isPending,
	};
}

/** 措施的內部註記（區塊本體在 governance/NotesSection，措施與案件共用）。 */
export default function MeasureNotesSection({
	measureId,
}: {
	measureId: string;
}) {
	return (
		<NotesSection
			titleId="measure-notes-title"
			bodyId="measure-note-body"
			scopeId={measureId}
			useNotes={useMeasureNotes}
			useCreateNote={useCreateMeasureNote}
		/>
	);
}
