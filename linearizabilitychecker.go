package linearizabilitychecker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"sort"
	"strings"
)

// Kind represents the lifecycle status of an operation.
type Kind string

const (
	KindInvoke Kind = "invoke"
	KindOk     Kind = "ok"
	KindFail   Kind = "fail"
	KindInfo   Kind = "info"
)

// Op represents a single operation in a concurrent execution history.
type Op struct {
	ID         int    `json:"id"`
	Process    int    `json:"process"`
	Kind       Kind   `json:"kind"`
	Key        string `json:"key,omitempty"`
	Input      any    `json:"input,omitempty"`
	Output     any    `json:"output,omitempty"`
	CallTime   int64  `json:"call_time"`
	ReturnTime int64  `json:"return_time"`
}

// Model defines the sequential specification of the system under test.
type Model[S any] struct {
	Init      func() S
	Step      func(state S, input any, output any) (bool, S)
	Equal     func(a S, b S) bool
	Partition func([]Op) [][]Op
	Describe  func(state S) string
}

// Options configure the linearizability search.
type Options struct {
	MaxHistoryOps  int
	MaxStates      int
	PartitionByKey bool
	OnEvent        func(event Event)
}

// Event records search telemetry without exposing data payloads.
type Event struct {
	Type           string `json:"type"`
	ExploredStates int    `json:"explored_states"`
	PrunedStates   int    `json:"pruned_states"`
	Depth          int    `json:"depth"`
}

// Report details why a history was not linearizable.
type Report struct {
	MinimalHistory []Op  `json:"minimal_history"`
	Partition      int   `json:"partition"`
	LongestPrefix  []int `json:"longest_prefix"`
}

// Result encapsulates the outcome of a linearizability check.
type Result struct {
	Ok             bool    `json:"ok"`
	Linearization  []int   `json:"linearization"`
	Counterexample *Report `json:"counterexample,omitempty"`
}

// CheckerError is the typed error returned by the checker.
type CheckerError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Error implements the standard error interface.
func (e *CheckerError) Error() string {
	if e == nil {
		return "<nil>"
	}
	return fmt.Sprintf("linearizabilitychecker: [%s] %s", e.Code, e.Message)
}

// Error alias for base error type contract.
type Error = CheckerError

const (
	DefaultMaxHistoryOps int   = 50000
	DefaultMaxStates     int   = 1000000
	MaxEDNBytes          int64 = 10485760 // 10 MB
	MaxJSONBytes         int64 = 10485760 // 10 MB
)

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// Check verifies whether history is linearizable with respect to model.
func Check[S any](ctx context.Context, model Model[S], history []Op, opts Options) (Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return Result{}, &CheckerError{Code: "search_cancelled", Message: err.Error()}
	}

	maxOps := opts.MaxHistoryOps
	if maxOps <= 0 {
		maxOps = DefaultMaxHistoryOps
	}
	if len(history) > maxOps {
		return Result{}, &CheckerError{
			Code:    "history_too_large",
			Message: fmt.Sprintf("history length %d exceeds maximum allowed %d", len(history), maxOps),
		}
	}

	if len(history) == 0 {
		return Result{Ok: true, Linearization: []int{}}, nil
	}

	maxStates := opts.MaxStates
	if maxStates <= 0 {
		maxStates = DefaultMaxStates
	}

	var partitions [][]Op
	if model.Partition != nil {
		partitions = model.Partition(history)
	} else if opts.PartitionByKey {
		partitions = partitionByKey(history)
	} else {
		partitions = [][]Op{history}
	}

	if len(partitions) == 0 {
		return Result{Ok: true, Linearization: []int{}}, nil
	}

	var combinedLinearization []int
	for pIdx, partOps := range partitions {
		res, err := checkPartition(ctx, model, partOps, maxStates, opts.OnEvent)
		if err != nil {
			return Result{}, err
		}
		if !res.Ok {
			minimal := shrinkCounterexample(ctx, model, partOps, maxStates)
			report := &Report{
				MinimalHistory: minimal,
				Partition:      pIdx,
				LongestPrefix:  res.Linearization,
			}
			return Result{Ok: false, Counterexample: report}, nil
		}
		combinedLinearization = append(combinedLinearization, res.Linearization...)
	}

	return Result{Ok: true, Linearization: combinedLinearization}, nil
}

