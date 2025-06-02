package querybuilder

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"time"
)

var (
	ErrUnknownColumn = errors.New("querybuilder: unknown column")
	ErrBadOperator   = errors.New("querybuilder: bad operator or value type")
)

type Filter struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`

	Type  string      `json:"type"`
	Value interface{} `json:"value"`
}

type Pagination struct {
	OrderBy        string `json:"order_by"`
	OrderDirection string `json:"order_direction"`
	Limit          uint64 `json:"limit"`
	Offset         uint64 `json:"offset"`
}

type ListRequest struct {
	Filters []Filter `json:"filters"`
	Fields  []string `json:"fields"`

	OrderBy        string `json:"order_by"`
	OrderDirection string `json:"order_direction"`
	Limit          uint64 `json:"limit"`
	Offset         uint64 `json:"offset"`
}

func (lr ListRequest) Pagination() Pagination {
	return Pagination{lr.OrderBy, lr.OrderDirection, lr.Limit, lr.Offset}
}

type modelMeta struct {
	BaseTable string
	Columns   map[string]string
	Relations []relationMeta
}

type relationMeta struct {
	Name        string
	Kind        string
	JoinTable   string
	TargetTable string
	JoinType    string
}

func parseModel(t reflect.Type) (modelMeta, error) {
	if t.Kind() != reflect.Struct {
		return modelMeta{}, errors.New("querybuilder: model must be struct")
	}
	var m modelMeta

	if mname, ok := reflect.Zero(t).Interface().(interface{ TableName() string }); ok {
		m.BaseTable = mname.TableName()
	} else {

		base := strings.TrimSuffix(t.Name(), "QueryBuilder")
		m.BaseTable = camelToSnake(base)
	}

	m.Columns = make(map[string]string)

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag := f.Tag.Get("schema")
		if tag == "" {
			continue
		}
		parts := strings.Split(tag, ";")
		for _, p := range parts {
			if strings.HasPrefix(p, "column=") {
				col := strings.TrimPrefix(p, "column=")
				m.Columns[f.Name] = col
			} else if strings.HasPrefix(p, "one2many:") {
				m.Relations = append(m.Relations, relationMeta{
					Name: f.Name, Kind: "one2many", JoinTable: strings.TrimPrefix(p, "one2many:"), JoinType: joinType(parts),
				})
			} else if strings.HasPrefix(p, "many2many:") {
				rest := strings.TrimPrefix(p, "many2many:")
				link, target := rest, ""
				if parts2 := strings.SplitN(rest, ":", 2); len(parts2) == 2 {
					link, target = parts2[0], parts2[1]
				}
				m.Relations = append(m.Relations, relationMeta{
					Name:        f.Name,
					Kind:        "many2many",
					JoinTable:   link,
					TargetTable: target,
					JoinType:    joinType(parts),
				})
			}
		}
	}
	return m, nil
}

func joinType(parts []string) string {
	for _, p := range parts {
		if strings.HasPrefix(p, "join=") {
			kind := strings.ToUpper(strings.TrimPrefix(p, "join="))
			switch kind {
			case "LEFT", "RIGHT", "FULL", "CROSS", "INNER":
				return kind
			}
		}
	}
	return "INNER"
}

var camelRE = regexp.MustCompile(`[A-Z][^A-Z]*`)

func camelToSnake(s string) string {
	words := camelRE.FindAllString(s, -1)
	for i, w := range words {
		words[i] = strings.ToLower(w)
	}
	return strings.Join(words, "_")
}

type SQLBuilder struct {
	meta modelMeta
	req  ListRequest
}

var allowedOperators = map[string]struct{}{
	"eq": {}, "ne": {}, "gt": {}, "lt": {}, "gte": {}, "lte": {},
	"like": {}, "ilike": {}, "in": {}, "isnull": {},
	"contains": {}, "startswith": {}, "endswith": {},
}

func castValue(f Filter) (interface{}, error) {
	switch strings.ToLower(f.Type) {
	case "int":
		switch v := f.Value.(type) {
		case float64:
			return int64(v), nil
		case string:
			return fmt.Sscan(v)
		default:
			return v, nil
		}
	case "float":
		return f.Value, nil
	case "bool":
		return f.Value, nil
	case "date", "datetime":
		switch v := f.Value.(type) {
		case string:
			layouts := []string{
				"02-01-2006 15:04:05",
				"02-01-2006 15:04",
				"02-01-2006",
			}
			for _, layout := range layouts {
				if t, err := time.ParseInLocation(layout, v, time.UTC); err == nil {
					return t, nil
				}
			}
			return nil, fmt.Errorf("querybuilder: failed to parse datetime: %s", v)
		default:
			return nil, ErrBadOperator
		}
	default:
		return f.Value, nil
	}
}

func BuildAST(model any, req ListRequest) (Builder, []any, error) {
	t := reflect.TypeOf(model)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return Builder{}, nil, fmt.Errorf("querybuilder: model must be struct, got %s", t.Kind())
	}

	meta, err := parseModel(t)
	if err != nil {
		return Builder{}, nil, err
	}
	baseAlias := "t"
	baseStr := fmt.Sprintf("%s %s", meta.BaseTable, baseAlias)

	b := Builder{
		Base:   baseStr,
		Limit:  int(req.Limit),
		Offset: int(req.Offset),
	}
	colExpr := func(goField string) string {
		if col, ok := meta.Columns[goField]; ok {
			return fmt.Sprintf("%s.%s", baseAlias, col)
		}
		if strings.Contains(goField, ".") {
			parts := strings.SplitN(goField, ".", 2)
			for _, rel := range meta.Relations {
				if strings.ToLower(rel.Name) == parts[0] {
					target := rel.JoinTable
					if rel.TargetTable != "" {
						target = rel.TargetTable
					}
					return fmt.Sprintf("%s.%s", target, parts[1])
				}
			}
		}
		return ""
	}

	selected := req.Fields
	if len(selected) == 0 {
		for f := range meta.Columns {
			selected = append(selected, f)
		}
	}
	for _, f := range selected {
		if expr := colExpr(f); expr != "" {
			b.Select = append(b.Select, Column{Expr: expr, As: strings.ToLower(f)})
		}
	}

	if hl, ok := model.(HasLaterals); ok {
		for _, ls := range hl.Laterals() {
			b.Joins = append(b.Joins, Join{
				Kind:    ls.Kind,
				Lateral: true,
				Table:   "(" + strings.TrimSpace(ls.SQL) + ")",
				Alias:   ls.Alias,
				On:      "true",
			})
			for as, expr := range ls.Columns {
				b.Select = append(b.Select, Column{
					Expr: fmt.Sprintf("%s AS %s", expr, as),
					As:   as,
				})
			}
		}
	}
	for _, rel := range meta.Relations {
		fieldWanted := false
		for _, f := range selected {
			if f == rel.Name {
				fieldWanted = true
				break
			}
		}
		if !fieldWanted {
			continue
		}

		switch rel.Kind {
		case "one2many":
			alias := rel.JoinTable
			b.Joins = append(b.Joins, Join{
				Kind:  rel.JoinType,
				Table: rel.JoinTable,
				Alias: alias,
				On:    fmt.Sprintf("%s.id = %s.%s_id", baseAlias, alias, strings.TrimSuffix(meta.BaseTable, "s")),
			})
			b.Select = append(b.Select, Column{
				Expr: fmt.Sprintf("COALESCE(jsonb_agg(DISTINCT row_to_json(%s)::jsonb),'[]'::jsonb)", alias),
				As:   strings.ToLower(rel.Name),
			})
		case "many2many":
			link := rel.JoinTable
			linkAlias := link
			baseSing := strings.TrimSuffix(meta.BaseTable, "s")
			b.Joins = append(b.Joins, Join{
				Kind:  rel.JoinType,
				Table: link,
				Alias: linkAlias,
				On:    fmt.Sprintf("%s.id = %s.%s_id", baseAlias, linkAlias, baseSing),
			})
			targetAlias := linkAlias
			if rel.TargetTable != "" {
				targetAlias = rel.TargetTable
				targetSing := strings.TrimSuffix(rel.TargetTable, "s")
				b.Joins = append(b.Joins, Join{
					Kind:  rel.JoinType,
					Table: rel.TargetTable,
					Alias: targetAlias,
					On:    fmt.Sprintf("%s.%s_id = %s.id", linkAlias, targetSing, targetAlias),
				})
			}
			b.Select = append(b.Select, Column{
				Expr: fmt.Sprintf("COALESCE(jsonb_agg(DISTINCT row_to_json(%s)::jsonb),'[]'::jsonb)", targetAlias),
				As:   strings.ToLower(rel.Name),
			})
		}
	}
	var args []any
	phCounter := 1
	nextPH := func() string {
		s := fmt.Sprintf("$%d", phCounter)
		phCounter++
		return s
	}

	for _, ft := range req.Filters {
		dbCol := colExpr(ft.Field)
		if dbCol == "" {
			return Builder{}, nil, fmt.Errorf("%w: %s", ErrUnknownColumn, ft.Field)
		}
		predExpr, predArgs, err := buildPredicate(dbCol, ft, nextPH)
		if err != nil {
			return Builder{}, nil, err
		}
		b.Where = append(b.Where, Predicate{Expr: predExpr, Args: predArgs})
		args = append(args, predArgs...)
	}
	if req.OrderBy != "" {
		if col := colExpr(req.OrderBy); col != "" {
			dir := strings.ToUpper(req.OrderDirection)
			if dir != "DESC" {
				dir = "ASC"
			}
			b.Order = fmt.Sprintf("%s %s", col, dir)
		}
	}
	hasAgg := false
	for _, c := range b.Select {
		if strings.Contains(c.Expr, "jsonb_agg") ||
			strings.Contains(c.Expr, "row_to_json") {
			hasAgg = true
			break
		}
	}
	if hasAgg {
		for _, c := range b.Select {
			if strings.Contains(c.Expr, "jsonb_agg") ||
				strings.Contains(c.Expr, "row_to_json") {
				continue
			}
			exprNoAlias := c.Expr
			if idx := strings.Index(strings.ToLower(c.Expr), " as "); idx != -1 {
				exprNoAlias = c.Expr[:idx]
			}
			b.Group = append(b.Group, exprNoAlias)
		}
	}

	if b.Limit == 0 {
		b.Limit = 25
	}

	return b, args, nil
}
func buildPredicate(dbCol string, f Filter, ph func() string) (string, []any, error) {
	op := strings.ToLower(f.Operator)
	_, ok := allowedOperators[op]
	if !ok {
		return "", nil, fmt.Errorf("%w: %s", ErrBadOperator, f.Operator)
	}
	val, err := castValue(f)
	if err != nil {
		return "", nil, err
	}

	switch op {
	case "eq":
		if f.Type == "date" || f.Type == "datetime" {
			if start, end, ok := tryParseTimeEquality(f.Value); ok {
				return fmt.Sprintf("(%s) BETWEEN %s AND %s", dbCol, ph(), ph()), []any{start, end}, nil
			}
		}
		return dbCol + " = " + ph(), []any{val}, nil
	case "ne":
		return dbCol + " <> " + ph(), []any{val}, nil
	case "gt":
		return dbCol + " > " + ph(), []any{val}, nil
	case "lt":
		return dbCol + " < " + ph(), []any{val}, nil
	case "gte":
		return dbCol + " >= " + ph(), []any{val}, nil
	case "lte":
		return dbCol + " <= " + ph(), []any{val}, nil
	case "contains":
		s, ok := val.(string)
		if !ok {
			return "", nil, ErrBadOperator
		}
		return dbCol + " ILIKE " + ph(), []any{"%" + s + "%"}, nil
	case "startswith":
		s, ok := val.(string)
		if !ok {
			return "", nil, ErrBadOperator
		}
		return dbCol + " ILIKE " + ph(), []any{s + "%"}, nil
	case "endswith":
		s, ok := val.(string)
		if !ok {
			return "", nil, ErrBadOperator
		}
		return dbCol + " ILIKE " + ph(), []any{"%" + s}, nil
	case "isnull":
		b, ok := val.(bool)
		if !ok {
			return "", nil, ErrBadOperator
		}
		if b {
			return dbCol + " IS NULL", nil, nil
		}
		return dbCol + " IS NOT NULL", nil, nil
	case "in":
		switch vs := val.(type) {
		case []interface{}:
			var finalVals []any
			for _, v := range vs {
				switch v := v.(type) {
				case float64:
					finalVals = append(finalVals, int64(v))
				case int:
					finalVals = append(finalVals, int64(v))
				case int64:
					finalVals = append(finalVals, v)
				case string:
					finalVals = append(finalVals, v)
				default:
					return "", nil, fmt.Errorf("querybuilder: unsupported type in 'in' array: %T", v)
				}
			}
			phs := make([]string, len(finalVals))
			for i := range finalVals {
				phs[i] = ph()
			}
			return fmt.Sprintf("%s IN (%s)", dbCol, strings.Join(phs, ",")), finalVals, nil
		default:
			return "", nil, fmt.Errorf("querybuilder: expected array for 'in', got: %T", val)
		}
	default:
		return "", nil, ErrBadOperator
	}
}

func tryParseTimeEquality(value interface{}) (start, end time.Time, ok bool) {
	strVal, ok := value.(string)
	if !ok {
		return time.Time{}, time.Time{}, false
	}

	layouts := []struct {
		layout string
		add    time.Duration
	}{
		{"02-01-2006 15:04:05", time.Second - time.Nanosecond},
		{"02-01-2006 15:04", time.Minute - time.Nanosecond},
		{"02-01-2006", 24*time.Hour - time.Nanosecond},
	}

	for _, l := range layouts {
		t, err := time.ParseInLocation(l.layout, strVal, time.UTC)
		if err == nil {
			return t, t.Add(l.add), true
		}
	}

	return time.Time{}, time.Time{}, false
}
