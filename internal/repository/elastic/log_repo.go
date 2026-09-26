// Package elastic acts as a repository for elasticsearch
package elastic

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"strconv"

	"github.com/anuraghagawane/luma/internal/domain"
	"github.com/elastic/elastic-transport-go/v8/elastictransport"
	"github.com/elastic/go-elasticsearch/v9"
	"github.com/elastic/go-elasticsearch/v9/typedapi/esdsl"
	"github.com/elastic/go-elasticsearch/v9/typedapi/types"
)

type LogRepo struct {
	client *elasticsearch.TypedClient
	index  string
}

func NewLogRepo(addresses []string, index string) (*LogRepo, error) {
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
			esdsl.NewTypeMapping().AddProperty("eventid", esdsl.NewKeywordProperty()).AddProperty("tenant", esdsl.NewKeywordProperty()).AddProperty("service", esdsl.NewKeywordProperty()).AddProperty("host", esdsl.NewKeywordProperty()).AddProperty("message", esdsl.NewTextProperty()).AddProperty("timestamp", esdsl.NewDateProperty()).AddProperty("loglevel", esdsl.NewKeywordProperty())).Do(context.Background())
		if err != nil {
			log.Fatal("Failed to create index", err)
		}
	}

	return &LogRepo{client, index}, nil
}

func (r *LogRepo) Index(ctx context.Context, id string, document domain.Log) error {
	_, err := r.client.Index(r.index).Id(id).Document(document).Do(ctx)
	if err != nil {
		return convertESErrorToDomain(err)
	}
	return nil
}

func convertESErrorToDomain(err error) error {
	var apiErr *types.ElasticsearchError
	if errors.As(err, &apiErr) {
		if apiErr.Status >= 500 || apiErr.Status == 429 {
			return &domain.LogError{
				Type:    domain.ErrorTypeRetryable,
				Message: err.Error(),
				Code:    apiErr.Status,
			}
		}

		if apiErr.Status >= 400 && apiErr.Status < 500 {
			return &domain.LogError{
				Type:    domain.ErrorTypePermanent,
				Message: err.Error(),
				Code:    apiErr.Status,
			}
		}
	}

	var netErr net.Error

	if errors.As(err, &netErr) && netErr.Timeout() {
		return &domain.LogError{
			Type:    domain.ErrorTypeRetryable,
			Message: err.Error(),
		}
	}

	return &domain.LogError{
		Type:    domain.ErrorTypeRetryable,
		Message: err.Error(),
	}
}

func (r *LogRepo) Query(ctx context.Context, logQuery domain.LogQuery) ([]domain.Log, error) {
	filters := []types.QueryVariant{
		esdsl.NewTermQuery("tenant", esdsl.NewFieldValue().String(logQuery.Tenant)),
		esdsl.NewDateRangeQuery("timestamp").
			Gte(strconv.FormatInt(logQuery.From, 10)).
			Lte(strconv.FormatInt(logQuery.To, 10)),
	}

	if logQuery.LogLevel != "" {
		filters = append(filters, esdsl.NewTermQuery("loglevel", esdsl.NewFieldValue().String(string(logQuery.LogLevel))))
	}

	if logQuery.Service != "" {
		filters = append(filters, esdsl.NewTermQuery("service", esdsl.NewFieldValue().String(logQuery.Service)))
	}

	if logQuery.Keyword != "" {
		filters = append(filters, esdsl.NewMatchQuery("message", logQuery.Keyword))
	}

	res, err := r.client.Search().Index(r.index).Query(esdsl.NewBoolQuery().Filter(filters...)).Do(ctx)
	if err != nil {
		log.Println("Failed to search", err)
		return nil, err
	}

	foundLogs := []domain.Log{}

	for _, rawLog := range res.Hits.Hits {
		var formattedLog domain.Log
		err := json.Unmarshal(rawLog.Source_, &formattedLog)
		if err != nil {
			log.Printf("Failed to unmarshal es response: %v", err)
			return nil, err
		}
		foundLogs = append(foundLogs, formattedLog)
	}

	return foundLogs, nil
}
