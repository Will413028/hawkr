package goldenset

import "strings"

const (
	FailureRanking = "ranking"
	FailureRecall  = "recall"

	FailureUnmappable = "unmappable"
)

type Observation struct {
	Query            Query
	RankedCategories []string
	IsNIL            bool

	Unmappable bool

	ResolvedLabel string
	RankedLabels  []string
}

type Failure struct {
	Query Query
	Kind  string
	Got   []string
}

type Report struct {
	Total    int
	KnownGap int

	Unmappable        int
	PrecisionAt1      float64
	PrecisionAt1Upper float64
	RecallAtK         float64
	MRR               float64
	Failures          []Failure
}

func Evaluate(obs []Observation, k int) Report {
	r := Report{Total: len(obs)}
	if len(obs) == 0 {
		return r
	}
	var p1, recall, mrr float64
	for _, o := range obs {
		known := o.Query.Status == StatusKnownGap
		if known {
			r.KnownGap++
		}

		if o.Unmappable && o.Query.ExpectLabel != "" && (o.ResolvedLabel != "" || len(o.RankedLabels) > 0) {
			o.Unmappable = false
		}
		if o.Unmappable {
			r.Unmappable++
			if !known {
				r.Failures = append(r.Failures, Failure{Query: o.Query, Kind: FailureUnmappable, Got: []string{o.ResolvedLabel}})
			}
			continue
		}
		if o.Query.ExpectsNIL() {
			if o.IsNIL {
				p1++
				recall++
				mrr++
			} else if !known {
				r.Failures = append(r.Failures, Failure{Query: o.Query, Kind: FailureRanking, Got: o.RankedCategories})
			}
			continue
		}
		hit := rankOf(o, k)
		switch {
		case hit == 0:
			p1++
			recall++
			mrr++
		case hit > 0:
			recall++
			mrr += 1.0 / float64(hit+1)
			if !known {
				r.Failures = append(r.Failures, Failure{Query: o.Query, Kind: FailureRanking, Got: o.RankedCategories})
			}
		default:
			if !known {
				r.Failures = append(r.Failures, Failure{Query: o.Query, Kind: FailureRecall, Got: o.RankedCategories})
			}
		}
	}
	n := float64(len(obs))
	r.PrecisionAt1, r.RecallAtK, r.MRR = p1/n, recall/n, mrr/n
	r.PrecisionAt1Upper = (p1 + float64(r.Unmappable)) / n
	return r
}

func rankOf(o Observation, k int) int {
	for i, c := range o.RankedCategories {
		if i >= k {
			break
		}
		if categoryPathMatches(o.Query.ExpectCategory, c) {
			return i
		}
	}
	for i, l := range o.RankedLabels {
		if i >= k {
			break
		}
		if o.Query.ExpectLabel != "" && l == o.Query.ExpectLabel {
			return i
		}
	}
	if o.Query.ExpectLabel != "" && o.ResolvedLabel == o.Query.ExpectLabel {
		return 0
	}
	return -1
}

func categoryPathMatches(expect, observed string) bool {
	if expect == "" {
		return false
	}
	return observed == expect || strings.HasPrefix(observed, expect+" > ")
}
