// services/ai-service/internal/procurement/usecase/graph.go
package usecase

import (
	"github.com/Will413028/hawkr/ai-service/internal/procurement/cropidentity"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/workflowagent"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/workflow"
)

// NewProcurementGraph 組出一輪採購對話的 graph：
//
//	Start → extract_slots → validate_slots → crop_gate → crop_disambiguation_interrupt
//	  → crop_resume_reconcile → turn_terminal
//
// extract_slots／validate_slots／crop_gate 一律用 workflow.NewFunctionNode／
// NewEmittingFunctionNode 且 inputSchema／outputSchema 傳 nil（見 graph_nodes.go 三個
// 建構子的文件）：NewFunctionNodeFromState 會撞 jsonschema-go v0.4.3 對任何
// reflect.Struct 值一律回錯的上游 bug，https://github.com/google/jsonschema-go/issues/23。
//
// 圖內沒有 llmagent：每個節點都是 deterministic guard，terminal 的回覆只由 slot
// 完整度組出，不需要任何路由決策。要重新引入 AgentNode 時，doc57 §8.4 的 loop
// budget 是前置。
//
// price_preview node 刻意不在這張圖裡——見 NewPricePreviewNode 的範圍聲明。
// realizer 跟 narrator 一樣在 graph 構造時閉包進 terminal，不是每輪 turn 才傳入。
func NewProcurementGraph(extractor SlotExtractor, resolver cropidentity.Resolver, newID func() int64, pricePreviewChecker PricePreviewChecker, narrator StepNarrator, realizer SurfaceRealizer) (agent.Agent, error) {
	extractNode := NewExtractSlotsNode(extractor, newID)
	validateNode := NewValidateSlotsNode(newID)
	cropGateNode := NewCropGateNode(resolver, newID)
	interruptNode := NewCropDisambiguationInterruptNode(newID)
	// 消歧 resume 回來時把 progress.CropPending 收乾淨——見該建構子的文件；
	// 沒有它 terminal 會再挑一次同一題消歧問句。
	reconcileNode := NewCropResumeReconcileNode()
	terminal := workflow.NewEmittingFunctionNode("turn_terminal",
		func(ctx agent.Context, _ any, emit func(*session.Event) error) (any, error) {
			return runGraphTerminalNode(ctx, emit, pricePreviewChecker, narrator, realizer)
		},
		workflow.NodeConfig{})

	return workflowagent.New(workflowagent.Config{
		Name:        "procurement_graph",
		Description: "deterministic guard chain over Slots.Apply/CropGate ending in an LLM-free terminal",
		Edges:       workflow.Chain(workflow.Start, extractNode, validateNode, cropGateNode, interruptNode, reconcileNode, terminal),
	})
}