func partitionByKey(history []Op) [][]Op {
	groups := make(map[string][]Op)
	var keys []string
	for _, op := range history {
		if _, ok := groups[op.Key]; !ok {
			keys = append(keys, op.Key)
		}
		groups[op.Key] = append(groups[op.Key], op)
	}
	sort.Strings(keys)
	res := make([][]Op, len(keys))
	for i, k := range keys {
		res[i] = groups[k]
	}
	return res
}

type internalOp struct {
	op         Op
	origIndex  int
	callTime   int64
	returnTime int64
	isInfo     bool
}

func checkPartition[S any](
	ctx context.Context,
	model Model[S],
	history []Op,
	maxStates int,
	onEvent func(Event),
) (Result, error) {
	var ops []internalOp
	for i, op := range history {
		if op.Kind == KindFail {
			continue
		}
		rTime := op.ReturnTime
		isInfo := false
		if op.Kind == KindInfo || rTime == 0 {
			rTime = math.MaxInt64
			isInfo = true
		}
		cTime := op.CallTime
		if rTime < cTime && !isInfo {
			return Result{}, &CheckerError{
				Code:    "invalid_history_sequence",
				Message: fmt.Sprintf("operation %d return time %d precedes call time %d", op.ID, rTime, cTime),
			}
		}
		ops = append(ops, internalOp{
			op:         op,
			origIndex:  i,
			callTime:   cTime,
			returnTime: rTime,
			isInfo:     isInfo,
		})
	}

	if len(ops) == 0 {
		return Result{Ok: true, Linearization: []int{}}, nil
	}

	n := len(ops)
	var requiredCount int
	for _, op := range ops {
		if !op.isInfo {
			requiredCount++
		}
	}

	equalFn := model.Equal
	if equalFn == nil {
		equalFn = func(a S, b S) bool {
			return reflect.DeepEqual(a, b)
		}
	}

	initState := model.Init()
	memo := make(map[uint64][]S)
	statesCount := 0
	prunedCount := 0
	exploredCount := 0
	var longestPrefix []int

	var dfs func(mask uint64, state S, currentLin []int, reqCompleted int) (bool, []int, error)
	dfs = func(mask uint64, state S, currentLin []int, reqCompleted int) (bool, []int, error) {
		exploredCount++
		if onEvent != nil {
			onEvent(Event{
				Type:           "progress",
				ExploredStates: exploredCount,
				PrunedStates:   prunedCount,
				Depth:          len(currentLin),
			})
		}

		if reqCompleted == requiredCount {
			return true, currentLin, nil
		}

		if len(currentLin) > len(longestPrefix) {
			longestPrefix = make([]int, len(currentLin))
			copy(longestPrefix, currentLin)
		}

		if err := ctx.Err(); err != nil {
			return false, nil, &CheckerError{Code: "search_cancelled", Message: err.Error()}
		}

		if existing, ok := memo[mask]; ok {
			for _, s := range existing {
				if equalFn(s, state) {
					prunedCount++
					return false, nil, nil
				}
			}
		}
		if statesCount >= maxStates {
			return false, nil, &CheckerError{
				Code:    "depth_limit_exceeded",
				Message: fmt.Sprintf("max explored states %d reached", maxStates),
			}
		}
		memo[mask] = append(memo[mask], state)
		statesCount++

		var minReturn int64 = math.MaxInt64
		for i := 0; i < n; i++ {
			if (mask & (1 << i)) == 0 {
				if ops[i].returnTime < minReturn {
					minReturn = ops[i].returnTime
				}
			}
		}

		for i := 0; i < n; i++ {
			if (mask & (1 << i)) != 0 {
				continue
			}
			if ops[i].callTime > minReturn {
				continue
			}

			ok, nextState := model.Step(state, ops[i].op.Input, ops[i].op.Output)
			if !ok {
				continue
			}

			nextMask := mask | (1 << i)
			nextReq := reqCompleted
			if !ops[i].isInfo {
				nextReq++
			}

			nextLin := append(currentLin, ops[i].origIndex)
			success, resLin, err := dfs(nextMask, nextState, nextLin, nextReq)
			if err != nil {
				return false, nil, err
			}
			if success {
				return true, resLin, nil
			}
		}

		return false, nil, nil
	}

	if n > 62 {
		return Result{}, &CheckerError{
			Code:    "history_too_large",
			Message: fmt.Sprintf("partition size %d exceeds 62 operations", n),
		}
	}

	ok, lin, err := dfs(0, initState, nil, 0)
	if err != nil {
		return Result{}, err
	}
	if ok {
		return Result{Ok: true, Linearization: lin}, nil
	}
	return Result{Ok: false, Linearization: longestPrefix}, nil
}

