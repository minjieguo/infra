package database

import "gorm.io/gorm"

type QueryBuilder struct {
	where  []Condition
	or     [][]Condition
	order  []string
	paging *pagingParam
}

type QueryOperator int

const (
	OpEqual QueryOperator = iota
	OpNotEqual
	OpNotEqualAlt
	OpBetterThan
	OpBetterOrEqual
	OpLessThan
	OpLessOrEqual
	OpLike
	OpNotLike
	OpIn
	OpNotIn
	OpIsNull
)

type Condition struct {
	Field    string
	Operator QueryOperator
	Value    any
}

type pagingParam struct {
	index int
	size  int
	count *int64
}

func NewQueryBuilder() *QueryBuilder {
	return &QueryBuilder{}
}

func (c *Client) NewQueryBuilder() *QueryBuilder {
	return NewQueryBuilder()
}

func (qb *QueryBuilder) Where(field string, operator QueryOperator, value any) *QueryBuilder {
	qb.where = append(qb.where, Condition{
		Field:    field,
		Operator: operator,
		Value:    value,
	})
	return qb
}

func (qb *QueryBuilder) WhereEqual(field string, value any) *QueryBuilder {
	return qb.Where(field, OpEqual, value)
}

func (qb *QueryBuilder) WhereNotEqual(field string, value any) *QueryBuilder {
	return qb.Where(field, OpNotEqual, value)
}

func (qb *QueryBuilder) WhereGreaterThan(field string, value any) *QueryBuilder {
	return qb.Where(field, OpBetterThan, value)
}

func (qb *QueryBuilder) WhereGreaterOrEqual(field string, value any) *QueryBuilder {
	return qb.Where(field, OpBetterOrEqual, value)
}

func (qb *QueryBuilder) WhereLessThan(field string, value any) *QueryBuilder {
	return qb.Where(field, OpLessThan, value)
}

func (qb *QueryBuilder) WhereLessOrEqual(field string, value any) *QueryBuilder {
	return qb.Where(field, OpLessOrEqual, value)
}

func (qb *QueryBuilder) WhereLike(field string, value any) *QueryBuilder {
	return qb.Where(field, OpLike, value)
}

func (qb *QueryBuilder) WhereNotLike(field string, value any) *QueryBuilder {
	return qb.Where(field, OpNotLike, value)
}

func (qb *QueryBuilder) WhereIn(field string, value any) *QueryBuilder {
	return qb.Where(field, OpIn, value)
}

func (qb *QueryBuilder) WhereNotIn(field string, value any) *QueryBuilder {
	return qb.Where(field, OpNotIn, value)
}

func (qb *QueryBuilder) WhereIsNull(field string) *QueryBuilder {
	return qb.Where(field, OpIsNull, nil)
}

func (qb *QueryBuilder) Or(conditions []Condition) *QueryBuilder {
	qb.or = append(qb.or, conditions)
	return qb
}

func (qb *QueryBuilder) Order(expr string) *QueryBuilder {
	qb.order = append(qb.order, expr)
	return qb
}

func (qb *QueryBuilder) Paging(index, size int, count *int64) *QueryBuilder {
	if index <= 0 {
		index = 1
	}
	if size <= 0 {
		size = 20
	}
	if size > 1000 {
		size = 1000
	}
	qb.paging = &pagingParam{index: index, size: size, count: count}
	return qb
}

func (qb *QueryBuilder) Build(client *Client) *gorm.DB {
	if client == nil {
		return nil
	}
	return qb.Apply(client.DB())
}

func (qb *QueryBuilder) Apply(db *gorm.DB) *gorm.DB {
	if db == nil {
		return nil
	}
	ret := db
	for _, condition := range qb.where {
		ret = condition.apply(ret, false)
	}
	for _, v := range qb.or {
		if len(v) == 0 {
			continue
		}
		ordb := db.Session(&gorm.Session{NewDB: true})
		for _, condition := range v {
			ordb = condition.apply(ordb, true)
		}
		ret = ret.Where(ordb)
	}
	if qb.paging != nil {
		if qb.paging.count != nil {
			ret = ret.Count(qb.paging.count)
		}
		offset := (qb.paging.index - 1) * qb.paging.size
		ret = ret.Limit(qb.paging.size).Offset(offset)
	}
	for _, orderExpr := range qb.order {
		ret = ret.Order(orderExpr)
	}
	return ret
}

func (condition Condition) apply(db *gorm.DB, or bool) *gorm.DB {
	expr := condition.Field + " " + condition.Operator.SQL()
	if condition.Operator.WithoutValue() {
		if or {
			return db.Or(expr)
		}
		return db.Where(expr)
	}
	if or {
		return db.Or(expr, condition.Value)
	}
	return db.Where(expr, condition.Value)
}

func (op QueryOperator) SQL() string {
	switch op {
	case OpEqual:
		return "= ?"
	case OpNotEqual:
		return "!= ?"
	case OpNotEqualAlt:
		return "<> ?"
	case OpBetterThan:
		return "> ?"
	case OpBetterOrEqual:
		return ">= ?"
	case OpLessThan:
		return "< ?"
	case OpLessOrEqual:
		return "<= ?"
	case OpLike:
		return "LIKE ?"
	case OpNotLike:
		return "NOT LIKE ?"
	case OpIn:
		return "IN ?"
	case OpNotIn:
		return "NOT IN ?"
	case OpIsNull:
		return "IS NULL"
	default:
		return "= ?"
	}
}

func (op QueryOperator) WithoutValue() bool {
	return op == OpIsNull
}
