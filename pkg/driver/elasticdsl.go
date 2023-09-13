package driver

import (
	"encoding/json"
	"fmt"
	"github.com/grindlemire/go-lucene/pkg/lucene/expr"
	"strings"
)

// TODO: refactor and simplify the rendering process
//       expression has some SQL related things like Column
//       top-down instead of bottom-up?

// NOTE: THIS IS JUST A PROOF OF CONCEPT

// ElasticDSLDriver transforms a parsed lucene expression to a elastic DSL query.
type ElasticDSLDriver struct {
	// https://www.elastic.co/guide/en/elasticsearch/reference/current/parent-join.html
	// field -> join_field type map: e.g.: question.id: question, question.name: question, answer.author: answer, answer.content: answer, comment.author: comment, comment.content: comment
	Fields map[string]string
}

type QueryStringData struct {
	Fields  []string `json:"fields"`
	Lenient bool     `json:"lenient"`
	Query   string   `json:"query"`
}

type QueryString struct {
	QueryString QueryStringData `json:"query_string"`
}

type RangeData struct {
	LessThan         interface{} `json:"lt,omitempty"`
	LessThanEqual    interface{} `json:"lte,omitempty"`
	GreaterThan      interface{} `json:"gt,omitempty"`
	GreaterThanEqual interface{} `json:"gte,omitempty"`
}

type Range struct {
	Range map[string]RangeData `json:"range,omitempty"`
}

type BoolData struct {
	Must    interface{} `json:"must,omitempty"`
	Should  interface{} `json:"should,omitempty"`
	Filter  interface{} `json:"filter,omitempty"`
	MustNot interface{} `json:"must_not,omitempty"`
}

type HasChild struct {
	HasChild interface{} `json:"has_child,omitempty"`
}

type HasChildQuery struct {
	Query interface{} `json:"query,omitempty"`
	Type  interface{} `json:"type,omitempty"`
}

type Bool struct {
	Bool BoolData `json:"bool"`
}

func (b ElasticDSLDriver) RenderToString(e *expr.Expression) (s string, err error) {
	transformed, err := b.Render(e, nil)
	if err != nil {
		return s, err
	}

	bytes, err := json.MarshalIndent(map[string]interface{}{"query": transformed}, "", "    ")
	if err != nil {
		return s, err
	}

	return string(bytes), nil
}

