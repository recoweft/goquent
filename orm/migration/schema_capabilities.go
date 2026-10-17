package migration

import (
	"github.com/recoweft/goquent/orm/publicoutput"
	"regexp"
)

// CapabilityAssessment separates general syntax support from target conditions.
// No assessment authorizes execution or proves a lock-free operation.
type CapabilityAssessment = publicoutput.CapabilityAssessment

// OperationCapability is a closed display classification, not execution evidence.
type OperationCapability = publicoutput.CapabilityView

var pg16Version = regexp.MustCompile(`^16\.[0-9]+(?: \([^\r\n]*\))?$`)
var mysql84Version = regexp.MustCompile(`^8\.4\.[0-9]+$`)

func capabilitiesFor(c *SchemaContext, steps []MigrationStep) []OperationCapability {
	out := make([]OperationCapability, 0, len(steps))
	seen := map[struct {
		op         string
		concurrent bool
	}]bool{}
	for _, s := range steps {
		op := stepType(string(s.Type))
		key := struct {
			op         string
			concurrent bool
		}{op, s.Concurrent}
		if seen[key] {
			continue
		}
		seen[key] = true
		u := CapabilityAssessment{Support: "unknown", Conditions: "unknown"}
		v := OperationCapability{Operation: op, Source: "unverified", LockFree: u, NoRebuild: u, Transactional: u, Concurrent: u, ConcurrentRequested: s.Concurrent}
		if c != nil && c.Coverage.Version == CoverageComplete && c.VersionSource == "catalog_select" && c.observedVersion == c.ServerVersion && c.observedVersion != "" {
			v.Source = "live_observation"
			if c.Dialect == "postgres" && pg16Version.MatchString(c.ServerVersion) {
				switch s.Type {
				case AddIndex:
					v.Concurrent = CapabilityAssessment{Support: "conditional", Conditions: "unknown"}
					v.Transactional = CapabilityAssessment{Support: "supported", Conditions: "unknown"}
					if s.Concurrent {
						v.Transactional = CapabilityAssessment{Support: "unsupported", Conditions: "not_satisfied"}
					}
					v.LockFree = CapabilityAssessment{Support: "unsupported", Conditions: "not_satisfied"}
					v.NoRebuild = CapabilityAssessment{Support: "conditional", Conditions: "unknown"}
				case AddForeignKey, DropForeignKey, AddCheck, DropCheck, AlterColumnDefault:
					v.Transactional = CapabilityAssessment{Support: "supported", Conditions: "unknown"}
					v.LockFree = CapabilityAssessment{Support: "unsupported", Conditions: "not_satisfied"}
				}
			} else if c.Dialect == "mysql" && mysql84Version.MatchString(c.ServerVersion) {
				switch s.Type {
				case AddIndex, DropIndex, AddForeignKey, DropForeignKey, AddCheck, DropCheck, AlterColumnDefault:
					v.Transactional = CapabilityAssessment{Support: "unsupported", Conditions: "not_satisfied"}
					v.Concurrent = CapabilityAssessment{Support: "unsupported", Conditions: "not_satisfied"}
					// Online DDL still takes metadata locks; engine and table conditions are unverified.
					v.LockFree = CapabilityAssessment{Support: "unsupported", Conditions: "not_satisfied"}
					v.NoRebuild = CapabilityAssessment{Support: "conditional", Conditions: "unknown"}
				}
			}
		}
		out = append(out, v)
	}
	return out
}
func safeCapabilities(src []OperationCapability) []OperationCapability {
	out := make([]OperationCapability, min(len(src), 128))
	for i, v := range src[:len(out)] {
		out[i] = v.PublicView()
	}
	return out
}
