package hybrid_race_tagger

import (
	"testing"
	"time"

	pb "github.com/fitglue/server/src/go/pkg/types/pb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Helper to create a lap with the given properties
func makeLap(distance, duration float64, startOffset time.Duration) *pb.Lap {
	baseTime := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	return &pb.Lap{
		TotalDistance:    distance,
		TotalElapsedTime: duration,
		StartTime:        timestamppb.New(baseTime.Add(startOffset)),
		Records:          []*pb.Record{},
	}
}

// Helper to create a lap with records for testing record merging
func makeLapWithRecords(distance, duration float64, startOffset time.Duration, recordCount int) *pb.Lap {
	lap := makeLap(distance, duration, startOffset)
	for i := 0; i < recordCount; i++ {
		lap.Records = append(lap.Records, &pb.Record{})
	}
	return lap
}

func TestApplyMerges_EmptyMergeGroups(t *testing.T) {
	laps := []*pb.Lap{
		makeLap(1000, 300, 0),
		makeLap(500, 200, 5*time.Minute),
		makeLap(1000, 350, 10*time.Minute),
	}

	result := applyMerges(laps, [][]int{})

	if len(result) != 3 {
		t.Errorf("Expected 3 laps, got %d", len(result))
	}
	// Should return the same laps unchanged
	for i, lap := range result {
		if lap != laps[i] {
			t.Errorf("Lap %d should be the same pointer", i)
		}
	}
}

func TestApplyMerges_PreservesChronologicalOrder(t *testing.T) {
	// This test verifies the fix for the order preservation bug
	// Scenario: 4 laps where laps 1 and 2 should be merged
	// The merged lap should appear at position 1 (where lap 1 was)
	laps := []*pb.Lap{
		makeLap(1000, 300, 0),            // Lap 0: Run 1
		makeLap(500, 200, 5*time.Minute), // Lap 1: SkiErg part 1
		makeLap(500, 200, 8*time.Minute), // Lap 2: SkiErg part 2
		makeLap(1000, 350, 12*time.Minute), // Lap 3: Run 2
	}

	// Merge laps 1 and 2 (SkiErg parts)
	mergeGroups := [][]int{{1, 2}}

	result := applyMerges(laps, mergeGroups)

	// Expected: [Lap0, Merged(1,2), Lap3] = 3 laps
	if len(result) != 3 {
		t.Fatalf("Expected 3 laps after merge, got %d", len(result))
	}

	// First lap should be original lap 0
	if result[0] != laps[0] {
		t.Error("First lap should be original lap 0")
	}

	// Second lap should be the merged lap with combined distance/duration
	merged := result[1]
	expectedDistance := 1000.0 // 500 + 500
	expectedDuration := 400.0  // 200 + 200
	if merged.TotalDistance != expectedDistance {
		t.Errorf("Merged lap distance: expected %v, got %v", expectedDistance, merged.TotalDistance)
	}
	if merged.TotalElapsedTime != expectedDuration {
		t.Errorf("Merged lap duration: expected %v, got %v", expectedDuration, merged.TotalElapsedTime)
	}

	// Third lap should be original lap 3
	if result[2] != laps[3] {
		t.Error("Third lap should be original lap 3")
	}
}

func TestApplyMerges_MultipleMergeGroups(t *testing.T) {
	// Scenario: Merge laps 1-2 and laps 4-5
	laps := []*pb.Lap{
		makeLap(1000, 300, 0),              // 0: Run
		makeLap(300, 100, 5*time.Minute),   // 1: Station part 1
		makeLap(700, 150, 7*time.Minute),   // 2: Station part 2
		makeLap(1000, 320, 10*time.Minute), // 3: Run
		makeLap(400, 180, 15*time.Minute),  // 4: Station part 1
		makeLap(600, 220, 18*time.Minute),  // 5: Station part 2
		makeLap(1000, 340, 22*time.Minute), // 6: Run
	}

	mergeGroups := [][]int{{1, 2}, {4, 5}}

	result := applyMerges(laps, mergeGroups)

	// Expected: [0, Merged(1,2), 3, Merged(4,5), 6] = 5 laps
	if len(result) != 5 {
		t.Fatalf("Expected 5 laps after merge, got %d", len(result))
	}

	// Check order: lap0, merged1, lap3, merged2, lap6
	if result[0] != laps[0] {
		t.Error("Position 0 should be original lap 0")
	}
	if result[1].TotalDistance != 1000 { // 300 + 700
		t.Errorf("Position 1 (merged 1,2) distance: expected 1000, got %v", result[1].TotalDistance)
	}
	if result[2] != laps[3] {
		t.Error("Position 2 should be original lap 3")
	}
	if result[3].TotalDistance != 1000 { // 400 + 600
		t.Errorf("Position 3 (merged 4,5) distance: expected 1000, got %v", result[3].TotalDistance)
	}
	if result[4] != laps[6] {
		t.Error("Position 4 should be original lap 6")
	}
}

func TestApplyMerges_NonContiguousLapsRejected(t *testing.T) {
	// Scenario: Attempt to merge laps 1 and 3 (skipping lap 2)
	// This should be rejected - non-contiguous merges are not allowed
	laps := []*pb.Lap{
		makeLap(1000, 300, 0),              // 0
		makeLap(400, 100, 5*time.Minute),   // 1
		makeLap(1000, 320, 7*time.Minute),  // 2 (not merged)
		makeLap(600, 150, 12*time.Minute),  // 3
		makeLap(1000, 350, 15*time.Minute), // 4
	}

	mergeGroups := [][]int{{1, 3}} // Non-contiguous!

	result := applyMerges(laps, mergeGroups)

	// Non-contiguous merge should return original laps unchanged
	if len(result) != len(laps) {
		t.Fatalf("Expected original %d laps (non-contiguous rejected), got %d", len(laps), len(result))
	}

	for i, lap := range result {
		if lap != laps[i] {
			t.Errorf("Lap %d should be unchanged", i)
		}
	}
}

func TestApplyMerges_NonContiguousWithGapRejected(t *testing.T) {
	// Scenario: Attempt to merge laps 1, 2, and 4 (missing 3)
	laps := []*pb.Lap{
		makeLap(1000, 300, 0),
		makeLap(300, 100, 5*time.Minute),
		makeLap(400, 120, 7*time.Minute),
		makeLap(300, 80, 9*time.Minute),
		makeLap(1000, 350, 12*time.Minute),
	}

	mergeGroups := [][]int{{1, 2, 4}} // Gap at index 3!

	result := applyMerges(laps, mergeGroups)

	// Should return original laps unchanged
	if len(result) != len(laps) {
		t.Fatalf("Expected original %d laps (non-contiguous rejected), got %d", len(laps), len(result))
	}
}

func TestApplyMerges_OutOfOrderIndices(t *testing.T) {
	// Scenario: Merge group has indices in reverse order [2, 1]
	// Should still work correctly
	laps := []*pb.Lap{
		makeLap(1000, 300, 0),
		makeLap(500, 200, 5*time.Minute),
		makeLap(500, 200, 8*time.Minute),
		makeLap(1000, 350, 12*time.Minute),
	}

	// Indices out of order
	mergeGroups := [][]int{{2, 1}}

	result := applyMerges(laps, mergeGroups)

	if len(result) != 3 {
		t.Fatalf("Expected 3 laps after merge, got %d", len(result))
	}

	// Merged lap should still be at position 1 (min of 1, 2)
	if result[0] != laps[0] {
		t.Error("Position 0 should be original lap 0")
	}
	if result[1].TotalDistance != 1000 {
		t.Errorf("Position 1 (merged) distance: expected 1000, got %v", result[1].TotalDistance)
	}
	if result[2] != laps[3] {
		t.Error("Position 2 should be original lap 3")
	}

	// StartTime should be from lap 1 (the earlier one), not lap 2
	expectedStartTime := laps[1].StartTime.AsTime()
	actualStartTime := result[1].StartTime.AsTime()
	if !actualStartTime.Equal(expectedStartTime) {
		t.Errorf("Merged lap StartTime should be from lap 1. Expected %v, got %v", expectedStartTime, actualStartTime)
	}
}

func TestApplyMerges_ThreeLapMerge(t *testing.T) {
	// Scenario: Merge three consecutive laps
	laps := []*pb.Lap{
		makeLap(1000, 300, 0),             // 0: Run
		makeLap(300, 100, 5*time.Minute),  // 1: Station part 1
		makeLap(400, 120, 7*time.Minute),  // 2: Station part 2
		makeLap(300, 80, 9*time.Minute),   // 3: Station part 3
		makeLap(1000, 350, 12*time.Minute), // 4: Run
	}

	mergeGroups := [][]int{{1, 2, 3}}

	result := applyMerges(laps, mergeGroups)

	if len(result) != 3 {
		t.Fatalf("Expected 3 laps after merge, got %d", len(result))
	}

	// Check merged lap has combined values
	merged := result[1]
	if merged.TotalDistance != 1000 { // 300 + 400 + 300
		t.Errorf("Merged distance: expected 1000, got %v", merged.TotalDistance)
	}
	if merged.TotalElapsedTime != 300 { // 100 + 120 + 80
		t.Errorf("Merged duration: expected 300, got %v", merged.TotalElapsedTime)
	}
}

func TestMergeLaps_CombinesDistanceAndDuration(t *testing.T) {
	laps := []*pb.Lap{
		makeLap(400, 100, 0),
		makeLap(300, 80, 2*time.Minute),
		makeLap(300, 70, 4*time.Minute),
	}

	result := mergeLaps(laps, []int{0, 1, 2})

	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if result.TotalDistance != 1000 {
		t.Errorf("Expected distance 1000, got %v", result.TotalDistance)
	}
	if result.TotalElapsedTime != 250 {
		t.Errorf("Expected duration 250, got %v", result.TotalElapsedTime)
	}
}

func TestMergeLaps_SortsIndicesForStartTime(t *testing.T) {
	laps := []*pb.Lap{
		makeLap(400, 100, 0),            // Earliest
		makeLap(300, 80, 2*time.Minute), // Middle
		makeLap(300, 70, 4*time.Minute), // Latest
	}

	// Pass indices in reverse order
	result := mergeLaps(laps, []int{2, 0, 1})

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// StartTime should be from lap 0 (earliest), not lap 2
	expectedStartTime := laps[0].StartTime.AsTime()
	actualStartTime := result.StartTime.AsTime()
	if !actualStartTime.Equal(expectedStartTime) {
		t.Errorf("StartTime should be from earliest lap. Expected %v, got %v", expectedStartTime, actualStartTime)
	}
}

func TestMergeLaps_CombinesRecords(t *testing.T) {
	laps := []*pb.Lap{
		makeLapWithRecords(400, 100, 0, 5),
		makeLapWithRecords(300, 80, 2*time.Minute, 3),
		makeLapWithRecords(300, 70, 4*time.Minute, 4),
	}

	result := mergeLaps(laps, []int{0, 1, 2})

	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	// Should have 5 + 3 + 4 = 12 records
	if len(result.Records) != 12 {
		t.Errorf("Expected 12 records, got %d", len(result.Records))
	}
}

func TestMergeLaps_EmptyIndices(t *testing.T) {
	laps := []*pb.Lap{
		makeLap(400, 100, 0),
	}

	result := mergeLaps(laps, []int{})

	if result != nil {
		t.Error("Expected nil result for empty indices")
	}
}

func TestMergeLaps_InvalidIndices(t *testing.T) {
	laps := []*pb.Lap{
		makeLap(400, 100, 0),
		makeLap(300, 80, 2*time.Minute),
	}

	// First index is out of bounds
	result := mergeLaps(laps, []int{5, 0})

	// After sorting, first index is 0 which is valid
	if result == nil {
		t.Fatal("Expected non-nil result (index 0 is valid)")
	}

	// Only lap 0 should be included (lap 5 doesn't exist)
	if result.TotalDistance != 400 {
		t.Errorf("Expected distance 400 (only lap 0), got %v", result.TotalDistance)
	}
}

func TestMergeLaps_AllInvalidIndices(t *testing.T) {
	laps := []*pb.Lap{
		makeLap(400, 100, 0),
	}

	// All indices are out of bounds
	result := mergeLaps(laps, []int{5, 10})

	if result != nil {
		t.Error("Expected nil result when all indices are invalid")
	}
}

func TestMergeLaps_NegativeIndices(t *testing.T) {
	laps := []*pb.Lap{
		makeLap(400, 100, 0),
		makeLap(300, 80, 2*time.Minute),
	}

	// Mix of negative and valid indices
	result := mergeLaps(laps, []int{-1, 0, 1})

	// After sorting, -1 is first and invalid, so should return nil
	if result != nil {
		t.Error("Expected nil result when first sorted index is negative")
	}
}

func TestMergeLaps_SingleIndex(t *testing.T) {
	laps := []*pb.Lap{
		makeLap(400, 100, 0),
		makeLap(300, 80, 2*time.Minute),
	}

	result := mergeLaps(laps, []int{1})

	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if result.TotalDistance != 300 {
		t.Errorf("Expected distance 300, got %v", result.TotalDistance)
	}
	if result.TotalElapsedTime != 80 {
		t.Errorf("Expected duration 80, got %v", result.TotalElapsedTime)
	}
}

// Integration test simulating a real Hyrox scenario
func TestApplyMerges_HyroxScenario(t *testing.T) {
	// Simulate a Hyrox race where watch recorded extra laps for some stations
	// Real scenario: Run1, SkiErg(2 laps), Run2, SledPush(3 laps), Run3, ...
	laps := []*pb.Lap{
		makeLap(1000, 300, 0),             // 0: Run 1
		makeLap(500, 180, 5*time.Minute),  // 1: SkiErg part 1
		makeLap(500, 170, 8*time.Minute),  // 2: SkiErg part 2
		makeLap(1000, 280, 12*time.Minute), // 3: Run 2
		makeLap(100, 60, 17*time.Minute),  // 4: Sled Push part 1
		makeLap(100, 55, 18*time.Minute),  // 5: Sled Push part 2
		makeLap(50, 45, 19*time.Minute),   // 6: Sled Push part 3
		makeLap(1000, 310, 22*time.Minute), // 7: Run 3
	}

	// User merges: SkiErg (1,2), Sled Push (4,5,6)
	mergeGroups := [][]int{{1, 2}, {4, 5, 6}}

	result := applyMerges(laps, mergeGroups)

	// Expected: Run1, MergedSkiErg, Run2, MergedSledPush, Run3 = 5 laps
	if len(result) != 5 {
		t.Fatalf("Expected 5 laps, got %d", len(result))
	}

	// Verify order and values
	// Position 0: Run 1 (unchanged)
	if result[0].TotalDistance != 1000 {
		t.Errorf("Run 1 distance wrong: expected 1000, got %v", result[0].TotalDistance)
	}

	// Position 1: Merged SkiErg (1000m total)
	if result[1].TotalDistance != 1000 {
		t.Errorf("SkiErg merged distance wrong: expected 1000, got %v", result[1].TotalDistance)
	}
	if result[1].TotalElapsedTime != 350 { // 180 + 170
		t.Errorf("SkiErg merged duration wrong: expected 350, got %v", result[1].TotalElapsedTime)
	}

	// Position 2: Run 2 (unchanged)
	if result[2].TotalDistance != 1000 {
		t.Errorf("Run 2 distance wrong: expected 1000, got %v", result[2].TotalDistance)
	}

	// Position 3: Merged Sled Push (250m total)
	if result[3].TotalDistance != 250 { // 100 + 100 + 50
		t.Errorf("Sled Push merged distance wrong: expected 250, got %v", result[3].TotalDistance)
	}
	if result[3].TotalElapsedTime != 160 { // 60 + 55 + 45
		t.Errorf("Sled Push merged duration wrong: expected 160, got %v", result[3].TotalElapsedTime)
	}

	// Position 4: Run 3 (unchanged)
	if result[4].TotalDistance != 1000 {
		t.Errorf("Run 3 distance wrong: expected 1000, got %v", result[4].TotalDistance)
	}
}
