package agent

import (
	"google.golang.org/adk/v2/session"
)

func NewRunners(agents *Agents, sessionSvc session.Service) (*Runners, error) {
	return provideRunners(agents, sessionSvc)
}

func SpecialistToolOwners(deps *ToolDeps) map[string]string {
	owners := make(map[string]string)
	for role, specs := range map[string][]specialistSpec{
		"seller": sellerSpecialistSpecs(),
		"buyer":  buyerSpecialistSpecs(),
	} {
		for _, spec := range specs {
			key := role + ":" + spec.Key
			owners[spec.Name] = key
			for _, t := range spec.buildTools(deps) {
				owners[t.Name()] = key
			}
		}
	}
	return owners
}

func CoordinatorExcluded(specKey string) bool { return buyerCoordinatorExcluded[specKey] }
