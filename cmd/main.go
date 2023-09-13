package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/grindlemire/go-lucene"
	"github.com/grindlemire/go-lucene/pkg/driver"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Printf("Please provide a lucene query\n")
		os.Exit(1)
	}

	e, err := lucene.Parse(os.Args[1])
	if err != nil {
		fmt.Printf("Error parsing: %s\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Parsed  input: %s\n", e)
	fmt.Fprintf(os.Stderr, "Verbose input: %#v\n", e)

	s, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		fmt.Printf("Error marshalling to json: %s\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "\n%s\n\n", s)

	m := map[string]string{}
	if len(os.Args) > 2 {
		err = json.Unmarshal([]byte(os.Args[2]), &m)
		if err != nil {
			fmt.Printf("Error unmarshalling to joins_field json: %s\nexample\n{\"answer.author\": \"answer\"}\n", err)
			os.Exit(1)
		}
	}

	fmt.Fprintf(os.Stderr, "curl -X GET \"localhost:9200/_search?pretty\" -H 'Content-Type: application/json' -d'\n")
	dsl, err := driver.ElasticDSLDriver{Fields: m}.RenderToString(e)
	if err != nil {
		fmt.Printf("Error rendering to DSL: %s\n", err)
		os.Exit(1)
	}
	fmt.Printf("%s\n", dsl)
	fmt.Fprintf(os.Stderr, "'\n")
}
