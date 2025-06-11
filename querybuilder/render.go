package querybuilder

import (
	"fmt"
	"strconv"
	"strings"
)

func (b Builder) Render() (string, string, []any) {
	var sql strings.Builder
	var count strings.Builder
	var args []any

	sql.WriteString("SELECT ")
	for i, c := range b.Select {
		if i > 0 {
			sql.WriteByte(',')
		}
		sql.WriteString(c.Expr)
		if c.As != "" && !strings.Contains(strings.ToLower(c.Expr), " as ") {
			sql.WriteString(" AS ")
			sql.WriteString(c.As)
		}
	}
	sql.WriteString(" FROM ")
	sql.WriteString(b.Base)

	for _, j := range b.Joins {
		sql.WriteByte(' ')
		sql.WriteString(j.Kind)
		sql.WriteString(" JOIN ")
		if j.Lateral {
			sql.WriteString("LATERAL ")
		}
		sql.WriteString(j.Table)
		if j.Alias != "" {
			sql.WriteByte(' ')
			sql.WriteString(j.Alias)
		}
		sql.WriteString(" ON ")
		sql.WriteString(j.On)
	}

	if len(b.Where) > 0 {
		sql.WriteString(" WHERE ")
		for i, p := range b.Where {
			if i > 0 {
				sql.WriteString(" AND ")
			}
			sql.WriteString(p.Expr)
			args = append(args, p.Args...)
		}
	}

	if len(b.Group) > 0 {
		sql.WriteString(" GROUP BY ")
		sql.WriteString(strings.Join(b.Group, ","))
	}

	if b.Order != "" {
		sql.WriteString(" ORDER BY ")
		sql.WriteString(b.Order)
	}
	if b.Limit > 0 {
		sql.WriteString(" LIMIT ")
		sql.WriteString(strconv.Itoa(b.Limit))
	}
	sql.WriteString(" OFFSET ")
	sql.WriteString(strconv.Itoa(b.Offset))
	count.WriteString(fmt.Sprintf("SELECT COUNT(*) FROM (%s)", sql.String()))

	return sql.String(), count.String(), args
}
