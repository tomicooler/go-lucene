# lucene query -> elastic DSL transformer PoC

DO NOT USE IT. Not working yet, need to re-do the tranform.


The goal is to transform lucene query to elastic DSL. Fields can be marked to support has\_child queries (join\_type).


example:
```
 ./main "field_1:value_1 OR field_2:value_2 AND field_3:fazzy~"
Parsed  input: field_1:value_1 OR field_2:value_2 AND field_3:fazzy~
Verbose input: (LITERAL(COLUMN(field_1)):LITERAL("value_1")) OR ((LITERAL(COLUMN(field_2)):LITERAL("value_2")) AND (FUZZY(LITERAL(COLUMN(field_3)):LITERAL("fazzy"))))

{
  "left": {
    "left": "field_1",
    "operator": "EQUALS",
    "right": "value_1"
  },
  "operator": "OR",
  "right": {
    "left": {
      "left": "field_2",
      "operator": "EQUALS",
      "right": "value_2"
    },
    "operator": "AND",
    "right": {
      "left": {
        "left": "field_3",
        "operator": "EQUALS",
        "right": "fazzy"
      },
      "operator": "FUZZY"
    }
  }
}

curl -X GET "localhost:9200/_search?pretty" -H 'Content-Type: application/json' -d'
{
    "query": {
        "bool": {
            "should": [
                {
                    "query_string": {
                        "fields": [
                            "field_1"
                        ],
                        "lenient": true,
                        "query": "value_1"
                    }
                },
                {
                    "bool": {
                        "must": [
                            {
                                "query_string": {
                                    "fields": [
                                        "field_2"
                                    ],
                                    "lenient": true,
                                    "query": "value_2"
                                }
                            },
                            {
                                "query_string": {
                                    "fields": [
                                        "field_3"
                                    ],
                                    "lenient": true,
                                    "query": "fazzy~"
                                }
                            }
                        ]
                    }
                }
            ]
        }
    }
}
'
```

example with join\_field
```
./main "field_1:value_1 OR log.content:excaption~" '{"log.content": "log"}'
Parsed  input: field_1:value_1 OR log.content:excaption~
Verbose input: (LITERAL(COLUMN(field_1)):LITERAL("value_1")) OR (FUZZY(LITERAL(COLUMN(log.content)):LITERAL("excaption")))

{
  "left": {
    "left": "field_1",
    "operator": "EQUALS",
    "right": "value_1"
  },
  "operator": "OR",
  "right": {
    "left": {
      "left": "log.content",
      "operator": "EQUALS",
      "right": "excaption"
    },
    "operator": "FUZZY"
  }
}

curl -X GET "localhost:9200/_search?pretty" -H 'Content-Type: application/json' -d'
{
    "query": {
        "bool": {
            "should": [
                {
                    "query_string": {
                        "fields": [
                            "field_1"
                        ],
                        "lenient": true,
                        "query": "value_1"
                    }
                },
                {
                    "has_child": {
                        "query": {
                            "query_string": {
                                "fields": [
                                    "log.content"
                                ],
                                "lenient": true,
                                "query": "excaption~"
                            }
                        },
                        "type": "log"
                    }
                }
            ]
        }
    }
}
'
```


# go-lucene

A lucene parser written in go with no dependencies.

With this package you can quickly integrate lucene style searching inside your app and generate sql filters for a particular query. There are no external dependencies and the grammar fully supports [Apache Lucene 9.4.2](https://lucene.apache.org/core/9_4_2/queryparser/org/apache/lucene/queryparser/classic/package-summary.html#package.description).

Out of the box go-lucene support postgres compliant sql generation but it can be extended to support different flavors of sql (or no sql) as well.

# Usage

```go
// suppose you want a query for a red apple that is not a honey crisp and is younger than 5 months old
myQuery := `color:red AND (NOT type:"honey crisp" OR age_in_months:[5 TO *])`
expression, err := lucene.Parse(myQuery)
if err != nil {
    // handle error
}

filter, err := driver.NewPostgresDriver().Render(expression)
if err != nil {
    // handle error
}

SQLTemplate := `
SELECT *
FROM apples
WHERE %s
LIMIT 10;
`
sqlQuery := fmt.Sprintf(SQLTemplate, filter)

// sqlQuery is:
`
SELECT *
FROM apples
WHERE
    color = red
    AND (
      NOT(type = 'honey crisp')
      OR age_in_months >= 5
    )
LIMIT 10;
`
```

## Extending with a custom driver

Just embed the `Base` driver in your custom driver and override the `RenderFN`'s with your own custom rendering functions. Please contribute drivers back so others can use it too :).

```Go
import (
    "github.com/grindlemire/go-lucene/pkg/driver"
    "github.com/grindlemire/go-lucene/pkg/lucene/expr"
)

// MyDriver ...
type MyDriver struct {
	Base
}

// NewMyDriver ...
func NewMyDriver() MyDriver {
	fns := map[expr.Operator]driver.RenderFN{
        // suppose we have our own literal rendering function
		expr.Literal: myLiteral,
	}

	for op, sharedFN := range driver.Shared {
		_, found := fns[op]
		if !found {
			fns[op] = sharedFN
		}
	}

	return MyDriver{
		driver.Base{
			renderFNs: fns,
		},
	}
}

// myLiteral ...
func myLiteral(left, right string) (string, error) {
    // ....
}
```