func shrinkCounterexample[S any](ctx context.Context, model Model[S], history []Op, maxStates int) []Op {
	curr := make([]Op, len(history))
	copy(curr, history)

	changed := true
	for changed && len(curr) > 1 {
		changed = false
		for i := 0; i < len(curr); i++ {
			candidate := make([]Op, 0, len(curr)-1)
			candidate = append(candidate, curr[:i]...)
			candidate = append(candidate, curr[i+1:]...)
			res, err := checkPartition(ctx, model, candidate, maxStates, nil)
			if err == nil && !res.Ok {
				curr = candidate
				changed = true
				break
			}
		}
	}
	return curr
}

// RegisterModel returns a Model for a single atomic register.
func RegisterModel() Model[any] {
	return Model[any]{
		Init: func() any { return nil },
		Step: func(state any, input any, output any) (bool, any) {
			inMap, okInMap := input.(map[string]any)
			if okInMap {
				f, _ := inMap["f"].(string)
				f = strings.TrimPrefix(f, ":")
				if f == "write" {
					return true, inMap["value"]
				}
				if f == "read" {
					if output == nil && state == nil {
						return true, state
					}
					return reflect.DeepEqual(state, output), state
				}
			}

			if input != nil {
				return true, input
			}
			if output == nil && state == nil {
				return true, state
			}
			return reflect.DeepEqual(state, output), state
		},
		Equal: func(a, b any) bool {
			return reflect.DeepEqual(a, b)
		},
		Describe: func(state any) string {
			return fmt.Sprintf("Register(%v)", state)
		},
	}
}

// KVRegisterModel returns a Model for a key-value store.
func KVRegisterModel() Model[map[string]any] {
	return Model[map[string]any]{
		Init: func() map[string]any {
			return make(map[string]any)
		},
		Step: func(state map[string]any, input any, output any) (bool, map[string]any) {
			clone := make(map[string]any, len(state))
			for k, v := range state {
				clone[k] = v
			}
			m, ok := input.(map[string]any)
			if !ok {
				return false, state
			}
			opType, _ := m["f"].(string)
			opType = strings.TrimPrefix(opType, ":")
			key, _ := m["key"].(string)
			if opType == "write" {
				clone[key] = m["value"]
				return true, clone
			}
			if opType == "read" {
				currVal := state[key]
				return reflect.DeepEqual(currVal, output), state
			}
			return false, state
		},
		Equal: func(a, b map[string]any) bool {
			return reflect.DeepEqual(a, b)
		},
		Partition: func(ops []Op) [][]Op {
			return partitionByKey(ops)
		},
		Describe: func(state map[string]any) string {
			return fmt.Sprintf("KV(%v)", state)
		},
	}
}

// FIFOQueueModel returns a Model for a FIFO queue.
func FIFOQueueModel() Model[[]any] {
	return Model[[]any]{
		Init: func() []any {
			return []any{}
		},
		Step: func(state []any, input any, output any) (bool, []any) {
			if inMap, ok := input.(map[string]any); ok {
				f, _ := inMap["f"].(string)
				f = strings.TrimPrefix(f, ":")
				if f == "enqueue" || f == "push" {
					next := make([]any, len(state), len(state)+1)
					copy(next, state)
					next = append(next, inMap["value"])
					return true, next
				}
				if f == "dequeue" || f == "pop" {
					if len(state) == 0 {
						if output == nil {
							return true, state
						}
						return false, state
					}
					if reflect.DeepEqual(state[0], output) {
						next := make([]any, len(state)-1)
						copy(next, state[1:])
						return true, next
					}
					return false, state
				}
			}

			if input != nil {
				next := make([]any, len(state), len(state)+1)
				copy(next, state)
				next = append(next, input)
				return true, next
			}
			if len(state) == 0 {
				if output == nil {
					return true, state
				}
				return false, state
			}
			if reflect.DeepEqual(state[0], output) {
				next := make([]any, len(state)-1)
				copy(next, state[1:])
				return true, next
			}
			return false, state
		},
		Equal: func(a, b []any) bool {
			return reflect.DeepEqual(a, b)
		},
		Describe: func(state []any) string {
			return fmt.Sprintf("FIFOQueue(%v)", state)
		},
	}
}

