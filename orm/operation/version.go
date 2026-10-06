package operation

import "github.com/recoweft/goquent/orm/internal/planversion"

// JSONVersion is the current diagnostic/input envelope version, not authorization.
const JSONVersion = planversion.Current

// ErrJSONVersion identifies invalid, ambiguous or unsupported envelope versions.
var ErrJSONVersion = planversion.ErrVersion
