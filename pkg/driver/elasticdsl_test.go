package driver

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/grindlemire/go-lucene/pkg/lucene/expr"
)

func TestElasticDSLDriver(t *testing.T) {
	type tc struct {
		input *expr.Expression
		want  string
	}

	tcs := map[string]tc{
		"simple_equals": {
			input: expr.Eq("a", 5),
			want: `
			{
				"query": {
					"query_string": {
						"fields": [
							"a"
						],
						"lenient": true,
						"query": "5"
					}
				}
			}`,
		},
		"simple_and": {
			input: expr.AND(expr.Eq("a", 5), expr.Eq("b", "foo")),
			want: `
			{
				"query": {
					"bool": {
						"must": [
							{
								"query_string": {
									"fields": [
										"a"
									],
									"lenient": true,
									"query": "5"
								}
							},
							{
								"query_string": {
									"fields": [
										"b"
									],
									"lenient": true,
									"query": "foo"
								}
							}
						]
					}
				}
			}`,
		},
		"nested_and": {
			input: expr.AND(expr.Eq("a", 5), expr.AND(expr.Eq("b", 5), expr.Eq("c", "foo"))),
			want: `
			{
				"query": {
					"bool": {
						"must": [
							{
								"query_string": {
									"fields": [
										"a"
									],
									"lenient": true,
									"query": "5"
								}
							},
							{
								"bool": {
									"must": [
										{
											"query_string": {
												"fields": [
													"b"
												],
												"lenient": true,
												"query": "5"
											}
										},
										{
											"query_string": {
												"fields": [
													"c"
												],
												"lenient": true,
												"query": "foo"
											}
										}
									]
								}
							}
						]
					}
				}
			}`,
		},
		"simple_or": {
			input: expr.OR(expr.Eq("a", 5), expr.Eq("b", "foo")),
			want: `
			{
				"query": {
					"bool": {
						"should": [
							{
								"query_string": {
									"fields": [
										"a"
									],
									"lenient": true,
									"query": "5"
								}
							},
							{
								"query_string": {
									"fields": [
										"b"
									],
									"lenient": true,
									"query": "foo"
								}
							}
						]
					}
				}
			}`,
		},
		"simple_not": {
			input: expr.NOT(expr.Eq("a", 1)),
			want: `
			{
				"query": {
					"bool": {
						"must_not": [
							{
								"query_string": {
									"fields": [
										"a"
									],
									"lenient": true,
									"query": "1"
								}
							}
						]
					}
				}
			}`,
		},
		"simple_like": {
			input: expr.LIKE("a", "%(b|d)%"),
			want: `
			{
				"query": {
					"query_string": {
						"fields": [
							"a"
						],
						"lenient": true,
						"query": "%(b|d)%"
					}
				}
			}`,
		},
		"string_range": {
			input: expr.Rang("a", "foo", "bar", true),
			want: `
			{
				"query": {
					"range": {
						"a": {
							"lte": "bar",
							"gte": "foo"
						}
					}
				}
			}`,
		},
		"mixed_number_range": {
			input: expr.Rang("a", 1.1, 10, true),
			want: `
			{
				"query": {
					"range": {
						"a": {
							"lte": 10,
							"gte": 1.1
						}
					}
				}
			}`,
		},
		"mixed_number_range_exlusive": {
			input: expr.Rang("a", 1, 10.1, false),
			want: `
			{
				"query": {
					"range": {
						"a": {
							"lt": 10.1,
							"gt": 1
						}
					}
				}
			}`,
		},
		"int_range": {
			input: expr.Rang("a", 1, 10, true),
			want: `
			{
				"query": {
					"range": {
						"a": {
							"lte": 10,
							"gte": 1
						}
					}
				}
			}`,
		},
		"int_range_exclusive": {
			input: expr.Rang("a", 1, 10, false),
			want: `
			{
				"query": {
					"range": {
						"a": {
							"lt": 10,
							"gt": 1
						}
					}
				}
			}`,
		},
		"float_range": {
			input: expr.Rang("a", 1.0, 10.0, true),
			want: `
			{
				"query": {
					"range": {
						"a": {
							"lte": 10.0,
							"gte": 1.0
						}
					}
				}
			}`,
		},
		"float_range_exclusive": {
			input: expr.Rang("a", 1.0, 10.0, false),
			want: `
			{
				"query": {
					"range": {
						"a": {
							"lt": 10,
							"gt": 1.0
						}
					}
				}
			}`,
		},
		"lt_range": {
			input: expr.Rang("a", "*", 10, false),
			want: `
			{
				"query": {
					"range": {
						"a": {
							"lt": 10
						}
					}
				}
			}`,
		},
		"lte_range": {
			input: expr.Rang("a", "*", 10, true),
			want: `
			{
				"query": {
					"range": {
						"a": {
							"lte": 10
						}
					}
				}
			}`,
		},
		"gt_range": {
			input: expr.Rang("a", 1, "*", false),
			want: `
			{
				"query": {
					"range": {
						"a": {
							"gt": 1
						}
					}
				}
			}`,
		},
		"gte_range": {
			input: expr.Rang("a", 1, "*", true),
			want: `
			{
				"query": {
					"range": {
						"a": {
							"gte": 1
						}
					}
				}
			}`,
		},
		"lt": {
			input: expr.LESS("a", 10),
			want: `
			{
				"query": {
					"range": {
						"a": {
							"lt": "10"
						}
					}
				}
			}`,
		},
		"lte": {
			input: expr.LESSEQ("a", 10),
			want: `
			{
				"query": {
					"range": {
						"a": {
							"lte": "10"
						}
					}
				}
			}`,
		},
		"gt": {
			input: expr.GREATER("a", 10),
			want: `
			{
				"query": {
					"range": {
						"a": {
							"gt": "10"
						}
					}
				}
			}`,
		},
		"gte": {
			input: expr.GREATEREQ("a", 10),
			want: `
			{
				"query": {
					"range": {
						"a": {
							"gte": "10"
						}
					}
				}
			}`,
		},
		"must_ignored": {
			input: expr.MUST(expr.Eq("a", 1)),
			want: `
			{
				"query": {
					"bool": {
						"must": [
							{
								"query_string": {
									"fields": [
										"a"
									],
									"lenient": true,
									"query": "1"
								}
							}
						]
					}
				}
			}`,
		},
		"fuzzy_equals": {
			input: expr.FUZZY(expr.Eq("a", "smthg"), 2),
			want: `
			{
				"query": {
					"query_string": {
						"fields": [
							"a"
						],
						"lenient": true,
						"query": "smthg~2"
					}
				}
			}`,
		},
		"nested_filter": {
			input: expr.Expr(
				expr.Expr(
					expr.Expr(
						"a",
						expr.Equals,
						"foo",
					),
					expr.Or,
					expr.Expr(
						"b",
						expr.Equals,
						expr.REGEXP("/b*ar/"),
					),
				),
				expr.And,
				expr.Expr(
					expr.Rang("c", "aaa", "*", false),
					expr.Not,
				),
			),
			want: `
			{
				"query": {
					"bool": {
						"must": [
							{
								"bool": {
									"should": [
										{
											"query_string": {
												"fields": [
													"a"
												],
												"lenient": true,
												"query": "foo"
											}
										},
										{
											"query_string": {
												"fields": [
													"b"
												],
												"lenient": true,
												"query": "/b*ar/"
											}
										}
									]
								}
							},
							{
								"bool": {
									"must_not": [
										{
											"range": {
												"c": {
													"gt": "aaa"
												}
											}
										}
									]
								}
							}
						]
					}
				}
			}`,
		},
		"space_in_fieldname": {
			input: expr.Eq("a b", 1),
			want: `
			{
				"query": {
					"query_string": {
						"fields": [
							"a\\ b"
						],
						"lenient": true,
						"query": "1"
					}
				}
			}`,
		},
		"quoted_column_name": {
			input: expr.Eq(`"foobar"`, 1),
			want: `
			{
				"query": {
					"query_string": {
						"fields": [
							"\"foobar\""
						],
						"lenient": true,
						"query": "1"
					}
				}
			}`,
		},
		"simple_literal": {
			input: expr.Lit("a"),
			want: `
			{
				"query": {
					"query_string": {
						"fields": [
							"*"
						],
						"lenient": true,
						"query": "a"
					}
				}
			}`,
		},
		"a_or_b_without_fields": {
			input: expr.OR("a", "b"),
			want: `
			{
				"query": {
					"bool": {
						"should": [
							{
								"query_string": {
									"fields": [
										"*"
									],
									"lenient": true,
									"query": "a"
								}
							},
							{
								"query_string": {
									"fields": [
										"*"
									],
									"lenient": true,
									"query": "b"
								}
							}
						]
					}
				}
			}`,
		},
		"a_or_b_expr": {
			input: expr.OR("a", expr.Eq("b", "c")),
			want: `
			{
				"query": {
					"bool": {
						"should": [
							{
								"query_string": {
									"fields": [
										"*"
									],
									"lenient": true,
									"query": "a"
								}
							},
							{
								"query_string": {
									"fields": [
										"b"
									],
									"lenient": true,
									"query": "c"
								}
							}
						]
					}
				}
			}`,
		},
		"simple_fuzzy": {
			input: expr.FUZZY("foo", 2),
			want: `
			{
				"query": {
					"query_string": {
						"fields": [
							"*"
						],
						"lenient": true,
						"query": "foo~2"
					}
				}
			}`,
		},
		"simple_boost": {
			input: expr.BOOST("foo", 2),
			want: `
			{
				"query": {
					"query_string": {
						"fields": [
							"*"
						],
						"lenient": true,
						"query": "foo^2.000000"
					}
				}
			}`,
		},
		"simple_regexp": {
			input: expr.REGEXP("/a/"),
			want: `
			{
				"query": {
					"query_string": {
						"fields": [
							"*"
						],
						"lenient": true,
						"query": "/a/"
					}
				}
			}`,
		},
		"simple_wild": {
			input: expr.REGEXP("a*"),
			want: `
			{
				"query": {
					"query_string": {
						"fields": [
							"*"
						],
						"lenient": true,
						"query": "a*"
					}
				}
			}`,
		},
		"in_list": {
			input: expr.IN(
				"a",
				expr.LIST(
					expr.Lit("foo"),
					expr.Lit("baz"),
					expr.Lit("bar"),
				),
			),
			want: `
			{
				"query": {
					"bool": {
						"should": [
							{
								"query_string": {
									"fields": [
										"a"
									],
									"lenient": true,
									"query": "foo"
								}
							},
							{
								"query_string": {
									"fields": [
										"a"
									],
									"lenient": true,
									"query": "baz"
								}
							},
							{
								"query_string": {
									"fields": [
										"a"
									],
									"lenient": true,
									"query": "bar"
								}
							}
						]
					}
				}
			}`,
		},
		"nested_and_has_child": {
			input: expr.AND(expr.Eq("question.text", "sup?"), expr.AND(expr.Eq("answer.author", "sudo"), expr.Eq("c", "foo"))),
			want: `
			{
				"query": {
					"bool": {
						"must": [
							{
								"query_string": {
									"fields": [
										"question.text"
									],
									"lenient": true,
									"query": "sup?"
								}
							},
							{
								"bool": {
									"must": [
										{
											"has_child": {
												"query": {
													"query_string": {
														"fields": [
															"answer.author"
														],
														"lenient": true,
														"query": "sudo"
													}
												},
												"type": "answer"
											}
										},
										{
											"query_string": {
												"fields": [
													"c"
												],
												"lenient": true,
												"query": "foo"
											}
										}
									]
								}
							}
						]
					}
				}
			}`,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {

			s, err := json.MarshalIndent(tc.input, "", "  ")
			if err != nil {
				fmt.Printf("Error marshalling to json: %s\n", err)
				os.Exit(1)
			}

			fmt.Printf("%s\n", s)

			got, err := ElasticDSLDriver{
				Fields: map[string]string{
					"answer.author":   "answer",
					"answer.content":  "answer",
					"comment.author":  "comment",
					"comment.content": "comment",
				},
			}.RenderToString(tc.input)
			if err != nil {
				t.Fatalf("got an unexpected error when rendering: %v", err)
			}

			var m interface{}
			err = json.Unmarshal([]byte(tc.want), &m)
			if err != nil {
				t.Fatalf("could not unmarshal %v", err)
			}

			want, err := json.MarshalIndent(m, "", "    ")
			if err != nil {
				t.Fatalf("could not marshal %v", err)
			}

			var a interface{}
			err = json.Unmarshal([]byte(got), &a)
			if err != nil {
				t.Fatalf("could not unmarshal %v", err)
			}

			if !reflect.DeepEqual(m, a) {
				t.Fatalf("%s\n===expected===\n%s\n===actual===\n%s\n", "generated DSL query does not match", want, got)
			}
		})
	}
}