// SetModel returns a Model for a concurrent Set.
func SetModel() Model[map[any]struct{}] {
	return Model[map[any]struct{}]{
		Init: func() map[any]struct{} {
			return make(map[any]struct{})
		},
		Step: func(state map[any]struct{}, input any, output any) (bool, map[any]struct{}) {
			m, ok := input.(map[string]any)
			if !ok {
				return false, state
			}
			opType, _ := m["f"].(string)
			opType = strings.TrimPrefix(opType, ":")
			val := m["value"]
			next := make(map[any]struct{}, len(state))
			for k, v := range state {
				next[k] = v
			}
			if opType == "add" {
				next[val] = struct{}{}
				return true, next
			}
			if opType == "read" {
				outSlice, ok := output.([]any)
				if !ok {
					outSet, ok2 := output.(map[any]struct{})
					if ok2 {
						return reflect.DeepEqual(state, outSet), state
					}
					return false, state
				}
				if len(outSlice) != len(state) {
					return false, state
				}
				for _, item := range outSlice {
					if _, exists := state[item]; !exists {
						return false, state
					}
				}
				return true, state
			}
			return false, state
		},
		Equal: func(a, b map[any]struct{}) bool {
			return reflect.DeepEqual(a, b)
		},
		Describe: func(state map[any]struct{}) string {
			return fmt.Sprintf("Set(%d items)", len(state))
		},
	}
}

// CounterModel returns a Model for an incrementable/readable monotonic counter.
func CounterModel() Model[int64] {
	return Model[int64]{
		Init: func() int64 { return 0 },
		Step: func(state int64, input any, output any) (bool, int64) {
			if inMap, ok := input.(map[string]any); ok {
				f, _ := inMap["f"].(string)
				f = strings.TrimPrefix(f, ":")
				if f == "add" {
					delta := int64(1)
					if v, ok := inMap["value"].(float64); ok {
						delta = int64(v)
					} else if v, ok := inMap["value"].(int); ok {
						delta = int64(v)
					} else if v, ok := inMap["value"].(int64); ok {
						delta = v
					}
					return true, state + delta
				}
				if f == "read" {
					var outVal int64
					if v, ok := output.(int64); ok {
						outVal = v
					} else if v, ok := output.(int); ok {
						outVal = int64(v)
					} else if v, ok := output.(float64); ok {
						outVal = int64(v)
					}
					return state == outVal, state
				}
			}
			if input != nil {
				delta := int64(1)
				if v, ok := input.(int64); ok {
					delta = v
				} else if v, ok := input.(int); ok {
					delta = int64(v)
				} else if v, ok := input.(float64); ok {
					delta = int64(v)
				}
				return true, state + delta
			}
			var outVal int64
			if v, ok := output.(int64); ok {
				outVal = v
			} else if v, ok := output.(int); ok {
				outVal = int64(v)
			} else if v, ok := output.(float64); ok {
				outVal = int64(v)
			}
			return state == outVal, state
		},
		Equal: func(a, b int64) bool {
			return a == b
		},
		Describe: func(state int64) string {
			return fmt.Sprintf("Counter(%d)", state)
		},
	}
}

// CASModel returns a Model for a compare-and-swap register.
func CASModel() Model[any] {
	return Model[any]{
		Init: func() any { return nil },
		Step: func(state any, input any, output any) (bool, any) {
			m, ok := input.(map[string]any)
			if !ok {
				if input == nil {
					return reflect.DeepEqual(state, output), state
				}
				return true, input
			}
			f, _ := m["f"].(string)
			f = strings.TrimPrefix(f, ":")
			if f == "read" {
				return reflect.DeepEqual(state, output), state
			}
			if f == "write" {
				return true, m["value"]
			}
			if f == "cas" {
				exp := m["expected"]
				tgt := m["target"]
				outBool, _ := output.(bool)
				if reflect.DeepEqual(state, exp) {
					if !outBool {
						return false, state
					}
					return true, tgt
				}
				if outBool {
					return false, state
				}
				return true, state
			}
			return false, state
		},
		Equal: func(a, b any) bool {
			return reflect.DeepEqual(a, b)
		},
		Describe: func(state any) string {
			return fmt.Sprintf("CAS(%v)", state)
		},
	}
}

