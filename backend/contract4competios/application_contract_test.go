package contract4competios

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

type individualEnrollmentApplicationStub struct{}

func (individualEnrollmentApplicationStub) GetCompetition(context.Context, GetCompetitionRequest) (CompetitionProjection, error) {
	return CompetitionProjection{}, nil
}

func (individualEnrollmentApplicationStub) GetEntry(context.Context, GetEntryRequest) (EntryProjection, error) {
	return EntryProjection{}, nil
}

func (individualEnrollmentApplicationStub) RequestEntry(context.Context, RequestEntryCommand) (EntryOutcome, error) {
	return EntryOutcome{}, nil
}

func (individualEnrollmentApplicationStub) WithdrawEntry(context.Context, WithdrawEntryCommand) (EntryOutcome, error) {
	return EntryOutcome{}, nil
}

var _ IndividualEnrollmentApplication = individualEnrollmentApplicationStub{}

type managedEnrollmentApplicationStub struct {
	individualEnrollmentApplicationStub
}

func (managedEnrollmentApplicationStub) InviteEntry(context.Context, InviteEntryCommand) (EntryOutcome, error) {
	return EntryOutcome{}, nil
}

func (managedEnrollmentApplicationStub) AcceptEntry(context.Context, AcceptEntryCommand) (EntryOutcome, error) {
	return EntryOutcome{}, nil
}

func (managedEnrollmentApplicationStub) ApproveEntry(context.Context, ApproveEntryCommand) (EntryOutcome, error) {
	return EntryOutcome{}, nil
}

func (managedEnrollmentApplicationStub) RejectEntry(context.Context, RejectEntryCommand) (EntryOutcome, error) {
	return EntryOutcome{}, nil
}

func (managedEnrollmentApplicationStub) RevokeInvitation(context.Context, RevokeInvitationCommand) (EntryOutcome, error) {
	return EntryOutcome{}, nil
}

var _ ManagedEnrollmentApplication = managedEnrollmentApplicationStub{}

func TestIndividualEnrollmentApplicationFreezesPublicMethodSet(t *testing.T) {
	interfaceType := reflect.TypeOf((*IndividualEnrollmentApplication)(nil)).Elem()
	want := map[string]reflect.Type{
		"GetCompetition": reflect.TypeOf((func(context.Context, GetCompetitionRequest) (CompetitionProjection, error))(nil)),
		"GetEntry":       reflect.TypeOf((func(context.Context, GetEntryRequest) (EntryProjection, error))(nil)),
		"RequestEntry":   reflect.TypeOf((func(context.Context, RequestEntryCommand) (EntryOutcome, error))(nil)),
		"WithdrawEntry":  reflect.TypeOf((func(context.Context, WithdrawEntryCommand) (EntryOutcome, error))(nil)),
	}

	if interfaceType.NumMethod() != len(want) {
		t.Fatalf("IndividualEnrollmentApplication has %d methods, want %d", interfaceType.NumMethod(), len(want))
	}
	for name, wantType := range want {
		method, ok := interfaceType.MethodByName(name)
		if !ok {
			t.Errorf("IndividualEnrollmentApplication is missing %s", name)
			continue
		}
		if method.Type != wantType {
			t.Errorf("IndividualEnrollmentApplication.%s has type %s, want %s", name, method.Type, wantType)
		}
	}
}

func TestManagedEnrollmentApplicationFreezesPublicMethodSet(t *testing.T) {
	interfaceType := reflect.TypeOf((*ManagedEnrollmentApplication)(nil)).Elem()
	want := map[string]reflect.Type{
		"GetCompetition":   reflect.TypeOf((func(context.Context, GetCompetitionRequest) (CompetitionProjection, error))(nil)),
		"GetEntry":         reflect.TypeOf((func(context.Context, GetEntryRequest) (EntryProjection, error))(nil)),
		"RequestEntry":     reflect.TypeOf((func(context.Context, RequestEntryCommand) (EntryOutcome, error))(nil)),
		"WithdrawEntry":    reflect.TypeOf((func(context.Context, WithdrawEntryCommand) (EntryOutcome, error))(nil)),
		"InviteEntry":      reflect.TypeOf((func(context.Context, InviteEntryCommand) (EntryOutcome, error))(nil)),
		"AcceptEntry":      reflect.TypeOf((func(context.Context, AcceptEntryCommand) (EntryOutcome, error))(nil)),
		"ApproveEntry":     reflect.TypeOf((func(context.Context, ApproveEntryCommand) (EntryOutcome, error))(nil)),
		"RejectEntry":      reflect.TypeOf((func(context.Context, RejectEntryCommand) (EntryOutcome, error))(nil)),
		"RevokeInvitation": reflect.TypeOf((func(context.Context, RevokeInvitationCommand) (EntryOutcome, error))(nil)),
	}

	if interfaceType.NumMethod() != len(want) {
		t.Fatalf("ManagedEnrollmentApplication has %d methods, want %d", interfaceType.NumMethod(), len(want))
	}
	for name, wantType := range want {
		method, ok := interfaceType.MethodByName(name)
		if !ok {
			t.Errorf("ManagedEnrollmentApplication is missing %s", name)
			continue
		}
		if method.Type != wantType {
			t.Errorf("ManagedEnrollmentApplication.%s has type %s, want %s", name, method.Type, wantType)
		}
	}
}

