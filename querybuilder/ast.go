package querybuilder

type Column struct {
	Expr string
	As   string
}

type Join struct {
	Kind    string
	Lateral bool
	Table   string
	Alias   string
	On      string
}

type Predicate struct {
	Expr string
	Args []any
}

type Builder struct {
	Base   string
	Select []Column
	Joins  []Join
	Where  []Predicate
	Group  []string
	Order  string
	Limit  int
	Offset int
}
