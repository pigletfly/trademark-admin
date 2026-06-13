package quotation

import (
	"encoding/json"
	"slices"

	"github.com/google/uuid"

	"github.com/pigletfly/trademark-admin/apps/api/internal/platform/audit"
)

const (
	AgentLevelA = "agent_a"
	AgentLevelB = "agent_b"
)

var validRegistrationMethods = map[string]struct{}{
	"madrid": {},
	"single": {},
}

var validInfoSections = map[string]struct{}{
	"acceptance_time":           {},
	"registration_time":         {},
	"required_documents":        {},
	"registration_method_intro": {},
	"real_cases":                {},
}

type methodCountrySelection struct {
	madrid []uuid.UUID
	single []uuid.UUID
}

func normalizeCountryIDs(primary uuid.UUID, selected []uuid.UUID) ([]uuid.UUID, error) {
	if len(selected) == 0 {
		if primary == uuid.Nil {
			return nil, ErrInvalidFormInput
		}
		return []uuid.UUID{primary}, nil
	}
	out := make([]uuid.UUID, 0, len(selected))
	for _, id := range selected {
		if id == uuid.Nil {
			return nil, ErrInvalidFormInput
		}
		if !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out, nil
}

func normalizeUUIDList(selected []uuid.UUID) ([]uuid.UUID, error) {
	out := make([]uuid.UUID, 0, len(selected))
	for _, id := range selected {
		if id == uuid.Nil {
			return nil, ErrInvalidFormInput
		}
		if !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out, nil
}

func normalizeNiceCategoryCodes(codes []int) ([]int, error) {
	out := make([]int, 0, len(codes))
	for _, code := range codes {
		if code < 1 || code > 45 {
			return nil, ErrInvalidFormInput
		}
		if !slices.Contains(out, code) {
			out = append(out, code)
		}
	}
	return out, nil
}

func normalizeMethodCountrySelection(
	primary uuid.UUID,
	legacyCountryIDs []uuid.UUID,
	registrationMethods []string,
	madridCountryIDs []uuid.UUID,
	singleCountryIDs []uuid.UUID,
) (methodCountrySelection, error) {
	madridIDs, err := normalizeUUIDList(madridCountryIDs)
	if err != nil {
		return methodCountrySelection{}, err
	}
	singleIDs, err := normalizeUUIDList(singleCountryIDs)
	if err != nil {
		return methodCountrySelection{}, err
	}
	if len(madridIDs) > 0 || len(singleIDs) > 0 {
		return methodCountrySelection{madrid: madridIDs, single: singleIDs}, nil
	}

	countryIDs, err := normalizeCountryIDs(primary, legacyCountryIDs)
	if err != nil {
		return methodCountrySelection{}, err
	}
	methods, err := normalizeRegistrationMethods(registrationMethods)
	if err != nil {
		return methodCountrySelection{}, err
	}

	selection := methodCountrySelection{
		madrid: make([]uuid.UUID, 0),
		single: make([]uuid.UUID, 0),
	}
	for _, method := range methods {
		switch method {
		case "madrid":
			selection.madrid = append(selection.madrid, countryIDs...)
		case "single":
			selection.single = append(selection.single, countryIDs...)
		}
	}
	return selection, nil
}

func normalizeRegistrationMethods(values []string) ([]string, error) {
	if len(values) == 0 {
		return []string{"single"}, nil
	}
	return normalizeStringEnumList(values, validRegistrationMethods)
}

func normalizeInfoSections(values []string) ([]string, error) {
	return normalizeStringEnumList(values, validInfoSections)
}

func normalizeStringEnumList(values []string, valid map[string]struct{}) ([]string, error) {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := valid[value]; !ok {
			return nil, ErrInvalidFormInput
		}
		if !slices.Contains(out, value) {
			out = append(out, value)
		}
	}
	return out, nil
}

func normalizeAgentLevel(value string) (string, error) {
	switch value {
	case "":
		return AgentLevelA, nil
	case AgentLevelA, AgentLevelB:
		return value, nil
	default:
		return "", ErrInvalidFormInput
	}
}

func serviceTierForAgentLevel(agentLevel string) string {
	if agentLevel == AgentLevelB {
		return "standard"
	}
	return "basic"
}

func agentLevelFromServiceTier(tier string) string {
	if tier == "standard" || tier == "premium" {
		return AgentLevelB
	}
	return AgentLevelA
}

func encodeJSONB(value any) (audit.JSONB, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return audit.JSONB(raw), nil
}

func decodeJSONB[T any](raw audit.JSONB) []T {
	if len(raw) == 0 {
		return nil
	}
	var out []T
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}
	return out
}

func appendUniqueUUIDs(groups ...[]uuid.UUID) []uuid.UUID {
	var out []uuid.UUID
	for _, group := range groups {
		for _, id := range group {
			if !slices.Contains(out, id) {
				out = append(out, id)
			}
		}
	}
	return out
}

