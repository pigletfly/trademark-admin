package pricing

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"

	"github.com/google/uuid"
)

const defaultMadridCHFExchangeRateCNYCents int64 = 880

// ErrInvalidRegistrationMethod is returned when a quote asks for an
// unsupported pricing path.
var ErrInvalidRegistrationMethod = errors.New("pricing: invalid registration method")

// MethodCalcInput selects method-based pricing rows for a quotation.
type MethodCalcInput struct {
	CountryIDs                    []uuid.UUID
	RegistrationMethods           []string
	NiceCategoryCount             int
	MadridCHFExchangeRateCNYCents int64
}

// MethodPricingSet carries the active rows needed to calculate a quote.
type MethodPricingSet struct {
	Madrid      []MadridPricingEntry
	SingleClass []SingleClassPricingEntry
}

// CalculateMethodPricing reduces Madrid and single-filing pricing rows
// into deterministic quotation lines.
func CalculateMethodPricing(set MethodPricingSet, input MethodCalcInput) (CalcResult, error) {
	methods, err := normalizeMethodInput(input.RegistrationMethods)
	if err != nil {
		return CalcResult{}, err
	}
	if len(input.CountryIDs) == 0 {
		return CalcResult{}, ErrNoMatchingEntries
	}
	classCount := input.NiceCategoryCount
	if classCount < 1 {
		classCount = 1
	}
	chfRate := input.MadridCHFExchangeRateCNYCents
	if chfRate <= 0 {
		chfRate = defaultMadridCHFExchangeRateCNYCents
	}

	var lines []CalcLine
	var total int64
	for _, method := range methods {
		switch method {
		case RegistrationMethodSingle:
			next, subtotal, err := calculateSingleClassLines(set.SingleClass, input.CountryIDs, classCount)
			if err != nil {
				return CalcResult{}, err
			}
			lines = append(lines, next...)
			total += subtotal
		case RegistrationMethodMadrid:
			next, subtotal, err := calculateMadridLines(set.Madrid, input.CountryIDs, chfRate)
			if err != nil {
				return CalcResult{}, err
			}
			lines = append(lines, next...)
			total += subtotal
		}
	}
	if len(lines) == 0 {
		return CalcResult{}, ErrNoMatchingEntries
	}
	return CalcResult{
		Lines:         lines,
		TotalCNYCents: total,
		Signature:     methodSignature(input, lines, total),
	}, nil
}

func normalizeMethodInput(methods []string) ([]string, error) {
	if len(methods) == 0 {
		return []string{RegistrationMethodSingle}, nil
	}
	out := make([]string, 0, len(methods))
	for _, method := range methods {
		if !IsValidRegistrationMethod(method) {
			return nil, ErrInvalidRegistrationMethod
		}
		if !containsString(out, method) {
			out = append(out, method)
		}
	}
	return out, nil
}

func calculateSingleClassLines(entries []SingleClassPricingEntry, countryIDs []uuid.UUID, classCount int) ([]CalcLine, int64, error) {
	activeByCountry := map[uuid.UUID]SingleClassPricingEntry{}
	for _, entry := range entries {
		if entry.EffectiveTo != nil {
			continue
		}
		activeByCountry[entry.CountryID] = entry
	}

	lines := make([]CalcLine, 0, len(countryIDs)*2)
	var total int64
	for _, countryID := range countryIDs {
		entry, ok := activeByCountry[countryID]
		if !ok {
			return nil, 0, ErrNoMatchingEntries
		}
		sourceID := entry.ID
		unit := entry.FirstClassFeeCNYCents
		cid := countryID
		lines = append(lines, CalcLine{
			FeeItem:            "Single filing first class fee",
			AmountCNYCents:     entry.FirstClassFeeCNYCents,
			SourcePricingTable: SingleClassPricingTable,
			SourcePricingID:    &sourceID,
			RegistrationMethod: RegistrationMethodSingle,
			CountryID:          &cid,
			CountryArea:        entry.CountryArea,
			Quantity:           1,
			UnitAmountCNYCents: &unit,
		})
		total += entry.FirstClassFeeCNYCents
		additionalClasses := classCount - 1
		if additionalClasses > 0 {
			additionalUnit := entry.AdditionalClassFeeCNYCents
			lines = append(lines, CalcLine{
				FeeItem:            "Single filing additional class fee",
				AmountCNYCents:     entry.AdditionalClassFeeCNYCents * int64(additionalClasses),
				SourcePricingTable: SingleClassPricingTable,
				SourcePricingID:    &sourceID,
				RegistrationMethod: RegistrationMethodSingle,
				CountryID:          &cid,
				CountryArea:        entry.CountryArea,
				Quantity:           additionalClasses,
				UnitAmountCNYCents: &additionalUnit,
			})
			total += entry.AdditionalClassFeeCNYCents * int64(additionalClasses)
		}
	}
	return lines, total, nil
}

