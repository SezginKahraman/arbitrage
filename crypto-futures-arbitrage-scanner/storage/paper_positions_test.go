package storage

import (
	"context"
	"errors"
	"math"
	"testing"
)

func paperPositionInput() PaperPositionInput {
	return PaperPositionInput{
		Contract: "COTI_USDT", Direction: "long", Strategy: "trend_pullback", Interval: "15m",
		EntryPrice: 0.01050, StopPrice: 0.01000, TargetPrice: 0.01150,
		Contracts: 20_000, BaseQuantity: 20_000, Notional: 210, RiskAmount: 10, EstimatedFees: 0.315,
		AnalysisAtMS: 1_700_000_000_000,
	}
}

func TestSQLitePaperPositionLifecyclePersistsNetRealizedPnL(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	created, err := store.CreatePaperPosition(ctx, paperPositionInput(), 1_700_000_001_000)
	if err != nil {
		t.Fatalf("CreatePaperPosition: %v", err)
	}
	if created.ID <= 0 || created.Status != PaperPositionOpen || created.OpenedAtMS != 1_700_000_001_000 {
		t.Fatalf("created = %+v", created)
	}

	closed, err := store.ClosePaperPosition(ctx, created.ID, 0.01120, 1_700_000_901_000)
	if err != nil {
		t.Fatalf("ClosePaperPosition: %v", err)
	}
	wantPnL := (0.01120-0.01050)*20_000 - 0.315
	if closed.Status != PaperPositionClosed || closed.ExitPrice != 0.01120 || math.Abs(closed.RealizedPnL-wantPnL) > 1e-9 {
		t.Fatalf("closed = %+v, want net PnL %.6f", closed, wantPnL)
	}

	items, err := store.ListPaperPositions(ctx, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != created.ID || items[0].ClosedAtMS == nil {
		t.Fatalf("items = %+v", items)
	}
}

func TestSQLitePaperPositionCalculatesShortPnL(t *testing.T) {
	store := openTestStore(t)
	input := paperPositionInput()
	input.Direction = "short"
	input.StopPrice = 0.01100
	input.TargetPrice = 0.00950

	created, err := store.CreatePaperPosition(context.Background(), input, 1_000)
	if err != nil {
		t.Fatal(err)
	}
	closed, err := store.ClosePaperPosition(context.Background(), created.ID, 0.01000, 2_000)
	if err != nil {
		t.Fatal(err)
	}
	wantPnL := (0.01050-0.01000)*20_000 - 0.315
	if math.Abs(closed.RealizedPnL-wantPnL) > 1e-9 {
		t.Fatalf("short PnL = %.6f, want %.6f", closed.RealizedPnL, wantPnL)
	}
}

func TestSQLitePaperPositionRejectsInvalidOrRepeatedClose(t *testing.T) {
	store := openTestStore(t)
	input := paperPositionInput()
	input.Direction = "sideways"
	if _, err := store.CreatePaperPosition(context.Background(), input, 1_000); err == nil {
		t.Fatal("invalid direction was accepted")
	}

	input = paperPositionInput()
	created, err := store.CreatePaperPosition(context.Background(), input, 1_000)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ClosePaperPosition(context.Background(), created.ID, 0.011, 2_000); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ClosePaperPosition(context.Background(), created.ID, 0.012, 3_000); !errors.Is(err, ErrPaperPositionClosed) {
		t.Fatalf("second close error = %v, want ErrPaperPositionClosed", err)
	}
}

func TestPaperPositionValidationRejectsUnknownAnalysisMetadata(t *testing.T) {
	input := paperPositionInput()
	input.Strategy = "oracle"
	if err := ValidatePaperPositionInput(input); err == nil {
		t.Fatal("unknown strategy was accepted")
	}
	input = paperPositionInput()
	input.Interval = "2m"
	if err := ValidatePaperPositionInput(input); err == nil {
		t.Fatal("unknown interval was accepted")
	}
}

func TestPaperPositionValidationAcceptsSingleCharacterBaseAsset(t *testing.T) {
	input := paperPositionInput()
	input.Contract = "H_USDT"
	if err := ValidatePaperPositionInput(input); err != nil {
		t.Fatalf("H_USDT paper position rejected: %v", err)
	}
}