func TestIndividualEnrollmentCommandsFreezeBoundedPublicShapes(t *testing.T) {
	assertCommandShape(t, reflect.TypeOf(RequestEntryCommand{}), []commandField{
		{name: "Caller", jsonName: "caller", typ: reflect.TypeOf(CallerContext{})},
		{name: "Metadata", jsonName: "metadata", typ: reflect.TypeOf(CommandMetadata{})},
		{name: "CompetitionID", jsonName: "competitionID", typ: reflect.TypeOf(CompetitionID(""))},
		{name: "EntryID", jsonName: "entryID", typ: reflect.TypeOf(EntryID(""))},
		{name: "Participant", jsonName: "participant", typ: reflect.TypeOf(ParticipantReference{})},
	})
	assertCommandShape(t, reflect.TypeOf(WithdrawEntryCommand{}), []commandField{
		{name: "Caller", jsonName: "caller", typ: reflect.TypeOf(CallerContext{})},
		{name: "Metadata", jsonName: "metadata", typ: reflect.TypeOf(CommandMetadata{})},
		{name: "CompetitionID", jsonName: "competitionID", typ: reflect.TypeOf(CompetitionID(""))},
		{name: "EntryID", jsonName: "entryID", typ: reflect.TypeOf(EntryID(""))},
	})
}

func TestManagedEnrollmentCommandsFreezeBoundedPublicShapes(t *testing.T) {
	assertCommandShape(t, reflect.TypeOf(InviteEntryCommand{}), []commandField{
		{name: "Caller", jsonName: "caller", typ: reflect.TypeOf(CallerContext{})},
		{name: "Metadata", jsonName: "metadata", typ: reflect.TypeOf(CommandMetadata{})},
		{name: "CompetitionID", jsonName: "competitionID", typ: reflect.TypeOf(CompetitionID(""))},
		{name: "EntryID", jsonName: "entryID", typ: reflect.TypeOf(EntryID(""))},
		{name: "Participant", jsonName: "participant", typ: reflect.TypeOf(ParticipantReference{})},
	})
	for _, commandType := range []reflect.Type{
		reflect.TypeOf(AcceptEntryCommand{}),
		reflect.TypeOf(ApproveEntryCommand{}),
		reflect.TypeOf(RejectEntryCommand{}),
		reflect.TypeOf(RevokeInvitationCommand{}),
	} {
		assertCommandShape(t, commandType, []commandField{
			{name: "Caller", jsonName: "caller", typ: reflect.TypeOf(CallerContext{})},
			{name: "Metadata", jsonName: "metadata", typ: reflect.TypeOf(CommandMetadata{})},
			{name: "CompetitionID", jsonName: "competitionID", typ: reflect.TypeOf(CompetitionID(""))},
			{name: "EntryID", jsonName: "entryID", typ: reflect.TypeOf(EntryID(""))},
		})
	}
}

type commandField struct {
	name     string
	jsonName string
	typ      reflect.Type
}

func assertCommandShape(t *testing.T, commandType reflect.Type, want []commandField) {
	t.Helper()
	got := flattenCommandFields(commandType)
	if len(got) != len(want) {
		t.Fatalf("%s has %d public fields, want %d: %v", commandType, len(got), len(want), got)
	}
	for index, field := range got {
		expected := want[index]
		if field.Name != expected.name || field.Type != expected.typ || jsonName(field) != expected.jsonName {
			t.Errorf("%s public field %d = %s (%s, %s), want %s (%s, %s)", commandType, index, field.Name, field.Type, jsonName(field), expected.name, expected.typ, expected.jsonName)
		}
	}

	assertNoEnrollmentBoundaryFields(t, commandType, map[reflect.Type]bool{})
}

func flattenCommandFields(commandType reflect.Type) []reflect.StructField {
	var fields []reflect.StructField
	for index := 0; index < commandType.NumField(); index++ {
		field := commandType.Field(index)
		if field.Anonymous && field.Type.Kind() == reflect.Struct {
			fields = append(fields, flattenCommandFields(field.Type)...)
			continue
		}
		fields = append(fields, field)
	}
	return fields
}

func jsonName(field reflect.StructField) string {
	return strings.Split(field.Tag.Get("json"), ",")[0]
}

func assertNoEnrollmentBoundaryFields(t *testing.T, typ reflect.Type, visited map[reflect.Type]bool) {
	t.Helper()
	if typ.Kind() == reflect.Pointer || typ.Kind() == reflect.Slice {
		assertNoEnrollmentBoundaryFields(t, typ.Elem(), visited)
		return
	}
	if typ.Kind() != reflect.Struct || visited[typ] {
		return
	}
	visited[typ] = true
	for index := 0; index < typ.NumField(); index++ {
		field := typ.Field(index)
		lowerName := strings.ToLower(field.Name + " " + field.Tag.Get("json"))
		for _, forbidden := range []string{"role", "authority", "evidence", "acceptance", "roster", "state", "outcome"} {
			if strings.Contains(lowerName, forbidden) {
				t.Errorf("%s exposes forbidden enrolment field %q", typ, field.Name)
			}
		}
		assertNoEnrollmentBoundaryFields(t, field.Type, visited)
	}
}
