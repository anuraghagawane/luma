// Package elastic acts as a repository for elasticsearch
package elastic

import (
	"context"
	"crypto/tls"
	"encoding/base64"
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
	"github.com/elastic/go-elasticsearch/v9/typedapi/types/enums/sortorder"
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

type cursorData struct {
	PitID       string             `json:"pit_id"`
	SearchAfter []types.FieldValue `json:"search_after,omitempty"`
}

func (r *LogRepo) Query(ctx context.Context, logQuery domain.LogQuery) (*domain.QueryResponse, error) {
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

	var cursor cursorData
	if logQuery.Cursor != "" {
		if err := decodeCursor(logQuery.Cursor, &cursor); err != nil {
			log.Println("Failed to decode cursor", err)
			return nil, err
		}
	} else {
		pit, err := r.client.OpenPointInTime(r.index).KeepAlive("5m").Do(ctx)
		if err != nil {
			log.Println("Failed to created pit", err)
			return nil, err
		}
		cursor.PitID = pit.Id
	}

	req := r.client.Search().Query(esdsl.NewBoolQuery().Filter(filters...)).Pit(esdsl.NewPointInTimeReference().Id(cursor.PitID).KeepAlive(esdsl.NewDuration().String("5m"))).Sort(esdsl.NewSortOptions().AddSortOption("timestamp", esdsl.NewFieldSort(sortorder.Asc)), esdsl.NewSortOptions().AddSortOption("_shard_doc", esdsl.NewFieldSort(sortorder.Asc))).Size(int(logQuery.Limit))

	if len(cursor.SearchAfter) > 0 {
		req = req.SearchAfterValues(cursor.SearchAfter)
	}

	res, err := req.Do(ctx)
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

	nextCursor := cursorData{PitID: cursor.PitID}

	if len(res.Hits.Hits) > 0 {
		nextCursor.SearchAfter = res.Hits.Hits[len(res.Hits.Hits)-1].Sort
	}
	if res.PitId != nil {
		nextCursor.PitID = *res.PitId // ES may rotate the pit id — always carry the latest forward
	}

	var encodedCursor *string
	if len(res.Hits.Hits) > 0 {
		encodedCursor, err = encodeCursor(nextCursor)
	} else {
		encodedCursor = nil
		_, _ = r.client.ClosePointInTime().Id(nextCursor.PitID).Do(ctx)
	}
	if err != nil {
		return nil, err
	}

	return &domain.QueryResponse{Cursor: encodedCursor, Logs: foundLogs}, nil
}

func encodeCursor(c cursorData) (*string, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}

	encoded := base64.StdEncoding.EncodeToString(b)

	return &encoded, nil
}

func decodeCursor(s string, c *cursorData) error {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return err
	}

	return json.Unmarshal(b, c)
}
