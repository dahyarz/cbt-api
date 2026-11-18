package grading

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/sasadu/cbt-api/internal/model"
)

// Tipe yang dinilai otomatis. Selain ini dianggap essay/manual.
var manualTypes = map[string]bool{"ESSAY": true, "SHORT_ANSWER": true, "UPLOAD": true}

func IsManual(t string) bool { return manualTypes[strings.ToUpper(t)] }

func Grade(questions []model.Question, answers map[string]json.RawMessage) (final, max int, scores map[string]int, hasManual bool) {
	scores = map[string]int{}
	for _, q := range questions {
		max += q.Points
		if IsManual(q.Type) {
			hasManual = true
			continue
		}
		given, ok := answers[q.ID]
		if !ok || len(given) == 0 {
			scores[q.ID] = 0
			continue
		}
		if equal(q.CorrectAnswer, given) {
			scores[q.ID] = q.Points
			final += q.Points
		} else {
			scores[q.ID] = 0
		}
	}
	return
}

func equal(correct, given json.RawMessage) bool {
	var a, b any
	if json.Unmarshal(correct, &a) != nil || json.Unmarshal(given, &b) != nil {
		return strings.EqualFold(strings.TrimSpace(string(correct)), strings.TrimSpace(string(given)))
	}
	return fmt.Sprint(canonical(a)) == fmt.Sprint(canonical(b))
}

func canonical(v any) any {
	switch t := v.(type) {
	case string:
		return strings.ToLower(strings.TrimSpace(t))
	case []any:
		out := make([]string, 0, len(t))
		for _, e := range t { out = append(out, fmt.Sprint(canonical(e))) }
		sort.Strings(out)
		return out
	case map[string]any:
		out := map[string]string{}
		for k, e := range t { out[k] = fmt.Sprint(canonical(e)) }
		return out
	default:
		return v
	}
}
