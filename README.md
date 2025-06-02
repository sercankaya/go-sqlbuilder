# QueryBuilder

## 📘 Description

A flexible and secure Go library designed to generate advanced SQL queries. This project offers dynamic filtering, sorting, pagination, detailed querying on related data (one2many, many2many), and PostgreSQL LATERAL support via REST API. QueryBuilder enables backend developers to easily develop data listing APIs without writing complex queries.

---

## 🚀 Installation

```bash
go get github.com/sercankaya/go-querybuilder
cd go-querybuilder
go mod tidy
```

---

## 🏗️ Usage

### 1. Struct Definition

```go
type LocationQueryBuilder struct {
	ID        int64     `schema:"column=id"`
	Title     string    `schema:"column=title"`
	CreatedAt time.Time `schema:"column=created_at"`
	Groups    []Group   `schema:"many2many:location_group_members:location_groups;join=LEFT"`
}

func (LocationQueryBuilder) TableName() string {
	return "locations"
}
```

```go
// Many2Many example
type UserQueryBuilder struct {
    ID     int64    `schema:"column=id"`
    Name   string   `schema:"column=name"`
    Roles  []Role   `schema:"many2many:user_roles:roles"`
}

func (UserQueryBuilder) TableName() string {
    return "users"
}

type Role struct {
    ID   int64  `schema:"column=id"`
    Name string `schema:"column=name"`
}

func (Role) TableName() string {
    return "roles"
}

// One2Many example
type AuthorQueryBuilder struct {
    ID      int64     `schema:"column=id"`
    Name    string    `schema:"column=name"`
    Books   []Book    `schema:"one2many:author_id:books"`
}

func (AuthorQueryBuilder) TableName() string {
    return "authors"
}

type Book struct {
    ID       int64  `schema:"column=id"`
    Title    string `schema:"column=title"`
    AuthorID int64  `schema:"column=author_id"`
}

func (Book) TableName() string {
    return "books"
}
```

### 2. Simple Listing

```http
POST /locations/list
Content-Type: application/json

{
  "fields": ["ID", "Title"],
  "limit": 10,
  "offset": 0
}
```

### 3. Filtering Operators

| Operator   | Description   | Example                                                    |
| ---------- | ------------- | ---------------------------------------------------------- |
| eq         | Equal         | {"field":"ID", "operator":"eq", "value": 5}                |
| ne         | Not equal     | {"field":"Title", "operator":"ne", "value": "test"}        |
| gt         | Greater than  | {"field":"ID", "operator":"gt", "value": 10}               |
| lt         | Less than     | {"field":"ID", "operator":"lt", "value": 10}               |
| gte        | Greater equal | {"field":"ID", "operator":"gte", "value": 10}              |
| lte        | Less equal    | {"field":"ID", "operator":"lte", "value": 10}              |
| contains   | LIKE %val%    | {"field":"Title", "operator":"contains", "value": "abc"}   |
| startswith | val%          | {"field":"Title", "operator":"startswith", "value": "ab"}  |
| endswith   | %val          | {"field":"Title", "operator":"endswith", "value": "yz"}    |
| isnull     | null check    | {"field":"Title", "operator":"isnull", "value": true}      |
| in         | Multiple equal| {"field":"ID", "operator":"in", "value": [1, 2, 3]}        |

### 4. Date Filtering (date / datetime)

```json
{
  "field": "CreatedAt",
  "operator": "eq",
  "type": "date",
  "value": "01-01-2025"
}
```

Automatically generates a query between the start and end of the day: `BETWEEN '2025-01-01 00:00:00' AND '2025-01-01 23:59:59.999999'`

### 5. many2many and one2many Relationship Examples

```json
{
  "fields": ["ID", "Title", "Groups"],
  "filters": [
    {
      "field": "Groups.Name",
      "operator": "contains",
      "value": "admin"
    }
  ]
}
```

> This example returns related records where the `Name` value inside `Groups` contains "admin".

### 6. Sorting and Pagination

```json
{
  "order_by": "ID",
  "order_direction": "desc",
  "limit": 20,
  "offset": 0
}
```

---

## 🔄 Using LATERAL

If the model implements the `HasLaterals` interface like below, special lateral queries are also supported.

```go
type LocationQueryBuilder struct { ... }

func (LocationQueryBuilder) Laterals() []querybuilder.LateralSpec {
  return []querybuilder.LateralSpec{
    {
      Kind:  "LEFT",
      Alias: "lgc",
      SQL:   "SELECT COUNT(*) AS group_count FROM location_group_members lgm WHERE lgm.location_id = t.id",
      Columns: map[string]string{
        "group_count": "lgc.group_count",
      },
    },
  }
}
```

Request:

```json
{
  "fields": ["ID", "Title", "GroupCount"]
}
```

---

## 🛠️ Entity Generation

For all queries to work correctly, structs must be defined with `schema:"column=..."`, `schema:"many2many:..."`, `schema:"one2many:..."` tags.

If you have an SQL table schema, you can quickly generate structs with tools like `gorm` or `xo` and then manually add tags.

---

## 📌 Notes

* PostgreSQL is supported.
* JSON aggregation is automatic.
* Column names are matched with Go struct field names.
* Secure parameter binding protects against SQL injection.

---

## 🧪 Example curl

```bash
curl -X POST http://localhost:8080/locations/list \
  -H "Content-Type: application/json" \
  -d '{
    "fields": ["ID", "Title", "Groups"],
    "filters": [
      { "field": "Title", "operator": "contains", "value": "office" },
      { "field": "CreatedAt", "operator": "eq", "type": "date", "value": "01-06-2025" }
    ],
    "order_by": "ID",
    "order_direction": "desc",
    "limit": 10,
    "offset": 0
  }'
```
