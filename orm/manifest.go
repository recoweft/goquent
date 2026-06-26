package orm

import (
	"time"

	"github.com/recoweft/goquent/orm/manifest"
)

type Manifest = manifest.Manifest
type ManifestTable = manifest.Table
type ManifestColumn = manifest.Column
type ManifestIndex = manifest.Index
type ManifestRelation = manifest.Relation
type ManifestPolicy = manifest.Policy
type ManifestQueryExample = manifest.QueryExample
type ManifestVerification = manifest.Verification
type ManifestFreshnessCheck = manifest.FreshnessCheck
type ManifestOptions = manifest.Options

const (
	ManifestVersion           = manifest.Version
	WarningManifestStale      = manifest.WarningStale
	WarningManifestUnreadable = manifest.WarningUnreadable
)

func GenerateManifest(opts ManifestOptions) (*Manifest, error) {
	return manifest.Generate(opts)
}

func LoadManifest(path string) (*Manifest, error) {
	return manifest.Load(path)
}

func VerifyManifest(stored, current *Manifest) ManifestVerification {
	return manifest.Verify(stored, current, time.Time{})
}

func ManifestJSONSchema() ([]byte, error) {
	return manifest.JSONSchema()
}

func ValidateManifest(m *Manifest) error {
	return manifest.Validate(m)
}
