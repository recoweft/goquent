package mysql

import (
	"encoding/json"
	"log"
	"strings"

	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/jsonutils"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/structs"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/db/base"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/db/interfaces"
)

type WhereMySQLBuilder struct {
	base.WhereBaseBuilder
	whereBaseBuilder *base.WhereBaseBuilder
	u                interfaces.SQLUtils
}

func NewWhereMySQLBuilder(util interfaces.SQLUtils, wg []structs.WhereGroup) *WhereMySQLBuilder {
	return &WhereMySQLBuilder{
		whereBaseBuilder: base.NewWhereBaseBuilder(util, wg),
		u:                util,
	}
}

func (wb *WhereMySQLBuilder) Where(sb *[]byte, wg []structs.WhereGroup) ([]interface{}, error) {
	values, _, err := base.RenderPredicates(sb, wg, 0, wb.RenderLeaf)
	return values, err
}
func (wb *WhereMySQLBuilder) RenderLeaf(sb *[]byte, c structs.Where) ([]any, error) {
	switch {
	case c.FullText != nil:
		return wb.ProcessFullText(sb, c), nil
	case c.JsonContains != nil:
		return wb.ProcessJsonContains(sb, c), nil
	case c.JsonLength != nil:
		return wb.ProcessJsonLength(sb, c), nil
	default:
		return wb.whereBaseBuilder.RenderLeaf(sb, c)
	}
}

func (wb *WhereMySQLBuilder) ProcessFullText(sb *[]byte, c structs.Where) []interface{} {
	// parse options
	mode := "IN NATURAL LANGUAGE MODE"
	expand := ""
	if c.FullText.Options != nil {
		if mmode, ok := c.FullText.Options["mode"]; ok {
			if mmode.(string) == "boolean" {
				mode = "IN BOOLEAN MODE"
			}
		}
		if with, ok := c.FullText.Options["expanded"]; ok {
			if with.(bool) {
				expand = " WITH QUERY EXPANSION"
			}
		}
	}

	*sb = append(*sb, "MATCH ("...)
	for i, column := range c.FullText.Columns {
		if i > 0 {
			*sb = append(*sb, ", "...)
		}
		*sb = wb.u.EscapeReference(*sb, column)
	}
	*sb = append(*sb, ") AGAINST ("+wb.u.GetPlaceholder()+" "+mode+expand+")"...)
	values := append(c.Value, c.FullText.Search)

	return values
}

func (wb *WhereMySQLBuilder) ProcessJsonContains(sb *[]byte, c structs.Where) []interface{} {
	field, path := jsonutils.ParseJsonFieldAndPath(c.Column)
	*sb = append(*sb, "JSON_CONTAINS("...)
	*sb = wb.u.EscapeReference(*sb, field)
	*sb = append(*sb, ", "...)
	*sb = append(*sb, wb.u.GetPlaceholder()...)
	if len(path) > 0 {
		*sb = append(*sb, ", '$."+strings.Join(path, ".")+"')"...)
	} else {
		*sb = append(*sb, ")"...)
	}

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

func (wb *WhereMySQLBuilder) ProcessJsonLength(sb *[]byte, c structs.Where) []interface{} {
	field, path := jsonutils.ParseJsonFieldAndPath(c.Column)
	*sb = append(*sb, "JSON_LENGTH("...)
	*sb = wb.u.EscapeReference(*sb, field)
	if len(path) > 0 {
		*sb = append(*sb, ", '$."+strings.Join(path, ".")+"')"...)
	} else {
		*sb = append(*sb, ")"...)
	}
	*sb = append(*sb, " "...)
	*sb = append(*sb, c.JsonLength.Operator...)
	*sb = append(*sb, " "...)
	*sb = append(*sb, wb.u.GetPlaceholder()...)
	return []interface{}{c.JsonLength.Value}
}