func (b ElasticDSLDriver) Render(e *expr.Expression, p *expr.Expression) (s interface{}, err error) {
	if e == nil {
		return "", nil
	}

	pEq := false
	if p != nil {
		pEq = p.Op == expr.Equals ||
			p.Op == expr.Range ||
			p.Op == expr.Greater ||
			p.Op == expr.Less ||
			p.Op == expr.GreaterEq ||
			p.Op == expr.LessEq ||
			p.Op == expr.In ||
			p.Op == expr.Like ||
			p.Op == expr.Fuzzy ||
			p.Op == expr.Boost ||
			p.Op == expr.Regexp ||
			p.Op == expr.Wild
	}

	left, err := b.serialize(e.Left, e)
	if err != nil {
		return nil, err
	}

	right, err := b.serialize(e.Right, e)
	if err != nil {
		return nil, err
	}

	switch e.Op {
	case expr.And:
		return Bool{BoolData{Must: []interface{}{left, right}}}, nil
	case expr.Or:
		return Bool{BoolData{Should: []interface{}{left, right}}}, nil
	case expr.Equals:
		qs, err := makeQueryString(left.(string), right.(string))
		if err != nil {
			return nil, err
		}
		return b.wrapInHasChild(left.(string), qs) // todo implement wrapping on ranges and everywhere else
	case expr.Like:
		return makeQueryString(left.(string), right.(string))
	case expr.Not:
		return Bool{BoolData{MustNot: []interface{}{left}}}, nil
	case expr.Range:
		rb, isRangeBoundary := e.Right.(*expr.RangeBoundary)
		if isRangeBoundary {
			// I died here
			anyMin := false
			switch rb.Min.(type) {
			case *expr.Expression:
				if rb.Min.(*expr.Expression).Op == expr.Wild {
					anyMin = true
				}
			}
			anyMax := false
			switch rb.Max.(type) {
			case *expr.Expression:
				if rb.Max.(*expr.Expression).Op == expr.Wild {
					anyMax = true
				}
			}

			if anyMin {
				if rb.Inclusive {
					return Range{map[string]RangeData{left.(string): {LessThanEqual: rb.Max}}}, nil
				} else {
					return Range{map[string]RangeData{left.(string): {LessThan: rb.Max}}}, nil
				}
			}
			if anyMax {
				if rb.Inclusive {
					return Range{map[string]RangeData{left.(string): {GreaterThanEqual: rb.Min}}}, nil
				} else {
					return Range{map[string]RangeData{left.(string): {GreaterThan: rb.Min}}}, nil
				}
			}

			if rb.Inclusive {
				return Range{map[string]RangeData{left.(string): {LessThanEqual: rb.Max, GreaterThanEqual: rb.Min}}}, nil
			} else {
				return Range{map[string]RangeData{left.(string): {LessThan: rb.Max, GreaterThan: rb.Min}}}, nil
			}
		}
		break
	case expr.Must:
		return Bool{BoolData{Must: []interface{}{left}}}, nil
	case expr.MustNot:
		return Bool{BoolData{MustNot: []interface{}{left}}}, nil
	case expr.Boost:
		// TODO: this is a mess it only works for a:b^2 -> BOOST(LITERAL(COLUMN(run.user)):LITERAL("a")^2.0)
		if exp, isExpr := e.Left.(*expr.Expression); isExpr {
			if exp.Right != nil {
				orig := exp.Right.(*expr.Expression).Left.(string)

				boost := fmt.Sprintf("%s^", orig)
				pwr := e.BoostPower()
				if pwr != 1.0 {
					boost += fmt.Sprintf("%f", pwr)
				}
				exp.Right.(*expr.Expression).Left = boost

				qs, err := makeQueryString(fmt.Sprintf("%s", exp.Left), boost)
				if err != nil {
					return nil, err
				}
				return b.wrapInHasChild(fmt.Sprintf("%s", exp.Left), qs)
			}
			boost := fmt.Sprintf("%s^", left)
			pwr := e.BoostPower()
			if pwr != 1.0 {
				boost += fmt.Sprintf("%f", pwr)
			}
			return makeLiteralOrQueryString(boost, !pEq)
		}
	case expr.Fuzzy: // todo use fuzzy
		// TODO: this is a mess it only works for a:b~2 -> FUZZY(LITERAL(COLUMN(run.user)):LITERAL("a")~2)
		if exp, isExpr := e.Left.(*expr.Expression); isExpr {
			if exp.Right != nil {
				orig := exp.Right.(*expr.Expression).Left.(string)

				fuzzy := fmt.Sprintf("%s~", orig)
				dist := e.FuzzyDistance()
				if dist != 1 {
					fuzzy += fmt.Sprintf("%d", dist)
				}
				exp.Right.(*expr.Expression).Left = fuzzy

				qs, err := makeQueryString(fmt.Sprintf("%s", exp.Left), fuzzy)
				if err != nil {
					return nil, err
				}
				return b.wrapInHasChild(fmt.Sprintf("%s", exp.Left), qs)
			}
			fuzzy := fmt.Sprintf("%s~", left)
			dist := e.FuzzyDistance()
			if dist != 1 {
				fuzzy += fmt.Sprintf("%d", dist)
			}
			return makeLiteralOrQueryString(fuzzy, !pEq)
		}
	case expr.Literal:
		return makeLiteralOrQueryString(left.(string), !pEq)
	case expr.Wild: // todo use wildcard
		return makeLiteralOrQueryString(left.(string), !pEq)
	case expr.Regexp: // todo use regexp
		return makeLiteralOrQueryString(left.(string), !pEq)
	case expr.Greater:
		return Range{map[string]RangeData{left.(string): {GreaterThan: right}}}, nil
	case expr.Less:
		return Range{map[string]RangeData{left.(string): {LessThan: right}}}, nil
	case expr.GreaterEq:
		return Range{map[string]RangeData{left.(string): {GreaterThanEqual: right}}}, nil
	case expr.LessEq:
		return Range{map[string]RangeData{left.(string): {LessThanEqual: right}}}, nil
	case expr.In:
		ll, isList := e.Right.(*expr.Expression).Left.([]*expr.Expression)
		if isList {
			queries := make([]interface{}, 0)
			for _, query := range ll {
				q, err := makeQueryString(left.(string), query.Left.(string))
				if err != nil {
					return nil, err
				}
				queries = append(queries, q)
			}

			return Bool{BoolData{Should: queries}}, nil
		}
		// TODO: in could contain expressions not just list, e.g.: a:(b OR c OR D AND d)
		return e.Left.(string), nil
	case expr.List:
		return e.Left, nil
	}

	return e.Left.(string), nil
}

// https://www.elastic.co/guide/en/elasticsearch/reference/current/query-dsl-query-string-query.html
// reserved: + - = && || > < ! ( ) { } [ ] ^ " ~ * ? : \ /
// < > can't be escaped
func makeQueryString(field string, query string) (interface{}, error) {
	// https://regex101.com/r/IlOU2U/1
	//var re = regexp.MustCompile(`(\+|-|=|&&|\|\||!|\(|\)|{|}|\[|]|\^|"|~|\*|\?|:|\\|/)`)
	//s := re.ReplaceAllString(query, `\$1`)
	//s = strings.ReplaceAll(s, "<", "")
	//s = strings.ReplaceAll(s, ">", "")
	//nope, the user has to do the escaping
	return QueryString{QueryStringData{[]string{field}, true, query}}, nil
}

func (b ElasticDSLDriver) wrapInHasChild(field string, query interface{}) (interface{}, error) {
	if b.Fields != nil {
		if joinType, ok := b.Fields[field]; ok {
			return HasChild{HasChildQuery{query, joinType}}, nil
		}
	}

	return query, nil
}

func makeLiteralOrQueryString(lit string, query bool) (interface{}, error) {
	if query {
		return makeQueryString("*", lit)
	}

	return lit, nil
}

func (b ElasticDSLDriver) serialize(in any, p *expr.Expression) (s interface{}, err error) {
	if in == nil {
		return "", nil
	}

	switch v := in.(type) {
	case *expr.Expression:
		return b.Render(v, p)
	case []*expr.Expression:
		return v, nil
	case *expr.RangeBoundary:
		return v, nil
	case expr.Column:
		ss := strings.ReplaceAll(string(v), " ", "\\ ")
		return fmt.Sprintf("%s", ss), nil
	case string:
		return fmt.Sprintf("%s", v), nil
	default:
		return fmt.Sprintf("%v", v), nil
	}
}
