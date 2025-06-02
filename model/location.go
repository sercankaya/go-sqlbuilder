package model

import (
	"SqlBuilder/querybuilder"
	"encoding/json"
	"time"
)

type LocationQueryBuilder struct {
	ID         int64           `schema:"column=id"`
	CreatedAt  time.Time       `schema:"column=created_at;format=02-01-2006"`
	UpdatedAt  time.Time       `schema:"column=updated_at"`
	DeletedAt  *time.Time      `schema:"column=deleted_at"`
	Title      string          `schema:"column=title"`
	Address    string          `schema:"column=address"`
	GeoJSON    json.RawMessage `schema:"column=location"`
	OrderIndex *int64          `schema:"column=order_index"`

	Groups []LocationGroupQueryBuilder `schema:"many2many:location_group_members:location_groups"`
}

func (LocationQueryBuilder) TableName() string { return "locations" }

func (LocationQueryBuilder) Laterals() []querybuilder.LateralSpec {
	return []querybuilder.LateralSpec{
		{
			Columns: map[string]string{"groupcount": "lgc.group_count"},
			SQL: `
				SELECT count(*) AS group_count
				FROM location_group_members lgm
				WHERE lgm.location_id = t.id
			`,
			Alias: "lgc",
			Kind:  "LEFT",
		},
	}
}

type LocationGroupQueryBuilder struct {
	ID        int64      `schema:"column=id"`
	CreatedAt time.Time  `schema:"column=created_at;format=02-01-2006 15:04:05"`
	UpdatedAt time.Time  `schema:"column=updated_at"`
	DeletedAt *time.Time `schema:"column=deleted_at"`
	Name      string     `schema:"column=name"`

	Locations []LocationQueryBuilder `schema:"many2many:location_group_members:locations"`
}

func (LocationGroupQueryBuilder) TableName() string { return "location_groups" }

type LocationGroupMembersQueryBuilder struct {
	LocationGroupID int64 `schema:"column=location_group_id"`
	LocationID      int64 `schema:"column=location_id"`
}

func (LocationGroupMembersQueryBuilder) TableName() string { return "location_group_members" }