func (s methodCountrySelection) countryIDs() []uuid.UUID {
	return appendUniqueUUIDs(s.madrid, s.single)
}

func (s methodCountrySelection) registrationMethods() []string {
	methods := make([]string, 0, 2)
	if len(s.madrid) > 0 {
		methods = append(methods, "madrid")
	}
	if len(s.single) > 0 {
		methods = append(methods, "single")
	}
	return methods
}

func legacyQuotationCountryIDs(q *Quotation) []uuid.UUID {
	ids := decodeJSONB[uuid.UUID](q.CountryIDs)
	if len(ids) == 0 && q.CountryID != uuid.Nil {
		return []uuid.UUID{q.CountryID}
	}
	return ids
}

func legacyQuotationRegistrationMethods(q *Quotation) []string {
	methods := decodeJSONB[string](q.RegistrationMethods)
	if len(methods) == 0 {
		return []string{"single"}
	}
	return methods
}

func quotationMethodCountrySelection(q *Quotation) methodCountrySelection {
	if q == nil {
		return methodCountrySelection{
			madrid: make([]uuid.UUID, 0),
			single: make([]uuid.UUID, 0),
		}
	}
	madridIDs := decodeJSONB[uuid.UUID](q.MadridCountryIDs)
	singleIDs := decodeJSONB[uuid.UUID](q.SingleCountryIDs)
	if len(madridIDs) > 0 || len(singleIDs) > 0 {
		return methodCountrySelection{madrid: madridIDs, single: singleIDs}
	}
	legacyIDs := legacyQuotationCountryIDs(q)
	methods := legacyQuotationRegistrationMethods(q)
	selection := methodCountrySelection{
		madrid: make([]uuid.UUID, 0),
		single: make([]uuid.UUID, 0),
	}
	for _, method := range methods {
		switch method {
		case "madrid":
			selection.madrid = append(selection.madrid, legacyIDs...)
		case "single":
			selection.single = append(selection.single, legacyIDs...)
		}
	}
	return selection
}

func quotationCountryIDs(q *Quotation) []uuid.UUID {
	ids := decodeJSONB[uuid.UUID](q.CountryIDs)
	if len(ids) == 0 {
		selection := quotationMethodCountrySelection(q)
		ids = selection.countryIDs()
	}
	if len(ids) == 0 && q.CountryID != uuid.Nil {
		return []uuid.UUID{q.CountryID}
	}
	return ids
}

func quotationMadridCountryIDs(q *Quotation) []uuid.UUID {
	return quotationMethodCountrySelection(q).madrid
}

func quotationSingleCountryIDs(q *Quotation) []uuid.UUID {
	return quotationMethodCountrySelection(q).single
}

func quotationNiceCategoryCodes(q *Quotation) []int {
	return decodeJSONB[int](q.NiceCategoryCodes)
}

func quotationRegistrationMethods(q *Quotation) []string {
	methods := decodeJSONB[string](q.RegistrationMethods)
	if len(methods) == 0 {
		methods = quotationMethodCountrySelection(q).registrationMethods()
	}
	if len(methods) == 0 {
		return []string{"single"}
	}
	return methods
}

func applyQuotationFormDefaults(q *Quotation) error {
	selection, err := normalizeMethodCountrySelection(
		q.CountryID,
		decodeJSONB[uuid.UUID](q.CountryIDs),
		decodeJSONB[string](q.RegistrationMethods),
		decodeJSONB[uuid.UUID](q.MadridCountryIDs),
		decodeJSONB[uuid.UUID](q.SingleCountryIDs),
	)
	if err != nil {
		return err
	}
	countryIDs := selection.countryIDs()
	registrationMethods := selection.registrationMethods()
	if len(countryIDs) == 0 {
		return ErrInvalidFormInput
	}
	q.CountryID = countryIDs[0]
	q.CountryIDs, err = encodeJSONB(countryIDs)
	if err != nil {
		return err
	}
	q.MadridCountryIDs, err = encodeJSONB(selection.madrid)
	if err != nil {
		return err
	}
	q.SingleCountryIDs, err = encodeJSONB(selection.single)
	if err != nil {
		return err
	}
	q.RegistrationMethods, err = encodeJSONB(registrationMethods)
	if err != nil {
		return err
	}
	if len(q.NiceCategoryCodes) == 0 {
		raw, err := encodeJSONB([]int{})
		if err != nil {
			return err
		}
		q.NiceCategoryCodes = raw
	}
	if q.AgentLevel == "" {
		q.AgentLevel = agentLevelFromServiceTier(q.ServiceTier)
	}
	if len(q.InfoSections) == 0 {
		raw, err := encodeJSONB([]string{})
		if err != nil {
			return err
		}
		q.InfoSections = raw
	}
	return nil
}
