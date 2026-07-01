package main

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/aaron70/goaty/utils"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second * 10)
	defer cancel()
	es := utils.Must(elasticsearch.NewClient(elasticsearch.Config{}))

 	req := esapi.IndexRequest{
 		Index:      "test",
		Body:       bytes.NewBufferString(`{ "name": "test2" }`),
 	}

	res := utils.Must(req.Do(ctx, es))

	fmt.Printf("%+v\n", res)
}