func calculateMadridLines(entries []MadridPricingEntry, countryIDs []uuid.UUID, chfRateCNYCents int64) ([]CalcLine, int64, error) {
	var base *MadridPricingEntry
	activeByCountry := map[uuid.UUID]MadridPricingEntry{}
	for i := range entries {
		entry := entries[i]
		if entry.EffectiveTo != nil {
			continue
		}
		if entry.IsBaseFee {
			copied := entry
			base = &copied
			continue
		}
		if entry.CountryID != nil {
			activeByCountry[*entry.CountryID] = entry
		}
	}
	if base == nil {
		return nil, 0, ErrNoMatchingEntries
	}

	lines := make([]CalcLine, 0, 2+len(countryIDs)*2)
	var total int64
	baseID := base.ID
	baseOfficialCHF := base.OfficialFeeCHFCents
	baseOfficialCNY := convertCHFCentsToCNYCents(base.OfficialFeeCHFCents, chfRateCNYCents)
	lines = append(lines, CalcLine{
		FeeItem:             "Madrid base official fee",
		AmountCNYCents:      baseOfficialCNY,
		SourcePricingTable:  MadridPricingTable,
		SourcePricingID:     &baseID,
		RegistrationMethod:  RegistrationMethodMadrid,
		CountryArea:         base.CountryArea,
		Quantity:            1,
		UnitAmountCNYCents:  &baseOfficialCNY,
		OfficialFeeCHFCents: &baseOfficialCHF,
	})
	total += baseOfficialCNY

	baseAgencyUnit := base.AgencyFeeCNYCents
	lines = append(lines, CalcLine{
		FeeItem:            "Madrid base agency fee",
		AmountCNYCents:     base.AgencyFeeCNYCents,
		SourcePricingTable: MadridPricingTable,
		SourcePricingID:    &baseID,
		RegistrationMethod: RegistrationMethodMadrid,
		CountryArea:        base.CountryArea,
		Quantity:           1,
		UnitAmountCNYCents: &baseAgencyUnit,
	})
	total += base.AgencyFeeCNYCents

	for _, countryID := range countryIDs {
		entry, ok := activeByCountry[countryID]
		if !ok {
			return nil, 0, ErrNoMatchingEntries
		}
		sourceID := entry.ID
		cid := countryID
		officialCHF := entry.OfficialFeeCHFCents
		officialCNY := convertCHFCentsToCNYCents(entry.OfficialFeeCHFCents, chfRateCNYCents)
		lines = append(lines, CalcLine{
			FeeItem:             "Madrid designated country official fee",
			AmountCNYCents:      officialCNY,
			SourcePricingTable:  MadridPricingTable,
			SourcePricingID:     &sourceID,
			RegistrationMethod:  RegistrationMethodMadrid,
			CountryID:           &cid,
			CountryArea:         entry.CountryArea,
			Quantity:            1,
			UnitAmountCNYCents:  &officialCNY,
			OfficialFeeCHFCents: &officialCHF,
		})
		total += officialCNY

		agencyUnit := entry.AgencyFeeCNYCents
		lines = append(lines, CalcLine{
			FeeItem:            "Madrid designated country agency fee",
			AmountCNYCents:     entry.AgencyFeeCNYCents,
			SourcePricingTable: MadridPricingTable,
			SourcePricingID:    &sourceID,
			RegistrationMethod: RegistrationMethodMadrid,
			CountryID:          &cid,
			CountryArea:        entry.CountryArea,
			Quantity:           1,
			UnitAmountCNYCents: &agencyUnit,
		})
		total += entry.AgencyFeeCNYCents
	}
	return lines, total, nil
}

func convertCHFCentsToCNYCents(chfCents, chfRateCNYCents int64) int64 {
	return (chfCents*chfRateCNYCents + 50) / 100
}

func methodSignature(input MethodCalcInput, lines []CalcLine, total int64) string {
	sortedLines := make([]CalcLine, len(lines))
	copy(sortedLines, lines)
	sort.Slice(sortedLines, func(i, j int) bool {
		if sortedLines[i].RegistrationMethod != sortedLines[j].RegistrationMethod {
			return sortedLines[i].RegistrationMethod < sortedLines[j].RegistrationMethod
		}
		if sortedLines[i].CountryArea != sortedLines[j].CountryArea {
			return sortedLines[i].CountryArea < sortedLines[j].CountryArea
		}
		if sortedLines[i].FeeItem != sortedLines[j].FeeItem {
			return sortedLines[i].FeeItem < sortedLines[j].FeeItem
		}
		return sortedLines[i].AmountCNYCents < sortedLines[j].AmountCNYCents
	})

	h := sha256.New()
	fmt.Fprintf(h, "method-v1|classes=%d|", input.NiceCategoryCount)
	for _, line := range sortedLines {
		countryID := ""
		if line.CountryID != nil {
			countryID = line.CountryID.String()
		}
		sourceID := ""
		if line.SourcePricingID != nil {
			sourceID = line.SourcePricingID.String()
		}
		unit := int64(0)
		if line.UnitAmountCNYCents != nil {
			unit = *line.UnitAmountCNYCents
		}
		chf := int64(0)
		if line.OfficialFeeCHFCents != nil {
			chf = *line.OfficialFeeCHFCents
		}
		fmt.Fprintf(
			h,
			"%s|%s|%s|%s|%s|%d:%s|qty=%d|unit=%d|chf=%d|amount=%d;",
			line.RegistrationMethod,
			countryID,
			line.CountryArea,
			line.SourcePricingTable,
			sourceID,
			len(line.FeeItem),
			line.FeeItem,
			line.Quantity,
			unit,
			chf,
			line.AmountCNYCents,
		)
	}
	fmt.Fprintf(h, "=%d", total)
	return hex.EncodeToString(h.Sum(nil))
}

func containsString(values []string, value string) bool {
	for _, current := range values {
		if current == value {
			return true
		}
	}
	return false
}
