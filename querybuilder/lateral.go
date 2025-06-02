package querybuilder

type LateralSpec struct {
	Columns map[string]string

	SQL   string
	Alias string
	Kind  string
}
type HasLaterals interface {
	Laterals() []LateralSpec
}
