package storage

import (
	"context"
	"testing"
)

func validAlertInput() AlertRuleInput {
	return AlertRuleInput{
		Name: "COTI spot gap", Symbol: "COTIUSDT", MarketMode: AlertMarketSpot,
		MinSpreadPct: 0.75, CooldownSeconds: 300, Enabled: true, BrowserEnabled: true,
	}
}

func TestSQLiteStoreCreatesListsAndUpdatesAlertRules(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	created, err := store.CreateAlertRule(ctx, validAlertInput(), 1_000)
	if err != nil {
		t.Fatalf("CreateAlertRule: %v", err)
	}
	if created.ID == 0 || created.CreatedAtMS != 1_000 || created.UpdatedAtMS != 1_000 {
		t.Fatalf("created rule = %+v", created)
	}

	input := validAlertInput()
	input.Name = "COTI actionable gap"
	input.MinSpreadPct = 0.9
	input.Enabled = false
	updated, err := store.UpdateAlertRule(ctx, created.ID, input, 2_000)
	if err != nil {
		t.Fatalf("UpdateAlertRule: %v", err)
	}
	if updated.Name != input.Name || updated.MinSpreadPct != 0.9 || updated.Enabled || updated.UpdatedAtMS != 2_000 {
		t.Fatalf("updated rule = %+v", updated)
	}

	rules, err := store.ListAlertRules(ctx)
	if err != nil {
		t.Fatalf("ListAlertRules: %v", err)
	}
	if len(rules) != 1 || rules[0].ID != created.ID || rules[0].Name != input.Name {
		t.Fatalf("rules = %+v", rules)
	}
}

func TestSQLiteStoreEvaluatesAlertRulesWithMarketAndCooldown(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()
	input := validAlertInput()
	input.BuySource = "gate_spot"
	input.SellSource = "binance_spot"
	rule, err := store.CreateAlertRule(ctx, input, 1_000)
	if err != nil {
		t.Fatal(err)
	}

	observation := AlertObservation{
		Symbol: "COTIUSDT", BuySource: "gate_spot", SellSource: "binance_spot",
		BuyPrice: 0.011, SellPrice: 0.012, GrossSpreadPct: 0.82, ObservedAtMS: 10_000,
	}
	triggers, err := store.EvaluateAlerts(ctx, observation)
	if err != nil {
		t.Fatalf("EvaluateAlerts: %v", err)
	}
	if len(triggers) != 1 || triggers[0].RuleID != rule.ID || triggers[0].RuleName != rule.Name {
		t.Fatalf("triggers = %+v", triggers)
	}

	observation.ObservedAtMS = 20_000
	triggers, err = store.EvaluateAlerts(ctx, observation)
	if err != nil {
		t.Fatal(err)
	}
	if len(triggers) != 0 {
		t.Fatalf("cooldown triggers = %+v, want none", triggers)
	}

	observation.ObservedAtMS = 311_000
	triggers, err = store.EvaluateAlerts(ctx, observation)
	if err != nil {
		t.Fatal(err)
	}
	if len(triggers) != 1 {
		t.Fatalf("post-cooldown triggers = %+v, want one", triggers)
	}

	recent, err := store.ListAlertTriggers(ctx, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(recent) != 2 || recent[0].TriggeredAtMS != 311_000 || recent[1].TriggeredAtMS != 10_000 {
		t.Fatalf("recent triggers = %+v", recent)
	}
}

func TestSQLiteStoreDoesNotTriggerDisabledWrongMarketOrBelowThresholdRules(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	disabled := validAlertInput()
	disabled.Enabled = false
	if _, err := store.CreateAlertRule(ctx, disabled, 1_000); err != nil {
		t.Fatal(err)
	}
	futures := validAlertInput()
	futures.Name = "Futures only"
	futures.MarketMode = AlertMarketFutures
	if _, err := store.CreateAlertRule(ctx, futures, 1_000); err != nil {
		t.Fatal(err)
	}
	high := validAlertInput()
	high.Name = "High threshold"
	high.MinSpreadPct = 2
	if _, err := store.CreateAlertRule(ctx, high, 1_000); err != nil {
		t.Fatal(err)
	}

	triggers, err := store.EvaluateAlerts(ctx, AlertObservation{
		Symbol: "COTIUSDT", BuySource: "gate_spot", SellSource: "binance_spot",
		BuyPrice: 1, SellPrice: 1.01, GrossSpreadPct: 0.82, ObservedAtMS: 10_000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(triggers) != 0 {
		t.Fatalf("triggers = %+v, want none", triggers)
	}
}

func TestAlertRuleValidationRejectsUnsafeOrInvalidInputs(t *testing.T) {
	store := openTestStore(t)
	ctx := context.Background()

	tests := []AlertRuleInput{
		{},
		{Name: "bad mode", MarketMode: "margin", CooldownSeconds: 60, BrowserEnabled: true},
		{Name: "bad spread", MarketMode: AlertMarketAll, MinSpreadPct: -1, CooldownSeconds: 60, BrowserEnabled: true},
		{Name: "bad cooldown", MarketMode: AlertMarketAll, MinSpreadPct: 1, CooldownSeconds: 0, BrowserEnabled: true},
		{Name: "same source", MarketMode: AlertMarketAll, BuySource: "gate_spot", SellSource: "gate_spot", MinSpreadPct: 1, CooldownSeconds: 60, BrowserEnabled: true},
	}
	for _, input := range tests {
		if _, err := store.CreateAlertRule(ctx, input, 1_000); err == nil {
			t.Fatalf("CreateAlertRule(%+v) succeeded", input)
		}
	}
}
