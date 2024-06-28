package main

import (
	"context"
	"net/http"
	"os"
	"regexp"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/deepmap/oapi-codegen/v2/pkg/securityprovider"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/mattdavis90/immich-stacker/client"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type Stack struct {
	IDs    []openapi_types.UUID
	Parent *openapi_types.UUID
}

func (s Stack) Stackable() bool {
	return s.Parent != nil && len(s.IDs) > 0
}

type Stats struct {
	Stackable    int
	NotStackable int
	Success      int
	Failed       int
}

type Config struct {
	APIKey         string `env:"API_KEY"`
	Endpoint       string `env:"ENDPOINT"`
	Match          string `env:"MATCH"`
	Parent         string `env:"PARENT"`
	LogLevel       string `env:"LOG_LEVEL" envDefault:"INFO"`
	CompareCreated bool   `env:"COMPARE_CREATED" envDefault:"false"`
}

func getEnv(e string) string {
	ret := os.Getenv(e)
	if ret == "" {
		log.Fatal().Str("Env", e).Msg("Missing envvar")
	}
	return ret
}

func main() {
	godotenv.Load()

	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	cfg := Config{}
	opts := env.Options{RequiredIfNoDef: true, Prefix: "IMMICH_"}
	if err := env.ParseWithOptions(&cfg, opts); err != nil {
		log.Fatal().Err(err).Msg("Error loading config")
	}

	level, err := zerolog.ParseLevel(cfg.LogLevel)
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}
	zerolog.SetGlobalLevel(level)

	m, err := regexp.Compile(cfg.Match)
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}

	p, err := regexp.Compile(cfg.Parent)
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}

	sp, err := securityprovider.NewSecurityProviderApiKey("header", "x-api-key", cfg.APIKey)
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}

	log.Info().Str("endpoint", cfg.Endpoint).Msg("Connecting to Immich")

	hc := http.Client{}

	c, err := client.NewClientWithResponses(
		cfg.Endpoint,
		client.WithHTTPClient(&hc),
		client.WithRequestEditorFn(sp.Intercept),
	)
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}

	ctx := context.Background()

    resp, err := c.GetServerVersionWithResponse(ctx)
    if err != nil {
        log.Fatal().Err(err).Msg("")
    }
    if resp.StatusCode() != http.StatusOK {
        log.Fatal().Int("status", resp.StatusCode()).Msg("Expected HTTP 200")
    }
    if resp.JSON200 == nil {
        log.Fatal().Msg("nil return")
    }
    v := resp.JSON200
    log.Info().Int("major", v.Major).Int("minor", v.Minor).Int("patch", v.Patch).Msg("Server version")

	log.Info().Msg("Requesting all assets")

	total := 0
	stacks := make(map[string]*Stack)
	var page float32 = 1

	for {
		t := true
		var s float32 = 1000

		log.Debug().Float32("page", page).Float32("page_size", s).Msg("Requesting next page")
		resp, err := c.SearchMetadataWithResponse(ctx, client.MetadataSearchDto{WithStacked: &t, Size: &s, Page: &page})
		if err != nil {
			log.Fatal().Err(err).Msg("")
		}
		if resp.StatusCode() != http.StatusOK {
			log.Fatal().Int("status", resp.StatusCode()).Msg("Expected HTTP 200")
		}
		if resp.JSON200 == nil {
			log.Fatal().Msg("nil return")
		}

		assets := resp.JSON200.Assets.Items
		l := len(assets)

		log.Debug().Float32("page", page).Int("count", l).Int("total", total).Msg("Retrieved page")

		total += l
		for _, a := range assets {
			if a.StackCount != nil && *a.StackCount > 0 {
				continue
			}

			if m.Match([]byte(a.OriginalFileName)) {
				id := openapi_types.UUID(uuid.MustParse(a.Id))
				key := string(m.ReplaceAll([]byte(a.OriginalFileName), []byte("")))
				if cfg.CompareCreated {
					key += "_" + a.FileCreatedAt.Local().String()
				}

				s, ok := stacks[key]
				if !ok {
					s = &Stack{
						IDs: make([]uuid.UUID, 0),
					}
					stacks[key] = s
				}

				if p.Match([]byte(a.OriginalFileName)) {
					s.Parent = &id
				} else {
					s.IDs = append(s.IDs, id)
				}
			}
		}

		if resp.JSON200.Assets.NextPage == nil {
			break
		}

		page += 1
	}

	log.Info().Int("total", total).Int("matches", len(stacks)).Msg("Retrieved assets")

	stats := Stats{}

	for f, s := range stacks {
		if s.Stackable() {
			stats.Stackable++

			log.Debug().Str("filename", f).Msg("Stacking")
			resp, err := c.UpdateAssetsWithResponse(ctx, client.AssetBulkUpdateDto{
				Ids:           s.IDs,
				StackParentId: s.Parent,
			})
			if err != nil {
				log.Error().Err(err)
				stats.Failed++
			} else if resp.StatusCode() != http.StatusNoContent {
				log.Error().Int("status", resp.StatusCode()).Msg("Expected HTTP 204")
				stats.Failed++
			} else {
				log.Info().Str("filename", f).Msg("Created stack")
				stats.Success++
			}
		} else {
			log.Debug().Str("filename", f).Msg("Skipped")
			stats.NotStackable++
		}
	}

	log.Info().
		Int("stackable", stats.Stackable).
		Int("success", stats.Success).
		Int("failed", stats.Failed).
		Int("not_stackable", stats.NotStackable).
		Msg("Finished")
}
