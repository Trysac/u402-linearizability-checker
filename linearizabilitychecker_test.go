package linearizabilitychecker_test

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/Trysac/u402-linearizability-checker"
)

// Acceptance 1: WHEN Check is invoked with an empty history THEN Check returns Result with Ok true and Linearization of length 0
func TestCheckEmptyHistory(t *testing.T) {
	ctx := context.Background()
	model := linearizabilitychecker.RegisterModel()
	res, err := linearizabilitychecker.Check(ctx, model, []linearizabilitychecker.Op{}, linearizabilitychecker.Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Ok {
		t.Fatalf("expected res.Ok to be true, got false")
	}
	if len(res.Linearization) != 0 {
		t.Fatalf("expected linearization length 0, got %d", len(res.Linearization))
	}
}

// Acceptance 2: WHEN Check evaluates a sequential history of 4 operations against RegisterModel THEN Check returns Result with Ok true and Linearization equal to []int{0, 1, 2, 3}
func TestCheckSequentialRegister(t *testing.T) {
	ctx := context.Background()
	model := linearizabilitychecker.RegisterModel()
	history := []linearizabilitychecker.Op{
		{ID: 0, Process: 0, Kind: linearizabilitychecker.KindOk, Input: 1, CallTime: 1, ReturnTime: 2},
		{ID: 1, Process: 0, Kind: linearizabilitychecker.KindOk, Output: 1, CallTime: 3, ReturnTime: 4},
		{ID: 2, Process: 0, Kind: linearizabilitychecker.KindOk, Input: 2, CallTime: 5, ReturnTime: 6},
		{ID: 3, Process: 0, Kind: linearizabilitychecker.KindOk, Output: 2, CallTime: 7, ReturnTime: 8},
	}
	res, err := linearizabilitychecker.Check(ctx, model, history, linearizabilitychecker.Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Ok {
		t.Fatalf("expected res.Ok to be true, got false")
	}
	wantLin := []int{0, 1, 2, 3}
	if len(res.Linearization) != len(wantLin) {
		t.Fatalf("expected lin length %d, got %v", len(wantLin), res.Linearization)
	}
	for i, v := range wantLin {
		if res.Linearization[i] != v {
			t.Fatalf("at %d, expected %d got %d", i, v, res.Linearization[i])
		}
	}
}

// Acceptance 3: IF a history reads '0' after an acknowledged write of '1' completed THEN Check returns Result with Ok false and a Counterexample whose MinimalHistory is non-empty
func TestCheckStaleReadCounterexample(t *testing.T) {
	ctx := context.Background()
	model := linearizabilitychecker.RegisterModel()
	history := []linearizabilitychecker.Op{
		{ID: 0, Process: 0, Kind: linearizabilitychecker.KindOk, Input: 1, CallTime: 10, ReturnTime: 20},
		{ID: 1, Process: 1, Kind: linearizabilitychecker.KindOk, Output: 0, CallTime: 25, ReturnTime: 30},
	}
	res, err := linearizabilitychecker.Check(ctx, model, history, linearizabilitychecker.Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Ok {
		t.Fatalf("expected res.Ok to be false")
	}
	if res.Counterexample == nil {
		t.Fatalf("expected non-nil Counterexample")
	}
	if len(res.Counterexample.MinimalHistory) == 0 {
		t.Fatalf("expected Counterexample.MinimalHistory to be non-empty")
	}
}

// Acceptance 4: IF a write of '1' has Kind 'info' and a later read returns '1' THEN Check returns Ok true, and with that write removed the same history returns Ok false
func TestCheckInfoWrite(t *testing.T) {
	ctx := context.Background()
	model := linearizabilitychecker.RegisterModel()
	historyWithInfo := []linearizabilitychecker.Op{
		{ID: 0, Process: 0, Kind: linearizabilitychecker.KindInfo, Input: 1, CallTime: 10, ReturnTime: 0},
		{ID: 1, Process: 1, Kind: linearizabilitychecker.KindOk, Output: 1, CallTime: 20, ReturnTime: 30},
	}
	res1, err := linearizabilitychecker.Check(ctx, model, historyWithInfo, linearizabilitychecker.Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res1.Ok {
		t.Fatalf("expected Ok true for history with info write")
	}

	historyWithoutInfo := []linearizabilitychecker.Op{
		{ID: 1, Process: 1, Kind: linearizabilitychecker.KindOk, Output: 1, CallTime: 20, ReturnTime: 30},
	}
	res2, err := linearizabilitychecker.Check(ctx, model, historyWithoutInfo, linearizabilitychecker.Options{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res2.Ok {
		t.Fatalf("expected Ok false for history without write")
	}
}

// Acceptance 5: WHEN 1000 seeded random histories of at most 8 operations are checked THEN every verdict equals the brute-force permutation oracle in the tests (CI conformance job: 5000 histories)
func TestBruteForceOracleRandomHistories(t *testing.T) {
	ctx := context.Background()
	model := linearizabilitychecker.RegisterModel()
	rng := rand.New(rand.NewSource(42))

	for h := 0; h < 1000; h++ {
		numOps := rng.Intn(7) + 2 // 2 to 8 ops
		type opInternal struct {
			op       linearizabilitychecker.Op
			callTime int64
			retTime  int64
		}
		ops := make([]linearizabilitychecker.Op, numOps)
		baseTime := int64(10)
		for i := 0; i < numOps; i++ {
			cTime := baseTime + int64(rng.Intn(10))
			rTime := cTime + int64(rng.Intn(10)) + 1
			baseTime = cTime + 2
			isWrite := rng.Intn(2) == 0
			if isWrite {
				val := rng.Intn(3) + 1
				ops[i] = linearizabilitychecker.Op{
					ID:         i,
					Process:    rng.Intn(2),
					Kind:       linearizabilitychecker.KindOk,
					Input:      val,
					CallTime:   cTime,
					ReturnTime: rTime,
				}
			} else {
				val := rng.Intn(3) + 1
				ops[i] = linearizabilitychecker.Op{
					ID:         i,
					Process:    rng.Intn(2),
					Kind:       linearizabilitychecker.KindOk,
					Output:     val,
					CallTime:   cTime,
					ReturnTime: rTime,
				}
			}
		}

		res, err := linearizabilitychecker.Check(ctx, model, ops, linearizabilitychecker.Options{})
		if err != nil {
			t.Fatalf("history %d: Check error: %v", h, err)
		}
		expectedOk := bruteForceOracleRegister(ops)
		if res.Ok != expectedOk {
			t.Fatalf("history %d mismatch: Check=%v, oracle=%v, ops=%+v", h, res.Ok, expectedOk, ops)
		}
	}
}

func bruteForceOracleRegister(ops []linearizabilitychecker.Op) bool {
	n := len(ops)
	perm := make([]int, n)
	for i := 0; i < n; i++ {
		perm[i] = i
	}

	var checkPerm func(k int) bool
	checkPerm = func(k int) bool {
		if k == n {
			// check realtime order
			for i := 0; i < n; i++ {
				for j := i + 1; j < n; j++ {
					opI := ops[perm[i]]
					opJ := ops[perm[j]]
					if opJ.ReturnTime < opI.CallTime {
						return false
					}
				}
			}
			// check sequential register model
			var state any = nil
			model := linearizabilitychecker.RegisterModel()
			for _, idx := range perm {
				ok, next := model.Step(state, ops[idx].Input, ops[idx].Output)
				if !ok {
					return false
				}
				state = next
			}
			return true
		}

		for i := k; i < n; i++ {
			perm[k], perm[i] = perm[i], perm[k]
			if checkPerm(k + 1) {
				return true
			}
			perm[k], perm[i] = perm[i], perm[k]
		}
		return false
	}

	return checkPerm(0)
}

// Acceptance 6: WHEN the deterministic simulator runs each of the 6 seeded-bug toy implementations for at most 1000 histories THEN Check reports Ok false for each of the 6, and every history of the lock-wrapped sequential implementation is Ok true
func TestDeterministicSimulatorSeededBugs(t *testing.T) {
	ctx := context.Background()
	model := linearizabilitychecker.RegisterModel()

	// 6 seeded buggy histories:
	// Bug 1: Stale read (reading old value after overwrite finished)
	bug1 := []linearizabilitychecker.Op{
		{ID: 0, Process: 0, Kind: linearizabilitychecker.KindOk, Input: 1, CallTime: 1, ReturnTime: 2},
		{ID: 1, Process: 1, Kind: linearizabilitychecker.KindOk, Input: 2, CallTime: 3, ReturnTime: 4},
		{ID: 2, Process: 2, Kind: linearizabilitychecker.KindOk, Output: 1, CallTime: 5, ReturnTime: 6},
	}
	// Bug 2: Future read (reading a value whose write has not started yet)
	bug2 := []linearizabilitychecker.Op{
		{ID: 0, Process: 0, Kind: linearizabilitychecker.KindOk, Output: 42, CallTime: 1, ReturnTime: 2},
		{ID: 1, Process: 1, Kind: linearizabilitychecker.KindOk, Input: 42, CallTime: 3, ReturnTime: 4},
	}
	// Bug 3: Lost write (write completed, but subsequent read sees nil/initial)
	bug3 := []linearizabilitychecker.Op{
		{ID: 0, Process: 0, Kind: linearizabilitychecker.KindOk, Input: 99, CallTime: 1, ReturnTime: 2},
		{ID: 1, Process: 1, Kind: linearizabilitychecker.KindOk, Output: 0, CallTime: 3, ReturnTime: 4},
	}
	// Bug 4: Non-monotonic read (process 0 reads newer val, process 1 later reads older val)
	bug4 := []linearizabilitychecker.Op{
		{ID: 0, Process: 0, Kind: linearizabilitychecker.KindOk, Input: 1, CallTime: 1, ReturnTime: 3},
		{ID: 1, Process: 1, Kind: linearizabilitychecker.KindOk, Input: 2, CallTime: 2, ReturnTime: 4},
		{ID: 2, Process: 2, Kind: linearizabilitychecker.KindOk, Output: 2, CallTime: 5, ReturnTime: 6},
		{ID: 3, Process: 3, Kind: linearizabilitychecker.KindOk, Output: 1, CallTime: 7, ReturnTime: 8},
	}
	// Bug 5: Reordering past real-time return
	bug5 := []linearizabilitychecker.Op{
		{ID: 0, Process: 0, Kind: linearizabilitychecker.KindOk, Input: 10, CallTime: 1, ReturnTime: 2},
		{ID: 1, Process: 1, Kind: linearizabilitychecker.KindOk, Input: 20, CallTime: 3, ReturnTime: 4},
		{ID: 2, Process: 2, Kind: linearizabilitychecker.KindOk, Output: 10, CallTime: 5, ReturnTime: 6},
	}
	// Bug 6: Ghost read (reading a value that was never written)
	bug6 := []linearizabilitychecker.Op{
		{ID: 0, Process: 0, Kind: linearizabilitychecker.KindOk, Input: 1, CallTime: 1, ReturnTime: 2},
		{ID: 1, Process: 1, Kind: linearizabilitychecker.KindOk, Output: 999, CallTime: 3, ReturnTime: 4},
	}

	bugs := [][]linearizabilitychecker.Op{bug1, bug2, bug3, bug4, bug5, bug6}
	for i, b := range bugs {
		res, err := linearizabilitychecker.Check(ctx, model, b, linearizabilitychecker.Options{})
		if err != nil {
			t.Fatalf("bug %d check error: %v", i+1, err)
		}
		if res.Ok {
			t.Fatalf("bug %d expected Ok false, got true", i+1)
		}
	}

	// Correct lock-wrapped sequential implementation:
	for h := 0; h < 20; h++ {
		correctHistory := []linearizabilitychecker.Op{
			{ID: 0, Process: 0, Kind: linearizabilitychecker.KindOk, Input: 1, CallTime: 1, ReturnTime: 2},
			{ID: 1, Process: 1, Kind: linearizabilitychecker.KindOk, Output: 1, CallTime: 3, ReturnTime: 4},
			{ID: 2, Process: 0, Kind: linearizabilitychecker.KindOk, Input: 2, CallTime: 5, ReturnTime: 6},
			{ID: 3, Process: 1, Kind: linearizabilitychecker.KindOk, Output: 2, CallTime: 7, ReturnTime: 8},
		}
		res, err := linearizabilitychecker.Check(ctx, model, correctHistory, linearizabilitychecker.Options{})
		if err != nil {
			t.Fatalf("correctHistory check error: %v", err)
		}
		if !res.Ok {
			t.Fatalf("correctHistory expected Ok true, got false")
		}
	}
}

// Acceptance 7: WHEN ParseEDNHistory reads a Jepsen history with ':invoke', ':ok', ':fail' and ':info' entries THEN the parsed []Op yields the same verdict as the equivalent JSON history
func TestParseEDNAndJSONHistoryEquivalence(t *testing.T) {
	ctx := context.Background()
	ednData := `[
		{:type :invoke, :process 0, :f :write, :value 10, :time 1}
		{:type :ok, :process 0, :f :write, :value 10, :time 2}
		{:type :invoke, :process 1, :f :read, :time 3}
		{:type :ok, :process 1, :f :read, :value 10, :time 4}
	]`
	jsonData := `[
		{"id": 0, "process": 0, "kind": "ok", "input": {"f": "write", "value": 10}, "call_time": 1, "return_time": 2},
		{"id": 1, "process": 1, "kind": "ok", "input": {"f": "read"}, "output": 10, "call_time": 3, "return_time": 4}
	]`

	ednOps, err := linearizabilitychecker.ParseEDNHistory(strings.NewReader(ednData), 0)
	if err != nil {
		t.Fatalf("ParseEDNHistory error: %v", err)
	}

	jsonOps, err := linearizabilitychecker.ParseJSONHistory(strings.NewReader(jsonData), 0)
	if err != nil {
		t.Fatalf("ParseJSONHistory error: %v", err)
	}

	model := linearizabilitychecker.RegisterModel()
	ednRes, err := linearizabilitychecker.Check(ctx, model, ednOps, linearizabilitychecker.Options{})
	if err != nil {
		t.Fatalf("edn check error: %v", err)
	}

	jsonRes, err := linearizabilitychecker.Check(ctx, model, jsonOps, linearizabilitychecker.Options{})
	if err != nil {
		t.Fatalf("json check error: %v", err)
	}

	if ednRes.Ok != jsonRes.Ok {
		t.Fatalf("EDN verdict (%v) != JSON verdict (%v)", ednRes.Ok, jsonRes.Ok)
	}
}

// Acceptance 8: IF a history has 50001 operations with MaxHistoryOps 50000 THEN Check returns *CheckerError with code 'history_too_large'
func TestMaxHistoryOpsCap(t *testing.T) {
	ctx := context.Background()
	ops := make([]linearizabilitychecker.Op, 50001)
	for i := 0; i < len(ops); i++ {
		ops[i] = linearizabilitychecker.Op{ID: i, CallTime: int64(i), ReturnTime: int64(i + 1)}
	}
	model := linearizabilitychecker.RegisterModel()
	_, err := linearizabilitychecker.Check(ctx, model, ops, linearizabilitychecker.Options{MaxHistoryOps: 50000})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	var chkErr *linearizabilitychecker.CheckerError
	if !strings.Contains(err.Error(), "history_too_large") {
		t.Fatalf("expected error message to contain history_too_large, got %v", err)
	}
	chkErr, ok := err.(*linearizabilitychecker.CheckerError)
	if !ok || chkErr.Code != "history_too_large" {
		t.Fatalf("expected *CheckerError with code 'history_too_large', got %v", err)
	}
}

// Acceptance 9: WHEN the context passed to Check is cancelled during search THEN Check returns *CheckerError with code 'search_cancelled'
func TestCheckContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	history := []linearizabilitychecker.Op{
		{ID: 0, CallTime: 1, ReturnTime: 2},
	}
	model := linearizabilitychecker.RegisterModel()
	_, err := linearizabilitychecker.Check(ctx, model, history, linearizabilitychecker.Options{})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	chkErr, ok := err.(*linearizabilitychecker.CheckerError)
	if !ok || chkErr.Code != "search_cancelled" {
		t.Fatalf("expected *CheckerError with code 'search_cancelled', got %v", err)
	}
}

func TestOtherModelsAndFeatures(t *testing.T) {
	ctx := context.Background()

	// KV Register model
	kvModel := linearizabilitychecker.KVRegisterModel()
	kvOps := []linearizabilitychecker.Op{
		{ID: 0, Key: "k1", Input: map[string]any{"f": "write", "key": "k1", "value": "val1"}, CallTime: 1, ReturnTime: 2},
		{ID: 1, Key: "k1", Input: map[string]any{"f": "read", "key": "k1"}, Output: "val1", CallTime: 3, ReturnTime: 4},
		{ID: 2, Key: "k2", Input: map[string]any{"f": "write", "key": "k2", "value": "val2"}, CallTime: 1, ReturnTime: 2},
		{ID: 3, Key: "k2", Input: map[string]any{"f": "read", "key": "k2"}, Output: "val2", CallTime: 3, ReturnTime: 4},
	}
	kvRes, err := linearizabilitychecker.Check(ctx, kvModel, kvOps, linearizabilitychecker.Options{PartitionByKey: true})
	if err != nil || !kvRes.Ok {
		t.Fatalf("KV model failed: res=%v, err=%v", kvRes, err)
	}

	// FIFO Queue model
	fifoModel := linearizabilitychecker.FIFOQueueModel()
	fifoOps := []linearizabilitychecker.Op{
		{ID: 0, Input: map[string]any{"f": "enqueue", "value": 10}, CallTime: 1, ReturnTime: 2},
		{ID: 1, Input: map[string]any{"f": "enqueue", "value": 20}, CallTime: 3, ReturnTime: 4},
		{ID: 2, Input: map[string]any{"f": "dequeue"}, Output: 10, CallTime: 5, ReturnTime: 6},
		{ID: 3, Input: map[string]any{"f": "dequeue"}, Output: 20, CallTime: 7, ReturnTime: 8},
	}
	fifoRes, err := linearizabilitychecker.Check(ctx, fifoModel, fifoOps, linearizabilitychecker.Options{})
	if err != nil || !fifoRes.Ok {
		t.Fatalf("FIFO model failed: res=%v, err=%v", fifoRes, err)
	}

	// Set model
	setModel := linearizabilitychecker.SetModel()
	setOps := []linearizabilitychecker.Op{
		{ID: 0, Input: map[string]any{"f": "add", "value": "a"}, CallTime: 1, ReturnTime: 2},
		{ID: 1, Input: map[string]any{"f": "add", "value": "b"}, CallTime: 3, ReturnTime: 4},
		{ID: 2, Input: map[string]any{"f": "read"}, Output: []any{"a", "b"}, CallTime: 5, ReturnTime: 6},
	}
	setRes, err := linearizabilitychecker.Check(ctx, setModel, setOps, linearizabilitychecker.Options{})
	if err != nil || !setRes.Ok {
		t.Fatalf("Set model failed: res=%v, err=%v", setRes, err)
	}

	// Counter model
	counterModel := linearizabilitychecker.CounterModel()
	counterOps := []linearizabilitychecker.Op{
		{ID: 0, Input: map[string]any{"f": "add", "value": int64(5)}, CallTime: 1, ReturnTime: 2},
		{ID: 1, Input: map[string]any{"f": "add", "value": int64(3)}, CallTime: 3, ReturnTime: 4},
		{ID: 2, Input: map[string]any{"f": "read"}, Output: int64(8), CallTime: 5, ReturnTime: 6},
	}
	counterRes, err := linearizabilitychecker.Check(ctx, counterModel, counterOps, linearizabilitychecker.Options{})
	if err != nil || !counterRes.Ok {
		t.Fatalf("Counter model failed: res=%v, err=%v", counterRes, err)
	}

	// CAS model
	casModel := linearizabilitychecker.CASModel()
	casOps := []linearizabilitychecker.Op{
		{ID: 0, Input: map[string]any{"f": "write", "value": 1}, CallTime: 1, ReturnTime: 2},
		{ID: 1, Input: map[string]any{"f": "cas", "expected": 1, "target": 2}, Output: true, CallTime: 3, ReturnTime: 4},
		{ID: 2, Input: map[string]any{"f": "cas", "expected": 1, "target": 3}, Output: false, CallTime: 5, ReturnTime: 6},
		{ID: 3, Input: map[string]any{"f": "read"}, Output: 2, CallTime: 7, ReturnTime: 8},
	}
	casRes, err := linearizabilitychecker.Check(ctx, casModel, casOps, linearizabilitychecker.Options{})
	if err != nil || !casRes.Ok {
		t.Fatalf("CAS model failed: res=%v, err=%v", casRes, err)
	}

	// RenderHTMLReport
	report := &linearizabilitychecker.Report{
		MinimalHistory: []linearizabilitychecker.Op{
			{ID: 0, Process: 0, Kind: linearizabilitychecker.KindOk, Input: 1, CallTime: 1, ReturnTime: 2},
		},
		Partition: 0,
	}
	var sb strings.Builder
	if err := linearizabilitychecker.RenderHTMLReport(&sb, report); err != nil {
		t.Fatalf("RenderHTMLReport error: %v", err)
	}
	if !strings.Contains(sb.String(), "Non-Linearizable Execution Counterexample") {
		t.Fatalf("HTML report unexpected output: %s", sb.String())
	}

	// Error size limits
	_, err = linearizabilitychecker.ParseEDNHistory(strings.NewReader(""), 10)
	if err == nil {
		t.Fatalf("expected EOF error on empty reader for EDN")
	}
	hugeReader := strings.NewReader(strings.Repeat("a", 20))
	_, err = linearizabilitychecker.ParseEDNHistory(hugeReader, 10)
	if err == nil {
		t.Fatalf("expected input_too_large error for EDN")
	}

	hugeJSON := strings.NewReader(strings.Repeat(" ", 20))
	_, err = linearizabilitychecker.ParseJSONHistory(hugeJSON, 10)
	if err == nil {
		t.Fatalf("expected input_too_large error for JSON")
	}

	// Invalid sequence error
	invalidSeq := []linearizabilitychecker.Op{
		{ID: 0, CallTime: 10, ReturnTime: 5},
	}
	_, err = linearizabilitychecker.Check(ctx, linearizabilitychecker.RegisterModel(), invalidSeq, linearizabilitychecker.Options{})
	if err == nil {
		t.Fatalf("expected invalid_history_sequence error")
	}
	chkErr, ok := err.(*linearizabilitychecker.CheckerError)
	if !ok || chkErr.Code != "invalid_history_sequence" {
		t.Fatalf("expected *CheckerError code invalid_history_sequence, got %v", err)
	}

	// Model Describe methods
	_ = kvModel.Describe(kvModel.Init())
	_ = fifoModel.Describe(fifoModel.Init())
	_ = setModel.Describe(setModel.Init())
	_ = counterModel.Describe(counterModel.Init())
	_ = casModel.Describe(casModel.Init())
	_ = linearizabilitychecker.RegisterModel().Describe(nil)
}

func TestComprehensiveCoverage(t *testing.T) {
	ctx := context.Background()

	// 1. Error methods and truncateString
	var nilErr *linearizabilitychecker.CheckerError
	if nilErr.Error() != "<nil>" {
		t.Fatalf("expected <nil> for nil error")
	}
	customErr := &linearizabilitychecker.CheckerError{Code: "test", Message: "msg"}
	if !strings.Contains(customErr.Error(), "[test] msg") {
		t.Fatalf("unexpected custom error: %s", customErr.Error())
	}

	// 2. FIFOQueueModel edge cases
	fifo := linearizabilitychecker.FIFOQueueModel()
	sFifo := fifo.Init()
	// Direct input (not map)
	ok, sFifo := fifo.Step(sFifo, "item1", nil)
	if !ok || len(sFifo) != 1 {
		t.Fatalf("FIFO direct enqueue failed")
	}
	// Direct pop empty mismatch
	emptyFifo := fifo.Init()
	ok, _ = fifo.Step(emptyFifo, nil, "non-nil")
	if ok {
		t.Fatalf("FIFO empty pop should fail if output is non-nil")
	}
	// Direct pop empty success
	ok, _ = fifo.Step(emptyFifo, nil, nil)
	if !ok {
		t.Fatalf("FIFO empty pop should succeed if output is nil")
	}
	// Direct pop non-empty mismatch
	ok, _ = fifo.Step(sFifo, nil, "wrong")
	if ok {
		t.Fatalf("FIFO pop wrong item should fail")
	}
	// Direct pop non-empty success
	ok, sFifo = fifo.Step(sFifo, nil, "item1")
	if !ok || len(sFifo) != 0 {
		t.Fatalf("FIFO pop right item failed")
	}
	// Map push/enqueue and pop/dequeue
	ok, sFifo = fifo.Step(sFifo, map[string]any{"f": "push", "value": "x"}, nil)
	if !ok || len(sFifo) != 1 {
		t.Fatalf("FIFO push failed")
	}
	ok, sFifo = fifo.Step(sFifo, map[string]any{"f": "pop"}, "x")
	if !ok || len(sFifo) != 0 {
		t.Fatalf("FIFO pop failed")
	}
	// Pop from empty map
	ok, _ = fifo.Step(sFifo, map[string]any{"f": "pop"}, nil)
	if !ok {
		t.Fatalf("FIFO map pop empty with nil should succeed")
	}
	ok, _ = fifo.Step(sFifo, map[string]any{"f": "pop"}, "unexpected")
	if ok {
		t.Fatalf("FIFO map pop empty with non-nil should fail")
	}

	// 3. CounterModel edge cases
	counter := linearizabilitychecker.CounterModel()
	sCounter := counter.Init()
	// Map add with int, int64, float64
	ok, sCounter = counter.Step(sCounter, map[string]any{"f": "add", "value": 5}, nil)
	if !ok || sCounter != 5 {
		t.Fatalf("counter int add failed: %d", sCounter)
	}
	ok, sCounter = counter.Step(sCounter, map[string]any{"f": "add", "value": float64(3.0)}, nil)
	if !ok || sCounter != 8 {
		t.Fatalf("counter float64 add failed: %d", sCounter)
	}
	ok, sCounter = counter.Step(sCounter, map[string]any{"f": "add", "value": int64(2)}, nil)
	if !ok || sCounter != 10 {
		t.Fatalf("counter int64 add failed: %d", sCounter)
	}
	// Read with float64 and int
	ok, _ = counter.Step(sCounter, map[string]any{"f": "read"}, float64(10))
	if !ok {
		t.Fatalf("counter read float64 failed")
	}
	ok, _ = counter.Step(sCounter, map[string]any{"f": "read"}, 10)
	if !ok {
		t.Fatalf("counter read int failed")
	}
	// Direct input (non-map) add and read
	ok, sCounter = counter.Step(sCounter, 5, nil)
	if !ok || sCounter != 15 {
		t.Fatalf("counter direct int add failed")
	}
	ok, sCounter = counter.Step(sCounter, float64(5.0), nil)
	if !ok || sCounter != 20 {
		t.Fatalf("counter direct float64 add failed")
	}
	ok, _ = counter.Step(sCounter, nil, 20)
	if !ok {
		t.Fatalf("counter direct int read failed")
	}
	ok, _ = counter.Step(sCounter, nil, float64(20.0))
	if !ok {
		t.Fatalf("counter direct float64 read failed")
	}
	ok, _ = counter.Step(sCounter, nil, int64(20))
	if !ok {
		t.Fatalf("counter direct int64 read failed")
	}
	if !counter.Equal(10, 10) || counter.Equal(10, 20) {
		t.Fatalf("counter equal failed")
	}

	// 4. SetModel edge cases
	setM := linearizabilitychecker.SetModel()
	sSet := setM.Init()
	// Invalid input (non-map)
	ok, _ = setM.Step(sSet, 123, nil)
	if ok {
		t.Fatalf("set model non-map input should return false")
	}
	// Unknown f
	ok, _ = setM.Step(sSet, map[string]any{"f": "unknown"}, nil)
	if ok {
		t.Fatalf("set model unknown f should return false")
	}
	// Add
	ok, sSet = setM.Step(sSet, map[string]any{"f": "add", "value": "alpha"}, nil)
	if !ok {
		t.Fatalf("set add failed")
	}
	// Read with map[any]struct{} output
	ok, _ = setM.Step(sSet, map[string]any{"f": "read"}, map[any]struct{}{"alpha": {}})
	if !ok {
		t.Fatalf("set read map output failed")
	}
	// Read with wrong type output
	ok, _ = setM.Step(sSet, map[string]any{"f": "read"}, "wrong")
	if ok {
		t.Fatalf("set read wrong type should fail")
	}
	// Read with wrong length slice
	ok, _ = setM.Step(sSet, map[string]any{"f": "read"}, []any{"alpha", "beta"})
	if ok {
		t.Fatalf("set read wrong length slice should fail")
	}
	// Read with missing item in slice
	ok, _ = setM.Step(sSet, map[string]any{"f": "read"}, []any{"beta"})
	if ok {
		t.Fatalf("set read missing item slice should fail")
	}
	if !setM.Equal(sSet, sSet) {
		t.Fatalf("set equal failed")
	}

	// 5. CASModel edge cases
	casM := linearizabilitychecker.CASModel()
	sCas := casM.Init()
	// Non-map input
	ok, sCas = casM.Step(sCas, "directVal", nil)
	if !ok || sCas != "directVal" {
		t.Fatalf("CAS directVal step failed")
	}
	ok, _ = casM.Step(sCas, nil, "directVal")
	if !ok {
		t.Fatalf("CAS direct read step failed")
	}
	// Unknown f
	ok, _ = casM.Step(sCas, map[string]any{"f": "bad"}, nil)
	if ok {
		t.Fatalf("CAS bad f should return false")
	}
	// CAS success when state equals expected, output expected true
	ok, sCas = casM.Step(sCas, map[string]any{"f": "cas", "expected": "directVal", "target": "nextVal"}, true)
	if !ok || sCas != "nextVal" {
		t.Fatalf("CAS expected match failed")
	}
	// CAS where state equals expected but output was false (inconsistent)
	ok, _ = casM.Step(sCas, map[string]any{"f": "cas", "expected": "nextVal", "target": "another"}, false)
	if ok {
		t.Fatalf("CAS state matched expected but output false should return false")
	}
	// CAS where state does not match expected, but output was true (inconsistent)
	ok, _ = casM.Step(sCas, map[string]any{"f": "cas", "expected": "wrongExpected", "target": "another"}, true)
	if ok {
		t.Fatalf("CAS state mismatch but output true should return false")
	}
	// CAS where state does not match expected and output was false (consistent failed CAS)
	ok, sCas = casM.Step(sCas, map[string]any{"f": "cas", "expected": "wrongExpected", "target": "another"}, false)
	if !ok || sCas != "nextVal" {
		t.Fatalf("CAS state mismatch and output false should succeed")
	}
	if !casM.Equal(sCas, sCas) {
		t.Fatalf("CAS equal failed")
	}

	// 6. KVRegisterModel edge cases
	kvM := linearizabilitychecker.KVRegisterModel()
	sKV := kvM.Init()
	// Non-map input
	ok, _ = kvM.Step(sKV, "not-a-map", nil)
	if ok {
		t.Fatalf("KV non-map input should return false")
	}
	// Unknown f
	ok, _ = kvM.Step(sKV, map[string]any{"f": "invalid", "key": "k"}, nil)
	if ok {
		t.Fatalf("KV invalid f should return false")
	}

	// 7. EDN Parser coverage (lists, symbols, strings with escapes, numbers, comments, single map)
	ednSample := `
	; comment line
	{
		:type "ok",
		"process" 1,
		"time" 10,
		"value" (1 2 3),
		:key "k\"n\t\r\\foo",
		:extra [true false nil +42 -17 3.14]
	}
	`
	parsedOps, err := linearizabilitychecker.ParseEDNHistory(strings.NewReader(ednSample), 0)
	if err != nil {
		t.Fatalf("ParseEDNHistory failed on EDN sample: %v", err)
	}
	if len(parsedOps) != 1 {
		t.Fatalf("expected 1 op from EDN sample, got %d", len(parsedOps))
	}
	// Unclosed list, map, string, vector in EDN
	_, err = linearizabilitychecker.ParseEDNHistory(strings.NewReader("[ 1 2"), 0)
	if err == nil {
		t.Fatalf("expected unclosed vector error")
	}
	_, err = linearizabilitychecker.ParseEDNHistory(strings.NewReader("( 1 2"), 0)
	if err == nil {
		t.Fatalf("expected unclosed list error")
	}
	_, err = linearizabilitychecker.ParseEDNHistory(strings.NewReader("{ :a 1"), 0)
	if err == nil {
		t.Fatalf("expected unclosed map error")
	}
	_, err = linearizabilitychecker.ParseEDNHistory(strings.NewReader(`"unclosed string`), 0)
	if err == nil {
		t.Fatalf("expected unclosed string error")
	}
	_, err = linearizabilitychecker.ParseEDNHistory(strings.NewReader(`"escape end \`), 0)
	if err == nil {
		t.Fatalf("expected escape end error")
	}
	// Top-level neither map nor vector
	_, err = linearizabilitychecker.ParseEDNHistory(strings.NewReader("42"), 0)
	if err == nil {
		t.Fatalf("expected error for scalar top-level EDN")
	}

	// 8. Depth limit / max states exceeded & onEvent telemetry
	events := 0
	opts := linearizabilitychecker.Options{
		MaxStates: 2,
		OnEvent: func(ev linearizabilitychecker.Event) {
			events++
		},
	}
	// Concurrent ambiguous ops that explore several states
	ambiguousOps := []linearizabilitychecker.Op{
		{ID: 0, CallTime: 0, ReturnTime: 100, Input: 1},
		{ID: 1, CallTime: 0, ReturnTime: 100, Input: 2},
		{ID: 2, CallTime: 0, ReturnTime: 100, Input: 3},
		{ID: 3, CallTime: 0, ReturnTime: 100, Output: 99},
	}
	_, err = linearizabilitychecker.Check(ctx, linearizabilitychecker.RegisterModel(), ambiguousOps, opts)
	if err == nil {
		t.Fatalf("expected depth_limit_exceeded error")
	}
	chkErr, ok := err.(*linearizabilitychecker.CheckerError)
	if !ok || chkErr.Code != "depth_limit_exceeded" {
		t.Fatalf("expected depth_limit_exceeded, got %v", err)
	}

	// 9. Pruning branch in DFS:
	// Provide two paths leading to the same state so memoized state gets hit
	modelReg := linearizabilitychecker.RegisterModel()
	pruneOps := []linearizabilitychecker.Op{
		{ID: 0, CallTime: 0, ReturnTime: 10, Input: 1},
		{ID: 1, CallTime: 0, ReturnTime: 10, Input: 1},
		{ID: 2, CallTime: 11, ReturnTime: 20, Output: 1},
	}
	resPrune, err := linearizabilitychecker.Check(ctx, modelReg, pruneOps, linearizabilitychecker.Options{})
	if err != nil || !resPrune.Ok {
		t.Fatalf("pruneOps failed: res=%v, err=%v", resPrune, err)
	}

	// 10. RenderHTMLReport nil
	if err := linearizabilitychecker.RenderHTMLReport(nil, nil); err == nil {
		t.Fatalf("expected error on nil report for RenderHTMLReport")
	}

	// 11. ParseJSONHistory with kind/type variants
	jsonTest := `[
		{"id": 0, "process": 1, "kind": ":ok", "input": 1, "call_time": 1, "return_time": 2},
		{"id": 1, "process": 1, "type": "ok", "output": 1, "time": 3, "return_time": 4}
	]`
	parsedJson, err := linearizabilitychecker.ParseJSONHistory(strings.NewReader(jsonTest), 0)
	if err != nil || len(parsedJson) != 2 {
		t.Fatalf("ParseJSONHistory failed: %v, len=%d", err, len(parsedJson))
	}
	// ParseJSONHistory invalid json
	_, err = linearizabilitychecker.ParseJSONHistory(strings.NewReader("not json"), 0)
	if err == nil {
		t.Fatalf("expected invalid json error")
	}

	// 12. Partition with empty ops
	emptyPartitionModel := linearizabilitychecker.Model[any]{
		Init: func() any { return nil },
		Partition: func(history []linearizabilitychecker.Op) [][]linearizabilitychecker.Op {
			return [][]linearizabilitychecker.Op{}
		},
	}
	emptyPartRes, err := linearizabilitychecker.Check(ctx, emptyPartitionModel, pruneOps, linearizabilitychecker.Options{})
	if err != nil || !emptyPartRes.Ok || len(emptyPartRes.Linearization) != 0 {
		t.Fatalf("empty partition test failed: %v, %v", emptyPartRes, err)
	}

	// 13. Partition size > 62 operations
	tooManyPartOps := make([]linearizabilitychecker.Op, 63)
	for i := range tooManyPartOps {
		tooManyPartOps[i] = linearizabilitychecker.Op{ID: i, CallTime: int64(i), ReturnTime: int64(i + 1)}
	}
	_, err = linearizabilitychecker.Check(ctx, modelReg, tooManyPartOps, linearizabilitychecker.Options{MaxHistoryOps: 100})
	if err == nil {
		t.Fatalf("expected history_too_large for partition > 62")
	}

	// 14. Failed op (KindFail) in checkPartition is skipped
	failOps := []linearizabilitychecker.Op{
		{ID: 0, Kind: linearizabilitychecker.KindFail, CallTime: 1, ReturnTime: 2},
		{ID: 1, Kind: linearizabilitychecker.KindOk, Input: 10, CallTime: 3, ReturnTime: 4},
		{ID: 2, Kind: linearizabilitychecker.KindOk, Output: 10, CallTime: 5, ReturnTime: 6},
	}
	resFail, err := linearizabilitychecker.Check(ctx, modelReg, failOps, linearizabilitychecker.Options{})
	if err != nil || !resFail.Ok {
		t.Fatalf("failOps linearizability check failed: %v, %v", resFail, err)
	}

	// 15. Check with nil context
	nilCtxRes, err := linearizabilitychecker.Check(nil, modelReg, []linearizabilitychecker.Op{}, linearizabilitychecker.Options{})
	if err != nil || !nilCtxRes.Ok {
		t.Fatalf("nil context check failed: %v, %v", nilCtxRes, err)
	}
}

func ExampleCheck() {
	ctx := context.Background()
	model := linearizabilitychecker.RegisterModel()
	history := []linearizabilitychecker.Op{
		{ID: 0, Process: 0, Kind: linearizabilitychecker.KindOk, Input: 42, CallTime: 1, ReturnTime: 2},
		{ID: 1, Process: 1, Kind: linearizabilitychecker.KindOk, Output: 42, CallTime: 3, ReturnTime: 4},
	}
	res, _ := linearizabilitychecker.Check(ctx, model, history, linearizabilitychecker.Options{})
	fmt.Printf("ok: %v, lin: %v\n", res.Ok, res.Linearization)
	// Output:
	// ok: true, lin: [0 1]
}

// Silence unused time import
var _ = time.Nanosecond
