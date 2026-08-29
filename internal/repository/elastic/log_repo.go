// Package elastic acts as a repository for elasticsearch
package elastic

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"

	"github.com/anuraghagawane/luma/internal/domain"
	"github.com/elastic/elastic-transport-go/v8/elastictransport"
	"github.com/elastic/go-elasticsearch/v9"
	"github.com/elastic/go-elasticsearch/v9/typedapi/esdsl"
)

type LogRepo struct {
	client *elasticsearch.TypedClient
	index  string
}

func NewLogRepo(addresses []string, index string) (*LogRepo, error) {
	fmt.Println(addresses, index)
	client, err := elasticsearch.NewTyped(
		elasticsearch.WithAddresses(addresses...),
		elasticsearch.WithTransportOptions(
			elastictransport.WithTransport(&http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true, // Disables TLS certificate validation
				},
			}),
		),
	)
	if err != nil {
		return nil, err
	}

	// create index if not present
	exists, err := client.Indices.Exists(index).Do(context.Background())
	if err != nil {
		log.Fatal("Exists check failed: ", err)
	}
	if !exists {
		_, err = client.Indices.Create(index).Mappings(
			esdsl.NewTypeMapping().AddProperty("eventid", esdsl.NewKeywordProperty()).AddProperty("tenant", esdsl.NewKeywordProperty()).AddProperty("host", esdsl.NewKeywordProperty()).AddProperty("message", esdsl.NewTextProperty()).AddProperty("timestamp", esdsl.NewDateProperty()).AddProperty("loglevel", esdsl.NewKeywordProperty())).Do(context.Background())
		if err != nil {
			log.Fatal("Failed to create index", err)
		}
	}

	return &LogRepo{client, index}, nil
}

func (r *LogRepo) Index(ctx context.Context, id string, document domain.Log) error {
	_, err := r.client.Index(r.index).Id(id).Document(document).Do(ctx)
	return err
}
