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

func quotationCountryIDs(q *Quotation) []uuid.UUID {
	ids := decodeJSONB[uuid.UUID](q.CountryIDs)
	if len(ids) == 0 && q.CountryID != uuid.Nil {
		return []uuid.UUID{q.CountryID}
	}
	return ids
}

func applyQuotationFormDefaults(q *Quotation) error {
	if len(q.CountryIDs) == 0 {
		countryIDs, err := normalizeCountryIDs(q.CountryID, nil)
		if err != nil {
			return err
		}
		raw, err := encodeJSONB(countryIDs)
		if err != nil {
			return err
		}
		q.CountryIDs = raw
	}
	if len(q.NiceCategoryCodes) == 0 {
		raw, err := encodeJSONB([]int{})
		if err != nil {
			return err
		}
		q.NiceCategoryCodes = raw
	}
	if len(q.RegistrationMethods) == 0 {
		raw, err := encodeJSONB([]string{"single"})
		if err != nil {
			return err
		}
		q.RegistrationMethods = raw
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
