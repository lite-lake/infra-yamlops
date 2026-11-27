package service

import (
	"testing"

	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
	"github.com/lite-lake/infra-yamlops/internal/domain/repository"
	"github.com/lite-lake/infra-yamlops/internal/domain/valueobject"
)

func newRecord(dnsDomain string, rtype entity.DNSRecordType, name, value string) *entity.DNSRecord {
	return &entity.DNSRecord{
		Domain: dnsDomain,
		Type:   rtype,
		Name:   name,
		Value:  value,
		TTL:    600,
	}
}

func recordMap(records ...*entity.DNSRecord) map[string]*entity.DNSRecord {
	out := make(map[string]*entity.DNSRecord, len(records))
	for _, r := range records {
		key := r.Domain + ":" + string(r.Type) + ":" + r.Name + ":" + r.EffectiveLine() + ":" + r.Value
		out[key] = r
	}
	return out
}

func TestPlanRecords_SkipsSystemNSRecords(t *testing.T) {
	state := &repository.DeploymentState{
		Records: recordMap(
			newRecord("kissfocus.vip", entity.DNSRecordTypeNS, "@", "dns21.dnspod.net"),
			newRecord("kissfocus.vip", entity.DNSRecordTypeA, "@", "38.244.14.167"),
		),
	}
	svc := NewDifferService(state)
	plan := valueobject.NewPlan()

	cfg := []entity.DNSRecord{
		{Domain: "kissfocus.vip", Type: entity.DNSRecordTypeA, Name: "@", Value: "38.244.14.167", TTL: 600},
	}
	svc.PlanRecords(plan, cfg, valueobject.NewScope())

	if got := len(plan.Changes()); got != 0 {
		t.Fatalf("expected no changes, got %d: %+v", got, plan.Changes())
	}
}

func TestPlanRecords_NSGuardDoesNotHideRealChanges(t *testing.T) {
	state := &repository.DeploymentState{
		Records: recordMap(
			newRecord("kissfocus.vip", entity.DNSRecordTypeNS, "@", "dns21.dnspod.net"),
			newRecord("kissfocus.vip", entity.DNSRecordTypeA, "@", "1.1.1.1"),
		),
	}
	svc := NewDifferService(state)
	plan := valueobject.NewPlan()

	cfg := []entity.DNSRecord{
		{Domain: "kissfocus.vip", Type: entity.DNSRecordTypeA, Name: "@", Value: "2.2.2.2", TTL: 600},
	}
	svc.PlanRecords(plan, cfg, valueobject.NewScope())

	changes := plan.Changes()
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d: %+v", len(changes), changes)
	}
	if changes[0].Type() != valueobject.ChangeTypeUpdate {
		t.Fatalf("expected update change, got %v", changes[0].Type())
	}
}

func TestPlanRecords_LineIsPartOfRecordIdentity(t *testing.T) {
	state := &repository.DeploymentState{
		Records: recordMap(
			newRecord("kissfocus.vip", entity.DNSRecordTypeA, "@", "38.244.14.167"),
		),
	}
	svc := NewDifferService(state)
	plan := valueobject.NewPlan()

	cfg := []entity.DNSRecord{
		{Domain: "kissfocus.vip", Type: entity.DNSRecordTypeA, Name: "@", Value: "38.244.14.167", TTL: 600},
		{Domain: "kissfocus.vip", Type: entity.DNSRecordTypeA, Name: "@", Value: "199.68.217.251", TTL: 600, Line: "oversea"},
	}
	svc.PlanRecords(plan, cfg, valueobject.NewScope())

	changes := plan.Changes()
	if len(changes) != 1 {
		t.Fatalf("expected 1 create for oversea line, got %d: %+v", len(changes), changes)
	}
	if changes[0].Type() != valueobject.ChangeTypeCreate {
		t.Fatalf("expected create change, got %v", changes[0].Type())
	}
}
