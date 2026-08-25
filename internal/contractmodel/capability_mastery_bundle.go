package contractmodel

import (
	"encoding/json"
	"fmt"
	"regexp"
)

const CapabilityMasteryBundleContractVersion = "capability-mastery-bundle.v1"

type CapabilityMasteryContractError struct {
	Code   string
	Detail string
}

func (e *CapabilityMasteryContractError) Error() string {
	return fmt.Sprintf("capability mastery contract %s: %s", e.Code, e.Detail)
}

var capabilityMasterySHA256 = regexp.MustCompile(`^[a-f0-9]{64}$`)

// ValidateCapabilityMasteryBundleJSON checks only the released identity join
// that Brain is allowed to see. Learner evidence and execution remain outside
// this package's ownership boundary.
func ValidateCapabilityMasteryBundleJSON(payload []byte) error {
	var root map[string]any
	if err := json.Unmarshal(payload, &root); err != nil {
		return &CapabilityMasteryContractError{Code: "invalid_bundle", Detail: "invalid JSON"}
	}
	if root["contractVersion"] != CapabilityMasteryBundleContractVersion {
		return &CapabilityMasteryContractError{Code: "unsupported_contract_version", Detail: "expected capability-mastery-bundle.v1"}
	}
	for key, version := range map[string]string{"capabilityDossier": "capability-dossier.v1", "assessmentPlan": "capability-assessment-plan.v1", "learningSession": "learning-session.v1", "evidence": "capability-evidence.v2", "mastery": "capability-mastery.v2", "reflection": "session-reflection.v1"} {
		section, ok := object(root[key])
		if !ok || section["contractVersion"] != version {
			return &CapabilityMasteryContractError{Code: "invalid_bundle", Detail: key + " version"}
		}
	}
	dossier := mustObject(root["capabilityDossier"])
	plan := mustObject(root["assessmentPlan"])
	session := mustObject(root["learningSession"])
	mastery := mustObject(root["mastery"])
	primary, ok := object(dossier["primaryQuestion"])
	if !ok || primary["status"] != "published" || primary["role"] != "primary" || !locales(primary["locales"]) || !hash(primary["contentHash"]) {
		return &CapabilityMasteryContractError{Code: "incomplete-role-specific-primary-card", Detail: "primary card"}
	}
	registry, ok := object(dossier["registry"])
	if !ok || registry["key"] != dossier["capabilityKey"] || registry["lifecycle"] != "active" {
		return &CapabilityMasteryContractError{Code: "inactive-capability-registry", Detail: "registry"}
	}
	family, ok := object(dossier["taskFamily"])
	if !ok || family["assessmentPlanId"] != plan["planId"] {
		return &CapabilityMasteryContractError{Code: "family-assessment-plan-missing", Detail: "assessment plan"}
	}
	selected, ok := object(session["taskRevision"])
	if !ok || session["cardRevisionId"] != primary["revisionId"] || session["cardContentHash"] != primary["contentHash"] {
		return &CapabilityMasteryContractError{Code: "stale-question-card-hash", Detail: "question identity"}
	}
	var revision map[string]any
	for _, value := range asSlice(dossier["revisions"]) {
		candidate, ok := object(value)
		if ok && candidate["taskId"] == selected["taskId"] && number(candidate["revision"]) == number(selected["revision"]) {
			revision = candidate
			break
		}
	}
	if revision == nil || revision["immutableHash"] != selected["immutableHash"] || !hash(selected["immutableHash"]) {
		return &CapabilityMasteryContractError{Code: "stale-task-revision-hash", Detail: "task identity"}
	}
	if revision["profile"] != selected["profile"] {
		return &CapabilityMasteryContractError{Code: "wrong-profile-for-capability", Detail: "profile"}
	}
	if session["releaseReadiness"] == "released" && (family["status"] != "released" || family["runnable"] != true || revision["status"] != "released") {
		return &CapabilityMasteryContractError{Code: "contradictory-released-runnable", Detail: "release"}
	}
	if mastery["provenance"] != "human" {
		return &CapabilityMasteryContractError{Code: "non-human-mastery-provenance", Detail: "mastery provenance"}
	}
	return nil
}

func object(value any) (map[string]any, bool) { item, ok := value.(map[string]any); return item, ok }
func mustObject(value any) map[string]any     { item, _ := object(value); return item }
func asSlice(value any) []any                 { items, _ := value.([]any); return items }
func number(value any) float64                { n, _ := value.(float64); return n }
func hash(value any) bool {
	text, ok := value.(string)
	return ok && capabilityMasterySHA256.MatchString(text)
}
func locales(value any) bool {
	values, ok := value.([]any)
	if !ok {
		return false
	}
	en, ru := false, false
	for _, item := range values {
		if text, ok := item.(string); ok {
			en = en || text == "en"
			ru = ru || text == "ru"
		}
	}
	return en && ru
}
