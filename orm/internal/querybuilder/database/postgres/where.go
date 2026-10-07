package postgres

import (
	"encoding/json"
	"log"

	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/jsonutils"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/structs"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/db/base"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/db/interfaces"
)

type WherePostgreSQLBuilder struct {
	base.WhereBaseBuilder
	whereBaseBuilder *base.WhereBaseBuilder
	u                interfaces.SQLUtils
}

func NewWherePostgreSQLBuilder(util interfaces.SQLUtils, wg []structs.WhereGroup) *WherePostgreSQLBuilder {
	return &WherePostgreSQLBuilder{
		whereBaseBuilder: base.NewWhereBaseBuilder(util, wg),
		u:                util,
	}
}

func (wb *WherePostgreSQLBuilder) Where(sb *[]byte, wg []structs.WhereGroup) ([]interface{}, error) {
	values, _, err := base.RenderPredicates(sb, wg, 0, wb.RenderLeaf)
	return values, err
}
func (wb *WherePostgreSQLBuilder) RenderLeaf(sb *[]byte, c structs.Where) ([]any, error) {
	switch {
	case c.FullText != nil:
		return wb.ProcessFullText(sb, c)
	case c.JsonContains != nil:
		return wb.ProcessJsonContains(sb, c), nil
	case c.JsonLength != nil:
		return wb.ProcessJsonLength(sb, c), nil
	default:
		return wb.whereBaseBuilder.RenderLeaf(sb, c)
	}
}

func (wb *WherePostgreSQLBuilder) ProcessFullText(sb *[]byte, c structs.Where) ([]interface{}, error) {
	values := make([]interface{}, 0)

	// parse options
	language := "english"
	if c.FullText.Options != nil {
		if lang, ok := c.FullText.Options["language"]; ok {
			language = lang.(string)
		}
	}

	mode := "plainto_tsquery"
	if c.FullText.Options != nil {
		if mmode, ok := c.FullText.Options["mode"]; ok {
			if mmode.(string) == "phrase" {
				mode = "phraseto_tsquery"
			}
			if mmode.(string) == "websearch" {
				mode = "websearch_to_tsquery"
			}
		}
	}

	*sb = append(*sb, "("...)
	for i, column := range c.FullText.Columns {
		if i > 0 {
			*sb = append(*sb, " || "...)
		}
		*sb = append(*sb, "to_tsvector("...)
		*sb = append(*sb, wb.u.GetPlaceholder()...)
		*sb = append(*sb, ", "...)
		*sb = wb.u.EscapeReference(*sb, column)
		*sb = append(*sb, ")"...)
		values = append(values, language)
	}
	*sb = append(*sb, ") @@ "+mode+"("+wb.u.GetPlaceholder()+", "+wb.u.GetPlaceholder()+")"...)
	values = append(values, language, c.FullText.Search)

	return values, nil
}

func (wb *WherePostgreSQLBuilder) ProcessJsonContains(sb *[]byte, c structs.Where) []interface{} {
	field, path := jsonutils.ParseJsonFieldAndPath(c.Column)
	*sb = append(*sb, jsonutils.BuildJsonPathSQL(wb.u, field, path)...)
	*sb = append(*sb, "::jsonb @> "...)
	*sb = append(*sb, wb.u.GetPlaceholder()...)

	var jsonVal []byte
	var err error
	if len(c.JsonContains.Values) == 1 {
		jsonVal, err = json.Marshal(c.JsonContains.Values[0])
	} else {
		jsonVal, err = json.Marshal(c.JsonContains.Values)
	}
	if err != nil {
		log.Print("PUBLIC_OUTPUT: JSON encoding failed; details omitted")
	}
	return []interface{}{string(jsonVal)}
}

func (wb *WherePostgreSQLBuilder) ProcessJsonLength(sb *[]byte, c structs.Where) []interface{} {
	field, path := jsonutils.ParseJsonFieldAndPath(c.Column)
	*sb = append(*sb, "jsonb_array_length("...)
	*sb = append(*sb, jsonutils.BuildJsonPathSQL(wb.u, field, path)...)
	*sb = append(*sb, "::jsonb) "...)
	*sb = append(*sb, c.JsonLength.Operator...)
	*sb = append(*sb, " "...)
	*sb = append(*sb, wb.u.GetPlaceholder()...)
	return []interface{}{c.JsonLength.Value}
}
