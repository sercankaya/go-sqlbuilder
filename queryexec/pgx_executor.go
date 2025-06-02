package queryexec

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5/pgtype"
	"reflect"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	entity "SqlBuilder/model"
	"SqlBuilder/querybuilder"
)

func camelToSnake(s string) string {
	var out []rune
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			out = append(out, '_')
		}
		out = append(out, rune(strings.ToLower(string(r))[0]))
	}
	return string(out)
}

func Fetch(ctx context.Context, pool *pgxpool.Pool,
	model any, req querybuilder.ListRequest) ([]map[string]any, error) {

	ast, args, err := querybuilder.BuildAST(entity.LocationQueryBuilder{}, req)
	sqlStr, _ := ast.Render()

	rows, err := pool.Query(ctx, sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols := rows.FieldDescriptions()
	formats := fieldFormats(model)
	want := make(map[string]struct{}, len(req.Fields)*2)
	for _, f := range req.Fields {
		lc := strings.ToLower(f)
		want[lc] = struct{}{}
		want[camelToSnake(f)] = struct{}{}
	}

	var out []map[string]any
	for rows.Next() {
		vals, err := rows.Values()
		if err != nil {
			return nil, err
		}
		rec := map[string]any{}
		for i, fd := range cols {
			name := strings.ToLower(fd.Name)
			if len(want) > 0 {
				if _, ok := want[name]; !ok {
					continue
				}
			}

			v := vals[i]

			if t, ok := v.(time.Time); ok {
				if fs, ok := formats[name]; ok {
					v = t.Format(fs)
				}
			} else {
				if fd.DataTypeOID == pgtype.JSONOID || fd.DataTypeOID == pgtype.JSONBOID {
					var raw []byte
					fmt.Printf("col %-20s OID=%d  go-type=%T\n", fd.Name, fd.DataTypeOID, v)
					switch vv := v.(type) {
					case []byte:
						raw = vv
					case string:
						raw = []byte(vv)
					case []interface{}:
						v = applyFormatsPath(vv, formats, strings.ToLower(name))
						if outBytes, err := json.Marshal(v); err == nil {
							v = json.RawMessage(outBytes)
						}
					case map[string]interface{}:
						v = applyFormatsPath(vv, formats, strings.ToLower(name))
						if outBytes, err := json.Marshal(v); err == nil {
							v = json.RawMessage(outBytes)
						}
					default:
						if txt, ok := vv.(interface{ MarshalJSON() ([]byte, error) }); ok {
							if b, err := txt.MarshalJSON(); err == nil {
								raw = b
							}
						}
					}

					if len(raw) > 0 {
						var j any
						if err := json.Unmarshal(raw, &j); err == nil {
							j = applyFormatsPath(j, formats, strings.ToLower(name))
							if outBytes, err := json.Marshal(j); err == nil {
								v = json.RawMessage(outBytes)
							}
						}
					}
				}
			}

			rec[name] = v
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

func fieldFormats(model any) map[string]string {
	out := make(map[string]string)
	seen := make(map[reflect.Type]bool)

	var walk func(t reflect.Type, prefix string)
	walk = func(t reflect.Type, prefix string) {
		if t == nil {
			return
		}
		if t.Kind() == reflect.Ptr {
			t = t.Elem()
		}
		if t.Kind() != reflect.Struct {
			return
		}
		if seen[t] {
			return
		}
		seen[t] = true

		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)

			layout := f.Tag.Get("format")
			if layout == "" {
				if s := f.Tag.Get("schema"); s != "" {
					for _, p := range strings.Split(s, ";") {
						if strings.HasPrefix(p, "format=") {
							layout = strings.TrimPrefix(p, "format=")
						}
					}
				}
			}

			fieldName := strings.ToLower(f.Name)
			baseKey := fieldName
			if s := f.Tag.Get("schema"); s != "" {
				for _, p := range strings.Split(s, ";") {
					if strings.HasPrefix(p, "column=") {
						baseKey = strings.ToLower(strings.TrimPrefix(p, "column="))
					}
				}
			}

			pathKey := baseKey
			if prefix != "" {
				pathKey = prefix + "." + baseKey
			}

			if layout != "" {
				out[pathKey] = layout
				if prefix == "" {
					if _, ok := out[fieldName]; !ok {
						out[fieldName] = layout
					}
				}

				if s := f.Tag.Get("schema"); s != "" {
					for _, p := range strings.Split(s, ";") {
						if strings.HasPrefix(p, "column=") {
							col := strings.ToLower(strings.TrimPrefix(p, "column="))
							colKey := col
							if prefix != "" {
								colKey = prefix + "." + col
							}
							out[colKey] = layout
							if prefix == "" {
								if _, ok := out[col]; !ok {
									out[col] = layout
								}
							}
						}
					}
				}
			}

			ft := f.Type
			for ft.Kind() == reflect.Ptr || ft.Kind() == reflect.Slice || ft.Kind() == reflect.Array {
				ft = ft.Elem()
			}
			if ft.Kind() == reflect.Struct {
				nextPrefix := fieldName
				if prefix != "" {
					nextPrefix = prefix + "." + fieldName
				}
				walk(ft, nextPrefix)
			}
		}
	}

	walk(reflect.TypeOf(model), "")
	return out
}

func applyFormatsPath(node any, formats map[string]string, path string) any {
	switch n := node.(type) {
	case map[string]any:
		for k, v := range n {
			childPath := k
			if path != "" {
				childPath = path + "." + k
			}
			n[k] = applyFormatsPath(v, formats, childPath)

			if str, ok := n[k].(string); ok {
				if layout, ok := formats[strings.ToLower(childPath)]; ok {
					if t, err := time.Parse(time.RFC3339, str); err == nil {
						n[k] = t.Format(layout)
					} else if t, err := time.Parse(time.RFC3339Nano, str); err == nil {
						n[k] = t.Format(layout)
					}
				} else if path == "" {
					if layout, ok := formats[strings.ToLower(k)]; ok {
						if t, err := time.Parse(time.RFC3339, str); err == nil {
							n[k] = t.Format(layout)
						} else if t, err := time.Parse(time.RFC3339Nano, str); err == nil {
							n[k] = t.Format(layout)
						}
					}
				}
			}
		}
	case []any:
		for i, v := range n {
			n[i] = applyFormatsPath(v, formats, path)
		}
	}
	return node
}