// ParseJSONHistory parses an array of operations from a JSON stream.
func ParseJSONHistory(r io.Reader, maxBytes int64) ([]Op, error) {
	if maxBytes <= 0 {
		maxBytes = MaxJSONBytes
	}
	lr := io.LimitReader(r, maxBytes+1)
	data, err := io.ReadAll(lr)
	if err != nil {
		return nil, &CheckerError{Code: "input_too_large", Message: err.Error()}
	}
	if int64(len(data)) > maxBytes {
		return nil, &CheckerError{
			Code:    "input_too_large",
			Message: fmt.Sprintf("input size %d exceeds limit %d", len(data), maxBytes),
		}
	}

	var rawList []map[string]any
	if err := json.Unmarshal(data, &rawList); err != nil {
		return nil, &CheckerError{
			Code:    "invalid_history_sequence",
			Message: truncateString(err.Error(), 200),
		}
	}

	ops := make([]Op, 0, len(rawList))
	for idx, item := range rawList {
		op := Op{ID: idx}
		if idVal, ok := item["id"]; ok {
			if num, ok := idVal.(float64); ok {
				op.ID = int(num)
			}
		}
		if pVal, ok := item["process"]; ok {
			if num, ok := pVal.(float64); ok {
				op.Process = int(num)
			}
		}
		if kVal, ok := item["type"].(string); ok {
			op.Kind = Kind(strings.TrimPrefix(kVal, ":"))
		} else if kVal, ok := item["kind"].(string); ok {
			op.Kind = Kind(strings.TrimPrefix(kVal, ":"))
		}
		if keyVal, ok := item["key"].(string); ok {
			op.Key = keyVal
		}
		if inVal, ok := item["input"]; ok {
			op.Input = inVal
		}
		if outVal, ok := item["output"]; ok {
			op.Output = outVal
		}
		if ctVal, ok := item["call_time"]; ok {
			if num, ok := ctVal.(float64); ok {
				op.CallTime = int64(num)
			}
		} else if ctVal, ok := item["time"]; ok {
			if num, ok := ctVal.(float64); ok {
				op.CallTime = int64(num)
			}
		}
		if rtVal, ok := item["return_time"]; ok {
			if num, ok := rtVal.(float64); ok {
				op.ReturnTime = int64(num)
			}
		}

		if op.Kind == "" {
			op.Kind = KindOk
		}
		ops = append(ops, op)
	}
	return ops, nil
}

// RenderHTMLReport writes a self-contained HTML visualization of the counterexample report.
func RenderHTMLReport(w io.Writer, report *Report) error {
	if report == nil {
		return errors.New("report is nil")
	}
	sb := strings.Builder{}
	sb.WriteString("<!DOCTYPE html>\n<html>\n<head>\n<meta charset=\"utf-8\">\n")
	sb.WriteString("<title>Linearizability Checker Report</title>\n")
	sb.WriteString("<style>\n")
	sb.WriteString("body { font-family: monospace; padding: 20px; background: #fafafa; }\n")
	sb.WriteString(".card { background: white; border: 1px solid #ccc; padding: 15px; border-radius: 4px; }\n")
	sb.WriteString("table { border-collapse: collapse; width: 100%; margin-top: 10px; }\n")
	sb.WriteString("th, td { border: 1px solid #ddd; padding: 8px; text-align: left; }\n")
	sb.WriteString("th { background-color: #f2f2f2; }\n")
	sb.WriteString("</style>\n</head>\n<body>\n")
	sb.WriteString("<div class=\"card\">\n")
	fmt.Fprintf(&sb, "<h2>Non-Linearizable Execution Counterexample (Partition %d)</h2>\n", report.Partition)
	fmt.Fprintf(&sb, "<p>Minimal History Size: %d</p>\n", len(report.MinimalHistory))
	sb.WriteString("<table>\n<tr><th>ID</th><th>Process</th><th>Kind</th><th>Key</th><th>Input</th><th>Output</th><th>Call</th><th>Return</th></tr>\n")
	for _, op := range report.MinimalHistory {
		fmt.Fprintf(&sb, "<tr><td>%d</td><td>%d</td><td>%s</td><td>%s</td><td>%v</td><td>%v</td><td>%d</td><td>%d</td></tr>\n",
			op.ID, op.Process, op.Kind, op.Key, op.Input, op.Output, op.CallTime, op.ReturnTime)
	}
	sb.WriteString("</table>\n")
	sb.WriteString("</div>\n</body>\n</html>\n")
	_, err := w.Write([]byte(sb.String()))
	return err
}
